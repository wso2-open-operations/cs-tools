// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const acTestCaseID = "92000000-0000-0000-0000-000000000001"

type fakeAutoclosureRepo struct {
	candidates  []repository.AutoclosureCandidateRow
	claimWins   bool
	claims      []string
	restores    int
	events      []repository.AutoclosureEvent
	lastQuery   repository.AutoclosureCandidateQuery
	advanced    int
	countSince  time.Time
	displayName string
	quarantined int
	// ctxErrAtRestore and ctxErrAtRecord are the context's error at the moment
	// the repository was called: non-nil means the call was made on a context
	// that had already been cancelled.
	ctxErrAtRestore error
	ctxErrAtRecord  error
}

func (r *fakeAutoclosureRepo) FindCandidates(_ context.Context, q repository.AutoclosureCandidateQuery) ([]repository.AutoclosureCandidateRow, error) {
	r.lastQuery = q
	return r.candidates, nil
}
func (r *fakeAutoclosureRepo) ClaimStep(_ context.Context, _ repository.AutoclosureCandidateRow, newStep string, _ time.Time) (bool, error) {
	r.claims = append(r.claims, newStep)
	return r.claimWins, nil
}
func (r *fakeAutoclosureRepo) RestoreStep(ctx context.Context, _ repository.AutoclosureCandidateRow, _ time.Time) error {
	r.restores++
	r.ctxErrAtRestore = ctx.Err()
	return nil
}
func (r *fakeAutoclosureRepo) DisplayNameOf(_ context.Context, email string) (string, error) {
	if r.displayName == "" {
		return email, nil
	}
	return r.displayName, nil
}
func (r *fakeAutoclosureRepo) CountAdvancedSince(_ context.Context, since time.Time) (int, error) {
	r.countSince = since
	return r.advanced, nil
}
func (r *fakeAutoclosureRepo) RecordEvent(ctx context.Context, ev repository.AutoclosureEvent) error {
	r.events = append(r.events, ev)
	r.ctxErrAtRecord = ctx.Err()
	return nil
}
func (r *fakeAutoclosureRepo) CountQuarantined(context.Context, domain.AutoclosureStage, time.Time, int, int) (int, error) {
	return r.quarantined, nil
}

type fakeAutoclosureCases struct {
	comments   []domain.CreateCaseCommentRequest
	commentBy  []string
	commentErr error
	updates    []domain.UpdateCaseRequest
	updateCtx  []context.Context
	updateErr  error
	view       domain.CaseView
	// beforeComment runs just before a comment is created, to simulate the
	// request being cancelled mid-action.
	beforeComment func()
}

func (c *fakeAutoclosureCases) CreateCaseCommentAs(_ context.Context, req domain.CreateCaseCommentRequest, by string) (domain.CreateCaseCommentResponse, error) {
	if c.beforeComment != nil {
		c.beforeComment()
	}
	c.comments = append(c.comments, req)
	c.commentBy = append(c.commentBy, by)
	return domain.CreateCaseCommentResponse{}, c.commentErr
}
func (c *fakeAutoclosureCases) GetCaseByID(context.Context, string) (domain.CaseView, error) {
	return c.view, nil
}
func (c *fakeAutoclosureCases) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	c.updates = append(c.updates, req)
	c.updateCtx = append(c.updateCtx, ctx)
	return domain.UpdateCaseResponse{}, c.updateErr
}

type internalAccess struct{ AccessService }

func (internalAccess) ResolveScope(context.Context) (AccessScope, error) {
	return AccessScope{Unrestricted: true}, nil
}

type customerAccess struct{ AccessService }

func (customerAccess) ResolveScope(context.Context) (AccessScope, error) {
	return AccessScope{ProjectIDs: []string{"p"}}, nil
}

func newACService(repo *fakeAutoclosureRepo, cases *fakeAutoclosureCases) *autoclosureService {
	return &autoclosureService{
		repo: repo, cases: cases, access: internalAccess{}, systemEmail: "system@example.com",
		now: func() time.Time { return time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC) },
	}
}

func dueCandidate(state domain.CaseState, step *string) repository.AutoclosureCandidateRow {
	return repository.AutoclosureCandidateRow{
		AutoclosureCandidate: domain.AutoclosureCandidate{ID: acTestCaseID, Number: "CS1", State: state, Step: step},
		RowType:              repository.AutoclosureRowCase,
	}
}

