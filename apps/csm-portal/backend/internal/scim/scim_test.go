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

package scim

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// testClient builds a Client pointed at srv, bypassing OAuth2 entirely (the
// client-credentials token fetch is irrelevant to what these tests exercise).
func testClient(srv *httptest.Server) *Client {
	return &Client{http: srv.Client(), baseURL: srv.URL}
}

func TestClient_GetRole(t *testing.T) {
	t.Run("hits the internal org's role-by-id path and extracts member emails", func(t *testing.T) {
		var gotMethod, gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id": "0bbeea4f-5ada-49ba-8f19-90ae6a116daa",
				"displayName": "timecard-approver-role-stg",
				"users": [
					{"display": "DEFAULT/jane.doe@wso2.com", "value": "70aa5b28-c8ca-4fc7-9da4-6a9ebfaad99e"},
					{"display": "DEFAULT/john.smith@wso2.com", "value": "81bb6c39-d9db-5gd8-ae15-7b0fcbbae0a0"}
				]
			}`))
		}))
		defer srv.Close()

		members, err := testClient(srv).GetRole(context.Background(), "0bbeea4f-5ada-49ba-8f19-90ae6a116daa")
		if err != nil {
			t.Fatalf("GetRole: %v", err)
		}

		if gotMethod != http.MethodGet {
			t.Errorf("method = %q, want GET", gotMethod)
		}
		if gotPath != "/organizations/internal/roles/0bbeea4f-5ada-49ba-8f19-90ae6a116daa" {
			t.Errorf("path = %q, want /organizations/internal/roles/0bbeea4f-5ada-49ba-8f19-90ae6a116daa", gotPath)
		}

		want := []RoleMember{
			{ID: "70aa5b28-c8ca-4fc7-9da4-6a9ebfaad99e", Email: "jane.doe@wso2.com"},
			{ID: "81bb6c39-d9db-5gd8-ae15-7b0fcbbae0a0", Email: "john.smith@wso2.com"},
		}
		if !reflect.DeepEqual(members, want) {
			t.Errorf("members = %+v, want %+v", members, want)
		}
	})

	t.Run("a display with no domain prefix falls back to the raw value", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id": "r-1", "displayName": "role", "users": [{"display": "jane.doe@wso2.com", "value": "u-1"}]}`))
		}))
		defer srv.Close()

		members, err := testClient(srv).GetRole(context.Background(), "r-1")
		if err != nil {
			t.Fatalf("GetRole: %v", err)
		}
		want := []RoleMember{{ID: "u-1", Email: "jane.doe@wso2.com"}}
		if !reflect.DeepEqual(members, want) {
			t.Errorf("members = %+v, want %+v", members, want)
		}
	})

	t.Run("a role with no users returns an empty, non-nil slice", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id": "r-1", "displayName": "role", "users": []}`))
		}))
		defer srv.Close()

		members, err := testClient(srv).GetRole(context.Background(), "r-1")
		if err != nil {
			t.Fatalf("GetRole: %v", err)
		}
		if members == nil || len(members) != 0 {
			t.Errorf("members = %+v, want an empty non-nil slice", members)
		}
	})

	t.Run("propagates a 404 as apierror.Error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"role not found"}`))
		}))
		defer srv.Close()

		_, err := testClient(srv).GetRole(context.Background(), "unknown-id")
		apiErr, ok := err.(*apierror.Error)
		if !ok {
			t.Fatalf("err = %v (%T), want *apierror.Error", err, err)
		}
		if apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
		}
	})
}

