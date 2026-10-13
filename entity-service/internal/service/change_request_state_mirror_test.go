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
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The one post-commit rule of the dual-write data source: after the repository
// commits, when change_request.state differs from the state the transaction
// found, one job for that move is dispatched to the previous system after the
// fields the same call mirrored; when it does not differ, nothing is -- whoever
// the caller and whatever the request named. The job obeys the previous
// system's change model (snChangeModelMoves): it reads the record, writes a
// state only where the model allows it from THEIR state, sends Request
// Approval as requestApproval, leaves the approval cascades and the customer's
// outcomes to their engine and reads those back, and records every move it
// cannot send as a visible failure. One test per rule, each asserting exactly
// what the mirror was asked to write, in order, and what was recorded.

// mirrorRecorder is the mirror to the previous system, recording every call
// in the order the dispatcher ran them (one record's writes are serialized).
// snState is the state its record reads back ("" = none), as the enum string
// snCRStateLabelToString reports.
type mirrorRecorder struct {
	ChangeRequestService
	mu        sync.Mutex
	patches   []domain.PatchChangeRequestRequest
	decisions []string
	// patchErr / decisionErr / getErr make the next calls fail (the write-back
	// failure path).
	patchErr    error
	decisionErr error
	getErr      error
	snState     string
	reads       int
	// log is every call as "patch:{...}" / "decision:approved", in order.
	log []string
}

func (m *mirrorRecorder) GetChangeRequest(_ context.Context, id string) (domain.ChangeRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads++
	if m.getErr != nil {
		return domain.ChangeRequest{}, m.getErr
	}
	cr := domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}
	if m.snState != "" {
		st := m.snState
		cr.State = &st
	}
	return cr, nil
}

// setSNState moves the previous system's record (what GetChangeRequest reads).
func (m *mirrorRecorder) setSNState(state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snState = state
}

func (m *mirrorRecorder) readCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reads
}

func (m *mirrorRecorder) PatchChangeRequest(_ context.Context, _ string, req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patches = append(m.patches, req)
	b, _ := json.Marshal(req)
	m.log = append(m.log, "patch:"+string(b))
	return domain.PatchChangeRequestResponse{}, m.patchErr
}

func (m *mirrorRecorder) DecideChangeRequestApproval(_ context.Context, _ string, decision string) (domain.ChangeRequestApprovalDecisionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decisions = append(m.decisions, decision)
	m.log = append(m.log, "decision:"+decision)
	return domain.ChangeRequestApprovalDecisionResponse{}, m.decisionErr
}

func (m *mirrorRecorder) calls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.log...)
}

// settle waits until the mirror has been called n times, then a quiet period
// more (for a call that should NOT come), and returns the calls.
func (m *mirrorRecorder) settle(t *testing.T, n int) []string {
	t.Helper()
	deadline := time.Now().Add(mirrorWriteCeiling)
	for time.Now().Before(deadline) && len(m.calls()) < n {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(mirrorQuietPeriod)
	return m.calls()
}

func wantCalls(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: the mirror was asked to write\n  %s\nwant exactly\n  %s", what, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func states(prior, committed string) repository.ChangeRequestStates {
	var p, c *string
	if prior != "" {
		p = &prior
	}
	if committed != "" {
		c = &committed
	}
	return repository.ChangeRequestStates{Prior: p, Committed: c}
}

// dualWritePatchHarness is the dual-write service whose repository commits
// `committed` and reports `st` for every PATCH. The previous system's record
// starts where ours was (st.Prior) -- the two were in step before the move.
func dualWritePatchHarness(committed domain.ChangeRequest, st repository.ChangeRequestStates) (*mirrorRecorder, *recordingSNWritebackFailures, ChangeRequestService) {
	mirror := &mirrorRecorder{}
	if st.Prior != nil {
		mirror.snState = strings.ToLower(*st.Prior)
	}
	failures := &recordingSNWritebackFailures{}
	repo := &stubChangeRequestRepo{
		patchChangeRequestStates: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, repository.ChangeRequestStates, error) {
			committed.ID = id
			return committed, st, nil
		},
	}
	return mirror, failures, NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(failures))
}

func patchAs(t *testing.T, svc ChangeRequestService, ctx context.Context, req domain.PatchChangeRequestRequest) {
	t.Helper()
	if _, err := svc.PatchChangeRequest(ctx, testUUID, req); err != nil {
		t.Fatalf("PatchChangeRequest: %v", err)
	}
}

// stateWrite is the one write the state rule makes, as the recorder logs it.
func stateWrite(state string) string {
	st := domain.ChangeRequestState(state)
	return fieldsWrite(domain.PatchChangeRequestRequest{State: &st})
}

// fieldsWrite is a fields write, as the recorder logs it.
func fieldsWrite(req domain.PatchChangeRequestRequest) string {
	b, _ := json.Marshal(req)
	return "patch:" + string(b)
}

// requestApprovalWrite is the previous system's own Request Approval, as the
// recorder logs it.
func requestApprovalWrite() string {
	yes := true
	return fieldsWrite(domain.PatchChangeRequestRequest{RequestApproval: &yes})
}

// wantFailure asserts exactly one outstanding failure row, of op, whose
// reason contains every one of parts.
func wantFailure(t *testing.T, failures *recordingSNWritebackFailures, op string, parts ...string) domain.SNWritebackFailure {
	t.Helper()
	waitFor(t, func() bool { return failures.count() >= 1 })
	time.Sleep(mirrorQuietPeriod)
	rows := failures.outstanding()
	if len(rows) != 1 || rows[0].Operation != op || rows[0].EntityID != testUUID {
		t.Fatalf("failure rows = %+v, want one %q row for the change request", rows, op)
	}
	for _, p := range parts {
		if !strings.Contains(rows[0].Error, p) {
			t.Fatalf("failure reason = %q, want it to contain %q", rows[0].Error, p)
		}
	}
	return rows[0]
}

