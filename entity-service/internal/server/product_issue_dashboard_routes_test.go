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
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

var productIssueDashboardRoutes = []string{
	"/product-work-items/dashboard",
	"/reports/product-issues",
	"/reports/product-issues/recipients",
}

// TestProductIssueDashboardRoutesAreInternalOnly pins that each route is registered
// wrapped in internalOnly, and that a customer caller is refused with 403 before the
// service is reached (the handler here has no service: reaching it would panic).
func TestProductIssueDashboardRoutesAreInternalOnly(t *testing.T) {
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	for _, path := range productIssueDashboardRoutes {
		re := regexp.MustCompile(`HandleFunc\("GET ` + regexp.QuoteMeta(path) + `",\s*([^\n]+)`)
		m := re.FindAllStringSubmatch(string(src), -1)
		if len(m) != 1 {
			t.Fatalf("GET %s is registered %d times in routes.go, want 1", path, len(m))
		}
		if !strings.HasPrefix(strings.TrimSpace(m[0][1]), "internalOnly(") {
			t.Errorf("GET %s must be wrapped in internalOnly, got %q", path, strings.TrimSpace(m[0][1]))
		}
	}

	h := handler.NewProductIssueDashboardHandler(nil)
	customer := fakeAccess{scope: service.AccessScope{ViewerEmail: "customer@example.com"}}
	for path, fn := range map[string]http.HandlerFunc{
		"/product-work-items/dashboard":      h.GetDashboard,
		"/reports/product-issues":            h.GetReport,
		"/reports/product-issues/recipients": h.GetRecipients,
	} {
		rec := httptest.NewRecorder()
		internalOnly(customer, fn)(rec, httptest.NewRequest(http.MethodGet, path+"?period=week", nil))
		if rec.Code != http.StatusForbidden {
			t.Errorf("customer GET %s = %d, want 403", path, rec.Code)
		}
	}
}

// TestProductIssueDashboardPatternsDoNotConflict registers the routes next to the
// id-addressed one: ServeMux panics at registration on an ambiguous pair.
func TestProductIssueDashboardPatternsDoNotConflict(t *testing.T) {
	mux := http.NewServeMux()
	noop := func(http.ResponseWriter, *http.Request) {}
	for _, p := range []string{
		"GET /product-work-items/{id}", "POST /product-work-items/search", "POST /product-work-items/{id}/close",
		"GET /product-work-items/dashboard", "GET /reports/product-issues", "GET /reports/product-issues/recipients",
		"GET /reports/sla-breaches",
	} {
		mux.HandleFunc(p, noop)
	}
}
