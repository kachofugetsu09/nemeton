package meeting

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kachofugetsu09/nemeton/internal/artifact"
	"github.com/kachofugetsu09/nemeton/internal/event"
	"github.com/kachofugetsu09/nemeton/internal/realitybundle"
	"github.com/kachofugetsu09/nemeton/internal/runner"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

const (
	defaultMaxRounds  = 3
	defaultRunTimeout = 30 * time.Minute
)

type Publisher interface {
	PublishEvent(string, store.StreamEvent)
	PublishDelta(string, string, runner.Delta)
}

type Scheduler interface {
	Enqueue(string)
}

type CreateInput struct {
	ProjectID    string
	Kind         string
	Title        string
	Brief        string
	Providers    map[string]string
	Participants []ParticipantInput
}

type ParticipantInput struct {
	Seat            string
	Role            string
	Provider        string
	Model           string
	ProviderOptions map[string]string
}

type HumanInput struct {
	Content string
}

type DispositionInput struct {
	CandidateID string
	Disposition string
	Reason      string
}

type ReviewInput struct {
	ResultAction string
	Items        []event.SemanticReviewItem
	Comment      string
}

type Content struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	MediaType string `json:"media_type"`
	Body      string `json:"body"`
}

type Handoff struct {
	Schema          string                    `json:"schema"`
	MeetingID       string                    `json:"meeting_id"`
	Title           string                    `json:"title"`
	OriginalBrief   string                    `json:"original_brief"`
	ResultContentID string                    `json:"result_content_id"`
	ResultDigest    string                    `json:"result_digest"`
	Result          string                    `json:"result"`
	ApprovedContext []store.SemanticCandidate `json:"approved_context"`
}

// Markdown renders a self-contained handoff without writing the target repository.
func (h Handoff) Markdown() string {
	// 1. Preserve the approved Result exactly as recorded.
	var document strings.Builder
	fmt.Fprintf(&document, "---\nschema: %s\nmeeting_id: %s\nresult_content_id: %s\nresult_digest: %s\n---\n\n", h.Schema, h.MeetingID, h.ResultContentID, h.ResultDigest)
	fmt.Fprintf(&document, "# %s\n\n## Original request\n\n%s\n\n## Approved design Result\n\n%s\n", h.Title, h.OriginalBrief, strings.TrimSpace(h.Result))

	// 2. Append the Human-selected context that future work must preserve.
	document.WriteString("\n\n## Approved project context\n")
	if len(h.ApprovedContext) == 0 {
		document.WriteString("\nNo additional project context was approved.\n")
		return document.String()
	}
	for _, item := range h.ApprovedContext {
		fmt.Fprintf(&document, "\n- **%s:** %s\n", item.Kind, item.Statement)
		if item.Rationale != "" {
			fmt.Fprintf(&document, "  - Rationale: %s\n", item.Rationale)
		}
		if len(item.SourceRefs) > 0 {
			fmt.Fprintf(&document, "  - Sources: `%s`\n", strings.Join(item.SourceRefs, "`, `"))
		}
	}
	return document.String()
}

type Service struct {
	store         *store.Store
	artifacts     artifact.Store
	runners       *runner.Registry
	worktreesRoot string
	publisher     Publisher
	schedulerMu   sync.RWMutex
	scheduler     Scheduler
	runRecordMu   sync.Mutex
}

func NewService(database *store.Store, artifacts artifact.Store, runners *runner.Registry, worktreesRoot string, publisher Publisher) *Service {
	return &Service{store: database, artifacts: artifacts, runners: runners,
		worktreesRoot: worktreesRoot, publisher: publisher}
}

func (s *Service) SetScheduler(scheduler Scheduler) {
	s.schedulerMu.Lock()
	defer s.schedulerMu.Unlock()
	s.scheduler = scheduler
}

