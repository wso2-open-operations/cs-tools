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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The ServiceNow data source is the one that has no per-customer designation, so
// the legacy visibility rule (every state except New, Assess and Authorize) is
// applied by this service: see sn_change_request_customer_view.go. Everything
// below runs against a FAKE ServiceNow that does what the real search does not
// promise to do: it answers every state it holds, whatever the request asked for,
// so a test passes only when this service itself keeps the three states out.

const snViewProject = "00000000-0000-4000-8000-000000000001"

// The request's scope is resolved once per request by callerIdentityMiddleware
// (AccessService.ResolveScope) and carried in the context: unrestricted means WSO2
// staff, anything else (or nothing resolved at all) is a customer.
func snStaffCtx() context.Context {
	return repository.WithCallerIdentity(contextWithUserIDToken("token"), repository.SearchScope{Unrestricted: true, HasInternalAccess: true})
}

func snCustomerCtx() context.Context {
	return repository.WithCallerIdentity(contextWithUserIDToken("token"), repository.SearchScope{
		ViewerEmail: "dana@customer.example", ProjectIDs: []string{snViewProject},
	})
}

// snViewStates is ServiceNow's own vocabulary: label -> numeric key.
var snViewStates = []struct {
	label string
	key   int
	enum  domain.ChangeRequestState
}{
	{"New", -5, domain.ChangeRequestStateNew},
	{"Assess", -4, domain.ChangeRequestStateAssess},
	{"Authorize", -3, domain.ChangeRequestStateAuthorize},
	{"Customer Approval", 5, domain.ChangeRequestStateCustomerApproval},
	{"Scheduled", -2, domain.ChangeRequestStateScheduled},
	{"Implement", -1, domain.ChangeRequestStateImplement},
	{"Review", 0, domain.ChangeRequestStateReview},
	{"Customer Review", 1, domain.ChangeRequestStateCustomerReview},
	{"Rollback", 2, domain.ChangeRequestStateRollback},
	{"Closed", 3, domain.ChangeRequestStateClosed},
	{"Canceled", 4, domain.ChangeRequestStateCanceled},
}

// fakeServiceNowSearch records what the service sent and answers one change
// request in EVERY state, ignoring the state list it was sent.
type fakeServiceNowSearch struct {
	calls atomic.Int32
	body  map[string]any
}

func (f *fakeServiceNowSearch) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests/search", func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if err := json.NewDecoder(r.Body).Decode(&f.body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		rows := make([]map[string]any, 0, len(snViewStates))
		for i, st := range snViewStates {
			rows = append(rows, map[string]any{
				"id":        fmt.Sprintf("%032x", i+1),
				"number":    fmt.Sprintf("CR-FAKE-%02d", i+1),
				"title":     "Example change " + st.label,
				"createdOn": "2026-01-01 00:00:00",
				"project":   map[string]any{"id": "00000000000040008000000000000001", "name": "Example Corp Platform"},
				"state":     map[string]any{"label": st.label},
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"changeRequests": rows, "totalRecords": len(rows), "offset": 0, "limit": 50})
	})
	mux.HandleFunc("/change-requests/aggregate", func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if err := json.NewDecoder(r.Body).Decode(&f.body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		groups := make([]map[string]any, 0, len(snViewStates))
		for _, st := range snViewStates {
			groups = append(groups, map[string]any{"key": fmt.Sprint(st.key), "label": st.label, "count": 2})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"groups": groups, "othersCount": 0, "totalRecords": 2 * len(snViewStates)})
	})
	return mux
}

func (f *fakeServiceNowSearch) sentStateKeys(t *testing.T) []int {
	t.Helper()
	filters, _ := f.body["filters"].(map[string]any)
	raw, present := filters["stateKeys"]
	if !present {
		return nil
	}
	list, _ := raw.([]any)
	keys := make([]int, 0, len(list))
	for _, v := range list {
		keys = append(keys, int(v.(float64)))
	}
	return keys
}

func snViewSearch(states ...domain.ChangeRequestState) domain.SearchChangeRequestsRequest {
	return domain.SearchChangeRequestsRequest{
		Filters:    domain.SearchChangeRequestsFilters{ProjectIDs: []string{snViewProject}, States: states},
		Pagination: domain.Pagination{Limit: 50},
	}
}

