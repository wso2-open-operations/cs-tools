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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// TestProjectContactRowToDomain_AccountRoles pins that the contacts search
// carries the account-level roles as their own field, never merged into the
// project roles: one answers "what may they do on this project", the other
// "what are they across the account". Both portals read the admin entry to
// render a badge without a second call per row.
func TestProjectContactRowToDomain_AccountRoles(t *testing.T) {
	cases := []struct {
		name             string
		row              repository.ProjectContactRow
		wantRoles        []string
		wantAccountRoles []string
	}{
		{
			name: "an account admin carries the derived role alongside their project roles",
			row: repository.ProjectContactRow{
				Email:          "jane@acme.com",
				ResolvedUserID: sampleStr("9a2e8d6a-3b4c-4d5e-8f90-123456789abc"),
				ResolvedEmail:  sampleStr("jane@acme.com"),
				Roles:          []string{"PORTAL_USER", "ADMIN"},
				AccountRoles:   []string{"customer", "customer_admin", "external"},
			},
			wantRoles:        []string{"PORTAL_USER", "ADMIN"},
			wantAccountRoles: []string{"customer", "customer_admin", "external"},
		},
		{
			name: "a non-admin carries no admin entry",
			row: repository.ProjectContactRow{
				Email:          "bob@acme.com",
				ResolvedUserID: sampleStr("7c2e8d6a-3b4c-4d5e-8f90-123456789abc"),
				Roles:          []string{"PORTAL_USER"},
				AccountRoles:   []string{"customer", "external"},
			},
			wantRoles:        []string{"PORTAL_USER"},
			wantAccountRoles: []string{"customer", "external"},
		},
		{
			name: "a row with no linked user has neither list, rendered as empty rather than null",
			row: repository.ProjectContactRow{
				Email: "ghost@acme.com",
			},
			wantRoles:        []string{},
			wantAccountRoles: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := projectContactRowToDomain(tc.row)
			if !reflect.DeepEqual(got.Roles, tc.wantRoles) {
				t.Errorf("roles = %v, want %v", got.Roles, tc.wantRoles)
			}
			if !reflect.DeepEqual(got.AccountRoles, tc.wantAccountRoles) {
				t.Errorf("accountRoles = %v, want %v", got.AccountRoles, tc.wantAccountRoles)
			}
			// The two lists must stay separate: a project role must never
			// leak into the account list, and vice versa.
			for _, r := range got.Roles {
				for _, a := range got.AccountRoles {
					if r == a {
						t.Errorf("the two lists must not be merged; %q appears in both", r)
					}
				}
			}
		})
	}
}
