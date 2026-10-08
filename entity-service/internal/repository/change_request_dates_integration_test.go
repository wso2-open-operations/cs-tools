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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The planned window on EVERY path that writes it: what reaches Postgres is a
// date-time parsed in Go (change_request_window.go), never text for the
// database's own date parser. The customer proposal and the Re-schedule have their
// own tests (change_request_customer_proposal_integration_test.go); these cover
// the staff PATCH, both creates (the portal's and the ServiceNow-first one, which
// used to hand the raw text to `::text::timestamptz`), and the proposal's
// effective start.

// hostileWindowTexts is what Postgres' parser would have taken and what a person
// might type: relative words, infinity, epoch, an impossible date, an empty or
// blank value, a date alone, a year past four digits, prose and a SQL fragment.
var hostileWindowTexts = []string{
	"now", "tomorrow", "today", "yesterday", "epoch", "infinity", "-infinity", "+infinity", "allballs",
	"2026-13-45", "2030-02-30 09:00:00", "", "   ", "\t", "2030-03-01", "2030-03-01T09:00:00", "99999-01-01 00:00:00",
	"in 3 days", "next friday", "1 week", "2030-03-01 09:00:00 America/New_York",
	"2030-03-01 09:00:00'; DROP TABLE change_request; --", "2030-03-01 09:00:00\x00", "1999-12-31 23:59:59", "2101-01-01 00:00:00",
}

func (f *crFlow) countChangeRequests() int {
	f.t.Helper()
	var n int
	if err := f.scoped.QueryRow(f.sys, `SELECT COUNT(*) FROM work_item WHERE subject = $1`, crFlowSubject).Scan(&n); err != nil {
		f.t.Fatalf("count change requests: %v", err)
	}
	return n
}

// A staff PATCH of either bound, as the form sends it: refused with a readable
// 400 that names the field, and the change request is left exactly as it was.
func TestChangeRequestDatesIntegration_PatchRefusesWhatPostgresWouldHaveParsed(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	for _, hostile := range hostileWindowTexts {
		for field, req := range map[string]domain.PatchChangeRequestRequest{
			"plannedStartOn": {PlannedStartOn: sp(hostile), Title: sp("must not be written")},
			"plannedEndOn":   {PlannedEndOn: sp(hostile), Title: sp("must not be written")},
		} {
			_, err := f.patch(id, req)
			if err == nil {
				t.Fatalf("PATCH %s = %q was accepted", field, hostile)
			}
			f.wantValidationError(field+" = "+hostile, err, field+" must be a valid date-time")
			if low := strings.ToLower(err.Error()); strings.Contains(low, "sqlstate") || strings.Contains(low, "timestamptz") {
				t.Fatalf("PATCH %s = %q leaks the database: %q", field, hostile, err.Error())
			}
		}
	}
	f.wantPlanned(id, "after the refused PATCHes", rsStart1, rsEnd1)
	if got := f.subjectOf(id); got != crFlowSubject {
		t.Fatalf("a refused PATCH wrote the title: %q", got)
	}
	// Staff Re-schedule is the same entry.
	f.driveToCustomerApproval(id)
	for _, hostile := range hostileWindowTexts {
		if err := f.reschedule(id, sp(hostile), nil); err == nil {
			t.Fatalf("a Re-schedule to %q was accepted", hostile)
		}
	}
	f.expect(id, "after the refused Re-schedules", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantPlanned(id, "after the refused Re-schedules", rsStart1, rsEnd1)
}

// The two accepted layouts are stored as the same UTC instant on a PATCH.
func TestChangeRequestDatesIntegration_PatchAcceptsRFC3339AndTheZonelessLayoutAsUTC(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	for _, tc := range []struct{ start, end, wantStart, wantEnd string }{
		{"2031-05-01T10:00:00Z", "2031-05-01T12:00:00Z", "2031-05-01T10:00:00Z", "2031-05-01T12:00:00Z"},
		{"2031-05-01 10:00:00", "2031-05-01 12:00:00", "2031-05-01T10:00:00Z", "2031-05-01T12:00:00Z"},
		{"2031-05-01T15:30:00+05:30", "2031-05-01T18:30:00+05:30", "2031-05-01T10:00:00Z", "2031-05-01T13:00:00Z"},
		{"2000-01-01 00:00:00", "2100-12-31 23:59:59", "2000-01-01T00:00:00Z", "2100-12-31T23:59:59Z"},
	} {
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{PlannedStartOn: sp(tc.start), PlannedEndOn: sp(tc.end)}); err != nil {
			t.Fatalf("PATCH %s .. %s: %v", tc.start, tc.end, err)
		}
		f.wantPlanned(id, "after "+tc.start, tc.wantStart, tc.wantEnd)
	}
}