func hiddenKeys() []int  { return []int{-5, -4, -3} }
func visibleKeys() []int { return []int{5, -2, -1, 0, 1, 2, 3, 4} }

// The rule itself, as a table: the ServiceNow keys a customer's search can carry.
func TestCustomerVisibleChangeRequestStates(t *testing.T) {
	got := domainCRStatesToSNIDs(customerVisibleChangeRequestStates)
	if !slices.Equal(got, visibleKeys()) {
		t.Fatalf("a customer's visible states map to %v, want %v (everything past Authorize)", got, visibleKeys())
	}
	for _, st := range snViewStates {
		hidden := slices.Contains(hiddenKeys(), st.key)
		if got := isCustomerVisibleChangeRequestState(st.enum); got == hidden {
			t.Errorf("state %s (key %d): visible = %v, want %v", st.label, st.key, got, !hidden)
		}
	}
	if isCustomerVisibleChangeRequestState("something_servicenow_added") || isCustomerVisibleChangeRequestState("") {
		t.Error("a state this service has no word for must not be visible: the rule is a list of what may be shown")
	}
}

// A customer who names NO state is not "unfiltered": the search carries every
// visible state, never New, Assess or Authorize, and the three rows ServiceNow
// answered anyway are not shown.
func TestSNChangeRequestSearch_CustomerNamingNoStateGetsTheVisibleStatesOnly(t *testing.T) {
	fake := &fakeServiceNowSearch{}
	svc := NewServiceNowChangeRequestService(newTestSNClient(t, fake.handler(t)))

	res, err := svc.SearchChangeRequests(snCustomerCtx(), snViewSearch())
	if err != nil {
		t.Fatalf("SearchChangeRequests: %v", err)
	}
	if got := fake.sentStateKeys(t); !slices.Equal(got, visibleKeys()) {
		t.Fatalf("stateKeys sent to ServiceNow = %v, want %v (an absent list would return every state)", got, visibleKeys())
	}
	assertNoHiddenRows(t, res)
	if len(res.ChangeRequests) != len(visibleKeys()) {
		t.Fatalf("a customer got %d change requests, want the %d in visible states", len(res.ChangeRequests), len(visibleKeys()))
	}
	if res.Total != len(visibleKeys()) {
		t.Fatalf("total = %d, want %d: a change request that is not shown is not counted", res.Total, len(visibleKeys()))
	}
}

// A customer who NAMES a hidden state (the id the state filter used to offer, or one
// typed into the API by hand) is answered "none" without a request: it never
// becomes "no state filter".
func TestSNChangeRequestSearch_CustomerNamingOnlyHiddenStatesGetsNothingAndAsksNothing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		states []domain.ChangeRequestState
	}{
		{"Authorize", []domain.ChangeRequestState{domain.ChangeRequestStateAuthorize}},
		{"Assess", []domain.ChangeRequestState{domain.ChangeRequestStateAssess}},
		{"New", []domain.ChangeRequestState{domain.ChangeRequestStateNew}},
		{"all three", []domain.ChangeRequestState{domain.ChangeRequestStateNew, domain.ChangeRequestStateAssess, domain.ChangeRequestStateAuthorize}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeServiceNowSearch{}
			svc := NewServiceNowChangeRequestService(newTestSNClient(t, fake.handler(t)))
			res, err := svc.SearchChangeRequests(snCustomerCtx(), snViewSearch(tc.states...))
			if err != nil {
				t.Fatalf("SearchChangeRequests: %v", err)
			}
			if fake.calls.Load() != 0 {
				t.Fatalf("ServiceNow was asked %d time(s) for a state no customer may see; the answer is none", fake.calls.Load())
			}
			if len(res.ChangeRequests) != 0 || res.Total != 0 {
				t.Fatalf("got %d change requests (total %d), want none", len(res.ChangeRequests), res.Total)
			}
			if res.Limit != 50 {
				t.Fatalf("limit = %d, want the page size asked for (50)", res.Limit)
			}
		})
	}
}

