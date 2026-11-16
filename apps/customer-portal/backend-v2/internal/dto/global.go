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

import "github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"

// FeedbackEmojiChip is the portal's own copy of entity.FeedbackEmojiChip.
type FeedbackEmojiChip struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// FeedbackEmoji is the portal's own copy of entity.FeedbackEmoji.
type FeedbackEmoji struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Value           string              `json:"value"`
	UnselectedImage string              `json:"unselectedImage"`
	SelectedImage   string              `json:"selectedImage"`
	Chips           []FeedbackEmojiChip `json:"chips"`
}

func mapFeedbackEmojis(emojis []entity.FeedbackEmoji) []FeedbackEmoji {
	out := make([]FeedbackEmoji, 0, len(emojis))
	for _, e := range emojis {
		chips := make([]FeedbackEmojiChip, 0, len(e.Chips))
		for _, c := range e.Chips {
			chips = append(chips, FeedbackEmojiChip{ID: c.ID, Name: c.Name, Value: c.Value})
		}
		out = append(out, FeedbackEmoji{
			ID:              e.ID,
			Name:            e.Name,
			Value:           e.Value,
			UnselectedImage: e.UnselectedImage,
			SelectedImage:   e.SelectedImage,
			Chips:           chips,
		})
	}
	return out
}

// FeatureFlags is a portal-only value that never comes from entity-service
// at all, injected directly into the metadata response. Hardcoded to this
// API's own default; if this needs to vary per environment, wire it to an
// env var the same way other configurable values in this backend are read
// in cmd/server/main.go.
type FeatureFlags struct {
	UsageMetricsEnabled bool `json:"usageMetricsEnabled"`
}

var defaultFeatureFlags = FeatureFlags{UsageMetricsEnabled: true}

// MetadataResponse is the portal's response for GET /metadata.
type MetadataResponse struct {
	TimeZones      []ReferenceItem `json:"timeZones"`
	ProjectTypes   []ReferenceItem `json:"projectTypes"`
	FeedbackEmojis []FeedbackEmoji `json:"feedbackEmojies"`
	FeatureFlags   FeatureFlags    `json:"featureFlags"`
}

// MapMetadataResponse builds the portal response from entity-service's
// SystemMetadataResponse.
func MapMetadataResponse(r entity.SystemMetadataResponse) MetadataResponse {
	return MetadataResponse{
		TimeZones:      mapChoiceListItems(r.TimeZones),
		ProjectTypes:   mapReferenceTableItems(r.ProjectTypes),
		FeedbackEmojis: mapFeedbackEmojis(r.FeedbackEmojis),
		FeatureFlags:   defaultFeatureFlags,
	}
}

// globalSearchProjectsMaxLimit caps GlobalSearchRequest.ProjectsPagination.Limit
// at this API's own maximum.
const globalSearchProjectsMaxLimit = 50

// GlobalSearchFilters holds the optional filter criteria for POST /search.
// Types is renamed to entity-service's "tables" field on the way out.
type GlobalSearchFilters struct {
	SearchQuery string   `json:"searchQuery,omitempty"`
	Types       []string `json:"types,omitempty"`
}

// GlobalSearchSort specifies the sort field/order for POST /search.
type GlobalSearchSort struct {
	Field string `json:"field,omitempty"`
	Order string `json:"order,omitempty"`
}

// GlobalSearchRequest is the portal's request body for POST /search.
type GlobalSearchRequest struct {
	Filters            *GlobalSearchFilters `json:"filters,omitempty"`
	SortBy             *GlobalSearchSort    `json:"sortBy,omitempty"`
	ProjectsPagination *entity.Pagination   `json:"projectsPagination,omitempty"`
	CasesPagination    *entity.Pagination   `json:"casesPagination,omitempty"`
}

// BuildEntityGlobalSearchRequest translates the portal's global-search
// request into entity-service's shape: renames filters.types to entity's
// filters.tables and caps projectsPagination.limit at
// globalSearchProjectsMaxLimit.
func BuildEntityGlobalSearchRequest(req GlobalSearchRequest) entity.GlobalSearchRequest {
	out := entity.GlobalSearchRequest{}
	if req.Filters != nil {
		out.Filters = &entity.GlobalSearchFilters{SearchQuery: req.Filters.SearchQuery, Tables: req.Filters.Types}
	}
	if req.SortBy != nil {
		out.SortBy = &entity.GlobalSearchSort{Field: req.SortBy.Field, Order: req.SortBy.Order}
	}
	if req.ProjectsPagination != nil {
		p := *req.ProjectsPagination
		if p.Limit > globalSearchProjectsMaxLimit {
			p.Limit = globalSearchProjectsMaxLimit
		}
		out.ProjectsPagination = &p
	}
	out.CasesPagination = req.CasesPagination
	return out
}

// GlobalSearchProject is a project row in a global search result.
type GlobalSearchProject struct {
	ID                  string        `json:"id"`
	Name                string        `json:"name"`
	Description         *string       `json:"description"`
	Key                 string        `json:"key"`
	Type                ReferenceItem `json:"type"`
	CreatedOn           string        `json:"createdOn"`
	StartDate           *string       `json:"startDate"`
	EndDate             *string       `json:"endDate"`
	HasPdpSubscription  bool          `json:"hasPdpSubscription"`
	ClosureState        *string       `json:"closureState"`
	Account             ReferenceItem `json:"account"`
	ActiveChatsCount    int           `json:"activeChatsCount"`
	ActionRequiredCount int           `json:"actionRequiredCount"`
	OutstandingCount    int           `json:"outstandingCount"`
}

