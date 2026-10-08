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

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Which change requests a customer may see (change_request_visibility.go): the
// change requests DESIGNATED to them -- the customer's approval or review was
// asked of them -- and, for change requests created before the strict-visibility
// cutover instant ("legacy"), what customers have always seen: everything past
// Authorize, to the registered contacts of the project.
//
// The harness is the customer-group one (crFlow, DSN-gated by
// CHANGE_REQUEST_TEST_DSN). Run it twice, as the suite normally does: as a
// Postgres superuser (no row-level security: the repository's own SQL is the only
// thing between a customer and a row) and as the non-superuser role the
// application connects as (row-level security enforced too). The rule is the
// repository's, so every assertion here is the same under both.
//
// The cast (see seedScope): Alice and Bob are the registered PORTAL_USER contacts
// of project A, who are ASKED when a change request of A reaches a customer
// stage; Sam is registered on A with only SECURITY_CONTACT and Ian with a
// deactivated user, neither of whom is ever asked; Ivy is only invited to A; Carol
// is a registered contact of project B; a stranger is a user with no contact row.

// visStrictSinceLongAgo is strict mode for every change request the tests create.
func visStrictSinceLongAgo() repository.CRVisibility {
	from := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	return repository.CRVisibility{StrictFrom: &from}
}

// visStrictFrom is the cutover at the given instant.
func visStrictFrom(t time.Time) repository.CRVisibility {
	return repository.CRVisibility{StrictFrom: &t}
}

// useVisibility rebuilds the flow's repository (the one every flow helper drives
// the change request through) with the given visibility policy.
func (f *crFlow) useVisibility(vis repository.CRVisibility) {
	f.repo = repository.NewChangeRequestRepository(f.scoped, vis)
}

// visRepos are the customer-reachable readers built with one policy.
type visRepos struct {
	cr       repository.ChangeRequestRepository
	stats    repository.ProjectStatsRepository
	comments repository.CommentRepository
	notices  repository.CRNoticeRepository
	cases    repository.CaseRepository
}

func (f *crFlow) visRepos(vis repository.CRVisibility) visRepos {
	return visRepos{
		cr:       repository.NewChangeRequestRepository(f.scoped, vis),
		stats:    repository.NewProjectStatsRepository(f.scoped, vis),
		comments: repository.NewCommentRepository(f.scoped, vis),
		notices:  repository.NewCRNoticeRepository(f.scoped, vis),
		cases:    repository.NewCaseRepository(f.scoped, vis),
	}
}

func (f *crFlow) wantNotFound(what string, err error) {
	f.t.Helper()
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		f.t.Fatalf("%s: err = %v (%T), want *apierror.NotFoundError", what, err, err)
	}
}

// wantHiddenFrom asserts the change request does not exist for each of the
// customers: GET answers 404.
func (f *crFlow) wantHiddenFrom(id, when string, users ...string) {
	f.t.Helper()
	for _, u := range users {
		if _, err := f.getAsContact(id, u); err != nil {
			f.wantNotFound(fmt.Sprintf("GET %s as %s", when, u), err)
			continue
		}
		f.t.Fatalf("GET %s as %s: the change request was returned, want 404", when, u)
	}
}

// persona is one caller of the customer portal.
type persona struct {
	name string
	ctx  context.Context
}

func (f *crFlow) personas() map[string]persona {
	mk := func(name, userID string) persona { return persona{name, asContact(userID)} }
	return map[string]persona{
		"alice":    mk("alice", crScopeUserA1),
		"bob":      mk("bob", crScopeUserA2),
		"sam":      mk("sam", crScopeUserSecurity),
		"ian":      mk("ian", crScopeUserInactive),
		"ivy":      mk("ivy", crScopeUserInvited),
		"carol":    mk("carol", crScopeUserB1),
		"stranger": persona{"stranger", repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: "nobody-at-all@example.com"})},
	}
}

// seen is everything one caller can reach of one change request through the
// customer-reachable reads.
type seen struct {
	listed, detail, approvals, comments, attachments bool
	listTotal, aggTotal, statsTotal                  int
}

// probe reads the change request id (of project) every way a customer can.
func (f *crFlow) probe(r visRepos, ctx context.Context, id, project string) seen {
	f.t.Helper()
	var s seen
	filters := domain.SearchChangeRequestsFilters{ProjectIDs: []string{project}}
	views, total, err := r.cr.SearchChangeRequests(ctx, domain.SearchChangeRequestsRequest{Filters: filters, Pagination: domain.Pagination{Limit: 50}}, nil, nil, nil, nil)
	if err != nil {
		f.t.Fatalf("SearchChangeRequests: %v", err)
	}
	for _, v := range views {
		if v.ID == id {
			s.listed = true
		}
	}
	if total != len(views) {
		f.t.Fatalf("SearchChangeRequests total = %d but the page holds %d rows", total, len(views))
	}
	s.listTotal = total

	agg, err := r.cr.AggregateChangeRequests(ctx, domain.AggregateChangeRequestsRequest{Filters: filters, GroupBy: "state"}, "state", 20, nil, nil, nil)
	if err != nil {
		f.t.Fatalf("AggregateChangeRequests: %v", err)
	}
	s.aggTotal = agg.TotalRecords

	visible := func(what string, err error) bool {
		f.t.Helper()
		if err == nil {
			return true
		}
		f.wantNotFound(what, err)
		return false
	}
	_, err = r.cr.GetChangeRequestByID(ctx, id)
	s.detail = visible("GetChangeRequestByID", err)
	_, err = r.cr.GetChangeRequestApprovals(ctx, id)
	s.approvals = visible("GetChangeRequestApprovals", err)
	_, _, err = r.comments.SearchComments(ctx, id, domain.ReferenceTypeChangeRequest, nil, true, domain.Pagination{Limit: 10})
	s.comments = visible("SearchComments", err)
	_, _, err = r.cases.SearchWorkItemAttachments(ctx, id, domain.ReferenceTypeChangeRequest, domain.Pagination{Limit: 10})
	s.attachments = visible("SearchWorkItemAttachments", err)

	counts, err := r.stats.ChangeRequestStateCounts(ctx, project)
	if err != nil {
		f.t.Fatalf("ChangeRequestStateCounts: %v", err)
	}
	for _, c := range counts {
		s.statsTotal += c.Count
	}
	return s
}