func TestAutoclosureAdvance_StagesPostTheRightCommentAndStep(t *testing.T) {
	for _, tc := range []struct {
		name     string
		stage    domain.AutoclosureStage
		state    domain.CaseState
		from     *string
		wantStep string
		wantText string
	}{
		{"SP first", domain.AutoclosureStageFirstReminder, domain.CaseStateSolutionProposed, nil, "FIRST_COMMENT", "solution has been proposed"},
		{"AI first", domain.AutoclosureStageFirstReminder, domain.CaseStateAwaitingInfo, strp("DEFAULT"), "FIRST_COMMENT", "no further updates have been received. Please share"},
		{"SP second", domain.AutoclosureStageSecondReminder, domain.CaseStateSolutionProposed, strp("FIRST_COMMENT"), "SECOND_COMMENT", "closed off after one week"},
		{"AI second", domain.AutoclosureStageSecondReminder, domain.CaseStateAwaitingInfo, strp("FIRST_COMMENT"), "SECOND_COMMENT", "Gentle reminder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeAutoclosureRepo{claimWins: true, candidates: []repository.AutoclosureCandidateRow{dueCandidate(tc.state, tc.from)}}
			cases := &fakeAutoclosureCases{}
			resp, err := newACService(repo, cases).Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: tc.stage})
			if err != nil {
				t.Fatalf("Advance: %v", err)
			}
			if resp.Outcome != domain.AutoclosureOutcomeAdvanced || resp.ToStep == nil || *resp.ToStep != tc.wantStep {
				t.Fatalf("response = %+v, want advanced to %s", resp, tc.wantStep)
			}
			if len(cases.comments) != 1 || !strings.Contains(cases.comments[0].Content, tc.wantText) {
				t.Fatalf("comments = %+v, want one containing %q", cases.comments, tc.wantText)
			}
			if cases.commentBy[0] != "system@example.com" || cases.comments[0].Type != domain.CommentTypeComment {
				t.Errorf("comment must be a customer-visible comment by the system user, got %q / %s", cases.commentBy[0], cases.comments[0].Type)
			}
			if len(cases.updates) != 0 {
				t.Errorf("a reminder must not change the ticket's state")
			}
			if len(repo.events) != 1 || repo.events[0].Outcome != "ADVANCED" {
				t.Errorf("audit = %+v, want one ADVANCED row", repo.events)
			}
		})
	}
}

func TestAutoclosureAdvance_AwaitingInfoCloseSetsTheNonResponsivenessCode(t *testing.T) {
	repo := &fakeAutoclosureRepo{claimWins: true, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateAwaitingInfo, strp("SECOND_COMMENT"))}}
	cases := &fakeAutoclosureCases{}
	if _, err := newACService(repo, cases).Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageClose}); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if len(cases.updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(cases.updates))
	}
	u := cases.updates[0]
	if u.State == nil || *u.State != domain.CaseStateClosed {
		t.Errorf("state = %v, want closed", u.State)
	}
	if u.ResolutionCode == nil || *u.ResolutionCode != domain.CaseResolutionCodeAbruptlyClosedDueToNonResponsiveness {
		t.Errorf("an Awaiting Info close must carry the auto-closure resolution code, got %v", u.ResolutionCode)
	}
	if u.CloseNotes == nil || !strings.Contains(*u.CloseNotes, "did not receive an update") {
		t.Errorf("closeNotes = %v, want the closure text", u.CloseNotes)
	}
	if email, ok := autoclosureActorFromContext(cases.updateCtx[0]); !ok || email != "system@example.com" {
		t.Errorf("the close must be made as the system user, ctx actor = %q (%v)", email, ok)
	}
	if len(cases.comments) != 1 || !strings.Contains(cases.comments[0].Content, "did not receive an update") {
		t.Errorf("closure comment missing: %+v", cases.comments)
	}
}

func TestAutoclosureAdvance_SolutionProposedCloseKeepsItsOwnResolution(t *testing.T) {
	code := domain.CaseResolutionCodeSolvedWorkaroundProvided
	cause := domain.CaseCauseUnknown
	repo := &fakeAutoclosureRepo{claimWins: true, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateSolutionProposed, strp("SECOND_COMMENT"))}}
	cases := &fakeAutoclosureCases{view: domain.CaseView{ResolutionCode: &code, Cause: &cause}}
	if _, err := newACService(repo, cases).Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageClose}); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	u := cases.updates[0]
	if u.ResolutionCode == nil || *u.ResolutionCode != code {
		t.Errorf("a Solution Proposed close must keep the proposed resolution %v, got %v (it must never get the Awaiting Info code)", code, u.ResolutionCode)
	}
}

