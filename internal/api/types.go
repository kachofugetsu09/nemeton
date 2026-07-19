package api

import "github.com/kachofugetsu09/nemeton/internal/store"

type Health struct {
	Status              string   `json:"status"`
	DataDir             string   `json:"data_dir"`
	WorktreesRoot       string   `json:"worktrees_root"`
	MaintenanceProjects []string `json:"maintenance_projects"`
	OrphanArtifacts     []string `json:"orphan_artifacts"`
}

type OpenRequest struct {
	Path              string `json:"path"`
	IntegrationBranch string `json:"integration_branch,omitempty"`
}

type RelinkRequest struct {
	Path              string `json:"path"`
	IntegrationBranch string `json:"integration_branch,omitempty"`
}

type ProjectResponse struct {
	Project store.Snapshot `json:"project"`
}

type CurrentStateResponse struct {
	CurrentState store.CurrentState `json:"current_state"`
}

type CreateMeetingRequest struct {
	Kind      string            `json:"kind,omitempty"`
	Title     string            `json:"title"`
	Brief     string            `json:"brief"`
	Providers map[string]string `json:"providers,omitempty"`
}

type HumanInputRequest struct {
	Content string `json:"content"`
}

type RatificationRequest struct {
	CandidateID string `json:"candidate_id"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason,omitempty"`
}

type MeetingResponse struct {
	Meeting store.MeetingSnapshot `json:"meeting"`
}

type Problem struct {
	Code   string `json:"code"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
}

func (p *Problem) Error() string {
	return p.Detail
}
