// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"alert-core-service/internal/model"
)

const ruleID = "01a1c41d-6165-7832-ac75-1768a6257fae"

type fakeStore struct {
	incs              []model.Incident
	err               error
	gotLimit, gotOffs int
	values            []string
	gotField, gotQ    string
}

func (f *fakeStore) MutedCounts(_ context.Context, ids []string) (map[string]int, error) {
	counts := map[string]int{}
	for _, inc := range f.incs {
		for _, id := range ids {
			if inc.MuteRuleID == id {
				counts[id]++
			}
		}
	}
	return counts, f.err
}

func (f *fakeStore) FieldValues(_ context.Context, field, q string, limit int) ([]string, bool, error) {
	f.gotField, f.gotQ, f.gotLimit = field, q, limit
	if field != "service" {
		return nil, false, nil
	}
	return f.values, true, f.err
}

func (f *fakeStore) MutedByRule(_ context.Context, id string, limit, offset int) ([]model.Incident, int, error) {
	f.gotLimit, f.gotOffs = limit, offset
	if f.err != nil {
		return nil, 0, f.err
	}
	var out []model.Incident
	for _, inc := range f.incs {
		if inc.MuteRuleID == id {
			out = append(out, inc)
		}
	}
	return out, len(out), nil
}

func (f *fakeStore) MutedByRuleSummary(_ context.Context, id string) (map[string]int, error) {
	counts := map[string]int{}
	for _, inc := range f.incs {
		if inc.MuteRuleID == id {
			counts[inc.MuteOutcome()]++
		}
	}
	return counts, f.err
}

func serve(t *testing.T, store MutedStore, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	h := New(slog.New(slog.NewTextHandler(io.Discard, nil)), store)
	mux.HandleFunc("GET /alert-mute-rules/{id}/muted-incidents", h.MutedIncidents)
	mux.HandleFunc("GET /alert-field-values", h.FieldValues)
	mux.HandleFunc("GET /alert-mute-rules/muted-counts", h.MutedCounts)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

func TestMutedIncidentsListsOutcomes(t *testing.T) {
	now := time.Now()
	store := &fakeStore{incs: []model.Incident{
		{ID: 1, MuteRuleID: ruleID, Muted: true, Service: "svc", SeverityLabel: "critical", AlertCount: 3, IncidentNumber: "PENDING-x"},
		{ID: 2, MuteRuleID: ruleID, UnmutedAt: &now, CSMConfirmed: true, CSMAttempts: 1, IncidentNumber: "INC0000042"},
		{ID: 3, MuteRuleID: ruleID, UnmutedAt: &now, Status: "closed", IncidentNumber: "PENDING-y"},
		{ID: 4, MuteRuleID: ruleID, UnmutedAt: &now, CSMAttempts: 1, IncidentNumber: "PENDING-z"},
		{ID: 5, MuteRuleID: "other"},
	}}
	w := serve(t, store, "/alert-mute-rules/"+ruleID+"/muted-incidents?limit=10&offset=0")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body)
	}
	var resp MutedIncidentsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 4 || len(resp.Items) != 4 || resp.Limit != 10 {
		t.Fatalf("resp = %+v, want the rule's 4 incidents", resp)
	}
	want := []struct{ outcome, number string }{{"muted", ""}, {"incident", "INC0000042"}, {"closed_quietly", ""}, {"delivering", ""}}
	for i, wnt := range want {
		if resp.Items[i].Outcome != wnt.outcome || resp.Items[i].IncidentNumber != wnt.number {
			t.Errorf("item %d = %+v, want outcome %s number %q", i, resp.Items[i], wnt.outcome, wnt.number)
		}
	}
	if resp.Items[0].Severity != "critical" || resp.Items[0].AlertCount != 3 {
		t.Errorf("item 0 = %+v, want the raw severity label and alert count", resp.Items[0])
	}
	if resp.Summary["muted"] != 1 || resp.Summary["incident"] != 1 || resp.Summary["closed_quietly"] != 1 || resp.Summary["delivering"] != 1 {
		t.Errorf("summary = %v", resp.Summary)
	}
}

func TestMutedIncidentsDefaultsAndValidation(t *testing.T) {
	store := &fakeStore{}
	if w := serve(t, store, "/alert-mute-rules/"+ruleID+"/muted-incidents"); w.Code != http.StatusOK || store.gotLimit != defaultLimit || store.gotOffs != 0 {
		t.Fatalf("status = %d limit = %d offset = %d, want 200 with the defaults", w.Code, store.gotLimit, store.gotOffs)
	}
	for _, target := range []string{
		"/alert-mute-rules/not-a-uuid/muted-incidents",
		"/alert-mute-rules/" + ruleID + "/muted-incidents?limit=0",
		"/alert-mute-rules/" + ruleID + "/muted-incidents?limit=101",
		"/alert-mute-rules/" + ruleID + "/muted-incidents?offset=-1",
		"/alert-mute-rules/" + ruleID + "/muted-incidents?limit=x",
	} {
		if w := serve(t, store, target); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, w.Code)
		}
	}
}

func TestMutedIncidentsStoreError(t *testing.T) {
	w := serve(t, &fakeStore{err: errors.New("db down")}, "/alert-mute-rules/"+ruleID+"/muted-incidents")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestFieldValues(t *testing.T) {
	store := &fakeStore{values: []string{"client-prod-alert-integration", "Managed Services"}}
	w := serve(t, store, "/alert-field-values?field=service&q=client&limit=5")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body)
	}
	var got FieldValuesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Field != "service" || len(got.Values) != 2 || store.gotQ != "client" || store.gotLimit != 5 {
		t.Fatalf("got %+v, store saw q=%q limit=%d", got, store.gotQ, store.gotLimit)
	}

	empty := serve(t, &fakeStore{}, "/alert-field-values?field=service")
	if empty.Code != http.StatusOK || !json.Valid(empty.Body.Bytes()) || string(empty.Body.Bytes()) != `{"field":"service","values":[]}`+"\n" {
		t.Fatalf("empty: %d %s", empty.Code, empty.Body)
	}
}

func TestFieldValuesRejectsBadInput(t *testing.T) {
	for _, target := range []string{
		"/alert-field-values?field=project",
		"/alert-field-values?field=service&limit=0",
		"/alert-field-values?field=service&limit=101",
	} {
		if w := serve(t, &fakeStore{}, target); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, w.Code)
		}
	}
	if w := serve(t, &fakeStore{err: errors.New("db down")}, "/alert-field-values?field=service"); w.Code != http.StatusInternalServerError {
		t.Errorf("store error: status = %d, want 500", w.Code)
	}
}

func TestMutedCounts(t *testing.T) {
	const other = "11111111-2222-3333-4444-555555555555"
	store := &fakeStore{incs: []model.Incident{{ID: 1, MuteRuleID: ruleID}, {ID: 2, MuteRuleID: ruleID}}}
	w := serve(t, store, "/alert-mute-rules/muted-counts?ids="+ruleID+","+other)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body)
	}
	var got MutedCountsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Counts[ruleID] != 2 || got.Counts[other] != 0 || len(got.Counts) != 2 {
		t.Fatalf("counts = %v", got.Counts)
	}
	for _, target := range []string{"/alert-mute-rules/muted-counts", "/alert-mute-rules/muted-counts?ids=nope"} {
		if w := serve(t, store, target); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, w.Code)
		}
	}
}
