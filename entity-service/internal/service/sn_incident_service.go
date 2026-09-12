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
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// publishIncidentCreatedTimeout bounds publishIncidentCreated's publish call
// — see that function's doc comment for why this runs synchronously rather
// than detached.
const publishIncidentCreatedTimeout = 5 * time.Second

// snIncidentsResponse mirrors the Choreo POST /incidents/search response.
type snIncidentsResponse struct {
	Incidents    []snIncident `json:"incidents"`
	TotalRecords int          `json:"totalRecords"`
	Offset       int          `json:"offset"`
	Limit        int          `json:"limit"`
}

type snIncident struct {
	ID              *string              `json:"id"`
	Number          *string              `json:"number"`
	OpenedOn        *string              `json:"openedOn"`
	Subject         *string              `json:"subject"`
	Caller          *snIncidentEntityRef `json:"caller"`
	Priority        *snIncidentIntLabel  `json:"priority"`
	State           *snIncidentIntLabel  `json:"state"`
	Category        *snIncidentStrLabel  `json:"category"`
	Parent          *snIncidentEntityRef `json:"parent"`
	ParentIncident  *snIncidentEntityRef `json:"parentIncident"`
	AssignmentGroup *snIncidentEntityRef `json:"assignmentGroup"`
	AssignedTo      *snIncidentEntityRef `json:"assignedTo"`
	CreatedOn       string               `json:"createdOn"`
	CreatedBy       string               `json:"createdBy"`
	UpdatedOn       string               `json:"updatedOn"`
	UpdatedBy       string               `json:"updatedBy"`
}

type snIncidentEntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snIncidentIntLabel struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type snIncidentStrLabel struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// snIncidentSearchPayload is the Choreo POST /incidents/search request body.
type snIncidentSearchPayload struct {
	Filters    snIncidentFilters   `json:"filters,omitempty"`
	SortBy     *snIncidentSort     `json:"sortBy,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snIncidentSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

type snIncidentFilters struct {
	SearchQuery  string   `json:"searchQuery,omitempty"`
	PriorityKeys []int    `json:"priorityKeys,omitempty"` // SN expects int keys
	ParentIDs    []string `json:"parentIds,omitempty"`
	// Number: see domain.SearchIncidentsFilters.Number doc comment. Exact
	// match against ServiceNow's `number` column -- not part of the
	// free-text SearchQuery scan.
	Number string `json:"number,omitempty"`
	// StateKeys: see domain.SearchIncidentsFilters.StateKeys doc comment.
	StateKeys []int `json:"stateKeys,omitempty"`
	// AssignmentGroupIDs: sys_user_group sys_ids (converted from UUIDs).
	AssignmentGroupIDs []string `json:"assignmentGroupIds,omitempty"`
	// BusinessServiceIDs: business_service sys_ids (converted from UUIDs).
	BusinessServiceIDs []string `json:"businessServiceIds,omitempty"`
	// StartCreatedDate/EndCreatedDate: see domain.SearchIncidentsFilters
	// Filters "createdOn" doc comment. Wire keys match case search's own
	// startCreatedDate/endCreatedDate exactly (same UtcDateTimeString
	// contract on the Ballerina/SN side).
	StartCreatedDate string `json:"startCreatedDate,omitempty"`
	EndCreatedDate   string `json:"endCreatedDate,omitempty"`
	// SlaViolated: see domain.SearchIncidentsFilters Filters "slaViolated" doc
	// comment. nil (omitted) means the filter was not supplied.
	SlaViolated *bool `json:"slaViolated,omitempty"`
	// MadeSla: see domain.SearchIncidentsFilters Filters "madeSla" doc
	// comment. nil (omitted) means the filter was not supplied. Deliberately
	// kept separate from SlaViolated above -- this is ServiceNow's own raw,
	// less-reliable `made_sla` field, carried through only for exact parity
	// with SN's native incident dashboards; prefer SlaViolated otherwise.
	MadeSla *bool `json:"madeSla,omitempty"`
	// ProductNames: see domain.SearchIncidentsFilters Filters "productName"
	// doc comment. Matched as a union against the incident's backing
	// business_service name.
	ProductNames []string `json:"productNames,omitempty"`
}

// snIncidentPriorityKeyMap maps domain IncidentPriority enums to SN numeric priority keys.
var snIncidentPriorityKeyMap = map[domain.IncidentPriority]int{
	domain.IncidentPriorityCritical: 1,
	domain.IncidentPriorityHigh:     2,
	domain.IncidentPriorityModerate: 3,
	domain.IncidentPriorityLow:      4,
	domain.IncidentPriorityPlanning: 5,
}

// snIncidentPriorityLabelMap maps SN numeric priority keys to domain enum strings.
var snIncidentPriorityLabelMap = map[int]string{
	1: "CRITICAL",
	2: "HIGH",
	3: "MODERATE",
	4: "LOW",
	5: "PLANNING",
}

var validIncidentPriority = map[domain.IncidentPriority]bool{
	domain.IncidentPriorityCritical: true,
	domain.IncidentPriorityHigh:     true,
	domain.IncidentPriorityModerate: true,
	domain.IncidentPriorityLow:      true,
	domain.IncidentPriorityPlanning: true,
}

var snIncidentStateLabelMap = map[int]string{
	1: "NEW",
	2: "IN_PROGRESS",
	3: "ON_HOLD",
	6: "RESOLVED",
	7: "CLOSED",
	8: "CANCELLED",
}

// snIncidentStateKeyMap maps domain IncidentState enums to SN numeric state keys.
var snIncidentStateKeyMap = map[domain.IncidentState]int{
	domain.IncidentStateNew:        1,
	domain.IncidentStateInProgress: 2,
	domain.IncidentStateOnHold:     3,
	domain.IncidentStateResolved:   6,
	domain.IncidentStateClosed:     7,
	domain.IncidentStateCancelled:  8,
}

var validIncidentState = map[domain.IncidentState]bool{
	domain.IncidentStateNew:        true,
	domain.IncidentStateInProgress: true,
	domain.IncidentStateOnHold:     true,
	domain.IncidentStateResolved:   true,
	domain.IncidentStateClosed:     true,
	domain.IncidentStateCancelled:  true,
}

var snIncidentCategoryLabelMap = map[string]string{
	"inquiry":              "INQUIRY",
	"service_interruption": "SERVICE_INTERRUPTION",
	"security":             "SECURITY",
}

var validIncidentSortField = map[domain.IncidentSortField]bool{
	domain.IncidentSortFieldCreatedOn: true,
	domain.IncidentSortFieldUpdatedOn: true,
	domain.IncidentSortFieldOpenedOn:  true,
}

var validIncidentSortOrder = map[domain.IncidentSortOrder]bool{
	domain.IncidentSortOrderAsc:  true,
	domain.IncidentSortOrderDesc: true,
}

type snIncidentService struct {
	client *integrationservice.Client
	// publisher is nil when Event Hub is not configured — every call site
	// must check before using it. See publishIncidentCreated.
	publisher EventPublisherService
}

// NewServiceNowIncidentService constructs an IncidentService backed by the
// Choreo API. publisher may be nil (see snIncidentService.publisher's doc
// comment).
func NewServiceNowIncidentService(client *integrationservice.Client, publisher EventPublisherService) IncidentService {
	return &snIncidentService{client: client, publisher: publisher}
}

func (s *snIncidentService) SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest) (domain.SearchIncidentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	if err := validateExactNumber("number", req.Filters.Number); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	if req.SortBy.Field != "" && !validIncidentSortField[req.SortBy.Field] {
		return domain.SearchIncidentsResponse{}, &apierror.ValidationError{Msg: "sortBy.field contains invalid value: " + string(req.SortBy.Field)}
	}
	if req.SortBy.Order != "" && !validIncidentSortOrder[req.SortBy.Order] {
		return domain.SearchIncidentsResponse{}, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + string(req.SortBy.Order)}
	}
	for _, p := range req.Filters.Priorities {
		if !validIncidentPriority[p] {
			return domain.SearchIncidentsResponse{}, &apierror.ValidationError{Msg: "priorities contains invalid value: " + string(p)}
		}
	}
	if err := validateUUIDs("parentIds", req.Filters.ParentIDs); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	parsedFilters, err := ParseIncidentFieldFilters(req.Filters.Filters, time.Now().UTC())
	if err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	if parsedFilters.EndCreatedDate != nil && parsedFilters.StartCreatedDate != nil &&
		parsedFilters.EndCreatedDate.Before(*parsedFilters.StartCreatedDate) {
		return domain.SearchIncidentsResponse{}, &apierror.ValidationError{Msg: "createdOn: lte value must not be before gte value"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	var snSortBy *snIncidentSort
	if req.SortBy.Field != "" {
		order := string(req.SortBy.Order)
		if order == "" {
			order = "desc"
		}
		snSortBy = &snIncidentSort{Field: string(req.SortBy.Field), Order: order}
	}

	priorityKeys := make([]int, 0, len(req.Filters.Priorities))
	for _, p := range req.Filters.Priorities {
		priorityKeys = append(priorityKeys, snIncidentPriorityKeyMap[p])
	}

	payload := snIncidentSearchPayload{
		Filters: snIncidentFilters{
			SearchQuery:        req.Filters.SearchQuery,
			PriorityKeys:       priorityKeys,
			ParentIDs:          uuidsToSysids(req.Filters.ParentIDs),
			Number:             stringPtrValue(req.Filters.Number),
			StateKeys:          parsedFilters.StateKeys,
			AssignmentGroupIDs: uuidsToSysids(parsedFilters.AssignmentGroupIDs),
			BusinessServiceIDs: uuidsToSysids(parsedFilters.BusinessServiceIDs),
			StartCreatedDate:   formatSNDateTimeUTC(parsedFilters.StartCreatedDate),
			EndCreatedDate:     formatSNDateTimeUTC(parsedFilters.EndCreatedDate),
			SlaViolated:        parsedFilters.SlaViolated,
			MadeSla:            parsedFilters.MadeSla,
			ProductNames:       parsedFilters.ProductNames,
		},
		SortBy:     snSortBy,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/incidents/search", token, payload)
	if err != nil {
		return domain.SearchIncidentsResponse{}, err
	}

	var snResp snIncidentsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchIncidentsResponse{}, fmt.Errorf("sn incidents: parse response: %w", err)
	}

	views := make([]domain.SearchIncidentView, 0, len(snResp.Incidents))
	for _, inc := range snResp.Incidents {
		view := domain.SearchIncidentView{
			OpenedOn:  inc.OpenedOn,
			Subject:   inc.Subject,
			CreatedOn: inc.CreatedOn,
			CreatedBy: inc.CreatedBy,
			UpdatedOn: inc.UpdatedOn,
			UpdatedBy: inc.UpdatedBy,
		}
		if inc.ID != nil && *inc.ID != "" {
			id := sysidToUUID(*inc.ID)
			view.ID = &id
		}
		if inc.Number != nil {
			view.Number = inc.Number
		}
		if inc.Caller != nil {
			view.Caller = &domain.EntityRef{ID: sysidToUUID(inc.Caller.ID), Name: inc.Caller.Name}
		}
		if inc.Priority != nil {
			if label, ok := snIncidentPriorityLabelMap[inc.Priority.ID]; ok {
				view.Priority = &label
			}
		}
		if inc.State != nil {
			if label, ok := snIncidentStateLabelMap[inc.State.ID]; ok {
				view.State = &label
			}
		}
		if inc.Category != nil {
			if label, ok := snIncidentCategoryLabelMap[inc.Category.ID]; ok {
				view.Category = &label
			}
		}
		if inc.Parent != nil {
			view.Parent = &domain.EntityRef{ID: sysidToUUID(inc.Parent.ID), Name: inc.Parent.Name}
		}
		if inc.ParentIncident != nil {
			view.ParentIncident = &domain.EntityRef{ID: sysidToUUID(inc.ParentIncident.ID), Name: inc.ParentIncident.Name}
		}
		if inc.AssignmentGroup != nil {
			view.AssignmentGroup = &domain.EntityRef{ID: sysidToUUID(inc.AssignmentGroup.ID), Name: inc.AssignmentGroup.Name}
		}
		if inc.AssignedTo != nil {
			view.AssignedTo = &domain.EntityRef{ID: sysidToUUID(inc.AssignedTo.ID), Name: inc.AssignedTo.Name}
		}
		views = append(views, view)
	}

	return domain.SearchIncidentsResponse{
		Incidents: views,
		Total:     snResp.TotalRecords,
		Limit:     req.Pagination.Limit,
		Offset:    req.Pagination.Offset,
	}, nil
}

// snIncidentAggregatePayload is the Choreo POST /incidents/aggregate request body.
type snIncidentAggregatePayload struct {
	Filters   snIncidentFilters `json:"filters,omitempty"`
	GroupBy   string            `json:"groupBy"`
	MaxGroups int               `json:"maxGroups,omitempty"`
}

// validIncidentAggregateField is the allow-list for
// AggregateIncidentsRequest.GroupBy, matching openapi.yaml's
// AggregateIncidentsRequest.groupBy enum exactly.
var validIncidentAggregateField = map[string]bool{
	"state":           true,
	"assignmentGroup": true,
	"businessService": true,
}

// AggregateIncidents implements IncidentService by calling the Choreo POST
// /incidents/aggregate endpoint: a single server-side aggregation over the
// requested field, capped to the top MaxGroups buckets with the remainder
// folded into AggregateResponse.OthersCount. Filter parsing and validation
// mirror SearchIncidents.
func (s *snIncidentService) AggregateIncidents(ctx context.Context, req domain.AggregateIncidentsRequest) (domain.AggregateResponse, error) {
	if req.GroupBy == "" {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy is required"}
	}
	if !validIncidentAggregateField[req.GroupBy] {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + req.GroupBy}
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.AggregateResponse{}, err
	}
	if err := validateExactNumber("number", req.Filters.Number); err != nil {
		return domain.AggregateResponse{}, err
	}
	for _, p := range req.Filters.Priorities {
		if !validIncidentPriority[p] {
			return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "priorities contains invalid value: " + string(p)}
		}
	}
	if err := validateUUIDs("parentIds", req.Filters.ParentIDs); err != nil {
		return domain.AggregateResponse{}, err
	}
	parsedFilters, err := ParseIncidentFieldFilters(req.Filters.Filters, time.Now().UTC())
	if err != nil {
		return domain.AggregateResponse{}, err
	}
	if parsedFilters.EndCreatedDate != nil && parsedFilters.StartCreatedDate != nil &&
		parsedFilters.EndCreatedDate.Before(*parsedFilters.StartCreatedDate) {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "createdOn: lte value must not be before gte value"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	priorityKeys := make([]int, 0, len(req.Filters.Priorities))
	for _, p := range req.Filters.Priorities {
		priorityKeys = append(priorityKeys, snIncidentPriorityKeyMap[p])
	}

	payload := snIncidentAggregatePayload{
		Filters: snIncidentFilters{
			SearchQuery:        req.Filters.SearchQuery,
			PriorityKeys:       priorityKeys,
			ParentIDs:          uuidsToSysids(req.Filters.ParentIDs),
			Number:             stringPtrValue(req.Filters.Number),
			StateKeys:          parsedFilters.StateKeys,
			AssignmentGroupIDs: uuidsToSysids(parsedFilters.AssignmentGroupIDs),
			BusinessServiceIDs: uuidsToSysids(parsedFilters.BusinessServiceIDs),
			StartCreatedDate:   formatSNDateTimeUTC(parsedFilters.StartCreatedDate),
			EndCreatedDate:     formatSNDateTimeUTC(parsedFilters.EndCreatedDate),
			SlaViolated:        parsedFilters.SlaViolated,
			MadeSla:            parsedFilters.MadeSla,
			ProductNames:       parsedFilters.ProductNames,
		},
		GroupBy:   req.GroupBy,
		MaxGroups: req.MaxGroups,
	}

	raw, err := s.client.Post(ctx, "/incidents/aggregate", token, payload)
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	var resp domain.AggregateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("sn incidents: parse aggregate response: %w", err)
	}
	// "assignmentGroup" and "businessService" are the only ID-valued fields
	// in validIncidentAggregateField; SN returns their bucket keys as raw
	// sys_ids, so convert them to this platform's UUIDs before returning.
	// "state" is a plain enum and is left as-is.
	if req.GroupBy == "assignmentGroup" || req.GroupBy == "businessService" {
		for i := range resp.Groups {
			resp.Groups[i].Key = sysidToUUID(resp.Groups[i].Key)
		}
	}
	return resp, nil
}

// snIncidentCategoryKeyMap maps domain IncidentCategory enums to SN category string values.
var snIncidentCategoryKeyMap = map[domain.IncidentCategory]string{
	domain.IncidentCategoryInquiry:             "inquiry",
	domain.IncidentCategoryServiceInterruption: "service_interruption",
	domain.IncidentCategorySecurity:            "security",
}

// snIncidentSubcategoryKeyMap maps domain IncidentSubcategory enums to SN subcategory string values.
var snIncidentSubcategoryKeyMap = map[domain.IncidentSubcategory]string{
	domain.IncidentSubcategoryDHCP:                  "dhcp",
	domain.IncidentSubcategoryOracle:                "oracle",
	domain.IncidentSubcategoryCPU:                   "cpu",
	domain.IncidentSubcategoryKeyboard:              "keyboard",
	domain.IncidentSubcategoryDOSDDOS:               "DOS/ DDOS",
	domain.IncidentSubcategoryPrivilegeEscalations:  "Privilege escalations",
	domain.IncidentSubcategoryThreatIntelligence:    "Threat intelligence",
	domain.IncidentSubcategoryScansAndProbes:        "Scans and Probes",
	domain.IncidentSubcategoryApplicationSecurity:   "Application Security",
	domain.IncidentSubcategoryConfigChangeRequest:   "Config Change Request",
	domain.IncidentSubcategoryIPAddress:             "ip address",
	domain.IncidentSubcategoryFullOutage:            "Full Outage",
	domain.IncidentSubcategorySQLServer:             "sql server",
	domain.IncidentSubcategorySlowness:              "Slowness",
	domain.IncidentSubcategoryMemory:                "memory",
	domain.IncidentSubcategoryMouse:                 "mouse",
	domain.IncidentSubcategoryPrivacy:               "Privacy",
	domain.IncidentSubcategoryDataBreach:            "Data Breach",
	domain.IncidentSubcategorySystemCompromises:     "System Compromises",
	domain.IncidentSubcategoryDNS:                   "dns",
	domain.IncidentSubcategoryOS:                    "os",
	domain.IncidentSubcategoryDisk:                  "disk",
	domain.IncidentSubcategoryVPN:                   "vpn",
	domain.IncidentSubcategoryMalware:               "Malware",
	domain.IncidentSubcategoryVulnerability:         "Vulnerability",
	domain.IncidentSubcategoryUnauthorizedAccess:    "Unauthorized Access",
	domain.IncidentSubcategoryIdentityProtection:    "Identity Protection",
	domain.IncidentSubcategoryPhishing:              "Phishing",
	domain.IncidentSubcategoryImproperConfiguration: "Improper configuration",
	domain.IncidentSubcategoryInformationRequest:    "Information Request",
	domain.IncidentSubcategoryDB2:                   "db2",
	domain.IncidentSubcategoryPartialOutage:         "Partial Outage",
	domain.IncidentSubcategoryEmail:                 "email",
	domain.IncidentSubcategoryMonitor:               "monitor",
	domain.IncidentSubcategoryWireless:              "wireless",
}

// snIncidentContactTypeKeyMap maps domain IncidentContactType enums to SN contact_type string values.
var snIncidentContactTypeKeyMap = map[domain.IncidentContactType]string{
	domain.IncidentContactTypeSelfService:   "self-service",
	domain.IncidentContactTypeEmail:         "email",
	domain.IncidentContactTypeWalkIn:        "walk-in",
	domain.IncidentContactTypeAzure:         "1",
	domain.IncidentContactTypeEmailInternal: "email internal",
	domain.IncidentContactTypeSite247:       "2",
	domain.IncidentContactTypeDirect:        "direct",
	domain.IncidentContactTypePhone:         "phone",
	domain.IncidentContactTypeSentinel:      "sentinel",
	domain.IncidentContactTypeVirtualAgent:  "virtual_agent",
	domain.IncidentContactTypeChat:          "chat",
	domain.IncidentContactTypeEmailExternal: "email external",
}

// snIncidentImpactKeyMap maps domain IncidentImpact enums to SN numeric impact keys.
var snIncidentImpactKeyMap = map[domain.IncidentImpact]int{
	domain.IncidentImpactHigh:   1,
	domain.IncidentImpactMedium: 2,
	domain.IncidentImpactLow:    3,
}

// snIncidentUrgencyKeyMap maps domain IncidentUrgency enums to SN numeric urgency keys.
var snIncidentUrgencyKeyMap = map[domain.IncidentUrgency]int{
	domain.IncidentUrgencyHigh:   1,
	domain.IncidentUrgencyMedium: 2,
	domain.IncidentUrgencyLow:    3,
}

var validIncidentCategory = map[domain.IncidentCategory]bool{
	domain.IncidentCategoryInquiry:             true,
	domain.IncidentCategoryServiceInterruption: true,
	domain.IncidentCategorySecurity:            true,
}

var validIncidentSubcategory = map[domain.IncidentSubcategory]bool{
	domain.IncidentSubcategoryDHCP:                  true,
	domain.IncidentSubcategoryOracle:                true,
	domain.IncidentSubcategoryCPU:                   true,
	domain.IncidentSubcategoryKeyboard:              true,
	domain.IncidentSubcategoryDOSDDOS:               true,
	domain.IncidentSubcategoryPrivilegeEscalations:  true,
	domain.IncidentSubcategoryThreatIntelligence:    true,
	domain.IncidentSubcategoryScansAndProbes:        true,
	domain.IncidentSubcategoryApplicationSecurity:   true,
	domain.IncidentSubcategoryConfigChangeRequest:   true,
	domain.IncidentSubcategoryIPAddress:             true,
	domain.IncidentSubcategoryFullOutage:            true,
	domain.IncidentSubcategorySQLServer:             true,
	domain.IncidentSubcategorySlowness:              true,
	domain.IncidentSubcategoryMemory:                true,
	domain.IncidentSubcategoryMouse:                 true,
	domain.IncidentSubcategoryPrivacy:               true,
	domain.IncidentSubcategoryDataBreach:            true,
	domain.IncidentSubcategorySystemCompromises:     true,
	domain.IncidentSubcategoryDNS:                   true,
	domain.IncidentSubcategoryOS:                    true,
	domain.IncidentSubcategoryDisk:                  true,
	domain.IncidentSubcategoryVPN:                   true,
	domain.IncidentSubcategoryMalware:               true,
	domain.IncidentSubcategoryVulnerability:         true,
	domain.IncidentSubcategoryUnauthorizedAccess:    true,
	domain.IncidentSubcategoryIdentityProtection:    true,
	domain.IncidentSubcategoryPhishing:              true,
	domain.IncidentSubcategoryImproperConfiguration: true,
	domain.IncidentSubcategoryInformationRequest:    true,
	domain.IncidentSubcategoryDB2:                   true,
	domain.IncidentSubcategoryPartialOutage:         true,
	domain.IncidentSubcategoryEmail:                 true,
	domain.IncidentSubcategoryMonitor:               true,
	domain.IncidentSubcategoryWireless:              true,
}

var validIncidentContactType = map[domain.IncidentContactType]bool{
	domain.IncidentContactTypeSelfService:   true,
	domain.IncidentContactTypeEmail:         true,
	domain.IncidentContactTypeWalkIn:        true,
	domain.IncidentContactTypeAzure:         true,
	domain.IncidentContactTypeEmailInternal: true,
	domain.IncidentContactTypeSite247:       true,
	domain.IncidentContactTypeDirect:        true,
	domain.IncidentContactTypePhone:         true,
	domain.IncidentContactTypeSentinel:      true,
	domain.IncidentContactTypeVirtualAgent:  true,
	domain.IncidentContactTypeChat:          true,
	domain.IncidentContactTypeEmailExternal: true,
}

var validIncidentImpact = map[domain.IncidentImpact]bool{
	domain.IncidentImpactHigh:   true,
	domain.IncidentImpactMedium: true,
	domain.IncidentImpactLow:    true,
}

var validIncidentUrgency = map[domain.IncidentUrgency]bool{
	domain.IncidentUrgencyHigh:   true,
	domain.IncidentUrgencyMedium: true,
	domain.IncidentUrgencyLow:    true,
}

// snCreateIncidentPayload is the Choreo POST /incidents request body.
type snCreateIncidentPayload struct {
	CallerID            string   `json:"callerId"`
	CategoryKey         string   `json:"categoryKey"`
	SubcategoryKey      *string  `json:"subcategoryKey,omitempty"`
	ServiceID           string   `json:"serviceId"`
	ServiceOfferingID   *string  `json:"serviceOfferingId,omitempty"`
	ConfigurationItemID *string  `json:"configurationItemId,omitempty"`
	ContactTypeKey      *string  `json:"contactTypeKey,omitempty"`
	ImpactKey           int      `json:"impactKey"`
	UrgencyKey          int      `json:"urgencyKey"`
	AssignmentGroupID   *string  `json:"assignmentGroupId,omitempty"`
	AssignedEngineerID  *string  `json:"assignedEngineerId,omitempty"`
	Subject             string   `json:"subject"`
	WatchList           []string `json:"watchList,omitempty"`
	AdditionalComments  *string  `json:"additionalComments,omitempty"`
	WorkNotes           *string  `json:"workNotes,omitempty"`
	ParentID            *string  `json:"parentId,omitempty"`
	ParentIncidentID    *string  `json:"parentIncidentId,omitempty"`
	ChangeRequestID     *string  `json:"changeRequestId,omitempty"`
	ProblemID           *string  `json:"problemId,omitempty"`
	CausedByID          *string  `json:"causedById,omitempty"`
}

// snCreateIncidentResponse mirrors the Choreo POST /incidents response.
type snCreateIncidentResponse struct {
	Message  string `json:"message"`
	Incident struct {
		ID        string `json:"id"`
		Number    string `json:"number"`
		CreatedOn string `json:"createdOn"`
		CreatedBy string `json:"createdBy"`
	} `json:"incident"`
}

func (s *snIncidentService) CreateIncident(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	if req.Subject == "" {
		return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "subject is required"}
	}
	if req.CallerID == "" {
		return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "callerId is required"}
	}
	if req.Category == "" {
		return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "category is required"}
	}
	if req.ServiceID == "" {
		return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "serviceId is required"}
	}
	if req.Impact == "" {
		return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "impact is required"}
	}
	if req.Urgency == "" {
		return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "urgency is required"}
	}
	if !validIncidentCategory[req.Category] {
		return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid category: " + string(req.Category)}
	}
	if req.Subcategory != nil && !validIncidentSubcategory[*req.Subcategory] {
		return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid subcategory: " + string(*req.Subcategory)}
	}
	if req.ContactType != nil && !validIncidentContactType[*req.ContactType] {
		return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid contactType: " + string(*req.ContactType)}
	}
	if !validIncidentImpact[req.Impact] {
		return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid impact: " + string(req.Impact)}
	}
	if !validIncidentUrgency[req.Urgency] {
		return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid urgency: " + string(req.Urgency)}
	}

	uuidFields := map[string]string{
		"callerId":  req.CallerID,
		"serviceId": req.ServiceID,
	}
	for field, val := range uuidFields {
		if err := validateUUIDs(field, []string{val}); err != nil {
			return domain.CreateIncidentResponse{}, err
		}
	}
	optionalUUIDs := map[string]*string{
		"serviceOfferingId":   req.ServiceOfferingID,
		"configurationItemId": req.ConfigurationItemID,
		"assignmentGroupId":   req.AssignmentGroupID,
		"assignedEngineerId":  req.AssignedEngineerID,
		"parentId":            req.ParentID,
		"parentIncidentId":    req.ParentIncidentID,
		"changeRequestId":     req.ChangeRequestID,
		"problemId":           req.ProblemID,
		"causedById":          req.CausedByID,
	}
	for field, val := range optionalUUIDs {
		if val != nil {
			if err := validateUUIDs(field, []string{*val}); err != nil {
				return domain.CreateIncidentResponse{}, err
			}
		}
	}
	token := middleware.UserIDTokenFromContext(ctx)

	// The backing service's incident-create payload declares the watch list as
	// email addresses, not user ids, so the incoming platform UUIDs are resolved
	// to emails first.
	watchList, err := watchListEmails(ctx, s.client, token, "watchList", req.WatchList)
	if err != nil {
		return domain.CreateIncidentResponse{}, err
	}

	payload := snCreateIncidentPayload{
		CallerID:           uuidToSysid(req.CallerID),
		CategoryKey:        snIncidentCategoryKeyMap[req.Category],
		ServiceID:          uuidToSysid(req.ServiceID),
		ImpactKey:          snIncidentImpactKeyMap[req.Impact],
		UrgencyKey:         snIncidentUrgencyKeyMap[req.Urgency],
		Subject:            req.Subject,
		WatchList:          watchList,
		AdditionalComments: req.AdditionalComments,
		WorkNotes:          req.WorkNotes,
	}
	if req.Subcategory != nil {
		v := snIncidentSubcategoryKeyMap[*req.Subcategory]
		payload.SubcategoryKey = &v
	}
	if req.ContactType != nil {
		v := snIncidentContactTypeKeyMap[*req.ContactType]
		payload.ContactTypeKey = &v
	}
	if req.ServiceOfferingID != nil {
		v := uuidToSysid(*req.ServiceOfferingID)
		payload.ServiceOfferingID = &v
	}
	if req.ConfigurationItemID != nil {
		v := uuidToSysid(*req.ConfigurationItemID)
		payload.ConfigurationItemID = &v
	}
	if req.AssignmentGroupID != nil {
		v := uuidToSysid(*req.AssignmentGroupID)
		payload.AssignmentGroupID = &v
	}
	if req.AssignedEngineerID != nil {
		v := uuidToSysid(*req.AssignedEngineerID)
		payload.AssignedEngineerID = &v
	}
	if req.ParentID != nil {
		v := uuidToSysid(*req.ParentID)
		payload.ParentID = &v
	}
	if req.ParentIncidentID != nil {
		v := uuidToSysid(*req.ParentIncidentID)
		payload.ParentIncidentID = &v
	}
	if req.ChangeRequestID != nil {
		v := uuidToSysid(*req.ChangeRequestID)
		payload.ChangeRequestID = &v
	}
	if req.ProblemID != nil {
		v := uuidToSysid(*req.ProblemID)
		payload.ProblemID = &v
	}
	if req.CausedByID != nil {
		v := uuidToSysid(*req.CausedByID)
		payload.CausedByID = &v
	}

	raw, err := s.client.Post(ctx, "/incidents", token, payload)
	if err != nil {
		return domain.CreateIncidentResponse{}, err
	}

	var snResp snCreateIncidentResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateIncidentResponse{}, fmt.Errorf("sn create incident: parse response: %w", err)
	}

	resp := domain.CreateIncidentResponse{Message: snResp.Message}
	resp.Incident.ID = sysidToUUID(snResp.Incident.ID)
	resp.Incident.Number = snResp.Incident.Number
	resp.Incident.CreatedOn = snResp.Incident.CreatedOn
	resp.Incident.CreatedBy = snResp.Incident.CreatedBy
	s.publishIncidentCreated(ctx, req, resp.Incident.ID)
	return resp, nil
}

// publishIncidentCreated best-effort publishes an incident.created event for
// a newly created incident. Unlike publishCaseCreated, no enrichment round
// trip is needed: Title/ShortDescription come directly from req, which
// already carries everything the notification needs (Subject, and
// optionally AdditionalComments) without a follow-up GetIncidentByID call.
//
// ShortDescription falls back to req.Subject when req.AdditionalComments is
// absent — a freshly created incident often has no additional comments yet,
// and events.Validate on the receiving side requires ShortDescription
// non-empty.
//
// Product/CallTo/IncidentLink are all left unset — this service only
// publishes the fact that an incident was created; it has no
// product→Chat-space mapping or on-call number of its own (Product/CallTo),
// and doesn't know csm-notification-service's own portal URL configuration
// either (IncidentLink, unlike an earlier version of this payload, isn't a
// field at all — that service builds its own portal link from EntityID, the
// same way it already does for case.created). csm-notification-service
// substitutes its own configured Product/CallTo defaults when they're
// absent — see events.IncidentCreatedPayload's doc comment.
//
// Runs synchronously, bounded by publishIncidentCreatedTimeout — see
// publishCaseCreated's doc comment for why (same reasoning applies here).
// Any failure is logged and does not fail CreateIncident itself: the
// incident already exists in ServiceNow by this point.
func (s *snIncidentService) publishIncidentCreated(ctx context.Context, req domain.CreateIncidentRequest, incidentID string) {
	if s.publisher == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, publishIncidentCreatedTimeout)
	defer cancel()

	shortDescription := req.Subject
	if req.AdditionalComments != nil && *req.AdditionalComments != "" {
		shortDescription = *req.AdditionalComments
	}

	payload, err := json.Marshal(events.IncidentCreatedPayload{
		Title:            req.Subject,
		ShortDescription: shortDescription,
	})
	if err != nil {
		slog.ErrorContext(ctx, "sn create incident: encode incident.created payload failed", "incidentId", incidentID, "error", err)
		return
	}
	if err := s.publisher.Publish(ctx, events.TypeIncidentCreated, incidentID, payload); err != nil {
		// Not logging err itself: it can carry a raw Event Hub client error
		// (potentially including connection/broker details), and this
		// service's own convention is to log only ids and sanitised
		// summaries (see CLAUDE.md's Security section). The full error is
		// already durably recorded in event_publish_failures by Publish
		// itself (see EventPublisherService's doc comment) for anyone who
		// needs to debug this specific failure.
		slog.ErrorContext(ctx, "sn create incident: publish incident.created failed", "incidentId", incidentID)
	}
}

// snIncidentSubcategoryLabelMap maps SN subcategory string values to domain enum strings.
var snIncidentSubcategoryLabelMap = map[string]string{
	"dhcp":                   "DHCP",
	"oracle":                 "ORACLE",
	"cpu":                    "CPU",
	"keyboard":               "KEYBOARD",
	"DOS/ DDOS":              "DOS_DDOS",
	"Privilege escalations":  "PRIVILEGE_ESCALATIONS",
	"Threat intelligence":    "THREAT_INTELLIGENCE",
	"Scans and Probes":       "SCANS_AND_PROBES",
	"Application Security":   "APPLICATION_SECURITY",
	"Config Change Request":  "CONFIG_CHANGE_REQUEST",
	"ip address":             "IP_ADDRESS",
	"Full Outage":            "FULL_OUTAGE",
	"sql server":             "SQL_SERVER",
	"Slowness":               "SLOWNESS",
	"memory":                 "MEMORY",
	"mouse":                  "MOUSE",
	"Privacy":                "PRIVACY",
	"Data Breach":            "DATA_BREACH",
	"System Compromises":     "SYSTEM_COMPROMISES",
	"dns":                    "DNS",
	"os":                     "OS",
	"disk":                   "DISK",
	"vpn":                    "VPN",
	"Malware":                "MALWARE",
	"Vulnerability":          "VULNERABILITY",
	"Unauthorized Access":    "UNAUTHORIZED_ACCESS",
	"Identity Protection":    "IDENTITY_PROTECTION",
	"Phishing":               "PHISHING",
	"Improper configuration": "IMPROPER_CONFIGURATION",
	"Information Request":    "INFORMATION_REQUEST",
	"db2":                    "DB2",
	"Partial Outage":         "PARTIAL_OUTAGE",
	"email":                  "EMAIL",
	"monitor":                "MONITOR",
	"wireless":               "WIRELESS",
}

// snIncidentContactTypeLabelMap maps SN contact_type values to domain enum strings.
var snIncidentContactTypeLabelMap = map[string]string{
	"self-service":   "SELF_SERVICE",
	"email":          "EMAIL",
	"walk-in":        "WALK_IN",
	"1":              "AZURE",
	"email internal": "EMAIL_INTERNAL",
	"2":              "SITE_247",
	"direct":         "DIRECT",
	"phone":          "PHONE",
	"sentinel":       "SENTINEL",
	"virtual_agent":  "VIRTUAL_AGENT",
	"chat":           "CHAT",
	"email external": "EMAIL_EXTERNAL",
}

// snIncidentImpactLabelMap maps SN numeric impact IDs to domain enum strings.
var snIncidentImpactLabelMap = map[int]string{
	1: "HIGH",
	2: "MEDIUM",
	3: "LOW",
}

// snIncidentUrgencyLabelMap maps SN numeric urgency IDs to domain enum strings.
var snIncidentUrgencyLabelMap = map[int]string{
	1: "HIGH",
	2: "MEDIUM",
	3: "LOW",
}

// snGetIncidentResponse mirrors the Choreo GET /incidents/{id} response.
type snGetIncidentResponse struct {
	ID                    *string                     `json:"id"`
	Number                *string                     `json:"number"`
	OpenedOn              *string                     `json:"openedOn"`
	Subject               *string                     `json:"subject"`
	Caller                *snIncidentEntityRef        `json:"caller"`
	Priority              *snIncidentIntLabel         `json:"priority"`
	State                 *snIncidentIntLabel         `json:"state"`
	Category              *snIncidentStrLabel         `json:"category"`
	Subcategory           *snIncidentStrLabel         `json:"subcategory"`
	Parent                *snIncidentEntityRef        `json:"parent"`
	ParentIncident        *snIncidentEntityRef        `json:"parentIncident"`
	AssignmentGroup       *snIncidentEntityRef        `json:"assignmentGroup"`
	AssignedTo            *snIncidentEntityRef        `json:"assignedTo"`
	Service               *snIncidentEntityRef        `json:"service"`
	ServiceOffering       *snIncidentEntityRef        `json:"serviceOffering"`
	ConfigurationItem     *snIncidentEntityRef        `json:"configurationItem"`
	ContactType           *snIncidentStrLabel         `json:"contactType"`
	Impact                *snIncidentIntLabel         `json:"impact"`
	Urgency               *snIncidentIntLabel         `json:"urgency"`
	ChangeRequest         *snIncidentEntityRef        `json:"changeRequest"`
	Problem               *snIncidentEntityRef        `json:"problem"`
	CausedBy              *snIncidentEntityRef        `json:"causedBy"`
	AdditionalComments    *string                     `json:"additionalComments"`
	WorkNotes             *string                     `json:"workNotes"`
	WatchList             []snIncidentWatchListItem   `json:"watchList"`
	CreatedOn             string                      `json:"createdOn"`
	CreatedBy             string                      `json:"createdBy"`
	UpdatedOn             string                      `json:"updatedOn"`
	UpdatedBy             string                      `json:"updatedBy"`
	ResolutionCode        *snIncidentStrLabel         `json:"resolutionCode"`
	ResolutionNotes       *string                     `json:"resolutionNotes"`
	ResolvedBy            *string                     `json:"resolvedBy"`
	ResolvedOn            *string                     `json:"resolved"`
	IncidentReport        *string                     `json:"incidentReport"`
	LinkedServiceRequests []snLinkedServiceRequestRef `json:"linkedServiceRequests"`
}

type snIncidentWatchListItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *snIncidentService) GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.IncidentView{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/incidents/"+uuidToSysid(id), token)
	if err != nil {
		return domain.IncidentView{}, err
	}

	var sn snGetIncidentResponse
	if err := json.Unmarshal(raw, &sn); err != nil {
		return domain.IncidentView{}, fmt.Errorf("sn get incident: parse response: %w", err)
	}

	return mapSNIncidentToView(sn), nil
}

// mapSNIncidentToView maps a Choreo incident payload to the domain IncidentView representation.
// Shared by GetIncidentByID and UpdateIncident since both endpoints return the full incident detail.
func mapSNIncidentToView(sn snGetIncidentResponse) domain.IncidentView {
	view := domain.IncidentView{
		OpenedOn:           sn.OpenedOn,
		Subject:            sn.Subject,
		AdditionalComments: sn.AdditionalComments,
		WorkNotes:          sn.WorkNotes,
		CreatedOn:          sn.CreatedOn,
		CreatedBy:          sn.CreatedBy,
		UpdatedOn:          sn.UpdatedOn,
		UpdatedBy:          sn.UpdatedBy,
		ResolutionNotes:    sn.ResolutionNotes,
		ResolvedBy:         sn.ResolvedBy,
		ResolvedOn:         sn.ResolvedOn,
		IncidentReport:     sn.IncidentReport,
	}
	if sn.ResolutionCode != nil {
		view.ResolutionCode = &sn.ResolutionCode.Label
	}
	if sn.ID != nil && *sn.ID != "" {
		v := sysidToUUID(*sn.ID)
		view.ID = &v
	}
	if sn.Number != nil {
		view.Number = sn.Number
	}
	if sn.Caller != nil {
		view.Caller = &domain.EntityRef{ID: sysidToUUID(sn.Caller.ID), Name: sn.Caller.Name}
	}
	if sn.Priority != nil {
		if label, ok := snIncidentPriorityLabelMap[sn.Priority.ID]; ok {
			view.Priority = &label
		}
	}
	if sn.State != nil {
		if label, ok := snIncidentStateLabelMap[sn.State.ID]; ok {
			view.State = &label
		}
	}
	if sn.Category != nil {
		if label, ok := snIncidentCategoryLabelMap[sn.Category.ID]; ok {
			view.Category = &label
		}
	}
	if sn.Subcategory != nil {
		if label, ok := snIncidentSubcategoryLabelMap[sn.Subcategory.ID]; ok {
			view.Subcategory = &label
		}
	}
	if sn.Parent != nil {
		view.Parent = &domain.EntityRef{ID: sysidToUUID(sn.Parent.ID), Name: sn.Parent.Name}
	}
	if sn.ParentIncident != nil {
		view.ParentIncident = &domain.EntityRef{ID: sysidToUUID(sn.ParentIncident.ID), Name: sn.ParentIncident.Name}
	}
	if sn.AssignmentGroup != nil {
		view.AssignmentGroup = &domain.EntityRef{ID: sysidToUUID(sn.AssignmentGroup.ID), Name: sn.AssignmentGroup.Name}
	}
	if sn.AssignedTo != nil {
		view.AssignedTo = &domain.EntityRef{ID: sysidToUUID(sn.AssignedTo.ID), Name: sn.AssignedTo.Name}
	}
	if sn.Service != nil {
		view.Service = &domain.EntityRef{ID: sysidToUUID(sn.Service.ID), Name: sn.Service.Name}
	}
	if sn.ServiceOffering != nil {
		view.ServiceOffering = &domain.EntityRef{ID: sysidToUUID(sn.ServiceOffering.ID), Name: sn.ServiceOffering.Name}
	}
	if sn.ConfigurationItem != nil {
		view.ConfigurationItem = &domain.EntityRef{ID: sysidToUUID(sn.ConfigurationItem.ID), Name: sn.ConfigurationItem.Name}
	}
	if sn.ContactType != nil {
		if label, ok := snIncidentContactTypeLabelMap[sn.ContactType.ID]; ok {
			view.ContactType = &label
		}
	}
	if sn.Impact != nil {
		if label, ok := snIncidentImpactLabelMap[sn.Impact.ID]; ok {
			view.Impact = &label
		}
	}
	if sn.Urgency != nil {
		if label, ok := snIncidentUrgencyLabelMap[sn.Urgency.ID]; ok {
			view.Urgency = &label
		}
	}
	if sn.ChangeRequest != nil {
		view.ChangeRequest = &domain.EntityRef{ID: sysidToUUID(sn.ChangeRequest.ID), Name: sn.ChangeRequest.Name}
	}
	if sn.Problem != nil {
		view.Problem = &domain.EntityRef{ID: sysidToUUID(sn.Problem.ID), Name: sn.Problem.Name}
	}
	if sn.CausedBy != nil {
		view.CausedBy = &domain.EntityRef{ID: sysidToUUID(sn.CausedBy.ID), Name: sn.CausedBy.Name}
	}

	watchList := make([]domain.IncidentWatchListItem, 0, len(sn.WatchList))
	for _, w := range sn.WatchList {
		watchList = append(watchList, domain.IncidentWatchListItem{
			ID:    sysidToUUID(w.ID),
			Name:  w.Name,
			Email: w.Email,
		})
	}
	view.WatchList = watchList

	if len(sn.LinkedServiceRequests) > 0 {
		lsr := make([]domain.LinkedServiceRequestRef, 0, len(sn.LinkedServiceRequests))
		for _, r := range sn.LinkedServiceRequests {
			lsr = append(lsr, domain.LinkedServiceRequestRef{ID: sysidToUUID(r.ID), Number: r.Number, Name: r.Name})
		}
		view.LinkedServiceRequests = lsr
	}

	return view
}

// snUpdateIncidentPayload is the Choreo PATCH /incidents/{id} request body.
type snUpdateIncidentPayload struct {
	Subject             *string   `json:"subject,omitempty"`
	PriorityKey         *int      `json:"priorityKey,omitempty"`
	StateKey            *int      `json:"stateKey,omitempty"`
	CategoryKey         *string   `json:"categoryKey,omitempty"`
	SubcategoryKey      *string   `json:"subcategoryKey,omitempty"`
	ContactTypeKey      *string   `json:"contactTypeKey,omitempty"`
	ImpactKey           *int      `json:"impactKey,omitempty"`
	UrgencyKey          *int      `json:"urgencyKey,omitempty"`
	ResolutionCodeKey   *string   `json:"resolutionCodeKey,omitempty"`
	ParentID            *string   `json:"parentId,omitempty"`
	ParentIncidentID    *string   `json:"parentIncidentId,omitempty"`
	AssignmentGroupID   *string   `json:"assignmentGroupId,omitempty"`
	AssignedEngineerID  *string   `json:"assignedEngineerId,omitempty"`
	ServiceID           *string   `json:"serviceId,omitempty"`
	ServiceOfferingID   *string   `json:"serviceOfferingId,omitempty"`
	ConfigurationItemID *string   `json:"configurationItemId,omitempty"`
	ChangeRequestID     *string   `json:"changeRequestId,omitempty"`
	ProblemID           *string   `json:"problemId,omitempty"`
	CausedByID          *string   `json:"causedById,omitempty"`
	ResolvedByID        *string   `json:"resolvedById,omitempty"`
	ResolutionNotes     *string   `json:"resolutionNotes,omitempty"`
	IncidentReport      *string   `json:"incidentReport,omitempty"`
	AdditionalComments  *string   `json:"additionalComments,omitempty"`
	WorkNotes           *string   `json:"workNotes,omitempty"`
	WatchList           *[]string `json:"watchList,omitempty"`
}

// snUpdateIncidentResponse mirrors the Choreo PATCH /incidents/{id} response.
type snUpdateIncidentResponse struct {
	Message  string                `json:"message"`
	Incident snGetIncidentResponse `json:"incident"`
}

func (s *snIncidentService) UpdateIncident(ctx context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateIncidentResponse{}, err
	}

	hasUpdate := req.Subject != nil || req.Priority != nil || req.State != nil || req.Category != nil ||
		req.Subcategory != nil || req.ContactType != nil || req.Impact != nil || req.Urgency != nil ||
		req.ResolutionCode != nil || req.ParentID != nil || req.ParentIncidentID != nil ||
		req.AssignmentGroupID != nil || req.AssignedEngineerID != nil || req.ServiceID != nil ||
		req.ServiceOfferingID != nil || req.ConfigurationItemID != nil || req.ChangeRequestID != nil ||
		req.ProblemID != nil || req.CausedByID != nil || req.ResolvedByID != nil ||
		req.ResolutionNotes != nil || req.IncidentReport != nil || req.AdditionalComments != nil ||
		req.WorkNotes != nil || req.WatchList != nil
	if !hasUpdate {
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}

	if req.Priority != nil && !validIncidentPriority[*req.Priority] {
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid priority: " + string(*req.Priority)}
	}
	if req.State != nil && !validIncidentState[*req.State] {
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid state: " + string(*req.State)}
	}
	if req.Category != nil && !validIncidentCategory[*req.Category] {
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid category: " + string(*req.Category)}
	}
	if req.Subcategory != nil && !validIncidentSubcategory[*req.Subcategory] {
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid subcategory: " + string(*req.Subcategory)}
	}
	if req.ContactType != nil && !validIncidentContactType[*req.ContactType] {
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid contactType: " + string(*req.ContactType)}
	}
	if req.Impact != nil && !validIncidentImpact[*req.Impact] {
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid impact: " + string(*req.Impact)}
	}
	if req.Urgency != nil && !validIncidentUrgency[*req.Urgency] {
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "invalid urgency: " + string(*req.Urgency)}
	}

	optionalUUIDs := map[string]*string{
		"parentId":            req.ParentID,
		"parentIncidentId":    req.ParentIncidentID,
		"assignmentGroupId":   req.AssignmentGroupID,
		"assignedEngineerId":  req.AssignedEngineerID,
		"serviceId":           req.ServiceID,
		"serviceOfferingId":   req.ServiceOfferingID,
		"configurationItemId": req.ConfigurationItemID,
		"changeRequestId":     req.ChangeRequestID,
		"problemId":           req.ProblemID,
		"causedById":          req.CausedByID,
		"resolvedById":        req.ResolvedByID,
	}
	for field, val := range optionalUUIDs {
		if val != nil {
			if err := validateUUIDs(field, []string{*val}); err != nil {
				return domain.UpdateIncidentResponse{}, err
			}
		}
	}
	if req.WatchList != nil {
		if err := validateUUIDs("watchList", *req.WatchList); err != nil {
			return domain.UpdateIncidentResponse{}, err
		}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snUpdateIncidentPayload{
		Subject:            req.Subject,
		ResolutionNotes:    req.ResolutionNotes,
		IncidentReport:     req.IncidentReport,
		AdditionalComments: req.AdditionalComments,
		WorkNotes:          req.WorkNotes,
	}
	if req.WatchList != nil {
		// The backing service's incident-update payload declares the watch list as
		// usernames -- not emails as its create counterpart does, and not ids --
		// and it replaces the whole list, so an explicitly empty list must still be
		// sent to clear it rather than be skipped.
		userNames, err := watchListUserNames(ctx, s.client, token, "watchList", *req.WatchList)
		if err != nil {
			return domain.UpdateIncidentResponse{}, err
		}
		payload.WatchList = &userNames
	}
	if req.Priority != nil {
		v := snIncidentPriorityKeyMap[*req.Priority]
		payload.PriorityKey = &v
	}
	if req.State != nil {
		v := snIncidentStateKeyMap[*req.State]
		payload.StateKey = &v
	}
	if req.Category != nil {
		v := snIncidentCategoryKeyMap[*req.Category]
		payload.CategoryKey = &v
	}
	if req.Subcategory != nil {
		v := snIncidentSubcategoryKeyMap[*req.Subcategory]
		payload.SubcategoryKey = &v
	}
	if req.ContactType != nil {
		v := snIncidentContactTypeKeyMap[*req.ContactType]
		payload.ContactTypeKey = &v
	}
	if req.Impact != nil {
		v := snIncidentImpactKeyMap[*req.Impact]
		payload.ImpactKey = &v
	}
	if req.Urgency != nil {
		v := snIncidentUrgencyKeyMap[*req.Urgency]
		payload.UrgencyKey = &v
	}
	if req.ResolutionCode != nil {
		payload.ResolutionCodeKey = req.ResolutionCode
	}
	if req.ParentID != nil {
		v := uuidToSysid(*req.ParentID)
		payload.ParentID = &v
	}
	if req.ParentIncidentID != nil {
		v := uuidToSysid(*req.ParentIncidentID)
		payload.ParentIncidentID = &v
	}
	if req.AssignmentGroupID != nil {
		v := uuidToSysid(*req.AssignmentGroupID)
		payload.AssignmentGroupID = &v
	}
	if req.AssignedEngineerID != nil {
		v := uuidToSysid(*req.AssignedEngineerID)
		payload.AssignedEngineerID = &v
	}
	if req.ServiceID != nil {
		v := uuidToSysid(*req.ServiceID)
		payload.ServiceID = &v
	}
	if req.ServiceOfferingID != nil {
		v := uuidToSysid(*req.ServiceOfferingID)
		payload.ServiceOfferingID = &v
	}
	if req.ConfigurationItemID != nil {
		v := uuidToSysid(*req.ConfigurationItemID)
		payload.ConfigurationItemID = &v
	}
	if req.ChangeRequestID != nil {
		v := uuidToSysid(*req.ChangeRequestID)
		payload.ChangeRequestID = &v
	}
	if req.ProblemID != nil {
		v := uuidToSysid(*req.ProblemID)
		payload.ProblemID = &v
	}
	if req.CausedByID != nil {
		v := uuidToSysid(*req.CausedByID)
		payload.CausedByID = &v
	}
	if req.ResolvedByID != nil {
		v := uuidToSysid(*req.ResolvedByID)
		payload.ResolvedByID = &v
	}

	raw, err := s.client.Patch(ctx, "/incidents/"+uuidToSysid(req.ID), token, payload)
	if err != nil {
		return domain.UpdateIncidentResponse{}, err
	}

	var snResp snUpdateIncidentResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.UpdateIncidentResponse{}, fmt.Errorf("sn update incident: parse response: %w", err)
	}

	return domain.UpdateIncidentResponse{
		Message:  snResp.Message,
		Incident: mapSNIncidentToView(snResp.Incident),
	}, nil
}

// SearchIncidentActivities returns the activity feed for an incident. Confirmed by the
// team as a real, distinct endpoint from the case one (both are built on the same
// underlying activity-search mechanism), though this exact request/response shape has
// not been exercised against a live response.
func (s *snIncidentService) SearchIncidentActivities(ctx context.Context, req domain.SearchIncidentActivitiesRequest) (domain.SearchIncidentActivitiesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchIncidentActivitiesResponse{}, err
	}
	if err := validateUUIDs("id", []string{req.IncidentID}); err != nil {
		return domain.SearchIncidentActivitiesResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snSearchActivitiesPayload{
		Pagination:          snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
		IncludeFieldChanges: req.IncludeFieldChanges,
	}

	raw, err := s.client.Post(ctx, "/incidents/"+uuidToSysid(req.IncidentID)+"/activities/search", token, payload)
	if err != nil {
		return domain.SearchIncidentActivitiesResponse{}, err
	}

	var snResp snSearchActivitiesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchIncidentActivitiesResponse{}, fmt.Errorf("sn search incident activities: parse response: %w", err)
	}

	activities, err := mapSNActivitiesToDomain(snResp.Activity)
	if err != nil {
		return domain.SearchIncidentActivitiesResponse{}, fmt.Errorf("sn search incident activities: %w", err)
	}

	total := snResp.TotalRecords
	return domain.SearchIncidentActivitiesResponse{
		Activity: activities,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(activities) < total,
	}, nil
}
