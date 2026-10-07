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
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

var allTransitionStates = []domain.ChangeRequestState{
	domain.ChangeRequestStateNew, domain.ChangeRequestStateAssess, domain.ChangeRequestStateAuthorize,
	domain.ChangeRequestStateCustomerApproval, domain.ChangeRequestStateScheduled, domain.ChangeRequestStateImplement,
	domain.ChangeRequestStateReview, domain.ChangeRequestStateCustomerReview, domain.ChangeRequestStateRollback,
	domain.ChangeRequestStateClosed, domain.ChangeRequestStateCanceled,
}

func TestNormalizeRequestedChangeRequestState(t *testing.T) {
	for in, want := range map[string]domain.ChangeRequestState{
		"implement":         "implement",
		" IMPLEMENT ":       "implement",
		"Customer_Approval": "customer_approval",
		"\tCANCELED\n":      "canceled",
		"ROLLBACK":          "rollback",
	} {
		got, err := normalizeRequestedChangeRequestState(domain.ChangeRequestState(in))
		if err != nil || got != want {
			t.Errorf("normalize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "  ", "frobnicate", "in review", "assess;", "customer-review", "new\x00"} {
		_, err := normalizeRequestedChangeRequestState(domain.ChangeRequestState(bad))
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("normalize(%q): err = %v, want a *apierror.ValidationError", bad, err)
			continue
		}
		if want := fmt.Sprintf("state %q is not a change request state", bad); ve.Msg != want {
			t.Errorf("normalize(%q): message %q, want %q", bad, ve.Msg, want)
		}
	}
}

// What checkStaffStateRequest alone refuses, written out per state: the requests
// that are not an edge of the table and have no refusal of their own elsewhere.
// ("refused" = a 400; the rest is accepted here, and either an edge of the table or
// left to patchChangeRequestTx's case for that target.)
func TestCheckStaffStateRequest_Table(t *testing.T) {
	refused := map[string][]string{
		// The approval waits and New: only Cancel (and, from New, Request Approval).
		"NEW":       {"implement", "review", "customer_review", "closed"},
		"ASSESS":    {"implement", "review", "customer_review", "closed"},
		"AUTHORIZE": {"implement", "review", "customer_review", "closed"},
		"SCHEDULED": {"review", "customer_review", "closed"},
		"IMPLEMENT": {"customer_review", "closed"},
		"REVIEW":    {"implement"},
		// The customer's states are the customer's: their own refusals decide.
		"CUSTOMER_APPROVAL": nil,
		"CUSTOMER_REVIEW":   nil,
		// Final: everything but naming the state it is in.
		"ROLLBACK": {"new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "canceled"},
		"CLOSED":   {"new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review", "rollback", "canceled"},
		"CANCELED": {"new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review", "rollback", "closed"},
	}
	for _, from := range allTransitionStates {
		state := strings.ToUpper(string(from))
		want := map[string]bool{}
		for _, r := range refused[state] {
			want[r] = true
		}
		for _, review := range []bool{false, true} {
			for _, to := range allTransitionStates {
				err := checkStaffStateRequest(state, to, review)
				if got := err != nil; got != want[string(to)] {
					t.Errorf("%s -> %s (review=%v): refused = %v (%v), want %v", state, to, review, got, err, want[string(to)])
				}
				if err != nil {
					var ve *apierror.ValidationError
					if !errors.As(err, &ve) {
						t.Errorf("%s -> %s: %T, want a *apierror.ValidationError (a readable 400)", state, to, err)
					}
				}
			}
		}
	}
	// A NULL state is New.
	if err := checkStaffStateRequest("", domain.ChangeRequestStateImplement, false); err == nil {
		t.Error("NULL -> implement was accepted")
	}
	if err := checkStaffStateRequest("", domain.ChangeRequestStateNew, false); err != nil {
		t.Errorf("NULL -> new (a resend) was refused: %v", err)
	}
}

// The graph IS legalNextStates: every state the change is offered is one the
// PATCH takes (an edge of the table, or one of the targets whose refusal is
// patchChangeRequestTx's own), and a request the table does not offer is refused
// here unless it is such a target or the state the change is in.
func TestCheckStaffStateRequest_AgreesWithLegalNextStates(t *testing.T) {
	ownRefusal := map[domain.ChangeRequestState]bool{
		domain.ChangeRequestStateNew: true, domain.ChangeRequestStateAssess: true, domain.ChangeRequestStateAuthorize: true,
		domain.ChangeRequestStateCustomerApproval: true, domain.ChangeRequestStateScheduled: true, domain.ChangeRequestStateRollback: true,
	}
	for _, from := range allTransitionStates {
		s := string(from)
		for _, review := range []bool{false, true} {
			legal := map[string]bool{}
			for _, next := range legalChangeRequestNextStates(&s, review) {
				legal[next] = true
				if err := checkStaffStateRequest(strings.ToUpper(s), domain.ChangeRequestState(next), review); err != nil {
					t.Errorf("legalNextStates(%s, %v) offers %s, which the PATCH refuses: %v", s, review, next, err)
				}
			}
			if terminalChangeRequestState(strings.ToUpper(s)) && len(legal) != 0 {
				t.Errorf("%s is final but offers %v", s, legal)
			}
			for _, to := range allTransitionStates {
				if legal[string(to)] || to == from || ownRefusal[to] || customerStageSpecForState(strings.ToUpper(s)) != nil {
					continue
				}
				if to == domain.ChangeRequestStateCustomerReview && from == domain.ChangeRequestStateReview {
					continue // the box picks it or Closed: patchChangeRequestTx's cases
				}
				if to == domain.ChangeRequestStateClosed && from == domain.ChangeRequestStateReview {
					continue
				}
				if err := checkStaffStateRequest(strings.ToUpper(s), to, review); err == nil {
					t.Errorf("%s -> %s (review=%v) is not offered by legalNextStates but the PATCH accepts it", s, to, review)
				}
			}
		}
	}
}

// The targets of every state, with the review branch both ways, as a sorted list: the
// graph written out, so a change to the table shows in this diff.
func TestChangeRequestStaffTargets(t *testing.T) {
	want := map[domain.ChangeRequestState][]string{
		"new":               {"assess", "canceled"},
		"assess":            {"canceled"},
		"authorize":         {"canceled"},
		"customer_approval": {"authorize", "canceled"},
		"scheduled":         {"canceled", "implement"},
		"implement":         {"canceled", "review"},
		"review":            {"canceled", "closed", "customer_review", "rollback"},
		"customer_review":   {"canceled", "rollback"},
		"rollback":          nil,
		"closed":            nil,
		"canceled":          nil,
	}
	for st, w := range want {
		var got []string
		for _, next := range changeRequestStaffTargets(st) {
			got = append(got, string(next))
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(w, ",") {
			t.Errorf("changeRequestStaffTargets(%s) = %v, want %v", st, got, w)
		}
	}
	if got := changeRequestStaffTargets("garbage"); got != nil {
		t.Errorf("changeRequestStaffTargets(garbage) = %v, want nil", got)
	}
}

func TestTransitionRefusalMessages(t *testing.T) {
	msg := func(err error) string {
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("%v (%T) is not a *apierror.ValidationError", err, err)
		}
		return ve.Msg
	}
	for _, tc := range []struct {
		from      string
		requested domain.ChangeRequestState
		review    bool
		want      string
	}{
		{"CANCELED", "implement", false, `state "implement" cannot be set manually from canceled: a change request that is canceled cannot be moved`},
		{"CLOSED", "customer_review", true, `state "customer_review" cannot be set manually from closed: a change request that is closed cannot be moved`},
		{"ROLLBACK", "new", false, `state "new" cannot be set manually from rollback: a change request that is rolled back cannot be moved`},
		{"NEW", "implement", false, `state "implement" cannot be set manually from new: approval has not been requested yet (Request Approval is state "assess"); the moves open to staff from new are: assess, canceled`},
		{"ASSESS", "closed", true, `state "closed" cannot be set manually from assess: it is waiting for its peer approval, which moves it on by itself; the moves open to staff from assess are: only canceled`},
		{"AUTHORIZE", "review", false, `state "review" cannot be set manually from authorize: it is waiting for its CAB approval, which moves it on by itself (to Customer Approval first when the customer's approval is required); the moves open to staff from authorize are: only canceled`},
		{"SCHEDULED", "closed", false, `state "closed" cannot be set manually from scheduled: a change request goes through implement and review in order, one step at a time; the moves open to staff from scheduled are: implement, canceled`},
		{"IMPLEMENT", "closed", false, `state "closed" cannot be set manually from implement: a change request goes through implement and review in order, one step at a time; the moves open to staff from implement are: review, canceled`},
		{"REVIEW", "implement", true, `state "implement" cannot be set manually from review: a change request cannot go back to an earlier step; the moves open to staff from review are: customer_review, rollback, canceled`},
		{"REVIEW", "implement", false, `state "implement" cannot be set manually from review: a change request cannot go back to an earlier step; the moves open to staff from review are: closed, rollback, canceled`},
	} {
		if got := msg(checkStaffStateRequest(tc.from, tc.requested, tc.review)); got != tc.want {
			t.Errorf("%s -> %s: message\n  %q\nwant\n  %q", tc.from, tc.requested, got, tc.want)
		}
	}
}
