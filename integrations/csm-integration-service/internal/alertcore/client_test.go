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
package alertcore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/entity"
)

func TestNewClient_NilWithoutBaseURLOrToken(t *testing.T) {
	if NewClient(Config{BaseURL: "", Token: "t"}) != nil || NewClient(Config{BaseURL: "https://x", Token: " "}) != nil {
		t.Fatal("NewClient returned a client with a missing setting")
	}
}

func TestClient_SendsTokenCorrelationIDAndQuery(t *testing.T) {
	var gotAuth, gotCorr, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotCorr, gotPath, gotQuery = r.Header.Get("Authorization"), r.Header.Get("X-CSM-Correlation-ID"), r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL + "/", Token: "secret"})
	ctx := entity.WithCorrelationID(context.Background(), "cis-1")
	body, err := c.MutedIncidents(ctx, "11111111-1111-1111-1111-111111111111", url.Values{"limit": {"5"}})
	if err != nil {
		t.Fatalf("MutedIncidents: %v", err)
	}
	if string(body) != `{"items":[]}` {
		t.Errorf("body = %q", body)
	}
	if gotAuth != "Bearer secret" || gotCorr != "cis-1" {
		t.Errorf("auth %q correlation %q", gotAuth, gotCorr)
	}
	if gotPath != "/alert-mute-rules/11111111-1111-1111-1111-111111111111/muted-incidents" || gotQuery != "limit=5" {
		t.Errorf("path %q query %q", gotPath, gotQuery)
	}
}

func TestClient_NonSuccessIsAnAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	_, err := NewClient(Config{BaseURL: srv.URL, Token: "wrong"}).FieldValues(context.Background(), nil)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("err = %v, want *apierror.Error 401", err)
	}
}

func TestClient_DoesNotFollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://elsewhere.example/steal", http.StatusFound)
	}))
	defer srv.Close()

	_, err := NewClient(Config{BaseURL: srv.URL, Token: "secret"}).MutedCounts(context.Background(), nil)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusFound {
		t.Fatalf("err = %v, want the 302 as an error (redirect not followed)", err)
	}
}

func TestSafeBaseURL(t *testing.T) {
	for base, want := range map[string]bool{
		"https://alert-core.internal.example": true,
		"HTTPS://alert-core.internal.example": true,
		"http://localhost:8080":               true,
		"http://127.0.0.1:8080":               true,
		"http://[::1]:8080":                   true,
		"http://alert-core:8080":              false,
		"http://10.0.0.5:8080":                false,
		"ftp://alert-core":                    false,
		"alert-core:8080":                     false,
		"https://":                            false,
	} {
		if got := SafeBaseURL(base); got != want {
			t.Errorf("SafeBaseURL(%q) = %v, want %v", base, got, want)
		}
	}
	if NewClient(Config{BaseURL: "http://alert-core:8080", Token: "secret"}) != nil {
		t.Fatal("NewClient accepted a plain-http, non-loopback base URL")
	}
}
