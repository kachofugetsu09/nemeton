package api

import (
	"github.com/kachofugetsu09/nemeton/internal/meeting"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

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
	Kind         string               `json:"kind,omitempty"`
	Title        string               `json:"title"`
	Brief        string               `json:"brief"`
	Providers    map[string]string    `json:"providers,omitempty"`
	Participants []ParticipantRequest `json:"participants,omitempty"`
}

type ParticipantRequest struct {
	Seat            string            `json:"seat"`
	Role            string            `json:"role"`
	Provider        string            `json:"provider"`
	Model           string            `json:"model"`
	ProviderOptions map[string]string `json:"provider_options,omitempty"`
}

type HumanInputRequest struct {
	Content string `json:"content"`
}

type RatificationRequest struct {
	CandidateID string `json:"candidate_id"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason,omitempty"`
}

type ReviewItemRequest struct {
	CandidateID        string `json:"candidate_id"`
	DesignDisposition  string `json:"design_disposition"`
	ContextDisposition string `json:"context_disposition"`
}

type MeetingReviewRequest struct {
	ResultAction string              `json:"result_action"`
	Items        []ReviewItemRequest `json:"items"`
	Comment      string              `json:"comment,omitempty"`
}

type ContentResponse struct {
	Content meeting.Content `json:"content"`
}

type HandoffResponse struct {
	Handoff  meeting.Handoff `json:"handoff"`
	Markdown string          `json:"markdown"`
}

type ProviderCatalog struct {
	Providers []ProviderDescriptor `json:"providers"`
}

type ProviderDescriptor struct {
	ID        string          `json:"id"`
	Label     string          `json:"label"`
	Version   string          `json:"version,omitempty"`
	ModelMode string          `json:"model_mode"`
	Models    []ProviderModel `json:"models"`
	Option    string          `json:"option"`
}

type ProviderModel struct {
	ID            string   `json:"id"`
	Label         string   `json:"label"`
	Description   string   `json:"description,omitempty"`
	Default       bool     `json:"default"`
	OptionValues  []string `json:"option_values"`
	DefaultOption string   `json:"default_option"`
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
