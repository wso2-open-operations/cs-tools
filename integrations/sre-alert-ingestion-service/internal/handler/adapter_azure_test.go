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
	"strings"
	"testing"
)

func azureAlertJSON(severity, monitorCondition, monitoringService, alertRule string) []byte {
	return []byte(`{
		"data": {
			"essentials": {
				"alertId": "azure-alert-id-1",
				"alertRule": "` + alertRule + `",
				"severity": "` + severity + `",
				"monitorCondition": "` + monitorCondition + `",
				"monitoringService": "` + monitoringService + `"
			},
			"alertContext": {"foo": "bar"}
		}
	}`)
}

func TestMapAzurePayload_SeverityTable(t *testing.T) {
	cases := []struct {
		sev  string
		want string
	}{
		{"Sev0", "critical"},
		{"Sev1", "major"},
		{"Sev2", "minor"},
		{"Sev3", "warning"},
		{"Sev4", "ok"},
	}
	for _, tc := range cases {
		t.Run(tc.sev, func(t *testing.T) {
			body := azureAlertJSON(tc.sev, "Fired", "svc-checkout", "high_error_rate")
			req, err := mapAzurePayload(body)
			if err != nil {
				t.Fatalf("mapAzurePayload: %v", err)
			}
			if req.Severity != tc.want {
				t.Errorf("Severity = %q, want %q", req.Severity, tc.want)
			}
		})
	}
}

func TestMapAzurePayload_ResolvedForcesOk(t *testing.T) {
	body := azureAlertJSON("Sev0", "Resolved", "svc-checkout", "high_error_rate")
	req, err := mapAzurePayload(body)
	if err != nil {
		t.Fatalf("mapAzurePayload: %v", err)
	}
	if req.Severity != "ok" {
		t.Errorf("Severity = %q, want %q for a resolved condition regardless of Sev value", req.Severity, "ok")
	}
}

func TestMapAzurePayload_FieldMapping(t *testing.T) {
	body := azureAlertJSON("Sev1", "Fired", "svc-checkout", "high_error_rate")
	req, err := mapAzurePayload(body)
	if err != nil {
		t.Fatalf("mapAzurePayload: %v", err)
	}
	if req.Source != "azure" {
		t.Errorf("Source = %q, want %q", req.Source, "azure")
	}
	if req.Service != "svc-checkout" {
		t.Errorf("Service = %q, want %q", req.Service, "svc-checkout")
	}
	if req.MetricName != "high_error_rate" {
		t.Errorf("MetricName = %q, want %q", req.MetricName, "high_error_rate")
	}
	if req.UniqueIdentifier != "azure-alert-id-1" {
		t.Errorf("UniqueIdentifier = %q, want %q", req.UniqueIdentifier, "azure-alert-id-1")
	}
	if req.Description == "" {
		t.Error("Description is empty, want a rendered summary of alertContext")
	}
	if req.Environment != "" {
		t.Errorf("Environment = %q, want empty (not present in this payload shape)", req.Environment)
	}
}

func TestMapAzurePayload_ServiceFallback(t *testing.T) {
	body := azureAlertJSON("Sev1", "Fired", "", "high_error_rate")
	req, err := mapAzurePayload(body)
	if err != nil {
		t.Fatalf("mapAzurePayload: %v", err)
	}
	if req.Service != azureDefaultService {
		t.Errorf("Service = %q, want default %q when monitoringService is absent", req.Service, azureDefaultService)
	}
}

func TestMapAzurePayload_MetricNameFallback(t *testing.T) {
	body := azureAlertJSON("Sev1", "Fired", "svc-checkout", "")
	req, err := mapAzurePayload(body)
	if err != nil {
		t.Fatalf("mapAzurePayload: %v", err)
	}
	if req.MetricName != "svc-checkout" {
		t.Errorf("MetricName = %q, want fallback to monitoringService %q", req.MetricName, "svc-checkout")
	}

	body = azureAlertJSON("Sev1", "Fired", "", "")
	req, err = mapAzurePayload(body)
	if err != nil {
		t.Fatalf("mapAzurePayload: %v", err)
	}
	if req.MetricName != azureDefaultMetricName {
		t.Errorf("MetricName = %q, want fixed placeholder %q when both alertRule and monitoringService are absent", req.MetricName, azureDefaultMetricName)
	}
}

func TestMapAzurePayload_MalformedJSON(t *testing.T) {
	_, err := mapAzurePayload([]byte(`not json`))
	if err == nil {
		t.Fatal("mapAzurePayload should error on malformed JSON")
	}
}

