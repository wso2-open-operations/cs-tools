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
)

// entityUsageMetricsClient is the subset of internal/entity.CustomerEntityClient
// this file needs.
type entityUsageMetricsClient interface {
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployments(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error)
	SearchInstances(ctx context.Context, body []byte) ([]byte, error)
	SearchInstanceMetrics(ctx context.Context, body []byte) ([]byte, error)
	SearchInstanceMetricsStats(ctx context.Context, body []byte) ([]byte, error)
	SearchInstanceUsage(ctx context.Context, body []byte) ([]byte, error)
	SearchInstanceUsageStats(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployedProductMetrics(ctx context.Context, deployedProductID string, body []byte) ([]byte, error)
	SearchDeployedProductUsageCounts(ctx context.Context, deployedProductID string, body []byte) ([]byte, error)
}

// postgresUsageMetricsClient implements usageMetricsServiceNowClient by
// calling entity-service (Postgres) instead of the backing system's custom
// x_wso2_customer_0 scoped-app API directly.
//
// entity-service's instance/deployment/deployed-product metrics domain
// (instance_service.go, deployment_service.go, deployed_product_service.go)
// is backed by real synced tables (deployment_information,
// hourly_usage_summary, daily_usage_summary), not a backing-system passthrough --
// and several of its response types were already written to match
// The backing system's own field names field-for-field (see each method's own doc
// comment below for exactly which). Those are round-tripped through a typed
// Go struct purely to validate shape, not to rename or reshape anything.
//
// Where entity-service's Postgres path genuinely has no equivalent
// (deployment's numeric SN "type" id, deployed-product cores/tps/category/
// updates, instance environmentType), the backing-system-shaped output field is
// still present and set to its documented empty/null value rather than
// omitted, so existing frontend code that reads it unconditionally keeps
// working -- same convention as postgresSplAccountClient/
// postgresSplProjectClient elsewhere in this package.
type postgresUsageMetricsClient struct {
	entity entityUsageMetricsClient
}

// NewPostgresUsageMetricsClient builds a postgresUsageMetricsClient.
// entity is typically the same *entity.CustomerEntityClient every other CS
// Portal handler already uses.
func NewPostgresUsageMetricsClient(entity entityUsageMetricsClient) *postgresUsageMetricsClient {
	return &postgresUsageMetricsClient{entity: entity}
}

// --- shared wire types ---

// entityUsageMetricsFilters is entity-service's domain.InstanceSearchFilters/
// InstanceDateRangeFilters shape, which already matches the backing system's own
// UsageMetricsFilters field-for-field (startDate/endDate/projectIds/
// deploymentIds/deployedProductIds) -- used for both the instances/search
// request (filters is optional there) and the two metrics/usage date-range
// requests (required there there), and reused verbatim on the way in since
// nothing renames it.
type entityUsageMetricsFilters struct {
	StartDate          string   `json:"startDate,omitempty"`
	EndDate            string   `json:"endDate,omitempty"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	DeployedProductIDs []string `json:"deployedProductIds,omitempty"`
}

// entityInstanceMetadata mirrors entity-service's domain.InstanceMetadata,
// whose JSON field names already match the backing system's SnInstanceMetadata
// exactly (id/coreCount/updates/jdkVersion/deploymentMetadata/createdOn/
// updatedOn/customCreatedOn/customUpdatedOn).
type entityInstanceMetadata struct {
	ID                 string         `json:"id"`
	CoreCount          *int           `json:"coreCount"`
	Updates            *int           `json:"updates"`
	JDKVersion         *string        `json:"jdkVersion"`
	DeploymentMetadata map[string]any `json:"deploymentMetadata"`
	CreatedOn          string         `json:"createdOn"`
	UpdatedOn          string         `json:"updatedOn"`
	CustomCreatedOn    *string        `json:"customCreatedOn"`
	CustomUpdatedOn    *string        `json:"customUpdatedOn"`
}

// --- GetProjects (typeahead used outside the dedicated usage-metrics
// projects/search endpoint below) ---

// usageMetricsProjectTypeahead mirrors the backing system's customer_project
// TableQuery projection used by GetAllProjects: {sys_id, short_description,
// number}. Number is set to the project's key -- entity-service's project
// table has no separate "number" distinct from "key", the same documented
// substitution postgresSplProjectClient already makes.
type usageMetricsProjectTypeahead struct {
	SysID            string `json:"sys_id"`
	ShortDescription string `json:"short_description"`
	Number           string `json:"number"`
}

// GetAllProjects implements usageMetricsServiceNowClient.
func (c *postgresUsageMetricsClient) GetAllProjects(ctx context.Context, search string) ([]byte, error) {
	req := entitySearchProjectsRequest{
		Pagination:  entityPagination{Limit: 30, Offset: 0},
		SearchQuery: search,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service projects typeahead request: %w", err)
	}
	raw, err := c.entity.SearchProjects(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entitySearchProjectsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service projects typeahead response: %w", err)
	}

	out := make([]usageMetricsProjectTypeahead, 0, len(resp.Projects))
	for _, p := range resp.Projects {
		out = append(out, usageMetricsProjectTypeahead{SysID: p.ID, ShortDescription: p.Name, Number: p.Key})
	}
	return json.Marshal(out)
}

// --- SearchProjects (POST /usage-metrics/projects/search) ---

// snProjectSearchRequest is the incoming request shape from the frontend --
// filters nested under "filters", matching the backing system's ProjectSearchPayload.
type snProjectSearchRequest struct {
	Filters struct {
		SearchQuery string `json:"searchQuery"`
	} `json:"filters"`
	Pagination entityPagination `json:"pagination"`
}

// entityProjectsFullSearchResponse decodes entity-service's
// POST /projects/search response, including Offset/Limit -- unlike
// entitySearchProjectsResponse in projects_postgres.go, which only
// captures Projects/Total since its own callers never needed pagination
// echoed back.
type entityProjectsFullSearchResponse struct {
	Projects []entityProjectView `json:"projects"`
	Total    int                 `json:"total"`
	Limit    int                 `json:"limit"`
	Offset   int                 `json:"offset"`
}

// snProjectListItem is the minimal project shape the usage-metrics project
// picker actually reads (id/name/key) -- the backing system's SnProjectItem carries
// several more fields (description/type/closureState/startDate/endDate) that
// no current caller of this endpoint uses.
type snProjectListItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

type snProjectsSearchResponse struct {
	Projects     []snProjectListItem `json:"projects"`
	TotalRecords int                 `json:"totalRecords"`
	Offset       int                 `json:"offset"`
	Limit        int                 `json:"limit"`
}

// SearchUsageMetricsProjects implements usageMetricsServiceNowClient.
func (c *postgresUsageMetricsClient) SearchUsageMetricsProjects(ctx context.Context, payload []byte) ([]byte, error) {
	var in snProjectSearchRequest
	if err := json.Unmarshal(payload, &in); err != nil {
		return nil, fmt.Errorf("decode usage-metrics projects/search request: %w", err)
	}
	req := entitySearchProjectsRequest{Pagination: in.Pagination, SearchQuery: in.Filters.SearchQuery}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service projects request: %w", err)
	}
	raw, err := c.entity.SearchProjects(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entityProjectsFullSearchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service projects response: %w", err)
	}

	out := snProjectsSearchResponse{
		Projects:     make([]snProjectListItem, 0, len(resp.Projects)),
		TotalRecords: resp.Total,
		Offset:       resp.Offset,
		Limit:        resp.Limit,
	}
	for _, p := range resp.Projects {
		out.Projects = append(out.Projects, snProjectListItem{ID: p.ID, Name: p.Name, Key: p.Key})
	}
	return json.Marshal(out)
}

// --- SearchDeployments (POST /usage-metrics/deployments/search) ---

// snDeploymentsSearchRequest is the incoming request shape from the
// frontend, matching the backing system's DeploymentSearchPayload.
type snDeploymentsSearchRequest struct {
	Filters struct {
		ProjectIDs []string `json:"projectIds"`
	} `json:"filters"`
	Pagination entityPagination `json:"pagination"`
}

// entityDeploymentsSearchRequest mirrors entity-service's own
// domain.SearchDeploymentsRequest exactly: a FLAT request body
// (projectIds/deploymentTypes/searchQuery are top-level, not nested under a
// "filters" object) -- the same flat-vs-nested distinction already found for
// domain.SearchProjectsRequest and domain.SearchDeployedProductsRequest.
type entityDeploymentsSearchRequest struct {
	Pagination entityPagination `json:"pagination"`
	ProjectIDs []string         `json:"projectIds"`
}

// entityDeploymentView mirrors entity-service's domain.DeploymentView, minus
// the fields this endpoint's backing-system-shaped output doesn't need
// (createdBy).
type entityDeploymentView struct {
	ID                   string    `json:"id"`
	Number               string    `json:"number"`
	Name                 string    `json:"name"`
	Type                 string    `json:"type"`
	Description          *string   `json:"description"`
	URL                  *string   `json:"url"`
	Project              entityRef `json:"project"`
	CreatedOn            string    `json:"createdOn"`
	UpdatedOn            string    `json:"updatedOn"`
	DeployedProductCount int       `json:"deployedProductCount"`
}

type entityDeploymentsSearchResponse struct {
	Deployments []entityDeploymentView `json:"deployments"`
	Total       int                    `json:"total"`
	Limit       int                    `json:"limit"`
	Offset      int                    `json:"offset"`
}

// deploymentTypeLabel maps entity-service's DeploymentType enum onto the
// human label the backing system's own deployment_type reference records use --
// deliberately including "Production" for primary_production specifically
// (not "Primary Production"), since the frontend's own environment-tab logic
// does `type.label.toLowerCase() === "production"` to find the default tab.
var deploymentTypeLabel = map[string]string{
	"primary_production": "Production",
	"staging":            "Staging",
	"qa":                 "QA",
	"stress":             "Stress",
	"uat":                "UAT",
	"development":        "Development",
}

// snDeploymentType mirrors the backing system's SnDeploymentType. Id is set to the
// same enum string as Label's source rather than a numeric the backing system
// sys_id/reference id -- entity-service's Postgres schema has no separate
// deployment_type reference table, only a check-constraint enum column, so
// there is no numeric id to carry here. A documented substitution, not a bug.
type snDeploymentType struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// snDeploymentItem mirrors the backing system's SnDeploymentItem. Version is always
// null: entity-service's Postgres deployment table has no equivalent column
// (legacy-data-source-only field).
type snDeploymentItem struct {
	ID                   string           `json:"id"`
	Number               *string          `json:"number"`
	Name                 string           `json:"name"`
	CreatedOn            string           `json:"createdOn"`
	UpdatedOn            string           `json:"updatedOn"`
	Description          *string          `json:"description"`
	URL                  *string          `json:"url"`
	Version              *string          `json:"version"`
	Project              entityRef        `json:"project"`
	Type                 snDeploymentType `json:"type"`
	DeployedProductCount int              `json:"deployedProductCount"`
}

type snDeploymentsSearchResponse struct {
	Deployments  []snDeploymentItem `json:"deployments"`
	TotalRecords int                `json:"totalRecords"`
	Offset       int                `json:"offset"`
	Limit        int                `json:"limit"`
}

// SearchUsageMetricsDeployments implements usageMetricsServiceNowClient.
func (c *postgresUsageMetricsClient) SearchUsageMetricsDeployments(ctx context.Context, payload []byte) ([]byte, error) {
	var in snDeploymentsSearchRequest
	if err := json.Unmarshal(payload, &in); err != nil {
		return nil, fmt.Errorf("decode usage-metrics deployments/search request: %w", err)
	}
	req := entityDeploymentsSearchRequest{Pagination: in.Pagination, ProjectIDs: in.Filters.ProjectIDs}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service deployments request: %w", err)
	}
	raw, err := c.entity.SearchDeployments(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entityDeploymentsSearchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service deployments response: %w", err)
	}

	out := snDeploymentsSearchResponse{
		Deployments:  make([]snDeploymentItem, 0, len(resp.Deployments)),
		TotalRecords: resp.Total,
		Offset:       resp.Offset,
		Limit:        resp.Limit,
	}
	for _, d := range resp.Deployments {
		label := deploymentTypeLabel[d.Type]
		if label == "" {
			label = d.Type
		}
		out.Deployments = append(out.Deployments, snDeploymentItem{
			ID:                   d.ID,
			Number:               &d.Number,
			Name:                 d.Name,
			CreatedOn:            d.CreatedOn,
			UpdatedOn:            d.UpdatedOn,
			Description:          d.Description,
			URL:                  d.URL,
			Version:              nil,
			Project:              d.Project,
			Type:                 snDeploymentType{ID: d.Type, Label: label},
			DeployedProductCount: d.DeployedProductCount,
		})
	}
	return json.Marshal(out)
}

// --- SearchDeployedProducts (POST /usage-metrics/deployed-products/search) ---

// snDeployedProductsSearchRequest is the incoming request shape from the
// frontend, matching the backing system's DeployedProductSearchPayload.
type snDeployedProductsSearchRequest struct {
	Filters struct {
		DeploymentIDs []string `json:"deploymentIds"`
	} `json:"filters"`
	Pagination entityPagination `json:"pagination"`
}

// entityDeployedProductsSearchRequest mirrors entity-service's own
// domain.SearchDeployedProductsRequest exactly: a FLAT request body
// (deploymentIds is top-level, not nested under "filters").
type entityDeployedProductsSearchRequest struct {
	Pagination    entityPagination `json:"pagination"`
	DeploymentIDs []string         `json:"deploymentIds"`
}

type entityProductUpdateEntry struct {
	UpdateLevel int     `json:"updateLevel"`
	Date        string  `json:"date"`
	Details     *string `json:"details"`
}

// entityDeployedProductView mirrors entity-service's domain.DeployedProductView.
// Cores/TPS/Category/Updates are always nil/empty on the Postgres data
// source (see that type's own doc comment) -- carried through here anyway so
// a future Postgres-side backfill of any of them starts working with no
// further change to this file.
type entityDeployedProductView struct {
	ID          string                     `json:"id"`
	Deployment  entityRef                  `json:"deployment"`
	Product     entityProductRef           `json:"product"`
	Version     *entityRef                 `json:"version"`
	Cores       *int                       `json:"cores"`
	TPS         *float64                   `json:"tps"`
	Category    *string                    `json:"category"`
	Description *string                    `json:"description"`
	Updates     []entityProductUpdateEntry `json:"updates"`
	CreatedOn   string                     `json:"createdOn"`
	UpdatedOn   string                     `json:"updatedOn"`
}

type entityProductRef struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Abbreviation *string `json:"abbreviation,omitempty"`
}

type entityDeployedProductsSearchResponse struct {
	DeployedProducts []entityDeployedProductView `json:"deployedProducts"`
	Total            int                         `json:"total"`
	Limit            int                         `json:"limit"`
	Offset           int                         `json:"offset"`
}

// snDeployedProductItem mirrors the backing system's SnDeployedProductItem.
type snDeployedProductItem struct {
	ID          string                     `json:"id"`
	Description *string                    `json:"description"`
	Cores       *int                       `json:"cores"`
	TPS         *float64                   `json:"tps"`
	Updates     []entityProductUpdateEntry `json:"updates"`
	CreatedOn   string                     `json:"createdOn"`
	UpdatedOn   string                     `json:"updatedOn"`
	Category    *entityRef                 `json:"category"`
	Deployment  *entityRef                 `json:"deployment"`
	Product     *entityRef                 `json:"product"`
	Version     *entityRef                 `json:"version"`
}

type snDeployedProductsSearchResponse struct {
	DeployedProducts []snDeployedProductItem `json:"deployedProducts"`
	TotalRecords     int                     `json:"totalRecords"`
	Offset           int                     `json:"offset"`
	Limit            int                     `json:"limit"`
}

// SearchUsageMetricsDeployedProducts implements usageMetricsServiceNowClient.
func (c *postgresUsageMetricsClient) SearchUsageMetricsDeployedProducts(ctx context.Context, payload []byte) ([]byte, error) {
	var in snDeployedProductsSearchRequest
	if err := json.Unmarshal(payload, &in); err != nil {
		return nil, fmt.Errorf("decode usage-metrics deployed-products/search request: %w", err)
	}
	req := entityDeployedProductsSearchRequest{Pagination: in.Pagination, DeploymentIDs: in.Filters.DeploymentIDs}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service deployed-products request: %w", err)
	}
	raw, err := c.entity.SearchDeployedProducts(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entityDeployedProductsSearchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service deployed-products response: %w", err)
	}

	out := snDeployedProductsSearchResponse{
		DeployedProducts: make([]snDeployedProductItem, 0, len(resp.DeployedProducts)),
		TotalRecords:     resp.Total,
		Offset:           resp.Offset,
		Limit:            resp.Limit,
	}
	for _, p := range resp.DeployedProducts {
		item := snDeployedProductItem{
			ID:          p.ID,
			Description: p.Description,
			Cores:       p.Cores,
			TPS:         p.TPS,
			Updates:     p.Updates,
			CreatedOn:   p.CreatedOn,
			UpdatedOn:   p.UpdatedOn,
			Deployment:  &p.Deployment,
			Product:     &entityRef{ID: p.Product.ID, Name: p.Product.Name},
		}
		if p.Version != nil {
			item.Version = p.Version
		}
		out.DeployedProducts = append(out.DeployedProducts, item)
	}
	return json.Marshal(out)
}

// --- SearchInstances (POST /usage-metrics/instances/search) ---

// entityInstance mirrors entity-service's domain.Instance, whose JSON field
// names already match the backing system's SnInstance for everything except
// environmentType (Postgres has no equivalent column) and the top-level
// response's total/totalRecords naming (handled in
// entityInstancesSearchResponse/snInstancesSearchResponse below).
type entityInstance struct {
	ID              string                  `json:"id"`
	Key             string                  `json:"key"`
	Project         *entityRef              `json:"project"`
	Deployment      *entityRef              `json:"deployment"`
	Product         *entityRef              `json:"product"`
	DeployedProduct *entityRef              `json:"deployedProduct"`
	CreatedOn       string                  `json:"createdOn"`
	UpdatedOn       string                  `json:"updatedOn"`
	Metadata        *entityInstanceMetadata `json:"metadata"`
}

type entityInstancesSearchResponse struct {
	Instances []entityInstance `json:"instances"`
	Total     int              `json:"total"`
	Offset    int              `json:"offset"`
	Limit     int              `json:"limit"`
}

type snInstance struct {
	ID              string                  `json:"id"`
	Key             string                  `json:"key"`
	Project         *entityRef              `json:"project"`
	Deployment      *entityRef              `json:"deployment"`
	Product         *entityRef              `json:"product"`
	DeployedProduct *entityRef              `json:"deployedProduct"`
	EnvironmentType *string                 `json:"environmentType"`
	CreatedOn       string                  `json:"createdOn"`
	UpdatedOn       string                  `json:"updatedOn"`
	Metadata        *entityInstanceMetadata `json:"metadata"`
}

type snInstancesSearchResponse struct {
	Instances    []snInstance `json:"instances"`
	Offset       int          `json:"offset"`
	Limit        int          `json:"limit"`
	TotalRecords int          `json:"totalRecords"`
}

// SearchUsageMetricsInstances implements usageMetricsServiceNowClient. The
// request shape (filters nested under "filters", filters itself optional)
// already matches entity-service's domain.SearchInstancesRequest exactly, so
// it is round-tripped through a typed struct only to validate shape, not
// reshaped.
func (c *postgresUsageMetricsClient) SearchUsageMetricsInstances(ctx context.Context, payload []byte) ([]byte, error) {
	var req struct {
		Filters    *entityUsageMetricsFilters `json:"filters,omitempty"`
		Pagination entityPagination           `json:"pagination"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("decode usage-metrics instances/search request: %w", err)
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service instances request: %w", err)
	}
	raw, err := c.entity.SearchInstances(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entityInstancesSearchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service instances response: %w", err)
	}

	out := snInstancesSearchResponse{
		Instances:    make([]snInstance, 0, len(resp.Instances)),
		Offset:       resp.Offset,
		Limit:        resp.Limit,
		TotalRecords: resp.Total,
	}
	for _, i := range resp.Instances {
		out.Instances = append(out.Instances, snInstance{
			ID:              i.ID,
			Key:             i.Key,
			Project:         i.Project,
			Deployment:      i.Deployment,
			Product:         i.Product,
			DeployedProduct: i.DeployedProduct,
			EnvironmentType: nil,
			CreatedOn:       i.CreatedOn,
			UpdatedOn:       i.UpdatedOn,
			Metadata:        i.Metadata,
		})
	}
	return json.Marshal(out)
}

