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
	"reflect"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The customer's proposed time (customer_updated_on) and WSO2's answer
// (customer_updated_date_confirmation) are PostgreSQL-only: the previous system's change request API has
// no field for either. What is mirrored of the acts of that conversation is decided from WHO SENT
// the PATCH (a customer's window is a proposal and never goes) and, for the acts WSO2 performs,
// from what PostgreSQL COMMITTED; every other PATCH mirrors exactly what it always did.

func sPtr(s string) *string { return &s }

func statePtr(s domain.ChangeRequestState) *domain.ChangeRequestState { return &s }

// The identities a PATCH can arrive under, as the server's identity middleware stamps them: the
// customer (neither unrestricted nor staff), WSO2 staff, staff who also hold an external record
// ("external wins" for what they may LIST, they are still staff), an internal client credential
// (unrestricted, no viewer), and nobody (what every test of this file used before the caller
// mattered, and what a background job without an identity would be).
func callerCtx(t *testing.T, scope *repository.SearchScope) context.Context {
	t.Helper()
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if scope == nil {
		return ctx
	}
	return repository.WithCallerIdentity(ctx, *scope)
}

var (
	customerCaller          = repository.SearchScope{ViewerEmail: "jane.doe@example.com", ProjectIDs: []string{testUUID}}
	staffCaller             = repository.SearchScope{Unrestricted: true, ViewerEmail: "jane.doe@example.com", HasInternalAccess: true}
	staffWithExternalCaller = repository.SearchScope{ViewerEmail: "jane.doe@example.com", HasInternalAccess: true, ProjectIDs: []string{testUUID}}
	internalClientCaller    = repository.SearchScope{Unrestricted: true}
	everyNonCustomerCall    = map[string]*repository.SearchScope{
		"no identity":                            nil,
		"staff":                                  &staffCaller,
		"staff who also hold an external record": &staffWithExternalCaller,
		"an internal client credential":          &internalClientCaller,
	}
)

// What a test of the mirror expects of a PATCH, which decides how long runMirrorAs waits.
type mirrorExpectation bool

const (
	// expectMirrorWrite: the mirror is to be asked to PATCH. The wait is only a ceiling (the select
	// returns the moment the write arrives), so it is generous: a write is handed to the dispatcher's
	// worker at once, but under -race on a loaded machine "at once" can be a while, and a positive case
	// must not fail for that.
	expectMirrorWrite mirrorExpectation = true
	// expectNoMirrorWrite: the mirror is NOT to be called. There is nothing to wait for, so this is the
	// only case that waits the whole period: long enough to outlast a goroutine, short enough not to
	// cost the suite seconds.
	expectNoMirrorWrite mirrorExpectation = false
)

const (
	mirrorWriteCeiling = 2 * time.Second
	mirrorQuietPeriod  = 150 * time.Millisecond
)

// runMirror sends req (as nobody in particular, see callerCtx) through the dual-write service whose
// repository answers with committed, and returns what the mirror to the previous system was asked to PATCH (nil
// when it was not called at all, within the wait expect gives: see mirrorExpectation).
func runMirror(t *testing.T, expect mirrorExpectation, req domain.PatchChangeRequestRequest, committed domain.ChangeRequest) *domain.PatchChangeRequestRequest {
	t.Helper()
	return runMirrorAs(t, callerCtx(t, nil), expect, req, committed)
}

