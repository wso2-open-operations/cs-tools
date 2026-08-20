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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snFlexibleID decodes a JSON value that may arrive as either a string or a
// number into a string — matches upstream choice-list items, whose id field
// is typed int|string on the Choreo side.
type snFlexibleID string

func (f *snFlexibleID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = snFlexibleID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*f = snFlexibleID(n.String())
	return nil
}

// snChoiceOption mirrors the Choreo ChoiceListItem shape.
type snChoiceOption struct {
	ID    snFlexibleID `json:"id"`
	Label string       `json:"label"`
	Count *int         `json:"count,omitempty"`
}

func (i snChoiceOption) toDomain() domain.ChoiceListItem {
	return domain.ChoiceListItem{ID: string(i.ID), Label: i.Label, Count: i.Count}
}

func toDomainChoiceListItems(items []snChoiceOption) []domain.ChoiceListItem {
	out := make([]domain.ChoiceListItem, 0, len(items))
	for _, i := range items {
		out = append(out, i.toDomain())
	}
	return out
}

// snReferenceTableItem mirrors the Choreo ReferenceTableItem shape — its id
// is always a plain sysid, unlike snChoiceOption's.
type snReferenceTableItem struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Number     *string `json:"number,omitempty"`
	InternalID *string `json:"internalId,omitempty"`
	Count      *int    `json:"count,omitempty"`
	// Abbreviation is the short product/reference name ServiceNow carries
	// alongside the display name — e.g. "APIM" for "API Manager". Ballerina
	// declares it on this record too (types.bal ReferenceTableItem); it was the
	// only field of that record this service did not decode.
	Abbreviation *string `json:"abbreviation,omitempty"`
}

func (i snReferenceTableItem) toDomain() domain.ReferenceTableItem {
	return domain.ReferenceTableItem{
		ID:           sysidToUUID(i.ID),
		Name:         i.Name,
		Number:       i.Number,
		InternalID:   i.InternalID,
		Count:        i.Count,
		Abbreviation: i.Abbreviation,
	}
}

func toDomainReferenceTableItems(items []snReferenceTableItem) []domain.ReferenceTableItem {
	out := make([]domain.ReferenceTableItem, 0, len(items))
	for _, i := range items {
		out = append(out, i.toDomain())
	}
	return out
}

// snProjectFeatures mirrors the Choreo ProjectFeatures shape.
type snProjectFeatures struct {
	ProjectType                    snReferenceTableItem `json:"projectType"`
	AcceptedSeverityValues         []snChoiceOption     `json:"acceptedSeverityValues"`
	HasServiceRequestWriteAccess   bool                 `json:"hasServiceRequestWriteAccess"`
	HasServiceRequestReadAccess    bool                 `json:"hasServiceRequestReadAccess"`
	HasSraWriteAccess              bool                 `json:"hasSraWriteAccess"`
	HasSraReadAccess               bool                 `json:"hasSraReadAccess"`
	HasChangeRequestReadAccess     bool                 `json:"hasChangeRequestReadAccess"`
	HasEngagementsReadAccess       bool                 `json:"hasEngagementsReadAccess"`
	HasUpdatesReadAccess           bool                 `json:"hasUpdatesReadAccess"`
	HasTimeLogsReadAccess          bool                 `json:"hasTimeLogsReadAccess"`
	HasDeploymentWriteAccess       bool                 `json:"hasDeploymentWriteAccess"`
	HasDeploymentReadAccess        bool                 `json:"hasDeploymentReadAccess"`
	HasComponentAnalysisReadAccess bool                 `json:"hasComponentAnalysisReadAccess"`
	HasUsageMetricsReadAccess      bool                 `json:"hasUsageMetricsReadAccess"`
	DefaultCaseProductCategories   []string             `json:"defaultCaseProductCategories,omitempty"`
	SrProductCategories            []string             `json:"srProductCategories,omitempty"`
}

