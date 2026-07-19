package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/kachofugetsu09/nemeton/internal/artifact"
	"github.com/kachofugetsu09/nemeton/internal/event"
)

func (s *Store) Reconcile(ctx context.Context, artifacts artifact.Store) (ReconcileResult, error) {
	// 1. Reject database, event, and artifact corruption that replay cannot repair.
	if err := s.checkDatabaseIntegrity(ctx); err != nil {
		return ReconcileResult{}, err
	}
	streams, err := s.projectStreams(ctx)
	if err != nil {
		return ReconcileResult{}, err
	}
	var maintenance []string
	referencedArtifacts := make(map[string]bool)
	for projectID, version := range streams {
		events, err := loadEvents(ctx, s.db, projectID)
		if err != nil {
			return ReconcileResult{}, err
		}
		if err := validateStream(projectID, version, events); err != nil {
			return ReconcileResult{}, err
		}
		for _, item := range events {
			for _, reference := range item.Artifacts {
				record := artifact.Record{Digest: reference.Digest, MediaType: reference.MediaType, ByteSize: reference.ByteSize, StorageURI: reference.StorageURI}
				if err := artifacts.Verify(record); err != nil {
					return ReconcileResult{}, &Error{Code: "artifact_corrupt", Detail: fmt.Sprintf("verify Project %s artifact %s: %v", projectID, reference.Digest, err), Err: err}
				}
				referencedArtifacts[reference.StorageURI] = true
			}
		}

		// 2. Mark relational drift as repairable maintenance state.
		consistent, err := s.projectionConsistent(ctx, projectID, version)
		if err != nil {
			return ReconcileResult{}, err
		}
		if !consistent {
			maintenance = append(maintenance, projectID)
		}
	}
	sort.Strings(maintenance)
	orphans, err := artifacts.Orphans(referencedArtifacts)
	if err != nil {
		return ReconcileResult{}, err
	}
	return ReconcileResult{MaintenanceProjects: maintenance, OrphanArtifacts: orphans}, nil
}

func (s *Store) checkDatabaseIntegrity(ctx context.Context) error {
	// 1. Verify SQLite pages and the exact executable schema contract.
	var quickCheck string
	if err := s.db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&quickCheck); err != nil {
		return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("run SQLite quick_check: %v", err), Err: err}
	}
	if quickCheck != "ok" {
		return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("SQLite quick_check failed: %s", quickCheck)}
	}
	items, err := loadMigrations()
	if err != nil {
		return err
	}
	current, err := currentSchemaVersion(ctx, s.db)
	if err != nil {
		return err
	}
	latest := items[len(items)-1].version
	if current != latest {
		return &Error{Code: "store_incompatible", Detail: fmt.Sprintf("database schema version %d does not match supported version %d", current, latest)}
	}

	// 2. Reject durable rows that cannot be owned or repaired by Project replay.
	return s.checkDanglingReferences(ctx)
}

func (s *Store) checkDanglingReferences(ctx context.Context) error {
	checks := []struct {
		name  string
		query string
	}{
		{"event artifact without event", `SELECT COUNT(*) FROM event_artifacts ea LEFT JOIN domain_events de ON de.event_id = ea.event_id WHERE de.event_id IS NULL`},
		{"Project projection without stream", `SELECT COUNT(*) FROM projects p LEFT JOIN project_streams ps ON ps.project_id = p.id WHERE ps.project_id IS NULL`},
		{"binding projection without stream", `SELECT COUNT(*) FROM repository_bindings rb LEFT JOIN project_streams ps ON ps.project_id = rb.project_id WHERE ps.project_id IS NULL`},
		{"Reality projection without stream", `SELECT COUNT(*) FROM reality_revisions rr LEFT JOIN project_streams ps ON ps.project_id = rr.project_id WHERE ps.project_id IS NULL`},
		{"projection state without stream", `SELECT COUNT(*) FROM projection_states p LEFT JOIN project_streams ps ON ps.project_id = p.project_id WHERE ps.project_id IS NULL`},
	}
	for _, check := range checks {
		var count int
		if err := s.db.QueryRowContext(ctx, check.query).Scan(&count); err != nil {
			return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("check %s: %v", check.name, err), Err: err}
		}
		if count != 0 {
			return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("database contains %d %s rows", count, check.name)}
		}
	}
	return nil
}

