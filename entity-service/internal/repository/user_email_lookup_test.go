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
	"strings"
	"testing"
)

// The email lookups must stay index-friendly: a per-row LATERAL probe on
// LOWER(email) and a lower(email) = lower($1) index condition, never a
// DISTINCT ON derived table over the whole "user" table.
func TestUserByEmailJoinShape(t *testing.T) {
	got := userByEmailJoin("u", "c.created_by")
	for _, want := range []string{
		"LEFT JOIN LATERAL (",
		"SELECT u2.id, u2.name, u2.first_name, u2.last_name",
		"WHERE LOWER(u2.email) = LOWER(c.created_by)",
		"ORDER BY u2.is_active DESC NULLS LAST, u2.created_on DESC, u2.id",
		"LIMIT 1",
		") u ON TRUE",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("join missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "DISTINCT ON") || strings.Contains(got, "email_key") {
		t.Errorf("join must not use the DISTINCT ON derived table:\n%s", got)
	}
}

func TestGetUserByEmailSQLShape(t *testing.T) {
	norm := strings.Join(strings.Fields(getUserByEmailSQL), " ")
	for _, want := range []string{
		"WHERE lower(email) = lower($1) AND (email = $1 OR ($2 AND email !~ '[^[:ascii:]]'))",
		"ORDER BY (email = $1) DESC LIMIT 2",
	} {
		if !strings.Contains(norm, want) {
			t.Errorf("query missing %q in:\n%s", want, norm)
		}
	}
	if strings.Contains(norm, "WHERE email = $1 OR") {
		t.Errorf("query must not use the unindexable OR form:\n%s", norm)
	}
}
