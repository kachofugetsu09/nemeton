package store

import "github.com/kachofugetsu09/nemeton/internal/event"

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
