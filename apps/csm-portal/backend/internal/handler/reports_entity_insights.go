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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/viewer"
)

// CS report reads the project, the cases opened in the range, and the
// project's deployments. SLA clocks are not read: those rows are not populated in Postgres yet.

type entitySearchDeploymentsReportRequest struct {
	Pagination entityPagination `json:"pagination"`
	ProjectIDs []string         `json:"projectIds"`
}

type entityDeploymentReportView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type entitySearchDeploymentsReportResponse struct {
	Deployments []entityDeploymentReportView `json:"deployments"`
	Total       int                          `json:"total"`
}

type entitySearchDeployedProductsReportRequest struct {
	Pagination    entityPagination `json:"pagination"`
	DeploymentIDs []string         `json:"deploymentIds"`
}

type entityReportDeployedProduct struct {
	Deployment entityRef `json:"deployment"`
	Product    entityRef `json:"product"`
	Version    *struct {
		Name                           string  `json:"name"`
		SupportEoLDate                 *string `json:"supportEoLDate"`
		EarliestPossibleSupportEOLDate *string `json:"earliestPossibleSupportEolDate"`
	} `json:"version"`
	Cores   *int     `json:"cores"`
	TPS     *float64 `json:"tps"`
	Updates []struct {
		UpdateLevel int `json:"updateLevel"`
	} `json:"updates"`
}

type entitySearchDeployedProductsReportResponse struct {
	DeployedProducts []entityReportDeployedProduct `json:"deployedProducts"`
	Total            int                           `json:"total"`
}

// GetProjectReportDetails builds the CS page from Postgres via entity-service:
// the project (name, key, subscription, dates, query hours, account tier),
// cases opened in the range, and deployments with their products.
func (c *postgresReportsClient) GetProjectReportDetails(ctx context.Context, projectID, from, to string) (viewer.CSReportDetails, error) {
	project, err := c.resolveProject(ctx, projectID)
	if err != nil {
		return viewer.CSReportDetails{}, err
	}
	cases, err := c.casesOpenedBetween(ctx, project.ID, from, to)
	if err != nil {
		return viewer.CSReportDetails{}, err
	}
	deployments, err := c.projectDeployments(ctx, project.ID)
	if err != nil {
		return viewer.CSReportDetails{}, err
	}
	return buildCSReport(project, cases, deployments), nil
}

func (c *postgresReportsClient) casesOpenedBetween(ctx context.Context, projectID, from, to string) ([]entitySearchCaseView, error) {
	filters := []entityCaseFieldFilter{{Field: "projectId", Op: "in", Values: []string{projectID}}}
	if from != "" {
		filters = append(filters, entityCaseFieldFilter{Field: "createdOn", Op: "gte", Values: []string{from}})
	}
	if to != "" {
		filters = append(filters, entityCaseFieldFilter{Field: "createdOn", Op: "lte", Values: []string{to}})
	}
	return c.searchAllCases(ctx, entitySearchCasesFilters{Filters: filters}, entityCaseSort{Field: "createdOn", Order: "desc"})
}

func buildCSReport(project entityProjectDetailsView, cases []entitySearchCaseView, deployments []viewer.ProjectDeployment) viewer.CSReportDetails {
	records := make([]viewer.CaseRecordDetail, 0, len(cases))
	months := map[string]*viewer.Count{}

	for _, cv := range cases {
		kind, label := reportCaseKind(cv)
		records = append(records, viewer.CaseRecordDetail{
			CaseSysID:      cv.ID,
			CaseNumber:     cv.Number,
			EngagementType: derefStr(cv.EngagementType),
			CaseType:       label,
			CasePriority:   severityLabel(derefStr(cv.Severity)),
			CaseState:      displayCaseState(derefStr(cv.State)),
			Opened:         reportTimestamp(cv.CreatedOn),
			Description:    derefStr(cv.Description),
			Updated:        reportTimestamp(cv.UpdatedOn),
			Deployment:     refName(cv.Deployment),
			ProductName:    refName(cv.Product),
		})
		if kind == "incident" || kind == "query" {
			month := reportMonth(cv.CreatedOn)
			if month != "" {
				if months[month] == nil {
					months[month] = &viewer.Count{}
				}
				if kind == "incident" {
					months[month].IncidentCount++
				} else {
					months[month].QueryCount++
				}
			}
		}
	}

	monthKeys := make([]string, 0, len(months))
	for key := range months {
		monthKeys = append(monthKeys, key)
	}
	sort.Strings(monthKeys)
	monthly := make([]viewer.MonthlyCount, 0, len(monthKeys))
	for _, key := range monthKeys {
		monthly = append(monthly, viewer.MonthlyCount{YearAndMonth: key, Counts: *months[key]})
	}

	return viewer.CSReportDetails{
		SubscriptionDetails: viewer.SubscriptionDetail{
			ProjectName:        project.Name,
			ProjectKey:         project.Key,
			ProjectType:        project.SubscriptionType,
			AccountName:        project.Account.Name,
			StartDate:          reportDate(derefStr(project.StartDate)),
			EndDate:            reportDate(derefStr(project.EndDate)),
			SupportTier:        project.Account.Tier,
			Subscription:       project.SubscriptionType,
			TotalQueryHours:    hoursOrEmpty(project.TotalQueryHours),
			ConsumedQueryHours: hoursOrEmpty(project.ConsumedQueryHours),
		},
		CasesRecords:       records,
		SlaDetails:         viewer.SlaStats{SlaRecords: []viewer.SlaRecordDetail{}},
		ProjectDeployments: deployments,
		MonthlyCounts:      monthly,
	}
}

