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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snCasesResponse mirrors the Choreo POST /cases/search response.
type snCasesResponse struct {
	Cases        []snCase `json:"cases"`
	TotalRecords int      `json:"totalRecords"`
	Offset       int      `json:"offset"`
	Limit        int      `json:"limit"`
}

// snServiceRequestsResponse mirrors the Choreo POST /cases/search response for service_request cases.
type snServiceRequestsResponse struct {
	Cases        []snServiceRequestCase `json:"cases"`
	TotalRecords int                    `json:"totalRecords"`
	Offset       int                    `json:"offset"`
	Limit        int                    `json:"limit"`
}

type snServiceRequestCase struct {
	ID               string                     `json:"id"`
	InternalID       string                     `json:"internalId"`
	Number           string                     `json:"number"`
	Title            *string                    `json:"title"`
	Description      *string                    `json:"description"`
	CreatedOn        string                     `json:"createdOn"`
	CreatedBy        string                     `json:"createdBy"`
	State            *snCaseState               `json:"state"`
	WorkState        *snServiceRequestWorkState `json:"workState"`
	Project          snCaseEntityRef            `json:"project"`
	Deployment       snCaseEntityRef            `json:"deployment"`
	DeployedProduct  snCaseDeployedProduct      `json:"deployedProduct"`
	Product          *snCaseEntityRef           `json:"product"`
	Catalog          *snCaseEntityRef           `json:"catalog"`
	CatalogItem      *snCaseEntityRef           `json:"catalogItem"`
	AssignedTeam     *snCaseEntityRef           `json:"assignedTeam"`
	AssignedEngineer *snAssignedEngineerRef      `json:"assignedEngineer"`
	ParentCase       *snCaseRef                 `json:"parentCase"`
	RelatedCase      *snCaseRef                 `json:"relatedCase"`
}

type snServiceRequestWorkState struct {
	ID    *int   `json:"id"`
	Label string `json:"label"`
}

type snCase struct {
	ID               string                `json:"id"`
	InternalID       string                `json:"internalId"`
	Number           string                `json:"number"`
	Title            string                `json:"title"`
	Description      string                `json:"description"`
	CreatedOn        string                `json:"createdOn"`
	UpdatedOn        *string               `json:"updatedOn"`
	CreatedBy        string                `json:"createdBy"`
	Project          snCaseEntityRef       `json:"project"`
	Deployment       snCaseEntityRef       `json:"deployment"`
	DeployedProduct  snCaseDeployedProduct `json:"deployedProduct"`
	Product          *snCaseEntityRef      `json:"product"`
	State            *snCaseState          `json:"state"`
	WorkState        *snCaseLabel          `json:"workState"`
	Severity         *snCaseLabel          `json:"severity"`
	IssueType        *snCaseIssueType      `json:"issueType"`
	AssignedEngineer *snAssignedEngineerRef `json:"assignedEngineer"`
	ParentCase       *snCaseRef            `json:"parentCase"`
	RelatedCase      *snCaseRef            `json:"relatedCase"`
	Account          *snCaseAccount        `json:"account"`
}

type snCaseEntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snCaseDeployedProduct struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type snCaseRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

type snAssignedEngineerRef struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email"`
}

type snCaseAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type snCaseState struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type snCaseLabel struct {
	Label string `json:"label"`
}

type snCaseIssueType struct {
	ID    json.Number `json:"id"`
	Label string      `json:"label"`
}

