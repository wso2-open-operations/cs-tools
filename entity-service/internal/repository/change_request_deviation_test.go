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
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Blank is the webapp's isBlankHtml rule (IsBlankRichText): no text once the
// markup is removed, so an emptied editor's markup is stored as NULL, not as a
// value the webapp would show as no deviation.
func TestNormalizeImplementationDeviation(t *testing.T) {
	s := func(v string) *string { return &v }
	for _, blank := range []*string{nil, s(""), s("   "), s("\n\t"), s("<p><br></p>"), s("<p>&nbsp;</p>"), s(" <p> &nbsp; </p>\n"),
		s("<p><img src=\"https://example.com/diagram.png\"></p>")} {
		if got := normalizeImplementationDeviation(blank); got != nil {
			t.Errorf("normalizeImplementationDeviation(%q) = %q, want nil (NULL)", derefOr(blank), *got)
		}
	}
	for _, text := range []string{" <p>kept as sent</p> ", "x", "<p><br></p><p>Used the standby.</p>"} {
		v := s(text)
		if got := normalizeImplementationDeviation(v); got != v {
			t.Errorf("the non-blank deviation %q is not stored as sent: %v", text, got)
		}
	}
}

func TestSameImplementationDeviation(t *testing.T) {
	s := func(v string) *string { return &v }
	for _, c := range []struct {
		a, b *string
		want bool
	}{
		{nil, nil, true},
		{s("<p>a</p>"), s("<p>a</p>"), true},
		{nil, s("<p>a</p>"), false},
		{s("<p>a</p>"), nil, false},
		{s("<p>a</p>"), s("<p>b</p>"), false},
	} {
		if got := sameImplementationDeviation(c.a, c.b); got != c.want {
			t.Errorf("sameImplementationDeviation(%s, %s) = %v, want %v", derefOr(c.a), derefOr(c.b), got, c.want)
		}
	}
}

// A customer's create may not carry a deviation (403, as a customer's PATCH
// carrying one is refused); staff, internal clients and a context with no
// identity are not judged, and a blank one is none.
func TestRefuseExternalCreateDeviation(t *testing.T) {
	s := func(v string) *string { return &v }
	customer := WithCallerIdentity(context.Background(), SearchScope{ViewerEmail: "customer@example.com"})
	staff := WithCallerIdentity(context.Background(), SearchScope{Unrestricted: true, ViewerEmail: "staff@wso2.com"})
	mixed := WithCallerIdentity(context.Background(), SearchScope{HasInternalAccess: true, ViewerEmail: "staff@wso2.com"})

	err := RefuseExternalCreateDeviation(customer, s("<p>Rolled out to one region first.</p>"))
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) || fe.Code != apierror.CodeChangeRequestForbidden || fe.Msg != externalCreateDeviationRefusal {
		t.Fatalf("a customer's create with a deviation: err = %v, want the 403 change_request_forbidden", err)
	}
	for _, blank := range []*string{nil, s(""), s("<p><br></p>")} {
		if err := RefuseExternalCreateDeviation(customer, blank); err != nil {
			t.Errorf("a customer's create with the blank deviation %q was refused: %v", derefOr(blank), err)
		}
	}
	for name, ctx := range map[string]context.Context{"staff": staff, "mixed": mixed, "no identity": context.Background()} {
		if err := RefuseExternalCreateDeviation(ctx, s("<p>x</p>")); err != nil {
			t.Errorf("%s: a create with a deviation was refused: %v", name, err)
		}
	}
	if externalCreateDeviationRefusal != "customers cannot set implementationDeviation: the deviation from the implementation plan is recorded by WSO2 staff" {
		t.Errorf("message changed: %q", externalCreateDeviationRefusal)
	}
}

