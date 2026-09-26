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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// grafanaPayload is Grafana's own native alert-webhook body, modeled to
// only the fields this adapter reads.
type grafanaPayload struct {
	State string `json:"state"`
	Title string `json:"title"`
	Tags  struct {
		Service  string `json:"service"`
		Severity string `json:"severity"`
	} `json:"tags"`
}

// grafanaSeverityTable maps this vendor's tags.severity convention ('1'
// through '4') onto this service's five-value vocabulary. Anything else,
// including missing, maps to "ok" — matching what the prior ServiceNow-based
// pipeline already did for its own "other/missing" case, so there's no
// deviation to flag here (unlike Azure/Site24x7/OpenSearch above).
var grafanaSeverityTable = map[string]string{
	"1": "critical",
	"2": "major",
	"3": "minor",
	"4": "warning",
}

// grafanaDefaultService is used only when tags.service is empty —
// AlertRequest.validate requires Service non-empty. When tags.service is
// set, it is passed through as-is (see mapGrafanaPayload): unlike the prior
// ServiceNow-based pipeline, which only honored this field when it
// case-insensitively equaled "CHOREO" (tied to SN's own sys_id-lookup
// routing table) and silently dropped it otherwise, AlertRequest.Service is
// a free-text field here with no equivalent routing table to satisfy, so
// whatever the vendor sent is used directly.
const grafanaDefaultService = "Grafana Monitoring"

// mapGrafanaPayload parses a Grafana native alert-webhook body into
// (req, ok, err). ok is false when state isn't "alerting" — the caller
// (CreateAlertFromGrafana) responds 200 in that case rather than treating it
// as an error, matching the prior ServiceNow-based pipeline's own filter
// (a non-alerting Grafana webhook genuinely isn't an incident-creation
// trigger), which is worth keeping here.
func mapGrafanaPayload(body []byte) (req AlertRequest, ok bool, err error) {
	var p grafanaPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return AlertRequest{}, false, fmt.Errorf("grafana: %w", err)
	}

	if p.State != "alerting" {
		return AlertRequest{}, false, nil
	}

	severity := "ok"
	if s, ok := grafanaSeverityTable[p.Tags.Severity]; ok {
		severity = s
	}

	service := p.Tags.Service
	if service == "" {
		service = grafanaDefaultService
	}

	return AlertRequest{
		Source:      "grafana",
		Severity:    severity,
		Service:     service,
		MetricName:  p.Title,
		Description: renderGrafanaDescription(p),
		// No UniqueIdentifier: this payload shape carries nothing suitable
		// for cross-alert grouping.
	}, true, nil
}

// renderGrafanaDescription dumps the full raw payload into a readable
// summary — matches the prior ServiceNow-based pipeline's own behavior of
// putting the raw payload into a journal comment, for the same reason: this
// adapter's own mapped fields are lossy, so give the engineer the full
// context Grafana actually sent.
func renderGrafanaDescription(p grafanaPayload) string {
	fields := map[string]any{
		"state":         p.State,
		"title":         p.Title,
		"tags.service":  p.Tags.Service,
		"tags.severity": p.Tags.Severity,
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, fields[k]))
	}
	return "Grafana alert: " + strings.Join(parts, ", ")
}

// CreateAlertFromGrafana handles POST /alerts/adapters/grafana: translates
// a Grafana native alert-webhook payload into this service's generic
// AlertRequest, then reuses enqueueAlert unchanged. A non-"alerting" state
// gets a 200 with a small acknowledgment body, never a 400 — see
// mapGrafanaPayload's doc comment.
func (h *AlertHandler) CreateAlertFromGrafana(w http.ResponseWriter, r *http.Request) {
	// Checked first, before reading or parsing the body at all: this
	// adapter's Source is the fixed literal "grafana", not derived from the
	// payload, so authorization never depends on payload content. Doing
	// this after the ignored-payload short-circuit below let a caller
	// authenticated as a different source still get a 200 for a
	// non-"alerting" state instead of the 403 the mismatch warrants.
	if !h.requireAuthenticatedSource(w, r, "grafana") {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	req, ok, err := mapGrafanaPayload(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "state is not \"alerting\""})
		return
	}

	id, alertNumber, err := h.enqueueAlert(r.Context(), req)
	h.writeEnqueueResult(w, r, id, alertNumber, err)
}
