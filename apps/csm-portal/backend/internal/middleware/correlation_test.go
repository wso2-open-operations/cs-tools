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

package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCorrelationID(t *testing.T) {
	run := func(supplied string, set bool) (echoed, inContext string) {
		h := CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inContext = CorrelationIDFromContext(r.Context())
		}))
		r := httptest.NewRequest(http.MethodGet, "/x", nil)
		if set {
			r.Header.Set(correlationIDHeader, supplied)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Header().Get(correlationIDHeader), inContext
	}

	t.Run("keeps a well-formed supplied id", func(t *testing.T) {
		for _, id := range []string{"11111111-2222-4333-8444-555555555555", "abc", strings.Repeat("a", 64)} {
			echoed, ctx := run(id, true)
			if echoed != id || ctx != id {
				t.Fatalf("id %q: echoed %q, context %q", id, echoed, ctx)
			}
		}
	})

	t.Run("generates one when absent", func(t *testing.T) {
		echoed, ctx := run("", false)
		if !validCorrelationID.MatchString(echoed) || echoed != ctx {
			t.Fatalf("generated id %q / %q", echoed, ctx)
		}
	})

	t.Run("replaces a malformed or oversized id", func(t *testing.T) {
		for _, bad := range []string{strings.Repeat("a", 65), "has space", "semi;colon", "<script>", "a/b", "ünïcode"} {
			echoed, ctx := run(bad, true)
			if echoed == bad || ctx == bad {
				t.Fatalf("malformed id %q was kept", bad)
			}
			if !validCorrelationID.MatchString(echoed) || echoed != ctx {
				t.Fatalf("replacement for %q is %q / %q", bad, echoed, ctx)
			}
		}
	})
}