// TestMapAzurePayload_MissingEssentialsRejected guards against a payload
// that unmarshals successfully but carries no real Azure alert data — Data
// and Essentials are pointers specifically so this is detectable (nil)
// rather than silently defaulting through every fallback and enqueuing a
// misleading alert.
func TestMapAzurePayload_MissingEssentialsRejected(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "empty object", body: `{}`},
		{name: "data present, essentials absent", body: `{"data":{"alertContext":{}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := mapAzurePayload([]byte(tc.body)); err == nil {
				t.Fatalf("mapAzurePayload(%q) should error on missing data.essentials, got nil", tc.body)
			}
		})
	}
}

func TestCreateAlertFromAzure_Success(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	body := azureAlertJSON("Sev0", "Fired", "svc-checkout", "high_error_rate")
	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/azure", bytes.NewReader(body))
	r = withAuthenticatedUsername(r, "azure")
	w := httptest.NewRecorder()
	h.CreateAlertFromAzure(w, r)

	assertStatus(t, w, http.StatusAccepted)
	if len(store.enqueuedPayloads) != 1 {
		t.Fatalf("Enqueue called %d times, want 1", len(store.enqueuedPayloads))
	}
}

// TestCreateAlertFromAzure_MismatchedAuthenticatedSourceReturns403 is the
// template regression test for the authorization gap this fix closes: a
// caller authenticated with a DIFFERENT vendor's credential (e.g. the
// site24x7 Basic Auth user) must not be able to hit another vendor's
// dedicated adapter route and have it accepted as that route's own fixed
// Source literal.
func TestCreateAlertFromAzure_MismatchedAuthenticatedSourceReturns403(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	body := azureAlertJSON("Sev0", "Fired", "svc-checkout", "high_error_rate")
	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/azure", bytes.NewReader(body))
	r = withAuthenticatedUsername(r, "site24x7")
	w := httptest.NewRecorder()
	h.CreateAlertFromAzure(w, r)

	assertStatus(t, w, http.StatusForbidden)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called when the authenticated identity does not match this adapter's fixed source")
	}
}

// TestCreateAlertFromAzure_NoAuthenticatedUsernameReturns500 covers the
// defensive path: should never happen given main.go's route wiring, but a
// missing authenticated identity must fail closed, not silently pass through.
func TestCreateAlertFromAzure_NoAuthenticatedUsernameReturns500(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	body := azureAlertJSON("Sev0", "Fired", "svc-checkout", "high_error_rate")
	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/azure", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateAlertFromAzure(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
	assertErrorMessage(t, w, ErrMsgInternal)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called when there is no authenticated identity in context")
	}
}

func TestCreateAlertFromAzure_MalformedBodyReturns400(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/azure", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.CreateAlertFromAzure(w, r)

	assertStatus(t, w, http.StatusBadRequest)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called for a malformed body")
	}
}

func TestCreateAlertFromAzure_ValidationFailureReturns400(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	// alertId containing a tag-delimiter character fails AlertRequest.validate.
	body := []byte(`{"data":{"essentials":{"alertId":"id]with[delims","alertRule":"r","severity":"Sev1","monitorCondition":"Fired","monitoringService":"svc"}}}`)
	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/azure", bytes.NewReader(body))
	r = withAuthenticatedUsername(r, "azure")
	w := httptest.NewRecorder()
	h.CreateAlertFromAzure(w, r)

	assertStatus(t, w, http.StatusBadRequest)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called when validation fails")
	}
}

func TestCreateAlertFromAzure_StoreFailureReturns500(t *testing.T) {
	store := &mockStore{enqueueFn: func(ctx context.Context, id string, buildPayload func(string) ([]byte, error)) (string, error) {
		return "", errors.New("connection refused")
	}}
	h := NewAlertHandler(store, "caller-1", nil)

	body := azureAlertJSON("Sev0", "Fired", "svc-checkout", "high_error_rate")
	r := httptest.NewRequest(http.MethodPost, "/alerts/adapters/azure", bytes.NewReader(body))
	r = withAuthenticatedUsername(r, "azure")
	w := httptest.NewRecorder()
	h.CreateAlertFromAzure(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
	assertErrorMessage(t, w, ErrMsgInternal)
}

func TestFlattenJSON(t *testing.T) {
	got := flattenJSON([]byte(`{"b":2,"a":1}`))
	if !strings.Contains(got, "a=1") || !strings.Contains(got, "b=2") {
		t.Errorf("flattenJSON = %q, want it to contain both key=value pairs", got)
	}
	// Non-object input falls back to raw bytes rather than erroring.
	got = flattenJSON([]byte(`[1,2,3]`))
	if got != "[1,2,3]" {
		t.Errorf("flattenJSON(array) = %q, want raw passthrough", got)
	}
}
