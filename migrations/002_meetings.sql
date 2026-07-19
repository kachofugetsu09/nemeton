CREATE TABLE meetings (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    reality_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    brief TEXT NOT NULL,
    status TEXT NOT NULL,
    max_rounds INTEGER NOT NULL CHECK (max_rounds > 0),
    cycle INTEGER NOT NULL CHECK (cycle > 0),
    current_round INTEGER NOT NULL CHECK (current_round >= 0),
    reality_bundle_digest TEXT NOT NULL,
    human_question TEXT NOT NULL,
    result_digest TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

CREATE INDEX meetings_project_updated ON meetings(project_id, updated_at DESC);

CREATE TABLE meeting_participants (
    id TEXT PRIMARY KEY,
    meeting_id TEXT NOT NULL,
    seat TEXT NOT NULL,
    role TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    status TEXT NOT NULL,
    session_id TEXT NOT NULL,
    workdir TEXT NOT NULL,
    proposal_digest TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(meeting_id, seat)
) STRICT;

CREATE INDEX meeting_participants_meeting ON meeting_participants(meeting_id, seat);

CREATE TABLE agent_runs (
    id TEXT PRIMARY KEY,
    meeting_id TEXT NOT NULL,
    participant_id TEXT NOT NULL,
    phase TEXT NOT NULL,
    provider TEXT NOT NULL,
    status TEXT NOT NULL,
    session_id TEXT NOT NULL,
    input_digest TEXT NOT NULL,
    output_digest TEXT NOT NULL,
    workdir TEXT NOT NULL,
    exit_code INTEGER NOT NULL,
    error TEXT NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT NOT NULL
) STRICT;

CREATE INDEX agent_runs_meeting_participant ON agent_runs(meeting_id, participant_id, started_at);

CREATE TABLE meeting_contents (
    id TEXT PRIMARY KEY,
    meeting_id TEXT NOT NULL,
    participant_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    cycle INTEGER NOT NULL CHECK (cycle > 0),
    round INTEGER NOT NULL CHECK (round >= 0),
    content_digest TEXT NOT NULL,
    refs_json BLOB NOT NULL,
    created_at TEXT NOT NULL
) STRICT;

CREATE INDEX meeting_contents_meeting_kind ON meeting_contents(meeting_id, kind, cycle, round);

CREATE TABLE meeting_conflicts (
    id TEXT PRIMARY KEY,
    meeting_id TEXT NOT NULL,
    question TEXT NOT NULL,
    status TEXT NOT NULL,
    cycle INTEGER NOT NULL CHECK (cycle > 0),
    round INTEGER NOT NULL CHECK (round >= 0),
    positions_json BLOB NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

CREATE INDEX meeting_conflicts_meeting ON meeting_conflicts(meeting_id, status, id);

CREATE TABLE semantic_candidates (
    id TEXT PRIMARY KEY,
    meeting_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    statement TEXT NOT NULL,
    source_refs_json BLOB NOT NULL,
    status TEXT NOT NULL,
    rationale TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

CREATE INDEX semantic_candidates_meeting ON semantic_candidates(meeting_id, status, id);

CREATE TABLE meeting_event_index (
    meeting_id TEXT NOT NULL,
    project_sequence INTEGER NOT NULL CHECK (project_sequence > 0),
    event_id TEXT NOT NULL UNIQUE,
    PRIMARY KEY(meeting_id, project_sequence)
) STRICT;

CREATE TABLE current_states (
    project_id TEXT PRIMARY KEY,
    projected_through_sequence INTEGER NOT NULL CHECK (projected_through_sequence >= 0),
    document_json BLOB NOT NULL,
    result_digest TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;
