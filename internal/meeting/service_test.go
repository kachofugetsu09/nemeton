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
	case strings.Contains(prompt, "sole Recorder continuing"):
		references := regexp.MustCompile(`"id":"([0-9a-f-]+)"`).FindAllStringSubmatch(prompt, -1)
		id := "missing"
		if len(references) > 0 {
			id = references[0][1]
		}
		switch {
		case strings.Contains(prompt, "Protocol correction:"):
			return `{"action":"reconvene","response":"","opening":"Reconsider the rejected core boundary with the original roster.","synthesis":"","candidates":[]}`
		case strings.Contains(prompt, "POLICY_CORRECTION"):
			data, _ := json.Marshal(map[string]any{"action": "patch", "response": "", "opening": "", "synthesis": "Invalid local patch for a rejected boundary.", "candidates": []map[string]any{{"kind": "boundary", "statement": "The daemon owns committed sequence recovery.", "rationale": "This deliberately violates the reconvene policy.", "source_refs": []string{id}}}})
			return string(data)
		case strings.Contains(prompt, "CORE_REJECT"):
			return `{"action":"reconvene","response":"","opening":"Reconsider the rejected state ownership while preserving locked context.","synthesis":"","candidates":[]}`
		case strings.Contains(prompt, "LOCAL_PATCH"):
			data, _ := json.Marshal(map[string]any{"action": "patch", "response": "", "opening": "", "synthesis": "Complete patched design with explicit restart and replay acceptance.", "candidates": []map[string]any{{"kind": "invariant", "statement": "Committed replay never creates a new external side effect.", "rationale": "The review requested an explicit acceptance boundary.", "source_refs": []string{id}}, {"kind": "boundary", "statement": "The daemon owns committed sequence recovery.", "rationale": "The local clarification does not change ownership.", "source_refs": []string{id}}}})
			return string(data)
		case strings.Contains(prompt, "ANSWER_ONLY"):
			return `{"action":"answer","response":"The committed sequence is owned by the daemon projection; the browser only supplies its last observed sequence.","opening":"","synthesis":"","candidates":[]}`
		default:
			return `{"action":"reconvene","response":"","opening":"Revisit the unresolved core design with the original roster.","synthesis":"","candidates":[]}`
		}
	case strings.Contains(prompt, "sole Recorder for a Nemeton protocol v2"):
		references := regexp.MustCompile(`"id":"([0-9a-f-]+)"`).FindAllStringSubmatch(prompt, -1)
		id := "missing"
		if len(references) > 0 {
			id = references[0][1]
		}
		data, _ := json.Marshal(map[string]any{"synthesis": "Complete self-contained design owned by the daemon with deterministic replay and explicit acceptance.",
			"candidates": []map[string]any{
				{"kind": "invariant", "statement": "Committed meeting results are immutable.", "rationale": "The event stream owns revisions.", "source_refs": []string{id}},
				{"kind": "boundary", "statement": "The daemon owns recovery and failure propagation.", "rationale": "Clients consume projections.", "source_refs": []string{id}},
			}})
		return string(data)
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