// --- SearchInstanceMetrics / SearchInstanceUsages (time-series) ---

// entityInstanceDataPoint mirrors entity-service's domain.InstanceDataPoint,
// whose JSON field names already match the backing system's SnMetricDataPoint
// exactly.
type entityInstanceDataPoint struct {
	Date               string         `json:"date"`
	CreatedOn          string         `json:"createdOn"`
	CoreCount          *int           `json:"coreCount"`
	JDKVersion         *string        `json:"jdkVersion"`
	Updates            *int           `json:"updates"`
	DeploymentMetadata map[string]any `json:"deploymentMetadata"`
}

// entityInstanceMetric mirrors entity-service's domain.InstanceMetric, whose
// JSON field names already match the backing system's SnMetric exactly.
type entityInstanceMetric struct {
	InstanceID      string                    `json:"instanceId"`
	InstanceKey     string                    `json:"instanceKey"`
	Project         *entityRef                `json:"project"`
	Deployment      *entityRef                `json:"deployment"`
	Product         *entityRef                `json:"product"`
	DeployedProduct *entityRef                `json:"deployedProduct"`
	DataPoints      []entityInstanceDataPoint `json:"dataPoints"`
}

// entityInstanceMetricsResponse mirrors entity-service's own
// domain.InstanceMetricsResponse. Its JSON shape (metrics/totalInstances/
// startDate/endDate) already matches the backing system's SnMetricsSearchResponse
// exactly -- decoded and re-encoded verbatim, not reshaped.
type entityInstanceMetricsResponse struct {
	Metrics        []entityInstanceMetric `json:"metrics"`
	TotalInstances int                    `json:"totalInstances"`
	StartDate      string                 `json:"startDate"`
	EndDate        string                 `json:"endDate"`
}

