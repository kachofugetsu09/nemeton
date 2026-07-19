package api

import (
	"sort"
	"sync"

	"github.com/kachofugetsu09/nemeton/internal/store"
)

type RuntimeState struct {
	mu                  sync.RWMutex
	maintenanceProjects map[string]bool
	orphanArtifacts     []string
}

func NewRuntimeState(result store.ReconcileResult) *RuntimeState {
	state := &RuntimeState{}
	state.Update(result)
	return state
}

func (s *RuntimeState) Update(result store.ReconcileResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maintenanceProjects = make(map[string]bool, len(result.MaintenanceProjects))
	for _, projectID := range result.MaintenanceProjects {
		s.maintenanceProjects[projectID] = true
	}
	s.orphanArtifacts = append([]string(nil), result.OrphanArtifacts...)
}

func (s *RuntimeState) OrphanArtifacts() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.orphanArtifacts...)
}

func (s *RuntimeState) Status() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.maintenanceProjects) > 0 {
		return "maintenance"
	}
	return "ready"
}

func (s *RuntimeState) MaintenanceProjects() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	projects := make([]string, 0, len(s.maintenanceProjects))
	for projectID := range s.maintenanceProjects {
		projects = append(projects, projectID)
	}
	sort.Strings(projects)
	return projects
}

func (s *RuntimeState) InMaintenance() bool {
	return s.Status() == "maintenance"
}
