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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAppConfig(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewAppConfigHandler("https://github.com/example/example-repo/issues/new")
		r := httptest.NewRequest(http.MethodGet, "/app-config", nil)
		w := httptest.NewRecorder()
		h.GetAppConfig(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("returns the configured report-issue URL", func(t *testing.T) {
		h := NewAppConfigHandler("https://github.com/example/example-repo/issues/new")
		r := withUser(httptest.NewRequest(http.MethodGet, "/app-config", nil))
		w := httptest.NewRecorder()
		h.GetAppConfig(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		resp := decodeJSON[appConfigResponse](t, w)
		if resp.ReportIssueURL != "https://github.com/example/example-repo/issues/new" {
			t.Errorf("ReportIssueURL = %q, want the configured URL", resp.ReportIssueURL)
		}
	})

	t.Run("returns an empty string, never omitted, when unset", func(t *testing.T) {
		h := NewAppConfigHandler("")
		r := withUser(httptest.NewRequest(http.MethodGet, "/app-config", nil))
		w := httptest.NewRecorder()
		h.GetAppConfig(w, r)

		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[appConfigResponse](t, w)
		if resp.ReportIssueURL != "" {
			t.Errorf("ReportIssueURL = %q, want empty", resp.ReportIssueURL)
		}
	})
}
