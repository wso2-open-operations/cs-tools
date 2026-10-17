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
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Limits of the ServiceNow-shaped API this file reproduces, versus entity-service's.
const (
	catalogDefaultLimit = 10
	catalogMaxLimit     = 100
	// entityMaxPageSize is entity-service's page cap; a larger caller limit is
	// served by several upstream calls.
	entityMaxPageSize = 50
	maxSfIDs          = 100
	// maxCatalogScan bounds how many products or versions are read to order them before paging.
	maxCatalogScan = 5000
	// maxSfScopedDeployments bounds the deployments gathered to scope a deployed-products search by sfIds.
	maxSfScopedDeployments = 5000
)

const (
	errMsgInvalidLimit  = "Invalid limit. It must be a positive integer."
	errMsgInvalidOffset = "Invalid offset. It must be a non-negative integer."
	errMsgTooManySfIDs  = "Too many sfIds."
)

// entityCatalogClient abstracts the entity service operations used by CatalogHandler.
type entityCatalogClient interface {
	SearchProducts(ctx context.Context, body []byte) ([]byte, error)
	SearchProductVersions(ctx context.Context, productID string, body []byte) ([]byte, error)
	SearchDeployments(ctx context.Context, body []byte) ([]byte, error)
	SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error)
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
}

// CatalogHandler serves the product, product-version, deployment and
// deployed-product reads in the response shape of the ServiceNow scripted REST
// API ("SF WSO2 Product & Deployment API") that third-party consumers already
// integrate against, built from entity-service's responses.
//
// Unlike most handlers here this one reshapes upstream bodies, on purpose: it
// is a compatibility facade (see CloudStatusHandler for the other one), so
// entity-service stays a clean internal API. Fields entity-service cannot supply
// are returned as null (deployment.version, productVersion.abbreviation,
// productVersion.product.name) or empty (products.warnings).
type CatalogHandler struct {
	entity entityCatalogClient
}

// NewCatalogHandler creates a CatalogHandler backed by the given entity client.
func NewCatalogHandler(entity entityCatalogClient) *CatalogHandler {
	return &CatalogHandler{entity: entity}
}

// ----- shared helpers -----

// parseLimitOffset reads ?limit (default 10, 1..100) and ?offset (default 0, >= 0).
func parseLimitOffset(r *http.Request) (limit, offset int, msg string) {
	limit, offset = catalogDefaultLimit, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > catalogMaxLimit {
			return 0, 0, errMsgInvalidLimit
		}
		limit = n
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return 0, 0, errMsgInvalidOffset
		}
		offset = n
	}
	return limit, offset, ""
}

// parseSfIDs splits ?sfIds into a de-duplicated list. present is false when the
// parameter was absent or blank, meaning "no project filter".
func parseSfIDs(r *http.Request) (ids []string, present bool, msg string) {
	raw := r.URL.Query().Get("sfIds")
	seen := map[string]struct{}{}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		ids = append(ids, p)
	}
	if len(ids) > maxSfIDs {
		return nil, false, errMsgTooManySfIDs
	}
	return ids, len(ids) > 0, ""
}

// page is one upstream result page.
type page[T any] struct {
	items   []T
	total   int
	hasMore bool
}

// fetchWindow returns limit items starting at offset, issuing as many
// entity-service calls as its page cap requires. total is the upstream total.
func fetchWindow[T any](limit, offset int, fetch func(limit, offset int) (page[T], error)) ([]T, int, error) {
	items := []T{}
	total := 0
	for len(items) < limit {
		n := min(entityMaxPageSize, limit-len(items))
		p, err := fetch(n, offset+len(items))
		if err != nil {
			return nil, 0, err
		}
		total = p.total
		items = append(items, p.items...)
		if !p.hasMore || len(p.items) == 0 {
			break
		}
	}
	return items, total, nil
}

// dateTimeLayout is the "YYYY-MM-DD HH:MM:SS" layout the API returns for createdOn/updatedOn.
const dateTimeLayout = "2006-01-02 15:04:05"

// formatDateTime re-renders an entity-service RFC 3339 timestamp in dateTimeLayout (UTC).
// A value that doesn't parse is returned unchanged.
func formatDateTime(v *string) *string {
	if v == nil {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, *v)
	if err != nil {
		return v
	}
	out := t.UTC().Format(dateTimeLayout)
	return &out
}

