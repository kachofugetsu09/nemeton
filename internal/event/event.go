package event

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

const (
	ProjectRegistered              = "ProjectRegistered.v1"
	RepositoryBound                = "RepositoryBound.v1"
	RepositoryRelinked             = "RepositoryRelinked.v1"
	RealityCaptured                = "RealityCaptured.v1"
	MeetingCreated                 = "MeetingCreated.v1"
	ParticipantAdded               = "MeetingParticipantAdded.v1"
	MeetingStatusSet               = "MeetingStatusSet.v1"
	AgentRunRecorded               = "AgentRunRecorded.v1"
	MeetingContentAdded            = "MeetingContentAdded.v1"
	MeetingConflictSet             = "MeetingConflictSet.v1"
	SemanticCandidateAdded         = "SemanticCandidateAdded.v1"
	SemanticCandidateDispositioned = "SemanticCandidateDispositioned.v1"
	MeetingCreatedV2               = "MeetingCreated.v2"
	ParticipantAddedV2             = "MeetingParticipantAdded.v2"
	MeetingReviewed                = "MeetingReviewed.v1"
	MeetingResultApproved          = "MeetingResultApproved.v1"
	SchemaVersion                  = 1
)

var idPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Envelope struct {
	EventID       string
	ProjectID     string
	Sequence      int64
	AggregateType string
	AggregateID   string
	EventType     string
	SchemaVersion int
	PayloadJSON   []byte
	Actor         string
	CausationID   string
	CorrelationID string
	OccurredAt    string
	RecordedAt    string
	Artifacts     []ArtifactRef
}

type ArtifactRef struct {
	Digest     string `json:"digest"`
	Relation   string `json:"relation"`
	MediaType  string `json:"media_type"`
	ByteSize   int64  `json:"byte_size"`
	StorageURI string `json:"storage_uri"`
}

type ProjectRegisteredPayload struct {
	ProjectID string `json:"project_id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type RemoteIdentity struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type RepositoryBindingPayload struct {
	ProjectID           string           `json:"project_id"`
	DisplayPath         string           `json:"display_path"`
	CanonicalRoot       string           `json:"canonical_root"`
	GitCommonDir        string           `json:"git_common_dir"`
	RemoteIdentities    []RemoteIdentity `json:"remote_identities"`
	DiscoveredWorktrees []string         `json:"discovered_worktrees"`
	IntegrationBranch   string           `json:"integration_branch"`
	ManagedWorktreeRoot string           `json:"managed_worktree_root"`
	BoundAt             string           `json:"bound_at"`
}

type RepositoryRelinkedPayload struct {
	RepositoryBindingPayload
	PreviousGitCommonDir      string `json:"previous_git_common_dir"`
	PreviousIntegrationBranch string `json:"previous_integration_branch"`
}

type RealityCapturedPayload struct {
	RealityID         string `json:"reality_id"`
	ProjectID         string `json:"project_id"`
	IntegrationBranch string `json:"integration_branch"`
	CommitOID         string `json:"commit_oid"`
	TreeOID           string `json:"tree_oid"`
	ManifestDigest    string `json:"manifest_digest"`
	DirtyObserved     bool   `json:"dirty_observed"`
	Status            string `json:"status"`
	CapturedAt        string `json:"captured_at"`
}

type MeetingCreatedPayload struct {
	MeetingID           string `json:"meeting_id"`
	ProjectID           string `json:"project_id"`
	RealityID           string `json:"reality_id"`
	Kind                string `json:"kind"`
	Title               string `json:"title"`
	Brief               string `json:"brief"`
	Status              string `json:"status"`
	MaxRounds           int    `json:"max_rounds"`
	Cycle               int    `json:"cycle"`
	CurrentRound        int    `json:"current_round"`
	RealityBundleDigest string `json:"reality_bundle_digest"`
	CreatedAt           string `json:"created_at"`
}

type MeetingCreatedV2Payload struct {
	MeetingCreatedPayload
	ProtocolVersion int `json:"protocol_version"`
}

type MeetingParticipantAddedPayload struct {
	MeetingID     string `json:"meeting_id"`
	ParticipantID string `json:"participant_id"`
	Seat          string `json:"seat"`
	Role          string `json:"role"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Status        string `json:"status"`
	Workdir       string `json:"workdir"`
	AddedAt       string `json:"added_at"`
}

type MeetingParticipantAddedV2Payload struct {
	MeetingParticipantAddedPayload
	ProviderOptions map[string]string `json:"provider_options"`
}

