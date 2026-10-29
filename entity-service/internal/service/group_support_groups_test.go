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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubGroupRepo records which search ran.
type stubGroupRepo struct{ called string }

// SearchGroups records that the unfiltered group search was called.
func (s *stubGroupRepo) SearchGroups(context.Context, string, int, int) ([]domain.Group, int, error) {
	s.called = "teams"
	return []domain.Group{}, 0, nil
}

// SearchIncidentExtraGroups records that the extra-group search was called.
func (s *stubGroupRepo) SearchIncidentExtraGroups(context.Context, string, int, int) ([]domain.Group, int, error) {
	s.called = "extra groups"
	return []domain.Group{}, 0, nil
}

// SearchSupportGroups records that the support-group search was called.
func (s *stubGroupRepo) SearchSupportGroups(context.Context, string, int, int) ([]domain.Group, int, error) {
	s.called = "support groups"
	return []domain.Group{}, 0, nil
}

// supportGroupsOnly switches the Postgres search from the team registry to the
// incident support-group set; without it the search is unchanged.
func TestSearchGroups_SupportGroupsOnlyPicksTheSupportGroupSet(t *testing.T) {
	for want, filters := range map[string]*domain.SearchGroupsFilters{
		"teams":          nil,
		"support groups": {SupportGroupsOnly: true},
	} {
		repo := &stubGroupRepo{}
		if _, err := NewGroupService(repo).SearchGroups(context.Background(), domain.SearchGroupsRequest{Filters: filters}); err != nil {
			t.Fatalf("SearchGroups: %v", err)
		}
		if repo.called != want {
			t.Errorf("filters %+v searched %s, want %s", filters, repo.called, want)
		}
	}
}

// incidentExtraGroupsOnly picks the extra-group search, wins over
// supportGroupsOnly, and leaves the other searches alone when false.
func TestSearchGroups_IncidentExtraGroupsOnly(t *testing.T) {
	for name, c := range map[string]struct {
		filters *domain.SearchGroupsFilters
		want    string
	}{
		"extra only":       {&domain.SearchGroupsFilters{IncidentExtraGroupsOnly: true}, "extra groups"},
		"both flags":       {&domain.SearchGroupsFilters{IncidentExtraGroupsOnly: true, SupportGroupsOnly: true}, "extra groups"},
		"explicit false":   {&domain.SearchGroupsFilters{IncidentExtraGroupsOnly: false}, "teams"},
		"support only":     {&domain.SearchGroupsFilters{SupportGroupsOnly: true}, "support groups"},
		"extra with query": {&domain.SearchGroupsFilters{IncidentExtraGroupsOnly: true, SearchQuery: "soc"}, "extra groups"},
	} {
		repo := &stubGroupRepo{}
		if _, err := NewGroupService(repo).SearchGroups(context.Background(), domain.SearchGroupsRequest{Filters: c.filters}); err != nil {
			t.Fatalf("%s: SearchGroups: %v", name, err)
		}
		if repo.called != c.want {
			t.Errorf("%s: searched %s, want %s", name, repo.called, c.want)
		}
	}
}

// On ServiceNow the set is the distinct support groups of the services,
// matched, sorted and paged here.
func TestSNSearchGroups_SupportGroupsOnly(t *testing.T) {
	services := []snServiceFixture{
		{sysid: "00000000000000000000000000000001", group: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", groupName: "Zeta SRE"},
		{sysid: "00000000000000000000000000000002", group: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", groupName: "Alpha SRE"},
		{sysid: "00000000000000000000000000000003", group: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", groupName: "Zeta SRE"},
		{sysid: "00000000000000000000000000000004"},
		{sysid: "00000000000000000000000000000005", group: "cccccccccccccccccccccccccccccccc", groupName: "Billing Ops"},
	}
	var body map[string]any
	var lookups int32
	svc := NewServiceNowGroupService(newTestSNClient(t, snCreateCapturingClient(t, services, &body, &lookups)))

	all, err := svc.SearchGroups(contextWithUserIDToken("token"), domain.SearchGroupsRequest{Filters: &domain.SearchGroupsFilters{SupportGroupsOnly: true}})
	if err != nil {
		t.Fatalf("SearchGroups: %v", err)
	}
	if all.Total != 3 || len(all.Groups) != 3 || all.Groups[0].Name != "Alpha SRE" || all.Groups[2].Name != "Zeta SRE" {
		t.Errorf("got %+v, want the 3 distinct groups by name", all)
	}
	if all.Groups[0].ID != sysidToUUID("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb") {
		t.Errorf("id = %s, want a UUID", all.Groups[0].ID)
	}

	sre, err := svc.SearchGroups(contextWithUserIDToken("token"), domain.SearchGroupsRequest{
		Filters:    &domain.SearchGroupsFilters{SupportGroupsOnly: true, SearchQuery: "sre"},
		Pagination: domain.Pagination{Limit: 1, Offset: 1},
	})
	if err != nil {
		t.Fatalf("SearchGroups: %v", err)
	}
	if sre.Total != 2 || len(sre.Groups) != 1 || sre.Groups[0].Name != "Zeta SRE" {
		t.Errorf("got %+v, want page 2 of the 2 SRE groups", sre)
	}
}
