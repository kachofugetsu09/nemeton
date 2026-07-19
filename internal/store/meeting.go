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

func applyMeetingEvent(ctx context.Context, tx *sql.Tx, item event.Envelope) (bool, error) {
	var meetingID string
	switch item.EventType {
	case event.MeetingCreated:
		var payload event.MeetingCreatedPayload
		if err := decodePayload(item, &payload); err != nil {
			return true, err
		}
		meetingID = payload.MeetingID
		if err := requireReference(ctx, tx, `SELECT COUNT(*) FROM reality_revisions WHERE id = ? AND project_id = ?`,
			"Reality Revision", payload.RealityID, payload.ProjectID); err != nil {
			return true, err
		}
		_, err := tx.ExecContext(ctx, `
            INSERT INTO meetings(
                id, project_id, reality_id, kind, title, brief, status,
                max_rounds, cycle, current_round, reality_bundle_digest,
                human_question, result_digest, created_at, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', ?, ?)`,
			payload.MeetingID, payload.ProjectID, payload.RealityID, payload.Kind,
			payload.Title, payload.Brief, payload.Status, payload.MaxRounds,
			payload.Cycle, payload.CurrentRound, payload.RealityBundleDigest,
			payload.CreatedAt, payload.CreatedAt)
		if err != nil {
			return true, wrapReducerError(item, err)
		}
	case event.ParticipantAdded:
		var payload event.MeetingParticipantAddedPayload
		if err := decodePayload(item, &payload); err != nil {
			return true, err
		}
		meetingID = payload.MeetingID
		if err := requireReference(ctx, tx, `SELECT COUNT(*) FROM meetings WHERE id = ?`, "Meeting", payload.MeetingID); err != nil {
			return true, err
		}
		_, err := tx.ExecContext(ctx, `
            INSERT INTO meeting_participants(
                id, meeting_id, seat, role, provider, model, status,
                session_id, workdir, proposal_digest, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, '', ?)`,
			payload.ParticipantID, payload.MeetingID, payload.Seat, payload.Role,
			payload.Provider, payload.Model, payload.Status, payload.Workdir, payload.AddedAt)
		if err != nil {
			return true, wrapReducerError(item, err)
		}
	case event.MeetingStatusSet:
		var payload event.MeetingStatusSetPayload
		if err := decodePayload(item, &payload); err != nil {
			return true, err
		}
		meetingID = payload.MeetingID
		result, err := tx.ExecContext(ctx, `
            UPDATE meetings
            SET status = ?, cycle = ?, current_round = ?, human_question = ?,
                result_digest = ?, updated_at = ?
            WHERE id = ?`, payload.Status, payload.Cycle, payload.CurrentRound,
			payload.HumanQuestion, payload.ResultDigest, payload.UpdatedAt, payload.MeetingID)
		if err != nil {
			return true, wrapReducerError(item, err)
		}
		if err := requireOneRow(result, "Meeting", payload.MeetingID); err != nil {
			return true, err
		}
	case event.AgentRunRecorded:
		var payload event.AgentRunRecordedPayload
		if err := decodePayload(item, &payload); err != nil {
			return true, err
		}
		meetingID = payload.MeetingID
		if err := requireReference(ctx, tx, `SELECT COUNT(*) FROM meeting_participants WHERE id = ? AND meeting_id = ?`,
			"Meeting participant", payload.ParticipantID, payload.MeetingID); err != nil {
			return true, err
		}
		command, err := json.Marshal(payload.Command)
		if err != nil {
			return true, fmt.Errorf("encode Agent Run command: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
            INSERT INTO agent_runs(
                id, meeting_id, participant_id, phase, provider,
                provider_version, command_json, status, session_id,
                input_digest, output_digest, raw_stream_digest, stderr_digest,
                workdir, exit_code, error, started_at, finished_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(id) DO UPDATE SET
                provider_version = excluded.provider_version,
                command_json = excluded.command_json,
                status = excluded.status,
                session_id = excluded.session_id,
                output_digest = excluded.output_digest,
                raw_stream_digest = excluded.raw_stream_digest,
                stderr_digest = excluded.stderr_digest,
                exit_code = excluded.exit_code,
                error = excluded.error,
                finished_at = excluded.finished_at`,
			payload.RunID, payload.MeetingID, payload.ParticipantID, payload.Phase,
			payload.Provider, payload.ProviderVersion, command, payload.Status,
			payload.SessionID, payload.InputDigest, payload.OutputDigest,
			payload.RawStreamDigest, payload.StderrDigest, payload.Workdir,
			payload.ExitCode, payload.Error, payload.StartedAt, payload.FinishedAt)
		if err != nil {
			return true, wrapReducerError(item, err)
		}
		proposalDigest := ""
		if payload.Phase == "proposal" && payload.Status == "completed" {
			proposalDigest = payload.OutputDigest
		}
		_, err = tx.ExecContext(ctx, `
            UPDATE meeting_participants
            SET status = ?, session_id = CASE WHEN ? = '' THEN session_id ELSE ? END,
                workdir = ?,
                proposal_digest = CASE WHEN ? = '' THEN proposal_digest ELSE ? END,
                updated_at = CASE WHEN ? = '' THEN updated_at ELSE ? END
            WHERE id = ?`, payload.Status, payload.SessionID, payload.SessionID,
			payload.Workdir, proposalDigest, proposalDigest, payload.FinishedAt,
			payload.FinishedAt, payload.ParticipantID)
		if err != nil {
			return true, wrapReducerError(item, err)
		}
	case event.MeetingContentAdded:
		var payload event.MeetingContentAddedPayload
		if err := decodePayload(item, &payload); err != nil {
			return true, err
		}
		meetingID = payload.MeetingID
		if err := requireReference(ctx, tx, `SELECT COUNT(*) FROM meetings WHERE id = ?`, "Meeting", payload.MeetingID); err != nil {
			return true, err
		}
		if payload.ParticipantID != "" {
			if err := requireReference(ctx, tx, `SELECT COUNT(*) FROM meeting_participants WHERE id = ? AND meeting_id = ?`,
				"Meeting participant", payload.ParticipantID, payload.MeetingID); err != nil {
				return true, err
			}
		}
		refs, err := json.Marshal(payload.Refs)
		if err != nil {
			return true, fmt.Errorf("encode Meeting content refs: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
            INSERT INTO meeting_contents(
                id, meeting_id, participant_id, kind, cycle, round,
                content_digest, refs_json, created_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, payload.ContentID,
			payload.MeetingID, payload.ParticipantID, payload.Kind, payload.Cycle,
			payload.Round, payload.ContentDigest, refs, payload.CreatedAt)
		if err != nil {
			return true, wrapReducerError(item, err)
		}
	case event.MeetingConflictSet:
		var payload event.MeetingConflictSetPayload
		if err := decodePayload(item, &payload); err != nil {
			return true, err
		}
		meetingID = payload.MeetingID
		if err := requireReference(ctx, tx, `SELECT COUNT(*) FROM meetings WHERE id = ?`, "Meeting", payload.MeetingID); err != nil {
			return true, err
		}
		positions, err := json.Marshal(payload.Positions)
		if err != nil {
			return true, fmt.Errorf("encode Meeting conflict positions: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
            INSERT INTO meeting_conflicts(
                id, meeting_id, question, status, cycle, round,
                positions_json, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(id) DO UPDATE SET
                question = excluded.question,
                status = excluded.status,
                cycle = excluded.cycle,
                round = excluded.round,
                positions_json = excluded.positions_json,
                updated_at = excluded.updated_at`, payload.ConflictID,
			payload.MeetingID, payload.Question, payload.Status, payload.Cycle,
			payload.Round, positions, payload.UpdatedAt)
		if err != nil {
			return true, wrapReducerError(item, err)
		}
	case event.SemanticCandidateAdded:
		var payload event.SemanticCandidateAddedPayload
		if err := decodePayload(item, &payload); err != nil {
			return true, err
		}
		meetingID = payload.MeetingID
		if err := requireReference(ctx, tx, `SELECT COUNT(*) FROM meetings WHERE id = ?`, "Meeting", payload.MeetingID); err != nil {
			return true, err
		}
		for _, reference := range payload.SourceRefs {
			if err := requireReference(ctx, tx, `SELECT COUNT(*) FROM meeting_contents WHERE id = ? AND meeting_id = ?`,
				"Meeting content", reference, payload.MeetingID); err != nil {
				return true, err
			}
		}
		refs, err := json.Marshal(payload.SourceRefs)
		if err != nil {
			return true, fmt.Errorf("encode Candidate source refs: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
            INSERT INTO semantic_candidates(
                id, meeting_id, kind, statement, source_refs_json, status,
                rationale, created_at, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, payload.CandidateID,
			payload.MeetingID, payload.Kind, payload.Statement, refs, payload.Status,
			payload.Rationale, payload.CreatedAt, payload.CreatedAt)
		if err != nil {
			return true, wrapReducerError(item, err)
		}
	case event.SemanticCandidateDispositioned:
		var payload event.SemanticCandidateDispositionedPayload
		if err := decodePayload(item, &payload); err != nil {
			return true, err
		}
		meetingID = payload.MeetingID
		result, err := tx.ExecContext(ctx, `
            UPDATE semantic_candidates
            SET status = ?, updated_at = ?
            WHERE id = ? AND meeting_id = ?`, payload.Status, payload.UpdatedAt,
			payload.CandidateID, payload.MeetingID)
		if err != nil {
			return true, wrapReducerError(item, err)
		}
		if err := requireOneRow(result, "Candidate", payload.CandidateID); err != nil {
			return true, err
		}
	default:
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO meeting_event_index(meeting_id, project_sequence, event_id)
        VALUES (?, ?, ?)`, meetingID, item.Sequence, item.EventID); err != nil {
		return true, wrapReducerError(item, err)
	}
	return true, nil
}

func requireReference(ctx context.Context, tx *sql.Tx, query, kind string, args ...any) error {
	var count int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return fmt.Errorf("check %s reference: %w", kind, err)
	}
	if count != 1 {
		return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("%s reference does not resolve uniquely", kind)}
	}
	return nil
}

func requireOneRow(result sql.Result, kind, id string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read %s reducer result: %w", kind, err)
	}
	if rows != 1 {
		return &Error{Code: "store_corrupt", Detail: fmt.Sprintf("%s reducer expected one row for %s, updated %d", kind, id, rows)}
	}
	return nil
}

func (s *Store) InspectMeeting(ctx context.Context, meetingID string) (MeetingSnapshot, error) {
	meeting, err := meetingQuery(ctx, s.db, meetingID)
	if err != nil {
		return MeetingSnapshot{}, err
	}
	participants, err := participantQuery(ctx, s.db, meetingID)
	if err != nil {
		return MeetingSnapshot{}, err
	}
	runs, err := agentRunQuery(ctx, s.db, meetingID)
	if err != nil {
		return MeetingSnapshot{}, err
	}
	contents, err := contentQuery(ctx, s.db, meetingID)
	if err != nil {
		return MeetingSnapshot{}, err
	}
	conflicts, err := conflictQuery(ctx, s.db, meetingID)
	if err != nil {
		return MeetingSnapshot{}, err
	}
	candidates, err := candidateQuery(ctx, s.db, meetingID)
	if err != nil {
		return MeetingSnapshot{}, err
	}
	var version int64
	if err := s.db.QueryRowContext(ctx, `SELECT current_sequence FROM project_streams WHERE project_id = ?`, meeting.ProjectID).Scan(&version); err != nil {
		return MeetingSnapshot{}, fmt.Errorf("read Meeting stream version: %w", err)
	}
	var stateDigest string
	if err := s.db.QueryRowContext(ctx, `SELECT result_digest FROM current_states WHERE project_id = ?`, meeting.ProjectID).Scan(&stateDigest); err != nil {
		return MeetingSnapshot{}, fmt.Errorf("read Current State digest: %w", err)
	}
	return MeetingSnapshot{Meeting: meeting, Participants: participants, Runs: runs,
		Contents: contents, Conflicts: conflicts, Candidates: candidates,
		StreamVersion: version, CurrentStateDigest: stateDigest}, nil
}

func meetingQuery(ctx context.Context, source queryer, meetingID string) (Meeting, error) {
	var item Meeting
	err := source.QueryRowContext(ctx, `
        SELECT id, project_id, reality_id, kind, title, brief, status,
               max_rounds, cycle, current_round, reality_bundle_digest,
               human_question, result_digest, created_at, updated_at
        FROM meetings WHERE id = ?`, meetingID).Scan(&item.ID, &item.ProjectID,
		&item.RealityID, &item.Kind, &item.Title, &item.Brief, &item.Status,
		&item.MaxRounds, &item.Cycle, &item.CurrentRound,
		&item.RealityBundleDigest, &item.HumanQuestion, &item.ResultDigest,
		&item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Meeting{}, &Error{Code: "meeting_not_found", Detail: fmt.Sprintf("Meeting not found: %s", meetingID)}
	}
	if err != nil {
		return Meeting{}, fmt.Errorf("read Meeting projection: %w", err)
	}
	return item, nil
}

func participantQuery(ctx context.Context, source queryer, meetingID string) ([]MeetingParticipant, error) {
	rows, err := source.QueryContext(ctx, `
        SELECT id, meeting_id, seat, role, provider, model, status, session_id,
               workdir, proposal_digest, updated_at
        FROM meeting_participants WHERE meeting_id = ? ORDER BY seat`, meetingID)
	if err != nil {
		return nil, fmt.Errorf("read Meeting participants: %w", err)
	}
	defer rows.Close()
	var result []MeetingParticipant
	for rows.Next() {
		var item MeetingParticipant
		if err := rows.Scan(&item.ID, &item.MeetingID, &item.Seat, &item.Role,
			&item.Provider, &item.Model, &item.Status, &item.SessionID,
			&item.Workdir, &item.ProposalDigest, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan Meeting participant: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func agentRunQuery(ctx context.Context, source queryer, meetingID string) ([]AgentRun, error) {
	rows, err := source.QueryContext(ctx, `
        SELECT id, meeting_id, participant_id, phase, provider,
               provider_version, command_json, status, session_id,
               input_digest, output_digest, raw_stream_digest, stderr_digest,
               workdir, exit_code, error, started_at, finished_at
        FROM agent_runs WHERE meeting_id = ? ORDER BY started_at, id`, meetingID)
	if err != nil {
		return nil, fmt.Errorf("read Agent Runs: %w", err)
	}
	defer rows.Close()
	var result []AgentRun
	for rows.Next() {
		var item AgentRun
		var command []byte
		if err := rows.Scan(&item.ID, &item.MeetingID, &item.ParticipantID,
			&item.Phase, &item.Provider, &item.ProviderVersion, &command,
			&item.Status, &item.SessionID, &item.InputDigest, &item.OutputDigest,
			&item.RawStreamDigest, &item.StderrDigest, &item.Workdir,
			&item.ExitCode, &item.Error, &item.StartedAt, &item.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan Agent Run: %w", err)
		}
		if err := json.Unmarshal(command, &item.Command); err != nil {
			return nil, &Error{Code: "store_corrupt", Detail: fmt.Sprintf("decode Agent Run command: %v", err), Err: err}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func contentQuery(ctx context.Context, source queryer, meetingID string) ([]MeetingContent, error) {
	rows, err := source.QueryContext(ctx, `
        SELECT id, meeting_id, participant_id, kind, cycle, round,
               content_digest, refs_json, created_at
        FROM meeting_contents WHERE meeting_id = ? ORDER BY created_at, id`, meetingID)
	if err != nil {
		return nil, fmt.Errorf("read Meeting contents: %w", err)
	}
	defer rows.Close()
	var result []MeetingContent
	for rows.Next() {
		var item MeetingContent
		var refs []byte
		if err := rows.Scan(&item.ID, &item.MeetingID, &item.ParticipantID,
			&item.Kind, &item.Cycle, &item.Round, &item.ContentDigest, &refs,
			&item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan Meeting content: %w", err)
		}
		if err := json.Unmarshal(refs, &item.Refs); err != nil {
			return nil, &Error{Code: "store_corrupt", Detail: fmt.Sprintf("decode Meeting content refs: %v", err), Err: err}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func conflictQuery(ctx context.Context, source queryer, meetingID string) ([]MeetingConflict, error) {
	rows, err := source.QueryContext(ctx, `
        SELECT id, meeting_id, question, status, cycle, round,
               positions_json, updated_at
        FROM meeting_conflicts WHERE meeting_id = ? ORDER BY id`, meetingID)
	if err != nil {
		return nil, fmt.Errorf("read Meeting conflicts: %w", err)
	}
	defer rows.Close()
	var result []MeetingConflict
	for rows.Next() {
		var item MeetingConflict
		var positions []byte
		if err := rows.Scan(&item.ID, &item.MeetingID, &item.Question,
			&item.Status, &item.Cycle, &item.Round, &positions,
			&item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan Meeting conflict: %w", err)
		}
		if err := json.Unmarshal(positions, &item.Positions); err != nil {
			return nil, &Error{Code: "store_corrupt", Detail: fmt.Sprintf("decode Conflict positions: %v", err), Err: err}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func candidateQuery(ctx context.Context, source queryer, meetingID string) ([]SemanticCandidate, error) {
	rows, err := source.QueryContext(ctx, `
        SELECT id, meeting_id, kind, statement, source_refs_json, status,
               rationale, created_at, updated_at
        FROM semantic_candidates WHERE meeting_id = ? ORDER BY id`, meetingID)
	if err != nil {
		return nil, fmt.Errorf("read Semantic Candidates: %w", err)
	}
	defer rows.Close()
	var result []SemanticCandidate
	for rows.Next() {
		var item SemanticCandidate
		var refs []byte
		if err := rows.Scan(&item.ID, &item.MeetingID, &item.Kind,
			&item.Statement, &refs, &item.Status, &item.Rationale,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan Semantic Candidate: %w", err)
		}
		if err := json.Unmarshal(refs, &item.SourceRefs); err != nil {
			return nil, &Error{Code: "store_corrupt", Detail: fmt.Sprintf("decode Candidate refs: %v", err), Err: err}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ResumableMeetings(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT id FROM meetings
        WHERE status IN ('preparing', 'sealed_proposals', 'revealed',
                         'deliberating', 'recording', 'verifying')
        ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("read resumable Meetings: %w", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan resumable Meeting: %w", err)
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func (s *Store) MeetingEventsAfter(ctx context.Context, meetingID string, after int64) ([]StreamEvent, error) {
	if _, err := meetingQuery(ctx, s.db, meetingID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT de.sequence, de.event_id, de.event_type, de.aggregate_type,
               de.aggregate_id, de.payload_json, de.recorded_at
        FROM meeting_event_index mei
        JOIN domain_events de ON de.event_id = mei.event_id
        WHERE mei.meeting_id = ? AND mei.project_sequence > ?
        ORDER BY mei.project_sequence`, meetingID, after)
	if err != nil {
		return nil, fmt.Errorf("read Meeting events: %w", err)
	}
	defer rows.Close()
	var result []StreamEvent
	for rows.Next() {
		var item StreamEvent
		if err := rows.Scan(&item.Sequence, &item.EventID, &item.EventType,
			&item.AggregateType, &item.AggregateID, &item.Payload,
			&item.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan Meeting event: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type currentStateDocument struct {
	Schema     string              `json:"schema"`
	ProjectID  string              `json:"project_id"`
	Sequence   int64               `json:"sequence"`
	Meetings   []Meeting           `json:"meetings"`
	Candidates []SemanticCandidate `json:"candidates"`
}

func writeCurrentState(ctx context.Context, tx *sql.Tx, projectID string, sequence int64, updatedAt string) error {
	data, digest, err := currentStateData(ctx, tx, projectID, sequence)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
        INSERT INTO current_states(
            project_id, projected_through_sequence, document_json,
            result_digest, updated_at
        ) VALUES (?, ?, ?, ?, ?)
        ON CONFLICT(project_id) DO UPDATE SET
            projected_through_sequence = excluded.projected_through_sequence,
            document_json = excluded.document_json,
            result_digest = excluded.result_digest,
            updated_at = excluded.updated_at`, projectID, sequence, data, digest, updatedAt)
	if err != nil {
		return fmt.Errorf("write Current State: %w", err)
	}
	return nil
}

func currentStateData(ctx context.Context, source queryer, projectID string, sequence int64) ([]byte, string, error) {
	meetings, err := meetingsByProject(ctx, source, projectID)
	if err != nil {
		return nil, "", err
	}
	candidates, err := candidatesByProject(ctx, source, projectID)
	if err != nil {
		return nil, "", err
	}
	document := currentStateDocument{Schema: "nemeton.current-state.v1", ProjectID: projectID,
		Sequence: sequence, Meetings: meetings, Candidates: candidates}
	data, err := json.Marshal(document)
	if err != nil {
		return nil, "", fmt.Errorf("encode Current State: %w", err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	return data, digest, nil
}

func meetingsByProject(ctx context.Context, source queryer, projectID string) ([]Meeting, error) {
	rows, err := source.QueryContext(ctx, `
        SELECT id, project_id, reality_id, kind, title, brief, status,
               max_rounds, cycle, current_round, reality_bundle_digest,
               human_question, result_digest, created_at, updated_at
        FROM meetings WHERE project_id = ? ORDER BY created_at, id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("read Current State Meetings: %w", err)
	}
	defer rows.Close()
	result := make([]Meeting, 0)
	for rows.Next() {
		var item Meeting
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.RealityID,
			&item.Kind, &item.Title, &item.Brief, &item.Status, &item.MaxRounds,
			&item.Cycle, &item.CurrentRound, &item.RealityBundleDigest,
			&item.HumanQuestion, &item.ResultDigest, &item.CreatedAt,
			&item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan Current State Meeting: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func candidatesByProject(ctx context.Context, source queryer, projectID string) ([]SemanticCandidate, error) {
	rows, err := source.QueryContext(ctx, `
        SELECT sc.id, sc.meeting_id, sc.kind, sc.statement,
               sc.source_refs_json, sc.status, sc.rationale,
               sc.created_at, sc.updated_at
        FROM semantic_candidates sc
        JOIN meetings m ON m.id = sc.meeting_id
        WHERE m.project_id = ? ORDER BY sc.id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("read Current State Candidates: %w", err)
	}
	defer rows.Close()
	result := make([]SemanticCandidate, 0)
	for rows.Next() {
		var item SemanticCandidate
		var refs []byte
		if err := rows.Scan(&item.ID, &item.MeetingID, &item.Kind,
			&item.Statement, &refs, &item.Status, &item.Rationale,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan Current State Candidate: %w", err)
		}
		if err := json.Unmarshal(refs, &item.SourceRefs); err != nil {
			return nil, &Error{Code: "store_corrupt", Detail: fmt.Sprintf("decode Current State Candidate refs: %v", err), Err: err}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) CurrentState(ctx context.Context, projectID string) (CurrentState, error) {
	var sequence int64
	var data []byte
	var digest, updatedAt string
	err := s.db.QueryRowContext(ctx, `
        SELECT projected_through_sequence, document_json, result_digest, updated_at
        FROM current_states WHERE project_id = ?`, projectID).Scan(&sequence, &data, &digest, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return CurrentState{}, &Error{Code: "replay_required", Detail: fmt.Sprintf("Project %s Current State is missing", projectID)}
	}
	if err != nil {
		return CurrentState{}, fmt.Errorf("read Current State: %w", err)
	}
	var document currentStateDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return CurrentState{}, &Error{Code: "store_corrupt", Detail: fmt.Sprintf("decode Current State: %v", err), Err: err}
	}
	return CurrentState{ProjectID: projectID, ProjectedThroughSequence: sequence,
		Meetings: document.Meetings, Candidates: document.Candidates,
		ResultDigest: digest, UpdatedAt: updatedAt}, nil
}
