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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snChangeRequestsResponse mirrors the Choreo POST /change-requests/search response.
type snChangeRequestsResponse struct {
	ChangeRequests []snChangeRequest `json:"changeRequests"`
	TotalRecords   int               `json:"totalRecords"`
	Offset         int               `json:"offset"`
	Limit          int               `json:"limit"`
}

type snChangeRequest struct {
	ID               string         `json:"id"`
	Number           string         `json:"number"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	CreatedOn        string         `json:"createdOn"`
	UpdatedOn        *string        `json:"updatedOn"`
	Project          snCREntityRef  `json:"project"`
	Case             *snCREntityRef `json:"case"`
	Deployment       *snCREntityRef `json:"deployment"`
	DeployedProduct  *snCREntityRef `json:"deployedProduct"`
	Product          *snCREntityRef `json:"product"`
	AssignedEngineer *snCREntityRef `json:"assignedEngineer"`
	AssignedTeam     *snCREntityRef `json:"assignedTeam"`
	PlannedStartOn   *string        `json:"plannedStartOn"`
	PlannedEndOn     *string        `json:"plannedEndOn"`
	Duration         *string        `json:"duration"`
	Impact           *snCRLabel     `json:"impact"`
	State            *snCRLabel     `json:"state"`
	Type             *snCRLabel     `json:"type"`
}

type snCREntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snCRLabel struct {
	Label string `json:"label"`
}

// snChangeRequestSearchPayload is the Choreo POST /change-requests/search request body.
type snChangeRequestSearchPayload struct {
	Filters    snChangeRequestFilters `json:"filters,omitempty"`
	SortBy     *snCRSort              `json:"sortBy,omitempty"`
	Pagination snProjectPagination    `json:"pagination"`
}

type snCRSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

type snChangeRequestFilters struct {
	ProjectIDs      []string `json:"projectIds,omitempty"`
	SearchQuery     string   `json:"searchQuery,omitempty"`
	StateKeys       []int    `json:"stateKeys,omitempty"`
	ImpactKeys      []int    `json:"impactKeys,omitempty"`
	ClosedStartDate string   `json:"closedStartDate,omitempty"`
	ClosedEndDate   string   `json:"closedEndDate,omitempty"`
	// Number: see domain.SearchChangeRequestsFilters.Number doc comment.
	// Exact match against ServiceNow's `number` column -- not part of the
	// free-text SearchQuery scan.
	Number string `json:"number,omitempty"`
	// CreatedStartDate/CreatedEndDate: see domain.SearchChangeRequestsFilters
	// doc comment; mirrors ClosedStartDate/ClosedEndDate.
	CreatedStartDate string `json:"createdStartDate,omitempty"`
	CreatedEndDate   string `json:"createdEndDate,omitempty"`
	// AssignmentGroupIDs: sys_user_group sys_ids (converted from UUIDs).
	AssignmentGroupIDs []string `json:"assignmentGroupIds,omitempty"`
	// Approval: see domain.SearchChangeRequestsFilters.Filters doc comment
	// ("approval" field). ServiceNow's raw task.approval value, passed
	// through as-is -- not a key/enum mapping.
	Approval string `json:"approval,omitempty"`
	// AssignedUserIDs: sys_user sys_ids (converted from UUIDs). Wire key is
	// plural "assignedUserIds" to match Ballerina/SN's contract, even though
	// the domain-facing filter field is singular "assignedUserId".
	AssignedUserIDs []string `json:"assignedUserIds,omitempty"`
}

// snCRTypeIDMap maps domain ChangeRequestType enums to SN numeric type IDs.
var snCRTypeIDMap = map[domain.ChangeRequestType]int{
	domain.ChangeRequestTypeStandard:           1,
	domain.ChangeRequestTypeNormal:             2,
	domain.ChangeRequestTypeEmergency:          3,
	domain.ChangeRequestTypeModel:              4,
	domain.ChangeRequestTypeSiteReliabilityOps: 100,
	domain.ChangeRequestTypeAzure:              200,
}

// snCRStateIDMap maps domain ChangeRequestState enums to SN numeric state IDs.
var snCRStateIDMap = map[domain.ChangeRequestState]int{
	domain.ChangeRequestStateNew:              -5,
	domain.ChangeRequestStateAssess:           -4,
	domain.ChangeRequestStateAuthorize:        -3,
	domain.ChangeRequestStateCustomerApproval: 5,
	domain.ChangeRequestStateScheduled:        -2,
	domain.ChangeRequestStateImplement:        -1,
	domain.ChangeRequestStateReview:           0,
	domain.ChangeRequestStateCustomerReview:   1,
	domain.ChangeRequestStateRollback:         2,
	domain.ChangeRequestStateClosed:           3,
	domain.ChangeRequestStateCanceled:         4,
}

// snCRImpactIDMap maps domain ChangeRequestImpact enums to SN numeric impact IDs.
var snCRImpactIDMap = map[domain.ChangeRequestImpact]int{
	domain.ChangeRequestImpactHigh:   1,
	domain.ChangeRequestImpactMedium: 2,
	domain.ChangeRequestImpactLow:    3,
}

// snCRSortFieldMap maps domain ChangeRequestSortField values to SN field names.
var snCRSortFieldMap = map[domain.ChangeRequestSortField]string{
	domain.ChangeRequestSortFieldCreatedOn: "createdOn",
	domain.ChangeRequestSortFieldUpdatedOn: "updatedOn",
}

var validChangeRequestState = map[domain.ChangeRequestState]bool{
	domain.ChangeRequestStateNew:              true,
	domain.ChangeRequestStateAssess:           true,
	domain.ChangeRequestStateAuthorize:        true,
	domain.ChangeRequestStateCustomerApproval: true,
	domain.ChangeRequestStateScheduled:        true,
	domain.ChangeRequestStateImplement:        true,
	domain.ChangeRequestStateReview:           true,
	domain.ChangeRequestStateCustomerReview:   true,
	domain.ChangeRequestStateRollback:         true,
	domain.ChangeRequestStateClosed:           true,
	domain.ChangeRequestStateCanceled:         true,
}

var validChangeRequestImpact = map[domain.ChangeRequestImpact]bool{
	domain.ChangeRequestImpactHigh:   true,
	domain.ChangeRequestImpactMedium: true,
	domain.ChangeRequestImpactLow:    true,
}

var validChangeRequestSortField = map[domain.ChangeRequestSortField]bool{
	domain.ChangeRequestSortFieldCreatedOn: true,
	domain.ChangeRequestSortFieldUpdatedOn: true,
}

var validChangeRequestSortOrder = map[domain.ChangeRequestSortOrder]bool{
	domain.ChangeRequestSortOrderAsc:  true,
	domain.ChangeRequestSortOrderDesc: true,
}

func domainCRStatesToSNIDs(states []domain.ChangeRequestState) []int {
	ids := make([]int, 0, len(states))
	for _, s := range states {
		if id, ok := snCRStateIDMap[s]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func domainCRImpactsToSNIDs(impacts []domain.ChangeRequestImpact) []int {
	ids := make([]int, 0, len(impacts))
	for _, i := range impacts {
		if id, ok := snCRImpactIDMap[i]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// snCRStateLabelMap maps SN state labels (lowercased) to domain ChangeRequestState enums.
var snCRStateLabelMap = map[string]domain.ChangeRequestState{
	"new":               domain.ChangeRequestStateNew,
	"assess":            domain.ChangeRequestStateAssess,
	"authorize":         domain.ChangeRequestStateAuthorize,
	"customer approval": domain.ChangeRequestStateCustomerApproval,
	"scheduled":         domain.ChangeRequestStateScheduled,
	"implement":         domain.ChangeRequestStateImplement,
	"review":            domain.ChangeRequestStateReview,
	"customer review":   domain.ChangeRequestStateCustomerReview,
	"rollback":          domain.ChangeRequestStateRollback,
	"closed":            domain.ChangeRequestStateClosed,
	"canceled":          domain.ChangeRequestStateCanceled,
	"cancelled":         domain.ChangeRequestStateCanceled,
}

// snCRImpactLabelMap maps SN impact labels (lowercased word) to domain ChangeRequestImpact enums.
var snCRImpactLabelMap = map[string]domain.ChangeRequestImpact{
	"high":   domain.ChangeRequestImpactHigh,
	"medium": domain.ChangeRequestImpactMedium,
	"low":    domain.ChangeRequestImpactLow,
}

func snCRStateLabelToString(label *snCRLabel) *string {
	if label == nil {
		return nil
	}
	if v, ok := snCRStateLabelMap[strings.ToLower(label.Label)]; ok {
		s := string(v)
		return &s
	}
	s := label.Label
	return &s
}

func snCRImpactLabelToString(label *snCRLabel) *string {
	if label == nil {
		return nil
	}
	for _, word := range strings.Fields(label.Label) {
		if v, ok := snCRImpactLabelMap[strings.ToLower(strings.Trim(word, "()-"))]; ok {
			s := string(v)
			return &s
		}
	}
	s := label.Label
	return &s
}

func snCRTypeLabelToString(label *snCRLabel) *string {
	if label == nil {
		return nil
	}
	s := label.Label
	return &s
}

type snChangeRequestService struct {
	client *integrationservice.Client
}

// NewServiceNowChangeRequestService constructs a ChangeRequestService backed by the Choreo API.
func NewServiceNowChangeRequestService(client *integrationservice.Client) ChangeRequestService {
	return &snChangeRequestService{client: client}
}

func (s *snChangeRequestService) SearchChangeRequests(ctx context.Context, req domain.SearchChangeRequestsRequest) (domain.SearchChangeRequestsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchChangeRequestsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchChangeRequestsResponse{}, err
	}
	if err := validateExactNumber("number", req.Filters.Number); err != nil {
		return domain.SearchChangeRequestsResponse{}, err
	}

	if req.Filters.ClosedEndDate != nil && req.Filters.ClosedStartDate != nil &&
		req.Filters.ClosedEndDate.Before(*req.Filters.ClosedStartDate) {
		return domain.SearchChangeRequestsResponse{}, &apierror.ValidationError{Msg: "closedEndDate must not be before closedStartDate"}
	}

	for _, s := range req.Filters.States {
		if !validChangeRequestState[s] {
			return domain.SearchChangeRequestsResponse{}, &apierror.ValidationError{Msg: "states contains invalid value: " + string(s)}
		}
	}
	for _, i := range req.Filters.Impacts {
		if !validChangeRequestImpact[i] {
			return domain.SearchChangeRequestsResponse{}, &apierror.ValidationError{Msg: "impacts contains invalid value: " + string(i)}
		}
	}
	if req.SortBy.Field != "" && !validChangeRequestSortField[req.SortBy.Field] {
		return domain.SearchChangeRequestsResponse{}, &apierror.ValidationError{Msg: "sortBy.field contains invalid value: " + string(req.SortBy.Field)}
	}
	if req.SortBy.Order != "" && !validChangeRequestSortOrder[req.SortBy.Order] {
		return domain.SearchChangeRequestsResponse{}, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + string(req.SortBy.Order)}
	}
	if err := validateUUIDs("projectIds", req.Filters.ProjectIDs); err != nil {
		return domain.SearchChangeRequestsResponse{}, err
	}
	parsedFilters, err := ParseChangeRequestFieldFilters(req.Filters.Filters, time.Now().UTC())
	if err != nil {
		return domain.SearchChangeRequestsResponse{}, err
	}
	if parsedFilters.CreatedEndDate != nil && parsedFilters.CreatedStartDate != nil &&
		parsedFilters.CreatedEndDate.Before(*parsedFilters.CreatedStartDate) {
		return domain.SearchChangeRequestsResponse{}, &apierror.ValidationError{Msg: "createdOn: lte value must not be before gte value"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	// A customer is only ever sent the states a customer may see, whatever they
	// asked for (see sn_change_request_customer_view.go); staff are sent what they
	// asked for.
	states := req.Filters.States
	customerView := customerViewApplies(ctx)
	if customerView {
		var none bool
		if states, none = customerChangeRequestStates(states); none {
			return domain.SearchChangeRequestsResponse{
				ChangeRequests: []domain.SearchChangeRequestView{},
				Total:          0,
				Limit:          req.Pagination.Limit,
				Offset:         req.Pagination.Offset,
			}, nil
		}
	}

	var snSortBy *snCRSort
	if req.SortBy.Field != "" {
		snField := snCRSortFieldMap[req.SortBy.Field]
		order := string(req.SortBy.Order)
		if order == "" {
			order = "desc"
		}
		snSortBy = &snCRSort{Field: snField, Order: order}
	}

	payload := snChangeRequestSearchPayload{
		Filters: snChangeRequestFilters{
			ProjectIDs:         uuidsToSysids(req.Filters.ProjectIDs),
			SearchQuery:        req.Filters.SearchQuery,
			StateKeys:          domainCRStatesToSNIDs(states),
			ImpactKeys:         domainCRImpactsToSNIDs(req.Filters.Impacts),
			ClosedStartDate:    formatSNDateTimeUTC(req.Filters.ClosedStartDate),
			ClosedEndDate:      formatSNDateTimeUTC(req.Filters.ClosedEndDate),
			Number:             stringPtrValue(req.Filters.Number),
			CreatedStartDate:   formatSNDateTimeUTC(parsedFilters.CreatedStartDate),
			CreatedEndDate:     formatSNDateTimeUTC(parsedFilters.CreatedEndDate),
			AssignmentGroupIDs: uuidsToSysids(parsedFilters.AssignmentGroupIDs),
			Approval:           stringPtrValue(parsedFilters.Approval),
			AssignedUserIDs:    uuidsToSysids(parsedFilters.AssignedUserIDs),
		},
		SortBy:     snSortBy,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/change-requests/search", token, payload)
	if err != nil {
		return domain.SearchChangeRequestsResponse{}, err
	}

	var snResp snChangeRequestsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchChangeRequestsResponse{}, fmt.Errorf("sn change requests: parse response: %w", err)
	}

	views := make([]domain.SearchChangeRequestView, 0, len(snResp.ChangeRequests))
	for _, cr := range snResp.ChangeRequests {
		subject := cr.Title
		description := cr.Description

		updatedOn := cr.CreatedOn
		if cr.UpdatedOn != nil && *cr.UpdatedOn != "" {
			updatedOn = *cr.UpdatedOn
		}

		view := domain.SearchChangeRequestView{
			ID:             sysidToUUID(cr.ID),
			Number:         cr.Number,
			Subject:        &subject,
			Description:    &description,
			Project:        domain.EntityRef{ID: sysidToUUID(cr.Project.ID), Name: cr.Project.Name},
			PlannedStartOn: cr.PlannedStartOn,
			PlannedEndOn:   cr.PlannedEndOn,
			Duration:       cr.Duration,
			Impact:         snCRImpactLabelToString(cr.Impact),
			State:          snCRStateLabelToString(cr.State),
			Type:           snCRTypeLabelToString(cr.Type),
			CreatedOn:      cr.CreatedOn,
			UpdatedOn:      updatedOn,
		}
		if cr.Case != nil {
			view.Case = &domain.EntityRef{ID: sysidToUUID(cr.Case.ID), Name: cr.Case.Name}
		}
		if cr.Deployment != nil {
			view.Deployment = &domain.EntityRef{ID: sysidToUUID(cr.Deployment.ID), Name: cr.Deployment.Name}
		}
		if cr.DeployedProduct != nil {
			view.DeployedProduct = &domain.EntityRef{ID: sysidToUUID(cr.DeployedProduct.ID), Name: cr.DeployedProduct.Name}
		}
		if cr.Product != nil {
			view.Product = &domain.EntityRef{ID: sysidToUUID(cr.Product.ID), Name: cr.Product.Name}
		}
		if cr.AssignedEngineer != nil {
			view.AssignedEngineer = &domain.EntityRef{ID: sysidToUUID(cr.AssignedEngineer.ID), Name: cr.AssignedEngineer.Name}
		}
		if cr.AssignedTeam != nil {
			view.AssignedTeam = &domain.EntityRef{ID: sysidToUUID(cr.AssignedTeam.ID), Name: cr.AssignedTeam.Name}
		}
		views = append(views, view)
	}

	total := snResp.TotalRecords
	if customerView {
		// ServiceNow was asked for the visible states only, so nothing should be
		// dropped; a change request it returned anyway is not shown (and not counted).
		var dropped int
		if views, dropped = dropHiddenChangeRequests(ctx, views); dropped > 0 {
			total = max(total-dropped, len(views))
		}
	}

	return domain.SearchChangeRequestsResponse{
		ChangeRequests: views,
		Total:          total,
		Limit:          req.Pagination.Limit,
		Offset:         req.Pagination.Offset,
	}, nil
}

// snChangeRequestAggregatePayload is the Choreo POST /change-requests/aggregate
// request body.
type snChangeRequestAggregatePayload struct {
	Filters   snChangeRequestFilters `json:"filters,omitempty"`
	GroupBy   string                 `json:"groupBy"`
	MaxGroups int                    `json:"maxGroups,omitempty"`
}

// validChangeRequestAggregateField is the allow-list for
// AggregateChangeRequestsRequest.GroupBy, matching openapi.yaml's
// AggregateChangeRequestsRequest.groupBy enum exactly.
var validChangeRequestAggregateField = map[string]bool{
	"state":           true,
	"assignmentGroup": true,
}

// AggregateChangeRequests implements ChangeRequestService by calling the
// Choreo POST /change-requests/aggregate endpoint: a single server-side
// aggregation over the requested field, capped to the top MaxGroups buckets
// with the remainder folded into AggregateResponse.OthersCount. Filter
// parsing and validation mirror SearchChangeRequests.
func (s *snChangeRequestService) AggregateChangeRequests(ctx context.Context, req domain.AggregateChangeRequestsRequest) (domain.AggregateResponse, error) {
	if req.GroupBy == "" {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy is required"}
	}
	if !validChangeRequestAggregateField[req.GroupBy] {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + req.GroupBy}
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.AggregateResponse{}, err
	}
	if err := validateExactNumber("number", req.Filters.Number); err != nil {
		return domain.AggregateResponse{}, err
	}

	if req.Filters.ClosedEndDate != nil && req.Filters.ClosedStartDate != nil &&
		req.Filters.ClosedEndDate.Before(*req.Filters.ClosedStartDate) {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "closedEndDate must not be before closedStartDate"}
	}
	for _, s := range req.Filters.States {
		if !validChangeRequestState[s] {
			return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "states contains invalid value: " + string(s)}
		}
	}
	for _, i := range req.Filters.Impacts {
		if !validChangeRequestImpact[i] {
			return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "impacts contains invalid value: " + string(i)}
		}
	}
	if err := validateUUIDs("projectIds", req.Filters.ProjectIDs); err != nil {
		return domain.AggregateResponse{}, err
	}
	parsedFilters, err := ParseChangeRequestFieldFilters(req.Filters.Filters, time.Now().UTC())
	if err != nil {
		return domain.AggregateResponse{}, err
	}
	if parsedFilters.CreatedEndDate != nil && parsedFilters.CreatedStartDate != nil &&
		parsedFilters.CreatedEndDate.Before(*parsedFilters.CreatedStartDate) {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "createdOn: lte value must not be before gte value"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	// The same customer view as SearchChangeRequests.
	states := req.Filters.States
	customerView := customerViewApplies(ctx)
	if customerView {
		var none bool
		if states, none = customerChangeRequestStates(states); none {
			return domain.AggregateResponse{Groups: []domain.AggregateBucket{}}, nil
		}
	}

	payload := snChangeRequestAggregatePayload{
		Filters: snChangeRequestFilters{
			ProjectIDs:         uuidsToSysids(req.Filters.ProjectIDs),
			SearchQuery:        req.Filters.SearchQuery,
			StateKeys:          domainCRStatesToSNIDs(states),
			ImpactKeys:         domainCRImpactsToSNIDs(req.Filters.Impacts),
			ClosedStartDate:    formatSNDateTimeUTC(req.Filters.ClosedStartDate),
			ClosedEndDate:      formatSNDateTimeUTC(req.Filters.ClosedEndDate),
			Number:             stringPtrValue(req.Filters.Number),
			CreatedStartDate:   formatSNDateTimeUTC(parsedFilters.CreatedStartDate),
			CreatedEndDate:     formatSNDateTimeUTC(parsedFilters.CreatedEndDate),
			AssignmentGroupIDs: uuidsToSysids(parsedFilters.AssignmentGroupIDs),
			Approval:           stringPtrValue(parsedFilters.Approval),
			AssignedUserIDs:    uuidsToSysids(parsedFilters.AssignedUserIDs),
		},
		GroupBy:   req.GroupBy,
		MaxGroups: req.MaxGroups,
	}

	raw, err := s.client.Post(ctx, "/change-requests/aggregate", token, payload)
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	var resp domain.AggregateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("sn change requests: parse aggregate response: %w", err)
	}
	// "assignmentGroup" is an ID-valued field in
	// validChangeRequestAggregateField; SN returns its bucket keys as raw
	// sys_ids, so convert them to this platform's UUIDs before returning.
	if req.GroupBy == "assignmentGroup" {
		for i := range resp.Groups {
			resp.Groups[i].Key = sysidToUUID(resp.Groups[i].Key)
		}
	}
	// "state" is a plain enum, but SN's own groupBy implementation returns
	// its raw internal state value as the bucket key (e.g. "-5"), not this
	// platform's domain enum string. SN's response already carries the
	// correct human-readable label for each bucket (e.g. "New"), so remap
	// the key through the same label lookup snCRStateLabelToString uses
	// elsewhere in this file, rather than trying to parse the raw value.
	if req.GroupBy == "state" {
		for i := range resp.Groups {
			if v, ok := snCRStateLabelMap[strings.ToLower(resp.Groups[i].Label)]; ok {
				resp.Groups[i].Key = string(v)
			}
			// else: leave the key as-is, mirroring snCRStateLabelToString's
			// own defensive fallback for an unrecognized label.
		}
		if customerView {
			resp = dropHiddenStateBuckets(ctx, resp)
		}
	}
	return resp, nil
}

// snCreateChangeRequestPayload is the Choreo POST /change-requests request
// body. Deliberately has no stateKey field: the org's own Change Management
// process flow confirms every change request begins at New unconditionally
// (no branch at creation decides otherwise), and ServiceNow already defaults
// a fresh record to New on its own. An earlier revision of this payload
// accepted and forwarded a caller-chosen create-time state (New/Assess/
// Authorize) -- a real reported bug: the CSM Portal's own create form let a
// user pick Assess or Authorize directly, skipping the workflow's own
// assess/authorize gates entirely. CreateChangeRequest below now rejects any
// req.State other than New before this payload is even built.
type snCreateChangeRequestPayload struct {
	Subject             string  `json:"subject"`
	CategoryKey         *string `json:"categoryKey,omitempty"`
	ServiceID           *string `json:"serviceId,omitempty"`
	ServiceOfferingID   *string `json:"serviceOfferingId,omitempty"`
	ConfigurationItemID *string `json:"configurationItemId,omitempty"`
	PriorityKey         *string `json:"priorityKey,omitempty"`
	ImpactKey           *string `json:"impactKey,omitempty"`
	TypeKey             *string `json:"typeKey,omitempty"`
	GroupID             *string `json:"groupId,omitempty"`
	AssignedEngineerID  *string `json:"assignedEngineerId,omitempty"`
	RiskKey             *string `json:"riskKey,omitempty"`
	RequestedByID       *string `json:"requestedById,omitempty"`
	Description         *string `json:"description,omitempty"`
	Justification       *string `json:"justification,omitempty"`
	ImplementationPlan  *string `json:"implementationPlan,omitempty"`
	RiskImpactAnalysis  *string `json:"riskImpactAnalysis,omitempty"`
	BackoutPlan         *string `json:"backoutPlan,omitempty"`
	TestPlan            *string `json:"testPlan,omitempty"`
	PlannedStartDate    *string `json:"plannedStartDate,omitempty"`
	PlannedEndDate      *string `json:"plannedEndDate,omitempty"`
	Comment             *string `json:"comment,omitempty"`
	WorkNote            *string `json:"workNote,omitempty"`
	// Field-parity additions -- see domain.CreateChangeRequestRequest.
	AffectedServicesText         *string  `json:"affectedServicesText,omitempty"`
	AffectedComponentsText       *string  `json:"affectedComponentsText,omitempty"`
	RollbackDurationText         *string  `json:"rollbackDurationText,omitempty"`
	DeploymentProductIDs         []string `json:"deploymentProductIds,omitempty"`
	DurationInput                *int     `json:"durationInput,omitempty"`
	IsPlanningVisibleToCustomers *bool    `json:"isPlanningVisibleToCustomers,omitempty"`
}

// snCreateChangeRequestResponse mirrors the Choreo POST /change-requests response.
type snCreateChangeRequestResponse struct {
	Message       string `json:"message"`
	ChangeRequest struct {
		ID        string `json:"id"`
		Number    string `json:"number"`
		CreatedOn string `json:"createdOn"`
		CreatedBy string `json:"createdBy"`
	} `json:"changeRequest"`
}

// snCRCreateTypeIDMap maps domain ChangeRequestType enums to SN string type IDs for create.
var snCRCreateTypeIDMap = map[domain.ChangeRequestType]string{
	domain.ChangeRequestTypeStandard:           "standard",
	domain.ChangeRequestTypeNormal:             "normal",
	domain.ChangeRequestTypeEmergency:          "emergency",
	domain.ChangeRequestTypeModel:              "model",
	domain.ChangeRequestTypeSiteReliabilityOps: "site_reliability_ops",
	domain.ChangeRequestTypeAzure:              "azure",
}

// snCRRiskIDMap maps domain ChangeRequestRisk enums to SN string risk IDs.
var snCRRiskIDMap = map[domain.ChangeRequestRisk]string{
	domain.ChangeRequestRiskHigh:     "2",
	domain.ChangeRequestRiskModerate: "3",
	domain.ChangeRequestRiskLow:      "4",
}

// snCRPriorityIDMap maps domain ChangeRequestPriority enums to SN string priority IDs.
var snCRPriorityIDMap = map[domain.ChangeRequestPriority]string{
	domain.ChangeRequestPriorityCritical: "1",
	domain.ChangeRequestPriorityHigh:     "2",
	domain.ChangeRequestPriorityModerate: "3",
	domain.ChangeRequestPriorityLow:      "4",
}

// snCRImpactCreateIDMap maps domain ChangeRequestImpact enums to SN string impact IDs.
var snCRImpactCreateIDMap = map[domain.ChangeRequestImpact]string{
	domain.ChangeRequestImpactHigh:   "1",
	domain.ChangeRequestImpactMedium: "2",
	domain.ChangeRequestImpactLow:    "3",
}

// snCRCategoryIDMap maps domain ChangeRequestCategory enums to SN category string IDs.
var snCRCategoryIDMap = map[domain.ChangeRequestCategory]string{
	domain.ChangeRequestCategoryHardware:             "Hardware",
	domain.ChangeRequestCategorySoftware:             "Software",
	domain.ChangeRequestCategoryService:              "Service",
	domain.ChangeRequestCategorySystemSoftware:       "System Software",
	domain.ChangeRequestCategoryApplicationsSoftware: "Applications Software",
	domain.ChangeRequestCategoryNetwork:              "Network",
	domain.ChangeRequestCategoryTelecom:              "Telecom",
	domain.ChangeRequestCategoryDocumentation:        "Documentation",
	domain.ChangeRequestCategoryOther:                "Other",
	domain.ChangeRequestCategoryRegularReleaseCloud:  "Regular Release - Cloud",
	domain.ChangeRequestCategoryHotfixReleaseCloud:   "Hotfix Release - Cloud",
	domain.ChangeRequestCategoryDevOps:               "DevOps",
	domain.ChangeRequestCategoryCloudComputing:       "cloud computing",
}

func strPtr(s string) *string { return &s }

// snCRPriorityLabelMap maps SN numeric priority ids (as returned by GET) to
// domain ChangeRequestPriority enum strings. The inverse of snCRPriorityIDMap.
var snCRPriorityLabelMap = map[int]string{
	1: string(domain.ChangeRequestPriorityCritical),
	2: string(domain.ChangeRequestPriorityHigh),
	3: string(domain.ChangeRequestPriorityModerate),
	4: string(domain.ChangeRequestPriorityLow),
}

// snCRCategoryLabelMap maps SN category string ids (as returned by GET) to
// domain ChangeRequestCategory enum strings. The inverse of snCRCategoryIDMap.
var snCRCategoryLabelMap = map[string]string{
	"Hardware":                string(domain.ChangeRequestCategoryHardware),
	"Software":                string(domain.ChangeRequestCategorySoftware),
	"Service":                 string(domain.ChangeRequestCategoryService),
	"System Software":         string(domain.ChangeRequestCategorySystemSoftware),
	"Applications Software":   string(domain.ChangeRequestCategoryApplicationsSoftware),
	"Network":                 string(domain.ChangeRequestCategoryNetwork),
	"Telecom":                 string(domain.ChangeRequestCategoryTelecom),
	"Documentation":           string(domain.ChangeRequestCategoryDocumentation),
	"Other":                   string(domain.ChangeRequestCategoryOther),
	"Regular Release - Cloud": string(domain.ChangeRequestCategoryRegularReleaseCloud),
	"Hotfix Release - Cloud":  string(domain.ChangeRequestCategoryHotfixReleaseCloud),
	"DevOps":                  string(domain.ChangeRequestCategoryDevOps),
	"cloud computing":         string(domain.ChangeRequestCategoryCloudComputing),
}

// validChangeRequestPriority is the allow-list for CreateChangeRequestRequest.Priority
// and PatchChangeRequestRequest.Priority.
var validChangeRequestPriority = map[domain.ChangeRequestPriority]bool{
	domain.ChangeRequestPriorityCritical: true,
	domain.ChangeRequestPriorityHigh:     true,
	domain.ChangeRequestPriorityModerate: true,
	domain.ChangeRequestPriorityLow:      true,
}

// validChangeRequestCategory is the allow-list for CreateChangeRequestRequest.Category
// and PatchChangeRequestRequest.Category.
var validChangeRequestCategory = map[domain.ChangeRequestCategory]bool{
	domain.ChangeRequestCategoryHardware:             true,
	domain.ChangeRequestCategorySoftware:             true,
	domain.ChangeRequestCategoryService:              true,
	domain.ChangeRequestCategorySystemSoftware:       true,
	domain.ChangeRequestCategoryApplicationsSoftware: true,
	domain.ChangeRequestCategoryNetwork:              true,
	domain.ChangeRequestCategoryTelecom:              true,
	domain.ChangeRequestCategoryDocumentation:        true,
	domain.ChangeRequestCategoryOther:                true,
	domain.ChangeRequestCategoryRegularReleaseCloud:  true,
	domain.ChangeRequestCategoryHotfixReleaseCloud:   true,
	domain.ChangeRequestCategoryDevOps:               true,
	domain.ChangeRequestCategoryCloudComputing:       true,
}

// snCRIntChoice mirrors a Choreo {id: <int>, label: <string>} choice-field shape.
type snCRIntChoice struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// snCRStrChoice mirrors a Choreo {id: <string>, label: <string>} choice-field shape.
type snCRStrChoice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// snUTCDateTimeLayout is the layout the downstream create endpoint requires for
// planned start/end timestamps. The platform's own API accepts a single datetime
// format everywhere (snCreatedOnLayout, "YYYY-MM-DD HH:mm:ss"); the downstream
// create endpoint diverges from its own update endpoint and demands this one, so
// the adapter converts here rather than leaking the inconsistency upwards.
const snUTCDateTimeLayout = "2006-01-02T15:04:05Z"

// snPlannedTimestamp returns a planned start / end in the one layout ServiceNow's
// change request API takes, "YYYY-MM-DD HH:mm:ss" in UTC (snCreatedOnLayout).
//
// The API this service fronts documents two layouts for the planned window: that
// one, and RFC 3339 with a zone designator ("2030-03-01T09:00:00Z",
// "...+05:30"), which the PostgreSQL data source reads as an instant. An RFC 3339
// value is therefore converted to the zoneless UTC layout here, the same
// conversion the dual-write mirror makes (repository.StrictMirrorPlannedTimestamp,
// which reads an instant as PlannedTimestampForServiceNow does); a value already
// in the zoneless layout is forwarded as it was sent (one spelled a little
// differently, "2030-03-01 9:00:00", is written back in the layout). Anything
// else is a ValidationError naming the field, never an opaque downstream pattern
// failure: "infinity", "now", "tomorrow", a date alone, a zone name; a year
// outside 2000 to 2100 in either layout (the range the PostgreSQL data source
// holds every planned window to); a zoneless value with a fractional second,
// which Go's parser takes but ServiceNow's layout has none (an RFC 3339 value
// with one is an instant and is converted to whole seconds).
func snPlannedTimestamp(field, value string) (string, error) {
	converted, err := repository.StrictMirrorPlannedTimestamp(value)
	switch {
	case err == nil:
		return converted, nil
	case errors.Is(err, repository.ErrPlannedTimestampFraction), errors.Is(err, repository.ErrPlannedTimestampYear):
		return "", &apierror.ValidationError{
			Msg: fmt.Sprintf("%s must follow the format: YYYY-MM-DD HH:mm:ss (%s)", field, err.Error()),
		}
	}
	return "", &apierror.ValidationError{
		Msg: fmt.Sprintf("%s must follow the format: YYYY-MM-DD HH:mm:ss", field),
	}
}

// toDownstreamUTCDateTime parses a planned date-time (see snPlannedTimestamp:
// "YYYY-MM-DD HH:mm:ss" interpreted as UTC, or RFC 3339 with a zone) and re-emits
// it in the layout the downstream create endpoint requires. It returns a
// ValidationError naming the field when the input does not parse, so bad input is
// rejected with a specific message instead of an opaque downstream
// pattern-validation failure.
func toDownstreamUTCDateTime(field, value string) (string, error) {
	converted, err := snPlannedTimestamp(field, value)
	if err != nil {
		return "", err
	}
	t, err := time.Parse(snCreatedOnLayout, converted)
	if err != nil {
		return "", &apierror.ValidationError{
			Msg: fmt.Sprintf("%s must follow the format: YYYY-MM-DD HH:mm:ss", field),
		}
	}
	return t.Format(snUTCDateTimeLayout), nil
}

// CreateChangeRequest implements ChangeRequestService for the ServiceNow data source.
func (s *snChangeRequestService) CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
	if err := repository.RejectRemovedCreateFields(req); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	if req.Subject == "" {
		return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: "subject is required"}
	}
	// ServiceNow's create payload has no project or deployment field; refuse
	// rather than drop them silently. (The PostgreSQL-first dual-write service
	// strips them before it mirrors, so this only fires when ServiceNow is the
	// sole data source.)
	if req.ProjectID != nil || len(req.DeploymentIDs) > 0 {
		return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: "projectId and deploymentIds are not supported on the ServiceNow data source"}
	}
	if req.Category != nil {
		if _, ok := snCRCategoryIDMap[*req.Category]; !ok {
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid category %q", *req.Category)}
		}
	}
	// The type is mandatory and must be standard/normal/emergency -- it decides
	// the approval flow ServiceNow runs (Standard: none; Normal: approvals;
	// Emergency: expedited).
	if err := repository.ValidateCreateChangeRequestType(req.Type); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	if _, ok := snCRCreateTypeIDMap[*req.Type]; !ok {
		return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid type %q", *req.Type)}
	}
	// An Emergency change takes no customer step. The two customer boxes are not part of
	// ServiceNow's create payload (this data source would drop them), but the refusal is
	// the same on every create path rather than a request that is accepted and ignored.
	if err := repository.ValidateCreateChangeRequestCustomerGates(req.Type, req.CustomerApprovalRequired, req.CustomerReviewRequired); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	// A change request can only ever be created at New -- see
	// snCreateChangeRequestPayload's own doc comment for why. req.State is
	// still accepted (rather than removed from CreateChangeRequestRequest
	// entirely) so a caller that explicitly asks for New gets the same 200 it
	// always has; anything else is rejected outright rather than silently
	// downgraded to New, since silently ignoring a caller's explicit request
	// would look like success while doing something else.
	if req.State != nil && *req.State != domain.ChangeRequestStateNew {
		return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("a change request can only be created in the New state, not %q", *req.State)}
	}
	if req.Risk != nil {
		if _, ok := snCRRiskIDMap[*req.Risk]; !ok {
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid risk %q", *req.Risk)}
		}
	}
	if req.Priority != nil {
		if _, ok := snCRPriorityIDMap[*req.Priority]; !ok {
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid priority %q", *req.Priority)}
		}
	}
	if req.Impact != nil {
		if _, ok := snCRImpactCreateIDMap[*req.Impact]; !ok {
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid impact %q", *req.Impact)}
		}
	}

	uuidFields := map[string]*string{
		"serviceId":           req.ServiceID,
		"serviceOfferingId":   req.ServiceOfferingID,
		"configurationItemId": req.ConfigurationItemID,
		"groupId":             req.GroupID,
		"assignedEngineerId":  req.AssignedEngineerID,
		"requestedById":       req.RequestedByID,
	}
	for field, val := range uuidFields {
		if val != nil {
			if err := validateUUIDs(field, []string{*val}); err != nil {
				return domain.CreateChangeRequestResponse{}, err
			}
		}
	}
	if req.DeploymentProductIDs != nil {
		if err := validateUUIDs("deploymentProductIds", req.DeploymentProductIDs); err != nil {
			return domain.CreateChangeRequestResponse{}, err
		}
	}
	if req.DurationInput != nil && *req.DurationInput < 0 {
		return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: "durationInput must be a non-negative integer"}
	}
	if req.DurationInput != nil {
		if req.PlannedStartDate == nil || req.PlannedEndDate == nil {
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: "durationInput requires both plannedStartDate and plannedEndDate"}
		}
		startText, err := snPlannedTimestamp("plannedStartDate", *req.PlannedStartDate)
		if err != nil {
			return domain.CreateChangeRequestResponse{}, err
		}
		endText, err := snPlannedTimestamp("plannedEndDate", *req.PlannedEndDate)
		if err != nil {
			return domain.CreateChangeRequestResponse{}, err
		}
		// Both were just validated against this layout.
		start, _ := time.Parse(snCreatedOnLayout, startText)
		end, _ := time.Parse(snCreatedOnLayout, endText)
		if want := int(end.Sub(start).Seconds()); *req.DurationInput != want {
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{
				Msg: fmt.Sprintf("durationInput (%d) must match plannedEndDate - plannedStartDate (%d)", *req.DurationInput, want),
			}
		}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snCreateChangeRequestPayload{
		Subject:                      req.Subject,
		Description:                  req.Description,
		Justification:                req.Justification,
		ImplementationPlan:           req.ImplementationPlan,
		RiskImpactAnalysis:           req.RiskImpactAnalysis,
		BackoutPlan:                  req.BackoutPlan,
		TestPlan:                     req.TestPlan,
		Comment:                      req.Comment,
		WorkNote:                     req.WorkNote,
		AffectedServicesText:         req.AffectedServicesText,
		AffectedComponentsText:       req.AffectedComponentsText,
		RollbackDurationText:         req.RollbackDurationText,
		DeploymentProductIDs:         uuidsToSysids(req.DeploymentProductIDs),
		DurationInput:                req.DurationInput,
		IsPlanningVisibleToCustomers: req.IsPlanningVisibleToCustomers,
	}
	if req.PlannedStartDate != nil {
		v, err := toDownstreamUTCDateTime("plannedStartDate", *req.PlannedStartDate)
		if err != nil {
			return domain.CreateChangeRequestResponse{}, err
		}
		payload.PlannedStartDate = &v
	}
	if req.PlannedEndDate != nil {
		v, err := toDownstreamUTCDateTime("plannedEndDate", *req.PlannedEndDate)
		if err != nil {
			return domain.CreateChangeRequestResponse{}, err
		}
		payload.PlannedEndDate = &v
	}
	if req.Category != nil {
		v := snCRCategoryIDMap[*req.Category]
		payload.CategoryKey = &v
	}
	if req.Type != nil {
		v := snCRCreateTypeIDMap[*req.Type]
		payload.TypeKey = &v
	}
	// req.State is validated above but never forwarded -- payload has no
	// stateKey field at all, see snCreateChangeRequestPayload's own comment.
	if req.Risk != nil {
		v := snCRRiskIDMap[*req.Risk]
		payload.RiskKey = &v
	}
	if req.Priority != nil {
		v := snCRPriorityIDMap[*req.Priority]
		payload.PriorityKey = &v
	}
	if req.Impact != nil {
		v := snCRImpactCreateIDMap[*req.Impact]
		payload.ImpactKey = &v
	}
	if req.ServiceID != nil {
		payload.ServiceID = strPtr(uuidToSysid(*req.ServiceID))
	}
	if req.ServiceOfferingID != nil {
		payload.ServiceOfferingID = strPtr(uuidToSysid(*req.ServiceOfferingID))
	}
	if req.ConfigurationItemID != nil {
		payload.ConfigurationItemID = strPtr(uuidToSysid(*req.ConfigurationItemID))
	}
	if req.GroupID != nil {
		payload.GroupID = strPtr(uuidToSysid(*req.GroupID))
	}
	if req.AssignedEngineerID != nil {
		payload.AssignedEngineerID = strPtr(uuidToSysid(*req.AssignedEngineerID))
	}
	if req.RequestedByID != nil {
		payload.RequestedByID = strPtr(uuidToSysid(*req.RequestedByID))
	}

	raw, err := s.client.Post(ctx, "/change-requests", token, payload)
	if err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}

	var snResp snCreateChangeRequestResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		// The change request has already been created downstream. Unlike the update
		// path there is no identifier to hand back, so this stays an error — but it
		// must say the write succeeded, or the caller will retry and create a
		// duplicate change request.
		slog.WarnContext(ctx, "sn create change request: write applied but response could not be parsed", "error", err)
		return domain.CreateChangeRequestResponse{}, fmt.Errorf(
			"change request was created but the response could not be parsed; do not retry: %w", err)
	}

	resp := domain.CreateChangeRequestResponse{Message: snResp.Message}
	resp.ChangeRequest.ID = sysidToUUID(snResp.ChangeRequest.ID)
	resp.ChangeRequest.Number = snResp.ChangeRequest.Number
	resp.ChangeRequest.CreatedOn = snResp.ChangeRequest.CreatedOn
	resp.ChangeRequest.CreatedBy = snResp.ChangeRequest.CreatedBy
	return resp, nil
}

// snCRPatchStateIDMap maps domain ChangeRequestState enums to SN numeric state IDs for PATCH.
//
// Every state of change_request_state_enum has a key here, but a key is not
// a licence to write it: ServiceNow enforces its change model on EVERY write
// (verified on the dev instance against the scoped API the Choreo
// pass-through calls), so a raw stateKey is accepted only for a move its
// model allows from ITS current state, and a refused one comes back as a
// bare 500 ("Failed to update change request" / "The update operation did
// not complete successfully."), which the pass-through turns into a generic
// error. The dual-write mirror (changeRequestStateMirror) therefore never
// writes a state blindly: it reads ServiceNow's current state first and
// consults snChangeModelMoves, sends Request Approval as requestApproval
// (never stateKey -4), lets ServiceNow's own engine make the approval
// cascades and the customer's outcomes (mirroring the decision / the answer,
// then reading back and recording a divergence), and records every move it
// cannot send as a visible write-back failure that names the state and the
// reason. "scheduled" is never a MANUAL transition on either side (see
// withoutCustomerOutcomeStates and the PATCH's own refusal in the
// repository): the mirror sends it only as the result of a move PostgreSQL
// itself made, and only from a state the model allows it from.
var snCRPatchStateIDMap = map[domain.ChangeRequestState]int{
	domain.ChangeRequestStateNew:              -5,
	domain.ChangeRequestStateAssess:           -4,
	domain.ChangeRequestStateAuthorize:        -3,
	domain.ChangeRequestStateScheduled:        -2,
	domain.ChangeRequestStateImplement:        -1,
	domain.ChangeRequestStateReview:           0,
	domain.ChangeRequestStateCustomerReview:   1,
	domain.ChangeRequestStateRollback:         2,
	domain.ChangeRequestStateClosed:           3,
	domain.ChangeRequestStateCanceled:         4,
	domain.ChangeRequestStateCustomerApproval: 5,
}

// snMirrorableState turns a committed change_request.state label ("ASSESS",
// as PostgreSQL reports it) into the state the mirror sends, and says whether
// ServiceNow's PATCH has a key for it (snCRPatchStateIDMap). "" (a NULL
// state) has none.
func snMirrorableState(label string) (domain.ChangeRequestState, bool) {
	st := domain.ChangeRequestState(strings.ToLower(strings.TrimSpace(label)))
	_, ok := snCRPatchStateIDMap[st]
	return st, ok
}

// snChangeModelMoves is the previous system's change model as a direct state
// write sees it: from each of ITS states, the states a stateKey write may
// name (verified on the dev instance: from Assess, Scheduled and Customer
// Approval are refused and Canceled accepted; the Normal model lists
// New -> Assess / Authorize / Scheduled / Canceled, Assess -> Authorize
// (automatic) / Canceled / New, Authorize -> Scheduled (automatic) /
// Implement / Review / Closed / Canceled / New, Scheduled -> Implement /
// Canceled, Implement -> Review / Closed / Canceled, Review -> Closed /
// Authorize / Canceled). The two "automatic" edges are the engine's own
// (the approval cascades) and are not written directly. Customer Approval,
// Customer Review and Rollback are NOT in the model: the scoped application
// reaches them only through its own paths, so no direct write names them
// (snStateInModel). Canceled is allowed from every non-final state, Customer
// Approval and Customer Review included; Closed, Canceled and Rollback are
// final there as here.
var snChangeModelMoves = map[domain.ChangeRequestState][]domain.ChangeRequestState{
	domain.ChangeRequestStateNew:              {domain.ChangeRequestStateAssess, domain.ChangeRequestStateAuthorize, domain.ChangeRequestStateScheduled, domain.ChangeRequestStateCanceled},
	domain.ChangeRequestStateAssess:           {domain.ChangeRequestStateCanceled, domain.ChangeRequestStateNew},
	domain.ChangeRequestStateAuthorize:        {domain.ChangeRequestStateImplement, domain.ChangeRequestStateReview, domain.ChangeRequestStateClosed, domain.ChangeRequestStateCanceled, domain.ChangeRequestStateNew},
	domain.ChangeRequestStateScheduled:        {domain.ChangeRequestStateImplement, domain.ChangeRequestStateCanceled},
	domain.ChangeRequestStateImplement:        {domain.ChangeRequestStateReview, domain.ChangeRequestStateClosed, domain.ChangeRequestStateCanceled},
	domain.ChangeRequestStateReview:           {domain.ChangeRequestStateClosed, domain.ChangeRequestStateAuthorize, domain.ChangeRequestStateCanceled},
	domain.ChangeRequestStateCustomerApproval: {domain.ChangeRequestStateCanceled},
	domain.ChangeRequestStateCustomerReview:   {domain.ChangeRequestStateCanceled},
}

// snStateInModel reports whether the previous system's change model has a
// direct move INTO st at all (see snChangeModelMoves).
func snStateInModel(st domain.ChangeRequestState) bool {
	for _, targets := range snChangeModelMoves {
		for _, t := range targets {
			if t == st {
				return true
			}
		}
	}
	return false
}

// snDirectStateMoveAllowed reports whether the previous system's change model
// accepts a direct write of `to` on a record that is in `from` (its own
// state, as snCRStateLabelToString reports it; "" for an unknown one).
func snDirectStateMoveAllowed(from, to domain.ChangeRequestState) bool {
	for _, t := range snChangeModelMoves[from] {
		if t == to {
			return true
		}
	}
	return false
}

// snPatchChangeRequestPayload mirrors the Choreo PATCH /change-requests/{id} request body.
type snPatchChangeRequestPayload struct {
	Title                        *string `json:"title,omitempty"`
	Description                  *string `json:"description,omitempty"`
	ProjectID                    *string `json:"projectId,omitempty"`
	CaseID                       *string `json:"caseId,omitempty"`
	DeploymentID                 *string `json:"deploymentId,omitempty"`
	DeployedProductID            *string `json:"deployedProductId,omitempty"`
	AssignedEngineerID           *string `json:"assignedEngineerId,omitempty"`
	AssignedTeamID               *string `json:"assignedTeamId,omitempty"`
	PlannedStartOn               *string `json:"plannedStartOn,omitempty"`
	PlannedEndOn                 *string `json:"plannedEndOn,omitempty"`
	ImpactKey                    *int    `json:"impactKey,omitempty"`
	StateKey                     *int    `json:"stateKey,omitempty"`
	TypeKey                      *string `json:"typeKey,omitempty"`
	Justification                *string `json:"justification,omitempty"`
	ImpactDescription            *string `json:"impactDescription,omitempty"`
	ServiceOutage                *string `json:"serviceOutage,omitempty"`
	CommunicationPlan            *string `json:"communicationPlan,omitempty"`
	RollbackPlan                 *string `json:"rollbackPlan,omitempty"`
	TestPlan                     *string `json:"testPlan,omitempty"`
	IsCustomerApproved           *bool   `json:"isCustomerApproved,omitempty"`
	IsCustomerReviewed           *bool   `json:"isCustomerReviewed,omitempty"`
	RequestApproval              *bool   `json:"requestApproval,omitempty"`
	IsPlanningVisibleToCustomers *bool   `json:"isPlanningVisibleToCustomers,omitempty"`

	// Field-parity additions. Except Comment/WorkNote, every one of these is
	// json.RawMessage so an explicit null ("field": null) can be distinguished
	// from an omitted field -- omitempty drops a nil RawMessage entirely, while
	// json.RawMessage("null") is sent through unchanged. See
	// domain.PatchChangeRequestRequest for the tri-state contract.
	ImplementationPlan     json.RawMessage `json:"implementationPlan,omitempty"`
	PriorityKey            json.RawMessage `json:"priorityKey,omitempty"`
	CategoryKey            json.RawMessage `json:"categoryKey,omitempty"`
	RequestedByID          json.RawMessage `json:"requestedById,omitempty"`
	AffectedServicesText   json.RawMessage `json:"affectedServicesText,omitempty"`
	AffectedComponentsText json.RawMessage `json:"affectedComponentsText,omitempty"`
	RollbackDurationText   json.RawMessage `json:"rollbackDurationText,omitempty"`
	DeploymentProductIDs   json.RawMessage `json:"deploymentProductIds,omitempty"`
	DurationInput          json.RawMessage `json:"durationInput,omitempty"`
	// Comment and WorkNote append a journal entry; they are never null (the
	// backing API rejects an empty/whitespace value with 400) so a plain
	// pointer is enough.
	Comment  *string `json:"comment,omitempty"`
	WorkNote *string `json:"workNote,omitempty"`
}

// rawJSONOrNull marshals v (or emits the JSON literal null when v is nil) into
// a json.RawMessage, for building a tri-state PATCH payload field from a
// **T domain pointer-to-pointer.
func rawJSONOrNull(v any) (json.RawMessage, error) {
	if v == nil {
		return json.RawMessage("null"), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// snPatchChangeRequestResponse mirrors the Choreo PATCH /change-requests/{id} response.
type snPatchChangeRequestResponse struct {
	Message       string                `json:"message"`
	ChangeRequest snChangeRequestDetail `json:"changeRequest"`
}

func (s *snChangeRequestService) PatchChangeRequest(ctx context.Context, id string, req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
	if err := repository.RejectRemovedPatchFields(req); err != nil {
		return domain.PatchChangeRequestResponse{}, err
	}
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.PatchChangeRequestResponse{}, err
	}
	if req.DeploymentIDs != nil {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "deploymentIds is not supported on the ServiceNow data source"}
	}
	// WSO2's answer to a time the customer proposed lives in PostgreSQL (customer_updated_on /
	// customer_updated_date_confirmation): ServiceNow is the authority on this data source, and
	// its own flow answers it there.
	if req.ConfirmCustomerUpdatedDate != nil || req.ExpectedCustomerUpdatedOn != nil {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "confirmCustomerUpdatedDate is not supported on the ServiceNow data source: answer the customer's proposed date in ServiceNow"}
	}

	if req.Title == nil && req.Description == nil && req.ProjectID == nil && req.CaseID == nil &&
		req.DeploymentID == nil && req.DeployedProductID == nil && req.AssignedEngineerID == nil &&
		req.AssignedTeamID == nil && req.PlannedStartOn == nil && req.PlannedEndOn == nil &&
		req.Impact == nil && req.State == nil && req.Type == nil && req.Justification == nil &&
		req.ImpactDescription == nil && req.ServiceOutage == nil && req.CommunicationPlan == nil &&
		req.RollbackPlan == nil && req.TestPlan == nil && req.IsCustomerApproved == nil &&
		req.IsCustomerReviewed == nil && req.RequestApproval == nil &&
		req.ImplementationPlan == nil && req.Priority == nil && req.Category == nil &&
		req.RequestedByID == nil && req.AffectedServicesText == nil && req.AffectedComponentsText == nil &&
		req.RollbackDurationText == nil &&
		req.DeploymentProductIDs == nil && req.Comment == nil && req.WorkNote == nil &&
		req.DurationInput == nil && req.IsPlanningVisibleToCustomers == nil {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}

	exclusiveApprovalFields := 0
	if req.IsCustomerApproved != nil {
		exclusiveApprovalFields++
	}
	if req.IsCustomerReviewed != nil {
		exclusiveApprovalFields++
	}
	if req.RequestApproval != nil {
		exclusiveApprovalFields++
	}
	if exclusiveApprovalFields > 1 {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "isCustomerApproved, isCustomerReviewed, and requestApproval are mutually exclusive"}
	}

	if req.Title != nil && *req.Title == "" {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "title cannot be empty"}
	}
	if req.Impact != nil {
		if _, ok := snCRImpactIDMap[*req.Impact]; !ok {
			return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid impact %q", *req.Impact)}
		}
	}
	if req.State != nil {
		if _, ok := snCRPatchStateIDMap[*req.State]; !ok {
			return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid state %q", *req.State)}
		}
	}
	if req.Type != nil {
		if _, ok := snCRCreateTypeIDMap[*req.Type]; !ok {
			return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid type %q", *req.Type)}
		}
	}
	// The planned window goes downstream in the one layout ServiceNow takes: an RFC
	// 3339 value is converted to it (snPlannedTimestamp), a zoneless one is
	// forwarded as sent. req is this call's own copy, so re-pointing its fields
	// does not touch the caller's.
	if req.PlannedStartOn != nil {
		v, err := snPlannedTimestamp("plannedStartOn", *req.PlannedStartOn)
		if err != nil {
			return domain.PatchChangeRequestResponse{}, err
		}
		req.PlannedStartOn = &v
	}
	if req.PlannedEndOn != nil {
		v, err := snPlannedTimestamp("plannedEndOn", *req.PlannedEndOn)
		if err != nil {
			return domain.PatchChangeRequestResponse{}, err
		}
		req.PlannedEndOn = &v
	}
	if req.Priority != nil && *req.Priority != nil && !validChangeRequestPriority[**req.Priority] {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid priority %q", **req.Priority)}
	}
	if req.Category != nil && *req.Category != nil && !validChangeRequestCategory[**req.Category] {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid category %q", **req.Category)}
	}
	if req.Comment != nil && strings.TrimSpace(*req.Comment) == "" {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "comment cannot be empty"}
	}
	if req.WorkNote != nil && strings.TrimSpace(*req.WorkNote) == "" {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "workNote cannot be empty"}
	}
	if req.DurationInput != nil && *req.DurationInput != nil && **req.DurationInput < 0 {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "durationInput must be a non-negative integer"}
	}
	// Unlike CreateChangeRequest, this does not cross-check DurationInput against
	// the effective planned window: PatchChangeRequestRequest.DurationInput's doc
	// says the "effective" start/end falls back to the CR's stored value when not
	// provided in this same request, which this handler cannot see without an
	// extra GET before every PATCH that touches duration. Left to the backing
	// data source for now.

	uuidFields := map[string]*string{
		"projectId":          req.ProjectID,
		"caseId":             req.CaseID,
		"deploymentId":       req.DeploymentID,
		"deployedProductId":  req.DeployedProductID,
		"assignedEngineerId": req.AssignedEngineerID,
		"assignedTeamId":     req.AssignedTeamID,
	}
	for field, val := range uuidFields {
		if val != nil {
			if err := validateUUIDs(field, []string{*val}); err != nil {
				return domain.PatchChangeRequestResponse{}, err
			}
		}
	}
	// Tri-state UUID fields: only validate when a non-null value is being set;
	// an explicit null (clear) or an omitted field needs no UUID validation.
	triStateUUIDFields := map[string]**string{
		"requestedById": req.RequestedByID,
	}
	for field, val := range triStateUUIDFields {
		if val != nil && *val != nil {
			if err := validateUUIDs(field, []string{**val}); err != nil {
				return domain.PatchChangeRequestResponse{}, err
			}
		}
	}
	if req.DeploymentProductIDs != nil {
		if err := validateUUIDs("deploymentProductIds", *req.DeploymentProductIDs); err != nil {
			return domain.PatchChangeRequestResponse{}, err
		}
	}

	payload := snPatchChangeRequestPayload{
		Title:                        req.Title,
		Description:                  req.Description,
		PlannedStartOn:               req.PlannedStartOn,
		PlannedEndOn:                 req.PlannedEndOn,
		Justification:                req.Justification,
		ImpactDescription:            req.ImpactDescription,
		ServiceOutage:                req.ServiceOutage,
		CommunicationPlan:            req.CommunicationPlan,
		RollbackPlan:                 req.RollbackPlan,
		TestPlan:                     req.TestPlan,
		IsCustomerApproved:           req.IsCustomerApproved,
		IsCustomerReviewed:           req.IsCustomerReviewed,
		RequestApproval:              req.RequestApproval,
		Comment:                      req.Comment,
		WorkNote:                     req.WorkNote,
		IsPlanningVisibleToCustomers: req.IsPlanningVisibleToCustomers,
	}
	if req.ImplementationPlan != nil {
		v, err := rawJSONOrNull(*req.ImplementationPlan)
		if err != nil {
			return domain.PatchChangeRequestResponse{}, fmt.Errorf("sn patch change request: marshal implementationPlan: %w", err)
		}
		payload.ImplementationPlan = v
	}
	if req.Priority != nil {
		var v any
		if *req.Priority != nil {
			v = snCRPriorityIDMap[**req.Priority]
		}
		raw, err := rawJSONOrNull(v)
		if err != nil {
			return domain.PatchChangeRequestResponse{}, fmt.Errorf("sn patch change request: marshal priority: %w", err)
		}
		payload.PriorityKey = raw
	}
	if req.Category != nil {
		var v any
		if *req.Category != nil {
			v = snCRCategoryIDMap[**req.Category]
		}
		raw, err := rawJSONOrNull(v)
		if err != nil {
			return domain.PatchChangeRequestResponse{}, fmt.Errorf("sn patch change request: marshal category: %w", err)
		}
		payload.CategoryKey = raw
	}
	if req.RequestedByID != nil {
		var v any
		if *req.RequestedByID != nil {
			v = uuidToSysid(**req.RequestedByID)
		}
		raw, err := rawJSONOrNull(v)
		if err != nil {
			return domain.PatchChangeRequestResponse{}, fmt.Errorf("sn patch change request: marshal requestedById: %w", err)
		}
		payload.RequestedByID = raw
	}
	if req.AffectedServicesText != nil {
		v, err := rawJSONOrNull(*req.AffectedServicesText)
		if err != nil {
			return domain.PatchChangeRequestResponse{}, fmt.Errorf("sn patch change request: marshal affectedServicesText: %w", err)
		}
		payload.AffectedServicesText = v
	}
	if req.AffectedComponentsText != nil {
		v, err := rawJSONOrNull(*req.AffectedComponentsText)
		if err != nil {
			return domain.PatchChangeRequestResponse{}, fmt.Errorf("sn patch change request: marshal affectedComponentsText: %w", err)
		}
		payload.AffectedComponentsText = v
	}
	if req.RollbackDurationText != nil {
		v, err := rawJSONOrNull(*req.RollbackDurationText)
		if err != nil {
			return domain.PatchChangeRequestResponse{}, fmt.Errorf("sn patch change request: marshal rollbackDurationText: %w", err)
		}
		payload.RollbackDurationText = v
	}
	if req.DeploymentProductIDs != nil {
		b, err := json.Marshal(uuidsToSysids(*req.DeploymentProductIDs))
		if err != nil {
			return domain.PatchChangeRequestResponse{}, fmt.Errorf("sn patch change request: marshal deploymentProductIds: %w", err)
		}
		payload.DeploymentProductIDs = b
	}
	if req.DurationInput != nil {
		var v any
		if *req.DurationInput != nil {
			v = **req.DurationInput
		}
		raw, err := rawJSONOrNull(v)
		if err != nil {
			return domain.PatchChangeRequestResponse{}, fmt.Errorf("sn patch change request: marshal durationInput: %w", err)
		}
		payload.DurationInput = raw
	}
	if req.ProjectID != nil {
		payload.ProjectID = strPtr(uuidToSysid(*req.ProjectID))
	}
	if req.CaseID != nil {
		payload.CaseID = strPtr(uuidToSysid(*req.CaseID))
	}
	if req.DeploymentID != nil {
		payload.DeploymentID = strPtr(uuidToSysid(*req.DeploymentID))
	}
	if req.DeployedProductID != nil {
		payload.DeployedProductID = strPtr(uuidToSysid(*req.DeployedProductID))
	}
	if req.AssignedEngineerID != nil {
		payload.AssignedEngineerID = strPtr(uuidToSysid(*req.AssignedEngineerID))
	}
	if req.AssignedTeamID != nil {
		payload.AssignedTeamID = strPtr(uuidToSysid(*req.AssignedTeamID))
	}
	if req.Impact != nil {
		v := snCRImpactIDMap[*req.Impact]
		payload.ImpactKey = &v
	}
	if req.State != nil {
		v := snCRPatchStateIDMap[*req.State]
		payload.StateKey = &v
	}
	if req.Type != nil {
		v := snCRCreateTypeIDMap[*req.Type]
		payload.TypeKey = &v
	}

	raw, err := s.client.Patch(ctx, "/change-requests/"+uuidToSysid(id), token, payload)
	if err != nil {
		return domain.PatchChangeRequestResponse{}, err
	}

	var snResp snPatchChangeRequestResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		// The write has already been applied downstream at this point. Reporting a
		// response-shape drift as a failed update is wrong and actively misleading:
		// the caller retries or tells the user nothing changed, while the change
		// request has in fact moved. Degrade the body instead of failing the call.
		slog.WarnContext(ctx, "sn patch change request: write applied but response could not be parsed",
			"changeRequestID", id, "error", err)
		return domain.PatchChangeRequestResponse{
			Message:       "Change request updated. The updated change request could not be read back; reload to see current values.",
			ChangeRequest: domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}},
		}, nil
	}

	return domain.PatchChangeRequestResponse{
		Message:       snResp.Message,
		ChangeRequest: mapSNChangeRequestDetailToView(snResp.ChangeRequest),
	}, nil
}

// snChangeRequestDetail mirrors the Choreo GET /change-requests/{id} response.
type snChangeRequestDetail struct {
	snChangeRequest
	CreatedBy           string         `json:"createdBy"`
	Justification       *string        `json:"justification"`
	ImpactDescription   *string        `json:"impactDescription"`
	ServiceOutage       *string        `json:"serviceOutage"`
	CommunicationPlan   *string        `json:"communicationPlan"`
	RollbackPlan        *string        `json:"rollbackPlan"`
	TestPlan            *string        `json:"testPlan"`
	HasCustomerApproved bool           `json:"hasCustomerApproved"`
	HasCustomerReviewed bool           `json:"hasCustomerReviewed"`
	ApprovedBy          *snCREntityRef `json:"approvedBy"`
	ApprovedOn          *string        `json:"approvedOn"`
	LegalNextStates     []string       `json:"legalNextStates"`

	// Field-parity additions -- see domain.ChangeRequest for the grouping.
	ImplementationPlan *string        `json:"implementationPlan"`
	Priority           *snCRIntChoice `json:"priority"`
	Category           *snCRStrChoice `json:"category"`
	RequestedBy        *snCREntityRef `json:"requestedBy"`

	AffectedServicesText   *string         `json:"affectedServicesText"`
	AffectedComponentsText *string         `json:"affectedComponentsText"`
	RollbackDurationText   *string         `json:"rollbackDurationText"`
	Environments           []snCREntityRef `json:"environments"`
	DeploymentProducts     []snCREntityRef `json:"deploymentProducts"`
	CustomerGroup          *snCREntityRef  `json:"customerGroup"`

	ChangeRequestType            *snCRIntChoice  `json:"changeRequestType"`
	Likelihood                   *snCRIntChoice  `json:"likelihood"`
	IsPlanningVisibleToCustomers bool            `json:"isPlanningVisibleToCustomers"`
	ConfirmCustomerUpdatedDate   *string         `json:"confirmCustomerUpdatedDate"`
	CustomerUpdatedOn            *string         `json:"customerUpdatedOn"`
	Labels                       []string        `json:"labels"`
	Deployments                  []snCREntityRef `json:"deployments"`

	WorkStart    *string `json:"workStart"`
	WorkEnd      *string `json:"workEnd"`
	GitReference *string `json:"gitReference"`
}

func (s *snChangeRequestService) GetChangeRequest(ctx context.Context, id string) (domain.ChangeRequest, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ChangeRequest{}, err
	}

	raw, err := s.client.Get(ctx, "/change-requests/"+uuidToSysid(id), token)
	if err != nil {
		return domain.ChangeRequest{}, err
	}

	var cr snChangeRequestDetail
	if err := json.Unmarshal(raw, &cr); err != nil {
		return domain.ChangeRequest{}, fmt.Errorf("sn get change request: parse response: %w", err)
	}

	return mapSNChangeRequestDetailToView(cr), nil
}

// snChangeRequestApprovalsResponse mirrors the Choreo GET /change-requests/{id}/approvals response.
type snChangeRequestApprovalsResponse struct {
	Approvals []snChangeRequestApproval `json:"approvals"`
}

type snChangeRequestApproval struct {
	Stage        string                    `json:"stage"`
	ApproverType string                    `json:"approverType"`
	ApproverName string                    `json:"approverName"`
	Status       string                    `json:"status"`
	Approvers    []snChangeRequestApprover `json:"approvers"`
}

type snChangeRequestApprover struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	RespondedOn *string `json:"respondedOn"`
}

// GetChangeRequestApprovals returns the approval stages and per-approver status for a
// single change request identified by UUID.
func (s *snChangeRequestService) GetChangeRequestApprovals(ctx context.Context, id string) (domain.ChangeRequestApprovals, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ChangeRequestApprovals{}, err
	}

	raw, err := s.client.Get(ctx, "/change-requests/"+uuidToSysid(id)+"/approvals", token)
	if err != nil {
		return domain.ChangeRequestApprovals{}, err
	}

	var snResp snChangeRequestApprovalsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.ChangeRequestApprovals{}, fmt.Errorf("sn get change request approvals: parse response: %w", err)
	}

	approvals := make([]domain.ChangeRequestApproval, 0, len(snResp.Approvals))
	for _, a := range snResp.Approvals {
		approvers := make([]domain.ChangeRequestApprover, 0, len(a.Approvers))
		for _, ap := range a.Approvers {
			approvers = append(approvers, domain.ChangeRequestApprover{
				ID:          sysidToUUID(ap.ID),
				Name:        ap.Name,
				Status:      ap.Status,
				RespondedOn: ap.RespondedOn,
			})
		}
		approvals = append(approvals, domain.ChangeRequestApproval{
			Stage:        a.Stage,
			ApproverType: domain.ChangeRequestApproverType(a.ApproverType),
			ApproverName: a.ApproverName,
			Status:       domain.ChangeRequestApprovalStatus(a.Status),
			Approvers:    approvers,
		})
	}

	return domain.ChangeRequestApprovals{Approvals: approvals}, nil
}

// snChangeRequestApprovalDecisionPayload mirrors the Choreo
// POST /change-requests/{id}/approvals/decision request body.
type snChangeRequestApprovalDecisionPayload struct {
	Decision string `json:"decision"`
}

// snChangeRequestApprovalDecisionResponse mirrors the Choreo
// POST /change-requests/{id}/approvals/decision response.
type snChangeRequestApprovalDecisionResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// changeRequestApprovalDecisions is the set of valid values accepted for a
// change request approval decision.
var changeRequestApprovalDecisions = map[string]bool{
	"approved": true,
	"rejected": true,
}

// DecideChangeRequestApproval submits the caller's decision on their own pending
// approval for a change request. ServiceNow enforces that only the caller's own
// pending approval can be acted on; the change request's own state cascades
// automatically via ServiceNow's existing business rule.
func (s *snChangeRequestService) DecideChangeRequestApproval(ctx context.Context, id, decision string) (domain.ChangeRequestApprovalDecisionResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ChangeRequestApprovalDecisionResponse{}, err
	}

	if !changeRequestApprovalDecisions[decision] {
		return domain.ChangeRequestApprovalDecisionResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid decision %q", decision)}
	}

	payload := snChangeRequestApprovalDecisionPayload{Decision: decision}

	raw, err := s.client.Post(ctx, "/change-requests/"+uuidToSysid(id)+"/approvals/decision", token, payload)
	if err != nil {
		return domain.ChangeRequestApprovalDecisionResponse{}, err
	}

	var snResp snChangeRequestApprovalDecisionResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.ChangeRequestApprovalDecisionResponse{}, fmt.Errorf("sn decide change request approval: parse response: %w", err)
	}

	return domain.ChangeRequestApprovalDecisionResponse{
		ID:    sysidToUUID(snResp.ID),
		State: snResp.State,
	}, nil
}

// mapSNChangeRequestDetailToView maps a Choreo change-request detail payload to the domain view,
// shared by GetChangeRequest and PatchChangeRequest.
func mapSNChangeRequestDetailToView(cr snChangeRequestDetail) domain.ChangeRequest {
	subject := cr.Title
	description := cr.Description

	updatedOn := cr.CreatedOn
	if cr.UpdatedOn != nil && *cr.UpdatedOn != "" {
		updatedOn = *cr.UpdatedOn
	}

	view := domain.SearchChangeRequestView{
		ID:             sysidToUUID(cr.ID),
		Number:         cr.Number,
		Subject:        &subject,
		Description:    &description,
		Project:        domain.EntityRef{ID: sysidToUUID(cr.Project.ID), Name: cr.Project.Name},
		PlannedStartOn: cr.PlannedStartOn,
		PlannedEndOn:   cr.PlannedEndOn,
		Duration:       cr.Duration,
		Impact:         snCRImpactLabelToString(cr.Impact),
		State:          snCRStateLabelToString(cr.State),
		Type:           snCRTypeLabelToString(cr.Type),
		CreatedOn:      cr.CreatedOn,
		UpdatedOn:      updatedOn,
	}
	if cr.Case != nil {
		view.Case = &domain.EntityRef{ID: sysidToUUID(cr.Case.ID), Name: cr.Case.Name}
	}
	if cr.Deployment != nil {
		view.Deployment = &domain.EntityRef{ID: sysidToUUID(cr.Deployment.ID), Name: cr.Deployment.Name}
	}
	if cr.DeployedProduct != nil {
		view.DeployedProduct = &domain.EntityRef{ID: sysidToUUID(cr.DeployedProduct.ID), Name: cr.DeployedProduct.Name}
	}
	if cr.Product != nil {
		view.Product = &domain.EntityRef{ID: sysidToUUID(cr.Product.ID), Name: cr.Product.Name}
	}
	if cr.AssignedEngineer != nil {
		view.AssignedEngineer = &domain.EntityRef{ID: sysidToUUID(cr.AssignedEngineer.ID), Name: cr.AssignedEngineer.Name}
	}
	if cr.AssignedTeam != nil {
		view.AssignedTeam = &domain.EntityRef{ID: sysidToUUID(cr.AssignedTeam.ID), Name: cr.AssignedTeam.Name}
	}

	result := domain.ChangeRequest{
		SearchChangeRequestView: view,
		CreatedBy:               cr.CreatedBy,
		Justification:           cr.Justification,
		ImpactDescription:       cr.ImpactDescription,
		ServiceOutage:           cr.ServiceOutage,
		CommunicationPlan:       cr.CommunicationPlan,
		RollbackPlan:            cr.RollbackPlan,
		TestPlan:                cr.TestPlan,
		HasCustomerApproved:     cr.HasCustomerApproved,
		HasCustomerReviewed:     cr.HasCustomerReviewed,
		ApprovedOn:              cr.ApprovedOn,
		LegalNextStates:         withoutCustomerOutcomeStates(cr.LegalNextStates, view.State),

		// Field-parity additions.
		ImplementationPlan:           cr.ImplementationPlan,
		AffectedServicesText:         cr.AffectedServicesText,
		AffectedComponentsText:       cr.AffectedComponentsText,
		RollbackDurationText:         cr.RollbackDurationText,
		IsPlanningVisibleToCustomers: cr.IsPlanningVisibleToCustomers,
		ConfirmCustomerUpdatedDate:   cr.ConfirmCustomerUpdatedDate,
		CustomerUpdatedOn:            cr.CustomerUpdatedOn,
		Labels:                       cr.Labels,
		WorkStart:                    cr.WorkStart,
		WorkEnd:                      cr.WorkEnd,
		GitReference:                 cr.GitReference,
	}
	if cr.ApprovedBy != nil {
		result.ApprovedBy = &domain.EntityRef{ID: sysidToUUID(cr.ApprovedBy.ID), Name: cr.ApprovedBy.Name}
	}
	if cr.Priority != nil {
		if label, ok := snCRPriorityLabelMap[cr.Priority.ID]; ok {
			result.Priority = &label
		} else {
			result.Priority = &cr.Priority.Label
		}
	}
	if cr.Category != nil {
		if label, ok := snCRCategoryLabelMap[cr.Category.ID]; ok {
			result.Category = &label
		} else {
			result.Category = &cr.Category.Label
		}
	}
	if cr.RequestedBy != nil {
		result.RequestedBy = &domain.EntityRef{ID: sysidToUUID(cr.RequestedBy.ID), Name: cr.RequestedBy.Name}
	}
	if cr.ChangeRequestType != nil {
		result.ChangeRequestType = &cr.ChangeRequestType.Label
	}
	if cr.Likelihood != nil {
		result.Likelihood = &cr.Likelihood.Label
	}
	if len(cr.DeploymentProducts) > 0 {
		products := make([]domain.EntityRef, 0, len(cr.DeploymentProducts))
		for _, p := range cr.DeploymentProducts {
			products = append(products, domain.EntityRef{ID: sysidToUUID(p.ID), Name: p.Name})
		}
		result.DeploymentProducts = products
	}
	if len(cr.Deployments) > 0 {
		deployments := make([]domain.EntityRef, 0, len(cr.Deployments))
		for _, d := range cr.Deployments {
			deployments = append(deployments, domain.EntityRef{ID: sysidToUUID(d.ID), Name: d.Name})
		}
		result.Deployments = deployments
	}

	return result
}

// withoutCustomerOutcomeStates drops the two next states that only the CUSTOMER
// can reach from the next states ServiceNow offers, so that this data source's
// legalNextStates answer is the one the PostgreSQL data source gives
// (repository.changeRequestForwardNextStates): "scheduled" is never offered,
// from any state -- there is no Schedule action, a change becomes Scheduled when
// its CAB approval is granted or, out of Customer
// Approval, when the customer approves -- and "closed" is never offered from
// Customer Review, which only the customer's own review closes. No staff action
// records the customer's approval or review on their behalf (a compliance rule:
// ServiceNow's record of it is audited), so the portal must never be handed
// either as something to click. Rollback (the failed-review off-ramp
// ServiceNow offers from Review and Customer Review) and Cancel are never
// stripped; "closed" from Review stays. This filters OUR response only: what
// ServiceNow itself returns is unchanged.
func withoutCustomerOutcomeStates(states []string, state *string) []string {
	if states == nil {
		return nil
	}
	fromCustomerReview := state != nil && strings.EqualFold(*state, string(domain.ChangeRequestStateCustomerReview))
	out := make([]string, 0, len(states))
	for _, st := range states {
		if strings.EqualFold(st, string(domain.ChangeRequestStateScheduled)) {
			continue
		}
		if fromCustomerReview && strings.EqualFold(st, string(domain.ChangeRequestStateClosed)) {
			continue
		}
		out = append(out, st)
	}
	return out
}

// GetChangeRequestLinkOptions implements ChangeRequestService. The
// project/deployment/environment cascade is derived from PostgreSQL tables, so
// it is not available when ServiceNow is the data source.
func (s *snChangeRequestService) GetChangeRequestLinkOptions(_ context.Context, _ domain.ChangeRequestLinkOptionsRequest) (domain.ChangeRequestLinkOptionsResponse, error) {
	return domain.ChangeRequestLinkOptionsResponse{}, &apierror.ValidationError{Msg: "change request link options are not supported on the ServiceNow data source"}
}