func wantNoFailure(t *testing.T, what string, failures *recordingSNWritebackFailures) {
	t.Helper()
	if n := failures.count(); n != 0 {
		t.Fatalf("%s: %d write-back failures recorded: %+v", what, n, failures.outstanding())
	}
}

// --- PatchChangeRequest: every cause of a state move through the PATCH ---

func TestChangeRequestStateMirror_StaffMoveIsWrittenWhereTheModelAllowsIt(t *testing.T) {
	// The direct writes the previous system's model accepts from the state
	// its record is in: one read, one write, nothing else.
	for _, tc := range []struct {
		name         string
		prior, after string
		req          domain.ChangeRequestState
	}{
		{"Scheduled -> Implement", "SCHEDULED", "IMPLEMENT", domain.ChangeRequestStateImplement},
		{"Implement -> Review", "IMPLEMENT", "REVIEW", domain.ChangeRequestStateReview},
		{"Review -> Closed", "REVIEW", "CLOSED", domain.ChangeRequestStateClosed},
		{"Cancel out of Scheduled", "SCHEDULED", "CANCELED", domain.ChangeRequestStateCanceled},
		{"Cancel out of Customer Approval (allowed from every non-final state)", "CUSTOMER_APPROVAL", "CANCELED", domain.ChangeRequestStateCanceled},
		{"Cancel out of Assess", "ASSESS", "CANCELED", domain.ChangeRequestStateCanceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mirror, failures, svc := dualWritePatchHarness(committedAt(strings.ToLower(tc.after), "", ""), states(tc.prior, tc.after))
			patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: &tc.req})
			wantCalls(t, tc.name, mirror.settle(t, 1), stateWrite(strings.ToLower(tc.after)))
			wantNoFailure(t, tc.name, failures)
			if mirror.readCount() != 1 {
				t.Fatalf("the record was read %d times, want once before the write", mirror.readCount())
			}
		})
	}
}

func TestChangeRequestStateMirror_RequestApprovalIsThePreviousSystemsOwnAction(t *testing.T) {
	// Request Approval on a Normal change (New -> Assess) is sent as
	// requestApproval: true -- its record moves to Assess and its policy
	// provisions its stage -- never as a state write of Assess.
	mirror, failures, svc := dualWritePatchHarness(committedAt("assess", "", ""), states("NEW", "ASSESS"))
	patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAssess)})
	wantCalls(t, "Request Approval", mirror.settle(t, 1), requestApprovalWrite())
	wantNoFailure(t, "Request Approval", failures)

	// ...and from a migrated row whose state is NULL (New for the PATCH), the
	// record of the previous system reading New as well.
	mirror, _, svc = dualWritePatchHarness(committedAt("assess", "", ""), states("", "ASSESS"))
	mirror.setSNState("new")
	patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAssess)})
	wantCalls(t, "Request Approval from a NULL state", mirror.settle(t, 1), requestApprovalWrite())

	// Request Approval on a type whose first state is written directly: the
	// model allows New -> Authorize (Emergency) and New -> Scheduled
	// (Standard) as a direct write.
	for _, tc := range []struct{ after, want string }{{"AUTHORIZE", "authorize"}, {"SCHEDULED", "scheduled"}} {
		mirror, failures, svc := dualWritePatchHarness(committedAt(tc.want, "", ""), states("NEW", tc.after))
		patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAssess)})
		wantCalls(t, "Request Approval -> "+tc.after, mirror.settle(t, 1), stateWrite(tc.want))
		wantNoFailure(t, "Request Approval -> "+tc.after, failures)
	}
	// A Standard change that needs the customer goes straight to Customer
	// Approval, which the model has no direct move into: not sent, recorded.
	mirror, failures, svc = dualWritePatchHarness(committedAt("customer_approval", "", ""), states("NEW", "CUSTOMER_APPROVAL"))
	patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAssess)})
	wantCalls(t, "Request Approval -> Customer Approval", mirror.settle(t, 0))
	wantFailure(t, failures, snWritebackOpState, `"customer_approval"`, "not mirrorable", "no direct move", `is in "new"`)
}