func TestClient_SearchUser(t *testing.T) {
	t.Run("requests the id attribute and returns it on UserInfo", func(t *testing.T) {
		// Regression test: the search request's "attributes" list controls
		// exactly which fields Asgardeo returns -- id is not included for
		// free, so a request that omits it comes back with an empty id and
		// silently breaks GetUserByID's caller downstream (withPortalRoles).
		var gotBody struct {
			Attributes []string `json:"attributes"`
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"totalResults": 1,
				"itemsPerPage": 1,
				"Resources": [{"id": "70aa5b28-c8ca-4fc7-9da4-6a9ebfaad99e", "userName": "DEFAULT/jane.doe@wso2.com"}]
			}`))
		}))
		defer srv.Close()

		info, err := testClient(srv).SearchUser(context.Background(), "jane.doe@wso2.com")
		if err != nil {
			t.Fatalf("SearchUser: %v", err)
		}

		found := false
		for _, a := range gotBody.Attributes {
			if a == "id" {
				found = true
			}
		}
		if !found {
			t.Errorf("request attributes = %v, want it to include %q", gotBody.Attributes, "id")
		}

		if info == nil || info.ID != "70aa5b28-c8ca-4fc7-9da4-6a9ebfaad99e" {
			t.Errorf("info = %+v, want ID = %q", info, "70aa5b28-c8ca-4fc7-9da4-6a9ebfaad99e")
		}
	})
}

func TestClient_GetUserByID(t *testing.T) {
	t.Run("hits the internal org's user-by-id path and extracts role and group assignments by ID and name", func(t *testing.T) {
		var gotMethod, gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id": "70aa5b28-c8ca-4fc7-9da4-6a9ebfaad99e",
				"userName": "DEFAULT/jane.doe@wso2.com",
				"roles": [
					{"value": "role-id-1", "display": "app-csm-admin", "audienceType": "application"},
					{"value": "role-id-2", "display": "app-csm-admin", "audienceType": "application"}
				],
				"groups": [
					{"value": "group-id-1", "display": "DEFAULT/csm-engineers"},
					{"value": "group-id-2", "display": "DEFAULT/csm-engineers"}
				]
			}`))
		}))
		defer srv.Close()

		assignments, err := testClient(srv).GetUserByID(context.Background(), "70aa5b28-c8ca-4fc7-9da4-6a9ebfaad99e")
		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}

		if gotMethod != http.MethodGet {
			t.Errorf("method = %q, want GET", gotMethod)
		}
		if gotPath != "/organizations/internal/users/70aa5b28-c8ca-4fc7-9da4-6a9ebfaad99e" {
			t.Errorf("path = %q, want /organizations/internal/users/70aa5b28-c8ca-4fc7-9da4-6a9ebfaad99e", gotPath)
		}

		// Two distinct roles sharing the same display name must both survive
		// -- this is the whole reason GetUserByID returns IDs, not names.
		wantRoles := []RoleAssignment{
			{ID: "role-id-1", Name: "app-csm-admin"},
			{ID: "role-id-2", Name: "app-csm-admin"},
		}
		if !reflect.DeepEqual(assignments.Roles, wantRoles) {
			t.Errorf("assignments.Roles = %+v, want %+v", assignments.Roles, wantRoles)
		}

		// Same reasoning for groups: two distinct groups sharing a display
		// name must both survive, by ID. The fixture's "DEFAULT/" prefix is
		// the real shape Asgardeo returns (confirmed live) -- Name must come
		// back with it stripped, exactly like a role member's email already
		// does via emailFromDisplay.
		wantGroups := []GroupAssignment{
			{ID: "group-id-1", Name: "csm-engineers"},
			{ID: "group-id-2", Name: "csm-engineers"},
		}
		if !reflect.DeepEqual(assignments.Groups, wantGroups) {
			t.Errorf("assignments.Groups = %+v, want %+v (the \"DEFAULT/\" prefix must be stripped)", assignments.Groups, wantGroups)
		}
	})

	t.Run("a user with no roles or groups returns empty, non-nil slices", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id": "u-1", "userName": "DEFAULT/jane.doe@wso2.com"}`))
		}))
		defer srv.Close()

		assignments, err := testClient(srv).GetUserByID(context.Background(), "u-1")
		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}
		if assignments.Roles == nil || len(assignments.Roles) != 0 {
			t.Errorf("assignments.Roles = %+v, want an empty non-nil slice", assignments.Roles)
		}
		if assignments.Groups == nil || len(assignments.Groups) != 0 {
			t.Errorf("assignments.Groups = %+v, want an empty non-nil slice", assignments.Groups)
		}
	})

	t.Run("propagates a 404 as apierror.Error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"user not found"}`))
		}))
		defer srv.Close()

		_, err := testClient(srv).GetUserByID(context.Background(), "unknown-id")
		apiErr, ok := err.(*apierror.Error)
		if !ok {
			t.Fatalf("err = %v (%T), want *apierror.Error", err, err)
		}
		if apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
		}
	})
}

func TestClient_AddRoleMembers(t *testing.T) {
	t.Run("POSTs the internal org's role-members path with the given emails", func(t *testing.T) {
		var gotMethod, gotPath string
		var gotBody scimAddRoleMembersRequest
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"addedUsers":["jane.doe@wso2.com"],"failedUsers":[],"addedGroups":[],"failedGroups":[]}`))
		}))
		defer srv.Close()

		err := testClient(srv).AddRoleMembers(context.Background(), "11111111-1111-1111-1111-111111111111", []string{"jane.doe@wso2.com"})
		if err != nil {
			t.Fatalf("AddRoleMembers: %v", err)
		}
		if gotMethod != http.MethodPost {
			t.Errorf("method = %q, want POST", gotMethod)
		}
		if gotPath != "/organizations/internal/roles/11111111-1111-1111-1111-111111111111/users" {
			t.Errorf("path = %q", gotPath)
		}
		if len(gotBody.Emails) != 1 || gotBody.Emails[0] != "jane.doe@wso2.com" {
			t.Errorf("request body emails = %+v", gotBody.Emails)
		}
	})

	t.Run("an email the SCIM service couldn't resolve is reported as an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"addedUsers":[],"failedUsers":["unknown@wso2.com"],"addedGroups":[],"failedGroups":[]}`))
		}))
		defer srv.Close()

		err := testClient(srv).AddRoleMembers(context.Background(), "r-1", []string{"unknown@wso2.com"})
		if err == nil {
			t.Fatal("expected an error for a failed email, got nil")
		}
		if !strings.Contains(err.Error(), "unknown@wso2.com") {
			t.Errorf("error %q does not name the failed email", err)
		}
	})

	t.Run("propagates a 404 as apierror.Error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"role not found"}`))
		}))
		defer srv.Close()

		err := testClient(srv).AddRoleMembers(context.Background(), "unknown-id", []string{"jane.doe@wso2.com"})
		apiErr, ok := err.(*apierror.Error)
		if !ok {
			t.Fatalf("err = %v (%T), want *apierror.Error", err, err)
		}
		if apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
		}
	})
}