func (s *Service) Create(ctx context.Context, input CreateInput) (store.MeetingSnapshot, error) {
	// 1. Validate the human command and freeze its exact repository Reality.
	if !event.ValidID(input.ProjectID) {
		return store.MeetingSnapshot{}, newError("invalid_argument", "invalid Project ID")
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Brief = strings.TrimSpace(input.Brief)
	if input.Title == "" || input.Brief == "" {
		return store.MeetingSnapshot{}, newError("invalid_argument", "Meeting title and brief are required")
	}
	if input.Kind == "" {
		input.Kind = "change"
	}
	if input.Kind != "change" && input.Kind != "project" {
		return store.MeetingSnapshot{}, newError("invalid_argument", "Meeting kind must be change or project")
	}
	protocolVersion := 1
	if len(input.Participants) > 0 {
		protocolVersion = 2
		if err := validateParticipants(input.Participants); err != nil {
			return store.MeetingSnapshot{}, err
		}
	} else if err := validateProviderOverrides(input.Providers); err != nil {
		return store.MeetingSnapshot{}, err
	}
	project, err := s.store.Inspect(ctx, input.ProjectID)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	bundle, bundleData, err := realitybundle.Capture(ctx, project.Binding.CanonicalRoot)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	bundleRecord, err := s.artifacts.Put(bundleData, "application/vnd.nemeton.reality-bundle+json;version=1")
	if err != nil {
		return store.MeetingSnapshot{}, err
	}

	// 2. Create every durable seat in one atomic Project-stream transaction.
	meetingID, err := event.NewID()
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	correlationID, err := event.NewID()
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	now := time.Now().UTC()
	createdPayload := event.MeetingCreatedPayload{
		MeetingID: meetingID, ProjectID: input.ProjectID,
		RealityID: project.CurrentReality.ID, Kind: input.Kind, Title: input.Title,
		Brief: input.Brief, Status: "draft", MaxRounds: defaultMaxRounds,
		Cycle: 1, CurrentRound: 0, RealityBundleDigest: bundleRecord.Digest,
		CreatedAt: now.Format(time.RFC3339Nano),
	}
	createdType := event.MeetingCreated
	var createdBody any = createdPayload
	if protocolVersion == 2 {
		createdType = event.MeetingCreatedV2
		createdBody = event.MeetingCreatedV2Payload{MeetingCreatedPayload: createdPayload, ProtocolVersion: 2}
	}
	created, err := event.NewEnvelope(input.ProjectID, "meeting", meetingID,
		createdType, "human", correlationID, createdBody, now)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	created.Artifacts = []event.ArtifactRef{artifactRef(bundleRecord, "reality_bundle")}
	events := []event.Envelope{created}
	definitions := participantDefinitions(input.Providers)
	if protocolVersion == 2 {
		definitions = make([]participantDefinition, 0, len(input.Participants))
		briefRecord, err := s.artifacts.Put([]byte(input.Brief), "text/plain;charset=utf-8")
		if err != nil {
			return store.MeetingSnapshot{}, err
		}
		contentID, err := event.NewID()
		if err != nil {
			return store.MeetingSnapshot{}, err
		}
		briefContent, err := event.NewEnvelope(input.ProjectID, "meeting_content", contentID,
			event.MeetingContentAdded, "human", correlationID, event.MeetingContentAddedPayload{
				MeetingID: meetingID, ContentID: contentID, Kind: "human_input", Cycle: 1,
				Round: 0, ContentDigest: briefRecord.Digest, CreatedAt: now.Format(time.RFC3339Nano)}, now)
		if err != nil {
			return store.MeetingSnapshot{}, err
		}
		briefContent.Artifacts = []event.ArtifactRef{artifactRef(briefRecord, "meeting_content")}
		events = append(events, briefContent)
		for _, participant := range input.Participants {
			definitions = append(definitions, participantDefinition{Seat: participant.Seat,
				Role: participant.Role, Provider: participant.Provider, Model: participant.Model,
				ProviderOptions: participant.ProviderOptions})
		}
	}
	for _, definition := range definitions {
		participantID, err := event.NewID()
		if err != nil {
			return store.MeetingSnapshot{}, err
		}
		root := filepath.Join(s.worktreesRoot, "meetings", meetingID, participantID)
		workdir := filepath.Join(root, "workdir", filepath.Base(bundle.SourceRoot))
		participantPayload := event.MeetingParticipantAddedPayload{MeetingID: meetingID,
			ParticipantID: participantID, Seat: definition.Seat, Role: definition.Role,
			Provider: definition.Provider, Model: definition.Model, Status: "pending",
			Workdir: workdir, AddedAt: now.Format(time.RFC3339Nano)}
		participantType := event.ParticipantAdded
		var participantBody any = participantPayload
		if protocolVersion == 2 {
			participantType = event.ParticipantAddedV2
			participantBody = event.MeetingParticipantAddedV2Payload{
				MeetingParticipantAddedPayload: participantPayload,
				ProviderOptions:                definition.ProviderOptions,
			}
		}
		item, err := event.NewEnvelope(input.ProjectID, "meeting_participant", participantID,
			participantType, "facilitator", correlationID, participantBody, now)
		if err != nil {
			return store.MeetingSnapshot{}, err
		}
		events = append(events, item)
	}
	if err := s.append(ctx, input.ProjectID, project.StreamVersion, meetingID, events); err != nil {
		return store.MeetingSnapshot{}, err
	}
	return s.store.InspectMeeting(ctx, meetingID)
}

func (s *Service) Inspect(ctx context.Context, meetingID string) (store.MeetingSnapshot, error) {
	if !event.ValidID(meetingID) {
		return store.MeetingSnapshot{}, newError("invalid_argument", "invalid Meeting ID")
	}
	return s.store.InspectMeeting(ctx, meetingID)
}

func (s *Service) Start(ctx context.Context, meetingID string) (store.MeetingSnapshot, error) {
	snapshot, err := s.Inspect(ctx, meetingID)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	if snapshot.Meeting.Status != "draft" {
		return store.MeetingSnapshot{}, newError("invalid_meeting_state", fmt.Sprintf("Meeting %s is %s, want draft", meetingID, snapshot.Meeting.Status))
	}
	if err := s.setStatus(ctx, snapshot, "preparing", 0, "", ""); err != nil {
		return store.MeetingSnapshot{}, err
	}
	s.enqueue(meetingID)
	return s.store.InspectMeeting(ctx, meetingID)
}

func (s *Service) Answer(ctx context.Context, meetingID string, input HumanInput) (store.MeetingSnapshot, error) {
	snapshot, err := s.Inspect(ctx, meetingID)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	input.Content = strings.TrimSpace(input.Content)
	if snapshot.Meeting.Status != "needs_user_input" || input.Content == "" {
		return store.MeetingSnapshot{}, newError("invalid_meeting_state", "Meeting must need a non-empty Human input")
	}
	record, err := s.artifacts.Put([]byte(input.Content), "text/plain;charset=utf-8")
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	now := time.Now().UTC()
	contentID, err := event.NewID()
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	correlationID, err := event.NewID()
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	content, err := event.NewEnvelope(snapshot.Meeting.ProjectID, "meeting_content", contentID,
		event.MeetingContentAdded, "human", correlationID, event.MeetingContentAddedPayload{
			MeetingID: meetingID, ContentID: contentID, Kind: "human_input",
			Cycle: snapshot.Meeting.Cycle + 1, Round: 0, ContentDigest: record.Digest,
			CreatedAt: now.Format(time.RFC3339Nano)}, now)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	content.Artifacts = []event.ArtifactRef{artifactRef(record, "meeting_content")}
	status, err := statusEvent(snapshot.Meeting.ProjectID, meetingID, "deliberating",
		snapshot.Meeting.Cycle+1, 0, "", "", "human", correlationID, now)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	if err := s.append(ctx, snapshot.Meeting.ProjectID, snapshot.StreamVersion, meetingID, []event.Envelope{content, status}); err != nil {
		return store.MeetingSnapshot{}, err
	}
	s.enqueue(meetingID)
	return s.store.InspectMeeting(ctx, meetingID)
}

func (s *Service) Disposition(ctx context.Context, meetingID string, input DispositionInput) (store.MeetingSnapshot, error) {
	snapshot, err := s.Inspect(ctx, meetingID)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	if snapshot.Meeting.Status != "awaiting_human" {
		return store.MeetingSnapshot{}, newError("invalid_meeting_state", "Meeting is not awaiting Human ratification")
	}
	if input.Disposition != "selected" && input.Disposition != "rejected" && input.Disposition != "deferred" {
		return store.MeetingSnapshot{}, newError("invalid_argument", "disposition must be selected, rejected, or deferred")
	}
	var candidateFound bool
	for _, candidate := range snapshot.Candidates {
		if candidate.ID == input.CandidateID && candidate.Status == "proposed" {
			candidateFound = true
			break
		}
	}
	if !candidateFound {
		return store.MeetingSnapshot{}, newError("invalid_argument", "Candidate is not pending in this Meeting")
	}
	now := time.Now().UTC()
	correlationID, err := event.NewID()
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	item, err := event.NewEnvelope(snapshot.Meeting.ProjectID, "semantic_candidate",
		input.CandidateID, event.SemanticCandidateDispositioned, "human", correlationID,
		event.SemanticCandidateDispositionedPayload{MeetingID: meetingID,
			CandidateID: input.CandidateID, Status: input.Disposition,
			Reason: strings.TrimSpace(input.Reason), UpdatedAt: now.Format(time.RFC3339Nano)}, now)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	events := []event.Envelope{item}
	remaining := 0
	for _, candidate := range snapshot.Candidates {
		if candidate.Status == "proposed" && candidate.ID != input.CandidateID {
			remaining++
		}
	}
	if remaining == 0 {
		status, err := statusEvent(snapshot.Meeting.ProjectID, meetingID, "concluded",
			snapshot.Meeting.Cycle, snapshot.Meeting.CurrentRound, "",
			snapshot.Meeting.ResultDigest, "human", correlationID, now)
		if err != nil {
			return store.MeetingSnapshot{}, err
		}
		events = append(events, status)
	}
	if err := s.append(ctx, snapshot.Meeting.ProjectID, snapshot.StreamVersion, meetingID, events); err != nil {
		return store.MeetingSnapshot{}, err
	}
	return s.store.InspectMeeting(ctx, meetingID)
}

func (s *Service) Review(ctx context.Context, meetingID string, input ReviewInput) (store.MeetingSnapshot, error) {
	// Persist one complete option-first review and deterministically advance its owner.

	// 1. Validate the command against the exact current Result revision.
	snapshot, err := s.Inspect(ctx, meetingID)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	if snapshot.Meeting.ProtocolVersion != 2 || snapshot.Meeting.Status != "awaiting_user_review" {
		return store.MeetingSnapshot{}, newError("invalid_meeting_state", "Meeting is not awaiting a protocol v2 review")
	}
	if input.ResultAction != "approve" && input.ResultAction != "continue" {
		return store.MeetingSnapshot{}, newError("invalid_argument", "result_action must be approve or continue")
	}
	pending := make(map[string]bool)
	for _, candidate := range snapshot.Candidates {
		if contains(candidate.SourceRefs, snapshot.Meeting.ResultContentID) {
			pending[candidate.ID] = true
		}
	}
	if len(input.Items) != len(pending) {
		return store.MeetingSnapshot{}, newError("invalid_argument", "review must disposition every item in the current Result")
	}
	seen := make(map[string]bool, len(input.Items))
	for _, item := range input.Items {
		if !pending[item.CandidateID] || seen[item.CandidateID] || !validReviewItem(item) {
			return store.MeetingSnapshot{}, newError("invalid_argument", "review contains an invalid or duplicate item")
		}
		if input.ResultAction == "approve" && item.DesignDisposition != "accepted" {
			return store.MeetingSnapshot{}, newError("invalid_argument", "approving the complete Result requires every design item to be accepted")
		}
		for _, candidate := range snapshot.Candidates {
			if candidate.ID != item.CandidateID {
				continue
			}
			if candidate.ContextDisposition == "persist" &&
				(item.DesignDisposition != "accepted" || item.ContextDisposition != "persist") {
				return store.MeetingSnapshot{}, newError("invalid_argument", "persisted project context is locked")
			}
			if candidate.DesignDisposition == "rejected" && item.DesignDisposition != "rejected" {
				return store.MeetingSnapshot{}, newError("invalid_argument", "rejected semantic items cannot be silently reintroduced")
			}
		}
		seen[item.CandidateID] = true
	}

	// 2. Store optional prose and the structured choices in one stream transaction.
	now := time.Now().UTC()
	correlationID, err := event.NewID()
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	events := make([]event.Envelope, 0, 4)
	commentDigest := ""
	comment := strings.TrimSpace(input.Comment)
	if comment != "" {
		record, err := s.artifacts.Put([]byte(comment), "text/plain;charset=utf-8")
		if err != nil {
			return store.MeetingSnapshot{}, err
		}
		commentDigest = record.Digest
		contentID, err := event.NewID()
		if err != nil {
			return store.MeetingSnapshot{}, err
		}
		content, err := event.NewEnvelope(snapshot.Meeting.ProjectID, "meeting_content", contentID,
			event.MeetingContentAdded, "human", correlationID, event.MeetingContentAddedPayload{
				MeetingID: meetingID, ContentID: contentID, Kind: "review_comment",
				Cycle: snapshot.Meeting.Cycle, Round: snapshot.Meeting.CurrentRound,
				ContentDigest: record.Digest, Refs: []string{snapshot.Meeting.ResultContentID},
				CreatedAt: now.Format(time.RFC3339Nano)}, now)
		if err != nil {
			return store.MeetingSnapshot{}, err
		}
		content.Artifacts = []event.ArtifactRef{artifactRef(record, "meeting_content")}
		events = append(events, content)
	}
	review, err := event.NewEnvelope(snapshot.Meeting.ProjectID, "meeting", meetingID,
		event.MeetingReviewed, "human", correlationID, event.MeetingReviewedPayload{
			MeetingID: meetingID, ResultContentID: snapshot.Meeting.ResultContentID,
			ResultAction: input.ResultAction, Items: input.Items, CommentDigest: commentDigest,
			ReviewedAt: now.Format(time.RFC3339Nano)}, now)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	events = append(events, review)

	// 3. Approval concludes without another model call; continuation belongs to Recorder.
	status := "recorder_reviewing"
	if input.ResultAction == "approve" {
		approved, err := event.NewEnvelope(snapshot.Meeting.ProjectID, "meeting", meetingID,
			event.MeetingResultApproved, "human", correlationID, event.MeetingResultApprovedPayload{
				MeetingID: meetingID, ResultContentID: snapshot.Meeting.ResultContentID,
				ResultDigest: snapshot.Meeting.ResultDigest, ApprovedAt: now.Format(time.RFC3339Nano)}, now)
		if err != nil {
			return store.MeetingSnapshot{}, err
		}
		events = append(events, approved)
		status = "concluded"
	}
	statusItem, err := statusEventWithContent(snapshot.Meeting.ProjectID, meetingID, status,
		snapshot.Meeting.Cycle, snapshot.Meeting.CurrentRound, "", snapshot.Meeting.ResultDigest,
		snapshot.Meeting.ResultContentID, "human", correlationID, now)
	if err != nil {
		return store.MeetingSnapshot{}, err
	}
	events = append(events, statusItem)
	if err := s.append(ctx, snapshot.Meeting.ProjectID, snapshot.StreamVersion, meetingID, events); err != nil {
		return store.MeetingSnapshot{}, err
	}
	if status == "recorder_reviewing" {
		s.enqueue(meetingID)
	}
	return s.store.InspectMeeting(ctx, meetingID)
}

func (s *Service) ReadContent(ctx context.Context, meetingID, contentID string) (Content, error) {
	snapshot, err := s.Inspect(ctx, meetingID)
	if err != nil {
		return Content{}, err
	}
	for _, item := range snapshot.Contents {
		if item.ID == contentID {
			data, err := s.artifacts.Read(item.ContentDigest)
			if err != nil {
				return Content{}, err
			}
			return Content{ID: item.ID, Kind: item.Kind, MediaType: "application/json", Body: string(data)}, nil
		}
	}
	return Content{}, newError("content_not_found", "Meeting content is not referenced by this Meeting")
}

func (s *Service) Handoff(ctx context.Context, meetingID string) (Handoff, error) {
	snapshot, err := s.Inspect(ctx, meetingID)
	if err != nil {
		return Handoff{}, err
	}
	if snapshot.Meeting.Status != "concluded" || snapshot.Meeting.ApprovedResultDigest == "" {
		return Handoff{}, newError("invalid_meeting_state", "Meeting has no approved Result")
	}
	data, err := s.artifacts.Read(snapshot.Meeting.ApprovedResultDigest)
	if err != nil {
		return Handoff{}, err
	}
	approved := make([]store.SemanticCandidate, 0)
	for _, candidate := range snapshot.Candidates {
		if candidate.ContextDisposition == "persist" {
			approved = append(approved, candidate)
		}
	}
	return Handoff{Schema: "nemeton.coding-design-handoff.v1", MeetingID: meetingID,
		Title: snapshot.Meeting.Title, OriginalBrief: snapshot.Meeting.Brief,
		ResultContentID: snapshot.Meeting.ResultContentID,
		ResultDigest:    snapshot.Meeting.ApprovedResultDigest, Result: string(data),
		ApprovedContext: approved}, nil
}

func validReviewItem(item event.SemanticReviewItem) bool {
	switch item.DesignDisposition {
	case "accepted":
		return item.ContextDisposition == "persist" || item.ContextDisposition == "result_only"
	case "rejected", "deferred":
		return item.ContextDisposition == "none"
	default:
		return false
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func (s *Service) EventsAfter(ctx context.Context, meetingID string, after int64) ([]store.StreamEvent, error) {
	if after < 0 {
		return nil, newError("invalid_argument", "after_sequence must be non-negative")
	}
	return s.store.MeetingEventsAfter(ctx, meetingID, after)
}

func (s *Service) Run(ctx context.Context, meetingID string) error {
	// 1. Close incomplete runs left by a previous daemon before advancing.
	snapshot, err := s.store.InspectMeeting(ctx, meetingID)
	if err != nil {
		return err
	}
	if err := s.interruptUnfinishedRuns(ctx, snapshot); err != nil {
		return err
	}
	snapshot, err = s.store.InspectMeeting(ctx, meetingID)
	if err != nil {
		return err
	}
	if snapshot.Meeting.Status == "recorder_reviewing" {
		if err := s.recorderReview(ctx, snapshot); err != nil {
			return s.failMeeting(ctx, meetingID, err)
		}
		return nil
	}
	if snapshot.Meeting.Status == "reconvening" {
		if err := s.setStatus(ctx, snapshot, "sealed_proposals", 0, "", snapshot.Meeting.ResultDigest); err != nil {
			return err
		}
		snapshot, err = s.store.InspectMeeting(ctx, meetingID)
		if err != nil {
			return err
		}
	}
	if snapshot.Meeting.Status == "preparing" || snapshot.Meeting.Status == "sealed_proposals" {
		if err := s.prepareWorkspaces(ctx, snapshot); err != nil {
			return s.failMeeting(ctx, meetingID, err)
		}
		if err := s.ensureProposals(ctx, meetingID); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return s.failMeeting(ctx, meetingID, err)
		}
		snapshot, err = s.store.InspectMeeting(ctx, meetingID)
		if err != nil {
			return err
		}
		if snapshot.Meeting.ProtocolVersion == 2 && designParticipantCount(snapshot) == 0 {
			if err := s.setStatus(ctx, snapshot, "recording", 0, "", ""); err != nil {
				return err
			}
		} else if snapshot.Meeting.Status != "revealed" {
			if err := s.setStatus(ctx, snapshot, "revealed", 0, "", ""); err != nil {
				return err
			}
		}
	}

	// 2. Run a bounded deliberation cycle or persist a Human input request.
	snapshot, err = s.store.InspectMeeting(ctx, meetingID)
	if err != nil {
		return err
	}
	if snapshot.Meeting.Status == "revealed" || snapshot.Meeting.Status == "deliberating" {
		converged, err := s.deliberate(ctx, snapshot)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return s.failMeeting(ctx, meetingID, err)
		}
		if !converged {
			return nil
		}
	}

	// 3. Compile, verify, and hand candidates to the Human.
	snapshot, err = s.store.InspectMeeting(ctx, meetingID)
	if err != nil {
		return err
	}
	if snapshot.Meeting.Status == "recording" {
		record := s.record
		if snapshot.Meeting.ProtocolVersion == 2 {
			record = s.recordV2
		}
		if err := record(ctx, snapshot); err != nil {
			return s.failMeeting(ctx, meetingID, err)
		}
	}
	snapshot, err = s.store.InspectMeeting(ctx, meetingID)
	if err != nil {
		return err
	}
	if snapshot.Meeting.ProtocolVersion == 1 && snapshot.Meeting.Status == "verifying" {
		if err := s.verify(ctx, snapshot); err != nil {
			return s.failMeeting(ctx, meetingID, err)
		}
	}
	return nil
}

func (s *Service) prepareWorkspaces(ctx context.Context, snapshot store.MeetingSnapshot) error {
	bundleData, err := s.artifacts.Read(snapshot.Meeting.RealityBundleDigest)
	if err != nil {
		return err
	}
	bundle, err := realitybundle.Decode(bundleData)
	if err != nil {
		return err
	}
	for _, participant := range snapshot.Participants {
		if info, err := os.Stat(participant.Workdir); err == nil && info.IsDir() {
			continue
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect participant Workdir %s: %w", participant.Workdir, err)
		}
		root := filepath.Dir(filepath.Dir(participant.Workdir))
		if _, err := realitybundle.Prepare(ctx, bundle, root); err != nil {
			return fmt.Errorf("prepare %s Workdir: %w", participant.Seat, err)
		}
	}
	current, err := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
	if err != nil {
		return err
	}
	if current.Meeting.Status == "preparing" {
		return s.setStatus(ctx, current, "sealed_proposals", 0, "", "")
	}
	return nil
}

func (s *Service) ensureProposals(ctx context.Context, meetingID string) error {
	snapshot, err := s.store.InspectMeeting(ctx, meetingID)
	if err != nil {
		return err
	}
	var designers []store.MeetingParticipant
	for _, participant := range snapshot.Participants {
		if isDesignRole(participant.Role) && !hasCycleContent(snapshot, participant.ID, "proposal") {
			designers = append(designers, participant)
		}
	}
	if len(designers) == 0 {
		return nil
	}
	type completed struct {
		participant store.MeetingParticipant
		result      runResult
		err         error
	}
	results := make(chan completed, len(designers))
	for _, participant := range designers {
		participant := participant
		go func() {
			prompt := proposalPrompt(snapshot.Meeting, participant)
			result, err := s.execute(ctx, snapshot, participant, "proposal", prompt)
			results <- completed{participant: participant, result: result, err: err}
		}()
	}
	var failures []string
	for range designers {
		item := <-results
		if item.err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", item.participant.Seat, item.err))
			continue
		}
		var output proposalOutput
		if err := decodeStructured(item.result.RunnerResult.Output, &output); err != nil {
			protocolErr := fmt.Errorf("decode %s proposal: %w", item.participant.Seat, err)
			if persistErr := s.persistRunFailure(ctx, meetingID, item.participant, "proposal", item.result, protocolErr); persistErr != nil {
				return persistErr
			}
			failures = append(failures, protocolErr.Error())
			continue
		}
		if strings.TrimSpace(output.Summary) == "" {
			protocolErr := fmt.Errorf("%s proposal must contain a summary", item.participant.Seat)
			if persistErr := s.persistRunFailure(ctx, meetingID, item.participant, "proposal", item.result, protocolErr); persistErr != nil {
				return persistErr
			}
			failures = append(failures, protocolErr.Error())
			continue
		}
		if err := s.persistRunResult(ctx, meetingID, item.participant, "proposal", item.result, "proposal", nil); err != nil {
			return err
		}
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		return fmt.Errorf("missing Design seat: %s", strings.Join(failures, "; "))
	}
	return nil
}

