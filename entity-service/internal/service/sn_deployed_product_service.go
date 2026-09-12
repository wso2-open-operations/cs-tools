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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snDeployedProductsResponse mirrors the SN integration service POST /deployed-products/search response.
type snDeployedProductsResponse struct {
	DeployedProducts []snDeployedProduct `json:"deployedProducts"`
	TotalRecords     int                 `json:"totalRecords"`
	Offset           int                 `json:"offset"`
	Limit            int                 `json:"limit"`
}

type snDeployedProduct struct {
	ID         string                    `json:"id"`
	Deployment snDeployedProductRef      `json:"deployment"`
	Product    snDeployedProductRef      `json:"product"`
	Version    *snDeployedProductVersion `json:"version"`
	Cores      *int                      `json:"cores"`
	TPS        *float64                  `json:"tps"` // Ballerina decimal? serialises as 100.0
	Category   *snDeployedProductRef     `json:"category"`
	Updates    []snProductUpdate         `json:"updates"`
	CreatedOn  string                    `json:"createdOn"`
	UpdatedOn  string                    `json:"updatedOn"`
}

// snProductUpdate is the wire shape of a single deployed-product update-history entry,
// matching Ballerina's ProductUpdate record.
type snProductUpdate struct {
	UpdateLevel int     `json:"updateLevel"`
	Date        string  `json:"date"`
	Details     *string `json:"details"`
}

// toSNProductUpdates converts the domain update-history array to its wire shape.
// Returns nil for a nil input so an absent Updates field stays omitted (omitempty) on the
// wire, and a non-nil empty slice for an explicit empty array so a caller can clear history.
func toSNProductUpdates(updates []domain.ProductUpdateEntry) []snProductUpdate {
	if updates == nil {
		return nil
	}
	out := make([]snProductUpdate, 0, len(updates))
	for _, u := range updates {
		out = append(out, snProductUpdate{UpdateLevel: u.UpdateLevel, Date: u.Date, Details: u.Details})
	}
	return out
}

// validateProductUpdates rejects an update-history array containing a negative
// updateLevel or a malformed date before it is ever forwarded to the backing data
// source. It stops at (and reports) the first invalid entry rather than partially
// processing the array. A nil or empty Updates is valid (nothing to check).
func validateProductUpdates(updates []domain.ProductUpdateEntry) error {
	for i, u := range updates {
		if u.UpdateLevel < 0 {
			return &apierror.ValidationError{Msg: fmt.Sprintf("updates[%d].updateLevel must not be negative", i)}
		}
		if err := validateDateOnly(fmt.Sprintf("updates[%d].date", i), u.Date); err != nil {
			return err
		}
	}
	return nil
}

// fromSNProductUpdates converts the wire update-history array to its domain shape.
func fromSNProductUpdates(updates []snProductUpdate) []domain.ProductUpdateEntry {
	if updates == nil {
		return nil
	}
	out := make([]domain.ProductUpdateEntry, 0, len(updates))
	for _, u := range updates {
		out = append(out, domain.ProductUpdateEntry{UpdateLevel: u.UpdateLevel, Date: u.Date, Details: u.Details})
	}
	return out
}

type snDeployedProductRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snDeployedProductVersion struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	ReleasedDate   *string `json:"releasedOn"`
	SupportEoLDate *string `json:"endOfLifeOn"`
}

// snDeployedProductSearchPayload is the SN integration service POST /deployed-products/search request body.
type snDeployedProductSearchPayload struct {
	Filters    snDeployedProductFilters `json:"filters"`
	Pagination snProjectPagination      `json:"pagination"`
}

type snDeployedProductFilters struct {
	DeploymentIDs []string `json:"deploymentIds,omitempty"`
}

type snDeployedProductService struct {
	client *integrationservice.Client
}

// NewServiceNowDeployedProductService constructs a DeployedProductService backed by the SN integration service.
func NewServiceNowDeployedProductService(client *integrationservice.Client) DeployedProductService {
	return &snDeployedProductService{client: client}
}

