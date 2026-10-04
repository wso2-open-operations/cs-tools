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

package entityhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

// tokenServer serves both a token endpoint and an API endpoint, counting
// grants and recording the Authorization header the API saw.
func tokenServer(t *testing.T, grants *int32, apiStatus int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			atomic.AddInt32(grants, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600}`))
		default:
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(apiStatus)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
}

func TestSharedTransportGrantsOnceAcrossClients(t *testing.T) {
	var grants int32
	srv := tokenServer(t, &grants, http.StatusOK)
	defer srv.Close()

	rt, err := NewTransport(Credentials{TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		c, err := ClientFor(rt, Credentials{}, srv.URL, 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Do(context.Background(), c, srv.URL, "test", http.MethodGet, "/x", nil); err != nil {
			t.Fatalf("client %d: %v", i, err)
		}
	}
	if grants != 1 {
		t.Fatalf("three clients on one shared transport must share one grant, got %d", grants)
	}
}

func TestClientForWithoutSharedBuildsItsOwn(t *testing.T) {
	var grants int32
	srv := tokenServer(t, &grants, http.StatusOK)
	defer srv.Close()

	c, err := ClientFor(nil, Credentials{TokenURL: srv.URL + "/token"}, srv.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Do(context.Background(), c, srv.URL, "test", http.MethodPost, "/x", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if grants != 1 {
		t.Fatalf("want one grant, got %d", grants)
	}
}

func TestDoReturnsAPIErrorOnNon2xx(t *testing.T) {
	var grants int32
	srv := tokenServer(t, &grants, http.StatusServiceUnavailable)
	defer srv.Close()

	c, _ := ClientFor(nil, Credentials{TokenURL: srv.URL + "/token"}, srv.URL, time.Second)
	_, err := Do(context.Background(), c, srv.URL, "test", http.MethodGet, "/x", nil)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want *apierror.Error 503, got %v", err)
	}
}

func TestPageDone(t *testing.T) {
	cases := []struct {
		name                         string
		got, offset, total, pageSize int
		want                         bool
	}{
		{"empty page", 0, 50, 100, 50, true},
		{"full page, more to come", 50, 50, 120, 50, false},
		{"reached total", 20, 120, 120, 50, true},
		{"short non-final page keeps going", 30, 30, 120, 50, false},
		{"no total, full page keeps going", 50, 50, 0, 50, false},
		{"no total, short page ends", 10, 60, 0, 50, true},
	}
	for _, tc := range cases {
		if got := PageDone(tc.got, tc.offset, tc.total, tc.pageSize); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestInsecureURLsAreRejected(t *testing.T) {
	if _, err := NewTransport(Credentials{TokenURL: "http://example.invalid/token"}); err == nil {
		t.Error("a plaintext non-loopback token URL must be rejected")
	}
	rt, _ := NewTransport(Credentials{TokenURL: "https://example.invalid/token"})
	if _, err := ClientFor(rt, Credentials{}, "http://example.invalid", time.Second); err == nil {
		t.Error("a plaintext non-loopback base URL must be rejected")
	}
}