func (f snProjectFeatures) toDomain() domain.ProjectFeatures {
	return domain.ProjectFeatures{
		ProjectType:                    f.ProjectType.toDomain(),
		AcceptedSeverityValues:         toDomainChoiceListItems(f.AcceptedSeverityValues),
		HasServiceRequestWriteAccess:   f.HasServiceRequestWriteAccess,
		HasServiceRequestReadAccess:    f.HasServiceRequestReadAccess,
		HasSraWriteAccess:              f.HasSraWriteAccess,
		HasSraReadAccess:               f.HasSraReadAccess,
		HasChangeRequestReadAccess:     f.HasChangeRequestReadAccess,
		HasEngagementsReadAccess:       f.HasEngagementsReadAccess,
		HasUpdatesReadAccess:           f.HasUpdatesReadAccess,
		HasTimeLogsReadAccess:          f.HasTimeLogsReadAccess,
		HasDeploymentWriteAccess:       f.HasDeploymentWriteAccess,
		HasDeploymentReadAccess:        f.HasDeploymentReadAccess,
		HasComponentAnalysisReadAccess: f.HasComponentAnalysisReadAccess,
		HasUsageMetricsReadAccess:      f.HasUsageMetricsReadAccess,
		DefaultCaseProductCategories:   f.DefaultCaseProductCategories,
		SrProductCategories:            f.SrProductCategories,
	}
}

// snProjectMetadataResponse mirrors the Choreo GET /projects/{id}/metadata response.
type snProjectMetadataResponse struct {
	CaseStates                  []snChoiceOption       `json:"caseStates"`
	CallRequestStates           []snChoiceOption       `json:"callRequestStates"`
	ChangeRequestStates         []snChoiceOption       `json:"changeRequestStates"`
	ConversationStates          []snChoiceOption       `json:"conversationStates"`
	TimeCardStates              []snChoiceOption       `json:"timeCardStates"`
	ChangeRequestImpacts        []snChoiceOption       `json:"changeRequestImpacts"`
	Severities                  []snChoiceOption       `json:"severities"`
	SeverityBasedAllocationTime map[string]int         `json:"severityBasedAllocationTime"`
	IssueTypes                  []snChoiceOption       `json:"issueTypes"`
	DeploymentTypes             []snChoiceOption       `json:"deploymentTypes"`
	CaseTypes                   []snReferenceTableItem `json:"caseTypes"`
	EngagementTypes             []snChoiceOption       `json:"engagementTypes"`
	EngagementPaymentTypes      []snChoiceOption       `json:"engagementPaymentTypes"`
	Features                    snProjectFeatures      `json:"features"`
}

// snProjectStatsOutstandingCount mirrors the outstandingCount object embedded
// in the Choreo GET /projects/{id}/stats response.
type snProjectStatsOutstandingCount struct {
	CaseCount           int `json:"caseCount"`
	ServiceRequestCount int `json:"serviceRequestCount"`
	EngagementCount     int `json:"engagementCount"`
	SraCount            int `json:"sraCount"`
	ChangeRequestCount  int `json:"changeRequestCount"`
	AnnouncementCount   int `json:"announcementCount"`
}

// snProjectStatsResponse mirrors the Choreo GET /projects/{id}/stats response.
type snProjectStatsResponse struct {
	TotalHours           float64                        `json:"totalHours"`
	BillableHours        float64                        `json:"billableHours"`
	SLAStatus            string                         `json:"slaStatus"`
	DeploymentCount      int                            `json:"deploymentCount"`
	DeployedProductCount int                            `json:"deployedProductCount"`
	InstanceCount        int                            `json:"instanceCount"`
	OutstandingCount     snProjectStatsOutstandingCount `json:"outstandingCount"`
}

// snResolvedCountBreakdown mirrors the resolvedCount object shared by the
// Choreo case-stats and change-request-stats responses.
type snResolvedCountBreakdown struct {
	Total          int `json:"total"`
	CurrentMonth   int `json:"currentMonth"`
	PastThirtyDays int `json:"pastThirtyDays"`
}

func (r snResolvedCountBreakdown) toDomain() domain.ResolvedCountBreakdown {
	return domain.ResolvedCountBreakdown{Total: r.Total, CurrentMonth: r.CurrentMonth, PastThirtyDays: r.PastThirtyDays}
}

// snProjectCaseStatsChangeRate mirrors the changeRate object embedded in the
// Choreo GET /projects/{id}/cases/stats response.
type snProjectCaseStatsChangeRate struct {
	ResolvedEngagements float64 `json:"resolvedEngagements"`
	AverageResponseTime float64 `json:"averageResponseTime"`
}

// snCasesTrend mirrors one entry of the casesTrend array embedded in the
// Choreo GET /projects/{id}/cases/stats response.
type snCasesTrend struct {
	Period     string           `json:"period"`
	Severities []snChoiceOption `json:"severities"`
}