// wantSeen asserts what a caller reaches of the one change request in the
// project: every read finds it, or none does (and every count is 0 or 1).
func (f *crFlow) wantSeen(r visRepos, who persona, id, project, when string, want bool) {
	f.t.Helper()
	got := f.probe(r, who.ctx, id, project)
	n := 0
	if want {
		n = 1
	}
	if got.listed != want || got.detail != want || got.approvals != want || got.comments != want || got.attachments != want ||
		got.listTotal != n || got.aggTotal != n || got.statsTotal != n {
		f.t.Fatalf("%s as %s: visible to the caller = %v, but the reads say %+v (want every read %v and every count %d)", when, who.name, want, got, want, n)
	}
}

// wantHiddenWrites asserts that every WRITE a customer can send for a change
// request they may not see is a 404 (never a 403 that confirms it exists, never a
// validation message), and that nothing about the change request changed.
func (f *crFlow) wantHiddenWrites(r visRepos, who persona, id, userID, when string) {
	f.t.Helper()
	before := f.customerSnapshot(id, map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob"}) + " | stages " + f.stageLabels(id) + " | " + f.updatedBy(id)
	email := crFlowEmail(userID)
	title, assess := "renamed by a stranger", domain.ChangeRequestStateAssess
	for _, tc := range []struct {
		name string
		req  domain.PatchChangeRequestRequest
	}{
		{"approve", domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(true)}},
		{"reject", domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(false)}},
		{"confirm the review", domain.PatchChangeRequestRequest{IsCustomerReviewed: boolp(true)}},
		{"propose a time", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2)}},
		{"a field no customer may set", domain.PatchChangeRequestRequest{Title: &title}},
		{"a state", domain.PatchChangeRequestRequest{State: &assess}},
	} {
		_, err := r.cr.PatchChangeRequest(who.ctx, id, tc.req, email)
		f.wantNotFound(fmt.Sprintf("%s: PATCH (%s) as %s", when, tc.name, who.name), err)
	}
	for _, decision := range []string{"approved", "rejected"} {
		_, err := r.cr.DecideChangeRequestApproval(who.ctx, id, userID, decision, email)
		f.wantNotFound(fmt.Sprintf("%s: decision %s as %s", when, decision, who.name), err)
	}
	_, err := r.comments.CreateComment(who.ctx, id, domain.ReferenceTypeChangeRequest, "COMMENT", "a comment on a change request I cannot see", email)
	f.wantNotFound(fmt.Sprintf("%s: comment as %s", when, who.name), err)
	after := f.customerSnapshot(id, map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob"}) + " | stages " + f.stageLabels(id) + " | " + f.updatedBy(id)
	if after != before {
		f.t.Fatalf("%s: refused writes as %s changed the change request:\n  before: %s\n  after:  %s", when, who.name, before, after)
	}
}

// userIDOf maps a persona name to the user id its email is built from.
var visUserIDs = map[string]string{
	"alice": crScopeUserA1, "bob": crScopeUserA2, "sam": crScopeUserSecurity,
	"ian": crScopeUserInactive, "ivy": crScopeUserInvited, "carol": crScopeUserB1,
	"stranger": "3bbbbbbb-0000-0000-0000-0000000000ff",
}

// phase asserts, for every persona, whether they can see the change request.
func (f *crFlow) phase(r visRepos, id, project, when string, visibleTo ...string) {
	f.t.Helper()
	want := map[string]bool{}
	for _, v := range visibleTo {
		want[v] = true
	}
	names := make([]string, 0)
	for n := range f.personas() {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		who := f.personas()[n]
		f.wantSeen(r, who, id, project, when, want[n])
		if !want[n] {
			f.wantHiddenWrites(r, who, id, visUserIDs[n], when)
		}
	}
}

// ---------------------------------------------------------------------------

// A change request is invisible to every customer until the customer's approval
// is first asked, then visible for ever -- in EVERY later state -- to exactly the
// contacts who were asked: Alice and Bob. The path takes in a proposed new time
// (which waits in Customer Approval), WSO2's different time (the customers are asked
// again with a fresh request), the customer's answer, Implement, Review, the customer
// review and Closed, and goes through the approval cascade that escalates the
// session identity inside the customer's own transaction. Everything a customer
// can reach is checked in each state: the list and its total, the aggregate, the
// detail, the approvals, the comments and the stat cards, and the writes.
func TestChangeRequestVisibilityIntegration_DesignatedStaysVisibleThroughTheLifecycle(t *testing.T) {
	f := newCustomerGroupFlow(t)
	vis := visStrictSinceLongAgo()
	f.useVisibility(vis)
	r := f.visRepos(vis)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	f.setPlanned(id, rsStart1, rsEnd1)
	const a = crScopeProjectA

	f.phase(r, id, a, "in New")
	f.requestApproval(id)
	f.phase(r, id, a, "in Assess")
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	f.phase(r, id, a, "in Authorize (before the customer was ever asked)")

	// CAB approval enters Customer Approval and asks the customer: Alice and Bob
	// are designated, and nobody else.
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.phase(r, id, a, "in Customer Approval", "alice", "bob")

	// Alice proposes another time: it waits in Customer Approval for WSO2 -- the
	// change request stays visible to both.
	if _, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2)}); err != nil {
		t.Fatalf("proposal: %v", err)
	}
	f.expect(id, "in Customer Approval after the proposal", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.phase(r, id, a, "in Customer Approval after a proposed new time", "alice", "bob")

	// WSO2 proposes a different time: the request they were asked is superseded by a
	// fresh one -- and it must STAY visible to both.
	if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
		t.Fatalf("WSO2's different time: %v", err)
	}
	f.expect(id, "in Customer Approval after WSO2's different time", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.phase(r, id, a, "in Customer Approval again (asked about WSO2's time)", "alice", "bob")

	// Alice answers (the cascade: Bob's row is Cancelled, the change is Scheduled).
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("approval: %v", err)
	}
	f.expect(id, "in Scheduled", "SCHEDULED", "implement", "canceled")
	f.phase(r, id, a, "in Scheduled (Bob's row is Cancelled)", "alice", "bob")

	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.phase(r, id, a, "in Implement", "alice", "bob")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	f.phase(r, id, a, "in Review", "alice", "bob")
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	f.phase(r, id, a, "in Customer Review", "alice", "bob")

	if _, err := f.reviewAs(id, crScopeUserA2, true); err != nil {
		t.Fatalf("customer review: %v", err)
	}
	f.expect(id, "in Closed", "CLOSED")
	f.phase(r, id, a, "in Closed", "alice", "bob")
}