func TestAutoclosureAdvance_ASecondCallDoesNothing(t *testing.T) {
	// Not due any more: whatever listed the ticket is stale.
	repo := &fakeAutoclosureRepo{claimWins: true}
	cases := &fakeAutoclosureCases{}
	resp, err := newACService(repo, cases).Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageFirstReminder})
	if err != nil || resp.Outcome != domain.AutoclosureOutcomeSkipped {
		t.Fatalf("resp=%+v err=%v, want skipped", resp, err)
	}
	if len(cases.comments) != 0 || len(repo.claims) != 0 || len(repo.events) != 0 {
		t.Errorf("a skipped ticket must write nothing: comments=%d claims=%d events=%d", len(cases.comments), len(repo.claims), len(repo.events))
	}
	if repo.lastQuery.OnlyID != acTestCaseID {
		t.Errorf("Advance must re-check the ticket by its id, OnlyID = %q", repo.lastQuery.OnlyID)
	}
}

func TestAutoclosureAdvance_LosingTheClaimPostsNothing(t *testing.T) {
	repo := &fakeAutoclosureRepo{claimWins: false, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateAwaitingInfo, nil)}}
	cases := &fakeAutoclosureCases{}
	resp, err := newACService(repo, cases).Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageFirstReminder})
	if err != nil || resp.Outcome != domain.AutoclosureOutcomeSkipped {
		t.Fatalf("resp=%+v err=%v, want skipped", resp, err)
	}
	if len(cases.comments) != 0 {
		t.Errorf("the run that lost the claim posted a comment: that is the duplicate reminder this exists to prevent")
	}
}

func TestAutoclosureAdvance_AFailedActionPutsTheStepBack(t *testing.T) {
	repo := &fakeAutoclosureRepo{claimWins: true, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateSolutionProposed, nil)}}
	cases := &fakeAutoclosureCases{commentErr: errors.New("boom")}
	_, err := newACService(repo, cases).Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageFirstReminder})
	if err == nil {
		t.Fatal("want the comment failure returned")
	}
	if repo.restores != 1 {
		t.Errorf("restores = %d, want 1: a ticket marked reminded that never was would never be reminded", repo.restores)
	}
	if len(repo.events) != 1 || repo.events[0].Outcome != "FAILED" {
		t.Errorf("audit = %+v, want one FAILED row (an unclassified error repeats, so it counts toward the failure cap)", repo.events)
	}
}

func TestAutoclosureAdvance_ARefusedCloseIsRolledBackAndPostsNoClosureComment(t *testing.T) {
	repo := &fakeAutoclosureRepo{claimWins: true, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateAwaitingInfo, strp("SECOND_COMMENT"))}}
	cases := &fakeAutoclosureCases{updateErr: &apierror.ConflictError{Msg: "Cannot close this case while child case X is still open"}}
	_, err := newACService(repo, cases).Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageClose})
	if err == nil {
		t.Fatal("want the refusal returned")
	}
	if len(cases.comments) != 0 {
		t.Errorf("a \"this case is closed\" comment was posted on a case that is not closed")
	}
	if repo.restores != 1 {
		t.Errorf("restores = %d, want 1", repo.restores)
	}
}

func TestAutoclosureAdvance_ReleaseHoldOnlyMovesTheStep(t *testing.T) {
	repo := &fakeAutoclosureRepo{claimWins: true, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateSolutionProposed, strp("ON_HOLD"))}}
	cases := &fakeAutoclosureCases{}
	resp, err := newACService(repo, cases).Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageReleaseHold})
	if err != nil || resp.Outcome != domain.AutoclosureOutcomeAdvanced {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if len(repo.claims) != 1 || repo.claims[0] != "DEFAULT" {
		t.Errorf("claims = %v, want DEFAULT", repo.claims)
	}
	if len(cases.comments) != 0 || len(cases.updates) != 0 {
		t.Errorf("releasing a hold must not comment or change state")
	}
}

