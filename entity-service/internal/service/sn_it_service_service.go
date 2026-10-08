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
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snITServicesResponse mirrors the Choreo POST /services/search response.
type snITServicesResponse struct {
	Services     []snITService `json:"services"`
	TotalRecords int           `json:"totalRecords"`
	Offset       int           `json:"offset"`
	Limit        int           `json:"limit"`
}

type snITService struct {
	ID                    string            `json:"id"`
	Name                  *string           `json:"name"`
	Class                 *string           `json:"class"`
	BusinessCriticality   *snITServiceLabel `json:"businessCriticality"`
	ServiceClassification *snITServiceLabel `json:"serviceClassification"`
	SupportGroup          *snITServiceLabel `json:"supportGroup"`
}

type snITServiceLabel struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// snITServiceSearchPayload is the Choreo POST /services/search request body.
type snITServiceSearchPayload struct {
	Filters    snITServiceFilters  `json:"filters"`
	Pagination snProjectPagination `json:"pagination"`
}

type snITServiceFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

type snITServiceService struct {
	client *integrationservice.Client
}

// NewServiceNowITServiceService constructs an ITServiceService backed by the Choreo API.
func NewServiceNowITServiceService(client *integrationservice.Client) ITServiceService {
	return &snITServiceService{client: client}
}

// snITServiceRankPages bounds how many upstream pages SearchITServices fetches
// to rank one query: snITServiceRankPages * maxLimit (250) matches. A query
// that matches more than that is ranked among the first 250 the upstream
// returns, which is still far better than the first 20; the caller narrows by
// typing more.
const snITServiceRankPages = 5

// snITServiceRankWindow is the number of matches a ranked search covers.
const snITServiceRankWindow = snITServiceRankPages * maxLimit

// snITServiceClassOffering is the CMDB class of a service offering, which the
// upstream returns alongside real services in the same list.
const snITServiceClassOffering = "service_offering"

// SearchITServices implements ITServiceService.
//
// ServiceNow's POST /services/search filters on `name CONTAINS searchQuery` and
// returns matches in creation order, newest first, 50 at most per page. A type-
// ahead that shows the first 20 of that therefore cannot reach an old service
// whose name is also the prefix of many newer ones: searching "Choreo" matches
// 55 records, 45 of them "Choreo EU - ... Service Offering" rows, and the
// "Choreo" service itself is the 55th. Incidents raised against an offering
// instead of the service have no support group, so they were never assigned to
// the SRE team and never alerted.
//
// A non-empty query is therefore ranked here: the matches (up to
// snITServiceRankWindow of them) are fetched, ordered exact name, then name
// prefix, then word prefix, then anywhere in the name, and within each tier
// services before service offerings, and only then is the requested page cut
// from that order. Total is still the upstream's count of matches.
//
// An empty query is passed straight through (there is nothing to rank by), and
// so is a page that starts past the ranked window, so a caller that walks the
// whole result page by page (the SRE alert service resolves a label that way)
// still reaches every match: the window holds exactly the first
// snITServiceRankWindow upstream matches, so the pages after it continue where
// it ends with nothing repeated and nothing skipped. The one page that crosses
// the window's end is completed from the rows after it (one more call), so
// every page is full whatever its size.
func (s *snITServiceService) SearchITServices(ctx context.Context, req domain.SearchITServicesRequest) (domain.SearchITServicesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchITServicesResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	// Trimmed before it goes upstream too: `name CONTAINS " choreo"` finds
	// nothing, and the portal's own trim is not something this endpoint can
	// rely on from every caller.
	var filters snITServiceFilters
	if req.Filters != nil {
		filters.SearchQuery = strings.TrimSpace(req.Filters.SearchQuery)
	}
	offset, limit := req.Pagination.Offset, req.Pagination.Limit

	query := filters.SearchQuery
	if query == "" || offset >= snITServiceRankWindow {
		page, err := s.fetchITServicePage(ctx, token, filters, offset, limit)
		if err != nil {
			return domain.SearchITServicesResponse{}, err
		}
		return buildITServicesResponse(page.Services, page.TotalRecords, req.Pagination), nil
	}

	window, total, err := s.fetchITServiceRankWindow(ctx, token, filters)
	if err != nil {
		return domain.SearchITServicesResponse{}, err
	}
	rankITServices(window, query)

	// A page that runs past the rows the window holds is completed from the
	// upstream rows right after the window, which is exactly where a page
	// starting at the window's end begins. Without this it would be short, and
	// a caller that steps by its page size (rather than by how many rows came
	// back) would skip the rows between the short page and its next offset.
	// "Past the rows the window holds" covers both a page crossing the nominal
	// end (250) and one ending at it when deduplication left the window a row or
	// two short; the missing rows are the ones that slid off the end of the
	// shifted last page, i.e. the ones right after it.
	if wantEnd := offset + limit; wantEnd > len(window) && total > snITServiceRankWindow {
		// Never more than the upstream's page ceiling, which it answers with an
		// error: an upstream holding far fewer rows than its count claims leaves
		// a large shortfall that no single page can fill.
		need := min(wantEnd-len(window), maxLimit)
		tail, err := s.fetchITServicePage(ctx, token, filters, snITServiceRankWindow, need)
		if err != nil {
			return domain.SearchITServicesResponse{}, err
		}
		window = append(window, tail.Services...)
	}

	end := min(offset+limit, len(window))
	var page []snITService
	if offset < end {
		page = window[offset:end]
	}
	return buildITServicesResponse(page, total, req.Pagination), nil
}

