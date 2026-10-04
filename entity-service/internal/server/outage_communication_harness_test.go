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

// A standing outage-communication endpoint, for the cross-module delivery run.
//
// entity-service and csm-scheduled-tasks are separate Go modules, so no single
// test can hold both sides. This serves the REAL handler over REAL HTTP against
// the REAL database and stays up, so the task's own package can be pointed at
// it and exercised end to end.
//
// It wraps the mux in an identity shim rather than the production auth
// middleware: that middleware needs a live identity-provider token and would be testing
// the identity provider, not this port. Everything below the transport is what production
// runs.
//
//	OUTAGE_COMM_SERVE_ADDR=127.0.0.1:9190 \
//	OUTAGE_COMM_TEST_DSN='postgres://…' \
//	go test ./internal/server/ -run TestServeOutageCommunicationHarness -timeout 30m
func TestServeOutageCommunicationHarness(t *testing.T) {
	addr := os.Getenv("OUTAGE_COMM_SERVE_ADDR")
	if addr == "" {
		t.Skip("OUTAGE_COMM_SERVE_ADDR not set; this is a harness, not a test")
	}
	dsn := os.Getenv("OUTAGE_COMM_TEST_DSN")
	if dsn == "" {
		t.Fatal("OUTAGE_COMM_TEST_DSN is required")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	const harnessClientID = "outage-comm-harness"
	accessSvc := service.NewAccessService(
		repository.NewAccessRepository(pool),
		map[string]bool{harnessClientID: true},
	)

	h := handler.NewOutageCommunicationHandler(
		service.NewOutageCommunicationService(
			repository.NewOutageCommunicationRepository(pool), accessSvc))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /outage-communications/sweep", h.SweepOutageCommunications)
	mux.HandleFunc("GET /outages/{number}/communication-log", h.GetOutageCommunicationLog)

	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.WithIdentity(r.Context(), auth.Identity{
			Validated: true, ClientID: harnessClientID,
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

	t.Logf("outage communication endpoints serving on %s", srv.URL)

	window := 300 * time.Second
	if v := os.Getenv("OUTAGE_COMM_SERVE_SECONDS"); v != "" {
		if d, err := time.ParseDuration(v + "s"); err == nil {
			window = d
		}
	}
	time.Sleep(window)
	t.Logf("harness window elapsed; shutting down")
}
