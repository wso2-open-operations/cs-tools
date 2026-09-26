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

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockEntityAnnouncementRegistryClient stubs the two upstream searches this
// handler orchestrates. Each Fn, when set, is called once per page the
// handler's own bounded loop requests; when unset, returns a single empty
// page so a test that doesn't care about one side of the fetch doesn't have
// to stub it.
type mockEntityAnnouncementRegistryClient struct {
	searchCasesFn                func(ctx context.Context, body []byte) ([]byte, error)
	searchAnnouncementRequestsFn func(ctx context.Context, body []byte) ([]byte, error)
}

func (m *mockEntityAnnouncementRegistryClient) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchCasesFn != nil {
		return m.searchCasesFn(ctx, body)
	}
	return []byte(`{"cases":[],"total":0,"offset":0,"limit":50}`), nil
}

func (m *mockEntityAnnouncementRegistryClient) SearchAnnouncementRequests(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchAnnouncementRequestsFn != nil {
		return m.searchAnnouncementRequestsFn(ctx, body)
	}
	return []byte(`{"requests":[],"total":0,"offset":0,"limit":50}`), nil
}

// singlePageCases returns a searchCasesFn serving exactly the given cases as
// one page (total = len(cases)), regardless of the requested offset.
func singlePageCases(casesJSON string, total int) func(context.Context, []byte) ([]byte, error) {
	return func(_ context.Context, body []byte) ([]byte, error) {
		var req struct {
			Pagination struct {
				Offset int `json:"offset"`
			} `json:"pagination"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Pagination.Offset > 0 {
			return []byte(`{"cases":[],"total":` + itoa(total) + `,"offset":` + itoa(req.Pagination.Offset) + `,"limit":50}`), nil
		}
		return []byte(`{"cases":` + casesJSON + `,"total":` + itoa(total) + `,"offset":0,"limit":50}`), nil
	}
}

func singlePageRequests(requestsJSON string, total int) func(context.Context, []byte) ([]byte, error) {
	return func(_ context.Context, body []byte) ([]byte, error) {
		var req struct {
			Pagination struct {
				Offset int `json:"offset"`
			} `json:"pagination"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Pagination.Offset > 0 {
			return []byte(`{"requests":[],"total":` + itoa(total) + `,"offset":` + itoa(req.Pagination.Offset) + `,"limit":50}`), nil
		}
		return []byte(`{"requests":` + requestsJSON + `,"total":` + itoa(total) + `,"offset":0,"limit":50}`), nil
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	if neg {
		digits = "-" + digits
	}
	return digits
}

func TestSearchAnnouncementRegistry_RequiresAuth(t *testing.T) {
	h := NewAnnouncementRegistryHandler(&mockEntityAnnouncementRegistryClient{})
	r := httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"pagination":{"limit":20}}`))
	w := httptest.NewRecorder()
	h.SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusUnauthorized)
}

// A negative offset used to reach rows[start:end] unvalidated — slicing a Go
// slice with a negative start index panics, so this must be rejected as a
// 400 before any fetch happens rather than crashing the request.
func TestSearchAnnouncementRegistry_RejectsNegativeOffset(t *testing.T) {
	h := NewAnnouncementRegistryHandler(&mockEntityAnnouncementRegistryClient{})
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"pagination":{"limit":20,"offset":-1}}`)))
	w := httptest.NewRecorder()
	h.SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestSearchAnnouncementRegistry_GroupsCasesBelongingToTheSameRequest(t *testing.T) {
	client := &mockEntityAnnouncementRegistryClient{
		searchCasesFn: singlePageCases(`[
			{"id":"case-1","number":"CS001","internalId":"ACME-1","subject":"Maintenance","updatedOn":"2026-07-02T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-1","name":"Acme"}},
			{"id":"case-2","number":"CS002","internalId":"BOLT-1","subject":"Maintenance","updatedOn":"2026-07-01T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-2","name":"Bolt"}}
		]`, 2),
		searchAnnouncementRequestsFn: singlePageRequests(`[
			{"id":"req-1","subject":"Maintenance","createdBy":"jane@example.com","createdAt":"2026-07-01T00:00:00Z","updatedAt":"2026-07-02T00:00:00Z","isSecurityAnnouncement":true,"publishedCaseIds":["case-1","case-2"]}
		]`, 1),
	}
	h := NewAnnouncementRegistryHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"pagination":{"limit":20}}`)))
	w := httptest.NewRecorder()
	h.SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusOK)

	var got registrySearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v; raw: %s", err, w.Body.String())
	}
	if len(got.Rows) != 1 {
		t.Fatalf("expected both cases collapsed into 1 row, got %d: %+v", len(got.Rows), got.Rows)
	}
	row := got.Rows[0]
	if row.Kind != "batch" || row.AnnouncementRequestID != "req-1" || row.ProjectCount != 2 {
		t.Fatalf("expected a batch row for req-1 with projectCount 2, got %+v", row)
	}
	if !row.IsSecurityAnnouncement {
		t.Fatalf("expected the batch row to forward the request's own isSecurityAnnouncement, got %+v", row)
	}
	if got.Total != 1 {
		t.Fatalf("total = %d, want 1", got.Total)
	}
	// The collapsed row must still carry every individual case's own number/
	// WSO2 reference/project — a batch row that only reports a count gives
	// the caller no way to ever find out which case belongs to which
	// project (a real gap reported live: nothing anywhere in the UI could
	// answer "what's the CS number for project X in this send").
	if len(row.Cases) != 2 {
		t.Fatalf("expected both member cases listed on the row, got %+v", row.Cases)
	}
	byCaseID := map[string]registryCaseMember{}
	for _, m := range row.Cases {
		byCaseID[m.CaseID] = m
	}
	if byCaseID["case-1"].CaseNumber != "CS001" || byCaseID["case-1"].WSO2CaseID != "ACME-1" || byCaseID["case-1"].ProjectName != "Acme" {
		t.Fatalf("expected case-1's own number/wso2CaseId/project, got %+v", byCaseID["case-1"])
	}
	if byCaseID["case-2"].CaseNumber != "CS002" || byCaseID["case-2"].WSO2CaseID != "BOLT-1" || byCaseID["case-2"].ProjectName != "Bolt" {
		t.Fatalf("expected case-2's own number/wso2CaseId/project, got %+v", byCaseID["case-2"])
	}
}

// Reported live: a batch row's Created By column showed a raw, unreadable
// IdP account id (e.g. "e441e951-a2f5-4813-b308-817f128f4660") right next to
// "case" rows in the same list, which already show a resolved name/email --
// registryAnnouncementRequestView.CreatedBy is that raw id, with nothing
// preferring the request's own CreatedByEmail the way the Pending tab's row
// already does.
func TestSearchAnnouncementRegistry_BatchRowPrefersCreatedByEmailOverTheRawId(t *testing.T) {
	client := &mockEntityAnnouncementRegistryClient{
		searchCasesFn: singlePageCases(`[
			{"id":"case-1","number":"CS001","internalId":"ACME-1","subject":"Maintenance","updatedOn":"2026-07-02T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-1","name":"Acme"}}
		]`, 1),
		searchAnnouncementRequestsFn: singlePageRequests(`[
			{"id":"req-1","subject":"Maintenance","createdBy":"e441e951-a2f5-4813-b308-817f128f4660","createdByEmail":"jane@example.com","createdAt":"2026-07-01T00:00:00Z","updatedAt":"2026-07-02T00:00:00Z","publishedCaseIds":["case-1"]}
		]`, 1),
	}
	h := NewAnnouncementRegistryHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"pagination":{"limit":20}}`)))
	w := httptest.NewRecorder()
	h.SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusOK)

	var got registrySearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v; raw: %s", err, w.Body.String())
	}
	if len(got.Rows) != 1 || got.Rows[0].CreatedBy != "jane@example.com" {
		t.Fatalf("expected the batch row's createdBy to be the resolved email, not the raw id, got %+v", got.Rows)
	}
}

