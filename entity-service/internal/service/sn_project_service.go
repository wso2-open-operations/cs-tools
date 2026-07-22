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

// Package service is declared in interfaces.go.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snProjectsResponse mirrors the Choreo POST /projects/search response.
type snProjectsResponse struct {
	Projects     []snProject `json:"projects"`
	TotalRecords int         `json:"totalRecords"`
	Offset       int         `json:"offset"`
	Limit        int         `json:"limit"`
}

// snProjectClosureFields groups the closure-related fields shared by
// snProject and snProjectDetailsResponse; ServiceNow specific, no Postgres
// equivalent. Embedded anonymously; JSON field names are unaffected.
type snProjectClosureFields struct {
	ClosureState                    *string `json:"closureState"`
	EndDateClosureState             *string `json:"endDateClosureState"`
	InvoiceDueDateClosureState      *string `json:"invoiceDueDateClosureState"`
	ComplianceViolationClosureState *string `json:"complianceViolationClosureState"`
	ComplianceViolationDate         *string `json:"complianceViolationDate"`
}

type snProject struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Key       string        `json:"key"`
	Type      snProjectType `json:"type"`
	EndDate   string        `json:"endDate"`
	CreatedOn string        `json:"createdOn"`
	snProjectClosureFields
}

type snProjectType struct {
	Name string `json:"name"`
}

// snSearchProjectsPayload is the Choreo POST /projects/search request body.
type snSearchProjectsPayload struct {
	Filters    snProjectFilters    `json:"filters"`
	Pagination snProjectPagination `json:"pagination"`
}

type snProjectFilters struct {
	SearchQuery   string `json:"searchQuery,omitempty"`
	ClosureStatus string `json:"closureStatus,omitempty"`
	EndDateFrom   string `json:"endDateFrom,omitempty"`
	EndDateTo     string `json:"endDateTo,omitempty"`
	SortBy        string `json:"sortBy,omitempty"`
	SortOrder     string `json:"sortOrder,omitempty"`
}

type snProjectPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// snCreatedOnLayout is the datetime format used by the Choreo API.
const snCreatedOnLayout = "2006-01-02 15:04:05"

// snDateLayout is the date-only format used by the Choreo API for start/end dates.
const snDateLayout = "2006-01-02"

type snProjectService struct {
	client     *integrationservice.Client
	pgFallback ProjectService
}

// NewSNProjectService constructs a ProjectService backed by the Choreo API.
// pgFallback is used for GetProjectByID (no SN single-project endpoint).
func NewServiceNowProjectService(client *integrationservice.Client, pgFallback ProjectService) ProjectService {
	return &snProjectService{client: client, pgFallback: pgFallback}
}

// SearchProjects implements ProjectService. It calls the Choreo API, parses the
// response, and maps each item to domain.ProjectView so the shape is identical
// to the Postgres path.
func (s *snProjectService) SearchProjects(ctx context.Context, req domain.SearchProjectsRequest) (domain.SearchProjectsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProjectsResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchProjectsResponse{}, err
	}
	if err := validateProjectSearchFilters(req); err != nil {
		return domain.SearchProjectsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snSearchProjectsPayload{
		Filters: snProjectFilters{
			SearchQuery:   req.SearchQuery,
			ClosureStatus: req.ClosureStatus,
			EndDateFrom:   req.EndDateFrom,
			EndDateTo:     req.EndDateTo,
			SortBy:        req.SortBy,
			SortOrder:     req.SortOrder,
		},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	raw, err := s.client.Post(ctx, "/projects/search", token, payload)
	if err != nil {
		return domain.SearchProjectsResponse{}, err
	}

	var snResp snProjectsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchProjectsResponse{}, fmt.Errorf("sn projects: parse response: %w", err)
	}

	views := make([]domain.ProjectView, 0, len(snResp.Projects))
	for _, p := range snResp.Projects {
		createdOn, err := time.Parse(snCreatedOnLayout, p.CreatedOn)
		if err != nil {
			return domain.SearchProjectsResponse{}, fmt.Errorf("sn projects: parse createdOn %q: %w", p.CreatedOn, err)
		}
		subType, err := snTypeNameToSubscriptionType(p.Type.Name)
		if err != nil {
			return domain.SearchProjectsResponse{}, fmt.Errorf("sn projects: project %q: %w", p.ID, err)
		}
		var endDate *time.Time
		if p.EndDate != "" {
			parsed, err := time.Parse(snDateLayout, p.EndDate)
			if err != nil {
				return domain.SearchProjectsResponse{}, fmt.Errorf("sn projects: parse endDate %q: %w", p.EndDate, err)
			}
			endDate = &parsed
		}
		views = append(views, domain.ProjectView{
			ID:               sysidToUUID(p.ID),
			Name:             p.Name,
			Key:              p.Key,
			SubscriptionType: subType,
			EndDate:          endDate,
			CreatedOn:        createdOn,
			ProjectClosureFields: domain.ProjectClosureFields{
				ClosureState:                    p.ClosureState,
				EndDateClosureState:             p.EndDateClosureState,
				InvoiceDueDateClosureState:      p.InvoiceDueDateClosureState,
				ComplianceViolationClosureState: p.ComplianceViolationClosureState,
				ComplianceViolationDate:         p.ComplianceViolationDate,
			},
		})
	}

	total := snResp.TotalRecords
	return domain.SearchProjectsResponse{
		Projects: views,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(views) < total,
	}, nil
}

