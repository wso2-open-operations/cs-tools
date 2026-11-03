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
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/wso2-open-operations/cs-tools/operations/legacy-product-api/internal/apierror"
)

// This file is the Legacy Product API: the read contract (v1) of the retired
// Spring Boot product service (wso2-product-service, on its own MySQL
// database), served from the CSM product
// catalogue (entity-service's products API, edited in the CSM Portal). Every
// legacy path is served at the root, exactly as before, so a consumer moves by
// changing the base URL. The consumers are
//
//   - the wso2.com Support Matrix and Release Matrix (Drupal module
//     wso2_marketing_services): /products/support-matrix,
//     /products/release-matrix, /carbon-platform-history,
//     /carbon-kernel-releases;
//   - the security advisory automations: /products,
//     /productNameForAbbreviation;
//   - UMT (endpoints unconfirmed, so every read is kept).
//
// "Legacy" below means the retired service's behaviour that v1 reproduces.
// Contract changes go into a new version, not v1.
//
// The handler reshapes every upstream body: the legacy JSON (key names and
// order, yyyy-M-d dates without zero padding, Title-case support status, one
// row per public listing with a deploymentProfile list) is the contract those
// consumers parse, and entity-service stays a clean internal API that owes
// nothing to it. Upstream dates arrive as RFC 3339 timestamps and statuses in
// upper case; normalizeVersions turns both into the date-only, lower-case
// form the mapping below works on.
//
// Mapping (one legacy row = one public listing = every live product version
// row sharing a publicId):
//
//   - id = publicId; rows with no publicId and retired rows are never returned.
//     Legacy's list feeds left out its 3 active=0 rows, which were not
//     imported; its by-id routes did return them, and here they are 404
//     (accepted: no consumer reads them by id);
//   - productName = publicName ?? productName; abbreviation =
//     publicAbbreviation ?? productCode; productVersion = version;
//   - deploymentProfile = the listing's profiles in a fixed order (see
//     legacyProfileOrder), null when it has none;
//   - status = legacyStatus ?? true; active = 1; productGroup = null.
//
// Deliberate differences from legacy, all documented on the handler that has
// them: productNameForAbbreviation answers every requested abbreviation (the
// legacy loop returned after the first); a listing's releaseLines are
// de-duplicated; productGroup is always null and /product-group is [];
// writes answer 405 (they already failed with 500 in legacy); a missing
// /deploymentProfile id is a 404, not a 500.

// entityProductAPIClient abstracts the entity service's product catalogue
// reads used by ProductAPIHandler (see entity.Client).
type entityProductAPIClient interface {
	// ListProductVersions returns {"productVersions":[...]}: every live row on
	// feed ("" = all live rows), all pages joined.
	ListProductVersions(ctx context.Context, feed string) ([]byte, error)
	ListReleaseLines(ctx context.Context, includeRetired bool) ([]byte, error)
	ListCarbonPlatformHistory(ctx context.Context) ([]byte, error)
	ListCarbonKernelReleases(ctx context.Context) ([]byte, error)
}

// Upstream feed names (entity-service POST /product-versions/search feed).
const (
	legacyFeedSupportMatrix = "support-matrix"
	legacyFeedReleaseMatrix = "release-matrix"
)

// errMsgLegacyReadOnly is the body of every legacy write route.
const errMsgLegacyReadOnly = "The product catalogue is managed in the CSM Portal. This API is read-only."

// Fallback messages for upstream failures.
const (
	errMsgLegacyProducts = "Failed to fetch products."
	errMsgLegacyLines    = "Failed to fetch release lines."
	errMsgLegacyCarbon   = "Failed to fetch carbon platform history."
	errMsgLegacyKernel   = "Failed to fetch carbon kernel releases."
)

// Path-parameter checks copied from legacy InputValidationUtil, which answered
// 400 with an empty body on a mismatch.
var (
	legacyVersionRe  = regexp.MustCompile(`^[a-zA-Z0-9.\-]+$`)
	legacyPlatformRe = regexp.MustCompile(`^[a-zA-Z0-9\s\-.]+$`)
)

const (
	legacyMaxVersionLen  = 20
	legacyMaxPlatformLen = 50
)

// legacyProfileOrder is the order deploymentProfile lists profiles in. It is
// the order legacy stored them in (the `;` list), so APIM 3.x/4.x and EI 6.x
// listings come out exactly as before. Profiles not listed sort after these,
// alphabetically.
var legacyProfileOrder = map[string]int{
	"All In One":       0,
	"Gateway worker":   1,
	"Key Manager":      2,
	"Traffic Manager":  3,
	"API Publisher":    4,
	"Developer Portal": 5,
	"Integrator":       6,
	"Business Process": 7,
	"Message Broker":   8,
	"Micro-Integrator": 9,
	"Analytics":        10,
}

// ProductAPIHandler serves the Product API's read endpoints from the
// CSM product catalogue. See the file comment.
type ProductAPIHandler struct {
	entity entityProductAPIClient
}

// NewProductAPIHandler creates a ProductAPIHandler backed by the
// given entity client.
func NewProductAPIHandler(entity entityProductAPIClient) *ProductAPIHandler {
	return &ProductAPIHandler{entity: entity}
}

