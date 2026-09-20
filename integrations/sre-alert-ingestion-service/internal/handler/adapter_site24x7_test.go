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

func site24x7AlertJSON(status string) []byte {
	return []byte(`{
		"STATUS": "` + status + `",
		"MONITORNAME": "checkout-api",
		"MONITOR_ID": "mon-123",
		"INCIDENT_REASON": "Connection timeout",
		"INCIDENT_DETAILS": "5 consecutive failures"
	}`)
}

func TestMapSite24x7Payload_SeverityTable(t *testing.T) {
	cases := []struct {
		status string
		want   string
	}{
		{"DOWN", "critical"},
		{"CRITICAL", "critical"},
		{"TROUBLE", "warning"},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			req, ok, err := mapSite24x7Payload(site24x7AlertJSON(tc.status))
			if err != nil {
				t.Fatalf("mapSite24x7Payload: %v", err)
			}
			if !ok {
				t.Fatalf("ok = false, want true for actionable STATUS %q", tc.status)
			}
			if req.Severity != tc.want {
				t.Errorf("Severity = %q, want %q", req.Severity, tc.want)
			}
		})
	}
}

func TestMapSite24x7Payload_NonActionableStatusIsFiltered(t *testing.T) {
	cases := []string{"UP", "trouble", "down", "critical", "", "SUSPENDED"}
	for _, status := range cases {
		t.Run(status, func(t *testing.T) {
			_, ok, err := mapSite24x7Payload(site24x7AlertJSON(status))
			if err != nil {
				t.Fatalf("mapSite24x7Payload: %v", err)
			}
			if ok {
				t.Errorf("ok = true for STATUS %q, want false (case-sensitive match required, only TROUBLE/DOWN/CRITICAL actionable)", status)
			}
		})
	}
}

func TestMapSite24x7Payload_FieldMapping(t *testing.T) {
	req, ok, err := mapSite24x7Payload(site24x7AlertJSON("DOWN"))
	if err != nil {
		t.Fatalf("mapSite24x7Payload: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if req.Source != "site24x7" {
		t.Errorf("Source = %q, want %q", req.Source, "site24x7")
	}
	if req.MetricName != "checkout-api" {
		t.Errorf("MetricName = %q, want %q", req.MetricName, "checkout-api")
	}
	if req.UniqueIdentifier != "mon-123" {
		t.Errorf("UniqueIdentifier = %q, want %q", req.UniqueIdentifier, "mon-123")
	}
	if req.Service != site24x7DefaultService {
		t.Errorf("Service = %q, want default %q (no service field in this payload)", req.Service, site24x7DefaultService)
	}
	if req.Description == "" {
		t.Error("Description is empty, want the incident reason/details combined")
	}
}

func TestMapSite24x7Payload_MalformedJSON(t *testing.T) {
	_, _, err := mapSite24x7Payload([]byte(`not json`))
	if err == nil {
		t.Fatal("mapSite24x7Payload should error on malformed JSON")
	}
}

func TestCreateAlertFromSite24x7_Success(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1")

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/site24x7", bytes.NewReader(site24x7AlertJSON("DOWN")))
	w := httptest.NewRecorder()
	h.CreateAlertFromSite24x7(w, r)

	assertStatus(t, w, http.StatusAccepted)
	if len(store.enqueuedPayloads) != 1 {
		t.Fatalf("Enqueue called %d times, want 1", len(store.enqueuedPayloads))
	}
}

func TestCreateAlertFromSite24x7_NonActionableStatusReturns200AndNeverEnqueues(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1")

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/site24x7", bytes.NewReader(site24x7AlertJSON("UP")))
	w := httptest.NewRecorder()
	h.CreateAlertFromSite24x7(w, r)

	assertStatus(t, w, http.StatusOK)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called for a non-actionable STATUS")
	}
}

func TestCreateAlertFromSite24x7_MalformedBodyReturns400(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1")

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/site24x7", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.CreateAlertFromSite24x7(w, r)

	assertStatus(t, w, http.StatusBadRequest)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called for a malformed body")
	}
}

func TestCreateAlertFromSite24x7_StoreFailureReturns500(t *testing.T) {
	store := &mockStore{enqueueFn: func(ctx context.Context, id string, buildPayload func(string) ([]byte, error)) (string, error) {
		return "", errors.New("connection refused")
	}}
	h := NewAlertHandler(store, "caller-1")

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/site24x7", bytes.NewReader(site24x7AlertJSON("DOWN")))
	w := httptest.NewRecorder()
	h.CreateAlertFromSite24x7(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
	assertErrorMessage(t, w, ErrMsgInternal)
}
