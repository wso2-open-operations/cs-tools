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
	"strconv"
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

type snCase struct {
	ID                    string                      `json:"id"`
	InternalID            string                      `json:"internalId"`
	Number                string                      `json:"number"`
	Title                 string                      `json:"title"`
	Description           string                      `json:"description"`
	CreatedOn             string                      `json:"createdOn"`
	UpdatedOn             *string                     `json:"updatedOn"`
	CreatedBy             string                      `json:"createdBy"`
	Project               snCaseEntityRef             `json:"project"`
	Deployment            snCaseEntityRef             `json:"deployment"`
	DeployedProduct       snCaseDeployedProduct       `json:"deployedProduct"`
	Product               *snCaseEntityRef            `json:"product"`
	State                 *snCaseState                `json:"state"`
	WorkState             *snCaseLabel                `json:"workState"`
	Severity              *snCaseLabel                `json:"severity"`
	IssueType             *snCaseIssueType            `json:"issueType"`
	EngagementType        *snCaseLabel                `json:"engagementType"`
	CaseType              *snCaseEntityRef            `json:"caseType"`
	Catalog               *snCaseEntityRef            `json:"catalog"`
	CatalogItem           *snCaseEntityRef            `json:"catalogItem"`
	AssignedTeam          *snCaseEntityRef            `json:"assignedTeam"`
	Conversation          *snCaseEntityRef            `json:"conversation"`
	AssignedEngineer      *snAssignedEngineerRef      `json:"assignedEngineer"`
	ParentCase            *snCaseRef                  `json:"parentCase"`
	RelatedCase           *snCaseRef                  `json:"relatedCase"`
	Account               *snCaseAccount              `json:"account"`
	LinkedServiceRequests []snLinkedServiceRequestRef `json:"linkedServiceRequests"`
	ResolutionCode        *struct {
		ID    json.Number `json:"id"`
		Label string      `json:"label"`
	} `json:"resolutionCode"`
	Cause *struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"cause"`
	ResolutionNotes *string `json:"resolutionNotes"`
	ResolvedOn      *string `json:"resolvedOn"`
	// FixEta is the single customer-facing fix-commitment date/time
	// (u_fix_eta_shared). Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main): no Ballerina field currently
	// surfaces this — the Choreo GET /cases/{id} response does not include a
	// "fixEta" key today, so this always unmarshals to nil. Ask: add a
	// "fixEta" (glide_date_time) field to servicenow:CaseResponse.
	FixEta *string `json:"fixEta"`
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

type snLinkedServiceRequestRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Name   string `json:"name"`
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

// snCaseTypeMap maps domain case type strings to the ServiceNow caseType values.
var snCaseTypeMap = map[string]string{
	"case":                     "default_case",
	"service_request":          "service_request",
	"security_report_analysis": "security_report_analysis",
	"announcement":             "announcement",
	"engagement":               "engagement",
}

// snCaseTypeSysidMap maps ServiceNow caseType sysids to domain case type values.
var snCaseTypeSysidMap = map[string]string{
	"8d4b87bd1b18f010cb6898aebd4bcb59": "case",
	"0d5b8fbd1b18f010cb6898aebd4bcba5": "case",
	"5aeff1201b74c210264c997a234bcb54": "service_request",
	"ab36479047ccf510a0a29cd3846d43ee": "security_report_analysis",
	"3b8b43311b58f010cb6898aebd4bcb8f": "announcement",
	"8f8fc2c41b0bd550d64e64a2604bcb38": "engagement",
}

// snCaseTypeToDomain converts a SN caseType entity ref to the domain type string.
// Maps by sysid; defaults to "case" when caseType is null or the sysid is unrecognised.
func snCaseTypeToDomain(ct *snCaseEntityRef) *string {
	domainType := "case"
	if ct != nil {
		if mapped, ok := snCaseTypeSysidMap[ct.ID]; ok {
			domainType = mapped
		}
	}
	return &domainType
}

func domainTypeKeysToSN(typeKeys []string) []string {
	result := make([]string, 0, len(typeKeys))
	for _, t := range typeKeys {
		if sn, ok := snCaseTypeMap[t]; ok {
			result = append(result, sn)
		}
	}
	return result
}

// snSortFieldMap maps domain CaseSortField values to SN field names.
var snSortFieldMap = map[domain.CaseSortField]string{
	domain.CaseSortFieldCreatedOn: "createdOn",
	domain.CaseSortFieldUpdatedOn: "updatedOn",
	domain.CaseSortFieldSeverity:  "severity",
	domain.CaseSortFieldState:     "state",
}

type snCaseFilters struct {
	CaseTypes          []string `json:"caseTypes"`
	SearchQuery        string   `json:"searchQuery,omitempty"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	DeployedProductIDs []string `json:"deployedProductIds,omitempty"`
	StateKeys          []int    `json:"stateKeys,omitempty"`
	SeverityKeys       []int    `json:"severityKeys,omitempty"`
	IssueTypeKeys      []int    `json:"issueTypeKeys,omitempty"`
	EngagementTypeKeys []int    `json:"engagementTypeKeys,omitempty"`
	ClosedStartDate    string   `json:"closedStartDate,omitempty"`
	ClosedEndDate      string   `json:"closedEndDate,omitempty"`
	StartCreatedDate   string   `json:"startCreatedDate,omitempty"`
	EndCreatedDate     string   `json:"endCreatedDate,omitempty"`
	StartUpdatedDate   string   `json:"startUpdatedDate,omitempty"`
	EndUpdatedDate     string   `json:"endUpdatedDate,omitempty"`
	CreatedBy          []string `json:"createdBy,omitempty"`
	CreatedByMe        bool     `json:"createdByMe,omitempty"`
	WorkStateKeys      []int    `json:"workStateKeys,omitempty"`
	AssignedUserIDs    []string `json:"assignedUserIds,omitempty"`
	ProductNames       []string `json:"productNames,omitempty"`
	// Tags: Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main), see domain.SearchCasesFilters.Tags doc comment.
	// Forwarded to Choreo so filtering starts working the moment Ballerina adds
	// support, but the current POST /cases/search contract ignores this field.
	Tags []string `json:"tags,omitempty"`
}

