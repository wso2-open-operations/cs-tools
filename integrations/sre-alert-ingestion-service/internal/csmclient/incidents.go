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

package csmclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// CreateIncidentRequest is the request body for csm-integration-service's
// POST /incidents — a thin proxy of entity-service's own
// CreateIncidentRequest. Field names and JSON tags are copied verbatim from
// that contract; this service does not define its own incident shape.
type CreateIncidentRequest struct {
	CallerID            string   `json:"callerId"`
	Category            string   `json:"category"` // "INQUIRY" | "SERVICE_INTERRUPTION" | "SECURITY"
	Subcategory         *string  `json:"subcategory,omitempty"`
	ServiceID           string   `json:"serviceId"`
	ServiceOfferingID   *string  `json:"serviceOfferingId,omitempty"`
	ConfigurationItemID *string  `json:"configurationItemId,omitempty"`
	ContactType         *string  `json:"contactType,omitempty"`
	Impact              string   `json:"impact"`  // "HIGH" | "MEDIUM" | "LOW"
	Urgency             string   `json:"urgency"` // "HIGH" | "MEDIUM" | "LOW"
	AssignmentGroupID   *string  `json:"assignmentGroupId,omitempty"`
	AssignedEngineerID  *string  `json:"assignedEngineerId,omitempty"`
	Subject             string   `json:"subject"`
	WatchList           []string `json:"watchList,omitempty"`
	AdditionalComments  *string  `json:"additionalComments,omitempty"`
	WorkNotes           *string  `json:"workNotes,omitempty"`
	ChangeRequestID     *string  `json:"changeRequestId,omitempty"`
	ProblemID           *string  `json:"problemId,omitempty"`
}

// createdIncident is the subset of the response's nested "incident" object
// this service actually reads. The real response carries more fields;
// unmarshaling into a struct that only names these two ignores the rest
// rather than requiring an exact shape match, per the tolerant-decoding
// instruction in the upstream contract.
type createdIncident struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

// createIncidentResponse is the response body for POST /incidents, decoded
// tolerantly (see createdIncident).
type createIncidentResponse struct {
	Message  string          `json:"message"`
	Incident createdIncident `json:"incident"`
}

// CreateIncidentResult is what CreateIncident returns on success: just
// enough of the upstream response for the worker to record against the
// buffered alert row (incident_id) and for logs.
type CreateIncidentResult struct {
	IncidentID     string
	IncidentNumber string
}

// UnresolvedServiceIDSentinel is the CreateIncidentRequest.ServiceID value
// internal/handler.MapToIncident writes when its static SRE_ALERT_SERVICE_MAP
// lookup has no entry for the alert's raw Service label at buffering time
// (see that function's doc comment for the full hybrid-resolution design).
//
// Deliberately the empty string — Go's own zero value for this field — for
// two reasons: it is unambiguous against a real CMDB service UUID (always a
// non-empty, hyphenated 36-character string, per entity-service's own
// `format: uuid` contract), and it reuses rather than shadows the zero value
// ServiceID would already carry if this field were simply left unset. This
// sentinel is never sent to csm-integration-service as-is — internal/worker
// checks for exactly this value immediately before calling CreateIncident,
// and resolves a real UUID (from its own in-memory cache, a live
// /services/search call, or the configured unknown-service fallback) before
// the request goes out. See internal/worker.resolveServiceID.
const UnresolvedServiceIDSentinel = ""