// GlobalSearchCase is a case row in a global search result.
type GlobalSearchCase struct {
	ID               string         `json:"id"`
	InternalID       string         `json:"internalId"`
	Number           string         `json:"number"`
	Title            *string        `json:"title"`
	Description      *string        `json:"description"`
	CreatedOn        string         `json:"createdOn"`
	CreatedBy        string         `json:"createdBy"`
	UpdatedOn        string         `json:"updatedOn"`
	Project          *ReferenceItem `json:"project"`
	CaseType         *ReferenceItem `json:"caseType"`
	State            *ReferenceItem `json:"state"`
	Severity         *ReferenceItem `json:"severity"`
	AssignedEngineer *ReferenceItem `json:"assignedEngineer"`
	Account          ReferenceItem  `json:"account"`
}

// GlobalSearchResponse is the portal's response for POST /search.
type GlobalSearchResponse struct {
	Query         string                `json:"query"`
	ProjectsTotal int                   `json:"projectsTotal"`
	CasesTotal    int                   `json:"casesTotal"`
	Projects      []GlobalSearchProject `json:"projects"`
	Cases         []GlobalSearchCase    `json:"cases"`
}

// globalSearchCaseState is a case's state in a global search result. Work In
// Progress and Waiting On WSO2 become the merged In Progress state (matched on
// the label or the id, since on the Postgres data source entity-service sends
// the raw enum as both); every other state is passed on as entity-service sent
// it, so nothing else about global search changes.
func globalSearchCaseState(state entity.ChoiceListItem) *ReferenceItem {
	if inProgressFamilyFromText(state.Label) || inProgressFamilyFromText(state.ID) {
		return &ReferenceItem{ID: caseStateInProgressID, Label: caseStateInProgressLabel}
	}
	return &ReferenceItem{ID: state.ID, Label: state.Label}
}

// MapGlobalSearchResponse builds the portal response from entity-service's
// GlobalSearchResponse.
func MapGlobalSearchResponse(r entity.GlobalSearchResponse) GlobalSearchResponse {
	projects := make([]GlobalSearchProject, 0, len(r.Projects))
	for _, p := range r.Projects {
		projects = append(projects, GlobalSearchProject{
			ID:                  p.ID,
			Name:                p.Name,
			Description:         p.Description,
			Key:                 p.Key,
			Type:                ReferenceItem{ID: p.Type.ID, Label: p.Type.Name},
			CreatedOn:           p.CreatedOn,
			StartDate:           p.StartDate,
			EndDate:             p.EndDate,
			HasPdpSubscription:  p.HasPdpSubscription,
			ClosureState:        p.ClosureState,
			Account:             ReferenceItem{ID: p.Account.ID, Label: p.Account.Name},
			ActiveChatsCount:    p.ActiveChatsCount,
			ActionRequiredCount: p.ActionRequiredCount,
			OutstandingCount:    p.OutstandingCount,
		})
	}

	cases := make([]GlobalSearchCase, 0, len(r.Cases))
	for _, c := range r.Cases {
		out := GlobalSearchCase{
			ID:          c.ID,
			InternalID:  c.InternalID,
			Number:      c.Number,
			Title:       c.Title,
			Description: c.Description,
			CreatedOn:   c.CreatedOn,
			CreatedBy:   c.CreatedBy,
			UpdatedOn:   c.UpdatedOn,
			Account:     ReferenceItem{ID: c.Account.ID, Label: c.Account.Name},
		}
		if c.Project != nil {
			out.Project = &ReferenceItem{ID: c.Project.ID, Label: c.Project.Name}
		}
		if c.CaseType != nil {
			out.CaseType = &ReferenceItem{ID: c.CaseType.ID, Label: c.CaseType.Name}
		}
		if c.State != nil {
			out.State = globalSearchCaseState(*c.State)
		}
		if c.Severity != nil {
			out.Severity = &ReferenceItem{ID: c.Severity.ID, Label: c.Severity.Label}
		}
		if c.AssignedEngineer != nil {
			out.AssignedEngineer = &ReferenceItem{ID: c.AssignedEngineer.ID, Label: c.AssignedEngineer.Name}
		}
		cases = append(cases, out)
	}

	return GlobalSearchResponse{
		Query:         r.Query,
		ProjectsTotal: r.ProjectsTotal,
		CasesTotal:    r.CasesTotal,
		Projects:      projects,
		Cases:         cases,
	}
}

// VulnerabilityMetaResponse is the portal's response for
// GET /products/vulnerabilities/meta.
type VulnerabilityMetaResponse struct {
	Severities []ReferenceItem              `json:"severities"`
	Products   []VulnerabilityProductOption `json:"products"`
}

// VulnerabilityProductOption is one product name with the distinct versions
// available for it, used to populate the product/version search filters.
type VulnerabilityProductOption struct {
	Name     string   `json:"name"`
	Versions []string `json:"versions"`
}

// MapVulnerabilityMeta builds the portal response from entity-service's
// VulnerabilityMetaResponse.
func MapVulnerabilityMeta(r entity.VulnerabilityMetaResponse) VulnerabilityMetaResponse {
	// products and versions are always arrays (never null), even when
	// entity-service omits them.
	products := make([]VulnerabilityProductOption, 0, len(r.Products))
	for _, p := range r.Products {
		versions := make([]string, 0, len(p.Versions))
		versions = append(versions, p.Versions...)
		products = append(products, VulnerabilityProductOption{Name: p.Name, Versions: versions})
	}
	return VulnerabilityMetaResponse{
		Severities: mapChoiceListItems(r.Severities),
		Products:   products,
	}
}
