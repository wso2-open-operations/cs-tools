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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type productCatalogService struct {
	repo         repository.ProductCatalogRepository
	products     repository.ProductRepository
	versions     repository.ProductVersionRepository
	access       AccessService
	managerRoles []string
}

// NewProductCatalogService constructs a ProductCatalogService. managerRoles is
// AUTH_PRODUCT_CATALOG_MANAGER_ROLES plus AUTH_ADMIN_ROLES: either may edit.
func NewProductCatalogService(repo repository.ProductCatalogRepository, products repository.ProductRepository,
	versions repository.ProductVersionRepository, access AccessService, managerRoles []string) ProductCatalogService {
	return &productCatalogService{repo: repo, products: products, versions: versions, access: access, managerRoles: managerRoles}
}

func (s *productCatalogService) requireReader(ctx context.Context) error {
	return RequireInternalCaller(ctx, s.access, "the product catalogue is only available to internal callers")
}

// requireManager returns the caller's email, the actor of every write.
func (s *productCatalogService) requireManager(ctx context.Context) (string, error) {
	if err := s.requireReader(ctx); err != nil {
		return "", err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" || !holdsAnyRole(id.Roles, s.managerRoles) {
		return "", &apierror.ForbiddenError{Msg: "only a product catalogue manager can change the catalogue"}
	}
	return id.UserEmail, nil
}

// ----- products -----

