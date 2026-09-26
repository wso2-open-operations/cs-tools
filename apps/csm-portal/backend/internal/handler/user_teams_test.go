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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// capturedSearch runs POST /users/search through the handler and returns the
// body the entity service actually received.
func capturedSearch(t *testing.T, body string) (string, *httptest.ResponseRecorder) {
	t.Helper()
	var captured string
	h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
		searchUsersFn: func(_ context.Context, b []byte) ([]byte, error) {
			captured = string(b)
			return []byte(`{"users":[],"total":0,"limit":10,"offset":0}`), nil
		},
	}, testDirectory(t), false)
	w := httptest.NewRecorder()
	h.SearchUsers(w, withUser(httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(body))))
	return captured, w
}

// TestSearchUsers_TeamFilterResolvesToGroupNames: the entity service has no
// registry any more, so it cannot be sent a team key. This layer owns the
// mapping and must translate before forwarding.
func TestSearchUsers_TeamFilterResolvesToGroupNames(t *testing.T) {
	captured, w := capturedSearch(t, `{"filters":{"teamIds":["abt-1","beta"]},"pagination":{"limit":5}}`)
	assertStatus(t, w, http.StatusOK)

	var got struct {
		Filters struct {
			TeamIDs    []string `json:"teamIds"`
			GroupNames []string `json:"groupNames"`
		} `json:"filters"`
		Pagination struct {
			Limit int `json:"limit"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal([]byte(captured), &got); err != nil {
		t.Fatalf("forwarded body is not JSON: %v (%s)", err, captured)
	}
	if got.Filters.TeamIDs != nil {
		t.Errorf("teamIds was forwarded upstream (%v); the entity service cannot resolve it", got.Filters.TeamIDs)
	}
	if len(got.Filters.GroupNames) != 2 ||
		got.Filters.GroupNames[0] != "ABT One" || got.Filters.GroupNames[1] != "Beta Team" {
		t.Errorf("groupNames = %v, want the two teams' display names", got.Filters.GroupNames)
	}
	// Everything else survives the rewrite.
	if got.Pagination.Limit != 5 {
		t.Errorf("pagination.limit = %d, want the caller's 5", got.Pagination.Limit)
	}
}

// An empty teamIds array is not a filter; it must not become an empty
// groupNames list, which would mean something different upstream.
func TestSearchUsers_EmptyTeamFilterIsDropped(t *testing.T) {
	captured, w := capturedSearch(t, `{"filters":{"teamIds":[]}}`)
	assertStatus(t, w, http.StatusOK)
	if strings.Contains(captured, "groupNames") || strings.Contains(captured, "teamIds") {
		t.Errorf("forwarded body = %s, want neither filter", captured)
	}
}

// A body with no filters must go upstream byte-for-byte, so this rewrite can
// never be the cause of an unrelated regression.
func TestSearchUsers_UnfilteredBodyIsForwardedVerbatim(t *testing.T) {
	body := `{"pagination":{"limit":10,"offset":20},"sortBy":{"field":"name"}}`
	captured, w := capturedSearch(t, body)
	assertStatus(t, w, http.StatusOK)
	if captured != body {
		t.Errorf("forwarded body = %s, want it unchanged (%s)", captured, body)
	}
}

// TestSearchUsers_TeamFilterAcceptsUUIDForm: a teamIds entry may also be the
// platform UUID form of a team's backing group id -- the same shape
// accounts.creTeam.id/sreTeam.id already expose -- and must resolve to the
// same group name a teamKey would.
func TestSearchUsers_TeamFilterAcceptsUUIDForm(t *testing.T) {
	// abt-1's fixture CreGroupID "aaaa...aaaa" (32 hex chars, see
	// testTeamRegistry in helpers_test.go) resolves to this UUID.
	captured, w := capturedSearch(t, `{"filters":{"teamIds":["aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"]}}`)
	assertStatus(t, w, http.StatusOK)

	var got struct {
		Filters struct {
			GroupNames []string `json:"groupNames"`
		} `json:"filters"`
	}
	if err := json.Unmarshal([]byte(captured), &got); err != nil {
		t.Fatalf("forwarded body is not JSON: %v (%s)", err, captured)
	}
	if len(got.Filters.GroupNames) != 1 || got.Filters.GroupNames[0] != "ABT One" {
		t.Errorf("groupNames = %v, want [\"ABT One\"] resolved from the UUID form", got.Filters.GroupNames)
	}
}

// A UUID-shaped teamIds entry that matches no configured team's group id is
// still the same caller-facing "unknown team" error as an unknown teamKey.
func TestSearchUsers_UnknownTeamUUIDIsRejectedWithoutCallingUpstream(t *testing.T) {
	called := false
	h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
		searchUsersFn: func(_ context.Context, _ []byte) ([]byte, error) {
			called = true
			return []byte(`{}`), nil
		},
	}, testDirectory(t), false)
	w := httptest.NewRecorder()
	h.SearchUsers(w, withUser(httptest.NewRequest(http.MethodPost, "/users/search",
		strings.NewReader(`{"filters":{"teamIds":["ffffffff-ffff-ffff-ffff-ffffffffffff"]}}`))))

	assertStatus(t, w, http.StatusBadRequest)
	assertErrorMessage(t, w, "teamIds contains unknown team: ffffffff-ffff-ffff-ffff-ffffffffffff")
	if called {
		t.Error("the entity service was called with an unresolvable team UUID")
	}
}

// An unknown team key is a client error, not a silent empty page. It has to be
// caught here now: the entity service has nothing to check it against.
func TestSearchUsers_UnknownTeamKeyIsRejectedWithoutCallingUpstream(t *testing.T) {
	called := false
	h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
		searchUsersFn: func(_ context.Context, _ []byte) ([]byte, error) {
			called = true
			return []byte(`{}`), nil
		},
	}, testDirectory(t), false)
	w := httptest.NewRecorder()
	h.SearchUsers(w, withUser(httptest.NewRequest(http.MethodPost, "/users/search",
		strings.NewReader(`{"filters":{"teamIds":["no-such-team"]}}`))))

	assertStatus(t, w, http.StatusBadRequest)
	assertErrorMessage(t, w, "teamIds contains unknown team: no-such-team")
	if called {
		t.Error("the entity service was called with an unresolvable team key")
	}
}

// isValidUserRole moved here with the allow-list; it must still reject.
func TestSearchUsers_RejectsARoleOutsideTheAllowList(t *testing.T) {
	called := false
	h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
		searchUsersFn: func(_ context.Context, _ []byte) ([]byte, error) {
			called = true
			return []byte(`{"users":[],"total":0}`), nil
		},
	}, testDirectory(t), false)

	w := httptest.NewRecorder()
	h.SearchUsers(w, withUser(httptest.NewRequest(http.MethodPost, "/users/search",
		strings.NewReader(`{"filters":{"roleIds":["admin"]}}`))))
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorMessage(t, w, "roleIds contains invalid value: admin")
	if called {
		t.Fatal("the entity service was called with a role outside the allow-list")
	}

	// A configured role still gets through.
	w = httptest.NewRecorder()
	h.SearchUsers(w, withUser(httptest.NewRequest(http.MethodPost, "/users/search",
		strings.NewReader(`{"filters":{"roleIds":["agent"]}}`))))
	assertStatus(t, w, http.StatusOK)
	if !called {
		t.Fatal("a configured role did not reach the entity service")
	}
}

func TestSearchUsers_EnforcesFilterCaps(t *testing.T) {
	tooManyTeams := make([]string, teamIDFilterLimit+1)
	for i := range tooManyTeams {
		tooManyTeams[i] = fmt.Sprintf("team-%d", i)
	}
	encoded, _ := json.Marshal(tooManyTeams)
	_, w := capturedSearch(t, `{"filters":{"teamIds":`+string(encoded)+`}}`)
	assertStatus(t, w, http.StatusBadRequest)
}

// TestSearchUsers_TeamResolutionMakesNoEntityCalls: repeated team-filtered
// searches must issue exactly one entity call each -- the one that does the
// membership query. The key-to-name mapping never adds a second.
func TestSearchUsers_TeamResolutionMakesNoEntityCalls(t *testing.T) {
	calls := 0
	h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
		searchUsersFn: func(_ context.Context, _ []byte) ([]byte, error) {
			calls++
			return []byte(`{"users":[],"total":0}`), nil
		},
	}, testDirectory(t), false)

	const iterations = 20
	for i := 0; i < iterations; i++ {
		w := httptest.NewRecorder()
		h.SearchUsers(w, withUser(httptest.NewRequest(http.MethodPost, "/users/search",
			strings.NewReader(`{"filters":{"teamIds":["abt-1"]}}`))))
		assertStatus(t, w, http.StatusOK)
	}
	if calls != iterations {
		t.Fatalf("entity was called %d times for %d searches; team resolution is adding calls", calls, iterations)
	}
}

// TestGetUser_DerivesTeamsFromGroups: the entity service reports groups; which
// of them are teams is this layer's knowledge, so it adds the block back.
func TestGetUser_DerivesTeamsFromGroups(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"staff@example.com","lockedOut":true,"groups":[` +
				`{"id":"g-1","name":"ABT One"},{"id":"g-2","name":"Some Other Group"}]}`), nil
		},
	}, testDirectory(t), false)

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	got := decodeJSON[struct {
		Email     string `json:"email"`
		LockedOut bool   `json:"lockedOut"`
		Groups    []struct {
			Name string `json:"name"`
		} `json:"groups"`
		Teams []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Family string `json:"family"`
		} `json:"teams"`
	}](t, w)

	if len(got.Groups) != 2 {
		t.Errorf("groups = %+v, want both passed through untouched", got.Groups)
	}
	if len(got.Teams) != 1 {
		t.Fatalf("teams = %+v, want just the one registry team", got.Teams)
	}
	if got.Teams[0].ID != "abt-1" || got.Teams[0].Name != "ABT One" || got.Teams[0].Family != "cre-abt" {
		t.Errorf("teams[0] = %+v, want the abt-1 row", got.Teams[0])
	}
	if got.Email != "staff@example.com" {
		t.Errorf("email = %q, want the rest of the profile preserved", got.Email)
	}
	// The entity service's lockedOut field is not something this layer knows
	// about; the envelope-based re-encoding in withUserTeams must still carry
	// it through untouched.
	if !got.LockedOut {
		t.Errorf("lockedOut = %v, want true (passed through from the entity service)", got.LockedOut)
	}
}

// A user in no registry team gets an empty teams list, never a missing key.
func TestGetUser_EmptyTeamsWhenNoneMatch(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","groups":[{"id":"g-2","name":"Some Other Group"}]}`), nil
		},
	}, testDirectory(t), false)

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	got := decodeJSON[map[string]any](t, w)
	teams, ok := got["teams"].([]any)
	if !ok {
		t.Fatalf("teams = %v, want an array", got["teams"])
	}
	if len(teams) != 0 {
		t.Errorf("teams = %v, want empty", teams)
	}
}
