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
	"sort"
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
	ID          string                    `json:"id"`
	Deployment  snDeployedProductRef      `json:"deployment"`
	Product     snDeployedProductRef      `json:"product"`
	Version     *snDeployedProductVersion `json:"version"`
	Cores       *int                      `json:"cores"`
	TPS         *float64                  `json:"tps"` // Ballerina decimal? serialises as 100.0
	Category    *snDeployedProductRef     `json:"category"`
	Description *string                   `json:"description"`
	Updates     []snProductUpdate         `json:"updates"`
	CreatedOn   string                    `json:"createdOn"`
	UpdatedOn   string                    `json:"updatedOn"`
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
	// Abbreviation is the short product key ("wso2am", "wso2is"), carried by
	// the upstream ReferenceTableItem record for the product reference and
	// absent on the others. It is the only identifier the product-updates
	// catalogue recognises — see domain.ProductRef.
	Abbreviation *string `json:"abbreviation"`
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
	DeploymentIDs     []string `json:"deploymentIds,omitempty"`
	ProductCategories []string `json:"productCategories,omitempty"`
}

type snDeployedProductService struct {
	client *integrationservice.Client
	// deploymentSvc is used only by SearchProjectsByProductVersion, to
	// enumerate every deployment platform-wide (deployment -> project is a
	// join this service has no other way to make — see that method's own
	// doc comment). Always the ServiceNow-backed DeploymentService in
	// practice: routes.go only ever constructs this service at all when
	// cfg.DataSource is already ServiceNow, and it wires the matching
	// DeploymentService from that same branch.
	deploymentSvc DeploymentService
	// projectSvc is used only by SearchProjectsByProductVersion, to resolve
	// which of the product-version-matched projects are actually eligible
	// for an announcement (excludeClosureStates/excludeSubscriptionTypes) —
	// see that method's own doc comment. Same practice as deploymentSvc:
	// always the ServiceNow-backed ProjectService, wired from the same
	// routes.go branch.
	projectSvc ProjectService
}

