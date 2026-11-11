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

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

const mutedRuleID = "01a1c41d-6165-7832-ac75-1768a6257fae"

type fakeAlertCore struct {
	body              []byte
	err               error
	gotID             string
	gotLimit, gotOffs int
	gotField, gotQ    string
	gotIDs            []string
}

func (f *fakeAlertCore) MutedCounts(_ context.Context, ids []string) ([]byte, error) {
	f.gotIDs = ids
	return f.body, f.err
}

func (f *fakeAlertCore) FieldValues(_ context.Context, field, q string, limit int) ([]byte, error) {
	f.gotField, f.gotQ, f.gotLimit = field, q, limit
	return f.body, f.err
}

func (f *fakeAlertCore) MutedIncidents(_ context.Context, id string, limit, offset int) ([]byte, error) {
	f.gotID, f.gotLimit, f.gotOffs = id, limit, offset
	return f.body, f.err
}

func callMutedAlerts(h *AlertMuteRuleMutedAlertsHandler, target string, user bool) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /alert-mute-rules/{id}/muted-alerts", h.ListMutedAlerts)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if user {
		req = withUser(req)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestListMutedAlertsForwardsPage(t *testing.T) {
	core := &fakeAlertCore{body: []byte(`{"items":[],"total":0}`)}
	w := callMutedAlerts(NewAlertMuteRuleMutedAlertsHandler(core), "/alert-mute-rules/"+mutedRuleID+"/muted-alerts?limit=50&offset=50", true)
	assertStatus(t, w, http.StatusOK)
	if w.Body.String() != `{"items":[],"total":0}` {
		t.Errorf("body = %s, want alert-core's body unchanged", w.Body)
	}
	if core.gotID != mutedRuleID || core.gotLimit != 50 || core.gotOffs != 50 {
		t.Errorf("forwarded id=%s limit=%d offset=%d", core.gotID, core.gotLimit, core.gotOffs)
	}
}

func TestListMutedAlertsDefaults(t *testing.T) {
	core := &fakeAlertCore{body: []byte(`{}`)}
	callMutedAlerts(NewAlertMuteRuleMutedAlertsHandler(core), "/alert-mute-rules/"+mutedRuleID+"/muted-alerts", true)
	if core.gotLimit != 20 || core.gotOffs != 0 {
		t.Errorf("limit=%d offset=%d, want 20 and 0", core.gotLimit, core.gotOffs)
	}
}

func TestListMutedAlertsGuards(t *testing.T) {
	h := NewAlertMuteRuleMutedAlertsHandler(&fakeAlertCore{})
	assertStatus(t, callMutedAlerts(h, "/alert-mute-rules/"+mutedRuleID+"/muted-alerts", false), http.StatusUnauthorized)
	assertStatus(t, callMutedAlerts(h, "/alert-mute-rules/nope/muted-alerts", true), http.StatusBadRequest)
	for _, q := range []string{"limit=0", "limit=101", "offset=-1", "limit=x"} {
		assertStatus(t, callMutedAlerts(h, "/alert-mute-rules/"+mutedRuleID+"/muted-alerts?"+q, true), http.StatusBadRequest)
	}
}

func TestListMutedAlertsNotConfigured(t *testing.T) {
	w := callMutedAlerts(NewAlertMuteRuleMutedAlertsHandler(nil), "/alert-mute-rules/"+mutedRuleID+"/muted-alerts", true)
	assertStatus(t, w, http.StatusServiceUnavailable)
	assertErrorMessage(t, w, errMsgMutedAlertsNotConfigured)
}

func TestListMutedAlertsUpstreamErrors(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{&apierror.Error{StatusCode: http.StatusUnauthorized}, http.StatusServiceUnavailable}, // our token, not the engineer's session
		{&apierror.Error{StatusCode: http.StatusInternalServerError}, http.StatusServiceUnavailable},
		{&apierror.Error{StatusCode: http.StatusBadRequest}, http.StatusBadRequest},
		{errors.New("connection refused"), http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		w := callMutedAlerts(NewAlertMuteRuleMutedAlertsHandler(&fakeAlertCore{err: c.err}), "/alert-mute-rules/"+mutedRuleID+"/muted-alerts", true)
		assertStatus(t, w, c.want)
	}
}

func callFieldValues(h *AlertMuteRuleMutedAlertsHandler, target string, user bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if user {
		req = withUser(req)
	}
	w := httptest.NewRecorder()
	h.ListAlertFieldValues(w, req)
	return w
}

func TestListAlertFieldValuesForwards(t *testing.T) {
	core := &fakeAlertCore{body: []byte(`{"field":"service","values":["client-prod"]}`)}
	w := callFieldValues(NewAlertMuteRuleMutedAlertsHandler(core), "/alert-field-values?field=service&q=client&limit=10", true)
	assertStatus(t, w, http.StatusOK)
	if w.Body.String() != `{"field":"service","values":["client-prod"]}` || core.gotField != "service" || core.gotQ != "client" || core.gotLimit != 10 {
		t.Errorf("body=%s field=%s q=%s limit=%d", w.Body, core.gotField, core.gotQ, core.gotLimit)
	}
}

func TestListAlertFieldValuesGuards(t *testing.T) {
	h := NewAlertMuteRuleMutedAlertsHandler(&fakeAlertCore{})
	assertStatus(t, callFieldValues(h, "/alert-field-values?field=service", false), http.StatusUnauthorized)
	for _, q := range []string{"field=project", "field=", "field=service&limit=0", "field=service&limit=101"} {
		assertStatus(t, callFieldValues(h, "/alert-field-values?"+q, true), http.StatusBadRequest)
	}
	assertStatus(t, callFieldValues(NewAlertMuteRuleMutedAlertsHandler(nil), "/alert-field-values?field=service", true), http.StatusServiceUnavailable)
	failing := NewAlertMuteRuleMutedAlertsHandler(&fakeAlertCore{err: &apierror.Error{StatusCode: http.StatusUnauthorized}})
	assertStatus(t, callFieldValues(failing, "/alert-field-values?field=service", true), http.StatusServiceUnavailable)
}

func callMutedCounts(h *AlertMuteRuleMutedAlertsHandler, target string, user bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if user {
		req = withUser(req)
	}
	w := httptest.NewRecorder()
	h.ListMutedCounts(w, req)
	return w
}

func TestListMutedCounts(t *testing.T) {
	core := &fakeAlertCore{body: []byte(`{"counts":{}}`)}
	h := NewAlertMuteRuleMutedAlertsHandler(core)
	w := callMutedCounts(h, "/alert-mute-rules/muted-counts?ids="+mutedRuleID+","+mutedRuleID, true)
	assertStatus(t, w, http.StatusOK)
	if len(core.gotIDs) != 1 || core.gotIDs[0] != mutedRuleID {
		t.Errorf("forwarded ids = %v, want the duplicate dropped", core.gotIDs)
	}
	assertStatus(t, callMutedCounts(h, "/alert-mute-rules/muted-counts?ids="+mutedRuleID, false), http.StatusUnauthorized)
	for _, q := range []string{"", "ids=", "ids=nope", "ids=" + mutedRuleID + ",x"} {
		assertStatus(t, callMutedCounts(h, "/alert-mute-rules/muted-counts?"+q, true), http.StatusBadRequest)
	}
	assertStatus(t, callMutedCounts(NewAlertMuteRuleMutedAlertsHandler(nil), "/alert-mute-rules/muted-counts?ids="+mutedRuleID, true), http.StatusServiceUnavailable)
}
