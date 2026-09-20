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
	"context"
	"encoding/json"
	"net/http"
)

// Status values reported by the health probes.
const (
	statusOK          = "ok"
	statusUnavailable = "unavailable"
)

// Database status values reported by DatabaseCheck.
const (
	dbStatusUp   = "up"
	dbStatusDown = "down"
	// dbStatusNotConfigured is reported when this deployment has no pool at
	// all — DATA_SOURCE=servicenow, where reads go through the ServiceNow
	// integration service and db.NewPoolIfNeeded deliberately returns no
	// pool. It is reported with a 200, not a 503: this probe exists to
	// alert on a Postgres outage, and a ServiceNow-mode deployment has no
	// Postgres to be out. Answering 503 there would alert continuously on a
	// database that is not supposed to exist. The distinct `database` value
	// is what still makes the difference visible to anyone reading the body.
	dbStatusNotConfigured = "not_configured"
)

// DBPinger is the one thing a database check needs from the connection pool.
// Narrow by design: it keeps this handler testable without a real database,
// and *pgxpool.Pool satisfies it as-is.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves the health probes on the health listener.
type HealthHandler struct {
	// db is nil when this deployment runs without a Postgres pool. Callers
	// must pass an untyped nil rather than a nil *pgxpool.Pool: a nil
	// pointer stored in an interface makes the interface itself non-nil,
	// so `db != nil` would pass and the probe would call Ping on a nil
	// pool. See NewHealthHandler's own call site in server.NewHealthServer.
	db DBPinger
}

// NewHealthHandler builds a HealthHandler. Pass a nil DBPinger for a
// deployment with no Postgres pool; the database check then reports the
// database as not configured rather than down.
func NewHealthHandler(db DBPinger) *HealthHandler {
	return &HealthHandler{db: db}
}

// HealthCheck handles GET /health. It always responds 200 with
// {"status":"ok"} and makes no dependency calls at all: it answers only
// "this component is up and serving". Keeping it dependency-free is what
// makes it safe as a liveness probe — a database outage must never get this
// instance restarted or pulled from rotation. The database is a separate
// probe, see DatabaseCheck.
//
// Registered on both listeners: the main service port (internal/server/
// routes.go) and the health port (internal/server/health.go).
func HealthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Alerting probes are polled continuously and often sit behind a CDN or
	// gateway; a cached 200 would keep reporting healthy through an actual
	// outage, which is the exact failure these endpoints exist to catch.
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": statusOK})
}

// DatabaseCheck handles GET /health/database on the health listener. It does a
// pooled round trip to Postgres and answers 503 when that fails, so a
// database outage is a usable alert condition on its own.
//
// Responds 200 with {"status":"ok","database":"up"} when the database
// answers, 503 with {"status":"unavailable","database":"down"} when the ping
// fails, and 200 with "not_configured" for a deployment that has no pool at
// all — that last one is not a failure, see dbStatusNotConfigured.
//
// The failure body carries no error detail — no driver message, host, or
// port. This endpoint is publicly reachable by design (external alerting has
// no credentials), so it reports only whether the dependency is up, never
// anything about the infrastructure behind it.
func (h *HealthHandler) DatabaseCheck(w http.ResponseWriter, r *http.Request) {
	dbStatus := dbStatusNotConfigured
	status := statusOK
	code := http.StatusOK

	// Only a deployment that actually has a pool can fail this probe: with
	// no pool there is no Postgres to be down (see dbStatusNotConfigured).
	if h.db != nil {
		if err := h.db.Ping(r.Context()); err != nil {
			dbStatus = dbStatusDown
			status = statusUnavailable
			code = http.StatusServiceUnavailable
		} else {
			dbStatus = dbStatusUp
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":   status,
		"database": dbStatus,
	})
}