func TestChangeRequestStateMirror_StateWithFieldsSendsFieldsThenState(t *testing.T) {
	// A field and a state in one request: the fields first (as the previous
	// system's API takes them), then the state, two writes in that order.
	mirror, _, svc := dualWritePatchHarness(committedAt("canceled", "2030-03-08T09:00:00Z", ""), states("SCHEDULED", "CANCELED"))
	patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateCanceled), PlannedStartOn: sPtr("2030-03-08T09:00:00Z")})
	wantCalls(t, "Cancel with a window", mirror.settle(t, 2), fieldsWrite(domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00")}), stateWrite("canceled"))
}

func TestChangeRequestStateMirror_MoveTheModelRefusesIsRecordedNotSent(t *testing.T) {
	t.Run("a move the model does not allow from the state their record is in", func(t *testing.T) {
		// Ours moved Scheduled -> Implement, but their record still reads
		// Assess (their engine never resolved a stage): Assess -> Implement is
		// not in the model, so nothing is written and the failure says so.
		mirror, failures, svc := dualWritePatchHarness(committedAt("implement", "", ""), states("SCHEDULED", "IMPLEMENT"))
		mirror.setSNState("assess")
		patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateImplement)})
		wantCalls(t, "a refused move", mirror.settle(t, 0))
		row := wantFailure(t, failures, snWritebackOpState, `"implement"`, "not allowed by the previous system's change model", `"assess"`)
		if string(row.Payload) != `{"state":"implement","after":"PatchChangeRequest"}` {
			t.Fatalf("failure payload = %s", row.Payload)
		}
	})
	t.Run("a state the model has no direct move into", func(t *testing.T) {
		// Review -> Customer Review: the previous system reaches Customer
		// Review only through its own paths, which this service's client has
		// no field for; recorded rather than guessed. Rollback likewise.
		for _, tc := range []struct{ prior, after string }{{"REVIEW", "CUSTOMER_REVIEW"}, {"REVIEW", "ROLLBACK"}, {"CUSTOMER_REVIEW", "ROLLBACK"}} {
			mirror, failures, svc := dualWritePatchHarness(committedAt(strings.ToLower(tc.after), "", ""), states(tc.prior, tc.after))
			patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestState(strings.ToLower(tc.after)))})
			wantCalls(t, tc.after, mirror.settle(t, 0))
			wantFailure(t, failures, snWritebackOpState, `"`+strings.ToLower(tc.after)+`"`, "not mirrorable", "no direct move")
		}
	})
	t.Run("a state with no key at all", func(t *testing.T) {
		mirror, failures, svc := dualWritePatchHarness(committedAt("implement", "", ""), states("SCHEDULED", "SOMETHING_THE_SYNC_INVENTED"))
		patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateImplement)})
		wantCalls(t, "an unknown label", mirror.settle(t, 0))
		wantFailure(t, failures, snWritebackOpState, "something_the_sync_invented", "no key", "not sent")
		if mirror.readCount() != 0 {
			t.Fatal("the record was read for a state that could never be written")
		}
	})
	t.Run("their record cannot be read", func(t *testing.T) {
		mirror, failures, svc := dualWritePatchHarness(committedAt("implement", "", ""), states("SCHEDULED", "IMPLEMENT"))
		mirror.getErr = &apierror.ServiceUnavailableError{Msg: "snclient: down"}
		patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateImplement)})
		wantCalls(t, "unreadable", mirror.settle(t, 0))
		wantFailure(t, failures, snWritebackOpState, `"implement"`, "could not be read", "snclient: down")
	})
	t.Run("their record is already in our state: nothing to write, nothing recorded", func(t *testing.T) {
		mirror, failures, svc := dualWritePatchHarness(committedAt("implement", "", ""), states("SCHEDULED", "IMPLEMENT"))
		mirror.setSNState("implement")
		patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateImplement)})
		wantCalls(t, "already there", mirror.settle(t, 0))
		wantNoFailure(t, "already there", failures)
		if mirror.readCount() != 1 {
			t.Fatalf("reads = %d", mirror.readCount())
		}
	})
}

func TestChangeRequestStateMirror_A500OnAStateWriteIsARefusalThatNamesTheState(t *testing.T) {
	// The previous system refuses a state its model does not take with a
	// bare 500, which the pass-through turns into a generic message: the
	// failure row still says which state was sent and where the record was.
	mirror, failures, svc := dualWritePatchHarness(committedAt("closed", "", ""), states("REVIEW", "CLOSED"))
	mirror.patchErr = &apierror.DownstreamError{Msg: "An unexpected error occurred."}
	patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateClosed)})
	wantCalls(t, "a refused write", mirror.settle(t, 1), stateWrite("closed"))
	row := wantFailure(t, failures, snWritebackOpState, `state write {state: "closed"}`, `was in "review"`, "An unexpected error occurred.")
	if string(row.Payload) != `{"state":"closed","after":"PatchChangeRequest"}` {
		t.Fatalf("failure payload = %s", row.Payload)
	}
}

func TestChangeRequestStateMirror_CustomersAnswerIsMirroredThenReadBack(t *testing.T) {
	yes, no := true, false
	for _, tc := range []struct {
		name         string
		req          domain.PatchChangeRequestRequest
		prior, after string
	}{
		{"approval given", domain.PatchChangeRequestRequest{IsCustomerApproved: &yes}, "CUSTOMER_APPROVAL", "SCHEDULED"},
		{"approval refused", domain.PatchChangeRequestRequest{IsCustomerApproved: &no}, "CUSTOMER_APPROVAL", "CANCELED"},
		{"review given", domain.PatchChangeRequestRequest{IsCustomerReviewed: &yes}, "CUSTOMER_REVIEW", "CLOSED"},
		{"review refused", domain.PatchChangeRequestRequest{IsCustomerReviewed: &no}, "CUSTOMER_REVIEW", "ROLLBACK"},
	} {
		t.Run(tc.name+": their engine followed", func(t *testing.T) {
			// The window the answer was given for is a precondition checked
			// against PostgreSQL only: never sent. The answer is the previous
			// system's own path; its engine moves its record, which the
			// read-back finds in our state: nothing written, nothing recorded.
			req := tc.req
			req.ExpectedPlannedStartOn, req.ExpectedPlannedEndOn = sPtr("2030-03-01T09:00:00Z"), sPtr("2030-03-01T11:00:00Z")
			mirror, failures, svc := dualWritePatchHarness(committedAt(strings.ToLower(tc.after), "2030-03-01T09:00:00Z", "2030-03-01T11:00:00Z"), states(tc.prior, tc.after))
			mirror.setSNState(strings.ToLower(tc.after))
			patchAs(t, svc, callerCtx(t, &customerCaller), req)
			wantCalls(t, tc.name, mirror.settle(t, 1), fieldsWrite(tc.req))
			wantNoFailure(t, tc.name, failures)
			if mirror.readCount() != 1 {
				t.Fatalf("reads = %d, want the one read-back", mirror.readCount())
			}
		})
		t.Run(tc.name+": their engine did not follow", func(t *testing.T) {
			mirror, failures, svc := dualWritePatchHarness(committedAt(strings.ToLower(tc.after), "", ""), states(tc.prior, tc.after))
			patchAs(t, svc, callerCtx(t, &customerCaller), tc.req)
			wantCalls(t, tc.name, mirror.settle(t, 1), fieldsWrite(tc.req))
			row := wantFailure(t, failures, snWritebackOpStateCheck, "state divergence after PatchChangeRequest", `ours="`+strings.ToLower(tc.after)+`"`, `theirs="`+strings.ToLower(tc.prior)+`"`)
			if string(row.Payload) != `{"state":"`+strings.ToLower(tc.after)+`","after":"PatchChangeRequest"}` {
				t.Fatalf("failure payload = %s", row.Payload)
			}
		})
	}
	t.Run("an answer that resolves nothing yet (another contact's turn) mirrors the answer only", func(t *testing.T) {
		mirror, failures, svc := dualWritePatchHarness(committedAt("customer_approval", "", ""), states("CUSTOMER_APPROVAL", "CUSTOMER_APPROVAL"))
		patchAs(t, svc, callerCtx(t, &customerCaller), domain.PatchChangeRequestRequest{IsCustomerApproved: &yes})
		wantCalls(t, "answer without a move", mirror.settle(t, 1), fieldsWrite(domain.PatchChangeRequestRequest{IsCustomerApproved: &yes}))
		wantNoFailure(t, "answer without a move", failures)
		if mirror.readCount() != 0 {
			t.Fatal("the record was read back although nothing moved")
		}
	})
}

