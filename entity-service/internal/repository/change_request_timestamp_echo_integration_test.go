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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// What the change request prints is what a client sends back. The planned window, the customer's
// proposed start and the work window are timestamptz values with MICROSECONDS (the write path keeps
// them: a browser that computes "now plus an hour" sends milliseconds), and every answer that names
// the window it was given for (a customer's approval, Accept proposed time, Propose a different
// time, a decline, a Re-schedule) compares what it names with what is stored, exactly. A read that
// printed whole seconds only would therefore make such a window unanswerable for ever: the client
// echoes "...:00Z" and the stored value is "...:00.123456Z" (409 change_request_schedule_changed on
// every attempt, however often the page is refreshed). The read prints the fraction when there is
// one (RFC 3339 with the fractional digits it needs, UTC) and nothing else changes: a value that
// holds whole seconds, which is every value the previous system wrote and every migrated row, reads
// as it always did.

const (
	fracStart = "2030-03-01T09:00:00.123456Z"
	fracEnd   = "2030-03-01T11:00:00.654321Z"
	// fracProposal is a start a customer proposes with half a second in it.
	fracProposal = "2030-03-08T09:00:00.5Z"
)

// fractionalInCustomerApproval is a change in Customer Approval whose planned window carries
// microseconds, as a client that wrote one through the PATCH leaves it.
func fractionalInCustomerApproval(t *testing.T) (*crFlow, string) {
	t.Helper()
	f := newCustomerGroupFlow(t)
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.setPlanned(id, fracStart, fracEnd)
	return f, id
}

func TestChangeRequestTimestampEchoIntegration_TheReadPrintsWhatTheWritePathKept(t *testing.T) {
	t.Run("a window written through the PATCH is read back as it was written", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
		cr, err := f.patch(id, domain.PatchChangeRequestRequest{PlannedStartOn: sp(fracStart), PlannedEndOn: sp(fracEnd)})
		if err != nil {
			t.Fatalf("writing a window with microseconds: %v", err)
		}
		// The receipt of the PATCH is the same read.
		if cr.PlannedStartOn == nil || *cr.PlannedStartOn != fracStart || cr.PlannedEndOn == nil || *cr.PlannedEndOn != fracEnd {
			t.Fatalf("the receipt = %v .. %v, want %s .. %s", cr.PlannedStartOn, cr.PlannedEndOn, fracStart, fracEnd)
		}
		f.wantPlanned(id, "on a fresh read", fracStart, fracEnd)
		// Fewer digits than six are not padded: the shortest form that says the instant.
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{PlannedStartOn: sp("2030-03-01T09:00:00.100Z"), PlannedEndOn: sp("2030-03-01T11:00:00.000Z")}); err != nil {
			t.Fatalf("writing a window with milliseconds: %v", err)
		}
		f.wantPlanned(id, "after writing milliseconds", "2030-03-01T09:00:00.1Z", "2030-03-01T11:00:00Z")
		// Beyond microseconds nothing is stored, and nothing is printed that was not stored.
		if _, err := f.patch(id, domain.PatchChangeRequestRequest{PlannedStartOn: sp("2030-03-01T09:00:00.1234567Z")}); err != nil {
			t.Fatalf("writing a window with a seventh digit: %v", err)
		}
		f.wantPlanned(id, "after writing a seventh digit", "2030-03-01T09:00:00.123456Z", "2030-03-01T11:00:00Z")
	})

	t.Run("the work window, which the sync and the previous system maintain, prints its fraction too", func(t *testing.T) {
		f, id := fractionalInCustomerApproval(t)
		f.execSQL(`UPDATE change_request SET work_start_on = $2::text::timestamptz, work_end_on = $3::text::timestamptz WHERE id = $1`, id, fracStart, fracEnd)
		cr := f.get(id)
		if cr.WorkStart == nil || *cr.WorkStart != fracStart || cr.WorkEnd == nil || *cr.WorkEnd != fracEnd {
			t.Fatalf("the work window reads %v .. %v, want %s .. %s", cr.WorkStart, cr.WorkEnd, fracStart, fracEnd)
		}
	})

	t.Run("the list prints the same as the detail", func(t *testing.T) {
		f, id := fractionalInCustomerApproval(t)
		rows, _, err := f.repo.SearchChangeRequests(asContact(crScopeUserA1), domain.SearchChangeRequestsRequest{
			Filters:    domain.SearchChangeRequestsFilters{ProjectIDs: []string{crScopeProjectA}},
			Pagination: domain.Pagination{Offset: 0, Limit: 50},
		}, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("the project's list: %v", err)
		}
		for _, row := range rows {
			if row.ID != id {
				continue
			}
			if row.PlannedStartOn == nil || *row.PlannedStartOn != fracStart || row.PlannedEndOn == nil || *row.PlannedEndOn != fracEnd {
				t.Fatalf("the list shows %v .. %v, want %s .. %s", row.PlannedStartOn, row.PlannedEndOn, fracStart, fracEnd)
			}
			return
		}
		t.Fatalf("the change request is missing from the project's list of %d", len(rows))
	})

	t.Run("a migrated row holds whole seconds and reads as it always did", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.migratedInCustomerApproval()
		f.syncWritesConversation(id, sp(rsStart2), "")
		f.execSQL(`UPDATE change_request SET work_start_on = $2::text::timestamptz, work_end_on = $3::text::timestamptz WHERE id = $1`, id, rsStart1, rsEnd1)
		cr := f.get(id)
		if cr.PlannedStartOn == nil || *cr.PlannedStartOn != rsStart1 || cr.PlannedEndOn == nil || *cr.PlannedEndOn != rsEnd1 {
			t.Fatalf("a migrated window reads %v .. %v, want %s .. %s", cr.PlannedStartOn, cr.PlannedEndOn, rsStart1, rsEnd1)
		}
		if cr.CustomerUpdatedOn == nil || *cr.CustomerUpdatedOn != rsStart2 {
			t.Fatalf("customerUpdatedOn = %v, want %s", cr.CustomerUpdatedOn, rsStart2)
		}
		if cr.WorkStart == nil || *cr.WorkStart != rsStart1 || cr.WorkEnd == nil || *cr.WorkEnd != rsEnd1 {
			t.Fatalf("work window = %v .. %v, want %s .. %s", cr.WorkStart, cr.WorkEnd, rsStart1, rsEnd1)
		}
		for _, s := range []*string{cr.PlannedStartOn, cr.PlannedEndOn, cr.CustomerUpdatedOn, cr.WorkStart, cr.WorkEnd} {
			if strings.Contains(*s, ".") {
				t.Fatalf("a whole-second value reads with a fraction: %s", *s)
			}
		}
		// ...and a customer answers it by echoing what it reads.
		if _, err := f.approveAsFor(id, crScopeUserA1, cr.PlannedStartOn, cr.PlannedEndOn); err != nil {
			t.Fatalf("a customer's approval echoing a migrated window: %v", err)
		}
	})
}

