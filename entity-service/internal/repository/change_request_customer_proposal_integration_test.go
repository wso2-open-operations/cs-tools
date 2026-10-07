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
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// A customer proposes a new implementation time with PATCH {plannedStartOn,
// plannedEndOn}: the whole window, moved with its length kept, which is how a
// customer re-plans a change (a start alone is refused as soon as it passes the
// stored end, see the bad-window test below). Same harness as the other
// customer-outcome tests; the proposal is the Re-schedule of every change type
// started by a contact.

// The proposal of a whole window, in each of the date formats the portal can
// send, for each change type: the window is applied as given and its length is
// unchanged, the customer's pending request is superseded, and -- per type --
// the change goes back through CAB / ECAB (Normal / Emergency) or stays in
// Customer Approval with the customer asked again (Standard).
func TestChangeRequestCustomerProposalIntegration_WholeWindowKeepsTheDuration(t *testing.T) {
	// What the stored window reads back as after a proposal: RFC 3339, UTC.
	const wantStart, wantEnd = "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"
	for _, tc := range []struct {
		name       string
		typ        domain.ChangeRequestType
		start, end string // as the customer sends them
	}{
		{"Normal, RFC 3339 in UTC", domain.ChangeRequestTypeNormal, "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"},
		{"Emergency, the webapp's 'YYYY-MM-DD HH:MM:SS' (UTC)", domain.ChangeRequestTypeEmergency, "2030-03-08 09:00:00", "2030-03-08 11:00:00"},
		{"Standard, RFC 3339 with an offset", domain.ChangeRequestTypeStandard, "2030-03-08T14:30:00+05:30", "2030-03-08T16:30:00+05:30"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
			id := f.createWithProject(tc.typ, sp(crScopeProjectA), true, false)
			f.setPlanned(id, rsStart1, rsEnd1)
			before := windowOf(t, f.get(id))

			var cabApprover, wantStages, wantStagesAfterApproval string
			switch tc.typ {
			case domain.ChangeRequestTypeNormal:
				f.driveToCustomerApproval(id)
				cabApprover = crCABMemberUserID1
				wantStages = "Peer Approval,CAB Approval,Customer Approval,CAB Approval"
				wantStagesAfterApproval = "Peer Approval,CAB Approval,Customer Approval,CAB Approval,Customer Approval"
			case domain.ChangeRequestTypeEmergency:
				f.requestApproval(id)
				if err := f.decide(id, crECABMemberUserID, "approved"); err != nil {
					t.Fatalf("ECAB approval: %v", err)
				}
				f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
				cabApprover = crECABMemberUserID
				wantStages = "ECAB Approval,Customer Approval,ECAB Approval"
				wantStagesAfterApproval = "ECAB Approval,Customer Approval,ECAB Approval,Customer Approval"
			default:
				f.requestApproval(id)
				f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
				wantStages = "Customer Approval,Customer Approval"
			}
			f.wantCanAnswer(id, "before the proposal", true, crScopeUserA1, crScopeUserA2)

			if _, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(tc.start), PlannedEndOn: sp(tc.end)}); err != nil {
				t.Fatalf("proposal: %v", err)
			}
			f.wantPlanned(id, "after the proposal", wantStart, wantEnd)
			if after := windowOf(t, f.get(id)); after != before {
				t.Fatalf("the window's length changed from %v to %v", before, after)
			}
			if got := f.stageLabels(id); got != wantStages {
				t.Fatalf("stages after the proposal = %s, want %s", got, wantStages)
			}
			// The customer's request they were just asked is superseded: its rows are
			// cancelled, whatever the type.
			custom := f.customerStages(id)
			assertApprovers(t, "the superseded customer request", custom[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
			if a, _ := f.customerOutcome(id); a {
				t.Fatal("a proposal stamped the customer's approval")
			}

			if tc.typ == domain.ChangeRequestTypeStandard {
				// Nothing internal to repeat: still in Customer Approval, asked again.
				f.expect(id, "after the proposal", "CUSTOMER_APPROVAL", "authorize", "canceled")
				assertApprovers(t, "the fresh customer request", custom[1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
				f.wantCanAnswer(id, "after the proposal", true, crScopeUserA1, crScopeUserA2)
			} else {
				f.expect(id, "after the proposal", "AUTHORIZE", "canceled")
				f.wantCanAnswer(id, "while the new plan awaits internal approval", false, crScopeUserA1, crScopeUserA2)
				if err := f.decide(id, cabApprover, "approved"); err != nil {
					t.Fatalf("approval of the new plan: %v", err)
				}
				f.expect(id, "after the new plan is approved", "CUSTOMER_APPROVAL", "authorize", "canceled")
				if got := f.stageLabels(id); got != wantStagesAfterApproval {
					t.Fatalf("stages after the new plan is approved = %s, want %s", got, wantStagesAfterApproval)
				}
				f.wantCanAnswer(id, "once the customer is asked again", true, crScopeUserA1, crScopeUserA2)
			}

			// The customer answers the new plan, and the window is the proposed one.
			if _, err := f.approveAs(id, crScopeUserA2, true); err != nil {
				t.Fatalf("approval of the new plan: %v", err)
			}
			f.expect(id, "after the customer approved the new plan", "SCHEDULED", "implement", "canceled")
			f.wantPlanned(id, "after the customer approved the new plan", wantStart, wantEnd)
		})
	}
}

// What a customer is told when the window they propose cannot be applied: a
// readable validation message, nothing about the database, and nothing changed.
func TestChangeRequestCustomerProposalIntegration_BadWindowMessages(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	stagesBefore := f.stageLabels(id)

	const (
		notChanged  = "re-scheduling requires a changed planned start or end: send plannedStartOn and/or plannedEndOn with a value different from the stored one"
		endsBefore  = "the planned start must not be after the planned end"
		notADate    = "plannedStartOn must be a valid date-time"
		endNotADate = "plannedEndOn must be a valid date-time"
		emptyWindow = "the planned start must not be the same as the planned end"
		inThePast   = "is in the past"
		nothingSent = "at least one field must be provided"
	)
	for _, tc := range []struct {
		name string
		req  domain.PatchChangeRequestRequest
		want string
	}{
		{"the stored window again", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart1), PlannedEndOn: sp(rsEnd1)}, notChanged},
		{"the stored start alone", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart1)}, notChanged},
		{"the stored end alone", domain.PatchChangeRequestRequest{PlannedEndOn: sp(rsEnd1)}, notChanged},
		{"the same instant in another offset", domain.PatchChangeRequestRequest{PlannedStartOn: sp("2030-03-01T14:30:00+05:30")}, notChanged},
		{"a window that ends before it starts", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart3), PlannedEndOn: sp(rsEnd1)}, endsBefore},
		{"a start after the stored end (the end is not moved for the customer)", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart3)}, endsBefore},
		{"an end before the stored start", domain.PatchChangeRequestRequest{PlannedEndOn: sp("2030-02-28T11:00:00Z")}, endsBefore},
		{"text that is not a date", domain.PatchChangeRequestRequest{PlannedStartOn: sp("next tuesday")}, notADate},
		{"an end that is not a date", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp("later")}, endNotADate},
		{"an empty start", domain.PatchChangeRequestRequest{PlannedStartOn: sp("")}, notADate},
		{"a window with no length", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsStart2)}, emptyWindow},
		{"a window with no length, in two spellings", domain.PatchChangeRequestRequest{PlannedStartOn: sp("2030-03-08 09:00:00"), PlannedEndOn: sp("2030-03-08T14:30:00+05:30")}, emptyWindow},
		{"a window in the past", domain.PatchChangeRequestRequest{PlannedStartOn: sp("2001-05-01T10:00:00Z"), PlannedEndOn: sp("2001-05-01T12:00:00Z")}, inThePast},
		{"an end in the past", domain.PatchChangeRequestRequest{PlannedEndOn: sp("2001-05-01T12:00:00Z")}, inThePast},
		{"nothing at all", domain.PatchChangeRequestRequest{}, nothingSent},
	} {
		_, err := f.patchAsContact(id, crScopeUserA1, tc.req)
		f.wantValidationError(tc.name, err, tc.want)
		if msg := err.Error(); strings.Contains(strings.ToLower(msg), "timestamptz") || strings.Contains(strings.ToLower(msg), "sqlstate") || strings.Contains(msg, "pq:") || strings.Contains(msg, "ERROR:") {
			t.Fatalf("%s: the message leaks the database: %q", tc.name, msg)
		}
	}

	// What was refused left the window as it was; a proposal that is fine still goes through.
	f.wantPlanned(id, "after the refused proposals", rsStart1, rsEnd1)
	if _, err := f.patchAsContact(id, crScopeUserA2, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2)}); err != nil {
		t.Fatalf("a proposal that is fine: %v", err)
	}

	// Everything refused above left the change as it was (the one proposal that
	// was accepted is the last step; the refusals ran against the stored window).
	if stagesBefore == f.stageLabels(id) {
		t.Fatal("the accepted proposal did not re-schedule the change")
	}
}