// RegisterRoutes mounts every legacy path at the root.
//
// The routes live here rather than one by one in cmd/server/main.go so the
// tests exercise the real patterns: the literal segments (support-matrix,
// release-matrix, versions, latest, platforms, latest-by-platform) must win
// over the {id} wildcard beside them, which Go's ServeMux does by
// specificity, and a test proves it.
func (h *ProductAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /products", h.GetProducts)
	mux.HandleFunc("GET /products/support-matrix", h.GetSupportMatrix)
	mux.HandleFunc("GET /products/release-matrix", h.GetReleaseMatrix)
	mux.HandleFunc("GET /products/{id}", h.GetProduct)
	mux.HandleFunc("GET /searchProduct", h.SearchProduct)
	mux.HandleFunc("GET /getAllProductVersion", h.GetAllProductVersion)
	mux.HandleFunc("GET /productNameForAbbreviation", h.GetProductNameForAbbreviation)
	mux.HandleFunc("GET /deploymentProfile", h.GetDeploymentProfile)
	mux.HandleFunc("GET /release-line", h.GetReleaseLines)
	mux.HandleFunc("GET /product-group", h.GetProductGroups)
	mux.HandleFunc("GET /carbon-platform-history", h.GetCarbonPlatformHistory)
	mux.HandleFunc("GET /carbon-platform-history/versions", h.GetCarbonVersions)
	mux.HandleFunc("GET /carbon-platform-history/latest", h.GetLatestCarbonPlatform)
	mux.HandleFunc("GET /carbon-platform-history/version/{version}", h.GetCarbonPlatformHistoryByVersion)
	mux.HandleFunc("GET /carbon-platform-history/{id}", h.GetCarbonPlatformHistoryByID)
	mux.HandleFunc("GET /carbon-kernel-releases", h.GetCarbonKernelReleases)
	mux.HandleFunc("GET /carbon-kernel-releases/platforms", h.GetCarbonKernelPlatforms)
	mux.HandleFunc("GET /carbon-kernel-releases/latest-by-platform", h.GetLatestKernelByPlatform)
	mux.HandleFunc("GET /carbon-kernel-releases/platform/{platform}", h.GetCarbonKernelReleasesByPlatform)
	mux.HandleFunc("GET /carbon-kernel-releases/{id}", h.GetCarbonKernelReleaseByID)

	// The retired service's writes. They are not served: the catalogue is
	// edited in the CSM Portal, which audits every change.
	mux.HandleFunc("POST /products", h.ReadOnly)
	mux.HandleFunc("PUT /products/{id}", h.ReadOnly)
	mux.HandleFunc("DELETE /products/{id}", h.ReadOnly)
	mux.HandleFunc("POST /release-line", h.ReadOnly)
	mux.HandleFunc("POST /product-group", h.ReadOnly)
}

// ----- upstream (entity-service) shapes: only the fields used here -----

type lpCarbon struct {
	PlatformRelease     *string `json:"platformRelease"`
	PlatformVersion     *string `json:"platformVersion"`
	CarbonKernelVersion *string `json:"carbonKernelVersion"`
	ReleaseChunk        *string `json:"releaseChunk"`
	KernelPatchNo       *string `json:"kernelPatchNo"`
}

type lpVersion struct {
	ProductName                    string    `json:"productName"`
	ProductCode                    *string   `json:"productCode"`
	Version                        string    `json:"version"`
	DeploymentProfile              *string   `json:"deploymentProfile"`
	CurrentSupportStatus           *string   `json:"currentSupportStatus"`
	ReleaseDate                    *string   `json:"releaseDate"`
	EarliestPossibleSupportEOLDate *string   `json:"earliestPossibleSupportEolDate"`
	SupportEOLDate                 *string   `json:"supportEolDate"`
	SupportEOLAnnouncementDate     *string   `json:"supportEolAnnouncementDate"`
	PublicID                       *int64    `json:"publicId"`
	PublicName                     *string   `json:"publicName"`
	PublicAbbreviation             *string   `json:"publicAbbreviation"`
	Description                    *string   `json:"description"`
	IsPublic                       bool      `json:"isPublic"`
	ShowInSupportMatrix            bool      `json:"showInSupportMatrix"`
	Tags                           []string  `json:"tags"`
	ReleaseLinks                   *string   `json:"releaseLinks"`
	LegacyStatus                   *bool     `json:"legacyStatus"`
	ReleaseLineID                  *string   `json:"releaseLineId"`
	ReleaseLineName                *string   `json:"releaseLineName"`
	Carbon                         *lpCarbon `json:"carbon"`
	RetiredOn                      *string   `json:"retiredOn"`
}

type lpVersionList struct {
	Versions []lpVersion `json:"productVersions"`
}

type lpReleaseLine struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	StartDate *string `json:"startDate"`
	Status    *string `json:"status"`
	LegacyID  *int64  `json:"legacyId"`
	RetiredOn *string `json:"retiredOn"`
}

type lpReleaseLineList struct {
	ReleaseLines []lpReleaseLine `json:"releaseLines"`
}

type lpCarbonHistory struct {
	ID              int     `json:"id"`
	CarbonVersion   *string `json:"carbonVersion"`
	ReleaseDate     *string `json:"releaseDate"`
	ReleaseDateText *string `json:"releaseDateText"`
	legacyProductVersions
}

type lpCarbonHistoryList struct {
	Items []lpCarbonHistory `json:"items"`
}

type lpKernelRelease struct {
	ID             int     `json:"id"`
	KernelRelease  *string `json:"kernelRelease"`
	ReleaseDate    *string `json:"releaseDate"`
	CarbonPlatform *string `json:"carbonPlatform"`
}

type lpKernelReleaseList struct {
	Items []lpKernelRelease `json:"items"`
}

// ----- legacy (response) shapes. Field order is the legacy JSON key order. -----

// legacyProduct is legacy ProductDto (IsPublic and ShowInSupportMatrix set)
// or PublicProductDto (both nil, so omitted). Legacy Jackson wrote its keys
// alphabetically; the fields are declared in that order.
type legacyProduct struct {
	Abbreviation                   *string              `json:"abbreviation"`
	Active                         int                  `json:"active"`
	CarbonPlatform                 legacyCarbonPlatform `json:"carbonPlatform"`
	CurrentStatusOfSupport         *string              `json:"currentStatusOfSupport"`
	DeploymentProfile              []string             `json:"deploymentProfile"`
	Description                    *string              `json:"description"`
	EarliestPossibleSupportEolDate *string              `json:"earliestPossibleSupportEolDate"`
	ID                             int64                `json:"id"`
	IsPublic                       *bool                `json:"isPublic,omitempty"`
	ProductGroup                   *struct{}            `json:"productGroup"`
	ProductName                    string               `json:"productName"`
	ProductVersion                 string               `json:"productVersion"`
	ReleaseDate                    *string              `json:"releaseDate"`
	ReleaseLines                   []legacyReleaseLine  `json:"releaseLines"`
	ReleaseLinks                   *string              `json:"releaseLinks"`
	ShowInSupportMatrix            *bool                `json:"showInSupportMatrix,omitempty"`
	Status                         bool                 `json:"status"`
	SupportEolAnnouncementDate     *string              `json:"supportEolAnnouncementDate"`
	SupportEolDate                 *string              `json:"supportEolDate"`
	Tags                           []string             `json:"tags"`

	releaseDate string // upstream ISO date, for ordering; "" when null
}

type legacyCarbonPlatform struct {
	PlatformRelease     *string `json:"platformRelease"`
	PlatformVersion     *string `json:"platformVersion"`
	CarbonKernelVersion *string `json:"carbonKernelVersion"`
	ReleaseChunk        *string `json:"releaseChunk"`
	KernelPatchNo       *string `json:"kernelPatchNo"`
}

