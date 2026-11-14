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

// internalUsers classifies people as INTERNAL-type users (WSO2 staff), by the
// user row itself. A reference that carries an id is classified by that id.
// A reference with only an email is classified by the one user row
// userByEmailJoin would pick for it (active first, newest, then id), because
// "user".email is not unique: another row with the same email must not decide.
// Both results are keyed lower-case.
func internalUsers(ctx context.Context, q querier, ids, emails []string) (byID, byEmail map[string]bool, err error) {
	byID, byEmail = map[string]bool{}, map[string]bool{}
	if len(ids) > 0 {
		rows, qerr := q.Query(ctx, `SELECT id::text, user_type = 'INTERNAL' FROM "user" WHERE id::text = ANY($1)`, ids)
		if qerr != nil {
			return nil, nil, fmt.Errorf("look up users by id: %w", qerr)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var internal bool
			if err := rows.Scan(&id, &internal); err != nil {
				return nil, nil, fmt.Errorf("scan user by id: %w", err)
			}
			byID[strings.ToLower(id)] = internal
		}
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
	}
	if len(emails) > 0 {
		rows, qerr := q.Query(ctx, `SELECT DISTINCT ON (LOWER(email)) LOWER(email), user_type = 'INTERNAL'
			FROM "user" WHERE LOWER(email) = ANY($1)
			ORDER BY LOWER(email), is_active DESC NULLS LAST, created_on DESC, id`, emails)
		if qerr != nil {
			return nil, nil, fmt.Errorf("look up users by email: %w", qerr)
		}
		defer rows.Close()
		for rows.Next() {
			var e string
			var internal bool
			if err := rows.Scan(&e, &internal); err != nil {
				return nil, nil, fmt.Errorf("scan user by email: %w", err)
			}
			byEmail[e] = internal
		}
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
	}
	return byID, byEmail, nil
}

// markInternalRefs sets IsInternal on each reference whose user is an
// INTERNAL-type user. Best effort: a failed lookup leaves the flag unset
// rather than failing the list it belongs to.
func markInternalRefs(ctx context.Context, q querier, refs []*domain.UserReference) {
	var ids, emails []string
	seenID, seenEmail := map[string]bool{}, map[string]bool{}
	for _, r := range refs {
		if r == nil {
			continue
		}
		if r.ID != nil && strings.TrimSpace(*r.ID) != "" {
			id := strings.ToLower(strings.TrimSpace(*r.ID))
			if !seenID[id] {
				seenID[id] = true
				ids = append(ids, id)
			}
			continue
		}
		e := strings.ToLower(strings.TrimSpace(r.Email))
		if e != "" && !seenEmail[e] {
			seenEmail[e] = true
			emails = append(emails, e)
		}
	}
	if len(ids) == 0 && len(emails) == 0 {
		return
	}
	byID, byEmail, err := internalUsers(ctx, q, ids, emails)
	if err != nil {
		slog.WarnContext(ctx, "could not mark internal users", "err", err)
		return
	}
	for _, r := range refs {
		if r == nil {
			continue
		}
		if r.ID != nil && strings.TrimSpace(*r.ID) != "" {
			r.IsInternal = byID[strings.ToLower(strings.TrimSpace(*r.ID))]
		} else {
			r.IsInternal = byEmail[strings.ToLower(strings.TrimSpace(r.Email))]
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
