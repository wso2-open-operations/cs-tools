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
	"strings"
	"testing"
	"time"
)

// The customer-visibility predicate of change requests, as a unit: what it is
// made of and what it binds, with no database. The rule it implements is
// exercised against Postgres by TestChangeRequestVisibilityIntegration_*.

func restrictedCtx(email string) context.Context {
	return WithCallerIdentity(context.Background(), SearchScope{ViewerEmail: email})
}

func TestCRVisibilityClause_AnUnrestrictedCallerIsNotNarrowed(t *testing.T) {
	for name, scope := range map[string]SearchScope{
		"internal":                  {Unrestricted: true},
		"internal with an email":    {Unrestricted: true, ViewerEmail: "staff@wso2.com"},
		"the CSM BFF with an email": {Unrestricted: true, ViewerEmail: "staff@wso2.com", HasInternalAccess: true},
	} {
		sql, args := CRVisibility{}.clause(WithCallerIdentity(context.Background(), scope), "wi", "cr", 1)
		if sql != "" || args != nil {
			t.Errorf("%s: clause = %q %v, want none", name, sql, args)
		}
		if and, got := (CRVisibility{}).andClause(WithCallerIdentity(context.Background(), scope), "wi", "cr", []any{"x"}); and != "" || len(got) != 1 {
			t.Errorf("%s: andClause = %q %v, want the arguments untouched", name, and, got)
		}
	}
}

// A restricted caller with no email, or no identity at all, matches nothing.
func TestCRVisibilityClause_FailsClosed(t *testing.T) {
	for name, ctx := range map[string]context.Context{
		"no email":          restrictedCtx(""),
		"a blank email":     restrictedCtx("   "),
		"no identity":       context.Background(),
		"staff with a user": WithCallerIdentity(context.Background(), SearchScope{HasInternalAccess: true}),
	} {
		sql, args := CRVisibility{}.clause(ctx, "wi", "cr", 1)
		if sql != "FALSE" || args != nil {
			t.Errorf("%s: clause = %q %v, want FALSE and no arguments", name, sql, args)
		}
	}
}

// The email is lower-cased and trimmed once, bound (never inlined), and the
// placeholders follow the caller's argument numbering.
func TestCRVisibilityClause_BindsTheViewerAndTheCutover(t *testing.T) {
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	sql, args := CRVisibility{StrictFrom: &from}.clause(restrictedCtx("  Alice@Example.COM "), "w", "c", 4)
	if len(args) != 2 || args[0] != "alice@example.com" {
		t.Fatalf("args = %v, want [alice@example.com, <cutover>]", args)
	}
	if got, ok := args[1].(*time.Time); !ok || got == nil || !got.Equal(from) {
		t.Fatalf("second argument = %#v, want the cutover instant", args[1])
	}
	for _, want := range []string{"$4::text", "$5::timestamptz", "w.project_id", "w.id", "c.state::text", "w.created_on"} {
		if !strings.Contains(sql, want) {
			t.Errorf("the clause should mention %q:\n%s", want, sql)
		}
	}
	for _, bad := range []string{"$1", "$2", "$3", "$6", "alice", "Alice"} {
		if strings.Contains(sql, bad) {
			t.Errorf("the clause must not contain %q (placeholders start at 4; the email is bound, not inlined):\n%s", bad, sql)
		}
	}
	// No cutover: a typed NULL.
	_, args = CRVisibility{}.clause(restrictedCtx("a@b.c"), "wi", "cr", 1)
	if got, ok := args[1].(*time.Time); !ok || got != nil {
		t.Fatalf("with no cutover the second argument = %#v, want a nil *time.Time", args[1])
	}
}

// Membership is part of the clause itself, not left to row-level security: the
// same SQL must hold for a connection that skips policies.
func TestCRVisibilityClause_CarriesItsOwnMembershipCheck(t *testing.T) {
	sql, _ := CRVisibility{}.clause(restrictedCtx("a@b.c"), "wi", "cr", 1)
	for _, want := range []string{
		"FROM project_contact pc",
		"pc.state = 'REGISTERED'",
		"approval_stage_approver asa",
		"JOIN approval_stage ast ON ast.id = asa.stage_id",
		`FROM "user" u WHERE LOWER(u.email)`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("the clause should contain %q:\n%s", want, sql)
		}
	}
	if strings.Contains(sql, "current_setting") || strings.Contains(sql, "app.viewer") {
		t.Errorf("the clause must not read the session settings (six code paths blank them mid-transaction):\n%s", sql)
	}
}

