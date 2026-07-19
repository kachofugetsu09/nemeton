package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kachofugetsu09/nemeton/internal/event"
)

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type projectionModel struct {
	Project   Project   `json:"project"`
	Binding   Binding   `json:"binding"`
	Realities []Reality `json:"realities"`
}

func snapshotQuery(ctx context.Context, source queryer, projectID string) (Snapshot, error) {
	project, err := projectQuery(ctx, source, projectID)
	if err != nil {
		return Snapshot{}, err
	}
	binding, err := bindingQuery(ctx, source, projectID)
	if err != nil {
		return Snapshot{}, err
	}
	reality, err := currentRealityQuery(ctx, source, projectID)
	if err != nil {
		return Snapshot{}, err
	}
	var version int64
	var digest string
	err = source.QueryRowContext(ctx, `
        SELECT ps.projected_through_sequence, ps.result_digest
        FROM projection_states ps
        WHERE ps.project_id = ? AND ps.projection_name = 'project'`, projectID).Scan(&version, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, &Error{Code: "replay_required", Detail: fmt.Sprintf("Project %s projection state is missing", projectID)}
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("read projection state: %w", err)
	}
	return Snapshot{Project: project, Binding: binding, CurrentReality: reality, StreamVersion: version, ResultDigest: digest}, nil
}

func projectQuery(ctx context.Context, source queryer, projectID string) (Project, error) {
	var project Project
	err := source.QueryRowContext(ctx, `SELECT id, status, created_at, updated_at FROM projects WHERE id = ?`, projectID).Scan(&project.ID, &project.Status, &project.CreatedAt, &project.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, &Error{Code: "project_not_found", Detail: fmt.Sprintf("project not found: %s", projectID)}
	}
	if err != nil {
		return Project{}, fmt.Errorf("read Project projection: %w", err)
	}
	return project, nil
}

func bindingQuery(ctx context.Context, source queryer, projectID string) (Binding, error) {
	var binding Binding
	var remotesJSON []byte
	var worktreesJSON []byte
	err := source.QueryRowContext(ctx, `
        SELECT project_id, display_path, canonical_root, git_common_dir,
               remote_identities_json, discovered_worktrees_json,
               integration_branch, managed_worktree_root, updated_at
        FROM repository_bindings WHERE project_id = ?`, projectID).Scan(
		&binding.ProjectID, &binding.DisplayPath, &binding.CanonicalRoot,
		&binding.GitCommonDir, &remotesJSON, &worktreesJSON,
		&binding.IntegrationBranch, &binding.ManagedWorktreeRoot, &binding.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Binding{}, &Error{Code: "replay_required", Detail: fmt.Sprintf("Project %s binding projection is missing", projectID)}
	}
	if err != nil {
		return Binding{}, fmt.Errorf("read Repository Binding projection: %w", err)
	}
	if err := json.Unmarshal(remotesJSON, &binding.RemoteIdentities); err != nil {
		return Binding{}, &Error{Code: "store_corrupt", Detail: fmt.Sprintf("decode Project %s remote identities: %v", projectID, err), Err: err}
	}
	if err := json.Unmarshal(worktreesJSON, &binding.DiscoveredWorktrees); err != nil {
		return Binding{}, &Error{Code: "store_corrupt", Detail: fmt.Sprintf("decode Project %s worktrees: %v", projectID, err), Err: err}
	}
	return binding, nil
}

func currentRealityQuery(ctx context.Context, source queryer, projectID string) (Reality, error) {
	var reality Reality
	var dirty int
	err := source.QueryRowContext(ctx, `
        SELECT id, project_id, sequence, integration_branch, commit_oid, tree_oid,
               manifest_digest, dirty_observed, status, captured_at
        FROM reality_revisions
        WHERE project_id = ? ORDER BY sequence DESC LIMIT 1`, projectID).Scan(
		&reality.ID, &reality.ProjectID, &reality.Sequence,
		&reality.IntegrationBranch, &reality.CommitOID, &reality.TreeOID,
		&reality.ManifestDigest, &dirty, &reality.Status, &reality.CapturedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Reality{}, &Error{Code: "replay_required", Detail: fmt.Sprintf("Project %s Reality projection is missing", projectID)}
	}
	if err != nil {
		return Reality{}, fmt.Errorf("read current Reality projection: %w", err)
	}
	reality.DirtyObserved = dirty == 1
	return reality, nil
}

