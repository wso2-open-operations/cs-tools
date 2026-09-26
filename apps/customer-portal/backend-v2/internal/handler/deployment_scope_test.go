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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

const (
	scopeProjectID      = "11111111-1111-1111-1111-111111111111"
	scopeOtherProjectID = "33333333-3333-3333-3333-333333333333"
	scopeDeploymentID   = "22222222-2222-2222-2222-222222222222"
)

// scopeFakeEntity serves a fixed set of deployments per project, so a test can
// describe what the caller is allowed to see and then assert what the handler
// did with it.
type scopeFakeEntity struct {
	entityDeploymentClient

	// byProject maps a project id to the deployment ids the caller can see in
	// it. A project absent from the map returns nothing, which is how
	// entity-service presents a project the caller has no access to.
	byProject map[string][]string
	// pageLimit, when non-zero, serves the ids in pages of this size so the
	// paging loop can be exercised.
	pageLimit int
	// forceHasMore reports HasMore on every page regardless of how many ids
	// are left, modelling an upstream whose paging metadata is wrong or
	// hostile. The helper must not take it at its word.
	forceHasMore bool
	searchErr    error

	searchCalls int
	updateCalls int
	gotUpdateID string
}

func (f *scopeFakeEntity) SearchDeployments(_ context.Context, req entity.SearchDeploymentsRequest) (entity.SearchDeploymentsResponse, error) {
	f.searchCalls++
	if f.searchErr != nil {
		return entity.SearchDeploymentsResponse{}, f.searchErr
	}

	var ids []string
	if len(req.ProjectIDs) == 1 {
		ids = f.byProject[req.ProjectIDs[0]]
	}

	limit := f.pageLimit
	if limit == 0 {
		limit = len(ids) + 1
	}
	start := min(req.Pagination.Offset, len(ids))
	end := min(start+limit, len(ids))

	views := make([]entity.DeploymentView, 0, end-start)
	for _, id := range ids[start:end] {
		views = append(views, entity.DeploymentView{ID: id})
	}
	return entity.SearchDeploymentsResponse{
		Deployments: views,
		Total:       len(ids),
		Limit:       limit,
		Offset:      req.Pagination.Offset,
		HasMore:     f.forceHasMore || end < len(ids),
	}, nil
}

func (f *scopeFakeEntity) UpdateDeployment(_ context.Context, id string, _ entity.UpdateDeploymentRequest) (entity.UpdateDeploymentResponse, error) {
	f.updateCalls++
	f.gotUpdateID = id
	return entity.UpdateDeploymentResponse{}, nil
}

func patchDeploymentMux(fake *scopeFakeEntity) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /projects/{projectId}/deployments/{id}", NewDeploymentHandler(fake).PatchDeployment)
	return mux
}

const validPatchBody = `{"name":"Renamed"}`

// TestPatchDeployment_UpdatesWhenDeploymentIsInProject is the happy path: the
// deployment is one of the project's own, so the update goes through.
func TestPatchDeployment_UpdatesWhenDeploymentIsInProject(t *testing.T) {
	fake := &scopeFakeEntity{byProject: map[string][]string{
		scopeProjectID: {"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", scopeDeploymentID},
	}}

	w := httptest.NewRecorder()
	patchDeploymentMux(fake).ServeHTTP(w, authedRequest(http.MethodPatch,
		"/projects/"+scopeProjectID+"/deployments/"+scopeDeploymentID, validPatchBody))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if fake.updateCalls != 1 {
		t.Fatalf("updateCalls = %d, want 1", fake.updateCalls)
	}
	if fake.gotUpdateID != scopeDeploymentID {
		t.Errorf("updated %q, want the path deployment %q", fake.gotUpdateID, scopeDeploymentID)
	}
}

