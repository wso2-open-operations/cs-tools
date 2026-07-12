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

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

func newTestEngineeringClient(t *testing.T, tokenSrv, apiSrv *httptest.Server) *EngineeringEntityClient {
	t.Helper()
	return NewEngineeringEntityClient(EngineeringEntityConfig{
		BaseURL:      apiSrv.URL,
		TokenURL:     tokenSrv.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Scopes:       []string{"engineering"},
	})
}

func newEngineeringTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
}

func TestCreateGitIssue_ValidatesArgumentsBeforeCallingUpstream(t *testing.T) {
	called := false
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()
	tokenSrv := newEngineeringTokenServer(t)
	defer tokenSrv.Close()

	c := newTestEngineeringClient(t, tokenSrv, apiSrv)

	t.Run("rejects missing repo coordinates", func(t *testing.T) {
		if _, err := c.CreateGitIssue(context.Background(), "", "wso2-open-operations", "cs-tools", "title", "", nil); err == nil {
			t.Fatal("expected error for missing orgName, got nil")
		}
	})

	t.Run("rejects empty title", func(t *testing.T) {
		if _, err := c.CreateGitIssue(context.Background(), "wso2-open-operations", "wso2-open-operations", "cs-tools", "", "", nil); err == nil {
			t.Fatal("expected error for empty title, got nil")
		}
	})

	if called {
		t.Error("upstream should not have been called for invalid arguments")
	}
}

func TestCreateGitIssue_SendsExpectedRequest(t *testing.T) {
	var capturedPath string
	var capturedAuth string
	var capturedBody createGitHubIssueRequest
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(GitHubIssue{
			ID:     1,
			NodeID: "node-1",
			Number: 42,
			State:  "open",
			Title:  "Something broke",
			Labels: []GitHubIssueLabel{{Name: "bug", Color: "d73a4a"}},
		})
	}))
	defer apiSrv.Close()
	tokenSrv := newEngineeringTokenServer(t)
	defer tokenSrv.Close()

	c := newTestEngineeringClient(t, tokenSrv, apiSrv)

	issue, err := c.CreateGitIssue(context.Background(),
		"wso2-open-operations", "wso2-open-operations", "cs-tools",
		"Something broke", "steps to reproduce", []string{"bug"})
	if err != nil {
		t.Fatalf("CreateGitIssue returned error: %v", err)
	}

	wantPath := "/orgs/wso2-open-operations/repos/wso2-open-operations/cs-tools/issues"
	if capturedPath != wantPath {
		t.Errorf("path = %q, want %q", capturedPath, wantPath)
	}
	if capturedAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", capturedAuth, "Bearer test-token")
	}
	if capturedBody.Title != "Something broke" {
		t.Errorf("Title = %q, want %q", capturedBody.Title, "Something broke")
	}
	if len(capturedBody.Labels) != 1 || capturedBody.Labels[0] != "bug" {
		t.Errorf("Labels = %v, want [bug]", capturedBody.Labels)
	}
	if issue.Number != 42 {
		t.Errorf("issue.Number = %d, want 42", issue.Number)
	}
}

func TestCreateGitIssue_MapsUpstreamError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Repository not found: cs-tools in organization: wso2-open-operations."})
	}))
	defer apiSrv.Close()
	tokenSrv := newEngineeringTokenServer(t)
	defer tokenSrv.Close()

	c := newTestEngineeringClient(t, tokenSrv, apiSrv)

	_, err := c.CreateGitIssue(context.Background(), "wso2-open-operations", "wso2-open-operations", "cs-tools", "title", "", nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *apierror.Error, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func TestNewEngineeringEntityClient_ConstructsWithZeroValueConfig(t *testing.T) {
	// NewEngineeringEntityClient must never fail or panic even when the
	// engineering entity service has not been configured for a given deployment.
	c := NewEngineeringEntityClient(EngineeringEntityConfig{})
	if c == nil {
		t.Fatal("NewEngineeringEntityClient returned nil for zero-value EngineeringEntityConfig")
	}
}
