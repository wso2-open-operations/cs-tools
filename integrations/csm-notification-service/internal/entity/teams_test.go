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
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// TestGetTeamChatWebhooks_SendsBearerToken is GetCaseDescription's
// GET /teams/chat-webhooks counterpart — see that test's own doc comment.
func TestGetTeamChatWebhooks_SendsBearerToken(t *testing.T) {
	var gotAuthorization, gotPath string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"teams": []map[string]string{
				{"name": "Apollo", "chatWebhookUrl": "https://chat.googleapis.com/v1/spaces/apollo"},
			},
		})
	}))
	defer apiSrv.Close()
	tokenSrv := newCustomerTokenServer(t)
	defer tokenSrv.Close()

	c := newTestCustomerClient(t, tokenSrv, apiSrv)

	got, err := c.GetTeamChatWebhooks(context.Background())
	if err != nil {
		t.Fatalf("GetTeamChatWebhooks returned error: %v", err)
	}
	want := map[string]string{"Apollo": "https://chat.googleapis.com/v1/spaces/apollo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetTeamChatWebhooks() = %v, want %v", got, want)
	}
	if gotPath != "/teams/chat-webhooks" {
		t.Errorf("request path = %q, want /teams/chat-webhooks", gotPath)
	}
	if gotAuthorization != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", gotAuthorization, "Bearer test-token")
	}
}

// TestGetTeamChatWebhooks_SkipsBlankAndDuplicateNames mirrors
// NewGoogleChatClient's own defensive posture for its config-sourced
// spaces: a blank name/URL is dropped, and two entries sharing a name
// collapse to "no webhook" for it rather than guessing which one is right.
func TestGetTeamChatWebhooks_SkipsBlankAndDuplicateNames(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"teams": []map[string]string{
				{"name": "", "chatWebhookUrl": "https://example.com/blank-name"},
				{"name": "Castor", "chatWebhookUrl": ""},
				{"name": "Apollo", "chatWebhookUrl": "https://example.com/apollo-1"},
				{"name": "Apollo", "chatWebhookUrl": "https://example.com/apollo-2"},
				{"name": "Vega", "chatWebhookUrl": "https://example.com/vega"},
			},
		})
	}))
	defer apiSrv.Close()
	tokenSrv := newCustomerTokenServer(t)
	defer tokenSrv.Close()

	c := newTestCustomerClient(t, tokenSrv, apiSrv)

	got, err := c.GetTeamChatWebhooks(context.Background())
	if err != nil {
		t.Fatalf("GetTeamChatWebhooks returned error: %v", err)
	}
	want := map[string]string{"Apollo": "", "Vega": "https://example.com/vega"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetTeamChatWebhooks() = %v, want %v", got, want)
	}
}

// TestGetTeamGroupEmails_SendsBearerToken mirrors
// TestGetTeamChatWebhooks_SendsBearerToken for GET /teams/group-emails.
func TestGetTeamGroupEmails_SendsBearerToken(t *testing.T) {
	var gotAuthorization, gotPath string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"teams": []map[string]string{
				{"key": "americas_cre_team", "groupEmail": "americas-cre@example.test"},
			},
		})
	}))
	defer apiSrv.Close()
	tokenSrv := newCustomerTokenServer(t)
	defer tokenSrv.Close()

	c := newTestCustomerClient(t, tokenSrv, apiSrv)

	got, err := c.GetTeamGroupEmails(context.Background())
	if err != nil {
		t.Fatalf("GetTeamGroupEmails returned error: %v", err)
	}
	want := map[string]string{"americas_cre_team": "americas-cre@example.test"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetTeamGroupEmails() = %v, want %v", got, want)
	}
	if gotPath != "/teams/group-emails" {
		t.Errorf("request path = %q, want /teams/group-emails", gotPath)
	}
	if gotAuthorization != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", gotAuthorization, "Bearer test-token")
	}
}

// TestGetTeamGroupEmails_SkipsBlankAndDuplicateKeys mirrors
// TestGetTeamChatWebhooks_SkipsBlankAndDuplicateNames, keyed by key instead
// of name.
func TestGetTeamGroupEmails_SkipsBlankAndDuplicateKeys(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"teams": []map[string]string{
				{"key": "", "groupEmail": "blank-key@example.test"},
				{"key": "castor", "groupEmail": ""},
				{"key": "apollo", "groupEmail": "apollo-1@example.test"},
				{"key": "apollo", "groupEmail": "apollo-2@example.test"},
				{"key": "vega", "groupEmail": "vega@example.test"},
			},
		})
	}))
	defer apiSrv.Close()
	tokenSrv := newCustomerTokenServer(t)
	defer tokenSrv.Close()

	c := newTestCustomerClient(t, tokenSrv, apiSrv)

	got, err := c.GetTeamGroupEmails(context.Background())
	if err != nil {
		t.Fatalf("GetTeamGroupEmails returned error: %v", err)
	}
	want := map[string]string{"apollo": "", "vega": "vega@example.test"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetTeamGroupEmails() = %v, want %v", got, want)
	}
}
