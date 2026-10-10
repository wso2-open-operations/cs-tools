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
	"sync/atomic"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// rowsRegistryClient is the registry client plus the optional grouped-rows
// read. The plain mock does not implement it, so older tests keep exercising
// the legacy path unchanged.
type rowsRegistryClient struct {
	mockEntityAnnouncementRegistryClient
	rowsFn     func(ctx context.Context, body []byte) ([]byte, error)
	rowsCalls  atomic.Int32
	legacyHits atomic.Int32
}

func (m *rowsRegistryClient) SearchAnnouncementRegistryRows(ctx context.Context, body []byte) ([]byte, error) {
	m.rowsCalls.Add(1)
	return m.rowsFn(ctx, body)
}

func (m *rowsRegistryClient) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	m.legacyHits.Add(1)
	return m.mockEntityAnnouncementRegistryClient.SearchCases(ctx, body)
}

const rowsPayload = `{"rows":[` +
	`{"kind":"batch","subject":"Maintenance","createdBy":"jane.doe@example.com","createdOn":"2026-07-01T00:00:00Z","updatedOn":"2026-07-02T00:00:00Z","announcementRequestId":"00000000-0000-0000-0000-000000000001","projectCount":2,"isSecurityAnnouncement":true,` +
	`"cases":[{"caseId":"case-1","caseNumber":"CS001","wso2CaseId":"ACME-1","projectName":"Acme"},{"caseId":"case-2","caseNumber":"CS002","wso2CaseId":"BOLT-1","projectName":"Bolt"}]},` +
	`{"kind":"case","subject":"Legacy","createdBy":"Jane Doe","createdOn":"2026-06-01T00:00:00Z","updatedOn":"2026-06-02T00:00:00Z","caseId":"case-3","caseNumber":"CS003","wso2CaseId":"CORP-1","state":"open","projectName":"Corp"}` +
	`],"total":7,"limit":2,"offset":4,"hasMore":true}`

func staticRows(payload string) func(context.Context, []byte) ([]byte, error) {
	return func(context.Context, []byte) ([]byte, error) { return []byte(payload), nil }
}

func failingRows(status int) func(context.Context, []byte) ([]byte, error) {
	return func(context.Context, []byte) ([]byte, error) {
		return nil, &apierror.Error{StatusCode: status, Body: `{"message":"upstream secret detail"}`}
	}
}

// sentRowsBody captures the body the handler sends to the rows route.
func sentRowsBody(t *testing.T, search string) map[string]any {
	t.Helper()
	var sent []byte
	client := &rowsRegistryClient{rowsFn: func(_ context.Context, body []byte) ([]byte, error) {
		sent = body
		return []byte(`{"rows":[],"total":0,"limit":20,"offset":0,"hasMore":false}`), nil
	}}
	code, body := runRegistry(t, NewAnnouncementRegistryHandler(client), search)
	if code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", code, body)
	}
	var got map[string]any
	if err := json.Unmarshal(sent, &got); err != nil {
		t.Fatalf("sent body is not JSON: %v (%s)", err, sent)
	}
	return got
}

