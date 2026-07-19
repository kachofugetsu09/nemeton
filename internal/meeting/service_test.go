package meeting

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kachofugetsu09/nemeton/internal/artifact"
	"github.com/kachofugetsu09/nemeton/internal/event"
	"github.com/kachofugetsu09/nemeton/internal/project"
	"github.com/kachofugetsu09/nemeton/internal/runner"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

const providerProcessEnvironment = "NEMETON_PROGRAMMATIC_PROVIDER_PROCESS"

var programmaticCalls atomic.Int64

func TestProgrammaticProviderProcess(t *testing.T) {
	if os.Getenv(providerProcessEnvironment) != "1" {
		return
	}
	prompt := os.Getenv("NEMETON_PROGRAMMATIC_PROMPT")
	workdir := os.Getenv("NEMETON_PROGRAMMATIC_WORKDIR")
	if err := os.WriteFile(filepath.Join(workdir, "nemeton-provider-evidence.txt"), []byte("programmatic provider executed\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(31)
	}
	output := programmaticOutput(prompt)
	if _, err := os.Stdout.WriteString(output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(32)
	}
	os.Exit(0)
}

type programmaticBackend struct {
	provider string
}

func (b programmaticBackend) Execute(ctx context.Context, request runner.Request, emit func(runner.Delta)) (runner.Result, error) {
	programmaticCalls.Add(1)
	started := time.Now().UTC()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=TestProgrammaticProviderProcess")
	command.Dir = request.Workdir
	command.Env = append(os.Environ(), providerProcessEnvironment+"=1",
		"NEMETON_PROGRAMMATIC_PROMPT="+request.Prompt,
		"NEMETON_PROGRAMMATIC_WORKDIR="+request.Workdir)
	output, err := command.CombinedOutput()
	result := runner.Result{Provider: b.provider, Version: "programmatic-v1",
		Command:   []string{"programmatic-provider"},
		SessionID: request.SessionID, RawStream: append([]byte(nil), output...),
		StartedAt: started, FinishedAt: time.Now().UTC()}
	if result.SessionID == "" {
		result.SessionID = b.provider + "-session"
		if request.PinSession != nil {
			if err := request.PinSession(result.SessionID); err != nil {
				return result, err
			}
		}
	}
	if err != nil {
		return result, fmt.Errorf("programmatic Provider: %w: %s", err, output)
	}
	result.Output = strings.TrimSpace(string(output))
	emit(runner.Delta{Provider: b.provider, Type: "text", Content: result.Output})
	return result, nil
}

func programmaticOutput(prompt string) string {
	switch {
	case strings.Contains(prompt, "MALFORMED_PROVIDER_OUTPUT"):
		return "not-json"
	case strings.Contains(prompt, "You are the Recorder"):
		reference := regexp.MustCompile(`"id":"([0-9a-f-]+)"`).FindStringSubmatch(prompt)
		id := "missing"
		if len(reference) == 2 {
			id = reference[1]
		}
		data, _ := json.Marshal(map[string]any{"synthesis": "evidence-backed synthesis",
			"candidates": []map[string]any{{"kind": "decision", "statement": "adopt the evidence-backed design", "rationale": "all seats accepted", "source_refs": []string{id}}}})
		return string(data)
	case strings.Contains(prompt, "You are the Verifier"):
		return `{"clear":true,"blocking_findings":[]}`
	case strings.Contains(prompt, "bounded Nemeton deliberation"):
		if strings.Contains(prompt, "NO_CONSENSUS") && strings.Contains(prompt, "adversary") && !strings.Contains(prompt, "HUMAN_RESOLVED") {
			return `{"verdict":"reject","canonical_statement":"reject design","rationale":"counterexample","evidence":["experiment"]}`
		}
		return `{"verdict":"accept","canonical_statement":"adopt evidence-backed design","rationale":"verified","evidence":["experiment"]}`
	default:
		return `{"summary":"proposal","claims":["claim"],"evidence":["repository experiment"],"risks":[],"candidate_items":[{"kind":"decision","statement":"adopt design"}]}`
	}
}

func TestPersistentMeetingConvergesRatifiesAndReplays(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repository := createMeetingRepository(t, filepath.Join(root, "repository"))
	database, artifacts, projectService, service := meetingHarness(t, ctx, root)
	projectSnapshot, err := projectService.Open(ctx, repository, "main")
	if err != nil {
		t.Fatalf("open Project: %v", err)
	}
	meetingSnapshot, err := service.Create(ctx, CreateInput{ProjectID: projectSnapshot.Project.ID,
		Title: "Design a change", Brief: "Reach an evidence-backed design"})
	if err != nil {
		t.Fatalf("create Meeting: %v", err)
	}
	if len(meetingSnapshot.Participants) != 5 {
		t.Fatalf("participant count = %d, want 5", len(meetingSnapshot.Participants))
	}
	if _, err := service.Start(ctx, meetingSnapshot.Meeting.ID); err != nil {
		t.Fatalf("start Meeting: %v", err)
	}
	if err := service.Run(ctx, meetingSnapshot.Meeting.ID); err != nil {
		t.Fatalf("run Meeting: %v", err)
	}
	completed, err := service.Inspect(ctx, meetingSnapshot.Meeting.ID)
	if err != nil {
		t.Fatalf("inspect completed Meeting: %v", err)
	}
	if completed.Meeting.Status != "awaiting_human" || len(completed.Candidates) != 1 {
		t.Fatalf("completed Meeting = %#v", completed)
	}
	if completed.Meeting.CurrentRound != 1 {
		t.Fatalf("converged round = %d, want 1", completed.Meeting.CurrentRound)
	}
	for _, participant := range completed.Participants {
		if _, err := os.Stat(filepath.Join(participant.Workdir, "nemeton-provider-evidence.txt")); err != nil {
			t.Fatalf("participant %s did not execute in its Workdir: %v", participant.Seat, err)
		}
	}
	for _, run := range completed.Runs {
		if run.Status == "completed" && (run.ProviderVersion == "" || len(run.Command) == 0 || run.RawStreamDigest == "" || run.StderrDigest == "") {
			t.Fatalf("completed Run lacks execution evidence: %#v", run)
		}
	}
	beforeReplayDigest := completed.CurrentStateDigest
	beforeReplayCalls := programmaticCalls.Load()
	if _, err := database.Replay(ctx, projectSnapshot.Project.ID); err != nil {
		t.Fatalf("replay Project with Meeting: %v", err)
	}
	replayed, err := service.Inspect(ctx, meetingSnapshot.Meeting.ID)
	if err != nil {
		t.Fatalf("inspect replayed Meeting: %v", err)
	}
	if replayed.CurrentStateDigest != beforeReplayDigest {
		t.Fatalf("Current State digest after replay = %s, want %s", replayed.CurrentStateDigest, beforeReplayDigest)
	}
	if programmaticCalls.Load() != beforeReplayCalls {
		t.Fatal("replay invoked a Provider")
	}
	ratified, err := service.Disposition(ctx, meetingSnapshot.Meeting.ID, DispositionInput{
		CandidateID: replayed.Candidates[0].ID, Disposition: "selected"})
	if err != nil {
		t.Fatalf("ratify Candidate: %v", err)
	}
	if ratified.Meeting.Status != "concluded" || ratified.Candidates[0].Status != "selected" {
		t.Fatalf("ratified Meeting = %#v", ratified)
	}
	_ = artifacts
}

func TestPersistentMeetingStopsAfterThreeRounds(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repository := createMeetingRepository(t, filepath.Join(root, "repository"))
	_, _, projectService, service := meetingHarness(t, ctx, root)
	projectSnapshot, err := projectService.Open(ctx, repository, "main")
	if err != nil {
		t.Fatalf("open Project: %v", err)
	}
	snapshot, err := service.Create(ctx, CreateInput{ProjectID: projectSnapshot.Project.ID,
		Title: "Keep a real dissent", Brief: "NO_CONSENSUS must remain explicit"})
	if err != nil {
		t.Fatalf("create Meeting: %v", err)
	}
	if _, err := service.Start(ctx, snapshot.Meeting.ID); err != nil {
		t.Fatalf("start Meeting: %v", err)
	}
	if err := service.Run(ctx, snapshot.Meeting.ID); err != nil {
		t.Fatalf("run non-converging Meeting: %v", err)
	}
	stopped, err := service.Inspect(ctx, snapshot.Meeting.ID)
	if err != nil {
		t.Fatalf("inspect stopped Meeting: %v", err)
	}
	if stopped.Meeting.Status != "needs_user_input" || stopped.Meeting.CurrentRound != 3 || stopped.Meeting.HumanQuestion == "" {
		t.Fatalf("non-converging Meeting = %#v", stopped.Meeting)
	}
	resumed, err := service.Answer(ctx, snapshot.Meeting.ID, HumanInput{Content: "HUMAN_RESOLVED: use the shared canonical statement"})
	if err != nil {
		t.Fatalf("answer non-converging Meeting: %v", err)
	}
	if resumed.Meeting.Status != "deliberating" || resumed.Meeting.Cycle != 2 || resumed.Meeting.CurrentRound != 0 {
		t.Fatalf("resumed Meeting = %#v", resumed.Meeting)
	}
	if err := service.Run(ctx, snapshot.Meeting.ID); err != nil {
		t.Fatalf("run resumed Meeting: %v", err)
	}
	resolved, err := service.Inspect(ctx, snapshot.Meeting.ID)
	if err != nil {
		t.Fatalf("inspect resolved Meeting: %v", err)
	}
	if resolved.Meeting.Status != "awaiting_human" || resolved.Meeting.Cycle != 2 || resolved.Meeting.CurrentRound != 1 {
		t.Fatalf("resolved Meeting = %#v", resolved.Meeting)
	}
}

func TestMeetingRecoveryInterruptsAnUnfinishedRunBeforeRetry(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repository := createMeetingRepository(t, filepath.Join(root, "repository"))
	_, artifacts, projectService, service := meetingHarness(t, ctx, root)
	projectSnapshot, err := projectService.Open(ctx, repository, "main")
	if err != nil {
		t.Fatalf("open Project: %v", err)
	}
	snapshot, err := service.Create(ctx, CreateInput{ProjectID: projectSnapshot.Project.ID,
		Title: "Recover a run", Brief: "Persist the interrupted boundary"})
	if err != nil {
		t.Fatalf("create Meeting: %v", err)
	}
	snapshot, err = service.Start(ctx, snapshot.Meeting.ID)
	if err != nil {
		t.Fatalf("start Meeting: %v", err)
	}
	participant := snapshot.Participants[0]
	for _, candidate := range snapshot.Participants {
		if candidate.Role == "designer" {
			participant = candidate
			break
		}
	}
	inputRecord, err := artifacts.Put([]byte("interrupted input"), "text/plain")
	if err != nil {
		t.Fatalf("put interrupted input: %v", err)
	}
	runID, err := event.NewID()
	if err != nil {
		t.Fatalf("new interrupted Run ID: %v", err)
	}
	if err := service.recordRun(ctx, snapshot, participant, runID, "proposal", "running", "persisted-session",
		inputRecord, artifact.Record{}, runner.Result{}, artifact.Record{}, artifact.Record{},
		0, "", time.Now().UTC(), time.Time{}); err != nil {
		t.Fatalf("record unfinished Run: %v", err)
	}
	if err := service.Run(ctx, snapshot.Meeting.ID); err != nil {
		t.Fatalf("recover Meeting: %v", err)
	}
	recovered, err := service.Inspect(ctx, snapshot.Meeting.ID)
	if err != nil {
		t.Fatalf("inspect recovered Meeting: %v", err)
	}
	interrupted := 0
	completedProposals := 0
	resumedSession := ""
	for _, run := range recovered.Runs {
		if run.ID == runID && run.Status == "interrupted" {
			interrupted++
		}
		if run.ParticipantID == participant.ID && run.Phase == "proposal" && run.Status == "completed" {
			completedProposals++
			resumedSession = run.SessionID
		}
	}
	if interrupted != 1 || completedProposals != 1 || resumedSession != "persisted-session" {
		t.Fatalf("recovered Runs interrupted=%d completed proposals=%d session=%q", interrupted, completedProposals, resumedSession)
	}
}

func TestMeetingFailsWhenItsSourceRealityChanges(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repository := createMeetingRepository(t, filepath.Join(root, "repository"))
	_, _, projectService, service := meetingHarness(t, ctx, root)
	projectSnapshot, err := projectService.Open(ctx, repository, "main")
	if err != nil {
		t.Fatalf("open Project: %v", err)
	}
	snapshot, err := service.Create(ctx, CreateInput{ProjectID: projectSnapshot.Project.ID,
		Title: "Observe source", Brief: "Detect a stale source checkout"})
	if err != nil {
		t.Fatalf("create Meeting: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("# changed after Meeting create\n"), 0o600); err != nil {
		t.Fatalf("change Meeting source: %v", err)
	}
	if _, err := service.Start(ctx, snapshot.Meeting.ID); err != nil {
		t.Fatalf("start Meeting: %v", err)
	}
	if err := service.Run(ctx, snapshot.Meeting.ID); err == nil || !strings.Contains(err.Error(), "source repository changed") {
		t.Fatalf("source change Run error = %v", err)
	}
	failed, err := service.Inspect(ctx, snapshot.Meeting.ID)
	if err != nil {
		t.Fatalf("inspect failed Meeting: %v", err)
	}
	if failed.Meeting.Status != "failed" || !strings.Contains(failed.Meeting.HumanQuestion, "source repository changed") {
		t.Fatalf("source-changed Meeting = %#v", failed.Meeting)
	}
}

func TestMalformedProviderOutputFailsRunsWithEvidence(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repository := createMeetingRepository(t, filepath.Join(root, "repository"))
	_, _, projectService, service := meetingHarness(t, ctx, root)
	projectSnapshot, err := projectService.Open(ctx, repository, "main")
	if err != nil {
		t.Fatalf("open Project: %v", err)
	}
	snapshot, err := service.Create(ctx, CreateInput{ProjectID: projectSnapshot.Project.ID,
		Title: "Reject malformed output", Brief: "MALFORMED_PROVIDER_OUTPUT"})
	if err != nil {
		t.Fatalf("create Meeting: %v", err)
	}
	if _, err := service.Start(ctx, snapshot.Meeting.ID); err != nil {
		t.Fatalf("start Meeting: %v", err)
	}
	if err := service.Run(ctx, snapshot.Meeting.ID); err == nil || !strings.Contains(err.Error(), "does not contain a JSON object") {
		t.Fatalf("malformed Provider Run error = %v", err)
	}
	failed, err := service.Inspect(ctx, snapshot.Meeting.ID)
	if err != nil {
		t.Fatalf("inspect malformed Meeting: %v", err)
	}
	failedRuns := 0
	for _, run := range failed.Runs {
		if run.Status == "running" {
			t.Fatalf("malformed Run remained running: %#v", run)
		}
		if run.Status == "failed" {
			failedRuns++
			if run.RawStreamDigest == "" || run.StderrDigest == "" || run.Error == "" {
				t.Fatalf("malformed Run lacks failure evidence: %#v", run)
			}
		}
	}
	if failed.Meeting.Status != "failed" || failedRuns != 3 {
		t.Fatalf("malformed Meeting status=%s failed Runs=%d", failed.Meeting.Status, failedRuns)
	}
}

func meetingHarness(t *testing.T, ctx context.Context, root string) (*store.Store, artifact.Store, *project.Service, *Service) {
	t.Helper()
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(filepath.Join(data, "artifacts"), 0o700); err != nil {
		t.Fatalf("create artifact root: %v", err)
	}
	database, err := store.Open(ctx, filepath.Join(data, "nemeton.db"), filepath.Join(data, "backups"))
	if err != nil {
		t.Fatalf("open Store: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	artifacts := artifact.New(filepath.Join(data, "artifacts"))
	worktrees := filepath.Join(root, "worktrees")
	projectService := project.NewService(database, artifacts, worktrees)
	registry := runner.NewRegistry(map[string]runner.Backend{
		"codex":    programmaticBackend{provider: "codex"},
		"opencode": programmaticBackend{provider: "opencode"},
	})
	return database, artifacts, projectService, NewService(database, artifacts, registry, worktrees, nil)
}

func createMeetingRepository(t *testing.T, path string) string {
	t.Helper()
	runMeetingCommand(t, "", "git", "init", "-b", "main", path)
	runMeetingCommand(t, path, "git", "config", "user.name", "Nemeton Test")
	runMeetingCommand(t, path, "git", "config", "user.email", "nemeton@example.invalid")
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("# Test\n"), 0o600); err != nil {
		t.Fatalf("write repository: %v", err)
	}
	runMeetingCommand(t, path, "git", "add", "README.md")
	runMeetingCommand(t, path, "git", "commit", "-m", "base")
	return path
}

func runMeetingCommand(t *testing.T, directory, name string, args ...string) {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run %s %v: %v\n%s", name, args, err, output)
	}
}