func TestAutoclosureTemplateOverrides(t *testing.T) {
	repo := &fakeAutoclosureRepo{claimWins: true, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateAwaitingInfo, nil)}}
	cases := &fakeAutoclosureCases{}
	_, err := newACService(repo, cases).Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{
		Stage:     domain.AutoclosureStageFirstReminder,
		Templates: map[string]string{"awaiting_info.first_reminder": "custom wording", "solution_proposed.first_reminder": "not used"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cases.comments[0].Content != "custom wording" {
		t.Errorf("content = %q, want the override", cases.comments[0].Content)
	}
}

func TestAutoclosureDefaultTemplatesCoverEveryStateAndKind(t *testing.T) {
	for _, state := range []string{"awaiting_info", "solution_proposed"} {
		for _, kind := range []string{"first_reminder", "second_reminder", "closure"} {
			if strings.TrimSpace(autoclosureDefaultTemplates[state+"."+kind]) == "" {
				t.Errorf("no default template for %s.%s", state, kind)
			}
		}
	}
}

func TestAutoclosureService_RefusesCustomersAndBadInput(t *testing.T) {
	repo := &fakeAutoclosureRepo{}
	svc := newACService(repo, &fakeAutoclosureCases{})
	svc.access = customerAccess{}
	if _, err := svc.FindCandidates(context.Background(), domain.AutoclosureCandidatesRequest{Stage: domain.AutoclosureStageClose}); err == nil {
		t.Error("a customer must not be able to list tickets due for closure")
	}
	if _, err := svc.Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageClose}); err == nil {
		t.Error("a customer must not be able to close a ticket through the workflow")
	}

	svc.access = internalAccess{}
	for name, req := range map[string]domain.AutoclosureCandidatesRequest{
		"unknown stage":      {Stage: "nope"},
		"negative interval":  {Stage: domain.AutoclosureStageClose, IntervalDays: -1},
		"huge interval":      {Stage: domain.AutoclosureStageClose, IntervalDays: 4000},
		"huge limit":         {Stage: domain.AutoclosureStageClose, Limit: 99999},
		"a state not served": {Stage: domain.AutoclosureStageClose, States: []domain.CaseState{domain.CaseStateOpen}},
	} {
		var ve *apierror.ValidationError
		if _, err := svc.FindCandidates(context.Background(), req); !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want a validation error", name, err)
		}
	}
	if _, err := svc.Advance(context.Background(), "not-a-uuid", domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageClose}); err == nil {
		t.Error("a malformed id must be refused")
	}
}

func TestAutoclosureFindCandidates_DefaultsAndBothStates(t *testing.T) {
	repo := &fakeAutoclosureRepo{}
	svc := newACService(repo, &fakeAutoclosureCases{})
	if _, err := svc.FindCandidates(context.Background(), domain.AutoclosureCandidatesRequest{Stage: domain.AutoclosureStageFirstReminder}); err != nil {
		t.Fatal(err)
	}
	q := repo.lastQuery
	if q.IntervalDays != 7 || q.Limit != 500 || len(q.States) != 2 || q.SystemEmail != "system@example.com" {
		t.Errorf("query = %+v, want 7 days, limit 500, both states and the system user", q)
	}
}

func TestAutoclosureFindCandidates_DailyLimitIsSharedAndSurvivesRetries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		advanced  int
		limit     int
		daily     int
		stage     domain.AutoclosureStage
		wantLimit int
		wantCall  bool
	}{
		{"nothing done today", 0, 0, 500, domain.AutoclosureStageFirstReminder, 500, true},
		{"a daily limit above the default page size is not halved", 0, 0, 800, domain.AutoclosureStageFirstReminder, 800, true},
		{"part of the budget used", 480, 0, 500, domain.AutoclosureStageClose, 20, true},
		{"budget used up", 500, 0, 500, domain.AutoclosureStageFirstReminder, 0, false},
		{"budget overrun by an earlier setting", 650, 0, 500, domain.AutoclosureStageSecondReminder, 0, false},
		{"a smaller per-call limit still applies", 0, 50, 500, domain.AutoclosureStageFirstReminder, 50, true},
		{"a hold release is never counted", 500, 0, 500, domain.AutoclosureStageReleaseHold, 500, true},
		{"no daily cap", 9999, 0, 0, domain.AutoclosureStageFirstReminder, 500, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeAutoclosureRepo{advanced: tc.advanced}
			svc := newACService(repo, &fakeAutoclosureCases{})
			resp, err := svc.FindCandidates(context.Background(), domain.AutoclosureCandidatesRequest{Stage: tc.stage, Limit: tc.limit, DailyLimit: tc.daily})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Candidates == nil {
				t.Error("candidates must serialise as [], not null")
			}
			if tc.wantCall && repo.lastQuery.Limit != tc.wantLimit {
				t.Errorf("limit asked of the repository = %d, want %d", repo.lastQuery.Limit, tc.wantLimit)
			}
			if !tc.wantCall && repo.lastQuery.Limit != 0 {
				t.Errorf("the repository was queried with limit %d although the budget is spent", repo.lastQuery.Limit)
			}
			if tc.daily > 0 && tc.stage != domain.AutoclosureStageReleaseHold {
				// The budget is per UTC day: counting starts at 00:00 UTC of the clock's day.
				if want := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC); !repo.countSince.Equal(want) {
					t.Errorf("counted from %v, want %v", repo.countSince, want)
				}
			}
		})
	}
}

