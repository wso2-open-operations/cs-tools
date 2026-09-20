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

// site24x7Payload is Site24x7's own native alert-webhook body, modeled to
// only the fields this adapter reads.
type site24x7Payload struct {
	Status          string `json:"STATUS"`
	MonitorName     string `json:"MONITORNAME"`
	MonitorID       string `json:"MONITOR_ID"`
	IncidentReason  string `json:"INCIDENT_REASON"`
	IncidentDetails string `json:"INCIDENT_DETAILS"`
}

// site24x7ActionableStatuses is the set of Site24x7 STATUS values this
// adapter acts on — matched case-sensitively, matching the vendor's own
// convention (Site24x7 always sends these in upper case; there's no reason
// to guess at case-insensitivity here). Anything else is a status this
// integration doesn't create incidents for (e.g. "UP") and is filtered out
// by mapSite24x7Payload below.
var site24x7ActionableStatuses = map[string]bool{
	"TROUBLE":  true,
	"DOWN":     true,
	"CRITICAL": true,
}

// site24x7SeverityTable gives STATUS a real severity mapping — DOWN/CRITICAL
// are treated as this service's "critical", TROUBLE as "warning". This is a
// deliberate deviation from the prior ServiceNow-based pipeline, which had
// no real per-status severity mapping for this vendor at all: every alert
// through that path got the same hardcoded low-priority classification
// regardless of STATUS, a gap the source review documented, not a design
// choice worth replicating.
var site24x7SeverityTable = map[string]string{
	"DOWN":     "critical",
	"CRITICAL": "critical",
	"TROUBLE":  "warning",
}

// site24x7DefaultService is used since this payload carries no field
// identifying an affected service/deployment — AlertRequest.validate
// requires Service non-empty.
const site24x7DefaultService = "Site24x7 Monitoring"

// mapSite24x7Payload parses a Site24x7 native webhook body into an
// AlertRequest, or reports ok=false if STATUS isn't one of the actionable
// values this integration creates incidents for (site24x7ActionableStatuses)
// — the caller (CreateAlertFromSite24x7) responds 200 in that case rather
// than treating it as an error, since the payload itself is perfectly valid,
// it's just not a status this integration acts on. This is a deliberate
// improvement over the prior ServiceNow-based pipeline, which silently
// dropped non-matching statuses with no visibility at all.
func mapSite24x7Payload(body []byte) (req AlertRequest, ok bool, err error) {
	var p site24x7Payload
	if err := json.Unmarshal(body, &p); err != nil {
		return AlertRequest{}, false, fmt.Errorf("site24x7: %w", err)
	}

	if !site24x7ActionableStatuses[p.Status] {
		return AlertRequest{}, false, nil
	}

	severity := site24x7SeverityTable[p.Status]

	return AlertRequest{
		Source:           "site24x7",
		Severity:         severity,
		Service:          site24x7DefaultService,
		MetricName:       p.MonitorName,
		UniqueIdentifier: p.MonitorID,
		Description:      p.IncidentReason + ": " + p.IncidentDetails,
	}, true, nil
}

// CreateAlertFromSite24x7 handles POST /alerts/adapters/site24x7:
// translates a Site24x7 native alert-webhook payload into this service's
// generic AlertRequest, then reuses enqueueAlert unchanged. Non-actionable
// STATUS values (anything but TROUBLE/DOWN/CRITICAL) get a 200 with a small
// acknowledgment body, never a 400 — see mapSite24x7Payload's doc comment.
func (h *AlertHandler) CreateAlertFromSite24x7(w http.ResponseWriter, r *http.Request) {
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

	req, ok, err := mapSite24x7Payload(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "STATUS is not one of TROUBLE, DOWN, CRITICAL"})
		return
	}

	id, alertNumber, err := h.enqueueAlert(r.Context(), req)
	h.writeEnqueueResult(w, r, id, alertNumber, err)
}
