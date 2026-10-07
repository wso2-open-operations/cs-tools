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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestIsExternalCaller(t *testing.T) {
	ctxWith := func(scope SearchScope) context.Context { return WithCallerIdentity(context.Background(), scope) }
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{"a customer: resolved, restricted", ctxWith(SearchScope{ViewerEmail: "dave@example.com"}), true},
		{"a customer with project ids", ctxWith(SearchScope{ProjectIDs: []string{"p1"}, ViewerEmail: "dave@example.com"}), true},
		{"internal staff: unrestricted", ctxWith(SearchScope{Unrestricted: true, ViewerEmail: "alice@example.com"}), false},
		{"the system identity", WithSystemIdentity(context.Background()), false},
		{"staff who also hold an external record", ctxWith(SearchScope{ViewerEmail: "alice@example.com", HasInternalAccess: true}), false},
		{"no identity at all", context.Background(), false},
	} {
		if got := isExternalCaller(tc.ctx); got != tc.want {
			t.Errorf("%s: isExternalCaller = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestClassifyExternalPatch(t *testing.T) {
	yes, no := true, false
	start, end := "2030-03-01T09:00:00Z", "2030-03-01T11:00:00Z"
	state := domain.ChangeRequestStateScheduled

	t.Run("the customer's answers", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			req      domain.PatchChangeRequestRequest
			spec     *customerStageSpec
			approved bool
		}{
			{"approve", domain.PatchChangeRequestRequest{IsCustomerApproved: &yes}, &customerApprovalStageSpec, true},
			{"reject", domain.PatchChangeRequestRequest{IsCustomerApproved: &no}, &customerApprovalStageSpec, false},
			{"confirm the review", domain.PatchChangeRequestRequest{IsCustomerReviewed: &yes}, &customerReviewStageSpec, true},
			{"fail the review", domain.PatchChangeRequestRequest{IsCustomerReviewed: &no}, &customerReviewStageSpec, false},
		} {
			got, err := classifyExternalPatch(tc.req)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if got.kind != customerPatchAnswer || got.spec != tc.spec || got.approved != tc.approved {
				t.Errorf("%s: classified as %+v, want an answer for %s approved=%v", tc.name, got, tc.spec.label, tc.approved)
			}
		}
	})

	t.Run("an answer for the window the customer was shown", func(t *testing.T) {
		for _, expected := range [][2]*string{{&start, nil}, {nil, &end}, {&start, &end}} {
			got, err := classifyExternalPatch(domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, ExpectedPlannedStartOn: expected[0], ExpectedPlannedEndOn: expected[1]})
			if err != nil || got.kind != customerPatchAnswer || got.spec != &customerApprovalStageSpec {
				t.Fatalf("classified as %+v (%v), want the approval", got, err)
			}
			if (expected[0] == nil) != (got.expectedStart == nil) || (expected[1] == nil) != (got.expectedEnd == nil) {
				t.Errorf("expected window %v..%v classified as %v..%v", expected[0], expected[1], got.expectedStart, got.expectedEnd)
			}
			if expected[0] != nil && got.expectedStart.Format(time.RFC3339) != start {
				t.Errorf("expected start read as %v, want %s", got.expectedStart, start)
			}
		}
		got, err := classifyExternalPatch(domain.PatchChangeRequestRequest{IsCustomerReviewed: &no, ExpectedPlannedEndOn: &end})
		if err != nil || got.kind != customerPatchAnswer || got.spec != &customerReviewStageSpec || got.expectedEnd == nil {
			t.Fatalf("review classified as %+v (%v), want the review with its expected end", got, err)
		}
	})

	t.Run("a proposed window", func(t *testing.T) {
		for name, req := range map[string]domain.PatchChangeRequestRequest{
			"a start":        {PlannedStartOn: &start},
			"an end":         {PlannedEndOn: &end},
			"a whole window": {PlannedStartOn: &start, PlannedEndOn: &end},
		} {
			got, err := classifyExternalPatch(req)
			if err != nil || got.kind != customerPatchProposal {
				t.Errorf("%s: classified as %+v (%v), want a proposal", name, got, err)
			}
		}
	})

	t.Run("anything else is forbidden, alone or with an allowed field", func(t *testing.T) {
		impact := domain.ChangeRequestImpact("high")
		extra := map[string]func(*domain.PatchChangeRequestRequest){
			"title":                    func(r *domain.PatchChangeRequestRequest) { r.Title = &start },
			"description":              func(r *domain.PatchChangeRequestRequest) { r.Description = &start },
			"projectId":                func(r *domain.PatchChangeRequestRequest) { r.ProjectID = &start },
			"state":                    func(r *domain.PatchChangeRequestRequest) { r.State = &state },
			"impact":                   func(r *domain.PatchChangeRequestRequest) { r.Impact = &impact },
			"assignedTeamId":           func(r *domain.PatchChangeRequestRequest) { r.AssignedTeamID = &start },
			"requestApproval":          func(r *domain.PatchChangeRequestRequest) { r.RequestApproval = &yes },
			"onHold":                   func(r *domain.PatchChangeRequestRequest) { r.OnHold = &yes },
			"comment":                  func(r *domain.PatchChangeRequestRequest) { r.Comment = &start },
			"workNote":                 func(r *domain.PatchChangeRequestRequest) { r.WorkNote = &start },
			"customerApprovalRequired": func(r *domain.PatchChangeRequestRequest) { r.CustomerApprovalRequired = &no },
			"deploymentIds":            func(r *domain.PatchChangeRequestRequest) { r.DeploymentIDs = &[]string{} },
			"implementationPlan":       func(r *domain.PatchChangeRequestRequest) { r.ImplementationPlan = new(*string) },
		}
		for field, set := range extra {
			for name, base := range map[string]domain.PatchChangeRequestRequest{
				"alone":                {},
				"with the answer":      {IsCustomerApproved: &yes},
				"with the review":      {IsCustomerReviewed: &no},
				"with a proposed time": {PlannedStartOn: &start},
			} {
				req := base
				set(&req)
				_, err := classifyExternalPatch(req)
				var fe *apierror.ForbiddenError
				if !errors.As(err, &fe) {
					t.Errorf("%s %s: err = %v (%T), want *apierror.ForbiddenError", field, name, err, err)
					continue
				}
				if !strings.Contains(fe.Msg, "customers can only record") {
					t.Errorf("%s %s: message %q does not say what a customer can do", field, name, fe.Msg)
				}
			}
		}
		// Every field of the request is covered above or is one of the four: a
		// field added to the contract later must either be added to this list
		// (refused) or be a deliberate decision to let a customer set it.
		allowed := map[string]bool{"IsCustomerApproved": true, "IsCustomerReviewed": true, "PlannedStartOn": true, "PlannedEndOn": true,
			"ExpectedPlannedStartOn": true, "ExpectedPlannedEndOn": true}
		covered := map[string]bool{
			"Title": true, "Description": true, "ProjectID": true, "State": true, "Impact": true, "AssignedTeamID": true,
			"RequestApproval": true, "OnHold": true, "Comment": true, "WorkNote": true, "CustomerApprovalRequired": true,
			"DeploymentIDs": true, "ImplementationPlan": true,
		}
		typ := reflect.TypeOf(domain.PatchChangeRequestRequest{})
		for i := 0; i < typ.NumField(); i++ {
			name := typ.Field(i).Name
			if allowed[name] || covered[name] {
				continue
			}
			// Not enumerated above: it must still be refused. Set it through reflection.
			req := domain.PatchChangeRequestRequest{}
			f := reflect.ValueOf(&req).Elem().Field(i)
			f.Set(reflect.New(f.Type().Elem()))
			if _, err := classifyExternalPatch(req); err == nil {
				t.Errorf("field %s is accepted from an external caller", name)
			}
		}
	})

	t.Run("ambiguous combinations are a validation error", func(t *testing.T) {
		for name, tc := range map[string]struct {
			req  domain.PatchChangeRequestRequest
			want string
		}{
			"both outcomes":                       {domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, IsCustomerReviewed: &yes}, "not both"},
			"an answer and a window":              {domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, PlannedStartOn: &start}, "separate requests"},
			"a review and a window":               {domain.PatchChangeRequestRequest{IsCustomerReviewed: &no, PlannedEndOn: &end}, "separate requests"},
			"nothing at all":                      {domain.PatchChangeRequestRequest{}, "at least one field"},
			"all four at once":                    {domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, IsCustomerReviewed: &yes, PlannedStartOn: &start, PlannedEndOn: &end}, "separate requests"},
			"the expected window alone":           {domain.PatchChangeRequestRequest{ExpectedPlannedStartOn: &start}, "go with the customer's approval or review"},
			"the expected window with a proposal": {domain.PatchChangeRequestRequest{PlannedStartOn: &start, ExpectedPlannedEndOn: &end}, "go with the customer's approval or review"},
			"an expected window that is no date":  {domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, ExpectedPlannedStartOn: new(string)}, "expectedPlannedStartOn must be a date-time"},
		} {
			_, err := classifyExternalPatch(tc.req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || !strings.Contains(ve.Msg, tc.want) {
				t.Errorf("%s: err = %v (%T), want a ValidationError containing %q", name, err, err, tc.want)
			}
		}
	})
}