// Designation reads exactly the two stage labels provisionCustomerStage writes.
func TestCRVisibility_TheCustomerStageLabelsArePinned(t *testing.T) {
	if got := strings.Join(crVisibilityCustomerStageLabels, "|"); got != "Customer Approval|Customer Review" {
		t.Fatalf("designation labels = %q", got)
	}
	if customerApprovalStageSpec.label != crVisibilityCustomerStageLabels[0] || customerReviewStageSpec.label != crVisibilityCustomerStageLabels[1] {
		t.Fatalf("the labels provisionCustomerStage writes (%q, %q) drifted from the ones visibility reads (%v)",
			customerApprovalStageSpec.label, customerReviewStageSpec.label, crVisibilityCustomerStageLabels)
	}
}

// Only "this person was asked" designates; the sync's not-asked states do not.
func TestCRVisibility_OnlyAskedApproverStatesDesignate(t *testing.T) {
	got := strings.Join(crDesignationApproverStates, ",")
	if got != "REQUESTED,APPROVED,REJECTED,CANCELLED" {
		t.Fatalf("designating states = %s", got)
	}
	for _, notAsked := range []string{"NOT_REQUESTED", "NOT_REQUIRED", "NOT_ENTITLED"} {
		for _, s := range crDesignationApproverStates {
			if s == notAsked {
				t.Errorf("%s must not designate", notAsked)
			}
		}
	}
}

// A legacy change request is visible in exactly the states customers saw before
// the strict rule: every state past Authorize, never New / Assess / Authorize.
func TestCRVisibility_TheLegacyStates(t *testing.T) {
	in := map[string]bool{}
	for _, s := range crLegacyVisibleStates {
		in[s] = true
	}
	for _, hidden := range []string{crStateNew, crStateAssess, crStateAuthorize} {
		if in[hidden] {
			t.Errorf("%s must not be visible to a legacy change request's contacts", hidden)
		}
	}
	for state := range knownChangeRequestStates {
		switch state {
		case crStateNew, crStateAssess, crStateAuthorize:
			continue
		}
		if !in[state] {
			t.Errorf("%s must be visible to a legacy change request's contacts", state)
		}
	}
	if len(crLegacyVisibleStates) != len(knownChangeRequestStates)-3 {
		t.Errorf("legacy states = %v, want every known state but New / Assess / Authorize", crLegacyVisibleStates)
	}
}

func TestCRVisibility_IsLegacyBoundary(t *testing.T) {
	cut := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		vis  CRVisibility
		at   time.Time
		want bool
	}{
		{"no cutover: everything is legacy", CRVisibility{}, cut.AddDate(50, 0, 0), true},
		{"before the cutover", CRVisibility{StrictFrom: &cut}, cut.Add(-time.Nanosecond), true},
		{"at the cutover", CRVisibility{StrictFrom: &cut}, cut, false},
		{"after the cutover", CRVisibility{StrictFrom: &cut}, cut.Add(time.Nanosecond), false},
	} {
		if got := tc.vis.isLegacy(tc.at); got != tc.want {
			t.Errorf("%s: isLegacy = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestFirstCRVisibilityAndSQLStringList(t *testing.T) {
	cut := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if got := firstCRVisibility(nil); got.StrictFrom != nil {
		t.Errorf("no argument = %+v, want the zero policy", got)
	}
	if got := firstCRVisibility([]CRVisibility{{StrictFrom: &cut}}); got.StrictFrom == nil || !got.StrictFrom.Equal(cut) {
		t.Errorf("one argument = %+v, want it back", got)
	}
	if got := sqlStringList([]string{"A", "it's"}); got != `'A', 'it''s'` {
		t.Errorf("sqlStringList = %s", got)
	}
}

// The policy travels in the context from the repository method that owns it to
// the helpers under it; a context that carries none reads back as the zero policy.
func TestCRVisibilityContext(t *testing.T) {
	cut := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if got := crVisibilityFromContext(context.Background()); got.StrictFrom != nil {
		t.Errorf("a bare context carries %+v, want the zero policy", got)
	}
	ctx := withCRVisibility(context.Background(), CRVisibility{StrictFrom: &cut})
	if got := crVisibilityFromContext(ctx); got.StrictFrom == nil || !got.StrictFrom.Equal(cut) {
		t.Errorf("a stamped context carries %+v, want the cutover", got)
	}
}
