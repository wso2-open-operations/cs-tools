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

// TestGetGroupSendsGetToGroupsID pins the entity-service call shape for the
// group page opened from an approval stage: GET /groups/{id} (a "group" id,
// not the team lookup) with no body and the id escaped into the path.
func TestGetGroupSendsGetToGroupsID(t *testing.T) {
	t.Parallel()

	const id = "22222222-2222-4222-8222-222222222222"
	var gotMethod, gotPath string
	var gotBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/groups/", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + id + `","name":"CAB Approval","members":[],"total":0}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewCustomerEntityClient(CustomerEntityConfig{
		BaseURL:      srv.URL,
		TokenURL:     srv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	raw, err := client.GetGroup(context.Background(), id)
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/groups/"+id {
		t.Errorf("path = %q, want /groups/%s", gotPath, id)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %q, want none", gotBody)
	}
	if string(raw) == "" {
		t.Error("empty response")
	}
}

func TestNewCustomerEntityClientTimeout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		cfg  time.Duration
		want time.Duration
	}{
		{"unset uses default", 0, 60 * time.Second},
		{"configured value applied", 90 * time.Second, 90 * time.Second},
	} {
		c := NewCustomerEntityClient(CustomerEntityConfig{Timeout: tc.cfg})
		if c.http.Timeout != tc.want {
			t.Errorf("%s: client timeout = %v, want %v", tc.name, c.http.Timeout, tc.want)
		}
	}
}