type proposalOutput struct {
	Summary        string   `json:"summary"`
	Claims         []string `json:"claims"`
	Evidence       []string `json:"evidence"`
	Risks          []string `json:"risks"`
	CandidateItems []struct {
		Kind      string `json:"kind"`
		Statement string `json:"statement"`
	} `json:"candidate_items"`
}

type deliberationOutput struct {
	Verdict            string   `json:"verdict"`
	CanonicalStatement string   `json:"canonical_statement"`
	Rationale          string   `json:"rationale"`
	Evidence           []string `json:"evidence"`
}

func (s *Service) deliberate(ctx context.Context, snapshot store.MeetingSnapshot) (bool, error) {
	conflict, err := s.ensureConflict(ctx, snapshot)
	if err != nil {
		return false, err
	}
	for round := snapshot.Meeting.CurrentRound + 1; round <= snapshot.Meeting.MaxRounds; round++ {
		current, err := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
		if err != nil {
			return false, err
		}
		if err := s.setStatus(ctx, current, "deliberating", round, "", ""); err != nil {
			return false, err
		}
		current, err = s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
		if err != nil {
			return false, err
		}
		materials, err := s.meetingMaterials(current)
		if err != nil {
			return false, err
		}
		positions := make(map[string]any)
		allAccept := true
		canonical := ""
		for _, participant := range current.Participants {
			if !isDesignRole(participant.Role) {
				continue
			}
			result, err := s.execute(ctx, current, participant, "deliberation",
				deliberationPrompt(current.Meeting, participant, conflict.Question, round, materials))
			if err != nil {
				return false, err
			}
			var output deliberationOutput
			if err := decodeStructured(result.RunnerResult.Output, &output); err != nil {
				protocolErr := fmt.Errorf("decode %s deliberation: %w", participant.Seat, err)
				if err := s.persistRunFailure(ctx, current.Meeting.ID, participant, "deliberation", result, protocolErr); err != nil {
					return false, err
				}
				return false, protocolErr
			}
			if output.Verdict != "accept" && output.Verdict != "reject" && output.Verdict != "conditional" {
				protocolErr := fmt.Errorf("%s deliberation returned invalid verdict %q", participant.Seat, output.Verdict)
				if err := s.persistRunFailure(ctx, current.Meeting.ID, participant, "deliberation", result, protocolErr); err != nil {
					return false, err
				}
				return false, protocolErr
			}
			if output.Verdict != "accept" || strings.TrimSpace(output.CanonicalStatement) == "" {
				allAccept = false
			}
			normalized := normalizeStatement(output.CanonicalStatement)
			if canonical == "" {
				canonical = normalized
			} else if canonical != normalized {
				allAccept = false
			}
			positions[participant.ID] = output
			if err := s.persistRunResult(ctx, current.Meeting.ID, participant,
				"deliberation", result, "position", []string{conflict.ID}); err != nil {
				return false, err
			}
		}
		current, err = s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
		if err != nil {
			return false, err
		}
		status := "open"
		if allAccept {
			status = "resolved"
		}
		if err := s.setConflict(ctx, current, conflict.ID, conflict.Question,
			status, round, positions); err != nil {
			return false, err
		}
		if allAccept {
			current, err = s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
			if err != nil {
				return false, err
			}
			if err := s.setStatus(ctx, current, "recording", round, "", ""); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	current, err := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
	if err != nil {
		return false, err
	}
	if current.Meeting.ProtocolVersion == 2 {
		if err := s.setStatus(ctx, current, "recording", current.Meeting.MaxRounds, "", ""); err != nil {
			return false, err
		}
		return true, nil
	}
	question := fmt.Sprintf("Conflict %s did not reach unanimous acceptance after %d rounds: %s",
		conflict.ID, current.Meeting.MaxRounds, conflict.Question)
	if err := s.setStatus(ctx, current, "needs_user_input",
		current.Meeting.MaxRounds, question, ""); err != nil {
		return false, err
	}
	return false, nil
}

func (s *Service) ensureConflict(ctx context.Context, snapshot store.MeetingSnapshot) (store.MeetingConflict, error) {
	for _, conflict := range snapshot.Conflicts {
		if conflict.Status == "open" && conflict.Cycle == snapshot.Meeting.Cycle {
			return conflict, nil
		}
	}
	conflictID, err := event.NewID()
	if err != nil {
		return store.MeetingConflict{}, err
	}
	question := "Which evidence-backed canonical design should answer the Meeting brief?"
	if err := s.setConflict(ctx, snapshot, conflictID, question, "open", 0, map[string]any{}); err != nil {
		return store.MeetingConflict{}, err
	}
	return store.MeetingConflict{ID: conflictID, MeetingID: snapshot.Meeting.ID,
		Question: question, Status: "open", Cycle: snapshot.Meeting.Cycle}, nil
}

type recorderOutput struct {
	Synthesis  string `json:"synthesis"`
	Candidates []struct {
		Kind       string   `json:"kind"`
		Statement  string   `json:"statement"`
		Rationale  string   `json:"rationale"`
		SourceRefs []string `json:"source_refs"`
	} `json:"candidates"`
}

type recorderReviewOutput struct {
	Action     string `json:"action"`
	Response   string `json:"response"`
	Opening    string `json:"opening"`
	Synthesis  string `json:"synthesis"`
	Candidates []struct {
		Kind       string   `json:"kind"`
		Statement  string   `json:"statement"`
		Rationale  string   `json:"rationale"`
		SourceRefs []string `json:"source_refs"`
	} `json:"candidates"`
}

func (s *Service) recordV2(ctx context.Context, snapshot store.MeetingSnapshot) error {
	// Ask the sole Recorder for a complete, source-grounded Result revision.
	recorder, err := participantByRole(snapshot, "recorder")
	if err != nil {
		return err
	}
	materials, err := s.meetingMaterials(snapshot)
	if err != nil {
		return err
	}
	result, err := s.execute(ctx, snapshot, recorder, "recording", recorderV2Prompt(snapshot.Meeting, materials))
	if err != nil {
		return err
	}
	var output recorderOutput
	if err := decodeStructured(result.RunnerResult.Output, &output); err != nil {
		protocolErr := fmt.Errorf("decode Recorder Result: %w", err)
		if err := s.persistRunFailure(ctx, snapshot.Meeting.ID, recorder, "recording", result, protocolErr); err != nil {
			return err
		}
		return protocolErr
	}
	if err := validateRecorderResult(output, snapshot.Contents); err != nil {
		if persistErr := s.persistRunFailure(ctx, snapshot.Meeting.ID, recorder, "recording", result, err); persistErr != nil {
			return persistErr
		}
		return err
	}
	if err := s.persistRunResult(ctx, snapshot.Meeting.ID, recorder, "recording", result, "meeting_result", nil); err != nil {
		return err
	}
	current, err := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
	if err != nil {
		return err
	}
	resultContent, err := contentByDigest(current, result.OutputRecord.Digest, "meeting_result")
	if err != nil {
		return err
	}
	return s.persistResultCandidates(ctx, current, resultContent, result.OutputRecord.Digest, output.Candidates)
}

func (s *Service) recorderReview(ctx context.Context, snapshot store.MeetingSnapshot) error {
	// Let Recorder choose the smallest valid next action from review state.
	recorder, err := participantByRole(snapshot, "recorder")
	if err != nil {
		return err
	}
	materials, err := s.meetingMaterials(snapshot)
	if err != nil {
		return err
	}
	prompt := recorderReviewPrompt(snapshot.Meeting, materials)
	for attempt := 1; attempt <= 2; attempt++ {
		result, err := s.execute(ctx, snapshot, recorder, "recorder_review", prompt)
		if err != nil {
			return err
		}
		var output recorderReviewOutput
		decodeErr := decodeStructured(result.RunnerResult.Output, &output)
		if decodeErr != nil {
			err = fmt.Errorf("decode Recorder review: %w", decodeErr)
		} else {
			err = validateRecorderReview(snapshot, output)
		}
		if err == nil {
			return s.applyRecorderReview(ctx, snapshot, recorder, result, output)
		}
		if persistErr := s.persistRunFailure(ctx, snapshot.Meeting.ID, recorder, "recorder_review", result, err); persistErr != nil {
			return persistErr
		}
		if attempt == 2 {
			current, inspectErr := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
			if inspectErr != nil {
				return inspectErr
			}
			question := "Recorder could not produce a protocol-compliant decision after 2 attempts: " + err.Error()
			return s.setStatusWithContent(ctx, current, "awaiting_user_review", current.Meeting.Cycle,
				current.Meeting.CurrentRound, question, current.Meeting.ResultDigest, current.Meeting.ResultContentID)
		}
		prompt = recorderReviewCorrectionPrompt(prompt, err.Error())
	}
	panic("unreachable Recorder review attempt count")
}

func validateRecorderReview(snapshot store.MeetingSnapshot, output recorderReviewOutput) error {
	if requiresSwarmReview(snapshot) && output.Action != "reconvene" {
		return fmt.Errorf("a rejected boundary or invariant requires reconvene, got %s", output.Action)
	}
	switch output.Action {
	case "answer":
		if strings.TrimSpace(output.Response) == "" {
			return fmt.Errorf("Recorder answer must contain a response")
		}
	case "patch":
		converted := recorderOutput{Synthesis: output.Synthesis}
		for _, candidate := range output.Candidates {
			converted.Candidates = append(converted.Candidates, struct {
				Kind       string   `json:"kind"`
				Statement  string   `json:"statement"`
				Rationale  string   `json:"rationale"`
				SourceRefs []string `json:"source_refs"`
			}{Kind: candidate.Kind, Statement: candidate.Statement, Rationale: candidate.Rationale,
				SourceRefs: candidate.SourceRefs})
		}
		if err := validateRecorderResult(converted, snapshot.Contents); err != nil {
			return err
		}
	case "reconvene":
		if strings.TrimSpace(output.Opening) == "" {
			return fmt.Errorf("Recorder reconvene decision must contain an opening")
		}
	default:
		return fmt.Errorf("Recorder returned invalid action %q", output.Action)
	}
	return nil
}

func requiresSwarmReview(snapshot store.MeetingSnapshot) bool {
	for _, candidate := range snapshot.Candidates {
		if !contains(candidate.SourceRefs, snapshot.Meeting.ResultContentID) || candidate.DesignDisposition != "rejected" {
			continue
		}
		if candidate.Kind == "boundary" || candidate.Kind == "invariant" {
			return true
		}
	}
	return false
}

func (s *Service) applyRecorderReview(ctx context.Context, snapshot store.MeetingSnapshot,
	recorder store.MeetingParticipant, result runResult, output recorderReviewOutput) error {
	// Persist one validated Recorder decision and its observable state transition.
	switch output.Action {
	case "answer":
		if err := s.persistRunResult(ctx, snapshot.Meeting.ID, recorder, "recorder_review", result, "recorder_answer", []string{snapshot.Meeting.ResultContentID}); err != nil {
			return err
		}
		current, err := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
		if err != nil {
			return err
		}
		return s.setStatusWithContent(ctx, current, "awaiting_user_review", current.Meeting.Cycle,
			current.Meeting.CurrentRound, "", current.Meeting.ResultDigest, current.Meeting.ResultContentID)
	case "patch":
		converted := recorderOutput{Synthesis: output.Synthesis}
		for _, candidate := range output.Candidates {
			converted.Candidates = append(converted.Candidates, struct {
				Kind       string   `json:"kind"`
				Statement  string   `json:"statement"`
				Rationale  string   `json:"rationale"`
				SourceRefs []string `json:"source_refs"`
			}{Kind: candidate.Kind, Statement: candidate.Statement, Rationale: candidate.Rationale,
				SourceRefs: candidate.SourceRefs})
		}
		if err := s.persistRunResult(ctx, snapshot.Meeting.ID, recorder, "recorder_review", result,
			"meeting_result", []string{snapshot.Meeting.ResultContentID}); err != nil {
			return err
		}
		current, err := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
		if err != nil {
			return err
		}
		content, err := contentByDigest(current, result.OutputRecord.Digest, "meeting_result")
		if err != nil {
			return err
		}
		return s.persistResultCandidates(ctx, current, content, result.OutputRecord.Digest, converted.Candidates)
	case "reconvene":
		if err := s.persistRunResult(ctx, snapshot.Meeting.ID, recorder, "recorder_review", result,
			"recorder_opening", []string{snapshot.Meeting.ResultContentID}); err != nil {
			return err
		}
		current, err := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
		if err != nil {
			return err
		}
		if err := s.setStatusWithContent(ctx, current, "reconvening", current.Meeting.Cycle+1,
			0, "", current.Meeting.ResultDigest, current.Meeting.ResultContentID); err != nil {
			return err
		}
		s.enqueue(snapshot.Meeting.ID)
		return nil
	}
	panic("validated Recorder action became invalid")
}

func validateRecorderResult(output recorderOutput, contents []store.MeetingContent) error {
	if strings.TrimSpace(output.Synthesis) == "" || len(output.Candidates) == 0 {
		return fmt.Errorf("Recorder must produce a complete synthesis and at least one Candidate")
	}
	for _, candidate := range output.Candidates {
		if strings.TrimSpace(candidate.Statement) == "" || len(candidate.SourceRefs) == 0 {
			return fmt.Errorf("Recorder Candidate must have a statement and source refs")
		}
		for _, reference := range candidate.SourceRefs {
			if !contentExists(contents, reference) {
				return fmt.Errorf("Recorder Candidate references unknown Meeting content %s", reference)
			}
		}
	}
	return nil
}

func (s *Service) persistResultCandidates(ctx context.Context, snapshot store.MeetingSnapshot,
	resultContent store.MeetingContent, resultDigest string, candidates []struct {
		Kind       string   `json:"kind"`
		Statement  string   `json:"statement"`
		Rationale  string   `json:"rationale"`
		SourceRefs []string `json:"source_refs"`
	}) error {
	// Atomically publish the new immutable Result pointer and its review items.
	now := time.Now().UTC()
	correlationID, err := event.NewID()
	if err != nil {
		return err
	}
	events := make([]event.Envelope, 0, len(candidates)+1)
	for _, candidate := range candidates {
		id, err := event.NewID()
		if err != nil {
			return err
		}
		item, err := event.NewEnvelope(snapshot.Meeting.ProjectID, "semantic_candidate", id,
			event.SemanticCandidateAdded, "recorder", correlationID, event.SemanticCandidateAddedPayload{
				MeetingID: snapshot.Meeting.ID, CandidateID: id, Kind: candidate.Kind,
				Statement: candidate.Statement, SourceRefs: []string{resultContent.ID}, Status: "proposed",
				Rationale: candidate.Rationale, CreatedAt: now.Format(time.RFC3339Nano)}, now)
		if err != nil {
			return err
		}
		events = append(events, item)
	}
	status, err := statusEventWithContent(snapshot.Meeting.ProjectID, snapshot.Meeting.ID,
		"awaiting_user_review", snapshot.Meeting.Cycle, snapshot.Meeting.CurrentRound, "",
		resultDigest, resultContent.ID, "recorder", correlationID, now)
	if err != nil {
		return err
	}
	events = append(events, status)
	return s.append(ctx, snapshot.Meeting.ProjectID, snapshot.StreamVersion, snapshot.Meeting.ID, events)
}

func (s *Service) record(ctx context.Context, snapshot store.MeetingSnapshot) error {
	recorder, err := participantByRole(snapshot, "recorder")
	if err != nil {
		return err
	}
	materials, err := s.meetingMaterials(snapshot)
	if err != nil {
		return err
	}
	result, err := s.execute(ctx, snapshot, recorder, "recording", recorderPrompt(snapshot.Meeting, materials))
	if err != nil {
		return err
	}
	var output recorderOutput
	if err := decodeStructured(result.RunnerResult.Output, &output); err != nil {
		protocolErr := fmt.Errorf("decode Recorder output: %w", err)
		if err := s.persistRunFailure(ctx, snapshot.Meeting.ID, recorder, "recording", result, protocolErr); err != nil {
			return err
		}
		return protocolErr
	}
	if strings.TrimSpace(output.Synthesis) == "" || len(output.Candidates) == 0 {
		protocolErr := fmt.Errorf("Recorder must produce a synthesis and at least one Candidate")
		if err := s.persistRunFailure(ctx, snapshot.Meeting.ID, recorder, "recording", result, protocolErr); err != nil {
			return err
		}
		return protocolErr
	}
	for _, candidate := range output.Candidates {
		if strings.TrimSpace(candidate.Statement) == "" || len(candidate.SourceRefs) == 0 {
			protocolErr := fmt.Errorf("Recorder Candidate must have a statement and source refs")
			if err := s.persistRunFailure(ctx, snapshot.Meeting.ID, recorder, "recording", result, protocolErr); err != nil {
				return err
			}
			return protocolErr
		}
		for _, reference := range candidate.SourceRefs {
			if !contentExists(snapshot.Contents, reference) {
				protocolErr := fmt.Errorf("Recorder Candidate references unknown Meeting content %s", reference)
				if err := s.persistRunFailure(ctx, snapshot.Meeting.ID, recorder, "recording", result, protocolErr); err != nil {
					return err
				}
				return protocolErr
			}
		}
	}
	if err := s.persistRunResult(ctx, snapshot.Meeting.ID, recorder, "recording", result, "synthesis", nil); err != nil {
		return err
	}
	current, err := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	correlationID, err := event.NewID()
	if err != nil {
		return err
	}
	var events []event.Envelope
	for _, candidate := range output.Candidates {
		id, err := event.NewID()
		if err != nil {
			return err
		}
		item, err := event.NewEnvelope(current.Meeting.ProjectID, "semantic_candidate", id,
			event.SemanticCandidateAdded, "recorder", correlationID,
			event.SemanticCandidateAddedPayload{MeetingID: current.Meeting.ID,
				CandidateID: id, Kind: candidate.Kind, Statement: candidate.Statement,
				SourceRefs: candidate.SourceRefs, Status: "proposed",
				Rationale: candidate.Rationale, CreatedAt: now.Format(time.RFC3339Nano)}, now)
		if err != nil {
			return err
		}
		events = append(events, item)
	}
	status, err := statusEvent(current.Meeting.ProjectID, current.Meeting.ID, "verifying",
		current.Meeting.Cycle, current.Meeting.CurrentRound, "", result.OutputRecord.Digest,
		"facilitator", correlationID, now)
	if err != nil {
		return err
	}
	events = append(events, status)
	return s.append(ctx, current.Meeting.ProjectID, current.StreamVersion, current.Meeting.ID, events)
}

type verifierOutput struct {
	Clear            bool     `json:"clear"`
	BlockingFindings []string `json:"blocking_findings"`
}

func (s *Service) verify(ctx context.Context, snapshot store.MeetingSnapshot) error {
	verifier, err := participantByRole(snapshot, "verifier")
	if err != nil {
		return err
	}
	materials, err := s.meetingMaterials(snapshot)
	if err != nil {
		return err
	}
	result, err := s.execute(ctx, snapshot, verifier, "verifying", verifierPrompt(snapshot.Meeting, materials))
	if err != nil {
		return err
	}
	var output verifierOutput
	if err := decodeStructured(result.RunnerResult.Output, &output); err != nil {
		protocolErr := fmt.Errorf("decode Verifier output: %w", err)
		if err := s.persistRunFailure(ctx, snapshot.Meeting.ID, verifier, "verifying", result, protocolErr); err != nil {
			return err
		}
		return protocolErr
	}
	if !output.Clear && len(output.BlockingFindings) == 0 {
		protocolErr := fmt.Errorf("Verifier must provide blocking findings when clear is false")
		if err := s.persistRunFailure(ctx, snapshot.Meeting.ID, verifier, "verifying", result, protocolErr); err != nil {
			return err
		}
		return protocolErr
	}
	if err := s.persistRunResult(ctx, snapshot.Meeting.ID, verifier, "verifying", result, "verification", nil); err != nil {
		return err
	}
	if err := s.verifySourceObservation(ctx, snapshot); err != nil {
		return err
	}
	current, err := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
	if err != nil {
		return err
	}
	if !output.Clear || len(output.BlockingFindings) > 0 {
		question := "Verifier reported blocking findings: " + strings.Join(output.BlockingFindings, "; ")
		return s.setStatus(ctx, current, "needs_user_input", current.Meeting.CurrentRound, question, current.Meeting.ResultDigest)
	}
	return s.setStatus(ctx, current, "awaiting_human", current.Meeting.CurrentRound, "", current.Meeting.ResultDigest)
}

func (s *Service) verifySourceObservation(ctx context.Context, snapshot store.MeetingSnapshot) error {
	// Verify that Provider work left the observed source repository byte-for-byte unchanged.

	// 1. Load the frozen source observation from the Meeting's Reality Bundle.
	bundleData, err := s.artifacts.Read(snapshot.Meeting.RealityBundleDigest)
	if err != nil {
		return err
	}
	bundle, err := realitybundle.Decode(bundleData)
	if err != nil {
		return err
	}

	// 2. Compare the current source view at the final Provider boundary.
	observed, err := realitybundle.Observe(ctx, bundle.SourceRoot)
	if err != nil {
		return fmt.Errorf("observe Meeting source after Provider execution: %w", err)
	}
	if observed != bundle.Observation {
		return fmt.Errorf("Meeting source repository changed during Provider execution")
	}
	return nil
}

type runResult struct {
	RunnerResult runner.Result
	InputRecord  artifact.Record
	OutputRecord artifact.Record
	RawRecord    artifact.Record
	StderrRecord artifact.Record
	Error        string
}

func (s *Service) execute(ctx context.Context, snapshot store.MeetingSnapshot, participant store.MeetingParticipant, phase, prompt string) (runResult, error) {
	inputRecord, err := s.artifacts.Put([]byte(prompt), "text/plain;charset=utf-8")
	if err != nil {
		return runResult{}, err
	}
	runID, err := event.NewID()
	if err != nil {
		return runResult{}, err
	}
	started := time.Now().UTC()
	s.runRecordMu.Lock()
	current, err := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
	if err != nil {
		s.runRecordMu.Unlock()
		return runResult{}, err
	}
	if err := s.recordRun(ctx, current, participant, runID, phase, "running",
		participant.SessionID, inputRecord, artifact.Record{}, runner.Result{}, artifact.Record{}, artifact.Record{},
		0, "", started, time.Time{}); err != nil {
		s.runRecordMu.Unlock()
		return runResult{}, err
	}
	s.runRecordMu.Unlock()
	runCtx, cancel := context.WithTimeout(ctx, defaultRunTimeout)
	defer cancel()
	var sessionMu sync.Mutex
	var sessionErr error
	pinSession := func(sessionID string) error {
		s.runRecordMu.Lock()
		defer s.runRecordMu.Unlock()
		current, err := s.store.InspectMeeting(runCtx, snapshot.Meeting.ID)
		if err == nil {
			err = s.recordRun(runCtx, current, participant, runID, phase, "running",
				sessionID, inputRecord, artifact.Record{}, runner.Result{}, artifact.Record{}, artifact.Record{},
				0, "", started, time.Time{})
		}
		if err != nil {
			sessionMu.Lock()
			sessionErr = err
			sessionMu.Unlock()
			cancel()
		}
		return err
	}
	result, runErr := s.runners.Execute(runCtx, runner.Request{Provider: participant.Provider,
		Model: participant.Model, Options: participant.ProviderOptions,
		Prompt: prompt, Workdir: participant.Workdir,
		OutputDir: filepath.Join(filepath.Dir(filepath.Dir(participant.Workdir)), "output"),
		SessionID: participant.SessionID, Timeout: defaultRunTimeout, PinSession: pinSession}, func(delta runner.Delta) {
		if s.publisher != nil {
			s.publisher.PublishDelta(snapshot.Meeting.ID, participant.ID, delta)
		}
	})
	sessionMu.Lock()
	pinErr := sessionErr
	sessionMu.Unlock()
	if pinErr != nil {
		runErr = fmt.Errorf("persist Provider Session: %w", pinErr)
	}
	rawRecord, rawErr := s.artifacts.Put(result.RawStream, "application/x-ndjson")
	if rawErr != nil {
		return runResult{}, rawErr
	}
	stderrRecord, stderrErr := s.artifacts.Put([]byte(result.Stderr), "text/plain;charset=utf-8")
	if stderrErr != nil {
		return runResult{}, stderrErr
	}
	outputRecord := artifact.Record{}
	if result.Output != "" {
		outputRecord, err = s.artifacts.Put([]byte(result.Output), "application/json")
		if err != nil {
			return runResult{}, err
		}
	}
	completed := time.Now().UTC()
	if runErr != nil {
		s.runRecordMu.Lock()
		defer s.runRecordMu.Unlock()
		current, inspectErr := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
		if inspectErr != nil {
			return runResult{}, inspectErr
		}
		if err := s.recordRun(ctx, current, participant, runID, phase, "failed",
			result.SessionID, inputRecord, outputRecord, result, rawRecord, stderrRecord,
			result.ExitCode, runErr.Error(),
			started, completed); err != nil {
			return runResult{}, err
		}
		return runResult{RunnerResult: result, InputRecord: inputRecord,
			OutputRecord: outputRecord, RawRecord: rawRecord, StderrRecord: stderrRecord,
			Error: runErr.Error()}, runErr
	}
	return runResult{RunnerResult: result, InputRecord: inputRecord,
		OutputRecord: outputRecord, RawRecord: rawRecord, StderrRecord: stderrRecord}, nil
}

func (s *Service) persistRunResult(ctx context.Context, meetingID string, participant store.MeetingParticipant, phase string, result runResult, kind string, refs []string) error {
	s.runRecordMu.Lock()
	defer s.runRecordMu.Unlock()
	current, err := s.store.InspectMeeting(ctx, meetingID)
	if err != nil {
		return err
	}
	var run store.AgentRun
	for index := len(current.Runs) - 1; index >= 0; index-- {
		if current.Runs[index].ParticipantID == participant.ID && current.Runs[index].Phase == phase && current.Runs[index].Status == "running" {
			run = current.Runs[index]
			break
		}
	}
	if run.ID == "" {
		return fmt.Errorf("running Agent Run not found for %s %s", participant.Seat, phase)
	}
	now := time.Now().UTC()
	correlationID, err := event.NewID()
	if err != nil {
		return err
	}
	runEvent, err := event.NewEnvelope(current.Meeting.ProjectID, "agent_run", run.ID,
		event.AgentRunRecorded, "runner", correlationID, event.AgentRunRecordedPayload{
			MeetingID: meetingID, RunID: run.ID, ParticipantID: participant.ID,
			Phase: phase, Provider: participant.Provider, Status: "completed",
			ProviderVersion: result.RunnerResult.Version, Command: result.RunnerResult.Command,
			SessionID: result.RunnerResult.SessionID, InputDigest: result.InputRecord.Digest,
			OutputDigest: result.OutputRecord.Digest, RawStreamDigest: result.RawRecord.Digest,
			StderrDigest: result.StderrRecord.Digest, Workdir: participant.Workdir,
			ExitCode: result.RunnerResult.ExitCode, StartedAt: run.StartedAt,
			FinishedAt: now.Format(time.RFC3339Nano)}, now)
	if err != nil {
		return err
	}
	runEvent.Artifacts = []event.ArtifactRef{artifactRef(result.OutputRecord, "runner_output"),
		artifactRef(result.RawRecord, "runner_stream"), artifactRef(result.StderrRecord, "runner_stderr")}
	contentID, err := event.NewID()
	if err != nil {
		return err
	}
	content, err := event.NewEnvelope(current.Meeting.ProjectID, "meeting_content", contentID,
		event.MeetingContentAdded, participant.Role, correlationID,
		event.MeetingContentAddedPayload{MeetingID: meetingID, ContentID: contentID,
			ParticipantID: participant.ID, Kind: kind, Cycle: current.Meeting.Cycle,
			Round: current.Meeting.CurrentRound, ContentDigest: result.OutputRecord.Digest,
			Refs: refs, CreatedAt: now.Format(time.RFC3339Nano)}, now)
	if err != nil {
		return err
	}
	content.Artifacts = []event.ArtifactRef{artifactRef(result.OutputRecord, "meeting_content")}
	return s.append(ctx, current.Meeting.ProjectID, current.StreamVersion, meetingID, []event.Envelope{runEvent, content})
}

func (s *Service) persistRunFailure(ctx context.Context, meetingID string, participant store.MeetingParticipant, phase string, result runResult, cause error) error {
	// Persist a Provider protocol failure with the same evidence as a process failure.
	s.runRecordMu.Lock()
	defer s.runRecordMu.Unlock()
	current, err := s.store.InspectMeeting(ctx, meetingID)
	if err != nil {
		return errors.Join(cause, err)
	}
	var run store.AgentRun
	for index := len(current.Runs) - 1; index >= 0; index-- {
		if current.Runs[index].ParticipantID == participant.ID && current.Runs[index].Phase == phase && current.Runs[index].Status == "running" {
			run = current.Runs[index]
			break
		}
	}
	if run.ID == "" {
		return errors.Join(cause, fmt.Errorf("running Agent Run not found for %s %s", participant.Seat, phase))
	}
	started, err := time.Parse(time.RFC3339Nano, run.StartedAt)
	if err != nil {
		return errors.Join(cause, fmt.Errorf("parse Agent Run start: %w", err))
	}
	if err := s.recordRun(ctx, current, participant, run.ID, phase, "failed",
		result.RunnerResult.SessionID, result.InputRecord, result.OutputRecord,
		result.RunnerResult, result.RawRecord, result.StderrRecord,
		result.RunnerResult.ExitCode, cause.Error(), started, time.Now().UTC()); err != nil {
		return errors.Join(cause, err)
	}
	return nil
}

func (s *Service) recordRun(ctx context.Context, snapshot store.MeetingSnapshot, participant store.MeetingParticipant, runID, phase, status, sessionID string, inputRecord, outputRecord artifact.Record, result runner.Result, rawRecord, stderrRecord artifact.Record, exitCode int, failure string, started, finished time.Time) error {
	correlationID, err := event.NewID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	finishedAt := ""
	if !finished.IsZero() {
		finishedAt = finished.Format(time.RFC3339Nano)
	}
	item, err := event.NewEnvelope(snapshot.Meeting.ProjectID, "agent_run", runID,
		event.AgentRunRecorded, "runner", correlationID, event.AgentRunRecordedPayload{
			MeetingID: snapshot.Meeting.ID, RunID: runID, ParticipantID: participant.ID,
			Phase: phase, Provider: participant.Provider, Status: status,
			ProviderVersion: result.Version, Command: result.Command,
			SessionID: sessionID, InputDigest: inputRecord.Digest,
			OutputDigest: outputRecord.Digest, RawStreamDigest: rawRecord.Digest,
			StderrDigest: stderrRecord.Digest, Workdir: participant.Workdir,
			ExitCode: exitCode, Error: failure, StartedAt: started.Format(time.RFC3339Nano),
			FinishedAt: finishedAt}, now)
	if err != nil {
		return err
	}
	if inputRecord.Digest != "" {
		item.Artifacts = append(item.Artifacts, artifactRef(inputRecord, "runner_input"))
	}
	if outputRecord.Digest != "" {
		item.Artifacts = append(item.Artifacts, artifactRef(outputRecord, "runner_output"))
	}
	if rawRecord.Digest != "" {
		item.Artifacts = append(item.Artifacts, artifactRef(rawRecord, "runner_stream"))
	}
	if stderrRecord.Digest != "" {
		item.Artifacts = append(item.Artifacts, artifactRef(stderrRecord, "runner_stderr"))
	}
	return s.append(ctx, snapshot.Meeting.ProjectID, snapshot.StreamVersion, snapshot.Meeting.ID, []event.Envelope{item})
}

func (s *Service) interruptUnfinishedRuns(ctx context.Context, snapshot store.MeetingSnapshot) error {
	for _, run := range snapshot.Runs {
		if run.Status != "running" {
			continue
		}
		participant, err := participantByID(snapshot, run.ParticipantID)
		if err != nil {
			return err
		}
		current, err := s.store.InspectMeeting(ctx, snapshot.Meeting.ID)
		if err != nil {
			return err
		}
		started, err := time.Parse(time.RFC3339Nano, run.StartedAt)
		if err != nil {
			return fmt.Errorf("parse interrupted Agent Run start: %w", err)
		}
		if err := s.recordRun(ctx, current, participant, run.ID, run.Phase,
			"interrupted", run.SessionID, artifact.Record{}, artifact.Record{}, runner.Result{}, artifact.Record{}, artifact.Record{}, -1,
			"daemon recovered an unfinished Agent Run", started, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) meetingMaterials(snapshot store.MeetingSnapshot) (string, error) {
	type material struct {
		ID            string `json:"id"`
		Kind          string `json:"kind"`
		ParticipantID string `json:"participant_id,omitempty"`
		Content       string `json:"content"`
	}
	materials := make([]material, 0, len(snapshot.Contents))
	for _, content := range snapshot.Contents {
		data, err := s.artifacts.Read(content.ContentDigest)
		if err != nil {
			return "", err
		}
		materials = append(materials, material{ID: content.ID, Kind: content.Kind,
			ParticipantID: content.ParticipantID, Content: string(data)})
	}
	document := struct {
		OriginalQuestion string                    `json:"original_question"`
		ResultContentID  string                    `json:"result_content_id,omitempty"`
		Contents         []material                `json:"contents"`
		Candidates       []store.SemanticCandidate `json:"candidate_reviews"`
	}{OriginalQuestion: snapshot.Meeting.Brief, ResultContentID: snapshot.Meeting.ResultContentID,
		Contents: materials, Candidates: snapshot.Candidates}
	data, err := json.Marshal(document)
	if err != nil {
		return "", fmt.Errorf("encode Meeting materials: %w", err)
	}
	return string(data), nil
}

func (s *Service) setConflict(ctx context.Context, snapshot store.MeetingSnapshot, conflictID, question, status string, round int, positions map[string]any) error {
	correlationID, err := event.NewID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	item, err := event.NewEnvelope(snapshot.Meeting.ProjectID, "meeting_conflict", conflictID,
		event.MeetingConflictSet, "facilitator", correlationID,
		event.MeetingConflictSetPayload{MeetingID: snapshot.Meeting.ID,
			ConflictID: conflictID, Question: question, Status: status,
			Cycle: snapshot.Meeting.Cycle, Round: round, Positions: positions,
			UpdatedAt: now.Format(time.RFC3339Nano)}, now)
	if err != nil {
		return err
	}
	return s.append(ctx, snapshot.Meeting.ProjectID, snapshot.StreamVersion, snapshot.Meeting.ID, []event.Envelope{item})
}

func (s *Service) setStatus(ctx context.Context, snapshot store.MeetingSnapshot, status string, round int, question, resultDigest string) error {
	return s.setStatusWithContent(ctx, snapshot, status, snapshot.Meeting.Cycle,
		round, question, resultDigest, "")
}

func (s *Service) setStatusWithContent(ctx context.Context, snapshot store.MeetingSnapshot,
	status string, cycle, round int, question, resultDigest, resultContentID string) error {
	correlationID, err := event.NewID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	item, err := statusEventWithContent(snapshot.Meeting.ProjectID, snapshot.Meeting.ID, status,
		cycle, round, question, resultDigest, resultContentID, "facilitator", correlationID, now)
	if err != nil {
		return err
	}
	return s.append(ctx, snapshot.Meeting.ProjectID, snapshot.StreamVersion, snapshot.Meeting.ID, []event.Envelope{item})
}

func (s *Service) failMeeting(ctx context.Context, meetingID string, cause error) error {
	recoveryContext := context.WithoutCancel(ctx)
	snapshot, err := s.store.InspectMeeting(recoveryContext, meetingID)
	if err != nil {
		return errors.Join(cause, err)
	}
	if snapshot.Meeting.Status == "failed" {
		return cause
	}
	if err := s.setStatus(recoveryContext, snapshot, "failed", snapshot.Meeting.CurrentRound, cause.Error(), snapshot.Meeting.ResultDigest); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func (s *Service) append(ctx context.Context, projectID string, expected int64, meetingID string, events []event.Envelope) error {
	if _, err := s.store.Append(ctx, store.AppendRequest{ProjectID: projectID,
		ExpectedVersion: expected, Events: events}); err != nil {
		return err
	}
	if s.publisher == nil {
		return nil
	}
	committed, err := s.store.MeetingEventsAfter(ctx, meetingID, expected)
	if err != nil {
		return err
	}
	lastSequence := expected + int64(len(events))
	for _, item := range committed {
		if item.Sequence > lastSequence {
			break
		}
		s.publisher.PublishEvent(meetingID, item)
	}
	return nil
}

func (s *Service) enqueue(meetingID string) {
	s.schedulerMu.RLock()
	scheduler := s.scheduler
	s.schedulerMu.RUnlock()
	if scheduler != nil {
		scheduler.Enqueue(meetingID)
	}
}

type participantDefinition struct {
	Seat            string
	Role            string
	Provider        string
	Model           string
	ProviderOptions map[string]string
}

func validateParticipants(participants []ParticipantInput) error {
	seats := make(map[string]bool, len(participants))
	recorders := 0
	for index, participant := range participants {
		participant.Seat = strings.TrimSpace(participant.Seat)
		participant.Model = strings.TrimSpace(participant.Model)
		if participant.Seat == "" || seats[participant.Seat] {
			return newError("invalid_argument", fmt.Sprintf("participant %d must have a unique seat", index))
		}
		seats[participant.Seat] = true
		if participant.Role != "designer" && participant.Role != "recorder" {
			return newError("invalid_argument", "protocol v2 participants must be designer or recorder")
		}
		if participant.Role == "recorder" {
			recorders++
		}
		if participant.Provider != "codex" && participant.Provider != "opencode" {
			return newError("invalid_argument", fmt.Sprintf("unsupported Provider %q", participant.Provider))
		}
		if participant.Model == "" {
			return newError("invalid_argument", fmt.Sprintf("participant %s requires an explicit model", participant.Seat))
		}
		for key, value := range participant.ProviderOptions {
			if strings.TrimSpace(value) == "" ||
				(participant.Provider == "codex" && key != "reasoning_effort") ||
				(participant.Provider == "opencode" && key != "variant") {
				return newError("invalid_argument", fmt.Sprintf("unsupported option %q for %s", key, participant.Provider))
			}
		}
	}
	if recorders != 1 {
		return newError("invalid_argument", "protocol v2 requires exactly one Recorder")
	}
	return nil
}

func participantDefinitions(overrides map[string]string) []participantDefinition {
	definitions := []participantDefinition{
		{Seat: "designer", Role: "designer", Provider: "codex"},
		{Seat: "maintainer", Role: "maintainer", Provider: "opencode"},
		{Seat: "adversary", Role: "adversary", Provider: "codex"},
		{Seat: "recorder", Role: "recorder", Provider: "opencode"},
		{Seat: "verifier", Role: "verifier", Provider: "codex"},
	}
	for index := range definitions {
		if provider := overrides[definitions[index].Seat]; provider != "" {
			definitions[index].Provider = provider
		}
	}
	return definitions
}

func validateProviderOverrides(overrides map[string]string) error {
	allowedSeats := map[string]bool{"designer": true, "maintainer": true,
		"adversary": true, "recorder": true, "verifier": true}
	for seat, provider := range overrides {
		if !allowedSeats[seat] {
			return newError("invalid_argument", fmt.Sprintf("unknown Meeting seat %q", seat))
		}
		if provider != "codex" && provider != "opencode" {
			return newError("invalid_argument", fmt.Sprintf("unsupported Provider %q for seat %s", provider, seat))
		}
	}
	return nil
}

func artifactRef(record artifact.Record, relation string) event.ArtifactRef {
	return event.ArtifactRef{Digest: record.Digest, Relation: relation,
		MediaType: record.MediaType, ByteSize: record.ByteSize, StorageURI: record.StorageURI}
}

func statusEvent(projectID, meetingID, status string, cycle, round int, question, resultDigest, actor, correlationID string, now time.Time) (event.Envelope, error) {
	return statusEventWithContent(projectID, meetingID, status, cycle, round, question,
		resultDigest, "", actor, correlationID, now)
}

func statusEventWithContent(projectID, meetingID, status string, cycle, round int, question,
	resultDigest, resultContentID, actor, correlationID string, now time.Time) (event.Envelope, error) {
	return event.NewEnvelope(projectID, "meeting", meetingID, event.MeetingStatusSet,
		actor, correlationID, event.MeetingStatusSetPayload{MeetingID: meetingID,
			Status: status, Cycle: cycle, CurrentRound: round,
			HumanQuestion: question, ResultDigest: resultDigest, ResultContentID: resultContentID,
			UpdatedAt: now.Format(time.RFC3339Nano)}, now)
}

func isDesignRole(role string) bool {
	return role == "designer" || role == "maintainer" || role == "adversary"
}

func participantByRole(snapshot store.MeetingSnapshot, role string) (store.MeetingParticipant, error) {
	for _, participant := range snapshot.Participants {
		if participant.Role == role {
			return participant, nil
		}
	}
	return store.MeetingParticipant{}, fmt.Errorf("Meeting %s has no %s", snapshot.Meeting.ID, role)
}

func participantByID(snapshot store.MeetingSnapshot, participantID string) (store.MeetingParticipant, error) {
	for _, participant := range snapshot.Participants {
		if participant.ID == participantID {
			return participant, nil
		}
	}
	return store.MeetingParticipant{}, fmt.Errorf("Meeting %s has no participant %s", snapshot.Meeting.ID, participantID)
}

func contentExists(contents []store.MeetingContent, contentID string) bool {
	for _, content := range contents {
		if content.ID == contentID {
			return true
		}
	}
	return false
}

func contentByDigest(snapshot store.MeetingSnapshot, digest, kind string) (store.MeetingContent, error) {
	for index := len(snapshot.Contents) - 1; index >= 0; index-- {
		content := snapshot.Contents[index]
		if content.ContentDigest == digest && content.Kind == kind {
			return content, nil
		}
	}
	return store.MeetingContent{}, fmt.Errorf("Meeting %s has no %s content for digest %s",
		snapshot.Meeting.ID, kind, digest)
}

func hasCycleContent(snapshot store.MeetingSnapshot, participantID, kind string) bool {
	for _, content := range snapshot.Contents {
		if content.ParticipantID == participantID && content.Kind == kind &&
			content.Cycle == snapshot.Meeting.Cycle {
			return true
		}
	}
	return false
}

func designParticipantCount(snapshot store.MeetingSnapshot) int {
	count := 0
	for _, participant := range snapshot.Participants {
		if isDesignRole(participant.Role) {
			count++
		}
	}
	return count
}

func decodeStructured(output string, target any) error {
	text := strings.TrimSpace(output)
	if strings.HasPrefix(text, "```") {
		firstNewline := strings.IndexByte(text, '\n')
		lastFence := strings.LastIndex(text, "```")
		if firstNewline >= 0 && lastFence > firstNewline {
			text = strings.TrimSpace(text[firstNewline+1 : lastFence])
		}
	}
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start < 0 || end < start {
		return fmt.Errorf("Provider output does not contain a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(text[start : end+1]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func normalizeStatement(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

type Error struct {
	Code   string
	Detail string
}

func (e *Error) Error() string { return e.Detail }

func newError(code, detail string) error { return &Error{Code: code, Detail: detail} }

func ResultDigest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