// --- Pending Verification queueing after an auto-close ---------------------

type fakeVerificationQueue struct {
	enabled     bool
	enabledErr  error
	lookupErr   error
	createErr   error
	created     []domain.CreatePendingVerificationRequest
	lookedUp    []string
	settingRead int
}

func (q *fakeVerificationQueue) GetWorkItemTypeAndProject(_ context.Context, id string) (string, string, *string, error) {
	q.lookedUp = append(q.lookedUp, id)
	return "CASE", "project-1", nil, q.lookupErr
}
func (q *fakeVerificationQueue) GetProjectVerificationEnabled(context.Context, string) (bool, error) {
	q.settingRead++
	return q.enabled, q.enabledErr
}
func (q *fakeVerificationQueue) Create(_ context.Context, req domain.CreatePendingVerificationRequest) (domain.PendingVerification, error) {
	q.created = append(q.created, req)
	return domain.PendingVerification{}, q.createErr
}

func closeWith(t *testing.T, q *fakeVerificationQueue, state domain.CaseState, rowType repository.AutoclosureRowType, displayName ...string) (*fakeAutoclosureRepo, *fakeAutoclosureCases, error) {
	t.Helper()
	cand := dueCandidate(state, strp("SECOND_COMMENT"))
	cand.RowType = rowType
	repo := &fakeAutoclosureRepo{claimWins: true, candidates: []repository.AutoclosureCandidateRow{cand}}
	if len(displayName) > 0 {
		repo.displayName = displayName[0]
	}
	cases := &fakeAutoclosureCases{}
	svc := newACService(repo, cases)
	if q != nil {
		svc.verification = q
	}
	_, err := svc.Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageClose})
	return repo, cases, err
}

func TestAutoclosureClose_QueuesTheCaseForCustomerVerification(t *testing.T) {
	for state, want := range map[domain.CaseState]domain.PendingVerificationPreviousStatus{
		domain.CaseStateAwaitingInfo:     domain.PendingVerificationPreviousStatusAwaitingInfo,
		domain.CaseStateSolutionProposed: domain.PendingVerificationPreviousStatusSolutionProposed,
	} {
		t.Run(string(state), func(t *testing.T) {
			q := &fakeVerificationQueue{enabled: true}
			repo, _, err := closeWith(t, q, state, repository.AutoclosureRowCase, "Support Team (auto-closure)")
			if err != nil {
				t.Fatalf("Advance: %v", err)
			}
			if len(q.created) != 1 {
				t.Fatalf("queue entries = %d, want 1", len(q.created))
			}
			got := q.created[0]
			if got.WorkItemID != acTestCaseID || got.AddedReason != domain.PendingVerificationAddedReasonAutoClosed {
				t.Errorf("entry = %+v, want an AUTO_CLOSED entry for the case", got)
			}
			if got.PreviousStatus == nil || *got.PreviousStatus != want {
				t.Errorf("previousStatus = %v, want %s (the state it was closed from)", got.PreviousStatus, want)
			}
			if got.AddedBy != "Support Team (auto-closure)" {
				t.Errorf("addedBy = %q, want the system user's display name: the customer portal prints it as stored", got.AddedBy)
			}
			if got.Note == nil || !strings.Contains(*got.Note, "Automatically closed") {
				t.Errorf("note = %v, want the explanation customers see on the card", got.Note)
			}
			if len(repo.events) != 1 || !strings.Contains(repo.events[0].Detail, "queued for customer verification") {
				t.Errorf("audit = %+v, want the queueing noted", repo.events)
			}
		})
	}
}