// snCaseSearchPayload is the Choreo POST /cases/search request body.
type snCaseSearchPayload struct {
	Filters    snCaseFilters       `json:"filters,omitempty"`
	SortBy     *snCaseSort         `json:"sortBy,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snCaseSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// snSortFieldMap maps domain CaseSortField values to SN field names.
var snSortFieldMap = map[domain.CaseSortField]string{
	domain.CaseSortFieldCreatedAt: "createdOn",
	domain.CaseSortFieldUpdatedAt: "updatedOn",
}

type snCaseFilters struct {
	CaseTypes          []string  `json:"caseTypes"`
	SearchQuery        string    `json:"searchQuery,omitempty"`
	ProjectIDs         []string  `json:"projectIds,omitempty"`
	DeploymentIDs      []string  `json:"deploymentIds,omitempty"`
	DeployedProductIDs []string  `json:"deployedProductIds,omitempty"`
	StateKeys          []int     `json:"stateKeys,omitempty"`
	SeverityKeys       []int     `json:"severityKeys,omitempty"`
	IssueTypeKeys      []int     `json:"issueTypeKeys,omitempty"`
	ClosedStartDate    string    `json:"closedStartDate,omitempty"`
	ClosedEndDate      string    `json:"closedEndDate,omitempty"`
	StartCreatedDate   string    `json:"startCreatedDate,omitempty"`
	EndCreatedDate     string    `json:"endCreatedDate,omitempty"`
	StartUpdatedDate   string    `json:"startUpdatedDate,omitempty"`
	EndUpdatedDate     string    `json:"endUpdatedDate,omitempty"`
	CreatedBy          []string  `json:"createdBy,omitempty"`
	CreatedByMe        bool      `json:"createdByMe,omitempty"`
}

// snStateIDMap maps domain CaseState enums to SN numeric state IDs.
var snStateIDMap = map[domain.CaseState]int{
	domain.CaseStateOpen:             1,
	domain.CaseStateWorkInProgress:   10,
	domain.CaseStateAwaitingInfo:     18,
	domain.CaseStateWaitingOnWSO2:    1003,
	domain.CaseStateSolutionProposed: 6,
	domain.CaseStateClosed:           3,
}

// snSeverityIDMap maps domain CasePriority enums to SN numeric severity IDs.
// CasePriorityCatastrophic is intentionally absent: ServiceNow's severity scale
// only goes up to Critical (P1=10) and has no catastrophic equivalent.
var snSeverityIDMap = map[domain.CasePriority]int{
	domain.CasePriorityCritical: 10,
	domain.CasePriorityHigh:     11,
	domain.CasePriorityMedium:   12,
	domain.CasePriorityLow:      13,
}

// snIssueTypeIDMap maps domain CaseIssueType enums to SN numeric issue type IDs.
var snIssueTypeIDMap = map[domain.CaseIssueType]int{
	domain.CaseIssueTypeTotalOutage:            1,
	domain.CaseIssueTypePartialOutage:          2,
	domain.CaseIssueTypePerformanceDegradation: 3,
	domain.CaseIssueTypeQuestion:               4,
	domain.CaseIssueTypeSecurityOrCompliance:   5,
	domain.CaseIssueTypeError:                  6,
}

func domainStatesToSNIDs(states []domain.CaseState) []int {
	ids := make([]int, 0, len(states))
	for _, s := range states {
		if id, ok := snStateIDMap[s]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func domainPrioritiesToSNIDs(priorities []domain.CasePriority) []int {
	ids := make([]int, 0, len(priorities))
	for _, p := range priorities {
		if id, ok := snSeverityIDMap[p]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func domainIssueTypesToSNIDs(issueTypes []domain.CaseIssueType) []int {
	ids := make([]int, 0, len(issueTypes))
	for _, it := range issueTypes {
		if id, ok := snIssueTypeIDMap[it]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func formatSNDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(snCreatedOnLayout)
}

type snCaseService struct {
	client     *integrationservice.Client
	pgFallback CaseService
}

// NewSNCaseService constructs a CaseService that delegates SearchCases to the
// Choreo API and all write/read-by-id operations to pgFallback.
func NewServiceNowCaseService(client *integrationservice.Client, pgFallback CaseService) CaseService {
	return &snCaseService{client: client, pgFallback: pgFallback}
}

// snPriorityID maps domain CasePriority to the ServiceNow severity choice-list value.
var snPriorityID = map[domain.CasePriority]int{
	domain.CasePriorityCatastrophic: 14,
	domain.CasePriorityCritical:     10,
	domain.CasePriorityHigh:         11,
	domain.CasePriorityMedium:       12,
	domain.CasePriorityLow:          13,
}

// snIssueTypeID maps domain CaseIssueType to the ServiceNow issue-type choice-list value.
var snIssueTypeID = map[domain.CaseIssueType]int{
	domain.CaseIssueTypeTotalOutage:            1,
	domain.CaseIssueTypePartialOutage:          2,
	domain.CaseIssueTypePerformanceDegradation: 3,
	domain.CaseIssueTypeQuestion:               4,
	domain.CaseIssueTypeSecurityOrCompliance:   5,
	domain.CaseIssueTypeError:                  6,
}

type snCreateCasePayload struct {
	Type              string `json:"type"`
	ProjectID         string `json:"projectId"`
	DeploymentID      string `json:"deploymentId"`
	DeployedProductID string `json:"deployedProductId"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	SeverityKey       int    `json:"severityKey"`
	IssueTypeKey      int    `json:"issueTypeKey"`
}

type snCreateCaseResponse struct {
	Message string `json:"message"`
	Case    struct {
		ID         string       `json:"id"`
		InternalID string       `json:"internalId"`
		Number     string       `json:"number"`
		CreatedBy  string       `json:"createdBy"`
		CreatedOn  string       `json:"createdOn"`
		State      *snCaseState `json:"state"`
	} `json:"case"`
}

func (s *snCaseService) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
	if err := validateCreateCaseRequest(req); err != nil {
		return domain.CreateCaseResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CreateCaseResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snCreateCasePayload{
		Type:              "default_case",
		ProjectID:         uuidToSysid(req.ProjectID),
		DeploymentID:      uuidToSysid(req.DeploymentID),
		DeployedProductID: uuidToSysid(req.DeployedProductID),
		Title:             req.Subject,
		Description:       req.Description,
		SeverityKey:       snPriorityID[req.Priority],
		IssueTypeKey:      snIssueTypeID[req.IssueType],
	}

	raw, err := s.client.Post(ctx, "/cases", token, payload)
	if err != nil {
		return domain.CreateCaseResponse{}, err
	}

	var snResp snCreateCaseResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateCaseResponse{}, fmt.Errorf("sn create case: parse response: %w", err)
	}

	createdOn, err := time.Parse(snCreatedOnLayout, snResp.Case.CreatedOn)
	if err != nil {
		return domain.CreateCaseResponse{}, fmt.Errorf("sn create case: parse createdOn %q: %w", snResp.Case.CreatedOn, err)
	}

	stateLabel := ""
	if snResp.Case.State != nil {
		stateLabel = snResp.Case.State.Label
	}

	return domain.CreateCaseResponse{
		Message: snResp.Message,
		Case: domain.CreateCaseDetails{
			ID:         sysidToUUID(snResp.Case.ID),
			InternalID: snResp.Case.InternalID,
			Number:     snResp.Case.Number,
			CreatedBy:  snResp.Case.CreatedBy,
			CreatedOn:  createdOn,
			State:      stateLabel,
		},
	}, nil
}