// A batch of 3+ member cases is the shape reported live (a real published
// request whose "3 projects" summary had no way to reveal any of the 3
// underlying CS numbers) — locks in that every member beyond the first two
// is retained too, not just enough to prove grouping works at all.
func TestSearchAnnouncementRegistry_BatchRowListsEveryMemberCaseNotJustTheFirst(t *testing.T) {
	client := &mockEntityAnnouncementRegistryClient{
		searchCasesFn: singlePageCases(`[
			{"id":"case-1","number":"CS001","subject":"Maintenance","updatedOn":"2026-07-03T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-1","name":"Acme"}},
			{"id":"case-2","number":"CS002","subject":"Maintenance","updatedOn":"2026-07-02T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-2","name":"Bolt"}},
			{"id":"case-3","number":"CS003","subject":"Maintenance","updatedOn":"2026-07-01T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-3","name":"Cinder"}}
		]`, 3),
		searchAnnouncementRequestsFn: singlePageRequests(`[
			{"id":"req-1","subject":"Maintenance","createdBy":"jane@example.com","createdAt":"2026-07-01T00:00:00Z","updatedAt":"2026-07-03T00:00:00Z","publishedCaseIds":["case-1","case-2","case-3"]}
		]`, 1),
	}
	h := NewAnnouncementRegistryHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"pagination":{"limit":20}}`)))
	w := httptest.NewRecorder()
	h.SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusOK)

	var got registrySearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0].ProjectCount != 3 || len(got.Rows[0].Cases) != 3 {
		t.Fatalf("expected 1 row, projectCount 3, 3 member cases, got %+v", got.Rows)
	}
}

// A projectIds filter that matches only one of a batch's own cases must
// still list every member on the row — filters decide whether the batch is
// included at all (yes, since one of its cases matches), never which
// members get resolved once it is. Reported live: with the earlier
// filtered-only implementation, "Delivered to 3 projects" would have listed
// just the 1 matching case and silently dropped the other 2 CS numbers.
func TestSearchAnnouncementRegistry_BatchMembersResolvedIndependentlyOfCaseFilters(t *testing.T) {
	const oneMatchingCase = `[{"id":"case-1","number":"CS001","subject":"Maintenance","updatedOn":"2026-07-03T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-1","name":"Acme"}}]`
	const allThreeCases = `[
		{"id":"case-1","number":"CS001","subject":"Maintenance","updatedOn":"2026-07-03T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-1","name":"Acme"}},
		{"id":"case-2","number":"CS002","subject":"Maintenance","updatedOn":"2026-07-02T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-2","name":"Bolt"}},
		{"id":"case-3","number":"CS003","subject":"Maintenance","updatedOn":"2026-07-01T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-3","name":"Cinder"}}
	]`
	client := &mockEntityAnnouncementRegistryClient{
		searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
			var decoded struct {
				Filters struct {
					Filters []map[string]any `json:"filters"`
				} `json:"filters"`
			}
			_ = json.Unmarshal(body, &decoded)
			// The handler's own fieldFilters always starts with the
			// hardcoded type=announcement entry; a real projectIds filter
			// appends a second one. The internal unfiltered re-fetch (for
			// batch-member resolution) never adds that second entry.
			if len(decoded.Filters.Filters) > 1 {
				return []byte(`{"cases":` + oneMatchingCase + `,"total":1,"offset":0,"limit":50}`), nil
			}
			return []byte(`{"cases":` + allThreeCases + `,"total":3,"offset":0,"limit":50}`), nil
		},
		searchAnnouncementRequestsFn: singlePageRequests(`[
			{"id":"req-1","subject":"Maintenance","createdBy":"jane@example.com","createdAt":"2026-07-01T00:00:00Z","updatedAt":"2026-07-03T00:00:00Z","publishedCaseIds":["case-1","case-2","case-3"]}
		]`, 1),
	}
	h := NewAnnouncementRegistryHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"projectIds":["p-1"],"pagination":{"limit":20}}`)))
	w := httptest.NewRecorder()
	h.SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusOK)

	var got registrySearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0].ProjectCount != 3 {
		t.Fatalf("expected the batch row still included (matched via case-1) with projectCount 3, got %+v", got.Rows)
	}
	if len(got.Rows[0].Cases) != 3 {
		t.Fatalf("expected all 3 member cases resolved despite the projectIds filter matching only one, got %+v", got.Rows[0].Cases)
	}
}