// snCreateDeployedProductPayload is the Choreo POST /deployed-products request body.
type snCreateDeployedProductPayload struct {
	ProjectID    string   `json:"projectId"`
	DeploymentID string   `json:"deploymentId"`
	ProductID    string   `json:"productId"`
	VersionID    string   `json:"versionId"`
	Cores        *int     `json:"cores,omitempty"`
	TPS          *float64 `json:"tps,omitempty"` // Ballerina decimal?
	Description  *string  `json:"description,omitempty"`
}

type snCreateDeployedProductResponse struct {
	Message         string `json:"message"`
	DeployedProduct struct {
		ID        string `json:"id"`
		CreatedOn string `json:"createdOn"`
		CreatedBy string `json:"createdBy"`
	} `json:"deployedProduct"`
}

// snUpdateDeployedProductPayload is the Choreo PATCH /deployed-products/{id} request body.
// Description is json.RawMessage so an explicit null can be distinguished from an omitted field.
// Updates, when present, whole-array-replaces the deployed product's update-level history.
// Updates is a pointer so that a caller-supplied empty array (clear all history) can be told
// apart from an absent field: encoding/json's omitempty treats a zero-length slice the same as
// nil regardless of nil-ness, so only a non-nil *pointer* to an empty slice serialises as "[]"
// on the wire instead of being dropped.
type snUpdateDeployedProductPayload struct {
	Cores       *int               `json:"cores,omitempty"`
	TPS         *float64           `json:"tps,omitempty"` // Ballerina decimal?
	Description json.RawMessage    `json:"description,omitempty"`
	Updates     *[]snProductUpdate `json:"updates,omitempty"`
	Active      *bool              `json:"active,omitempty"`
}

type snUpdateDeployedProductResponse struct {
	Message         string `json:"message"`
	DeployedProduct struct {
		ID        string            `json:"id"`
		UpdatedOn string            `json:"updatedOn"`
		UpdatedBy string            `json:"updatedBy"`
		Updates   []snProductUpdate `json:"updates"`
	} `json:"deployedProduct"`
}

// CreateDeployedProduct implements DeployedProductService for the ServiceNow data source.
func (s *snDeployedProductService) CreateDeployedProduct(ctx context.Context, req domain.CreateDeployedProductRequest) (domain.CreateDeployedProductResponse, error) {
	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return domain.CreateDeployedProductResponse{}, err
	}
	if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
		return domain.CreateDeployedProductResponse{}, err
	}
	if err := validateUUIDs("productId", []string{req.ProductID}); err != nil {
		return domain.CreateDeployedProductResponse{}, err
	}
	if err := validateUUIDs("versionId", []string{req.VersionID}); err != nil {
		return domain.CreateDeployedProductResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snCreateDeployedProductPayload{
		ProjectID:    uuidToSysid(req.ProjectID),
		DeploymentID: uuidToSysid(req.DeploymentID),
		ProductID:    uuidToSysid(req.ProductID),
		VersionID:    uuidToSysid(req.VersionID),
		Cores:        req.Cores,
		TPS:          req.TPS,
		Description:  req.Description,
	}

	raw, err := s.client.Post(ctx, "/deployed-products", token, payload)
	if err != nil {
		return domain.CreateDeployedProductResponse{}, err
	}

	var snResp snCreateDeployedProductResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateDeployedProductResponse{}, fmt.Errorf("sn create deployed product: parse response: %w", err)
	}

	createdOn, err := time.Parse(snCreatedOnLayout, snResp.DeployedProduct.CreatedOn)
	if err != nil {
		return domain.CreateDeployedProductResponse{}, fmt.Errorf("sn create deployed product: parse createdOn %q: %w", snResp.DeployedProduct.CreatedOn, err)
	}

	return domain.CreateDeployedProductResponse{
		Message: snResp.Message,
		DeployedProduct: domain.CreatedDeployedProduct{
			ID:        sysidToUUID(snResp.DeployedProduct.ID),
			CreatedOn: createdOn,
			CreatedBy: snResp.DeployedProduct.CreatedBy,
		},
	}, nil
}