func TestProtocolV2ReviewLoopPersistsContextAndReplaysWithoutProviders(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repository := createMeetingRepository(t, filepath.Join(root, "repository"))
	database, _, projectService, service := meetingHarness(t, ctx, root)
	projectSnapshot, err := projectService.Open(ctx, repository, "main")
	if err != nil {
		t.Fatalf("open Project: %v", err)
	}
	created, err := service.Create(ctx, CreateInput{ProjectID: projectSnapshot.Project.ID,
		Title: "Design durable recovery", Brief: "Design restart and replay ownership",
		Participants: []ParticipantInput{
			{Seat: "designer-a", Role: "designer", Provider: "codex", Model: "gpt-5.4-mini", ProviderOptions: map[string]string{"reasoning_effort": "medium"}},
			{Seat: "designer-b", Role: "designer", Provider: "opencode", Model: "opencode-go/deepseek-v4-pro", ProviderOptions: map[string]string{"variant": "high"}},
			{Seat: "recorder", Role: "recorder", Provider: "opencode", Model: "opencode-go/deepseek-v4-pro", ProviderOptions: map[string]string{"variant": "high"}},
		}})
	if err != nil {
		t.Fatalf("create protocol v2 Meeting: %v", err)
	}
	if created.Meeting.ProtocolVersion != 2 || len(created.Participants) != 3 {
		t.Fatalf("created protocol v2 Meeting = %#v", created)
	}
	if _, err := service.Start(ctx, created.Meeting.ID); err != nil {
		t.Fatalf("start protocol v2 Meeting: %v", err)
	}
	if err := service.Run(ctx, created.Meeting.ID); err != nil {
		t.Fatalf("run protocol v2 Meeting: %v", err)
	}
	current := inspectMeetingForTest(t, ctx, service, created.Meeting.ID)
	if current.Meeting.Status != "awaiting_user_review" || len(current.Candidates) != 2 || current.Meeting.ResultContentID == "" {
		t.Fatalf("initial protocol v2 Result = %#v", current)
	}
	initialSessions := participantSessions(current)

	invalidItems := make([]event.SemanticReviewItem, 0, len(current.Candidates))
	for index, candidate := range current.Candidates {
		if !contains(candidate.SourceRefs, current.Meeting.ResultContentID) {
			continue
		}
		designDisposition := "accepted"
		contextDisposition := "result_only"
		if index == 0 {
			designDisposition = "unreviewed"
			contextDisposition = "none"
		}
		invalidItems = append(invalidItems, event.SemanticReviewItem{CandidateID: candidate.ID,
			DesignDisposition: designDisposition, ContextDisposition: contextDisposition})
	}
	if _, err := service.Review(ctx, current.Meeting.ID, ReviewInput{
		ResultAction: "continue", Items: invalidItems}); err == nil {
		t.Fatal("review accepted internal unreviewed disposition from the Human boundary")
	}
	invalidItems[0] = event.SemanticReviewItem{CandidateID: invalidItems[0].CandidateID,
		DesignDisposition: "rejected", ContextDisposition: "none"}
	if _, err := service.Review(ctx, current.Meeting.ID, ReviewInput{
		ResultAction: "approve", Items: invalidItems}); err == nil {
		t.Fatal("complete Result approval accepted a rejected design item")
	}

	// Recorder answers a factual question without changing the Result.
	resultID := current.Meeting.ResultContentID
	current = reviewPersistFirstForTest(t, ctx, service, current, "ANSWER_ONLY: who owns the committed sequence?")
	if err := service.Run(ctx, current.Meeting.ID); err != nil {
		t.Fatalf("run Recorder answer: %v", err)
	}
	current = inspectMeetingForTest(t, ctx, service, current.Meeting.ID)
	if current.Meeting.Status != "awaiting_user_review" || current.Meeting.ResultContentID != resultID || !hasKind(current, "recorder_answer") {
		t.Fatalf("Recorder answer changed the Result: %#v", current.Meeting)
	}

	// Recorder makes a local full revision, then a core rejection forces a same-roster cycle.
	current = reviewPersistFirstForTest(t, ctx, service, current, "LOCAL_PATCH: make restart acceptance explicit.")
	if err := service.Run(ctx, current.Meeting.ID); err != nil {
		t.Fatalf("run Recorder patch: %v", err)
	}
	current = inspectMeetingForTest(t, ctx, service, current.Meeting.ID)
	if current.Meeting.ResultContentID == resultID || current.Meeting.Status != "awaiting_user_review" {
		t.Fatalf("Recorder patch did not create a Result revision: %#v", current.Meeting)
	}
	current = reviewMixedForTest(t, ctx, service, current, "POLICY_CORRECTION: reconsider the core recovery owner.")
	if !requiresSwarmReview(current) {
		t.Fatalf("structured core rejection was not recognized: %#v", current.Candidates)
	}
	if err := service.Run(ctx, current.Meeting.ID); err != nil {
		t.Fatalf("run Recorder reconvene decision: %v", err)
	}
	current = inspectMeetingForTest(t, ctx, service, current.Meeting.ID)
	if current.Meeting.Status != "reconvening" || current.Meeting.Cycle != 2 || !hasKind(current, "recorder_opening") {
		t.Fatalf("Recorder did not reconvene: meeting=%#v runs=%#v candidates=%#v", current.Meeting, current.Runs, current.Candidates)
	}
	failedCorrections := 0
	for _, run := range current.Runs {
		if run.Phase == "recorder_review" && run.Status == "failed" && strings.Contains(run.Error, "requires reconvene") {
			failedCorrections++
		}
	}
	if failedCorrections != 1 {
		t.Fatalf("bounded Recorder correction failures = %d, want 1", failedCorrections)
	}
	if err := service.Run(ctx, current.Meeting.ID); err != nil {
		t.Fatalf("run reconvened cycle: %v", err)
	}
	current = inspectMeetingForTest(t, ctx, service, current.Meeting.ID)
	if current.Meeting.Status != "awaiting_user_review" || current.Meeting.Cycle != 2 {
		t.Fatalf("reconvened Result = %#v", current.Meeting)
	}
	for id, session := range initialSessions {
		if participantSessions(current)[id] != session {
			t.Fatalf("participant %s session changed across cycles", id)
		}
	}

	// Whole-result approval persists selected context and yields a deterministic Handoff.
	current = reviewAllForTest(t, ctx, service, current, "approve", "persist", "")
	if current.Meeting.Status != "concluded" || current.Meeting.ApprovedResultDigest == "" {
		t.Fatalf("approved Meeting = %#v", current.Meeting)
	}
	handoff, err := service.Handoff(ctx, current.Meeting.ID)
	if err != nil || len(handoff.ApprovedContext) < 3 {
		t.Fatalf("Handoff approved context = %d, error %v", len(handoff.ApprovedContext), err)
	}
	markdown := handoff.Markdown()
	for _, required := range []string{handoff.Schema, handoff.MeetingID, handoff.ResultDigest,
		current.Meeting.Brief, handoff.Result, handoff.ApprovedContext[0].Statement} {
		if !strings.Contains(markdown, required) {
			t.Fatalf("Markdown Handoff omits %q", required)
		}
	}
	beforeDigest := current.CurrentStateDigest
	beforeCalls := programmaticCalls.Load()
	if _, err := database.Replay(ctx, projectSnapshot.Project.ID); err != nil {
		t.Fatalf("replay protocol v2 Project: %v", err)
	}
	replayed := inspectMeetingForTest(t, ctx, service, current.Meeting.ID)
	if replayed.CurrentStateDigest != beforeDigest || programmaticCalls.Load() != beforeCalls {
		t.Fatalf("replay digest/calls changed: digest=%s calls=%d", replayed.CurrentStateDigest, programmaticCalls.Load()-beforeCalls)
	}
}

