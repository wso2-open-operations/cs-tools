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
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// A standing cloud-status endpoint, for the cross-service delivery run.
//
// entity-service and csm-scheduled-tasks are separate Go modules, so no single
// test can hold both sides. This serves the REAL handler over REAL HTTP
// against the REAL database and stays up, so the task's own package — running
// in its own module — can be pointed at it and exercised end to end.
//
// It deliberately omits the auth middleware the production router wraps every
// route in. That middleware is not part of this feature and requires a live
// identity-provider token; including it would test the identity provider, not the cloud status port.
// Everything below the transport is exactly what production runs.
//
//	CLOUD_STATUS_SERVE_ADDR=127.0.0.1:9120 \
//	CLOUD_STATUS_TEST_DSN='postgres://…' \
//	CLOUD_STATUS_TEST_SERVICE_IDS='62d30e53-…' \
//	go test ./internal/server/ -run TestServeCloudStatusHarness -timeout 5m
func TestServeCloudStatusHarness(t *testing.T) {
	addr := os.Getenv("CLOUD_STATUS_SERVE_ADDR")
	if addr == "" {
		t.Skip("CLOUD_STATUS_SERVE_ADDR not set; this is a harness, not a test")
	}
	dsn := os.Getenv("CLOUD_STATUS_TEST_DSN")
	if dsn == "" {
		t.Fatal("CLOUD_STATUS_TEST_DSN is required")
	}
	scope := os.Getenv("CLOUD_STATUS_TEST_SERVICE_IDS")
	if scope == "" {
		t.Fatal("CLOUD_STATUS_TEST_SERVICE_IDS is required")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	h := handler.NewCloudStatusHandler(
		// Comma-separated, matching CLOUD_STATUS_SERVICE_IDS in the real
		// config. A single-element slice would make the harness quietly
		// narrower than production -- and the difference shows up exactly
		// where it matters, on an outage in a region the scope omits.
		service.NewCloudStatusService(repository.NewCloudStatusRepository(pool), splitScope(scope)),
	)

	mux := http.NewServeMux()
	dash := handler.NewCloudStatusDashboardHandler(
		service.NewCloudStatusDashboardService(repository.NewCloudStatusDashboardRepository(pool)),
	)
	mux.HandleFunc("GET /cloud-status/monitors", dash.Monitors)
	mux.HandleFunc("GET /cloud-status/incidents", dash.Incidents)
	mux.HandleFunc("GET /cloud-status/availabilities", dash.Availabilities)
	mux.HandleFunc("GET /cloud-status/availability-history", dash.AvailabilityHistory)
	mux.HandleFunc("GET /cloud-status/incidents/{id}", dash.IncidentDetail)

	mux.HandleFunc("POST /internal/cloud-status/sweep", h.Sweep)
	mux.HandleFunc("GET /internal/cloud-status/pending", h.Pending)
	mux.HandleFunc("POST /internal/cloud-status/{id}/delivery", h.RecordDelivery)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen on %s: %v", addr, err)
	}
	srv := httptest.NewUnstartedServer(mux)
	_ = srv.Listener.Close()
	srv.Listener = l
	srv.Start()
	defer srv.Close()

	t.Logf("cloud status endpoints serving on %s", srv.URL)

	// Stay up long enough for the other module's run, then shut down on its
	// own rather than needing to be killed.
	window := 90 * time.Second
	if v := os.Getenv("CLOUD_STATUS_SERVE_SECONDS"); v != "" {
		if d, err := time.ParseDuration(v + "s"); err == nil {
			window = d
		}
	}
	time.Sleep(window)
	t.Logf("harness window elapsed; shutting down")
}

// splitScope parses CLOUD_STATUS_TEST_SERVICE_IDS the way the service parses
// CLOUD_STATUS_SERVICE_IDS: comma-separated, blanks dropped.
func splitScope(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