func (s *snCaseService) GetCaseByID(ctx context.Context, id string) (domain.CaseView, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CaseView{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	raw, err := s.client.Get(ctx, "/cases/"+uuidToSysid(id), token)
	if err != nil {
		return domain.CaseView{}, err
	}

	var c snCase
	if err := json.Unmarshal(raw, &c); err != nil {
		return domain.CaseView{}, fmt.Errorf("sn get case: parse response: %w", err)
	}

	createdOn, err := time.Parse(snCreatedOnLayout, c.CreatedOn)
	if err != nil {
		return domain.CaseView{}, fmt.Errorf("sn get case: parse createdOn %q: %w", c.CreatedOn, err)
	}
	updatedOn := createdOn
	if c.UpdatedOn != nil && *c.UpdatedOn != "" {
		updatedOn, err = time.Parse(snCreatedOnLayout, *c.UpdatedOn)
		if err != nil {
			return domain.CaseView{}, fmt.Errorf("sn get case: parse updatedOn %q: %w", *c.UpdatedOn, err)
		}
	}

	state, err := snCaseStateLabelToEnum(c.State)
	if err != nil {
		return domain.CaseView{}, fmt.Errorf("sn get case %q: %w", c.ID, err)
	}

	cv := domain.CaseView{
		ID:          sysidToUUID(c.ID),
		Number:      c.Number,
		InternalID:  c.InternalID,
		Subject:     c.Title,
		Description: c.Description,
		Priority:    snSeverityToPriority(c.Severity),
		IssueType:   snIssueTypeToEnum(c.IssueType),
		State:       state,
		WorkState:   snWorkStateLabelToEnum(c.WorkState),
		CreatedOn:   createdOn,
		UpdatedOn:   updatedOn,
		CreatedByDetails: domain.UserRef{
			Email: c.CreatedBy,
		},
		ProjectDetails:    domain.EntityRef{ID: sysidToUUID(c.Project.ID), Name: c.Project.Name},
		DeploymentDetails: domain.EntityRef{ID: sysidToUUID(c.Deployment.ID), Name: c.Deployment.Name},
		DeployedProductDetails: domain.DeployedProductRef{
			ID:          sysidToUUID(c.DeployedProduct.ID),
			DisplayName: strings.TrimSpace(c.DeployedProduct.Name + " " + c.DeployedProduct.Version),
		},
	}

	if c.Product != nil {
		cv.ProductDetails = domain.EntityRef{ID: sysidToUUID(c.Product.ID), Name: c.Product.Name}
	}
	if c.AssignedEngineer != nil {
		cv.AssignedEngineer = &domain.AssignedEngineerRef{ID: sysidToUUID(c.AssignedEngineer.ID), Name: c.AssignedEngineer.Name, Email: c.AssignedEngineer.Email}
	}
	if c.ParentCase != nil {
		cv.ParentCase = &domain.CaseNumberRef{ID: sysidToUUID(c.ParentCase.ID), Number: c.ParentCase.Number}
	}
	if c.RelatedCase != nil {
		cv.RelatedCase = &domain.CaseNumberRef{ID: sysidToUUID(c.RelatedCase.ID), Number: c.RelatedCase.Number}
	}
	if c.Account != nil {
		cv.AccountDetails = &domain.AccountRef{ID: sysidToUUID(c.Account.ID), Name: c.Account.Name, Type: c.Account.Type}
	}

	return cv, nil
}

type snCreateCommentPayload struct {
	ReferenceID   string `json:"referenceId"`
	ReferenceType string `json:"referenceType"`
	Type          string `json:"type"`
	Content       string `json:"content"`
}

type snCreateCommentResponse struct {
	Message string `json:"message"`
	Comment struct {
		ID        string `json:"id"`
		CreatedOn string `json:"createdOn"`
		CreatedBy string `json:"createdBy"`
	} `json:"comment"`
}

func (s *snCaseService) CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CreateCaseCommentResponse, error) {
	if !validCommentType[req.Type] {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + string(req.Type)}
	}
	if req.Content == "" {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "content is required"}
	}
	if req.Type == domain.CommentTypeActivity {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "type 'activity' is not supported for ServiceNow"}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CreateCaseCommentResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	snType := snCommentTypeMap[req.Type]

	payload := snCreateCommentPayload{
		ReferenceID:   uuidToSysid(req.CaseID),
		ReferenceType: "case",
		Type:          snType,
		Content:       req.Content,
	}

	raw, err := s.client.Post(ctx, "/comments", token, payload)
	if err != nil {
		return domain.CreateCaseCommentResponse{}, err
	}

	var snResp snCreateCommentResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateCaseCommentResponse{}, fmt.Errorf("sn create comment: parse response: %w", err)
	}

	createdOn, err := time.Parse(snCreatedOnLayout, snResp.Comment.CreatedOn)
	if err != nil {
		return domain.CreateCaseCommentResponse{}, fmt.Errorf("sn create comment: parse createdOn %q: %w", snResp.Comment.CreatedOn, err)
	}

	return domain.CreateCaseCommentResponse{
		Message: snResp.Message,
		Comment: domain.CaseCommentDetail{
			ID:        sysidToUUID(snResp.Comment.ID),
			CreatedOn: createdOn,
			CreatedBy: snResp.Comment.CreatedBy,
		},
	}, nil
}

type snCommentFilters struct {
	Type string `json:"type,omitempty"`
}

type snSearchCommentsPayload struct {
	ReferenceID   string             `json:"referenceId"`
	ReferenceType string             `json:"referenceType"`
	Filters       *snCommentFilters  `json:"filters,omitempty"`
	Pagination    snProjectPagination `json:"pagination"`
}