// UpdateDeployedProduct implements DeployedProductService for the ServiceNow data source.
func (s *snDeployedProductService) UpdateDeployedProduct(ctx context.Context, req domain.UpdateDeployedProductRequest) (domain.UpdateDeployedProductResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateDeployedProductResponse{}, err
	}
	if req.DeploymentID != nil {
		if err := validateUUIDs("deploymentId", []string{*req.DeploymentID}); err != nil {
			return domain.UpdateDeployedProductResponse{}, err
		}
	}

	hasDetailFields := req.Cores != nil || req.TPS != nil || len(req.Description) > 0 || req.Updates != nil
	if !hasDetailFields && req.Active == nil {
		return domain.UpdateDeployedProductResponse{}, &apierror.ValidationError{Msg: "at least one of cores, tps, or description must be provided, or active must be set to false"}
	}
	if req.Active != nil && *req.Active {
		return domain.UpdateDeployedProductResponse{}, &apierror.ValidationError{Msg: "active can only be set to false"}
	}
	if req.Active != nil && hasDetailFields {
		return domain.UpdateDeployedProductResponse{}, &apierror.ValidationError{Msg: "cores, tps, and description must not be provided when deactivating"}
	}
	if err := validateProductUpdates(req.Updates); err != nil {
		return domain.UpdateDeployedProductResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	// When deploymentId is provided, verify the product belongs to that deployment
	// before mutating it to prevent cross-deployment modification (IDOR).
	// Choreo caps pagination at 50, so pages are iterated until the product is found
	// or all results are exhausted.
	if req.DeploymentID != nil {
		const scopePageSize = 50
		productSysid := uuidToSysid(req.ID)
		deploymentSysid := uuidToSysid(*req.DeploymentID)
		found := false
		for offset := 0; !found; offset += scopePageSize {
			searchPayload := snDeployedProductSearchPayload{
				Filters:    snDeployedProductFilters{DeploymentIDs: []string{deploymentSysid}},
				Pagination: snProjectPagination{Limit: scopePageSize, Offset: offset},
			}
			raw, err := s.client.Post(ctx, "/deployed-products/search", token, searchPayload)
			if err != nil {
				return domain.UpdateDeployedProductResponse{}, err
			}
			var searchResp snDeployedProductsResponse
			if err := json.Unmarshal(raw, &searchResp); err != nil {
				return domain.UpdateDeployedProductResponse{}, fmt.Errorf("sn update deployed product: parse scope check: %w", err)
			}
			for _, dp := range searchResp.DeployedProducts {
				if dp.ID == productSysid {
					found = true
					break
				}
			}
			if offset+len(searchResp.DeployedProducts) >= searchResp.TotalRecords {
				break
			}
		}
		if !found {
			return domain.UpdateDeployedProductResponse{}, &apierror.NotFoundError{Msg: "deployed product not found for the given deployment"}
		}
	}

	payload := snUpdateDeployedProductPayload{
		Cores:  req.Cores,
		TPS:    req.TPS,
		Active: req.Active,
	}
	if len(req.Description) > 0 {
		payload.Description = req.Description
	}
	if req.Updates != nil {
		updates := toSNProductUpdates(req.Updates)
		payload.Updates = &updates
	}

	raw, err := s.client.Patch(ctx, "/deployed-products/"+uuidToSysid(req.ID), token, payload)
	if err != nil {
		return domain.UpdateDeployedProductResponse{}, err
	}

	var snResp snUpdateDeployedProductResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.UpdateDeployedProductResponse{}, fmt.Errorf("sn update deployed product: parse response: %w", err)
	}

	updatedOn, err := time.Parse(snCreatedOnLayout, snResp.DeployedProduct.UpdatedOn)
	if err != nil {
		return domain.UpdateDeployedProductResponse{}, fmt.Errorf("sn update deployed product: parse updatedOn %q: %w", snResp.DeployedProduct.UpdatedOn, err)
	}

	return domain.UpdateDeployedProductResponse{
		Message: snResp.Message,
		DeployedProduct: domain.UpdatedDeployedProduct{
			ID:        sysidToUUID(snResp.DeployedProduct.ID),
			UpdatedOn: updatedOn,
			UpdatedBy: snResp.DeployedProduct.UpdatedBy,
			Updates:   fromSNProductUpdates(snResp.DeployedProduct.Updates),
		},
	}, nil
}