func (s *productCatalogService) GetProduct(ctx context.Context, id string) (domain.Product, error) {
	if err := s.requireReader(ctx); err != nil {
		return domain.Product{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.Product{}, err
	}
	return s.loadProduct(ctx, id)
}

func (s *productCatalogService) loadProduct(ctx context.Context, id string) (domain.Product, error) {
	p, err := s.products.GetProduct(ctx, id)
	if err != nil {
		return domain.Product{}, err
	}
	if p == nil {
		return domain.Product{}, &apierror.NotFoundError{Msg: "product not found"}
	}
	return *p, nil
}

func (s *productCatalogService) CreateProduct(ctx context.Context, payload json.RawMessage) (domain.Product, error) {
	ids, err := s.write(ctx, domain.CatalogEntityProduct, repository.CatalogOpCreate, "", payload)
	if err != nil {
		return domain.Product{}, err
	}
	return s.loadProduct(ctx, ids[0])
}

func (s *productCatalogService) UpdateProduct(ctx context.Context, id string, payload json.RawMessage) (domain.Product, error) {
	if _, err := s.write(ctx, domain.CatalogEntityProduct, repository.CatalogOpUpdate, id, payload); err != nil {
		return domain.Product{}, err
	}
	return s.loadProduct(ctx, id)
}

// ----- product versions -----

// maxAllVersionsLimit bounds a page of POST /product-versions/search: the
// whole catalogue (about 460 rows) fits in one page.
const maxAllVersionsLimit = 1000

func (s *productCatalogService) SearchAllProductVersions(ctx context.Context, req domain.SearchAllProductVersionsRequest) (domain.SearchProductVersionsResponse, error) {
	if err := s.requireReader(ctx); err != nil {
		return domain.SearchProductVersionsResponse{}, err
	}
	search := req.SearchProductVersionsRequest
	search.ProductID = req.ProductID
	if search.ProductID != "" {
		if err := validateUUIDs("productId", []string{search.ProductID}); err != nil {
			return domain.SearchProductVersionsResponse{}, err
		}
	}
	if err := normalizePaginationMax(&search.Pagination, maxAllVersionsLimit); err != nil {
		return domain.SearchProductVersionsResponse{}, err
	}
	if err := validateVersionSearch(search); err != nil {
		return domain.SearchProductVersionsResponse{}, err
	}
	versions, total, err := s.versions.SearchProductVersions(ctx, search)
	if err != nil {
		return domain.SearchProductVersionsResponse{}, err
	}
	return domain.SearchProductVersionsResponse{
		ProductVersions: versions,
		Total:           total,
		Limit:           search.Pagination.Limit,
		Offset:          search.Pagination.Offset,
		HasMore:         search.Pagination.Offset+len(versions) < total,
	}, nil
}

func (s *productCatalogService) GetProductVersion(ctx context.Context, id string) (domain.ProductVersion, error) {
	if err := s.requireReader(ctx); err != nil {
		return domain.ProductVersion{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ProductVersion{}, err
	}
	rows, err := s.versions.GetProductVersions(ctx, []string{id})
	if err != nil {
		return domain.ProductVersion{}, err
	}
	if len(rows) == 0 {
		return domain.ProductVersion{}, &apierror.NotFoundError{Msg: "product version not found"}
	}
	return rows[0], nil
}

// CreateProductVersions adds a version of a product: one row per deployment
// profile in payload.profiles, all on one public listing.
func (s *productCatalogService) CreateProductVersions(ctx context.Context, productID string, payload json.RawMessage) (domain.ProductVersionList, error) {
	if err := validateUUIDs("productId", []string{productID}); err != nil {
		return domain.ProductVersionList{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(normalizePayload(payload), &raw); err != nil {
		return domain.ProductVersionList{}, &apierror.ValidationError{Msg: "body must be a JSON object"}
	}
	if _, ok := raw["productId"]; ok {
		return domain.ProductVersionList{}, &apierror.ValidationError{Msg: "productId: comes from the path"}
	}
	raw["productId"], _ = json.Marshal(productID)
	body, _ := json.Marshal(raw)
	ids, err := s.write(ctx, domain.CatalogEntityProductVersion, repository.CatalogOpCreate, "", body)
	if err != nil {
		return domain.ProductVersionList{}, err
	}
	return s.loadVersions(ctx, ids)
}

// UpdateProductVersion changes a version. Listing-level fields apply to every
// live row of its public listing; deploymentProfile and publicId to this row.
func (s *productCatalogService) UpdateProductVersion(ctx context.Context, id string, payload json.RawMessage) (domain.ProductVersionList, error) {
	ids, err := s.write(ctx, domain.CatalogEntityProductVersion, repository.CatalogOpUpdate, id, payload)
	if err != nil {
		return domain.ProductVersionList{}, err
	}
	return s.loadVersions(ctx, ids)
}

// RetireProductVersion retires a version: every live row of its public
// listing, or the one row when it has none.
func (s *productCatalogService) RetireProductVersion(ctx context.Context, id string) (domain.ProductVersionList, error) {
	ids, err := s.write(ctx, domain.CatalogEntityProductVersion, repository.CatalogOpRetire, id, nil)
	if err != nil {
		return domain.ProductVersionList{}, err
	}
	return s.loadVersions(ctx, ids)
}

func (s *productCatalogService) loadVersions(ctx context.Context, ids []string) (domain.ProductVersionList, error) {
	rows, err := s.versions.GetProductVersions(ctx, ids)
	if err != nil {
		return domain.ProductVersionList{}, err
	}
	return domain.ProductVersionList{ProductVersions: rows}, nil
}

// ----- release lines -----

func (s *productCatalogService) ListReleaseLines(ctx context.Context, includeRetired bool) (domain.ReleaseLineList, error) {
	if err := s.requireReader(ctx); err != nil {
		return domain.ReleaseLineList{}, err
	}
	rows, err := s.repo.ListReleaseLines(ctx, includeRetired)
	if err != nil {
		return domain.ReleaseLineList{}, err
	}
	return domain.ReleaseLineList{ReleaseLines: rows}, nil
}

func (s *productCatalogService) CreateReleaseLine(ctx context.Context, payload json.RawMessage) (domain.ReleaseLine, error) {
	ids, err := s.write(ctx, domain.CatalogEntityReleaseLine, repository.CatalogOpCreate, "", payload)
	if err != nil {
		return domain.ReleaseLine{}, err
	}
	return s.loadReleaseLine(ctx, ids[0])
}

func (s *productCatalogService) UpdateReleaseLine(ctx context.Context, id string, payload json.RawMessage) (domain.ReleaseLine, error) {
	if _, err := s.write(ctx, domain.CatalogEntityReleaseLine, repository.CatalogOpUpdate, id, payload); err != nil {
		return domain.ReleaseLine{}, err
	}
	return s.loadReleaseLine(ctx, id)
}

func (s *productCatalogService) RetireReleaseLine(ctx context.Context, id string) (domain.ReleaseLine, error) {
	if _, err := s.write(ctx, domain.CatalogEntityReleaseLine, repository.CatalogOpRetire, id, nil); err != nil {
		return domain.ReleaseLine{}, err
	}
	return s.loadReleaseLine(ctx, id)
}

func (s *productCatalogService) loadReleaseLine(ctx context.Context, id string) (domain.ReleaseLine, error) {
	l, err := s.repo.GetReleaseLine(ctx, id)
	if err != nil {
		return domain.ReleaseLine{}, err
	}
	if l == nil {
		return domain.ReleaseLine{}, &apierror.NotFoundError{Msg: "release line not found"}
	}
	return *l, nil
}

// ----- reference data -----

func (s *productCatalogService) ListCarbonPlatformHistory(ctx context.Context) (domain.CarbonPlatformHistoryList, error) {
	if err := s.requireReader(ctx); err != nil {
		return domain.CarbonPlatformHistoryList{}, err
	}
	rows, err := s.repo.ListCarbonPlatformHistory(ctx)
	if err != nil {
		return domain.CarbonPlatformHistoryList{}, err
	}
	return domain.CarbonPlatformHistoryList{Items: rows}, nil
}

func (s *productCatalogService) ListCarbonKernelReleases(ctx context.Context) (domain.CarbonKernelReleaseList, error) {
	if err := s.requireReader(ctx); err != nil {
		return domain.CarbonKernelReleaseList{}, err
	}
	rows, err := s.repo.ListCarbonKernelReleases(ctx)
	if err != nil {
		return domain.CarbonKernelReleaseList{}, err
	}
	return domain.CarbonKernelReleaseList{Items: rows}, nil
}

// ----- writes -----

// write validates one change and applies it at once. The rules that span
// fields (date order, an abbreviation when public, joining a listing) are
// checked by the repository inside the write transaction, on the rows as
// written. It returns the ids of
// the rows written; updated_by / retired_by name the person.
func (s *productCatalogService) write(ctx context.Context, entityType, operation, entityID string, payload json.RawMessage) ([]string, error) {
	actor, err := s.requireManager(ctx)
	if err != nil {
		return nil, err
	}
	apply, err := prepareCatalogChange(entityType, operation, entityID, payload)
	if err != nil {
		return nil, err
	}
	if operation != repository.CatalogOpCreate {
		exists, err := s.repo.EntityExists(ctx, entityType, entityID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, &apierror.NotFoundError{Msg: strings.ReplaceAll(entityType, "_", " ") + " not found"}
		}
	}
	return s.repo.ApplyChange(ctx, actor, apply)
}

// validateVersionSearch checks the filters both version searches share.
func validateVersionSearch(req domain.SearchProductVersionsRequest) error {
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return err
	}
	if req.DeploymentProfile != nil && utf8.RuneCountInString(*req.DeploymentProfile) > maxDeploymentProfileLen {
		return &apierror.ValidationError{Msg: "deploymentProfile cannot exceed 255 characters"}
	}
	switch req.Feed {
	case "", domain.CatalogFeedSupportMatrix, domain.CatalogFeedReleaseMatrix:
	default:
		return &apierror.ValidationError{Msg: "feed must be support-matrix or release-matrix"}
	}
	return nil
}

// normalizePaginationMax is normalizePagination with a route-specific cap.
func normalizePaginationMax(p *domain.Pagination, limitCap int) error {
	if p.Limit <= 0 {
		p.Limit = defaultLimit
	}
	if p.Limit > limitCap {
		return &apierror.ValidationError{Msg: fmt.Sprintf("limit cannot exceed %d", limitCap)}
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return nil
}

func normalizePayload(p json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(p)) == 0 || bytes.Equal(bytes.TrimSpace(p), []byte("null")) {
		return json.RawMessage(`{}`)
	}
	return p
}

// Field kinds a change payload key can hold.
const (
	kindString = iota
	kindDate
	kindBool
	kindInt
	kindUUID
	kindTags
	kindEnum
)

// catalogField is one settable payload key. column and cast are constants
// interpolated into SQL (see repository.CatalogSet).
type catalogField struct {
	column string
	cast   string
	kind   int
	// maxLen bounds a string; 0 means no bound.
	maxLen int
	// enum lists the accepted lowercase values of a kindEnum field.
	enum []string
	// notNull rejects an explicit null.
	notNull bool
	// createOnly keys are accepted only on CREATE; required ones must be set there.
	createOnly bool
	required   bool
	// rowOnly keys of a product version UPDATE change the one row; every
	// other key changes all live rows of its public listing.
	rowOnly bool
}

var (
	supportStatuses = []string{"available", "extended", "deprecated", "discontinued"}

	productVersionFields = map[string]catalogField{
		"productId":                      {column: "product_id", cast: "%s::uuid", kind: kindUUID, notNull: true, createOnly: true, required: true},
		"version":                        {column: "version", cast: "%s", kind: kindString, maxLen: 20, notNull: true, createOnly: true, required: true},
		"deploymentProfile":              {column: "deployment_profile", cast: "%s", kind: kindString, maxLen: 255, rowOnly: true},
		"publicId":                       {column: "public_id", cast: "%s::bigint", kind: kindInt, rowOnly: true},
		"currentSupportStatus":           {column: "current_support_status", cast: "upper(%s)::product_version_support_status_enum", kind: kindEnum, enum: supportStatuses},
		"releaseDate":                    {column: "release_date", cast: "%s::date", kind: kindDate},
		"earliestPossibleSupportEolDate": {column: "earliest_possible_support_eol_date", cast: "%s::date", kind: kindDate},
		"supportEolDate":                 {column: "support_eol_date", cast: "%s::date", kind: kindDate},
		"supportEolAnnouncementDate":     {column: "support_eol_announcement_date", cast: "%s::date", kind: kindDate},
		"publicName":                     {column: "public_name", cast: "%s", kind: kindString, maxLen: 150},
		"publicAbbreviation":             {column: "public_abbreviation", cast: "%s", kind: kindString, maxLen: 64},
		"description":                    {column: "description", cast: "%s", kind: kindString, maxLen: 2000},
		"releaseLinks":                   {column: "release_links", cast: "%s", kind: kindString, maxLen: 255},
		"isPublic":                       {column: "is_public", cast: "%s::boolean", kind: kindBool, notNull: true},
		"showInSupportMatrix":            {column: "show_in_support_matrix", cast: "%s::boolean", kind: kindBool, notNull: true},
		"tags":                           {column: "tags", cast: "%s::text[]", kind: kindTags},
		"legacyStatus":                   {column: "legacy_status", cast: "%s::boolean", kind: kindBool, notNull: true},
		"releaseLineId":                  {column: "release_line_id", cast: "%s::uuid", kind: kindUUID},
	}

	releaseLineFields = map[string]catalogField{
		"name":      {column: "name", cast: "%s", kind: kindString, maxLen: 100, notNull: true, required: true},
		"startDate": {column: "start_date", cast: "%s::date", kind: kindDate},
		"status":    {column: "status", cast: "%s", kind: kindString, maxLen: 50},
	}

	productFields = map[string]catalogField{
		"name":         {column: "name", cast: "%s", kind: kindString, maxLen: 100, notNull: true, required: true},
		"class":        {column: "category", cast: "upper(%s)::product_category_enum", kind: kindEnum, enum: []string{"software", "service"}, notNull: true, required: true},
		"code":         {column: "code", cast: "%s", kind: kindString, maxLen: 64},
		"unit":         {column: "unit", cast: "upper(%s)::product_unit_enum", kind: kindEnum, enum: []string{"integration", "iot", "iam", "choreo", "asgardeo", "apim", "analytics", "devant", "bijira"}},
		"businessUnit": {column: "business_unit", cast: "upper(%s)::product_business_unit_enum", kind: kindEnum, enum: []string{"integration_software", "integration_cloud", "iam", "corporate"}},
		"manufacturer": {column: "manufacturer", cast: "upper(%s)::product_manufacturer_enum", kind: kindEnum, enum: []string{"wso2", "wso2_cloud"}},
		"description":  {column: "description", cast: "%s", kind: kindString, maxLen: 2000},
	}
)

// prepareCatalogChange validates a write and turns its body into column
// assignments.
func prepareCatalogChange(entityType, operation, entityID string, payload json.RawMessage) (repository.CatalogApply, error) {
	a := repository.CatalogApply{EntityType: entityType, Operation: operation, EntityID: entityID}
	isVersion := entityType == domain.CatalogEntityProductVersion

	var fields map[string]catalogField
	switch entityType {
	case domain.CatalogEntityProductVersion:
		fields = productVersionFields
	case domain.CatalogEntityReleaseLine:
		fields = releaseLineFields
	case domain.CatalogEntityProduct:
		fields = productFields
	default:
		return a, &apierror.ValidationError{Msg: "entityType must be product, product_version or release_line"}
	}

	switch operation {
	case repository.CatalogOpCreate:
	case repository.CatalogOpUpdate, repository.CatalogOpRetire:
		if err := validateUUIDs("id", []string{entityID}); err != nil {
			return a, err
		}
		if operation == repository.CatalogOpRetire && entityType == domain.CatalogEntityProduct {
			return a, &apierror.ValidationError{Msg: "a product cannot be retired; retire its versions"}
		}
	default:
		return a, fmt.Errorf("unknown catalog operation %q", operation)
	}
	if operation == repository.CatalogOpRetire {
		return a, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(normalizePayload(payload), &raw); err != nil {
		return a, &apierror.ValidationError{Msg: "body must be a JSON object"}
	}
	if len(raw) == 0 && operation == repository.CatalogOpUpdate {
		return a, &apierror.ValidationError{Msg: "body sets no field"}
	}
	if isVersion && operation == repository.CatalogOpCreate {
		profiles, err := parseCatalogProfiles(raw["profiles"])
		if err != nil {
			return a, err
		}
		a.Profiles = profiles
		delete(raw, "profiles")
		if _, ok := raw["deploymentProfile"]; ok {
			return a, &apierror.ValidationError{Msg: "deploymentProfile: use profiles when adding a version"}
		}
	}

	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := raw[key]
		isNull := bytes.Equal(bytes.TrimSpace(value), []byte("null"))

		if isVersion && key == "carbon" {
			carbon, err := parseCatalogCarbon(value, isNull)
			if err != nil {
				return a, err
			}
			a.CarbonSet, a.Carbon = true, carbon
			continue
		}

		f, ok := fields[key]
		if !ok {
			return a, &apierror.ValidationError{Msg: fmt.Sprintf("%s: unknown field for a %s", key, strings.ReplaceAll(entityType, "_", " "))}
		}
		if f.createOnly && operation != repository.CatalogOpCreate {
			return a, &apierror.ValidationError{Msg: key + ": can only be set when creating"}
		}
		v, err := parseCatalogValue(key, f, value, isNull)
		if err != nil {
			return a, err
		}
		set := repository.CatalogSet{Column: f.column, Cast: f.cast, Value: v}
		if f.rowOnly {
			a.RowSets = append(a.RowSets, set)
		} else {
			a.Sets = append(a.Sets, set)
		}
	}

	if operation == repository.CatalogOpCreate {
		for key, f := range fields {
			if !f.required {
				continue
			}
			if _, ok := raw[key]; !ok {
				return a, &apierror.ValidationError{Msg: key + ": is required"}
			}
		}
	}
	return a, nil
}

// parseCatalogValue checks one payload value against its field and returns
// what to bind: a *string, *bool, *int64 or []string (nil pointer for null).
func parseCatalogValue(key string, f catalogField, value json.RawMessage, isNull bool) (any, error) {
	if isNull {
		if f.notNull || f.required {
			return nil, &apierror.ValidationError{Msg: key + ": cannot be null"}
		}
		switch f.kind {
		case kindBool:
			return (*bool)(nil), nil
		case kindInt:
			return (*int64)(nil), nil
		case kindTags:
			return []string{}, nil
		default:
			return (*string)(nil), nil
		}
	}
	switch f.kind {
	case kindBool:
		var b bool
		if err := json.Unmarshal(value, &b); err != nil {
			return nil, &apierror.ValidationError{Msg: key + ": must be true or false"}
		}
		return &b, nil
	case kindInt:
		var n int64
		if err := json.Unmarshal(value, &n); err != nil || n <= 0 {
			return nil, &apierror.ValidationError{Msg: key + ": must be a positive integer"}
		}
		return &n, nil
	case kindTags:
		var tags []string
		if err := json.Unmarshal(value, &tags); err != nil {
			return nil, &apierror.ValidationError{Msg: key + ": must be an array of strings"}
		}
		clean := make([]string, 0, len(tags))
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t == "" || utf8.RuneCountInString(t) > 50 {
				return nil, &apierror.ValidationError{Msg: key + ": each tag must be 1-50 characters"}
			}
			clean = append(clean, t)
		}
		return clean, nil
	}

	var str string
	if err := json.Unmarshal(value, &str); err != nil {
		return nil, &apierror.ValidationError{Msg: key + ": must be a string"}
	}
	str = strings.TrimSpace(str)
	switch f.kind {
	case kindDate:
		if _, err := time.Parse("2006-01-02", str); err != nil {
			return nil, &apierror.ValidationError{Msg: key + ": must be a date (YYYY-MM-DD)"}
		}
	case kindUUID:
		if err := validateUUIDs(key, []string{str}); err != nil {
			return nil, err
		}
	case kindEnum:
		str = strings.ToLower(str)
		valid := false
		for _, e := range f.enum {
			if e == str {
				valid = true
				break
			}
		}
		if !valid {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("%s: must be one of %s", key, strings.Join(f.enum, ", "))}
		}
	case kindString:
		if str == "" {
			if f.notNull || f.required {
				return nil, &apierror.ValidationError{Msg: key + ": cannot be empty"}
			}
			return (*string)(nil), nil
		}
		if f.maxLen > 0 && utf8.RuneCountInString(str) > f.maxLen {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("%s: must be at most %d characters", key, f.maxLen)}
		}
	}
	return &str, nil
}