func TestAutoclosureClose_DoesNotQueueWhatTheFeatureDoesNotCover(t *testing.T) {
	// A security report analysis is not a Case: the feature accepts Cases only.
	q := &fakeVerificationQueue{enabled: true}
	if _, _, err := closeWith(t, q, domain.CaseStateAwaitingInfo, repository.AutoclosureRowSRA); err != nil {
		t.Fatal(err)
	}
	if len(q.lookedUp) != 0 || len(q.created) != 0 {
		t.Errorf("a security report analysis was looked up or queued: %+v", q)
	}
	// A project that has not turned the feature on is left alone, quietly.
	q = &fakeVerificationQueue{enabled: false}
	repo, _, err := closeWith(t, q, domain.CaseStateAwaitingInfo, repository.AutoclosureRowCase)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.created) != 0 {
		t.Errorf("queued on a project with the feature off")
	}
	if repo.events[0].Detail != "" {
		t.Errorf("audit detail = %q, want none for a project that does not use the feature", repo.events[0].Detail)
	}
	// No queue wired at all (a deployment without the feature): the close is unaffected.
	if _, cases, err := closeWith(t, nil, domain.CaseStateAwaitingInfo, repository.AutoclosureRowCase); err != nil || len(cases.updates) != 1 {
		t.Errorf("close without a queue: err=%v updates=%d", err, len(cases.updates))
	}
}

func TestAutoclosureClose_AQueueProblemNeverUndoesTheClose(t *testing.T) {
	for name, q := range map[string]*fakeVerificationQueue{
		"already queued":     {enabled: true, createErr: &apierror.ConflictError{Msg: "this case already has an unverified pending verification entry"}},
		"create fails":       {enabled: true, createErr: errors.New("boom")},
		"case lookup fails":  {enabled: true, lookupErr: errors.New("boom")},
		"setting read fails": {enabled: true, enabledErr: errors.New("boom")},
	} {
		t.Run(name, func(t *testing.T) {
			repo, cases, err := closeWith(t, q, domain.CaseStateSolutionProposed, repository.AutoclosureRowCase)
			if err != nil {
				t.Fatalf("a queue problem failed the close: %v", err)
			}
			if len(cases.updates) != 1 || len(cases.comments) != 1 {
				t.Errorf("the case was not fully closed: updates=%d comments=%d", len(cases.updates), len(cases.comments))
			}
			if repo.restores != 0 || repo.events[0].Outcome != "ADVANCED" {
				t.Errorf("the close was rolled back or recorded as failed: restores=%d audit=%+v", repo.restores, repo.events)
			}
		})
	}
}

func TestAutoclosureReminders_NeverTouchTheQueue(t *testing.T) {
	q := &fakeVerificationQueue{enabled: true}
	repo := &fakeAutoclosureRepo{claimWins: true, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateAwaitingInfo, nil)}}
	svc := newACService(repo, &fakeAutoclosureCases{})
	svc.verification = q
	if _, err := svc.Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageFirstReminder}); err != nil {
		t.Fatal(err)
	}
	if len(q.lookedUp) != 0 || len(q.created) != 0 {
		t.Errorf("a reminder touched the verification queue: %+v", q)
	}
}

func TestAutoclosureClose_AddedByFallsBackToTheEmailWhenTheNameIsUnavailable(t *testing.T) {
	q := &fakeVerificationQueue{enabled: true}
	if _, _, err := closeWith(t, q, domain.CaseStateAwaitingInfo, repository.AutoclosureRowCase); err != nil {
		t.Fatal(err)
	}
	if len(q.created) != 1 || q.created[0].AddedBy != "system@example.com" {
		t.Errorf("entries = %+v, want addedBy to fall back to the system user's email", q.created)
	}
}

// --- Review follow-ups: daily limit in Advance, dry run, cleanup after cancel, failure cap