// SearchDeployedProducts implements DeployedProductService.
func (s *snDeployedProductService) SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) (domain.SearchDeployedProductsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchDeployedProductsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snDeployedProductSearchPayload{
		Filters:    snDeployedProductFilters{DeploymentIDs: uuidsToSysids(req.DeploymentIDs)},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	raw, err := s.client.Post(ctx, "/deployed-products/search", token, payload)
	if err != nil {
		return domain.SearchDeployedProductsResponse{}, err
	}

	var snResp snDeployedProductsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchDeployedProductsResponse{}, fmt.Errorf("sn deployed products: parse response: %w", err)
	}

	views := make([]domain.DeployedProductView, 0, len(snResp.DeployedProducts))
	for _, dp := range snResp.DeployedProducts {
		createdOn, err := time.Parse(snCreatedOnLayout, dp.CreatedOn)
		if err != nil {
			return domain.SearchDeployedProductsResponse{}, fmt.Errorf("sn deployed products: parse createdOn %q: %w", dp.CreatedOn, err)
		}
		updatedOn, err := time.Parse(snCreatedOnLayout, dp.UpdatedOn)
		if err != nil {
			return domain.SearchDeployedProductsResponse{}, fmt.Errorf("sn deployed products: parse updatedOn %q: %w", dp.UpdatedOn, err)
		}

		var versionRef *domain.DeployedProductVersionRef
		if dp.Version != nil {
			var releasedDate, eolDate *time.Time
			if dp.Version.ReleasedDate != nil {
				t, err := time.Parse(time.DateOnly, *dp.Version.ReleasedDate)
				if err != nil {
					return domain.SearchDeployedProductsResponse{}, fmt.Errorf("sn deployed products: parse version releasedOn %q: %w", *dp.Version.ReleasedDate, err)
				}
				releasedDate = &t
			}
			if dp.Version.SupportEoLDate != nil {
				t, err := time.Parse(time.DateOnly, *dp.Version.SupportEoLDate)
				if err != nil {
					return domain.SearchDeployedProductsResponse{}, fmt.Errorf("sn deployed products: parse version endOfLifeOn %q: %w", *dp.Version.SupportEoLDate, err)
				}
				eolDate = &t
			}
			versionRef = &domain.DeployedProductVersionRef{
				ID:             sysidToUUID(dp.Version.ID),
				Name:           dp.Version.Name,
				ReleasedDate:   releasedDate,
				SupportEoLDate: eolDate,
			}
		}

		var category *string
		if dp.Category != nil {
			category = &dp.Category.Name
		}

		views = append(views, domain.DeployedProductView{
			ID:         sysidToUUID(dp.ID),
			Deployment: domain.EntityRef{ID: sysidToUUID(dp.Deployment.ID), Name: dp.Deployment.Name},
			Product:    domain.EntityRef{ID: sysidToUUID(dp.Product.ID), Name: dp.Product.Name},
			Version:    versionRef,
			Cores:      dp.Cores,
			TPS:        dp.TPS,
			Category:   category,
			Updates:    fromSNProductUpdates(dp.Updates),
			CreatedOn:  createdOn,
			UpdatedOn:  updatedOn,
		})
	}

	total := snResp.TotalRecords
	return domain.SearchDeployedProductsResponse{
		DeployedProducts: views,
		Total:            total,
		Limit:            req.Pagination.Limit,
		Offset:           req.Pagination.Offset,
		HasMore:          req.Pagination.Offset+len(views) < total,
	}, nil
}

// snDeployedProductMetricsInstance mirrors the Choreo
// DeployedProductMetricsInstance shape.
type snDeployedProductMetricsInstance struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Cores int    `json:"cores"`
}

