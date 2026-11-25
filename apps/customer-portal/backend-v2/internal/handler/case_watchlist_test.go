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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// fakeWatchListEntityClient records what CreateCase/UpdateCase were actually
// called with, and answers SearchUsers from a fixed email->id fixture -- the
// same shape every other fake in this package uses (embed the interface,
// override only what the test needs).
type fakeWatchListEntityClient struct {
	entityCaseClient
	usersByEmail   map[string]string
	searchUsersErr error
	searchCalls    int

	gotCreateCaseReq entity.CreateCaseRequest
	gotUpdateCaseReq entity.UpdateCaseRequest
}

func (f *fakeWatchListEntityClient) SearchUsers(ctx context.Context, req entity.SearchUsersRequest) (entity.SearchUsersResponse, error) {
	f.searchCalls++
	if f.searchUsersErr != nil {
		return entity.SearchUsersResponse{}, f.searchUsersErr
	}
	var users []entity.UserSummary
	for _, email := range req.Filters.Emails {
		if id, ok := f.usersByEmail[email]; ok {
			users = append(users, entity.UserSummary{ID: id, Email: email})
		}
	}
	return entity.SearchUsersResponse{Users: users}, nil
}

func (f *fakeWatchListEntityClient) GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error) {
	return entity.ProjectDetailsView{ID: id}, nil
}

func (f *fakeWatchListEntityClient) CreateCase(ctx context.Context, req entity.CreateCaseRequest) (entity.CreateCaseResponse, error) {
	f.gotCreateCaseReq = req
	return entity.CreateCaseResponse{}, nil
}

func (f *fakeWatchListEntityClient) UpdateCase(ctx context.Context, id string, req entity.UpdateCaseRequest) (entity.UpdateCaseResponse, error) {
	f.gotUpdateCaseReq = req
	return entity.UpdateCaseResponse{}, nil
}