// runMirrorAs is runMirror for the caller ctx carries.
func runMirrorAs(t *testing.T, ctx context.Context, expect mirrorExpectation, req domain.PatchChangeRequestRequest, committed domain.ChangeRequest) *domain.PatchChangeRequestRequest {
	t.Helper()
	called := make(chan domain.PatchChangeRequestRequest, 1)
	mirror := &stubMirrorChangeRequestService{
		patchChangeRequest: func(_ context.Context, _ string, r domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
			called <- r
			return domain.PatchChangeRequestResponse{}, nil
		},
	}
	repo := &stubChangeRequestRepo{
		patchChangeRequest: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
			committed.ID = id
			return committed, nil
		},
	}
	failures := &recordingSNWritebackFailures{}
	svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(failures))
	if _, err := svc.PatchChangeRequest(ctx, testUUID, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wait := mirrorQuietPeriod
	if expect == expectMirrorWrite {
		wait = mirrorWriteCeiling
	}
	select {
	case got := <-called:
		return &got
	case <-time.After(wait):
		if n := failures.count(); n != 0 {
			t.Fatalf("nothing was dispatched but %d write-back failures were recorded", n)
		}
		return nil
	}
}

// customerProposals are the shapes of a customer's proposed window, as the portal sends them.
var customerProposals = map[string]domain.PatchChangeRequestRequest{
	"the start alone":                   {PlannedStartOn: sPtr("2030-03-08 09:00:00")},
	"the start and the derived end":     {PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00")},
	"the start in RFC 3339 with offset": {PlannedStartOn: sPtr("2030-03-08T14:30:00+05:30"), PlannedEndOn: sPtr("2030-03-08T16:30:00+05:30")},
}

func committedAt(state, start, end string) domain.ChangeRequest {
	cr := domain.ChangeRequest{}
	cr.State = sPtr(state)
	if start != "" {
		cr.PlannedStartOn = sPtr(start)
	}
	if end != "" {
		cr.PlannedEndOn = sPtr(end)
	}
	return cr
}

func TestChangeRequestService_PatchChangeRequest_TheTimeConversationMirror(t *testing.T) {
	const planStart, planEnd = "2030-03-01T09:00:00Z", "2030-03-01T11:00:00Z"
	withProposal := func(cr domain.ChangeRequest, start string) domain.ChangeRequest {
		cr.CustomerProposal = &domain.ChangeRequestCustomerProposal{StartOn: start, Answer: "pending"}
		return cr
	}

	t.Run("a customer's proposal mirrors nothing", func(t *testing.T) {
		for name, req := range customerProposals {
			got := runMirrorAs(t, callerCtx(t, &customerCaller), expectNoMirrorWrite, req, withProposal(committedAt("customer_approval", planStart, planEnd), "2030-03-08T09:00:00Z"))
			if got != nil {
				t.Fatalf("%s: the mirror was asked to PATCH %+v: the plan did not move, the proposal has no field in the previous system", name, *got)
			}
		}
	})

	t.Run("Accept mirrors Scheduled and the committed window, in the previous system's layout", func(t *testing.T) {
		req := domain.PatchChangeRequestRequest{ConfirmCustomerUpdatedDate: sPtr("agree"), ExpectedCustomerUpdatedOn: sPtr("2030-03-08T09:00:00Z"),
			ExpectedPlannedStartOn: sPtr(planStart), ExpectedPlannedEndOn: sPtr(planEnd)}
		got := runMirror(t, expectMirrorWrite, req, committedAt("scheduled", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"))
		if got == nil {
			t.Fatal("Accept was not mirrored")
		}
		want := domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateScheduled), PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00")}
		if !reflect.DeepEqual(*got, want) {
			t.Fatalf("Accept mirrored %+v, want exactly {state: scheduled, plannedStartOn, plannedEndOn} %+v", *got, want)
		}
	})

	t.Run("a Re-schedule or a counter-proposal mirrors the window and not the state", func(t *testing.T) {
		for name, req := range map[string]domain.PatchChangeRequestRequest{
			"a plain Re-schedule": {State: statePtr(domain.ChangeRequestStateAuthorize), PlannedStartOn: sPtr("2030-03-08T09:00:00Z"), PlannedEndOn: sPtr("2030-03-08T11:00:00Z")},
			"a different time with the proposal and the window it saw": {State: statePtr(domain.ChangeRequestStateAuthorize), PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00"),
				ExpectedCustomerUpdatedOn: sPtr("2030-03-05T09:00:00Z"), ExpectedPlannedStartOn: sPtr(planStart), ExpectedPlannedEndOn: sPtr(planEnd)},
		} {
			got := runMirror(t, expectMirrorWrite, req, committedAt("customer_approval", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"))
			if got == nil {
				t.Fatalf("%s: nothing mirrored", name)
			}
			want := domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00")}
			if !reflect.DeepEqual(*got, want) {
				t.Fatalf("%s: mirrored %+v, want the window only %+v (the previous system stays where PostgreSQL stays)", name, *got, want)
			}
		}
	})

	t.Run("a decline mirrors nothing", func(t *testing.T) {
		req := domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAuthorize), ExpectedCustomerUpdatedOn: sPtr("2030-03-08T09:00:00Z"),
			ExpectedPlannedStartOn: sPtr(planStart), ExpectedPlannedEndOn: sPtr(planEnd)}
		if got := runMirror(t, expectNoMirrorWrite, req, committedAt("customer_approval", planStart, planEnd)); got != nil {
			t.Fatalf("a decline mirrored %+v", *got)
		}
	})

	t.Run("an Accept-only request passes the at-least-one-field check", func(t *testing.T) {
		repo := &stubChangeRequestRepo{patchChangeRequest: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
			return committedAt("scheduled", planStart, planEnd), nil
		}}
		svc := NewChangeRequestService(repo, stubUserRepo{})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
		if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{ConfirmCustomerUpdatedDate: sPtr("agree")}); err != nil {
			t.Fatalf("an Accept-only request: %v", err)
		}
		// ...and a request with no field at all is still refused.
		_, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) || ve.Msg != "at least one field must be provided" {
			t.Fatalf("an empty request: %v", err)
		}
	})
}

