package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/kachofugetsu09/nemeton/internal/artifact"
	"github.com/kachofugetsu09/nemeton/internal/event"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

func TestMeetingEventProjectionTransactionRollsBackTogether(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	databasePath := filepath.Join(root, "nemeton.db")
	database, err := store.Open(ctx, databasePath, filepath.Join(root, "backups"))
	if err != nil {
		t.Fatalf("open Store: %v", err)
	}
	defer database.Close()
	artifacts := artifact.New(filepath.Join(root, "artifacts"))
	record, err := artifacts.Put([]byte(`{"schema":"reality"}`), "application/json")
	if err != nil {
		t.Fatalf("put Reality artifact: %v", err)
	}
	projectID := transactionID(t)
	realityID := transactionID(t)
	correlationID := transactionID(t)
	now := time.Now().UTC()
	registered := transactionEnvelope(t, projectID, "project", projectID, event.ProjectRegistered,
		correlationID, event.ProjectRegisteredPayload{ProjectID: projectID, Status: "active", CreatedAt: now.Format(time.RFC3339Nano)}, now)
	bound := transactionEnvelope(t, projectID, "repository_binding", projectID, event.RepositoryBound,
		correlationID, event.RepositoryBindingPayload{ProjectID: projectID, DisplayPath: "/tmp/project",
			CanonicalRoot: "/tmp/project", GitCommonDir: "/tmp/project/.git",
			RemoteIdentities: []event.RemoteIdentity{}, DiscoveredWorktrees: []string{"/tmp/project"},
			IntegrationBranch: "main", ManagedWorktreeRoot: "/tmp/worktrees", BoundAt: now.Format(time.RFC3339Nano)}, now)
	reality := transactionEnvelope(t, projectID, "reality", realityID, event.RealityCaptured,
		correlationID, event.RealityCapturedPayload{RealityID: realityID, ProjectID: projectID,
			IntegrationBranch: "main", CommitOID: "commit", TreeOID: "tree",
			ManifestDigest: record.Digest, Status: "current", CapturedAt: now.Format(time.RFC3339Nano)}, now)
	reality.Artifacts = []event.ArtifactRef{{Digest: record.Digest, Relation: "reality_manifest",
		MediaType: record.MediaType, ByteSize: record.ByteSize, StorageURI: record.StorageURI}}
	initial, err := database.Append(ctx, store.AppendRequest{ProjectID: projectID,
		Events: []event.Envelope{registered, bound, reality}})
	if err != nil {
		t.Fatalf("append Project: %v", err)
	}
	initialState, err := database.CurrentState(ctx, projectID)
	if err != nil {
		t.Fatalf("read initial Current State: %v", err)
	}

	raw, err := sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatalf("open fault connection: %v", err)
	}
	if _, err := raw.Exec(`
        CREATE TRIGGER force_participant_projection_failure
        BEFORE INSERT ON meeting_participants
        BEGIN
            SELECT RAISE(ABORT, 'forced participant failure');
        END`); err != nil {
		raw.Close()
		t.Fatalf("create failure trigger: %v", err)
	}
	defer raw.Close()

	meetingID := transactionID(t)
	participantID := transactionID(t)
	bundle, err := artifacts.Put([]byte(`{"schema":"bundle"}`), "application/json")
	if err != nil {
		t.Fatalf("put Bundle artifact: %v", err)
	}
	created := transactionEnvelope(t, projectID, "meeting", meetingID, event.MeetingCreated,
		correlationID, event.MeetingCreatedPayload{MeetingID: meetingID, ProjectID: projectID,
			RealityID: realityID, Kind: "change", Title: "atomic", Brief: "atomic",
			Status: "draft", MaxRounds: 3, Cycle: 1, RealityBundleDigest: bundle.Digest,
			CreatedAt: now.Format(time.RFC3339Nano)}, now)
	created.Artifacts = []event.ArtifactRef{{Digest: bundle.Digest, Relation: "reality_bundle",
		MediaType: bundle.MediaType, ByteSize: bundle.ByteSize, StorageURI: bundle.StorageURI}}
	participant := transactionEnvelope(t, projectID, "meeting_participant", participantID,
		event.ParticipantAdded, correlationID, event.MeetingParticipantAddedPayload{
			MeetingID: meetingID, ParticipantID: participantID, Seat: "designer",
			Role: "designer", Provider: "codex", Status: "pending",
			Workdir: "/tmp/worktree", AddedAt: now.Format(time.RFC3339Nano)}, now)
	if _, err := database.Append(ctx, store.AppendRequest{ProjectID: projectID,
		ExpectedVersion: initial.StreamVersion, Events: []event.Envelope{created, participant}}); err == nil {
		t.Fatal("faulted Meeting append unexpectedly succeeded")
	}
	current, err := database.Inspect(ctx, projectID)
	if err != nil {
		t.Fatalf("inspect Project after rollback: %v", err)
	}
	if current.StreamVersion != initial.StreamVersion || current.ResultDigest != initial.ResultDigest {
		t.Fatalf("Meeting rollback changed Project: current=%#v initial=%#v", current, initial)
	}
	currentState, err := database.CurrentState(ctx, projectID)
	if err != nil {
		t.Fatalf("read Current State after rollback: %v", err)
	}
	if currentState.ProjectedThroughSequence != initialState.ProjectedThroughSequence || currentState.ResultDigest != initialState.ResultDigest {
		t.Fatalf("Meeting rollback changed Current State: current=%#v initial=%#v", currentState, initialState)
	}
	if _, err := database.InspectMeeting(ctx, meetingID); err == nil {
		t.Fatal("rolled-back Meeting projection exists")
	}
}

func transactionID(t *testing.T) string {
	t.Helper()
	id, err := event.NewID()
	if err != nil {
		t.Fatalf("new ID: %v", err)
	}
	return id
}

func transactionEnvelope(t *testing.T, projectID, aggregateType, aggregateID, eventType, correlationID string, payload any, now time.Time) event.Envelope {
	t.Helper()
	item, err := event.NewEnvelope(projectID, aggregateType, aggregateID, eventType,
		"semantic-gate", correlationID, payload, now)
	if err != nil {
		t.Fatalf("new %s event: %v", eventType, err)
	}
	return item
}