// Visible states named next to hidden ones keep only the visible ones.
func TestSNChangeRequestSearch_CustomerNamingMixedStatesKeepsTheVisibleOnes(t *testing.T) {
	fake := &fakeServiceNowSearch{}
	svc := NewServiceNowChangeRequestService(newTestSNClient(t, fake.handler(t)))
	res, err := svc.SearchChangeRequests(snCustomerCtx(),
		snViewSearch(domain.ChangeRequestStateAuthorize, domain.ChangeRequestStateScheduled, domain.ChangeRequestStateNew, domain.ChangeRequestStateClosed))
	if err != nil {
		t.Fatalf("SearchChangeRequests: %v", err)
	}
	if got := fake.sentStateKeys(t); !slices.Equal(got, []int{-2, 3}) {
		t.Fatalf("stateKeys sent = %v, want [-2 3] (Scheduled, Closed)", got)
	}
	assertNoHiddenRows(t, res)
}

// Staff are not narrowed: what they name is what is sent, and naming nothing is
// still no state filter.
func TestSNChangeRequestSearch_StaffAreNotNarrowed(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{"a scope resolved as unrestricted", snStaffCtx()},
		{"unrestricted, with a customer record of their own", repository.WithCallerIdentity(contextWithUserIDToken("token"), repository.SearchScope{Unrestricted: true})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeServiceNowSearch{}
			svc := NewServiceNowChangeRequestService(newTestSNClient(t, fake.handler(t)))

			if _, err := svc.SearchChangeRequests(tc.ctx, snViewSearch(domain.ChangeRequestStateNew, domain.ChangeRequestStateAssess, domain.ChangeRequestStateAuthorize)); err != nil {
				t.Fatalf("SearchChangeRequests: %v", err)
			}
			if got := fake.sentStateKeys(t); !slices.Equal(got, hiddenKeys()) {
				t.Fatalf("stateKeys sent = %v, want %v as asked", got, hiddenKeys())
			}

			res, err := svc.SearchChangeRequests(tc.ctx, snViewSearch())
			if err != nil {
				t.Fatalf("SearchChangeRequests: %v", err)
			}
			if got := fake.sentStateKeys(t); got != nil {
				t.Fatalf("stateKeys sent = %v, want none (no filter) for staff naming nothing", got)
			}
			if len(res.ChangeRequests) != len(snViewStates) {
				t.Fatalf("staff got %d change requests, want all %d", len(res.ChangeRequests), len(snViewStates))
			}
		})
	}
}

// Anyone whose scope is not unrestricted gets the customer's view, and so does a
// request for which no scope was resolved at all (no identity, an unknown client, a
// customer portal client with no database to look the user up in): fail closed.
func TestSNChangeRequestSearch_EveryoneWhoIsNotUnrestrictedGetsTheCustomerView(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{"a customer's scope", snCustomerCtx()},
		{"a scope with no email and no projects", repository.WithCallerIdentity(contextWithUserIDToken("token"), repository.SearchScope{})},
		{"WSO2 staff who also hold a customer record (external wins)", repository.WithCallerIdentity(contextWithUserIDToken("token"), repository.SearchScope{HasInternalAccess: true, ViewerEmail: "agent@example.test"})},
		{"no scope resolved", contextWithUserIDToken("token")},
		{"no scope and no user token", context.Background()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeServiceNowSearch{}
			svc := NewServiceNowChangeRequestService(newTestSNClient(t, fake.handler(t)))
			res, err := svc.SearchChangeRequests(tc.ctx, snViewSearch())
			if err != nil {
				t.Fatalf("SearchChangeRequests: %v", err)
			}
			if got := fake.sentStateKeys(t); !slices.Equal(got, visibleKeys()) {
				t.Fatalf("stateKeys sent = %v, want %v", got, visibleKeys())
			}
			assertNoHiddenRows(t, res)
		})
	}
}

// The narrowing comes after validation: a malformed request is still a 400.
func TestSNChangeRequestSearch_CustomerStillGetsValidationErrors(t *testing.T) {
	svc := NewServiceNowChangeRequestService(nil)
	_, err := svc.SearchChangeRequests(snCustomerCtx(), snViewSearch("not_a_state"))
	if err == nil {
		t.Fatal("an unknown state was accepted")
	}
}

