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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/servicenow"
)

// entityReportsClient is the subset of internal/entity.CustomerEntityClient
// this file needs.
type entityReportsClient interface {
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
	GetProject(ctx context.Context, id string) ([]byte, error)
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	SearchTimeCards(ctx context.Context, body []byte) ([]byte, error)
}

type entityTimeCardRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type entityTimeCardView struct {
	ID          string             `json:"id"`
	TotalTime   float64            `json:"totalTime"`
	WorkDate    string             `json:"workDate"`
	HasBillable bool               `json:"hasBillable"`
	State       *string            `json:"state"`
	User        *entityTimeCardRef `json:"user"`
}

type entitySearchTimeCardsRequest struct {
	Pagination entityPagination            `json:"pagination"`
	Filters    entitySearchTimeCardsFilter `json:"filters"`
}

type entitySearchTimeCardsFilter struct {
	CaseID string `json:"caseId,omitempty"`
}

type entitySearchTimeCardsResponse struct {
	TimeCards []entityTimeCardView `json:"timeCards"`
	Total     int                  `json:"total"`
}

// postgresReportsClient implements reportsClient for
// GetTimeLogBreakdown only, by calling entity-service (Postgres). GetSLAReport
// and GetProjectReportDetails are NOT migrated: both are backed by bespoke
// The backing system scoped-app endpoints (/api/wso2/case_sla/report,
// /api/wso2/cs_report/project-insights) computing statistics (SLA percentile
// breakdowns, monthly case counts, subscription/SLA summaries) that don't
// exist in any form on the Postgres side -- building them would mean
// inventing the underlying business rules (which percentile, what counts as
// "response" vs "workaround" vs "resolution" time) from the backing system
// response shape alone, not wiring up already-defined data. Both stay on the
// wrapped the backing system client.
type postgresReportsClient struct {
	entity entityReportsClient
	sn     reportsClient
}

// NewPostgresReportsClient builds a postgresReportsClient. entity is
// typically the same *entity.CustomerEntityClient every other CS Portal
// handler already uses; sn is the existing the backing system client, kept for the
// two reports above.
func NewPostgresReportsClient(entity entityReportsClient, sn reportsClient) *postgresReportsClient {
	return &postgresReportsClient{entity: entity, sn: sn}
}

// GetSLAReport implements reportsClient by delegating to the wrapped
// The backing system client — see this type's own doc comment for why.
func (c *postgresReportsClient) GetSLAReport(ctx context.Context, projectSysID, from, to string) (servicenow.SLAReportDetails, error) {
	return c.sn.GetSLAReport(ctx, projectSysID, from, to)
}

// GetProjectReportDetails implements reportsClient by delegating to the
// wrapped the backing system client — see this type's own doc comment for why.
func (c *postgresReportsClient) GetProjectReportDetails(ctx context.Context, projectSysID, from, to string) (servicenow.CSReportDetails, error) {
	return c.sn.GetProjectReportDetails(ctx, projectSysID, from, to)
}

// formatHoursMinutes converts a fractional-hours value to the backing system's own
// "1h 2m"/"2m" display format (mirrors internal/servicenow/reports.go's
// formatTime, duplicated here rather than exported since it's a small,
// self-contained formatter and this file has no other dependency on that
// package's internals).
func formatHoursMinutes(hours float64) string {
	totalMinutes := int(hours*60 + 0.5) // round to nearest minute
	h := totalMinutes / 60
	m := totalMinutes % 60
	if h == 0 {
		return strconv.Itoa(m) + "m"
	}
	return strconv.Itoa(h) + "h " + strconv.Itoa(m) + "m"
}

// resolveProjectByNumber resolves SPL's projectId (the backing system's project
// "number", which Postgres has no distinct equivalent of -- see
// postgresSplProjectClient's own doc comment on why Number==Key here) to
// entity-service's internal project detail. Search-then-exact-match, the
// same pattern used throughout this migration for every number-keyed lookup.
func (c *postgresReportsClient) resolveProjectByNumber(ctx context.Context, projectNumber string) (entityProjectDetailsView, error) {
	// Limit is entity-service's own maxLimit (see its SearchProjects
	// validation) -- SearchQuery is a substring match against name/key/
	// subscription type, and the exact-Key match below only looks inside
	// this one page, so a low limit risked missing the target project
	// whenever more than that many projects contained projectNumber as a
	// substring.
	body, err := json.Marshal(entitySearchProjectsRequest{
		Pagination:  entityPagination{Limit: 50, Offset: 0},
		SearchQuery: projectNumber,
	})
	if err != nil {
		return entityProjectDetailsView{}, fmt.Errorf("marshal entity-service projects request: %w", err)
	}
	raw, err := c.entity.SearchProjects(ctx, body)
	if err != nil {
		return entityProjectDetailsView{}, err
	}
	var resp entitySearchProjectsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return entityProjectDetailsView{}, fmt.Errorf("unmarshal entity-service projects response: %w", err)
	}
	var projectID string
	for _, p := range resp.Projects {
		if p.Key == projectNumber {
			projectID = p.ID
			break
		}
	}
	if projectID == "" {
		return entityProjectDetailsView{}, servicenow.ErrProjectNotFound
	}

	detailRaw, err := c.entity.GetProject(ctx, projectID)
	if err != nil {
		return entityProjectDetailsView{}, err
	}
	var detail entityProjectDetailsView
	if err := json.Unmarshal(detailRaw, &detail); err != nil {
		return entityProjectDetailsView{}, fmt.Errorf("unmarshal entity-service project detail response: %w", err)
	}
	return detail, nil
}