// snDeployedProductMetricsChartEntry mirrors the Choreo
// DeployedProductMetricsChartEntry shape.
type snDeployedProductMetricsChartEntry struct {
	Date          string                             `json:"date"`
	InstanceCount int                                `json:"instanceCount"`
	TotalCores    int                                `json:"totalCores"`
	MinCores      int                                `json:"minCores"`
	MaxCores      int                                `json:"maxCores"`
	AvgCores      float64                            `json:"avgCores"`
	Instances     []snDeployedProductMetricsInstance `json:"instances"`
}

// snDeployedProductMetricsDateRange mirrors the Choreo dateRange shape shared
// by both metrics and usage-counts summaries.
type snDeployedProductMetricsDateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// snDeployedProductMetricsSummary mirrors the Choreo DeployedProductMetricsSummary shape.
type snDeployedProductMetricsSummary struct {
	DateRange      snDeployedProductMetricsDateRange `json:"dateRange"`
	TotalInstances int                               `json:"totalInstances"`
	MinCores       *int                              `json:"minCores"`
	MaxCores       *int                              `json:"maxCores"`
	AvgCores       *float64                          `json:"avgCores"`
}

// snDeployedProductMetricsResponse mirrors the Choreo
// POST /deployed-products/{id}/metrics/search response.
type snDeployedProductMetricsResponse struct {
	DeployedProduct snReferenceTableItem                 `json:"deployedProduct"`
	Summary         snDeployedProductMetricsSummary      `json:"summary"`
	ChartData       []snDeployedProductMetricsChartEntry `json:"chartData"`
}

type snDeployedProductMetricsSearchPayload struct {
	DeploymentID string `json:"deploymentId"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
}

func (s *snDeployedProductService) SearchDeployedProductMetrics(ctx context.Context, id string, req domain.DeployedProductMetricsRequest) (domain.DeployedProductMetricsResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.DeployedProductMetricsResponse{}, err
	}
	if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
		return domain.DeployedProductMetricsResponse{}, err
	}
	if err := validateDateRange(req.StartDate, req.EndDate); err != nil {
		return domain.DeployedProductMetricsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snDeployedProductMetricsSearchPayload{
		DeploymentID: uuidToSysid(req.DeploymentID),
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
	}

	raw, err := s.client.Post(ctx, "/deployed-products/"+uuidToSysid(id)+"/metrics/search", token, payload)
	if err != nil {
		return domain.DeployedProductMetricsResponse{}, err
	}

	var snResp snDeployedProductMetricsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.DeployedProductMetricsResponse{}, fmt.Errorf("sn deployed product metrics: parse response: %w", err)
	}

	chartData := make([]domain.DeployedProductMetricsChartEntry, 0, len(snResp.ChartData))
	for _, e := range snResp.ChartData {
		instances := make([]domain.DeployedProductMetricsInstance, 0, len(e.Instances))
		for _, i := range e.Instances {
			instances = append(instances, domain.DeployedProductMetricsInstance{ID: i.ID, Name: i.Name, Cores: i.Cores})
		}
		chartData = append(chartData, domain.DeployedProductMetricsChartEntry{
			Date:          e.Date,
			InstanceCount: e.InstanceCount,
			TotalCores:    e.TotalCores,
			MinCores:      e.MinCores,
			MaxCores:      e.MaxCores,
			AvgCores:      e.AvgCores,
			Instances:     instances,
		})
	}

	return domain.DeployedProductMetricsResponse{
		DeployedProduct: snResp.DeployedProduct.toDomain(),
		Summary: domain.DeployedProductMetricsSummary{
			DateRange: domain.DeployedProductMetricsDateRange{
				Start: snResp.Summary.DateRange.Start,
				End:   snResp.Summary.DateRange.End,
			},
			TotalInstances: snResp.Summary.TotalInstances,
			MinCores:       snResp.Summary.MinCores,
			MaxCores:       snResp.Summary.MaxCores,
			AvgCores:       snResp.Summary.AvgCores,
		},
		ChartData: chartData,
	}, nil
}

// snUsageCountInstance mirrors the Choreo UsageCountInstance shape.
type snUsageCountInstance struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// snUsageCountEntry mirrors the Choreo UsageCountEntry shape.
type snUsageCountEntry struct {
	Value       float64                `json:"value"`
	Aggregation string                 `json:"aggregation"`
	Instances   []snUsageCountInstance `json:"instances"`
}

// snDeployedProductUsageCountsChartEntry mirrors the Choreo
// DeployedProductUsageCountsChartEntry shape. Counts is keyed by count-type
// name, an open set defined by ServiceNow, not a fixed enum.
type snDeployedProductUsageCountsChartEntry struct {
	Date   string                       `json:"date"`
	Counts map[string]snUsageCountEntry `json:"counts"`
}

// snCountTypeAggregation mirrors the Choreo CountTypeAggregation shape.
type snCountTypeAggregation struct {
	Aggregation string  `json:"aggregation"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Avg         float64 `json:"avg"`
}

