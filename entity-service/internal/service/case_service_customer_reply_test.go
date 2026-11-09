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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// These tests pin caseService.applyCustomerReplyStateTransition, the port of
// snCaseService's rule of the same name to the Postgres data source: a
// customer's comment on a case-like record that is Awaiting Info or Solution
// Proposed moves it to Waiting on WSO2 through the service's own UpdateCase.
// Every test drives the public CreateCaseComment path with the recording
// repository, so what is asserted is what the rule actually asks UpdateCase
// for, and the activity entry / status-changed event / SLA effect / mirror
// that UpdateCase then produces -- not the rule's internals.

const customerReplyCustomerEmail = "dave.mendis@example.com"

// customerReplyExternalAccess is the AccessService of a registered portal
// contact posting through the customer portal: a project-scoped identity
// that is neither Unrestricted nor HasInternalAccess -- exactly what
// repository.IsExternalCaller calls an external caller.
var customerReplyExternalAccess = stubAccess{scope: AccessScope{ProjectIDs: []string{"proj-1"}, ViewerEmail: customerReplyCustomerEmail, ViaCustomerPortal: true}}

// recordedActivity is one RecordCaseFieldChangeActivity call.
type recordedActivity struct {
	field, oldValue, newValue, actor string
}

// customerReplyFixture is the recording repository behind every test here:
// the record's current state and type are what getCaseByID answers, and the
// moves/activity slices are what UpdateCase was asked for.
type customerReplyFixture struct {
	repo     *stubCaseRepo
	userRepo stubUserRepo
	mu       sync.Mutex
	moves    []domain.CaseState
	activity []recordedActivity
}

func newCustomerReplyFixture(caseType string, state domain.CaseState) *customerReplyFixture {
	f := &customerReplyFixture{}
	f.repo = &stubCaseRepo{
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			return domain.CaseComment{ID: "comment-1", CaseID: req.CaseID, Type: req.Type, Content: req.Content}, nil
		},
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			current, typ := state, caseType
			return domain.CaseView{
				ID: testDeploymentUUID, Number: "CS0001", InternalID: "WSO2-CS-1", Subject: "s",
				Type: &typ, State: &current,
				ProjectDetails: &domain.EntityRef{ID: "proj-1", Name: "Project One"},
				// Someone to notify, or neither case.* event is published.
				WatchList: []domain.WatchListUser{{Email: "watcher@example.com"}},
			}, nil
		},
		updateCase: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if req.State != nil {
				f.moves = append(f.moves, *req.State)
			}
			return domain.Case{ID: req.ID, State: req.State}, nil, nil
		},
		recordCaseFieldChangeActivity: func(_ context.Context, _, field, oldValue, newValue, actor string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.activity = append(f.activity, recordedActivity{field, oldValue, newValue, actor})
			return nil
		},
	}
	f.userRepo = stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
		return domain.User{ID: "user-customer", Email: email, FirstName: "Dave", LastName: "Mendis"}, nil
	}}
	return f
}

func (f *customerReplyFixture) movesTo() []domain.CaseState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.CaseState(nil), f.moves...)
}

func (f *customerReplyFixture) activities() []recordedActivity {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedActivity(nil), f.activity...)
}

func customerReplyRequest(commentType domain.CommentType) domain.CreateCaseCommentRequest {
	return domain.CreateCaseCommentRequest{CaseID: testDeploymentUUID, Type: commentType, Content: "Here is the log you asked for"}
}

