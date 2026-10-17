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
	"encoding/json"
	"reflect"
	"testing"
)

// TestScimRoles_UnmarshalJSON is the regression guard for two real bugs found
// in sequence against a real SCIM service: (1) "roles" is an array of
// {value, display, ...} objects, not a bare string/array of strings as first
// assumed -- decoding it the naive way failed every GetUser call for an
// internal user with "json: cannot unmarshal object into Go value of type
// string"; (2) once that was fixed, "value" turned out to be the role
// resource's own opaque id (a UUID), not its name -- a role id is never a
// readable string, so anything trying to recognize a role by matching
// against Value alone would silently match nothing. The actual role name is
// "display" instead. Every shape scimRoles claims to accept is exercised
// here so a future regression on any one of them fails a test instead of
// only being caught live again.
func TestScimRoles_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    []string
		wantErr bool
	}{
		{name: "array of value/display objects (the real shape)", json: `[{"value":"11111111-1111-1111-1111-111111111111","display":"app-csm-example-role-one","audienceType":"application"},{"value":"22222222-2222-2222-2222-222222222222","display":"app-csm-example-role-two"}]`, want: []string{"app-csm-example-role-one", "app-csm-example-role-two"}},
		{name: "single value/display object", json: `{"value":"22222222-2222-2222-2222-222222222222","display":"app-csm-example-role-two"}`, want: []string{"app-csm-example-role-two"}},
		{name: "array of strings", json: `["app-csm-example-role-one","app-csm-example-role-two"]`, want: []string{"app-csm-example-role-one", "app-csm-example-role-two"}},
		{name: "bare string", json: `"app-csm-example-role-two"`, want: []string{"app-csm-example-role-two"}},
		{name: "empty string", json: `""`, want: nil},
		{name: "null", json: `null`, want: nil},
		{name: "empty array", json: `[]`, want: []string{}},
		{name: "malformed", json: `42`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got scimRoles
			err := json.Unmarshal([]byte(tt.json), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual([]string(got), tt.want) {
				t.Errorf("got %#v, want %#v", []string(got), tt.want)
			}
		})
	}
}

// TestScimRoleAssignments_UnmarshalJSON mirrors TestScimRoles_UnmarshalJSON's
// own shape coverage, but for the decoder GetUserByID uses: unlike scimRoles,
// which collapses each entry to its display name alone, scimRoleAssignments
// keeps each entry's ID (the role's own Asgardeo UUID) too -- the whole
// reason GetUserByID exists instead of trusting a search response's own
// "roles" field (see that method's own doc comment): matching a held role
// against ASGARDEO_ROLE_IDS needs the ID, since two distinct roles can share
// a display name. A shape with no id available (a bare string, or an
// object missing "value") decodes with an empty ID rather than failing.
func TestScimRoleAssignments_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    []RoleAssignment
		wantErr bool
	}{
		{
			name: "array of value/display objects (the real shape)",
			json: `[{"value":"11111111-1111-1111-1111-111111111111","display":"app-csm-example-role-one","audienceType":"application"},{"value":"22222222-2222-2222-2222-222222222222","display":"app-csm-example-role-two"}]`,
			want: []RoleAssignment{
				{ID: "11111111-1111-1111-1111-111111111111", Name: "app-csm-example-role-one"},
				{ID: "22222222-2222-2222-2222-222222222222", Name: "app-csm-example-role-two"},
			},
		},
		{
			name: "two distinct roles sharing the same display name keep distinct IDs",
			json: `[{"value":"11111111-1111-1111-1111-111111111111","display":"app-csm-admin"},{"value":"22222222-2222-2222-2222-222222222222","display":"app-csm-admin"}]`,
			want: []RoleAssignment{
				{ID: "11111111-1111-1111-1111-111111111111", Name: "app-csm-admin"},
				{ID: "22222222-2222-2222-2222-222222222222", Name: "app-csm-admin"},
			},
		},
		{
			name: "single value/display object",
			json: `{"value":"22222222-2222-2222-2222-222222222222","display":"app-csm-example-role-two"}`,
			want: []RoleAssignment{{ID: "22222222-2222-2222-2222-222222222222", Name: "app-csm-example-role-two"}},
		},
		{
			name: "array of strings has no id available",
			json: `["app-csm-example-role-one","app-csm-example-role-two"]`,
			want: []RoleAssignment{{Name: "app-csm-example-role-one"}, {Name: "app-csm-example-role-two"}},
		},
		{name: "bare string has no id available", json: `"app-csm-example-role-two"`, want: []RoleAssignment{{Name: "app-csm-example-role-two"}}},
		{name: "empty string", json: `""`, want: nil},
		{name: "null", json: `null`, want: nil},
		{name: "empty array", json: `[]`, want: []RoleAssignment{}},
		{name: "malformed", json: `42`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got scimRoleAssignments
			err := json.Unmarshal([]byte(tt.json), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual([]RoleAssignment(got), tt.want) {
				t.Errorf("got %#v, want %#v", []RoleAssignment(got), tt.want)
			}
		})
	}
}