// snProjectDetailsResponse mirrors the Choreo GET /projects/{id} response.
type snProjectDetailsResponse struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Key       string           `json:"key"`
	SfID      string           `json:"sfId"`
	CreatedOn string           `json:"createdOn"`
	StartDate string           `json:"startDate"`
	EndDate   string           `json:"endDate"`
	Type      snProjectType    `json:"type"`
	Account   snProjectAccount `json:"account"`
	snProjectClosureFields
}

type snProjectAccount struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	ActivationDate  *string `json:"activationDate"`
	SupportTier     string  `json:"supportTier"`
	Region          *string `json:"region"`
	HasAgent        bool    `json:"hasAgent"`
	HasKbReferences bool    `json:"hasKbReferences"`
}

// GetProjectByID implements ProjectService by calling the Choreo GET /projects/{id} endpoint.
func (s *snProjectService) GetProjectByID(ctx context.Context, id string) (domain.ProjectDetailsView, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/projects/"+uuidToSysid(id), token)
	if err != nil {
		return domain.ProjectDetailsView{}, err
	}

	var sn snProjectDetailsResponse
	if err := json.Unmarshal(raw, &sn); err != nil {
		return domain.ProjectDetailsView{}, fmt.Errorf("sn projects: parse detail response: %w", err)
	}

	createdOn, err := time.Parse(snCreatedOnLayout, sn.CreatedOn)
	if err != nil {
		return domain.ProjectDetailsView{}, fmt.Errorf("sn projects: parse createdOn %q: %w", sn.CreatedOn, err)
	}

	startDate, err := time.Parse(snDateLayout, sn.StartDate)
	if err != nil {
		return domain.ProjectDetailsView{}, fmt.Errorf("sn projects: parse startDate %q: %w", sn.StartDate, err)
	}

	endDate, err := time.Parse(snDateLayout, sn.EndDate)
	if err != nil {
		return domain.ProjectDetailsView{}, fmt.Errorf("sn projects: parse endDate %q: %w", sn.EndDate, err)
	}

	subType, err := snTypeNameToSubscriptionType(sn.Type.Name)
	if err != nil {
		return domain.ProjectDetailsView{}, fmt.Errorf("sn projects: project %q: %w", sn.ID, err)
	}

	var activationDate *time.Time
	if sn.Account.ActivationDate != nil && *sn.Account.ActivationDate != "" {
		t, err := time.Parse(snDateLayout, *sn.Account.ActivationDate)
		if err != nil {
			return domain.ProjectDetailsView{}, fmt.Errorf("sn projects: parse account activationDate %q: %w", *sn.Account.ActivationDate, err)
		}
		activationDate = &t
	}

	return domain.ProjectDetailsView{
		ID:               sysidToUUID(sn.ID),
		SfID:             sn.SfID,
		Name:             sn.Name,
		Key:              sn.Key,
		SubscriptionType: subType,
		StartDate:        startDate,
		EndDate:          endDate,
		CreatedOn:        createdOn,
		UpdatedOn:        createdOn,
		ProjectClosureFields: domain.ProjectClosureFields{
			ClosureState:                    sn.ClosureState,
			EndDateClosureState:             sn.EndDateClosureState,
			InvoiceDueDateClosureState:      sn.InvoiceDueDateClosureState,
			ComplianceViolationClosureState: sn.ComplianceViolationClosureState,
			ComplianceViolationDate:         sn.ComplianceViolationDate,
		},
		Account: domain.ProjectAccountRef{
			ID:                  sysidToUUID(sn.Account.ID),
			Name:                sn.Account.Name,
			ActivationDate:      activationDate,
			Tier:                sn.Account.SupportTier,
			Region:              sn.Account.Region,
			AgentEnabled:        sn.Account.HasAgent,
			KbReferencesEnabled: sn.Account.HasKbReferences,
		},
	}, nil
}

// snProjectUpdatePayload is the Choreo PATCH /projects/{id} request body.
type snProjectUpdatePayload struct {
	HasAgent                        *bool   `json:"hasAgent,omitempty"`
	HasKbReferences                 *bool   `json:"hasKbReferences,omitempty"`
	EndDateClosureState             *string `json:"endDateClosureState,omitempty"`
	InvoiceDueDateClosureState      *string `json:"invoiceDueDateClosureState,omitempty"`
	ComplianceViolationClosureState *string `json:"complianceViolationClosureState,omitempty"`
}