// TestCaseService_CreateCaseComment_CustomerReplyMovesToWaitingOnWSO2 is the
// rule's own table: one row per (record type, current state). A CASE and a
// SERVICE_REQUEST sit in the same table, on the same code path, with the
// same expectations -- the rule carries no type-specific logic, and this is
// what proves there is no divergence between the two. Only Awaiting Info and
// Solution Proposed move; every other state, Closed included, stays.
func TestCaseService_CreateCaseComment_CustomerReplyMovesToWaitingOnWSO2(t *testing.T) {
	states := []struct {
		state domain.CaseState
		label string
		moves bool
	}{
		{domain.CaseStateAwaitingInfo, "Awaiting Info", true},
		{domain.CaseStateSolutionProposed, "Solution Proposed", true},
		{domain.CaseStateOpen, "Open", false},
		{domain.CaseStateWorkInProgress, "Work In Progress", false},
		{domain.CaseStateReopened, "Reopened", false},
		{domain.CaseStateWaitingOnWSO2, "Waiting on WSO2", false},
		{domain.CaseStateClosed, "Closed", false},
	}
	for _, caseType := range []string{"case", "service_request"} {
		for _, tc := range states {
			t.Run(caseType+"/"+string(tc.state), func(t *testing.T) {
				f := newCustomerReplyFixture(caseType, tc.state)
				publisher := &mockEventPublisher{}
				svc := NewCaseService(f.repo, f.userRepo, publisher, customerReplyExternalAccess, nil)
				ctx := contextWithUserIDToken(fakeJWTWithEmail(t, customerReplyCustomerEmail))

				resp, err := svc.CreateCaseComment(ctx, customerReplyRequest(domain.CommentTypeComment))
				if err != nil {
					t.Fatalf("CreateCaseComment: %v", err)
				}
				if resp.Comment.ID != "comment-1" || resp.Comment.CreatedBy != customerReplyCustomerEmail {
					t.Errorf("comment response = %+v, want the written comment unchanged", resp.Comment)
				}

				moves := f.movesTo()
				if !tc.moves {
					if len(moves) != 0 {
						t.Fatalf("UpdateCase asked for %v from %s, want no move", moves, tc.state)
					}
					if acts := f.activities(); len(acts) != 0 {
						t.Errorf("activity recorded %v from %s, want none", acts, tc.state)
					}
					if _, published := findPublishCall(publisher.calls, events.TypeStatusChanged); published {
						t.Errorf("case.status_changed published from %s, want none", tc.state)
					}
					return
				}
				if len(moves) != 1 || moves[0] != domain.CaseStateWaitingOnWSO2 {
					t.Fatalf("UpdateCase asked for %v, want exactly one move to waiting_on_wso2", moves)
				}
				acts := f.activities()
				want := recordedActivity{"state", tc.label, "Waiting on WSO2", customerReplyCustomerEmail}
				if len(acts) != 1 || acts[0] != want {
					t.Errorf("activity = %+v, want [%+v] (the lifecycle entry a manual move writes, with the customer as actor)", acts, want)
				}
				call, published := findPublishCall(publisher.calls, events.TypeStatusChanged)
				if !published {
					t.Fatalf("case.status_changed not published; published %v", publishedTypes(publisher.calls))
				}
				var payload events.StatusChangedPayload
				if err := json.Unmarshal(call.payload, &payload); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				if payload.NewStatus != "Waiting on WSO2" {
					t.Errorf("NewStatus = %q, want %q", payload.NewStatus, "Waiting on WSO2")
				}
				// comment_added still goes out first, then status_changed --
				// the order snCaseService.CreateCaseComment produces.
				if types := publishedTypes(publisher.calls); len(types) != 2 || types[0] != events.TypeCommentAdded || types[1] != events.TypeStatusChanged {
					t.Errorf("published %v, want [case.comment_added case.status_changed]", types)
				}
			})
		}
	}
}

// TestCaseService_CreateCaseComment_CustomerWorkNoteDoesNotMove: the rule is
// for a customer-visible reply only. A work note from the same external
// caller (were one ever written) leaves an Awaiting Info record alone.
func TestCaseService_CreateCaseComment_CustomerWorkNoteDoesNotMove(t *testing.T) {
	f := newCustomerReplyFixture("case", domain.CaseStateAwaitingInfo)
	svc := NewCaseService(f.repo, f.userRepo, nil, customerReplyExternalAccess, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, customerReplyCustomerEmail))

	if _, err := svc.CreateCaseComment(ctx, customerReplyRequest(domain.CommentTypeWorkNote)); err != nil {
		t.Fatalf("CreateCaseComment: %v", err)
	}
	if moves := f.movesTo(); len(moves) != 0 {
		t.Fatalf("UpdateCase asked for %v after a work note, want no move", moves)
	}
}

