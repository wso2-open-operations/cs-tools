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
	"errors"
	"fmt"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/apierror"
)

// CreateAlertIncidentMappingRequest is the request body for
// csm-integration-service's POST /alert-incident-mappings — a thin proxy of
// that service's own contract, field names and JSON tags copied verbatim,
// matching the convention CreateIncidentRequest already follows in this
// package. Records one buffered alert against the CSM incident it ended up
// delivered to (whether that incident was just created for it, or an
// earlier alert's still-open incident it was grouped onto instead — see
// internal/worker's incident-grouping logic).
type CreateAlertIncidentMappingRequest struct {
	AlertNumber      string  `json:"alertNumber"`
	Source           string  `json:"source"`
	UniqueIdentifier *string `json:"uniqueIdentifier,omitempty"`
	Service          *string `json:"service,omitempty"`
	MetricName       *string `json:"metricName,omitempty"`
	AlertStatus      string  `json:"alertStatus"`
	IncidentID       string  `json:"incidentId"`
	IncidentNumber   *string `json:"incidentNumber,omitempty"`
}

// LookupAlertIncidentMappingsRequest is the request body for
// csm-integration-service's POST /alert-incident-mappings/lookup.
type LookupAlertIncidentMappingsRequest struct {
	Source           string `json:"source"`
	UniqueIdentifier string `json:"uniqueIdentifier"`
}

// AlertIncidentMappingView is one row of the alert-incident-mapping table,
// as returned by both the create and lookup endpoints.
type AlertIncidentMappingView struct {
	ID               string  `json:"id"`
	AlertNumber      string  `json:"alertNumber"`
	Source           string  `json:"source"`
	UniqueIdentifier string  `json:"uniqueIdentifier"`
	Service          *string `json:"service,omitempty"`
	MetricName       *string `json:"metricName,omitempty"`
	AlertStatus      string  `json:"alertStatus"`
	IncidentID       string  `json:"incidentId"`
	IncidentNumber   *string `json:"incidentNumber,omitempty"`
	CreatedAt        string  `json:"createdAt"`
}

// lookupAlertIncidentMappingsResponse is the response body for
// POST /alert-incident-mappings/lookup.
type lookupAlertIncidentMappingsResponse struct {
	Mappings []AlertIncidentMappingView `json:"mappings"`
}

// CreateAlertIncidentMapping calls POST /alert-incident-mappings on
// csm-integration-service to record alertNumber against incidentId.
//
// A 409 (alertNumber already has a recorded mapping) is treated as
// already-recorded, not an error worth failing delivery over: it reports
// (nil, nil) rather than an error, matching the "this alert is already
// grouped/recorded, nothing more to do" outcome the caller wants. Every
// other non-2xx or transport error is returned as-is for the caller
// (internal/worker) to log — every call site of this method treats a
// failure here as best-effort/non-blocking, never as a reason to fail the
// overall delivery (see internal/worker's doc comments on those call sites).
func (c *Client) CreateAlertIncidentMapping(ctx context.Context, req CreateAlertIncidentMappingRequest) (*AlertIncidentMappingView, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("csmclient: marshal CreateAlertIncidentMappingRequest: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/alert-incident-mappings", body)
	if err != nil {
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
			return nil, nil
		}
		return nil, err
	}

	var view AlertIncidentMappingView
	if err := json.Unmarshal(respBody, &view); err != nil {
		return nil, fmt.Errorf("csmclient: decode CreateAlertIncidentMapping response: %w", err)
	}
	return &view, nil
}

// LookupAlertIncidentMappings calls POST /alert-incident-mappings/lookup on
// csm-integration-service for (source, uniqueIdentifier), returning any
// recorded mappings most-recent-first (per the endpoint's own contract), or
// an empty slice if none exist.
func (c *Client) LookupAlertIncidentMappings(ctx context.Context, source, uniqueIdentifier string) ([]AlertIncidentMappingView, error) {
	req := LookupAlertIncidentMappingsRequest{Source: source, UniqueIdentifier: uniqueIdentifier}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("csmclient: marshal LookupAlertIncidentMappingsRequest: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/alert-incident-mappings/lookup", body)
	if err != nil {
		return nil, err
	}

	var resp lookupAlertIncidentMappingsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("csmclient: decode LookupAlertIncidentMappings response: %w", err)
	}
	return resp.Mappings, nil
}
