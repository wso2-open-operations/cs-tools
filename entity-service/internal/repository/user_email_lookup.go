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
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

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
				SELECT u2.id, u2.name, u2.first_name, u2.last_name, u2.user_type
				FROM "user" u2
				WHERE LOWER(u2.email) = LOWER(` + emailExpr + `)
				ORDER BY u2.is_active DESC NULLS LAST, u2.created_on DESC, u2.id
				LIMIT 1
			) ` + alias + ` ON TRUE`
}

// internalUserEmails returns which of emails (compared case-insensitively)
// belong to an INTERNAL-type user, keyed by the lower-cased email. A response
// that shows a person's name uses it to mark WSO2 staff, one query for the
// whole page rather than a column on every list query.
func internalUserEmails(ctx context.Context, q querier, emails []string) (map[string]bool, error) {
	out := map[string]bool{}
	lowered := make([]string, 0, len(emails))
	seen := map[string]bool{}
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" && !seen[e] {
			seen[e] = true
			lowered = append(lowered, e)
		}
	}
	if len(lowered) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT DISTINCT LOWER(email) FROM "user"
		WHERE user_type = 'INTERNAL' AND LOWER(email) = ANY($1)`, lowered)
	if err != nil {
		return nil, fmt.Errorf("look up internal users: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, fmt.Errorf("scan internal user: %w", err)
		}
		out[e] = true
	}
	return out, rows.Err()
}

// markInternalRefs sets IsInternal on each reference whose email belongs to an
// INTERNAL-type user. Best effort: a failed lookup leaves the flag unset
// rather than failing the list it belongs to.
func markInternalRefs(ctx context.Context, q querier, refs []*domain.UserReference) {
	emails := make([]string, 0, len(refs))
	for _, r := range refs {
		if r != nil {
			emails = append(emails, r.Email)
		}
	}
	internal, err := internalUserEmails(ctx, q, emails)
	if err != nil {
		slog.WarnContext(ctx, "could not mark internal users", "err", err)
		return
	}
	for _, r := range refs {
		if r != nil && internal[strings.ToLower(strings.TrimSpace(r.Email))] {
			r.IsInternal = true
		}
	}
}

// markInternalUploaders is markInternalRefs for a page of attachments.
func markInternalUploaders(ctx context.Context, q querier, attachments []domain.Attachment) {
	refs := make([]*domain.UserReference, 0, len(attachments))
	for _, a := range attachments {
		refs = append(refs, a.CreatedBy)
	}
	markInternalRefs(ctx, q, refs)
}

// markInternalActors is markInternalRefs for a page of case activities.
func markInternalActors(ctx context.Context, q querier, activities []domain.CaseActivity) {
	refs := make([]*domain.UserReference, 0, len(activities))
	for _, a := range activities {
		refs = append(refs, a.CreatedBy)
	}
	markInternalRefs(ctx, q, refs)
}