// countingCaseSearch serves casesJSON as one page for every call and counts
// the ones whose filter list holds only the handler's hardcoded
// type=announcement entry — i.e. the internal unfiltered re-fetch, as
// opposed to a caller-filtered fetch, which always appends at least one
// more entry.
func countingCaseSearch(casesJSON string, total int, unfiltered *int) func(context.Context, []byte) ([]byte, error) {
	page := singlePageCases(casesJSON, total)
	return func(ctx context.Context, body []byte) ([]byte, error) {
		var decoded struct {
			Filters struct {
				Filters []map[string]any `json:"filters"`
			} `json:"filters"`
		}
		_ = json.Unmarshal(body, &decoded)
		if len(decoded.Filters.Filters) == 1 {
			*unfiltered++
		}
		return page(ctx, body)
	}
}

func TestSearchAnnouncementRegistry_SkipsTheUnfilteredRefetchWhenTheFilterKeptEveryMember(t *testing.T) {
	const allThreeCases = `[
		{"id":"case-1","number":"CS001","subject":"Maintenance","state":"Open","updatedOn":"2026-07-03T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-1","name":"Acme"}},
		{"id":"case-2","number":"CS002","subject":"Maintenance","state":"Open","updatedOn":"2026-07-02T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-2","name":"Bolt"}},
		{"id":"case-3","number":"CS003","subject":"Maintenance","state":"Open","updatedOn":"2026-07-01T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-3","name":"Cinder"}}
	]`
	unfiltered := 0
	client := &mockEntityAnnouncementRegistryClient{
		searchCasesFn: countingCaseSearch(allThreeCases, 3, &unfiltered),
		searchAnnouncementRequestsFn: singlePageRequests(`[
			{"id":"req-1","subject":"Maintenance","createdBy":"jane@example.com","createdAt":"2026-07-01T00:00:00Z","updatedAt":"2026-07-03T00:00:00Z","publishedCaseIds":["case-1","case-2","case-3"]}
		]`, 1),
	}
	h := NewAnnouncementRegistryHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"states":["Open"],"pagination":{"limit":20}}`)))
	w := httptest.NewRecorder()
	h.SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusOK)

	var got registrySearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Rows) != 1 || len(got.Rows[0].Cases) != 3 {
		t.Fatalf("expected one batch row listing all 3 members, got %+v", got.Rows)
	}
	if unfiltered != 0 {
		t.Fatalf("expected no unfiltered re-fetch when the filter kept every member of the displayed batch, got %d", unfiltered)
	}
}

func TestSearchAnnouncementRegistry_ResolvesMembersOnlyForTheRequestedPage(t *testing.T) {
	// case-1b belongs to req-1 but is hidden by the caller's filter, so
	// req-1's row is the one that would force an unfiltered re-fetch. It
	// sorts second, so a limit of 1 never returns it — and the page that is
	// returned must not pay for it.
	const filteredCases = `[
		{"id":"case-2a","number":"CS020","subject":"Upgrade","updatedOn":"2026-07-09T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-9","name":"Zenith"}},
		{"id":"case-1","number":"CS001","subject":"Maintenance","updatedOn":"2026-07-03T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-1","name":"Acme"}}
	]`
	unfiltered := 0
	client := &mockEntityAnnouncementRegistryClient{
		searchCasesFn: countingCaseSearch(filteredCases, 2, &unfiltered),
		searchAnnouncementRequestsFn: singlePageRequests(`[
			{"id":"req-1","subject":"Maintenance","createdBy":"jane@example.com","createdAt":"2026-07-01T00:00:00Z","updatedAt":"2026-07-03T00:00:00Z","publishedCaseIds":["case-1","case-1b"]},
			{"id":"req-2","subject":"Upgrade","createdBy":"jane@example.com","createdAt":"2026-07-01T00:00:00Z","updatedAt":"2026-07-09T00:00:00Z","publishedCaseIds":["case-2a"]}
		]`, 2),
	}
	h := NewAnnouncementRegistryHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"projectIds":["p-9","p-1"],"pagination":{"limit":1}}`)))
	w := httptest.NewRecorder()
	h.SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusOK)

	var got registrySearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Total != 2 || len(got.Rows) != 1 || got.Rows[0].AnnouncementRequestID != "req-2" {
		t.Fatalf("expected page 1 to hold req-2's row only, out of 2 total, got total=%d rows=%+v", got.Total, got.Rows)
	}
	if len(got.Rows[0].Cases) != 1 || got.Rows[0].Cases[0].CaseNumber != "CS020" {
		t.Fatalf("expected the returned row's own member resolved, got %+v", got.Rows[0].Cases)
	}
	if unfiltered != 0 {
		t.Fatalf("expected no unfiltered re-fetch for a member belonging to a row off the requested page, got %d", unfiltered)
	}
}

