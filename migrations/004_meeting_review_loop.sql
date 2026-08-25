ALTER TABLE meetings ADD COLUMN protocol_version INTEGER NOT NULL DEFAULT 1 CHECK (protocol_version IN (1, 2));
ALTER TABLE meetings ADD COLUMN result_content_id TEXT NOT NULL DEFAULT '';
ALTER TABLE meetings ADD COLUMN approved_result_digest TEXT NOT NULL DEFAULT '';

ALTER TABLE meeting_participants ADD COLUMN provider_options_json BLOB NOT NULL DEFAULT '{}';

ALTER TABLE semantic_candidates ADD COLUMN design_disposition TEXT NOT NULL DEFAULT 'unreviewed';
ALTER TABLE semantic_candidates ADD COLUMN context_disposition TEXT NOT NULL DEFAULT 'none';

CREATE INDEX semantic_candidates_context
    ON semantic_candidates(meeting_id, context_disposition, id);