func TestChangeRequestStateMirror_AcceptProposedTimeMirrorsTheWindowThenTriesScheduled(t *testing.T) {
	req := domain.PatchChangeRequestRequest{ConfirmCustomerUpdatedDate: sPtr("agree"), ExpectedCustomerUpdatedOn: sPtr("2030-03-08T09:00:00Z"),
		ExpectedPlannedStartOn: sPtr("2030-03-01T09:00:00Z"), ExpectedPlannedEndOn: sPtr("2030-03-01T11:00:00Z")}
	window := fieldsWrite(domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00")})
	t.Run("their record is in Customer Approval: the model has no move to Scheduled from it, recorded", func(t *testing.T) {
		mirror, failures, svc := dualWritePatchHarness(committedAt("scheduled", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"), states("CUSTOMER_APPROVAL", "SCHEDULED"))
		patchAs(t, svc, callerCtx(t, &staffCaller), req)
		wantCalls(t, "Accept", mirror.settle(t, 1), window)
		wantFailure(t, failures, snWritebackOpState, `"scheduled"`, "not allowed by the previous system's change model", `"customer_approval"`)
	})
	t.Run("their record is in a state the model allows Scheduled from: written", func(t *testing.T) {
		mirror, failures, svc := dualWritePatchHarness(committedAt("scheduled", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"), states("CUSTOMER_APPROVAL", "SCHEDULED"))
		mirror.setSNState("new")
		patchAs(t, svc, callerCtx(t, &staffCaller), req)
		wantCalls(t, "Accept", mirror.settle(t, 2), window, stateWrite("scheduled"))
		wantNoFailure(t, "Accept", failures)
	})
}

// --- nothing is dispatched when the state did not move ---

func TestChangeRequestStateMirror_NothingWhenTheStateDidNotMove(t *testing.T) {
	t.Run("a resend of the state the change is in", func(t *testing.T) {
		for _, st := range []string{"authorize", "implement", "review", "closed", "canceled"} {
			mirror, failures, svc := dualWritePatchHarness(committedAt(st, "", ""), states(strings.ToUpper(st), strings.ToUpper(st)))
			patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestState(st))})
			wantCalls(t, "resend of "+st, mirror.settle(t, 0))
			wantNoFailure(t, "resend of "+st, failures)
			if mirror.readCount() != 0 {
				t.Fatalf("resend of %s: the record was read", st)
			}
		}
	})
	t.Run("the Re-schedule / counter-proposal wire: the window only, never the state", func(t *testing.T) {
		req := domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAuthorize), PlannedStartOn: sPtr("2030-03-08T09:00:00Z"), PlannedEndOn: sPtr("2030-03-08T11:00:00Z")}
		mirror, failures, svc := dualWritePatchHarness(committedAt("customer_approval", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"), states("CUSTOMER_APPROVAL", "CUSTOMER_APPROVAL"))
		patchAs(t, svc, callerCtx(t, &staffCaller), req)
		wantCalls(t, "Re-schedule", mirror.settle(t, 1), fieldsWrite(domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00")}))
		wantNoFailure(t, "Re-schedule", failures)
	})
	t.Run("a decline on the authorize wire", func(t *testing.T) {
		req := domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAuthorize), ExpectedCustomerUpdatedOn: sPtr("2030-03-08T09:00:00Z")}
		mirror, failures, svc := dualWritePatchHarness(committedAt("customer_approval", "2030-03-01T09:00:00Z", ""), states("CUSTOMER_APPROVAL", "CUSTOMER_APPROVAL"))
		patchAs(t, svc, callerCtx(t, &staffCaller), req)
		wantCalls(t, "decline", mirror.settle(t, 0))
		wantNoFailure(t, "decline", failures)
	})
	t.Run("a customer's proposed window (external caller)", func(t *testing.T) {
		mirror, failures, svc := dualWritePatchHarness(committedAt("customer_approval", "2030-03-01T09:00:00Z", ""), states("CUSTOMER_APPROVAL", "CUSTOMER_APPROVAL"))
		patchAs(t, svc, callerCtx(t, &customerCaller), domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00")})
		wantCalls(t, "proposal", mirror.settle(t, 0))
		wantNoFailure(t, "proposal", failures)
	})
	t.Run("a field edit", func(t *testing.T) {
		mirror, _, svc := dualWritePatchHarness(committedAt("implement", "", ""), states("IMPLEMENT", "IMPLEMENT"))
		patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{Title: sPtr("renamed")})
		wantCalls(t, "a title", mirror.settle(t, 1), fieldsWrite(domain.PatchChangeRequestRequest{Title: sPtr("renamed")}))
	})
	t.Run("no dispatcher (every other data source)", func(t *testing.T) {
		repo := &stubChangeRequestRepo{patchChangeRequestStates: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, repository.ChangeRequestStates, error) {
			return committedAt("canceled", "", ""), states("NEW", "CANCELED"), nil
		}}
		svc := NewChangeRequestService(repo, stubUserRepo{})
		patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateCanceled)})
	})
}