// SearchInstanceMetrics implements usageMetricsServiceNowClient.
func (c *postgresUsageMetricsClient) SearchInstanceMetrics(ctx context.Context, payload []byte) ([]byte, error) {
	var req struct {
		Filters entityUsageMetricsFilters `json:"filters"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("decode usage-metrics instances/metrics/search request: %w", err)
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service instance metrics request: %w", err)
	}
	raw, err := c.entity.SearchInstanceMetrics(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entityInstanceMetricsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service instance metrics response: %w", err)
	}
	return json.Marshal(resp)
}

// entityInstanceSummary mirrors entity-service's domain.InstanceSummary,
// whose JSON field names already match the backing system's SnPeriodSummary exactly
// (period/counts, with counts an open map keyed by count-type name).
type entityInstanceSummary struct {
	Period string         `json:"period"`
	Counts map[string]int `json:"counts"`
}

// entityInstanceUsageEntry mirrors entity-service's domain.InstanceUsageEntry,
// whose JSON field names already match the backing system's SnUsage exactly.
type entityInstanceUsageEntry struct {
	InstanceID      string                  `json:"instanceId"`
	InstanceKey     string                  `json:"instanceKey"`
	Project         *entityRef              `json:"project"`
	Deployment      *entityRef              `json:"deployment"`
	Product         *entityRef              `json:"product"`
	DeployedProduct *entityRef              `json:"deployedProduct"`
	PeriodSummaries []entityInstanceSummary `json:"periodSummaries"`
}

// entityInstanceUsageResponse mirrors entity-service's own
// domain.InstanceUsageResponse. Its JSON shape (usages/totalInstances/
// startDate/endDate) already matches the backing system's SnUsagesSearchResponse
// exactly -- decoded and re-encoded verbatim, not reshaped.
type entityInstanceUsageResponse struct {
	Usages         []entityInstanceUsageEntry `json:"usages"`
	TotalInstances int                        `json:"totalInstances"`
	StartDate      string                     `json:"startDate"`
	EndDate        string                     `json:"endDate"`
}

// SearchInstanceUsages implements usageMetricsServiceNowClient.
func (c *postgresUsageMetricsClient) SearchInstanceUsages(ctx context.Context, payload []byte) ([]byte, error) {
	var req struct {
		Filters entityUsageMetricsFilters `json:"filters"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("decode usage-metrics instances/usages/search request: %w", err)
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service instance usage request: %w", err)
	}
	raw, err := c.entity.SearchInstanceUsage(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entityInstanceUsageResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service instance usage response: %w", err)
	}
	return json.Marshal(resp)
}

