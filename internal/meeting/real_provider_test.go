//go:build realproviders

package meeting

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kachofugetsu09/nemeton/internal/artifact"
	"github.com/kachofugetsu09/nemeton/internal/event"
	"github.com/kachofugetsu09/nemeton/internal/project"
	"github.com/kachofugetsu09/nemeton/internal/runner"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

func TestRealProviderCertification(t *testing.T) {
	if os.Getenv("NEMETON_REAL_PROVIDER_CERTIFICATION") != "1" {
		t.Skip("set NEMETON_REAL_PROVIDER_CERTIFICATION=1 for delivery-only real calls")
	}
	ctx := context.Background()
	root := t.TempDir()
	repository := createMeetingRepository(t, filepath.Join(root, "fixture-repository"))
	_, _, projects, service := realMeetingHarness(t, ctx, root)
	projectSnapshot, err := projects.Open(ctx, repository, "main")
	if err != nil {
		t.Fatalf("open fixture Project: %v", err)
	}

	t.Run("Codex direct approval", func(t *testing.T) {
		snapshot := realRecorderMeeting(t, ctx, service, projectSnapshot.Project.ID,
			"codex", "gpt-5.4-mini", map[string]string{"reasoning_effort": "low"},
			"设计一个提交后不可变、replay 不产生新副作用的本地事件恢复边界。")
		approved := reviewAllForTest(t, ctx, service, snapshot, "approve", "persist", "")
		if approved.Meeting.Status != "concluded" {
			t.Fatalf("Codex direct approval status = %s", approved.Meeting.Status)
		}
		if _, err := service.Handoff(ctx, approved.Meeting.ID); err != nil {
			t.Fatalf("read Codex Handoff: %v", err)
		}
		logProviderEvidence(t, approved)
	})

	t.Run("OpenCode answer patch and reconvene", func(t *testing.T) {
		snapshot := realRecorderMeeting(t, ctx, service, projectSnapshot.Project.ID,
			"opencode", "opencode-go/deepseek-v4-pro", map[string]string{"variant": "high"},
			"设计 daemon、WebSocket 客户端和持久化事件之间的状态所有权、断线恢复与失败传播。")
		resultID := snapshot.Meeting.ResultContentID

		snapshot = reviewAllForTest(t, ctx, service, snapshot, "continue", "result_only",
			"我有一个疑问：浏览器重连时，最后 committed sequence 到底由谁拥有，为什么不会和 daemon 的 projection 分叉？")
		if err := service.Run(ctx, snapshot.Meeting.ID); err != nil {
			t.Fatalf("run real Recorder answer: %v", err)
		}
		snapshot = inspectMeetingForTest(t, ctx, service, snapshot.Meeting.ID)
		if !hasKind(snapshot, "recorder_answer") || snapshot.Meeting.ResultContentID != resultID {
			t.Fatalf("real Recorder did not choose answer")
		}

		snapshot = reviewAllForTest(t, ctx, service, snapshot, "continue", "result_only",
			"方案方向不变，但验收标准缺少 daemon restart 后不重复 committed event，以及客户端从最后 committed sequence 恢复这两条。")
		if err := service.Run(ctx, snapshot.Meeting.ID); err != nil {
			t.Fatalf("run real Recorder patch: %v", err)
		}
		snapshot = inspectMeetingForTest(t, ctx, service, snapshot.Meeting.ID)
		if snapshot.Meeting.ResultContentID == resultID {
			t.Fatalf("real Recorder did not create a full patch revision")
		}

		items := make([]event.SemanticReviewItem, 0)
		index := 0
		for _, candidate := range snapshot.Candidates {
			if !contains(candidate.SourceRefs, snapshot.Meeting.ResultContentID) {
				continue
			}
			item := event.SemanticReviewItem{CandidateID: candidate.ID,
				DesignDisposition: "accepted", ContextDisposition: "result_only"}
			if index == 0 {
				item.ContextDisposition = "persist"
			} else if index == 1 {
				item.DesignDisposition = "rejected"
				item.ContextDisposition = "none"
			}
			items = append(items, item)
			index++
		}
		if len(items) < 2 {
			t.Fatalf("real patched Result produced %d candidates, need at least 2", len(items))
		}
		snapshot, err = service.Review(ctx, snapshot.Meeting.ID, ReviewInput{
			ResultAction: "continue", Items: items,
			Comment: "客户端状态所有权的处理仍然不成立，尤其是断线后的唯一事实来源和失败传播边界。"})
		if err != nil {
			t.Fatalf("persist real partial review: %v", err)
		}
		if err := service.Run(ctx, snapshot.Meeting.ID); err != nil {
			t.Fatalf("run real Recorder core decision: %v", err)
		}
		snapshot = inspectMeetingForTest(t, ctx, service, snapshot.Meeting.ID)
		if snapshot.Meeting.Status != "reconvening" || snapshot.Meeting.Cycle != 2 {
			t.Fatalf("real Recorder did not reconvene: %#v", snapshot.Meeting)
		}
		logProviderEvidence(t, snapshot)
	})
}

func realMeetingHarness(t *testing.T, ctx context.Context, root string) (*store.Store, artifact.Store, *project.Service, *Service) {
	t.Helper()
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(filepath.Join(data, "artifacts"), 0o700); err != nil {
		t.Fatalf("create real certification artifact root: %v", err)
	}
	database, err := store.Open(ctx, filepath.Join(data, "nemeton.db"), filepath.Join(data, "backups"))
	if err != nil {
		t.Fatalf("open real certification Store: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	artifacts := artifact.New(filepath.Join(data, "artifacts"))
	worktrees := filepath.Join(root, "worktrees")
	projects := project.NewService(database, artifacts, worktrees)
	return database, artifacts, projects, NewService(database, artifacts, runner.Production(), worktrees, nil)
}

func realRecorderMeeting(t *testing.T, ctx context.Context, service *Service, projectID,
	provider, model string, options map[string]string, brief string) store.MeetingSnapshot {
	t.Helper()
	created, err := service.Create(ctx, CreateInput{ProjectID: projectID, Title: "Real Provider certification",
		Brief: brief, Participants: []ParticipantInput{{Seat: "recorder", Role: "recorder",
			Provider: provider, Model: model, ProviderOptions: options}}})
	if err != nil {
		t.Fatalf("create real Meeting: %v", err)
	}
	if _, err := service.Start(ctx, created.Meeting.ID); err != nil {
		t.Fatalf("start real Meeting: %v", err)
	}
	if err := service.Run(ctx, created.Meeting.ID); err != nil {
		t.Fatalf("run real Meeting: %v", err)
	}
	snapshot := inspectMeetingForTest(t, ctx, service, created.Meeting.ID)
	if snapshot.Meeting.Status != "awaiting_user_review" || len(snapshot.Candidates) < 2 {
		t.Fatalf("real Result is incomplete: status=%s candidates=%d", snapshot.Meeting.Status, len(snapshot.Candidates))
	}
	return snapshot
}

func logProviderEvidence(t *testing.T, snapshot store.MeetingSnapshot) {
	t.Helper()
	for _, run := range snapshot.Runs {
		if run.Status == "completed" {
			t.Logf("provider=%s version=%q phase=%s command=%v output=%s",
				run.Provider, run.ProviderVersion, run.Phase, run.Command, run.OutputDigest)
		}
	}
}