// fetchITServicePage calls the upstream search for one page.
func (s *snITServiceService) fetchITServicePage(ctx context.Context, token string, filters snITServiceFilters, offset, limit int) (snITServicesResponse, error) {
	payload := snITServiceSearchPayload{
		Filters:    filters,
		Pagination: snProjectPagination{Limit: limit, Offset: offset},
	}
	raw, err := s.client.Post(ctx, "/services/search", token, payload)
	if err != nil {
		return snITServicesResponse{}, err
	}
	var snResp snITServicesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return snITServicesResponse{}, fmt.Errorf("sn services: parse response: %w", err)
	}
	return snResp, nil
}

// fetchITServiceRankWindow returns the first snITServiceRankWindow matches of
// the query, in the upstream's own order, and the upstream's total match count.
// The first page tells how many more pages exist; those are fetched
// concurrently. A short first page means the upstream has no more.
func (s *snITServiceService) fetchITServiceRankWindow(ctx context.Context, token string, filters snITServiceFilters) ([]snITService, int, error) {
	first, err := s.fetchITServicePage(ctx, token, filters, 0, maxLimit)
	if err != nil {
		return nil, 0, err
	}
	window := first.Services
	total := first.TotalRecords
	if len(first.Services) < maxLimit || total <= maxLimit {
		return window, total, nil
	}

	pages := min(snITServiceRankPages, (total+maxLimit-1)/maxLimit)
	rest := make([][]snITService, pages-1)
	eg, egCtx := errgroup.WithContext(ctx)
	for i := range rest {
		eg.Go(func() error {
			page, err := s.fetchITServicePage(egCtx, token, filters, (i+1)*maxLimit, maxLimit)
			if err != nil {
				return err
			}
			rest[i] = page.Services
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}
	for _, page := range rest {
		window = append(window, page...)
	}
	return dedupeITServices(window), total, nil
}

// dedupeITServices drops a service that appears twice, keeping its first
// place. The pages are fetched at slightly different moments, so a record
// created in between shifts the later pages by one and repeats the row at the
// seam; one call to the upstream could never do that.
func dedupeITServices(services []snITService) []snITService {
	seen := make(map[string]struct{}, len(services))
	out := services[:0]
	for _, svc := range services {
		if _, dup := seen[svc.ID]; dup {
			continue
		}
		seen[svc.ID] = struct{}{}
		out = append(out, svc)
	}
	return out
}

// rankITServices orders services in place, best match for query first. The
// sort is stable, so equally ranked services keep the upstream's order.
func rankITServices(services []snITService, query string) {
	q := normalizeITServiceName(query)
	type ranked struct {
		svc  snITService
		tier int
		off  bool
	}
	rows := make([]ranked, len(services))
	for i, svc := range services {
		rows[i] = ranked{
			svc:  svc,
			tier: itServiceMatchTier(normalizeITServiceName(derefString(svc.Name)), q),
			off:  svc.Class != nil && strings.EqualFold(*svc.Class, snITServiceClassOffering),
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].tier != rows[j].tier {
			return rows[i].tier < rows[j].tier
		}
		return !rows[i].off && rows[j].off
	})
	for i := range rows {
		services[i] = rows[i].svc
	}
}

// itServiceMatchTier says how well a normalized name matches a normalized
// query: 0 exact, 1 starts with it, 2 a word of the name starts with it, 3 it
// appears elsewhere in the name (or the match was on something else).
func itServiceMatchTier(name, query string) int {
	switch {
	case name == query:
		return 0
	case strings.HasPrefix(name, query):
		return 1
	case strings.Contains(name, " "+query), strings.Contains(name, "-"+query):
		return 2
	default:
		return 3
	}
}

// normalizeITServiceName lower-cases s and collapses runs of whitespace, so
// "Choreo  EU - DP" (two spaces, as one real offering is spelled) compares like
// the single-spaced form.
func normalizeITServiceName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// buildITServicesResponse maps upstream services to the domain response.
func buildITServicesResponse(upstream []snITService, total int, p domain.Pagination) domain.SearchITServicesResponse {
	services := make([]domain.ITService, 0, len(upstream))
	for _, svc := range upstream {
		item := domain.ITService{
			ID:    sysidToUUID(svc.ID),
			Name:  svc.Name,
			Class: svc.Class,
		}
		if svc.BusinessCriticality != nil {
			if bc, ok := snBusinessCriticalityIDToEnum[svc.BusinessCriticality.ID]; ok {
				item.BusinessCriticality = &bc
			}
		}
		if svc.ServiceClassification != nil {
			if sc, ok := snServiceClassificationLabelToEnum[svc.ServiceClassification.Label]; ok {
				item.ServiceClassification = &sc
			}
		}
		if svc.SupportGroup != nil {
			item.SupportGroup = &domain.EntityRef{
				ID:   sysidToUUID(svc.SupportGroup.ID),
				Name: svc.SupportGroup.Label,
			}
		}
		services = append(services, item)
	}

	return domain.SearchITServicesResponse{
		Services: services,
		Total:    total,
		Limit:    p.Limit,
		Offset:   p.Offset,
	}
}

// snBusinessCriticalityIDToEnum maps ServiceNow business criticality IDs to domain enums.
var snBusinessCriticalityIDToEnum = map[string]domain.BusinessCriticality{
	"1": domain.BusinessCriticalityMostCritical,
	"2": domain.BusinessCriticalitySomewhatCritical,
	"3": domain.BusinessCriticalityLessCritical,
	"4": domain.BusinessCriticalityNotCritical,
}

// snServiceClassificationLabelToEnum maps ServiceNow service classification labels to domain enums.
var snServiceClassificationLabelToEnum = map[string]domain.ServiceClassification{
	"Business Service":              domain.ServiceClassificationBusinessService,
	"Technology Management Service": domain.ServiceClassificationTechnologyManagementService,
	"Application Service":           domain.ServiceClassificationApplicationService,
}
