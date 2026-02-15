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

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/log"
)

func TestHandler_Authenticate(t *testing.T) {
	tests := []struct {
		name           string
		cfgKey         string
		requestKey     string
		expectedStatus int
		expectedResult bool
	}{
		{
			name:           "No key configured",
			cfgKey:         "",
			requestKey:     "",
			expectedStatus: http.StatusOK,
			expectedResult: true,
		},
		{
			name:           "Valid key provided",
			cfgKey:         "secret-key",
			requestKey:     "secret-key",
			expectedStatus: http.StatusOK,
			expectedResult: true,
		},
		{
			name:           "Invalid key provided",
			cfgKey:         "secret-key",
			requestKey:     "wrong-key",
			expectedStatus: http.StatusUnauthorized,
			expectedResult: false,
		},
		{
			name:           "Missing key provided",
			cfgKey:         "secret-key",
			requestKey:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedResult: false,
		},
	}

	logger := log.NewAppLogger("INFO")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{HookAPIKey: tt.cfgKey}
			h := &Handler{cfg: cfg, logger: logger}

			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			if tt.requestKey != "" {
				req.Header.Set("API-Key", tt.requestKey)
			}
			w := httptest.NewRecorder()

			got := h.authenticate(req, w)

			if got != tt.expectedResult {
				t.Errorf("authenticate() = %v, want %v", got, tt.expectedResult)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
