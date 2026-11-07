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
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/viewer"
)

// entityReportsClient is the subset of internal/entity.CustomerEntityClient
// this file needs.
type entityReportsClient interface {
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
	GetProject(ctx context.Context, id string) ([]byte, error)
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	SearchTimeCards(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployments(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error)
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
	Case        *entityTimeCardRef `json:"case"`
}

type entitySearchTimeCardsRequest struct {
	Pagination entityPagination            `json:"pagination"`
	Filters    entitySearchTimeCardsFilter `json:"filters"`
}

type entitySearchTimeCardsFilter struct {
	CaseID     string   `json:"caseId,omitempty"`
	ProjectIDs []string `json:"projectIds,omitempty"`
}

type entitySearchTimeCardsResponse struct {
	TimeCards []entityTimeCardView `json:"timeCards"`
	Total     int                  `json:"total"`
}

// postgresReportsClient implements reportsClient by calling entity-service
// (Postgres). The CS report is built from the project, its cases, and its
// deployments. SLA percentile clocks are not stored in Postgres, so that
// report stays retired.
type postgresReportsClient struct {
	entity entityReportsClient
}

// NewPostgresReportsClient builds a postgresReportsClient. entity is
// typically the same *entity.CustomerEntityClient every other CS Portal
// handler already uses.
func NewPostgresReportsClient(entity entityReportsClient) *postgresReportsClient {
	return &postgresReportsClient{entity: entity}
}

// displayTimeCardState turns entity-service's lowercase time-card state
// into the label the report shows ("Approved", and the rest).
func displayTimeCardState(state string) string {
	switch strings.ToLower(state) {
	case "approved":
		return "Approved"
	case "pending":
		return "Pending"
	case "submitted":
		return "Submitted"
	case "rejected":
		return "Rejected"
	case "processed":
		return "Processed"
	case "recalled":
		return "Recalled"
	default:
		return state
	}
}

// formatHoursMinutes converts a fractional-hours value to the report's
// "1h 2m"/"2m" display format.
func formatHoursMinutes(hours float64) string {
	return formatMinutes(hours * 60)
}

// formatMinutes renders a duration given in minutes as "1h 2m"/"2m". Callers
// sum raw minutes and format once, so rounding never accumulates. A negative
// duration formats its magnitude behind a single "-" ("-1h 30m"); a value that
// rounds to zero carries no sign.
func formatMinutes(minutes float64) string {
	sign := ""
	if minutes < 0 {
		minutes = -minutes
		sign = "-"
	}
	totalMinutes := int(minutes + 0.5) // round to nearest minute
	if totalMinutes == 0 {
		sign = ""
	}
	h := totalMinutes / 60
	m := totalMinutes % 60
	if h == 0 {
		return sign + strconv.Itoa(m) + "m"
	}
	return sign + strconv.Itoa(h) + "h " + strconv.Itoa(m) + "m"
}

// resolveProject resolves the report's projectId query parameter to
// entity-service's project detail. The project detail page links to this
// report with entity-service's own project id (a UUID), while the original
// callers passed the project key, so both are accepted: a UUID is looked up
// directly, anything else goes through resolveProjectByNumber. An unknown id
// of either shape is viewer.ErrProjectNotFound so the route 404s.
func (c *postgresReportsClient) resolveProject(ctx context.Context, projectID string) (entityProjectDetailsView, error) {
	if !uuidRe.MatchString(projectID) {
		return c.resolveProjectByNumber(ctx, projectID)
	}
	detail, err := c.projectDetail(ctx, projectID)
	if err != nil {
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return entityProjectDetailsView{}, viewer.ErrProjectNotFound
		}
		return entityProjectDetailsView{}, err
	}
	return detail, nil
}

// projectDetail fetches and decodes GET /projects/{id}.
func (c *postgresReportsClient) projectDetail(ctx context.Context, projectID string) (entityProjectDetailsView, error) {
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

// resolveProjectByNumber resolves a project key (the "number" the original
// report callers passed -- see postgresSplProjectClient's own doc comment on
// why Number==Key here) to entity-service's internal project detail.
// Search-then-exact-match, the same pattern used throughout this migration
// for every number-keyed lookup.
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
		return entityProjectDetailsView{}, viewer.ErrProjectNotFound
	}
	return c.projectDetail(ctx, projectID)
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

