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
)

// choreoDPPayload is an internal alert-forwarder's own native alert body,
// modeled to only the fields this adapter reads.
//
// Unlike every other adapter in this package, severity/impact/urgency here
// are raw integers, not this service's own string severity vocabulary — see
// choreoDPSeverityTable and mapChoreoDPPayload below for how each is
// translated. "status" is deliberately not modeled at all: this source
// always sends "New" (ServiceNow incidents already start in that state,
// so the field carries no information this service needs), and there is
// nothing else to branch on the way Site24x7/Grafana branch on their own
// status/state fields.
type choreoDPPayload struct {
	Severity    int    `json:"severity"`
	Impact      int    `json:"impact"`
	Urgency     int    `json:"urgency"`
	Service     string `json:"service"`
	MetricName  string `json:"metric_name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Environment string `json:"environment"`
	Source      string `json:"source"`
}

// choreoDPSeverityTable maps this source's own numeric severity (1=High,
// 2=Medium, 3=Low — ServiceNow's own impact/urgency/priority convention,
// already used elsewhere in this codebase, e.g. entity-service's own
// Impact/Urgency key documentation) onto this service's five-value severity
// vocabulary (critical/major/minor/warning/ok — see internal/severity's own
// doc comment for that set). Only 1/2/3 are recognized; anything else falls
// back to "warning", not "critical" — see mapChoreoDPPayload's own comment
// for why this adapter, like openSearchSeverityTable before it, deliberately
// does not fail open to the most alarming classification on an unrecognized
// value.
var choreoDPSeverityTable = map[int]string{
	1: "critical",
	2: "major",
	3: "minor",
}

// choreoDPImpactUrgencyTable maps the same 1=High/2=Medium/3=Low convention
// onto csmclient.CreateIncidentRequest's own "HIGH"/"MEDIUM"/"LOW"
// vocabulary — the values AlertRequest.Impact/Urgency must carry (see that
// field's own doc comment). Unrecognized values map to "", which
// mapChoreoDPPayload treats as "leave Impact/Urgency unset" — falling
// through to MapToIncident's normal severity.MapImpactUrgency derivation
// rather than propagating a meaningless value.
var choreoDPImpactUrgencyTable = map[int]string{
	1: "HIGH",
	2: "MEDIUM",
	3: "LOW",
}

// choreoDPDefaultSeverity is this adapter's safe-default severity for a
// payload whose own severity int isn't one of choreoDPSeverityTable's
// recognized keys. "warning", not "critical" — an unrecognized value must
// never fail open to the most alarming classification (see
// openSearchSeverityTable's own doc comment for the same convention and the
// reasoning behind it).
const choreoDPDefaultSeverity = "warning"

// mapChoreoDPPayload parses this source's native alert body into an
// AlertRequest.
//
// Impact/Urgency are set from the payload's own impact/urgency fields via
// choreoDPImpactUrgencyTable, bypassing MapToIncident's usual
// severity.MapImpactUrgency(req.Severity) derivation for this alert — see
// AlertRequest.Impact/Urgency's own doc comment for the full override
// contract. req.Severity is still populated (AlertRequest.validate requires
// it non-empty even though this adapter's own Impact/Urgency values are what
// actually drive the created incident's severity fields) via
// choreoDPSeverityTable, independently of the impact/urgency mapping above.
func mapChoreoDPPayload(body []byte) (AlertRequest, error) {
	var p choreoDPPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return AlertRequest{}, fmt.Errorf("choreodp: %w", err)
	}

	severity := choreoDPDefaultSeverity
	if s, ok := choreoDPSeverityTable[p.Severity]; ok {
		severity = s
	}

	req := AlertRequest{
		Source:      p.Source,
		Severity:    severity,
		Service:     p.Service,
		MetricName:  p.MetricName,
		Category:    p.Category,
		Environment: p.Environment,
		Description: p.Description,
	}

	if impact, ok := choreoDPImpactUrgencyTable[p.Impact]; ok {
		req.Impact = &impact
	}
	if urgency, ok := choreoDPImpactUrgencyTable[p.Urgency]; ok {
		req.Urgency = &urgency
	}

	return req, nil
}

// CreateAlertFromChoreoDP handles POST /alerts/adapters/choreodp: translates
// this source's own native alert payload into this service's generic
// AlertRequest, then reuses enqueueAlert unchanged — same
// validation/buffering/worker/grouping/dedup/escalation path as POST
// /alerts. See this file's mapChoreoDPPayload for the mapping rules,
// including the Impact/Urgency override this adapter is the first to use.
func (h *AlertHandler) CreateAlertFromChoreoDP(w http.ResponseWriter, r *http.Request) {
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

	req, err := mapChoreoDPPayload(body)
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
