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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// CodeDashboard is an advanced dashboard the caller saved (entity-service
// /users/me/code-dashboards). Source is the dashboard's JavaScript as text;
// this backend checks its size, the webapp checks and runs it, entity-service
// stores it without parsing it. The owner is always the caller of the
// x-user-id-token, so it is not here.
type CodeDashboard struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	CreatedOn string `json:"createdOn"`
	UpdatedOn string `json:"updatedOn"`
}

// CodeDashboardSummary is one entry of the caller's list: no source.
type CodeDashboardSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedOn string `json:"createdOn"`
	UpdatedOn string `json:"updatedOn"`
}

type codeDashboardListResponse struct {
	Dashboards []CodeDashboardSummary `json:"dashboards"`
}

type saveCodeDashboardRequest struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

func codeDashboardPath(id string) string {
	return "/users/me/code-dashboards/" + url.PathEscape(id)
}

// ListCodeDashboards calls GET /users/me/code-dashboards. Never nil on success.
func (c *CustomerEntityClient) ListCodeDashboards(ctx context.Context) ([]CodeDashboardSummary, error) {
	raw, err := c.do(ctx, http.MethodGet, "/users/me/code-dashboards", nil)
	if err != nil {
		return nil, err
	}
	var resp codeDashboardListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("entity: decode code dashboard list: %w", err)
	}
	if resp.Dashboards == nil {
		resp.Dashboards = []CodeDashboardSummary{}
	}
	return resp.Dashboards, nil
}

// GetCodeDashboard calls GET /users/me/code-dashboards/{id}. Another user's
// dashboard is a 404 *apierror.Error, the same as a missing one.
func (c *CustomerEntityClient) GetCodeDashboard(ctx context.Context, id string) (CodeDashboard, error) {
	raw, err := c.do(ctx, http.MethodGet, codeDashboardPath(id), nil)
	if err != nil {
		return CodeDashboard{}, err
	}
	var d CodeDashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return CodeDashboard{}, fmt.Errorf("entity: decode code dashboard: %w", err)
	}
	return d, nil
}

// SaveCodeDashboard calls PUT /users/me/code-dashboards/{id}: creates the
// dashboard, or replaces it when the caller already has one with this id.
// A taken name is a 409 *apierror.Error; another user's id is a 404.
func (c *CustomerEntityClient) SaveCodeDashboard(ctx context.Context, id, name, source string) (CodeDashboard, error) {
	body, err := json.Marshal(saveCodeDashboardRequest{Name: name, Source: source})
	if err != nil {
		return CodeDashboard{}, fmt.Errorf("entity: encode code dashboard: %w", err)
	}
	raw, err := c.do(ctx, http.MethodPut, codeDashboardPath(id), body)
	if err != nil {
		return CodeDashboard{}, err
	}
	var d CodeDashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return CodeDashboard{}, fmt.Errorf("entity: decode saved code dashboard: %w", err)
	}
	return d, nil
}

// DeleteCodeDashboard calls DELETE /users/me/code-dashboards/{id} (204).
func (c *CustomerEntityClient) DeleteCodeDashboard(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, codeDashboardPath(id), nil)
	return err
}

// SharedCodeDashboard is one code dashboard shared with the caller, with its
// source (the owner's current version, view only). Owner, SharedViaTeam and
// SharedViaLink as in SharedCustomDashboardSummary, which is also the shape of
// each entry of ListSharedCodeDashboards. The sharing calls below answer the
// custom dashboard share, team share and link types: entity-service's code
// dashboard routes answer the same bodies.
type SharedCodeDashboard struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Source        string                 `json:"source"`
	Owner         *CustomDashboardPerson `json:"owner"`
	SharedViaTeam *CustomDashboardTeam   `json:"sharedViaTeam"`
	SharedViaLink bool                   `json:"sharedViaLink"`
	SharedOn      string                 `json:"sharedOn"`
	UpdatedOn     string                 `json:"updatedOn"`
}

// ListCodeDashboardShares calls GET /users/me/code-dashboards/{id}/shares
// (owner only: another user's dashboard is a 404). Never nil on success.
func (c *CustomerEntityClient) ListCodeDashboardShares(ctx context.Context, id string) ([]CustomDashboardShare, error) {
	raw, err := c.do(ctx, http.MethodGet, codeDashboardPath(id)+"/shares", nil)
	if err != nil {
		return nil, err
	}
	return decodeShareList(raw)
}

// AddCodeDashboardShare calls PUT /users/me/code-dashboards/{id}/shares/{userId}
// (no body) and returns the shares after the change; the same 400s and 404s
// as AddCustomDashboardShare.
func (c *CustomerEntityClient) AddCodeDashboardShare(ctx context.Context, id, userID string) ([]CustomDashboardShare, error) {
	raw, err := c.do(ctx, http.MethodPut, codeDashboardPath(id)+"/shares/"+url.PathEscape(userID), nil)
	if err != nil {
		return nil, err
	}
	return decodeShareList(raw)
}

// RemoveCodeDashboardShare calls DELETE /users/me/code-dashboards/{id}/shares/{userId} (204).
func (c *CustomerEntityClient) RemoveCodeDashboardShare(ctx context.Context, id, userID string) error {
	_, err := c.do(ctx, http.MethodDelete, codeDashboardPath(id)+"/shares/"+url.PathEscape(userID), nil)
	return err
}

