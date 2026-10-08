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

package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFileContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/repos/wso2/choreo/contents/.github/servicenow-config.yml":
			if got := r.Header.Get("Accept"); got != "application/vnd.github.raw+json" {
				t.Errorf("Accept = %q, want the raw media type", got)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer tok" {
				t.Errorf("Authorization = %q", got)
			}
			_, _ = w.Write([]byte("case:\n  account: x\n"))
		case "/repos/wso2/big/contents/f.yml":
			_, _ = w.Write([]byte(strings.Repeat("a", maxFileBytes+1)))
		case "/repos/wso2/locked/contents/f.yml":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Resource not accessible"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := NewClient(Config{BaseURL: srv.URL, Token: "tok"})
	ctx := context.Background()

	got, err := c.FileContent(ctx, "wso2", "choreo", ".github/servicenow-config.yml")
	if err != nil || string(got) != "case:\n  account: x\n" {
		t.Errorf("present file: got %q, %v", got, err)
	}
	got, err = c.FileContent(ctx, "wso2", "choreo", "missing.yml")
	if err != nil || got != nil {
		t.Errorf("a missing file is (nil, nil), got %q, %v", got, err)
	}
	if _, err := c.FileContent(ctx, "wso2", "big", "f.yml"); err == nil {
		t.Error("a file over the cap must be refused")
	}
	if _, err := c.FileContent(ctx, "wso2", "locked", "f.yml"); err == nil {
		t.Error("a 403 is a failure, not a missing file")
	}
}
