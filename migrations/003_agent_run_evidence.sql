ALTER TABLE agent_runs
ADD COLUMN provider_version TEXT NOT NULL DEFAULT '';

ALTER TABLE agent_runs
ADD COLUMN command_json BLOB NOT NULL DEFAULT X'5B5D';

ALTER TABLE agent_runs
ADD COLUMN raw_stream_digest TEXT NOT NULL DEFAULT '';

ALTER TABLE agent_runs
ADD COLUMN stderr_digest TEXT NOT NULL DEFAULT '';
