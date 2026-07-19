CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    applied_at TEXT NOT NULL
) STRICT;

CREATE TABLE project_streams (
    project_id TEXT PRIMARY KEY,
    current_sequence INTEGER NOT NULL CHECK (current_sequence >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

CREATE TABLE domain_events (
    row_id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    project_id TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    schema_version INTEGER NOT NULL CHECK (schema_version > 0),
    payload_json BLOB NOT NULL,
    actor TEXT NOT NULL,
    causation_id TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    recorded_at TEXT NOT NULL,
    UNIQUE (project_id, sequence)
) STRICT;

CREATE INDEX domain_events_project_sequence
    ON domain_events (project_id, sequence);

CREATE TABLE event_artifacts (
    event_id TEXT NOT NULL,
    artifact_digest TEXT NOT NULL,
    relation TEXT NOT NULL,
    media_type TEXT NOT NULL,
    byte_size INTEGER NOT NULL CHECK (byte_size >= 0),
    storage_uri TEXT NOT NULL,
    UNIQUE (event_id, artifact_digest, relation)
) STRICT;

CREATE INDEX event_artifacts_digest ON event_artifacts (artifact_digest);

CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

CREATE TABLE repository_bindings (
    project_id TEXT PRIMARY KEY,
    display_path TEXT NOT NULL,
    canonical_root TEXT NOT NULL,
    git_common_dir TEXT NOT NULL UNIQUE,
    remote_identities_json BLOB NOT NULL,
    discovered_worktrees_json BLOB NOT NULL,
    integration_branch TEXT NOT NULL,
    managed_worktree_root TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

CREATE TABLE reality_revisions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    integration_branch TEXT NOT NULL,
    commit_oid TEXT NOT NULL,
    tree_oid TEXT NOT NULL,
    manifest_digest TEXT NOT NULL,
    dirty_observed INTEGER NOT NULL CHECK (dirty_observed IN (0, 1)),
    status TEXT NOT NULL,
    captured_at TEXT NOT NULL,
    UNIQUE (project_id, sequence)
) STRICT;

CREATE INDEX reality_revisions_project_sequence
    ON reality_revisions (project_id, sequence DESC);

CREATE TABLE projection_states (
    project_id TEXT NOT NULL,
    projection_name TEXT NOT NULL,
    projected_through_sequence INTEGER NOT NULL CHECK (projected_through_sequence >= 0),
    schema_version INTEGER NOT NULL CHECK (schema_version > 0),
    result_digest TEXT NOT NULL,
    dirty INTEGER NOT NULL CHECK (dirty IN (0, 1)),
    updated_at TEXT NOT NULL,
    PRIMARY KEY (project_id, projection_name)
) STRICT;

CREATE TRIGGER domain_events_no_update
BEFORE UPDATE ON domain_events
BEGIN
    SELECT RAISE(ABORT, 'domain_events are append-only');
END;

CREATE TRIGGER domain_events_no_delete
BEFORE DELETE ON domain_events
BEGIN
    SELECT RAISE(ABORT, 'domain_events are append-only');
END;

CREATE TRIGGER event_artifacts_no_update
BEFORE UPDATE ON event_artifacts
BEGIN
    SELECT RAISE(ABORT, 'event_artifacts are append-only');
END;

CREATE TRIGGER event_artifacts_no_delete
BEFORE DELETE ON event_artifacts
BEGIN
    SELECT RAISE(ABORT, 'event_artifacts are append-only');
END;