func (c *postgresReportsClient) projectDeployments(ctx context.Context, projectID string) ([]viewer.ProjectDeployment, error) {
	deployments, err := c.searchAllDeployments(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if len(deployments) == 0 {
		return []viewer.ProjectDeployment{}, nil
	}
	ids := make([]string, len(deployments))
	names := map[string]string{}
	for i, d := range deployments {
		ids[i] = d.ID
		names[d.ID] = d.Name
	}
	products, err := c.searchAllDeployedProducts(ctx, ids)
	if err != nil {
		return nil, err
	}
	grouped := map[string][]viewer.Product{}
	for _, product := range products {
		grouped[product.Deployment.ID] = append(grouped[product.Deployment.ID], toReportProduct(product))
	}
	out := make([]viewer.ProjectDeployment, 0, len(deployments))
	for _, d := range deployments {
		items := grouped[d.ID]
		if items == nil {
			items = []viewer.Product{}
		}
		name := d.Name
		if name == "" {
			name = names[d.ID]
		}
		out = append(out, viewer.ProjectDeployment{Name: name, Products: items})
	}
	return out, nil
}

func (c *postgresReportsClient) searchAllDeployments(ctx context.Context, projectID string) ([]entityDeploymentReportView, error) {
	var all []entityDeploymentReportView
	for offset := 0; ; offset += entitySearchPageLimit {
		body, err := json.Marshal(entitySearchDeploymentsReportRequest{
			Pagination: entityPagination{Limit: entitySearchPageLimit, Offset: offset},
			ProjectIDs: []string{projectID},
		})
		if err != nil {
			return nil, fmt.Errorf("marshal entity-service deployments request: %w", err)
		}
		raw, err := c.entity.SearchDeployments(ctx, body)
		if err != nil {
			return nil, err
		}
		var resp entitySearchDeploymentsReportResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal entity-service deployments response: %w", err)
		}
		all = append(all, resp.Deployments...)
		if len(all) >= resp.Total || len(resp.Deployments) == 0 {
			return all, nil
		}
	}
}

func (c *postgresReportsClient) searchAllDeployedProducts(ctx context.Context, deploymentIDs []string) ([]entityReportDeployedProduct, error) {
	if len(deploymentIDs) == 0 {
		return nil, nil
	}
	var all []entityReportDeployedProduct
	for offset := 0; ; offset += entitySearchPageLimit {
		body, err := json.Marshal(entitySearchDeployedProductsReportRequest{
			Pagination:    entityPagination{Limit: entitySearchPageLimit, Offset: offset},
			DeploymentIDs: deploymentIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("marshal entity-service deployed-products request: %w", err)
		}
		raw, err := c.entity.SearchDeployedProducts(ctx, body)
		if err != nil {
			return nil, err
		}
		var resp entitySearchDeployedProductsReportResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal entity-service deployed-products response: %w", err)
		}
		all = append(all, resp.DeployedProducts...)
		if len(all) >= resp.Total || len(resp.DeployedProducts) == 0 {
			return all, nil
		}
	}
}

func toReportProduct(product entityReportDeployedProduct) viewer.Product {
	out := viewer.Product{Name: product.Product.Name}
	if product.Version != nil {
		out.Version = product.Version.Name
		out.EOLDate = reportDate(derefStr(product.Version.SupportEoLDate))
		out.EarliestPossibleSupportEOLDate = reportDate(derefStr(product.Version.EarliestPossibleSupportEOLDate))
		out.SupportStatus = supportStatus(product.Version.SupportEoLDate)
	}
	if product.Cores != nil {
		out.Cores = strconv.Itoa(*product.Cores)
	}
	if product.TPS != nil {
		out.TPS = strconv.FormatFloat(*product.TPS, 'f', -1, 64)
	}
	if len(product.Updates) > 0 {
		level := product.Updates[0].UpdateLevel
		out.UpdateLevelInfo = &level
	}
	return out
}

func supportStatus(eol *string) string {
	if eol == nil || *eol == "" {
		return ""
	}
	parsed, ok := parseReportTime(*eol)
	if !ok {
		return ""
	}
	if parsed.Before(time.Now().UTC()) {
		return "End of Support"
	}
	return "Supported"
}

func reportCaseKind(cv entitySearchCaseView) (kind, label string) {
	issue := strings.ToLower(derefStr(cv.IssueType))
	typ := strings.ToLower(cv.Type)
	if issue == "question" || strings.Contains(typ, "query") {
		return "query", "Query"
	}
	if typ == "case" || typ == "incident" {
		return "incident", "Incident"
	}
	if typ == "engagement" {
		return "engagement", "Engagement"
	}
	if typ == "" {
		return "", ""
	}
	return typ, cv.Type
}

func severityLabel(severity string) string {
	switch strings.ToLower(severity) {
	case "catastrophic":
		return "Catastrophic (P0)"
	case "critical":
		return "Critical (P1)"
	case "high":
		return "High (P2)"
	case "medium":
		return "Medium (P3)"
	case "low":
		return "Low (P4)"
	default:
		return severity
	}
}

func displayCaseState(state string) string {
	if label, ok := caseStateToDisplay[state]; ok {
		return label
	}
	return state
}

func refName(ref *entityRef) string {
	if ref == nil {
		return ""
	}
	return ref.Name
}

func reportTimestamp(raw string) string {
	parsed, ok := parseReportTime(raw)
	if !ok {
		return raw
	}
	return parsed.UTC().Format("2006-01-02 15:04:05")
}

func reportDate(raw string) string {
	parsed, ok := parseReportTime(raw)
	if !ok {
		if len(raw) >= 10 {
			return raw[:10]
		}
		return raw
	}
	return parsed.UTC().Format("2006-01-02")
}

func reportMonth(raw string) string {
	stamp := reportTimestamp(raw)
	if len(stamp) < 7 {
		return ""
	}
	return stamp[:7]
}

func parseReportTime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