func realitiesQuery(ctx context.Context, source queryer, projectID string) ([]Reality, error) {
	rows, err := source.QueryContext(ctx, `
        SELECT id, project_id, sequence, integration_branch, commit_oid, tree_oid,
               manifest_digest, dirty_observed, status, captured_at
        FROM reality_revisions WHERE project_id = ? ORDER BY sequence`, projectID)
	if err != nil {
		return nil, fmt.Errorf("read Reality revisions: %w", err)
	}
	defer rows.Close()
	var realities []Reality
	for rows.Next() {
		var item Reality
		var dirty int
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Sequence, &item.IntegrationBranch, &item.CommitOID, &item.TreeOID, &item.ManifestDigest, &dirty, &item.Status, &item.CapturedAt); err != nil {
			return nil, fmt.Errorf("scan Reality revision: %w", err)
		}
		item.DirtyObserved = dirty == 1
		realities = append(realities, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Reality revisions: %w", err)
	}
	return realities, nil
}

func projectionDigest(ctx context.Context, source queryer, projectID string) (string, error) {
	project, err := projectQuery(ctx, source, projectID)
	if err != nil {
		return "", err
	}
	binding, err := bindingQuery(ctx, source, projectID)
	if err != nil {
		return "", err
	}
	realities, err := realitiesQuery(ctx, source, projectID)
	if err != nil {
		return "", err
	}
	if len(realities) == 0 {
		return "", &Error{Code: "replay_required", Detail: fmt.Sprintf("Project %s has no Reality projection", projectID)}
	}
	data, err := json.Marshal(projectionModel{Project: project, Binding: binding, Realities: realities})
	if err != nil {
		return "", fmt.Errorf("encode normalized Project projection: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func writeProjectionState(ctx context.Context, tx *sql.Tx, projectID string, sequence int64, digest, updatedAt string) error {
	_, err := tx.ExecContext(ctx, `
        INSERT INTO projection_states(
            project_id, projection_name, projected_through_sequence,
            schema_version, result_digest, dirty, updated_at
        ) VALUES (?, 'project', ?, ?, ?, 0, ?)
        ON CONFLICT(project_id, projection_name) DO UPDATE SET
            projected_through_sequence = excluded.projected_through_sequence,
            schema_version = excluded.schema_version,
            result_digest = excluded.result_digest,
            dirty = 0,
            updated_at = excluded.updated_at`,
		projectID, sequence, ProjectionSchemaVersion, digest, updatedAt)
	if err != nil {
		return fmt.Errorf("write Project projection state: %w", err)
	}
	return nil
}

func loadEvents(ctx context.Context, source queryer, projectID string) ([]event.Envelope, error) {
	rows, err := source.QueryContext(ctx, `
        SELECT event_id, project_id, sequence, aggregate_type, aggregate_id,
               event_type, schema_version, payload_json, actor, causation_id,
               correlation_id, occurred_at, recorded_at
        FROM domain_events WHERE project_id = ? ORDER BY sequence`, projectID)
	if err != nil {
		return nil, fmt.Errorf("read Project event stream: %w", err)
	}
	var events []event.Envelope
	for rows.Next() {
		var item event.Envelope
		if err := rows.Scan(&item.EventID, &item.ProjectID, &item.Sequence, &item.AggregateType, &item.AggregateID, &item.EventType, &item.SchemaVersion, &item.PayloadJSON, &item.Actor, &item.CausationID, &item.CorrelationID, &item.OccurredAt, &item.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan Project event: %w", err)
		}
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate Project event stream: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close Project event stream: %w", err)
	}
	for index := range events {
		artifacts, err := loadEventArtifacts(ctx, source, events[index].EventID)
		if err != nil {
			return nil, err
		}
		events[index].Artifacts = artifacts
	}
	return events, nil
}

func loadEventArtifacts(ctx context.Context, source queryer, eventID string) ([]event.ArtifactRef, error) {
	rows, err := source.QueryContext(ctx, `
        SELECT artifact_digest, relation, media_type, byte_size, storage_uri
        FROM event_artifacts WHERE event_id = ? ORDER BY relation, artifact_digest`, eventID)
	if err != nil {
		return nil, fmt.Errorf("read event artifact references: %w", err)
	}
	defer rows.Close()
	var artifacts []event.ArtifactRef
	for rows.Next() {
		var item event.ArtifactRef
		if err := rows.Scan(&item.Digest, &item.Relation, &item.MediaType, &item.ByteSize, &item.StorageURI); err != nil {
			return nil, fmt.Errorf("scan event artifact reference: %w", err)
		}
		artifacts = append(artifacts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate event artifact references: %w", err)
	}
	return artifacts, nil
}