type snComment struct {
	ID                  string `json:"id"`
	ReferenceID         string `json:"referenceId"`
	Content             string `json:"content"`
	Type                string `json:"type"`
	CreatedOn           string `json:"createdOn"`
	CreatedBy           string `json:"createdBy"`
	CreatedByFirstName  string `json:"createdByFirstName"`
	CreatedByLastName   string `json:"createdByLastName"`
	CreatedByFullName   string `json:"createdByFullName"`
}

type snSearchCommentsResponse struct {
	Comments     []snComment `json:"comments"`
	Offset       int         `json:"offset"`
	Limit        int         `json:"limit"`
	TotalRecords int         `json:"totalRecords"`
}

var snCommentTypeMap = map[domain.CommentType]string{
	domain.CommentTypeComment:  "comments",
	domain.CommentTypeWorkNote: "work_notes",
}

func (s *snCaseService) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchCaseCommentsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snSearchCommentsPayload{
		ReferenceID:   uuidToSysid(req.CaseID),
		ReferenceType: "case",
		Pagination:    snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	if req.Filters != nil && req.Filters.Type != nil {
		snType, ok := snCommentTypeMap[*req.Filters.Type]
		if !ok {
			return domain.SearchCaseCommentsResponse{}, &apierror.ValidationError{
				Msg: "filters.type is not supported for ServiceNow: " + string(*req.Filters.Type),
			}
		}
		payload.Filters = &snCommentFilters{Type: snType}
	}

	raw, err := s.client.Post(ctx, "/comments/search", token, payload)
	if err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}

	var snResp snSearchCommentsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCaseCommentsResponse{}, fmt.Errorf("sn search comments: parse response: %w", err)
	}

	comments := make([]domain.CaseComment, 0, len(snResp.Comments))
	for _, c := range snResp.Comments {
		createdAt, err := time.Parse(snCreatedOnLayout, c.CreatedOn)
		if err != nil {
			return domain.SearchCaseCommentsResponse{}, fmt.Errorf("sn search comments: parse createdOn %q: %w", c.CreatedOn, err)
		}
		var commentType domain.CommentType
		switch c.Type {
		case "comments", "comment":
			commentType = domain.CommentTypeComment
		case "work_notes", "work_note":
			commentType = domain.CommentTypeWorkNote
		case "activity":
			commentType = domain.CommentTypeActivity
		default:
			commentType = domain.CommentTypeComment
		}
		comments = append(comments, domain.CaseComment{
			ID:      sysidToUUID(c.ID),
			CaseID:  sysidToUUID(c.ReferenceID),
			Type:    commentType,
			Content: c.Content,
			CreatedBy: domain.CommentUserRef{
				ID:        c.CreatedBy,
				FirstName: c.CreatedByFirstName,
				LastName:  c.CreatedByLastName,
				FullName:  c.CreatedByFullName,
			},
			CreatedOn: createdAt,
		})
	}

	total := snResp.TotalRecords
	return domain.SearchCaseCommentsResponse{
		Comments: comments,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(comments) < total,
	}, nil
}

type snUpdateCasePayload struct {
	StateKey      *int     `json:"stateKey,omitempty"`
	SeverityKey   *int     `json:"severityKey,omitempty"`
	WatchList     []string `json:"watchList,omitempty"`
	AssigneeEmail *string  `json:"assigneeEmail,omitempty"`
}

