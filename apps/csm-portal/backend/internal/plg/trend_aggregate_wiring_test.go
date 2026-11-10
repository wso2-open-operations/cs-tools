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

package plg

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	csmhandler "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/entityclient"
	plghandler "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/handler"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/service"
)

// fakeEntity stands in for entity-service: it records each call and answers
// with status and body.
type fakeEntity struct {
	mu     sync.Mutex
	calls  []string // "METHOD path body"
	status int
	body   string
}

func (f *fakeEntity) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path+" "+string(raw))
	status, body := f.status, f.body
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func (f *fakeEntity) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// trendAggregateMux registers PLG's real route table over the real handlers,
// services and entity client, pointed at entity, with an identity that lets
// the request through.
func trendAggregateMux(t *testing.T, entity *fakeEntity) *http.ServeMux {
	t.Helper()
	srv := httptest.NewServer(entity)
	t.Cleanup(srv.Close)
	client := entityclient.New(entityclient.Config{BaseURL: srv.URL})
	h := plghandler.NewHandlers(nil,
		service.NewOrganizationService(entityclient.OrganizationRepo{Client: client}),
		service.NewOrgPlatformService(entityclient.OrgPlatformRepo{Client: client}),
		nil, nil)

	mux := http.NewServeMux()
	guard := csmhandler.NewAccessGuard(testAccessConfig())
	route := func(pattern string, perm csmhandler.Permission, fn http.HandlerFunc) {
		mux.HandleFunc(pattern, guard.Require(perm, fn))
	}
	register(h, func(next http.Handler) http.Handler { return next }, route)
	return mux
}

// postAs sends body to path as a CS engineer and returns the response.
func postAs(t *testing.T, mux *http.ServeMux, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	user := &middleware.UserInfo{Email: "staff@example.com", UserID: "sub-1", Roles: []string{"test-cs-engineer"}}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req.WithContext(middleware.WithUserInfo(req.Context(), user)))
	return w
}

// sameJSON reports whether a and b decode to the same value.
func sameJSON(t *testing.T, a, b string) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal([]byte(a), &x); err != nil {
		t.Fatalf("decode %q: %v", a, err)
	}
	if err := json.Unmarshal([]byte(b), &y); err != nil {
		t.Fatalf("decode %q: %v", b, err)
	}
	return reflect.DeepEqual(x, y)
}

// decodeAs decodes s strictly into a T.
func decodeAs[T any](t *testing.T, s string) T {
	t.Helper()
	var v T
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("decode %s: %v", s, err)
	}
	return v
}