type legacyReleaseLine struct {
	ReleaseLineID        *int64  `json:"releaseLineId"`
	ReleaseLine          string  `json:"releaseLine"`
	ReleaseLineStartDate *string `json:"releaseLineStartDate"`
	ReleaseLineStatus    *string `json:"releaseLineStatus"`
}

type legacyDeploymentProfile struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// legacyProductVersions is legacy CarbonPlatformHistoryDto.ProductVersionsDto.
// Entity-service uses the same JSON names, so it also decodes the upstream row.
type legacyProductVersions struct {
	ESBVersion                     *string `json:"esbVersion"`
	GovernanceRegistryVersion      *string `json:"governanceRegistryVersion"`
	ApplicationServerVersion       *string `json:"applicationServerVersion"`
	DataServicesServerVersion      *string `json:"dataServicesServerVersion"`
	IdentityServerVersion          *string `json:"identityServerVersion"`
	BusinessProcessServerVersion   *string `json:"businessProcessServerVersion"`
	MashupServerVersion            *string `json:"mashupServerVersion"`
	GadgetServerVersion            *string `json:"gadgetServerVersion"`
	BusinessActivityMonitorVersion *string `json:"businessActivityMonitorVersion"`
	APIManagerVersion              *string `json:"apiManagerVersion"`
	BusinessRulesServerVersion     *string `json:"businessRulesServerVersion"`
	EventServerVersion             *string `json:"eventServerVersion"`
	MessageBrokerVersion           *string `json:"messageBrokerVersion"`
	ComplexEventProcessorVersion   *string `json:"complexEventProcessorVersion"`
	ElasticLoadBalancerVersion     *string `json:"elasticLoadBalancerVersion"`
	StorageServerVersion           *string `json:"storageServerVersion"`
	StratosVersion                 *string `json:"stratosVersion"`
	CloudGatewayVersion            *string `json:"cloudGatewayVersion"`
	WSASVersion                    *string `json:"wsasVersion"`
	AppFactoryVersion              *string `json:"appFactoryVersion"`
	UserEngagementServerVersion    *string `json:"userEngagementServerVersion"`
}

type legacyCarbonHistory struct {
	ID              int                   `json:"id"`
	CarbonVersion   *string               `json:"carbonVersion"`
	ReleaseDate     *string               `json:"releaseDate"`
	ProductVersions legacyProductVersions `json:"productVersions"`
}

type legacyKernelRelease struct {
	ID             int     `json:"id"`
	KernelRelease  *string `json:"kernelRelease"`
	ReleaseDate    *string `json:"releaseDate"`
	CarbonPlatform *string `json:"carbonPlatform"`
}

// ----- helpers -----

// writeLegacyJSON encodes v like legacy Jackson did: no HTML escaping (Go
// would otherwise turn & < > into & etc.) and no trailing newline.
// writeUnescapedJSON appends v's JSON to buf without HTML escaping (legacy
// wrote "&", "<" and ">" as is) and without the encoder's trailing newline.
func writeUnescapedJSON(buf *bytes.Buffer, v any) {
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v) // a string or *string always encodes
	buf.Truncate(buf.Len() - 1)
}

func writeLegacyJSON(w http.ResponseWriter, r *http.Request, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode legacy product API response", "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	writeJSON(w, http.StatusOK, bytes.TrimRight(buf.Bytes(), "\n"))
}

// writeLegacyEmpty writes a status with no body, which is what legacy's
// ResponseEntity.notFound()/badRequest().build() sent.
func writeLegacyEmpty(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}

// legacyDate turns an upstream "YYYY-MM-DD" date into legacy's "yyyy-M-d"
// (no zero padding, e.g. 2020-4-7). Null stays null; a value that is not a
// date is passed through rather than dropped.
func legacyDate(s *string) *string {
	if s == nil {
		return nil
	}
	t, err := time.Parse(time.DateOnly, *s)
	if err != nil {
		v := *s
		return &v
	}
	v := strconv.Itoa(t.Year()) + "-" + strconv.Itoa(int(t.Month())) + "-" + strconv.Itoa(t.Day())
	return &v
}

// legacyStatus turns a support status into legacy's Title case ("EXTENDED"
// or "extended" -> "Extended"). Null stays null, as legacy returned null for
// the one row stored without a status.
func legacyStatus(s *string) *string {
	if s == nil || *s == "" {
		return s
	}
	lower := strings.ToLower(*s)
	v := strings.ToUpper(lower[:1]) + lower[1:]
	return &v
}

// dateOnly turns an upstream RFC 3339 timestamp ("2020-04-07T00:00:00Z") into
// its "YYYY-MM-DD" date. A value already date-only, null, or not a date is
// returned unchanged.
func dateOnly(s *string) *string {
	if s == nil || len(*s) <= len(time.DateOnly) {
		return s
	}
	if _, err := time.Parse(time.DateOnly, (*s)[:len(time.DateOnly)]); err != nil {
		return s
	}
	v := (*s)[:len(time.DateOnly)]
	return &v
}

// normalizeVersions puts upstream rows in the form the mapping works on:
// date-only dates and a lower-case support status.
func normalizeVersions(versions []lpVersion) {
	for i := range versions {
		v := &versions[i]
		v.ReleaseDate = dateOnly(v.ReleaseDate)
		v.EarliestPossibleSupportEOLDate = dateOnly(v.EarliestPossibleSupportEOLDate)
		v.SupportEOLDate = dateOnly(v.SupportEOLDate)
		v.SupportEOLAnnouncementDate = dateOnly(v.SupportEOLAnnouncementDate)
		if v.CurrentSupportStatus != nil {
			s := strings.ToLower(*v.CurrentSupportStatus)
			v.CurrentSupportStatus = &s
		}
	}
}

// normalizeReleaseLines turns the lines' start timestamps into dates.
func normalizeReleaseLines(lines []lpReleaseLine) {
	for i := range lines {
		lines[i].StartDate = dateOnly(lines[i].StartDate)
	}
}

func boolPtr(b bool) *bool { return &b }

// legacyUpstreamError answers a failed entity-service call the way legacy
// answered a failure of its own database: 503 when the upstream is down,
// overloaded or timed out, otherwise 500. Upstream 401/403/404/400 are never
// passed through, since a legacy caller would read them as a problem with its
// own request.
func legacyUpstreamError(w http.ResponseWriter, err error, fallback string) {
	if upstreamUnavailable(err) {
		writeError(w, http.StatusServiceUnavailable, fallback)
		return
	}
	writeError(w, http.StatusInternalServerError, fallback)
}

