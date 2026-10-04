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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestTokenFetchTimeout verifies that a stalled token endpoint fails requests
// within the configured timeout rather than blocking indefinitely.
func TestTokenFetchTimeout(t *testing.T) {
	t.Parallel()

	// Token server that stalls longer than tokenFetchTimeout but returns eventually
	// so httptest.Server.Close() can drain cleanly.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer tokenSrv.Close()

	// Use a short timeout so the test runs quickly.
	tokenFetchTimeout = 100 * time.Millisecond
	t.Cleanup(func() { tokenFetchTimeout = 10 * time.Second })

	client := NewCustomerEntityClient(CustomerEntityConfig{
		BaseURL:      tokenSrv.URL,
		TokenURL:     tokenSrv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	start := time.Now()
	_, err := client.GetCase(context.Background(), "11111111-1111-1111-1111-111111111111")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from hung token server, got nil")
	}
	// Should fail well within 1 s (configured at 100 ms); allow 2 s for CI jitter.
	if elapsed > 2*time.Second {
		t.Errorf("token fetch took %v; expected <2s with 100ms timeout", elapsed)
	}
}

// TestSearchTagsSendsPostWithBody pins the entity-service call shape for tag
// search: a POST to /tags/search carrying the caller's body byte-for-byte. The
// old contract was a GET with `q`/`limit` query params and is gone; asserting
// on the raw bytes (rather than decoding through a struct) is what makes this
// test able to catch a silent key rename.
func TestSearchTagsSendsPostWithBody(t *testing.T) {
	t.Parallel()

	const reqBody = `{"filters":{"searchQuery":"micro"},"limit":20}`

	var gotMethod, gotPath, gotRawQuery string
	var gotBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/tags/search", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tags":[]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewCustomerEntityClient(CustomerEntityConfig{
		BaseURL:      srv.URL,
		TokenURL:     srv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	if _, err := client.SearchTags(context.Background(), []byte(reqBody)); err != nil {
		t.Fatalf("SearchTags: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/tags/search" {
		t.Errorf("path = %q, want /tags/search", gotPath)
	}
	if gotRawQuery != "" {
		t.Errorf("query string = %q, want empty (the query moved into the body)", gotRawQuery)
	}
	if string(gotBody) != reqBody {
		t.Errorf("body = %s, want %s", gotBody, reqBody)
	}
}

// TestResponseBodiesAreCapped verifies that an upstream body over the read cap
// is an error rather than an unbounded read, for both the JSON and binary
// paths, and that a body at the cap is still accepted.
func TestResponseBodiesAreCapped(t *testing.T) {
	t.Parallel()

	jsonSize := maxUpstreamResponseBytes + 1
	binSize := maxBinaryResponseBytes + 1
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/cases/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, jsonSize))
	})
	mux.HandleFunc("/attachments/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		if r.URL.Query().Get("ok") == "1" || len(r.URL.Path) > 0 && r.URL.Path[len(r.URL.Path)-1] == 'k' {
			_, _ = w.Write(make([]byte, maxBinaryResponseBytes))
			return
		}
		_, _ = w.Write(make([]byte, binSize))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewCustomerEntityClient(CustomerEntityConfig{
		BaseURL:      srv.URL,
		TokenURL:     srv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	if _, err := client.GetCase(context.Background(), "11111111-1111-1111-1111-111111111111"); err == nil {
		t.Error("oversized JSON body: expected an error")
	}
	if _, _, err := client.doBinary(context.Background(), "/attachments/x/content"); err == nil {
		t.Error("oversized binary body: expected an error")
	}
	body, _, err := client.doBinary(context.Background(), "/attachments/x/ok")
	if err != nil || len(body) != maxBinaryResponseBytes {
		t.Errorf("binary body at the cap: len=%d err=%v", len(body), err)
	}
}