// CreateIncident calls POST /incidents on csm-integration-service.
//
// A 401 is possible here, but it is not an unconditional architectural
// limitation of this M2M-only service. The upstream entity-service
// operation this proxies is ServiceNow-backed; if no forwarded end-user
// identity token is present, it falls back to a separately-configured
// M2M ServiceNow credential and only 401s if that fallback credential is
// itself unconfigured in the target environment. A live end-to-end call
// through this exact chain against wso2sndev on 2026-09-20 succeeded with
// no 401, creating a real incident (INC0096966). So whether this 401s
// depends on the target ServiceNow environment's M2M credential
// configuration, not on csm-integration-service being M2M-only per se.
//
// Critically, if a 401 does occur, it must still be treated as a
// *retryable, CSM-side-unavailability* signal by the caller
// (internal/worker), not as a permanent client error that skips retry —
// which is the opposite of how a 401 is normally read. The reasoning:
// this 401 does not mean "this specific alert's payload is invalid" (a
// real 400 from bad input is the actual permanent-failure case, and is
// handled separately — see internal/worker's classifyErr). It means "CSM
// cannot currently accept this incident through this path" (e.g. the
// target environment's M2M ServiceNow credential isn't configured),
// which is exactly the condition this whole service exists to buffer
// through. Once that credential is configured (or reconfigured) and
// calls start succeeding, callers up the chain (SRE's monitoring tools)
// should see zero behavior change — alerts that used to sit in the
// buffer until the retry window naturally succeeds should now succeed
// sooner, not error out differently.
func (c *Client) CreateIncident(ctx context.Context, req CreateIncidentRequest) (*CreateIncidentResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("csmclient: marshal CreateIncidentRequest: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/incidents", body)
	if err != nil {
		return nil, err
	}

	var resp createIncidentResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("csmclient: decode CreateIncident response: %w", err)
	}
	if resp.Incident.ID == "" {
		// A 2xx with no incident ID is malformed, not a successful create —
		// treat it as a failure so the worker's normal retry path handles
		// it, rather than reporting success and persisting an empty
		// incident_id the rest of this service (dedup search, escalation
		// logging, the CSM-side mapping record) would then treat as real.
		return nil, fmt.Errorf("csmclient: CreateIncident response missing incident id")
	}

	return &CreateIncidentResult{
		IncidentID:     resp.Incident.ID,
		IncidentNumber: resp.Incident.Number,
	}, nil
}

// updateIncidentRequest is the request body for csm-integration-service's
// PATCH /incidents/{id} — a thin proxy of entity-service's own
// UpdateIncidentRequest, matching CreateIncidentRequest's own
// copied-verbatim convention. This service only ever sends WorkNotes (see
// UpdateIncident below), so every other field that request shape accepts is
// left unmodeled rather than a source of drift to keep in sync by hand.
type updateIncidentRequest struct {
	WorkNotes string `json:"workNotes"`
}

// UpdateIncident calls PATCH /incidents/{id} on csm-integration-service to
// push workNotes onto an already-existing incident.
//
// This backs internal/worker.tryGroup's group-attach path: when a new alert
// is found to be reporting the same condition as an earlier, still-open
// incident, this alert attaches to it instead of creating a new one — but
// until this method existed, that attach was silent: nothing was pushed to
// the incident itself, so an engineer looking at it had no record the
// condition fired again. workNotes is internal, engineer-facing metadata,
// never customer-visible — matching how internal/handler.MapToIncident
// already splits WorkNotes (internal) from AdditionalComments
// (customer-facing) for a freshly-created incident; this call only ever sets
// WorkNotes, never AdditionalComments.
//
// Like CreateIncident and every csmclient search call in this package, the
// underlying entity-service operation has a documented M2M-credential
// fallback on the ServiceNow data source (see csm-integration-service's own
// CLAUDE.md) — a 401 is possible but not unconditional. Unlike
// CreateIncident, though, there is no Postgres-data-source fallback at all:
// on that data source csm-integration-service reports a mapped 503. Callers
// in this package treat any error from this method as a candidate for their
// own best-effort/non-blocking handling (see internal/worker.tryGroup's call
// site) rather than inspecting the status code here — this method itself
// does no special-casing beyond returning whatever c.do reports.
func (c *Client) UpdateIncident(ctx context.Context, incidentID, workNotes string) error {
	body, err := json.Marshal(updateIncidentRequest{WorkNotes: workNotes})
	if err != nil {
		return fmt.Errorf("csmclient: marshal UpdateIncident request: %w", err)
	}

	_, err = c.do(ctx, http.MethodPatch, "/incidents/"+url.PathEscape(incidentID), body)
	return err
}
