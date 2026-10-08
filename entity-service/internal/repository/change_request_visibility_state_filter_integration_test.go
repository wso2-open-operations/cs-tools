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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The customer portal no longer drops New / Assess / Authorize from the states it
// asks for: which change requests a customer sees is decided HERE (the visibility
// fragment), never by the states the caller names. So naming a state can only
// narrow what the rule lets through, never widen it: a legacy change request in
// New, Assess or Authorize is not returned however the state filter is spelled
// (none, that state alone, every state, the three hidden ones), and one designated
// to the customer is returned in Authorize, also when Authorize is named.
//
// This is the Postgres data source. The ServiceNow data source, which has no
// designation, applies the legacy rule in the service (sn_change_request_customer_view.go).

// searchStatesAs lists the change requests of project that ctx may see when it names
// states (none named = no state filter), and whether id is among them.
func (f *crFlow) searchStatesAs(r visRepos, ctx context.Context, project, id string, states []domain.ChangeRequestState) (listed bool, total int) {
	f.t.Helper()
	views, n, err := r.cr.SearchChangeRequests(ctx, domain.SearchChangeRequestsRequest{
		Filters:    domain.SearchChangeRequestsFilters{ProjectIDs: []string{project}, States: states},
		Pagination: domain.Pagination{Limit: 50},
	}, nil, nil, nil, nil)
	if err != nil {
		f.t.Fatalf("SearchChangeRequests(states %v): %v", states, err)
	}
	for _, v := range views {
		if v.ID == id {
			listed = true
		}
	}
	if n != len(views) {
		f.t.Fatalf("SearchChangeRequests(states %v): total %d but the page holds %d rows", states, n, len(views))
	}
	return listed, n
}

func TestChangeRequestVisibilityIntegration_NamingAStateNeverWidensWhatACustomerSees_Legacy(t *testing.T) {
	type phase struct {
		stored string
		state  domain.ChangeRequestState
		legacy bool // visible to the project's registered contacts when legacy
	}
	phases := []phase{
		{"NEW", domain.ChangeRequestStateNew, false},
		{"ASSESS", domain.ChangeRequestStateAssess, false},
		{"AUTHORIZE", domain.ChangeRequestStateAuthorize, false},
		{"CUSTOMER_APPROVAL", domain.ChangeRequestStateCustomerApproval, true},
		{"SCHEDULED", domain.ChangeRequestStateScheduled, true},
		{"IMPLEMENT", domain.ChangeRequestStateImplement, true},
		{"REVIEW", domain.ChangeRequestStateReview, true},
		{"CUSTOMER_REVIEW", domain.ChangeRequestStateCustomerReview, true},
		{"ROLLBACK", domain.ChangeRequestStateRollback, true},
		{"CLOSED", domain.ChangeRequestStateClosed, true},
		{"CANCELED", domain.ChangeRequestStateCanceled, true},
	}
	var every []domain.ChangeRequestState
	for _, p := range phases {
		every = append(every, p.state)
	}
	hidden := []domain.ChangeRequestState{domain.ChangeRequestStateNew, domain.ChangeRequestStateAssess, domain.ChangeRequestStateAuthorize}

	f := newCustomerGroupFlow(t)
	vis := repository.CRVisibility{} // no cutover: every change request is legacy
	f.useVisibility(vis)
	r := f.visRepos(vis)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	alice := asContact(crScopeUserA1)
	stranger := f.personas()["stranger"].ctx

	for _, p := range phases {
		f.setStoredState(id, p.stored)
		for _, filter := range []struct {
			name   string
			states []domain.ChangeRequestState
			names  bool // whether it names p.state
		}{
			{"no state named", nil, true},
			{"that state alone", []domain.ChangeRequestState{p.state}, true},
			{"every state", every, true},
			{"New, Assess and Authorize", hidden, p.state == domain.ChangeRequestStateNew || p.state == domain.ChangeRequestStateAssess || p.state == domain.ChangeRequestStateAuthorize},
			{"another state alone", []domain.ChangeRequestState{otherState(p.state)}, false},
		} {
			want := p.legacy && filter.names
			listed, total := f.searchStatesAs(r, alice, crScopeProjectA, id, filter.states)
			if listed != want || total != btoi(want) {
				t.Errorf("a registered contact, change request in %s, %s: listed = %v (total %d), want %v", p.stored, filter.name, listed, total, want)
			}
			if listed, total := f.searchStatesAs(r, stranger, crScopeProjectA, id, filter.states); listed || total != 0 {
				t.Errorf("a stranger, change request in %s, %s: listed = %v (total %d), want nothing", p.stored, filter.name, listed, total)
			}
		}
	}
}

// A change request designated to the customer that sits in Authorize (one an older build
// sent back through the CAB when the customer proposed a time: a proposal today waits in
// Customer Approval, but the row stays on the customer's list wherever it is) is returned
// when no state is named, when Authorize is named and when every state is: and it is NOT
// returned to a contact it was never designated to, whatever they name (strict: nothing
// is legacy).
func TestChangeRequestVisibilityIntegration_NamingAStateNeverWidensWhatACustomerSees_Designated(t *testing.T) {
	f := newCustomerGroupFlow(t)
	vis := visStrictSinceLongAgo()
	f.useVisibility(vis)
	r := f.visRepos(vis)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	// The state an older build left it in (a proposal today keeps the change in Customer
	// Approval): forced, with the customer's request -- the designation -- exactly as the
	// real flow wrote it.
	f.setStoredState(id, "AUTHORIZE")
	f.expect(id, "in Authorize, as an older build left it", "AUTHORIZE", "canceled")

	alice, sam := asContact(crScopeUserA1), asContact(crScopeUserSecurity)
	authorize := []domain.ChangeRequestState{domain.ChangeRequestStateAuthorize}
	three := []domain.ChangeRequestState{domain.ChangeRequestStateNew, domain.ChangeRequestStateAssess, domain.ChangeRequestStateAuthorize}
	notAuthorize := []domain.ChangeRequestState{domain.ChangeRequestStateNew, domain.ChangeRequestStateAssess, domain.ChangeRequestStateScheduled}
	for _, tc := range []struct {
		name   string
		states []domain.ChangeRequestState
		alice  bool
	}{
		{"no state named", nil, true},
		{"Authorize named", authorize, true},
		{"New, Assess and Authorize named", three, true},
		{"states that exclude Authorize", notAuthorize, false},
	} {
		if listed, _ := f.searchStatesAs(r, alice, crScopeProjectA, id, tc.states); listed != tc.alice {
			t.Errorf("the contact it was designated to, %s: listed = %v, want %v", tc.name, listed, tc.alice)
		}
		if listed, total := f.searchStatesAs(r, sam, crScopeProjectA, id, tc.states); listed || total != 0 {
			t.Errorf("a registered contact it was never designated to, %s: listed = %v (total %d), want nothing", tc.name, listed, total)
		}
	}
}

func otherState(s domain.ChangeRequestState) domain.ChangeRequestState {
	if s == domain.ChangeRequestStateClosed {
		return domain.ChangeRequestStateCanceled
	}
	return domain.ChangeRequestStateClosed
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
