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

// CustomDashboard is a dashboard the caller saved (entity-service
// /users/me/dashboards). Definition is the dashboard JSON as text; this
// backend validates it, entity-service stores it without parsing it. The
// owner is always the caller of the x-user-id-token, so it is not here.
type CustomDashboard struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
	CreatedOn  string `json:"createdOn"`
	UpdatedOn  string `json:"updatedOn"`
}

// CustomDashboardSummary is one entry of the caller's list: no definition.
type CustomDashboardSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedOn string `json:"createdOn"`
	UpdatedOn string `json:"updatedOn"`
}

type customDashboardListResponse struct {
	Dashboards []CustomDashboardSummary `json:"dashboards"`
}

type saveCustomDashboardRequest struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

func customDashboardPath(id string) string {
	return "/users/me/dashboards/" + url.PathEscape(id)
}

// ListCustomDashboards calls GET /users/me/dashboards. Never nil on success.
func (c *CustomerEntityClient) ListCustomDashboards(ctx context.Context) ([]CustomDashboardSummary, error) {
	raw, err := c.do(ctx, http.MethodGet, "/users/me/dashboards", nil)
	if err != nil {
		return nil, err
	}
	var resp customDashboardListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("entity: decode custom dashboard list: %w", err)
	}
	if resp.Dashboards == nil {
		resp.Dashboards = []CustomDashboardSummary{}
	}
	return resp.Dashboards, nil
}

// GetCustomDashboard calls GET /users/me/dashboards/{id}. Another user's
// dashboard is a 404 *apierror.Error, the same as a missing one.
func (c *CustomerEntityClient) GetCustomDashboard(ctx context.Context, id string) (CustomDashboard, error) {
	raw, err := c.do(ctx, http.MethodGet, customDashboardPath(id), nil)
	if err != nil {
		return CustomDashboard{}, err
	}
	var d CustomDashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return CustomDashboard{}, fmt.Errorf("entity: decode custom dashboard: %w", err)
	}
	return d, nil
}

// SaveCustomDashboard calls PUT /users/me/dashboards/{id}: creates the
// dashboard, or replaces it when the caller already has one with this id.
// A taken name is a 409 *apierror.Error; another user's id is a 404.
func (c *CustomerEntityClient) SaveCustomDashboard(ctx context.Context, id, name, definition string) (CustomDashboard, error) {
	body, err := json.Marshal(saveCustomDashboardRequest{Name: name, Definition: definition})
	if err != nil {
		return CustomDashboard{}, fmt.Errorf("entity: encode custom dashboard: %w", err)
	}
	raw, err := c.do(ctx, http.MethodPut, customDashboardPath(id), body)
	if err != nil {
		return CustomDashboard{}, err
	}
	var d CustomDashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return CustomDashboard{}, fmt.Errorf("entity: decode saved custom dashboard: %w", err)
	}
	return d, nil
}

// DeleteCustomDashboard calls DELETE /users/me/dashboards/{id} (204).
func (c *CustomerEntityClient) DeleteCustomDashboard(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, customDashboardPath(id), nil)
	return err
}

// CustomDashboardPerson is a person on a dashboard share (entity-service's
// UserReference): the recipient on the owner's list, or the owner on a
// recipient's.
type CustomDashboardPerson struct {
	ID    *string `json:"id"`
	Email string  `json:"email"`
	Name  string  `json:"name"`
}

// CustomDashboardShare is one person a saved dashboard is shared with, as its
// owner sees it.
type CustomDashboardShare struct {
	User     CustomDashboardPerson `json:"user"`
	SharedOn string                `json:"sharedOn"`
}

type customDashboardShareListResponse struct {
	Shares []CustomDashboardShare `json:"shares"`
}

// CustomDashboardTeam is a team a dashboard is shared with, as entity-service
// names it: the team table's id (the team's group id), name and type (e.g.
// "cre-abt").
type CustomDashboardTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// CustomDashboardTeamShare is one team a saved dashboard is shared with, as
// its owner sees it.
type CustomDashboardTeamShare struct {
	Team     CustomDashboardTeam `json:"team"`
	SharedOn string              `json:"sharedOn"`
}

type customDashboardTeamShareListResponse struct {
	Shares []CustomDashboardTeamShare `json:"shares"`
}

// SharedCustomDashboardSummary is one dashboard another user shared with the
// caller, without its definition. Owner is nil once the owner's user has been
// deleted (the dashboard stays while it is shared). SharedViaTeam is the
// caller's team it was shared with; nil when it was shared with the caller
// directly.
type SharedCustomDashboardSummary struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Owner         *CustomDashboardPerson `json:"owner"`
	SharedViaTeam *CustomDashboardTeam   `json:"sharedViaTeam"`
	SharedOn      string                 `json:"sharedOn"`
	UpdatedOn     string                 `json:"updatedOn"`
}

type sharedCustomDashboardListResponse struct {
	Dashboards []SharedCustomDashboardSummary `json:"dashboards"`
}

// SharedCustomDashboard is one dashboard shared with the caller, with its
// definition (the owner's current version, view only). Owner and
// SharedViaTeam as in SharedCustomDashboardSummary.
type SharedCustomDashboard struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Definition    string                 `json:"definition"`
	Owner         *CustomDashboardPerson `json:"owner"`
	SharedViaTeam *CustomDashboardTeam   `json:"sharedViaTeam"`
	SharedOn      string                 `json:"sharedOn"`
	UpdatedOn     string                 `json:"updatedOn"`
}

