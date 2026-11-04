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
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/legacy-product-api/internal/apierror"
)

// tokenServer returns an httptest.Server that always issues a client-credentials
// access token, so tests can exercise Client without a real OAuth2 provider.
func tokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"bearer","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, upstream *httptest.Server) *Client {
	t.Helper()
	return NewClient(Config{
		BaseURL:      upstream.URL,
		TokenURL:     tokenServer(t).URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})
}

// TestTokenFetchTimeout verifies that a stalled token endpoint fails requests
// within the configured timeout rather than blocking indefinitely.
func TestTokenFetchTimeout(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer tokenSrv.Close()

	tokenFetchTimeout = 100 * time.Millisecond
	t.Cleanup(func() { tokenFetchTimeout = 10 * time.Second })

	client := NewClient(Config{
		BaseURL:      tokenSrv.URL,
		TokenURL:     tokenSrv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	start := time.Now()
	_, err := client.ListCarbonKernelReleases(context.Background())
	if err == nil {
		t.Fatal("expected error from hung token server, got nil")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("token fetch took %v; expected <2s with 100ms timeout", elapsed)
	}
}

// TestDoUpstreamError verifies a non-2xx answer becomes a typed *apierror.Error.
func TestDoUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"down"}`))
	}))
	defer upstream.Close()

	_, err := newTestClient(t, upstream).ListCarbonPlatformHistory(context.Background())
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want *apierror.Error 503", err)
	}
}

// TestDoForwardsCorrelationID verifies the correlation ID reaches entity-service.
func TestDoForwardsCorrelationID(t *testing.T) {
	var got string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-CSM-Correlation-ID")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	ctx := WithCorrelationID(context.Background(), "lpa-123")
	if _, err := newTestClient(t, upstream).ListCarbonKernelReleases(ctx); err != nil {
		t.Fatal(err)
	}
	if got != "lpa-123" {
		t.Errorf("X-CSM-Correlation-ID = %q", got)
	}
}

// TestReadRequestURIs pins the exact GET URIs, including when the optional
// query parameter is omitted.
func TestReadRequestURIs(t *testing.T) {
	var got []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		got = append(got, r.URL.RequestURI())
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	client := newTestClient(t, upstream)
	ctx := context.Background()
	for _, call := range []func() ([]byte, error){
		func() ([]byte, error) { return client.ListReleaseLines(ctx, false) },
		func() ([]byte, error) { return client.ListReleaseLines(ctx, true) },
		func() ([]byte, error) { return client.ListCarbonPlatformHistory(ctx) },
		func() ([]byte, error) { return client.ListCarbonKernelReleases(ctx) },
	} {
		if _, err := call(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	want := []string{"/release-lines", "/release-lines?includeRetired=true", "/carbon-platform-history", "/carbon-kernel-releases"}
	if len(got) != len(want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("request %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestListProductVersionsPages verifies the request body and that pages are
// followed until hasMore is false, rows kept in order.
func TestListProductVersionsPages(t *testing.T) {
	var reqs []productVersionsSearch
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/product-versions/search" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		var req productVersionsSearch
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("body %s: %v", raw, err)
		}
		reqs = append(reqs, req)
		// Two rows a page, three rows in all.
		var rows []string
		for i := req.Pagination.Offset; i < req.Pagination.Offset+2 && i < 3; i++ {
			rows = append(rows, `{"id":"`+strconv.Itoa(i)+`"}`)
		}
		more := req.Pagination.Offset+len(rows) < 3
		body := `{"productVersions":[` + join(rows) + `],"hasMore":` + strconv.FormatBool(more) + `}`
		_, _ = w.Write([]byte(body))
	}))
	defer upstream.Close()

	got, err := newTestClient(t, upstream).ListProductVersions(context.Background(), "support-matrix")
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"productVersions":[{"id":"0"},{"id":"1"},{"id":"2"}]}`; string(got) != want {
		t.Errorf("body = %s, want %s", got, want)
	}
	if len(reqs) != 2 || reqs[0].Pagination.Offset != 0 || reqs[1].Pagination.Offset != 2 {
		t.Fatalf("requests = %+v", reqs)
	}
	for _, r := range reqs {
		if r.Feed != "support-matrix" || !r.ExcludeRetired || r.Pagination.Limit != productVersionsPageSize {
			t.Errorf("request = %+v", r)
		}
	}
}

// TestListProductVersionsEmpty verifies an empty feed is an empty list, not null.
func TestListProductVersionsEmpty(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"productVersions":[],"hasMore":false}`))
	}))
	defer upstream.Close()

	got, err := newTestClient(t, upstream).ListProductVersions(context.Background(), "")
	if err != nil || string(got) != `{"productVersions":[]}` {
		t.Fatalf("got %s, %v", got, err)
	}
}

func join(rows []string) string {
	out := ""
	for i, r := range rows {
		if i > 0 {
			out += ","
		}
		out += r
	}
	return out
}