func inspectMeetingForTest(t *testing.T, ctx context.Context, service *Service, meetingID string) store.MeetingSnapshot {
	t.Helper()
	snapshot, err := service.Inspect(ctx, meetingID)
	if err != nil {
		t.Fatalf("inspect Meeting: %v", err)
	}
	return snapshot
}

func reviewAllForTest(t *testing.T, ctx context.Context, service *Service, snapshot store.MeetingSnapshot,
	action, contextDisposition, comment string) store.MeetingSnapshot {
	t.Helper()
	items := make([]event.SemanticReviewItem, 0)
	for _, candidate := range snapshot.Candidates {
		if contains(candidate.SourceRefs, snapshot.Meeting.ResultContentID) {
			items = append(items, event.SemanticReviewItem{CandidateID: candidate.ID,
				DesignDisposition: "accepted", ContextDisposition: contextDisposition})
		}
	}
	result, err := service.Review(ctx, snapshot.Meeting.ID, ReviewInput{ResultAction: action, Items: items, Comment: comment})
	if err != nil {
		t.Fatalf("review all Result items: %v", err)
	}
	return result
}

func reviewMixedForTest(t *testing.T, ctx context.Context, service *Service,
	snapshot store.MeetingSnapshot, comment string) store.MeetingSnapshot {
	t.Helper()
	items := make([]event.SemanticReviewItem, 0)
	index := 0
	for _, candidate := range snapshot.Candidates {
		if !contains(candidate.SourceRefs, snapshot.Meeting.ResultContentID) {
			continue
		}
		item := event.SemanticReviewItem{CandidateID: candidate.ID, DesignDisposition: "rejected", ContextDisposition: "none"}
		if index == 0 {
			item.DesignDisposition = "accepted"
			item.ContextDisposition = "persist"
		}
		items = append(items, item)
		index++
	}
	result, err := service.Review(ctx, snapshot.Meeting.ID, ReviewInput{ResultAction: "continue", Items: items, Comment: comment})
	if err != nil {
		t.Fatalf("review mixed Result items: %v", err)
	}
	return result
}

func reviewPersistFirstForTest(t *testing.T, ctx context.Context, service *Service,
	snapshot store.MeetingSnapshot, comment string) store.MeetingSnapshot {
	t.Helper()
	items := make([]event.SemanticReviewItem, 0)
	index := 0
	for _, candidate := range snapshot.Candidates {
		if !contains(candidate.SourceRefs, snapshot.Meeting.ResultContentID) {
			continue
		}
		contextDisposition := "result_only"
		if index == 0 {
			contextDisposition = "persist"
		}
		items = append(items, event.SemanticReviewItem{CandidateID: candidate.ID,
			DesignDisposition: "accepted", ContextDisposition: contextDisposition})
		index++
	}
	result, err := service.Review(ctx, snapshot.Meeting.ID, ReviewInput{
		ResultAction: "continue", Items: items, Comment: comment})
	if err != nil {
		t.Fatalf("review with locked first item: %v", err)
	}
	return result
}

func participantSessions(snapshot store.MeetingSnapshot) map[string]string {
	result := make(map[string]string, len(snapshot.Participants))
	for _, participant := range snapshot.Participants {
		result[participant.ID] = participant.SessionID
	}
	return result
}

func hasKind(snapshot store.MeetingSnapshot, kind string) bool {
	for _, content := range snapshot.Contents {
		if content.Kind == kind {
			return true
		}
	}
	return false
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
