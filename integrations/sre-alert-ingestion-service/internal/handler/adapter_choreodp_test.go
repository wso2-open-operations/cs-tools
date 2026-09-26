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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func choreoDPAlertJSON(severity, impact, urgency int) []byte {
	return []byte(fmt.Sprintf(`{
		"status": "New",
		"severity": %d,
		"impact": %d,
		"urgency": %d,
		"service": "client-medlineprod-alert-integration",
		"metric_name": "choreodp-system-mizzen-unable-connect-server-failed-default",
		"description": "<table>...</table>",
		"category": "service_interruption",
		"environment": "prod",
		"source": "choreodp-monitor"
	}`, severity, impact, urgency))
}

func TestMapChoreoDPPayload_SeverityTable(t *testing.T) {
	cases := []struct {
		severity int
		want     string
	}{
		{1, "critical"},
		{2, "major"},
		{3, "minor"},
		{4, "warning"},
		{0, "warning"},
		{99, "warning"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("severity=%d", tc.severity), func(t *testing.T) {
			req, err := mapChoreoDPPayload(choreoDPAlertJSON(tc.severity, 1, 1))
			if err != nil {
				t.Fatalf("mapChoreoDPPayload: %v", err)
			}
			if req.Severity != tc.want {
				t.Errorf("Severity = %q, want %q", req.Severity, tc.want)
			}
		})
	}
}

// TestMapChoreoDPPayload_ImpactUrgencyOverride pins the trap this adapter is
// the first to hit: unlike every other adapter, this source's impact/urgency
// are raw ServiceNow-convention integers (1=High, 2=Medium, 3=Low), passed
// through directly as AlertRequest.Impact/Urgency to bypass
// severity.MapImpactUrgency, not folded into the Severity string.
func TestMapChoreoDPPayload_ImpactUrgencyOverride(t *testing.T) {
	cases := []struct {
		name        string
		impact      int
		urgency     int
		wantImpact  *string
		wantUrgency *string
	}{
		{name: "1=HIGH", impact: 1, urgency: 1, wantImpact: strPtr("HIGH"), wantUrgency: strPtr("HIGH")},
		{name: "2=MEDIUM", impact: 2, urgency: 2, wantImpact: strPtr("MEDIUM"), wantUrgency: strPtr("MEDIUM")},
		{name: "3=LOW", impact: 3, urgency: 3, wantImpact: strPtr("LOW"), wantUrgency: strPtr("LOW")},
		{name: "unrecognized values left unset", impact: 9, urgency: 0, wantImpact: nil, wantUrgency: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := mapChoreoDPPayload(choreoDPAlertJSON(1, tc.impact, tc.urgency))
			if err != nil {
				t.Fatalf("mapChoreoDPPayload: %v", err)
			}
			if !strPtrEqual(req.Impact, tc.wantImpact) {
				t.Errorf("Impact = %v, want %v", strPtrVal(req.Impact), strPtrVal(tc.wantImpact))
			}
			if !strPtrEqual(req.Urgency, tc.wantUrgency) {
				t.Errorf("Urgency = %v, want %v", strPtrVal(req.Urgency), strPtrVal(tc.wantUrgency))
			}
		})
	}
}

func TestMapChoreoDPPayload_FieldMapping(t *testing.T) {
	req, err := mapChoreoDPPayload(choreoDPAlertJSON(1, 1, 1))
	if err != nil {
		t.Fatalf("mapChoreoDPPayload: %v", err)
	}
	if req.Source != "choreodp-monitor" {
		t.Errorf("Source = %q, want the payload's own source field %q", req.Source, "choreodp-monitor")
	}
	if req.Service != "client-medlineprod-alert-integration" {
		t.Errorf("Service = %q, want the payload's own service field", req.Service)
	}
	if req.MetricName != "choreodp-system-mizzen-unable-connect-server-failed-default" {
		t.Errorf("MetricName = %q, want the payload's own metric_name field", req.MetricName)
	}
	if req.Description != "<table>...</table>" {
		t.Errorf("Description = %q, want the payload's own raw HTML description field", req.Description)
	}
	if req.Category != "service_interruption" {
		t.Errorf("Category = %q, want the payload's own category field", req.Category)
	}
	if req.Environment != "prod" {
		t.Errorf("Environment = %q, want the payload's own environment field", req.Environment)
	}
	if req.UniqueIdentifier != "" {
		t.Errorf("UniqueIdentifier = %q, want empty (no field available for grouping in this payload shape)", req.UniqueIdentifier)
	}
}

func TestMapChoreoDPPayload_MalformedJSON(t *testing.T) {
	_, err := mapChoreoDPPayload([]byte(`not json`))
	if err == nil {
		t.Fatal("mapChoreoDPPayload should error on malformed JSON")
	}
}

func TestCreateAlertFromChoreoDP_Success(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/choreodp", bytes.NewReader(choreoDPAlertJSON(1, 1, 1)))
	r = withAuthenticatedUsername(r, "choreodp-monitor")
	w := httptest.NewRecorder()
	h.CreateAlertFromChoreoDP(w, r)

	assertStatus(t, w, http.StatusAccepted)
	if len(store.enqueuedPayloads) != 1 {
		t.Fatalf("Enqueue called %d times, want 1", len(store.enqueuedPayloads))
	}
}

// TestCreateAlertFromChoreoDP_MismatchedAuthenticatedSourceReturns403 mirrors
// TestCreateAlertFromAzure_MismatchedAuthenticatedSourceReturns403 -- see its
// doc comment. This source's Source comes straight from the payload's own
// "source" field (unlike every other adapter's fixed literal), so the
// mismatch check still applies exactly the same way.
func TestCreateAlertFromChoreoDP_MismatchedAuthenticatedSourceReturns403(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/choreodp", bytes.NewReader(choreoDPAlertJSON(1, 1, 1)))
	r = withAuthenticatedUsername(r, "azure")
	w := httptest.NewRecorder()
	h.CreateAlertFromChoreoDP(w, r)

	assertStatus(t, w, http.StatusForbidden)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called when the authenticated identity does not match this alert's claimed source")
	}
}

// TestCreateAlertFromChoreoDP_NoAuthenticatedUsernameReturns500 mirrors
// TestCreateAlertFromAzure_NoAuthenticatedUsernameReturns500 -- see its doc
// comment.
func TestCreateAlertFromChoreoDP_NoAuthenticatedUsernameReturns500(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/choreodp", bytes.NewReader(choreoDPAlertJSON(1, 1, 1)))
	w := httptest.NewRecorder()
	h.CreateAlertFromChoreoDP(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
	assertErrorMessage(t, w, ErrMsgInternal)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called when there is no authenticated identity in context")
	}
}

func TestCreateAlertFromChoreoDP_MalformedBodyReturns400(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/choreodp", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.CreateAlertFromChoreoDP(w, r)

	assertStatus(t, w, http.StatusBadRequest)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called for a malformed body")
	}
}

func TestCreateAlertFromChoreoDP_StoreFailureReturns500(t *testing.T) {
	store := &mockStore{enqueueFn: func(ctx context.Context, id string, buildPayload func(string) ([]byte, error)) (string, error) {
		return "", errors.New("connection refused")
	}}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/choreodp", bytes.NewReader(choreoDPAlertJSON(1, 1, 1)))
	r = withAuthenticatedUsername(r, "choreodp-monitor")
	w := httptest.NewRecorder()
	h.CreateAlertFromChoreoDP(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
	assertErrorMessage(t, w, ErrMsgInternal)
}

func strPtr(s string) *string { return &s }

func strPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func strPtrVal(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
