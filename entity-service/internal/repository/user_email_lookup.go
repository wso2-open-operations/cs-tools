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

// userByEmailJoin returns a LEFT JOIN LATERAL that resolves emailExpr (a
// free-text created_by / user_email column) to at most one "user" row,
// exposed as alias with id, name, first_name and last_name.
//
// "user".email is not unique (only user_name is), and migrated records leave
// several rows per address, so a plain email join fans one source row out
// into one result row per match. The lateral subquery keeps one row per
// source row, chosen deterministically: active first, then most recently
// created, then lowest id.
//
// It relies on the LOWER(email) index idx_user_email_lower (migration 0192):
// each source row does an index probe on LOWER(u2.email) = LOWER(emailExpr)
// and sorts only the few rows sharing that address. Do not replace this with
// a DISTINCT ON derived table: that blocks predicate pushdown and forces a
// full scan and sort of "user" on every call.
func userByEmailJoin(alias, emailExpr string) string {
	return `LEFT JOIN LATERAL (
				SELECT u2.id, u2.name, u2.first_name, u2.last_name
				FROM "user" u2
				WHERE LOWER(u2.email) = LOWER(` + emailExpr + `)
				ORDER BY u2.is_active DESC NULLS LAST, u2.created_on DESC, u2.id
				LIMIT 1
			) ` + alias + ` ON TRUE`
}