// --- DecideChangeRequestApproval: the cascade is their engine's; read back ---

func dualWriteDecisionHarness(st repository.ChangeRequestStates) (*mirrorRecorder, *recordingSNWritebackFailures, ChangeRequestService) {
	mirror := &mirrorRecorder{}
	if st.Prior != nil {
		mirror.snState = strings.ToLower(*st.Prior)
	}
	failures := &recordingSNWritebackFailures{}
	repo := &stubChangeRequestRepo{
		decideChangeRequestApprovalStates: func(context.Context, string, string, string, string) (string, repository.ChangeRequestStates, error) {
			return "approval-record-id", st, nil
		},
	}
	users := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
	}}
	return mirror, failures, NewChangeRequestServiceWithSNWriteback(repo, users, mirror, NewSNWritebackDispatcher(failures))
}

func TestChangeRequestStateMirror_ApprovalCascadeMirrorsTheDecisionAndReadsBack(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	for _, tc := range []struct {
		name         string
		prior, after string
		decision     string
	}{
		{"peer approval: Assess -> Authorize", "ASSESS", "AUTHORIZE", "approved"},
		{"CAB approval: Authorize -> Scheduled", "AUTHORIZE", "SCHEDULED", "approved"},
		{"CAB approval with the customer's approval required: Authorize -> Customer Approval", "AUTHORIZE", "CUSTOMER_APPROVAL", "approved"},
		{"the customer group's approval through the decision route: Customer Approval -> Scheduled", "CUSTOMER_APPROVAL", "SCHEDULED", "approved"},
		{"the customer group's review: Customer Review -> Closed", "CUSTOMER_REVIEW", "CLOSED", "approved"},
		{"a customer group's rejection: Customer Approval -> Canceled", "CUSTOMER_APPROVAL", "CANCELED", "rejected"},
	} {
		t.Run(tc.name+": their engine followed", func(t *testing.T) {
			// The decision is mirrored; their engine makes the move; the
			// read-back finds the record in our state. No state is ever
			// written for a cascade.
			mirror, failures, svc := dualWriteDecisionHarness(states(tc.prior, tc.after))
			mirror.setSNState(strings.ToLower(tc.after))
			if _, err := svc.DecideChangeRequestApproval(ctx, testUUID, tc.decision); err != nil {
				t.Fatalf("DecideChangeRequestApproval: %v", err)
			}
			wantCalls(t, tc.name, mirror.settle(t, 1), "decision:"+tc.decision)
			wantNoFailure(t, tc.name, failures)
			if mirror.readCount() != 1 {
				t.Fatalf("reads = %d, want the one read-back", mirror.readCount())
			}
		})
		t.Run(tc.name+": their engine did not follow", func(t *testing.T) {
			mirror, failures, svc := dualWriteDecisionHarness(states(tc.prior, tc.after))
			if _, err := svc.DecideChangeRequestApproval(ctx, testUUID, tc.decision); err != nil {
				t.Fatalf("DecideChangeRequestApproval: %v", err)
			}
			wantCalls(t, tc.name, mirror.settle(t, 1), "decision:"+tc.decision)
			wantFailure(t, failures, snWritebackOpStateCheck, "state divergence after DecideChangeRequestApproval", `ours="`+strings.ToLower(tc.after)+`"`, `theirs="`+strings.ToLower(tc.prior)+`"`)
		})
	}
	t.Run("a decision that resolves no stage mirrors the decision only", func(t *testing.T) {
		mirror, failures, svc := dualWriteDecisionHarness(states("ASSESS", "ASSESS"))
		if _, err := svc.DecideChangeRequestApproval(ctx, testUUID, "rejected"); err != nil {
			t.Fatalf("DecideChangeRequestApproval: %v", err)
		}
		wantCalls(t, "rejection", mirror.settle(t, 1), "decision:rejected")
		wantNoFailure(t, "rejection", failures)
		if mirror.readCount() != 0 {
			t.Fatal("the record was read back although nothing moved")
		}
	})
}

// --- the GitHub sync's SetState ---

type fakeGhMutationsForMirror struct {
	repository.GithubMutationRepository
	changed bool
	err     error
	got     []string
}

func (f *fakeGhMutationsForMirror) SetState(_ context.Context, id, state string) (bool, error) {
	f.got = append(f.got, id+":"+state)
	return f.changed, f.err
}

