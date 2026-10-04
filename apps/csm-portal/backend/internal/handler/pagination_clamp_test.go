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

package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClampSearchPagination(t *testing.T) {
	run := func(body string) string {
		var got string
		h := ClampSearchPagination(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
			b, _ := io.ReadAll(r.Body)
			got = string(b)
		})
		h(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(body)))
		return got
	}
	pagination := func(t *testing.T, body string) map[string]json.Number {
		t.Helper()
		var req struct {
			Pagination map[string]json.Number `json:"pagination"`
		}
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("not JSON: %v (%s)", err, body)
		}
		return req.Pagination
	}

	t.Run("in-range values are forwarded byte for byte", func(t *testing.T) {
		const body = `{"filters":{"x":1},"pagination":{"limit":50,"offset":200}}`
		if got := run(body); got != body {
			t.Fatalf("body changed: %s", got)
		}
	})

	t.Run("oversized limit and offset are clamped, other fields kept", func(t *testing.T) {
		got := run(`{"filters":{"n":12345678901234567890},"pagination":{"limit":100000,"offset":999999999}}`)
		p := pagination(t, got)
		if p["limit"] != "100" || p["offset"] != "100000" {
			t.Fatalf("pagination = %v", p)
		}
		if !strings.Contains(got, `"n":12345678901234567890`) {
			t.Fatalf("other fields changed: %s", got)
		}
	})

	t.Run("negative and zero values are raised to the floor", func(t *testing.T) {
		p := pagination(t, run(`{"pagination":{"limit":0,"offset":-5}}`))
		if p["limit"] != "1" || p["offset"] != "0" {
			t.Fatalf("pagination = %v", p)
		}
	})

	t.Run("bodies without pagination or not objects pass through", func(t *testing.T) {
		for _, body := range []string{`{}`, `null`, `[]`, `not json`, `{"pagination":"x"}`, `{"pagination":{"limit":"ten"}}`} {
			if got := run(body); got != body {
				t.Fatalf("body %q changed to %q", body, got)
			}
		}
	})

	t.Run("an oversized body still reaches the handler's own size check", func(t *testing.T) {
		var gotErr error
		h := ClampSearchPagination(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
			_, gotErr = io.ReadAll(r.Body)
		})
		h(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+10))))
		if _, ok := gotErr.(*http.MaxBytesError); !ok {
			t.Fatalf("err = %v, want *http.MaxBytesError", gotErr)
		}
	})
}

func TestIsSearchRoute(t *testing.T) {
	for pattern, want := range map[string]bool{
		"POST /cases/search":                  true,
		"POST /cases/{id}/comments/search":    true,
		"GET /cases/search":                   false,
		"POST /cases":                         false,
		"POST /usage-metrics/projects/search": true,
	} {
		if got := IsSearchRoute(pattern); got != want {
			t.Errorf("IsSearchRoute(%q) = %v, want %v", pattern, got, want)
		}
	}
}