// --- GetInstanceMetricsStats / GetInstanceUsagesStats ---

// entityInstanceStatsFilters extends entityUsageMetricsFilters with the
// dataSource discriminator these two stats endpoints alone accept, matching
// entity-service's domain.InstanceStatsFilters.
type entityInstanceStatsFilters struct {
	entityUsageMetricsFilters
	DataSource *int `json:"dataSource,omitempty"`
}

// entityInstanceMetricsStatsResponse mirrors entity-service's own
// domain.InstanceMetricsStatsResponse.
type entityInstanceMetricsStatsResponse struct {
	Stats     map[string]map[string]int   `json:"stats"`
	Summary   entityInstanceMetricSummary `json:"summary"`
	Total     int                         `json:"total"`
	StartDate string                      `json:"startDate"`
	EndDate   string                      `json:"endDate"`
}

type entityInstanceMetricSummary struct {
	Current float64 `json:"current"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Avg     float64 `json:"avg"`
}

// snStatsSummary mirrors the backing system's SnStatsSummary -- note curr, not
// current, and int-valued rather than float (entity-service's own
// aggregation is over integer core/count columns).
type snStatsSummary struct {
	Curr int     `json:"curr"`
	Min  int     `json:"min"`
	Max  int     `json:"max"`
	Avg  float64 `json:"avg"`
}

// snMetricsStatsResponse mirrors the backing system's SnMetricsStatsResponse --
// TotalRecords, not Total, is the only rename needed against
// entityInstanceMetricsStatsResponse.
type snMetricsStatsResponse struct {
	Stats        map[string]map[string]int `json:"stats"`
	Summary      snStatsSummary            `json:"summary"`
	TotalRecords int                       `json:"totalRecords"`
	StartDate    string                    `json:"startDate"`
	EndDate      string                    `json:"endDate"`
}

// GetInstanceMetricsStats implements usageMetricsServiceNowClient.
func (c *postgresUsageMetricsClient) GetInstanceMetricsStats(ctx context.Context, payload []byte) ([]byte, error) {
	var req struct {
		Filters entityInstanceStatsFilters `json:"filters"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("decode usage-metrics instances/metrics/stats request: %w", err)
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service instance metrics stats request: %w", err)
	}
	raw, err := c.entity.SearchInstanceMetricsStats(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entityInstanceMetricsStatsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service instance metrics stats response: %w", err)
	}

	out := snMetricsStatsResponse{
		Stats: resp.Stats,
		Summary: snStatsSummary{
			Curr: int(resp.Summary.Current),
			Min:  int(resp.Summary.Min),
			Max:  int(resp.Summary.Max),
			Avg:  resp.Summary.Avg,
		},
		TotalRecords: resp.Total,
		StartDate:    resp.StartDate,
		EndDate:      resp.EndDate,
	}
	return json.Marshal(out)
}

