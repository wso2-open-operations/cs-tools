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
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
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

	token := middleware.UserIDTokenFromContext(ctx)

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
			ProjectIDs:      uuidsToSysids(req.Filters.ProjectIDs),
			SearchQuery:     req.Filters.SearchQuery,
			StateKeys:       domainCRStatesToSNIDs(req.Filters.States),
			ImpactKeys:      domainCRImpactsToSNIDs(req.Filters.Impacts),
			ClosedStartDate: formatSNDate(req.Filters.ClosedStartDate),
			ClosedEndDate:   formatSNDate(req.Filters.ClosedEndDate),
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

	return domain.SearchChangeRequestsResponse{
		ChangeRequests: views,
		Total:          snResp.TotalRecords,
		Limit:          req.Pagination.Limit,
		Offset:         req.Pagination.Offset,
	}, nil
}

// snCreateChangeRequestPayload is the Choreo POST /change-requests request body.
type snCreateChangeRequestPayload struct {
	Subject             string  `json:"subject"`
	CategoryKey         *string `json:"categoryKey,omitempty"`
	ServiceID           *string `json:"serviceId,omitempty"`
	ServiceOfferingID   *string `json:"serviceOfferingId,omitempty"`
	ConfigurationItemID *string `json:"configurationItemId,omitempty"`
	PriorityKey         *string `json:"priorityKey,omitempty"`
	ImpactKey           *string `json:"impactKey,omitempty"`
	TypeKey             *string `json:"typeKey,omitempty"`
	StateKey            *string `json:"stateKey,omitempty"`
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

// snCRCreateStateIDMap maps domain ChangeRequestState enums to SN string state IDs for create.
var snCRCreateStateIDMap = map[domain.ChangeRequestState]string{
	domain.ChangeRequestStateNew:              "-5",
	domain.ChangeRequestStateAssess:           "-4",
	domain.ChangeRequestStateAuthorize:        "-3",
	domain.ChangeRequestStateCustomerApproval: "5",
	domain.ChangeRequestStateScheduled:        "-2",
	domain.ChangeRequestStateImplement:        "-1",
	domain.ChangeRequestStateReview:           "0",
	domain.ChangeRequestStateCustomerReview:   "1",
	domain.ChangeRequestStateRollback:         "2",
	domain.ChangeRequestStateClosed:           "3",
	domain.ChangeRequestStateCanceled:         "4",
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

// CreateChangeRequest implements ChangeRequestService for the ServiceNow data source.
func (s *snChangeRequestService) CreateChangeRequest(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
	if req.Subject == "" {
		return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: "subject is required"}
	}
	if req.Category != nil {
		if _, ok := snCRCategoryIDMap[*req.Category]; !ok {
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid category %q", *req.Category)}
		}
	}
	if req.Type != nil {
		if _, ok := snCRCreateTypeIDMap[*req.Type]; !ok {
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid type %q", *req.Type)}
		}
	}
	if req.State != nil {
		if _, ok := snCRCreateStateIDMap[*req.State]; !ok {
			return domain.CreateChangeRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid state %q", *req.State)}
		}
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

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snCreateChangeRequestPayload{
		Subject:            req.Subject,
		Description:        req.Description,
		Justification:      req.Justification,
		ImplementationPlan: req.ImplementationPlan,
		RiskImpactAnalysis: req.RiskImpactAnalysis,
		BackoutPlan:        req.BackoutPlan,
		TestPlan:           req.TestPlan,
		PlannedStartDate:   req.PlannedStartDate,
		PlannedEndDate:     req.PlannedEndDate,
		Comment:            req.Comment,
		WorkNote:           req.WorkNote,
	}
	if req.Category != nil {
		v := snCRCategoryIDMap[*req.Category]
		payload.CategoryKey = &v
	}
	if req.Type != nil {
		v := snCRCreateTypeIDMap[*req.Type]
		payload.TypeKey = &v
	}
	if req.State != nil {
		v := snCRCreateStateIDMap[*req.State]
		payload.StateKey = &v
	}
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
		return domain.CreateChangeRequestResponse{}, fmt.Errorf("sn create change request: parse response: %w", err)
	}

	resp := domain.CreateChangeRequestResponse{Message: snResp.Message}
	resp.ChangeRequest.ID = sysidToUUID(snResp.ChangeRequest.ID)
	resp.ChangeRequest.Number = snResp.ChangeRequest.Number
	resp.ChangeRequest.CreatedOn = snResp.ChangeRequest.CreatedOn
	resp.ChangeRequest.CreatedBy = snResp.ChangeRequest.CreatedBy
	return resp, nil
}

// snCRPatchStateIDMap maps domain ChangeRequestState enums to SN numeric state IDs for PATCH.
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

// snPatchChangeRequestPayload mirrors the Choreo PATCH /change-requests/{id} request body.
type snPatchChangeRequestPayload struct {
	Title              *string `json:"title,omitempty"`
	Description        *string `json:"description,omitempty"`
	ProjectID          *string `json:"projectId,omitempty"`
	CaseID             *string `json:"caseId,omitempty"`
	DeploymentID       *string `json:"deploymentId,omitempty"`
	DeployedProductID  *string `json:"deployedProductId,omitempty"`
	AssignedEngineerID *string `json:"assignedEngineerId,omitempty"`
	AssignedTeamID     *string `json:"assignedTeamId,omitempty"`
	PlannedStartOn     *string `json:"plannedStartOn,omitempty"`
	PlannedEndOn       *string `json:"plannedEndOn,omitempty"`
	ImpactKey          *int    `json:"impactKey,omitempty"`
	StateKey           *int    `json:"stateKey,omitempty"`
	TypeKey            *string `json:"typeKey,omitempty"`
	Justification      *string `json:"justification,omitempty"`
	ImpactDescription  *string `json:"impactDescription,omitempty"`
	ServiceOutage      *string `json:"serviceOutage,omitempty"`
	CommunicationPlan  *string `json:"communicationPlan,omitempty"`
	RollbackPlan       *string `json:"rollbackPlan,omitempty"`
	TestPlan           *string `json:"testPlan,omitempty"`
	IsCustomerApproved *bool   `json:"isCustomerApproved,omitempty"`
	IsCustomerReviewed *bool   `json:"isCustomerReviewed,omitempty"`
	RequestApproval    *bool   `json:"requestApproval,omitempty"`
}

// snPatchChangeRequestResponse mirrors the Choreo PATCH /change-requests/{id} response.
type snPatchChangeRequestResponse struct {
	Message       string                `json:"message"`
	ChangeRequest snChangeRequestDetail `json:"changeRequest"`
}

func (s *snChangeRequestService) PatchChangeRequest(ctx context.Context, id string, req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.PatchChangeRequestResponse{}, err
	}

	if req.Title == nil && req.Description == nil && req.ProjectID == nil && req.CaseID == nil &&
		req.DeploymentID == nil && req.DeployedProductID == nil && req.AssignedEngineerID == nil &&
		req.AssignedTeamID == nil && req.PlannedStartOn == nil && req.PlannedEndOn == nil &&
		req.Impact == nil && req.State == nil && req.Type == nil && req.Justification == nil &&
		req.ImpactDescription == nil && req.ServiceOutage == nil && req.CommunicationPlan == nil &&
		req.RollbackPlan == nil && req.TestPlan == nil && req.IsCustomerApproved == nil &&
		req.IsCustomerReviewed == nil && req.RequestApproval == nil {
		return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
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
	if req.PlannedStartOn != nil {
		if _, err := time.Parse(snCreatedOnLayout, *req.PlannedStartOn); err != nil {
			return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "plannedStartOn must follow the format: YYYY-MM-DD HH:mm:ss"}
		}
	}
	if req.PlannedEndOn != nil {
		if _, err := time.Parse(snCreatedOnLayout, *req.PlannedEndOn); err != nil {
			return domain.PatchChangeRequestResponse{}, &apierror.ValidationError{Msg: "plannedEndOn must follow the format: YYYY-MM-DD HH:mm:ss"}
		}
	}

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

	payload := snPatchChangeRequestPayload{
		Title:              req.Title,
		Description:        req.Description,
		PlannedStartOn:     req.PlannedStartOn,
		PlannedEndOn:       req.PlannedEndOn,
		Justification:      req.Justification,
		ImpactDescription:  req.ImpactDescription,
		ServiceOutage:      req.ServiceOutage,
		CommunicationPlan:  req.CommunicationPlan,
		RollbackPlan:       req.RollbackPlan,
		TestPlan:           req.TestPlan,
		IsCustomerApproved: req.IsCustomerApproved,
		IsCustomerReviewed: req.IsCustomerReviewed,
		RequestApproval:    req.RequestApproval,
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
		return domain.PatchChangeRequestResponse{}, fmt.Errorf("sn patch change request: parse response: %w", err)
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
		LegalNextStates:         cr.LegalNextStates,
	}
	if cr.ApprovedBy != nil {
		result.ApprovedBy = &domain.EntityRef{ID: sysidToUUID(cr.ApprovedBy.ID), Name: cr.ApprovedBy.Name}
	}

	return result
}
