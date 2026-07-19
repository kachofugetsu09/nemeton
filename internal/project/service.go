package project

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kachofugetsu09/nemeton/internal/artifact"
	"github.com/kachofugetsu09/nemeton/internal/event"
	"github.com/kachofugetsu09/nemeton/internal/gitrepo"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

type Service struct {
	store         *store.Store
	artifacts     artifact.Store
	worktreesRoot string
}

func NewService(database *store.Store, artifacts artifact.Store, worktreesRoot string) *Service {
	return &Service{store: database, artifacts: artifacts, worktreesRoot: worktreesRoot}
}

func (s *Service) Open(ctx context.Context, path, requestedBranch string) (store.Snapshot, error) {
	// 1. Resolve repository identity before changing persistent state.
	repository, err := gitrepo.Discover(ctx, path)
	if err != nil {
		return store.Snapshot{}, err
	}
	existing, found, err := s.store.FindByCommonDir(ctx, repository.GitCommonDir)
	if err != nil {
		return store.Snapshot{}, err
	}
	if found {
		if requestedBranch != "" && requestedBranch != existing.Binding.IntegrationBranch {
			return store.Snapshot{}, &Error{Code: "binding_conflict", Detail: fmt.Sprintf("Project %s is already bound to integration branch %s; use project relink to change it", existing.Project.ID, existing.Binding.IntegrationBranch)}
		}
		return s.captureExisting(ctx, repository, existing)
	}

	// 2. Confirm the integration branch and committed Reality.
	branch, err := repository.SelectIntegrationBranch(ctx, requestedBranch)
	if err != nil {
		return store.Snapshot{}, err
	}
	capture, manifest, err := repository.Capture(ctx, branch)
	if err != nil {
		return store.Snapshot{}, err
	}
	record, err := s.artifacts.Put(manifest, "application/vnd.nemeton.reality+json;version=1")
	if err != nil {
		return store.Snapshot{}, err
	}

	// 3. Register Project, binding, and Reality in one event transaction.
	projectID, err := event.NewID()
	if err != nil {
		return store.Snapshot{}, err
	}
	realityID, err := event.NewID()
	if err != nil {
		return store.Snapshot{}, err
	}
	correlationID, err := event.NewID()
	if err != nil {
		return store.Snapshot{}, err
	}
	now := time.Now().UTC()
	events, err := s.registrationEvents(repository, capture, record, projectID, realityID, correlationID, now)
	if err != nil {
		return store.Snapshot{}, err
	}
	return s.store.Append(ctx, store.AppendRequest{ProjectID: projectID, ExpectedVersion: 0, Events: events})
}

func (s *Service) Inspect(ctx context.Context, projectID string) (store.Snapshot, error) {
	return s.store.Inspect(ctx, projectID)
}

func (s *Service) Relink(ctx context.Context, projectID, path, requestedBranch string) (store.Snapshot, error) {
	// 1. Load the stable Project before accepting a new repository binding.
	current, err := s.store.Inspect(ctx, projectID)
	if err != nil {
		return store.Snapshot{}, err
	}
	repository, err := gitrepo.Discover(ctx, path)
	if err != nil {
		return store.Snapshot{}, err
	}
	if other, found, err := s.store.FindByCommonDir(ctx, repository.GitCommonDir); err != nil {
		return store.Snapshot{}, err
	} else if found && other.Project.ID != projectID {
		return store.Snapshot{}, &Error{Code: "binding_conflict", Detail: fmt.Sprintf("Git common-dir is already bound to Project %s", other.Project.ID)}
	}

	// 2. Explicitly confirm the replacement branch and committed Reality.
	branch, err := repository.SelectIntegrationBranch(ctx, requestedBranch)
	if err != nil {
		return store.Snapshot{}, err
	}
	capture, manifest, err := repository.Capture(ctx, branch)
	if err != nil {
		return store.Snapshot{}, err
	}
	record, err := s.artifacts.Put(manifest, "application/vnd.nemeton.reality+json;version=1")
	if err != nil {
		return store.Snapshot{}, err
	}

	// 3. Append relink and Reality events at the current stream revision.
	realityID, err := event.NewID()
	if err != nil {
		return store.Snapshot{}, err
	}
	correlationID, err := event.NewID()
	if err != nil {
		return store.Snapshot{}, err
	}
	now := time.Now().UTC()
	events, err := s.relinkEvents(repository, capture, record, current, realityID, correlationID, now)
	if err != nil {
		return store.Snapshot{}, err
	}
	return s.store.Append(ctx, store.AppendRequest{ProjectID: projectID, ExpectedVersion: current.StreamVersion, Events: events})
}

func (s *Service) Replay(ctx context.Context, projectID string) (store.Snapshot, error) {
	return s.store.Replay(ctx, projectID)
}

func (s *Service) Reconcile(ctx context.Context) (store.ReconcileResult, error) {
	return s.store.Reconcile(ctx, s.artifacts)
}