// TestScimGroupAssignments_UnmarshalJSON mirrors TestScimRoleAssignments_UnmarshalJSON
// for groups, with one addition every case below exercises: a group's own
// SCIM "display" comes back prefixed "<domain>/<name>" the same way a role
// member's email does (confirmed live against a real deployment) -- Name
// must always come back with that prefix already stripped, never the raw
// "<domain>/..." value.
func TestScimGroupAssignments_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    []GroupAssignment
		wantErr bool
	}{
		{
			name: "array of value/display objects (the real shape), domain prefix stripped",
			json: `[{"value":"11111111-1111-1111-1111-111111111111","display":"DEFAULT/example-group-one"},{"value":"22222222-2222-2222-2222-222222222222","display":"DEFAULT/example-group-two"}]`,
			want: []GroupAssignment{
				{ID: "11111111-1111-1111-1111-111111111111", Name: "example-group-one"},
				{ID: "22222222-2222-2222-2222-222222222222", Name: "example-group-two"},
			},
		},
		{
			name: "two distinct groups sharing the same display name keep distinct IDs",
			json: `[{"value":"11111111-1111-1111-1111-111111111111","display":"DEFAULT/example-group"},{"value":"22222222-2222-2222-2222-222222222222","display":"DEFAULT/example-group"}]`,
			want: []GroupAssignment{
				{ID: "11111111-1111-1111-1111-111111111111", Name: "example-group"},
				{ID: "22222222-2222-2222-2222-222222222222", Name: "example-group"},
			},
		},
		{
			name: "single value/display object, domain prefix stripped",
			json: `{"value":"22222222-2222-2222-2222-222222222222","display":"DEFAULT/example-group-two"}`,
			want: []GroupAssignment{{ID: "22222222-2222-2222-2222-222222222222", Name: "example-group-two"}},
		},
		{
			name: "array of strings, domain prefix stripped, no id available",
			json: `["DEFAULT/example-group-one","DEFAULT/example-group-two"]`,
			want: []GroupAssignment{{Name: "example-group-one"}, {Name: "example-group-two"}},
		},
		{
			name: "bare string, domain prefix stripped, no id available",
			json: `"DEFAULT/example-group-two"`,
			want: []GroupAssignment{{Name: "example-group-two"}},
		},
		{
			name: "a display with no domain prefix at all is kept as-is",
			json: `{"value":"33333333-3333-3333-3333-333333333333","display":"example-group-no-prefix"}`,
			want: []GroupAssignment{{ID: "33333333-3333-3333-3333-333333333333", Name: "example-group-no-prefix"}},
		},
		{name: "empty string", json: `""`, want: nil},
		{name: "null", json: `null`, want: nil},
		{name: "empty array", json: `[]`, want: []GroupAssignment{}},
		{name: "malformed", json: `42`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got scimGroupAssignments
			err := json.Unmarshal([]byte(tt.json), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual([]GroupAssignment(got), tt.want) {
				t.Errorf("got %#v, want %#v", []GroupAssignment(got), tt.want)
			}
		})
	}
}

// TestSearchUser_ExtractsRolesFromValueObjects proves SearchUser's own
// end-to-end decoding (scimSearchResponse -> scimUser -> UserInfo) survives
// the real object-array shape, not just the isolated scimRoles unit above.
// The fixture is entirely synthetic: several other-application roles
// alongside a couple of this-portal ones, plus an organization-scope role --
// UserInfo.Roles must carry every one of them through unfiltered, since
// recognizing which belong to this portal is AccessGuard.RolesFor's job
// (matched against its own AUTH_<ROLE>_ROLES configuration), not this
// package's.
func TestSearchUser_ExtractsRolesFromValueObjects(t *testing.T) {
	raw := []byte(`{
		"totalResults": 1,
		"startIndex": 1,
		"itemsPerPage": 1,
		"Resources": [{
			"id": "11111111-1111-1111-1111-111111111111",
			"roles": [
				{"value": "aaaaaaaa-0000-0000-0000-000000000001", "display": "some-other-app-admin", "audienceType": "application"},
				{"value": "aaaaaaaa-0000-0000-0000-000000000002", "display": "app-csm-example-role-one", "audienceType": "application"},
				{"value": "aaaaaaaa-0000-0000-0000-000000000003", "display": "app-csm-example-role-two", "audienceType": "application"},
				{"value": "aaaaaaaa-0000-0000-0000-000000000004", "display": "everyone", "audienceType": "organization"}
			]
		}]
	}`)
	var result scimSearchResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("Resources = %d, want 1", len(result.Resources))
	}
	want := []string{"some-other-app-admin", "app-csm-example-role-one", "app-csm-example-role-two", "everyone"}
	if !reflect.DeepEqual([]string(result.Resources[0].Roles), want) {
		t.Errorf("Roles = %v, want %v", []string(result.Resources[0].Roles), want)
	}
}
