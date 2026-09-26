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

import "testing"

func TestScopePredicate(t *testing.T) {
	if got, want := scopePredicate("wi.project_id", 3), "wi.project_id = ANY($3::text[]::uuid[])"; got != want {
		t.Fatalf("scopePredicate = %q, want %q", got, want)
	}

	var f searchFilter
	f.scope("p.id", SearchScope{Unrestricted: true})
	if f.where != "" || len(f.args) != 0 {
		t.Fatalf("unrestricted scope must add nothing, got where=%q args=%v", f.where, f.args)
	}
	f.scope("p.id", SearchScope{ProjectIDs: []string{}})
	if f.where != " AND p.id = ANY($1::text[]::uuid[])" || len(f.args) != 1 {
		t.Fatalf("empty restricted scope must still filter (match nothing), got where=%q args=%v", f.where, f.args)
	}
}