func TestSearchAnnouncementRegistry_ShowsACaseWithNoKnownRequestAsItsOwnRow(t *testing.T) {
	client := &mockEntityAnnouncementRegistryClient{
		searchCasesFn: singlePageCases(`[
			{"id":"case-legacy","number":"CS999","subject":"Old announcement","state":"Closed","updatedOn":"2026-01-01T00:00:00Z","createdOn":"2026-01-01T00:00:00Z","project":{"id":"p-1","name":"Acme"}}
		]`, 1),
	}
	h := NewAnnouncementRegistryHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"pagination":{"limit":20}}`)))
	w := httptest.NewRecorder()
	h.SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusOK)

	var got registrySearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0].Kind != "case" || got.Rows[0].CaseID != "case-legacy" {
		t.Fatalf("expected a single case row for the unmatched case, got %+v", got.Rows)
	}
	if got.Rows[0].ProjectName != "Acme" {
		t.Fatalf("expected projectName forwarded, got %+v", got.Rows[0])
	}
}

func TestSearchAnnouncementRegistry_PaginatesTheGroupedResult(t *testing.T) {
	client := &mockEntityAnnouncementRegistryClient{
		searchCasesFn: singlePageCases(`[
			{"id":"case-1","number":"CS001","subject":"First","updatedOn":"2026-07-03T00:00:00Z","createdOn":"2026-07-01T00:00:00Z"},
			{"id":"case-2","number":"CS002","subject":"Second","updatedOn":"2026-07-02T00:00:00Z","createdOn":"2026-07-01T00:00:00Z"},
			{"id":"case-3","number":"CS003","subject":"Third","updatedOn":"2026-07-01T00:00:00Z","createdOn":"2026-07-01T00:00:00Z"}
		]`, 3),
	}
	h := NewAnnouncementRegistryHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"pagination":{"offset":1,"limit":1}}`)))
	w := httptest.NewRecorder()
	h.SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusOK)

	var got registrySearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0].CaseID != "case-2" {
		t.Fatalf("expected page 2 (offset 1, limit 1) to be case-2, got %+v", got.Rows)
	}
	if got.Total != 3 || !got.HasMore {
		t.Fatalf("expected total=3, hasMore=true, got total=%d hasMore=%v", got.Total, got.HasMore)
	}
}

func TestSearchAnnouncementRegistry_FailsLoudlyRatherThanSilentlyTruncating(t *testing.T) {
	// A total that never matches how many rows are actually ever returned
	// forces fetchAllMatchingCases' safety bound to trip.
	client := &mockEntityAnnouncementRegistryClient{
		searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
			return []byte(`{"cases":[{"id":"case-1","number":"CS001","subject":"x","updatedOn":"2026-01-01T00:00:00Z","createdOn":"2026-01-01T00:00:00Z"}],"total":1000000,"offset":0,"limit":50}`), nil
		},
	}
	h := NewAnnouncementRegistryHandler(client)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"pagination":{"limit":20}}`)))
	w := httptest.NewRecorder()
	h.SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusInternalServerError)
}
