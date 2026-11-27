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

// SreAlertIncidentFilter narrows a search to alerts with or without an incident.
type SreAlertIncidentFilter string

const (
	SreAlertIncidentAll      SreAlertIncidentFilter = "all"
	SreAlertIncidentLinked   SreAlertIncidentFilter = "linked"
	SreAlertIncidentUnlinked SreAlertIncidentFilter = "unlinked"
)

// SreAlertSearchFilters narrows POST /sre-alerts/search; every list is any-of and an empty one matches all.
type SreAlertSearchFilters struct {
	SearchQuery  string                 `json:"searchQuery,omitempty"`
	Environments []string               `json:"environments,omitempty"`
	Services     []string               `json:"services,omitempty"`
	Severities   []string               `json:"severities,omitempty"`
	Sources      []string               `json:"sources,omitempty"`
	Tiers        []AlertTier            `json:"tiers,omitempty"`
	Incident     SreAlertIncidentFilter `json:"incident,omitempty"`
	// IncidentID narrows to the alerts of one incident, for the incident page's Alerts tab.
	IncidentID string `json:"incidentId,omitempty"`
	// ReceivedFrom is inclusive and ReceivedTo exclusive.
	ReceivedFrom *time.Time `json:"receivedFrom,omitempty"`
	ReceivedTo   *time.Time `json:"receivedTo,omitempty"`
}

// SreAlertSort orders a search; Field is receivedOn, number, severity, environment or service, Order asc or desc.
type SreAlertSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// SearchSreAlertsRequest is the body of POST /sre-alerts/search.
type SearchSreAlertsRequest struct {
	Filters    SreAlertSearchFilters `json:"filters"`
	SortBy     *SreAlertSort         `json:"sortBy,omitempty"`
	Pagination *Pagination           `json:"pagination,omitempty"`
}

// SreAlertIncidentRef is the incident an alert belongs to (alert.incident_id), with what a reader needs to recognise it.
type SreAlertIncidentRef struct {
	ID       string     `json:"id"`
	Number   string     `json:"number"`
	Subject  string     `json:"subject"`
	State    string     `json:"state"`
	Priority string     `json:"priority"`
	Impact   string     `json:"impact"`
	Urgency  string     `json:"urgency"`
	OpenedOn *time.Time `json:"openedOn"`
}

// SreAlertListItem is one alert in a search result: its UUID, ALT number and the fields a list shows.
type SreAlertListItem struct {
	ID          string               `json:"id"`
	Number      string               `json:"number"`
	Source      string               `json:"source"`
	Service     string               `json:"service"`
	Environment string               `json:"environment"`
	MetricName  string               `json:"metricName"`
	Severity    string               `json:"severity"`
	Category    string               `json:"category"`
	Tier        AlertTier            `json:"tier"`
	ReceivedOn  time.Time            `json:"receivedOn"`
	CreatedOn   time.Time            `json:"createdOn"`
	Incident    *SreAlertIncidentRef `json:"incident"`
}

// SearchSreAlertsResponse is a page of alerts with the total that match.
type SearchSreAlertsResponse struct {
	Alerts []SreAlertListItem `json:"alerts"`
	Total  int                `json:"total"`
	Offset int                `json:"offset"`
	Limit  int                `json:"limit"`
}

// SreAlertDetail is one alert with every stored field.
type SreAlertDetail struct {
	SreAlertListItem
	UniqueIdentifier string `json:"uniqueIdentifier"`
	Description      string `json:"description"`
	AssignmentGroup  string `json:"assignmentGroup"`
	// RawBody is the source's request body as text; only the message's first alert keeps it.
	RawBody *string `json:"rawBody"`
}

// SreAlertFacets are the values the alert filters offer, from recent alerts.
type SreAlertFacets struct {
	Environments []string `json:"environments"`
	Services     []string `json:"services"`
	Severities   []string `json:"severities"`
	Sources      []string `json:"sources"`
}