// snProjectCaseStatsResponse mirrors the Choreo GET /projects/{id}/cases/stats response.
type snProjectCaseStatsResponse struct {
	TotalCount                     int                          `json:"totalCount"`
	ActiveCount                    int                          `json:"activeCount"`
	OutstandingCount               int                          `json:"outstandingCount"`
	ActionRequiredCount            int                          `json:"actionRequiredCount"`
	AverageResponseTime            float64                      `json:"averageResponseTime"`
	ResolvedCount                  snResolvedCountBreakdown     `json:"resolvedCount"`
	ChangeRate                     snProjectCaseStatsChangeRate `json:"changeRate"`
	StateCount                     []snChoiceOption             `json:"stateCount"`
	SeverityCount                  []snChoiceOption             `json:"severityCount"`
	OutstandingSeverityCount       []snChoiceOption             `json:"outstandingSeverityCount"`
	EngagementTypeCount            []snChoiceOption             `json:"engagementTypeCount"`
	OutstandingEngagementTypeCount []snChoiceOption             `json:"outstandingEngagementTypeCount"`
	CaseTypeCount                  []snReferenceTableItem       `json:"caseTypeCount"`
	CasesTrend                     []snCasesTrend               `json:"casesTrend"`
}

// snProjectConversationStatsResponse mirrors the Choreo
// GET /projects/{id}/conversations/stats response.
type snProjectConversationStatsResponse struct {
	TotalCount  int              `json:"totalCount"`
	ActiveCount int              `json:"activeCount"`
	StateCount  []snChoiceOption `json:"stateCount"`
}

// snProjectDeploymentStatsResponse mirrors the Choreo
// GET /projects/{id}/deployments/stats response.
type snProjectDeploymentStatsResponse struct {
	TotalCount       int     `json:"totalCount"`
	LastDeploymentOn *string `json:"lastDeploymentOn"`
}

// snProjectTimeCardStatsResponse mirrors the Choreo
// GET /projects/{id}/time-cards/stats response.
type snProjectTimeCardStatsResponse struct {
	TotalHours       float64 `json:"totalHours"`
	BillableHours    float64 `json:"billableHours"`
	NonBillableHours float64 `json:"nonBillableHours"`
}

// snProjectChangeRequestStatsResponse mirrors the Choreo
// GET /projects/{id}/change-requests/stats response.
type snProjectChangeRequestStatsResponse struct {
	TotalCount          int                      `json:"totalCount"`
	ActiveCount         int                      `json:"activeCount"`
	OutstandingCount    int                      `json:"outstandingCount"`
	ActionRequiredCount int                      `json:"actionRequiredCount"`
	StateCount          []snChoiceOption         `json:"stateCount"`
	ResolvedCount       snResolvedCountBreakdown `json:"resolvedCount"`
}

type snProjectStatsService struct {
	client *integrationservice.Client
}

// NewServiceNowProjectStatsService constructs a ProjectStatsService backed by the Choreo API.
func NewServiceNowProjectStatsService(client *integrationservice.Client) ProjectStatsService {
	return &snProjectStatsService{client: client}
}

// GetProjectMetadata implements ProjectStatsService.
func (s *snProjectStatsService) GetProjectMetadata(ctx context.Context, projectID string) (domain.ProjectMetadataResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectMetadataResponse{}, err
	}
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/projects/"+uuidToSysid(projectID)+"/metadata", token)
	if err != nil {
		return domain.ProjectMetadataResponse{}, err
	}

	var snResp snProjectMetadataResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.ProjectMetadataResponse{}, fmt.Errorf("sn project metadata: parse response: %w", err)
	}

	return domain.ProjectMetadataResponse{
		CaseStates:                  toDomainChoiceListItems(snResp.CaseStates),
		CallRequestStates:           toDomainChoiceListItems(snResp.CallRequestStates),
		ChangeRequestStates:         toDomainChoiceListItems(snResp.ChangeRequestStates),
		ConversationStates:          toDomainChoiceListItems(snResp.ConversationStates),
		TimeCardStates:              toDomainChoiceListItems(snResp.TimeCardStates),
		ChangeRequestImpacts:        toDomainChoiceListItems(snResp.ChangeRequestImpacts),
		Severities:                  toDomainChoiceListItems(snResp.Severities),
		SeverityBasedAllocationTime: snResp.SeverityBasedAllocationTime,
		IssueTypes:                  toDomainChoiceListItems(snResp.IssueTypes),
		DeploymentTypes:             toDomainChoiceListItems(snResp.DeploymentTypes),
		CaseTypes:                   toDomainReferenceTableItems(snResp.CaseTypes),
		EngagementTypes:             toDomainChoiceListItems(snResp.EngagementTypes),
		EngagementPaymentTypes:      toDomainChoiceListItems(snResp.EngagementPaymentTypes),
		Features:                    snResp.Features.toDomain(),
	}, nil
}

