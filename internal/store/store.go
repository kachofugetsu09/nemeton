package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kachofugetsu09/nemeton/internal/event"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, databasePath, backupRoot string) (*Store, error) {
	// 1. Open a single logical writer connection without changing schema.
	_, statErr := os.Stat(databasePath)
	existed := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("inspect database file: %w", statErr)
	}
	database, err := sql.Open("sqlite", "file:"+databasePath+"?_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("connect SQLite database: %w", err)
	}
	if err := os.Chmod(databasePath, 0o600); err != nil {
		database.Close()
		return nil, fmt.Errorf("set database permissions: %w", err)
	}

	// 2. Back up an existing database before applying required migrations.
	items, err := loadMigrations()
	if err != nil {
		database.Close()
		return nil, err
	}
	current, err := currentSchemaVersion(ctx, database)
	if err != nil {
		database.Close()
		return nil, err
	}
	latest := items[len(items)-1].version
	if current > latest {
		database.Close()
		return nil, &Error{Code: "store_incompatible", Detail: fmt.Sprintf("database schema version %d is newer than supported version %d", current, latest)}
	}
	if existed && current < latest {
		if _, err := backupDatabase(ctx, database, backupRoot); err != nil {
			database.Close()
			return nil, err
		}
	}

	// 3. Apply schema and durability settings before serving commands.
	if err := applyMigrations(ctx, database, items, current); err != nil {
		database.Close()
		return nil, err
	}
	for _, statement := range []string{`PRAGMA journal_mode=WAL`, `PRAGMA synchronous=FULL`, `PRAGMA busy_timeout=5000`, `PRAGMA foreign_keys=OFF`} {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			database.Close()
			return nil, fmt.Errorf("configure SQLite with %q: %w", statement, err)
		}
	}
	return &Store{db: database}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) FindByCommonDir(ctx context.Context, commonDir string) (Snapshot, bool, error) {
	var projectID string
	err := s.db.QueryRowContext(ctx, `SELECT project_id FROM repository_bindings WHERE git_common_dir = ?`, commonDir).Scan(&projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("find repository binding: %w", err)
	}
	snapshot, err := s.Inspect(ctx, projectID)
	return snapshot, err == nil, err
}

func (s *Store) Inspect(ctx context.Context, projectID string) (Snapshot, error) {
	return snapshotQuery(ctx, s.db, projectID)
}

