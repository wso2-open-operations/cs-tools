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

// openSearchPayload is the native alert body sent by the OpenSearch
// (Choreo path) source, modeled to only the fields this adapter reads.
//
// Trap: this payload's own "source" field is NOT the vendor/system
// identity — it's a human-readable title-like field (the prior
// ServiceNow-based pipeline used it as the incident's short description).
// Do not confuse it with AlertRequest.Source, which must always be the
// fixed literal "opensearch" — see mapOpenSearchPayload below.
type openSearchPayload struct {
	Source      string `json:"source"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// openSearchSeverityTable maps this source's own severity vocabulary onto
// this service's five-value vocabulary. Unrecognized/missing values map to
// "ok" (see mapOpenSearchPayload) — a deliberate, safe-default deviation
// from the prior ServiceNow-based pipeline, which failed OPEN to the
// highest severity on any unrecognized value, a choice the source review
// explicitly flagged as risky and backwards. Do not replicate that here.
var openSearchSeverityTable = map[string]string{
	"critical":      "critical",
	"urgent":        "major",
	"warning":       "warning",
	"low":           "minor",
	"informational": "ok",
}

// openSearchDefaultService is used since this payload carries no field
// identifying an affected service/deployment — AlertRequest.validate
// requires Service non-empty.
const openSearchDefaultService = "OpenSearch Monitoring"

// mapOpenSearchPayload parses an OpenSearch (Choreo path) native alert body
// into an AlertRequest.
func mapOpenSearchPayload(body []byte) (AlertRequest, error) {
	var p openSearchPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return AlertRequest{}, fmt.Errorf("opensearch: %w", err)
	}

	// Safe default: any severity value this adapter doesn't recognize
	// (including an empty one) maps to "ok", the least-alarming
	// classification — not the highest, unlike the prior pipeline. See this
	// table's own doc comment for why.
	severity := "ok"
	if s, ok := openSearchSeverityTable[p.Severity]; ok {
		severity = s
	}

	return AlertRequest{
		// Fixed literal, deliberately NOT p.Source — see openSearchPayload's
		// doc comment: this payload's own "source" field is a human-readable
		// title, not the originating system's identity.
		Source:      "opensearch",
		Severity:    severity,
		Service:     openSearchDefaultService,
		MetricName:  p.Source,
		Description: p.Description,
		// No UniqueIdentifier: this payload shape carries nothing suitable
		// for cross-alert grouping. That's fine — grouping is optional (see
		// internal/worker.tryGroup's own doc comment).
	}, nil
}

// CreateAlertFromOpenSearch handles POST /alerts/adapters/opensearch:
// translates an OpenSearch (Choreo path) native alert payload into this
// service's generic AlertRequest, then reuses enqueueAlert unchanged.
func (h *AlertHandler) CreateAlertFromOpenSearch(w http.ResponseWriter, r *http.Request) {
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

	req, err := mapOpenSearchPayload(body)
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
