package acceptance

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kachofugetsu09/nemeton/internal/api"
	"github.com/kachofugetsu09/nemeton/internal/artifact"
	"github.com/kachofugetsu09/nemeton/internal/realitybundle"
	"github.com/kachofugetsu09/nemeton/internal/realtime"
)

func TestMeetingDraftPersistsAcrossAPIWebSocketRestartAndReplay(t *testing.T) {
	root := t.TempDir()
	repository := newRepository(t, filepath.Join(root, "repository"), "main")
	harness := newHarness(t, shortDataDir(t))
	daemon := harness.start(t)
	t.Cleanup(func() {
		if daemon != nil {
			daemon.stop(t)
		}
	})
	opened := harness.open(t, repository, "main")

	trackedSentinel := "NEMETON_MEETING_TRACKED_SENTINEL"
	untrackedSentinel := "NEMETON_MEETING_UNTRACKED_SENTINEL"
	writeFile(t, filepath.Join(repository, "README.md"), []byte("# meeting\n"+trackedSentinel+"\n"))
	writeFile(t, filepath.Join(repository, "meeting-untracked.txt"), []byte(untrackedSentinel+"\n"))
	before := snapshotRepository(t, repository)

	client := api.NewClient(filepath.Join(harness.dataDir, "nemetond.sock"))
	created, err := client.CreateMeeting(context.Background(), opened.Project.Project.ID, api.CreateMeetingRequest{
		Kind: "change", Title: "Draft acceptance", Brief: "Freeze the exact dirty Reality"})
	if err != nil {
		t.Fatalf("create Meeting through API: %v", err)
	}
	if created.Meeting.Meeting.Status != "draft" || len(created.Meeting.Participants) != 5 {
		t.Fatalf("created Meeting = %#v", created.Meeting)
	}
	assertEqual(t, "repository after Meeting create", before, snapshotRepository(t, repository))
	assertBundleIncludes(t, filepath.Join(harness.dataDir, "artifacts"),
		created.Meeting.Meeting.RealityBundleDigest, trackedSentinel, untrackedSentinel)

	connection, err := client.WatchMeeting(context.Background(), created.Meeting.Meeting.ID, opened.Project.StreamVersion)
	if err != nil {
		t.Fatalf("connect Meeting WebSocket: %v", err)
	}
	connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	lastSequence := opened.Project.StreamVersion
	for index := 0; index < 6; index++ {
		var message realtime.Message
		if err := connection.ReadJSON(&message); err != nil {
			connection.Close()
			t.Fatalf("read Meeting WebSocket backlog %d: %v", index, err)
		}
		if message.Type != "domain_event" || message.Sequence <= lastSequence {
			connection.Close()
			t.Fatalf("Meeting WebSocket message = %#v after %d", message, lastSequence)
		}
		lastSequence = message.Sequence
	}
	connection.Close()

	digest := created.Meeting.CurrentStateDigest
	daemon.stop(t)
	daemon = nil
	daemon = harness.start(t)
	restarted, err := client.InspectMeeting(context.Background(), created.Meeting.Meeting.ID)
	if err != nil {
		t.Fatalf("inspect Meeting after restart: %v", err)
	}
	if restarted.Meeting.CurrentStateDigest != digest {
		t.Fatalf("restart Current State digest = %s, want %s", restarted.Meeting.CurrentStateDigest, digest)
	}
	if _, err := client.Replay(context.Background(), opened.Project.Project.ID); err != nil {
		t.Fatalf("replay Project with draft Meeting: %v", err)
	}
	replayed, err := client.InspectMeeting(context.Background(), created.Meeting.Meeting.ID)
	if err != nil {
		t.Fatalf("inspect Meeting after replay: %v", err)
	}
	if replayed.Meeting.CurrentStateDigest != digest {
		t.Fatalf("replay Current State digest = %s, want %s", replayed.Meeting.CurrentStateDigest, digest)
	}
	assertEqual(t, "repository after Meeting replay", before, snapshotRepository(t, repository))
	daemon.stop(t)
	daemon = nil
}

func assertBundleIncludes(t *testing.T, root, digest string, sentinels ...string) {
	t.Helper()
	data, err := artifact.New(root).Read(digest)
	if err != nil {
		t.Fatalf("read Reality Bundle: %v", err)
	}
	bundle, err := realitybundle.Decode(data)
	if err != nil {
		t.Fatalf("decode Reality Bundle: %v", err)
	}
	staged, err := base64.StdEncoding.DecodeString(bundle.StagedPatch)
	if err != nil {
		t.Fatalf("decode staged patch: %v", err)
	}
	unstaged, err := base64.StdEncoding.DecodeString(bundle.UnstagedPatch)
	if err != nil {
		t.Fatalf("decode unstaged patch: %v", err)
	}
	combined := string(staged) + string(unstaged)
	for _, file := range bundle.Untracked {
		content, err := base64.StdEncoding.DecodeString(file.DataBase64)
		if err != nil {
			t.Fatalf("decode untracked %s: %v", file.Path, err)
		}
		combined += string(content)
	}
	for _, sentinel := range sentinels {
		if !strings.Contains(combined, sentinel) {
			t.Fatalf("Reality Bundle does not contain sentinel %q", sentinel)
		}
	}
}
