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

package csm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// CreateIncidentRequest proxies entity-service's own contract verbatim; this service does not define its own incident shape.
// It never carries an assignment group: entity-service sets that from the service's support group.
type CreateIncidentRequest struct {
	CallerID  string  `json:"callerId"`
	Category  string  `json:"category"` // "INQUIRY" | "SERVICE_INTERRUPTION" | "SECURITY"
	ServiceID string  `json:"serviceId"`
	Impact    string  `json:"impact"`  // "HIGH" | "MEDIUM" | "LOW"
	Urgency   string  `json:"urgency"` // "HIGH" | "MEDIUM" | "LOW"
	Subject   string  `json:"subject"`
	WorkNotes *string `json:"workNotes,omitempty"`
	// ContactType says how the incident was raised (entity-service's IncidentContactType); routing to the SRE escalation ladder reads it.
	ContactType *string `json:"contactType,omitempty"`
	// CorrelationID tags the incident with its fingerprint generation on ServiceNow's correlation_id field, for tracing it back to alert-core.
	CorrelationID *string `json:"correlationId,omitempty"`
}

// createdIncident is the subset of the response's nested "incident" object this service actually reads.
type createdIncident struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

// createIncidentResponse is the response body for POST /incidents, decoded tolerantly (unknown fields ignored).
type createIncidentResponse struct {
	Message  string          `json:"message"`
	Incident createdIncident `json:"incident"`
}

// CreateIncidentResult is what CreateIncident returns on success.
type CreateIncidentResult struct {
	IncidentID     string
	IncidentNumber string
}

// CreateIncident calls POST /incidents on csm-integration-service.
func (c *Client) CreateIncident(ctx context.Context, req CreateIncidentRequest) (*CreateIncidentResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("csm: marshal CreateIncidentRequest: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/incidents", body)
	if err != nil {
		return nil, err
	}

	var resp createIncidentResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("csm: decode CreateIncident response: %w", err)
	}
	if resp.Incident.ID == "" || resp.Incident.Number == "" {
		// A 2xx with no id/number is malformed; treat as failure so retry handles it, not a half-populated persist.
		return nil, fmt.Errorf("csm: CreateIncident response missing incident id or number")
	}

	return &CreateIncidentResult{IncidentID: resp.Incident.ID, IncidentNumber: resp.Incident.Number}, nil
}

// updateIncidentRequest models only WorkNotes; other PATCH /incidents/{id} fields are intentionally left unmodeled.
type updateIncidentRequest struct {
	WorkNotes string `json:"workNotes"`
}

// UpdateIncident pushes a work note, keyed by CSM's incident id (not the human-readable number).
func (c *Client) UpdateIncident(ctx context.Context, incidentID, workNotes string) error {
	body, err := json.Marshal(updateIncidentRequest{WorkNotes: workNotes})
	if err != nil {
		return fmt.Errorf("csm: marshal UpdateIncident request: %w", err)
	}
	_, err = c.do(ctx, http.MethodPatch, "/incidents/"+url.PathEscape(incidentID), body)
	return err
}

// searchIncidentsRequest is the request body for POST /incidents/search — only the subset this client needs (exact match on Number).
type searchIncidentsRequest struct {
	Filters    searchIncidentsFilters `json:"filters"`
	Pagination pagination             `json:"pagination"`
}

type searchIncidentsFilters struct {
	Number string `json:"number,omitempty"`
}

type pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type searchIncidentView struct {
	ID     *string `json:"id"`
	Number *string `json:"number"`
	State  *string `json:"state"`
}

type searchIncidentsResponse struct {
	Incidents []searchIncidentView `json:"incidents"`
	Total     int                  `json:"total"`
}

// openIncidentStates copies entity-service's IncidentState values by hand (separate Go module, not importable) — keep in sync manually.
var openIncidentStates = map[string]bool{"NEW": true, "IN_PROGRESS": true, "ON_HOLD": true}

// IncidentState is authoritative for open/closed since only CSM/agents ever close incidents; it looks up number exactly and accepts only the row whose id is incidentID; found is false on no match.
func (c *Client) IncidentState(ctx context.Context, incidentID, number string) (open bool, found bool, err error) {
	req := searchIncidentsRequest{
		Filters:    searchIncidentsFilters{Number: number},
		Pagination: pagination{Limit: 1, Offset: 0},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return false, false, fmt.Errorf("csm: marshal SearchIncidentsRequest: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/incidents/search", body)
	if err != nil {
		return false, false, err
	}

	var resp searchIncidentsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return false, false, fmt.Errorf("csm: decode incident search response: %w", err)
	}
	if len(resp.Incidents) == 0 {
		return false, false, nil
	}
	hit := resp.Incidents[0]
	// Another incident's row means the number filter was not applied; its state says nothing about this incident.
	if hit.ID == nil || *hit.ID != incidentID || hit.Number == nil || *hit.Number != number {
		return false, false, fmt.Errorf("csm: incident search for %s returned a different incident, number filter not applied", number)
	}
	if hit.State == nil {
		return false, false, nil
	}
	return openIncidentStates[*hit.State], true, nil
}