// The two other ways a change request ends, both after the customer's answer:
// the customer rejects the approval (Canceled) or the review (Rollback). Both
// contacts keep seeing it, the answering one and the one whose row was Cancelled.
func TestChangeRequestVisibilityIntegration_RollbackAndCanceledStayVisible(t *testing.T) {
	t.Run("Canceled: the customer rejected the approval", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		vis := visStrictSinceLongAgo()
		f.useVisibility(vis)
		r := f.visRepos(vis)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.driveToCustomerApproval(id)
		if _, err := f.approveAs(id, crScopeUserA2, false); err != nil {
			t.Fatalf("rejection: %v", err)
		}
		f.expect(id, "in Canceled", "CANCELED")
		f.phase(r, id, crScopeProjectA, "in Canceled", "alice", "bob")
	})
	t.Run("Rollback: the customer rejected the review", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		vis := visStrictSinceLongAgo()
		f.useVisibility(vis)
		r := f.visRepos(vis)
		id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), false, true)
		f.requestApproval(id)
		f.expect(id, "after Request Approval (Standard, no customer approval)", "SCHEDULED", "implement", "canceled")
		f.phase(r, id, crScopeProjectA, "in Scheduled, the customer's approval was not required")
		f.driveToCustomerReview(id)
		f.phase(r, id, crScopeProjectA, "in Customer Review", "alice", "bob")
		if _, err := f.reviewAs(id, crScopeUserA1, false); err != nil {
			t.Fatalf("rejection of the review: %v", err)
		}
		f.expect(id, "in Rollback", "ROLLBACK")
		f.phase(r, id, crScopeProjectA, "in Rollback", "alice", "bob")
	})
}

// Designation is per person and permanent. A contact who registers AFTER the
// request went out was never asked and never sees the change request, whatever
// state it reaches; a contact whose row a colleague's answer Cancelled still does;
// moving the change request to another project takes it from the old project's
// contacts and gives it to nobody in the new one.
func TestChangeRequestVisibilityIntegration_DesignationIsPerPersonAndPermanent(t *testing.T) {
	f := newCustomerGroupFlow(t)
	vis := visStrictSinceLongAgo()
	f.useVisibility(vis)
	r := f.visRepos(vis)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.driveToCustomerApproval(id)
	f.phase(r, id, crScopeProjectA, "in Customer Approval", "alice", "bob")

	// Ivy, invited so far, registers after the stage was provisioned.
	f.execSQL(`UPDATE project_contact SET state = 'REGISTERED' WHERE LOWER(email) = LOWER($1)`, crFlowEmail(crScopeUserInvited))
	f.phase(r, id, crScopeProjectA, "once Ivy registered afterwards (she was never asked)", "alice", "bob")

	// Alice answers: Bob's row is CANCELLED, and he still sees it. Ivy still does not.
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("approval: %v", err)
	}
	assertApprovers(t, "Customer Approval after", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "APPROVED", crScopeUserA2: "CANCELLED"})
	f.phase(r, id, crScopeProjectA, "in Scheduled with Bob's row Cancelled", "alice", "bob")

	// The change request moves to project B (a project change, by whatever means):
	// project A's contacts lose it; project B's Carol was never asked and is not
	// designated, so nobody sees it.
	f.execSQL(`UPDATE work_item SET project_id = $2 WHERE id = $1`, id, crScopeProjectB)
	f.phase(r, id, crScopeProjectB, "after moving to project B (nobody there was asked)")
	f.phase(r, id, crScopeProjectA, "after moving to project B, project A's list")

	// ...and back: the designation was never lost.
	f.execSQL(`UPDATE work_item SET project_id = $2 WHERE id = $1`, id, crScopeProjectA)
	f.phase(r, id, crScopeProjectA, "after moving back to project A", "alice", "bob")
}

// A change request that never required the customer is not for them: no stage is
// ever provisioned, nobody is designated, and strict mode keeps it invisible in
// every state (a legacy one is visible past Authorize, as it always was).
func TestChangeRequestVisibilityIntegration_NeverRequiredTheCustomer(t *testing.T) {
	t.Run("strict: invisible in every state", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		vis := visStrictSinceLongAgo()
		f.useVisibility(vis)
		r := f.visRepos(vis)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		f.phase(r, id, crScopeProjectA, "in New")
		f.requestApproval(id)
		f.phase(r, id, crScopeProjectA, "in Assess")
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.phase(r, id, crScopeProjectA, "in Scheduled")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.phase(r, id, crScopeProjectA, "in Implement")
		f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
		f.phase(r, id, crScopeProjectA, "in Review")
		f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
		f.phase(r, id, crScopeProjectA, "in Closed")
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("customer stages = %+v on a change request that never required the customer", f.customerStages(id))
		}
	})
	t.Run("legacy: visible past Authorize to the project's registered contacts, as before", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		r := f.visRepos(repository.CRVisibility{}) // no cutover: legacy
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
		f.phase(r, id, crScopeProjectA, "in New")
		f.requestApproval(id)
		f.phase(r, id, crScopeProjectA, "in Assess")
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		// Every registered contact of the project -- asked or not, PORTAL_USER or
		// not, active user or not -- as every project member saw it before. Ivy is
		// only invited, Carol is on another project: neither is a member.
		f.phase(r, id, crScopeProjectA, "in Scheduled (legacy)", "alice", "bob", "sam", "ian")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		f.phase(r, id, crScopeProjectA, "in Implement (legacy)", "alice", "bob", "sam", "ian")
	})
}

