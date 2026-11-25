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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// The ingest routes write registrations and set a company's owner user id, so
// only a service credential may call them: a person, internal or not, gets 403
// from the mux itself, before any handler runs.
func TestPlgIngestRoutesRefuseAPerson(t *testing.T) {
	mux := http.NewServeMux()
	// No pool: a refused request never reaches a handler, and an accepted one would fail loudly.
	registerPLGRoutes(mux, nil, fakeAccess{scope: service.AccessScope{Unrestricted: true}})

	for _, route := range []string{"/plg/registrations/ingest", "/plg/ingest-failures"} {
		for _, user := range []string{"engineer@example.com", "customer@example.com"} {
			req := httptest.NewRequest(http.MethodPost, route, strings.NewReader(`{}`))
			req = req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Validated: true, UserEmail: user}))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("POST %s as a person: status %d, want 403", route, rec.Code)
			}
		}
	}
}
