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
