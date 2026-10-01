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
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// TestServeQueryHoursReportHarness serves the real GET
// /query-hours/weekly-report against a real database, so the weekly report
// sub-cron can be exercised end to end without deploying anything.
//
// It is a HARNESS, not a test: skipped unless QH_REPORT_SERVE_ADDR is set, so
// an ordinary `go test ./...` never opens a socket or touches a remote DB.
//
// It omits the auth middleware the production router wraps every route in and
// injects the equivalent identity instead — the same shim the outage
// notification harness uses, and for the same reason: the endpoint is
// internal-only, so the real service code refuses a caller it cannot resolve
// a scope for.
//
// *** THIS SERVES REAL CUSTOMER DATA. *** The report names accounts,
// opportunities and projects, and carries account managers' and technical
// owners' email addresses. Bind it to localhost and nothing else.
func TestServeQueryHoursReportHarness(t *testing.T) {
	addr := os.Getenv("QH_REPORT_SERVE_ADDR")
	if addr == "" {
		t.Skip("QH_REPORT_SERVE_ADDR not set; this is a harness, not a test")
	}
	dsn := os.Getenv("QH_REPORT_TEST_DSN")
	if dsn == "" {
		t.Skip("QH_REPORT_TEST_DSN not set")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	const harnessClientID = "qh-report-harness"
	accessSvc := service.NewAccessService(repository.NewAccessRepository(pool),
		map[string]bool{harnessClientID: true})

	// notifier and publisher are nil: GetWeeklyReport is a pure read and
	// touches neither. A nil here would panic on the recompute paths, which
	// this harness does not route.
	h := handler.NewQueryHourHandler(
		service.NewQueryHourService(
			repository.NewQueryHourRepository(pool),
			nil, nil, accessSvc, false,
		))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /query-hours/weekly-report", h.GetWeeklyReport)

	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.WithIdentity(r.Context(), auth.Identity{
			Validated: true,
			ClientID:  harnessClientID,
		})
		mux.ServeHTTP(w, r.WithContext(ctx))
	})

	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen on %s: %v", addr, err)
	}
	srv := httptest.NewUnstartedServer(authed)
	_ = srv.Listener.Close()
	srv.Listener = l
	srv.Start()
	defer srv.Close()

	t.Logf("weekly report endpoint serving on %s", srv.URL)

	window := 5 * time.Minute
	if v := os.Getenv("QH_REPORT_SERVE_SECONDS"); v != "" {
		if d, err := time.ParseDuration(v + "s"); err == nil {
			window = d
		}
	}
	time.Sleep(window)
	t.Logf("harness window elapsed; shutting down")
}