// snProjectUpdateResponse mirrors the Choreo PATCH /projects/{id} response.
type snProjectUpdateResponse struct {
	Message string                `json:"message"`
	Project snProjectUpdateResult `json:"project"`
}

type snProjectUpdateResult struct {
	ID                              string  `json:"id"`
	UpdatedBy                       string  `json:"updatedBy"`
	UpdatedOn                       string  `json:"updatedOn"`
	ClosureState                    *string `json:"closureState"`
	EndDateClosureState             *string `json:"endDateClosureState"`
	InvoiceDueDateClosureState      *string `json:"invoiceDueDateClosureState"`
	ComplianceViolationClosureState *string `json:"complianceViolationClosureState"`
}

type snProjectUpdateService struct {
	client *integrationservice.Client
}

// NewServiceNowProjectUpdateService constructs a ProjectUpdateService backed by the Choreo API.
func NewServiceNowProjectUpdateService(client *integrationservice.Client) ProjectUpdateService {
	return &snProjectUpdateService{client: client}
}

// UpdateProject implements ProjectUpdateService by calling the Choreo PATCH
// /projects/{id} endpoint. Note: the overall closure state is not directly
// settable — SN derives it from the three closure sub-state fields via a
// business rule, so the response reflects whatever it recomputed to.
//
// Deliberately no enum validation on EndDateClosureState/InvoiceDueDateClosureState/
// ComplianceViolationClosureState (unlike ClosureStatus in SearchProjects):
// unlike the overall status, which this service's own business rule computes
// from a fixed 3-value set, these sub-states are set by an evolving SN Flow
// Designer process whose full value set isn't known here — live data has
// already shown values ("Pending Notified") beyond the small set observed
// during development. A validXxx map here would risk rejecting legitimate
// values the ACP automation needs to write.
func (s *snProjectUpdateService) UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest) (domain.ProjectUpdateResponse, error) {
	if req.HasAgent == nil && req.HasKbReferences == nil && req.EndDateClosureState == nil &&
		req.InvoiceDueDateClosureState == nil && req.ComplianceViolationClosureState == nil {
		return domain.ProjectUpdateResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snProjectUpdatePayload{
		HasAgent:                        req.HasAgent,
		HasKbReferences:                 req.HasKbReferences,
		EndDateClosureState:             req.EndDateClosureState,
		InvoiceDueDateClosureState:      req.InvoiceDueDateClosureState,
		ComplianceViolationClosureState: req.ComplianceViolationClosureState,
	}
	raw, err := s.client.Patch(ctx, "/projects/"+uuidToSysid(id), token, payload)
	if err != nil {
		return domain.ProjectUpdateResponse{}, err
	}

	var sn snProjectUpdateResponse
	if err := json.Unmarshal(raw, &sn); err != nil {
		return domain.ProjectUpdateResponse{}, fmt.Errorf("sn projects: parse update response: %w", err)
	}

	updatedOn, err := time.Parse(snCreatedOnLayout, sn.Project.UpdatedOn)
	if err != nil {
		return domain.ProjectUpdateResponse{}, fmt.Errorf("sn projects: parse updatedOn %q: %w", sn.Project.UpdatedOn, err)
	}

	return domain.ProjectUpdateResponse{
		Message: sn.Message,
		Project: domain.ProjectUpdateResult{
			ID:                              sysidToUUID(sn.Project.ID),
			UpdatedBy:                       sn.Project.UpdatedBy,
			UpdatedOn:                       updatedOn,
			ClosureState:                    sn.Project.ClosureState,
			EndDateClosureState:             sn.Project.EndDateClosureState,
			InvoiceDueDateClosureState:      sn.Project.InvoiceDueDateClosureState,
			ComplianceViolationClosureState: sn.Project.ComplianceViolationClosureState,
		},
	}, nil
}

// validClosureStatuses is the set of values the "Update WSO2 Closure State"
// SN business rule ever computes for the overall closure state — the only
// three literal strings that rule's branches assign. Safe to enforce here
// because we own that rule's logic, unlike the closure *sub-state* fields
// (see updateProject), whose value set is set by an evolving SN Flow
// Designer process and isn't fully known.
var validClosureStatuses = map[string]struct{}{
	"Open":       {},
	"Suspended":  {},
	"Restricted": {},
}

// validSortOrders is the set of accepted SortOrder values.
var validSortOrders = map[string]struct{}{
	"":     {}, // unset defaults to the service's own default ordering
	"asc":  {},
	"desc": {},
}

// validateProjectSearchFilters rejects unknown closureStatus/sortBy/sortOrder
// values and malformed end-date filters before they reach ServiceNow.
func validateProjectSearchFilters(req domain.SearchProjectsRequest) error {
	if req.ClosureStatus != "" {
		if _, ok := validClosureStatuses[req.ClosureStatus]; !ok {
			return &apierror.ValidationError{Msg: "closureStatus must be one of: Open, Suspended, Restricted"}
		}
	}
	if req.SortBy != "" && req.SortBy != "endDate" {
		return &apierror.ValidationError{Msg: `sortBy must be "endDate" if provided`}
	}
	if _, ok := validSortOrders[req.SortOrder]; !ok {
		return &apierror.ValidationError{Msg: `sortOrder must be "asc" or "desc" if provided`}
	}
	if req.EndDateFrom != "" {
		if _, err := time.Parse(snDateLayout, req.EndDateFrom); err != nil {
			return &apierror.ValidationError{Msg: "endDateFrom must be a valid date (yyyy-MM-dd)"}
		}
	}
	if req.EndDateTo != "" {
		if _, err := time.Parse(snDateLayout, req.EndDateTo); err != nil {
			return &apierror.ValidationError{Msg: "endDateTo must be a valid date (yyyy-MM-dd)"}
		}
	}
	return nil
}

// validSubscriptionTypes is the set of known SubscriptionType enum values.
var validSubscriptionTypes = map[domain.SubscriptionType]struct{}{
	domain.SubscriptionTypeDevelopmentSupport:       {},
	domain.SubscriptionTypeManagedCloudSubscription: {},
	domain.SubscriptionTypeEvaluationSubscription:   {},
	domain.SubscriptionTypeSubscription:             {},
	domain.SubscriptionTypeCloudEvaluationSupport:   {},
	domain.SubscriptionTypeInternal:                 {},
	domain.SubscriptionTypePlatformerSubscription:   {},
	domain.SubscriptionTypeCloudSupport:             {},
	domain.SubscriptionTypeProfessionalServices:     {},
}

// snTypeNameToSubscriptionType converts a SN project type name (e.g. "Cloud Support")
// to the domain SubscriptionType enum (e.g. "cloud_support"). Returns an error
// if the converted value is not a known enum value.
func snTypeNameToSubscriptionType(name string) (domain.SubscriptionType, error) {
	st := domain.SubscriptionType(strings.ToLower(strings.ReplaceAll(name, " ", "_")))
	if _, ok := validSubscriptionTypes[st]; !ok {
		return "", fmt.Errorf("unknown subscription type %q from ServiceNow", name)
	}
	return st, nil
}

// snContactSearchPayload is the Choreo POST /{resource}/{id}/contacts/search request
// body. It is shared by both the project-contacts and account-contacts endpoints.
type snContactSearchPayload struct {
	Filters    snContactFilters    `json:"filters,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snContactFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

// snProjectContactsResponse mirrors the Choreo POST /projects/{id}/contacts/search response.
type snProjectContactsResponse struct {
	Contacts     []snProjectContact `json:"contacts"`
	TotalRecords int                `json:"totalRecords"`
	Offset       int                `json:"offset"`
	Limit        int                `json:"limit"`
}

type snProjectContact struct {
	Name                 string   `json:"name"`
	Email                string   `json:"email"`
	RegistrationState    string   `json:"registrationState"`
	NotificationsEnabled bool     `json:"notificationsEnabled"`
	Roles                []string `json:"roles"`
}

type snProjectContactService struct {
	client *integrationservice.Client
}

// NewServiceNowProjectContactService constructs a ProjectContactService backed by the Choreo API.
func NewServiceNowProjectContactService(client *integrationservice.Client) ProjectContactService {
	return &snProjectContactService{client: client}
}

// SearchProjectContacts implements ProjectContactService. Supported by the ServiceNow
// data source only; there is no Postgres fallback.
func (s *snProjectContactService) SearchProjectContacts(ctx context.Context, projectID string, req domain.SearchProjectContactsRequest) (domain.SearchProjectContactsResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snContactSearchPayload{
		Filters:    snContactFilters{SearchQuery: req.Filters.SearchQuery},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/projects/"+uuidToSysid(projectID)+"/contacts/search", token, payload)
	if err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}

	var snResp snProjectContactsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchProjectContactsResponse{}, fmt.Errorf("sn project contacts: parse response: %w", err)
	}

	contacts := make([]domain.ProjectContact, 0, len(snResp.Contacts))
	for _, c := range snResp.Contacts {
		contacts = append(contacts, domain.ProjectContact{
			Name:                 c.Name,
			Email:                c.Email,
			RegistrationState:    c.RegistrationState,
			NotificationsEnabled: c.NotificationsEnabled,
			Roles:                c.Roles,
		})
	}

	return domain.SearchProjectContactsResponse{
		Contacts: contacts,
		Total:    snResp.TotalRecords,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}