// parseCatalogProfiles reads the profiles of a version CREATE: at least one,
// trimmed, no repeats; "" is a row with no deployment profile.
func parseCatalogProfiles(value json.RawMessage) ([]string, error) {
	var profiles []string
	if len(value) == 0 || json.Unmarshal(value, &profiles) != nil || len(profiles) == 0 {
		return nil, &apierror.ValidationError{Msg: "profiles: must be a non-empty array of deployment profiles (\"\" for none)"}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(profiles))
	for _, p := range profiles {
		p = strings.TrimSpace(p)
		if utf8.RuneCountInString(p) > maxDeploymentProfileLen {
			return nil, &apierror.ValidationError{Msg: "profiles: each must be at most 255 characters"}
		}
		if seen[strings.ToLower(p)] {
			return nil, &apierror.ValidationError{Msg: "profiles: " + strconv.Quote(p) + " is listed twice"}
		}
		seen[strings.ToLower(p)] = true
		out = append(out, p)
	}
	return out, nil
}

func parseCatalogCarbon(value json.RawMessage, isNull bool) (*domain.CatalogCarbon, error) {
	if isNull {
		return nil, nil
	}
	var raw map[string]*string
	if err := json.Unmarshal(value, &raw); err != nil {
		return nil, &apierror.ValidationError{Msg: "carbon: must be an object of strings or null"}
	}
	var c domain.CatalogCarbon
	targets := map[string]**string{
		"platformRelease":     &c.PlatformRelease,
		"platformVersion":     &c.PlatformVersion,
		"carbonKernelVersion": &c.CarbonKernelVersion,
		"releaseChunk":        &c.ReleaseChunk,
		"kernelPatchNo":       &c.KernelPatchNo,
	}
	for key, v := range raw {
		dst, ok := targets[key]
		if !ok {
			return nil, &apierror.ValidationError{Msg: "carbon." + key + ": unknown field"}
		}
		if v != nil {
			t := strings.TrimSpace(*v)
			if utf8.RuneCountInString(t) > 50 {
				return nil, &apierror.ValidationError{Msg: "carbon." + key + ": must be at most 50 characters"}
			}
			if t != "" {
				*dst = &t
			}
		}
	}
	return &c, nil
}