// searchAllProjectTimeCards pages through every time card on the project.
// One project search replaces a search per case, which made this report
// take minutes and outlive the request timeout.
func (c *postgresReportsClient) searchAllProjectTimeCards(ctx context.Context, projectID string) ([]entityTimeCardView, error) {
	var all []entityTimeCardView
	for offset := 0; ; offset += entitySearchPageLimit {
		body, err := json.Marshal(entitySearchTimeCardsRequest{
			Pagination: entityPagination{Limit: entitySearchPageLimit, Offset: offset},
			Filters:    entitySearchTimeCardsFilter{ProjectIDs: []string{projectID}},
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

// TimeLogPeriod is the optional period filter of the time-log breakdown.
// From and To are inclusive YYYY-MM-DD dates, empty meaning "not given".
// AllTime disables the project-date defaults for the bounds that are empty.
type TimeLogPeriod struct {
	From, To string
	AllTime  bool
}

const dateLayout = "2006-01-02"

// dayOf returns the YYYY-MM-DD prefix of a date or RFC 3339 timestamp, or ""
// when the value does not start with a valid date.
func dayOf(v string) string {
	if len(v) < len(dateLayout) {
		return ""
	}
	d := v[:len(dateLayout)]
	if _, err := time.Parse(dateLayout, d); err != nil {
		return ""
	}
	return d
}

// inPeriod reports whether day (YYYY-MM-DD) lies within the inclusive bounds;
// an empty bound is open. An unparseable day is never in a bounded period.
func inPeriod(day, from, to string) bool {
	if from == "" && to == "" {
		return true
	}
	if day == "" {
		return false
	}
	return (from == "" || day >= from) && (to == "" || day <= to)
}

// GetTimeLogBreakdown implements reportsClient. Cases are paged through
// searchAllCases. Time cards are one project search, then attached to the
// case they belong to.
func (c *postgresReportsClient) GetTimeLogBreakdown(ctx context.Context, projectID string, period TimeLogPeriod) (viewer.TimeLogBreakdownDetails, error) {
	project, err := c.resolveProject(ctx, projectID)
	if err != nil {
		return viewer.TimeLogBreakdownDetails{}, err
	}

	cases, err := c.searchAllCases(ctx,
		entitySearchCasesFilters{Filters: []entityCaseFieldFilter{{Field: "projectId", Op: "in", Values: []string{project.ID}}}},
		entityCaseSort{Field: "createdOn", Order: "desc"})
	if err != nil {
		return viewer.TimeLogBreakdownDetails{}, err
	}

	cardsByCase := map[string][]entityTimeCardView{}
	if len(cases) > 0 {
		allCards, err := c.searchAllProjectTimeCards(ctx, project.ID)
		if err != nil {
			return viewer.TimeLogBreakdownDetails{}, err
		}
		for _, tc := range allCards {
			// The report groups cards under cases. A card with no case has nowhere to go.
			if tc.Case == nil || tc.Case.ID == "" {
				continue
			}
			cardsByCase[tc.Case.ID] = append(cardsByCase[tc.Case.ID], tc)
		}
	}

	// Period actually used: an explicit bound wins, otherwise the project's
	// subscription dates, otherwise unbounded.
	from, to := period.From, period.To
	if !period.AllTime {
		if from == "" {
			from = dayOf(derefStr(project.StartDate))
		}
		if to == "" {
			to = dayOf(derefStr(project.EndDate))
		}
	}
	// A project whose own dates are inverted yields an empty result when the
	// caller gave no bound. Once the caller supplied a bound, an inverted
	// effective range (the defaulted side crossing it) is a bad request.
	if (period.From != "" || period.To != "") && from != "" && to != "" && from > to {
		return viewer.TimeLogBreakdownDetails{}, viewer.ErrInvalidTimeLogPeriod
	}

	result := viewer.TimeLogBreakdownDetails{
		ProjectName:         project.Name,
		ProjectKey:          project.Key,
		ProjectType:         project.SubscriptionType,
		RemainingQueryHours: queryHoursDisplay(project.RemainingQueryHours),
		TotalQueryHours:     queryHoursDisplay(project.TotalQueryHours),
		PeriodStart:         from,
		PeriodEnd:           to,
		Cases:               make([]viewer.TimeLogBreakdownCase, 0, len(cases)),
	}

	for _, cv := range cases {
		state := derefStr(cv.State)
		displayState, ok := caseStateToDisplay[state]
		if !ok {
			displayState = state
		}

		var timeCards []entityTimeCardView
		for _, tc := range cardsByCase[cv.ID] {
			if inPeriod(dayOf(tc.WorkDate), from, to) {
				timeCards = append(timeCards, tc)
			}
		}
		// A case is listed when it has an in-period card or was created in the period.
		if len(timeCards) == 0 && !inPeriod(dayOf(cv.CreatedOn), from, to) {
			continue
		}
		_, caseType := reportCaseKind(cv)
		caseResult := viewer.TimeLogBreakdownCase{
			CaseSysID:        cv.ID,
			CaseNumber:       cv.Number,
			CaseID:           cv.InternalID,
			CaseType:         caseType,
			ShortDescription: derefStr(cv.Subject),
			State:            displayState,
			TimeCards:        make([]viewer.TimeCardDetails, 0, len(timeCards)),
		}

		// totalTime is minutes. Sum raw minutes, format once.
		var totalMinutes, consumedMinutes float64
		for _, tc := range timeCards {
			totalMinutes += tc.TotalTime
			tcState := derefStr(tc.State)
			if tc.HasBillable && strings.EqualFold(tcState, "approved") {
				consumedMinutes += tc.TotalTime
			}
			isBillable := "false"
			if tc.HasBillable {
				isBillable = "true"
			}
			loggedBy := ""
			if tc.User != nil {
				loggedBy = tc.User.Name
			}
			caseResult.TimeCards = append(caseResult.TimeCards, viewer.TimeCardDetails{
				Total:      formatMinutes(tc.TotalTime),
				CreatedOn:  tc.WorkDate,
				CreatedBy:  loggedBy,
				IsBillable: isBillable,
				State:      displayTimeCardState(tcState),
			})
		}
		caseResult.TotalHours = formatMinutes(totalMinutes)
		caseResult.ConsumedQueryHours = formatMinutes(consumedMinutes)

		result.Cases = append(result.Cases, caseResult)
	}

	return result, nil
}

// queryHoursDisplay formats a project's query-hours figure (decimal hours)
// as "1h 2m"; nil stays empty.
func queryHoursDisplay(h *float64) string {
	if h == nil {
		return ""
	}
	return formatHoursMinutes(*h)
}
