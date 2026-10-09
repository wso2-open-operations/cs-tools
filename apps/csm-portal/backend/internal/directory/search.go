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

package directory

import "strings"

const (
	catalogDefaultLimit = 50
	catalogMaxLimit     = 200
)

// Pagination controls which page of a catalogue is returned.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// SearchFilters holds the optional filter criteria both catalogues accept.
type SearchFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
	// Family restricts POST /teams/search to teams whose TeamResult.Family
	// exactly matches (case-insensitive) — e.g. "cre-abt" for the ABT
	// dashboard's team picker. Ignored by SearchRoles. A team with no family
	// configured never matches a non-empty filter.
	Family string `json:"family,omitempty"`
}

// SearchRequest is the body of POST /teams/search and POST /roles/search.
type SearchRequest struct {
	Filters    SearchFilters `json:"filters"`
	Pagination Pagination    `json:"pagination"`
}

// TeamResult is one team as the API exposes it. ID is the registry key, stable
// across environments; the backing group's id is not.
type TeamResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Family may be empty: not every team is classified into a family. It is
	// what SearchRequest.Filters.Family filters on -- a discipline-scoped
	// picker (e.g. an SRE dashboard offering only sre-abt teams) requests it
	// via the frontend's own dashboard-type -> family mapping.
	Family string `json:"family,omitempty"`
	// CreGroupID is the backing CRE (Customer Renewal & Expansion) group's id
	// in this platform's UUID form, suitable for the case-search creTeam
	// filter. Omitted when the registry configured no backing CRE group id
	// for this team -- the team is still listed, just not filter-scopable.
	CreGroupID string `json:"creGroupId,omitempty"`
	// SreGroupID is the backing SRE (Site Reliability Engineering) group's id
	// in this platform's UUID form, suitable for the case-search sreTeam
	// filter. Omitted when the registry configured no backing SRE group id
	// for this team -- the team is still listed, just not filter-scopable.
	SreGroupID string `json:"sreGroupId,omitempty"`
}

// SearchTeamsResponse is the paginated result of a team search.
type SearchTeamsResponse struct {
	Teams  []TeamResult `json:"teams"`
	Total  int          `json:"total"`
	Offset int          `json:"offset"`
	Limit  int          `json:"limit"`
}

// RoleResult is one assignable platform role.
type RoleResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SearchRolesResponse is the paginated result of a role search.
type SearchRolesResponse struct {
	Roles  []RoleResult `json:"roles"`
	Total  int          `json:"total"`
	Offset int          `json:"offset"`
	Limit  int          `json:"limit"`
}

// SearchTeams serves the team catalogue entirely from the startup-resolved
// index: no upstream call, on this request or any other. SearchQuery matching
// is a case-insensitive substring of either the display name or the key;
// Family matching is a case-insensitive exact match against TeamResult.Family
// (a discipline-scoped picker, e.g. an SRE dashboard, passes "sre-abt" to
// exclude every other family, including teams with no family at all).
func (d *Directory) SearchTeams(req SearchRequest) SearchTeamsResponse {
	teams := d.teamResults
	if q := strings.TrimSpace(req.Filters.SearchQuery); q != "" {
		needle := strings.ToLower(q)
		filtered := make([]TeamResult, 0, len(teams))
		for _, t := range teams {
			if strings.Contains(strings.ToLower(t.Name), needle) ||
				strings.Contains(strings.ToLower(t.ID), needle) {
				filtered = append(filtered, t)
			}
		}
		teams = filtered
	}
	if fam := strings.TrimSpace(req.Filters.Family); fam != "" {
		filtered := make([]TeamResult, 0, len(teams))
		for _, t := range teams {
			if strings.EqualFold(t.Family, fam) {
				filtered = append(filtered, t)
			}
		}
		teams = filtered
	}

	total := len(teams)
	offset, limit, length := clampCatalogPagination(req.Pagination, total)

	// Copy the page out: teamResults is the shared startup snapshot, and a
	// sub-slice of it would let a caller's later append scribble on it.
	page := make([]TeamResult, length)
	copy(page, teams[offset:offset+length])

	return SearchTeamsResponse{Teams: page, Total: total, Offset: offset, Limit: limit}
}

// ClampPagination applies the catalogue's default and maximum page size and a
// non-negative offset to a caller-supplied page.
func ClampPagination(p Pagination) Pagination {
	if p.Limit <= 0 {
		p.Limit = catalogDefaultLimit
	}
	if p.Limit > catalogMaxLimit {
		p.Limit = catalogMaxLimit
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}

// SearchRoles serves the assignable-role catalogue from the startup-resolved
// list, with the same no-upstream-call guarantee as SearchTeams.
func (d *Directory) SearchRoles(req SearchRequest) SearchRolesResponse {
	roles := d.roleResults
	if q := strings.TrimSpace(req.Filters.SearchQuery); q != "" {
		needle := strings.ToLower(q)
		filtered := make([]RoleResult, 0, len(roles))
		for _, r := range roles {
			if strings.Contains(strings.ToLower(r.ID), needle) ||
				strings.Contains(strings.ToLower(r.Name), needle) {
				filtered = append(filtered, r)
			}
		}
		roles = filtered
	}

	total := len(roles)
	offset, limit, length := clampCatalogPagination(req.Pagination, total)

	page := make([]RoleResult, length)
	copy(page, roles[offset:offset+length])

	return SearchRolesResponse{Roles: page, Total: total, Offset: offset, Limit: limit}
}

// clampCatalogPagination bounds an offset/limit pair against a known total. It
// returns the offset, the effective page size to echo back to the caller, and a
// length that is always safe to slice with. The two differ on the last page:
// echoing the length as the limit would tell a caller advancing by limit that
// the page size shrank, which no other search here does.
func clampCatalogPagination(p Pagination, total int) (offset, limit, length int) {
	limit = p.Limit
	if limit <= 0 {
		limit = catalogDefaultLimit
	}
	if limit > catalogMaxLimit {
		limit = catalogMaxLimit
	}

	offset = p.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}

	length = limit
	if offset+length > total {
		length = total - offset
	}
	return offset, limit, length
}
