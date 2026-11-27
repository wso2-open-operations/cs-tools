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

// testConfig points a Config at baseURL with a fake token endpoint that issues
// "access-tok" to the client-credentials app "cis-app" / "cis-secret".
func testConfig(t *testing.T, baseURL string) Config {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if err := r.ParseForm(); err != nil || !ok || id != "cis-app" || secret != "cis-secret" ||
			r.PostForm.Get("grant_type") != "client_credentials" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-tok","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenSrv.Close)
	return Config{BaseURL: baseURL, TokenURL: tokenSrv.URL, ClientID: "cis-app", ClientSecret: "cis-secret"}
}

func TestNewClient_NilWithAMissingSetting(t *testing.T) {
	full := Config{BaseURL: "https://x", TokenURL: "https://idp/token", ClientID: "a", ClientSecret: "b"}
	for name, mutate := range map[string]func(*Config){
		"base URL":      func(c *Config) { c.BaseURL = "" },
		"token URL":     func(c *Config) { c.TokenURL = " " },
		"client id":     func(c *Config) { c.ClientID = "" },
		"client secret": func(c *Config) { c.ClientSecret = " " },
	} {
		c := full
		mutate(&c)
		if NewClient(c) != nil {
			t.Errorf("NewClient returned a client without a %s", name)
		}
	}
	if NewClient(full) == nil {
		t.Fatal("NewClient returned nil with every setting")
	}
}

// TestClient_TokenFailureNeverCallsAlertCore: without an access token the
// request is not sent at all.
func TestClient_TokenFailureNeverCallsAlertCore(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()
	c := testConfig(t, srv.URL)
	c.ClientSecret = "wrong"
	if _, err := NewClient(c).MutedCounts(context.Background(), nil); err == nil || called {
		t.Fatalf("err=%v called=%v, want an error and no call to alert-core", err, called)
	}
}

func TestClient_SendsTokenCorrelationIDAndQuery(t *testing.T) {
	var gotAuth, gotCorr, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotCorr, gotPath, gotQuery = r.Header.Get("Authorization"), r.Header.Get("X-CSM-Correlation-ID"), r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	c := NewClient(testConfig(t, srv.URL+"/"))
	ctx := entity.WithCorrelationID(context.Background(), "cis-1")
	body, err := c.MutedIncidents(ctx, "11111111-1111-1111-1111-111111111111", url.Values{"limit": {"5"}})
	if err != nil {
		t.Fatalf("MutedIncidents: %v", err)
	}
	if string(body) != `{"items":[]}` {
		t.Errorf("body = %q", body)
	}
	if gotAuth != "Bearer access-tok" || gotCorr != "cis-1" {
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

	_, err := NewClient(testConfig(t, srv.URL)).FieldValues(context.Background(), nil)
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

	_, err := NewClient(testConfig(t, srv.URL)).MutedCounts(context.Background(), nil)
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
	if NewClient(Config{BaseURL: "http://alert-core:8080", TokenURL: "https://idp/token", ClientID: "a", ClientSecret: "b"}) != nil {
		t.Fatal("NewClient accepted a plain-http, non-loopback base URL")
	}
	if NewClient(Config{BaseURL: "https://alert-core", TokenURL: "http://idp.example/token", ClientID: "a", ClientSecret: "b"}) != nil {
		t.Fatal("NewClient accepted a plain-http, non-loopback token URL: the client secret would go in plaintext")
	}
}

// TestClient_TokenRequestDoesNotFollowRedirects: a redirect from the token
// endpoint fails the fetch, and the client secret (Basic auth, or the form body
// the library retries with) never reaches the redirect target, even on the same host.
func TestClient_TokenRequestDoesNotFollowRedirects(t *testing.T) {
	leaked := false
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/leak" {
			// The secret arrives as Basic auth, or in the body when the library retries without it.
			if r.Header.Get("Authorization") != "" || (r.ParseForm() == nil && r.PostForm.Get("client_secret") != "") {
				leaked = true
			}
			return
		}
		http.Redirect(w, r, "/leak", http.StatusTemporaryRedirect)
	}))
	defer tokenSrv.Close()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()
	_, err := NewClient(Config{BaseURL: srv.URL, TokenURL: tokenSrv.URL + "/token", ClientID: "a", ClientSecret: "b"}).
		MutedCounts(context.Background(), nil)
	if err == nil || leaked || called {
		t.Fatalf("err=%v leaked=%v called=%v, want a failed token fetch, no secret at the redirect target, no API call", err, leaked, called)
	}
}
