package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kachofugetsu09/nemeton/internal/event"
	"github.com/kachofugetsu09/nemeton/internal/meeting"
	"github.com/kachofugetsu09/nemeton/internal/project"
	"github.com/kachofugetsu09/nemeton/internal/realtime"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

const maximumRequestBody = 1 << 20

type projectService interface {
	Open(context.Context, string, string) (store.Snapshot, error)
	Inspect(context.Context, string) (store.Snapshot, error)
	Relink(context.Context, string, string, string) (store.Snapshot, error)
	Replay(context.Context, string) (store.Snapshot, error)
	CurrentState(context.Context, string) (store.CurrentState, error)
	Reconcile(context.Context) (store.ReconcileResult, error)
}

type Server struct {
	service       projectService
	meetings      meetingService
	hub           *realtime.Hub
	state         *RuntimeState
	dataDir       string
	worktreesRoot string
	handler       http.Handler
}

type meetingService interface {
	Create(context.Context, meeting.CreateInput) (store.MeetingSnapshot, error)
	Inspect(context.Context, string) (store.MeetingSnapshot, error)
	Start(context.Context, string) (store.MeetingSnapshot, error)
	Answer(context.Context, string, meeting.HumanInput) (store.MeetingSnapshot, error)
	Disposition(context.Context, string, meeting.DispositionInput) (store.MeetingSnapshot, error)
	Review(context.Context, string, meeting.ReviewInput) (store.MeetingSnapshot, error)
	ReadContent(context.Context, string, string) (meeting.Content, error)
	Handoff(context.Context, string) (meeting.Handoff, error)
	EventsAfter(context.Context, string, int64) ([]store.StreamEvent, error)
}

func NewServer(service projectService, meetings meetingService, hub *realtime.Hub, state *RuntimeState, dataDir, worktreesRoot string) *Server {
	server := &Server{service: service, meetings: meetings, hub: hub, state: state, dataDir: dataDir, worktreesRoot: worktreesRoot}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", server.health)
	mux.HandleFunc("POST /v1/projects/open", server.openProject)
	mux.HandleFunc("GET /v1/projects/{project_id}", server.inspectProject)
	mux.HandleFunc("GET /v1/projects/{project_id}/current-state", server.currentState)
	mux.HandleFunc("POST /v1/projects/{project_id}/relink", server.relinkProject)
	mux.HandleFunc("POST /v1/projects/{project_id}/replay", server.replayProject)
	mux.HandleFunc("POST /v1/projects/{project_id}/meetings", server.createMeeting)
	mux.HandleFunc("GET /v1/meetings/{meeting_id}", server.inspectMeeting)
	mux.HandleFunc("POST /v1/meetings/{meeting_id}/start", server.startMeeting)
	mux.HandleFunc("POST /v1/meetings/{meeting_id}/inputs", server.answerMeeting)
	mux.HandleFunc("POST /v1/meetings/{meeting_id}/ratifications", server.ratifyMeeting)
	mux.HandleFunc("POST /v1/meetings/{meeting_id}/reviews", server.reviewMeeting)
	mux.HandleFunc("GET /v1/meetings/{meeting_id}/contents/{content_id}", server.readMeetingContent)
	mux.HandleFunc("GET /v1/meetings/{meeting_id}/handoff", server.meetingHandoff)
	mux.HandleFunc("GET /v1/providers", server.providerCatalog)
	mux.HandleFunc("GET /v1/meetings/{meeting_id}/ws", server.watchMeeting)
	server.handler = http.MaxBytesHandler(server.requireReady(mux), maximumRequestBody)
	return server
}

func (s *Server) currentState(response http.ResponseWriter, request *http.Request) {
	projectID := request.PathValue("project_id")
	if err := validateProjectID(projectID); err != nil {
		writeError(response, err)
		return
	}
	current, err := s.service.CurrentState(request.Context(), projectID)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, CurrentStateResponse{CurrentState: current})
}

func (s *Server) createMeeting(response http.ResponseWriter, request *http.Request) {
	projectID := request.PathValue("project_id")
	if err := validateProjectID(projectID); err != nil {
		writeError(response, err)
		return
	}
	var input CreateMeetingRequest
	if err := decodeRequest(request, &input); err != nil {
		writeError(response, err)
		return
	}
	participants := make([]meeting.ParticipantInput, 0, len(input.Participants))
	for _, participant := range input.Participants {
		participants = append(participants, meeting.ParticipantInput{Seat: participant.Seat,
			Role: participant.Role, Provider: participant.Provider, Model: participant.Model,
			ProviderOptions: participant.ProviderOptions})
	}
	snapshot, err := s.meetings.Create(request.Context(), meeting.CreateInput{
		ProjectID: projectID, Kind: input.Kind, Title: input.Title,
		Brief: input.Brief, Providers: input.Providers, Participants: participants})
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, MeetingResponse{Meeting: snapshot})
}