func TestChangeRequestStateMirror_GithubSetStateIsMirroredWhenItMoved(t *testing.T) {
	t.Run("a move the model allows", func(t *testing.T) {
		mirror := &mirrorRecorder{snState: "review"}
		failures := &recordingSNWritebackFailures{}
		inner := &fakeGhMutationsForMirror{changed: true}
		m := MirrorGithubStateChanges(inner, mirror, NewSNWritebackDispatcher(failures))
		changed, err := m.SetState(context.Background(), testUUID, "closed")
		if err != nil || !changed {
			t.Fatalf("SetState = %v, %v", changed, err)
		}
		if len(inner.got) != 1 || inner.got[0] != testUUID+":closed" {
			t.Fatalf("the wrapped repository got %v", inner.got)
		}
		wantCalls(t, "closed from the sync", mirror.settle(t, 1), stateWrite("closed"))
		wantNoFailure(t, "closed from the sync", failures)
	})
	t.Run("a move the model has no direct write for (Rollback)", func(t *testing.T) {
		mirror := &mirrorRecorder{snState: "review"}
		failures := &recordingSNWritebackFailures{}
		m := MirrorGithubStateChanges(&fakeGhMutationsForMirror{changed: true}, mirror, NewSNWritebackDispatcher(failures))
		if changed, err := m.SetState(context.Background(), testUUID, "rollback"); err != nil || !changed {
			t.Fatalf("SetState = %v, %v", changed, err)
		}
		wantCalls(t, "rollback", mirror.settle(t, 0))
		wantFailure(t, failures, snWritebackOpState, `"rollback"`, "not mirrorable")
	})
	t.Run("a write of the state the change already holds", func(t *testing.T) {
		mirror := &mirrorRecorder{}
		m := MirrorGithubStateChanges(&fakeGhMutationsForMirror{changed: false}, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
		if changed, err := m.SetState(context.Background(), testUUID, "closed"); err != nil || changed {
			t.Fatalf("SetState = %v, %v", changed, err)
		}
		wantCalls(t, "no move", mirror.settle(t, 0))
	})
	t.Run("a refused move", func(t *testing.T) {
		mirror := &mirrorRecorder{}
		refusal := &apierror.ValidationError{Msg: "refused"}
		m := MirrorGithubStateChanges(&fakeGhMutationsForMirror{err: refusal}, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
		if _, err := m.SetState(context.Background(), testUUID, "implement"); !errors.Is(err, refusal) {
			t.Fatalf("SetState err = %v, want the repository's refusal", err)
		}
		wantCalls(t, "refused", mirror.settle(t, 0))
	})
}

// --- a refused state move is recorded with the previous system's reason, listed on the detail, and replayable ---

func TestChangeRequestStateMirror_RefusedMoveIsVisibleAndReplayable(t *testing.T) {
	ctx := callerCtx(t, &staffCaller)
	mirror := &mirrorRecorder{snState: "review", patchErr: &apierror.DownstreamError{Msg: "An unexpected error occurred."}}
	failures := &recordingSNWritebackFailures{}
	committed := committedAt("closed", "", "")
	repo := &stubChangeRequestRepo{
		patchChangeRequestStates: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, repository.ChangeRequestStates, error) {
			committed.ID = id
			return committed, states("REVIEW", "CLOSED"), nil
		},
		getChangeRequestByID: func(_ context.Context, id string) (domain.ChangeRequest, error) {
			committed.ID = id
			return committed, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, dispatcher)

	patchAs(t, svc, ctx, domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateClosed)})
	mirror.settle(t, 1)
	row := wantFailure(t, failures, snWritebackOpState, `state write {state: "closed"}`, "An unexpected error occurred.")

	// The detail of the change request lists it, for an internal caller...
	cr, err := svc.GetChangeRequest(ctx, testUUID)
	if err != nil {
		t.Fatalf("GetChangeRequest: %v", err)
	}
	if len(cr.MirrorFailures) != 1 || cr.MirrorFailures[0].ID != row.ID || cr.MirrorFailures[0].Operation != "state" ||
		cr.MirrorFailures[0].Error != row.Error || string(cr.MirrorFailures[0].Payload) != `{"state":"closed","after":"PatchChangeRequest"}` {
		t.Fatalf("MirrorFailures = %+v, want the recorded row", cr.MirrorFailures)
	}
	// ...and not for a customer (nil: the mirror is not their business)...
	if cr, err := svc.GetChangeRequest(callerCtx(t, &customerCaller), testUUID); err != nil || cr.MirrorFailures != nil {
		t.Fatalf("a customer's detail carries MirrorFailures %+v, %v", cr.MirrorFailures, err)
	}
	// ...and never on a data source with no mirror.
	if cr, err := NewChangeRequestService(repo, stubUserRepo{}).GetChangeRequest(ctx, testUUID); err != nil || cr.MirrorFailures != nil {
		t.Fatalf("a plain data source's detail carries MirrorFailures %+v, %v", cr.MirrorFailures, err)
	}

	// A replay while the previous system still refuses: the row stays, its
	// reason refreshed, and the refusal comes back to the caller.
	mirror.patchErr = &apierror.DownstreamError{Msg: "still refused"}
	_, err = dispatcher.ReplaySNWritebackFailure(ctx, row.ID)
	var de *apierror.DownstreamError
	if !errors.As(err, &de) || !strings.Contains(err.Error(), "still refused") {
		t.Fatalf("replay err = %v, want the previous system's refusal", err)
	}
	if rows := failures.outstanding(); len(rows) != 1 || !strings.Contains(rows[0].Error, "still refused") {
		t.Fatalf("after a failed replay the rows are %+v", rows)
	}

	// A replay once their record is in a state the model does not allow the
	// move from: refused by this service, as a 409 that says from where.
	mirror.patchErr = nil
	mirror.setSNState("assess")
	_, err = dispatcher.ReplaySNWritebackFailure(ctx, row.ID)
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) || !strings.Contains(ce.Msg, `"assess"`) {
		t.Fatalf("replay from assess = %v, want the model's refusal", err)
	}

	// Once their record allows it, the replay re-sends exactly the recorded
	// write and clears the row; nothing is left on the detail.
	mirror.setSNState("review")
	resp, err := dispatcher.ReplaySNWritebackFailure(ctx, row.ID)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if resp.Failure.ID != row.ID {
		t.Fatalf("replay cleared %+v, want %s", resp.Failure, row.ID)
	}
	wantCalls(t, "the original and the replays that reached a write", mirror.calls(), stateWrite("closed"), stateWrite("closed"), stateWrite("closed"))
	if rows := failures.outstanding(); len(rows) != 0 {
		t.Fatalf("after a successful replay the rows are %+v", rows)
	}
	if cr, err := svc.GetChangeRequest(ctx, testUUID); err != nil || len(cr.MirrorFailures) != 0 || cr.MirrorFailures == nil {
		t.Fatalf("after the replay the detail lists %+v (%v); want an empty list, not null", cr.MirrorFailures, err)
	}
	// A second replay of the same id: gone.
	var nf *apierror.NotFoundError
	if _, err := dispatcher.ReplaySNWritebackFailure(ctx, row.ID); !errors.As(err, &nf) {
		t.Fatalf("replay of a cleared row = %v, want not found", err)
	}
}

