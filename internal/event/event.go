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
	ProjectRegistered  = "ProjectRegistered.v1"
	RepositoryBound    = "RepositoryBound.v1"
	RepositoryRelinked = "RepositoryRelinked.v1"
	RealityCaptured    = "RealityCaptured.v1"
	SchemaVersion      = 1
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