// NewServiceNowDeployedProductService constructs a DeployedProductService backed by the SN integration service.
func NewServiceNowDeployedProductService(client *integrationservice.Client, deploymentSvc DeploymentService, projectSvc ProjectService) DeployedProductService {
	return &snDeployedProductService{client: client, deploymentSvc: deploymentSvc, projectSvc: projectSvc}
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
		Filters: snDeployedProductFilters{
			DeploymentIDs:     uuidsToSysids(req.DeploymentIDs),
			ProductCategories: req.ProductCategories,
		},
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
			ID:          sysidToUUID(dp.ID),
			Deployment:  domain.EntityRef{ID: sysidToUUID(dp.Deployment.ID), Name: dp.Deployment.Name},
			Description: dp.Description,
			Product: domain.ProductRef{
				ID:           sysidToUUID(dp.Product.ID),
				Name:         dp.Product.Name,
				Abbreviation: dp.Product.Abbreviation,
			},
			Version:   versionRef,
			Cores:     dp.Cores,
			TPS:       dp.TPS,
			Category:  category,
			Updates:   fromSNProductUpdates(dp.Updates),
			CreatedOn: createdOn,
			UpdatedOn: updatedOn,
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

// maxProjectsByProductVersionDeploymentPages bounds the platform-wide
// deployment-enumeration loop in fetchAllDeploymentProjects against a wrong/
// always-true upstream hasMore, mirroring the same safety-bound convention
// sn_project_service.go's fetchAllProjectsFiltered uses for exclude-filter
// pagination.
const maxProjectsByProductVersionDeploymentPages = 200

// maxProjectsByProductVersionDeployedProductPages bounds, per chunk of
// deployment ids, how many pages of deployed products SearchProjectsByProductVersion
// pages through before giving up — same reasoning as the constant above.
const maxProjectsByProductVersionDeployedProductPages = 200

// mandatoryExcludeClosureStates and mandatoryExcludeSubscriptionTypes are the
// fixed exclusions SearchProjectsByProductVersion always applies — not a
// caller-supplied filter. This mirrors the real ServiceNow flow it replaces
// ("DRY RUN - Create [EOL] Product Announcements"), whose own first step
// ("Look Up Customer Project Records") applies this exact same four-condition
// exclusion unconditionally: Project Type is not Cloud Evaluation Support,
// Project Type is not Cloud Support, WSO2 Closure State is not Restricted,
// WSO2 Closure State is not Suspended. That flow gives whoever triggers it no
// way to opt out; this endpoint doesn't either.
//
// fetchEligibleProjectIDs additionally excludes any project whose contract
// has ended (EndDate in the past) — a fifth, unconditional exclusion beyond
// the four the SN flow above checks. WSO2 Closure State is frequently left
// unset (nil) for a project whose subscription simply expired rather than
// being explicitly marked "Suspended"/"Restricted", so closure-state alone
// misses it. The customer portal itself already treats an expired end date
// as equivalent to "Suspended" for access purposes (isProjectSuspended in
// apps/customer-portal/webapp/src/utils/permission.ts) and blocks the
// customer from even viewing the project — an EOL announcement audience
// must not include a project the customer portal itself considers
// inaccessible.
var mandatoryExcludeClosureStates = []string{"Restricted", "Suspended"}
var mandatoryExcludeSubscriptionTypes = []domain.SubscriptionType{
	domain.SubscriptionTypeCloudSupport,
	domain.SubscriptionTypeCloudEvaluationSupport,
}

// SearchProjectsByProductVersion implements DeployedProductService. There is
// no upstream query that goes directly from "product X, version Y" to the
// projects running it — SearchDeployedProducts only accepts DeploymentIDs as
// a filter, which assumes the caller already knows which deployments to look
// at. This resolves the reverse direction itself: page through every
// deployment platform-wide to learn its owning project (fetchAllDeploymentProjects),
// then page through every deployed product on those deployments — in chunks,
// since DeploymentIDs is the only supported filter, reusing the already-tested
// SearchDeployedProducts rather than hand-rolling new wire parsing — and keep
// the ones matching the requested product+version, joining back to the
// project via the first pass. The result is deduplicated by project (a
// project can have several deployments, or several matching deployed
// products on one deployment), then intersected with the mandatory-exclusion
// eligible set above (see fetchEligibleProjectIDs) before the caller's own
// pagination is applied, over the deduplicated, name-sorted set.
func (s *snDeployedProductService) SearchProjectsByProductVersion(ctx context.Context, req domain.SearchProjectsByProductVersionRequest) (domain.SearchProjectsByProductVersionResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}
	if err := validateUUIDs("productId", []string{req.ProductID}); err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}
	if err := validateUUIDs("productVersionId", []string{req.ProductVersionID}); err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}

	deploymentProjects, err := s.fetchAllDeploymentProjects(ctx)
	if err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}
	if len(deploymentProjects) == 0 {
		return domain.SearchProjectsByProductVersionResponse{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}, nil
	}

	deploymentIDs := make([]string, 0, len(deploymentProjects))
	for id := range deploymentProjects {
		deploymentIDs = append(deploymentIDs, id)
	}

	matchedProjects := make(map[string]domain.EntityRef)
	for start := 0; start < len(deploymentIDs); start += maxLimit {
		end := start + maxLimit
		if end > len(deploymentIDs) {
			end = len(deploymentIDs)
		}
		chunk := deploymentIDs[start:end]

		offset := 0
		for page := 0; ; page++ {
			if page == maxProjectsByProductVersionDeployedProductPages {
				return domain.SearchProjectsByProductVersionResponse{}, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf(
					"too many deployed products to resolve product/version matches safely (exceeded %d pages of %d for one deployment batch)",
					maxProjectsByProductVersionDeployedProductPages, maxLimit,
				)}
			}

			resp, err := s.SearchDeployedProducts(ctx, domain.SearchDeployedProductsRequest{
				Pagination:    domain.Pagination{Limit: maxLimit, Offset: offset},
				DeploymentIDs: chunk,
			})
			if err != nil {
				return domain.SearchProjectsByProductVersionResponse{}, err
			}
			for _, dp := range resp.DeployedProducts {
				if dp.Product.ID != req.ProductID {
					continue
				}
				if dp.Version == nil || dp.Version.ID != req.ProductVersionID {
					continue
				}
				if proj, ok := deploymentProjects[dp.Deployment.ID]; ok {
					matchedProjects[proj.ID] = proj
				}
			}
			offset += len(resp.DeployedProducts)
			if !resp.HasMore || len(resp.DeployedProducts) == 0 {
				break
			}
		}
	}

	eligible, err := s.fetchEligibleProjectIDs(ctx, mandatoryExcludeClosureStates, mandatoryExcludeSubscriptionTypes)
	if err != nil {
		return domain.SearchProjectsByProductVersionResponse{}, err
	}
	for id := range matchedProjects {
		if _, ok := eligible[id]; !ok {
			delete(matchedProjects, id)
		}
	}

	sorted := make([]domain.EntityRef, 0, len(matchedProjects))
	for _, p := range matchedProjects {
		sorted = append(sorted, p)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Name == sorted[j].Name {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Name < sorted[j].Name
	})

	total := len(sorted)
	start := req.Pagination.Offset
	if start > total {
		start = total
	}
	end := start + req.Pagination.Limit
	if end > total {
		end = total
	}

	return domain.SearchProjectsByProductVersionResponse{
		Projects: sorted[start:end],
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  end < total,
	}, nil
}