// Both create paths refuse a customer's deviation before anything else: the
// repository below has no database, so reaching one would panic.
func TestCreateChangeRequest_RefusesACustomersDeviationBeforeAnyWrite(t *testing.T) {
	customer := WithCallerIdentity(context.Background(), SearchScope{ViewerEmail: "customer@example.com"})
	deviation := "<p>Restarted the nodes one by one.</p>"
	typ := domain.ChangeRequestTypeNormal
	req := domain.CreateChangeRequestRequest{Subject: "s", Type: &typ, ImplementationDeviation: &deviation}
	r := &changeRequestRepo{}
	var fe *apierror.ForbiddenError
	if _, err := r.CreateChangeRequest(customer, req, "customer@example.com"); !errors.As(err, &fe) || fe.Code != apierror.CodeChangeRequestForbidden {
		t.Errorf("CreateChangeRequest: err = %v, want the 403 change_request_forbidden", err)
	}
	if _, err := r.CreateChangeRequestFromServiceNow(customer, req, "3aaaaaaa-0000-0000-0000-0000000000d2", "CHG1", "customer@example.com"); !errors.As(err, &fe) || fe.Code != apierror.CodeChangeRequestForbidden {
		t.Errorf("CreateChangeRequestFromServiceNow: err = %v, want the 403 change_request_forbidden", err)
	}
}

func derefOr(v *string) string {
	if v == nil {
		return "<nil>"
	}
	return *v
}

func TestDeviationFinalMsg(t *testing.T) {
	for state, want := range map[string]string{
		crStateClosed:   "implementationDeviation can no longer be changed: the change request is closed, so its record is final",
		crStateCanceled: "implementationDeviation can no longer be changed: the change request is canceled, so its record is final",
		crStateRollback: "implementationDeviation can no longer be changed: the change request is rolled back, so its record is final",
	} {
		if got := deviationFinalMsg(state); got != want {
			t.Errorf("deviationFinalMsg(%s) = %q, want %q", state, got, want)
		}
	}
}

func TestDeviationNotYetMsg(t *testing.T) {
	for state, want := range map[string]string{
		"":                      "implementationDeviation cannot be changed yet: the deviation from the implementation plan can be recorded once implementation has started (current state: new)",
		crStateAssess:           "implementationDeviation cannot be changed yet: the deviation from the implementation plan can be recorded once implementation has started (current state: assess)",
		crStateCustomerApproval: "implementationDeviation cannot be changed yet: the deviation from the implementation plan can be recorded once implementation has started (current state: customer_approval)",
		crStateScheduled:        "implementationDeviation cannot be changed yet: the deviation from the implementation plan can be recorded once implementation has started (current state: scheduled)",
	} {
		if got := deviationNotYetMsg(state); got != want {
			t.Errorf("deviationNotYetMsg(%q) = %q, want %q", state, got, want)
		}
	}
}

// The rule by stored state (the state before the PATCH is applied), for a
// request that CHANGES the deviation: one row per state and situation. Row ids
// are <STATE>/<situation>; outcomes are ok / not-yet / carried-over / final. A
// resend is not a change, so it never reaches the rule (the integration test
// pins it per state).
func TestDeviationEditRefusal_PerState(t *testing.T) {
	type situation struct {
		name             string
		stored, clearing bool
	}
	situations := []situation{
		{"set (none stored)", false, false},
		{"change (one stored)", true, false},
		{"clear (one stored)", true, true},
	}
	want := map[string][3]string{
		"NULL":                  {"not-yet", "carried-over", "ok"},
		crStateNew:              {"not-yet", "carried-over", "ok"},
		crStateAssess:           {"not-yet", "not-yet", "not-yet"},
		crStateAuthorize:        {"not-yet", "not-yet", "not-yet"},
		crStateCustomerApproval: {"not-yet", "not-yet", "not-yet"},
		crStateScheduled:        {"not-yet", "not-yet", "not-yet"},
		crStateImplement:        {"ok", "ok", "ok"},
		crStateReview:           {"ok", "ok", "ok"},
		crStateCustomerReview:   {"ok", "ok", "ok"},
		crStateClosed:           {"final", "final", "final"},
		crStateCanceled:         {"final", "final", "final"},
		crStateRollback:         {"final", "final", "final"},
	}
	outcome := func(err error) string {
		if err == nil {
			return "ok"
		}
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			return fmt.Sprintf("unexpected %T", err)
		}
		switch {
		case ve.Code == apierror.CodeChangeRequestDeviationFinal:
			return "final"
		case ve.Code == apierror.CodeChangeRequestDeviationNotYet && ve.Msg == deviationCarriedOverMsg:
			return "carried-over"
		case ve.Code == apierror.CodeChangeRequestDeviationNotYet:
			return "not-yet"
		}
		return "unexpected code " + ve.Code
	}
	for row, outcomes := range want {
		state := row
		if row == "NULL" {
			state = ""
		}
		for i, sit := range situations {
			if got := outcome(deviationEditRefusal(state, sit.stored, sit.clearing)); got != outcomes[i] {
				t.Errorf("%s/%s = %s, want %s", row, sit.name, got, outcomes[i])
			}
		}
	}
}

