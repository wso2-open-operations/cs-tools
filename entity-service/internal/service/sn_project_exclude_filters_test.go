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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNProjectService_SearchProjects_RejectsInvalidExcludeFilters verifies
// that an unknown value in ExcludeClosureStates/ExcludeSubscriptionTypes is
// rejected before any request reaches ServiceNow.
func TestSNProjectService_SearchProjects_RejectsInvalidExcludeFilters(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("upstream should not be called for an invalid filter value")
	}))
	svc := NewServiceNowProjectService(client, nil)

	t.Run("excludeClosureStates", func(t *testing.T) {
		_, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
			Pagination:           domain.Pagination{Limit: 10},
			ExcludeClosureStates: []string{"restricted"}, // lowercase — real values are capitalized
		})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})

	t.Run("excludeSubscriptionTypes", func(t *testing.T) {
		_, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
			Pagination:               domain.Pagination{Limit: 10},
			ExcludeSubscriptionTypes: []domain.SubscriptionType{"not_a_real_type"},
		})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})

	t.Run("excludeProjectKeys", func(t *testing.T) {
		_, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
			Pagination:         domain.Pagination{Limit: 10},
			ExcludeProjectKeys: []string{""},
		})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
		}
	})
}

// snTestProject builds a minimal ServiceNow project-search row for the tests
// below — only the fields fetchProjectsPage/the exclude filters read.
func snTestProject(id, name, key, typeName, closureState string) map[string]any {
	return map[string]any{
		"id": id, "name": name, "key": key,
		"type":    map[string]any{"name": typeName},
		"endDate": "", "createdOn": "2026-01-01 00:00:00",
		"account":      map[string]any{"id": "", "name": ""},
		"closureState": closureState,
	}
}

// TestSNProjectService_SearchProjects_ExcludesByClosureStateAndSubscriptionType
// verifies that projects matching either exclude list are dropped from the
// result, that the reported total reflects the filtered count (not
// ServiceNow's own totalRecords), and that the caller's requested
// offset/limit is applied to the filtered set.
func TestSNProjectService_SearchProjects_ExcludesByClosureStateAndSubscriptionType(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				snTestProject("11111111111111111111111111111111", "Keep — subscription/open", "KEEP1", "Subscription", "Open"),
				snTestProject("22222222222222222222222222222222", "Drop — restricted", "DROP1", "Subscription", "Restricted"),
				snTestProject("33333333333333333333333333333333", "Drop — cloud support", "DROP2", "Cloud Support", "Open"),
				snTestProject("44444444444444444444444444444444", "Drop — suspended cloud eval", "DROP3", "Cloud Evaluation Support", "Suspended"),
				snTestProject("55555555555555555555555555555555", "Keep — managed cloud", "KEEP2", "Managed Cloud Subscription", "Open"),
			},
			"totalRecords": 5, "offset": 0, "limit": 100,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	resp, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination:           domain.Pagination{Limit: 10, Offset: 0},
		ExcludeClosureStates: []string{"Restricted", "Suspended"},
		ExcludeSubscriptionTypes: []domain.SubscriptionType{
			domain.SubscriptionTypeCloudSupport,
			domain.SubscriptionTypeCloudEvaluationSupport,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("Total = %d, want 2 (filtered count, not ServiceNow's totalRecords of 5)", resp.Total)
	}
	if len(resp.Projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(resp.Projects))
	}
	gotKeys := []string{resp.Projects[0].Key, resp.Projects[1].Key}
	if gotKeys[0] != "KEEP1" || gotKeys[1] != "KEEP2" {
		t.Errorf("kept projects = %v, want [KEEP1 KEEP2]", gotKeys)
	}
	if resp.HasMore {
		t.Errorf("HasMore = true, want false (only 2 of 2 filtered results fit the limit 10)")
	}
}

// TestSNProjectService_SearchProjects_ExcludesByProjectKey verifies that
// ExcludeProjectKeys drops matching projects on its own (no closure-state or
// subscription-type filter needed to trigger the filtered path), and that
// matching is exact (a key that merely contains an excluded key as a
// substring is kept).
func TestSNProjectService_SearchProjects_ExcludesByProjectKey(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				snTestProject("11111111111111111111111111111111", "Keep", "KEEP1", "Subscription", "Open"),
				snTestProject("22222222222222222222222222222222", "Drop — Apexia", "APEXIA", "Subscription", "Open"),
				snTestProject("33333333333333333333333333333333", "Drop — Veridian", "VERIDIAN", "Subscription", "Open"),
				snTestProject("44444444444444444444444444444444", "Keep — not an exact match", "APEXIA2", "Subscription", "Open"),
			},
			"totalRecords": 4, "offset": 0, "limit": 100,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	resp, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination:         domain.Pagination{Limit: 10, Offset: 0},
		ExcludeProjectKeys: []string{"APEXIA", "VERIDIAN"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("Total = %d, want 2", resp.Total)
	}
	if len(resp.Projects) != 2 {
		t.Fatalf("got %d projects, want 2: %+v", len(resp.Projects), resp.Projects)
	}
	gotKeys := []string{resp.Projects[0].Key, resp.Projects[1].Key}
	if gotKeys[0] != "KEEP1" || gotKeys[1] != "APEXIA2" {
		t.Errorf("kept projects = %v, want [KEEP1 APEXIA2]", gotKeys)
	}
}