func TestStateForMessage(t *testing.T) {
	for in, want := range map[string]string{"": "NEW", "  ": "NEW", "SCHEDULED": "SCHEDULED", "CUSTOMER_REVIEW": "CUSTOMER_REVIEW"} {
		if got := stateForMessage(in); got != want {
			t.Errorf("stateForMessage(%q) = %q, want %q", in, got, want)
		}
	}
	// A change with no state reads as New in the stale-approval refusal, not as a
	// blank.
	var ce *apierror.ConflictError
	if err := staleApprovalRefusal(stageKindCustomerApproval, stateForMessage("")); !errors.As(err, &ce) || !strings.Contains(ce.Msg, "the change request is in New, but") {
		t.Errorf("refusal for a NULL state = %v, want it to say the change is in New", err)
	}
}

// neverQuerier is a crQuerier that fails the test if the database is touched:
// the answers customerCanAnswer gives without asking it.
type neverQuerier struct{ t *testing.T }

func (n neverQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	n.t.Helper()
	n.t.Fatal("customerCanAnswer queried the database for an answer that needs no query")
	return nil, nil
}

func (n neverQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	n.t.Helper()
	n.t.Fatal("customerCanAnswer queried the database for an answer that needs no query")
	return nil
}

// The answer is false, without a query, for every state that has no customer
// stage and for a viewer who cannot be identified.
func TestCustomerCanAnswer_NeedsNoQueryOutsideTheCustomerStates(t *testing.T) {
	for _, state := range []string{"", "new", "assess", "authorize", "scheduled", "implement", "review", "closed", "canceled", "rollback", "no-such-state"} {
		got, err := customerCanAnswer(context.Background(), neverQuerier{t}, "cr-1", nil, state, "dave@example.com")
		if err != nil || got {
			t.Errorf("state %q: customerCanAnswer = %v, %v; want false, nil", state, got, err)
		}
	}
	for _, email := range []string{"", "   "} {
		got, err := customerCanAnswer(context.Background(), neverQuerier{t}, "cr-1", nil, "customer_approval", email)
		if err != nil || got {
			t.Errorf("viewer %q: customerCanAnswer = %v, %v; want false, nil", email, got, err)
		}
	}
}

