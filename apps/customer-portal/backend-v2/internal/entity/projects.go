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

package entity

import (
	"context"
	"fmt"
	"net/url"
)

// SearchProjects calls POST /projects/search.
func (c *Client) SearchProjects(ctx context.Context, req SearchProjectsRequest) (SearchProjectsResponse, error) {
	var out SearchProjectsResponse
	err := c.postJSON(ctx, "/projects/search", req, &out)
	return out, err
}

// GetProject calls GET /projects/{id}.
func (c *Client) GetProject(ctx context.Context, id string) (ProjectDetailsView, error) {
	var out ProjectDetailsView
	err := c.getJSON(ctx, fmt.Sprintf("/projects/%s", url.PathEscape(id)), &out)
	return out, err
}

// UpdateProject patches a project's AI-assistant settings via
// PATCH /projects/{id}.
//
// Only the two agent toggles are patchable, and entity-service accepts exactly
// one per request — see UpdateProjectRequest.
func (c *Client) UpdateProject(ctx context.Context, id string, req UpdateProjectRequest) (UpdateProjectResponse, error) {
	var out UpdateProjectResponse
	err := c.patchJSON(ctx, fmt.Sprintf("/projects/%s", url.PathEscape(id)), req, &out)
	return out, err
}

// GetProjectMetadata calls GET /projects/{id}/metadata.
//
// NOTE: only entity-service's ServiceNow data source supports this route —
// see cs-tools/entity-service/internal/server/routes.go.
func (c *Client) GetProjectMetadata(ctx context.Context, id string) (ProjectMetadataResponse, error) {
	var out ProjectMetadataResponse
	err := c.getJSON(ctx, fmt.Sprintf("/projects/%s/metadata", url.PathEscape(id)), &out)
	return out, err
}

// GetProjectStats calls GET /projects/{id}/stats.
func (c *Client) GetProjectStats(ctx context.Context, id string) (ProjectStatsResponse, error) {
	var out ProjectStatsResponse
	err := c.getJSON(ctx, fmt.Sprintf("/projects/%s/stats", url.PathEscape(id)), &out)
	return out, err
}

// GetProjectCaseStats calls GET /projects/{id}/cases/stats. caseTypes and
// createdBy are both optional filters; pass caseTypes as nil/empty and
// createdBy as "" to omit them.
func (c *Client) GetProjectCaseStats(ctx context.Context, id string, caseTypes []string, createdBy string) (ProjectCaseStatsResponse, error) {
	q := url.Values{}
	for _, ct := range caseTypes {
		q.Add("caseTypes", ct)
	}
	if createdBy != "" {
		q.Set("createdBy", createdBy)
	}
	path := fmt.Sprintf("/projects/%s/cases/stats", url.PathEscape(id))
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out ProjectCaseStatsResponse
	err := c.getJSON(ctx, path, &out)
	return out, err
}

// GetProjectConversationStats calls GET /projects/{id}/conversations/stats.
// createdBy is an optional filter; pass "" to omit it.
func (c *Client) GetProjectConversationStats(ctx context.Context, id, createdBy string) (ProjectConversationStatsResponse, error) {
	path := fmt.Sprintf("/projects/%s/conversations/stats", url.PathEscape(id))
	if createdBy != "" {
		path += "?" + url.Values{"createdBy": {createdBy}}.Encode()
	}
	var out ProjectConversationStatsResponse
	err := c.getJSON(ctx, path, &out)
	return out, err
}

// GetProjectDeploymentStats calls GET /projects/{id}/deployments/stats.
func (c *Client) GetProjectDeploymentStats(ctx context.Context, id string) (ProjectDeploymentStatsResponse, error) {
	var out ProjectDeploymentStatsResponse
	err := c.getJSON(ctx, fmt.Sprintf("/projects/%s/deployments/stats", url.PathEscape(id)), &out)
	return out, err
}

// GetProjectTimeCardStats calls GET /projects/{id}/time-cards/stats.
// startDate/endDate (each yyyy-MM-dd) are an optional filter range; pass ""
// to omit either.
func (c *Client) GetProjectTimeCardStats(ctx context.Context, id, startDate, endDate string) (ProjectTimeCardStatsResponse, error) {
	q := url.Values{}
	if startDate != "" {
		q.Set("startDate", startDate)
	}
	if endDate != "" {
		q.Set("endDate", endDate)
	}
	path := fmt.Sprintf("/projects/%s/time-cards/stats", url.PathEscape(id))
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out ProjectTimeCardStatsResponse
	err := c.getJSON(ctx, path, &out)
	return out, err
}

// GetProjectChangeRequestStats calls GET /projects/{id}/change-requests/stats.
func (c *Client) GetProjectChangeRequestStats(ctx context.Context, id string) (ProjectChangeRequestStatsResponse, error) {
	var out ProjectChangeRequestStatsResponse
	err := c.getJSON(ctx, fmt.Sprintf("/projects/%s/change-requests/stats", url.PathEscape(id)), &out)
	return out, err
}
