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

// Incident lifecycle through the real entity-service routes: the
// IncidentHandler wired to the real ServiceNow-backed IncidentService
// (DATA_SOURCE=servicenow, the only data source on which an incident's state
// can be transitioned -- incidentService.UpdateIncident rejects `state` on
// both Postgres data sources), talking to a stateful in-memory ServiceNow
// stand-in that applies each POST/PATCH and serves the result back on GET.
//
// The stand-in is not ServiceNow: it proves what this service SENDS at every
// step (and how it maps what comes back), not ServiceNow's own business
// rules.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

const (
	lifecycleIncidentSysid = "7a000000000000000000000000000001"
	lifecycleIncidentID    = "7a000000-0000-0000-0000-000000000001"
	lifecycleCallerID      = "7a000000-0000-0000-0000-0000000000c1"
	lifecycleServiceID     = "7a000000-0000-0000-0000-000000000051"
)

// fakeSNIncidentStore is a single-incident ServiceNow stand-in. Field values
// are stored as the ServiceNow keys this service sends (categoryKey
// "service_interruption", stateKey 2, ...) and echoed back in the Choreo GET
// shape ({"id": <key>, "label": ...}), so the real response mapping runs on
// every read.
type fakeSNIncidentStore struct {
	t  *testing.T
	mu sync.Mutex

	created         bool
	subject         string
	categoryKey     string
	subcategoryKey  *string
	contactTypeKey  *string
	impactKey       int
	urgencyKey      int
	stateKey        int
	resolutionCode  *string
	resolutionNotes *string

	createBody  map[string]any
	patchBodies []map[string]any
}

