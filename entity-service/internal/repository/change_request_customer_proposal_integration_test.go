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
// plannedEndOn?}: a START, which waits in customer_updated_on for WSO2's answer while the
// planned window stays what WSO2 planned. The planned LENGTH is kept (a proposal is a
// start), so the end the portal's dialog derives may ride with the start, and it must be
// exactly start + length. Same harness as the other customer-outcome tests; the proposal
// is the same for every change type.

// The proposal of a whole window (the start plus the end that keeps the length), in each
// of the date formats the portal can send, for each change type: only customer_updated_on
// is written, the planned window and the customers' request are untouched, and accepting it
// applies the proposal with the length unchanged.
func TestChangeRequestCustomerProposalIntegration_WholeWindowKeepsTheDuration(t *testing.T) {
	// What the proposal reads back as: RFC 3339, UTC.
	const wantStart, wantEnd = "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"
	for _, tc := range []struct {
		name       string
		typ        domain.ChangeRequestType
		start, end string // as the customer sends them
	}{
		{"Normal, RFC 3339 in UTC", domain.ChangeRequestTypeNormal, "2030-03-08T09:00:00Z", "2030-03-08T11:00:00Z"},
		{"Normal, the webapp's 'YYYY-MM-DD HH:MM:SS' (UTC)", domain.ChangeRequestTypeNormal, "2030-03-08 09:00:00", "2030-03-08 11:00:00"},
		{"Standard, RFC 3339 with an offset", domain.ChangeRequestTypeStandard, "2030-03-08T14:30:00+05:30", "2030-03-08T16:30:00+05:30"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(tc.typ)
			before := windowOf(t, f.get(id))
			stages := f.stageLabels(id)
			f.wantCanAnswer(id, "before the proposal", true, crScopeUserA1, crScopeUserA2)

			if _, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(tc.start), PlannedEndOn: sp(tc.end)}); err != nil {
				t.Fatalf("proposal: %v", err)
			}
			f.wantConversation(id, "after the proposal", wantStart, "")
			f.wantPlanned(id, "after the proposal (the plan is not the proposal)", rsStart1, rsEnd1)
			f.expect(id, "after the proposal", "CUSTOMER_APPROVAL", "authorize", "canceled")
			if got := f.stageLabels(id); got != stages {
				t.Fatalf("stages after the proposal = %s, want them unchanged (%s)", got, stages)
			}
			assertApprovers(t, "the customers' request after the proposal", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
			if a, _ := f.customerOutcome(id); a {
				t.Fatal("a proposal stamped the customer's approval")
			}
			f.wantCanAnswer(id, "after the proposal", true, crScopeUserA1, crScopeUserA2)

			// WSO2 accepts: the window is the proposed one and its length is unchanged.
			f.mustAccept(id)
			f.wantPlanned(id, "after Accept", wantStart, wantEnd)
			if after := windowOf(t, f.get(id)); after != before {
				t.Fatalf("the window's length changed from %v to %v", before, after)
			}
			f.expect(id, "after Accept", "SCHEDULED", "implement", "canceled")
		})
	}
}

