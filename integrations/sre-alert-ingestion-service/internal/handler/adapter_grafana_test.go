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
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func grafanaAlertJSON(state, service, severity string) []byte {
	return []byte(`{
		"state": "` + state + `",
		"title": "High memory usage",
		"tags": {
			"service": "` + service + `",
			"severity": "` + severity + `"
		}
	}`)
}

func TestMapGrafanaPayload_SeverityTable(t *testing.T) {
	cases := []struct {
		severity string
		want     string
	}{
		{"1", "critical"},
		{"2", "major"},
		{"3", "minor"},
		{"4", "warning"},
		{"5", "ok"},
		{"", "ok"},
	}
	for _, tc := range cases {
		t.Run(tc.severity, func(t *testing.T) {
			req, ok, err := mapGrafanaPayload(grafanaAlertJSON("alerting", "checkout", tc.severity))
			if err != nil {
				t.Fatalf("mapGrafanaPayload: %v", err)
			}
			if !ok {
				t.Fatal("ok = false, want true for state=alerting")
			}
			if req.Severity != tc.want {
				t.Errorf("Severity = %q, want %q", req.Severity, tc.want)
			}
		})
	}
}

func TestMapGrafanaPayload_NonAlertingStateIsFiltered(t *testing.T) {
	cases := []string{"resolved", "no_data", "", "OK"}
	for _, state := range cases {
		t.Run(state, func(t *testing.T) {
			_, ok, err := mapGrafanaPayload(grafanaAlertJSON(state, "checkout", "1"))
			if err != nil {
				t.Fatalf("mapGrafanaPayload: %v", err)
			}
			if ok {
				t.Errorf("ok = true for state %q, want false (only \"alerting\" is actionable)", state)
			}
		})
	}
}

func TestMapGrafanaPayload_ServicePassthrough(t *testing.T) {
	req, ok, err := mapGrafanaPayload(grafanaAlertJSON("alerting", "checkout-service", "1"))
	if err != nil {
		t.Fatalf("mapGrafanaPayload: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if req.Service != "checkout-service" {
		t.Errorf("Service = %q, want the tags.service value passed through as-is", req.Service)
	}
}

func TestMapGrafanaPayload_ServiceDefaultWhenEmpty(t *testing.T) {
	req, ok, err := mapGrafanaPayload(grafanaAlertJSON("alerting", "", "1"))
	if err != nil {
		t.Fatalf("mapGrafanaPayload: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if req.Service != grafanaDefaultService {
		t.Errorf("Service = %q, want default %q when tags.service is empty", req.Service, grafanaDefaultService)
	}
}

func TestMapGrafanaPayload_FieldMapping(t *testing.T) {
	req, ok, err := mapGrafanaPayload(grafanaAlertJSON("alerting", "checkout", "1"))
	if err != nil {
		t.Fatalf("mapGrafanaPayload: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if req.Source != "grafana" {
		t.Errorf("Source = %q, want %q", req.Source, "grafana")
	}
	if req.MetricName != "High memory usage" {
		t.Errorf("MetricName = %q, want the payload's title", req.MetricName)
	}
	if req.UniqueIdentifier != "" {
		t.Errorf("UniqueIdentifier = %q, want empty (no field available for grouping)", req.UniqueIdentifier)
	}
	if req.Description == "" {
		t.Error("Description is empty, want a rendered summary of the raw payload")
	}
}

func TestMapGrafanaPayload_MalformedJSON(t *testing.T) {
	_, _, err := mapGrafanaPayload([]byte(`not json`))
	if err == nil {
		t.Fatal("mapGrafanaPayload should error on malformed JSON")
	}
}

func TestCreateAlertFromGrafana_Success(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1")

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/grafana", bytes.NewReader(grafanaAlertJSON("alerting", "checkout", "1")))
	w := httptest.NewRecorder()
	h.CreateAlertFromGrafana(w, r)

	assertStatus(t, w, http.StatusAccepted)
	if len(store.enqueuedPayloads) != 1 {
		t.Fatalf("Enqueue called %d times, want 1", len(store.enqueuedPayloads))
	}
}

func TestCreateAlertFromGrafana_NonAlertingStateReturns200AndNeverEnqueues(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1")

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/grafana", bytes.NewReader(grafanaAlertJSON("resolved", "checkout", "1")))
	w := httptest.NewRecorder()
	h.CreateAlertFromGrafana(w, r)

	assertStatus(t, w, http.StatusOK)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called for a non-alerting state")
	}
}

func TestCreateAlertFromGrafana_MalformedBodyReturns400(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1")

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/grafana", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.CreateAlertFromGrafana(w, r)

	assertStatus(t, w, http.StatusBadRequest)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called for a malformed body")
	}
}

func TestCreateAlertFromGrafana_StoreFailureReturns500(t *testing.T) {
	store := &mockStore{enqueueFn: func(ctx context.Context, id string, buildPayload func(string) ([]byte, error)) (string, error) {
		return "", errors.New("connection refused")
	}}
	h := NewAlertHandler(store, "caller-1")

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/grafana", bytes.NewReader(grafanaAlertJSON("alerting", "checkout", "1")))
	w := httptest.NewRecorder()
	h.CreateAlertFromGrafana(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
	assertErrorMessage(t, w, ErrMsgInternal)
}
