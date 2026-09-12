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

import (
	"strings"
	"testing"
)

// registryFixture exercises all three legal row shapes at once: four fields,
// three, and two. Rows with only two or three fields are not hypothetical --
// the deployed registry has several.
const registryFixture = "alpha|Alpha Team|CRE-ABT|aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa," +
	"beta|Beta Team|sre-abt," +
	"gamma|Gamma Team"

func mustDirectory(t *testing.T, registry, roles string) *Directory {
	t.Helper()
	teams, err := ParseTeamRegistry(registry)
	if err != nil {
		t.Fatalf("ParseTeamRegistry(%q): %v", registry, err)
	}
	parsedRoles, err := ParseRoles(roles)
	if err != nil {
		t.Fatalf("ParseRoles(%q): %v", roles, err)
	}
	dir, err := New(teams, parsedRoles)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return dir
}

// TestStartupResolution_BuildsTheExpectedIndex is the core of the move: one
// parse at startup has to produce every lookup the request path needs, with the
// backing group id already in this platform's UUID form.
func TestStartupResolution_BuildsTheExpectedIndex(t *testing.T) {
	dir := mustDirectory(t, registryFixture, "")

	if dir.TeamCount() != 3 {
		t.Fatalf("TeamCount = %d, want 3", dir.TeamCount())
	}

	team, ok := dir.TeamByKey("alpha")
	if !ok {
		t.Fatal("TeamByKey(alpha) missed")
	}
	if team.Name != "Alpha Team" || team.Family != FamilyCREAbt {
		t.Fatalf("alpha = %+v, want the configured display name and family", team)
	}

	// The reverse direction is what /users/me team resolution runs on.
	byName, ok := dir.TeamByGroupName("Beta Team")
	if !ok || byName.Key != "beta" || byName.Family != FamilySREAbt {
		t.Fatalf("TeamByGroupName(Beta Team) = %+v/%v, want the beta row", byName, ok)
	}

	if _, ok := dir.TeamByGroupName("Not A Team"); ok {
		t.Fatal("TeamByGroupName matched a group that is not in the registry")
	}

	names := dir.GroupNames()
	if len(names) != 3 || names[0] != "Alpha Team" {
		t.Fatalf("GroupNames = %v, want one name per configured team", names)
	}

	resp := dir.SearchTeams(SearchRequest{})
	if resp.Teams[0].CreGroupID != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Errorf("creGroupId = %q, want the UUID form resolved at startup", resp.Teams[0].CreGroupID)
	}
}

// TestRowsWithoutAGroupIDAreStillUsable: three of the deployed rows carry no
// backing group id. They must list, filter and resolve like any other team --
// only the case-search scoping id is missing.
func TestRowsWithoutAGroupIDAreStillUsable(t *testing.T) {
	dir := mustDirectory(t, registryFixture, "")
	resp := dir.SearchTeams(SearchRequest{})

	for _, want := range []string{"beta", "gamma"} {
		var found *TeamResult
		for i := range resp.Teams {
			if resp.Teams[i].ID == want {
				found = &resp.Teams[i]
			}
		}
		if found == nil {
			t.Fatalf("team %q is missing from the catalogue", want)
		}
		if found.CreGroupID != "" {
			t.Errorf("team %q creGroupId = %q, want it omitted", want, found.CreGroupID)
		}
		if _, ok := dir.TeamByGroupName(found.Name); !ok {
			t.Errorf("team %q cannot be resolved by group name", want)
		}
	}

	// A two-field row has no family, and that is legal.
	gamma, _ := dir.TeamByKey("gamma")
	if gamma.Family != "" {
		t.Errorf("gamma family = %q, want empty", gamma.Family)
	}
}

// TestParseTeamRegistry_BadRowsNameTheOffender: a bad row must stop the deploy
// and say which row it was, because a silently dropped team fails nowhere
// visible.
func TestParseTeamRegistry_BadRowsNameTheOffender(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantRow string
	}{
		{"unknown family", "alpha|Alpha Team|sre_abt", "sre_abt"},
		{"empty team key", "alpha|Alpha Team,|Beta Team", "|Beta Team"},
		{"empty display name", "alpha|Alpha Team,beta|", "beta|"},
		{"too many fields", "alpha|Alpha Team|cre|id|extra", "extra"},
		{"single field", "alpha", "alpha"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTeamRegistry(tc.raw)
			if err == nil {
				t.Fatalf("ParseTeamRegistry(%q) = nil error, want a failure", tc.raw)
			}
			if !strings.Contains(err.Error(), tc.wantRow) {
				t.Errorf("error %q does not name the offending row %q", err, tc.wantRow)
			}
			if !strings.Contains(err.Error(), "row ") {
				t.Errorf("error %q does not give the row number", err)
			}
		})
	}
}

