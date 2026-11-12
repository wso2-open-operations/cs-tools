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

package plgvalidation

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeServer serves a token and the two calls the clients make.
func fakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/token":
			_, _ = io.WriteString(w, `{"access_token":"t","token_type":"Bearer","expires_in":3600}`)
		case r.Header.Get("Authorization") != "Bearer t":
			w.WriteHeader(http.StatusUnauthorized)
		case r.Method == http.MethodGet && r.URL.Path == "/users/u-1/email":
			_, _ = io.WriteString(w, `{"userUuid":"u-1","email":"jane@acme.example"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/ai/email-domains/classify":
			raw, _ := io.ReadAll(r.Body)
			if string(raw) != `{"domain":"acme.example"}` {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"domain":"acme.example","classification":"CORPORATE","source":"model"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"email":"must-not-leak@acme.example"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClients(t *testing.T) {
	srv := fakeServer(t)
	cfg := Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret"}
	ctx := context.Background()

	dir, err := NewDirectoryClient(DirectoryConfig{Config: cfg, UserEmailPath: "/users/{userId}/email"})
	if err != nil {
		t.Fatal(err)
	}
	if email, err := dir.UserEmail(ctx, "u-1"); err != nil || email != "jane@acme.example" {
		t.Errorf("UserEmail = %q, %v", email, err)
	}
	if _, err := dir.UserEmail(ctx, "u-2"); err == nil || strings.Contains(err.Error(), "@") {
		t.Errorf("missing user: err %v, want a status and no body", err)
	}

	ai, err := NewAIClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if class, err := ai.Classify(ctx, "acme.example"); err != nil || class != "CORPORATE" {
		t.Errorf("Classify = %q, %v", class, err)
	}
}