// The legacy rule, state by state and instant by instant, on change requests
// nobody was ever asked about (their state is forced, as a migrated or older-build
// row would hold it):
//   - no cutover (unset): every change request is legacy -- visible to the
//     project's registered contacts in every state past Authorize, never in
//     New / Assess / Authorize;
//   - a cutover AFTER the change request was created: legacy, the same;
//   - a cutover at or BEFORE it: strict, never visible without a designation.
func TestChangeRequestVisibilityIntegration_LegacyRule(t *testing.T) {
	type phase struct {
		state   string
		legacyV bool // visible to the project's registered contacts when legacy
	}
	states := []phase{
		{"NEW", false}, {"ASSESS", false}, {"AUTHORIZE", false},
		{"CUSTOMER_APPROVAL", true}, {"SCHEDULED", true}, {"IMPLEMENT", true}, {"REVIEW", true},
		{"CUSTOMER_REVIEW", true}, {"ROLLBACK", true}, {"CLOSED", true}, {"CANCELED", true},
	}
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	var createdOn time.Time
	if err := f.scoped.QueryRow(f.sys, `SELECT created_on FROM work_item WHERE id = $1`, id).Scan(&createdOn); err != nil {
		t.Fatalf("read created_on: %v", err)
	}
	cases := []struct {
		name   string
		vis    repository.CRVisibility
		legacy bool
	}{
		{"unset: no cutover", repository.CRVisibility{}, true},
		{"cutover an hour after it was created", visStrictFrom(createdOn.Add(time.Hour)), true},
		{"cutover one microsecond after it was created", visStrictFrom(createdOn.Add(time.Microsecond)), true},
		{"cutover at the very instant it was created", visStrictFrom(createdOn), false},
		{"cutover an hour before it was created", visStrictFrom(createdOn.Add(-time.Hour)), false},
		{"cutover long ago", visStrictSinceLongAgo(), false},
	}
	for _, tc := range cases {
		r := f.visRepos(tc.vis)
		for _, st := range states {
			f.setStoredState(id, st.state)
			var visibleTo []string
			if tc.legacy && st.legacyV {
				visibleTo = []string{"alice", "bob", "sam", "ian"}
			}
			f.phase(r, id, crScopeProjectA, fmt.Sprintf("%s, state %s", tc.name, st.state), visibleTo...)
		}
	}
	// A NULL state (a pre-lifecycle row) counts as New: never visible.
	f.execSQL(`UPDATE change_request SET state = NULL WHERE id = $1`, id)
	f.phase(f.visRepos(repository.CRVisibility{}), id, crScopeProjectA, "a NULL state")
}

// A change request created AFTER the cutover instant is never legacy, however old
// the code that handles it: a Scheduled one nobody was asked about is invisible.
func TestChangeRequestVisibilityIntegration_CreatedAfterTheCutoverIsNeverLegacy(t *testing.T) {
	f := newCustomerGroupFlow(t)
	vis := visStrictFrom(time.Now().Add(-time.Hour))
	f.useVisibility(vis)
	r := f.visRepos(vis)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	f.setStoredState(id, "SCHEDULED")
	f.phase(r, id, crScopeProjectA, "created after the cutover, Scheduled, nobody asked")
	// ...and the same row, were it older than the cutover, is the legacy one.
	f.execSQL(`UPDATE work_item SET created_on = now() - interval '2 hours' WHERE id = $1`, id)
	f.phase(r, id, crScopeProjectA, "the same row made older than the cutover", "alice", "bob", "sam", "ian")
}

// A legacy change request a customer ACTED on stays visible afterwards: the act
// records the designation in its own transaction. Here the change request is
// legacy, waiting in Customer Approval with nobody asked (it got there under an
// older build); Alice proposes a new time, which gives it the stage it lacked (and
// designates Alice and Bob), and WSO2 accepts it -- the change request is Scheduled, a
// state a legacy change request is visible in to the project's registered contacts. (The
// states a legacy change request is HIDDEN in -- Authorize and before -- are never reached
// now: a proposal never leaves Customer Approval; designation keeping a change request
// visible there is TestChangeRequestVisibilityIntegration_DesignatedStaysVisibleThroughTheLifecycle's.)
func TestChangeRequestVisibilityIntegration_LegacyChangeRequestActedOnStaysVisible(t *testing.T) {
	t.Run("a proposed time", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		r := f.visRepos(repository.CRVisibility{}) // no cutover: legacy
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.setStoredState(id, "CUSTOMER_APPROVAL")
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("setup: %d customer stages", n)
		}
		f.phase(r, id, crScopeProjectA, "legacy, in Customer Approval, nobody asked", "alice", "bob", "sam", "ian")

		if _, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2)}); err != nil {
			t.Fatalf("proposal on a legacy change request with no stage: %v", err)
		}
		f.expect(id, "after the proposal", "CUSTOMER_APPROVAL", "authorize", "canceled")
		f.wantPlanned(id, "after the proposal (the plan is not the proposal)", rsStart1, rsEnd1)
		f.wantConversation(id, "after the proposal", rsStart2, "")
		st := f.customerStages(id)
		if len(st) != 1 {
			t.Fatalf("customer stages after the proposal = %+v, want the one the proposal provisioned", st)
		}
		// The stage the proposal needed asks every registered portal contact, and the
		// proposal leaves it live.
		assertApprovers(t, "the provisioned stage after the proposal", st[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
		f.phase(r, id, crScopeProjectA, "legacy, in Customer Approval after a proposed time", "alice", "bob", "sam", "ian")

		// WSO2 accepts: the change request is Scheduled, still visible to the project's contacts.
		f.mustAccept(id)
		f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
		f.phase(r, id, crScopeProjectA, "legacy, in Scheduled after the proposed time was accepted", "alice", "bob", "sam", "ian")
	})
	t.Run("a customer's answer", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		r := f.visRepos(repository.CRVisibility{})
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
		f.setStoredState(id, "CUSTOMER_REVIEW")
		if _, err := f.reviewAs(id, crScopeUserA2, false); err != nil {
			t.Fatalf("a rejected review on a legacy change request with no stage: %v", err)
		}
		f.expect(id, "in Rollback", "ROLLBACK")
		f.phase(r, id, crScopeProjectA, "legacy, in Rollback", "alice", "bob", "sam", "ian")
	})
}