// TestPatchDeployment_RejectsDeploymentFromAnotherProject is the IDOR this
// check exists for: a caller names a project they can reach but a deployment
// that lives somewhere else. The update must not happen.
func TestPatchDeployment_RejectsDeploymentFromAnotherProject(t *testing.T) {
	fake := &scopeFakeEntity{byProject: map[string][]string{
		scopeProjectID:      {"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
		scopeOtherProjectID: {scopeDeploymentID},
	}}

	w := httptest.NewRecorder()
	patchDeploymentMux(fake).ServeHTTP(w, authedRequest(http.MethodPatch,
		"/projects/"+scopeProjectID+"/deployments/"+scopeDeploymentID, validPatchBody))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if fake.updateCalls != 0 {
		t.Fatal("the deployment was updated despite belonging to another project")
	}
}

// TestPatchDeployment_RejectsProjectCallerCannotSee covers the other half: the
// pairing is real, but the caller has no access to the project, so the
// project-scoped search returns nothing.
func TestPatchDeployment_RejectsProjectCallerCannotSee(t *testing.T) {
	fake := &scopeFakeEntity{byProject: map[string][]string{}}

	w := httptest.NewRecorder()
	patchDeploymentMux(fake).ServeHTTP(w, authedRequest(http.MethodPatch,
		"/projects/"+scopeProjectID+"/deployments/"+scopeDeploymentID, validPatchBody))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if fake.updateCalls != 0 {
		t.Fatal("the deployment was updated for a project the caller cannot see")
	}
}

// TestPatchDeployment_RefusalsAreIndistinguishable guards the oracle: "not in
// this project" and "no access to this project" must produce byte-identical
// responses, or the endpoint can be used to enumerate other customers'
// deployment ids.
func TestPatchDeployment_RefusalsAreIndistinguishable(t *testing.T) {
	wrongProject := &scopeFakeEntity{byProject: map[string][]string{
		scopeProjectID:      {},
		scopeOtherProjectID: {scopeDeploymentID},
	}}
	noAccess := &scopeFakeEntity{byProject: map[string][]string{}}

	responses := make([]*httptest.ResponseRecorder, 0, 2)
	for _, fake := range []*scopeFakeEntity{wrongProject, noAccess} {
		w := httptest.NewRecorder()
		patchDeploymentMux(fake).ServeHTTP(w, authedRequest(http.MethodPatch,
			"/projects/"+scopeProjectID+"/deployments/"+scopeDeploymentID, validPatchBody))
		responses = append(responses, w)
	}

	if responses[0].Code != responses[1].Code {
		t.Fatalf("status codes differ: %d vs %d", responses[0].Code, responses[1].Code)
	}
	if responses[0].Body.String() != responses[1].Body.String() {
		t.Fatalf("bodies differ:\n  %s\n  %s", responses[0].Body.String(), responses[1].Body.String())
	}
}

// TestPatchDeployment_FindsDeploymentOnALaterPage checks the paging walk: a
// deployment beyond the first page must still be found, or a project with many
// deployments would start refusing legitimate updates.
func TestPatchDeployment_FindsDeploymentOnALaterPage(t *testing.T) {
	// Sized off the real constant (not a hardcoded page size) so this stays
	// correct if deploymentScopeCheckPageLimit ever changes again: two full
	// pages of filler ids, then the target as the sole entry on page three.
	filler := 2 * deploymentScopeCheckPageLimit
	ids := make([]string, 0, filler+1)
	for i := range filler {
		ids = append(ids, uuidForIndex(i))
	}
	ids = append(ids, scopeDeploymentID) // well past page one

	fake := &scopeFakeEntity{
		byProject: map[string][]string{scopeProjectID: ids},
		pageLimit: deploymentScopeCheckPageLimit,
	}

	w := httptest.NewRecorder()
	patchDeploymentMux(fake).ServeHTTP(w, authedRequest(http.MethodPatch,
		"/projects/"+scopeProjectID+"/deployments/"+scopeDeploymentID, validPatchBody))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if fake.searchCalls != 3 {
		t.Errorf("searchCalls = %d, want 3 pages of %d", fake.searchCalls, deploymentScopeCheckPageLimit)
	}
}

// TestPatchDeployment_StopsAtThePageCap guards the loop bound, with enough
// non-matching deployments that the walk would genuinely run past it.
//
// The fixture is deliberately one page larger than the cap allows: with the cap
// the walk makes exactly deploymentScopeCheckMaxPages calls, and without it the
// walk would make one more and still terminate — so removing the cap fails this
// test on a count rather than hanging it.
func TestPatchDeployment_StopsAtThePageCap(t *testing.T) {
	reachable := deploymentScopeCheckMaxPages * deploymentScopeCheckPageLimit
	ids := make([]string, 0, reachable+1)
	for i := range reachable + 1 {
		ids = append(ids, uuidForIndex(i))
	}

	fake := &scopeFakeEntity{
		byProject: map[string][]string{scopeProjectID: ids},
		pageLimit: deploymentScopeCheckPageLimit,
	}

	w := httptest.NewRecorder()
	patchDeploymentMux(fake).ServeHTTP(w, authedRequest(http.MethodPatch,
		"/projects/"+scopeProjectID+"/deployments/"+scopeDeploymentID, validPatchBody))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if fake.searchCalls != deploymentScopeCheckMaxPages {
		t.Fatalf("searchCalls = %d, want exactly the page cap %d", fake.searchCalls, deploymentScopeCheckMaxPages)
	}
}

// TestPatchDeployment_StopsOnEmptyPageDespiteHasMore covers the loop's other
// stop condition. An upstream that reports HasMore on a page it returned
// nothing for would otherwise spin the walk all the way to the page cap on
// every single update.
func TestPatchDeployment_StopsOnEmptyPageDespiteHasMore(t *testing.T) {
	fake := &scopeFakeEntity{
		byProject:    map[string][]string{scopeProjectID: {}},
		pageLimit:    deploymentScopeCheckPageLimit,
		forceHasMore: true,
	}

	w := httptest.NewRecorder()
	patchDeploymentMux(fake).ServeHTTP(w, authedRequest(http.MethodPatch,
		"/projects/"+scopeProjectID+"/deployments/"+scopeDeploymentID, validPatchBody))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if fake.searchCalls != 1 {
		t.Fatalf("searchCalls = %d, want 1 — an empty page must end the walk", fake.searchCalls)
	}
}

// TestPatchDeployment_RejectsNonUUIDProject guards the new path parameter. The
// project segment was previously ignored entirely.
func TestPatchDeployment_RejectsNonUUIDProject(t *testing.T) {
	fake := &scopeFakeEntity{byProject: map[string][]string{}}

	w := httptest.NewRecorder()
	patchDeploymentMux(fake).ServeHTTP(w, authedRequest(http.MethodPatch,
		"/projects/not-a-uuid/deployments/"+scopeDeploymentID, validPatchBody))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if fake.searchCalls != 0 {
		t.Error("upstream was called despite an invalid projectId")
	}
}

// TestPatchDeployment_ScopeCheckRunsBeforeTheBodyIsRead ensures an unauthorized
// caller cannot use body validation messages to learn anything — the refusal
// must not depend on the body being well-formed.
func TestPatchDeployment_ScopeCheckRunsBeforeTheBodyIsRead(t *testing.T) {
	fake := &scopeFakeEntity{byProject: map[string][]string{}}

	w := httptest.NewRecorder()
	patchDeploymentMux(fake).ServeHTTP(w, authedRequest(http.MethodPatch,
		"/projects/"+scopeProjectID+"/deployments/"+scopeDeploymentID, `{"both":"invalid"`))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 regardless of the malformed body", w.Code)
	}
	if fake.updateCalls != 0 {
		t.Fatal("upstream update was called")
	}
}

// TestPatchDeployment_UpstreamSearchFailureIsNotA404 — a broken upstream must
// not be reported as "not found", which would look like a permission decision.
func TestPatchDeployment_UpstreamSearchFailureIsNotA404(t *testing.T) {
	fake := &scopeFakeEntity{searchErr: errors.New("upstream exploded")}

	w := httptest.NewRecorder()
	patchDeploymentMux(fake).ServeHTTP(w, authedRequest(http.MethodPatch,
		"/projects/"+scopeProjectID+"/deployments/"+scopeDeploymentID, validPatchBody))

	if w.Code == http.StatusNotFound {
		t.Fatal("an upstream failure was reported as 404")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if fake.updateCalls != 0 {
		t.Fatal("the update ran despite the scope check failing")
	}
}

// uuidForIndex builds a distinct, valid-looking deployment id for bulk
// fixtures. The index occupies the whole final group, so ids stay unique well
// past the page cap — an earlier version wrapped after 256 and would have
// silently repeated across a fixture this size.
func uuidForIndex(i int) string {
	return fmt.Sprintf("bbbbbbbb-bbbb-bbbb-bbbb-%012x", i)
}