func (s *Server) reviewMeeting(response http.ResponseWriter, request *http.Request) {
	var input MeetingReviewRequest
	if err := decodeRequest(request, &input); err != nil {
		writeError(response, err)
		return
	}
	items := make([]event.SemanticReviewItem, 0, len(input.Items))
	for _, item := range input.Items {
		items = append(items, event.SemanticReviewItem{CandidateID: item.CandidateID,
			DesignDisposition: item.DesignDisposition, ContextDisposition: item.ContextDisposition})
	}
	snapshot, err := s.meetings.Review(request.Context(), request.PathValue("meeting_id"),
		meeting.ReviewInput{ResultAction: input.ResultAction, Items: items, Comment: input.Comment})
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusAccepted, MeetingResponse{Meeting: snapshot})
}

func (s *Server) readMeetingContent(response http.ResponseWriter, request *http.Request) {
	content, err := s.meetings.ReadContent(request.Context(), request.PathValue("meeting_id"),
		request.PathValue("content_id"))
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, ContentResponse{Content: content})
}

func (s *Server) meetingHandoff(response http.ResponseWriter, request *http.Request) {
	handoff, err := s.meetings.Handoff(request.Context(), request.PathValue("meeting_id"))
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, HandoffResponse{Handoff: handoff})
}

func (s *Server) providerCatalog(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, ProviderCatalog{Providers: []ProviderDescriptor{
		{ID: "codex", Label: "Codex", ModelMode: "editable", KnownModels: []string{"gpt-5.4-mini"},
			Option: "reasoning_effort", OptionValues: []string{"low", "medium", "high", "xhigh"}},
		{ID: "opencode", Label: "OpenCode", ModelMode: "editable", KnownModels: []string{"opencode-go/deepseek-v4-pro"},
			Option: "variant", OptionValues: []string{"low", "medium", "high"}},
	}})
}

func (s *Server) inspectMeeting(response http.ResponseWriter, request *http.Request) {
	meetingID := request.PathValue("meeting_id")
	snapshot, err := s.meetings.Inspect(request.Context(), meetingID)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, MeetingResponse{Meeting: snapshot})
}

func (s *Server) startMeeting(response http.ResponseWriter, request *http.Request) {
	var input struct{}
	if err := decodeRequest(request, &input); err != nil {
		writeError(response, err)
		return
	}
	snapshot, err := s.meetings.Start(request.Context(), request.PathValue("meeting_id"))
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusAccepted, MeetingResponse{Meeting: snapshot})
}

func (s *Server) answerMeeting(response http.ResponseWriter, request *http.Request) {
	var input HumanInputRequest
	if err := decodeRequest(request, &input); err != nil {
		writeError(response, err)
		return
	}
	snapshot, err := s.meetings.Answer(request.Context(), request.PathValue("meeting_id"), meeting.HumanInput{Content: input.Content})
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusAccepted, MeetingResponse{Meeting: snapshot})
}

func (s *Server) ratifyMeeting(response http.ResponseWriter, request *http.Request) {
	var input RatificationRequest
	if err := decodeRequest(request, &input); err != nil {
		writeError(response, err)
		return
	}
	snapshot, err := s.meetings.Disposition(request.Context(), request.PathValue("meeting_id"), meeting.DispositionInput{
		CandidateID: input.CandidateID, Disposition: input.Disposition, Reason: input.Reason})
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, MeetingResponse{Meeting: snapshot})
}

func (s *Server) watchMeeting(response http.ResponseWriter, request *http.Request) {
	meetingID := request.PathValue("meeting_id")
	if _, err := s.meetings.Inspect(request.Context(), meetingID); err != nil {
		writeError(response, err)
		return
	}
	after := request.URL.Query().Get("after_sequence")
	if after != "" {
		sequence, err := strconv.ParseInt(after, 10, 64)
		if err != nil || sequence < 0 {
			writeProblem(response, Problem{Code: "invalid_argument",
				Title: problemTitle("invalid_argument"), Detail: "after_sequence must be a non-negative integer",
				Status: http.StatusBadRequest})
			return
		}
	}
	_ = s.hub.Serve(s.meetings, response, request, meetingID)
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) requireReady(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !s.state.InMaintenance() || request.URL.Path == "/v1/health" || strings.HasSuffix(request.URL.Path, "/replay") {
			next.ServeHTTP(response, request)
			return
		}
		writeProblem(response, Problem{Code: "replay_required", Title: "Projection replay required", Detail: fmt.Sprintf("daemon is in maintenance mode; replay Projects %v", s.state.MaintenanceProjects()), Status: http.StatusServiceUnavailable})
	})
}

