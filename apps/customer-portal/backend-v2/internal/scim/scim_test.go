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

package scim

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestOrgForEmail(t *testing.T) {
	cases := []struct {
		email, want string
	}{
		{"jane.doe@wso2.com", "internal"},
		{"Jane.Doe@WSO2.COM", "internal"},
		{"jane.doe+tag@wso2.com", "internal"},
		{"jane.doe@wso2.com.evil.io", "external"},
		{"x@notwso2.com", "external"},
		{"a@gmail.com", "external"},
		{"", "external"},
	}
	for _, tc := range cases {
		if got := orgForEmail(tc.email); got != tc.want {
			t.Errorf("orgForEmail(%q) = %q, want %q", tc.email, got, tc.want)
		}
	}
}

// newTestClient returns a Client wired to an httptest server that serves both
// the token endpoint and the SCIM API, plus a getter for the recorded paths.
func newTestClient(t *testing.T) (*Client, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
			return
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		_, _ = w.Write([]byte(`{"totalResults":0,"itemsPerPage":0,"Resources":[]}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret"})
	return c, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), paths...)
	}
}

func TestSearchUserAndUpdateUserPhoneUseOrgByEmail(t *testing.T) {
	cases := []struct {
		email      string
		searchPath string
		patchPath  string
	}{
		{"jane.doe@wso2.com", "/organizations/internal/users/search", "/organizations/internal/users/u1"},
		{"jane.doe@example.com", "/organizations/external/users/search", "/organizations/external/users/u1"},
	}
	for _, tc := range cases {
		t.Run(tc.email, func(t *testing.T) {
			c, got := newTestClient(t)
			if _, err := c.SearchUser(context.Background(), tc.email); err != nil {
				t.Fatalf("SearchUser: %v", err)
			}
			if _, err := c.UpdateUserPhone(context.Background(), "u1", tc.email, "+10000000000"); err != nil {
				t.Fatalf("UpdateUserPhone: %v", err)
			}
			paths := got()
			if len(paths) != 2 || paths[0] != tc.searchPath || paths[1] != tc.patchPath {
				t.Errorf("paths = %v, want [%s %s]", paths, tc.searchPath, tc.patchPath)
			}
		})
	}
}