// entityInstanceUsageStatsResponse mirrors entity-service's own
// domain.InstanceUsageStatsResponse. the backing system's SnUsagesStatsResponse only
// requires "stats" (an open record with json...), so total/startDate/endDate
// are included here too rather than dropped -- harmless extra fields, and
// useful to any caller that does want them.
type entityInstanceUsageStatsResponse struct {
	Stats     map[string]map[string]int `json:"stats"`
	Total     int                       `json:"total"`
	StartDate string                    `json:"startDate"`
	EndDate   string                    `json:"endDate"`
}

type snUsagesStatsResponse struct {
	Stats        map[string]map[string]int `json:"stats"`
	TotalRecords int                       `json:"totalRecords"`
	StartDate    string                    `json:"startDate"`
	EndDate      string                    `json:"endDate"`
}

// GetInstanceUsagesStats implements usageMetricsServiceNowClient.
func (c *postgresUsageMetricsClient) GetInstanceUsagesStats(ctx context.Context, payload []byte) ([]byte, error) {
	var req struct {
		Filters entityInstanceStatsFilters `json:"filters"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("decode usage-metrics instances/usages/stats request: %w", err)
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service instance usage stats request: %w", err)
	}
	raw, err := c.entity.SearchInstanceUsageStats(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entityInstanceUsageStatsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service instance usage stats response: %w", err)
	}

	out := snUsagesStatsResponse{
		Stats:        resp.Stats,
		TotalRecords: resp.Total,
		StartDate:    resp.StartDate,
		EndDate:      resp.EndDate,
	}
	return json.Marshal(out)
}

// --- Per-deployed-product metrics / usage counts ---

// entityDeployedProductMetricsRequest mirrors both entity-service's own
// domain.DeployedProductMetricsRequest and the backing system's
// DeployedProductMetricsPayload -- deploymentId/startDate/endDate, identical
// on both sides -- so the incoming body is decoded and re-encoded verbatim,
// not reshaped.
type entityDeployedProductMetricsRequest struct {
	DeploymentID string `json:"deploymentId"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
}

// entityDeployedProductMetricsResponse mirrors entity-service's own
// domain.DeployedProductMetricsResponse. Its JSON shape (deployedProduct/
// summary/chartData, with summary.dateRange.start/end and per-entry
// date/instanceCount/totalCores/minCores/maxCores/avgCores/instances[].id/
// name/cores) already matches the backing system's SnDeployedProductMetricsResponse
// exactly -- decoded and re-encoded verbatim, not reshaped.
type entityDeployedProductMetricsResponse struct {
	DeployedProduct entityRef                           `json:"deployedProduct"`
	Summary         entityDeployedProductMetricsSummary `json:"summary"`
	ChartData       []entityDeployedProductChartEntry   `json:"chartData"`
}

type entityDeployedProductMetricsSummary struct {
	DateRange      entityDeployedProductDateRange `json:"dateRange"`
	TotalInstances int                            `json:"totalInstances"`
	MinCores       *int                           `json:"minCores"`
	MaxCores       *int                           `json:"maxCores"`
	AvgCores       *float64                       `json:"avgCores"`
}

type entityDeployedProductDateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type entityDeployedProductMetricsInstance struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Cores int    `json:"cores"`
}