// fetchAllDeploymentProjects pages through every deployment platform-wide
// (no project/type/search filter), returning a map of deployment id ->
// owning project ref. Bounded against a wrong/always-true upstream hasMore
// the same way sn_project_service.go's fetchAllProjectsFiltered is; returns
// an error rather than a silently incomplete map if the bound is hit before
// every deployment is seen — see that function's own reasoning for why
// silent truncation here would be worse than failing loudly: a caller
// resolving an EOL-announcement audience from a partial map could
// under-count real recipients and never know.
func (s *snDeployedProductService) fetchAllDeploymentProjects(ctx context.Context) (map[string]domain.EntityRef, error) {
	result := make(map[string]domain.EntityRef)
	offset := 0
	for page := 0; page < maxProjectsByProductVersionDeploymentPages; page++ {
		resp, err := s.deploymentSvc.SearchDeployments(ctx, domain.SearchDeploymentsRequest{
			Pagination: domain.Pagination{Limit: maxLimit, Offset: offset},
		})
		if err != nil {
			return nil, err
		}
		for _, d := range resp.Deployments {
			result[d.ID] = d.Project
		}
		offset += len(resp.Deployments)
		if !resp.HasMore || len(resp.Deployments) == 0 {
			return result, nil
		}
	}
	return nil, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf(
		"too many deployments to resolve product/version matches safely (exceeded %d pages of %d)",
		maxProjectsByProductVersionDeploymentPages, maxLimit,
	)}
}

// maxProjectsByProductVersionEligiblePages bounds the eligible-project
// enumeration loop in fetchEligibleProjectIDs against a wrong/always-true
// upstream hasMore, same convention as the other bounds in this file.
const maxProjectsByProductVersionEligiblePages = 200

// fetchEligibleProjectIDs pages through every project matching the given
// exclude filters via ProjectService.SearchProjects — the single source of
// truth for this exclusion logic (see SearchProjectsByProductVersion's own
// doc comment for why an EOL announcement's real audience is this set
// intersected with the product-version match, not the product-version match
// alone) — and returns their ids as a set. Bounded and fails loudly rather
// than truncating, same reasoning as fetchAllDeploymentProjects: a partial
// eligible set here would silently exclude projects that are actually
// eligible, under-counting the real audience.
func (s *snDeployedProductService) fetchEligibleProjectIDs(ctx context.Context, excludeClosureStates []string, excludeSubscriptionTypes []domain.SubscriptionType) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	offset := 0
	for page := 0; page < maxProjectsByProductVersionEligiblePages; page++ {
		resp, err := s.projectSvc.SearchProjects(ctx, domain.SearchProjectsRequest{
			Pagination:               domain.Pagination{Limit: maxLimit, Offset: offset},
			ExcludeClosureStates:     excludeClosureStates,
			ExcludeSubscriptionTypes: excludeSubscriptionTypes,
		})
		if err != nil {
			return nil, err
		}
		for _, p := range resp.Projects {
			if isProjectContractEnded(p.EndDate, time.Now()) {
				continue
			}
			result[p.ID] = struct{}{}
		}
		offset += len(resp.Projects)
		if !resp.HasMore || len(resp.Projects) == 0 {
			return result, nil
		}
	}
	return nil, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf(
		"too many projects to resolve eligible-project exclusions safely (exceeded %d pages of %d)",
		maxProjectsByProductVersionEligiblePages, maxLimit,
	)}
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