type MeetingStatusSetPayload struct {
	MeetingID       string `json:"meeting_id"`
	Status          string `json:"status"`
	Cycle           int    `json:"cycle"`
	CurrentRound    int    `json:"current_round"`
	HumanQuestion   string `json:"human_question"`
	ResultDigest    string `json:"result_digest"`
	ResultContentID string `json:"result_content_id,omitempty"`
	UpdatedAt       string `json:"updated_at"`
}

type SemanticReviewItem struct {
	CandidateID        string `json:"candidate_id"`
	DesignDisposition  string `json:"design_disposition"`
	ContextDisposition string `json:"context_disposition"`
}

type MeetingReviewedPayload struct {
	MeetingID       string               `json:"meeting_id"`
	ResultContentID string               `json:"result_content_id"`
	ResultAction    string               `json:"result_action"`
	Items           []SemanticReviewItem `json:"items"`
	CommentDigest   string               `json:"comment_digest,omitempty"`
	ReviewedAt      string               `json:"reviewed_at"`
}

type MeetingResultApprovedPayload struct {
	MeetingID       string `json:"meeting_id"`
	ResultContentID string `json:"result_content_id"`
	ResultDigest    string `json:"result_digest"`
	ApprovedAt      string `json:"approved_at"`
}

type AgentRunRecordedPayload struct {
	MeetingID       string   `json:"meeting_id"`
	RunID           string   `json:"run_id"`
	ParticipantID   string   `json:"participant_id"`
	Phase           string   `json:"phase"`
	Provider        string   `json:"provider"`
	ProviderVersion string   `json:"provider_version"`
	Command         []string `json:"command"`
	Status          string   `json:"status"`
	SessionID       string   `json:"session_id"`
	InputDigest     string   `json:"input_digest"`
	OutputDigest    string   `json:"output_digest"`
	RawStreamDigest string   `json:"raw_stream_digest"`
	StderrDigest    string   `json:"stderr_digest"`
	Workdir         string   `json:"workdir"`
	ExitCode        int      `json:"exit_code"`
	Error           string   `json:"error"`
	StartedAt       string   `json:"started_at"`
	FinishedAt      string   `json:"finished_at"`
}

type MeetingContentAddedPayload struct {
	MeetingID     string   `json:"meeting_id"`
	ContentID     string   `json:"content_id"`
	ParticipantID string   `json:"participant_id"`
	Kind          string   `json:"kind"`
	Cycle         int      `json:"cycle"`
	Round         int      `json:"round"`
	ContentDigest string   `json:"content_digest"`
	Refs          []string `json:"refs"`
	CreatedAt     string   `json:"created_at"`
}

type MeetingConflictSetPayload struct {
	MeetingID  string         `json:"meeting_id"`
	ConflictID string         `json:"conflict_id"`
	Question   string         `json:"question"`
	Status     string         `json:"status"`
	Cycle      int            `json:"cycle"`
	Round      int            `json:"round"`
	Positions  map[string]any `json:"positions"`
	UpdatedAt  string         `json:"updated_at"`
}

type SemanticCandidateAddedPayload struct {
	MeetingID   string   `json:"meeting_id"`
	CandidateID string   `json:"candidate_id"`
	Kind        string   `json:"kind"`
	Statement   string   `json:"statement"`
	SourceRefs  []string `json:"source_refs"`
	Status      string   `json:"status"`
	Rationale   string   `json:"rationale"`
	CreatedAt   string   `json:"created_at"`
}

type SemanticCandidateDispositionedPayload struct {
	MeetingID          string `json:"meeting_id"`
	CandidateID        string `json:"candidate_id"`
	Status             string `json:"status"`
	DesignDisposition  string `json:"design_disposition,omitempty"`
	ContextDisposition string `json:"context_disposition,omitempty"`
	Reason             string `json:"reason"`
	UpdatedAt          string `json:"updated_at"`
}

func NewID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate random ID: %w", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func ValidID(value string) bool {
	return idPattern.MatchString(value)
}

func NewEnvelope(projectID, aggregateType, aggregateID, eventType, actor, correlationID string, payload any, now time.Time) (Envelope, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("encode %s payload: %w", eventType, err)
	}
	eventID, err := NewID()
	if err != nil {
		return Envelope{}, err
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	return Envelope{
		EventID:       eventID,
		ProjectID:     projectID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		SchemaVersion: SchemaVersion,
		PayloadJSON:   data,
		Actor:         actor,
		CausationID:   correlationID,
		CorrelationID: correlationID,
		OccurredAt:    stamp,
		RecordedAt:    stamp,
	}, nil
}
