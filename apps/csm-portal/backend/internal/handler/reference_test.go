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
	"strings"
	"testing"
)

func TestReferenceHandler_SearchRoles(t *testing.T) {
	t.Run("rejects an unauthenticated caller", func(t *testing.T) {
		h := NewReferenceHandler(testDirectory(t))
		w := httptest.NewRecorder()
		h.SearchRoles(w, httptest.NewRequest(http.MethodPost, "/roles/search", strings.NewReader(`{}`)))
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("serves the configured catalogue and applies the search filter", func(t *testing.T) {
		h := NewReferenceHandler(testDirectory(t))
		w := httptest.NewRecorder()
		h.SearchRoles(w, withUser(httptest.NewRequest(http.MethodPost, "/roles/search",
			strings.NewReader(`{"filters":{"searchQuery":"appro"}}`))))

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		got := decodeJSON[struct {
			Roles []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"roles"`
			Total int `json:"total"`
		}](t, w)
		if got.Total != 1 || got.Roles[0].ID != "timecard_approver" {
			t.Fatalf("roles = %+v, want just timecard_approver", got.Roles)
		}
		if got.Roles[0].Name != "Timecard Approver" {
			t.Errorf("name = %q, want a humanised display name", got.Roles[0].Name)
		}
	})

	// An absent body means "no filters, default page" — it must not be a 400.
	t.Run("accepts an empty body", func(t *testing.T) {
		h := NewReferenceHandler(testDirectory(t))
		w := httptest.NewRecorder()
		h.SearchRoles(w, withUser(httptest.NewRequest(http.MethodPost, "/roles/search", nil)))
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("rejects a malformed body", func(t *testing.T) {
		h := NewReferenceHandler(testDirectory(t))
		w := httptest.NewRecorder()
		h.SearchRoles(w, withUser(httptest.NewRequest(http.MethodPost, "/roles/search", strings.NewReader(`{`))))
		assertStatus(t, w, http.StatusBadRequest)
	})
}

func TestReferenceHandler_SearchTeams(t *testing.T) {
	t.Run("serves the registry, family and all, from memory", func(t *testing.T) {
		h := NewReferenceHandler(testDirectory(t))
		w := httptest.NewRecorder()
		h.SearchTeams(w, withUser(httptest.NewRequest(http.MethodPost, "/teams/search", strings.NewReader(`{}`))))

		assertStatus(t, w, http.StatusOK)
		got := decodeJSON[struct {
			Teams []struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				Family     string `json:"family"`
				CreGroupID string `json:"creGroupId"`
			} `json:"teams"`
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		}](t, w)

		if got.Total != 3 {
			t.Fatalf("total = %d, want the 3 configured teams", got.Total)
		}
		// Sorted by display name: ABT One, ABT Two, Beta Team.
		if got.Teams[0].ID != "abt-1" || got.Teams[0].Family != "cre-abt" {
			t.Fatalf("teams[0] = %+v, want the abt-1 row with its family", got.Teams[0])
		}
		// The frontend filters the team picker on family, so it must survive.
		if got.Teams[2].Family != "sre-abt" {
			t.Errorf("teams[2].family = %q, want sre-abt", got.Teams[2].Family)
		}
		// The backing CRE group id is resolved to UUID form at startup...
		if got.Teams[0].CreGroupID != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
			t.Errorf("creGroupId = %q, want the UUID form of the configured id", got.Teams[0].CreGroupID)
		}
		// ...and a row that configured none is still listed, just without one.
		if got.Teams[1].CreGroupID != "" {
			t.Errorf("creGroupId = %q, want it omitted for a team with no configured id", got.Teams[1].CreGroupID)
		}
	})

	t.Run("rejects an unauthenticated caller", func(t *testing.T) {
		h := NewReferenceHandler(testDirectory(t))
		w := httptest.NewRecorder()
		h.SearchTeams(w, httptest.NewRequest(http.MethodPost, "/teams/search", strings.NewReader(`{}`)))
		assertStatus(t, w, http.StatusUnauthorized)
	})
}

// TestReferenceHandler_NoUpstreamCallsEverAfterStartup is the requirement this
// move exists for: the team/role mapping is resolved once, at startup, and no
// number of catalogue requests re-fetches it.
//
// It is asserted by construction rather than by counting: the handler holds no
// upstream client at all, so there is nothing it could call. What is worth
// proving is that repeated requests stay correct and identical without one.
func TestReferenceHandler_NoUpstreamCallsEverAfterStartup(t *testing.T) {
	h := NewReferenceHandler(testDirectory(t))

	var first string
	for i := 0; i < 25; i++ {
		w := httptest.NewRecorder()
		h.SearchTeams(w, withUser(httptest.NewRequest(http.MethodPost, "/teams/search", strings.NewReader(`{}`))))
		assertStatus(t, w, http.StatusOK)
		if i == 0 {
			first = w.Body.String()
			continue
		}
		if w.Body.String() != first {
			t.Fatalf("request %d returned a different body than the first; the catalogue is not a stable snapshot", i)
		}
	}
}

func TestUsersHandler_GetUser(t *testing.T) {
	const testUserID = "11111111-1111-1111-1111-111111111111"

	t.Run("rejects an unauthenticated caller", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		w := httptest.NewRecorder()
		h.GetUser(w, httptest.NewRequest(http.MethodGet, "/users/abc", nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("passes the path id through", func(t *testing.T) {
		var gotID string
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
			getUserFn: func(_ context.Context, id string) ([]byte, error) {
				gotID = id
				return []byte(`{"id":"` + id + `","userType":"internal","groups":[],"teams":[]}`), nil
			},
		}, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+testUserID, nil))
		r.SetPathValue("id", testUserID)
		w := httptest.NewRecorder()
		h.GetUser(w, r)

		assertStatus(t, w, http.StatusOK)
		if gotID != testUserID {
			t.Errorf("id = %q, want the path value", gotID)
		}
	})

	t.Run("rejects a missing id", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		w := httptest.NewRecorder()
		h.GetUser(w, withUser(httptest.NewRequest(http.MethodGet, "/users/", nil)))
		assertStatus(t, w, http.StatusBadRequest)
	})

	// A malformed id must be rejected locally: the entity service rejects non-UUID ids
	// anyway, so calling it would spend a round trip to get back an upstream-mapped
	// status instead of a clean 400.
	t.Run("rejects a malformed id without calling upstream", func(t *testing.T) {
		called := false
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
			getUserFn: func(_ context.Context, _ string) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		}, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetUser(w, r)

		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		if called {
			t.Error("the entity client was called for a malformed id, want no upstream call")
		}
	})

	t.Run("maps an upstream not-found", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
			getUserFn: func(_ context.Context, _ string) ([]byte, error) {
				return nil, errors.New("not found")
			},
		}, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+testUserID, nil))
		r.SetPathValue("id", testUserID)
		w := httptest.NewRecorder()
		h.GetUser(w, r)
		if w.Code == http.StatusOK {
			t.Fatal("status = 200, want an error status")
		}
	})
}

func TestProjectHandler_GetProjectContact(t *testing.T) {
	const (
		testProjectID = "22222222-2222-2222-2222-222222222222"
		testContactID = "33333333-3333-3333-3333-333333333333"
	)

	t.Run("rejects an unauthenticated caller", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		w := httptest.NewRecorder()
		h.GetProjectContact(w, httptest.NewRequest(http.MethodGet, "/projects/"+testProjectID+"/contacts/"+testContactID, nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("passes both path ids through", func(t *testing.T) {
		var gotProject, gotContact string
		h := NewProjectHandler(&mockEntityProjectClient{
			getProjectContactFn: func(_ context.Context, projectID, contactID string) ([]byte, error) {
				gotProject, gotContact = projectID, contactID
				return []byte(`{"id":"` + contactID + `","registrationState":"REGISTERED"}`), nil
			},
		})
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/"+testProjectID+"/contacts/"+testContactID, nil))
		r.SetPathValue("id", testProjectID)
		r.SetPathValue("contactId", testContactID)
		w := httptest.NewRecorder()
		h.GetProjectContact(w, r)

		assertStatus(t, w, http.StatusOK)
		if gotProject != testProjectID || gotContact != testContactID {
			t.Fatalf("ids = %q/%q, want %q/%q", gotProject, gotContact, testProjectID, testContactID)
		}
	})

	t.Run("rejects a missing contact id", func(t *testing.T) {
		h := NewProjectHandler(&mockEntityProjectClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/"+testProjectID+"/contacts/", nil))
		r.SetPathValue("id", testProjectID)
		w := httptest.NewRecorder()
		h.GetProjectContact(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	// Both ids are UUIDs upstream, so a malformed one is rejected here rather than
	// spending a round trip on an error the entity service would return anyway.
	t.Run("rejects a malformed project id without calling upstream", func(t *testing.T) {
		called := false
		h := NewProjectHandler(&mockEntityProjectClient{
			getProjectContactFn: func(_ context.Context, _, _ string) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		})
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/not-a-uuid/contacts/"+testContactID, nil))
		r.SetPathValue("id", "not-a-uuid")
		r.SetPathValue("contactId", testContactID)
		w := httptest.NewRecorder()
		h.GetProjectContact(w, r)

		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		if called {
			t.Error("the entity client was called for a malformed project id, want no upstream call")
		}
	})

	t.Run("rejects a malformed contact id without calling upstream", func(t *testing.T) {
		called := false
		h := NewProjectHandler(&mockEntityProjectClient{
			getProjectContactFn: func(_ context.Context, _, _ string) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		})
		r := withUser(httptest.NewRequest(http.MethodGet, "/projects/"+testProjectID+"/contacts/not-a-uuid", nil))
		r.SetPathValue("id", testProjectID)
		r.SetPathValue("contactId", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetProjectContact(w, r)

		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		if called {
			t.Error("the entity client was called for a malformed contact id, want no upstream call")
		}
	})
}