// What a customer is told when the time they propose cannot be taken: a readable
// validation message, nothing about the database, and nothing changed.
func TestChangeRequestCustomerProposalIntegration_BadWindowMessages(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	stagesBefore := f.stageLabels(id)

	const (
		isPlanned   = "plannedStartOn is the planned start already: propose a different start"
		keepsLength = "keeps the planned length of 2 hours"
		needsStart  = "a proposed implementation time needs a new start: send plannedStartOn"
		notADate    = "plannedStartOn must be a valid date-time"
		endNotADate = "plannedEndOn must be a valid date-time"
		startPast   = "plannedStartOn is in the past"
		endPast     = "plannedEndOn is in the past"
		nothingSent = "at least one field must be provided"
	)
	for _, tc := range []struct {
		name string
		req  domain.PatchChangeRequestRequest
		want string
	}{
		{"the stored window again", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart1), PlannedEndOn: sp(rsEnd1)}, isPlanned},
		{"the stored start alone", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart1)}, isPlanned},
		{"the stored end alone", domain.PatchChangeRequestRequest{PlannedEndOn: sp(rsEnd1)}, needsStart},
		{"the same instant in another offset", domain.PatchChangeRequestRequest{PlannedStartOn: sp("2030-03-01T14:30:00+05:30")}, isPlanned},
		{"a window that ends before it starts", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart3), PlannedEndOn: sp(rsEnd1)}, keepsLength},
		{"an end that is not the start plus the length", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd3)}, keepsLength},
		{"an end before the stored start, alone", domain.PatchChangeRequestRequest{PlannedEndOn: sp("2030-02-28T11:00:00Z")}, needsStart},
		{"text that is not a date", domain.PatchChangeRequestRequest{PlannedStartOn: sp("next tuesday")}, notADate},
		{"an end that is not a date", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp("later")}, endNotADate},
		{"an empty start", domain.PatchChangeRequestRequest{PlannedStartOn: sp("")}, notADate},
		{"a window with no length", domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsStart2)}, keepsLength},
		{"a window with no length, in two spellings", domain.PatchChangeRequestRequest{PlannedStartOn: sp("2030-03-08 09:00:00"), PlannedEndOn: sp("2030-03-08T14:30:00+05:30")}, keepsLength},
		{"a window in the past", domain.PatchChangeRequestRequest{PlannedStartOn: sp("2001-05-01T10:00:00Z"), PlannedEndOn: sp("2001-05-01T12:00:00Z")}, startPast},
		{"an end in the past", domain.PatchChangeRequestRequest{PlannedEndOn: sp("2001-05-01T12:00:00Z")}, endPast},
		{"nothing at all", domain.PatchChangeRequestRequest{}, nothingSent},
	} {
		_, err := f.patchAsContact(id, crScopeUserA1, tc.req)
		f.wantValidationError(tc.name, err, tc.want)
		if msg := err.Error(); strings.Contains(strings.ToLower(msg), "timestamptz") || strings.Contains(strings.ToLower(msg), "sqlstate") || strings.Contains(msg, "pq:") || strings.Contains(msg, "ERROR:") {
			t.Fatalf("%s: the message leaks the database: %q", tc.name, msg)
		}
	}

	// What was refused left the change as it was; a proposal that is fine still goes through.
	f.wantPlanned(id, "after the refused proposals", rsStart1, rsEnd1)
	f.wantConversation(id, "after the refused proposals", "", "")
	if _, err := f.patchAsContact(id, crScopeUserA2, domain.PatchChangeRequestRequest{PlannedStartOn: sp(rsStart2), PlannedEndOn: sp(rsEnd2)}); err != nil {
		t.Fatalf("a proposal that is fine: %v", err)
	}
	f.wantConversation(id, "after the proposal that is fine", rsStart2, "")
	if got := f.stageLabels(id); got != stagesBefore {
		t.Fatalf("the proposal changed the stages: %s, was %s (nothing but customer_updated_on is written)", got, stagesBefore)
	}
}

// Refused proposals change nothing: asserted separately from the message table
// so the check runs with no accepted proposal after it.
func TestChangeRequestCustomerProposalIntegration_RefusedWindowChangesNothing(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	before := f.snap(id)

	for _, req := range []domain.PatchChangeRequestRequest{
		{PlannedStartOn: sp(rsStart1), PlannedEndOn: sp(rsEnd1)},
		{PlannedStartOn: sp(rsStart3), PlannedEndOn: sp(rsEnd1)},
		{PlannedEndOn: sp(rsEnd3)},
		{PlannedStartOn: sp("next tuesday")},
	} {
		if _, err := f.patchAsContact(id, crScopeUserA1, req); err == nil {
			t.Fatalf("%s %s was accepted", derefStr(req.PlannedStartOn), derefStr(req.PlannedEndOn))
		}
	}
	if after := f.snap(id); after != before {
		t.Fatalf("a refused proposal changed the change request:\n  before: %s\n  after:  %s", before, after)
	}
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
				PlannedStartOn: sp("2031-05-01 10:00:00"), PlannedEndOn: sp("2031-05-01T17:30:00+05:30")}); err != nil {
				t.Fatalf("proposal: %v", err)
			}
			f.wantConversation(id, "after the proposal", "2031-05-01T10:00:00Z", "")
			f.wantPlanned(id, "after the proposal", rsStart1, rsEnd1)
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
