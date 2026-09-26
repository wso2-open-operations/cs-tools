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

package csmclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/apierror"
)

// tokenServer returns an httptest.Server that always issues a client-credentials
// access token, so tests can exercise Client without a real OAuth2 provider.
func tokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"bearer","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCreateIncident_Success(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody CreateIncidentRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message":"created","incident":{"id":"inc-1","number":"INC0001234","createdOn":"2026-08-27T00:00:00Z","createdBy":"system"}}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	result, err := client.CreateIncident(context.Background(), CreateIncidentRequest{
		CallerID:  "caller-1",
		Category:  "SERVICE_INTERRUPTION",
		ServiceID: "svc-1",
		Impact:    "HIGH",
		Urgency:   "HIGH",
		Subject:   "test alert",
	})
	if err != nil {
		t.Fatalf("CreateIncident() error = %v, want nil", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/incidents" {
		t.Errorf("request = %s %s, want POST /incidents", gotMethod, gotPath)
	}
	if gotBody.CallerID != "caller-1" || gotBody.ServiceID != "svc-1" {
		t.Errorf("request body = %+v, missing expected fields", gotBody)
	}
	if result.IncidentID != "inc-1" || result.IncidentNumber != "INC0001234" {
		t.Errorf("result = %+v, want IncidentID=inc-1 IncidentNumber=INC0001234", result)
	}
}

// TestCreateIncident_401IsAReturnedAPIError verifies a 401 from
// csm-integration-service surfaces as a typed *apierror.Error with
// StatusCode 401 — CreateIncident itself does not special-case or swallow
// it. Classifying it as retryable is the caller's (internal/worker's)
// responsibility, exercised separately in internal/worker's tests.
func TestCreateIncident_401IsAReturnedAPIError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Missing or invalid user ID token header."}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	_, err := client.CreateIncident(context.Background(), CreateIncidentRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *apierror.Error", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
}

func TestCreateIncident_400IsAReturnedAPIError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid payload"}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	_, err := client.CreateIncident(context.Background(), CreateIncidentRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *apierror.Error", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusBadRequest)
	}
}

func TestCreateIncident_ToleratesUnknownResponseFields(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message":"created","incident":{"id":"inc-2","number":"INC0002","createdOn":"2026-08-27T00:00:00Z","createdBy":"system","someBrandNewField":{"nested":true}}}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	result, err := client.CreateIncident(context.Background(), CreateIncidentRequest{})
	if err != nil {
		t.Fatalf("CreateIncident() error = %v, want nil", err)
	}
	if result.IncidentID != "inc-2" {
		t.Errorf("IncidentID = %q, want %q", result.IncidentID, "inc-2")
	}
}

func TestCreateIncident_ForwardsCorrelationID(t *testing.T) {
	var gotID string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Header.Get("X-CSM-Correlation-ID")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message":"created","incident":{"id":"inc-1","number":"INC1"}}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	ctx := WithCorrelationID(context.Background(), "sais-test-id")
	if _, err := client.CreateIncident(ctx, CreateIncidentRequest{}); err != nil {
		t.Fatalf("CreateIncident() error = %v, want nil", err)
	}
	if gotID != "sais-test-id" {
		t.Errorf("X-CSM-Correlation-ID = %q, want %q", gotID, "sais-test-id")
	}
}

func TestUpdateIncident_Success(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody updateIncidentRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"updated","incident":{"id":"inc-1"}}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	if err := client.UpdateIncident(context.Background(), "inc-1", "this condition fired again"); err != nil {
		t.Fatalf("UpdateIncident() error = %v, want nil", err)
	}
	if gotMethod != http.MethodPatch || gotPath != "/incidents/inc-1" {
		t.Errorf("request = %s %s, want PATCH /incidents/inc-1", gotMethod, gotPath)
	}
	if gotBody.WorkNotes != "this condition fired again" {
		t.Errorf("request body WorkNotes = %q, want %q", gotBody.WorkNotes, "this condition fired again")
	}
}

// TestUpdateIncident_PathEscapesIncidentID confirms a delimiter- or
// reserved-character-bearing incidentID cannot smuggle extra path segments
// into the request — the id is always sent as csm-integration-service
// itself returned it (a CMDB/ServiceNow sys_id), but PathEscape is applied
// unconditionally rather than trusting that shape.
func TestUpdateIncident_PathEscapesIncidentID(t *testing.T) {
	var gotEscapedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.URL.Path is net/http's already-decoded form ("/incidents/inc/1"
		// either way) — EscapedPath() is what actually went out on the wire,
		// which is what this test needs to distinguish "id sent as one
		// escaped segment" from "id sent as two path segments".
		gotEscapedPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"updated","incident":{"id":"inc-1"}}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	if err := client.UpdateIncident(context.Background(), "inc/1", "note"); err != nil {
		t.Fatalf("UpdateIncident() error = %v, want nil", err)
	}
	if gotEscapedPath != "/incidents/inc%2F1" {
		t.Errorf("request escaped path = %q, want the escaped %q", gotEscapedPath, "/incidents/inc%2F1")
	}
}

func TestUpdateIncident_401IsAReturnedAPIError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Missing or invalid user ID token header."}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	err := client.UpdateIncident(context.Background(), "inc-1", "note")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *apierror.Error", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
}

// TestUpdateIncident_503IsAReturnedAPIError pins the Postgres-data-source
// case (csm-integration-service's own CLAUDE.md: this endpoint has no
// Postgres fallback path, unlike PATCH /cases/{id}) — a 503 from upstream
// must surface as a typed error, not be swallowed.
func TestUpdateIncident_503IsAReturnedAPIError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"updating an incident is not available on this data source yet"}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	err := client.UpdateIncident(context.Background(), "inc-1", "note")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *apierror.Error", err)
	}
	if apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusServiceUnavailable)
	}
}

// TestNewClient_TokenFetchDoesNotFollowRedirects guards against the
// client-credentials POST (ClientID/ClientSecret in the form body) being
// resent to a redirect target: a 307/308 from the token endpoint must not
// be followed.
func TestNewClient_TokenFetchDoesNotFollowRedirects(t *testing.T) {
	redirectTargetCalled := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"bearer","expires_in":3600}`))
	}))
	defer target.Close()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer tokenSrv.Close()

	client := newClient(Config{BaseURL: tokenSrv.URL, TokenURL: tokenSrv.URL + "/token", ClientID: "id", ClientSecret: "secret"}, true)
	_, err := client.CreateIncident(context.Background(), CreateIncidentRequest{})
	if err == nil {
		t.Fatal("expected an error for an unfollowed token-endpoint redirect, got nil")
	}
	if redirectTargetCalled {
		t.Error("client followed the redirect; ClientID/ClientSecret were resubmitted to the redirect target")
	}
}

func TestTokenFetchTimeout(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer tokenSrv.Close()

	tokenFetchTimeout = 100 * time.Millisecond
	t.Cleanup(func() { tokenFetchTimeout = 10 * time.Second })

	client := newClient(Config{BaseURL: tokenSrv.URL, TokenURL: tokenSrv.URL + "/token", ClientID: "id", ClientSecret: "secret"}, true)

	start := time.Now()
	_, err := client.CreateIncident(context.Background(), CreateIncidentRequest{})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from hung token server, got nil")
	}
	if elapsed > 2*time.Second {
		t.Errorf("token fetch took %v; expected <2s with 100ms timeout", elapsed)
	}
}