func assertNoHiddenRows(t *testing.T, res domain.SearchChangeRequestsResponse) {
	t.Helper()
	for _, cr := range res.ChangeRequests {
		state := ""
		if cr.State != nil {
			state = *cr.State
		}
		if !isCustomerVisibleChangeRequestState(domain.ChangeRequestState(state)) {
			t.Errorf("a customer was handed change request %s in state %q", cr.Number, state)
		}
	}
}

// The aggregate (a count of change requests per state) follows the same rule.
func TestSNChangeRequestAggregate_CustomerGetsTheVisibleStatesOnly(t *testing.T) {
	fake := &fakeServiceNowSearch{}
	svc := NewServiceNowChangeRequestService(newTestSNClient(t, fake.handler(t)))
	req := domain.AggregateChangeRequestsRequest{GroupBy: "state", Filters: domain.SearchChangeRequestsFilters{ProjectIDs: []string{snViewProject}}}

	res, err := svc.AggregateChangeRequests(snCustomerCtx(), req)
	if err != nil {
		t.Fatalf("AggregateChangeRequests: %v", err)
	}
	if got := fake.sentStateKeys(t); !slices.Equal(got, visibleKeys()) {
		t.Fatalf("stateKeys sent = %v, want %v", got, visibleKeys())
	}
	if len(res.Groups) != len(visibleKeys()) {
		t.Fatalf("a customer got %d state buckets, want %d", len(res.Groups), len(visibleKeys()))
	}
	for _, b := range res.Groups {
		if !isCustomerVisibleChangeRequestState(domain.ChangeRequestState(b.Key)) {
			t.Errorf("a customer was handed the bucket %q", b.Key)
		}
	}
	if res.TotalRecords != 2*len(visibleKeys()) {
		t.Fatalf("total = %d, want %d: the hidden buckets are not counted", res.TotalRecords, 2*len(visibleKeys()))
	}

	// Hidden states only: none, and nothing is asked.
	fake2 := &fakeServiceNowSearch{}
	svc = NewServiceNowChangeRequestService(newTestSNClient(t, fake2.handler(t)))
	req.Filters.States = []domain.ChangeRequestState{domain.ChangeRequestStateAuthorize}
	res, err = svc.AggregateChangeRequests(snCustomerCtx(), req)
	if err != nil {
		t.Fatalf("AggregateChangeRequests: %v", err)
	}
	if fake2.calls.Load() != 0 || len(res.Groups) != 0 || res.TotalRecords != 0 {
		t.Fatalf("an aggregate of Authorize alone asked %d time(s) and returned %+v, want no request and no buckets", fake2.calls.Load(), res)
	}

	// Staff keep all eleven.
	fake3 := &fakeServiceNowSearch{}
	svc = NewServiceNowChangeRequestService(newTestSNClient(t, fake3.handler(t)))
	req.Filters.States = nil
	res, err = svc.AggregateChangeRequests(snStaffCtx(), req)
	if err != nil {
		t.Fatalf("AggregateChangeRequests: %v", err)
	}
	if len(res.Groups) != len(snViewStates) {
		t.Fatalf("staff got %d state buckets, want all %d", len(res.Groups), len(snViewStates))
	}
}

