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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
)

func TestReadOnly_MarksRequestContext(t *testing.T) {
	var seen []bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, db.IsReadOnly(r.Context()))
		w.WriteHeader(http.StatusNoContent)
	})
	for _, tc := range []struct {
		name string
		h    http.Handler
		want bool
	}{
		{"unwrapped handler is not marked", inner, false},
		{"ReadOnly marks", ReadOnly(inner), true},
		{"ReadOnlyFunc marks", ReadOnlyFunc(inner), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen = nil
			rec := httptest.NewRecorder()
			tc.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
			if rec.Code != http.StatusNoContent {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
			}
			if len(seen) != 1 || seen[0] != tc.want {
				t.Errorf("handler saw read-only = %v, want %v", seen, tc.want)
			}
		})
	}
}

func TestReadOnly_DoesNotMutateOriginalRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	ReadOnly(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(httptest.NewRecorder(), req)
	if db.IsReadOnly(req.Context()) {
		t.Error("the caller's request context was marked")
	}
}