// upstreamUnavailable reports whether err means entity-service could not be
// reached or could not answer: an upstream 429/502/503/504, a network error
// (connection refused, DNS, the client timeout), a cancelled or expired
// context, or a failed token fetch.
func upstreamUnavailable(err error) bool {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
		return false
	}
	var netErr net.Error
	var urlErr *url.Error
	var tokenErr *oauth2.RetrieveError
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		errors.As(err, &netErr) || errors.As(err, &urlErr) || errors.As(err, &tokenErr)
}

// readUpstream calls fetch and decodes its body into out. On failure it writes
// the error response (legacyUpstreamError, or 500 for a body that will not
// decode) and returns false.
func readUpstream(w http.ResponseWriter, r *http.Request, op, fallback string, fetch func() ([]byte, error), out any) bool {
	body, err := fetch()
	if err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "err", summarizeErr(err))
		legacyUpstreamError(w, err, fallback)
		return false
	}
	if err := json.Unmarshal(body, out); err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" returned an undecodable body", "err", err)
		writeError(w, http.StatusInternalServerError, fallback)
		return false
	}
	return true
}

// loadListings fetches the catalogue rows for feed ("" = all live rows) and the
// release lines, and returns the legacy listings in legacy order. withToggles
// selects ProductDto (isPublic, showInSupportMatrix) over PublicProductDto.
func (h *ProductAPIHandler) loadListings(w http.ResponseWriter, r *http.Request, feed string, withToggles bool) ([]legacyProduct, bool) {
	var versions lpVersionList
	if !readUpstream(w, r, "ListProductVersions", errMsgLegacyProducts,
		func() ([]byte, error) { return h.entity.ListProductVersions(r.Context(), feed) }, &versions) {
		return nil, false
	}
	normalizeVersions(versions.Versions)
	// Retired lines too: a live version can still point at a retired line,
	// and legacy rendered whatever line a row was linked to.
	var lines lpReleaseLineList
	if !readUpstream(w, r, "ListReleaseLines", errMsgLegacyProducts,
		func() ([]byte, error) { return h.entity.ListReleaseLines(r.Context(), true) }, &lines) {
		return nil, false
	}
	normalizeReleaseLines(lines.ReleaseLines)
	return buildLegacyProducts(versions.Versions, lines.ReleaseLines, withToggles), true
}