func TestResolveWatchListUserIDs_ResolvesKnownEmailsAndDropsUnknown(t *testing.T) {
	fake := &fakeWatchListEntityClient{
		usersByEmail: map[string]string{
			"alice@example.com": "11111111-1111-1111-1111-111111111111",
			"bob@example.com":   "22222222-2222-2222-2222-222222222222",
		},
	}
	h := NewCaseHandler(fake)

	got, err := h.resolveWatchListUserIDs(context.Background(), []string{
		"alice@example.com", "unknown@example.com", "bob@example.com",
	})
	if err != nil {
		t.Fatalf("resolveWatchListUserIDs() error = %v, want nil", err)
	}

	want := []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"}
	if len(got) != len(want) {
		t.Fatalf("resolveWatchListUserIDs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("resolveWatchListUserIDs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveWatchListUserIDs_EmptyInputSkipsUpstreamCall(t *testing.T) {
	fake := &fakeWatchListEntityClient{
		searchUsersErr: errors.New("SearchUsers must not be called for an empty watch list"),
	}
	h := NewCaseHandler(fake)

	got, err := h.resolveWatchListUserIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("resolveWatchListUserIDs(nil) error = %v, want nil", err)
	}
	if got != nil {
		t.Fatalf("resolveWatchListUserIDs(nil) = %v, want nil", got)
	}
}

// TestResolveWatchListUserIDs_UpstreamErrorIsPropagated is the regression
// test for a CodeRabbit finding on this fix's own PR: an upstream SearchUsers
// failure used to be swallowed and returned as an empty (but non-error)
// result, indistinguishable from "nobody matched" -- which let CreateCase
// silently create a case with no requested watchers, and let PatchCase
// silently forward a no-op update, both while still reporting success. The
// error must now come back to the caller instead.
func TestResolveWatchListUserIDs_UpstreamErrorIsPropagated(t *testing.T) {
	fake := &fakeWatchListEntityClient{searchUsersErr: errors.New("boom")}
	h := NewCaseHandler(fake)

	got, err := h.resolveWatchListUserIDs(context.Background(), []string{"alice@example.com"})
	if err == nil {
		t.Fatal("resolveWatchListUserIDs() error = nil, want a propagated error")
	}
	if got != nil {
		t.Fatalf("resolveWatchListUserIDs() = %v, want nil alongside the error", got)
	}
}

// TestCreateCase_WatchList_ForwardsEmailsAsIs: create sends the caller's
// watch-list emails to entity-service untouched and makes no user lookup.
// entity-service resolves emails itself on create; a lookup here only turned
// them into ids that entity-service had to turn back into emails (and the
// user search it used for that is gated to internal roles).
func TestCreateCase_WatchList_ForwardsEmailsAsIs(t *testing.T) {
	fake := &fakeWatchListEntityClient{
		usersByEmail: map[string]string{
			"alice@example.com": "11111111-1111-1111-1111-111111111111",
		},
	}
	h := NewCaseHandler(fake)

	reqBody := `{"projectId":"33333333-3333-3333-3333-333333333333","title":"Test Case","description":"Details","watchList":["alice@example.com","bob@example.com"]}`
	req := authedRequest(http.MethodPost, "/cases", reqBody)
	rec := httptest.NewRecorder()

	h.CreateCase(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
	want := []string{"alice@example.com", "bob@example.com"}
	got := fake.gotCreateCaseReq.WatchList
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("entity CreateCaseRequest.WatchList = %v, want %v (raw emails)", got, want)
	}
	if fake.searchCalls != 0 {
		t.Fatalf("SearchUsers called %d times on create, want 0", fake.searchCalls)
	}
}

// TestPatchCase_WatchList_ResolvesEmailsToUserIDs is the same regression,
// for the PATCH /cases/{id} path — this is the one a real user hit live
// ("watchList contains invalid UUID") when editing an existing case's watch
// list from the case details page.
func TestPatchCase_WatchList_ResolvesEmailsToUserIDs(t *testing.T) {
	fake := &fakeWatchListEntityClient{
		usersByEmail: map[string]string{
			"alice@example.com": "11111111-1111-1111-1111-111111111111",
			"bob@example.com":   "22222222-2222-2222-2222-222222222222",
		},
	}
	h := NewCaseHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /cases/{id}", h.PatchCase)

	reqBody := `{"watchList":["alice@example.com","bob@example.com"]}`
	req := authedRequest(http.MethodPatch, "/cases/33333333-3333-3333-3333-333333333333", reqBody)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	want := []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"}
	got := fake.gotUpdateCaseReq.WatchList
	if len(got) != len(want) {
		t.Fatalf("entity UpdateCaseRequest.WatchList = %v, want %v (resolved UUIDs, not raw emails)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entity UpdateCaseRequest.WatchList[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestPatchCase_WatchList_AllUnresolved_Rejects400 is the regression test for
// the CodeRabbit finding that a patch whose every submitted email fails to
// resolve must be rejected explicitly, rather than silently forwarding an
// empty watchList that satisfies neither this endpoint's nor entity-service's
// "exactly one of stateKey/watchList" requirement.
func TestPatchCase_WatchList_AllUnresolved_Rejects400(t *testing.T) {
	fake := &fakeWatchListEntityClient{usersByEmail: map[string]string{}}
	h := NewCaseHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /cases/{id}", h.PatchCase)

	reqBody := `{"watchList":["unknown1@example.com","unknown2@example.com"]}`
	req := authedRequest(http.MethodPatch, "/cases/33333333-3333-3333-3333-333333333333", reqBody)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
	}
	if fake.gotUpdateCaseReq.ID != "" {
		t.Fatalf("entity UpdateCase must not be called when every watch-list email is unresolved, got %+v", fake.gotUpdateCaseReq)
	}
}

// TestPatchCase_WatchDelta_ResolvesBothListsAndNeverSendsAWholeList: the delta
// fields carry emails like watchList does, so each is resolved to ids, and the
// entity request holds only the delta -- no WatchList that would replace (and
// drop a concurrently added watcher from) the stored list.
func TestPatchCase_WatchDelta_ResolvesBothListsAndNeverSendsAWholeList(t *testing.T) {
	fake := &fakeWatchListEntityClient{
		usersByEmail: map[string]string{
			"alice@example.com": "11111111-1111-1111-1111-111111111111",
			"bob@example.com":   "22222222-2222-2222-2222-222222222222",
		},
	}
	h := NewCaseHandler(fake)
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /cases/{id}", h.PatchCase)

	req := authedRequest(http.MethodPatch, "/cases/33333333-3333-3333-3333-333333333333",
		`{"addWatchers":["alice@example.com"],"removeWatchers":["bob@example.com"]}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	got := fake.gotUpdateCaseReq
	if len(got.AddWatchers) != 1 || got.AddWatchers[0] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("AddWatchers = %v, want alice's id", got.AddWatchers)
	}
	if len(got.RemoveWatchers) != 1 || got.RemoveWatchers[0] != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("RemoveWatchers = %v, want bob's id", got.RemoveWatchers)
	}
	if len(got.WatchList) != 0 {
		t.Fatalf("WatchList = %v, want none: a whole list would overwrite concurrent changes", got.WatchList)
	}
}

// A remove-only request is valid, and it is how the last watcher goes (a
// watchList replace cannot be empty).
func TestPatchCase_WatchDelta_RemoveOnly(t *testing.T) {
	fake := &fakeWatchListEntityClient{
		usersByEmail: map[string]string{"bob@example.com": "22222222-2222-2222-2222-222222222222"},
	}
	h := NewCaseHandler(fake)
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /cases/{id}", h.PatchCase)

	req := authedRequest(http.MethodPatch, "/cases/33333333-3333-3333-3333-333333333333", `{"removeWatchers":["bob@example.com"]}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(fake.gotUpdateCaseReq.RemoveWatchers) != 1 || len(fake.gotUpdateCaseReq.AddWatchers) != 0 {
		t.Fatalf("got %+v", fake.gotUpdateCaseReq)
	}
}

func TestPatchCase_WatchDelta_Refusals(t *testing.T) {
	cases := map[string]struct {
		body  string
		users map[string]string
	}{
		"with a state change":    {`{"stateKey":1,"addWatchers":["a@example.com"]}`, map[string]string{"a@example.com": "11111111-1111-1111-1111-111111111111"}},
		"with watchList":         {`{"watchList":["a@example.com"],"addWatchers":["a@example.com"]}`, map[string]string{"a@example.com": "11111111-1111-1111-1111-111111111111"}},
		"every email is unknown": {`{"addWatchers":["x@example.com"],"removeWatchers":["y@example.com"]}`, map[string]string{}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &fakeWatchListEntityClient{usersByEmail: tc.users}
			h := NewCaseHandler(fake)
			mux := http.NewServeMux()
			mux.HandleFunc("PATCH /cases/{id}", h.PatchCase)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, authedRequest(http.MethodPatch, "/cases/33333333-3333-3333-3333-333333333333", tc.body))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			if fake.gotUpdateCaseReq.ID != "" {
				t.Fatalf("entity UpdateCase must not be called, got %+v", fake.gotUpdateCaseReq)
			}
		})
	}
}