// TestCaseService_CreateCaseComment_InternalCallerCommentDoesNotMove: WSO2
// staff (an Unrestricted scope, or a mixed identity whose data scope is
// project-bound but who still has internal access), the system identity and
// an M2M client never trigger the rule, even with a customer-visible comment
// on an Awaiting Info record.
func TestCaseService_CreateCaseComment_InternalCallerCommentDoesNotMove(t *testing.T) {
	cases := []struct {
		name   string
		access AccessService
		asM2M  bool
	}{
		{"staff, unrestricted", alwaysUnrestrictedAccess{}, false},
		{"staff, mixed identity with internal access", stubAccess{scope: AccessScope{ProjectIDs: []string{"proj-1"}, ViewerEmail: "mixed@example.com", HasInternalAccess: true}}, false},
		{"M2M client, CreateCaseCommentAs", alwaysUnrestrictedAccess{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newCustomerReplyFixture("case", domain.CaseStateAwaitingInfo)
			publisher := &mockEventPublisher{}
			svc := NewCaseService(f.repo, f.userRepo, publisher, tc.access, nil)

			var err error
			if tc.asM2M {
				_, err = svc.CreateCaseCommentAs(context.Background(), customerReplyRequest(domain.CommentTypeComment), "svc-account@example.com")
			} else {
				_, err = svc.CreateCaseComment(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")), customerReplyRequest(domain.CommentTypeComment))
			}
			if err != nil {
				t.Fatalf("create comment: %v", err)
			}
			if moves := f.movesTo(); len(moves) != 0 {
				t.Fatalf("UpdateCase asked for %v, want no move for an internal caller", moves)
			}
			if _, published := findPublishCall(publisher.calls, events.TypeStatusChanged); published {
				t.Errorf("case.status_changed published for an internal caller's comment")
			}
		})
	}
}

// TestCaseService_CreateCaseComment_CustomerReplyMoveFailureIsLoggedAndTheCommentSucceeds:
// the comment is committed before the rule runs, so a failed state read or
// a failed move is logged at ERROR with the case id and the comment is still
// returned as created -- the same posture snCaseService's version takes.
func TestCaseService_CreateCaseComment_CustomerReplyMoveFailureIsLoggedAndTheCommentSucceeds(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(f *customerReplyFixture)
		wantLog string
	}{
		{
			name: "UpdateCase fails",
			arrange: func(f *customerReplyFixture) {
				f.repo.updateCase = func(context.Context, domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
					return domain.Case{}, nil, errors.New("boom")
				}
			},
			wantLog: "move case to waiting on wso2 after customer reply failed",
		},
		{
			name: "reading the current state fails",
			arrange: func(f *customerReplyFixture) {
				f.repo.getCaseByID = func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
					return domain.CaseView{}, errors.New("boom")
				}
			},
			wantLog: "customer reply state transition not evaluated, get case failed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newCustomerReplyFixture("service_request", domain.CaseStateSolutionProposed)
			tc.arrange(f)
			logs := captureSlog(t)
			svc := NewCaseService(f.repo, f.userRepo, nil, customerReplyExternalAccess, nil)
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, customerReplyCustomerEmail))

			resp, err := svc.CreateCaseComment(ctx, customerReplyRequest(domain.CommentTypeComment))
			if err != nil {
				t.Fatalf("CreateCaseComment returned %v, want the comment to succeed regardless of the move", err)
			}
			if resp.Comment.ID != "comment-1" {
				t.Errorf("comment id = %q, want comment-1", resp.Comment.ID)
			}
			out := logs.String()
			if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, tc.wantLog) || !strings.Contains(out, "caseId="+testDeploymentUUID) {
				t.Errorf("want an ERROR log line %q carrying caseId=%s, got:\n%s", tc.wantLog, testDeploymentUUID, out)
			}
		})
	}
}

// TestCaseService_CreateCaseComment_UnresolvedIdentityDoesNotMove: a caller
// whose scope cannot be resolved is not confirmed as a customer, whatever
// roles the author holds -- the rule has no CUSTOMER_ROLES fallback on this
// data source (see isCustomerCommentAuthor), and nothing below it could act
// without an identity anyway. The comment itself is still written.
func TestCaseService_CreateCaseComment_UnresolvedIdentityDoesNotMove(t *testing.T) {
	f := newCustomerReplyFixture("case", domain.CaseStateAwaitingInfo)
	f.userRepo.getUserRoles = func(context.Context, string) ([]string, error) { return []string{"customer"}, nil }
	svc := NewCaseService(f.repo, f.userRepo, nil, erroringAccess{err: errors.New("no verified identity")}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, customerReplyCustomerEmail))

	resp, err := svc.CreateCaseComment(ctx, customerReplyRequest(domain.CommentTypeComment))
	if err != nil {
		t.Fatalf("CreateCaseComment: %v", err)
	}
	if resp.Comment.ID != "comment-1" {
		t.Errorf("comment id = %q, want comment-1", resp.Comment.ID)
	}
	if moves := f.movesTo(); len(moves) != 0 {
		t.Fatalf("UpdateCase asked for %v, want no move for an unresolved identity", moves)
	}
}