// GetProjectStats implements ProjectStatsService.
func (s *snProjectStatsService) GetProjectStats(ctx context.Context, projectID string) (domain.ProjectStatsResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectStatsResponse{}, err
	}
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/projects/"+uuidToSysid(projectID)+"/stats", token)
	if err != nil {
		return domain.ProjectStatsResponse{}, err
	}

	var snResp snProjectStatsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.ProjectStatsResponse{}, fmt.Errorf("sn project stats: parse response: %w", err)
	}

	return domain.ProjectStatsResponse{
		TotalHours:           snResp.TotalHours,
		BillableHours:        snResp.BillableHours,
		SLAStatus:            snResp.SLAStatus,
		DeploymentCount:      snResp.DeploymentCount,
		DeployedProductCount: snResp.DeployedProductCount,
		InstanceCount:        snResp.InstanceCount,
		OutstandingCount: domain.ProjectStatsOutstandingCount{
			CaseCount:           snResp.OutstandingCount.CaseCount,
			ServiceRequestCount: snResp.OutstandingCount.ServiceRequestCount,
			EngagementCount:     snResp.OutstandingCount.EngagementCount,
			SraCount:            snResp.OutstandingCount.SraCount,
			ChangeRequestCount:  snResp.OutstandingCount.ChangeRequestCount,
			AnnouncementCount:   snResp.OutstandingCount.AnnouncementCount,
		},
	}, nil
}

// GetProjectCaseStats implements ProjectStatsService.
func (s *snProjectStatsService) GetProjectCaseStats(ctx context.Context, projectID string, req domain.ProjectCaseStatsRequest) (domain.ProjectCaseStatsResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectCaseStatsResponse{}, err
	}
	token := middleware.UserIDTokenFromContext(ctx)

	q := url.Values{}
	if len(req.CaseTypes) > 0 {
		// CaseTypes is this service's own domain vocabulary (case,
		// service_request, security_report_analysis, announcement,
		// engagement — see validCaseType), not UUIDs: there is no
		// per-project-stats "case type" entity with its own id, unlike
		// projectId above. normalizeCaseType resolves aliases like
		// "default_case" (see caseTypeAliases) before validation, matching
		// case search's own handling of the same field. Translate to
		// ServiceNow's caseTypes wire values the same way case search does
		// (domainTypeKeysToSN), so e.g. "case" is forwarded as "default_case".
		normalized := make([]string, len(req.CaseTypes))
		for i, t := range req.CaseTypes {
			normalized[i] = normalizeCaseType(t)
			if !validCaseType[normalized[i]] {
				return domain.ProjectCaseStatsResponse{}, &apierror.ValidationError{Msg: "caseTypes contains invalid value: " + t}
			}
		}
		for _, ct := range domainTypeKeysToSN(normalized) {
			q.Add("caseTypes", ct)
		}
	}
	if req.CreatedBy != "" {
		q.Set("createdBy", req.CreatedBy)
	}
	path := "/projects/" + uuidToSysid(projectID) + "/cases/stats"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	raw, err := s.client.Get(ctx, path, token)
	if err != nil {
		return domain.ProjectCaseStatsResponse{}, err
	}

	var snResp snProjectCaseStatsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.ProjectCaseStatsResponse{}, fmt.Errorf("sn project case stats: parse response: %w", err)
	}

	trend := make([]domain.CasesTrend, 0, len(snResp.CasesTrend))
	for _, t := range snResp.CasesTrend {
		trend = append(trend, domain.CasesTrend{Period: t.Period, Severities: toDomainChoiceListItems(t.Severities)})
	}

	return domain.ProjectCaseStatsResponse{
		TotalCount:          snResp.TotalCount,
		ActiveCount:         snResp.ActiveCount,
		OutstandingCount:    snResp.OutstandingCount,
		ActionRequiredCount: snResp.ActionRequiredCount,
		AverageResponseTime: snResp.AverageResponseTime,
		ResolvedCount:       snResp.ResolvedCount.toDomain(),
		ChangeRate: domain.ProjectCaseStatsChangeRate{
			ResolvedEngagements: snResp.ChangeRate.ResolvedEngagements,
			AverageResponseTime: snResp.ChangeRate.AverageResponseTime,
		},
		StateCount:                     toDomainChoiceListItems(snResp.StateCount),
		SeverityCount:                  toDomainChoiceListItems(snResp.SeverityCount),
		OutstandingSeverityCount:       toDomainChoiceListItems(snResp.OutstandingSeverityCount),
		EngagementTypeCount:            toDomainChoiceListItems(snResp.EngagementTypeCount),
		OutstandingEngagementTypeCount: toDomainChoiceListItems(snResp.OutstandingEngagementTypeCount),
		CaseTypeCount:                  toDomainReferenceTableItems(snResp.CaseTypeCount),
		CasesTrend:                     trend,
	}, nil
}