// snStateIDMap maps domain CaseState enums to SN numeric state IDs.
var snStateIDMap = map[domain.CaseState]int{
	domain.CaseStateOpen:             1,
	domain.CaseStateWorkInProgress:   10,
	domain.CaseStateAwaitingInfo:     18,
	domain.CaseStateWaitingOnWSO2:    1003,
	domain.CaseStateReopened:         1006,
	domain.CaseStateSolutionProposed: 6,
	domain.CaseStateClosed:           3,
}

// snSeverityIDMap maps domain CaseSeverity enums to SN numeric severity IDs.
var snSeverityIDMap = map[domain.CaseSeverity]int{
	domain.CaseSeverityCatastrophic: 14,
	domain.CaseSeverityCritical:     10,
	domain.CaseSeverityHigh:         11,
	domain.CaseSeverityMedium:       12,
	domain.CaseSeverityLow:          13,
}

// snIssueTypeIDMap maps domain CaseIssueType enums to SN numeric issue-type IDs.
var snIssueTypeIDMap = map[domain.CaseIssueType]int{
	domain.CaseIssueTypeTotalOutage:            1,
	domain.CaseIssueTypePartialOutage:          2,
	domain.CaseIssueTypePerformanceDegradation: 3,
	domain.CaseIssueTypeQuestion:               4,
	domain.CaseIssueTypeSecurityOrCompliance:   5,
	domain.CaseIssueTypeError:                  6,
}