func TestSearchAnnouncementRegistry_RowsRouteUsedAndPayloadPassedThrough(t *testing.T) {
	client := &rowsRegistryClient{rowsFn: staticRows(rowsPayload)}
	client.searchCasesFn = func(context.Context, []byte) ([]byte, error) {
		t.Error("legacy /cases/search was called although the rows route succeeded")
		return nil, nil
	}
	code, body := runRegistry(t, NewAnnouncementRegistryHandler(client), `{"pagination":{"offset":4,"limit":2}}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", code, body)
	}
	var want, got any
	if err := json.Unmarshal([]byte(rowsPayload), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(wantJSON) != string(gotJSON) {
		t.Fatalf("payload changed in transit:\n got  %s\n want %s", gotJSON, wantJSON)
	}
	if n := client.rowsCalls.Load(); n != 1 {
		t.Fatalf("rows route called %d times, want 1", n)
	}
}

func TestSearchAnnouncementRegistry_RowsRequestBody(t *testing.T) {
	t.Run("no filter", func(t *testing.T) {
		got := sentRowsBody(t, `{"pagination":{"offset":10,"limit":20}}`)
		filters := got["filters"].(map[string]any)
		if list := filters["filters"].([]any); len(list) != 0 {
			t.Fatalf("filters = %v, want empty", list)
		}
		if _, ok := filters["searchQuery"]; ok {
			t.Fatalf("searchQuery present without a search: %v", filters)
		}
		p := got["pagination"].(map[string]any)
		if p["offset"].(float64) != 10 || p["limit"].(float64) != 20 {
			t.Fatalf("pagination = %v", p)
		}
	})
	t.Run("states, projects and search", func(t *testing.T) {
		got := sentRowsBody(t, `{"states":["open","closed"],"projectIds":["proj-1"],"search":"eol","pagination":{"limit":20}}`)
		filters := got["filters"].(map[string]any)
		if filters["searchQuery"] != "eol" {
			t.Fatalf("searchQuery = %v", filters["searchQuery"])
		}
		list := filters["filters"].([]any)
		if len(list) != 2 {
			t.Fatalf("filters = %v", list)
		}
		state := list[0].(map[string]any)
		if state["field"] != "state" || state["op"] != "in" || len(state["values"].([]any)) != 2 {
			t.Fatalf("state filter = %v", state)
		}
		proj := list[1].(map[string]any)
		if proj["field"] != "projectId" || proj["op"] != "in" || proj["values"].([]any)[0] != "proj-1" {
			t.Fatalf("project filter = %v", proj)
		}
		for _, f := range list {
			if f.(map[string]any)["field"] == "type" {
				t.Fatal("type filter must not be sent: entity-service forces it")
			}
		}
	})
	t.Run("search only", func(t *testing.T) {
		got := sentRowsBody(t, `{"search":"patch","pagination":{"limit":5}}`)
		filters := got["filters"].(map[string]any)
		if filters["searchQuery"] != "patch" || len(filters["filters"].([]any)) != 0 {
			t.Fatalf("filters = %v", filters)
		}
	})
}

func TestSearchAnnouncementRegistry_RowsLimitClamping(t *testing.T) {
	cases := []struct {
		name, in string
		want     float64
	}{
		{"over the cap is clamped to 50", `{"pagination":{"limit":500}}`, 50},
		{"exactly the cap is kept", `{"pagination":{"limit":50}}`, 50},
		{"missing limit defaults to 20", `{"pagination":{}}`, 20},
		{"zero limit defaults to 20", `{"pagination":{"limit":0}}`, 20},
		{"negative limit defaults to 20", `{"pagination":{"limit":-3}}`, 20},
		{"small limit is kept", `{"pagination":{"limit":7}}`, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sentRowsBody(t, tc.in)
			if l := got["pagination"].(map[string]any)["limit"].(float64); l != tc.want {
				t.Fatalf("limit sent = %v, want %v", l, tc.want)
			}
		})
	}
}

// The response limit is whatever entity-service reports, not what was asked.
func TestSearchAnnouncementRegistry_ResponseLimitReflectsEntityResponse(t *testing.T) {
	client := &rowsRegistryClient{rowsFn: staticRows(`{"rows":[],"total":0,"limit":50,"offset":0,"hasMore":false}`)}
	code, body := runRegistry(t, NewAnnouncementRegistryHandler(client), `{"pagination":{"limit":500}}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	var got registrySearchResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Limit != 50 || got.Rows == nil {
		t.Fatalf("limit = %d rows nil = %v", got.Limit, got.Rows == nil)
	}
	if !strings.Contains(string(body), `"rows":[]`) {
		t.Fatalf("rows must serialise as [], body = %s", body)
	}
}

func TestSearchAnnouncementRegistry_Rows404FallsBackToLegacyGrouping(t *testing.T) {
	client := &rowsRegistryClient{rowsFn: failingRows(http.StatusNotFound)}
	client.searchCasesFn = singlePageCases(`[
		{"id":"case-1","number":"CS001","internalId":"ACME-1","subject":"Maintenance","updatedOn":"2026-07-02T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-1","name":"Acme"}},
		{"id":"case-2","number":"CS002","internalId":"BOLT-1","subject":"Maintenance","updatedOn":"2026-07-01T00:00:00Z","createdOn":"2026-07-01T00:00:00Z","project":{"id":"p-2","name":"Bolt"}}
	]`, 2)
	client.searchAnnouncementRequestsFn = singlePageRequests(`[
		{"id":"req-1","subject":"Maintenance","createdBy":"jane@example.com","createdAt":"2026-07-01T00:00:00Z","updatedAt":"2026-07-02T00:00:00Z","isSecurityAnnouncement":true,"publishedCaseIds":["case-1","case-2"]}
	]`, 1)
	code, body := runRegistry(t, NewAnnouncementRegistryHandler(client), `{"pagination":{"limit":20}}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", code, body)
	}
	var got registrySearchResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || len(got.Rows) != 1 || got.Rows[0].Kind != "batch" || len(got.Rows[0].Cases) != 2 || got.Limit != 20 {
		t.Fatalf("unexpected grouped result: %+v", got)
	}
	if client.rowsCalls.Load() != 1 || client.legacyHits.Load() == 0 {
		t.Fatalf("rows calls = %d, legacy hits = %d", client.rowsCalls.Load(), client.legacyHits.Load())
	}
}

func TestSearchAnnouncementRegistry_RowsOtherErrorsDoNotFallBack(t *testing.T) {
	cases := []struct {
		upstream, want int
	}{
		{http.StatusBadRequest, http.StatusBadRequest},
		{http.StatusForbidden, http.StatusForbidden},
		{http.StatusInternalServerError, http.StatusInternalServerError},
		{http.StatusServiceUnavailable, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.upstream), func(t *testing.T) {
			client := &rowsRegistryClient{rowsFn: failingRows(tc.upstream)}
			client.searchCasesFn = func(context.Context, []byte) ([]byte, error) {
				t.Error("legacy path used after a non-404 error")
				return nil, nil
			}
			code, body := runRegistry(t, NewAnnouncementRegistryHandler(client), `{"pagination":{"limit":20}}`)
			if code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", code, tc.want, body)
			}
			if strings.Contains(string(body), "upstream secret detail") {
				t.Fatalf("upstream detail leaked to the caller: %s", body)
			}
			if client.legacyHits.Load() != 0 {
				t.Fatal("legacy path was hit")
			}
		})
	}
}

func TestSearchAnnouncementRegistry_RowsCancelledContextDoesNotFallBack(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &rowsRegistryClient{rowsFn: func(context.Context, []byte) ([]byte, error) {
		cancel()
		return nil, &apierror.Error{StatusCode: http.StatusNotFound}
	}}
	client.searchCasesFn = func(context.Context, []byte) ([]byte, error) {
		t.Error("legacy path used after the request was cancelled")
		return nil, nil
	}
	r := httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"pagination":{"limit":20}}`)).WithContext(ctx)
	r = withUser(r)
	w := httptest.NewRecorder()
	NewAnnouncementRegistryHandler(client).SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusNotFound)
	if client.legacyHits.Load() != 0 {
		t.Fatal("legacy path was hit")
	}
}

func TestSearchAnnouncementRegistry_RowsUndecodableResponseIsAnError(t *testing.T) {
	client := &rowsRegistryClient{rowsFn: staticRows(`not json`)}
	code, _ := runRegistry(t, NewAnnouncementRegistryHandler(client), `{"pagination":{"limit":20}}`)
	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
	if client.legacyHits.Load() != 0 {
		t.Fatal("legacy path used for an undecodable body")
	}
}

func TestSearchAnnouncementRegistry_RowsRequiresAuth(t *testing.T) {
	client := &rowsRegistryClient{rowsFn: staticRows(rowsPayload)}
	r := httptest.NewRequest(http.MethodPost, "/announcements/registry/search", strings.NewReader(`{"pagination":{"limit":20}}`))
	w := httptest.NewRecorder()
	NewAnnouncementRegistryHandler(client).SearchAnnouncementRegistry(w, r)
	assertStatus(t, w, http.StatusUnauthorized)
	if client.rowsCalls.Load() != 0 {
		t.Fatal("rows route called without a user")
	}
}