type snUpdateCaseResponse struct {
	Message string `json:"message"`
	Case    struct {
		ID        string        `json:"id"`
		UpdatedOn string        `json:"updatedOn"`
		UpdatedBy string        `json:"updatedBy"`
		State     *snCaseState  `json:"state"`
		Severity  *snCaseLabel  `json:"severity"`
		WatchList []struct {
			ID       string `json:"id"`
			UserName string `json:"userName"`
			Name     string `json:"name"`
			Email    string `json:"email"`
		} `json:"watchList"`
		AssignedTo *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"assignedTo"`
	} `json:"case"`
}

func (s *snCaseService) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	fieldCount := 0
	if req.State != nil {
		fieldCount++
	}
	if req.Priority != nil {
		fieldCount++
	}
	if len(req.WatchList) > 0 {
		fieldCount++
	}
	if req.AssigneeEmail != nil {
		fieldCount++
	}
	if fieldCount == 0 {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "at least one of state, priority, watchList, or assigneeEmail must be provided"}
	}
	if fieldCount > 1 {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "only one of state, priority, watchList, or assigneeEmail may be provided per request"}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.UpdateCaseResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snUpdateCasePayload{}
	if req.State != nil {
		if !validCaseState[*req.State] {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(*req.State)}
		}
		id, ok := snStateIDMap[*req.State]
		if !ok {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "state " + string(*req.State) + " is not supported by ServiceNow"}
		}
		payload.StateKey = &id
	}
	if req.Priority != nil {
		if !validCasePriority[*req.Priority] {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "priority contains invalid value: " + string(*req.Priority)}
		}
		id, ok := snSeverityIDMap[*req.Priority]
		if !ok {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "priority " + string(*req.Priority) + " is not supported by ServiceNow"}
		}
		payload.SeverityKey = &id
	}
	if len(req.WatchList) > 0 {
		payload.WatchList = req.WatchList
	}
	if req.AssigneeEmail != nil {
		payload.AssigneeEmail = req.AssigneeEmail
	}

	raw, err := s.client.Patch(ctx, "/cases/"+uuidToSysid(req.ID), token, payload)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	var snResp snUpdateCaseResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.UpdateCaseResponse{}, fmt.Errorf("sn update case: parse response: %w", err)
	}

	updatedOn, err := time.Parse(snCreatedOnLayout, snResp.Case.UpdatedOn)
	if err != nil {
		return domain.UpdateCaseResponse{}, fmt.Errorf("sn update case: parse updatedOn %q: %w", snResp.Case.UpdatedOn, err)
	}

	resp := domain.UpdateCaseResponse{
		Message: snResp.Message,
		Case: domain.UpdatedCase{
			ID:        sysidToUUID(snResp.Case.ID),
			UpdatedOn: updatedOn,
			UpdatedBy: snResp.Case.UpdatedBy,
		},
	}

	if snResp.Case.State != nil {
		state, err := snCaseStateLabelToEnum(snResp.Case.State)
		if err == nil {
			resp.Case.State = state
		}
	}
	if snResp.Case.Severity != nil {
		resp.Case.Priority = snSeverityToPriority(snResp.Case.Severity)
	}
	if snResp.Case.AssignedTo != nil {
		resp.Case.AssignedTo = &domain.AssignedEngineerRef{
			ID:   sysidToUUID(snResp.Case.AssignedTo.ID),
			Name: snResp.Case.AssignedTo.Name,
		}
	}
	if len(snResp.Case.WatchList) > 0 {
		wl := make([]domain.WatchListUser, 0, len(snResp.Case.WatchList))
		for _, u := range snResp.Case.WatchList {
			wl = append(wl, domain.WatchListUser{
				ID:       sysidToUUID(u.ID),
				UserName: u.UserName,
				Name:     u.Name,
				Email:    u.Email,
			})
		}
		resp.Case.WatchList = wl
	}

	return resp, nil
}

type snCreateAttachmentPayload struct {
	ReferenceID   string  `json:"referenceId"`
	ReferenceType string  `json:"referenceType"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	File          string  `json:"file"`
	Description   *string `json:"description,omitempty"`
}

type snCreateAttachmentResponse struct {
	Message    string `json:"message"`
	Attachment struct {
		ID          string `json:"id"`
		SizeBytes   int    `json:"sizeBytes"`
		CreatedOn   string `json:"createdOn"`
		CreatedBy   string `json:"createdBy"`
		DownloadURL string `json:"downloadUrl"`
	} `json:"attachment"`
}

const maxAttachmentBytes = 10 * 1024 * 1024 // 10 MB decoded

func (s *snCaseService) CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
	if req.Name == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "name is required"}
	}
	if req.Type == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "type is required"}
	}
	if req.File == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file is required"}
	}

	// file must be a data URI: data:<mime>;base64,<encoded>
	const dataURIPrefix = "data:"
	const base64Marker = ";base64,"
	if !strings.HasPrefix(req.File, dataURIPrefix) {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file must be a base64 data URI (e.g. data:image/png;base64,...)"}
	}
	markerIdx := strings.Index(req.File, base64Marker)
	if markerIdx == -1 {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file must be a base64 data URI (e.g. data:image/png;base64,...)"}
	}
	rawBase64 := req.File[markerIdx+len(base64Marker):]

	// Early size guard: decoded size ≈ 3/4 of base64 length. Reject before allocating.
	if len(rawBase64)*3/4 > maxAttachmentBytes {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file exceeds maximum allowed size of 10 MB"}
	}

	decoded, err := base64.StdEncoding.DecodeString(rawBase64)
	if err != nil {
		// try URL-safe variant
		decoded, err = base64.URLEncoding.DecodeString(rawBase64)
		if err != nil {
			return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file contains invalid base64 data"}
		}
	}
	if len(decoded) > maxAttachmentBytes {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file exceeds maximum allowed size of 10 MB"}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CreateAttachmentResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snCreateAttachmentPayload{
		ReferenceID:   uuidToSysid(req.CaseID),
		ReferenceType: "case",
		Name:          req.Name,
		Type:          req.Type,
		File:          rawBase64,
		Description:   req.Description,
	}

	raw, err := s.client.Post(ctx, "/attachments", token, payload)
	if err != nil {
		return domain.CreateAttachmentResponse{}, err
	}

	var snResp snCreateAttachmentResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateAttachmentResponse{}, fmt.Errorf("sn create attachment: parse response: %w", err)
	}

	createdOn, err := time.Parse(snCreatedOnLayout, snResp.Attachment.CreatedOn)
	if err != nil {
		return domain.CreateAttachmentResponse{}, fmt.Errorf("sn create attachment: parse createdOn %q: %w", snResp.Attachment.CreatedOn, err)
	}

	return domain.CreateAttachmentResponse{
		Message: snResp.Message,
		Attachment: domain.AttachmentDetail{
			ID:          sysidToUUID(snResp.Attachment.ID),
			SizeBytes:   snResp.Attachment.SizeBytes,
			CreatedOn:   createdOn,
			CreatedBy:   snResp.Attachment.CreatedBy,
			DownloadURL: snResp.Attachment.DownloadURL,
		},
	}, nil
}

type snSearchAttachmentsPayload struct {
	ReferenceID   string             `json:"referenceId"`
	ReferenceType string             `json:"referenceType"`
	Pagination    snProjectPagination `json:"pagination"`
}

