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

package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/middleware"
)

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	handler := middleware.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	cases := []struct {
		header string
		want   string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"Content-Security-Policy", "upgrade-insecure-requests"},
		{"Strict-Transport-Security", "max-age=31536000; includeSubDomains"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "no-referrer"},
	}
	for _, tc := range cases {
		if got := w.Header().Get(tc.header); got != tc.want {
			t.Errorf("%s = %q, want %q", tc.header, got, tc.want)
		}
	}
}