func TestChangeRequestStateMirror_ADivergenceClearsWhenTheRecordsAgreeAgain(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	mirror, failures, svc := dualWriteDecisionHarness(states("ASSESS", "AUTHORIZE"))
	if _, err := svc.DecideChangeRequestApproval(ctx, testUUID, "approved"); err != nil {
		t.Fatalf("DecideChangeRequestApproval: %v", err)
	}
	mirror.settle(t, 1)
	row := wantFailure(t, failures, snWritebackOpStateCheck, "state divergence")
	dispatcher := svc.(*changeRequestService).snWriteback

	// Still apart: the row stays, the reason is refreshed, nothing is written.
	mirror.setSNState("new")
	var ce *apierror.ConflictError
	if _, err := dispatcher.ReplaySNWritebackFailure(ctx, row.ID); !errors.As(err, &ce) || !strings.Contains(ce.Msg, `theirs="new"`) {
		t.Fatalf("replay while apart = %v", err)
	}
	// A person put their record right (or their engine caught up): the
	// replay finds them in step and clears the row.
	mirror.setSNState("authorize")
	if _, err := dispatcher.ReplaySNWritebackFailure(ctx, row.ID); err != nil {
		t.Fatalf("replay once in step: %v", err)
	}
	if len(failures.outstanding()) != 0 {
		t.Fatalf("rows left: %+v", failures.outstanding())
	}
	wantCalls(t, "no state was ever written for a cascade", mirror.calls(), "decision:approved")
}

func TestChangeRequestStateMirror_ReplayOfEveryRecordedOperation(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	mirror := &mirrorRecorder{snState: "scheduled", patchErr: errors.New("down"), decisionErr: errors.New("down")}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	repo := &stubChangeRequestRepo{
		patchChangeRequestStates: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, repository.ChangeRequestStates, error) {
			return committedAt("implement", "", ""), states("SCHEDULED", "IMPLEMENT"), nil
		},
		decideChangeRequestApprovalStates: func(context.Context, string, string, string, string) (string, repository.ChangeRequestStates, error) {
			return "approval-record-id", states("ASSESS", "ASSESS"), nil
		},
	}
	users := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testUUID, Email: "jane.doe@example.com"}, nil
	}}
	svc := NewChangeRequestServiceWithSNWriteback(repo, users, mirror, dispatcher)

	// A fields write, a state write and a decision all fail: three rows.
	patchAs(t, svc, ctx, domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateImplement), Title: sPtr("renamed")})
	if _, err := svc.DecideChangeRequestApproval(ctx, testUUID, "approved"); err != nil {
		t.Fatalf("DecideChangeRequestApproval: %v", err)
	}
	mirror.settle(t, 3)
	waitFor(t, func() bool { return failures.count() == 3 })
	rows := failures.outstanding()
	ops := []string{rows[0].Operation, rows[1].Operation, rows[2].Operation}
	if !reflect.DeepEqual(ops, []string{snWritebackOpPatch, snWritebackOpState, snWritebackOpDecision}) {
		t.Fatalf("recorded operations %v", ops)
	}

	mirror.patchErr, mirror.decisionErr = nil, nil
	for _, row := range rows {
		if _, err := dispatcher.ReplaySNWritebackFailure(ctx, row.ID); err != nil {
			t.Fatalf("replay of %s: %v", row.Operation, err)
		}
	}
	calls := mirror.calls()
	wantCalls(t, "the replays re-send what was recorded", calls[3:], fieldsWrite(domain.PatchChangeRequestRequest{Title: sPtr("renamed")}), stateWrite("implement"), "decision:approved")
	if len(failures.outstanding()) != 0 {
		t.Fatalf("rows left after the replays: %+v", failures.outstanding())
	}
	// The outstanding list, newest first, is the operator's view.
	if got, err := dispatcher.ListSNWritebackFailures(ctx, "", "", 0); err != nil || len(got.Failures) != 0 {
		t.Fatalf("list after the replays = %+v, %v", got, err)
	}
}