func (s *Store) Append(ctx context.Context, request AppendRequest) (Snapshot, error) {
	if len(request.Events) == 0 {
		return Snapshot{}, fmt.Errorf("append request must contain at least one event")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin event transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Lock the Project stream at the expected semantic revision.
	current, exists, err := streamVersion(ctx, tx, request.ProjectID)
	if err != nil {
		return Snapshot{}, err
	}
	if current != request.ExpectedVersion {
		return Snapshot{}, &Error{Code: "stream_conflict", Detail: fmt.Sprintf("project stream version conflict: got %d, expected %d", current, request.ExpectedVersion)}
	}
	if !exists {
		stamp := request.Events[0].RecordedAt
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_streams(project_id, current_sequence, created_at, updated_at) VALUES (?, 0, ?, ?)`, request.ProjectID, stamp, stamp); err != nil {
			return Snapshot{}, fmt.Errorf("create project stream: %w", err)
		}
	}

	// 2. Append each immutable fact and apply its reducer.
	for index := range request.Events {
		request.Events[index].Sequence = current + int64(index) + 1
		if err := appendEvent(ctx, tx, request.Events[index]); err != nil {
			return Snapshot{}, err
		}
		if err := applyEvent(ctx, tx, request.Events[index]); err != nil {
			return Snapshot{}, err
		}
	}
	last := request.Events[len(request.Events)-1]
	newVersion := last.Sequence
	if _, err := tx.ExecContext(ctx, `UPDATE project_streams SET current_sequence = ?, updated_at = ? WHERE project_id = ?`, newVersion, last.RecordedAt, request.ProjectID); err != nil {
		return Snapshot{}, fmt.Errorf("advance project stream: %w", err)
	}

	// 3. Seal the projection digest before committing the atomic change.
	digest, err := projectionDigest(ctx, tx, request.ProjectID)
	if err != nil {
		return Snapshot{}, err
	}
	if err := writeProjectionState(ctx, tx, request.ProjectID, newVersion, digest, last.RecordedAt); err != nil {
		return Snapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit event transaction: %w", err)
	}
	return s.Inspect(ctx, request.ProjectID)
}

func (s *Store) Replay(ctx context.Context, projectID string) (Snapshot, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin replay transaction: %w", err)
	}
	defer tx.Rollback()
	version, exists, err := streamVersion(ctx, tx, projectID)
	if err != nil {
		return Snapshot{}, err
	}
	if !exists {
		return Snapshot{}, &Error{Code: "project_not_found", Detail: fmt.Sprintf("project not found: %s", projectID)}
	}
	events, err := loadEvents(ctx, tx, projectID)
	if err != nil {
		return Snapshot{}, err
	}
	if int64(len(events)) != version {
		return Snapshot{}, &Error{Code: "store_corrupt", Detail: fmt.Sprintf("project %s stream tail is %d but has %d events", projectID, version, len(events))}
	}

	// 1. Remove only the Project's derived relational state.
	for _, statement := range []string{
		`DELETE FROM projection_states WHERE project_id = ?`,
		`DELETE FROM reality_revisions WHERE project_id = ?`,
		`DELETE FROM repository_bindings WHERE project_id = ?`,
		`DELETE FROM projects WHERE id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, statement, projectID); err != nil {
			return Snapshot{}, fmt.Errorf("clear Project projection during replay: %w", err)
		}
	}

	// 2. Apply the same versioned reducer in Project sequence order.
	for index, item := range events {
		if item.Sequence != int64(index)+1 {
			return Snapshot{}, &Error{Code: "store_corrupt", Detail: fmt.Sprintf("project %s event sequence gap at %d", projectID, index+1)}
		}
		if err := applyEvent(ctx, tx, item); err != nil {
			return Snapshot{}, err
		}
	}
	digest, err := projectionDigest(ctx, tx, projectID)
	if err != nil {
		return Snapshot{}, err
	}
	stamp := events[len(events)-1].RecordedAt
	if err := writeProjectionState(ctx, tx, projectID, version, digest, stamp); err != nil {
		return Snapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit Project replay: %w", err)
	}
	return s.Inspect(ctx, projectID)
}

func streamVersion(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, projectID string) (int64, bool, error) {
	var version int64
	err := queryer.QueryRowContext(ctx, `SELECT current_sequence FROM project_streams WHERE project_id = ?`, projectID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read project stream version: %w", err)
	}
	return version, true, nil
}

