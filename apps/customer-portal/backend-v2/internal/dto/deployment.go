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

package dto

import (
	"encoding/json"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// DeploymentSummary is one item of the portal's response for
// POST /projects/{id}/deployments/search — Type and Project as IDLabelRef
// to match the frontend's own ProjectDeploymentItem type
// (apps/customer-portal/webapp/src/features/project-details/types/deployments.ts).
type DeploymentSummary struct {
	ID          string      `json:"id"`
	Number      string      `json:"number"`
	Name        string      `json:"name"`
	Type        *IDLabelRef `json:"type,omitempty"`
	Description *string     `json:"description,omitempty"`
	Project     *IDLabelRef `json:"project,omitempty"`
	CreatedOn   time.Time   `json:"createdOn"`
	UpdatedOn   time.Time   `json:"updatedOn"`
	URL         *string     `json:"url,omitempty"`
	// ProductCount is named productCount, NOT deployedProductCount: the
	// frontend's ProjectDeploymentItem reads productCount, and the Usage
	// Metrics page filters deployments on `(dep.productCount ?? 0) > 0`. An
	// absent value silently filters every deployment out, leaving that page
	// blank with no error and no network calls. The Ballerina backend does the
	// same rename (utils.bal: `productCount: deployment.deployedProductCount`).
	ProductCount int `json:"productCount"`
}

// SearchDeploymentsResponse is the portal's response for
// POST /projects/{id}/deployments/search. TotalRecords (not Total) to match
// the frontend's shared pagination envelope.
type SearchDeploymentsResponse struct {
	Deployments  []DeploymentSummary `json:"deployments"`
	TotalRecords int                 `json:"totalRecords"`
	Limit        int                 `json:"limit"`
	Offset       int                 `json:"offset"`
	HasMore      bool                `json:"hasMore"`
}

// MapSearchDeployments builds the portal response from entity-service's SearchDeploymentsResponse.
func MapSearchDeployments(r entity.SearchDeploymentsResponse) SearchDeploymentsResponse {
	deployments := make([]DeploymentSummary, 0, len(r.Deployments))
	for _, d := range r.Deployments {
		deployments = append(deployments, DeploymentSummary{
			ID:          d.ID,
			Number:      d.Number,
			Name:        d.Name,
			Type:        deploymentTypeRef(d.Type),
			Description: d.Description,
			Project:     entityRefToIDLabel(&d.Project),
			CreatedOn:   d.CreatedOn,
			UpdatedOn:   d.UpdatedOn,

			URL:          d.URL,
			ProductCount: d.DeployedProductCount,
		})
	}
	return SearchDeploymentsResponse{
		Deployments:  deployments,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
		HasMore:      r.HasMore,
	}
}

// DeploymentCreateRequest is the portal's request body for
// POST /projects/{id}/deployments — DeploymentTypeKey is the ServiceNow
// numeric choice-list key the frontend sends (CreateDeploymentRequest.
// deploymentTypeKey), translated to entity-service's string enum by
// BuildEntityCreateDeploymentRequest.
type DeploymentCreateRequest struct {
	DeploymentTypeKey int    `json:"deploymentTypeKey"`
	Description       string `json:"description"`
	Name              string `json:"name"`
}

// BuildEntityCreateDeploymentRequest translates the portal's create request
// into entity-service's request shape, forcing ProjectID from the path.
func BuildEntityCreateDeploymentRequest(projectID string, req DeploymentCreateRequest) entity.CreateDeploymentRequest {
	return entity.CreateDeploymentRequest{
		ProjectID:   projectID,
		Name:        req.Name,
		Type:        deploymentTypeIDToEnumPtr(req.DeploymentTypeKey),
		Description: req.Description,
	}
}

// DeploymentUpdateRequest is the portal's request body for
// PATCH /projects/{id}/deployments/{deploymentId} — TypeKey is the
// ServiceNow numeric choice-list key the frontend sends
// (PatchDeploymentRequest.typeKey), translated to entity-service's string
// enum by BuildEntityUpdateDeploymentRequest. Description is json.RawMessage
// (not *string) to preserve the three-state absent/null/value semantic
// entity-service's own UpdateDeploymentRequest.Description expects — a
// *string can't distinguish an omitted field from an explicit
// {"description": null}, which would silently drop a customer's attempt to
// clear the description (see the analogous convention already documented
// for UpdateDeployedProductRequest.Description in this backend's CLAUDE.md).
type DeploymentUpdateRequest struct {
	Active      *bool           `json:"active,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
	Name        *string         `json:"name,omitempty"`
	TypeKey     *int            `json:"typeKey,omitempty"`
}

// BuildEntityUpdateDeploymentRequest translates the portal's update request
// into entity-service's request shape.
func BuildEntityUpdateDeploymentRequest(id string, req DeploymentUpdateRequest) entity.UpdateDeploymentRequest {
	out := entity.UpdateDeploymentRequest{
		ID:          id,
		Name:        req.Name,
		Active:      req.Active,
		Description: req.Description,
	}
	if req.TypeKey != nil {
		out.Type = deploymentTypeIDToEnumPtr(*req.TypeKey)
	}
	return out
}

// DeploymentCreateResponse is the portal's response for POST /deployments.
type DeploymentCreateResponse struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
}

// MapDeploymentCreate builds the portal response from entity-service's CreateDeploymentResponse.
func MapDeploymentCreate(r entity.CreateDeploymentResponse) DeploymentCreateResponse {
	return DeploymentCreateResponse{
		ID:        r.Deployment.ID,
		CreatedOn: r.Deployment.CreatedOn,
	}
}

// DeploymentUpdateResponse is the portal's response for PATCH /deployments/{id}.
// Deliberately excludes entity-service's UpdatedBy (internal actor identity),
// consistent with the other update responses in this package.
type DeploymentUpdateResponse struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
}

// MapDeploymentUpdate builds the portal response from entity-service's UpdateDeploymentResponse.
func MapDeploymentUpdate(r entity.UpdateDeploymentResponse) DeploymentUpdateResponse {
	return DeploymentUpdateResponse{
		ID:        r.Deployment.ID,
		UpdatedOn: r.Deployment.UpdatedOn,
	}
}