// An in-flight legacy change request in Customer Approval / Customer Review with
// no live customer stage (the user's "Demo Test 1": in Customer Approval since an
// older build): it gets its live stage when the customer first acts, the answer is
// recorded like any other, provisioning is idempotent, and reading never writes.
func TestChangeRequestVisibilityIntegration_LegacyInFlightGetsItsStage(t *testing.T) {
	newStageless := func(t *testing.T, state string) (*crFlow, string) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
		f.setStoredState(id, state)
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("setup: %d customer stages", n)
		}
		return f, id
	}

	t.Run("reading is told true and creates nothing, however often", func(t *testing.T) {
		f, id := newStageless(t, "CUSTOMER_APPROVAL")
		for i := 0; i < 3; i++ {
			f.wantCanAnswer(id, "legacy, no stage", true, crScopeUserA1, crScopeUserA2)
			f.wantCanAnswer(id, "legacy, no stage", false, crScopeUserSecurity, crScopeUserInactive)
		}
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("reads provisioned %d customer stage(s)", n)
		}
		if _, err := f.getAsContact(id, crScopeUserA1); err != nil {
			t.Fatalf("GET: %v", err)
		}
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("GET provisioned %d customer stage(s)", n)
		}
	})

	t.Run("the customer's approval is recorded", func(t *testing.T) {
		f, id := newStageless(t, "CUSTOMER_APPROVAL")
		if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
			t.Fatalf("approve: %v", err)
		}
		f.expect(id, "after the approval", "SCHEDULED", "implement", "canceled")
		st := f.customerStages(id)
		if len(st) != 1 {
			t.Fatalf("customer stages = %+v, want exactly one", st)
		}
		assertApprovers(t, "the provisioned stage", st[0].approvers, map[string]string{crScopeUserA1: "APPROVED", crScopeUserA2: "CANCELLED"})
		if a, _ := f.customerOutcome(id); !a {
			t.Fatal("the answer did not stamp the customer's approval")
		}
		if got := f.updatedBy(id); got != crFlowEmail(crScopeUserA1) {
			t.Fatalf("updated_by = %q, want the approving contact", got)
		}
	})

	t.Run("the decision route does the same", func(t *testing.T) {
		f, id := newStageless(t, "CUSTOMER_APPROVAL")
		if _, err := f.repo.DecideChangeRequestApproval(asContact(crScopeUserA2), id, crScopeUserA2, "approved", crFlowEmail(crScopeUserA2)); err != nil {
			t.Fatalf("decision: %v", err)
		}
		f.expect(id, "after the decision", "SCHEDULED", "implement", "canceled")
		assertApprovers(t, "the provisioned stage", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "APPROVED"})
	})

	t.Run("a rejected customer review rolls the change back", func(t *testing.T) {
		f, id := newStageless(t, "CUSTOMER_REVIEW")
		if _, err := f.reviewAs(id, crScopeUserA1, false); err != nil {
			t.Fatalf("reject the review: %v", err)
		}
		f.expect(id, "after the rejection", "ROLLBACK")
		assertApprovers(t, "the provisioned Customer Review stage", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REJECTED", crScopeUserA2: "CANCELLED"})
	})

	t.Run("two customers answering at once provision exactly one stage", func(t *testing.T) {
		f, id := newStageless(t, "CUSTOMER_APPROVAL")
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, u := range []string{crScopeUserA1, crScopeUserA2} {
			wg.Add(1)
			go func(i int, u string) {
				defer wg.Done()
				_, errs[i] = f.approveAs(id, u, true)
			}(i, u)
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
				continue
			}
			f.wantConflictContaining("the answer that lost the race", err, "no longer pending")
		}
		if ok != 1 {
			t.Fatalf("%d of two concurrent answers were accepted (errors: %v), want exactly one", ok, errs)
		}
		st := f.customerStages(id)
		if len(st) != 1 {
			t.Fatalf("customer stages = %+v, want exactly one (provisioning twice must change nothing)", st)
		}
		approved, cancelled := 0, 0
		for _, s := range st[0].approvers {
			switch s {
			case "APPROVED":
				approved++
			case "CANCELLED":
				cancelled++
			}
		}
		if approved != 1 || cancelled != 1 || len(st[0].approvers) != 2 {
			t.Fatalf("approvers = %v, want one APPROVED and one CANCELLED", st[0].approvers)
		}
	})

	t.Run("a staff member's act never provisions", func(t *testing.T) {
		f, id := newStageless(t, "CUSTOMER_APPROVAL")
		if err := f.decide(id, crFlowPeerAID, "approved"); err == nil {
			t.Fatal("a staff decision with nothing pending was accepted")
		}
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("a staff act provisioned %d customer stage(s)", n)
		}
	})

	t.Run("a stage that was decided is never reopened", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		f.driveToCustomerApproval(id)
		if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
			t.Fatalf("approve: %v", err)
		}
		// Forced back into Customer Approval (the state is not reachable by an
		// API, only by a sync or a hand edit): the stage is decided, so it is
		// not provisioned again.
		f.setStoredState(id, "CUSTOMER_APPROVAL")
		f.wantCanAnswer(id, "after a decided stage", false, crScopeUserA1, crScopeUserA2)
		_, err := f.approveAs(id, crScopeUserA2, true)
		f.wantConflictContaining("answering a decided stage", err, "no customer approval is pending")
		if n := len(f.customerStages(id)); n != 1 {
			t.Fatalf("customer stages = %d, want the one decided stage only", n)
		}
	})

	t.Run("strict: a change request created after the cutover is not provisioned for", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		vis := visStrictSinceLongAgo()
		f.useVisibility(vis)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
		f.setStoredState(id, "CUSTOMER_APPROVAL")
		f.wantHiddenFrom(id, "strict, in Customer Approval, no stage", crScopeUserA1, crScopeUserA2)
		_, err := f.approveAs(id, crScopeUserA1, true)
		f.wantNotFound("answering a strict change request nobody was asked about", err)
		if n := len(f.customerStages(id)); n != 0 {
			t.Fatalf("a refused act provisioned %d customer stage(s)", n)
		}
	})
}

