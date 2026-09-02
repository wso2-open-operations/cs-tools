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

// CreateIncident calls POST /incidents on csm-integration-service.
//
// Every call currently receives a mapped 401: the upstream entity-service
// operation this proxies is ServiceNow-backed and requires a forwarded
// end-user identity token that csm-integration-service, being M2M-only, has
// no mechanism to supply (see csm-integration-service's own CLAUDE.md,
// "This service is M2M-only"). This is a known, already-decided limitation
// this service does not attempt to work around.
//
// Critically, that 401 must be treated as a *retryable, CSM-side-unavailability*
// signal by the caller (internal/worker), not as a permanent client error
// that skips retry — which is the opposite of how a 401 is normally read.
// The reasoning: this 401 does not mean "this specific alert's payload is
// invalid" (a real 400 from bad input is the actual permanent-failure case,
// and is handled separately — see internal/worker's classifyErr). It means
// "CSM cannot currently accept this incident through this path", which is
// exactly the condition this whole service exists to buffer through. Once
// the missing end-user-identity infrastructure exists and this starts
// succeeding, callers up the chain (SRE's monitoring tools) should see zero
// behavior change — alerts that used to sit in the buffer until the retry
// window naturally succeeds should now succeed sooner, not error out
// differently.
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

	return &CreateIncidentResult{
		IncidentID:     resp.Incident.ID,
		IncidentNumber: resp.Incident.Number,
	}, nil
}