func appendEvent(ctx context.Context, tx *sql.Tx, item event.Envelope) error {
	_, err := tx.ExecContext(ctx, `
        INSERT INTO domain_events(
            event_id, project_id, sequence, aggregate_type, aggregate_id,
            event_type, schema_version, payload_json, actor, causation_id,
            correlation_id, occurred_at, recorded_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.EventID, item.ProjectID, item.Sequence, item.AggregateType, item.AggregateID,
		item.EventType, item.SchemaVersion, item.PayloadJSON, item.Actor, item.CausationID,
		item.CorrelationID, item.OccurredAt, item.RecordedAt)
	if err != nil {
		return fmt.Errorf("append %s at sequence %d: %w", item.EventType, item.Sequence, err)
	}
	for _, artifact := range item.Artifacts {
		if _, err := tx.ExecContext(ctx, `INSERT INTO event_artifacts(event_id, artifact_digest, relation, media_type, byte_size, storage_uri) VALUES (?, ?, ?, ?, ?, ?)`, item.EventID, artifact.Digest, artifact.Relation, artifact.MediaType, artifact.ByteSize, artifact.StorageURI); err != nil {
			return fmt.Errorf("link artifact %s to event %s: %w", artifact.Digest, item.EventID, err)
		}
	}
	return nil
}

func applyEvent(ctx context.Context, tx *sql.Tx, item event.Envelope) error {
	if item.SchemaVersion != event.SchemaVersion {
		return &Error{Code: "unknown_event_version", Detail: fmt.Sprintf("unsupported %s schema version %d", item.EventType, item.SchemaVersion)}
	}
	switch item.EventType {
	case event.ProjectRegistered:
		var payload event.ProjectRegisteredPayload
		if err := decodePayload(item, &payload); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO projects(id, status, created_at, updated_at) VALUES (?, ?, ?, ?)`, payload.ProjectID, payload.Status, payload.CreatedAt, payload.CreatedAt)
		return wrapReducerError(item, err)
	case event.RepositoryBound:
		var payload event.RepositoryBindingPayload
		if err := decodePayload(item, &payload); err != nil {
			return err
		}
		return insertBinding(ctx, tx, item, payload)
	case event.RepositoryRelinked:
		var payload event.RepositoryRelinkedPayload
		if err := decodePayload(item, &payload); err != nil {
			return err
		}
		return updateBinding(ctx, tx, item, payload.RepositoryBindingPayload)
	case event.RealityCaptured:
		var payload event.RealityCapturedPayload
		if err := decodePayload(item, &payload); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE reality_revisions SET status = 'stale' WHERE project_id = ? AND status = 'current'`, payload.ProjectID); err != nil {
			return wrapReducerError(item, err)
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO reality_revisions(id, project_id, sequence, integration_branch, commit_oid, tree_oid, manifest_digest, dirty_observed, status, captured_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, payload.RealityID, payload.ProjectID, item.Sequence, payload.IntegrationBranch, payload.CommitOID, payload.TreeOID, payload.ManifestDigest, payload.DirtyObserved, payload.Status, payload.CapturedAt)
		return wrapReducerError(item, err)
	default:
		return &Error{Code: "unknown_event_version", Detail: fmt.Sprintf("unsupported event type %s", item.EventType)}
	}
}

func insertBinding(ctx context.Context, tx *sql.Tx, item event.Envelope, payload event.RepositoryBindingPayload) error {
	remotes, worktrees, err := bindingJSON(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO repository_bindings(project_id, display_path, canonical_root, git_common_dir, remote_identities_json, discovered_worktrees_json, integration_branch, managed_worktree_root, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, payload.ProjectID, payload.DisplayPath, payload.CanonicalRoot, payload.GitCommonDir, remotes, worktrees, payload.IntegrationBranch, payload.ManagedWorktreeRoot, payload.BoundAt)
	return wrapReducerError(item, err)
}

func updateBinding(ctx context.Context, tx *sql.Tx, item event.Envelope, payload event.RepositoryBindingPayload) error {
	remotes, worktrees, err := bindingJSON(payload)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE repository_bindings SET display_path = ?, canonical_root = ?, git_common_dir = ?, remote_identities_json = ?, discovered_worktrees_json = ?, integration_branch = ?, managed_worktree_root = ?, updated_at = ? WHERE project_id = ?`, payload.DisplayPath, payload.CanonicalRoot, payload.GitCommonDir, remotes, worktrees, payload.IntegrationBranch, payload.ManagedWorktreeRoot, payload.BoundAt, payload.ProjectID)
	if err != nil {
		return wrapReducerError(item, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read relink reducer result: %w", err)
	}
	if rows != 1 {
		return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("relink reducer expected one binding for project %s, updated %d", payload.ProjectID, rows)}
	}
	return nil
}

func bindingJSON(payload event.RepositoryBindingPayload) ([]byte, []byte, error) {
	remotes, err := json.Marshal(payload.RemoteIdentities)
	if err != nil {
		return nil, nil, fmt.Errorf("encode remote identities projection: %w", err)
	}
	worktrees, err := json.Marshal(payload.DiscoveredWorktrees)
	if err != nil {
		return nil, nil, fmt.Errorf("encode discovered worktrees projection: %w", err)
	}
	return remotes, worktrees, nil
}

func decodePayload(item event.Envelope, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(item.PayloadJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("decode %s payload: %v", item.EventType, err), Err: err}
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("decode %s payload: %v", item.EventType, err), Err: err}
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("unexpected trailing JSON value")
}

func wrapReducerError(item event.Envelope, err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "UNIQUE constraint failed: repository_bindings.git_common_dir") {
		return &Error{Code: "binding_conflict", Detail: "Git common-dir is already bound to another Project", Err: err}
	}
	return fmt.Errorf("apply %s reducer at sequence %d: %w", item.EventType, item.Sequence, err)
}
