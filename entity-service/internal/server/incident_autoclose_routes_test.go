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

const incidentAutoclosePath = "/incidents/autoclose"

// The sweep closes incidents across the whole database, so a caller without
// an internal client credential must be turned away before it runs.
func TestIncidentAutocloseRouteIsInternalOnly(t *testing.T) {
	router := newPortalWriteRouter(t, false)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, incidentAutoclosePath, strings.NewReader(`{"resolvedForDays":14}`)))
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "authorized internal client credential") {
		t.Errorf("POST %s = %d (%s), want 401 from the internalOnly gate", incidentAutoclosePath, rec.Code, strings.TrimSpace(rec.Body.String()))
	}
}

// On the ServiceNow data source (no pool) the route still exists and lets an
// internal caller through, but the unavailable stand-in answers 503 instead of
// reaching a repository with no database behind it. The 400 for an unknown
// field proves the request got past the gate to the handler, so the 503 is the
// service's and not the gate's.
func TestIncidentAutocloseRouteOnTheServiceNowDataSource(t *testing.T) {
	e := newSNRouteEnv(t)

	if rec := e.do(t, http.MethodPost, incidentAutoclosePath, `{"resolvedForDays":14,"days":14}`, snRouteM2MClient, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown field: status = %d (%s), want 400 from the handler", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	rec := e.do(t, http.MethodPost, incidentAutoclosePath, `{"resolvedForDays":14}`, snRouteM2MClient, "")
	if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "access scope") {
		t.Errorf("internal caller: status = %d (%s), want 503 from the unavailable service", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
}