func (s *Store) projectStreams(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, current_sequence FROM project_streams ORDER BY project_id`)
	if err != nil {
		return nil, fmt.Errorf("read project streams during reconcile: %w", err)
	}
	defer rows.Close()
	streams := make(map[string]int64)
	for rows.Next() {
		var projectID string
		var sequence int64
		if err := rows.Scan(&projectID, &sequence); err != nil {
			return nil, fmt.Errorf("scan project stream during reconcile: %w", err)
		}
		streams[projectID] = sequence
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project streams during reconcile: %w", err)
	}
	return streams, nil
}

func validateStream(projectID string, version int64, events []event.Envelope) error {
	if int64(len(events)) != version {
		return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("Project %s stream version is %d but contains %d events", projectID, version, len(events))}
	}
	for index, item := range events {
		want := int64(index) + 1
		if item.Sequence != want {
			return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("Project %s stream sequence gap: got %d, want %d", projectID, item.Sequence, want)}
		}
		if item.ProjectID != projectID {
			return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("event %s belongs to Project %s, not stream %s", item.EventID, item.ProjectID, projectID)}
		}
		if !event.ValidID(item.EventID) || !event.ValidID(item.ProjectID) || !event.ValidID(item.AggregateID) || !event.ValidID(item.CausationID) || !event.ValidID(item.CorrelationID) {
			return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("event %s contains an invalid durable ID", item.EventID)}
		}
		if item.Actor == "" || item.AggregateType == "" {
			return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("event %s contains an empty actor or aggregate type", item.EventID)}
		}
		if _, err := time.Parse(time.RFC3339Nano, item.OccurredAt); err != nil {
			return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("event %s has invalid occurred_at: %v", item.EventID, err), Err: err}
		}
		if _, err := time.Parse(time.RFC3339Nano, item.RecordedAt); err != nil {
			return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("event %s has invalid recorded_at: %v", item.EventID, err), Err: err}
		}
		if err := validateEventPayload(item); err != nil {
			return err
		}
	}
	return nil
}

func validateEventPayload(item event.Envelope) error {
	if item.SchemaVersion != event.SchemaVersion {
		return &Error{Code: "unknown_event_version", Detail: fmt.Sprintf("unsupported %s schema version %d", item.EventType, item.SchemaVersion)}
	}
	switch item.EventType {
	case event.ProjectRegistered:
		var payload event.ProjectRegisteredPayload
		if err := decodePayload(item, &payload); err != nil {
			return err
		}
		if payload.ProjectID != item.ProjectID || item.AggregateID != item.ProjectID || len(item.Artifacts) != 0 {
			return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("event %s has inconsistent Project registration", item.EventID)}
		}
		return nil
	case event.RepositoryBound:
		var payload event.RepositoryBindingPayload
		if err := decodePayload(item, &payload); err != nil {
			return err
		}
		return validateBindingEvent(item, payload.ProjectID)
	case event.RepositoryRelinked:
		var payload event.RepositoryRelinkedPayload
		if err := decodePayload(item, &payload); err != nil {
			return err
		}
		return validateBindingEvent(item, payload.ProjectID)
	case event.RealityCaptured:
		var payload event.RealityCapturedPayload
		if err := decodePayload(item, &payload); err != nil {
			return err
		}
		if payload.ProjectID != item.ProjectID || payload.RealityID != item.AggregateID || !event.ValidID(payload.RealityID) {
			return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("event %s has inconsistent Reality identity", item.EventID)}
		}
		if len(item.Artifacts) != 1 || item.Artifacts[0].Relation != "reality_manifest" || item.Artifacts[0].Digest != payload.ManifestDigest {
			return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("event %s has inconsistent Reality artifact reference", item.EventID)}
		}
		return nil
	default:
		return &Error{Code: "unknown_event_version", Detail: fmt.Sprintf("unsupported event type %s", item.EventType)}
	}
}

func validateBindingEvent(item event.Envelope, payloadProjectID string) error {
	if payloadProjectID != item.ProjectID || item.AggregateID != item.ProjectID || len(item.Artifacts) != 0 {
		return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("event %s has inconsistent Repository Binding", item.EventID)}
	}
	return nil
}

func (s *Store) projectionConsistent(ctx context.Context, projectID string, streamVersion int64) (bool, error) {
	var projected int64
	var recordedDigest string
	err := s.db.QueryRowContext(ctx, `SELECT projected_through_sequence, result_digest FROM projection_states WHERE project_id = ? AND projection_name = 'project'`, projectID).Scan(&projected, &recordedDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read projection state during reconcile: %w", err)
	}
	if projected != streamVersion {
		return false, nil
	}
	digest, err := projectionDigest(ctx, s.db, projectID)
	if err != nil {
		var storeError *Error
		if errors.As(err, &storeError) && (storeError.Code == "project_not_found" || storeError.Code == "replay_required") {
			return false, nil
		}
		return false, err
	}
	return digest == recordedDigest, nil
}
