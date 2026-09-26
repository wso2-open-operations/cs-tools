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
	"net/http"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeDeploymentService is a minimal, in-memory DeploymentService stub used
// only to drive SearchProjectsByProductVersion's fetchAllDeploymentProjects
// helper. It paginates over a fixed slice exactly the way a real
// implementation would (offset/limit, HasMore derived from the remainder),
// so the caller's paging loop exercises real page-boundary behaviour.
// CreateDeployment/UpdateDeployment are never called by that code path.
type fakeDeploymentService struct {
	deployments []domain.DeploymentView
	calls       int
}

func (f *fakeDeploymentService) SearchDeployments(_ context.Context, req domain.SearchDeploymentsRequest) (domain.SearchDeploymentsResponse, error) {
	f.calls++
	total := len(f.deployments)
	start := req.Pagination.Offset
	if start > total {
		start = total
	}
	end := start + req.Pagination.Limit
	if end > total {
		end = total
	}
	return domain.SearchDeploymentsResponse{
		Deployments: f.deployments[start:end],
		Total:       total,
		Limit:       req.Pagination.Limit,
		Offset:      req.Pagination.Offset,
		HasMore:     end < total,
	}, nil
}

func (f *fakeDeploymentService) CreateDeployment(context.Context, domain.CreateDeploymentRequest) (domain.CreateDeploymentResponse, error) {
	panic("fakeDeploymentService: CreateDeployment not implemented")
}

func (f *fakeDeploymentService) UpdateDeployment(context.Context, domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error) {
	panic("fakeDeploymentService: UpdateDeployment not implemented")
}

// alwaysMoreDeploymentService always reports HasMore: true and returns a
// full page of throwaway deployments regardless of offset, used only to
// force fetchAllDeploymentProjects' safety bound to trip.
type alwaysMoreDeploymentService struct{}

func (alwaysMoreDeploymentService) SearchDeployments(_ context.Context, req domain.SearchDeploymentsRequest) (domain.SearchDeploymentsResponse, error) {
	views := make([]domain.DeploymentView, req.Pagination.Limit)
	for i := range views {
		views[i] = domain.DeploymentView{
			ID:      fmt.Sprintf("dep-%d-%d", req.Pagination.Offset, i),
			Project: domain.EntityRef{ID: "proj-x", Name: "Project X"},
		}
	}
	return domain.SearchDeploymentsResponse{Deployments: views, Total: 1 << 30, HasMore: true}, nil
}

func (alwaysMoreDeploymentService) CreateDeployment(context.Context, domain.CreateDeploymentRequest) (domain.CreateDeploymentResponse, error) {
	panic("alwaysMoreDeploymentService: CreateDeployment not implemented")
}

func (alwaysMoreDeploymentService) UpdateDeployment(context.Context, domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error) {
	panic("alwaysMoreDeploymentService: UpdateDeployment not implemented")
}

// fakeProjectService is a minimal, in-memory ProjectService stub used only
// to drive SearchProjectsByProductVersion's fetchEligibleProjectIDs helper.
// SearchProjects paginates over a fixed slice the same way fakeDeploymentService
// does. GetProjectByID is never called by that code path.
type fakeProjectService struct {
	projects []domain.ProjectView
}

func (f *fakeProjectService) SearchProjects(_ context.Context, req domain.SearchProjectsRequest) (domain.SearchProjectsResponse, error) {
	total := len(f.projects)
	start := req.Pagination.Offset
	if start > total {
		start = total
	}
	end := start + req.Pagination.Limit
	if end > total {
		end = total
	}
	return domain.SearchProjectsResponse{
		Projects: f.projects[start:end],
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  end < total,
	}, nil
}

func (f *fakeProjectService) GetProjectByID(context.Context, string) (domain.ProjectDetailsView, error) {
	panic("fakeProjectService: GetProjectByID not implemented")
}

// alwaysMoreProjectService always reports HasMore: true and returns a full
// page of throwaway projects regardless of offset, used only to force
// fetchEligibleProjectIDs' safety bound to trip.
type alwaysMoreProjectService struct{}