type entityDeployedProductChartEntry struct {
	Date          string                                 `json:"date"`
	InstanceCount int                                    `json:"instanceCount"`
	TotalCores    int                                    `json:"totalCores"`
	MinCores      int                                    `json:"minCores"`
	MaxCores      int                                    `json:"maxCores"`
	AvgCores      float64                                `json:"avgCores"`
	Instances     []entityDeployedProductMetricsInstance `json:"instances"`
}

// GetDeployedProductMetrics implements usageMetricsServiceNowClient.
func (c *postgresUsageMetricsClient) GetDeployedProductMetrics(ctx context.Context, deployedProductID string, payload []byte) ([]byte, error) {
	var req entityDeployedProductMetricsRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("decode deployed-product metrics request: %w", err)
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service deployed-product metrics request: %w", err)
	}
	raw, err := c.entity.SearchDeployedProductMetrics(ctx, deployedProductID, body)
	if err != nil {
		return nil, err
	}
	var resp entityDeployedProductMetricsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service deployed-product metrics response: %w", err)
	}
	return json.Marshal(resp)
}

// entityDeployedProductUsageCountInstance mirrors entity-service's
// domain.UsageCountInstance, whose JSON field names already match
// The backing system's DeployedProductUsageCountInstance exactly.
type entityDeployedProductUsageCountInstance struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type entityDeployedProductUsageCountEntry struct {
	Value       float64                                   `json:"value"`
	Aggregation string                                    `json:"aggregation"`
	Instances   []entityDeployedProductUsageCountInstance `json:"instances"`
}

