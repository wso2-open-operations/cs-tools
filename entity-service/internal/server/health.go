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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
)

// Timeouts for the health listener. Tighter than the main server's: a probe is
// a single small GET from an alerting system, and a slow or half-open client
// on this listener must never be able to tie it up — a health endpoint that
// stops answering reads to the alerting system as an outage.
const (
	healthServerReadTimeout  = 5 * time.Second
	healthServerWriteTimeout = 5 * time.Second
	healthServerIdleTimeout  = 30 * time.Second
)

// NewHealthServer builds the health listener: a second http.Server, on its own
// port, whose mux carries only the two health probes.
//
// It is a separate listener rather than another route or basePath on the main
// server because the two have different exposure. The main server is published
// at organization visibility; this one is published publicly, so external
// alerting can poll it without credentials (see .choreo/component.yaml). Given
// that difference, "which routes are publicly reachable" should be decided by
// which mux a handler is registered on — something that is true in this
// process and visible in this file — rather than by a gateway path rule that
// lives in another system and fails open if it is ever wrong. Nothing but the
// routes below is reachable on this port, whatever happens to that config.
//
// Pass a nil pool for a deployment with no database (DATA_SOURCE=servicenow);
// the database probe then reports it as not configured instead of down.
func NewHealthServer(addr string, db *pgxpool.Pool) *http.Server {
	// Converted explicitly rather than passed straight through: a nil
	// *pgxpool.Pool assigned to the handler.DBPinger interface would make
	// that interface non-nil (it would hold a nil pointer with a concrete
	// type), and the handler's own `h.db != nil` check would then call Ping
	// on a nil pool. Keep the untyped nil untyped.
	var pinger handler.DBPinger
	if db != nil {
		pinger = db
	}
	healthHandler := handler.NewHealthHandler(pinger)

	// Two probes, deliberately separate. Alerting needs to tell "the
	// component is down" apart from "the component is up but its database
	// is not" — a single combined endpoint collapses both into one 503 and
	// loses that distinction exactly when it matters most.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.HealthCheck)
	mux.HandleFunc("GET /health/database", healthHandler.DatabaseCheck)

	return &http.Server{
		Addr: addr,
		// Recovery only. The main chain's other middleware is either
		// irrelevant here (UserIDToken — this endpoint is unauthenticated
		// by design) or actively unwanted (Logger would write a line for
		// every probe, and alerting polls continuously). Recovery stays
		// because a panic on this listener would otherwise take down the
		// whole process, including the main API.
		Handler:      middleware.Recovery(mux),
		ReadTimeout:  healthServerReadTimeout,
		WriteTimeout: healthServerWriteTimeout,
		IdleTimeout:  healthServerIdleTimeout,
	}
}