func TestChangeRequestTimestampEchoIntegration_EveryAnswerAcceptsWhatTheReadPrinted(t *testing.T) {
	t.Run("a customer's approval", func(t *testing.T) {
		f, id := fractionalInCustomerApproval(t)
		cr, err := f.getAsContact(id, crScopeUserA1)
		if err != nil || cr.PlannedStartOn == nil || cr.PlannedEndOn == nil {
			t.Fatalf("a customer's read: %+v (%v)", cr.PlannedStartOn, err)
		}
		// The window rounded to whole seconds is not the stored one: exact, so the precision matters.
		_, err = f.approveAsFor(id, crScopeUserA1, sp(rsStart1), sp(rsEnd1))
		wantRefusalCode(t, "an echo rounded to whole seconds", err, 409, apierror.CodeChangeRequestScheduleChanged)
		// What was printed is accepted.
		if _, err := f.approveAsFor(id, crScopeUserA1, cr.PlannedStartOn, cr.PlannedEndOn); err != nil {
			t.Fatalf("a customer's approval echoing %s .. %s: %v", *cr.PlannedStartOn, *cr.PlannedEndOn, err)
		}
		f.expect(id, "after the approval", "SCHEDULED", "implement", "canceled")
	})

	t.Run("a customer's rejection", func(t *testing.T) {
		f, id := fractionalInCustomerApproval(t)
		cr, _ := f.getAsContact(id, crScopeUserA2)
		if _, err := f.patchAsContact(id, crScopeUserA2, domain.PatchChangeRequestRequest{
			IsCustomerApproved: boolp(false), ExpectedPlannedStartOn: cr.PlannedStartOn, ExpectedPlannedEndOn: cr.PlannedEndOn}); err != nil {
			t.Fatalf("a customer's rejection echoing the window: %v", err)
		}
	})

	t.Run("Accept proposed time, with the proposal's own half second", func(t *testing.T) {
		f, id := fractionalInCustomerApproval(t)
		cr := f.mustPropose(id, crScopeUserA1, fracProposal)
		if cr.CustomerProposal == nil || cr.CustomerProposal.StartOn != fracProposal {
			t.Fatalf("customerProposal.startOn = %+v, want %s", cr.CustomerProposal, fracProposal)
		}
		// The two spellings of the proposed start agree.
		if cr.CustomerUpdatedOn == nil || *cr.CustomerUpdatedOn != fracProposal {
			t.Fatalf("customerUpdatedOn = %v, want %s: the proposal reads one way in both fields", cr.CustomerUpdatedOn, fracProposal)
		}
		// A page that rounded the planned window to whole seconds is stale, whichever it was told.
		req := f.acceptReq(id)
		req.ExpectedPlannedStartOn, req.ExpectedPlannedEndOn = sp(rsStart1), sp(rsEnd1)
		_, err := f.patch(id, req)
		wantRefusalCode(t, "an Accept for a window rounded to whole seconds", err, 409, apierror.CodeChangeRequestScheduleChanged)
		// The values the page read go back as they are.
		accepted := f.mustAccept(id)
		length := mustTime(t, fracEnd).Sub(mustTime(t, fracStart))
		wantEnd := mustTime(t, fracProposal).Add(length).UTC().Format(time.RFC3339Nano)
		if accepted.PlannedStartOn == nil || *accepted.PlannedStartOn != fracProposal || accepted.PlannedEndOn == nil || *accepted.PlannedEndOn != wantEnd {
			t.Fatalf("the window after Accept = %v .. %v, want %s .. %s", accepted.PlannedStartOn, accepted.PlannedEndOn, fracProposal, wantEnd)
		}
	})

	t.Run("Propose a different time, with fractions in its own window", func(t *testing.T) {
		f, id := fractionalInCustomerApproval(t)
		f.mustPropose(id, crScopeUserA2, fracProposal)
		if err := f.counter(id, sp("2030-03-15T09:00:00.25Z"), sp("2030-03-15T11:00:00.75Z")); err != nil {
			t.Fatalf("Propose a different time naming the printed window: %v", err)
		}
		f.wantPlanned(id, "after the different time", "2030-03-15T09:00:00.25Z", "2030-03-15T11:00:00.75Z")
		f.wantConversation(id, "after the different time", "2030-03-08T09:00:00Z", "DISAGREE")
	})

	t.Run("a decline", func(t *testing.T) {
		f, id := fractionalInCustomerApproval(t)
		f.mustPropose(id, crScopeUserA2, fracProposal)
		if err := f.counter(id, nil, nil); err != nil {
			t.Fatalf("a decline naming the printed window: %v", err)
		}
		f.wantPlanned(id, "after the decline", fracStart, fracEnd)
		f.wantAnswer(id, "after the decline", "disagreed")
		// The planned window restated, as printed, is the same decline (nothing moved).
		g, gid := fractionalInCustomerApproval(t)
		g.mustPropose(gid, crScopeUserA2, fracProposal)
		if err := g.counter(gid, sp(fracStart), sp(fracEnd)); err != nil {
			t.Fatalf("a decline restating the printed window: %v", err)
		}
		g.wantPlanned(gid, "after restating the window", fracStart, fracEnd)
		g.wantAnswer(gid, "after restating the window", "disagreed")
	})

	t.Run("a plain Re-schedule", func(t *testing.T) {
		f, id := fractionalInCustomerApproval(t)
		if err := f.counter(id, sp(rsStart3), sp(rsEnd3)); err != nil {
			t.Fatalf("a Re-schedule naming the printed window: %v", err)
		}
		f.wantPlanned(id, "after the Re-schedule", rsStart3, rsEnd3)
		// A rounded one is refused with the schedule code: nothing was written.
		g, gid := fractionalInCustomerApproval(t)
		req := g.counterReq(gid, sp(rsStart3), sp(rsEnd3))
		req.ExpectedPlannedStartOn, req.ExpectedPlannedEndOn = sp(rsStart1), sp(rsEnd1)
		_, err := g.patch(gid, req)
		wantRefusalCode(t, "a Re-schedule for a window rounded to whole seconds", err, 409, apierror.CodeChangeRequestScheduleChanged)
	})
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}
