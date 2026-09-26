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

// azurePayload is Azure Monitor's own common-alert-schema webhook body,
// modeled to only the fields this adapter actually reads — not an
// exhaustive schema of everything Azure Monitor can send.
//
// Data and Essentials are pointers, not value structs, so their absence is
// detectable after json.Unmarshal (nil) rather than silently unmarshaling
// into a zero-value struct — see mapAzurePayload's explicit nil check
// immediately below. A value struct would make `{}` and a real payload
// indistinguishable, letting every AlertRequest.validate-satisfying default
// this adapter applies (severity "ok", service "Managed Services", etc.)
// enqueue a misleading alert instead of rejecting the malformed input.
type azurePayload struct {
	Data *azureData `json:"data"`
}

type azureData struct {
	Essentials   *azureEssentials `json:"essentials"`
	AlertContext json.RawMessage  `json:"alertContext"`
}

type azureEssentials struct {
	AlertID           string `json:"alertId"`
	AlertRule         string `json:"alertRule"`
	Severity          string `json:"severity"`
	MonitorCondition  string `json:"monitorCondition"`
	MonitoringService string `json:"monitoringService"`
}

// azureSeverityTable maps Azure Monitor's own Sev0-Sev4 vocabulary onto
// this service's five-value severity vocabulary (critical/major/minor/
// warning/ok — see internal/severity.MapImpactUrgency's doc comment for why
// every adapter must target that exact set). Most to least severe, matching
// Azure's own convention (Sev0 is the most severe).
var azureSeverityTable = map[string]string{
	"sev0": "critical",
	"sev1": "major",
	"sev2": "minor",
	"sev3": "warning",
	"sev4": "ok",
}

// azureDefaultService is used when essentials.monitoringService is absent —
// Azure Monitor payloads don't always populate it, and AlertRequest.validate
// requires Service non-empty. This exact fallback string matches the prior
// ServiceNow-based pipeline's own default for the identical gap, kept
// verbatim rather than inventing a new one.
const azureDefaultService = "Managed Services"

// azureDefaultMetricName is used when essentials.alertRule is also absent
// (rare, but AlertRequest.validate requires MetricName non-empty) — falls
// back to whatever monitoringService is available, or a fixed placeholder
// if even that is missing, so this adapter always has something non-empty
// to offer.
const azureDefaultMetricName = "Azure Monitor alert"

// mapAzurePayload parses an Azure Monitor common-alert-schema webhook body
// into this service's own AlertRequest. Pure and independently testable,
// separate from the thin HTTP handler wrapping it (CreateAlertFromAzure
// below) — mirrors the CreateAlert/enqueueAlert separation in alerts.go.
func mapAzurePayload(body []byte) (AlertRequest, error) {
	var p azurePayload
	if err := json.Unmarshal(body, &p); err != nil {
		return AlertRequest{}, fmt.Errorf("azure: %w", err)
	}
	if p.Data == nil || p.Data.Essentials == nil {
		return AlertRequest{}, fmt.Errorf("azure: missing required data.essentials")
	}
	ess := *p.Data.Essentials

	severity := "ok"
	if s, ok := azureSeverityTable[strings.ToLower(strings.TrimSpace(ess.Severity))]; ok {
		severity = s
	}
	// Override: a resolved condition is never still "critical" (or any
	// other active severity) just because the original alert was — matches
	// how the prior ServiceNow pipeline's other Azure-handling flow already
	// treated a resolved condition for this same vendor.
	if strings.EqualFold(strings.TrimSpace(ess.MonitorCondition), "Resolved") {
		severity = "ok"
	}

	service := ess.MonitoringService
	if service == "" {
		service = azureDefaultService
	}

	metricName := ess.AlertRule
	if metricName == "" {
		metricName = ess.MonitoringService
	}
	if metricName == "" {
		metricName = azureDefaultMetricName
	}

	return AlertRequest{
		Source:           "azure",
		Severity:         severity,
		Service:          service,
		MetricName:       metricName,
		UniqueIdentifier: ess.AlertID,
		Description:      renderAzureDescription(p),
	}, nil
}

// renderAzureDescription flattens alertContext into a readable plain-text
// summary. AlertRequest.Description is not HTML (unlike the prior
// ServiceNow-based pipeline's own rendering) — a flat key:value listing is
// sufficient context for an engineer, no HTML formatting attempted.
// Callers must only invoke this after confirming p.Data.Essentials is
// non-nil (mapAzurePayload's own nil check covers this).
func renderAzureDescription(p azurePayload) string {
	ess := *p.Data.Essentials
	var b strings.Builder
	fmt.Fprintf(&b, "Azure Monitor alert: %s\n", ess.AlertRule)
	fmt.Fprintf(&b, "Monitor condition: %s\n", ess.MonitorCondition)
	if ess.MonitoringService != "" {
		fmt.Fprintf(&b, "Monitoring service: %s\n", ess.MonitoringService)
	}
	if len(p.Data.AlertContext) > 0 && string(p.Data.AlertContext) != "null" {
		fmt.Fprintf(&b, "Alert context: %s", flattenJSON(p.Data.AlertContext))
	}
	return strings.TrimRight(b.String(), "\n")
}

// flattenJSON renders an arbitrary JSON value as a stable, readable
// "key=value, key=value" listing for embedding in Description. Falls back
// to the raw bytes if the value isn't a JSON object (e.g. an array or
// scalar) since alertContext's shape is genuinely vendor/alert-type
// specific and not worth modeling exhaustively here.
func flattenJSON(raw json.RawMessage) string {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return string(raw)
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, obj[k]))
	}
	return strings.Join(parts, ", ")
}

// CreateAlertFromAzure handles POST /alerts/adapters/azure: translates an
// Azure Monitor common-alert-schema webhook into this service's generic
// AlertRequest, then reuses enqueueAlert unchanged — same
// validation/buffering/worker/grouping/dedup/escalation path as POST
// /alerts. See this file's mapAzurePayload for the mapping rules.
//
// Azure Monitor gets its own dedicated route (as does every other vendor
// added in this batch) rather than being shape-sniffed within one shared
// endpoint — a deliberate improvement over the prior ServiceNow-based
// pipeline, which ambiguously branched on payload shape within a single
// endpoint to tell Azure and Site24x7 apart. A dedicated route per vendor
// shape is simpler to reason about, route, and test.
func (h *AlertHandler) CreateAlertFromAzure(w http.ResponseWriter, r *http.Request) {
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

	req, err := mapAzurePayload(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !h.requireAuthenticatedSource(w, r, req.Source) {
		return
	}

	id, alertNumber, err := h.enqueueAlert(r.Context(), req)
	h.writeEnqueueResult(w, r, id, alertNumber, err)
}