func TestAutoclosureAdvance_TheDailyLimitIsCheckedAgainRightBeforeTheClaim(t *testing.T) {
	for _, tc := range []struct {
		name    string
		done    int
		limit   int
		stage   domain.AutoclosureStage
		wantRun bool
	}{
		{"room left", 499, 500, domain.AutoclosureStageFirstReminder, true},
		{"exactly spent", 500, 500, domain.AutoclosureStageFirstReminder, false},
		{"overrun by a racing run", 612, 500, domain.AutoclosureStageClose, false},
		{"no cap", 99999, 0, domain.AutoclosureStageFirstReminder, true},
		{"a hold release is never counted", 500, 500, domain.AutoclosureStageReleaseHold, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeAutoclosureRepo{claimWins: true, advanced: tc.done, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateAwaitingInfo, nil)}}
			cases := &fakeAutoclosureCases{}
			resp, err := newACService(repo, cases).Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: tc.stage, DailyLimit: tc.limit})
			if err != nil {
				t.Fatal(err)
			}
			ran := resp.Outcome == domain.AutoclosureOutcomeAdvanced
			if ran != tc.wantRun {
				t.Fatalf("outcome = %s, want advanced=%v", resp.Outcome, tc.wantRun)
			}
			if !tc.wantRun && (len(repo.claims) != 0 || len(cases.comments) != 0 || len(cases.updates) != 0) {
				t.Errorf("a ticket over the limit was claimed or acted on: claims=%v comments=%d", repo.claims, len(cases.comments))
			}
		})
	}
}

func TestAutoclosureAdvance_DryRunDecidesButChangesNothing(t *testing.T) {
	for _, stage := range []domain.AutoclosureStage{
		domain.AutoclosureStageReleaseHold, domain.AutoclosureStageFirstReminder,
		domain.AutoclosureStageSecondReminder, domain.AutoclosureStageClose,
	} {
		t.Run(string(stage), func(t *testing.T) {
			q := &fakeVerificationQueue{enabled: true}
			repo := &fakeAutoclosureRepo{claimWins: true, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateAwaitingInfo, strp2("SECOND_COMMENT"))}}
			cases := &fakeAutoclosureCases{}
			svc := newACService(repo, cases)
			svc.verification = q
			resp, err := svc.Advance(context.Background(), acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: stage, DryRun: true, DailyLimit: 500})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Outcome != domain.AutoclosureOutcomeDryRun || !strings.Contains(resp.Reason, "dry run: would") {
				t.Fatalf("response = %+v, want a dry_run that says what it would do", resp)
			}
			if len(repo.claims) != 0 || repo.restores != 0 {
				t.Errorf("a dry run claimed or restored a step: %v / %d", repo.claims, repo.restores)
			}
			if len(cases.comments) != 0 || len(cases.updates) != 0 {
				t.Errorf("a dry run posted or closed: comments=%d updates=%d", len(cases.comments), len(cases.updates))
			}
			if len(q.lookedUp) != 0 || len(q.created) != 0 {
				t.Errorf("a dry run touched the verification queue: %+v", q)
			}
			if len(repo.events) != 1 || repo.events[0].Outcome != "DRY_RUN" {
				t.Errorf("audit = %+v, want exactly one DRY_RUN row", repo.events)
			}
		})
	}
}

func TestAutoclosureAdvance_DryRunStillAppliesTheDailyLimitAndTheDueCheck(t *testing.T) {
	// Over the limit: a dry run reports it as a real run would, and records nothing.
	repo := &fakeAutoclosureRepo{claimWins: true, advanced: 500, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateAwaitingInfo, nil)}}
	resp, err := newACService(repo, &fakeAutoclosureCases{}).Advance(context.Background(), acTestCaseID,
		domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageFirstReminder, DryRun: true, DailyLimit: 500})
	if err != nil || resp.Outcome != domain.AutoclosureOutcomeSkipped || len(repo.events) != 0 {
		t.Fatalf("over the limit: resp=%+v err=%v events=%d, want skipped with no audit row", resp, err, len(repo.events))
	}
	// Not due: skipped, exactly as a real run would.
	repo = &fakeAutoclosureRepo{claimWins: true}
	resp, err = newACService(repo, &fakeAutoclosureCases{}).Advance(context.Background(), acTestCaseID,
		domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageFirstReminder, DryRun: true})
	if err != nil || resp.Outcome != domain.AutoclosureOutcomeSkipped || len(repo.events) != 0 {
		t.Fatalf("not due: resp=%+v err=%v events=%d", resp, err, len(repo.events))
	}
}

func strp2(s string) *string { return &s }