// The portal create and the ServiceNow-first create take the same window rules:
// each hostile text is refused for either bound, and NOTHING is created.
func TestChangeRequestDatesIntegration_BothCreatesRefuseWhatPostgresWouldHaveParsed(t *testing.T) {
	f := newCustomerGroupFlow(t)
	typ := domain.ChangeRequestTypeNormal
	before := f.countChangeRequests()
	n := 0
	for _, hostile := range hostileWindowTexts {
		for field, mod := range map[string]func(*domain.CreateChangeRequestRequest){
			"plannedStartDate": func(r *domain.CreateChangeRequestRequest) { r.PlannedStartDate = sp(hostile) },
			"plannedEndDate":   func(r *domain.CreateChangeRequestRequest) { r.PlannedEndDate = sp(hostile) },
		} {
			req := domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &typ}
			mod(&req)
			_, err := f.repo.CreateChangeRequest(f.sys, req, crFlowEmail(crFlowCreatorID))
			f.wantValidationError("portal create "+field+" = "+hostile, err, field+" must be a valid date-time")
			n++
			_, err = f.repo.CreateChangeRequestFromServiceNow(f.sys, req, "3aaaaaaa-0000-0000-0000-0000000000f8", "CRFLOWSN008", "x@example.com")
			f.wantValidationError("ServiceNow-first create "+field+" = "+hostile, err, field+" must be a valid date-time")
		}
	}
	if after := f.countChangeRequests(); after != before {
		t.Fatalf("%d change requests exist after %d refused creates, want the %d there were", after, n*2, before)
	}
}

// Valid windows on both creates are stored as the parsed UTC instant.
func TestChangeRequestDatesIntegration_BothCreatesStoreTheParsedWindow(t *testing.T) {
	f := newCustomerGroupFlow(t)
	typ := domain.ChangeRequestTypeNormal
	for i, tc := range []struct{ start, end, wantStart, wantEnd string }{
		{"2031-05-01T10:00:00Z", "2031-05-01T12:00:00Z", "2031-05-01T10:00:00Z", "2031-05-01T12:00:00Z"},
		{"2031-05-01 10:00:00", "2031-05-01 12:00:00", "2031-05-01T10:00:00Z", "2031-05-01T12:00:00Z"},
		{"2031-05-01T15:30:00+05:30", "2031-05-01T18:30:00+05:30", "2031-05-01T10:00:00Z", "2031-05-01T13:00:00Z"},
	} {
		req := domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &typ, PlannedStartDate: sp(tc.start), PlannedEndDate: sp(tc.end)}
		portal, err := f.repo.CreateChangeRequest(f.sys, req, crFlowEmail(crFlowCreatorID))
		if err != nil {
			t.Fatalf("portal create %s: %v", tc.start, err)
		}
		f.wantPlanned(portal.ChangeRequest.ID, "after the portal create", tc.wantStart, tc.wantEnd)
		snID := "3aaaaaaa-0000-0000-0000-0000000000e" + string(rune('0'+i))
		sn, err := f.repo.CreateChangeRequestFromServiceNow(f.sys, req, snID, "CRFLOWSN01"+string(rune('0'+i)), "x@example.com")
		if err != nil {
			t.Fatalf("ServiceNow-first create %s: %v", tc.start, err)
		}
		f.wantPlanned(sn.ChangeRequest.ID, "after the ServiceNow-first create", tc.wantStart, tc.wantEnd)
	}
}

