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
	"log/slog"
	"net/http"
)

// entityCloudStatusClient abstracts the entity service's public cloud status
// reads used by CloudStatusHandler.
type entityCloudStatusClient interface {
	GetCloudStatusMonitors(ctx context.Context, cloud string) ([]byte, error)
	GetCloudStatusIncidents(ctx context.Context, cloud string) ([]byte, error)
	GetCloudStatusAvailabilities(ctx context.Context, cloud string) ([]byte, error)
	GetCloudStatusAvailabilityHistory(ctx context.Context, cloud string) ([]byte, error)
	GetCloudStatusIncidentDetail(ctx context.Context, id, cloud string) ([]byte, error)
}

// CloudStatusHandler serves the public cloud status dashboard's reads,
// delegating to the entity service.
//
// The consumer is the public cloud status dashboard. It reaches this service
// the same way every other M2M consumer does -- through Choreo's API Manager
// gateway, which owns the trust boundary; see AccountHandler's doc comment.
//
// WHY THIS EXISTS AT ALL, rather than the dashboard calling entity-service
// directly: that service is not published to third parties, and the house
// pattern is that external consumers reach Postgres through this service.
// These handlers add no logic on purpose -- a pass-through that shaped or
// filtered anything would put dashboard behaviour in two repositories.
type CloudStatusHandler struct {
	entity entityCloudStatusClient
}

// writeCloudStatusResult wraps a body in the envelope the status dashboard
// has always received, and is the reason these endpoints are a drop-in
// replacement rather than a second integration.
//
// The status endpoints the dashboard was built against answer
//
//	{"result": {"code": 0, "message": "success", "cloud": "choreo", "data": …}}
//
// -- code/message/cloud/data wrapped in `result`. The dashboard checks
// `json?.result?.message == "success"` and then reads `json.result.data`.
//
// *** DROPPING THIS ENVELOPE IS WHAT FORCES A REWRITE DOWNSTREAM. *** An
// earlier version of these endpoints returned the bare data object, which
// looks cleaner and cost the dashboard a whole parallel controller, a
// per-call source selector and ~600 lines in a repository we do not own.
// Reproducing four fields here reduces that change to configuration: the
// same controller, pointed at a different host.
//
// It is deliberately added HERE and not in entity-service. This service is
// the compatibility façade for external consumers; entity-service stays a
// clean internal API that owes nothing to the legacy endpoints'
// conventions, and the envelope can be retired from this one layer once
// the dashboard no longer depends on it.
func writeCloudStatusResult(w http.ResponseWriter, cloud string, data []byte) {
	// data is already JSON from upstream, so it is spliced in rather than
	// decoded and re-encoded -- which also preserves the one thing a
	// round-trip would destroy: `availability` is a JSON string for six
	// clouds and a bare number for asgardeo.
	body, err := json.Marshal(map[string]any{
		"result": map[string]any{
			"code":    0,
			"message": "success",
			"cloud":   cloud,
			"data":    json.RawMessage(data),
		},
	})
	if err != nil {
		slog.Error("failed to wrap cloud status result", "cloud", cloud, "err", err)
		writeError(w, http.StatusInternalServerError, "Failed to encode cloud status response.")
		return
	}
	writeJSON(w, http.StatusOK, body)
}

// NewCloudStatusHandler creates a CloudStatusHandler backed by the given
// entity client.
func NewCloudStatusHandler(entity entityCloudStatusClient) *CloudStatusHandler {
	return &CloudStatusHandler{entity: entity}
}

// GetMonitors handles GET /cloud-status/monitors?cloud=… — the per-region,
// per-group monitor view the dashboard renders as its status page.
func (h *CloudStatusHandler) GetMonitors(w http.ResponseWriter, r *http.Request) {
	cloud := r.URL.Query().Get("cloud")
	result, err := h.entity.GetCloudStatusMonitors(r.Context(), cloud)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCloudStatusMonitors failed",
			"cloud", cloud, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to fetch cloud status monitors.")
		return
	}
	writeCloudStatusResult(w, cloud, result)
}

// GetIncidents handles GET /cloud-status/incidents?cloud=… — six months of
// incident history, one key per month whether or not it has incidents.
func (h *CloudStatusHandler) GetIncidents(w http.ResponseWriter, r *http.Request) {
	cloud := r.URL.Query().Get("cloud")
	result, err := h.entity.GetCloudStatusIncidents(r.Context(), cloud)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCloudStatusIncidents failed",
			"cloud", cloud, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to fetch cloud status incidents.")
		return
	}
	writeCloudStatusResult(w, cloud, result)
}

// GetAvailabilities handles GET /cloud-status/availabilities?cloud=… — the
// weighted uptime figure per region per window.
func (h *CloudStatusHandler) GetAvailabilities(w http.ResponseWriter, r *http.Request) {
	cloud := r.URL.Query().Get("cloud")
	result, err := h.entity.GetCloudStatusAvailabilities(r.Context(), cloud)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCloudStatusAvailabilities failed",
			"cloud", cloud, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to fetch cloud status availabilities.")
		return
	}
	writeCloudStatusResult(w, cloud, result)
}

// GetAvailabilityHistory handles GET /cloud-status/availability-history?cloud=…
// — the 90-day daily uptime chart, nested region → group → monitor.
func (h *CloudStatusHandler) GetAvailabilityHistory(w http.ResponseWriter, r *http.Request) {
	cloud := r.URL.Query().Get("cloud")
	result, err := h.entity.GetCloudStatusAvailabilityHistory(r.Context(), cloud)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCloudStatusAvailabilityHistory failed",
			"cloud", cloud, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to fetch cloud status availability history.")
		return
	}
	writeCloudStatusResult(w, cloud, result)
}

// GetIncidentDetail handles GET /cloud-status/incidents/{id}?cloud=… — one
// outage's public detail view.
//
// The 200 body has two shapes: the full detail, or an object carrying only
// `attachments` when the outage's incident does not qualify. Both are the
// upstream's, and forwarding raw bytes is what keeps them intact.
func (h *CloudStatusHandler) GetIncidentDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cloud := r.URL.Query().Get("cloud")
	result, err := h.entity.GetCloudStatusIncidentDetail(r.Context(), id, cloud)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCloudStatusIncidentDetail failed",
			"cloud", cloud, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to fetch cloud status incident detail.")
		return
	}
	writeCloudStatusResult(w, cloud, result)
}