type snAttachment struct {
	ID          string  `json:"id"`
	ReferenceID string  `json:"referenceId"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	SizeBytes   int     `json:"sizeBytes"`
	Description *string `json:"description"`
	CreatedBy   string  `json:"createdBy"`
	CreatedOn   string  `json:"createdOn"`
	DownloadURL *string `json:"downloadUrl"`
	PreviewURL  *string `json:"previewUrl"`
}

type snSearchAttachmentsResponse struct {
	Attachments  []snAttachment `json:"attachments"`
	TotalRecords int            `json:"totalRecords"`
	Offset       int            `json:"offset"`
	Limit        int            `json:"limit"`
}

func (s *snCaseService) SearchCaseAttachments(ctx context.Context, req domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchAttachmentsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snSearchAttachmentsPayload{
		ReferenceID:   uuidToSysid(req.CaseID),
		ReferenceType: "case",
		Pagination:    snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/attachments/search", token, payload)
	if err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}

	var snResp snSearchAttachmentsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchAttachmentsResponse{}, fmt.Errorf("sn search attachments: parse response: %w", err)
	}

	attachments := make([]domain.Attachment, 0, len(snResp.Attachments))
	for _, a := range snResp.Attachments {
		createdOn, err := time.Parse(snCreatedOnLayout, a.CreatedOn)
		if err != nil {
			return domain.SearchAttachmentsResponse{}, fmt.Errorf("sn search attachments: parse createdOn %q: %w", a.CreatedOn, err)
		}
		attachments = append(attachments, domain.Attachment{
			ID:          sysidToUUID(a.ID),
			CaseID:      sysidToUUID(a.ReferenceID),
			Name:        a.Name,
			Type:        a.Type,
			SizeBytes:   a.SizeBytes,
			Description: a.Description,
			CreatedBy:   a.CreatedBy,
			CreatedOn:   createdOn,
			DownloadURL: a.DownloadURL,
			PreviewURL:  a.PreviewURL,
		})
	}

	total := snResp.TotalRecords
	return domain.SearchAttachmentsResponse{
		Attachments: attachments,
		Total:       total,
		Limit:       req.Pagination.Limit,
		Offset:      req.Pagination.Offset,
		HasMore:     req.Pagination.Offset+len(attachments) < total,
	}, nil
}

func (s *snCaseService) GetCaseAttachmentContent(ctx context.Context, _, attachmentID string) ([]byte, string, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return nil, "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	resp, err := s.client.GetBinary(ctx, "/attachments/"+uuidToSysid(attachmentID)+"/content", token)
	if err != nil {
		return nil, "", err
	}

	return resp.Body, resp.ContentType, nil
}

// SearchCases implements CaseService by calling the Choreo POST /cases/search endpoint.
func (s *snCaseService) SearchCases(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchCasesResponse{}, err
	}

	if req.Filters.ClosedEndDate != nil && req.Filters.ClosedStartDate != nil &&
		req.Filters.ClosedEndDate.Before(*req.Filters.ClosedStartDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "closedEndDate must not be before closedStartDate"}
	}
	if req.Filters.EndCreatedDate != nil && req.Filters.StartCreatedDate != nil &&
		req.Filters.EndCreatedDate.Before(*req.Filters.StartCreatedDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "endCreatedDate must not be before startCreatedDate"}
	}
	if req.Filters.EndUpdatedDate != nil && req.Filters.StartUpdatedDate != nil &&
		req.Filters.EndUpdatedDate.Before(*req.Filters.StartUpdatedDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "endUpdatedDate must not be before startUpdatedDate"}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchCasesResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	var snSortBy *snCaseSort
	if req.SortBy.Field != "" {
		snField, ok := snSortFieldMap[req.SortBy.Field]
		if !ok {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "sortBy.field " + string(req.SortBy.Field) + " is not supported by ServiceNow"}
		}
		order := string(req.SortBy.Order)
		if order == "" {
			order = "desc"
		}
		snSortBy = &snCaseSort{Field: snField, Order: order}
	}

	payload := snCaseSearchPayload{
		Filters: snCaseFilters{
			CaseTypes:          []string{"default_case"},
			SearchQuery:        req.Filters.SearchQuery,
			ProjectIDs:         uuidsToSysids(req.Filters.ProjectIDs),
			DeploymentIDs:      uuidsToSysids(req.Filters.DeploymentIDs),
			DeployedProductIDs: uuidsToSysids(req.Filters.DeployedProductIDs),
			StateKeys:          domainStatesToSNIDs(req.Filters.StateKeys),
			SeverityKeys:       domainPrioritiesToSNIDs(req.Filters.PriorityKeys),
			IssueTypeKeys:      domainIssueTypesToSNIDs(req.Filters.IssueTypeKeys),
			ClosedStartDate:    formatSNDate(req.Filters.ClosedStartDate),
			ClosedEndDate:      formatSNDate(req.Filters.ClosedEndDate),
			StartCreatedDate:   formatSNDate(req.Filters.StartCreatedDate),
			EndCreatedDate:     formatSNDate(req.Filters.EndCreatedDate),
			StartUpdatedDate:   formatSNDate(req.Filters.StartUpdatedDate),
			EndUpdatedDate:     formatSNDate(req.Filters.EndUpdatedDate),
			CreatedBy:          req.Filters.CreatedBy,
			CreatedByMe:        req.Filters.CreatedByMe,
		},
		SortBy:     snSortBy,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/cases/search", token, payload)
	if err != nil {
		return domain.SearchCasesResponse{}, err
	}

	var snResp snCasesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCasesResponse{}, fmt.Errorf("sn cases: parse response: %w", err)
	}

	views := make([]domain.SearchCaseView, 0, len(snResp.Cases))
	for _, c := range snResp.Cases {
		createdOn, err := time.Parse(snCreatedOnLayout, c.CreatedOn)
		if err != nil {
			return domain.SearchCasesResponse{}, fmt.Errorf("sn cases: parse createdOn %q: %w", c.CreatedOn, err)
		}

		updatedOn := createdOn
		if c.UpdatedOn != nil && *c.UpdatedOn != "" {
			updatedOn, err = time.Parse(snCreatedOnLayout, *c.UpdatedOn)
			if err != nil {
				return domain.SearchCasesResponse{}, fmt.Errorf("sn cases: parse updatedOn %q: %w", *c.UpdatedOn, err)
			}
		}

		state, err := snCaseStateLabelToEnum(c.State)
		if err != nil {
			return domain.SearchCasesResponse{}, fmt.Errorf("sn cases: case %q: %w", c.ID, err)
		}

		views = append(views, domain.SearchCaseView{
			ID:                     sysidToUUID(c.ID),
			Number:                 c.Number,
			InternalID:             c.InternalID,
			Subject:                c.Title,
			Description:            c.Description,
			Priority:               snSeverityToPriority(c.Severity),
			IssueType:              snIssueTypeToEnum(c.IssueType),
			State:                  state,
			WorkState:              snWorkStateLabelToEnum(c.WorkState),
			CreatedOn:              createdOn,
			UpdatedOn:              updatedOn,
			CreatedBy:              domain.UserIDEmailRef{Email: c.CreatedBy},
			ProjectDetails:         domain.EntityRef{ID: sysidToUUID(c.Project.ID), Name: c.Project.Name},
			DeploymentDetails:      domain.EntityRef{ID: sysidToUUID(c.Deployment.ID), Name: c.Deployment.Name},
			DeployedProductDetails: domain.DeployedProductRef{ID: sysidToUUID(c.DeployedProduct.ID), DisplayName: strings.TrimSpace(c.DeployedProduct.Name + " " + c.DeployedProduct.Version)},
		})
		if c.AssignedEngineer != nil {
			v := &views[len(views)-1]
			v.AssignedEngineer = &domain.AssignedEngineerRef{ID: sysidToUUID(c.AssignedEngineer.ID), Name: c.AssignedEngineer.Name, Email: c.AssignedEngineer.Email}
		}
	}

	total := snResp.TotalRecords
	return domain.SearchCasesResponse{
		Cases:   views,
		Total:   total,
		Limit:   req.Pagination.Limit,
		Offset:  req.Pagination.Offset,
		HasMore: req.Pagination.Offset+len(views) < total,
	}, nil
}

// snCaseStateMap maps SN state labels (lowercased) to domain CaseState enums.
var snCaseStateMap = map[string]domain.CaseState{
	"open":              domain.CaseStateOpen,
	"work in progress":  domain.CaseStateWorkInProgress,
	"waiting on wso2":   domain.CaseStateWaitingOnWSO2,
	"awaiting info":     domain.CaseStateAwaitingInfo,
	"reopened":          domain.CaseStateWaitingOnWSO2,
	"solution proposed": domain.CaseStateSolutionProposed,
	"closed":            domain.CaseStateClosed,
}

func snCaseStateLabelToEnum(state *snCaseState) (domain.CaseState, error) {
	if state == nil {
		return domain.CaseStateOpen, nil
	}
	if v, ok := snCaseStateMap[strings.ToLower(state.Label)]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown case state %q from ServiceNow", state.Label)
}

// snSeverityLabel extracts the priority word from SN severity labels like
// "Low (P4)", "2 - High", "3 - Moderate" → "low", "high", "medium".
var snSeverityLabelMap = map[string]domain.CasePriority{
	"catastrophic": domain.CasePriorityCatastrophic,
	"critical":     domain.CasePriorityCritical,
	"high":         domain.CasePriorityHigh,
	"moderate":     domain.CasePriorityMedium,
	"medium":       domain.CasePriorityMedium,
	"low":          domain.CasePriorityLow,
}

func snSeverityToPriority(severity *snCaseLabel) domain.CasePriority {
	if severity == nil {
		return ""
	}
	// Labels arrive as e.g. "Low (P4)" or "2 - High"; scan words for a known priority.
	for _, word := range strings.Fields(severity.Label) {
		if p, ok := snSeverityLabelMap[strings.ToLower(strings.Trim(word, "(),"))]; ok {
			return p
		}
	}
	return ""
}

func snIssueTypeToEnum(issueType *snCaseIssueType) domain.CaseIssueType {
	if issueType == nil {
		return ""
	}
	// SN sends issueType.name as the human label e.g. "Error", "Total Outage".
	it := domain.CaseIssueType(strings.ToLower(strings.ReplaceAll(issueType.Label, " ", "_")))
	if validCaseIssueType[it] {
		return it
	}
	return ""
}

func snWorkStateLabelToEnum(ws *snCaseLabel) *domain.CaseWorkState {
	if ws == nil {
		return nil
	}
	v := domain.CaseWorkState(strings.ToLower(ws.Label))
	switch v {
	case domain.CaseWorkStateOngoing, domain.CaseWorkStatePaused:
		return &v
	default:
		return nil
	}
}

// SearchServiceRequests implements CaseService by calling the Choreo POST /cases/search
// endpoint with caseTypes filtered to ["service_request"].
func (s *snCaseService) SearchServiceRequests(ctx context.Context, req domain.SearchServiceRequestsRequest) (domain.SearchServiceRequestsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchServiceRequestsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchServiceRequestsResponse{}, err
	}

	if req.Filters.ClosedEndDate != nil && req.Filters.ClosedStartDate != nil &&
		req.Filters.ClosedEndDate.Before(*req.Filters.ClosedStartDate) {
		return domain.SearchServiceRequestsResponse{}, &apierror.ValidationError{Msg: "closedEndDate must not be before closedStartDate"}
	}
	if req.Filters.EndCreatedDate != nil && req.Filters.StartCreatedDate != nil &&
		req.Filters.EndCreatedDate.Before(*req.Filters.StartCreatedDate) {
		return domain.SearchServiceRequestsResponse{}, &apierror.ValidationError{Msg: "endCreatedDate must not be before startCreatedDate"}
	}
	if req.Filters.EndUpdatedDate != nil && req.Filters.StartUpdatedDate != nil &&
		req.Filters.EndUpdatedDate.Before(*req.Filters.StartUpdatedDate) {
		return domain.SearchServiceRequestsResponse{}, &apierror.ValidationError{Msg: "endUpdatedDate must not be before startUpdatedDate"}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchServiceRequestsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	var snSortBy *snCaseSort
	if req.SortBy.Field != "" {
		snField, ok := snSortFieldMap[req.SortBy.Field]
		if !ok {
			return domain.SearchServiceRequestsResponse{}, &apierror.ValidationError{Msg: "sortBy.field " + string(req.SortBy.Field) + " is not supported by ServiceNow"}
		}
		order := string(req.SortBy.Order)
		if order == "" {
			order = "desc"
		}
		snSortBy = &snCaseSort{Field: snField, Order: order}
	}

	payload := snCaseSearchPayload{
		Filters: snCaseFilters{
			CaseTypes:        []string{"service_request"},
			SearchQuery:      req.Filters.SearchQuery,
			ProjectIDs:       uuidsToSysids(req.Filters.ProjectIDs),
			DeploymentIDs:    uuidsToSysids(req.Filters.DeploymentIDs),
			StateKeys:        req.Filters.StateKeys,
			ClosedStartDate:  formatSNDate(req.Filters.ClosedStartDate),
			ClosedEndDate:    formatSNDate(req.Filters.ClosedEndDate),
			StartCreatedDate: formatSNDate(req.Filters.StartCreatedDate),
			EndCreatedDate:   formatSNDate(req.Filters.EndCreatedDate),
			StartUpdatedDate: formatSNDate(req.Filters.StartUpdatedDate),
			EndUpdatedDate:   formatSNDate(req.Filters.EndUpdatedDate),
			CreatedBy:        req.Filters.CreatedBy,
			CreatedByMe:      req.Filters.CreatedByMe,
		},
		SortBy:     snSortBy,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/cases/search", token, payload)
	if err != nil {
		return domain.SearchServiceRequestsResponse{}, err
	}

	var snResp snServiceRequestsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchServiceRequestsResponse{}, fmt.Errorf("sn service requests: parse response: %w", err)
	}

	views := make([]domain.ServiceRequestView, 0, len(snResp.Cases))
	for _, c := range snResp.Cases {
		view := domain.ServiceRequestView{
			ID:          sysidToUUID(c.ID),
			InternalID:  c.InternalID,
			Number:      c.Number,
			CreatedOn:   c.CreatedOn,
			CreatedBy:   c.CreatedBy,
			Title:       c.Title,
			Description: c.Description,
			State:       snServiceRequestStateLabel(c.State),
			Project:     domain.EntityRef{ID: sysidToUUID(c.Project.ID), Name: c.Project.Name},
			Deployment:  domain.EntityRef{ID: sysidToUUID(c.Deployment.ID), Name: c.Deployment.Name},
			DeployedProduct: domain.EntityRef{
				ID:   sysidToUUID(c.DeployedProduct.ID),
				Name: strings.TrimSpace(c.DeployedProduct.Name + " " + c.DeployedProduct.Version),
			},
		}
		if c.Product != nil {
			ref := domain.EntityRef{ID: sysidToUUID(c.Product.ID), Name: c.Product.Name}
			view.Product = &ref
		}
		if c.Catalog != nil {
			ref := domain.EntityRef{ID: sysidToUUID(c.Catalog.ID), Name: c.Catalog.Name}
			view.Catalog = &ref
		}
		if c.CatalogItem != nil {
			ref := domain.EntityRef{ID: sysidToUUID(c.CatalogItem.ID), Name: c.CatalogItem.Name}
			view.CatalogItem = &ref
		}
		if c.AssignedTeam != nil {
			ref := domain.EntityRef{ID: sysidToUUID(c.AssignedTeam.ID), Name: c.AssignedTeam.Name}
			view.AssignedTeam = &ref
		}
		if c.AssignedEngineer != nil {
			view.AssignedEngineer = &domain.AssignedEngineerRef{ID: sysidToUUID(c.AssignedEngineer.ID), Name: c.AssignedEngineer.Name, Email: c.AssignedEngineer.Email}
		}
		if c.ParentCase != nil {
			ref := domain.EntityRef{ID: sysidToUUID(c.ParentCase.ID), Name: c.ParentCase.Number}
			view.ParentCase = &ref
		}
		if c.RelatedCase != nil {
			ref := domain.EntityRef{ID: sysidToUUID(c.RelatedCase.ID), Name: c.RelatedCase.Number}
			view.RelatedCase = &ref
		}
		if c.WorkState != nil {
			view.WorkState = &domain.ServiceRequestWorkStateRef{ID: c.WorkState.ID, Label: c.WorkState.Label}
		}
		views = append(views, view)
	}

	return domain.SearchServiceRequestsResponse{
		ServiceRequests: views,
		TotalRecords: snResp.TotalRecords,
		Offset:       req.Pagination.Offset,
		Limit:        req.Pagination.Limit,
	}, nil
}

func snServiceRequestStateLabel(state *snCaseState) string {
	if state == nil {
		return ""
	}
	return state.Label
}