// TestTrendAndAggregateAreForwarded: each route sends the request to its
// entity-service endpoint and answers with what entity-service answered.
func TestTrendAndAggregateAreForwarded(t *testing.T) {
	for _, c := range []struct {
		path, upstream, body, answer string
		decoded                      func(string) any
	}{
		{"/plg/registrations/volume-trend", "/plg/registrations/volume-trend",
			`{"unacknowledgedOnly":true,"filters":{"productCodes":["IAM"]},"startDate":"__startOfYear:0__","endDate":"__today__","bucket":"month"}`,
			`{"days":74,"startDate":"2026-01-01","endDate":"2026-03-15","bucket":"month","series":[{"label":"Registered","points":[{"date":"2026-01-01","endDate":"2026-01-31","count":3}]}]}`,
			func(s string) any { return decodeAs[domain.RegistrationVolumeTrendRequest](t, s) }},
		{"/plg/organizations/aggregate", "/plg/organizations/aggregate",
			`{"filters":{"productCodes":["IAM"]},"groupBy":"owner","maxGroups":5}`,
			`{"groups":[{"key":"11111111-1111-1111-1111-111111111111","label":"Alice Owner","count":2},{"key":"unowned","label":"Unowned","count":1}],"othersCount":0,"totalRecords":3}`,
			func(s string) any { return decodeAs[domain.AggregateOrganizationsRequest](t, s) }},
		{"/plg/organizations/aggregate", "/plg/organizations/aggregate",
			`{"filters":{"productCodes":["IAM"],"countryNames":["Sri Lanka"]},"groupBy":"country","maxGroups":10}`,
			`{"groups":[{"key":"Sri Lanka","label":"Sri Lanka","count":4}],"othersCount":0,"totalRecords":4}`,
			func(s string) any { return decodeAs[domain.AggregateOrganizationsRequest](t, s) }},
		{"/plg/registrations/aggregate", "/plg/registrations/aggregate",
			`{"unacknowledgedOnly":true,"filters":{"productCodes":["IAM"],"countryUnknown":true},"groupBy":"country","maxGroups":10}`,
			`{"groups":[{"key":"unknown","label":"Unknown","count":2}],"othersCount":0,"totalRecords":2}`,
			func(s string) any { return decodeAs[domain.AggregateRegistrationsRequest](t, s) }},
		{"/plg/registrations/aggregate", "/plg/registrations/aggregate",
			`{"filters":{"lifecycleStages":["REGISTRATION"],"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME"},"groupBy":"playbookAnswer","maxGroups":10}`,
			`{"groups":[{"key":"unanswered","label":"Not answered","count":2},{"key":"MONITOR","label":"Monitor","count":1}],"othersCount":0,"totalRecords":3}`,
			func(s string) any { return decodeAs[domain.AggregateRegistrationsRequest](t, s) }},
		{"/plg/organizations/aggregate", "/plg/organizations/aggregate",
			`{"filters":{"playbookTask":"REGISTERED_EMAIL_CLASSIFICATION","playbookAnswers":["DISPOSABLE"]},"groupBy":"country"}`,
			`{"groups":[{"key":"India","label":"India","count":1}],"othersCount":0,"totalRecords":1}`,
			func(s string) any { return decodeAs[domain.AggregateOrganizationsRequest](t, s) }},
		{"/plg/organizations/aggregate", "/plg/organizations/aggregate",
			`{"filters":{"ownerEmails":["bob@wso2.com","alice@wso2.com"]},"groupBy":"owner"}`,
			`{"groups":[{"key":"11111111-1111-1111-1111-111111111111","label":"Bob Owner","count":2}],"othersCount":0,"totalRecords":2}`,
			func(s string) any { return decodeAs[domain.AggregateOrganizationsRequest](t, s) }},
		{"/plg/registrations/volume-trend", "/plg/registrations/volume-trend",
			`{"filters":{"ownerEmails":["bob@wso2.com"]},"days":30}`,
			`{"days":30,"series":[{"label":"Registered","points":[]}]}`,
			func(s string) any { return decodeAs[domain.RegistrationVolumeTrendRequest](t, s) }},
		{"/plg/registrations/volume-trend", "/plg/registrations/volume-trend",
			`{"filters":{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME","playbookUnanswered":true},"startDate":"2026-01-01","endDate":"2026-03-31","bucket":"month"}`,
			`{"days":90,"startDate":"2026-01-01","endDate":"2026-03-31","bucket":"month","series":[{"label":"Registered","points":[]}]}`,
			func(s string) any { return decodeAs[domain.RegistrationVolumeTrendRequest](t, s) }},
	} {
		entity := &fakeEntity{status: http.StatusOK, body: c.answer}
		w := postAs(t, trendAggregateMux(t, entity), c.path, c.body)
		if w.Code != http.StatusOK || !sameJSON(t, w.Body.String(), c.answer) {
			t.Errorf("%s: %d %s, want 200 and entity-service's answer", c.path, w.Code, w.Body.String())
		}
		calls := entity.recorded()
		if len(calls) != 1 {
			t.Fatalf("%s: entity-service calls = %q, want one", c.path, calls)
		}
		method, rest, _ := strings.Cut(calls[0], " ")
		path, sent, _ := strings.Cut(rest, " ")
		if method != http.MethodPost || path != c.upstream {
			t.Errorf("%s: called %s %s, want POST %s", c.path, method, path, c.upstream)
		}
		if !reflect.DeepEqual(c.decoded(sent), c.decoded(c.body)) {
			t.Errorf("%s: sent %s, want %s", c.path, sent, c.body)
		}
	}
}

// TestTrendWithNoBodyIsTheDefaultWindow: an empty body is every registration
// over entity-service's default window.
func TestTrendWithNoBodyIsTheDefaultWindow(t *testing.T) {
	entity := &fakeEntity{status: http.StatusOK, body: `{"days":30,"series":[{"label":"Registered","points":[]}]}`}
	if w := postAs(t, trendAggregateMux(t, entity), "/plg/registrations/volume-trend", ""); w.Code != http.StatusOK {
		t.Errorf("status = %d %s, want 200", w.Code, w.Body.String())
	}
	if calls := entity.recorded(); len(calls) != 1 {
		t.Errorf("entity-service calls = %q, want one", calls)
	}
}

