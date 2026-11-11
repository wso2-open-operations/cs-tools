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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// searchVia runs one of the two user search handlers and returns the body the
// entity service received.
func searchVia(t *testing.T, admin bool, body string) (map[string]json.RawMessage, *httptest.ResponseRecorder) {
	t.Helper()
	var captured []byte
	h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
		searchUsersFn: func(_ context.Context, b []byte) ([]byte, error) {
			captured = b
			return []byte(`{"users":[],"total":0}`), nil
		},
	}, testDirectory(t), nil)
	w := httptest.NewRecorder()
	r := withUser(httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(body)))
	if admin {
		h.SearchUsersAdmin(w, r)
	} else {
		h.SearchUsers(w, r)
	}
	var env map[string]json.RawMessage
	if captured != nil {
		if err := json.Unmarshal(captured, &env); err != nil {
			t.Fatalf("forwarded body is not JSON: %v (%s)", err, captured)
		}
	}
	return env, w
}

func filtersOf(t *testing.T, env map[string]json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	var f map[string]json.RawMessage
	if err := json.Unmarshal(env["filters"], &f); err != nil {
		t.Fatalf("filters is not an object: %v (%s)", err, env["filters"])
	}
	return f
}

// The general search forces internal whether the caller omits userType, sets
// the external value, sets an unknown value, or sends null.
func TestSearchUsers_ForcesInternalUserType(t *testing.T) {
	for name, body := range map[string]string{
		"omitted":      `{"filters":{"searchQuery":"jane"}}`,
		"no filters":   `{}`,
		"null filters": `{"filters":null}`,
		"external":     `{"filters":{"searchQuery":"jane","userType":"external"}}`,
		"customer":     `{"filters":{"searchQuery":"jane","userType":"customer"}}`,
		"null":         `{"filters":{"searchQuery":"jane","userType":null}}`,
		"array":        `{"filters":{"searchQuery":"jane","userType":["external","internal"]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			env, w := searchVia(t, false, body)
			assertStatus(t, w, http.StatusOK)
			if got := string(filtersOf(t, env)["userType"]); got != `"internal"` {
				t.Errorf("forwarded userType = %s, want \"internal\"", got)
			}
		})
	}
}

// Overriding userType must not drop or alter any other filter or field.
func TestSearchUsers_ForcedUserTypePreservesOtherFilters(t *testing.T) {
	env, w := searchVia(t, false,
		`{"pagination":{"limit":5,"offset":10},"sortBy":{"field":"name","order":"asc"},`+
			`"filters":{"searchQuery":"jane","emails":["jane.doe@example.com"],"active":true,"userType":"external"}}`)
	assertStatus(t, w, http.StatusOK)
	f := filtersOf(t, env)
	if string(f["searchQuery"]) != `"jane"` || string(f["emails"]) != `["jane.doe@example.com"]` || string(f["active"]) != `true` {
		t.Errorf("other filters changed: %v", f)
	}
	if string(env["pagination"]) != `{"limit":5,"offset":10}` || string(env["sortBy"]) != `{"field":"name","order":"asc"}` {
		t.Errorf("pagination/sortBy changed: %s %s", env["pagination"], env["sortBy"])
	}
}

// Team resolution still runs, and its output survives the override.
func TestSearchUsers_ForcedUserTypeKeepsTeamResolution(t *testing.T) {
	env, w := searchVia(t, false, `{"filters":{"teamIds":["abt-1"],"userType":"external"}}`)
	assertStatus(t, w, http.StatusOK)
	f := filtersOf(t, env)
	if string(f["groupNames"]) != `["ABT One"]` || string(f["userType"]) != `"internal"` {
		t.Errorf("filters = %v, want groupNames resolved and userType internal", f)
	}
}

// A body the override cannot reason about is rejected, never forwarded.
func TestSearchUsers_MalformedFiltersRejected(t *testing.T) {
	for name, body := range map[string]string{
		"filters string": `{"filters":"x"}`,
		"array body":     `[]`,
		"null body":      `null`,
	} {
		t.Run(name, func(t *testing.T) {
			env, w := searchVia(t, false, body)
			assertStatus(t, w, http.StatusBadRequest)
			if env != nil {
				t.Errorf("entity service was called with %v", env)
			}
		})
	}
}

// The admin search does not touch userType: external, internal and omitted all
// reach the entity service as sent.
func TestSearchUsersAdmin_DoesNotForceUserType(t *testing.T) {
	env, w := searchVia(t, true, `{"filters":{"searchQuery":"jane","userType":"external"}}`)
	assertStatus(t, w, http.StatusOK)
	if got := string(filtersOf(t, env)["userType"]); got != `"external"` {
		t.Errorf("forwarded userType = %s, want \"external\" untouched", got)
	}

	env, w = searchVia(t, true, `{"filters":{"searchQuery":"jane"}}`)
	assertStatus(t, w, http.StatusOK)
	if _, ok := filtersOf(t, env)["userType"]; ok {
		t.Errorf("admin search added a userType the caller did not send")
	}
}

// The admin search shares the request handling, so it still resolves teams.
func TestSearchUsersAdmin_StillResolvesTeams(t *testing.T) {
	env, w := searchVia(t, true, `{"filters":{"teamIds":["abt-1"]}}`)
	assertStatus(t, w, http.StatusOK)
	if got := string(filtersOf(t, env)["groupNames"]); got != `["ABT One"]` {
		t.Errorf("groupNames = %s", got)
	}
}