func (alwaysMoreProjectService) SearchProjects(_ context.Context, req domain.SearchProjectsRequest) (domain.SearchProjectsResponse, error) {
	views := make([]domain.ProjectView, req.Pagination.Limit)
	for i := range views {
		views[i] = domain.ProjectView{ID: fmt.Sprintf("proj-%d-%d", req.Pagination.Offset, i), Name: "irrelevant"}
	}
	return domain.SearchProjectsResponse{Projects: views, Total: 1 << 30, HasMore: true}, nil
}

func (alwaysMoreProjectService) GetProjectByID(context.Context, string) (domain.ProjectDetailsView, error) {
	panic("alwaysMoreProjectService: GetProjectByID not implemented")
}

var (
	testPBVProductSysid   = sysid32('3')
	testPBVVersionSysid   = sysid32('4')
	testPBVOtherProdSysid = sysid32('5')
	testPBVOtherVerSysid  = sysid32('6')
	testPBVProductUUID    = sysidToUUID(testPBVProductSysid)
	testPBVVersionUUID    = sysidToUUID(testPBVVersionSysid)
)

func TestSNDeployedProductService_SearchProjectsByProductVersion_RejectsInvalidProductID(t *testing.T) {
	svc := NewServiceNowDeployedProductService(newTestSNClient(t, http.NewServeMux()), &fakeDeploymentService{}, nil)

	_, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        "not-a-uuid",
		ProductVersionID: testPBVVersionUUID,
	})
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestSNDeployedProductService_SearchProjectsByProductVersion_RejectsInvalidProductVersionID(t *testing.T) {
	svc := NewServiceNowDeployedProductService(newTestSNClient(t, http.NewServeMux()), &fakeDeploymentService{}, nil)

	_, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: "not-a-uuid",
	})
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// deployedProductFixture builds the wire shape for one deployed product
// entry, matching a given deployment/product/version sysid triple.
func deployedProductFixture(id, deploymentSysid, productSysid, versionSysid string) map[string]any {
	return map[string]any{
		"id":         id,
		"deployment": map[string]any{"id": deploymentSysid, "name": "irrelevant"},
		"product":    map[string]any{"id": productSysid, "name": "irrelevant"},
		"version":    map[string]any{"id": versionSysid, "name": "irrelevant"},
		"createdOn":  "2026-01-01 00:00:00",
		"updatedOn":  "2026-01-02 00:00:00",
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_MatchesAndDedupes
// verifies the core join/filter/dedup logic: two deployments belonging to
// the same project both run the matching product+version (must collapse to
// one project entry), a third deployment in a different project also
// matches (must appear as its own entry), and a fourth deployment running a
// different product must be excluded entirely.
func TestSNDeployedProductService_SearchProjectsByProductVersion_MatchesAndDedupes(t *testing.T) {
	depA1, depA2, depB, depOther := sysid32('1'), sysid32('2'), sysid32('7'), sysid32('8')
	projA, projB := domain.EntityRef{ID: "proj-a-uuid", Name: "Project A"}, domain.EntityRef{ID: "proj-b-uuid", Name: "Project B"}

	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(depA1), Project: projA},
		{ID: sysidToUUID(depA2), Project: projA},
		{ID: sysidToUUID(depB), Project: projB},
		{ID: sysidToUUID(depOther), Project: projB},
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		items := []map[string]any{
			deployedProductFixture(sysid32('a'), depA1, testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('b'), depA2, testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('c'), depB, testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('d'), depOther, testPBVOtherProdSysid, testPBVOtherVerSysid),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": len(items), "offset": 0, "limit": 50,
		})
	})

	// Both projects are returned as eligible — this test is about the
	// deployment/deployed-product join and dedup, not the mandatory
	// exclusion, which every SearchProjectsByProductVersion call now applies
	// regardless (see TestSNDeployedProductService_SearchProjectsByProductVersion_IntersectsWithEligibleProjects).
	projectSvc := &fakeProjectService{projects: []domain.ProjectView{
		{ID: projA.ID, Name: projA.Name},
		{ID: projB.ID, Name: projB.Name},
	}}
	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc, projectSvc)

	resp, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("expected 2 distinct matching projects, got %d: %+v", resp.Total, resp.Projects)
	}
	names := map[string]bool{}
	for _, p := range resp.Projects {
		names[p.Name] = true
	}
	if !names["Project A"] || !names["Project B"] {
		t.Fatalf("expected Project A and Project B, got %+v", resp.Projects)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_StableOrderForEqualNames
// guards against a non-deterministic sort: two distinct projects sharing the
// exact same Name must still resolve to a stable order (by ID) rather than
// whatever order they happened to come out of the underlying Go map in.
// Without an ID tiebreaker, a caller paginating across separate requests
// could see the pair swap order between calls — skipping one project on one
// page and duplicating it on another.
func TestSNDeployedProductService_SearchProjectsByProductVersion_StableOrderForEqualNames(t *testing.T) {
	depLow, depHigh := sysid32('1'), sysid32('2')
	// Project IDs are UUIDs derived from sysids '1' and '9' respectively, so
	// projLow.ID < projHigh.ID lexicographically once hyphenated.
	projLow := domain.EntityRef{ID: sysidToUUID(sysid32('1')), Name: "Same Name"}
	projHigh := domain.EntityRef{ID: sysidToUUID(sysid32('9')), Name: "Same Name"}

	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(depLow), Project: projLow},
		{ID: sysidToUUID(depHigh), Project: projHigh},
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		items := []map[string]any{
			deployedProductFixture(sysid32('a'), depLow, testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('b'), depHigh, testPBVProductSysid, testPBVVersionSysid),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": len(items), "offset": 0, "limit": 50,
		})
	})

	projectSvc := &fakeProjectService{projects: []domain.ProjectView{
		{ID: projLow.ID, Name: projLow.Name},
		{ID: projHigh.ID, Name: projHigh.Name},
	}}
	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc, projectSvc)

	resp, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Projects) != 2 {
		t.Fatalf("expected 2 same-named projects, got %d: %+v", len(resp.Projects), resp.Projects)
	}
	if resp.Projects[0].ID != projLow.ID || resp.Projects[1].ID != projHigh.ID {
		t.Fatalf("expected equal-name projects ordered by ID ascending, got %+v", resp.Projects)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_PaginatesDedupedResult
// verifies the caller's own pagination window is applied after, not before,
// deduplication and sorting — a limit/offset here must slice the deduped
// project set, not the raw deployed-product matches.
func TestSNDeployedProductService_SearchProjectsByProductVersion_PaginatesDedupedResult(t *testing.T) {
	deps := []string{sysid32('1'), sysid32('2'), sysid32('3')}
	projects := []domain.EntityRef{
		{ID: "proj-a", Name: "Alpha"},
		{ID: "proj-b", Name: "Bravo"},
		{ID: "proj-c", Name: "Charlie"},
	}

	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(deps[0]), Project: projects[0]},
		{ID: sysidToUUID(deps[1]), Project: projects[1]},
		{ID: sysidToUUID(deps[2]), Project: projects[2]},
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		items := []map[string]any{
			deployedProductFixture(sysid32('a'), deps[0], testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('b'), deps[1], testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('c'), deps[2], testPBVProductSysid, testPBVVersionSysid),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": len(items), "offset": 0, "limit": 50,
		})
	})

	projectSvc := &fakeProjectService{projects: []domain.ProjectView{
		{ID: projects[0].ID, Name: projects[0].Name},
		{ID: projects[1].ID, Name: projects[1].Name},
		{ID: projects[2].ID, Name: projects[2].Name},
	}}
	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc, projectSvc)

	resp, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		Pagination:       domain.Pagination{Limit: 1, Offset: 1},
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 3 || !resp.HasMore || len(resp.Projects) != 1 {
		t.Fatalf("unexpected pagination result: %+v", resp)
	}
	if resp.Projects[0].Name != "Bravo" {
		t.Fatalf("expected the second name-sorted project (Bravo), got %q", resp.Projects[0].Name)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_PagesThroughMultipleDeployedProductPages
// verifies the per-chunk deployed-products loop keeps paging (using its own
// offset) until the upstream's hasMore is false, rather than stopping after
// one page — the match here only appears on the second page.
func TestSNDeployedProductService_SearchProjectsByProductVersion_PagesThroughMultipleDeployedProductPages(t *testing.T) {
	dep1, dep2 := sysid32('1'), sysid32('2')
	proj := domain.EntityRef{ID: "proj-a", Name: "Project A"}

	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(dep1), Project: proj},
		{ID: sysidToUUID(dep2), Project: proj},
	}}

	var requestOffsets []int
	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		var payload snDeployedProductSearchPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requestOffsets = append(requestOffsets, payload.Pagination.Offset)

		var items []map[string]any
		switch payload.Pagination.Offset {
		case 0:
			items = []map[string]any{deployedProductFixture(sysid32('a'), dep1, testPBVOtherProdSysid, testPBVOtherVerSysid)}
		default:
			items = []map[string]any{deployedProductFixture(sysid32('b'), dep2, testPBVProductSysid, testPBVVersionSysid)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": 2, "offset": payload.Pagination.Offset, "limit": 1,
		})
	})

	projectSvc := &fakeProjectService{projects: []domain.ProjectView{{ID: proj.ID, Name: proj.Name}}}
	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc, projectSvc)

	resp, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 || len(resp.Projects) != 1 || resp.Projects[0].Name != "Project A" {
		t.Fatalf("expected the second-page match to surface Project A, got %+v", resp)
	}
	if len(requestOffsets) < 2 {
		t.Fatalf("expected the deployed-products search to be called for more than one page, got offsets %v", requestOffsets)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_DeploymentEnumerationErrorsRatherThanTruncate
// forces fetchAllDeploymentProjects' safety bound to be exceeded (an upstream
// that always claims hasMore: true) and asserts this fails loudly with a
// ServiceUnavailableError rather than silently resolving against a partial,
// incomplete deployment-to-project map.
func TestSNDeployedProductService_SearchProjectsByProductVersion_DeploymentEnumerationErrorsRatherThanTruncate(t *testing.T) {
	svc := NewServiceNowDeployedProductService(newTestSNClient(t, http.NewServeMux()), alwaysMoreDeploymentService{}, nil)

	_, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if _, ok := err.(*apierror.ServiceUnavailableError); !ok {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_IntersectsWithEligibleProjects
// verifies the core requirement behind the mandatory exclusion this endpoint
// always applies: the real EOL-announcement audience is the intersection of
// "projects running this version" and "projects eligible for an announcement
// at all" (per the source-of-truth doc — and the real ServiceNow flow this
// replaces, whose own first step excludes Restricted/Suspended closure
// states and Cloud Support/Cloud Evaluation Support subscriptions
// unconditionally) — not the product-version match alone. Two projects
// match the product+version; only one is returned by the fake
// ProjectService's (already-filtered) eligible set, so only that one must
// survive in the final result. Note the request below carries no exclusion
// fields at all — there is nothing to opt into; this must happen regardless.
func TestSNDeployedProductService_SearchProjectsByProductVersion_IntersectsWithEligibleProjects(t *testing.T) {
	depEligible, depIneligible := sysid32('1'), sysid32('2')
	projEligible := domain.EntityRef{ID: "proj-eligible-uuid", Name: "Eligible Project"}
	projIneligible := domain.EntityRef{ID: "proj-ineligible-uuid", Name: "Ineligible Project"}

	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(depEligible), Project: projEligible},
		{ID: sysidToUUID(depIneligible), Project: projIneligible},
	}}
	// Only the eligible project is returned here — simulates it being the
	// sole survivor of Restricted/Suspended/Cloud-Support exclusion.
	projectSvc := &fakeProjectService{projects: []domain.ProjectView{
		{ID: projEligible.ID, Name: projEligible.Name},
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		items := []map[string]any{
			deployedProductFixture(sysid32('a'), depEligible, testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('b'), depIneligible, testPBVProductSysid, testPBVVersionSysid),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": len(items), "offset": 0, "limit": 50,
		})
	})

	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc, projectSvc)

	resp, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 || len(resp.Projects) != 1 || resp.Projects[0].ID != projEligible.ID {
		t.Fatalf("expected only the eligible project to survive, got %+v", resp.Projects)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_ExcludesContractEndedProjects
// covers the real bug behind this: a project whose subscription contract has
// simply expired (EndDate in the past) is treated as inaccessible by the
// customer portal itself (isProjectSuspended in
// apps/customer-portal/webapp/src/utils/permission.ts), yet often has no
// WSO2 Closure State set at all — so the ClosureState-only exclusion this
// file used to rely on let it straight through. Neither fake project below
// has closureState set to Restricted/Suspended, and the fake ProjectService
// returns both regardless of request filters (it has no notion of end-date
// exclusion — matching the real ServiceNow search, which has no such
// server-side filter either) — so fetchEligibleProjectIDs itself must be the
// one excluding the expired one.
func TestSNDeployedProductService_SearchProjectsByProductVersion_ExcludesContractEndedProjects(t *testing.T) {
	depActive, depExpired := sysid32('1'), sysid32('2')
	projActive := domain.EntityRef{ID: "proj-active-uuid", Name: "Active Project"}
	projExpired := domain.EntityRef{ID: "proj-expired-uuid", Name: "Expired Project"}

	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(depActive), Project: projActive},
		{ID: sysidToUUID(depExpired), Project: projExpired},
	}}

	pastEndDate := time.Now().AddDate(-1, 0, 0)
	projectSvc := &fakeProjectService{projects: []domain.ProjectView{
		{ID: projActive.ID, Name: projActive.Name},
		{ID: projExpired.ID, Name: projExpired.Name, EndDate: &pastEndDate},
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		items := []map[string]any{
			deployedProductFixture(sysid32('a'), depActive, testPBVProductSysid, testPBVVersionSysid),
			deployedProductFixture(sysid32('b'), depExpired, testPBVProductSysid, testPBVVersionSysid),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": len(items), "offset": 0, "limit": 50,
		})
	})

	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc, projectSvc)

	resp, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 || len(resp.Projects) != 1 || resp.Projects[0].ID != projActive.ID {
		t.Fatalf("expected only the active-contract project to survive, got %+v", resp.Projects)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_DeployedProductEnumerationErrorsRatherThanTruncate
// mirrors the test above for the per-chunk deployed-products loop: an
// upstream that always claims hasMore: true for a single deployment's
// deployed-products page must fail loudly once the page bound is exceeded,
// not return an incomplete/wrong match set.
func TestSNDeployedProductService_SearchProjectsByProductVersion_DeployedProductEnumerationErrorsRatherThanTruncate(t *testing.T) {
	dep := sysid32('1')
	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(dep), Project: domain.EntityRef{ID: "proj-a", Name: "Project A"}},
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		items := []map[string]any{deployedProductFixture(sysid32('a'), dep, testPBVOtherProdSysid, testPBVOtherVerSysid)}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": 1 << 30, "offset": 0, "limit": 50,
		})
	})

	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc, nil)

	_, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if _, ok := err.(*apierror.ServiceUnavailableError); !ok {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}

