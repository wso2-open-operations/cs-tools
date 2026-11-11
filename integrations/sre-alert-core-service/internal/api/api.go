// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package api serves read-only views of alert-core's data to other CSM services.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"alert-core-service/internal/model"
)

const (
	defaultLimit = 20
	maxLimit     = 100
	// maxQueryLen caps a field-values search; condition values are at most 255 characters.
	maxQueryLen = 255
)

// ruleIDPattern accepts the UUIDs entity-service gives alert mute rules.
var ruleIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// MutedStore reads the incidents an alert mute rule muted.
type MutedStore interface {
	MutedByRule(ctx context.Context, ruleID string, limit, offset int) ([]model.Incident, int, error)
	MutedByRuleSummary(ctx context.Context, ruleID string) (map[string]int, error)
	FieldValues(ctx context.Context, field, query string, limit int) (values []string, ok bool, err error)
	MutedCounts(ctx context.Context, ruleIDs []string) (map[string]int, error)
}

// Handler serves the api routes.
type Handler struct {
	logger *slog.Logger
	store  MutedStore
}

// New returns a Handler reading from store.
func New(logger *slog.Logger, store MutedStore) *Handler {
	return &Handler{logger: logger, store: store}
}

// MutedIncident is one incident an alert mute rule muted, as the CSM portal lists it.
type MutedIncident struct {
	ID               int64      `json:"id"`
	Service          string     `json:"service"`
	Environment      string     `json:"environment"`
	Source           string     `json:"source"`
	MetricName       string     `json:"metricName"`
	Category         string     `json:"category"`
	Severity         string     `json:"severity"`
	UniqueIdentifier string     `json:"uniqueIdentifier"`
	AlertCount       int        `json:"alertCount"`
	FirstSeen        time.Time  `json:"firstSeen"`
	LastSeen         time.Time  `json:"lastSeen"`
	MutedAt          *time.Time `json:"mutedAt"`
	UnmutedAt        *time.Time `json:"unmutedAt"`
	// Outcome is muted, incident, closed_quietly or delivering (see model.Incident.MuteOutcome).
	Outcome string `json:"outcome"`
	// IncidentNumber is the CSM incident number once the outcome is incident; empty otherwise.
	IncidentNumber string `json:"incidentNumber"`
}

// MutedIncidentsResponse is one page of a rule's muted incidents plus counts by outcome over all of them.
type MutedIncidentsResponse struct {
	Items   []MutedIncident `json:"items"`
	Total   int             `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
	Summary map[string]int  `json:"summary"`
}

// MutedIncidents handles GET /alert-mute-rules/{id}/muted-incidents?limit=&offset=.
func (h *Handler) MutedIncidents(w http.ResponseWriter, r *http.Request) {
	ruleID := r.PathValue("id")
	if !ruleIDPattern.MatchString(ruleID) {
		writeError(w, http.StatusBadRequest, "invalid alert mute rule id")
		return
	}
	limit, ok := intParam(r, "limit", defaultLimit, 1, maxLimit)
	if !ok {
		writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
		return
	}
	offset, ok := intParam(r, "offset", 0, 0, 1<<30)
	if !ok {
		writeError(w, http.StatusBadRequest, "offset must be zero or more")
		return
	}
	incs, total, err := h.store.MutedByRule(r.Context(), ruleID, limit, offset)
	if err != nil {
		h.logger.Error("api: failed to list muted incidents", "alert_mute_rule_id", ruleID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list muted incidents")
		return
	}
	summary, err := h.store.MutedByRuleSummary(r.Context(), ruleID)
	if err != nil {
		h.logger.Error("api: failed to summarize muted incidents", "alert_mute_rule_id", ruleID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list muted incidents")
		return
	}
	resp := MutedIncidentsResponse{Items: make([]MutedIncident, 0, len(incs)), Total: total, Limit: limit, Offset: offset, Summary: summary}
	for _, inc := range incs {
		resp.Items = append(resp.Items, toMutedIncident(inc))
	}
	writeJSON(w, http.StatusOK, resp)
}

// MutedCountsResponse maps each requested rule id to how many incidents it muted (0 when none).
type MutedCountsResponse struct {
	Counts map[string]int `json:"counts"`
}

// MutedCounts handles GET /alert-mute-rules/muted-counts?ids=a,b,c: how many incidents each rule muted, for a page of
// up to 100 rules in one call.
func (h *Handler) MutedCounts(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ids")
	if raw == "" {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	ids := strings.Split(raw, ",")
	if len(ids) > maxLimit {
		writeError(w, http.StatusBadRequest, "at most 100 ids")
		return
	}
	for _, id := range ids {
		if !ruleIDPattern.MatchString(id) {
			writeError(w, http.StatusBadRequest, "invalid alert mute rule id")
			return
		}
	}
	counts, err := h.store.MutedCounts(r.Context(), ids)
	if err != nil {
		h.logger.Error("api: failed to count muted incidents", "rules", len(ids), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count muted incidents")
		return
	}
	resp := MutedCountsResponse{Counts: make(map[string]int, len(ids))}
	for _, id := range ids {
		resp.Counts[id] = counts[id]
	}
	writeJSON(w, http.StatusOK, resp)
}

// FieldValuesResponse lists the distinct values real alerts carry in one field.
type FieldValuesResponse struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

// FieldValues handles GET /alert-field-values?field=&q=&limit=: the values alerts actually carry in field (service,
// environment, source, metric_name, category, severity or unique_identifier) that contain q, most recently seen first.
// The CSM portal offers them when an engineer writes an alert mute rule, since a rule matches these strings.
func (h *Handler) FieldValues(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")
	query := r.URL.Query().Get("q")
	if len(query) > maxQueryLen {
		writeError(w, http.StatusBadRequest, "q must be at most 255 characters")
		return
	}
	limit, ok := intParam(r, "limit", defaultLimit, 1, maxLimit)
	if !ok {
		writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
		return
	}
	values, known, err := h.store.FieldValues(r.Context(), field, query, limit)
	if err != nil {
		h.logger.Error("api: failed to list field values", "field", field, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list field values")
		return
	}
	if !known {
		writeError(w, http.StatusBadRequest, "unknown field")
		return
	}
	if values == nil {
		values = []string{}
	}
	writeJSON(w, http.StatusOK, FieldValuesResponse{Field: field, Values: values})
}

func toMutedIncident(inc model.Incident) MutedIncident {
	m := MutedIncident{
		ID: inc.ID, Service: inc.Service, Environment: inc.Environment, Source: inc.Source, MetricName: inc.MetricName,
		Category: inc.Category, Severity: inc.SeverityLabel, UniqueIdentifier: inc.UniqueIdentifier, AlertCount: inc.AlertCount,
		FirstSeen: inc.FirstSeen, LastSeen: inc.LastSeen, MutedAt: inc.MutedAt, UnmutedAt: inc.UnmutedAt, Outcome: inc.MuteOutcome(),
	}
	if m.Outcome == model.MuteOutcomeIncident {
		m.IncidentNumber = inc.IncidentNumber
	}
	return m
}

// intParam reads query parameter name, defaulting to def when absent; ok is false when it is not an integer in [lo, hi].
func intParam(r *http.Request, name string, def, lo, hi int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < lo || v > hi {
		return 0, false
	}
	return v, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
