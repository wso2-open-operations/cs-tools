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

package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// newTestSNClient builds an *integrationservice.Client backed by an httptest
// server: apiHandler serves every path other than the token endpoint, which is
// stubbed to always issue a fake bearer token. The server is closed
// automatically via t.Cleanup.
func newTestSNClient(t *testing.T, apiHandler http.Handler) *integrationservice.Client {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-token",
			"expires_in":   3600,
		})
	})
	mux.Handle("/", apiHandler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return integrationservice.New(srv.URL, integrationservice.ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth2/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})
}
