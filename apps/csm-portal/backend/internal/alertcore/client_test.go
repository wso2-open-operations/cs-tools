// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"sync/atomic"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// testConfig points a Config at baseURL with a fake token endpoint that issues "access-tok" to the client-credentials
// app "portal-app" / "portal-secret".
func testConfig(t *testing.T, baseURL string) Config {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if err := r.ParseForm(); err != nil || !ok || id != "portal-app" || secret != "portal-secret" ||
			r.PostForm.Get("grant_type") != "client_credentials" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-tok","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenSrv.Close)
	return Config{BaseURL: baseURL, TokenURL: tokenSrv.URL, ClientID: "portal-app", ClientSecret: "portal-secret"}
}

// TestNewClientSendsClientCredentialsAndScopes: the token request is a client-credentials grant with the configured
// scopes, and the access token is reused for the next call instead of fetched again.
func TestNewClientSendsClientCredentialsAndScopes(t *testing.T) {
	var tokenCalls atomic.Int32
	var gotScope string
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
		_ = r.ParseForm()
		gotScope = r.PostForm.Get("scope")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-tok","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := NewClient(Config{BaseURL: srv.URL, TokenURL: tokenSrv.URL, ClientID: "a", ClientSecret: "b", Scopes: []string{"read", "mute"}})
	for range 2 {
		if _, err := c.MutedCounts(context.Background(), []string{"x"}); err != nil {
			t.Fatal(err)
		}
	}
	if gotAuth != "Bearer access-tok" || gotScope != "read mute" || tokenCalls.Load() != 1 {
		t.Fatalf("auth=%q scope=%q token calls=%d, want the token once with both scopes", gotAuth, gotScope, tokenCalls.Load())
	}
}

// TestTokenFailureNeverCallsAlertCore: without an access token the request is not sent at all.
func TestTokenFailureNeverCallsAlertCore(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer tokenSrv.Close()
	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { called.Store(true) }))
	defer srv.Close()
	_, err := NewClient(Config{BaseURL: srv.URL, TokenURL: tokenSrv.URL, ClientID: "a", ClientSecret: "b"}).
		MutedIncidents(context.Background(), "abc", 20, 0)
	if err == nil || called.Load() {
		t.Fatalf("err=%v called=%v, want an error and no call to alert-core", err, called.Load())
	}
}

func TestMutedIncidentsSendsTokenAndPage(t *testing.T) {
	var gotAuth, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotQuery = r.Header.Get("Authorization"), r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"total":1}`))
	}))
	defer srv.Close()
	body, err := NewClient(testConfig(t, srv.URL+"/")).MutedIncidents(context.Background(), "abc", 20, 40)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"total":1}` || gotAuth != "Bearer access-tok" || gotPath != "/alert-mute-rules/abc/muted-incidents" || gotQuery != "limit=20&offset=40" {
		t.Fatalf("body=%s auth=%q path=%q query=%q", body, gotAuth, gotPath, gotQuery)
	}
}

func TestMutedIncidentsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer srv.Close()
	_, err := NewClient(testConfig(t, srv.URL)).MutedIncidents(context.Background(), "abc", 20, 0)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("err = %v, want an apierror with 401", err)
	}
}

func TestFieldValuesSendsFieldAndQuery(t *testing.T) {
	var gotAuth, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotQuery = r.Header.Get("Authorization"), r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"values":[]}`))
	}))
	defer srv.Close()
	body, err := NewClient(testConfig(t, srv.URL)).FieldValues(context.Background(), "service", "client prod", 10)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"values":[]}` || gotAuth != "Bearer access-tok" || gotPath != "/alert-field-values" ||
		gotQuery != "field=service&limit=10&q=client+prod" {
		t.Fatalf("body=%s auth=%q path=%q query=%q", body, gotAuth, gotPath, gotQuery)
	}
}

func TestGetRejectsOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, maxBody+1))
	}))
	defer srv.Close()
	if _, err := NewClient(testConfig(t, srv.URL)).MutedIncidents(context.Background(), "abc", 20, 0); err == nil {
		t.Fatal("want an error for a body over maxBody, got nil")
	}
}

func TestMutedCountsSendsIDs(t *testing.T) {
	var gotPath, gotIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotIDs = r.URL.Path, r.URL.Query().Get("ids")
		_, _ = w.Write([]byte(`{"counts":{}}`))
	}))
	defer srv.Close()
	if _, err := NewClient(testConfig(t, srv.URL)).MutedCounts(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/alert-mute-rules/muted-counts" || gotIDs != "a,b" {
		t.Fatalf("path=%q ids=%q", gotPath, gotIDs)
	}
}

// TestDoesNotFollowRedirects: a redirect is returned as an error, and the access token never reaches its target.
func TestDoesNotFollowRedirects(t *testing.T) {
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
	}))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/steal", http.StatusFound)
	}))
	defer srv.Close()
	_, err := NewClient(testConfig(t, srv.URL)).MutedCounts(context.Background(), []string{"a"})
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusFound || leaked.Load() {
		t.Fatalf("err=%v leaked=%v, want the 302 as an error and no call to the redirect target", err, leaked.Load())
	}
}