// A request cancelled between the claim and the action (the caller's timeout)
// must still put the step back and record the failure: those writes run on a
// context detached from the cancellation.
func TestAutoclosureAdvance_CleanupSurvivesACancelledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := &fakeAutoclosureRepo{claimWins: true, candidates: []repository.AutoclosureCandidateRow{dueCandidate(domain.CaseStateSolutionProposed, nil)}}
	cases := &fakeAutoclosureCases{}
	cases.beforeComment = func() { cancel() } // the caller gives up mid-action
	cases.commentErr = context.Canceled
	_, err := newACService(repo, cases).Advance(ctx, acTestCaseID, domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageFirstReminder})
	if err == nil {
		t.Fatal("want the cancellation returned")
	}
	if repo.restores != 1 {
		t.Fatalf("restores = %d, want the claimed step put back even though the request was cancelled", repo.restores)
	}
	if repo.ctxErrAtRestore != nil || repo.ctxErrAtRecord != nil {
		t.Errorf("cleanup ran on a cancelled context (restore=%v record=%v): it would have failed in production", repo.ctxErrAtRestore, repo.ctxErrAtRecord)
	}
	if len(repo.events) != 1 || repo.events[0].Outcome != "FAILED_TRANSIENT" {
		t.Errorf("audit = %+v, want one FAILED_TRANSIENT row (a cancelled request says nothing about the ticket, so it must not count toward the failure cap)", repo.events)
	}
}

func TestAutoclosureFindCandidates_AsksForTheFailureCapAndSaysWhenItSkips(t *testing.T) {
	repo := &fakeAutoclosureRepo{quarantined: 4}
	if _, err := newACService(repo, &fakeAutoclosureCases{}).FindCandidates(context.Background(),
		domain.AutoclosureCandidatesRequest{Stage: domain.AutoclosureStageClose}); err != nil {
		t.Fatal(err)
	}
	if repo.lastQuery.MaxFailures != autoclosureMaxFailures || repo.lastQuery.FailureWindowDays != autoclosureFailureWindowDays {
		t.Errorf("query = %+v, want the failure cap (%d in %d days) applied", repo.lastQuery, autoclosureMaxFailures, autoclosureFailureWindowDays)
	}
}

func TestAutoclosureAdvance_RecheckAppliesTheFailureCapToo(t *testing.T) {
	repo := &fakeAutoclosureRepo{claimWins: true}
	if _, err := newACService(repo, &fakeAutoclosureCases{}).Advance(context.Background(), acTestCaseID,
		domain.AutoclosureAdvanceRequest{Stage: domain.AutoclosureStageClose}); err != nil {
		t.Fatal(err)
	}
	if repo.lastQuery.MaxFailures != autoclosureMaxFailures {
		t.Errorf("the per-ticket re-check does not apply the failure cap: %+v", repo.lastQuery)
	}
}

func TestFailureOutcome_UnclassifiedErrorsCountTowardTheCap(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"validation", &apierror.ValidationError{Msg: "no cause"}, "FAILED"},
		{"conflict", &apierror.ConflictError{Msg: "open child"}, "FAILED"},
		{"forbidden", &apierror.ForbiddenError{Msg: "no"}, "FAILED"},
		{"not found", &apierror.NotFoundError{Msg: "gone"}, "FAILED"},
		{"wrapped refusal", fmt.Errorf("close: %w", &apierror.ConflictError{Msg: "open child"}), "FAILED"},
		// Untyped: a constraint violation or an internal error repeats every run.
		{"unclassified", errors.New("close ticket: something broke"), "FAILED"},
		{"constraint violation", &pgconn.PgError{Code: "23514"}, "FAILED"},
		{"wrapped constraint violation", fmt.Errorf("close ticket: %w", &pgconn.PgError{Code: "23503"}), "FAILED"},
		{"cancelled", context.Canceled, "FAILED_TRANSIENT"},
		{"wrapped timeout", fmt.Errorf("close ticket: %w", context.DeadlineExceeded), "FAILED_TRANSIENT"},
		{"outage", &apierror.ServiceUnavailableError{Msg: "down"}, "FAILED_TRANSIENT"},
		{"dropped connection", io.ErrUnexpectedEOF, "FAILED_TRANSIENT"},
		{"network", &net.OpError{Op: "dial", Err: errors.New("refused")}, "FAILED_TRANSIENT"},
		{"db connection exception", &pgconn.PgError{Code: "08006"}, "FAILED_TRANSIENT"},
		{"db shutting down", &pgconn.PgError{Code: "57P01"}, "FAILED_TRANSIENT"},
		{"db out of connections", &pgconn.PgError{Code: "53300"}, "FAILED_TRANSIENT"},
	} {
		if got := failureOutcome(tc.err); got != tc.want {
			t.Errorf("%s: outcome = %s, want %s", tc.name, got, tc.want)
		}
	}
}