// Row-level security and the visibility rule are two lines of defence; this one
// is the visibility rule alone: staff, the CSM portal's M2M identity (with and
// without an email) and every internal caller see every change request in every
// state, designated or not; a caller who also holds an external record (staff who
// are customers elsewhere) is restricted like a customer; a restricted caller with
// no email sees nothing.
func TestChangeRequestVisibilityIntegration_InternalCallersAreNotNarrowed(t *testing.T) {
	f := newCustomerGroupFlow(t)
	vis := visStrictSinceLongAgo()
	f.useVisibility(vis)
	r := f.visRepos(vis)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true) // New: nobody was asked

	unrestricted := map[string]context.Context{
		"the system identity":                  f.sys,
		"the CSM BFF client with a user email": repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true, ViewerEmail: crFlowEmail(crScopeUserA1), HasInternalAccess: true}),
		"an M2M client with no email":          repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true}),
		"internal staff resolved by token":     repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true, HasInternalAccess: true}),
	}
	for name, ctx := range unrestricted {
		f.wantSeen(r, persona{name, ctx}, id, crScopeProjectA, "in New", true)
	}
	// Staff who also hold an external (customer) record: restricted like a customer.
	mixed := persona{"staff with a customer record (Alice's email)", repository.WithCallerIdentity(context.Background(),
		repository.SearchScope{ViewerEmail: crFlowEmail(crScopeUserA1), HasInternalAccess: true})}
	f.wantSeen(r, mixed, id, crScopeProjectA, "in New", false)
	// No email: nobody to match, fail closed.
	blank := persona{"a restricted caller with no email", repository.WithCallerIdentity(context.Background(), repository.SearchScope{})}
	f.wantSeen(r, blank, id, crScopeProjectA, "in New", false)

	f.driveToCustomerApproval(id)
	for name, ctx := range unrestricted {
		f.wantSeen(r, persona{name, ctx}, id, crScopeProjectA, "in Customer Approval", true)
	}
	f.wantSeen(r, mixed, id, crScopeProjectA, "in Customer Approval", true)
	f.wantSeen(r, blank, id, crScopeProjectA, "in Customer Approval", false)
}

// By-comment-id operations reach a change request's comments without naming the
// change request: they must be as hidden as the search.
func TestChangeRequestVisibilityIntegration_CommentsByIDFollowTheChangeRequest(t *testing.T) {
	f := newCustomerGroupFlow(t)
	vis := visStrictSinceLongAgo()
	f.useVisibility(vis)
	r := f.visRepos(vis)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	sysComments := repository.NewCommentRepository(f.scoped, vis)
	row, err := sysComments.CreateComment(f.sys, id, domain.ReferenceTypeChangeRequest, "COMMENT", "visible to those who were asked", crFlowEmail(crFlowCreatorID))
	if err != nil {
		t.Fatalf("CreateComment as staff: %v", err)
	}
	alice, sam := asContact(crScopeUserA1), asContact(crScopeUserSecurity)

	// Not designated yet (New): Alice cannot reach it by id either.
	_, err = r.comments.GetCommentByID(alice, row.ID)
	f.wantNotFound("GetCommentByID before Alice was asked", err)
	if _, err := r.comments.UpdateComment(alice, row.ID, "edited", crFlowEmail(crScopeUserA1)); err == nil {
		t.Fatal("UpdateComment on a comment of a change request nobody asked Alice about succeeded")
	}
	f.wantNotFound("SoftDeleteComment before Alice was asked", r.comments.SoftDeleteComment(alice, row.ID, crFlowEmail(crScopeUserA1)))
	if hist, err := r.comments.GetCommentEditHistory(alice, row.ID); err != nil || len(hist) != 0 {
		t.Fatalf("GetCommentEditHistory before Alice was asked = %v, %v", hist, err)
	}

	f.driveToCustomerApproval(id)
	if got, err := r.comments.GetCommentByID(alice, row.ID); err != nil || got.ID != row.ID {
		t.Fatalf("GetCommentByID once Alice was asked = %+v, %v", got, err)
	}
	// Sam is a registered contact of the project who was never asked.
	_, err = r.comments.GetCommentByID(sam, row.ID)
	f.wantNotFound("GetCommentByID as Sam, never asked", err)
	f.wantNotFound("SoftDeleteComment as Sam, never asked", r.comments.SoftDeleteComment(sam, row.ID, crFlowEmail(crScopeUserSecurity)))
	if got, err := r.comments.GetCommentByID(f.sys, row.ID); err != nil || got.DeletedAt != nil {
		t.Fatalf("the comment after Sam's refused delete = %+v, %v", got, err)
	}
}

// Mail is a disclosure too: the customer notices of a change request go to the
// contacts it was designated to, never to the rest of the project -- except for a
// legacy change request nobody was asked about, which keeps the project audience.
func TestChangeRequestVisibilityIntegration_NoticeAudience(t *testing.T) {
	f := newCustomerGroupFlow(t)
	vis := visStrictSinceLongAgo()
	f.useVisibility(vis)
	r := f.visRepos(vis)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)

	emails := func(n repository.CRNoticeRepository, when string) []string {
		t.Helper()
		got, err := n.CustomerNoticeEmails(f.sys, id, crScopeProjectA)
		if err != nil {
			t.Fatalf("CustomerNoticeEmails %s: %v", when, err)
		}
		sort.Strings(got)
		return got
	}
	if got := emails(r.notices, "in New"); len(got) != 0 {
		t.Fatalf("audience of a strict change request nobody was asked about = %v, want nobody", got)
	}
	f.driveToCustomerApproval(id)
	want := []string{crFlowEmail(crScopeUserA1), crFlowEmail(crScopeUserA2)}
	sort.Strings(want)
	if got := emails(r.notices, "in Customer Approval"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("audience in Customer Approval = %v, want the contacts who were asked: %v", got, want)
	}
	// Ivy registers afterwards: still not told. Alice's row goes Cancelled by Bob's
	// answer: she is still told (the answer to a proposed time goes to both).
	f.execSQL(`UPDATE project_contact SET state = 'REGISTERED' WHERE LOWER(email) = LOWER($1)`, crFlowEmail(crScopeUserInvited))
	if _, err := f.approveAs(id, crScopeUserA2, true); err != nil {
		t.Fatalf("approval: %v", err)
	}
	if got := emails(r.notices, "after the answer"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("audience after the answer = %v, want %v", got, want)
	}

	// A legacy change request nobody was asked about: the whole project's audience.
	legacy := f.visRepos(repository.CRVisibility{})
	g := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	got, err := legacy.notices.CustomerNoticeEmails(f.sys, g, crScopeProjectA)
	if err != nil {
		t.Fatalf("CustomerNoticeEmails (legacy): %v", err)
	}
	project, err := legacy.notices.ProjectContactEmails(f.sys, crScopeProjectA)
	if err != nil {
		t.Fatalf("ProjectContactEmails: %v", err)
	}
	if strings.Join(got, ",") != strings.Join(project, ",") || len(got) < 4 {
		t.Fatalf("legacy audience = %v, want the project's contacts %v", got, project)
	}
	// The same change request under strict mode is nobody's.
	if got, err := r.notices.CustomerNoticeEmails(f.sys, g, crScopeProjectA); err != nil || len(got) != 0 {
		t.Fatalf("strict audience of a change request nobody was asked about = %v, %v", got, err)
	}
}