// A customer's proposal is a START (customer_updated_on holds one instant): an end
// alone is no proposal -- the window the change has keeps its length, so there is nothing
// to move with it -- however far in the future that end is, and whatever has become of
// the stored start. A start still to come is a proposal, with or without the end that
// keeps the length.
func TestChangeRequestDatesIntegration_AnEndOnlyProposalCannotKeepAStartThatHasPassed(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, "2025-01-01T09:00:00Z", "2031-01-01T11:00:00Z") // started long ago, ends far ahead
	f.driveToCustomerApproval(id)
	stages := f.stageLabels(id)

	future := time.Now().UTC().AddDate(0, 6, 0).Truncate(time.Second)
	_, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedEndOn: sp(future.Add(2 * time.Hour).Format(time.RFC3339))})
	f.wantValidationError("an end-only proposal over a start that has passed", err, "a proposed implementation time needs a new start: send plannedStartOn")
	f.expect(id, "after the refused proposal", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantPlanned(id, "after the refused proposal", "2025-01-01T09:00:00Z", "2031-01-01T11:00:00Z")
	f.wantConversation(id, "after the refused proposal", "", "")
	if got := f.stageLabels(id); got != stages {
		t.Fatalf("a refused proposal changed the stages: %s, was %s", got, stages)
	}
	// A start that is still to come is a start-only proposal: the planned length is kept.
	cr, err := f.patchAsContact(id, crScopeUserA1, domain.PatchChangeRequestRequest{PlannedStartOn: sp(future.Format(time.RFC3339))})
	if err != nil {
		t.Fatalf("a start-only proposal: %v", err)
	}
	f.expect(id, "after the accepted proposal", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantConversation(id, "after the accepted proposal", future.Format(time.RFC3339), "")
	f.wantPlanned(id, "after the accepted proposal", "2025-01-01T09:00:00Z", "2031-01-01T11:00:00Z")
	if p := cr.CustomerProposal; p == nil || p.Answer != "pending" || p.EndOn == nil {
		t.Fatalf("customerProposal after the proposal = %+v, want a pending proposal with the derived end", p)
	}
	// The end that keeps the length may ride with the start; any other end is refused.
	f2 := newCustomerGroupFlow(t)
	id2 := f2.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	if _, err := f2.patchAsContact(id2, crScopeUserA1, domain.PatchChangeRequestRequest{
		PlannedStartOn: sp(future.Format(time.RFC3339)), PlannedEndOn: sp(future.Add(2 * time.Hour).Format(time.RFC3339))}); err != nil {
		t.Fatalf("a proposal naming both bounds: %v", err)
	}
	f2.wantConversation(id2, "after the proposal naming both bounds", future.Format(time.RFC3339), "")
}

// ... and a start in the past is refused for a customer whichever bounds ride
// with it, while staff may record one (a Re-schedule to a time that has gone).
func TestChangeRequestDatesIntegration_PastStartIsRefusedForCustomersOnly(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	future := time.Now().UTC().AddDate(0, 6, 0).Truncate(time.Second).Format(time.RFC3339)
	for name, req := range map[string]domain.PatchChangeRequestRequest{
		"a past start alone":            {PlannedStartOn: sp("2001-05-01T10:00:00Z")},
		"a past start and a future end": {PlannedStartOn: sp("2001-05-01T10:00:00Z"), PlannedEndOn: sp(future)},
		"a start of now":                {PlannedStartOn: sp(time.Now().UTC().Add(-time.Second).Format(time.RFC3339))},
	} {
		_, err := f.patchAsContact(id, crScopeUserA1, req)
		f.wantValidationError(name, err, "plannedStartOn is in the past")
	}
	f.wantPlanned(id, "after the refused proposals", rsStart1, rsEnd1)
	if err := f.reschedule(id, sp("2001-05-01T10:00:00Z"), sp("2001-05-01T12:00:00Z")); err != nil {
		t.Fatalf("staff Re-schedule to a window in the past: %v", err)
	}
}