func (s *Server) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, Health{Status: s.state.Status(), DataDir: s.dataDir, WorktreesRoot: s.worktreesRoot, MaintenanceProjects: s.state.MaintenanceProjects(), OrphanArtifacts: s.state.OrphanArtifacts()})
}

func (s *Server) openProject(response http.ResponseWriter, request *http.Request) {
	var input OpenRequest
	if err := decodeRequest(request, &input); err != nil {
		writeError(response, err)
		return
	}
	if err := validateAbsolutePath(input.Path); err != nil {
		writeError(response, err)
		return
	}
	snapshot, err := s.service.Open(request.Context(), input.Path, input.IntegrationBranch)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, ProjectResponse{Project: snapshot})
}

func (s *Server) inspectProject(response http.ResponseWriter, request *http.Request) {
	projectID := request.PathValue("project_id")
	if err := validateProjectID(projectID); err != nil {
		writeError(response, err)
		return
	}
	snapshot, err := s.service.Inspect(request.Context(), projectID)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, ProjectResponse{Project: snapshot})
}

func (s *Server) relinkProject(response http.ResponseWriter, request *http.Request) {
	projectID := request.PathValue("project_id")
	if err := validateProjectID(projectID); err != nil {
		writeError(response, err)
		return
	}
	var input RelinkRequest
	if err := decodeRequest(request, &input); err != nil {
		writeError(response, err)
		return
	}
	if err := validateAbsolutePath(input.Path); err != nil {
		writeError(response, err)
		return
	}
	snapshot, err := s.service.Relink(request.Context(), projectID, input.Path, input.IntegrationBranch)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, ProjectResponse{Project: snapshot})
}

func (s *Server) replayProject(response http.ResponseWriter, request *http.Request) {
	projectID := request.PathValue("project_id")
	if err := validateProjectID(projectID); err != nil {
		writeError(response, err)
		return
	}
	var input struct{}
	if err := decodeRequest(request, &input); err != nil {
		writeError(response, err)
		return
	}
	snapshot, err := s.service.Replay(request.Context(), projectID)
	if err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.Reconcile(request.Context())
	if err != nil {
		writeError(response, err)
		return
	}
	s.state.Update(result)
	writeJSON(response, http.StatusOK, ProjectResponse{Project: snapshot})
}

func decodeRequest(request *http.Request, target any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &project.Error{Code: "invalid_argument", Detail: fmt.Sprintf("decode request JSON: %v", err), Err: err}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("unexpected trailing JSON value")
		}
		return &project.Error{Code: "invalid_argument", Detail: fmt.Sprintf("decode request JSON: %v", err), Err: err}
	}
	return nil
}

func validateAbsolutePath(path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return &project.Error{Code: "invalid_argument", Detail: "path must be absolute"}
	}
	if len(path) > 4096 {
		return &project.Error{Code: "invalid_argument", Detail: "path exceeds 4096 bytes"}
	}
	return nil
}

func validateProjectID(projectID string) error {
	if !event.ValidID(projectID) {
		return &project.Error{Code: "invalid_argument", Detail: fmt.Sprintf("invalid Project ID: %s", projectID)}
	}
	return nil
}

func writeError(response http.ResponseWriter, err error) {
	code := errorCode(err)
	status := problemStatus(code)
	writeProblem(response, Problem{Code: code, Title: problemTitle(code), Detail: err.Error(), Status: status})
}

func errorCode(err error) string {
	var meetingError *meeting.Error
	if errors.As(err, &meetingError) {
		return meetingError.Code
	}
	return project.ErrorCode(err)
}

func writeProblem(response http.ResponseWriter, problem Problem) {
	data, err := json.Marshal(problem)
	if err != nil {
		http.Error(response, "encode problem response", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "application/problem+json")
	response.WriteHeader(problem.Status)
	_, _ = response.Write(append(data, '\n'))
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(response, `{"code":"internal_error","title":"Internal error","detail":"encode response","status":500}`, http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_, _ = response.Write(append(data, '\n'))
}

func problemStatus(code string) int {
	switch code {
	case "invalid_argument":
		return http.StatusBadRequest
	case "project_not_found":
		return http.StatusNotFound
	case "meeting_not_found":
		return http.StatusNotFound
	case "ambiguous_integration_branch", "binding_conflict", "stream_conflict", "git_observation_changed", "invalid_meeting_state":
		return http.StatusConflict
	case "not_git_repository", "unsupported_bare_repository", "integration_branch_not_found", "broken_git_common_dir":
		return http.StatusUnprocessableEntity
	case "replay_required":
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func problemTitle(code string) string {
	return strings.ReplaceAll(code, "_", " ")
}
