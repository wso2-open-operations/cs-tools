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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListAlertMuteRules(t *testing.T) {
	t.Run("returns upstream response unchanged", func(t *testing.T) {
		upstream := `{"alertMuteRules":[{"id":"11111111-1111-1111-1111-111111111111","number":"AMR0000001","conditions":[{"field":"service","value":"svc"},{"field":"environment","value":"prod"}]}]}`
		h := NewAlertMuteRuleHandler(&mockEntityAlertMuteRuleClient{
			listActiveAlertMuteRulesFn: func(context.Context) ([]byte, error) { return []byte(upstream), nil },
		})
		w := httptest.NewRecorder()
		h.ListAlertMuteRules(w, httptest.NewRequest(http.MethodGet, "/alert-mute-rules?status=active", nil))
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if w.Body.String() != upstream {
			t.Errorf("body = %q, want upstream %q", w.Body.String(), upstream)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to list active alert mute rules.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				h := NewAlertMuteRuleHandler(&mockEntityAlertMuteRuleClient{
					listActiveAlertMuteRulesFn: func(context.Context) ([]byte, error) { return nil, tc.err },
				})
				w := httptest.NewRecorder()
				h.ListAlertMuteRules(w, httptest.NewRequest(http.MethodGet, "/alert-mute-rules?status=active", nil))
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})

	t.Run("only status=active is served", func(t *testing.T) {
		called := false
		h := NewAlertMuteRuleHandler(&mockEntityAlertMuteRuleClient{
			listActiveAlertMuteRulesFn: func(context.Context) ([]byte, error) { called = true; return []byte(`{}`), nil },
		})
		for _, target := range []string{"/alert-mute-rules", "/alert-mute-rules?status=expired", "/alert-mute-rules?status=Active"} {
			w := httptest.NewRecorder()
			h.ListAlertMuteRules(w, httptest.NewRequest(http.MethodGet, target, nil))
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, errMsgStatusActiveRequired)
		}
		if called {
			t.Fatal("entity service called for a status other than active")
		}
	})
}