// A blank row is skipped, so a trailing comma is not a deploy failure.
func TestParseTeamRegistry_TolerantOfBlankRows(t *testing.T) {
	teams, err := ParseTeamRegistry("alpha|Alpha Team,, ,beta|Beta Team,")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("got %d teams, want 2", len(teams))
	}
}

// Whitespace around a field survives a copy-paste into a configuration form.
func TestParseTeamRegistry_TrimsWhitespace(t *testing.T) {
	teams, err := ParseTeamRegistry(" alpha | Alpha Team | CRE ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if teams[0].Key != "alpha" || teams[0].Name != "Alpha Team" || teams[0].Family != FamilyCRE {
		t.Fatalf("team = %+v, want every field trimmed", teams[0])
	}
}

// A duplicate key or name would be swallowed by the lookup maps, shadowing one
// row with the other. Fail the startup instead.
func TestNew_RejectsDuplicates(t *testing.T) {
	dupKey, _ := ParseTeamRegistry("alpha|Alpha Team,alpha|Another Team")
	if _, err := New(dupKey, DefaultRoles); err == nil {
		t.Error("duplicate team key was accepted")
	}
	dupName, _ := ParseTeamRegistry("alpha|Alpha Team,beta|Alpha Team")
	if _, err := New(dupName, DefaultRoles); err == nil {
		t.Error("duplicate display name was accepted")
	}
	// Two teams configured with the same backing CreGroupID would shadow each
	// other in byCreGroupID exactly as a duplicate key would in byKey.
	dupGroupID, _ := ParseTeamRegistry(
		"alpha|Alpha Team||aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa,beta|Beta Team||aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if _, err := New(dupGroupID, DefaultRoles); err == nil {
		t.Error("duplicate creGroupId was accepted")
	}
	// Same reasoning, for the parallel SreGroupID/bySreGroupID map. The
	// 5-field form (key|name|family||sreGroupId) is needed to land the id in
	// SreGroupID's slot rather than CreGroupID's.
	dupSreGroupID, _ := ParseTeamRegistry(
		"alpha|Alpha Team|||bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb," +
			"beta|Beta Team|||bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if _, err := New(dupSreGroupID, DefaultRoles); err == nil {
		t.Error("duplicate sreGroupId was accepted")
	}
}

// An unconfigured registry is legal: the deployment still has to start.
func TestEmptyRegistryIsLegal(t *testing.T) {
	dir := mustDirectory(t, "", "")
	if dir.TeamCount() != 0 {
		t.Fatalf("TeamCount = %d, want 0", dir.TeamCount())
	}
	resp := dir.SearchTeams(SearchRequest{})
	if resp.Total != 0 || resp.Teams == nil {
		t.Fatalf("SearchTeams = %+v, want an empty non-nil catalogue", resp)
	}
}

// TestIsValidRole_RejectsUnknownRoles is the validation that came across with
// the allow-list; without it an unknown role returns a confidently empty page.
func TestIsValidRole_RejectsUnknownRoles(t *testing.T) {
	dir := mustDirectory(t, "", "agent,timecard_approver")

	if !dir.IsValidRole("agent") {
		t.Error("configured role \"agent\" was rejected")
	}
	if dir.IsValidRole("admin") {
		t.Error("\"admin\" was accepted, but it is not in the configured allow-list")
	}
	if dir.IsValidRole("not_a_role") {
		t.Error("an unknown role was accepted")
	}

	// Unconfigured falls back to the committed defaults, which do include admin.
	if !mustDirectory(t, "", "").IsValidRole("admin") {
		t.Error("the default allow-list should include admin")
	}
}

func TestParseRoles_RejectsDuplicates(t *testing.T) {
	if _, err := ParseRoles("agent,admin,agent"); err == nil {
		t.Error("a duplicate role was accepted")
	} else if !strings.Contains(err.Error(), "agent") {
		t.Errorf("error %q does not name the offending role", err)
	}
}