// snEngagementTypeIDMap maps domain EngagementType enums to SN numeric engagement-type IDs.
var snEngagementTypeIDMap = map[domain.EngagementType]int{
	domain.EngagementTypeMigration:             1,
	domain.EngagementTypeConsultancy:           2,
	domain.EngagementTypeNewFeatureImprovement: 3,
	domain.EngagementTypeFollowUp:              4,
	domain.EngagementTypeOnboarding:            5,
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

func domainSeveritiesToSNIDs(severities []domain.CaseSeverity) []int {
	ids := make([]int, 0, len(severities))
	for _, s := range severities {
		if id, ok := snSeverityIDMap[s]; ok {
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

func domainWorkStatesToSNIDs(workStates []domain.CaseWorkState) []int {
	ids := make([]int, 0, len(workStates))
	for _, ws := range workStates {
		if id, ok := snWorkStateIDMap[ws]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func domainEngagementTypesToSNIDs(engTypes []domain.EngagementType) []int {
	ids := make([]int, 0, len(engTypes))
	for _, et := range engTypes {
		if id, ok := snEngagementTypeIDMap[et]; ok {
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
	Type              string             `json:"type"`
	ProjectID         string             `json:"projectId"`
	DeploymentID      string             `json:"deploymentId"`
	DeployedProductID string             `json:"deployedProductId"`
	Title             string             `json:"title,omitempty"`
	Description       string             `json:"description,omitempty"`
	SeverityKey       int                `json:"severityKey,omitempty"`
	IssueTypeKey      int                `json:"issueTypeKey,omitempty"`
	CatalogID         string             `json:"catalogId,omitempty"`
	CatalogItemID     string             `json:"catalogItemId,omitempty"`
	Variables         []snCaseVariable   `json:"variables,omitempty"`
	RelatedCaseID     string             `json:"relatedCaseId,omitempty"`
	ConversationID    string             `json:"conversationId,omitempty"`
	WatchList         []string           `json:"watchList,omitempty"`
	Attachments       []snCaseAttachment `json:"attachments,omitempty"`
}

type snCaseVariable struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type snCaseAttachment struct {
	Name string `json:"name"`
	File string `json:"file"`
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

	snType, ok := snCaseTypeMap[req.Type]
	if !ok {
		return domain.CreateCaseResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + req.Type}
	}

	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return domain.CreateCaseResponse{}, err
	}
	if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
		return domain.CreateCaseResponse{}, err
	}
	if err := validateUUIDs("deployedProductId", []string{req.DeployedProductID}); err != nil {
		return domain.CreateCaseResponse{}, err
	}

	payload := snCreateCasePayload{
		Type:              snType,
		ProjectID:         uuidToSysid(req.ProjectID),
		DeploymentID:      uuidToSysid(req.DeploymentID),
		DeployedProductID: uuidToSysid(req.DeployedProductID),
	}

	switch req.Type {
	case "case":
		payload.Title = req.Subject
		payload.Description = req.Description
		payload.SeverityKey = snSeverityIDMap[req.Severity]
		payload.IssueTypeKey = snIssueTypeID[req.IssueType]
	case "service_request":
		if err := validateUUIDs("catalogId", []string{req.CatalogID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
		if err := validateUUIDs("catalogItemId", []string{req.CatalogItemID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
		payload.CatalogID = uuidToSysid(req.CatalogID)
		payload.CatalogItemID = uuidToSysid(req.CatalogItemID)
		if len(req.Variables) > 0 {
			vars := make([]snCaseVariable, 0, len(req.Variables))
			for i, v := range req.Variables {
				if err := validateUUIDs(fmt.Sprintf("variables[%d].id", i), []string{v.ID}); err != nil {
					return domain.CreateCaseResponse{}, err
				}
				vars = append(vars, snCaseVariable{ID: uuidToSysid(v.ID), Value: v.Value})
			}
			payload.Variables = vars
		}
	case "security_report_analysis":
		payload.Title = req.Subject
		payload.Description = req.Description
		if len(req.Attachments) > 0 {
			atts := make([]snCaseAttachment, 0, len(req.Attachments))
			for _, a := range req.Attachments {
				atts = append(atts, snCaseAttachment{Name: a.Name, File: a.File})
			}
			payload.Attachments = atts
		}
	}

	if len(req.WatchList) > 0 {
		payload.WatchList = req.WatchList
	}
	if req.RelatedCaseID != "" {
		if err := validateUUIDs("relatedCaseId", []string{req.RelatedCaseID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
		payload.RelatedCaseID = uuidToSysid(req.RelatedCaseID)
	}
	if req.ConversationID != "" {
		if err := validateUUIDs("conversationId", []string{req.ConversationID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
		payload.ConversationID = uuidToSysid(req.ConversationID)
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
		ID:             sysidToUUID(c.ID),
		Number:         c.Number,
		InternalID:     c.InternalID,
		Subject:        c.Title,
		Description:    c.Description,
		Severity:       snSeverityToSeverity(c.Severity),
		IssueType:      snIssueTypeToEnum(c.IssueType),
		State:          state,
		WorkState:      snWorkStateLabelToEnum(c.WorkState),
		Type:           snCaseTypeToDomain(c.CaseType),
		EngagementType: snLabelStr(c.EngagementType),
		CreatedOn:      createdOn,
		UpdatedOn:      updatedOn,
		CreatedByDetails: domain.UserRef{
			Email: c.CreatedBy,
		},
		ProjectDetails: domain.EntityRef{ID: sysidToUUID(c.Project.ID), Name: c.Project.Name},
	}

	if depID := sysidToUUID(c.Deployment.ID); depID != "" {
		cv.DeploymentDetails = &domain.EntityRef{ID: depID, Name: c.Deployment.Name}
	}
	if dpID := sysidToUUID(c.DeployedProduct.ID); dpID != "" {
		cv.DeployedProductDetails = &domain.DeployedProductRef{
			ID:          dpID,
			DisplayName: strings.TrimSpace(c.DeployedProduct.Name + " " + c.DeployedProduct.Version),
		}
	}
	if c.Product != nil {
		if id := sysidToUUID(c.Product.ID); id != "" {
			cv.ProductDetails = &domain.EntityRef{ID: id, Name: c.Product.Name}
		}
	}
	if c.Catalog != nil {
		if id := sysidToUUID(c.Catalog.ID); id != "" {
			cv.Catalog = &domain.EntityRef{ID: id, Name: c.Catalog.Name}
		}
	}
	if c.CatalogItem != nil {
		if id := sysidToUUID(c.CatalogItem.ID); id != "" {
			cv.CatalogItem = &domain.EntityRef{ID: id, Name: c.CatalogItem.Name}
		}
	}
	if c.AssignedTeam != nil {
		if id := sysidToUUID(c.AssignedTeam.ID); id != "" {
			cv.AssignedTeam = &domain.EntityRef{ID: id, Name: c.AssignedTeam.Name}
		}
	}
	if c.Conversation != nil {
		if id := sysidToUUID(c.Conversation.ID); id != "" {
			cv.Conversation = &domain.EntityRef{ID: id, Name: c.Conversation.Name}
		}
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
	if len(c.LinkedServiceRequests) > 0 {
		lsr := make([]domain.LinkedServiceRequestRef, 0, len(c.LinkedServiceRequests))
		for _, r := range c.LinkedServiceRequests {
			lsr = append(lsr, domain.LinkedServiceRequestRef{ID: sysidToUUID(r.ID), Number: r.Number, Name: r.Name})
		}
		cv.LinkedServiceRequests = lsr
	}
	if c.ResolutionCode != nil {
		if rc, ok := snResolutionCodeByID[c.ResolutionCode.ID.String()]; ok {
			cv.ResolutionCode = &rc
		}
	}
	if c.Cause != nil {
		if cause, ok := snCauseByID[c.Cause.ID]; ok {
			cv.Cause = &cause
		}
	}
	cv.ResolutionNotes = c.ResolutionNotes
	if c.ResolvedOn != nil && *c.ResolvedOn != "" {
		resolvedOn, err := time.Parse(snCreatedOnLayout, *c.ResolvedOn)
		if err != nil {
			return domain.CaseView{}, fmt.Errorf("sn get case: parse resolvedOn %q: %w", *c.ResolvedOn, err)
		}
		cv.ResolvedOn = &resolvedOn
	}
	// FixEta passes through once Ballerina's matching field (see snCase.FixEta doc
	// comment) lands; until then this is always nil.
	if c.FixEta != nil && *c.FixEta != "" {
		fixEta, err := time.Parse(snCreatedOnLayout, *c.FixEta)
		if err != nil {
			return domain.CaseView{}, fmt.Errorf("sn get case: parse fixEta %q: %w", *c.FixEta, err)
		}
		cv.FixEta = &fixEta
	}
	// Tags are not populated: Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main), see CaseView.Tags doc comment.
	// cv.Tags is left nil.

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
	ReferenceID   string              `json:"referenceId"`
	ReferenceType string              `json:"referenceType"`
	Filters       *snCommentFilters   `json:"filters,omitempty"`
	Pagination    snProjectPagination `json:"pagination"`
}

type snComment struct {
	ID                 string `json:"id"`
	ReferenceID        string `json:"referenceId"`
	Content            string `json:"content"`
	Type               string `json:"type"`
	CreatedOn          string `json:"createdOn"`
	CreatedBy          string `json:"createdBy"`
	CreatedByFirstName string `json:"createdByFirstName"`
	CreatedByLastName  string `json:"createdByLastName"`
	CreatedByFullName  string `json:"createdByFullName"`
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

// snCommentTypeToCommentType maps the SN API type string back to the domain enum.
var snCommentTypeToCommentType = map[string]domain.CommentType{
	"comments":   domain.CommentTypeComment,
	"work_notes": domain.CommentTypeWorkNote,
	"activity":   domain.CommentTypeActivity,
}

func (s *snCaseService) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

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
	StateKey       *int     `json:"stateKey,omitempty"`
	SeverityKey    *int     `json:"severityKey,omitempty"`
	WorkStateKey   *int     `json:"workStateKey,omitempty"`
	WatchList      []string `json:"watchList,omitempty"`
	AssigneeEmail  *string  `json:"assigneeEmail,omitempty"`
	ResolutionCode *int     `json:"resolutionCode,omitempty"`
	Cause          *string  `json:"cause,omitempty"`
	CloseNotes     *string  `json:"closeNotes,omitempty"`
	ParentID       *string  `json:"parentId,omitempty"`
	// FixEta writes the customer-facing fix-commitment date/time (u_fix_eta_shared).
	// Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main): no Ballerina write field exists yet for this — ask:
	// add a "fixEta" field to servicenow:CaseUpdatePayload backed by u_fix_eta_shared.
	FixEta *string `json:"fixEta,omitempty"`
}

// snResolutionStates are the state keys that allow resolution fields.
var snResolutionStates = map[int]bool{
	3: true, // closed
	6: true, // solution_proposed
}

// snResolutionCodeKey maps domain CaseResolutionCode enums to the ServiceNow integer keys.
var snResolutionCodeKey = map[domain.CaseResolutionCode]int{
	domain.CaseResolutionCodeSolvedFixedBySupportGuidanceProvided: 1,
	domain.CaseResolutionCodeSolvedFixedByClosingRelatedIncident:  16,
	domain.CaseResolutionCodeSolvedFixedByClosingRelatedRDTicket:  17,
	domain.CaseResolutionCodeSolvedWorkaroundProvided:             3,
	domain.CaseResolutionCodeSolvedByCustomer:                     4,
	domain.CaseResolutionCodeConsideredForRoadmap:                 18,
	domain.CaseResolutionCodeInconclusiveOutOfScope:               5,
	domain.CaseResolutionCodeInconclusiveCannotReproduce:          6,
	domain.CaseResolutionCodeInconclusiveNoWorkaround:             7,
	domain.CaseResolutionCodeDuplicateIssue:                       8,
	domain.CaseResolutionCodeVoidedCanceled:                       9,
	domain.CaseResolutionCodeOnHold:                               19,
	domain.CaseResolutionCodeConsideredForRoadmapAlt:              20,
	domain.CaseResolutionCodeSolvedFixedTheIssue:                  21,
	domain.CaseResolutionCodeSolvedWorkaroundProvidedAlt:          22,
	domain.CaseResolutionCodeSolvedByContributor:                  27,
	domain.CaseResolutionCodeSolvedByNovera:                       51,
	domain.CaseResolutionCodeAbruptlyClosedDueToNonResponsiveness: 52,
}

// snResolutionCodeByID maps ServiceNow resolution code id strings to domain CaseResolutionCode enums.
var snResolutionCodeByID = func() map[string]domain.CaseResolutionCode {
	m := make(map[string]domain.CaseResolutionCode, len(snResolutionCodeKey))
	for k, v := range snResolutionCodeKey {
		m[fmt.Sprintf("%d", v)] = k
	}
	return m
}()

// snCauseKey maps domain CaseCause enums to the ServiceNow integer choice
// values for sn_customerservice_case.cause on the PROD tenant (wso2),
// verified via sys_choice. Unlike resolution codes' scattered keys, prod's
// cause choice list happens to number sequentially 1-25 in picklist order.
//
// The DEV tenant (wso2sndev) configures this same field with the label text
// as its stored value instead of an integer — a real cross-tenant
// inconsistency, not a bug in this mapping. This map targets prod, the only
// tenant live customer traffic reaches; DEV-tenant testing of the cause
// field will not round-trip correctly against this mapping.
var snCauseKey = map[domain.CaseCause]int{
	domain.CaseCauseSolutionArchitecture:          1,
	domain.CaseCauseDeploymentArchitecture:        2,
	domain.CaseCauseUserErrorConfiguration:        3,
	domain.CaseCauseUserErrorProductConcept:       4,
	domain.CaseCauseUserErrorRuntime:              5,
	domain.CaseCauseUserErrorRecommendation:       6,
	domain.CaseCauseCustomizationLimitation:       7,
	domain.CaseCauseCustomizationBug:              8,
	domain.CaseCauseDocumentationGap:              9,
	domain.CaseCauseDocumentationError:            10,
	domain.CaseCauseProductLimitation:             11,
	domain.CaseCauseProductBug:                    12,
	domain.CaseCauseProductRegression:             13,
	domain.CaseCauseProductMigration:              14,
	domain.CaseCauseInfrastructureDatabase:        15,
	domain.CaseCauseInfrastructureOS:              16,
	domain.CaseCauseInfrastructureNetwork:         17,
	domain.CaseCauseInfrastructureJDK:             18,
	domain.CaseCauseInfrastructureLDAP:            19,
	domain.CaseCauseInfrastructureLoadBalancer:    20,
	domain.CaseCauseInfrastructureIAAS:            21,
	domain.CaseCauseInfrastructureExternalProduct: 22,
	domain.CaseCauseInfrastructureProxy:           23,
	domain.CaseCauseInfrastructureOther:           24,
	domain.CaseCauseUnknown:                       25,
}

// snCauseByID maps ServiceNow cause choice-value strings (the SN "cause"
// field's id, e.g. "12") to domain CaseCause enums.
var snCauseByID = func() map[string]domain.CaseCause {
	m := make(map[string]domain.CaseCause, len(snCauseKey))
	for k, v := range snCauseKey {
		m[strconv.Itoa(v)] = k
	}
	return m
}()

// snWorkStateIDMap maps domain CaseWorkState enums to SN numeric work state IDs.
var snWorkStateIDMap = map[domain.CaseWorkState]int{
	domain.CaseWorkStateOngoing: 1,
	domain.CaseWorkStatePaused:  2,
}

type snUpdateCaseResponse struct {
	Message string `json:"message"`
	Case    struct {
		ID        string       `json:"id"`
		UpdatedOn string       `json:"updatedOn"`
		UpdatedBy string       `json:"updatedBy"`
		State     *snCaseState `json:"state"`
		Severity  *snCaseLabel `json:"severity"`
		WorkState *snCaseLabel `json:"workState"`
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
		ResolutionCode *struct {
			ID    json.Number `json:"id"`
			Label string      `json:"label"`
		} `json:"resolutionCode"`
		Cause *struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"cause"`
		CloseNotes *string    `json:"closeNotes"`
		ResolvedOn *string    `json:"resolvedOn"`
		ParentCase *snCaseRef `json:"parentCase"`
		// FixEta: Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main), see snUpdateCasePayload.FixEta doc comment.
		FixEta *string `json:"fixEta"`
	} `json:"case"`
}

func (s *snCaseService) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	hasResolutionFields := req.ResolutionCode != nil || req.Cause != nil || req.CloseNotes != nil

	fieldCount := 0
	if req.State != nil {
		fieldCount++
	}
	if req.Severity != nil {
		fieldCount++
	}
	if req.WorkState != nil {
		fieldCount++
	}
	if len(req.WatchList) > 0 {
		fieldCount++
	}
	if req.AssigneeEmail != nil {
		fieldCount++
	}
	if req.ParentID != nil {
		fieldCount++
	}
	if req.FixEta != nil {
		fieldCount++
	}
	const updateCaseFieldList = "state, severity, workState, watchList, assigneeEmail, parentId, or fixEta"
	if fieldCount == 0 {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "at least one of " + updateCaseFieldList + " must be provided"}
	}
	if fieldCount > 1 {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "only one of " + updateCaseFieldList + " may be provided per request"}
	}
	if hasResolutionFields && req.State == nil {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "resolutionCode, cause, and closeNotes are only allowed when state is also provided"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

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
		if hasResolutionFields && !snResolutionStates[id] {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "resolutionCode, cause, and closeNotes are only allowed when state is closed or solution_proposed"}
		}
		if req.ResolutionCode != nil {
			key, ok := snResolutionCodeKey[*req.ResolutionCode]
			if !ok {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "resolutionCode contains invalid value: " + string(*req.ResolutionCode)}
			}
			payload.ResolutionCode = &key
		}
		if req.Cause != nil {
			key, ok := snCauseKey[*req.Cause]
			if !ok {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "cause contains invalid value: " + string(*req.Cause)}
			}
			val := strconv.Itoa(key)
			payload.Cause = &val
		}
		payload.CloseNotes = req.CloseNotes
	}
	if req.Severity != nil {
		if !validCaseSeverity[*req.Severity] {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "severity contains invalid value: " + string(*req.Severity)}
		}
		id, ok := snSeverityIDMap[*req.Severity]
		if !ok {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "severity " + string(*req.Severity) + " is not supported by ServiceNow"}
		}
		payload.SeverityKey = &id
	}
	if req.WorkState != nil {
		if !validCaseWorkState[*req.WorkState] {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "workState contains invalid value: " + string(*req.WorkState)}
		}
		id, ok := snWorkStateIDMap[*req.WorkState]
		if !ok {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "workState " + string(*req.WorkState) + " is not supported by ServiceNow"}
		}
		payload.WorkStateKey = &id
	}
	if len(req.WatchList) > 0 {
		payload.WatchList = req.WatchList
	}
	if req.AssigneeEmail != nil {
		payload.AssigneeEmail = req.AssigneeEmail
	}
	if req.ParentID != nil {
		if err := validateUUIDs("parentId", []string{*req.ParentID}); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		sysid := uuidToSysid(*req.ParentID)
		payload.ParentID = &sysid
	}
	if req.FixEta != nil {
		fixEta := formatSNDate(req.FixEta)
		payload.FixEta = &fixEta
	}

	// Close-gate: reject closing a case that still has an open, customer-visible task.
	// This is the authoritative server-side check (item 1's close-gating requirement) --
	// it only needs the existing read path (task search + task detail), so it is fully
	// wired even though task writes themselves are still Ballerina-blocked.
	if payload.StateKey != nil && *payload.StateKey == snStateIDMap[domain.CaseStateClosed] {
		blocked, err := s.hasOpenVisibleTasks(ctx, uuidToSysid(req.ID), token)
		if err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		if blocked {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "case cannot be closed while it has an open task visible to the customer"}
		}
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
		resp.Case.Severity = snSeverityToSeverity(snResp.Case.Severity)
	}
	resp.Case.WorkState = snWorkStateLabelToEnum(snResp.Case.WorkState)
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
	if snResp.Case.ResolutionCode != nil {
		if rc, ok := snResolutionCodeByID[snResp.Case.ResolutionCode.ID.String()]; ok {
			resp.Case.ResolutionCode = &rc
		}
	}
	if snResp.Case.Cause != nil {
		if c, ok := snCauseByID[snResp.Case.Cause.ID]; ok {
			resp.Case.Cause = &c
		}
	}
	resp.Case.CloseNotes = snResp.Case.CloseNotes
	if snResp.Case.ParentCase != nil {
		resp.Case.ParentCase = &domain.CaseNumberRef{ID: sysidToUUID(snResp.Case.ParentCase.ID), Number: snResp.Case.ParentCase.Number}
	}
	if snResp.Case.ResolvedOn != nil {
		resolvedOn, err := time.Parse(snCreatedOnLayout, *snResp.Case.ResolvedOn)
		if err != nil {
			return domain.UpdateCaseResponse{}, fmt.Errorf("sn update case: parse resolvedAt %q: %w", *snResp.Case.ResolvedOn, err)
		}
		resp.Case.ResolvedOn = &resolvedOn
	}
	// FixEta: Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main), see snUpdateCasePayload.FixEta doc comment;
	// snResp.Case.FixEta is always nil until Ballerina echoes it back.
	if snResp.Case.FixEta != nil && *snResp.Case.FixEta != "" {
		fixEta, err := time.Parse(snCreatedOnLayout, *snResp.Case.FixEta)
		if err != nil {
			return domain.UpdateCaseResponse{}, fmt.Errorf("sn update case: parse fixEta %q: %w", *snResp.Case.FixEta, err)
		}
		resp.Case.FixEta = &fixEta
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

	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.CreateAttachmentResponse{}, err
	}
	if _, ok := validReferenceTypes[req.ReferenceType]; !ok {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "referenceType is invalid: " + string(req.ReferenceType)}
	}

	payload := snCreateAttachmentPayload{
		ReferenceID:   uuidToSysid(req.ReferenceID),
		ReferenceType: string(req.ReferenceType),
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

var validReferenceTypes = map[domain.ReferenceType]struct{}{
	domain.ReferenceTypeCase:          {},
	domain.ReferenceTypeConversation:  {},
	domain.ReferenceTypeChangeRequest: {},
	domain.ReferenceTypeDeployment:    {},
	domain.ReferenceTypeIncident:      {},
}

type snSearchAttachmentsPayload struct {
	ReferenceID   string              `json:"referenceId"`
	ReferenceType string              `json:"referenceType"`
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

	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}
	if _, ok := validReferenceTypes[req.ReferenceType]; !ok {
		return domain.SearchAttachmentsResponse{}, &apierror.ValidationError{Msg: "referenceType is invalid: " + string(req.ReferenceType)}
	}

	payload := snSearchAttachmentsPayload{
		ReferenceID:   uuidToSysid(req.ReferenceID),
		ReferenceType: string(req.ReferenceType),
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
			ID:            sysidToUUID(a.ID),
			ReferenceID:   sysidToUUID(a.ReferenceID),
			ReferenceType: req.ReferenceType,
			Name:          a.Name,
			Type:          a.Type,
			SizeBytes:     a.SizeBytes,
			Description:   a.Description,
			CreatedBy:     a.CreatedBy,
			CreatedOn:     createdOn,
			DownloadURL:   a.DownloadURL,
			PreviewURL:    a.PreviewURL,
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

type snSearchActivitiesPayload struct {
	Pagination          snProjectPagination `json:"pagination"`
	IncludeFieldChanges *bool               `json:"includeFieldChanges,omitempty"`
}

type snFieldChange struct {
	Field         string `json:"field"`
	FieldLabel    string `json:"fieldLabel"`
	PreviousValue string `json:"previousValue"`
	NewValue      string `json:"newValue"`
}

type snActivity struct {
	ID                 string          `json:"id"`
	Type               string          `json:"type"`
	Content            string          `json:"content"`
	CreatedOn          string          `json:"createdOn"`
	CreatedBy          string          `json:"createdBy"`
	CreatedByFirstName string          `json:"createdByFirstName"`
	CreatedByLastName  string          `json:"createdByLastName"`
	CreatedByFullName  string          `json:"createdByFullName"`
	CommentType        string          `json:"commentType"`
	FileName           string          `json:"fileName"`
	ContentType        string          `json:"contentType"`
	SizeBytes          int             `json:"sizeBytes"`
	DownloadURL        string          `json:"downloadUrl"`
	Changes            []snFieldChange `json:"changes"`
}

type snSearchActivitiesResponse struct {
	Activity     []snActivity `json:"activity"`
	Offset       int          `json:"offset"`
	Limit        int          `json:"limit"`
	TotalRecords int          `json:"totalRecords"`
}

func (s *snCaseService) SearchCaseActivities(ctx context.Context, req domain.SearchCaseActivitiesRequest) (domain.SearchCaseActivitiesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCaseActivitiesResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{req.CaseID}); err != nil {
		return domain.SearchCaseActivitiesResponse{}, err
	}

	payload := snSearchActivitiesPayload{
		Pagination:          snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
		IncludeFieldChanges: req.IncludeFieldChanges,
	}

	raw, err := s.client.Post(ctx, "/cases/"+uuidToSysid(req.CaseID)+"/activities/search", token, payload)
	if err != nil {
		return domain.SearchCaseActivitiesResponse{}, err
	}

	var snResp snSearchActivitiesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCaseActivitiesResponse{}, fmt.Errorf("sn search activities: parse response: %w", err)
	}

	activities := make([]domain.CaseActivity, 0, len(snResp.Activity))
	for _, a := range snResp.Activity {
		createdOn, err := time.Parse(snCreatedOnLayout, a.CreatedOn)
		if err != nil {
			return domain.SearchCaseActivitiesResponse{}, fmt.Errorf("sn search activities: parse createdOn %q: %w", a.CreatedOn, err)
		}
		activity := domain.CaseActivity{
			ID:                 sysidToUUID(a.ID),
			Type:               domain.ActivityType(a.Type),
			Content:            a.Content,
			CreatedOn:          createdOn,
			CreatedBy:          a.CreatedBy,
			CreatedByFirstName: a.CreatedByFirstName,
			CreatedByLastName:  a.CreatedByLastName,
			CreatedByFullName:  a.CreatedByFullName,
		}
		switch domain.ActivityType(a.Type) {
		case domain.ActivityTypeComment:
			var ct domain.CommentType
			switch a.CommentType {
			case "comments", "comment":
				ct = domain.CommentTypeComment
			case "work_notes", "work_note":
				ct = domain.CommentTypeWorkNote
			case "activity":
				ct = domain.CommentTypeActivity
			default:
				ct = domain.CommentTypeComment
			}
			activity.CommentType = &ct
		case domain.ActivityTypeAttachment:
			activity.FileName = a.FileName
			activity.ContentType = a.ContentType
			activity.SizeBytes = a.SizeBytes
			activity.DownloadURL = a.DownloadURL
		case domain.ActivityTypeFieldChange:
			changes := make([]domain.FieldChange, 0, len(a.Changes))
			for _, ch := range a.Changes {
				changes = append(changes, domain.FieldChange{
					Field:         ch.Field,
					FieldLabel:    ch.FieldLabel,
					PreviousValue: ch.PreviousValue,
					NewValue:      ch.NewValue,
				})
			}
			activity.Changes = changes
		}
		activities = append(activities, activity)
	}

	total := snResp.TotalRecords
	return domain.SearchCaseActivitiesResponse{
		Activity: activities,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(activities) < total,
	}, nil
}

func (s *snCaseService) GetCaseAttachmentContent(ctx context.Context, attachmentID string) ([]byte, string, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	resp, err := s.client.GetBinary(ctx, "/attachments/"+uuidToSysid(attachmentID)+"/content", token)
	if err != nil {
		return nil, "", err
	}

	return resp.Body, resp.ContentType, nil
}

func (s *snCaseService) DeleteCaseAttachment(ctx context.Context, req domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Delete(ctx, "/attachments/"+uuidToSysid(req.AttachmentID), token)
	if err != nil {
		return domain.DeleteAttachmentResponse{}, err
	}

	var snResp struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.DeleteAttachmentResponse{}, fmt.Errorf("sn delete attachment: parse response: %w", err)
	}

	return domain.DeleteAttachmentResponse{Message: snResp.Message}, nil
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

	for _, ws := range req.Filters.WorkStates {
		if ws != domain.CaseWorkStateOngoing && ws != domain.CaseWorkStatePaused {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "workStates contains invalid value: " + string(ws)}
		}
	}
	if err := validateUUIDs("assignedUserIds", req.Filters.AssignedUserIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

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

	for _, t := range req.Filters.Types {
		if _, ok := snCaseTypeMap[t]; !ok {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "types contains invalid value: " + t}
		}
	}
	snCaseTypes := domainTypeKeysToSN(req.Filters.Types)
	if len(snCaseTypes) == 0 {
		snCaseTypes = []string{"default_case"}
	}

	payload := snCaseSearchPayload{
		Filters: snCaseFilters{
			CaseTypes:          snCaseTypes,
			SearchQuery:        req.Filters.SearchQuery,
			ProjectIDs:         uuidsToSysids(req.Filters.ProjectIDs),
			DeploymentIDs:      uuidsToSysids(req.Filters.DeploymentIDs),
			StateKeys:          domainStatesToSNIDs(req.Filters.States),
			SeverityKeys:       domainSeveritiesToSNIDs(req.Filters.Severities),
			IssueTypeKeys:      domainIssueTypesToSNIDs(req.Filters.IssueTypes),
			EngagementTypeKeys: domainEngagementTypesToSNIDs(req.Filters.EngagementTypes),
			ClosedStartDate:    formatSNDate(req.Filters.ClosedStartDate),
			ClosedEndDate:      formatSNDate(req.Filters.ClosedEndDate),
			StartCreatedDate:   formatSNDate(req.Filters.StartCreatedDate),
			EndCreatedDate:     formatSNDate(req.Filters.EndCreatedDate),
			StartUpdatedDate:   formatSNDate(req.Filters.StartUpdatedDate),
			EndUpdatedDate:     formatSNDate(req.Filters.EndUpdatedDate),
			CreatedBy:          req.Filters.CreatedBy,
			CreatedByMe:        req.Filters.CreatedByMe,
			WorkStateKeys:      domainWorkStatesToSNIDs(req.Filters.WorkStates),
			AssignedUserIDs:    uuidsToSysids(req.Filters.AssignedUserIDs),
			ProductNames:       req.Filters.ProductNames,
			Tags:               req.Filters.Tags,
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
		title := c.Title
		description := c.Description
		severityLabel := snSeverityLabelStr(c.Severity)
		issueTypeLabel := snIssueTypeLabelStr(c.IssueType)
		workStateLabel := snWorkStateLabelStr(c.WorkState)
		engagementTypeLabel := snLabelStr(c.EngagementType)

		stateLabel := ""
		if c.State != nil {
			stateLabel = c.State.Label
		}
		caseTypeDomain := ""
		if t := snCaseTypeToDomain(c.CaseType); t != nil {
			caseTypeDomain = *t
		}

		cv := domain.SearchCaseView{
			ID:             sysidToUUID(c.ID),
			Number:         c.Number,
			InternalID:     c.InternalID,
			CreatedOn:      c.CreatedOn,
			CreatedBy:      c.CreatedBy,
			Subject:        &title,
			Description:    &description,
			IssueType:      issueTypeLabel,
			State:          stateLabel,
			Severity:       severityLabel,
			EngagementType: engagementTypeLabel,
			WorkState:      workStateLabel,
			Type:           caseTypeDomain,
			Project:        domain.EntityRef{ID: sysidToUUID(c.Project.ID), Name: c.Project.Name},
		}
		if depID := sysidToUUID(c.Deployment.ID); depID != "" {
			cv.Deployment = &domain.EntityRef{ID: depID, Name: c.Deployment.Name}
		}
		if dpID := sysidToUUID(c.DeployedProduct.ID); dpID != "" {
			cv.DeployedProduct = &domain.EntityRef{ID: dpID, Name: strings.TrimSpace(c.DeployedProduct.Name + " " + c.DeployedProduct.Version)}
		}
		if c.Product != nil {
			if id := sysidToUUID(c.Product.ID); id != "" {
				cv.Product = &domain.EntityRef{ID: id, Name: c.Product.Name}
			}
		}
		if c.Catalog != nil {
			if id := sysidToUUID(c.Catalog.ID); id != "" {
				cv.Catalog = &domain.EntityRef{ID: id, Name: c.Catalog.Name}
			}
		}
		if c.CatalogItem != nil {
			if id := sysidToUUID(c.CatalogItem.ID); id != "" {
				cv.CatalogItem = &domain.EntityRef{ID: id, Name: c.CatalogItem.Name}
			}
		}
		if c.AssignedTeam != nil {
			if id := sysidToUUID(c.AssignedTeam.ID); id != "" {
				cv.AssignedTeam = &domain.EntityRef{ID: id, Name: c.AssignedTeam.Name}
			}
		}
		if c.Conversation != nil {
			cv.Conversation = &domain.EntityRef{ID: sysidToUUID(c.Conversation.ID), Name: c.Conversation.Name}
		}
		if c.AssignedEngineer != nil {
			cv.AssignedEngineer = &domain.AssignedEngineerRef{ID: sysidToUUID(c.AssignedEngineer.ID), Name: c.AssignedEngineer.Name, Email: c.AssignedEngineer.Email}
		}
		if c.ParentCase != nil {
			cv.ParentCase = &domain.EntityRef{ID: sysidToUUID(c.ParentCase.ID), Name: c.ParentCase.Number}
		}
		if c.RelatedCase != nil {
			cv.RelatedCase = &domain.EntityRef{ID: sysidToUUID(c.RelatedCase.ID), Name: c.RelatedCase.Number}
		}
		views = append(views, cv)
	}

	return domain.SearchCasesResponse{
		Cases:  views,
		Total:  snResp.TotalRecords,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}

// snSeverityLabelStr returns the raw SN severity label string, or nil if absent.
func snSeverityLabelStr(s *snCaseLabel) *string {
	if s == nil {
		return nil
	}
	return &s.Label
}

// snIssueTypeLabelStr returns the raw SN issue-type label string, or nil if absent.
func snIssueTypeLabelStr(it *snCaseIssueType) *string {
	if it == nil {
		return nil
	}
	return &it.Label
}

// snLabelStr returns the label of an snCaseLabel, or nil if absent.
func snLabelStr(l *snCaseLabel) *string {
	if l == nil {
		return nil
	}
	return &l.Label
}

// snWorkStateLabelStr normalizes an SN work-state label to the lowercased
// domain enum value ("ongoing"/"paused") as a string, or nil when absent or
// unrecognised. The search view carries workState as a plain string, but it
// must match the enum the detail endpoint returns (GET /cases/{id} maps via
// snWorkStateLabelToEnum) so both paths agree on casing. Returning the raw SN
// label here (e.g. "Ongoing") breaks clients that gate on the lowercased value.
func snWorkStateLabelStr(ws *snCaseLabel) *string {
	e := snWorkStateLabelToEnum(ws)
	if e == nil {
		return nil
	}
	s := string(*e)
	return &s
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
var snSeverityLabelMap = map[string]domain.CaseSeverity{
	"catastrophic": domain.CaseSeverityCatastrophic,
	"critical":     domain.CaseSeverityCritical,
	"high":         domain.CaseSeverityHigh,
	"moderate":     domain.CaseSeverityMedium,
	"medium":       domain.CaseSeverityMedium,
	"low":          domain.CaseSeverityLow,
}

func snSeverityToSeverity(severity *snCaseLabel) domain.CaseSeverity {
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

// hasOpenVisibleTasks reports whether the case identified by caseSysid has any
// open task that is visible to the customer. Used by the close-gate in
// UpdateCase (item 1's authoritative "can this case close?" check).
//
// SN's case-scoped task listing (snTask/snCaseTasksResponse, see
// sn_task_service.go) does not carry visibleToCustomer -- only the single-task
// detail response (snTaskDetail) does. So this walks the non-closed tasks
// returned by the search and fetches each one's detail to check visibility.
// Case task counts are small in practice, so a single page (limit 100) is
// fetched rather than paginating fully; if that assumption stops holding, this
// should switch to paging until exhausted.
func (s *snCaseService) hasOpenVisibleTasks(ctx context.Context, caseSysid, token string) (bool, error) {
	payload := snCaseTasksSearchPayload{Pagination: snProjectPagination{Limit: 100, Offset: 0}}

	raw, err := s.client.Post(ctx, "/cases/"+caseSysid+"/tasks/search", token, payload)
	if err != nil {
		return false, err
	}

	var tasksResp snCaseTasksResponse
	if err := json.Unmarshal(raw, &tasksResp); err != nil {
		return false, fmt.Errorf("sn update case: parse case tasks response: %w", err)
	}

	for _, t := range tasksResp.Tasks {
		if t.State != nil && *t.State == "CLOSED" {
			continue
		}

		detailRaw, err := s.client.Get(ctx, "/tasks/"+t.ID, token)
		if err != nil {
			return false, err
		}
		var detail snTaskDetail
		if err := json.Unmarshal(detailRaw, &detail); err != nil {
			return false, fmt.Errorf("sn update case: parse task detail response: %w", err)
		}
		if detail.VisibleToCustomer {
			return true, nil
		}
	}

	return false, nil
}

// snAddTagPayload is the Choreo POST /cases/{id}/tags request body.
//
// Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main): no Ballerina/Choreo endpoint exists yet for this. SN's
// tagging is the generic platform label/label_entry mechanism (table-agnostic,
// not a case column), so a new adapter is needed -- ask: add
// POST /cases/{id}/tags (body: {"label": string}) and
// DELETE /cases/{id}/tags/{tagId} to servicenow.bal, backed by the sys_label /
// label_entry tables scoped to reference_table="sn_customerservice_case".
type snAddTagPayload struct {
	Label string `json:"label"`
}

// snTag mirrors the Choreo tag shape (once it exists).
type snTag struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Color *string `json:"color"`
}

type snAddTagResponse struct {
	Message string `json:"message"`
	Tag     snTag  `json:"tag"`
}

// AddCaseTag attaches a free-text label to the case identified by caseID.
//
// Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main): see snAddTagPayload doc comment. This is implemented so
// the entity-service side is ready the moment Ballerina adds the endpoint;
// until then, calling it returns a downstream error (no such Choreo route today).
func (s *snCaseService) AddCaseTag(ctx context.Context, caseID, label string) (domain.Tag, error) {
	if err := validateUUIDs("id", []string{caseID}); err != nil {
		return domain.Tag{}, err
	}
	if strings.TrimSpace(label) == "" {
		return domain.Tag{}, &apierror.ValidationError{Msg: "label is required"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snAddTagPayload{Label: label}
	raw, err := s.client.Post(ctx, "/cases/"+uuidToSysid(caseID)+"/tags", token, payload)
	if err != nil {
		return domain.Tag{}, err
	}

	var snResp snAddTagResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.Tag{}, fmt.Errorf("sn add case tag: parse response: %w", err)
	}

	return domain.Tag{
		ID:    sysidToUUID(snResp.Tag.ID),
		Label: snResp.Tag.Label,
		Color: snResp.Tag.Color,
	}, nil
}

// RemoveCaseTag removes the tag identified by tagID from the case identified by caseID.
//
// Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main): see snAddTagPayload doc comment (no such Choreo route today).
func (s *snCaseService) RemoveCaseTag(ctx context.Context, caseID, tagID string) error {
	if err := validateUUIDs("id", []string{caseID}); err != nil {
		return err
	}
	if err := validateUUIDs("tagId", []string{tagID}); err != nil {
		return err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	_, err := s.client.Delete(ctx, "/cases/"+uuidToSysid(caseID)+"/tags/"+uuidToSysid(tagID), token)
	return err
}