// formatDate reduces an entity-service timestamp to the date-only "YYYY-MM-DD"
// used for release / end-of-life dates.
func formatDate(v *string) *string {
	if v == nil {
		return nil
	}
	if t, err := time.Parse(time.RFC3339Nano, *v); err == nil {
		out := t.UTC().Format(time.DateOnly)
		return &out
	}
	if len(*v) >= len(time.DateOnly) {
		out := (*v)[:len(time.DateOnly)]
		return &out
	}
	return v
}

// truncToInt cuts a decimal toward zero; an absent or null value stays null.
func truncToInt(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("null")
	}
	f, err := strconv.ParseFloat(string(raw), 64)
	if err != nil {
		return json.RawMessage("null")
	}
	return json.RawMessage(strconv.FormatInt(int64(f), 10))
}

func writeEnvelope(w http.ResponseWriter, data any) {
	body, err := json.Marshal(map[string]any{"success": true, "data": data})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *CatalogHandler) call(ctx context.Context, fn func() ([]byte, error), out any) error {
	raw, err := fn()
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// ----- sfIds -> project ids -----

type entityProjectsPage struct {
	Projects []struct {
		ID string `json:"id"`
	} `json:"projects"`
	HasMore bool `json:"hasMore"`
}

// resolveProjectIDs maps Salesforce ids to project UUIDs using entity-service's
// sfIds project filter; ids matching no project are simply absent from the result.
// It pages until entity-service reports no more, since one id can match several
// projects (bounded by maxCatalogScan).
func (h *CatalogHandler) resolveProjectIDs(ctx context.Context, sfIDs []string) ([]string, error) {
	ids, _, err := fetchWindow(maxCatalogScan, 0, func(n, o int) (page[string], error) {
		body, _ := json.Marshal(map[string]any{
			"sfIds":      sfIDs,
			"pagination": map[string]int{"limit": n, "offset": o},
		})
		var p entityProjectsPage
		if err := h.call(ctx, func() ([]byte, error) { return h.entity.SearchProjects(ctx, body) }, &p); err != nil {
			return page[string]{}, err
		}
		out := make([]string, 0, len(p.Projects))
		for _, pr := range p.Projects {
			out = append(out, pr.ID)
		}
		return page[string]{items: out, hasMore: p.HasMore}, nil
	})
	return ids, err
}

// ----- GET /products/search -----

type entityProductsPage struct {
	Products []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Class string `json:"class"`
	} `json:"products"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

type productView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

// productClasses maps the ?class filter to entity-service's product category
// ("" = every product) and to the label returned in each product's class.
var productClasses = map[string]struct{ category, label string }{
	"product_model":  {"", "Product Model"},
	"software_model": {"software", "Software Model"},
	"service_model":  {"service", "Service Model"},
}

// productClassLabels maps entity-service's product category to the class label.
var productClassLabels = map[string]string{
	"software": productClasses["software_model"].label,
	"service":  productClasses["service_model"].label,
}

// fetchAllProducts reads every product matching class (an entity-service
// category, or "" for all) and returns them ordered by name, which entity-service
// cannot do itself.
func (h *CatalogHandler) fetchAllProducts(ctx context.Context, category string) ([]productView, error) {
	items, _, err := fetchWindow(maxCatalogScan, 0, func(n, o int) (page[productView], error) {
		req := map[string]any{"pagination": map[string]int{"limit": n, "offset": o}}
		if category != "" {
			req["class"] = category
		}
		body, _ := json.Marshal(req)
		var p entityProductsPage
		if err := h.call(ctx, func() ([]byte, error) { return h.entity.SearchProducts(ctx, body) }, &p); err != nil {
			return page[productView]{}, err
		}
		out := make([]productView, 0, len(p.Products))
		for _, x := range p.Products {
			label, ok := productClassLabels[x.Class]
			if !ok {
				label = x.Class
			}
			out = append(out, productView{ID: x.ID, Name: x.Name, Class: label})
		}
		return page[productView]{items: out, total: p.Total, hasMore: p.HasMore}, nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, nil
}

// window returns the offset/limit slice of items; an offset past the end gives an empty slice.
func window[T any](items []T, limit, offset int) []T {
	if offset >= len(items) {
		return []T{}
	}
	return items[offset:min(offset+limit, len(items))]
}

// SearchProducts handles GET /products/search. An unrecognised ?class is ignored
// (no class filter), as it always was.
func (h *CatalogHandler) SearchProducts(w http.ResponseWriter, r *http.Request) {
	limit, offset, msg := parseLimitOffset(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	category := productClasses[strings.TrimSpace(r.URL.Query().Get("class"))].category

	all, err := h.fetchAllProducts(r.Context(), category)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProducts failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search products.")
		return
	}

	writeEnvelope(w, map[string]any{
		"products": window(all, limit, offset), "total": len(all), "offset": offset, "limit": limit,
		"warnings": []string{},
	})
}

// ----- GET /products/{id}/versions/search -----

type entityVersionsPage struct {
	ProductVersions []struct {
		ID                             string  `json:"id"`
		Version                        *string `json:"version"`
		CurrentSupportStatus           *string `json:"currentSupportStatus"`
		ReleaseDate                    *string `json:"releaseDate"`
		SupportEOLDate                 *string `json:"supportEolDate"`
		EarliestPossibleSupportEOLDate *string `json:"earliestPossibleSupportEolDate"`
	} `json:"productVersions"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

type productRefView struct {
	ID   string  `json:"id"`
	Name *string `json:"name"`
}

type productVersionView struct {
	ID                             string         `json:"id"`
	Version                        *string        `json:"version"`
	CurrentSupportStatus           *string        `json:"currentSupportStatus"`
	ReleaseDate                    *string        `json:"releaseDate"`
	SupportEOLDate                 *string        `json:"supportEolDate"`
	EarliestPossibleSupportEOLDate *string        `json:"earliestPossibleSupportEolDate"`
	Abbreviation                   *string        `json:"abbreviation"`
	Product                        productRefView `json:"product"`
}

// compareVersionsDesc orders dotted version strings newest first, comparing
// each part numerically (a non-numeric or missing part counts as 0).
func compareVersionsDesc(a, b string) bool {
	pa, pb := versionParts(a), versionParts(b)
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func versionParts(v string) []int {
	if v == "" {
		v = "0.0.0"
	}
	parts := strings.Split(v, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		out[i], _ = strconv.Atoi(leadingDigits(p))
	}
	return out
}

// leadingDigits mirrors parseInt: the numeric prefix of s, or "" when there is none.
func leadingDigits(s string) string {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i]
}

// SearchProductVersions handles GET /products/{id}/versions/search. Versions are
// ordered newest first across the whole product and then windowed, so every
// version is read before paging.
func (h *CatalogHandler) SearchProductVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	limit, offset, msg := parseLimitOffset(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	all, total, err := fetchWindow(maxCatalogScan, 0, func(n, o int) (page[productVersionView], error) {
		body, _ := json.Marshal(map[string]any{"pagination": map[string]int{"limit": n, "offset": o}})
		var p entityVersionsPage
		if err := h.call(r.Context(), func() ([]byte, error) { return h.entity.SearchProductVersions(r.Context(), id, body) }, &p); err != nil {
			return page[productVersionView]{}, err
		}
		out := make([]productVersionView, 0, len(p.ProductVersions))
		for _, v := range p.ProductVersions {
			out = append(out, productVersionView{
				ID: v.ID, Version: v.Version, CurrentSupportStatus: v.CurrentSupportStatus,
				ReleaseDate: formatDate(v.ReleaseDate), SupportEOLDate: formatDate(v.SupportEOLDate),
				EarliestPossibleSupportEOLDate: formatDate(v.EarliestPossibleSupportEOLDate),
				Product:                        productRefView{ID: id},
			})
		}
		return page[productVersionView]{items: out, total: p.Total, hasMore: p.HasMore}, nil
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProductVersions failed", "productID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search product versions.")
		return
	}

	// The product name is a convenience lookup: if it fails or finds no match the
	// versions are still returned, with product.name left null.
	if len(all) > 0 {
		products, perr := h.fetchAllProducts(r.Context(), "")
		if perr != nil {
			slog.WarnContext(r.Context(), "entity SearchProducts failed, product name omitted", "productID", id, "err", summarizeErr(perr))
		}
		for _, p := range products {
			if p.ID == id {
				name := p.Name
				for i := range all {
					all[i].Product.Name = &name
				}
				break
			}
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		return compareVersionsDesc(deref(all[i].Version), deref(all[j].Version))
	})
	writeEnvelope(w, map[string]any{"versions": window(all, limit, offset), "total": total, "offset": offset, "limit": limit})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ----- GET /deployments/search -----

type entityDeploymentsPage struct {
	Deployments []struct {
		ID          string  `json:"id"`
		Number      *string `json:"number"`
		Name        *string `json:"name"`
		Type        string  `json:"type"`
		Description *string `json:"description"`
		URL         *string `json:"url"`
		CreatedOn   *string `json:"createdOn"`
		UpdatedOn   *string `json:"updatedOn"`
		Project     *struct {
			ID   string  `json:"id"`
			Name *string `json:"name"`
		} `json:"project"`
	} `json:"deployments"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

type deploymentTypeView struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type deploymentView struct {
	ID          string              `json:"id"`
	Number      *string             `json:"number"`
	Name        *string             `json:"name"`
	Description *string             `json:"description"`
	URL         *string             `json:"url"`
	Version     *string             `json:"version"`
	CreatedOn   *string             `json:"createdOn"`
	UpdatedOn   *string             `json:"updatedOn"`
	Project     *productRefView     `json:"project"`
	Type        *deploymentTypeView `json:"type"`
}

// deploymentTypes maps entity-service's type enum to ServiceNow's type key and label.
var deploymentTypes = map[string]deploymentTypeView{
	"development":        {1, "Development"},
	"qa":                 {2, "QA"},
	"staging":            {3, "Staging"},
	"stress":             {4, "Stress"},
	"uat":                {5, "UAT"},
	"primary_production": {6, "Primary Production"},
}

// fetchDeployments pages through entity-service deployments. includeInactive
// is for scoping only: the deployments listing hides inactive ones, but a
// deployed product's own scope (and its deployment type) must not.
func (h *CatalogHandler) fetchDeployments(ctx context.Context, projectIDs []string, includeInactive bool, limit, offset int) ([]deploymentView, int, error) {
	return fetchWindow(limit, offset, func(n, o int) (page[deploymentView], error) {
		req := map[string]any{"pagination": map[string]int{"limit": n, "offset": o}}
		if projectIDs != nil {
			req["projectIds"] = projectIDs
		}
		if includeInactive {
			req["includeInactive"] = true
		}
		body, _ := json.Marshal(req)
		var p entityDeploymentsPage
		if err := h.call(ctx, func() ([]byte, error) { return h.entity.SearchDeployments(ctx, body) }, &p); err != nil {
			return page[deploymentView]{}, err
		}
		out := make([]deploymentView, 0, len(p.Deployments))
		for _, d := range p.Deployments {
			v := deploymentView{
				ID: d.ID, Number: d.Number, Name: d.Name, Description: d.Description, URL: d.URL,
				CreatedOn: formatDateTime(d.CreatedOn), UpdatedOn: formatDateTime(d.UpdatedOn),
			}
			if d.Project != nil {
				v.Project = &productRefView{ID: d.Project.ID, Name: d.Project.Name}
			}
			if t, ok := deploymentTypes[d.Type]; ok {
				v.Type = &t
			}
			out = append(out, v)
		}
		return page[deploymentView]{items: out, total: p.Total, hasMore: p.HasMore}, nil
	})
}

func writeEmptyList(w http.ResponseWriter, key string, totalKey string, limit, offset int) {
	writeEnvelope(w, map[string]any{key: []any{}, totalKey: 0, "offset": offset, "limit": limit})
}

// SearchDeployments handles GET /deployments/search.
func (h *CatalogHandler) SearchDeployments(w http.ResponseWriter, r *http.Request) {
	limit, offset, msg := parseLimitOffset(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	sfIDs, filtered, msg := parseSfIDs(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	var projectIDs []string
	if filtered {
		var err error
		if projectIDs, err = h.resolveProjectIDs(r.Context(), sfIDs); err != nil {
			slog.ErrorContext(r.Context(), "entity SearchProjects failed", "err", summarizeErr(err))
			mapUpstreamError(w, err, "Failed to search deployments.")
			return
		}
		if len(projectIDs) == 0 {
			writeEmptyList(w, "deployments", "totalRecords", limit, offset)
			return
		}
	}

	items, total, err := h.fetchDeployments(r.Context(), projectIDs, false, limit, offset)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployments failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search deployments.")
		return
	}
	writeEnvelope(w, map[string]any{"deployments": items, "totalRecords": total, "offset": offset, "limit": limit})
}

// ----- GET /deployed-products/search -----

type entityDeployedProduct struct {
	ID         string `json:"id"`
	Deployment *struct {
		ID   string  `json:"id"`
		Name *string `json:"name"`
	} `json:"deployment"`
	Product *struct {
		ID           string  `json:"id"`
		Name         *string `json:"name"`
		Abbreviation *string `json:"abbreviation"`
	} `json:"product"`
	Version *struct {
		ID           string  `json:"id"`
		Name         *string `json:"name"`
		ReleasedDate *string `json:"releasedDate"`
		SupportEoL   *string `json:"supportEoLDate"`
	} `json:"version"`
	Cores       json.RawMessage `json:"cores"`
	TPS         json.RawMessage `json:"tps"`
	Description *string         `json:"description"`
	Updates     []struct {
		UpdateLevel json.Number `json:"updateLevel"`
		Date        *string     `json:"date"`
		Details     *string     `json:"details"`
	} `json:"updates"`
	CreatedOn *string `json:"createdOn"`
	UpdatedOn *string `json:"updatedOn"`
}

type entityDeployedProductsPage struct {
	DeployedProducts []entityDeployedProduct `json:"deployedProducts"`
	Total            int                     `json:"total"`
	HasMore          bool                    `json:"hasMore"`
}

// deploymentTypeLabels resolves the type label of each distinct deployment on a
// page of deployed products (entity-service's deployed-product rows carry only
// the deployment's id and name). A deployment with no known type is absent.
func (h *CatalogHandler) deploymentTypeLabels(ctx context.Context, dps []entityDeployedProduct) (map[string]string, error) {
	seen := map[string]struct{}{}
	var ids []string
	for _, d := range dps {
		if d.Deployment == nil {
			continue
		}
		if _, dup := seen[d.Deployment.ID]; dup {
			continue
		}
		seen[d.Deployment.ID] = struct{}{}
		ids = append(ids, d.Deployment.ID)
	}
	labels := map[string]string{}
	if len(ids) == 0 {
		return labels, nil
	}
	body, _ := json.Marshal(map[string]any{
		"ids":             ids,
		"includeInactive": true,
		"pagination":      map[string]int{"limit": len(ids), "offset": 0},
	})
	var p entityDeploymentsPage
	if err := h.call(ctx, func() ([]byte, error) { return h.entity.SearchDeployments(ctx, body) }, &p); err != nil {
		return nil, err
	}
	for _, d := range p.Deployments {
		if t, ok := deploymentTypes[d.Type]; ok {
			labels[d.ID] = t.Label
		}
	}
	return labels, nil
}

type productUpdateView struct {
	UpdateLevel *string `json:"updateLevel"`
	Date        *string `json:"date"`
	Details     *string `json:"details"`
}

// deployedProductDeploymentView is the deployment ref on a deployed product;
// type is the deployment type's display label (a plain string, unlike the
// {id,label} object on a deployment itself).
type deployedProductDeploymentView struct {
	ID   string  `json:"id"`
	Name *string `json:"name"`
	Type *string `json:"type"`
}

type deployedProductView struct {
	ID          string  `json:"id"`
	Description *string `json:"description"`
	Version     *struct {
		ID   string  `json:"id"`
		Name *string `json:"name"`
	} `json:"version"`
	Cores       json.RawMessage                `json:"cores"`
	TPS         json.RawMessage                `json:"tps"`
	ReleasedOn  *string                        `json:"releasedOn"`
	EndOfLifeOn *string                        `json:"endOfLifeOn"`
	Updates     []productUpdateView            `json:"updates"`
	CreatedOn   *string                        `json:"createdOn"`
	UpdatedOn   *string                        `json:"updatedOn"`
	Deployment  *deployedProductDeploymentView `json:"deployment"`
	Product     *struct {
		ID           string  `json:"id"`
		Name         *string `json:"name"`
		Abbreviation *string `json:"abbreviation"`
	} `json:"product"`
	// Category is always null: the scripted API hard-coded it.
	Category *string `json:"category"`
}

func nullIfAbsent(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}

// SearchDeployedProducts handles GET /deployed-products/search.
func (h *CatalogHandler) SearchDeployedProducts(w http.ResponseWriter, r *http.Request) {
	limit, offset, msg := parseLimitOffset(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	sfIDs, filtered, msg := parseSfIDs(r)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	// entity-service filters deployed products by deployment, so sfIds become
	// projects, then those projects' deployments.
	var deploymentIDs []string
	if filtered {
		projectIDs, err := h.resolveProjectIDs(r.Context(), sfIDs)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity SearchProjects failed", "err", summarizeErr(err))
			mapUpstreamError(w, err, "Failed to search deployed products.")
			return
		}
		if len(projectIDs) > 0 {
			deps, _, err := h.fetchDeployments(r.Context(), projectIDs, true, maxSfScopedDeployments, 0)
			if err != nil {
				slog.ErrorContext(r.Context(), "entity SearchDeployments failed", "err", summarizeErr(err))
				mapUpstreamError(w, err, "Failed to search deployed products.")
				return
			}
			for _, d := range deps {
				deploymentIDs = append(deploymentIDs, d.ID)
			}
		}
		if len(deploymentIDs) == 0 {
			writeEmptyList(w, "deployedProducts", "totalRecords", limit, offset)
			return
		}
	}

	items, total, err := fetchWindow(limit, offset, func(n, o int) (page[deployedProductView], error) {
		req := map[string]any{"pagination": map[string]int{"limit": n, "offset": o}}
		if deploymentIDs != nil {
			req["deploymentIds"] = deploymentIDs
		}
		body, _ := json.Marshal(req)
		var p entityDeployedProductsPage
		if err := h.call(r.Context(), func() ([]byte, error) { return h.entity.SearchDeployedProducts(r.Context(), body) }, &p); err != nil {
			return page[deployedProductView]{}, err
		}
		typeLabels, err := h.deploymentTypeLabels(r.Context(), p.DeployedProducts)
		if err != nil {
			return page[deployedProductView]{}, err
		}
		out := make([]deployedProductView, 0, len(p.DeployedProducts))
		for _, d := range p.DeployedProducts {
			v := deployedProductView{
				ID: d.ID, Description: d.Description, Cores: nullIfAbsent(d.Cores), TPS: truncToInt(d.TPS),
				CreatedOn: formatDateTime(d.CreatedOn), UpdatedOn: formatDateTime(d.UpdatedOn),
				Product: d.Product,
			}
			if d.Deployment != nil {
				v.Deployment = &deployedProductDeploymentView{ID: d.Deployment.ID, Name: d.Deployment.Name}
				if l, ok := typeLabels[d.Deployment.ID]; ok {
					v.Deployment.Type = &l
				}
			}
			if d.Version != nil {
				v.Version = &struct {
					ID   string  `json:"id"`
					Name *string `json:"name"`
				}{d.Version.ID, d.Version.Name}
				v.ReleasedOn, v.EndOfLifeOn = formatDate(d.Version.ReleasedDate), formatDate(d.Version.SupportEoL)
			}
			if d.Updates != nil {
				v.Updates = make([]productUpdateView, 0, len(d.Updates))
				for _, u := range d.Updates {
					pu := productUpdateView{Date: u.Date, Details: u.Details}
					if u.UpdateLevel != "" {
						lvl := fmt.Sprint(u.UpdateLevel)
						pu.UpdateLevel = &lvl
					}
					v.Updates = append(v.Updates, pu)
				}
			}
			out = append(out, v)
		}
		return page[deployedProductView]{items: out, total: p.Total, hasMore: p.HasMore}, nil
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployedProducts failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search deployed products.")
		return
	}
	writeEnvelope(w, map[string]any{"deployedProducts": items, "totalRecords": total, "offset": offset, "limit": limit})
}