// The three stat cards count exactly what the list shows, per caller; the
// outstanding count takes a state list from the service (a customer's includes
// Authorize: see the service's crOutstandingStatesFor).
func TestChangeRequestVisibilityIntegration_StatsCountWhatTheListShows(t *testing.T) {
	f := newCustomerGroupFlow(t)
	vis := visStrictSinceLongAgo()
	f.useVisibility(vis)
	r := f.visRepos(vis)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)      // will be asked
	other := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false) // never
	f.setStoredState(other, "CLOSED")
	f.driveToCustomerApproval(id)
	alice, sam := asContact(crScopeUserA1), asContact(crScopeUserSecurity)

	outstanding := []string{"AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "ROLLBACK", "REVIEW", "CUSTOMER_REVIEW"}
	for _, tc := range []struct {
		who         string
		ctx         context.Context
		wantTotal   int
		wantOutstnd int
		wantClosed  int
	}{
		{"alice (asked)", alice, 1, 1, 0},
		{"sam (registered, never asked)", sam, 0, 0, 0},
		{"staff", f.sys, 2, 1, 1},
	} {
		counts, err := r.stats.ChangeRequestStateCounts(tc.ctx, crScopeProjectA)
		if err != nil {
			t.Fatalf("ChangeRequestStateCounts as %s: %v", tc.who, err)
		}
		total := 0
		for _, c := range counts {
			total += c.Count
		}
		if total != tc.wantTotal {
			t.Fatalf("state counts as %s = %v, want a total of %d", tc.who, counts, tc.wantTotal)
		}
		out, err := r.stats.OutstandingCounts(tc.ctx, crScopeProjectA, []string{"OPEN"}, outstanding)
		if err != nil {
			t.Fatalf("OutstandingCounts as %s: %v", tc.who, err)
		}
		if out["change_request"] != tc.wantOutstnd {
			t.Fatalf("outstanding change requests as %s = %d, want %d", tc.who, out["change_request"], tc.wantOutstnd)
		}
	}
	// The resolved buckets: the closed one nobody was asked about is not Alice's.
	f.execSQL(`UPDATE change_request SET closed_on = now() WHERE id = $1`, other)
	for _, tc := range []struct {
		who  string
		ctx  context.Context
		want int
	}{{"alice", alice, 0}, {"sam", sam, 0}, {"staff", f.sys, 1}} {
		month, last30, err := r.stats.ChangeRequestResolvedBuckets(tc.ctx, crScopeProjectA, "CLOSED")
		if err != nil {
			t.Fatalf("ChangeRequestResolvedBuckets as %s: %v", tc.who, err)
		}
		if month != tc.want || last30 != tc.want {
			t.Fatalf("resolved buckets as %s = %d / %d, want %d", tc.who, month, last30, tc.want)
		}
	}
}

