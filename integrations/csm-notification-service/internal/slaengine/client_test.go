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

package slaengine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func newTestEntityClient(t *testing.T, apiSrv *httptest.Server) *EntityClient {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "token_type": "Bearer", "expires_in": 3600})
	}))
	t.Cleanup(tokenSrv.Close)
	return NewEntityClient(EntityConfig{BaseURL: apiSrv.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})
}

// TestGetDurationPolicy_BuildsSeverityClockTypeMap verifies the response is
// grouped by severity then clockType, and durationSeconds is converted to a
// real time.Duration.
func TestGetDurationPolicy_BuildsSeverityClockTypeMap(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sla-duration-policy" {
			t.Errorf("path = %q, want /sla-duration-policy", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(slaDurationPolicyResponse{Policies: []slaDurationPolicyItem{
			{Severity: "CATASTROPHIC", ClockType: "response", DurationSeconds: 900},
			{Severity: "CATASTROPHIC", ClockType: "workaround", DurationSeconds: 14400},
			{Severity: "LOW", ClockType: "response", DurationSeconds: 86400},
		}})
	}))
	defer apiSrv.Close()

	c := newTestEntityClient(t, apiSrv)
	got, err := c.GetDurationPolicy(t.Context())
	if err != nil {
		t.Fatalf("GetDurationPolicy() error = %v, want nil", err)
	}

	if got["CATASTROPHIC"][ClockResponse] != 15*time.Minute {
		t.Errorf("CATASTROPHIC/response = %v, want 15m", got["CATASTROPHIC"][ClockResponse])
	}
	if got["CATASTROPHIC"][ClockWorkaround] != 4*time.Hour {
		t.Errorf("CATASTROPHIC/workaround = %v, want 4h", got["CATASTROPHIC"][ClockWorkaround])
	}
	if _, ok := got["CATASTROPHIC"][ClockResolution]; ok {
		t.Error("CATASTROPHIC/resolution present, want absent (no row in the response)")
	}
	if got["LOW"][ClockResponse] != 24*time.Hour {
		t.Errorf("LOW/response = %v, want 24h", got["LOW"][ClockResponse])
	}
}

func TestGetDurationPolicy_EmptyResult(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(slaDurationPolicyResponse{})
	}))
	defer apiSrv.Close()

	c := newTestEntityClient(t, apiSrv)
	got, err := c.GetDurationPolicy(t.Context())
	if err != nil {
		t.Fatalf("GetDurationPolicy() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestGetDurationPolicy_UpstreamError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer apiSrv.Close()

	c := newTestEntityClient(t, apiSrv)
	if _, err := c.GetDurationPolicy(t.Context()); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}

// TestGetActiveCSMSLAClocks_PagesThroughEveryRow confirms the client keeps
// requesting pages (always with source=csm) until it has every row the
// server reports via total, not just the first page.
func TestGetActiveCSMSLAClocks_PagesThroughEveryRow(t *testing.T) {
	const total = activeSLAStatusPageSize + 1 // forces a second page
	var gotPaths []string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.String())
		if got := r.URL.Query().Get("source"); got != "csm" {
			t.Errorf("source query param = %q, want csm", got)
		}
		offset := 0
		if raw := r.URL.Query().Get("offset"); raw != "" {
			offset, _ = strconv.Atoi(raw)
		}
		remaining := total - offset
		pageSize := activeSLAStatusPageSize
		if remaining < pageSize {
			pageSize = remaining
		}
		statuses := make([]activeSLAClock, pageSize)
		for i := range statuses {
			statuses[i] = activeSLAClock{CaseID: "case-" + strconv.Itoa(offset+i), ClockType: ClockResponse}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(searchActiveSLAStatusResponse{Statuses: statuses, Total: total})
	}))
	defer apiSrv.Close()

	c := newTestEntityClient(t, apiSrv)
	got, err := c.GetActiveCSMSLAClocks(t.Context())
	if err != nil {
		t.Fatalf("GetActiveCSMSLAClocks() error = %v, want nil", err)
	}
	if len(got) != total {
		t.Errorf("len(got) = %d, want %d", len(got), total)
	}
	if len(gotPaths) != 2 {
		t.Errorf("requests made = %d, want 2 (one per page)", len(gotPaths))
	}
}

func TestGetActiveCSMSLAClocks_UpstreamError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer apiSrv.Close()

	c := newTestEntityClient(t, apiSrv)
	if _, err := c.GetActiveCSMSLAClocks(t.Context()); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}
