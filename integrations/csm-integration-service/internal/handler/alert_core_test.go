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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const testRuleID = "11111111-1111-1111-1111-111111111111"

type mockAlertCoreClient struct {
	mutedIncidentsFn func(ctx context.Context, ruleID string, q url.Values) ([]byte, error)
	mutedCountsFn    func(ctx context.Context, q url.Values) ([]byte, error)
	fieldValuesFn    func(ctx context.Context, q url.Values) ([]byte, error)
}

func (m *mockAlertCoreClient) MutedIncidents(ctx context.Context, ruleID string, q url.Values) ([]byte, error) {
	return m.mutedIncidentsFn(ctx, ruleID, q)
}
func (m *mockAlertCoreClient) MutedCounts(ctx context.Context, q url.Values) ([]byte, error) {
	return m.mutedCountsFn(ctx, q)
}
func (m *mockAlertCoreClient) FieldValues(ctx context.Context, q url.Values) ([]byte, error) {
	return m.fieldValuesFn(ctx, q)
}

func serveAlertCore(h *AlertCoreHandler, target string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /alert-mute-rules/muted-counts", h.MutedCounts)
	mux.HandleFunc("GET /alert-mute-rules/{id}/muted-incidents", h.MutedIncidents)
	mux.HandleFunc("GET /alert-field-values", h.FieldValues)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

func TestAlertCore_NotConfiguredAnswers503(t *testing.T) {
	h := NewAlertCoreHandler(nil)
	for target, msg := range map[string]string{
		"/alert-mute-rules/" + testRuleID + "/muted-incidents": errMsgMutedAlertsNotConfigured,
		"/alert-mute-rules/muted-counts?ids=" + testRuleID:      errMsgMutedAlertsNotConfigured,
		"/alert-field-values?field=service":                     errMsgFieldValuesNotConfigured,
	} {
		w := serveAlertCore(h, target)
		assertStatus(t, w, http.StatusServiceUnavailable)
		assertErrorMessage(t, w, msg)
	}
}

func TestAlertCore_MutedIncidentsPassesThroughOnlyPagingParams(t *testing.T) {
	upstream := `{"items":[],"total":0,"limit":5,"offset":10,"summary":{}}`
	var gotID string
	var gotQ url.Values
	h := NewAlertCoreHandler(&mockAlertCoreClient{mutedIncidentsFn: func(_ context.Context, id string, q url.Values) ([]byte, error) {
		gotID, gotQ = id, q
		return []byte(upstream), nil
	}})
	w := serveAlertCore(h, "/alert-mute-rules/"+testRuleID+"/muted-incidents?limit=5&offset=10&evil=1")
	assertStatus(t, w, http.StatusOK)
	assertContentType(t, w, "application/json")
	if w.Body.String() != upstream {
		t.Errorf("body = %q, want upstream unchanged", w.Body.String())
	}
	if gotID != testRuleID || gotQ.Encode() != "limit=5&offset=10" {
		t.Errorf("forwarded id %q query %q, want %q and only limit/offset", gotID, gotQ.Encode(), testRuleID)
	}
}

func TestAlertCore_MutedIncidentsRejectsANonUUID(t *testing.T) {
	h := NewAlertCoreHandler(&mockAlertCoreClient{mutedIncidentsFn: func(context.Context, string, url.Values) ([]byte, error) {
		t.Fatal("alert-core called for an invalid id")
		return nil, nil
	}})
	w := serveAlertCore(h, "/alert-mute-rules/not-a-uuid/muted-incidents")
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorMessage(t, w, ErrMsgInvalidUUID)
}

func TestAlertCore_MutedCountsValidatesIDs(t *testing.T) {
	called := false
	h := NewAlertCoreHandler(&mockAlertCoreClient{mutedCountsFn: func(_ context.Context, q url.Values) ([]byte, error) {
		called = true
		return []byte(`{"counts":{}}`), nil
	}})
	tooMany := strings.TrimSuffix(strings.Repeat(testRuleID+",", 101), ",")
	for target, want := range map[string]string{
		"/alert-mute-rules/muted-counts":                          errMsgIDsRequired,
		"/alert-mute-rules/muted-counts?ids=" + tooMany:           errMsgTooManyIDs,
		"/alert-mute-rules/muted-counts?ids=" + testRuleID + ",x": ErrMsgInvalidUUID,
	} {
		w := serveAlertCore(h, target)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, want)
	}
	if called {
		t.Fatal("alert-core called for invalid ids")
	}
	w := serveAlertCore(h, "/alert-mute-rules/muted-counts?ids="+testRuleID)
	assertStatus(t, w, http.StatusOK)
}

func TestAlertCore_FieldValuesPassesThroughOnlyItsParams(t *testing.T) {
	var gotQ url.Values
	h := NewAlertCoreHandler(&mockAlertCoreClient{fieldValuesFn: func(_ context.Context, q url.Values) ([]byte, error) {
		gotQ = q
		return []byte(`{"field":"service","values":["api"]}`), nil
	}})
	w := serveAlertCore(h, "/alert-field-values?field=service&q=ap&limit=10&other=x")
	assertStatus(t, w, http.StatusOK)
	if gotQ.Encode() != "field=service&limit=10&q=ap" {
		t.Errorf("forwarded query %q, want only field/q/limit", gotQ.Encode())
	}
}

func TestAlertCore_UpstreamErrorsAreMapped(t *testing.T) {
	for _, tc := range upstreamErrors("Failed to list muted alerts.") {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(tc.name, "entity-service message") {
				t.Skip("alert-core's 400 body is {\"error\":...}, not entity-service's {\"message\":...}")
			}
			h := NewAlertCoreHandler(&mockAlertCoreClient{mutedIncidentsFn: func(context.Context, string, url.Values) ([]byte, error) {
				return nil, tc.err
			}})
			w := serveAlertCore(h, "/alert-mute-rules/"+testRuleID+"/muted-incidents")
			assertStatus(t, w, tc.wantCode)
			assertErrorMessage(t, w, tc.wantMsg)
		})
	}
}