// TestTrendAndAggregateRefuseABadRequestHere: an unknown filter value or field,
// and an aggregate with no body, are 400 without an entity-service call.
func TestTrendAndAggregateRefuseABadRequestHere(t *testing.T) {
	for name, c := range map[string]struct{ path, body string }{
		"trend, unknown stage":                  {"/plg/registrations/volume-trend", `{"filters":{"lifecycleStages":["WON"]}}`},
		"trend, unknown field":                  {"/plg/registrations/volume-trend", `{"months":3}`},
		"aggregate, unknown tier":               {"/plg/organizations/aggregate", `{"filters":{"subscriptionTiers":["GOLD"]},"groupBy":"owner"}`},
		"aggregate, unknown field":              {"/plg/organizations/aggregate", `{"groupBy":"owner","limit":5}`},
		"aggregate, no body":                    {"/plg/organizations/aggregate", ""},
		"aggregate, dates backwards":            {"/plg/organizations/aggregate", `{"filters":{"registeredFrom":"2026-02-01T00:00:00Z","registeredTo":"2026-01-01T00:00:00Z"},"groupBy":"owner"}`},
		"registration aggregate, unknown stage": {"/plg/registrations/aggregate", `{"filters":{"lifecycleStages":["WON"]},"groupBy":"country"}`},
		"registration aggregate, unknown field": {"/plg/registrations/aggregate", `{"groupBy":"country","countries":["India"]}`},
		"registration aggregate, no body":       {"/plg/registrations/aggregate", ""},
	} {
		entity := &fakeEntity{status: http.StatusOK, body: `{}`}
		if w := postAs(t, trendAggregateMux(t, entity), c.path, c.body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d %s, want 400", name, w.Code, w.Body.String())
		}
		if calls := entity.recorded(); len(calls) != 0 {
			t.Errorf("%s: entity-service was called: %q", name, calls)
		}
	}
}

// TestEntityServiceRefusalsComeBackAsTheirMessage: what entity-service checks
// (the window, the grouping, maxGroups) comes back as a 400 with its message.
func TestEntityServiceRefusalsComeBackAsTheirMessage(t *testing.T) {
	for _, c := range []struct{ path, body, message string }{
		{"/plg/registrations/volume-trend", `{"bucket":"month"}`, "bucket needs startDate and endDate"},
		{"/plg/organizations/aggregate", `{"groupBy":"tier"}`, `groupBy must be "owner" or "country", got "tier"`},
		{"/plg/registrations/aggregate", `{"groupBy":"owner"}`, `groupBy must be "country", got "owner"`},
		{"/plg/organizations/search", `{"filters":{"countryNames":["India"],"countryUnknown":true}}`, "filters.countryNames and filters.countryUnknown cannot be combined"},
		{"/plg/registrations/aggregate", `{"groupBy":"playbookAnswer"}`, "groupBy playbookAnswer needs filters.playbookTask"},
		{"/plg/registrations/search", `{"filters":{"playbookAnswers":["MONITOR"]}}`, "filters.playbookAnswers and filters.playbookUnanswered need filters.playbookTask"},
		{"/plg/organizations/search", `{"filters":{"ownerEmails":["Bob"]}}`, "filters.ownerEmails entries must be email addresses of at most 254 characters"},
		{"/plg/registrations/aggregate", `{"filters":{"ownerEmails":["bob@wso2.com"],"unowned":true},"groupBy":"country"}`, "filters.ownerEmails and filters.unowned cannot be combined"},
	} {
		answer, _ := json.Marshal(map[string]any{"code": 400, "message": c.message})
		entity := &fakeEntity{status: http.StatusBadRequest, body: string(answer)}
		w := postAs(t, trendAggregateMux(t, entity), c.path, c.body)
		var got struct{ Message string }
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if w.Code != http.StatusBadRequest || got.Message != c.message {
			t.Errorf("%s: %d %s, want 400 with %q", c.path, w.Code, w.Body.String(), c.message)
		}
	}
}