func TestSearchRoles_ServesTheConfiguredList(t *testing.T) {
	got := mustDirectory(t, "", "timecard_approver,agent").SearchRoles(SearchRequest{})
	if got.Total != 2 {
		t.Fatalf("total = %d, want 2", got.Total)
	}
	// Sorted by id, independent of configured order.
	if got.Roles[0].ID != "agent" || got.Roles[1].ID != "timecard_approver" {
		t.Fatalf("roles = %+v, want them sorted by id", got.Roles)
	}
	if got.Roles[1].Name != "Timecard Approver" {
		t.Errorf("name = %q, want a humanised display name", got.Roles[1].Name)
	}
}

// The echoed limit is the requested page size, not the page length: a caller
// advancing by limit must not be told the page shrank.
func TestClampCatalogPagination(t *testing.T) {
	tests := []struct {
		name                           string
		in                             Pagination
		total                          int
		wantOffset, wantLimit, wantLen int
	}{
		{"requested limit exceeds total", Pagination{Limit: 50}, 10, 0, 50, 10},
		{"full page", Pagination{Limit: 5}, 10, 0, 5, 5},
		{"last partial page", Pagination{Limit: 5, Offset: 8}, 10, 8, 5, 2},
		{"unset limit defaults", Pagination{}, 10, 0, catalogDefaultLimit, 10},
		{"limit above cap is clamped", Pagination{Limit: 5000}, 10, 0, catalogMaxLimit, 10},
		{"negative offset floors at zero", Pagination{Limit: 5, Offset: -3}, 10, 0, 5, 5},
		{"offset past total yields empty page", Pagination{Limit: 5, Offset: 99}, 10, 10, 5, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			offset, limit, length := clampCatalogPagination(tc.in, tc.total)
			if offset != tc.wantOffset || limit != tc.wantLimit || length != tc.wantLen {
				t.Errorf("clampCatalogPagination(%+v, %d) = (%d, %d, %d), want (%d, %d, %d)",
					tc.in, tc.total, offset, limit, length, tc.wantOffset, tc.wantLimit, tc.wantLen)
			}
		})
	}
}

func TestSearchTeams_FiltersOnNameOrKey(t *testing.T) {
	dir := mustDirectory(t, registryFixture, "")

	if got := dir.SearchTeams(SearchRequest{Filters: SearchFilters{SearchQuery: "beta"}}); got.Total != 1 {
		t.Errorf("searchQuery=beta matched %d teams, want 1", got.Total)
	}
	if got := dir.SearchTeams(SearchRequest{Filters: SearchFilters{SearchQuery: "TEAM"}}); got.Total != 3 {
		t.Errorf("searchQuery=TEAM matched %d teams, want all 3 (case-insensitive)", got.Total)
	}
}

func TestSearchTeams_FiltersOnFamily(t *testing.T) {
	dir := mustDirectory(t, registryFixture, "")

	if got := dir.SearchTeams(SearchRequest{Filters: SearchFilters{Family: "cre-abt"}}); got.Total != 1 || got.Teams[0].ID != "alpha" {
		t.Fatalf("family=cre-abt matched %+v, want just alpha", got.Teams)
	}
	// Case-insensitive: the registry row spelled it "CRE-ABT" (see
	// registryFixture), and parseFamily normalizes storage to lowercase, but a
	// caller filtering with either case must match.
	if got := dir.SearchTeams(SearchRequest{Filters: SearchFilters{Family: "CRE-ABT"}}); got.Total != 1 {
		t.Errorf("family=CRE-ABT (uppercase) matched %d teams, want 1", got.Total)
	}
	if got := dir.SearchTeams(SearchRequest{Filters: SearchFilters{Family: "sre-abt"}}); got.Total != 1 || got.Teams[0].ID != "beta" {
		t.Fatalf("family=sre-abt matched %+v, want just beta", got.Teams)
	}
	// gamma has no family at all -- a family filter must exclude it, not treat
	// an empty Family field as a wildcard match.
	if got := dir.SearchTeams(SearchRequest{Filters: SearchFilters{Family: "cre"}}); got.Total != 0 {
		t.Errorf("family=cre matched %d teams, want 0 (none of alpha/beta/gamma is plain cre)", got.Total)
	}
	// No family filter at all: every team, same as before this field existed.
	if got := dir.SearchTeams(SearchRequest{}); got.Total != 3 {
		t.Errorf("no family filter matched %d teams, want all 3", got.Total)
	}
	// Combined with searchQuery: both must match.
	if got := dir.SearchTeams(SearchRequest{Filters: SearchFilters{SearchQuery: "beta", Family: "cre-abt"}}); got.Total != 0 {
		t.Errorf("searchQuery=beta AND family=cre-abt matched %d teams, want 0 (beta is sre-abt)", got.Total)
	}
}