func (s *Service) captureExisting(ctx context.Context, repository gitrepo.Repository, current store.Snapshot) (store.Snapshot, error) {
	capture, manifest, err := repository.Capture(ctx, current.Binding.IntegrationBranch)
	if err != nil {
		return store.Snapshot{}, err
	}
	record, err := s.artifacts.Put(manifest, "application/vnd.nemeton.reality+json;version=1")
	if err != nil {
		return store.Snapshot{}, err
	}
	realityID, err := event.NewID()
	if err != nil {
		return store.Snapshot{}, err
	}
	correlationID, err := event.NewID()
	if err != nil {
		return store.Snapshot{}, err
	}
	item, err := realityEvent(current.Project.ID, realityID, capture, record, correlationID, time.Now().UTC())
	if err != nil {
		return store.Snapshot{}, err
	}
	return s.store.Append(ctx, store.AppendRequest{ProjectID: current.Project.ID, ExpectedVersion: current.StreamVersion, Events: []event.Envelope{item}})
}

func (s *Service) registrationEvents(repository gitrepo.Repository, capture gitrepo.Capture, record artifact.Record, projectID, realityID, correlationID string, now time.Time) ([]event.Envelope, error) {
	registered, err := event.NewEnvelope(projectID, "project", projectID, event.ProjectRegistered, "human", correlationID, event.ProjectRegisteredPayload{ProjectID: projectID, Status: "active", CreatedAt: now.Format(time.RFC3339Nano)}, now)
	if err != nil {
		return nil, err
	}
	bound, err := event.NewEnvelope(projectID, "repository_binding", projectID, event.RepositoryBound, "human", correlationID, bindingPayload(repository, projectID, capture.IntegrationBranch, s.worktreesRoot, now), now)
	if err != nil {
		return nil, err
	}
	reality, err := realityEvent(projectID, realityID, capture, record, correlationID, now)
	if err != nil {
		return nil, err
	}
	return []event.Envelope{registered, bound, reality}, nil
}

func (s *Service) relinkEvents(repository gitrepo.Repository, capture gitrepo.Capture, record artifact.Record, current store.Snapshot, realityID, correlationID string, now time.Time) ([]event.Envelope, error) {
	payload := event.RepositoryRelinkedPayload{
		RepositoryBindingPayload:  bindingPayload(repository, current.Project.ID, capture.IntegrationBranch, s.worktreesRoot, now),
		PreviousGitCommonDir:      current.Binding.GitCommonDir,
		PreviousIntegrationBranch: current.Binding.IntegrationBranch,
	}
	relinked, err := event.NewEnvelope(current.Project.ID, "repository_binding", current.Project.ID, event.RepositoryRelinked, "human", correlationID, payload, now)
	if err != nil {
		return nil, err
	}
	reality, err := realityEvent(current.Project.ID, realityID, capture, record, correlationID, now)
	if err != nil {
		return nil, err
	}
	return []event.Envelope{relinked, reality}, nil
}

func realityEvent(projectID, realityID string, capture gitrepo.Capture, record artifact.Record, correlationID string, now time.Time) (event.Envelope, error) {
	payload := event.RealityCapturedPayload{
		RealityID:         realityID,
		ProjectID:         projectID,
		IntegrationBranch: capture.IntegrationBranch,
		CommitOID:         capture.CommitOID,
		TreeOID:           capture.TreeOID,
		ManifestDigest:    record.Digest,
		DirtyObserved:     capture.DirtyObserved,
		Status:            "current",
		CapturedAt:        now.Format(time.RFC3339Nano),
	}
	item, err := event.NewEnvelope(projectID, "reality", realityID, event.RealityCaptured, "human", correlationID, payload, now)
	if err != nil {
		return event.Envelope{}, err
	}
	item.Artifacts = []event.ArtifactRef{{Digest: record.Digest, Relation: "reality_manifest", MediaType: record.MediaType, ByteSize: record.ByteSize, StorageURI: record.StorageURI}}
	return item, nil
}

func bindingPayload(repository gitrepo.Repository, projectID, integrationBranch, worktreesRoot string, now time.Time) event.RepositoryBindingPayload {
	return event.RepositoryBindingPayload{
		ProjectID:           projectID,
		DisplayPath:         repository.DisplayPath,
		CanonicalRoot:       repository.CanonicalRoot,
		GitCommonDir:        repository.GitCommonDir,
		RemoteIdentities:    repository.RemoteIdentities,
		DiscoveredWorktrees: repository.DiscoveredWorktrees,
		IntegrationBranch:   integrationBranch,
		ManagedWorktreeRoot: worktreesRoot,
		BoundAt:             now.Format(time.RFC3339Nano),
	}
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

func ErrorCode(err error) string {
	var projectError *Error
	if errors.As(err, &projectError) {
		return projectError.Code
	}
	var repositoryError *gitrepo.Error
	if errors.As(err, &repositoryError) {
		return repositoryError.Code
	}
	var storeError *store.Error
	if errors.As(err, &storeError) {
		return storeError.Code
	}
	return "internal_error"
}
