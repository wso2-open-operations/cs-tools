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

package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/emaildomains"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/handler"
)

// status sends method path through the service's routes. The handler has no
// generator (no key), so a registered AI route answers 503 not_configured
// and an unregistered one 404.
func status(t *testing.T, method, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	classifier := emaildomains.NewClassifier(emaildomains.NewLists(), net.DefaultResolver, time.Second, nil, "", 0)
	newMux(handler.NewDashboardHandler(nil), handler.NewEmailDomainHandler(classifier)).
		ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(`{"prompt":"p"}`)))
	return rec.Code
}

func TestRoutes(t *testing.T) {
	if got := status(t, http.MethodGet, "/health"); got != http.StatusOK {
		t.Errorf("/health = %d, want 200", got)
	}
	if got := status(t, http.MethodPost, "/ai/dashboards/generate"); got != http.StatusServiceUnavailable {
		t.Errorf("generate = %d, want 503 from the handler (registered, no key)", got)
	}
	if got := status(t, http.MethodPost, "/ai/email-domains/classify"); got != http.StatusBadRequest {
		t.Errorf("classify = %d, want 400 from the handler (registered, wrong body)", got)
	}
	if got := status(t, http.MethodPost, "/ai/dashboards/unknown"); got != http.StatusNotFound {
		t.Errorf("an unregistered path = %d, want 404", got)
	}
}