// A returned page must not alias the startup snapshot, or a caller appending to
// it would corrupt every later response.
func TestSearchTeams_PageDoesNotAliasTheSnapshot(t *testing.T) {
	dir := mustDirectory(t, registryFixture, "")

	page := dir.SearchTeams(SearchRequest{})
	page.Teams[0].Name = "mutated"

	if again := dir.SearchTeams(SearchRequest{}); again.Teams[0].Name == "mutated" {
		t.Fatal("mutating a returned page changed the startup snapshot")
	}
}

// TeamByUUID is the reverse of the id -> UUID conversion done at startup: a
// caller holding the platform UUID form of a team's backing CRE group id (the
// same form TeamResult.CreGroupID and accounts.creTeam.id expose) must be
// able to resolve the team from it.
func TestTeamByUUID_ResolvesTheConfiguredTeam(t *testing.T) {
	dir := mustDirectory(t, registryFixture, "")

	// alpha's fixture CreGroupID "aaaa...aaaa" (32 hex chars) resolves to this
	// UUID -- see TestStartupResolution_BuildsTheExpectedIndex.
	team, ok := dir.TeamByUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	if !ok {
		t.Fatal("TeamByUUID missed the configured alpha team")
	}
	if team.Key != "alpha" {
		t.Fatalf("TeamByUUID resolved %+v, want the alpha row", team)
	}

	// A well-formed UUID that matches no configured team's group id.
	if _, ok := dir.TeamByUUID("ffffffff-ffff-ffff-ffff-ffffffffffff"); ok {
		t.Error("TeamByUUID matched a UUID no team is configured with")
	}

	// beta and gamma have no CreGroupID configured at all, so they have no
	// UUID to be looked up by -- confirm neither is reachable through some
	// unintended fallback.
	if _, ok := dir.TeamByUUID(""); ok {
		t.Error("TeamByUUID matched the empty string against an unconfigured group id")
	}
}

// An SRE-only team -- no CreGroupID configured at all, e.g. a real SRE ABT
// like Apollo -- must still resolve through TeamByUUID via its SreGroupID.
// Before bySreGroupID existed, TeamByUUID only ever checked byCreGroupID, so
// such a team's UUID could never be found regardless of how correctly its
// SreGroupID was configured -- this is the regression this test guards.
func TestTeamByUUID_ResolvesSreOnlyTeam(t *testing.T) {
	dir := mustDirectory(t, "delta|Delta Team|sre-abt||bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "")

	team, ok := dir.TeamByUUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	if !ok {
		t.Fatal("TeamByUUID missed the SRE-only delta team")
	}
	if team.Key != "delta" {
		t.Fatalf("TeamByUUID resolved %+v, want the delta row", team)
	}

	// A well-formed UUID that matches no configured team's group id, CRE or
	// SRE.
	if _, ok := dir.TeamByUUID("ffffffff-ffff-ffff-ffff-ffffffffffff"); ok {
		t.Error("TeamByUUID matched a UUID no team is configured with")
	}
}

// Regression: sourceIDToUUID preserved the configured id's case, but canonical
// UUID text is lowercase and this value is compared against ids the entity
// service renders -- an uppercase configured id produced a creGroupId/sreGroupId
// that matched nothing on the creTeam/sreTeam filters, with no error anywhere.
func TestSourceIDToUUID_LowercasesTheConvertedID(t *testing.T) {
	const upper = "760E87B247C13910A0A29CD3846D4301"
	const lower = "760e87b247c13910a0a29cd3846d4301"

	got := sourceIDToUUID(upper)
	want := sourceIDToUUID(lower)
	if got != want {
		t.Fatalf("uppercase id converted to %q, want %q (same as its lowercase form)", got, want)
	}
	if got != "760e87b2-47c1-3910-a0a2-9cd3846d4301" {
		t.Fatalf("got %q, want canonical lowercase UUID text", got)
	}
}