// The case endpoints take an id and act on whatever work item it names, so a
// change request's id can be passed where a case's is expected -- the customer
// portal's POST /cases/{id}/comments forwards it as it is. A change request the
// caller may not see does not exist for those operations either: comments
// (write and read), tags, the watch list and the work-item attachments are 404,
// and a change request may not be re-parented as a case. A designated customer
// keeps what they had.
func TestChangeRequestVisibilityIntegration_CaseEndpointsCannotReachAHiddenChangeRequest(t *testing.T) {
	f := newCustomerGroupFlow(t)
	vis := visStrictSinceLongAgo()
	f.useVisibility(vis)
	cases := repository.NewCaseRepository(f.scoped, vis)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	alice := asContact(crScopeUserA1)

	hidden := func(when string) {
		t.Helper()
		_, err := cases.CreateCaseComment(alice, domain.CreateCaseCommentRequest{CaseID: id, CreatedBy: crFlowEmail(crScopeUserA1), Type: domain.CommentTypeComment, Content: "on a change request I cannot see"}, nil)
		f.wantNotFound(when+": CreateCaseComment", err)
		_, _, err = cases.SearchCaseComments(alice, domain.SearchCaseCommentsRequest{CaseID: id, Pagination: domain.Pagination{Limit: 10}})
		f.wantNotFound(when+": SearchCaseComments", err)
		_, _, err = cases.SearchWorkItemAttachments(alice, id, domain.ReferenceTypeChangeRequest, domain.Pagination{Limit: 10})
		f.wantNotFound(when+": SearchWorkItemAttachments", err)
		_, err = cases.AddCaseTag(alice, id, "a-tag", crFlowEmail(crScopeUserA1))
		f.wantNotFound(when+": AddCaseTag", err)
		f.wantNotFound(when+": RemoveCaseTag", cases.RemoveCaseTag(alice, id, "00000000-0000-0000-0000-000000000001", ""))
		_, _, err = cases.SetCaseWatchList(alice, id, nil, crFlowEmail(crScopeUserA1))
		f.wantNotFound(when+": SetCaseWatchList", err)
	}
	hidden("in New (nobody was asked)")
	var comments int
	if err := f.scoped.QueryRow(f.sys, `SELECT COUNT(*) FROM comment WHERE work_item_id = $1`, id).Scan(&comments); err != nil {
		t.Fatalf("count comments: %v", err)
	}
	if comments != 0 {
		t.Fatalf("a refused comment was written (%d comments)", comments)
	}

	f.driveToCustomerApproval(id)
	// Designated: the same calls are no longer refused for being hidden.
	if _, err := cases.CreateCaseComment(alice, domain.CreateCaseCommentRequest{CaseID: id, CreatedBy: crFlowEmail(crScopeUserA1), Type: domain.CommentTypeComment, Content: "a question for WSO2"}, nil); err != nil {
		t.Fatalf("CreateCaseComment as a designated contact: %v", err)
	}
	if _, total, err := cases.SearchCaseComments(alice, domain.SearchCaseCommentsRequest{CaseID: id, Pagination: domain.Pagination{Limit: 10}}); err != nil || total != 1 {
		t.Fatalf("SearchCaseComments as a designated contact = %d, %v", total, err)
	}
	if _, _, err := cases.SearchWorkItemAttachments(alice, id, domain.ReferenceTypeChangeRequest, domain.Pagination{Limit: 10}); err != nil {
		t.Fatalf("SearchWorkItemAttachments as a designated contact: %v", err)
	}
	// Sam is a registered contact of the project who was never asked.
	sam := asContact(crScopeUserSecurity)
	_, err := cases.CreateCaseComment(sam, domain.CreateCaseCommentRequest{CaseID: id, CreatedBy: crFlowEmail(crScopeUserSecurity), Type: domain.CommentTypeComment, Content: "x"}, nil)
	f.wantNotFound("CreateCaseComment as a contact who was never asked", err)

	// A change request is not a case: it cannot be re-parented through the case
	// endpoint, by anybody.
	for name, ctx := range map[string]context.Context{"staff": f.sys, "a designated contact": alice} {
		_, err := cases.UpdateCaseParent(ctx, id, crScopeProjectA, crFlowEmail(crFlowCreatorID))
		f.wantNotFound("UpdateCaseParent of a change request by "+name, err)
	}
}

// Edges of "a registered contact of the change request's current project": a
// contact whose membership is deactivated stops seeing what they were asked about
// (and sees it again when they are re-registered: the designation never left);
// the email is matched case-insensitively; and a change request with no project
// is nobody's.
func TestChangeRequestVisibilityIntegration_MembershipAndEmailEdges(t *testing.T) {
	f := newCustomerGroupFlow(t)
	vis := visStrictSinceLongAgo()
	f.useVisibility(vis)
	r := f.visRepos(vis)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.driveToCustomerApproval(id)
	f.phase(r, id, crScopeProjectA, "in Customer Approval", "alice", "bob")

	// Alice's membership of the project is deactivated: she no longer sees it, Bob does.
	f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED' WHERE LOWER(email) = LOWER($1)`, crFlowEmail(crScopeUserA1))
	f.phase(r, id, crScopeProjectA, "after Alice's membership was deactivated", "bob")
	// Re-registered: the designation was never lost.
	f.execSQL(`UPDATE project_contact SET state = 'REGISTERED' WHERE LOWER(email) = LOWER($1)`, crFlowEmail(crScopeUserA1))
	f.phase(r, id, crScopeProjectA, "after Alice was re-registered", "alice", "bob")

	// The viewer's email in another case: the same person (the identity's own
	// membership plumbing lower-cases too; padding is trimmed by the visibility
	// rule but is not something the identity middleware ever produces).
	shout := persona{"alice, shouting", repository.WithCallerIdentity(context.Background(),
		repository.SearchScope{ViewerEmail: strings.ToUpper(crFlowEmail(crScopeUserA1))})}
	f.wantSeen(r, shout, id, crScopeProjectA, "an upper-case email", true)

	// A change request with no project: legacy or strict, a customer cannot see it.
	orphan := f.createWithProject(domain.ChangeRequestTypeNormal, nil, false, false)
	f.setStoredState(orphan, "SCHEDULED")
	for _, v := range []repository.CRVisibility{{}, vis} {
		rv := f.visRepos(v)
		for _, who := range []string{"alice", "bob", "sam", "carol"} {
			if _, err := rv.cr.GetChangeRequestByID(f.personas()[who].ctx, orphan); err == nil {
				t.Fatalf("%s was handed a change request with no project", who)
			} else {
				f.wantNotFound("a change request with no project as "+who, err)
			}
		}
	}
}

// The creator of a change request who is also a registered contact of its project
// (a WSO2 engineer registered on a customer project, say) is listed on the
// customer stage as Cancelled -- never asked, as on every other stage -- and a
// listed row designates, so the change request is theirs to see; they still may
// not answer it.
func TestChangeRequestVisibilityIntegration_TheCreatorWhoIsAContact(t *testing.T) {
	f := newCustomerGroupFlow(t)
	vis := visStrictSinceLongAgo()
	f.useVisibility(vis)
	r := f.visRepos(vis)
	f.registerContact(crScopeProjectA, crScopeAccountID, crFlowCreatorID)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	creator := persona{"the creator", asContact(crFlowCreatorID)}

	f.wantSeen(r, creator, id, crScopeProjectA, "in New, before anybody was asked", false)
	f.driveToCustomerApproval(id)
	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers,
		map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED", crFlowCreatorID: "CANCELLED"})
	f.wantSeen(r, creator, id, crScopeProjectA, "in Customer Approval, listed Cancelled", true)
	cr, err := r.cr.GetChangeRequestByID(creator.ctx, id)
	if err != nil {
		t.Fatalf("GET as the creator: %v", err)
	}
	if cr.CustomerCanAnswer == nil || *cr.CustomerCanAnswer {
		t.Fatalf("customerCanAnswer for the creator = %v, want false", cr.CustomerCanAnswer)
	}
	if _, err := f.approveAs(id, crFlowCreatorID, true); err == nil {
		t.Fatal("the creator answered their own change request")
	}
}
