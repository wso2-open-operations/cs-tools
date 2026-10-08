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

package repository

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func tm(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func spp(s string) *string { return &s }

func TestFmtPlannedLength(t *testing.T) {
	for in, want := range map[time.Duration]string{
		2 * time.Hour:               "2 hours",
		time.Hour:                   "1 hour",
		90 * time.Minute:            "1 hour 30 minutes",
		26 * time.Hour:              "1 day 2 hours",
		48 * time.Hour:              "2 days",
		time.Second:                 "1 second",
		2*time.Hour + 3*time.Second: "2 hours 3 seconds",
		0:                           "0 seconds",
		500 * time.Millisecond:      "500ms",
	} {
		if got := fmtPlannedLength(in); got != want {
			t.Errorf("fmtPlannedLength(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestCustomerProposalFacts_LengthAndProposedEnd(t *testing.T) {
	f := customerProposalFacts{start: tm("2030-03-01T09:00:00Z"), end: tm("2030-03-01T11:30:00Z"), proposed: tm("2030-03-08T09:00:00Z")}
	if d, ok := f.plannedLength(); !ok || d != 150*time.Minute {
		t.Fatalf("plannedLength = %v, %v", d, ok)
	}
	if e := f.proposedEnd(); e == nil || !e.Equal(*tm("2030-03-08T11:30:00Z")) {
		t.Fatalf("proposedEnd = %v, want the proposed start plus the planned length", e)
	}
	for name, g := range map[string]customerProposalFacts{
		"no start":        {end: tm("2030-03-01T11:00:00Z"), proposed: tm("2030-03-08T09:00:00Z")},
		"no end":          {start: tm("2030-03-01T09:00:00Z"), proposed: tm("2030-03-08T09:00:00Z")},
		"an empty window": {start: tm("2030-03-01T09:00:00Z"), end: tm("2030-03-01T09:00:00Z"), proposed: tm("2030-03-08T09:00:00Z")},
		"an inverted one": {start: tm("2030-03-01T09:00:00Z"), end: tm("2030-03-01T08:00:00Z"), proposed: tm("2030-03-08T09:00:00Z")},
		"no proposal":     {start: tm("2030-03-01T09:00:00Z"), end: tm("2030-03-01T11:00:00Z")},
	} {
		if g.proposedEnd() != nil {
			t.Errorf("%s: proposedEnd = %v, want none", name, g.proposedEnd())
		}
	}
}

func TestCustomerProposalAnswer(t *testing.T) {
	p := tm("2030-03-08T09:00:00Z")
	for name, tc := range map[string]struct {
		f    customerProposalFacts
		want string
	}{
		"no proposed date":                 {customerProposalFacts{}, ""},
		"pending":                          {customerProposalFacts{proposed: p, pending: true}, "pending"},
		"pending wins over a stale answer": {customerProposalFacts{proposed: p, pending: true, confirmation: "AGREE"}, "pending"},
		"agreed":                           {customerProposalFacts{proposed: p, confirmation: "AGREE"}, "agreed"},
		"disagreed":                        {customerProposalFacts{proposed: p, confirmation: "DISAGREE"}, "disagreed"},
		"history without an answer":        {customerProposalFacts{proposed: p}, "unanswered"},
	} {
		if got := customerProposalAnswer(tc.f); got != tc.want {
			t.Errorf("%s: answer = %q, want %q", name, got, tc.want)
		}
	}
}

func TestAcceptBlock(t *testing.T) {
	now := *tm("2030-03-05T00:00:00Z")
	// A registered contact of the project is recorded as the proposer.
	recorded := proposer{known: true, email: "dave.mendis@example.com"}
	whole := customerProposalFacts{start: tm("2030-03-01T09:00:00Z"), end: tm("2030-03-01T11:00:00Z"), proposed: tm("2030-03-08T09:00:00Z"), pending: true}
	if got := acceptBlock(whole, recorded, now); got != "" {
		t.Fatalf("a waiting proposal over a whole window is blocked: %q", got)
	}
	held := whole
	held.onHold = true
	if got := acceptBlock(held, recorded, now); got != msgAcceptOnHold {
		t.Errorf("on hold: %q", got)
	}
	passed := whole
	passed.proposed = tm("2030-03-02T09:00:00Z")
	if got := acceptBlock(passed, recorded, now); !strings.Contains(got, "has already passed") || !strings.Contains(got, "2030-03-02T09:00:00Z") {
		t.Errorf("a passed proposal: %q", got)
	}
	empty := whole
	empty.end = nil
	if got := acceptBlock(empty, recorded, now); got != msgAcceptNoLength {
		t.Errorf("no length: %q", got)
	}
	// customer_updated_on is a column the previous system writes too: a window the proposal would push past the
	// range every planned window is held to is blocked, at the edge and beyond it, and not before.
	edge := whole
	edge.proposed, edge.start, edge.end = tm("2100-12-31T20:00:00Z"), tm("2030-03-01T09:00:00Z"), tm("2030-03-01T11:00:00Z")
	if got := acceptBlock(edge, recorded, now); got != "" {
		t.Errorf("a proposal whose window ends inside the last year of the range is blocked: %q", got)
	}
	over := edge
	over.proposed = tm("2100-12-31T23:00:00Z")
	if got := acceptBlock(over, recorded, now); got != msgAcceptTooFarAhead(*over.proposed, *tm("2101-01-01T01:00:00Z")) || !strings.Contains(got, "too far ahead") {
		t.Errorf("a window that ends after the range: %q", got)
	}
	far := edge
	far.proposed = tm("9999-12-31T23:30:00Z")
	if got := acceptBlock(far, recorded, now); !strings.Contains(got, "too far ahead") || !strings.Contains(got, "10000-01-01T01:30:00Z") {
		t.Errorf("a proposal in year 9999: %q", got)
	}
	// the hold is named before the passed time, as the refusals are ordered
	both := passed
	both.onHold = true
	if got := acceptBlock(both, recorded, now); got != msgAcceptOnHold {
		t.Errorf("on hold and passed: %q, want the hold first", got)
	}
	// A time nobody is recorded as having proposed is never accepted, and that is named before
	// everything else that could be wrong with it (nothing else is worth fixing about it).
	nobody := proposer{}
	if got := acceptBlock(whole, nobody, now); got != msgAcceptProposerNotRecorded {
		t.Errorf("no proposer recorded: %q, want %q", got, msgAcceptProposerNotRecorded)
	}
	for name, f := range map[string]customerProposalFacts{"on hold": held, "passed": passed, "no length": empty, "too far ahead": over, "everything": both} {
		if got := acceptBlock(f, nobody, now); got != msgAcceptProposerNotRecorded {
			t.Errorf("no proposer recorded and %s: %q, want the proposer first", name, got)
		}
	}
	// Words the pages and the contract show, so a reworded refusal is a deliberate act.
	for _, want := range []string{"nobody is recorded as having proposed this time", "written by someone at WSO2", "left over from an earlier cycle", `"Propose a different time"`} {
		if !strings.Contains(msgAcceptProposerNotRecorded, want) {
			t.Errorf("the refusal for an unrecorded proposer lacks %q: %s", want, msgAcceptProposerNotRecorded)
		}
	}
}

func TestValidateAcceptRequest(t *testing.T) {
	good := domain.PatchChangeRequestRequest{
		ConfirmCustomerUpdatedDate: spp("agree"), ExpectedCustomerUpdatedOn: spp("2030-03-08T09:00:00Z"),
		ExpectedPlannedStartOn: spp("2030-03-01T09:00:00Z"), ExpectedPlannedEndOn: spp("2030-03-01T11:00:00Z")}
	p, s, e, err := validateAcceptRequest(good)
	if err != nil || p == nil || s == nil || e == nil || !p.Equal(*tm("2030-03-08T09:00:00Z")) {
		t.Fatalf("a good Accept: %v %v %v %v", p, s, e, err)
	}
	for _, v := range []string{"agree", " AGREE ", "Agree"} {
		r := good
		r.ConfirmCustomerUpdatedDate = spp(v)
		if _, _, _, err := validateAcceptRequest(r); err != nil {
			t.Errorf("%q is refused: %v", v, err)
		}
	}
	title := "x"
	for name, tc := range map[string]struct {
		mod  func(*domain.PatchChangeRequestRequest)
		want string
	}{
		"disagree":           {func(r *domain.PatchChangeRequestRequest) { r.ConfirmCustomerUpdatedDate = spp("disagree") }, msgAcceptValueMustBeAgree},
		"empty":              {func(r *domain.PatchChangeRequestRequest) { r.ConfirmCustomerUpdatedDate = spp("") }, msgAcceptValueMustBeAgree},
		"nothing":            {func(r *domain.PatchChangeRequestRequest) { r.ConfirmCustomerUpdatedDate = nil }, msgAcceptValueMustBeAgree},
		"a title":            {func(r *domain.PatchChangeRequestRequest) { r.Title = &title }, msgAcceptAlone},
		"a window":           {func(r *domain.PatchChangeRequestRequest) { r.PlannedStartOn = spp("2030-03-08T09:00:00Z") }, msgAcceptAlone},
		"no proposal":        {func(r *domain.PatchChangeRequestRequest) { r.ExpectedCustomerUpdatedOn = nil }, msgAcceptNeedsExpectedProposal},
		"no start":           {func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedStartOn = nil }, msgAcceptNeedsExpectedWindow},
		"no end":             {func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedEndOn = nil }, msgAcceptNeedsExpectedWindow},
		"a proposal no date": {func(r *domain.PatchChangeRequestRequest) { r.ExpectedCustomerUpdatedOn = spp("infinity") }, "expectedCustomerUpdatedOn must be a date-time"},
		"a start no date":    {func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedStartOn = spp("now") }, "expectedPlannedStartOn must be a date-time"},
		"an end no date":     {func(r *domain.PatchChangeRequestRequest) { r.ExpectedPlannedEndOn = spp("2030-03-01") }, "expectedPlannedEndOn must be a date-time"},
	} {
		r := good
		tc.mod(&r)
		_, _, _, err := validateAcceptRequest(r)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || !strings.Contains(ve.Msg, tc.want) {
			t.Errorf("%s: err = %v, want a 400 saying %q", name, err, tc.want)
		}
	}
}

func TestJudgeStaffWindow(t *testing.T) {
	f := customerProposalFacts{start: tm("2030-03-01T09:00:00Z"), end: tm("2030-03-01T11:00:00Z"), proposed: tm("2030-03-08T09:00:00Z"), pending: true}
	for name, tc := range map[string]struct {
		start, end                               *string
		changed, inverted, empty, sameAsProposal bool
	}{
		"the plan restated":                 {spp("2030-03-01T09:00:00Z"), spp("2030-03-01T11:00:00Z"), false, false, false, false},
		"the plan in another zone":          {spp("2030-03-01T14:30:00+05:30"), spp("2030-03-01T16:30:00+05:30"), false, false, false, false},
		"nothing sent":                      {nil, nil, false, false, false, false},
		"a later window":                    {spp("2030-03-15T09:00:00Z"), spp("2030-03-15T11:00:00Z"), true, false, false, false},
		"the proposal as a window":          {spp("2030-03-08T09:00:00Z"), spp("2030-03-08T11:00:00Z"), true, false, false, true},
		"the proposal's start, another end": {spp("2030-03-08T09:00:00Z"), spp("2030-03-08T12:00:00Z"), true, false, false, false},
		"a start after the stored end":      {spp("2030-03-08T09:00:00Z"), nil, true, true, false, false},
		"an empty window":                   {spp("2030-03-15T09:00:00Z"), spp("2030-03-15T09:00:00Z"), true, false, true, false},
		"an end alone moved":                {nil, spp("2030-03-01T12:00:00Z"), true, false, false, false},
	} {
		w, err := judgeStaffWindow(f, tc.start, tc.end)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if w.changed != tc.changed || w.inverted != tc.inverted || w.empty != tc.empty || w.sameAsProposal != tc.sameAsProposal {
			t.Errorf("%s: %+v, want changed=%v inverted=%v empty=%v sameAsProposal=%v", name, w, tc.changed, tc.inverted, tc.empty, tc.sameAsProposal)
		}
	}
	// With no length to keep the proposal's start alone is "the same".
	noLength := customerProposalFacts{start: tm("2030-03-01T09:00:00Z"), proposed: tm("2030-03-08T09:00:00Z"), pending: true}
	if w, _ := judgeStaffWindow(noLength, spp("2030-03-08T09:00:00Z"), spp("2030-03-08T10:00:00Z")); !w.sameAsProposal {
		t.Errorf("a window starting at the proposal over a plan with no length: %+v", w)
	}
	if _, err := judgeStaffWindow(f, spp("infinity"), nil); err == nil {
		t.Error("an unparseable start was judged")
	}
}

func TestExpectedScheduleConflict(t *testing.T) {
	start, end := tm("2030-03-01T09:00:00Z"), tm("2030-03-01T11:00:00Z")
	if err := expectedScheduleConflict(start, end, nil, nil, readAgainSuffix); err != nil {
		t.Fatalf("no expectation: %v", err)
	}
	if err := expectedScheduleConflict(start, end, tm("2030-03-01T09:00:00Z"), tm("2030-03-01T11:00:00Z"), readAgainSuffix); err != nil {
		t.Fatalf("a matching window: %v", err)
	}
	err := expectedScheduleConflict(start, end, tm("2030-03-02T09:00:00Z"), nil, readAgainSuffix)
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) || !strings.HasSuffix(ce.Msg, "(it is now 2030-03-01T09:00:00Z to 2030-03-01T11:00:00Z); read it again before responding") {
		t.Fatalf("a stale start: %v", err)
	}
	err = expectedScheduleConflict(nil, nil, tm("2030-03-02T09:00:00Z"), nil, "read it again before giving your answer")
	if !errors.As(err, &ce) || !strings.Contains(ce.Msg, "(it is now no planned time is set); read it again before giving your answer") {
		t.Fatalf("a window that is gone: %v", err)
	}
}

func TestPendingProposalSQL_IsOneCoalescedAllowlist(t *testing.T) {
	// The predicate is ONE text used by every reader and every act, read as false (never NULL) on
	// unknown data, and its blocking half is the allowlist of customer stages.
	for _, want := range []string{
		"COALESCE(cr.state = 'CUSTOMER_APPROVAL'", "isfinite(cr.customer_updated_on)",
		"cr.customer_updated_date_confirmation IS NULL", "cr.customer_updated_on IS DISTINCT FROM cr.start_on",
		"NOT EXISTS (", ", false)",
	} {
		if !strings.Contains(pendingProposalSQL, want) {
			t.Errorf("the predicate lacks %q:\n%s", want, pendingProposalSQL)
		}
	}
	for _, want := range []string{"asa.state = 'REQUESTED'", "'Customer Approval'", "cr.customer_group_id"} {
		if !strings.Contains(otherApprovalAskedSQL, want) {
			t.Errorf("the blocking half lacks %q", want)
		}
	}
	if strings.Contains(pendingProposalSQL, "::change_request_confirmation_enum") || strings.Contains(otherApprovalAskedSQL, "::approval_stage") {
		t.Error("the predicate casts to an enum type: it must read the same on a database whose enum columns have the sync's shape")
	}
}
