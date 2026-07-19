package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/kachofugetsu09/nemeton/internal/event"
	"github.com/kachofugetsu09/nemeton/internal/project"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

const maximumRequestBody = 1 << 20

type projectService interface {
	Open(context.Context, string, string) (store.Snapshot, error)
	Inspect(context.Context, string) (store.Snapshot, error)
	Relink(context.Context, string, string, string) (store.Snapshot, error)
	Replay(context.Context, string) (store.Snapshot, error)
	Reconcile(context.Context) (store.ReconcileResult, error)
}

type Server struct {
	service       projectService
	state         *RuntimeState
	dataDir       string
	worktreesRoot string
	handler       http.Handler
}

func NewServer(service projectService, state *RuntimeState, dataDir, worktreesRoot string) *Server {
	server := &Server{service: service, state: state, dataDir: dataDir, worktreesRoot: worktreesRoot}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", server.health)
	mux.HandleFunc("POST /v1/projects/open", server.openProject)
	mux.HandleFunc("GET /v1/projects/{project_id}", server.inspectProject)
	mux.HandleFunc("POST /v1/projects/{project_id}/relink", server.relinkProject)
	mux.HandleFunc("POST /v1/projects/{project_id}/replay", server.replayProject)
	server.handler = http.MaxBytesHandler(server.requireReady(mux), maximumRequestBody)
	return server
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
	code := project.ErrorCode(err)
	status := problemStatus(code)
	writeProblem(response, Problem{Code: code, Title: problemTitle(code), Detail: err.Error(), Status: status})
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
	case "ambiguous_integration_branch", "binding_conflict", "stream_conflict", "git_observation_changed":
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