// ListCodeDashboardTeamShares calls GET /users/me/code-dashboards/{id}/team-shares.
// Never nil on success.
func (c *CustomerEntityClient) ListCodeDashboardTeamShares(ctx context.Context, id string) ([]CustomDashboardTeamShare, error) {
	raw, err := c.do(ctx, http.MethodGet, codeDashboardPath(id)+"/team-shares", nil)
	if err != nil {
		return nil, err
	}
	return decodeTeamShareList(raw)
}

// AddCodeDashboardTeamShare calls PUT /users/me/code-dashboards/{id}/team-shares/{teamId}
// (no body) and returns the team shares after the change.
func (c *CustomerEntityClient) AddCodeDashboardTeamShare(ctx context.Context, id, teamID string) ([]CustomDashboardTeamShare, error) {
	raw, err := c.do(ctx, http.MethodPut, codeDashboardPath(id)+"/team-shares/"+url.PathEscape(teamID), nil)
	if err != nil {
		return nil, err
	}
	return decodeTeamShareList(raw)
}

// RemoveCodeDashboardTeamShare calls DELETE /users/me/code-dashboards/{id}/team-shares/{teamId} (204).
func (c *CustomerEntityClient) RemoveCodeDashboardTeamShare(ctx context.Context, id, teamID string) error {
	_, err := c.do(ctx, http.MethodDelete, codeDashboardPath(id)+"/team-shares/"+url.PathEscape(teamID), nil)
	return err
}

// GetCodeDashboardLink calls GET /users/me/code-dashboards/{id}/link (owner only).
func (c *CustomerEntityClient) GetCodeDashboardLink(ctx context.Context, id string) (CustomDashboardLink, error) {
	raw, err := c.do(ctx, http.MethodGet, codeDashboardPath(id)+"/link", nil)
	if err != nil {
		return CustomDashboardLink{}, err
	}
	return decodeCustomDashboardLink(raw)
}

// EnableCodeDashboardLink calls PUT /users/me/code-dashboards/{id}/link (no
// body) and returns the link; a link that is already on keeps its id.
func (c *CustomerEntityClient) EnableCodeDashboardLink(ctx context.Context, id string) (CustomDashboardLink, error) {
	raw, err := c.do(ctx, http.MethodPut, codeDashboardPath(id)+"/link", nil)
	if err != nil {
		return CustomDashboardLink{}, err
	}
	return decodeCustomDashboardLink(raw)
}

// DisableCodeDashboardLink calls DELETE /users/me/code-dashboards/{id}/link (204).
func (c *CustomerEntityClient) DisableCodeDashboardLink(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, codeDashboardPath(id)+"/link", nil)
	return err
}

// OpenCodeDashboardLink calls POST /users/me/shared-code-dashboards/link with
// the link id in the body (never the path, so it stays out of access logs).
// An unknown or turned-off link is a 404 *apierror.Error.
func (c *CustomerEntityClient) OpenCodeDashboardLink(ctx context.Context, linkID string) (OpenedCustomDashboardLink, error) {
	body, err := json.Marshal(openCustomDashboardLinkRequest{LinkID: linkID})
	if err != nil {
		return OpenedCustomDashboardLink{}, fmt.Errorf("entity: encode code dashboard link: %w", err)
	}
	raw, err := c.do(ctx, http.MethodPost, "/users/me/shared-code-dashboards/link", body)
	if err != nil {
		return OpenedCustomDashboardLink{}, err
	}
	var o OpenedCustomDashboardLink
	if err := json.Unmarshal(raw, &o); err != nil {
		return OpenedCustomDashboardLink{}, fmt.Errorf("entity: decode opened code dashboard link: %w", err)
	}
	return o, nil
}

// ListSharedCodeDashboards calls GET /users/me/shared-code-dashboards. Never nil on success.
func (c *CustomerEntityClient) ListSharedCodeDashboards(ctx context.Context) ([]SharedCustomDashboardSummary, error) {
	raw, err := c.do(ctx, http.MethodGet, "/users/me/shared-code-dashboards", nil)
	if err != nil {
		return nil, err
	}
	var resp sharedCustomDashboardListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("entity: decode shared code dashboards: %w", err)
	}
	if resp.Dashboards == nil {
		resp.Dashboards = []SharedCustomDashboardSummary{}
	}
	return resp.Dashboards, nil
}

// GetSharedCodeDashboard calls GET /users/me/shared-code-dashboards/{id}; a
// dashboard not shared with the caller is a 404 *apierror.Error.
func (c *CustomerEntityClient) GetSharedCodeDashboard(ctx context.Context, id string) (SharedCodeDashboard, error) {
	raw, err := c.do(ctx, http.MethodGet, "/users/me/shared-code-dashboards/"+url.PathEscape(id), nil)
	if err != nil {
		return SharedCodeDashboard{}, err
	}
	var d SharedCodeDashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return SharedCodeDashboard{}, fmt.Errorf("entity: decode shared code dashboard: %w", err)
	}
	return d, nil
}
