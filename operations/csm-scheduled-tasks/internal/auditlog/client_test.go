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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.
package auditlog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

// newTestClient starts a loopback token endpoint and a loopback entity-service
// stand-in that records what it receives.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenSrv.Close)
	apiSrv := httptest.NewServer(handler)
	t.Cleanup(apiSrv.Close)

	c, err := NewClient(Config{BaseURL: apiSrv.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClient_RunMaintenancePostsAndDecodes(t *testing.T) {
	var method, path, auth string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"partitionsCreated":2,"healthProblems":["too few future months"],
			"repairedTriggers":[{"schema":"public","table":"project","problem":"registered table has no audit trigger"}],
			"unresolvedTriggers":[]}`))
	})

	result, err := c.RunMaintenance(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if method != http.MethodPost || path != "/audit-log/maintenance" || auth != "Bearer test-token" {
		t.Errorf("method=%q path=%q auth=%q", method, path, auth)
	}
	if result.PartitionsCreated != 2 || len(result.HealthProblems) != 1 || len(result.RepairedTriggers) != 1 || len(result.UnresolvedTriggers) != 0 {
		t.Errorf("result = %+v", result)
	}
	if result.RepairedTriggers[0].Table != "project" {
		t.Errorf("repaired trigger = %+v", result.RepairedTriggers[0])
	}
}

func TestClient_NonSuccessIsAnAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"only a service credential"}`, http.StatusForbidden)
	})

	_, err := c.RunMaintenance(context.Background())

	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("error = %v, want an *apierror.Error with status 403", err)
	}
}

func TestNewClient_RefusesInsecureURLs(t *testing.T) {
	if _, err := NewClient(Config{BaseURL: "http://entity.example.com", TokenURL: "https://sts.example.com/token"}); err == nil {
		t.Error("a plain-http base URL was accepted")
	}
	if _, err := NewClient(Config{BaseURL: "https://entity.example.com", TokenURL: "http://sts.example.com/token"}); err == nil {
		t.Error("a plain-http token URL was accepted")
	}
}