type entityDeployedProductUsageCountsChartEntry struct {
	Date   string                                          `json:"date"`
	Counts map[string]entityDeployedProductUsageCountEntry `json:"counts"`
}

type entityCountTypeAggregation struct {
	Aggregation string  `json:"aggregation"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Avg         float64 `json:"avg"`
}

type entityDeployedProductUsageCountsSummary struct {
	DateRange  entityDeployedProductDateRange        `json:"dateRange"`
	CountTypes map[string]entityCountTypeAggregation `json:"countTypes"`
}

// entityDeployedProductUsageCountsResponse mirrors entity-service's own
// domain.DeployedProductUsageCountsResponse. Its JSON shape already matches
// The backing system's SnDeployedProductUsageCountsResponse exactly -- decoded and
// re-encoded verbatim, not reshaped.
type entityDeployedProductUsageCountsResponse struct {
	DeployedProduct entityRef                                    `json:"deployedProduct"`
	Summary         entityDeployedProductUsageCountsSummary      `json:"summary"`
	ChartData       []entityDeployedProductUsageCountsChartEntry `json:"chartData"`
}

// GetDeployedProductUsageCounts implements usageMetricsServiceNowClient.
func (c *postgresUsageMetricsClient) GetDeployedProductUsageCounts(ctx context.Context, deployedProductID string, payload []byte) ([]byte, error) {
	var req entityDeployedProductMetricsRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("decode deployed-product usage-counts request: %w", err)
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service deployed-product usage-counts request: %w", err)
	}
	raw, err := c.entity.SearchDeployedProductUsageCounts(ctx, deployedProductID, body)
	if err != nil {
		return nil, err
	}
	var resp entityDeployedProductUsageCountsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service deployed-product usage-counts response: %w", err)
	}
	return json.Marshal(resp)
}