func customDashboardSharePath(id, userID string) string {
	return customDashboardPath(id) + "/shares/" + url.PathEscape(userID)
}

// decodeShareList reads entity-service's share list response; never nil.
func decodeShareList(raw []byte) ([]CustomDashboardShare, error) {
	var resp customDashboardShareListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("entity: decode custom dashboard shares: %w", err)
	}
	if resp.Shares == nil {
		resp.Shares = []CustomDashboardShare{}
	}
	return resp.Shares, nil
}

// ListCustomDashboardShares calls GET /users/me/dashboards/{id}/shares (owner
// only: another user's dashboard is a 404). Never nil on success.
func (c *CustomerEntityClient) ListCustomDashboardShares(ctx context.Context, id string) ([]CustomDashboardShare, error) {
	raw, err := c.do(ctx, http.MethodGet, customDashboardPath(id)+"/shares", nil)
	if err != nil {
		return nil, err
	}
	return decodeShareList(raw)
}

// AddCustomDashboardShare calls PUT /users/me/dashboards/{id}/shares/{userId}
// (no body) and returns the dashboard's shares after the change. Sharing with
// the caller or with someone who is not internal staff is a 400; an unknown
// user or dashboard a 404.
func (c *CustomerEntityClient) AddCustomDashboardShare(ctx context.Context, id, userID string) ([]CustomDashboardShare, error) {
	raw, err := c.do(ctx, http.MethodPut, customDashboardSharePath(id, userID), nil)
	if err != nil {
		return nil, err
	}
	return decodeShareList(raw)
}

// RemoveCustomDashboardShare calls DELETE /users/me/dashboards/{id}/shares/{userId}
// (204); not shared with that user is a 404.
func (c *CustomerEntityClient) RemoveCustomDashboardShare(ctx context.Context, id, userID string) error {
	_, err := c.do(ctx, http.MethodDelete, customDashboardSharePath(id, userID), nil)
	return err
}

// ListSharedCustomDashboards calls GET /users/me/shared-dashboards. Never nil on success.
func (c *CustomerEntityClient) ListSharedCustomDashboards(ctx context.Context) ([]SharedCustomDashboardSummary, error) {
	raw, err := c.do(ctx, http.MethodGet, "/users/me/shared-dashboards", nil)
	if err != nil {
		return nil, err
	}
	var resp sharedCustomDashboardListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("entity: decode shared custom dashboards: %w", err)
	}
	if resp.Dashboards == nil {
		resp.Dashboards = []SharedCustomDashboardSummary{}
	}
	return resp.Dashboards, nil
}

// GetSharedCustomDashboard calls GET /users/me/shared-dashboards/{id}; a
// dashboard not shared with the caller is a 404 *apierror.Error.
func (c *CustomerEntityClient) GetSharedCustomDashboard(ctx context.Context, id string) (SharedCustomDashboard, error) {
	raw, err := c.do(ctx, http.MethodGet, "/users/me/shared-dashboards/"+url.PathEscape(id), nil)
	if err != nil {
		return SharedCustomDashboard{}, err
	}
	var d SharedCustomDashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return SharedCustomDashboard{}, fmt.Errorf("entity: decode shared custom dashboard: %w", err)
	}
	return d, nil
}

func customDashboardTeamSharePath(id, teamID string) string {
	return customDashboardPath(id) + "/team-shares/" + url.PathEscape(teamID)
}

// decodeTeamShareList reads entity-service's team share list response; never nil.
func decodeTeamShareList(raw []byte) ([]CustomDashboardTeamShare, error) {
	var resp customDashboardTeamShareListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("entity: decode custom dashboard team shares: %w", err)
	}
	if resp.Shares == nil {
		resp.Shares = []CustomDashboardTeamShare{}
	}
	return resp.Shares, nil
}

// ListCustomDashboardTeamShares calls GET /users/me/dashboards/{id}/team-shares
// (owner only: another user's dashboard is a 404). Never nil on success.
func (c *CustomerEntityClient) ListCustomDashboardTeamShares(ctx context.Context, id string) ([]CustomDashboardTeamShare, error) {
	raw, err := c.do(ctx, http.MethodGet, customDashboardPath(id)+"/team-shares", nil)
	if err != nil {
		return nil, err
	}
	return decodeTeamShareList(raw)
}

// AddCustomDashboardTeamShare calls PUT /users/me/dashboards/{id}/team-shares/{teamId}
// (no body) and returns the dashboard's team shares after the change. An
// unknown team or dashboard is a 404.
func (c *CustomerEntityClient) AddCustomDashboardTeamShare(ctx context.Context, id, teamID string) ([]CustomDashboardTeamShare, error) {
	raw, err := c.do(ctx, http.MethodPut, customDashboardTeamSharePath(id, teamID), nil)
	if err != nil {
		return nil, err
	}
	return decodeTeamShareList(raw)
}

// RemoveCustomDashboardTeamShare calls DELETE /users/me/dashboards/{id}/team-shares/{teamId}
// (204); not shared with that team is a 404.
func (c *CustomerEntityClient) RemoveCustomDashboardTeamShare(ctx context.Context, id, teamID string) error {
	_, err := c.do(ctx, http.MethodDelete, customDashboardTeamSharePath(id, teamID), nil)
	return err
}
