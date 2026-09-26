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

func openSearchAlertJSON(severity string) []byte {
	return []byte(`{
		"source": "High CPU utilization on checkout-service",
		"description": "CPU exceeded 90% for 5 minutes",
		"timestamp": "2026-09-01T12:00:00Z",
		"severity": "` + severity + `"
	}`)
}

func TestMapOpenSearchPayload_SeverityTable(t *testing.T) {
	cases := []struct {
		severity string
		want     string
	}{
		{"critical", "critical"},
		{"urgent", "major"},
		{"warning", "warning"},
		{"low", "minor"},
		{"informational", "ok"},
		{"some-unrecognized-value", "ok"},
		{"", "ok"},
	}
	for _, tc := range cases {
		t.Run(tc.severity, func(t *testing.T) {
			req, err := mapOpenSearchPayload(openSearchAlertJSON(tc.severity))
			if err != nil {
				t.Fatalf("mapOpenSearchPayload: %v", err)
			}
			if req.Severity != tc.want {
				t.Errorf("Severity = %q, want %q", req.Severity, tc.want)
			}
		})
	}
}

// TestMapOpenSearchPayload_SourceIsFixedLiteral pins the trap this vendor's
// payload sets: its own "source" field is a human-readable title, not the
// system identity, and must never leak into AlertRequest.Source.
func TestMapOpenSearchPayload_SourceIsFixedLiteral(t *testing.T) {
	req, err := mapOpenSearchPayload(openSearchAlertJSON("critical"))
	if err != nil {
		t.Fatalf("mapOpenSearchPayload: %v", err)
	}
	if req.Source != "opensearch" {
		t.Errorf("Source = %q, want fixed literal %q (not the payload's own 'source' field)", req.Source, "opensearch")
	}
	if req.MetricName != "High CPU utilization on checkout-service" {
		t.Errorf("MetricName = %q, want the payload's own 'source' field mapped here", req.MetricName)
	}
}

func TestMapOpenSearchPayload_FieldMapping(t *testing.T) {
	req, err := mapOpenSearchPayload(openSearchAlertJSON("critical"))
	if err != nil {
		t.Fatalf("mapOpenSearchPayload: %v", err)
	}
	if req.Description != "CPU exceeded 90% for 5 minutes" {
		t.Errorf("Description = %q, want the payload's own description field", req.Description)
	}
	if req.Service != openSearchDefaultService {
		t.Errorf("Service = %q, want default %q (no service field in this payload)", req.Service, openSearchDefaultService)
	}
	if req.UniqueIdentifier != "" {
		t.Errorf("UniqueIdentifier = %q, want empty (no field available for grouping in this payload shape)", req.UniqueIdentifier)
	}
}

func TestMapOpenSearchPayload_MalformedJSON(t *testing.T) {
	_, err := mapOpenSearchPayload([]byte(`not json`))
	if err == nil {
		t.Fatal("mapOpenSearchPayload should error on malformed JSON")
	}
}

func TestCreateAlertFromOpenSearch_Success(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/opensearch", bytes.NewReader(openSearchAlertJSON("critical")))
	r = withAuthenticatedUsername(r, "opensearch")
	w := httptest.NewRecorder()
	h.CreateAlertFromOpenSearch(w, r)

	assertStatus(t, w, http.StatusAccepted)
	if len(store.enqueuedPayloads) != 1 {
		t.Fatalf("Enqueue called %d times, want 1", len(store.enqueuedPayloads))
	}
}

// TestCreateAlertFromOpenSearch_MismatchedAuthenticatedSourceReturns403
// mirrors TestCreateAlertFromAzure_MismatchedAuthenticatedSourceReturns403 --
// see its doc comment.
func TestCreateAlertFromOpenSearch_MismatchedAuthenticatedSourceReturns403(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/opensearch", bytes.NewReader(openSearchAlertJSON("critical")))
	r = withAuthenticatedUsername(r, "azure")
	w := httptest.NewRecorder()
	h.CreateAlertFromOpenSearch(w, r)

	assertStatus(t, w, http.StatusForbidden)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called when the authenticated identity does not match this adapter's fixed source")
	}
}

// TestCreateAlertFromOpenSearch_NoAuthenticatedUsernameReturns500 mirrors
// TestCreateAlertFromAzure_NoAuthenticatedUsernameReturns500 -- see its doc
// comment.
func TestCreateAlertFromOpenSearch_NoAuthenticatedUsernameReturns500(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/opensearch", bytes.NewReader(openSearchAlertJSON("critical")))
	w := httptest.NewRecorder()
	h.CreateAlertFromOpenSearch(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
	assertErrorMessage(t, w, ErrMsgInternal)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called when there is no authenticated identity in context")
	}
}

func TestCreateAlertFromOpenSearch_MalformedBodyReturns400(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/opensearch", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.CreateAlertFromOpenSearch(w, r)

	assertStatus(t, w, http.StatusBadRequest)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called for a malformed body")
	}
}

func TestCreateAlertFromOpenSearch_StoreFailureReturns500(t *testing.T) {
	store := &mockStore{enqueueFn: func(ctx context.Context, id string, buildPayload func(string) ([]byte, error)) (string, error) {
		return "", errors.New("connection refused")
	}}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/opensearch", bytes.NewReader(openSearchAlertJSON("critical")))
	r = withAuthenticatedUsername(r, "opensearch")
	w := httptest.NewRecorder()
	h.CreateAlertFromOpenSearch(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
	assertErrorMessage(t, w, ErrMsgInternal)
}