// Refused proposals change nothing: asserted separately from the message table
// so the check runs with no accepted proposal after it.
func TestChangeRequestCustomerProposalIntegration_RefusedWindowChangesNothing(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	stages := f.stageLabels(id)

	for _, req := range []domain.PatchChangeRequestRequest{
		{PlannedStartOn: sp(rsStart1), PlannedEndOn: sp(rsEnd1)},
		{PlannedStartOn: sp(rsStart3), PlannedEndOn: sp(rsEnd1)},
		{PlannedStartOn: sp(rsStart3)},
		{PlannedStartOn: sp("next tuesday")},
	} {
		if _, err := f.patchAsContact(id, crScopeUserA1, req); err == nil {
			t.Fatalf("%+v was accepted", req)
		}
	}
	f.expect(id, "after the refused proposals", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantPlanned(id, "after the refused proposals", rsStart1, rsEnd1)
	if got := f.stageLabels(id); got != stages {
		t.Fatalf("stages = %s, want them untouched (%s)", got, stages)
	}
	assertApprovers(t, "customer request untouched", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	f.wantCanAnswer(id, "after the refused proposals", true, crScopeUserA1, crScopeUserA2)
}

// What reaches the database as the planned window is a date-time this service
// parsed, never text for Postgres' own parser: 'infinity' (which stored fine and
// then made the change request unreadable for everyone, and its project's list a
// 500), 'now' / 'tomorrow' (relative to the server's clock), a bare date (midnight
// in the session's zone), a zone name, a year no date-time of ours has. Each is a
// readable 400 for a customer, as either bound, and nothing is stored: the change
// request still reads and the stages and window are what they were.
func TestChangeRequestCustomerProposalIntegration_HostileWindowsAreRefused(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	stages := f.stageLabels(id)

	for _, hostile := range []string{
		"infinity", "-infinity", "+infinity", "Infinity", "now", "today", "tomorrow", "yesterday", "epoch", "allballs",
		"2031-05-01",                           // a date alone
		"2031-05-01T10:00:00",                  // no zone, in the RFC 3339 shape
		"2031-05-01 10:00:00 America/New_York", // a zone name
		"2031-05-01 10:00:00 PST",
		"2031-5-1 10:00:00",
		"10000-01-01 00:00:00", "294276-12-31 23:59:59", "5874897-12-31 23:59:59",
		"1999-12-31T23:59:59Z", "2101-01-01T00:00:00Z", "0001-01-01T00:00:00Z",
		" 2031-05-01 10:00:00", "2031-05-01 10:00:00 ", "2031-05-01 10:00:00\x00", "",
		"2031-05-01 10:00:00'; DROP TABLE change_request; --",
	} {
		for _, req := range []domain.PatchChangeRequestRequest{
			{PlannedStartOn: sp(hostile)},
			{PlannedEndOn: sp(hostile)},
			{PlannedStartOn: sp("2031-05-01 10:00:00"), PlannedEndOn: sp(hostile)},
		} {
			_, err := f.patchAsContact(id, crScopeUserA1, req)
			if err == nil {
				t.Fatalf("%q was accepted as a proposed time (%+v)", hostile, req)
			}
			f.wantValidationError(hostile, err, "must be a valid date-time")
		}
	}

	f.expect(id, "after the refused proposals", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantPlanned(id, "after the refused proposals", rsStart1, rsEnd1)
	if got := f.stageLabels(id); got != stages {
		t.Fatalf("a refused proposal changed the stages: %s, was %s", got, stages)
	}
	f.wantCanAnswer(id, "after the refused proposals", true, crScopeUserA1, crScopeUserA2)
}

// A zone-less "YYYY-MM-DD HH:MM:SS" is UTC, whatever TimeZone the database
// session runs in: the same proposal is stored as the same instant on a UTC
// database and on one set to Colombo (+05:30), and an RFC 3339 value keeps its offset.
func TestChangeRequestCustomerProposalIntegration_ZonelessWindowIsUTCInAnySession(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	for _, zone := range []string{"UTC", "Asia/Colombo", "America/Los_Angeles"} {
		zone := zone
		t.Run(zone, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			cfg, err := pgxpool.ParseConfig(dsn)
			if err != nil {
				t.Fatalf("parse DSN: %v", err)
			}
			cfg.ConnConfig.RuntimeParams["timezone"] = zone
			pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
			if err != nil {
				t.Fatalf("connect: %v", err)
			}
			t.Cleanup(pool.Close)
			f.pool, f.scoped = pool, repository.NewScoped(pool)
			f.repo = repository.NewChangeRequestRepository(f.scoped)
			var session string
			if err := pool.QueryRow(context.Background(), `SHOW TimeZone`).Scan(&session); err != nil || session != zone {
				t.Fatalf("session TimeZone = %q (%v), want %q", session, err, zone)
			}

			id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
			f.setPlanned(id, rsStart1, rsEnd1)
			f.driveToCustomerApproval(id)

			if _, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{
				PlannedStartOn: sp("2031-05-01 10:00:00"), PlannedEndOn: sp("2031-05-01T21:30:00+05:30")}); err != nil {
				t.Fatalf("proposal: %v", err)
			}
			f.wantPlanned(id, "after the proposal", "2031-05-01T10:00:00Z", "2031-05-01T16:00:00Z")
		})
	}
}

// A row that already holds Postgres' infinity (from before the window was
// checked, or from a door other than this API) reads as having no planned time,
// instead of failing its whole read and the list it belongs to.
func TestChangeRequestCustomerProposalIntegration_NonFiniteRowStillReads(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.requestApproval(id)
	f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.execSQL(`UPDATE change_request SET start_on = '-infinity'::timestamptz, end_on = 'infinity'::timestamptz WHERE id = $1`, id)

	cr, err := f.getAsContact(id, crScopeUserA1)
	if err != nil {
		t.Fatalf("GET of a change request holding infinity: %v", err)
	}
	if cr.PlannedStartOn != nil || cr.PlannedEndOn != nil {
		t.Fatalf("planned window = %v .. %v, want none", cr.PlannedStartOn, cr.PlannedEndOn)
	}
	rows, _, err := f.repo.SearchChangeRequests(asContact(crScopeUserA1), domain.SearchChangeRequestsRequest{
		Filters:    domain.SearchChangeRequestsFilters{ProjectIDs: []string{crScopeProjectA}},
		Pagination: domain.Pagination{Offset: 0, Limit: 50},
	}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("the project's list with that row in it: %v", err)
	}
	found := false
	for _, row := range rows {
		found = found || row.ID == id
	}
	if !found {
		t.Fatalf("the change request is missing from the project's list of %d", len(rows))
	}
	// ...and the customer can still answer it.
	if _, err := f.approveAs(id, crScopeUserA1, true); err != nil {
		t.Fatalf("approving a change request that holds infinity: %v", err)
	}
	f.expect(id, "after approving", "SCHEDULED", "implement", "canceled")
}

// A WSO2 user's Re-schedule is held to the same date-times, but not to "still to
// come": re-planning a change whose window has gone by is theirs to do.
func TestChangeRequestCustomerProposalIntegration_StaffRescheduleSharesTheParserNotThePastRule(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, false)
	f.setPlanned(id, "2001-02-01T10:00:00Z", "2001-02-01T12:00:00Z")
	f.requestApproval(id)
	f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")

	err := f.reschedule(id, sp("infinity"), nil)
	f.wantValidationError("a staff Re-schedule to infinity", err, "plannedStartOn must be a valid date-time")
	err = f.reschedule(id, sp("2001-03-01T10:00:00Z"), sp("2001-03-01T10:00:00Z"))
	f.wantValidationError("a staff Re-schedule to an empty window", err, "the planned start must not be the same as the planned end")
	if err := f.reschedule(id, sp("2001-03-01T10:00:00Z"), sp("2001-03-01T12:00:00Z")); err != nil {
		t.Fatalf("a staff Re-schedule to a window in the past: %v", err)
	}
	f.wantPlanned(id, "after the staff Re-schedule", "2001-03-01T10:00:00Z", "2001-03-01T12:00:00Z")

	// A plain PATCH of the window (the Edit dialog) is parsed the same way.
	_, err = f.patch(id, domain.PatchChangeRequestRequest{PlannedEndOn: sp("infinity")})
	f.wantValidationError("an Edit of the end to infinity", err, "plannedEndOn must be a valid date-time")
}