// A customer's proposed window never reaches the previous system as the plan, and WHETHER IT IS ONE is
// decided from who sent the PATCH, not from the read model the repository builds AFTER the commit
// (GetChangeRequestByID -> fillCustomerProposal): that read runs in another transaction, logs and
// swallows its errors, and can see a conversation that has moved on. Whatever it comes back with,
// the answer is the same: nothing is mirrored.
func TestChangeRequestService_PatchChangeRequest_ACustomersWindowIsNeverMirrored(t *testing.T) {
	const planStart, planEnd = "2030-03-01T09:00:00Z", "2030-03-01T11:00:00Z"
	const proposed = "2030-03-08T09:00:00Z"
	proposedAt := func(cr domain.ChangeRequest, answer, start string) domain.ChangeRequest {
		cr.CustomerUpdatedOn = sPtr(proposed)
		cr.CustomerProposal = &domain.ChangeRequestCustomerProposal{StartOn: start, Answer: answer}
		return cr
	}
	for name, committed := range map[string]domain.ChangeRequest{
		"the read model shows the proposal waiting": proposedAt(committedAt("customer_approval", planStart, planEnd), "pending", proposed),
		// fillCustomerProposal failed (a lost connection, a timeout) and left the field unset: the
		// row says a time was proposed (customerUpdatedOn) and the read model says nothing.
		"the proposal read failed and left customerProposal unset": func() domain.ChangeRequest {
			cr := committedAt("customer_approval", planStart, planEnd)
			cr.CustomerUpdatedOn = sPtr(proposed)
			return cr
		}(),
		// WSO2 accepted between the customer's commit and the read: the plan IS the proposed time now.
		"WSO2 accepted it in between": proposedAt(committedAt("scheduled", proposed, "2030-03-08T11:00:00Z"), "agreed", proposed),
		// ...or asked for another time, or the proposal was overwritten by a colleague's or by the sync's.
		"WSO2 asked for another time in between":               proposedAt(committedAt("customer_approval", "2030-03-20T09:00:00Z", "2030-03-20T11:00:00Z"), "disagreed", proposed),
		"a colleague proposed another time in between":         proposedAt(committedAt("customer_approval", planStart, planEnd), "pending", "2030-03-09T09:00:00Z"),
		"the proposal is unanswered and unreadable as pending": proposedAt(committedAt("customer_approval", planStart, planEnd), "unanswered", proposed),
		// the change request was moved out of Customer Approval by someone else in between
		"the change was cancelled in between":     proposedAt(committedAt("canceled", planStart, planEnd), "pending", proposed),
		"the read returned a bare change request": committedAt("", "", ""),
	} {
		for shape, req := range customerProposals {
			if got := runMirrorAs(t, callerCtx(t, &customerCaller), expectNoMirrorWrite, req, committed); got != nil {
				t.Fatalf("%s / %s: the mirror was asked to PATCH %+v: a customer's window is a proposal, the previous system has no field for it and must not get it as the plan", name, shape, *got)
			}
		}
	}

	t.Run("the customer's own answer is mirrored as before, and its window is not", func(t *testing.T) {
		yes := true
		withWindowShown := domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, ExpectedPlannedStartOn: sPtr(planStart), ExpectedPlannedEndOn: sPtr(planEnd)}
		want := domain.PatchChangeRequestRequest{IsCustomerApproved: &yes}
		for name, ctx := range map[string]context.Context{"a customer": callerCtx(t, &customerCaller), "nobody in particular": callerCtx(t, nil)} {
			got := runMirrorAs(t, ctx, expectMirrorWrite, withWindowShown, committedAt("scheduled", planStart, planEnd))
			if got == nil || !reflect.DeepEqual(*got, want) {
				t.Fatalf("%s: the answer was mirrored as %+v, want exactly %+v", name, got, want)
			}
		}
	})

	t.Run("a window from anybody else is mirrored, whatever the read model says about a proposal", func(t *testing.T) {
		// The same requests under every non-customer identity are WSO2's own edits of the plan.
		for who, scope := range everyNonCustomerCall {
			for shape, req := range customerProposals {
				for name, committed := range map[string]domain.ChangeRequest{
					"no proposal": committedAt("customer_approval", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"),
					"the proposal read failed": func() domain.ChangeRequest {
						cr := committedAt("customer_approval", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z")
						cr.CustomerUpdatedOn = sPtr(proposed)
						return cr
					}(),
					"a proposal answered in between": proposedAt(committedAt("customer_approval", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"), "agreed", proposed),
				} {
					got := runMirrorAs(t, callerCtx(t, scope), expectMirrorWrite, req, committed)
					if got == nil || got.PlannedStartOn == nil || *got.PlannedStartOn != "2030-03-08 09:00:00" {
						t.Fatalf("%s / %s / %s: mirrored %+v, want the window as the previous system takes it (the plan was applied)", who, shape, name, got)
					}
				}
			}
		}
	})
}

// Every other PATCH is mirrored exactly as it was before the conversation existed: the same
// fields, the same values, the state included -- the acts above are the only exceptions, and they
// are recognised by who sent the request (a customer's window) and by what PostgreSQL committed
// (what WSO2 did), never by the shape of the request alone.
func TestChangeRequestService_PatchChangeRequest_EveryOtherPatchMirrorsAsBefore(t *testing.T) {
	const planStart, planEnd = "2030-03-01T09:00:00Z", "2030-03-01T11:00:00Z"
	title := "renamed"
	for _, tc := range []struct {
		name      string
		req       domain.PatchChangeRequestRequest
		committed domain.ChangeRequest
		want      domain.PatchChangeRequestRequest
	}{
		{"a title", domain.PatchChangeRequestRequest{Title: &title}, committedAt("new", "", ""), domain.PatchChangeRequestRequest{Title: &title}},
		{"Request Approval", domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAssess)}, committedAt("assess", "", ""),
			domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAssess)}},
		{"a resend of authorize on a change that is in Authorize (state kept: the change IS in Authorize)", domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAuthorize)},
			committedAt("authorize", "", ""), domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateAuthorize)}},
		{"Cancel", domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateCanceled)}, committedAt("canceled", "", ""),
			domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateCanceled)}},
		{"a staff edit of the window", domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08T14:30:00+05:30"), PlannedEndOn: sPtr("2030-03-08 11:00:00")},
			committedAt("customer_approval", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"),
			domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00"), PlannedEndOn: sPtr("2030-03-08 11:00:00")}},
		{"a staff edit of the window to exactly the time the customer proposed (it IS applied: mirrored)", domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00")},
			func() domain.ChangeRequest {
				cr := committedAt("customer_approval", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z")
				cr.CustomerProposal = &domain.ChangeRequestCustomerProposal{StartOn: "2030-03-08T09:00:00Z", Answer: "unanswered"}
				return cr
			}(), domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-08 09:00:00")}},
		{"a window edit while a proposal waits that moves to another time", domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-20 09:00:00")},
			func() domain.ChangeRequest {
				cr := committedAt("customer_approval", "2030-03-20T09:00:00Z", "2030-03-20T11:00:00Z")
				cr.CustomerProposal = &domain.ChangeRequestCustomerProposal{StartOn: "2030-03-08T09:00:00Z", Answer: "pending"}
				return cr
			}(), domain.PatchChangeRequestRequest{PlannedStartOn: sPtr("2030-03-20 09:00:00")}},
		{"a window edit with a state", domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateCanceled), PlannedStartOn: sPtr("2030-03-08 09:00:00")},
			committedAt("canceled", "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"),
			domain.PatchChangeRequestRequest{State: statePtr(domain.ChangeRequestStateCanceled), PlannedStartOn: sPtr("2030-03-08 09:00:00")}},
		{"a comment", domain.PatchChangeRequestRequest{Comment: sPtr("hello")}, committedAt("customer_approval", planStart, planEnd), domain.PatchChangeRequestRequest{Comment: sPtr("hello")}},
	} {
		// ...whoever of WSO2 (or nobody) sends it: only a CUSTOMER's window is held back.
		for who, scope := range everyNonCustomerCall {
			got := runMirrorAs(t, callerCtx(t, scope), expectMirrorWrite, tc.req, tc.committed)
			if got == nil {
				t.Fatalf("%s as %s: nothing was mirrored, want %+v", tc.name, who, tc.want)
			}
			if !reflect.DeepEqual(*got, tc.want) {
				t.Fatalf("%s as %s: mirrored %+v, want exactly what it always was: %+v", tc.name, who, *got, tc.want)
			}
		}
	}
}

// The data source that talks to the previous system directly has no PostgreSQL columns to hold the
// answer: that system is the authority there and answers the customer's proposed date itself. A request that carries the
// answer is refused up front and nothing is sent; proposals and Re-schedules are forwarded as ever.
func TestSNChangeRequestService_PatchChangeRequest_RefusesTheAnswerToAProposedTime(t *testing.T) {
	svc := NewServiceNowChangeRequestService(nil)
	for name, req := range map[string]domain.PatchChangeRequestRequest{
		"Accept":              {ConfirmCustomerUpdatedDate: sPtr("agree")},
		"Accept with a title": {ConfirmCustomerUpdatedDate: sPtr("agree"), Title: sPtr("x")},
		"the version alone":   {ExpectedCustomerUpdatedOn: sPtr("2030-03-08T09:00:00Z"), Title: sPtr("x")},
	} {
		_, err := svc.PatchChangeRequest(contextWithUserIDToken("token"), testCaseUUID, req)
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) || ve.Msg != "confirmCustomerUpdatedDate is not supported on the ServiceNow data source: answer the customer's proposed date in ServiceNow" {
			t.Fatalf("%s: err = %v, want the refusal naming where to answer", name, err)
		}
	}
}