func (f *fakeSNIncidentStore) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 3600})
	})
	// The support-group lookup DATA_SOURCE=servicenow makes before every
	// create. No services: these incidents are created unassigned.
	mux.HandleFunc("POST /services/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"services":[],"totalRecords":0}`))
	})
	mux.HandleFunc("POST /incidents", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			f.t.Errorf("fake SN: decode create body: %v", err)
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		f.createBody = body
		f.created = true
		f.stateKey = 1 // ServiceNow's own default for a new incident: New.
		f.subject, _ = body["subject"].(string)
		f.categoryKey, _ = body["categoryKey"].(string)
		f.subcategoryKey = optString(body, "subcategoryKey")
		f.contactTypeKey = optString(body, "contactTypeKey")
		f.impactKey = int(body["impactKey"].(float64))
		f.urgencyKey = int(body["urgencyKey"].(float64))
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"message":"Incident created successfully.","incident":{"id":%q,"number":"INC0090001","createdOn":"2026-10-05 00:00:00","createdBy":"engineer@example.com"}}`, lifecycleIncidentSysid)
	})
	mux.HandleFunc("PATCH /incidents/"+lifecycleIncidentSysid, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			f.t.Errorf("fake SN: decode patch body: %v", err)
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		f.patchBodies = append(f.patchBodies, body)
		if v, ok := body["stateKey"].(float64); ok {
			f.stateKey = int(v)
		}
		if v := optString(body, "subcategoryKey"); v != nil {
			f.subcategoryKey = v
		}
		if v := optString(body, "contactTypeKey"); v != nil {
			f.contactTypeKey = v
		}
		if v := optString(body, "resolutionCodeKey"); v != nil {
			f.resolutionCode = v
		}
		if v := optString(body, "resolutionNotes"); v != nil {
			f.resolutionNotes = v
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"message":"Incident updated successfully.","incident":%s}`, f.renderLocked())
	})
	mux.HandleFunc("GET /incidents/"+lifecycleIncidentSysid, func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if !f.created {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(f.renderLocked()))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.t.Errorf("fake SN: unexpected request %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	return mux
}

// renderLocked renders the stored incident in the Choreo GET /incidents/{id}
// shape. Caller holds f.mu.
func (f *fakeSNIncidentStore) renderLocked() string {
	strLabel := func(v *string) any {
		if v == nil {
			return nil
		}
		return map[string]string{"id": *v, "label": *v}
	}
	category := f.categoryKey
	out := map[string]any{
		"id":              lifecycleIncidentSysid,
		"number":          "INC0090001",
		"subject":         f.subject,
		"state":           map[string]any{"id": f.stateKey, "label": fmt.Sprint(f.stateKey)},
		"category":        strLabel(&category),
		"subcategory":     strLabel(f.subcategoryKey),
		"contactType":     strLabel(f.contactTypeKey),
		"impact":          map[string]any{"id": f.impactKey, "label": fmt.Sprint(f.impactKey)},
		"urgency":         map[string]any{"id": f.urgencyKey, "label": fmt.Sprint(f.urgencyKey)},
		"resolutionCode":  strLabel(f.resolutionCode),
		"resolutionNotes": f.resolutionNotes,
		"createdOn":       "2026-10-05 00:00:00",
		"createdBy":       "engineer@example.com",
		"updatedOn":       "2026-10-05 00:00:00",
		"updatedBy":       "engineer@example.com",
	}
	b, err := json.Marshal(out)
	if err != nil {
		f.t.Fatalf("fake SN: marshal incident: %v", err)
	}
	return string(b)
}

func optString(body map[string]any, key string) *string {
	if v, ok := body[key].(string); ok {
		return &v
	}
	return nil
}

// newIncidentLifecycleServer wires the real incident routes (same method +
// pattern strings as server/routes.go) to the real ServiceNow-backed
// IncidentService, behind the real x-user-id-token middleware.
func newIncidentLifecycleServer(t *testing.T) (http.Handler, *fakeSNIncidentStore) {
	t.Helper()
	store := &fakeSNIncidentStore{t: t}
	sn := httptest.NewServer(store.handler())
	t.Cleanup(sn.Close)
	client := integrationservice.New(sn.URL, integrationservice.ClientCredentialsConfig{
		TokenURL:     sn.URL + "/oauth2/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}, 45*time.Second)

	h := NewIncidentHandler(service.NewServiceNowIncidentService(client, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /incidents/{id}", h.GetIncident)
	mux.HandleFunc("PATCH /incidents/{id}", h.PatchIncident)
	mux.HandleFunc("POST /incidents", h.CreateIncident)
	return middleware.UserIDToken(mux), store
}

func doLifecycleRequest(t *testing.T, srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("x-user-id-token", "user-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	return w
}

func getLifecycleIncident(t *testing.T, srv http.Handler) domain.IncidentView {
	t.Helper()
	w := doLifecycleRequest(t, srv, http.MethodGet, "/incidents/"+lifecycleIncidentID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /incidents/{id}: status %d, body %s", w.Code, w.Body.String())
	}
	var view domain.IncidentView
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	return view
}

func strOrNil(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// TestIncidentLifecycle_WithoutSubcategory creates an incident with no
// subcategory (the create form no longer requires one) and walks it
// New -> In Progress -> Resolved -> Closed via PATCH /incidents/{id}, with
// the same request bodies EditIncidentDialog sends for each transition.
// After every step the incident is re-read via GET /incidents/{id}: state
// must be the new one, and the subcategory must still be absent, with
// category and the rest of the classification untouched.
func TestIncidentLifecycle_WithoutSubcategory(t *testing.T) {
	srv, store := newIncidentLifecycleServer(t)

	createBody := fmt.Sprintf(`{
		"subject": "Gateway returning 502s",
		"callerId": %q,
		"serviceId": %q,
		"category": "SERVICE_INTERRUPTION",
		"contactType": "EMAIL",
		"impact": "HIGH",
		"urgency": "LOW"
	}`, lifecycleCallerID, lifecycleServiceID)
	w := doLifecycleRequest(t, srv, http.MethodPost, "/incidents", createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /incidents without subcategory: status %d, body %s", w.Code, w.Body.String())
	}
	var created domain.CreateIncidentResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Incident.ID != lifecycleIncidentID {
		t.Fatalf("created id = %q, want %q", created.Incident.ID, lifecycleIncidentID)
	}
	if _, sent := store.createBody["subcategoryKey"]; sent {
		t.Errorf("create sent subcategoryKey %v to ServiceNow, want it omitted", store.createBody["subcategoryKey"])
	}

	assertStage := func(t *testing.T, stage string, wantState domain.IncidentState) {
		t.Helper()
		view := getLifecycleIncident(t, srv)
		if strOrNil(view.State) != string(wantState) {
			t.Errorf("%s: state = %s, want %s", stage, strOrNil(view.State), wantState)
		}
		if view.Subcategory != nil {
			t.Errorf("%s: subcategory = %s, want none", stage, *view.Subcategory)
		}
		if strOrNil(view.Category) != "SERVICE_INTERRUPTION" {
			t.Errorf("%s: category = %s, want SERVICE_INTERRUPTION", stage, strOrNil(view.Category))
		}
		if strOrNil(view.ContactType) != "EMAIL" {
			t.Errorf("%s: contactType = %s, want EMAIL", stage, strOrNil(view.ContactType))
		}
	}

	assertStage(t, "after create", domain.IncidentStateNew)

	steps := []struct {
		name      string
		body      string
		wantState domain.IncidentState
	}{
		{"start work", `{"state":"IN_PROGRESS"}`, domain.IncidentStateInProgress},
		// Resolving and closing both carry the resolution fields:
		// EditIncidentDialog blocks either transition without them.
		{"resolve", `{"state":"RESOLVED","resolutionCode":"SOLVED_PERMANENTLY","resolutionNotes":"Rolled back the bad gateway config."}`, domain.IncidentStateResolved},
		{"close", `{"state":"CLOSED","resolutionCode":"SOLVED_PERMANENTLY","resolutionNotes":"Rolled back the bad gateway config."}`, domain.IncidentStateClosed},
	}
	for _, step := range steps {
		w := doLifecycleRequest(t, srv, http.MethodPatch, "/incidents/"+lifecycleIncidentID, step.body)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: PATCH %s: status %d, body %s", step.name, step.body, w.Code, w.Body.String())
		}
		var updated domain.UpdateIncidentResponse
		if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
			t.Fatalf("%s: decode PATCH response: %v", step.name, err)
		}
		if strOrNil(updated.Incident.State) != string(step.wantState) {
			t.Errorf("%s: PATCH response state = %s, want %s", step.name, strOrNil(updated.Incident.State), step.wantState)
		}
		if updated.Incident.Subcategory != nil {
			t.Errorf("%s: PATCH response subcategory = %s, want none", step.name, *updated.Incident.Subcategory)
		}
		assertStage(t, "after "+step.name, step.wantState)
	}

	view := getLifecycleIncident(t, srv)
	if strOrNil(view.ResolutionCode) != "Solved (Permanently)" || strOrNil(view.ResolutionNotes) != "Rolled back the bad gateway config." {
		t.Errorf("closed incident resolution = %s / %s", strOrNil(view.ResolutionCode), strOrNil(view.ResolutionNotes))
	}

	// No transition may have touched the classification as a side effect.
	if len(store.patchBodies) != len(steps) {
		t.Fatalf("ServiceNow received %d PATCHes, want %d", len(store.patchBodies), len(steps))
	}
	for i, body := range store.patchBodies {
		for _, key := range []string{"subcategoryKey", "categoryKey", "contactTypeKey"} {
			if _, sent := body[key]; sent {
				t.Errorf("%s: PATCH sent %s=%v, want it omitted", steps[i].name, key, body[key])
			}
		}
	}
}

// TestIncidentLifecycle_ChannelSurvives: the UI's "Channel" is the
// incident's contactType. Created with SITE_247 -- the value whose
// ServiceNow key ("2") is furthest from its own name, so a mapping slip in
// either direction shows -- it must read back as SITE_247 after create and
// after every transition, and no transition may send contactTypeKey. Start
// work uses the detail page's own action-bar body (state plus claiming the
// incident), not EditIncidentDialog's bare state change.
func TestIncidentLifecycle_ChannelSurvives(t *testing.T) {
	srv, store := newIncidentLifecycleServer(t)
	const engineerID = "7a000000-0000-0000-0000-0000000000e1"

	createBody := fmt.Sprintf(`{
		"subject": "Site 24/7 monitor: gateway down",
		"callerId": %q,
		"serviceId": %q,
		"category": "SERVICE_INTERRUPTION",
		"contactType": "SITE_247",
		"impact": "HIGH",
		"urgency": "HIGH"
	}`, lifecycleCallerID, lifecycleServiceID)
	w := doLifecycleRequest(t, srv, http.MethodPost, "/incidents", createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /incidents: status %d, body %s", w.Code, w.Body.String())
	}
	if got, _ := store.createBody["contactTypeKey"].(string); got != "2" {
		t.Fatalf("create sent contactTypeKey %q to ServiceNow, want \"2\" (Site 24/7)", got)
	}

	assertChannel := func(t *testing.T, stage string, wantState domain.IncidentState) {
		t.Helper()
		view := getLifecycleIncident(t, srv)
		if strOrNil(view.State) != string(wantState) {
			t.Errorf("%s: state = %s, want %s", stage, strOrNil(view.State), wantState)
		}
		if strOrNil(view.ContactType) != "SITE_247" {
			t.Errorf("%s: channel (contactType) = %s, want SITE_247", stage, strOrNil(view.ContactType))
		}
	}
	assertChannel(t, "after create", domain.IncidentStateNew)

	steps := []struct {
		name      string
		body      string
		wantState domain.IncidentState
	}{
		{"start work", fmt.Sprintf(`{"state":"IN_PROGRESS","assignedEngineerId":%q}`, engineerID), domain.IncidentStateInProgress},
		{"resolve", `{"state":"RESOLVED","resolutionCode":"SOLVED_PERMANENTLY","resolutionNotes":"Monitor recovered after failover."}`, domain.IncidentStateResolved},
		{"close", `{"state":"CLOSED","resolutionCode":"SOLVED_PERMANENTLY","resolutionNotes":"Monitor recovered after failover."}`, domain.IncidentStateClosed},
	}
	for _, step := range steps {
		w := doLifecycleRequest(t, srv, http.MethodPatch, "/incidents/"+lifecycleIncidentID, step.body)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: PATCH %s: status %d, body %s", step.name, step.body, w.Code, w.Body.String())
		}
		var updated domain.UpdateIncidentResponse
		if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
			t.Fatalf("%s: decode PATCH response: %v", step.name, err)
		}
		if strOrNil(updated.Incident.ContactType) != "SITE_247" {
			t.Errorf("%s: PATCH response channel = %s, want SITE_247", step.name, strOrNil(updated.Incident.ContactType))
		}
		assertChannel(t, "after "+step.name, step.wantState)
	}

	for i, body := range store.patchBodies {
		if _, sent := body["contactTypeKey"]; sent {
			t.Errorf("%s: PATCH sent contactTypeKey=%v, want it omitted", steps[i].name, body["contactTypeKey"])
		}
	}
	if got, _ := store.patchBodies[0]["assignedEngineerId"].(string); got != strings.ReplaceAll(engineerID, "-", "") {
		t.Errorf("start work sent assignedEngineerId %q, want the engineer's sys_id", got)
	}
}