// snDeployedProductUsageCountsSummary mirrors the Choreo
// DeployedProductUsageCountsSummary shape.
type snDeployedProductUsageCountsSummary struct {
	DateRange  snDeployedProductMetricsDateRange `json:"dateRange"`
	CountTypes map[string]snCountTypeAggregation `json:"countTypes"`
}

// snDeployedProductUsageCountsResponse mirrors the Choreo
// POST /deployed-products/{id}/metrics/usage-counts/search response.
type snDeployedProductUsageCountsResponse struct {
	DeployedProduct snReferenceTableItem                     `json:"deployedProduct"`
	Summary         snDeployedProductUsageCountsSummary      `json:"summary"`
	ChartData       []snDeployedProductUsageCountsChartEntry `json:"chartData"`
}

func (s *snDeployedProductService) SearchDeployedProductUsageCounts(ctx context.Context, id string, req domain.DeployedProductUsageCountsRequest) (domain.DeployedProductUsageCountsResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.DeployedProductUsageCountsResponse{}, err
	}
	if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
		return domain.DeployedProductUsageCountsResponse{}, err
	}
	if err := validateDateRange(req.StartDate, req.EndDate); err != nil {
		return domain.DeployedProductUsageCountsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snDeployedProductMetricsSearchPayload{
		DeploymentID: uuidToSysid(req.DeploymentID),
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
	}

	raw, err := s.client.Post(ctx, "/deployed-products/"+uuidToSysid(id)+"/metrics/usage-counts/search", token, payload)
	if err != nil {
		return domain.DeployedProductUsageCountsResponse{}, err
	}

	var snResp snDeployedProductUsageCountsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.DeployedProductUsageCountsResponse{}, fmt.Errorf("sn deployed product usage counts: parse response: %w", err)
	}

	toDomainUsageCountEntry := func(e snUsageCountEntry) domain.UsageCountEntry {
		instances := make([]domain.UsageCountInstance, 0, len(e.Instances))
		for _, i := range e.Instances {
			instances = append(instances, domain.UsageCountInstance{ID: i.ID, Name: i.Name, Value: i.Value})
		}
		return domain.UsageCountEntry{Value: e.Value, Aggregation: e.Aggregation, Instances: instances}
	}

	chartData := make([]domain.DeployedProductUsageCountsChartEntry, 0, len(snResp.ChartData))
	for _, e := range snResp.ChartData {
		counts := make(map[string]domain.UsageCountEntry, len(e.Counts))
		for k, v := range e.Counts {
			counts[k] = toDomainUsageCountEntry(v)
		}
		chartData = append(chartData, domain.DeployedProductUsageCountsChartEntry{Date: e.Date, Counts: counts})
	}

	countTypes := make(map[string]domain.CountTypeAggregation, len(snResp.Summary.CountTypes))
	for k, v := range snResp.Summary.CountTypes {
		countTypes[k] = domain.CountTypeAggregation{Aggregation: v.Aggregation, Min: v.Min, Max: v.Max, Avg: v.Avg}
	}

	return domain.DeployedProductUsageCountsResponse{
		DeployedProduct: snResp.DeployedProduct.toDomain(),
		Summary: domain.DeployedProductUsageCountsSummary{
			DateRange: domain.DeployedProductMetricsDateRange{
				Start: snResp.Summary.DateRange.Start,
				End:   snResp.Summary.DateRange.End,
			},
			CountTypes: countTypes,
		},
		ChartData: chartData,
	}, nil
}