// TestCaseService_CreateCaseComment_CustomerReplyMoveIsMirroredAndAppliesSLAEffects
// proves the move goes through UpdateCase and not around it: under
// dual-write the one state write reaches the previous-system mirror through the
// same bare patchCaseFields a manual PATCH uses, and the SLA engine gets the
// same ApplyCaseStateEffects(waiting_on_wso2) -- resuming the clocks Awaiting
// Info had paused -- with no logic of the rule's own for either.
func TestCaseService_CreateCaseComment_CustomerReplyMoveIsMirroredAndAppliesSLAEffects(t *testing.T) {
	f := newCustomerReplyFixture("service_request", domain.CaseStateAwaitingInfo)
	var mu sync.Mutex
	var mirrored []domain.CaseState
	stateMirrored := make(chan struct{}, 1)
	mirror := &stubMirrorCaseService{
		createBareCaseComment: func(context.Context, string, domain.CommentType, string) (domain.CaseCommentDetail, error) {
			return domain.CaseCommentDetail{}, nil
		},
		patchCaseFieldsFn: func(_ context.Context, _ string, state *domain.CaseState, severity *domain.CaseSeverity, workState *domain.CaseWorkState, _ *bool, resolution *caseResolutionFields, _ bool) (domain.UpdatedCase, error) {
			mu.Lock()
			defer mu.Unlock()
			if severity != nil || workState != nil || resolution != nil {
				t.Errorf("mirror got severity=%v workState=%v resolution=%v, want only a state", severity, workState, resolution)
			}
			if state != nil {
				mirrored = append(mirrored, *state)
			}
			stateMirrored <- struct{}{}
			return domain.UpdatedCase{}, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	slaEngine := &fakeSLAEngineService{}
	svc := NewCaseServiceWithSNWriteback(f.repo, f.userRepo, nil, customerReplyExternalAccess, nil, NewSNWritebackDispatcher(failures), mirror, slaEngine, "")
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, customerReplyCustomerEmail))

	if _, err := svc.CreateCaseComment(ctx, customerReplyRequest(domain.CommentTypeComment)); err != nil {
		t.Fatalf("CreateCaseComment: %v", err)
	}
	select {
	case <-stateMirrored:
	case <-time.After(2 * time.Second):
		t.Fatal("the state move was never mirrored to the previous system")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(mirrored) != 1 || mirrored[0] != domain.CaseStateWaitingOnWSO2 {
		t.Errorf("mirrored states = %v, want [waiting_on_wso2]", mirrored)
	}
	if got := failures.count(); got != 0 {
		t.Errorf("sn_writeback_failures recorded %d, want 0", got)
	}
	if len(slaEngine.stateCalls) != 1 || slaEngine.stateCalls[0] != (fakeSLAStateCall{testDeploymentUUID, domain.CaseStateWaitingOnWSO2}) {
		t.Errorf("SLA state effects = %+v, want one ApplyCaseStateEffects(%s, waiting_on_wso2)", slaEngine.stateCalls, testDeploymentUUID)
	}
}

func TestCaseService_CreateCaseComment_LinksTheImagesItShows(t *testing.T) {
	f := newCustomerReplyFixture("case", domain.CaseStateOpen)
	linker := &recordingLinker{}
	svc := WithInlineImageLinker(NewCaseService(f.repo, f.userRepo, &mockEventPublisher{}, customerReplyExternalAccess, nil), linker)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, customerReplyCustomerEmail))
	req := customerReplyRequest(domain.CommentTypeComment)
	req.Content = `<img src="/attachments/inline/11111111-1111-4111-8111-111111111111/content">`

	if _, err := svc.CreateCaseComment(ctx, req); err != nil {
		t.Fatalf("CreateCaseComment: %v", err)
	}
	if len(linker.calls) != 1 || linker.calls[0].comment != "comment-1" || linker.calls[0].workItem != req.CaseID ||
		linker.calls[0].author != customerReplyCustomerEmail || linker.calls[0].content != req.Content {
		t.Errorf("linker calls = %+v, want one call for the saved comment, its case and its author", linker.calls)
	}
}