// markCustomerCanAnswer sets the field for a customer only, and a customer is
// always told true or false: here, false for a change request that is in no
// customer state (answered without a database). Staff and an unidentified
// caller are told nothing.
func TestMarkCustomerCanAnswer_WhoIsToldWhat(t *testing.T) {
	state := "scheduled"
	repo := &changeRequestRepo{} // a nil connection: none of these answers may need one
	customer := WithCallerIdentity(context.Background(), SearchScope{ViewerEmail: "dave@example.com"})

	cr := domain.ChangeRequest{}
	cr.ID, cr.State = "cr-1", &state
	repo.markCustomerCanAnswer(customer, &cr)
	if cr.CustomerCanAnswer == nil || *cr.CustomerCanAnswer {
		t.Fatalf("a customer reading a Scheduled change request is told %v, want false", cr.CustomerCanAnswer)
	}

	for name, ctx := range map[string]context.Context{
		"the system identity":           WithSystemIdentity(context.Background()),
		"an unrestricted caller":        WithCallerIdentity(context.Background(), SearchScope{Unrestricted: true, ViewerEmail: "alice@example.com"}),
		"staff holding an external one": WithCallerIdentity(context.Background(), SearchScope{ViewerEmail: "alice@example.com", HasInternalAccess: true}),
		"no identity":                   context.Background(),
	} {
		cr := domain.ChangeRequest{}
		cr.ID, cr.State = "cr-1", &state
		repo.markCustomerCanAnswer(ctx, &cr)
		if cr.CustomerCanAnswer != nil {
			t.Errorf("%s is told customerCanAnswer = %v, want nothing", name, *cr.CustomerCanAnswer)
		}
	}
}