// GET /projects/{id}/metadata is what every state filter is built from: ServiceNow's
// own vocabulary carries all eleven states, and a customer is offered the eight they
// may see.
func TestSNProjectMetadata_CustomerIsNotOfferedNewAssessOrAuthorize(t *testing.T) {
	choices := make([]map[string]any, 0, len(snViewStates))
	for _, st := range snViewStates {
		choices = append(choices, map[string]any{"id": st.key, "label": st.label})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"changeRequestStates": choices, "caseStates": []map[string]any{{"id": 1, "label": "Open"}}})
	})
	client := newTestSNClient(t, mux)

	ids := func(items []domain.ChoiceListItem) []string {
		out := make([]string, 0, len(items))
		for _, i := range items {
			out = append(out, i.ID)
		}
		return out
	}

	customer, err := NewServiceNowProjectStatsService(client).
		GetProjectMetadata(snCustomerCtx(), snViewProject)
	if err != nil {
		t.Fatalf("GetProjectMetadata: %v", err)
	}
	if got, want := ids(customer.ChangeRequestStates), []string{"5", "-2", "-1", "0", "1", "2", "3", "4"}; !slices.Equal(got, want) {
		t.Fatalf("a customer is offered the change request states %v, want %v", got, want)
	}
	if len(customer.CaseStates) != 1 {
		t.Fatalf("the other choice lists are not touched: case states = %v", customer.CaseStates)
	}

	staff, err := NewServiceNowProjectStatsService(client).
		GetProjectMetadata(snStaffCtx(), snViewProject)
	if err != nil {
		t.Fatalf("GetProjectMetadata: %v", err)
	}
	if len(staff.ChangeRequestStates) != len(snViewStates) {
		t.Fatalf("staff are offered %d change request states, want all %d", len(staff.ChangeRequestStates), len(snViewStates))
	}

	// A state is hidden by its key OR its label, whichever the choice carries.
	for _, tc := range []struct {
		item   domain.ChoiceListItem
		hidden bool
	}{
		{domain.ChoiceListItem{ID: "-3", Label: "Authorize"}, true},
		{domain.ChoiceListItem{ID: "-3"}, true},
		{domain.ChoiceListItem{ID: "-4", Label: "whatever"}, true},
		{domain.ChoiceListItem{ID: "-5", Label: "New"}, true},
		{domain.ChoiceListItem{ID: "x", Label: " authorize "}, true},
		{domain.ChoiceListItem{ID: "x", Label: "ASSESS"}, true},
		{domain.ChoiceListItem{ID: "x", Label: "New"}, true},
		{domain.ChoiceListItem{ID: "-2", Label: "Scheduled"}, false},
		{domain.ChoiceListItem{ID: "5", Label: "Customer Approval"}, false},
		{domain.ChoiceListItem{ID: "0", Label: "Review"}, false},
	} {
		if got := isCustomerHiddenChangeRequestStateItem(tc.item); got != tc.hidden {
			t.Errorf("isCustomerHiddenChangeRequestStateItem(%+v) = %v, want %v", tc.item, got, tc.hidden)
		}
	}
}

// The change request stats carry a count per state. A customer is not told how many
// change requests the project has in New, Assess or Authorize; the totals are
// ServiceNow's own (over every state) and pass through.
func TestSNProjectChangeRequestStats_CustomerIsNotToldTheCountsOfHiddenStates(t *testing.T) {
	counts := make([]map[string]any, 0, len(snViewStates))
	for _, st := range snViewStates {
		counts = append(counts, map[string]any{"id": st.key, "label": st.label, "count": 1})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"totalCount": len(snViewStates), "activeCount": 8, "outstandingCount": 8, "actionRequiredCount": 2,
			"stateCount": counts, "resolvedCount": map[string]any{"total": 1, "currentMonth": 0, "pastThirtyDays": 0},
		})
	})
	svc := NewServiceNowProjectStatsService(newTestSNClient(t, mux))

	customer, err := svc.GetProjectChangeRequestStats(snCustomerCtx(), snViewProject)
	if err != nil {
		t.Fatalf("GetProjectChangeRequestStats: %v", err)
	}
	var keys []string
	for _, c := range customer.StateCount {
		keys = append(keys, c.ID)
	}
	if want := []string{"5", "-2", "-1", "0", "1", "2", "3", "4"}; !slices.Equal(keys, want) {
		t.Fatalf("a customer is told the counts of %v, want %v", keys, want)
	}
	if customer.TotalCount != len(snViewStates) || customer.OutstandingCount != 8 {
		t.Fatalf("totals = %d / %d, want ServiceNow's own (%d / 8) passed through", customer.TotalCount, customer.OutstandingCount, len(snViewStates))
	}

	staff, err := svc.GetProjectChangeRequestStats(snStaffCtx(), snViewProject)
	if err != nil {
		t.Fatalf("GetProjectChangeRequestStats: %v", err)
	}
	if len(staff.StateCount) != len(snViewStates) {
		t.Fatalf("staff are told %d state counts, want all %d", len(staff.StateCount), len(snViewStates))
	}
}