// A deviation-only PATCH takes the gate (the stored state is needed), but in a
// state in which it can be recorded it is never judged: no query runs.
func TestCheckDeviationEditable_RecordableStatesAreNotJudged(t *testing.T) {
	s := func(v string) *string { return &v }
	if !changeRequestPatchNeedsGate(domain.PatchChangeRequestRequest{ImplementationDeviation: s("x")}) {
		t.Fatal("a deviation-only PATCH does not take the gate, so its state is never read")
	}
	for _, state := range []string{crStateImplement, crStateReview, crStateCustomerReview} {
		// A nil querier would panic if it were used.
		if err := checkDeviationEditable(context.Background(), nil, "id", changeRequestGateSnapshot{state: state}, s("x")); err != nil {
			t.Errorf("state %q: %v, want the deviation accepted", state, err)
		}
	}
	for _, state := range []string{"", crStateNew, crStateAssess, crStateScheduled, crStateClosed} {
		if err := checkDeviationEditable(context.Background(), nil, "id", changeRequestGateSnapshot{state: state}, nil); err != nil {
			t.Errorf("state %q: a PATCH without the deviation was judged: %v", state, err)
		}
	}
}

// Request Approval: only judged in the creation phase; the request's own value,
// when it carries the field, decides without a query.
func TestCheckRequestApprovalDeviationCleared_RequestValue(t *testing.T) {
	s := func(v string) *string { return &v }
	// A nil querier would panic if it were used.
	for _, state := range []string{crStateAssess, crStateAuthorize, crStateScheduled, crStateCustomerApproval} {
		if err := checkRequestApprovalDeviationCleared(context.Background(), nil, "id", state, s("<p>still here</p>")); err != nil {
			t.Errorf("a resend of Request Approval in %s was judged: %v", state, err)
		}
	}
	for _, state := range []string{"", crStateNew} {
		for _, blank := range []string{"", "   ", "<p><br></p>", "<p>&nbsp;</p>"} {
			if err := checkRequestApprovalDeviationCleared(context.Background(), nil, "id", state, s(blank)); err != nil {
				t.Errorf("%q: clearing it in the same PATCH: %v, want accepted", state, err)
			}
		}
		err := checkRequestApprovalDeviationCleared(context.Background(), nil, "id", state, s("<p>still here</p>"))
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Code != apierror.CodeChangeRequestDeviationNotIncorporated || ve.Msg != deviationNotIncorporatedMsg {
			t.Errorf("%q: a non-blank deviation in effect: err = %v, want the not-incorporated refusal", state, err)
		}
	}
	if deviationNotIncorporatedMsg != "approval cannot be requested until the deviation from the implementation plan carried over from the cloned change request has been folded into the implementation plan and cleared" {
		t.Errorf("message changed: %q", deviationNotIncorporatedMsg)
	}
}
