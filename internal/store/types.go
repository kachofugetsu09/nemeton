package store

import (
	"encoding/json"

	"github.com/kachofugetsu09/nemeton/internal/event"
)

const ProjectionSchemaVersion = 1

type Project struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Binding struct {
	ProjectID           string                 `json:"project_id"`
	DisplayPath         string                 `json:"display_path"`
	CanonicalRoot       string                 `json:"canonical_root"`
	GitCommonDir        string                 `json:"git_common_dir"`
	RemoteIdentities    []event.RemoteIdentity `json:"remote_identities"`
	DiscoveredWorktrees []string               `json:"discovered_worktrees"`
	IntegrationBranch   string                 `json:"integration_branch"`
	ManagedWorktreeRoot string                 `json:"managed_worktree_root"`
	UpdatedAt           string                 `json:"updated_at"`
}

type Reality struct {
	ID                string `json:"id"`
	ProjectID         string `json:"project_id"`
	Sequence          int64  `json:"sequence"`
	IntegrationBranch string `json:"integration_branch"`
	CommitOID         string `json:"commit_oid"`
	TreeOID           string `json:"tree_oid"`
	ManifestDigest    string `json:"manifest_digest"`
	DirtyObserved     bool   `json:"dirty_observed"`
	Status            string `json:"status"`
	CapturedAt        string `json:"captured_at"`
}

type Snapshot struct {
	Project        Project `json:"project"`
	Binding        Binding `json:"binding"`
	CurrentReality Reality `json:"current_reality"`
	StreamVersion  int64   `json:"stream_version"`
	ResultDigest   string  `json:"result_digest"`
}

type AppendRequest struct {
	ProjectID       string
	ExpectedVersion int64
	Events          []event.Envelope
}

type Meeting struct {
	ID                   string `json:"id"`
	ProjectID            string `json:"project_id"`
	RealityID            string `json:"reality_id"`
	Kind                 string `json:"kind"`
	Title                string `json:"title"`
	Brief                string `json:"brief"`
	Status               string `json:"status"`
	MaxRounds            int    `json:"max_rounds"`
	Cycle                int    `json:"cycle"`
	CurrentRound         int    `json:"current_round"`
	RealityBundleDigest  string `json:"reality_bundle_digest"`
	HumanQuestion        string `json:"human_question,omitempty"`
	ResultDigest         string `json:"result_digest"`
	ResultContentID      string `json:"result_content_id,omitempty"`
	ApprovedResultDigest string `json:"approved_result_digest,omitempty"`
	ProtocolVersion      int    `json:"protocol_version"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

type MeetingParticipant struct {
	ID              string            `json:"id"`
	MeetingID       string            `json:"meeting_id"`
	Seat            string            `json:"seat"`
	Role            string            `json:"role"`
	Provider        string            `json:"provider"`
	Model           string            `json:"model,omitempty"`
	ProviderOptions map[string]string `json:"provider_options,omitempty"`
	Status          string            `json:"status"`
	SessionID       string            `json:"session_id,omitempty"`
	Workdir         string            `json:"workdir"`
	ProposalDigest  string            `json:"proposal_digest,omitempty"`
	UpdatedAt       string            `json:"updated_at"`
}

type AgentRun struct {
	ID              string   `json:"id"`
	MeetingID       string   `json:"meeting_id"`
	ParticipantID   string   `json:"participant_id"`
	Phase           string   `json:"phase"`
	Provider        string   `json:"provider"`
	ProviderVersion string   `json:"provider_version,omitempty"`
	Command         []string `json:"command,omitempty"`
	Status          string   `json:"status"`
	SessionID       string   `json:"session_id,omitempty"`
	InputDigest     string   `json:"input_digest"`
	OutputDigest    string   `json:"output_digest,omitempty"`
	RawStreamDigest string   `json:"raw_stream_digest,omitempty"`
	StderrDigest    string   `json:"stderr_digest,omitempty"`
	Workdir         string   `json:"workdir"`
	ExitCode        int      `json:"exit_code"`
	Error           string   `json:"error,omitempty"`
	StartedAt       string   `json:"started_at"`
	FinishedAt      string   `json:"finished_at,omitempty"`
}

type MeetingContent struct {
	ID            string   `json:"id"`
	MeetingID     string   `json:"meeting_id"`
	ParticipantID string   `json:"participant_id,omitempty"`
	Kind          string   `json:"kind"`
	Cycle         int      `json:"cycle"`
	Round         int      `json:"round"`
	ContentDigest string   `json:"content_digest"`
	Refs          []string `json:"refs"`
	CreatedAt     string   `json:"created_at"`
}

type MeetingConflict struct {
	ID        string         `json:"id"`
	MeetingID string         `json:"meeting_id"`
	Question  string         `json:"question"`
	Status    string         `json:"status"`
	Cycle     int            `json:"cycle"`
	Round     int            `json:"round"`
	Positions map[string]any `json:"positions"`
	UpdatedAt string         `json:"updated_at"`
}

type SemanticCandidate struct {
	ID                 string   `json:"id"`
	MeetingID          string   `json:"meeting_id"`
	Kind               string   `json:"kind"`
	Statement          string   `json:"statement"`
	SourceRefs         []string `json:"source_refs"`
	Status             string   `json:"status"`
	DesignDisposition  string   `json:"design_disposition"`
	ContextDisposition string   `json:"context_disposition"`
	Rationale          string   `json:"rationale"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
}

type MeetingSnapshot struct {
	Meeting            Meeting              `json:"meeting"`
	Participants       []MeetingParticipant `json:"participants"`
	Runs               []AgentRun           `json:"runs"`
	Contents           []MeetingContent     `json:"contents"`
	Conflicts          []MeetingConflict    `json:"conflicts"`
	Candidates         []SemanticCandidate  `json:"candidates"`
	StreamVersion      int64                `json:"stream_version"`
	CurrentStateDigest string               `json:"current_state_digest"`
}

type CurrentState struct {
	ProjectID                string              `json:"project_id"`
	ProjectedThroughSequence int64               `json:"projected_through_sequence"`
	Meetings                 []Meeting           `json:"meetings"`
	Candidates               []SemanticCandidate `json:"candidates"`
	ApprovedContext          []SemanticCandidate `json:"approved_context"`
	ResultDigest             string              `json:"result_digest"`
	UpdatedAt                string              `json:"updated_at"`
}

type StreamEvent struct {
	Sequence      int64           `json:"sequence"`
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	Payload       json.RawMessage `json:"payload"`
	RecordedAt    string          `json:"recorded_at"`
}

type Error struct {
	Code   string
	Detail string
	Err    error
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

type ReconcileResult struct {
	MaintenanceProjects []string `json:"maintenance_projects"`
	OrphanArtifacts     []string `json:"orphan_artifacts"`
}

func (r ReconcileResult) Ready() bool {
	return len(r.MaintenanceProjects) == 0
}
