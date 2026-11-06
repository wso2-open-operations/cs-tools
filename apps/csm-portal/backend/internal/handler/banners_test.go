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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

func TestReadBannerSaveRejectsTrailingData(t *testing.T) {
	valid := `{"status":"published","sortOrder":0,"visibleInCustomerPortal":true,"visibleInCsmPortal":false,"html":"<p>Hi</p>"}`
	for _, body := range []string{valid + "]", valid + "}", valid + valid} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/portal-banners", strings.NewReader(body))
		req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{UserID: "user"}))
		if _, ok := readBannerSave(rec, req); ok || rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q accepted=%v status=%d", body, ok, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/portal-banners", strings.NewReader(valid))
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{UserID: "user"}))
	if _, ok := readBannerSave(rec, req); !ok {
		t.Fatalf("a save with no startsAt was refused: %d %s", rec.Code, rec.Body.String())
	}
}