// GetProjectConversationStats implements ProjectStatsService.
func (s *snProjectStatsService) GetProjectConversationStats(ctx context.Context, projectID, createdBy string) (domain.ProjectConversationStatsResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectConversationStatsResponse{}, err
	}
	token := middleware.UserIDTokenFromContext(ctx)

	q := url.Values{}
	if createdBy != "" {
		q.Set("createdBy", createdBy)
	}
	path := "/projects/" + uuidToSysid(projectID) + "/conversations/stats"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	raw, err := s.client.Get(ctx, path, token)
	if err != nil {
		return domain.ProjectConversationStatsResponse{}, err
	}

	var snResp snProjectConversationStatsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.ProjectConversationStatsResponse{}, fmt.Errorf("sn project conversation stats: parse response: %w", err)
	}

	return domain.ProjectConversationStatsResponse{
		TotalCount:  snResp.TotalCount,
		ActiveCount: snResp.ActiveCount,
		StateCount:  toDomainChoiceListItems(snResp.StateCount),
	}, nil
}

// GetProjectDeploymentStats implements ProjectStatsService.
func (s *snProjectStatsService) GetProjectDeploymentStats(ctx context.Context, projectID string) (domain.ProjectDeploymentStatsResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectDeploymentStatsResponse{}, err
	}
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/projects/"+uuidToSysid(projectID)+"/deployments/stats", token)
	if err != nil {
		return domain.ProjectDeploymentStatsResponse{}, err
	}

	var snResp snProjectDeploymentStatsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.ProjectDeploymentStatsResponse{}, fmt.Errorf("sn project deployment stats: parse response: %w", err)
	}

	return domain.ProjectDeploymentStatsResponse{
		TotalCount:       snResp.TotalCount,
		LastDeploymentOn: snResp.LastDeploymentOn,
	}, nil
}

// GetProjectTimeCardStats implements ProjectStatsService.
func (s *snProjectStatsService) GetProjectTimeCardStats(ctx context.Context, projectID, startDate, endDate string) (domain.ProjectTimeCardStatsResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectTimeCardStatsResponse{}, err
	}
	token := middleware.UserIDTokenFromContext(ctx)

	q := url.Values{}
	if startDate != "" {
		q.Set("startDate", startDate)
	}
	if endDate != "" {
		q.Set("endDate", endDate)
	}
	path := "/projects/" + uuidToSysid(projectID) + "/time-cards/stats"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	raw, err := s.client.Get(ctx, path, token)
	if err != nil {
		return domain.ProjectTimeCardStatsResponse{}, err
	}

	var snResp snProjectTimeCardStatsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.ProjectTimeCardStatsResponse{}, fmt.Errorf("sn project time card stats: parse response: %w", err)
	}

	return domain.ProjectTimeCardStatsResponse{
		TotalHours:       snResp.TotalHours,
		BillableHours:    snResp.BillableHours,
		NonBillableHours: snResp.NonBillableHours,
	}, nil
}

// GetProjectChangeRequestStats implements ProjectStatsService.
func (s *snProjectStatsService) GetProjectChangeRequestStats(ctx context.Context, projectID string) (domain.ProjectChangeRequestStatsResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectChangeRequestStatsResponse{}, err
	}
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/projects/"+uuidToSysid(projectID)+"/change-requests/stats", token)
	if err != nil {
		return domain.ProjectChangeRequestStatsResponse{}, err
	}

	var snResp snProjectChangeRequestStatsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.ProjectChangeRequestStatsResponse{}, fmt.Errorf("sn project change request stats: parse response: %w", err)
	}

	return domain.ProjectChangeRequestStatsResponse{
		TotalCount:          snResp.TotalCount,
		ActiveCount:         snResp.ActiveCount,
		OutstandingCount:    snResp.OutstandingCount,
		ActionRequiredCount: snResp.ActionRequiredCount,
		StateCount:          toDomainChoiceListItems(snResp.StateCount),
		ResolvedCount:       snResp.ResolvedCount.toDomain(),
	}, nil
}
