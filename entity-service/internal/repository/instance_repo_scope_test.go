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

package repository

import (
	"reflect"
	"testing"
)

func TestInstanceIDFilterClause_AppliesCallerScope(t *testing.T) {
	member := SearchScope{ProjectIDs: []string{"p-1", "p-2"}, ViewerEmail: "jane.doe@example.com"}
	cases := []struct {
		name                         string
		scope                        SearchScope
		projects, deps, deployedProd []string
		argIdx                       int
		wantSQL                      string
		wantArgs                     []any
	}{
		{
			name:     "internal caller, no request filter: unfiltered",
			scope:    SearchScope{Unrestricted: true},
			argIdx:   3,
			wantSQL:  "",
			wantArgs: nil,
		},
		{
			name:     "internal caller keeps the request's project filter",
			scope:    SearchScope{Unrestricted: true},
			projects: []string{"p-9"},
			argIdx:   3,
			wantSQL:  " AND proj.id = ANY($3::uuid[])",
			wantArgs: []any{[]string{"p-9"}},
		},
		{
			name:     "member with no request filter is held to their projects",
			scope:    member,
			argIdx:   3,
			wantSQL:  " AND proj.id = ANY($3::text[]::uuid[])",
			wantArgs: []any{[]string{"p-1", "p-2"}},
		},
		{
			name:     "member asking for another tenant's project gets the intersection",
			scope:    member,
			projects: []string{"other-tenant"},
			argIdx:   1,
			wantSQL:  " AND proj.id = ANY($1::uuid[]) AND proj.id = ANY($2::text[]::uuid[])",
			wantArgs: []any{[]string{"other-tenant"}, []string{"p-1", "p-2"}},
		},
		{
			name:     "member filtering by deployment is still held to their projects",
			scope:    member,
			deps:     []string{"d-1"},
			argIdx:   3,
			wantSQL:  " AND dep.id = ANY($3::uuid[]) AND proj.id = ANY($4::text[]::uuid[])",
			wantArgs: []any{[]string{"d-1"}, []string{"p-1", "p-2"}},
		},
		{
			name:         "member filtering by deployed product is still held to their projects",
			scope:        member,
			deployedProd: []string{"dp-1"},
			argIdx:       3,
			wantSQL:      " AND dprod.id = ANY($3::uuid[]) AND proj.id = ANY($4::text[]::uuid[])",
			wantArgs:     []any{[]string{"dp-1"}, []string{"p-1", "p-2"}},
		},
		{
			name:     "no identity (zero scope) matches nothing, never everything",
			scope:    SearchScope{},
			argIdx:   3,
			wantSQL:  " AND proj.id = ANY($3::text[]::uuid[])",
			wantArgs: []any{[]string{}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sql, args := instanceIDFilterClause(tc.scope, tc.projects, tc.deps, tc.deployedProd, tc.argIdx)
			if sql != tc.wantSQL {
				t.Errorf("sql = %q, want %q", sql, tc.wantSQL)
			}
			if !reflect.DeepEqual(args, tc.wantArgs) {
				t.Errorf("args = %#v, want %#v", args, tc.wantArgs)
			}
		})
	}
}