// TestSNDeployedProductService_SearchProjectsByProductVersion_EligibleProjectEnumerationErrorsRatherThanTruncate
// mirrors the other two enumeration-loop safety-bound tests in this file for
// fetchEligibleProjectIDs, now unconditional on every call: an upstream that
// always claims hasMore: true for the eligible-project search must fail
// loudly once the page bound is exceeded, not silently apply an incomplete
// (and therefore wrong) exclusion.
func TestSNDeployedProductService_SearchProjectsByProductVersion_EligibleProjectEnumerationErrorsRatherThanTruncate(t *testing.T) {
	dep := sysid32('1')
	deploymentSvc := &fakeDeploymentService{deployments: []domain.DeploymentView{
		{ID: sysidToUUID(dep), Project: domain.EntityRef{ID: "proj-a", Name: "Project A"}},
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		items := []map[string]any{deployedProductFixture(sysid32('a'), dep, testPBVProductSysid, testPBVVersionSysid)}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": items, "totalRecords": len(items), "offset": 0, "limit": 50,
		})
	})

	svc := NewServiceNowDeployedProductService(newTestSNClient(t, mux), deploymentSvc, alwaysMoreProjectService{})

	_, err := svc.SearchProjectsByProductVersion(contextWithUserIDToken("token"), domain.SearchProjectsByProductVersionRequest{
		ProductID:        testPBVProductUUID,
		ProductVersionID: testPBVVersionUUID,
	})
	if _, ok := err.(*apierror.ServiceUnavailableError); !ok {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}