// TestSNProjectService_SearchProjects_ExcludeFiltersPaginateFilteredResult
// verifies that the caller's own offset/limit windows the *filtered* result,
// not ServiceNow's raw page.
func TestSNProjectService_SearchProjects_ExcludeFiltersPaginateFilteredResult(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				snTestProject("11111111111111111111111111111111", "A", "A", "Subscription", "Open"),
				snTestProject("22222222222222222222222222222222", "B (dropped)", "B", "Cloud Support", "Open"),
				snTestProject("33333333333333333333333333333333", "C", "C", "Subscription", "Open"),
				snTestProject("44444444444444444444444444444444", "D", "D", "Subscription", "Open"),
			},
			"totalRecords": 4, "offset": 0, "limit": 100,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	resp, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination:               domain.Pagination{Limit: 2, Offset: 1},
		ExcludeSubscriptionTypes: []domain.SubscriptionType{domain.SubscriptionTypeCloudSupport},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Filtered set is [A, C, D] (B dropped); offset 1, limit 2 -> [C, D].
	if resp.Total != 3 {
		t.Fatalf("Total = %d, want 3", resp.Total)
	}
	if len(resp.Projects) != 2 || resp.Projects[0].Key != "C" || resp.Projects[1].Key != "D" {
		t.Fatalf("Projects = %+v, want [C D]", resp.Projects)
	}
	if resp.HasMore {
		t.Errorf("HasMore = true, want false (offset 1 + 2 returned == total 3)")
	}
}

// TestSNProjectService_SearchProjects_ExcludeFiltersPageThroughUpstream
// verifies that when an exclude filter is set, fetchAllProjectsFiltered pages
// through every ServiceNow result (not just the first internal page) before
// filtering and applying the caller's own pagination window.
func TestSNProjectService_SearchProjects_ExcludeFiltersPageThroughUpstream(t *testing.T) {
	// 5 upstream pages of snExcludeFilterPageSize (maxLimit, 50) each: 250
	// total rows, every 10th one filtered out by closure state, so requesting
	// everything exercises all five round trips.
	const totalUpstream = 250
	var calls int

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload struct {
			Pagination struct {
				Limit  int `json:"limit"`
				Offset int `json:"offset"`
			} `json:"pagination"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		rows := make([]map[string]any, 0, payload.Pagination.Limit)
		for i := 0; i < payload.Pagination.Limit; i++ {
			idx := payload.Pagination.Offset + i
			if idx >= totalUpstream {
				break
			}
			closureState := "Open"
			if idx%10 == 0 {
				closureState = "Restricted"
			}
			id := fmt.Sprintf("%032d", idx)
			rows = append(rows, snTestProject(id, fmt.Sprintf("Project %d", idx), fmt.Sprintf("P%d", idx), "Subscription", closureState))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": rows, "totalRecords": totalUpstream,
			"offset": payload.Pagination.Offset, "limit": payload.Pagination.Limit,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	// Limit is capped at maxLimit (50) — that only windows the *filtered*
	// result (checked below); fetchAllProjectsFiltered still pages through
	// every one of the 250 upstream rows before that window is applied.
	resp, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination:           domain.Pagination{Limit: maxLimit, Offset: 0},
		ExcludeClosureStates: []string{"Restricted"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 5 {
		t.Fatalf("upstream called %d times, want 5 (250 rows / %d-row internal page size)", calls, snExcludeFilterPageSize)
	}
	// 25 of the 250 rows (every 10th) are Restricted and filtered out.
	if resp.Total != 225 {
		t.Fatalf("Total = %d, want 225", resp.Total)
	}
	if len(resp.Projects) != maxLimit {
		t.Fatalf("got %d projects, want %d (the caller's own limit, windowed over the filtered result)", len(resp.Projects), maxLimit)
	}
	for _, p := range resp.Projects {
		if p.ClosureState != nil && *p.ClosureState == "Restricted" {
			t.Fatalf("result contains an excluded Restricted project: %+v", p)
		}
	}
}

// TestSNProjectService_SearchProjects_ExcludeFiltersErrorsRatherThanTruncate
// verifies that when fetchAllProjectsFiltered's safety-bound page count is
// reached before every upstream match has been fetched, SearchProjects
// returns an error rather than silently reporting the partial slice
// collected so far as the complete, authoritative result — a caller
// resolving an announcement audience must never under-count real recipients
// without being told the count is incomplete.
func TestSNProjectService_SearchProjects_ExcludeFiltersErrorsRatherThanTruncate(t *testing.T) {
	// One more row than the safety bound can ever fetch, so the loop always
	// exhausts maxExcludeFilterPages while offset is still short of total.
	const totalUpstream = maxExcludeFilterPages*snExcludeFilterPageSize + 1

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Pagination struct {
				Limit  int `json:"limit"`
				Offset int `json:"offset"`
			} `json:"pagination"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		rows := make([]map[string]any, 0, payload.Pagination.Limit)
		for i := 0; i < payload.Pagination.Limit; i++ {
			idx := payload.Pagination.Offset + i
			if idx >= totalUpstream {
				break
			}
			id := fmt.Sprintf("%032d", idx%1_000_000_000)
			rows = append(rows, snTestProject(id, fmt.Sprintf("Project %d", idx), fmt.Sprintf("P%d", idx), "Subscription", "Open"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": rows, "totalRecords": totalUpstream,
			"offset": payload.Pagination.Offset, "limit": payload.Pagination.Limit,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	_, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination:           domain.Pagination{Limit: maxLimit, Offset: 0},
		ExcludeClosureStates: []string{"Restricted"},
	})
	if err == nil {
		t.Fatal("expected an error when the result set exceeds the safety bound, got nil (silent truncation)")
	}
	var svcErr *apierror.ServiceUnavailableError
	if !errors.As(err, &svcErr) {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}