func TestChangeRequestStateMirror_ReplayRefusals(t *testing.T) {
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	var ve *apierror.ValidationError
	if _, err := dispatcher.ReplaySNWritebackFailure(context.Background(), "not-a-uuid"); !errors.As(err, &ve) {
		t.Fatalf("replay of a malformed id = %v, want a 400", err)
	}
	var nf *apierror.NotFoundError
	if _, err := dispatcher.ReplaySNWritebackFailure(context.Background(), testUUID); !errors.As(err, &nf) {
		t.Fatalf("replay of an unknown id = %v, want a 404", err)
	}
	// A row of an entity nobody registered a replay for (another entity's
	// pilot) is refused by name, and kept.
	dispatcher.Dispatch(context.Background(), "account", testUUID, "update", map[string]string{"name": "x"}, func(context.Context) error { return errors.New("down") })
	waitFor(t, func() bool { return failures.count() == 1 })
	row := failures.outstanding()[0]
	if _, err := dispatcher.ReplaySNWritebackFailure(context.Background(), row.ID); !errors.As(err, &ve) || !strings.Contains(ve.Msg, "account/update") {
		t.Fatalf("replay with no replayer = %v", err)
	}
	if len(failures.outstanding()) != 1 {
		t.Fatal("a refused replay removed the row")
	}
}

// --- field coverage: every field is classified, and the non-mirrored ones never leave ---

func TestMirrorablePatchFields_EveryFieldIsClassified(t *testing.T) {
	// The fields the previous system's API accepts, kept as sent, and the
	// fields deliberately not mirrored (mirrorablePatchFields' doc comment is
	// the list). A field added to PatchChangeRequestRequest without a place
	// in either list fails here, so none is ever forwarded by accident.
	kept := map[string]bool{
		"Title": true, "Description": true, "ProjectID": true, "CaseID": true, "DeploymentID": true, "DeployedProductID": true,
		"AssignedEngineerID": true, "AssignedTeamID": true, "PlannedStartOn": true, "PlannedEndOn": true, "Impact": true, "Type": true,
		"Justification": true, "ImpactDescription": true, "ServiceOutage": true, "CommunicationPlan": true, "RollbackPlan": true, "TestPlan": true,
		"IsCustomerApproved": true, "IsCustomerReviewed": true, "RequestApproval": true, "IsPlanningVisibleToCustomers": true,
		"ImplementationPlan": true, "Priority": true, "Category": true, "RequestedByID": true, "AffectedServicesText": true,
		"AffectedComponentsText": true, "RollbackDurationText": true, "Comment": true, "WorkNote": true, "DurationInput": true,
	}
	dropped := map[string]bool{
		"State": true, "ConfirmCustomerUpdatedDate": true, "ExpectedCustomerUpdatedOn": true, "ExpectedPlannedStartOn": true, "ExpectedPlannedEndOn": true,
		"CustomerApprovalRequired": true, "CustomerReviewRequired": true, "DeploymentIDs": true, "DeploymentProductIDs": true,
		"CustomerGroupID": true, "EnvironmentIDs": true, "OnHold": true, "OnHoldReason": true, "ServiceID": true, "ServiceOfferingID": true,
	}
	typ := reflect.TypeOf(domain.PatchChangeRequestRequest{})
	full := reflect.New(typ).Elem()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !kept[f.Name] && !dropped[f.Name] {
			t.Fatalf("PatchChangeRequestRequest.%s is neither kept nor deliberately dropped by the mirror: classify it", f.Name)
		}
		// Every field set to a non-nil value of its own type.
		full.Field(i).Set(reflect.New(f.Type.Elem()))
	}
	got := mirrorablePatchFields(full.Interface().(domain.PatchChangeRequestRequest))
	gv := reflect.ValueOf(got)
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if dropped[name] && !gv.Field(i).IsNil() {
			t.Errorf("%s is sent to the previous system; it has no field there", name)
		}
		if kept[name] && gv.Field(i).IsNil() {
			t.Errorf("%s was dropped from the mirror; the previous system's API accepts it", name)
		}
	}
}

func TestChangeRequestStateMirror_NonMirroredFieldsNeverReachTheMirror(t *testing.T) {
	// A staff PATCH that carries the whole non-mirrored set beside a title
	// (the gates, the hold, the services, the scope lists): the mirror gets
	// the title and nothing else.
	yes := true
	req := domain.PatchChangeRequestRequest{Title: sPtr("renamed"), CustomerApprovalRequired: &yes, CustomerReviewRequired: &yes,
		OnHold: &yes, OnHoldReason: sPtr("waiting"), ServiceID: sPtr(testUUID), ServiceOfferingID: sPtr(testUUID),
		DeploymentIDs: &[]string{testUUID}, DeploymentProductIDs: &[]string{testUUID}}
	mirror, _, svc := dualWritePatchHarness(committedAt("implement", "", ""), states("IMPLEMENT", "IMPLEMENT"))
	patchAs(t, svc, callerCtx(t, &staffCaller), req)
	wantCalls(t, "the non-mirrored set", mirror.settle(t, 1), fieldsWrite(domain.PatchChangeRequestRequest{Title: sPtr("renamed")}))
	// ...and a request of only such fields mirrors nothing at all (not even
	// an empty write the previous system would refuse as a failure).
	mirror, failures, svc := dualWritePatchHarness(committedAt("implement", "", ""), states("IMPLEMENT", "IMPLEMENT"))
	patchAs(t, svc, callerCtx(t, &staffCaller), domain.PatchChangeRequestRequest{OnHold: &yes, OnHoldReason: sPtr("waiting")})
	wantCalls(t, "a hold", mirror.settle(t, 0))
	if failures.count() != 0 {
		t.Fatalf("%d write-back failures recorded for a hold", failures.count())
	}
}
