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

package domain

import "time"

// Limits on a saved custom dashboard. The name and definition limits match
// the CHECK constraints of migration 0251.
const (
	// MaxCustomDashboards is the cap per owner, as MaxSavedFilterViews.
	MaxCustomDashboards = 50
	// MaxCustomDashboardNameLength is in characters (runes).
	MaxCustomDashboardNameLength = 255
	// MaxCustomDashboardDefinitionBytes is in bytes.
	MaxCustomDashboardDefinitionBytes = 65536
)

// CustomDashboard is a dashboard a CSM portal user generated and saved
// (migration 0251). Definition is the dashboard JSON as opaque text, like
// SavedFilterView.Qs: the CSM backend validates it, this service does not
// parse it. The owner is never on the wire; it is always the caller.
type CustomDashboard struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Definition string    `json:"definition"`
	CreatedOn  time.Time `json:"createdOn"`
	UpdatedOn  time.Time `json:"updatedOn"`
}

// CustomDashboardSummary is one entry of a list: everything but the
// definition, so a list needs no JSON.
type CustomDashboardSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedOn time.Time `json:"createdOn"`
	UpdatedOn time.Time `json:"updatedOn"`
}

// CustomDashboardList is the body of GET /users/me/dashboards.
type CustomDashboardList struct {
	Dashboards []CustomDashboardSummary `json:"dashboards"`
}

// SaveCustomDashboardRequest is the body of PUT /users/me/dashboards/{id}:
// the dashboard is created, or replaced when the caller already has one with
// that id. The id comes from the path.
type SaveCustomDashboardRequest struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

// CustomDashboardShare is one person a saved dashboard is shared with
// (migration 0252), as its owner sees it: GET /users/me/dashboards/{id}/shares.
// A share only lets that person view the dashboard. Only the owner lists or
// changes a dashboard's shares; a recipient never sees who else has it.
type CustomDashboardShare struct {
	User     UserReference `json:"user"`
	SharedOn time.Time     `json:"sharedOn"`
}

// CustomDashboardShareList is the body of GET /users/me/dashboards/{id}/shares.
type CustomDashboardShareList struct {
	Shares []CustomDashboardShare `json:"shares"`
}

// CustomDashboardTeamShareTeam is the team a dashboard is shared with: the
// `team` row's id, name and type (e.g. "cre-abt", "sre-abt").
type CustomDashboardTeamShareTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// CustomDashboardTeamShare is one team a saved dashboard is shared with
// (migration 0253), as its owner sees it: GET
// /users/me/dashboards/{id}/team-shares. Whoever is in the team when the
// dashboard is listed may view it.
type CustomDashboardTeamShare struct {
	Team     CustomDashboardTeamShareTeam `json:"team"`
	SharedOn time.Time                    `json:"sharedOn"`
}

// CustomDashboardTeamShareList is the body of GET /users/me/dashboards/{id}/team-shares.
type CustomDashboardTeamShareList struct {
	Shares []CustomDashboardTeamShare `json:"shares"`
}

// SharedCustomDashboardSummary is one dashboard another user shared with the
// caller, as listed in GET /users/me/shared-dashboards: no definition. Owner
// is who shared it (the dashboard's owner), nil once that user is deleted
// (migration 0255). SharedViaTeam is the caller's team
// it was shared with, nil when it was shared with the caller directly (a
// direct share wins when there are both; with several teams, the first by
// name). SharedOn is that share's date.
type SharedCustomDashboardSummary struct {
	ID            string                        `json:"id"`
	Name          string                        `json:"name"`
	Owner         *UserReference                `json:"owner"`
	SharedViaTeam *CustomDashboardTeamShareTeam `json:"sharedViaTeam"`
	SharedOn      time.Time                     `json:"sharedOn"`
	UpdatedOn     time.Time                     `json:"updatedOn"`
}

// SharedCustomDashboardList is the body of GET /users/me/shared-dashboards.
type SharedCustomDashboardList struct {
	Dashboards []SharedCustomDashboardSummary `json:"dashboards"`
}

// SharedCustomDashboard is one dashboard shared with the caller, with its
// definition (GET /users/me/shared-dashboards/{id}): the owner's current
// version, read only. Definition is opaque text, as in CustomDashboard.
// Owner and SharedViaTeam as in SharedCustomDashboardSummary.
type SharedCustomDashboard struct {
	ID            string                        `json:"id"`
	Name          string                        `json:"name"`
	Definition    string                        `json:"definition"`
	Owner         *UserReference                `json:"owner"`
	SharedViaTeam *CustomDashboardTeamShareTeam `json:"sharedViaTeam"`
	SharedOn      time.Time                     `json:"sharedOn"`
	UpdatedOn     time.Time                     `json:"updatedOn"`
}