// entitySearchPageLimit is entity-service's own hard cap on Pagination.Limit
// per request ("limit cannot exceed 50", confirmed against a real running
// instance) -- searchAllCases/searchAllTimeCards page through Total using
// this as the page size, rather than reading a single page and stopping.
const entitySearchPageLimit = 50

// searchAllCases pages through entity-service's SearchCases using body as
// the template request (its Pagination field is overwritten each page),
// returning every case rather than just the first entitySearchPageLimit.
func (c *postgresReportsClient) searchAllCases(ctx context.Context, filters entitySearchCasesFilters, sortBy entityCaseSort) ([]entitySearchCaseView, error) {
	var all []entitySearchCaseView
	for offset := 0; ; offset += entitySearchPageLimit {
		body, err := json.Marshal(entitySearchCasesRequest{
			Filters:    filters,
			SortBy:     sortBy,
			Pagination: entityPagination{Limit: entitySearchPageLimit, Offset: offset},
		})
		if err != nil {
			return nil, fmt.Errorf("marshal entity-service cases-by-project request: %w", err)
		}
		raw, err := c.entity.SearchCases(ctx, body)
		if err != nil {
			return nil, err
		}
		var resp entitySearchCasesResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal entity-service cases-by-project response: %w", err)
		}
		all = append(all, resp.Cases...)
		if len(all) >= resp.Total || len(resp.Cases) == 0 {
			return all, nil
		}
	}
}

// searchAllTimeCards pages through entity-service's SearchTimeCards for a
// single case, returning every time card rather than just the first
// entitySearchPageLimit.
func (c *postgresReportsClient) searchAllTimeCards(ctx context.Context, caseID string) ([]entityTimeCardView, error) {
	var all []entityTimeCardView
	for offset := 0; ; offset += entitySearchPageLimit {
		body, err := json.Marshal(entitySearchTimeCardsRequest{
			Pagination: entityPagination{Limit: entitySearchPageLimit, Offset: offset},
			Filters:    entitySearchTimeCardsFilter{CaseID: caseID},
		})
		if err != nil {
			return nil, fmt.Errorf("marshal entity-service time-cards request: %w", err)
		}
		raw, err := c.entity.SearchTimeCards(ctx, body)
		if err != nil {
			return nil, err
		}
		var resp entitySearchTimeCardsResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal entity-service time-cards response: %w", err)
		}
		all = append(all, resp.TimeCards...)
		if len(all) >= resp.Total || len(resp.TimeCards) == 0 {
			return all, nil
		}
	}
}

// GetTimeLogBreakdown implements reportsClient, paging through every
// case in the project and every time card per case (see
// searchAllCases/searchAllTimeCards) rather than reading a single
// entitySearchPageLimit-sized page and truncating the rest, which the backing system's
// own version of this report never did either.
func (c *postgresReportsClient) GetTimeLogBreakdown(ctx context.Context, projectID string) (servicenow.TimeLogBreakdownDetails, error) {
	project, err := c.resolveProjectByNumber(ctx, projectID)
	if err != nil {
		return servicenow.TimeLogBreakdownDetails{}, err
	}

	cases, err := c.searchAllCases(ctx,
		entitySearchCasesFilters{Filters: []entityCaseFieldFilter{{Field: "projectId", Op: "in", Values: []string{project.ID}}}},
		entityCaseSort{Field: "createdOn", Order: "desc"})
	if err != nil {
		return servicenow.TimeLogBreakdownDetails{}, err
	}

	result := servicenow.TimeLogBreakdownDetails{
		ProjectName:         project.Name,
		ProjectKey:          project.Key,
		ProjectType:         project.SubscriptionType,
		RemainingQueryHours: hoursOrEmpty(project.RemainingQueryHours),
		TotalQueryHours:     hoursOrEmpty(project.TotalQueryHours),
		Cases:               make([]servicenow.TimeLogBreakdownCase, 0, len(cases)),
	}

	for _, cv := range cases {
		state := derefStr(cv.State)
		displayState, ok := caseStateToDisplay[state]
		if !ok {
			displayState = state
		}

		timeCards, err := c.searchAllTimeCards(ctx, cv.ID)
		if err != nil {
			return servicenow.TimeLogBreakdownDetails{}, err
		}

		caseResult := servicenow.TimeLogBreakdownCase{
			CaseNumber:       cv.Number,
			CaseID:           cv.InternalID,
			ShortDescription: derefStr(cv.Subject),
			State:            displayState,
			TimeCards:        make([]servicenow.TimeCardDetails, 0, len(timeCards)),
		}

		var totalHours, consumedHours float64
		for _, tc := range timeCards {
			totalHours += tc.TotalTime
			tcState := derefStr(tc.State)
			if tc.HasBillable && tcState == "approved" {
				consumedHours += tc.TotalTime
			}
			isBillable := "false"
			if tc.HasBillable {
				isBillable = "true"
			}
			caseResult.TimeCards = append(caseResult.TimeCards, servicenow.TimeCardDetails{
				Total:      formatHoursMinutes(tc.TotalTime),
				CreatedOn:  tc.WorkDate,
				IsBillable: isBillable,
				State:      tcState,
			})
		}
		caseResult.TotalHours = formatHoursMinutes(totalHours)
		caseResult.ConsumedQueryHours = formatHoursMinutes(consumedHours)

		result.Cases = append(result.Cases, caseResult)
	}

	return result, nil
}
