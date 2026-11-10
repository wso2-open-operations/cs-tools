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

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The registration trend is a PLG route: POST only, and a bad request is
// refused by the service's own window check before any query (so no
// database is needed here).
func TestPLGRegistrationVolumeTrendRoute(t *testing.T) {
	mux := http.NewServeMux()
	registerPLGRoutes(mux, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/plg/registrations/volume-trend",
		strings.NewReader(`{"bucket":"month"}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "bucket needs startDate and endDate") {
		t.Errorf("POST with a bucket and no dates = %d (%s), want 400 from the window check", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/plg/registrations/volume-trend", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET = %d, want 405", rec.Code)
	}
}

// The owner aggregate is a PLG route too: a groupBy other than "owner" is
// refused by the service before any query.
func TestPLGOrganizationAggregateRoute(t *testing.T) {
	mux := http.NewServeMux()
	registerPLGRoutes(mux, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/plg/organizations/aggregate",
		strings.NewReader(`{"groupBy":"lifecycleStage"}`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `groupBy must be \"owner\"`) {
		t.Errorf("POST with groupBy lifecycleStage = %d (%s), want 400 from the groupBy check", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
}