// buildLegacyProducts groups catalogue rows into legacy listings: one per
// publicId, skipping rows with no publicId and retired rows.
//
// Order: release date newest first, then id ascending, which is what legacy's
// `ORDER BY releaseDate DESC` produced on the production copy (ties came back
// in id order). Listings with no release date come last.
func buildLegacyProducts(versions []lpVersion, lines []lpReleaseLine, withToggles bool) []legacyProduct {
	lineByID := make(map[string]lpReleaseLine, len(lines))
	for _, l := range lines {
		lineByID[l.ID] = l
	}

	var order []int64
	groups := map[int64][]lpVersion{}
	for _, v := range versions {
		if v.PublicID == nil || v.RetiredOn != nil {
			continue
		}
		id := *v.PublicID
		if _, seen := groups[id]; !seen {
			order = append(order, id)
		}
		groups[id] = append(groups[id], v)
	}

	out := make([]legacyProduct, 0, len(order))
	for _, id := range order {
		out = append(out, buildLegacyProduct(id, groups[id], lineByID, withToggles))
	}
	slices.SortStableFunc(out, func(a, b legacyProduct) int {
		if a.releaseDate != b.releaseDate {
			if a.releaseDate == "" {
				return 1
			}
			if b.releaseDate == "" {
				return -1
			}
			return strings.Compare(b.releaseDate, a.releaseDate)
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out
}

// buildLegacyProduct turns the rows of one listing into a legacy row. Publish
// guarantees the rows agree on name, abbreviation, status, the four dates,
// isPublic and showInSupportMatrix, so those come from the first row; the
// fields that may legitimately differ per profile row take the first non-empty
// value.
func buildLegacyProduct(id int64, rows []lpVersion, lineByID map[string]lpReleaseLine, withToggles bool) legacyProduct {
	first := rows[0]
	p := legacyProduct{
		Abbreviation:                   cmp.Or(first.PublicAbbreviation, first.ProductCode),
		Active:                         1,
		CurrentStatusOfSupport:         legacyStatus(first.CurrentSupportStatus),
		EarliestPossibleSupportEolDate: legacyDate(first.EarliestPossibleSupportEOLDate),
		ID:                             id,
		ProductName:                    first.ProductName,
		ProductVersion:                 first.Version,
		ReleaseDate:                    legacyDate(first.ReleaseDate),
		ReleaseLines:                   []legacyReleaseLine{},
		Status:                         true,
		SupportEolAnnouncementDate:     legacyDate(first.SupportEOLAnnouncementDate),
		SupportEolDate:                 legacyDate(first.SupportEOLDate),
	}
	if first.PublicName != nil {
		p.ProductName = *first.PublicName
	}
	if first.LegacyStatus != nil {
		p.Status = *first.LegacyStatus
	}
	if first.ReleaseDate != nil {
		p.releaseDate = *first.ReleaseDate
	}
	if withToggles {
		p.IsPublic = boolPtr(first.IsPublic)
		p.ShowInSupportMatrix = boolPtr(first.ShowInSupportMatrix)
	}

	seenProfile := map[string]bool{}
	seenLine := map[string]bool{}
	carbonSet := false
	for _, v := range rows {
		if p.Description == nil {
			p.Description = v.Description
		}
		if p.ReleaseLinks == nil {
			p.ReleaseLinks = v.ReleaseLinks
		}
		if len(p.Tags) == 0 && len(v.Tags) > 0 {
			p.Tags = v.Tags
		}
		if !carbonSet && v.Carbon != nil {
			p.CarbonPlatform = legacyCarbonPlatform(*v.Carbon)
			carbonSet = true
		}
		if v.DeploymentProfile != nil && !seenProfile[*v.DeploymentProfile] {
			seenProfile[*v.DeploymentProfile] = true
			p.DeploymentProfile = append(p.DeploymentProfile, *v.DeploymentProfile)
		}
		// De-duplicated on purpose: legacy repeated a line when a version was
		// linked to it twice (version 1085 on the production copy).
		if v.ReleaseLineID != nil && !seenLine[*v.ReleaseLineID] {
			seenLine[*v.ReleaseLineID] = true
			p.ReleaseLines = append(p.ReleaseLines, toLegacyReleaseLine(*v.ReleaseLineID, v.ReleaseLineName, lineByID))
		}
	}
	slices.SortStableFunc(p.DeploymentProfile, compareProfiles)
	return p
}

// compareProfiles orders profiles by legacyProfileOrder, then alphabetically.
func compareProfiles(a, b string) int {
	ia, oka := legacyProfileOrder[a]
	ib, okb := legacyProfileOrder[b]
	switch {
	case oka && okb:
		return cmp.Compare(ia, ib)
	case oka:
		return -1
	case okb:
		return 1
	}
	return strings.Compare(a, b)
}

// toLegacyReleaseLine renders a linked release line. releaseLineId is the
// line's legacy numeric id, or null for a line created in CSM (which has none).
func toLegacyReleaseLine(id string, name *string, lineByID map[string]lpReleaseLine) legacyReleaseLine {
	if l, ok := lineByID[id]; ok {
		return releaseLineToLegacy(l)
	}
	out := legacyReleaseLine{}
	if name != nil {
		out.ReleaseLine = *name
	}
	return out
}

func releaseLineToLegacy(l lpReleaseLine) legacyReleaseLine {
	return legacyReleaseLine{
		ReleaseLineID:        l.LegacyID,
		ReleaseLine:          l.Name,
		ReleaseLineStartDate: legacyDate(l.StartDate),
		ReleaseLineStatus:    l.Status,
	}
}

// legacyTagFilter reads the inclusion or exclusion query parameter the way
// legacy parseTagFilter did: comma-separated, trimmed, empties dropped; nil
// when absent or empty. Repeated parameters are joined, as Spring did.
func legacyTagFilter(r *http.Request, name string) []string {
	values, ok := r.URL.Query()[name]
	if !ok {
		return nil
	}
	var out []string
	for _, tok := range strings.Split(strings.Join(values, ","), ",") {
		if t := strings.TrimSpace(tok); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// legacyShouldInclude is legacy ProductService.shouldInclude: a listing with
// an excluded tag is dropped; with an inclusion list, a listing needs one of
// those tags (so untagged listings are dropped). Tags compare trimmed and
// case-insensitively.
func legacyShouldInclude(tags, exclusion, inclusion []string) bool {
	if inclusion == nil && exclusion == nil {
		return true
	}
	if len(tags) == 0 {
		return inclusion == nil
	}
	match := func(tag, filter string) bool {
		tag, filter = strings.TrimSpace(tag), strings.TrimSpace(filter)
		return tag != "" && filter != "" && strings.EqualFold(tag, filter)
	}
	for _, tag := range tags {
		for _, ex := range exclusion {
			if match(tag, ex) {
				return false
			}
		}
	}
	if len(inclusion) == 0 {
		return true
	}
	for _, tag := range tags {
		for _, in := range inclusion {
			if match(tag, in) {
				return true
			}
		}
	}
	return false
}

// filterListings applies the request's inclusion/exclusion tags and keep.
func filterListings(r *http.Request, listings []legacyProduct, keep func(legacyProduct) bool) []legacyProduct {
	exclusion := legacyTagFilter(r, "exclusion")
	inclusion := legacyTagFilter(r, "inclusion")
	out := make([]legacyProduct, 0, len(listings))
	for _, p := range listings {
		if keep != nil && !keep(p) {
			continue
		}
		if legacyShouldInclude(p.Tags, exclusion, inclusion) {
			out = append(out, p)
		}
	}
	return out
}

// requireQuery returns the named query parameter, or writes a 400 and returns
// false when it is absent (legacy's @RequestParam was required).
func requireQuery(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	q := r.URL.Query()
	if !q.Has(name) {
		writeError(w, http.StatusBadRequest, "Required request parameter '"+name+"' is not present.")
		return "", false
	}
	return q.Get(name), true
}

// legacyID parses a legacy numeric id path/query value; legacy answered 400
// for anything that was not a positive integer.
func legacyID(s string) (int64, bool) {
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil && id >= 1
}

// ----- product handlers -----

// GetProducts handles GET /products?inclusion=&exclusion= —
// every live listing (ProductDto).
func (h *ProductAPIHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	listings, ok := h.loadListings(w, r, "", true)
	if !ok {
		return
	}
	writeLegacyJSON(w, r, filterListings(r, listings, nil))
}

// GetSupportMatrix handles GET /products/support-matrix —
// the wso2.com Support Matrix feed (PublicProductDto).
func (h *ProductAPIHandler) GetSupportMatrix(w http.ResponseWriter, r *http.Request) {
	listings, ok := h.loadListings(w, r, legacyFeedSupportMatrix, false)
	if !ok {
		return
	}
	writeLegacyJSON(w, r, filterListings(r, listings, nil))
}

// GetReleaseMatrix handles GET /products/release-matrix —
// the wso2.com Release Matrix feed (PublicProductDto).
func (h *ProductAPIHandler) GetReleaseMatrix(w http.ResponseWriter, r *http.Request) {
	listings, ok := h.loadListings(w, r, legacyFeedReleaseMatrix, false)
	if !ok {
		return
	}
	writeLegacyJSON(w, r, filterListings(r, listings, nil))
}

// GetProduct handles GET /products/{id}. As in legacy, a
// non-positive or non-numeric id is a 400 and an unknown id a 404, both with
// an empty body.
func (h *ProductAPIHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, valid := legacyID(r.PathValue("id"))
	if !valid {
		writeLegacyEmpty(w, http.StatusBadRequest)
		return
	}
	listings, ok := h.loadListings(w, r, "", true)
	if !ok {
		return
	}
	for _, p := range listings {
		if p.ID == id {
			writeLegacyJSON(w, r, p)
			return
		}
	}
	writeLegacyEmpty(w, http.StatusNotFound)
}

// SearchProduct handles GET /searchProduct?key= — listings
// whose productName starts with key, case-insensitively (legacy: MySQL
// `LIKE CONCAT(?1, '%')` on a case-insensitive collation). LIKE wildcards in
// key are matched literally here. Legacy had no ORDER BY and so returned
// primary-key order; this sorts by id to match.
func (h *ProductAPIHandler) SearchProduct(w http.ResponseWriter, r *http.Request) {
	key, ok := requireQuery(w, r, "key")
	if !ok {
		return
	}
	listings, ok := h.loadListings(w, r, "", true)
	if !ok {
		return
	}
	prefix := strings.ToLower(key)
	out := filterListings(r, listings, func(p legacyProduct) bool {
		return strings.HasPrefix(strings.ToLower(p.ProductName), prefix)
	})
	slices.SortStableFunc(out, func(a, b legacyProduct) int { return cmp.Compare(a.ID, b.ID) })
	writeLegacyJSON(w, r, out)
}

// GetAllProductVersion handles GET /getAllProductVersion?product=
// — every listing of one product name. Legacy compared with `=` on a
// case-insensitive collation that ignores trailing spaces; so does this.
func (h *ProductAPIHandler) GetAllProductVersion(w http.ResponseWriter, r *http.Request) {
	product, ok := requireQuery(w, r, "product")
	if !ok {
		return
	}
	listings, ok := h.loadListings(w, r, "", true)
	if !ok {
		return
	}
	want := strings.TrimRight(product, " ")
	writeLegacyJSON(w, r, filterListings(r, listings, func(p legacyProduct) bool {
		return strings.EqualFold(strings.TrimRight(p.ProductName, " "), want)
	}))
}

// GetProductNameForAbbreviation handles GET
// /productNameForAbbreviation?abbrList=a,b,c — an object
// mapping each abbreviation to its product name, or null when unknown.
//
// *** DELIBERATE FIX: every requested abbreviation is answered. *** Legacy
// returned from inside its loop, so only the first one ever came back
// ({"wso2am":"API Manager"} for wso2am,wso2is,wso2mi). A caller that only
// read the first key still finds it; keys follow the request order.
//
// When several listings share an abbreviation, the lowest id wins, as in
// legacy (`SELECT DISTINCT` in primary-key order). Like legacy's MySQL
// comparison, trailing spaces on either side are ignored.
func (h *ProductAPIHandler) GetProductNameForAbbreviation(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireQuery(w, r, "abbrList"); !ok {
		return
	}
	var abbrs []string
	seen := map[string]bool{}
	for _, v := range r.URL.Query()["abbrList"] {
		for _, a := range strings.Split(v, ",") {
			if a = strings.TrimSpace(a); a != "" && !seen[a] {
				seen[a] = true
				abbrs = append(abbrs, a)
			}
		}
	}
	listings, ok := h.loadListings(w, r, "", true)
	if !ok {
		return
	}

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, a := range abbrs {
		var name *string
		var nameID int64
		want := strings.TrimRight(a, " ")
		for _, p := range listings {
			if p.Abbreviation == nil || !strings.EqualFold(strings.TrimRight(*p.Abbreviation, " "), want) {
				continue
			}
			if name == nil || p.ID < nameID {
				n := p.ProductName
				name, nameID = &n, p.ID
			}
		}
		if i > 0 {
			buf.WriteByte(',')
		}
		writeUnescapedJSON(&buf, a)
		buf.WriteByte(':')
		writeUnescapedJSON(&buf, name)
	}
	buf.WriteByte('}')
	writeJSON(w, http.StatusOK, buf.Bytes())
}

// GetDeploymentProfile handles GET /deploymentProfile?id= —
// the listing's profiles as [{"id":"0","title":"All In One"}, ...] (ids are
// strings, as in legacy). A listing with no profile gives [].
//
// Deliberate difference: an unknown id is a 404 with an empty body (legacy
// threw an IndexOutOfBounds and answered 500).
func (h *ProductAPIHandler) GetDeploymentProfile(w http.ResponseWriter, r *http.Request) {
	raw, ok := requireQuery(w, r, "id")
	if !ok {
		return
	}
	id, valid := legacyID(raw)
	if !valid {
		writeLegacyEmpty(w, http.StatusBadRequest)
		return
	}
	listings, ok := h.loadListings(w, r, "", true)
	if !ok {
		return
	}
	for _, p := range listings {
		if p.ID != id {
			continue
		}
		out := make([]legacyDeploymentProfile, 0, len(p.DeploymentProfile))
		for i, title := range p.DeploymentProfile {
			out = append(out, legacyDeploymentProfile{ID: strconv.Itoa(i), Title: title})
		}
		writeLegacyJSON(w, r, out)
		return
	}
	writeLegacyEmpty(w, http.StatusNotFound)
}

// ----- release lines and product groups -----

// GetReleaseLines handles GET /release-line — the live
// release lines, ordered by legacy id.
//
// releaseLineId is the line's legacy numeric id. A line created in CSM has
// none and is still listed, with releaseLineId null and sorted last by name:
// the wso2.com consumers read only the name, start date and status.
func (h *ProductAPIHandler) GetReleaseLines(w http.ResponseWriter, r *http.Request) {
	var lines lpReleaseLineList
	if !readUpstream(w, r, "ListReleaseLines", errMsgLegacyLines,
		func() ([]byte, error) { return h.entity.ListReleaseLines(r.Context(), false) }, &lines) {
		return
	}
	normalizeReleaseLines(lines.ReleaseLines)
	live := make([]lpReleaseLine, 0, len(lines.ReleaseLines))
	for _, l := range lines.ReleaseLines {
		if l.RetiredOn == nil {
			live = append(live, l)
		}
	}
	slices.SortStableFunc(live, func(a, b lpReleaseLine) int {
		switch {
		case a.LegacyID != nil && b.LegacyID != nil:
			return cmp.Compare(*a.LegacyID, *b.LegacyID)
		case a.LegacyID != nil:
			return -1
		case b.LegacyID != nil:
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	out := make([]legacyReleaseLine, 0, len(live))
	for _, l := range live {
		out = append(out, releaseLineToLegacy(l))
	}
	writeLegacyJSON(w, r, out)
}

// GetProductGroups handles GET /product-group. Product
// groups are not carried into CSM (three legacy rows used them and no known
// consumer reads them), so this is always [] and every listing's productGroup
// is null.
func (h *ProductAPIHandler) GetProductGroups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []byte("[]"))
}

// ReadOnly answers every legacy write route (POST/PUT/DELETE /products...,
// POST /release-line, POST /product-group) with 405. In legacy, POST and PUT
// already failed with 500; DELETE was a hard delete with no audit.
func (h *ProductAPIHandler) ReadOnly(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET, HEAD")
	writeError(w, http.StatusMethodNotAllowed, errMsgLegacyReadOnly)
}

// ----- carbon platform history -----

// carbonSortText is the text legacy sorted carbon_platform_history on: its
// release_date column was VARCHAR "M/d/yyyy", and `ORDER BY releaseDate DESC`
// compared that text, so the order is not chronological. The stored text is
// used when present; a date with no text is rendered the same way.
func carbonSortText(c lpCarbonHistory) (string, bool) {
	if c.ReleaseDateText != nil {
		return *c.ReleaseDateText, true
	}
	if c.ReleaseDate != nil {
		if t, err := time.Parse(time.DateOnly, *c.ReleaseDate); err == nil {
			return strconv.Itoa(int(t.Month())) + "/" + strconv.Itoa(t.Day()) + "/" + strconv.Itoa(t.Year()), true
		}
		return *c.ReleaseDate, true
	}
	return "", false
}

// sortCarbonLegacy orders rows like legacy `ORDER BY releaseDate DESC` on the
// text column: text descending, rows without a date last, ties by id
// descending (as the production copy returned its undated rows).
func sortCarbonLegacy(items []lpCarbonHistory) {
	slices.SortStableFunc(items, func(a, b lpCarbonHistory) int {
		ta, oka := carbonSortText(a)
		tb, okb := carbonSortText(b)
		switch {
		case oka && okb && ta != tb:
			return strings.Compare(tb, ta)
		case oka && !okb:
			return -1
		case !oka && okb:
			return 1
		}
		return cmp.Compare(b.ID, a.ID)
	})
}

// legacyCarbonDate reproduces legacy's carbon release date: the date as
// yyyy-M-d, or, for a text it could not parse, the text up to the first space
// or "T".
func legacyCarbonDate(c lpCarbonHistory) *string {
	if c.ReleaseDate != nil {
		return legacyDate(c.ReleaseDate)
	}
	if c.ReleaseDateText == nil {
		return nil
	}
	part := strings.TrimSpace(*c.ReleaseDateText)
	if i := strings.IndexAny(part, "T \t"); i >= 0 {
		part = part[:i]
	}
	for _, layout := range []string{time.DateOnly, "2006-1-2", "1/2/2006"} {
		if t, err := time.Parse(layout, part); err == nil {
			v := strconv.Itoa(t.Year()) + "-" + strconv.Itoa(int(t.Month())) + "-" + strconv.Itoa(t.Day())
			return &v
		}
	}
	return &part
}

func toLegacyCarbon(c lpCarbonHistory) legacyCarbonHistory {
	return legacyCarbonHistory{
		ID:              c.ID,
		CarbonVersion:   c.CarbonVersion,
		ReleaseDate:     legacyCarbonDate(c),
		ProductVersions: c.legacyProductVersions,
	}
}

func toLegacyCarbonList(items []lpCarbonHistory) []legacyCarbonHistory {
	out := make([]legacyCarbonHistory, 0, len(items))
	for _, c := range items {
		out = append(out, toLegacyCarbon(c))
	}
	return out
}

func (h *ProductAPIHandler) loadCarbon(w http.ResponseWriter, r *http.Request) ([]lpCarbonHistory, bool) {
	var list lpCarbonHistoryList
	if !readUpstream(w, r, "ListCarbonPlatformHistory", errMsgLegacyCarbon,
		func() ([]byte, error) { return h.entity.ListCarbonPlatformHistory(r.Context()) }, &list) {
		return nil, false
	}
	return list.Items, true
}

// GetCarbonPlatformHistory handles GET
// /carbon-platform-history?productType= — every row, or
// with productType (api-manager, identity-server, esb; case-insensitive) only
// rows with that product's version. Any other productType is a 400 with an
// empty body, as in legacy.
func (h *ProductAPIHandler) GetCarbonPlatformHistory(w http.ResponseWriter, r *http.Request) {
	var keep func(lpCarbonHistory) bool
	if q := r.URL.Query(); q.Has("productType") {
		switch strings.ToLower(q.Get("productType")) {
		case "api-manager":
			keep = func(c lpCarbonHistory) bool { return c.APIManagerVersion != nil }
		case "identity-server":
			keep = func(c lpCarbonHistory) bool { return c.IdentityServerVersion != nil }
		case "esb":
			keep = func(c lpCarbonHistory) bool { return c.ESBVersion != nil }
		default:
			writeLegacyEmpty(w, http.StatusBadRequest)
			return
		}
	}
	items, ok := h.loadCarbon(w, r)
	if !ok {
		return
	}
	if keep != nil {
		items = slices.DeleteFunc(items, func(c lpCarbonHistory) bool { return !keep(c) })
	}
	sortCarbonLegacy(items)
	writeLegacyJSON(w, r, toLegacyCarbonList(items))
}

// GetCarbonPlatformHistoryByID handles GET
// /carbon-platform-history/{id}.
func (h *ProductAPIHandler) GetCarbonPlatformHistoryByID(w http.ResponseWriter, r *http.Request) {
	id, valid := legacyID(r.PathValue("id"))
	if !valid {
		writeLegacyEmpty(w, http.StatusBadRequest)
		return
	}
	items, ok := h.loadCarbon(w, r)
	if !ok {
		return
	}
	for _, c := range items {
		if int64(c.ID) == id {
			writeLegacyJSON(w, r, toLegacyCarbon(c))
			return
		}
	}
	writeLegacyEmpty(w, http.StatusNotFound)
}

// GetCarbonVersions handles GET
// /carbon-platform-history/versions — the distinct carbon
// versions in ascending text order (legacy `SELECT DISTINCT ... ORDER BY`),
// a null version first if there is one.
func (h *ProductAPIHandler) GetCarbonVersions(w http.ResponseWriter, r *http.Request) {
	items, ok := h.loadCarbon(w, r)
	if !ok {
		return
	}
	seen := map[string]bool{}
	hasNull := false
	var versions []string
	for _, c := range items {
		if c.CarbonVersion == nil {
			hasNull = true
			continue
		}
		if !seen[*c.CarbonVersion] {
			seen[*c.CarbonVersion] = true
			versions = append(versions, *c.CarbonVersion)
		}
	}
	slices.Sort(versions)
	out := make([]*string, 0, len(versions)+1)
	if hasNull {
		out = append(out, nil)
	}
	for i := range versions {
		out = append(out, &versions[i])
	}
	writeLegacyJSON(w, r, out)
}

// GetCarbonPlatformHistoryByVersion handles GET
// /carbon-platform-history/version/{version} — the rows of
// one carbon version, in id order. The version must match legacy's
// [a-zA-Z0-9.-]{1,20}, else 400 with an empty body.
func (h *ProductAPIHandler) GetCarbonPlatformHistoryByVersion(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	if len(version) > legacyMaxVersionLen || !legacyVersionRe.MatchString(version) {
		writeLegacyEmpty(w, http.StatusBadRequest)
		return
	}
	items, ok := h.loadCarbon(w, r)
	if !ok {
		return
	}
	items = slices.DeleteFunc(items, func(c lpCarbonHistory) bool {
		return c.CarbonVersion == nil || !strings.EqualFold(*c.CarbonVersion, version)
	})
	slices.SortStableFunc(items, func(a, b lpCarbonHistory) int { return cmp.Compare(a.ID, b.ID) })
	writeLegacyJSON(w, r, toLegacyCarbonList(items))
}

// GetLatestCarbonPlatform handles GET
// /carbon-platform-history/latest — the first row of the
// legacy ordering. Because legacy sorted the date TEXT, that is 3.0.1
// ("9/8/2010") on the production copy, not the newest platform; this is
// reproduced, not fixed. 404 with an empty body when there are no rows.
func (h *ProductAPIHandler) GetLatestCarbonPlatform(w http.ResponseWriter, r *http.Request) {
	items, ok := h.loadCarbon(w, r)
	if !ok {
		return
	}
	if len(items) == 0 {
		writeLegacyEmpty(w, http.StatusNotFound)
		return
	}
	sortCarbonLegacy(items)
	writeLegacyJSON(w, r, toLegacyCarbon(items[0]))
}

// ----- carbon kernel releases -----

// sortKernels orders kernel releases by release date, newest first, undated
// last, ties by id.
func sortKernels(items []lpKernelRelease) {
	slices.SortStableFunc(items, func(a, b lpKernelRelease) int {
		switch {
		case a.ReleaseDate != nil && b.ReleaseDate != nil && *a.ReleaseDate != *b.ReleaseDate:
			return strings.Compare(*b.ReleaseDate, *a.ReleaseDate)
		case a.ReleaseDate != nil && b.ReleaseDate == nil:
			return -1
		case a.ReleaseDate == nil && b.ReleaseDate != nil:
			return 1
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

func toLegacyKernel(k lpKernelRelease) legacyKernelRelease {
	return legacyKernelRelease{
		ID:             k.ID,
		KernelRelease:  k.KernelRelease,
		ReleaseDate:    legacyDate(k.ReleaseDate),
		CarbonPlatform: k.CarbonPlatform,
	}
}

func toLegacyKernelList(items []lpKernelRelease) []legacyKernelRelease {
	out := make([]legacyKernelRelease, 0, len(items))
	for _, k := range items {
		out = append(out, toLegacyKernel(k))
	}
	return out
}

// loadKernels returns the kernel releases in legacy order.
func (h *ProductAPIHandler) loadKernels(w http.ResponseWriter, r *http.Request) ([]lpKernelRelease, bool) {
	var list lpKernelReleaseList
	if !readUpstream(w, r, "ListCarbonKernelReleases", errMsgLegacyKernel,
		func() ([]byte, error) { return h.entity.ListCarbonKernelReleases(r.Context()) }, &list) {
		return nil, false
	}
	sortKernels(list.Items)
	return list.Items, true
}

// kernelPlatforms returns the distinct non-null platforms, alphabetically, and
// whether a null platform exists.
func kernelPlatforms(items []lpKernelRelease) ([]string, bool) {
	seen := map[string]bool{}
	hasNull := false
	var out []string
	for _, k := range items {
		if k.CarbonPlatform == nil {
			hasNull = true
			continue
		}
		if !seen[*k.CarbonPlatform] {
			seen[*k.CarbonPlatform] = true
			out = append(out, *k.CarbonPlatform)
		}
	}
	slices.Sort(out)
	return out, hasNull
}

// GetCarbonKernelReleases handles GET /carbon-kernel-releases.
func (h *ProductAPIHandler) GetCarbonKernelReleases(w http.ResponseWriter, r *http.Request) {
	items, ok := h.loadKernels(w, r)
	if !ok {
		return
	}
	writeLegacyJSON(w, r, toLegacyKernelList(items))
}

// GetCarbonKernelReleaseByID handles GET
// /carbon-kernel-releases/{id}.
func (h *ProductAPIHandler) GetCarbonKernelReleaseByID(w http.ResponseWriter, r *http.Request) {
	id, valid := legacyID(r.PathValue("id"))
	if !valid {
		writeLegacyEmpty(w, http.StatusBadRequest)
		return
	}
	items, ok := h.loadKernels(w, r)
	if !ok {
		return
	}
	for _, k := range items {
		if int64(k.ID) == id {
			writeLegacyJSON(w, r, toLegacyKernel(k))
			return
		}
	}
	writeLegacyEmpty(w, http.StatusNotFound)
}

// GetCarbonKernelPlatforms handles GET
// /carbon-kernel-releases/platforms — distinct platforms,
// alphabetically (a null platform first, as MySQL sorts it).
func (h *ProductAPIHandler) GetCarbonKernelPlatforms(w http.ResponseWriter, r *http.Request) {
	items, ok := h.loadKernels(w, r)
	if !ok {
		return
	}
	platforms, hasNull := kernelPlatforms(items)
	out := make([]*string, 0, len(platforms)+1)
	if hasNull {
		out = append(out, nil)
	}
	for i := range platforms {
		out = append(out, &platforms[i])
	}
	writeLegacyJSON(w, r, out)
}

// GetCarbonKernelReleasesByPlatform handles GET
// /carbon-kernel-releases/platform/{platform}, newest
// first. The platform must match legacy's [a-zA-Z0-9 \s.-]{1,50}, else 400
// with an empty body.
func (h *ProductAPIHandler) GetCarbonKernelReleasesByPlatform(w http.ResponseWriter, r *http.Request) {
	platform := r.PathValue("platform")
	if strings.TrimSpace(platform) == "" || len(platform) > legacyMaxPlatformLen || !legacyPlatformRe.MatchString(platform) {
		writeLegacyEmpty(w, http.StatusBadRequest)
		return
	}
	items, ok := h.loadKernels(w, r)
	if !ok {
		return
	}
	items = slices.DeleteFunc(items, func(k lpKernelRelease) bool {
		return k.CarbonPlatform == nil || !strings.EqualFold(*k.CarbonPlatform, platform)
	})
	writeLegacyJSON(w, r, toLegacyKernelList(items))
}

// GetLatestKernelByPlatform handles GET
// /carbon-kernel-releases/latest-by-platform — for each
// platform, alphabetically, its newest release.
func (h *ProductAPIHandler) GetLatestKernelByPlatform(w http.ResponseWriter, r *http.Request) {
	items, ok := h.loadKernels(w, r)
	if !ok {
		return
	}
	platforms, _ := kernelPlatforms(items)
	out := make([]legacyKernelRelease, 0, len(platforms))
	for _, p := range platforms {
		for _, k := range items { // newest first, so the first match is the latest
			if k.CarbonPlatform != nil && *k.CarbonPlatform == p {
				out = append(out, toLegacyKernel(k))
				break
			}
		}
	}
	writeLegacyJSON(w, r, out)
}
