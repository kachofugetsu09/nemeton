package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kachofugetsu09/nemeton/internal/artifact"
	"github.com/kachofugetsu09/nemeton/internal/event"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

func TestExpectedStreamVersionRejectsStaleCommand(t *testing.T) {
	root := t.TempDir()
	database, err := store.Open(context.Background(), filepath.Join(root, "nemeton.db"), filepath.Join(root, "backups"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer database.Close()
	artifacts := artifact.New(filepath.Join(root, "artifacts"))
	if err := makeDirectory(filepath.Join(root, "artifacts")); err != nil {
		t.Fatalf("create artifact store: %v", err)
	}
	record, err := artifacts.Put([]byte(`{"schema":"test"}`), "application/json")
	if err != nil {
		t.Fatalf("put Reality artifact: %v", err)
	}

	// 1. Register a real Project stream at version three.
	projectID := newID(t)
	correlationID := newID(t)
	now := time.Now().UTC()
	registered := newEnvelope(t, projectID, "project", projectID, event.ProjectRegistered, correlationID, event.ProjectRegisteredPayload{ProjectID: projectID, Status: "active", CreatedAt: now.Format(time.RFC3339Nano)}, now)
	bound := newEnvelope(t, projectID, "repository_binding", projectID, event.RepositoryBound, correlationID, event.RepositoryBindingPayload{ProjectID: projectID, DisplayPath: "/tmp/project", CanonicalRoot: "/tmp/project", GitCommonDir: "/tmp/project/.git", RemoteIdentities: []event.RemoteIdentity{}, DiscoveredWorktrees: []string{"/tmp/project"}, IntegrationBranch: "trunk", ManagedWorktreeRoot: "/tmp/worktrees", BoundAt: now.Format(time.RFC3339Nano)}, now)
	reality := realityEnvelope(t, projectID, correlationID, record, now)
	initial, err := database.Append(context.Background(), store.AppendRequest{ProjectID: projectID, ExpectedVersion: 0, Events: []event.Envelope{registered, bound, reality}})
	if err != nil {
		t.Fatalf("append initial stream: %v", err)
	}

	// 2. Advance once, then submit a second command against the stale version.
	advancedReality := realityEnvelope(t, projectID, newID(t), record, now.Add(time.Second))
	advanced, err := database.Append(context.Background(), store.AppendRequest{ProjectID: projectID, ExpectedVersion: initial.StreamVersion, Events: []event.Envelope{advancedReality}})
	if err != nil {
		t.Fatalf("advance Project stream: %v", err)
	}
	staleReality := realityEnvelope(t, projectID, newID(t), record, now.Add(2*time.Second))
	_, err = database.Append(context.Background(), store.AppendRequest{ProjectID: projectID, ExpectedVersion: initial.StreamVersion, Events: []event.Envelope{staleReality}})
	var storeError *store.Error
	if !errors.As(err, &storeError) || storeError.Code != "stream_conflict" {
		t.Fatalf("stale append error = %v, want stream_conflict", err)
	}

	// 3. A rejected stale command must leave stream and projection unchanged.
	current, err := database.Inspect(context.Background(), projectID)
	if err != nil {
		t.Fatalf("inspect Project after stale command: %v", err)
	}
	if current.StreamVersion != advanced.StreamVersion || current.ResultDigest != advanced.ResultDigest {
		t.Fatalf("stale command changed projection: current=%#v advanced=%#v", current, advanced)
	}
}

func realityEnvelope(t *testing.T, projectID, correlationID string, record artifact.Record, now time.Time) event.Envelope {
	t.Helper()
	realityID := newID(t)
	payload := event.RealityCapturedPayload{RealityID: realityID, ProjectID: projectID, IntegrationBranch: "trunk", CommitOID: "0123456789abcdef", TreeOID: "fedcba9876543210", ManifestDigest: record.Digest, Status: "current", CapturedAt: now.Format(time.RFC3339Nano)}
	item := newEnvelope(t, projectID, "reality", realityID, event.RealityCaptured, correlationID, payload, now)
	item.Artifacts = []event.ArtifactRef{{Digest: record.Digest, Relation: "reality_manifest", MediaType: record.MediaType, ByteSize: record.ByteSize, StorageURI: record.StorageURI}}
	return item
}

func newEnvelope(t *testing.T, projectID, aggregateType, aggregateID, eventType, correlationID string, payload any, now time.Time) event.Envelope {
	t.Helper()
	item, err := event.NewEnvelope(projectID, aggregateType, aggregateID, eventType, "semantic-gate", correlationID, payload, now)
	if err != nil {
		t.Fatalf("create %s event: %v", eventType, err)
	}
	return item
}

func newID(t *testing.T) string {
	t.Helper()
	value, err := event.NewID()
	if err != nil {
		t.Fatalf("generate ID: %v", err)
	}
	return value
}

func makeDirectory(path string) error {
	return os.MkdirAll(path, 0o700)
}
