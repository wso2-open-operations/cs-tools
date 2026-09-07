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
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// watchersQuery reads one case's watch list. The join is LEFT because a watcher
// need not be a platform user: entries carried over from ServiceNow's collapsed
// glide_lists may name a group or a bare address, and those rows keep a NULL
// user_id (see migration 000017).
//
// Ordered by added_at so the list reads in the order people were added, with
// email as the tie-break to keep the order total -- two watchers added in the
// same transaction share an added_at, and an unstable order would make the
// response differ between identical reads.
const watchersQuery = `
	SELECT w.user_id, w.email,
	       COALESCE(u.user_name, ''),
	       COALESCE(TRIM(u.first_name || ' ' || u.last_name), '')
	FROM case_watchers w
	LEFT JOIN users u ON u.id = w.user_id
	WHERE w.case_id = $1
	ORDER BY w.added_at, w.email`

// listCaseWatchers runs watchersQuery against any query-capable handle, so it
// serves both the standalone read and the read-back inside ReplaceCaseWatchers'
// transaction (which must see that transaction's own writes).
func listCaseWatchers(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, caseID string) ([]domain.WatchListUser, error) {
	rows, err := q.Query(ctx, watchersQuery, caseID)
	if err != nil {
		return nil, fmt.Errorf("query case watchers: %w", err)
	}
	defer rows.Close()

	watchers := make([]domain.WatchListUser, 0)
	for rows.Next() {
		var userID *string
		var email, userName, name string
		if err := rows.Scan(&userID, &email, &userName, &name); err != nil {
			return nil, fmt.Errorf("scan case watcher: %w", err)
		}
		w := domain.WatchListUser{UserName: userName, Name: name, Email: email}
		// Unlike the ServiceNow path -- which nulls the reference id because a
		// collapsed glide_list entry might not be a user at all -- a non-NULL
		// user_id here is a foreign key into users, so the id is known to be a
		// user's and is emitted. A NULL user_id still yields a null id.
		if userID != nil {
			w.ID = *userID
			w.User = domain.NewUserReference(*userID, email, name)
		} else {
			w.User = domain.NewUserReference("", email, name)
		}
		watchers = append(watchers, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate case watchers: %w", err)
	}
	return watchers, nil
}

// ListCaseWatchers implements CaseRepository.
func (r *caseRepo) ListCaseWatchers(ctx context.Context, caseID string) ([]domain.WatchListUser, error) {
	return listCaseWatchers(ctx, r.db, caseID)
}

// ReplaceCaseWatchers implements CaseRepository.
//
// Replacement is wholesale and transactional: the old list is deleted and the
// resolved users inserted in one transaction, so a concurrent read sees either
// the whole previous list or the whole new one, never a half-cleared list. An
// empty userIDs clears the watch list -- that is a legitimate request, not a
// no-op, which is why the case's existence is checked explicitly rather than
// inferred from rows having been written.
func (r *caseRepo) ReplaceCaseWatchers(ctx context.Context, caseID string, userIDs []string) ([]domain.WatchListUser, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("replace case watchers: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM cases WHERE id = $1)`, caseID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check case exists: %w", err)
	}
	if !exists {
		return nil, &apierror.NotFoundError{Msg: "case not found"}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM case_watchers WHERE case_id = $1`, caseID); err != nil {
		return nil, fmt.Errorf("clear case watchers: %w", err)
	}

	if len(userIDs) > 0 {
		// Reject the whole request if any id is unresolvable, before writing
		// anything: a watch list silently missing a member is worse than a
		// rejected update, because nobody notices the person who stopped being
		// notified. An email-less user is unresolvable for the same reason the
		// ServiceNow path rejects it -- email is what the notification path
		// consumes, so such an entry could never be delivered to.
		unresolved, err := unresolvedWatcherIDs(ctx, tx, userIDs)
		if err != nil {
			return nil, err
		}
		if len(unresolved) > 0 {
			return nil, &apierror.ValidationError{
				Msg: "watchList contains users that do not exist or have no email address: " + strings.Join(unresolved, ", "),
			}
		}

		// LOWER(u.email) matches the normalisation the (case_id, email) primary
		// key assumes. ON CONFLICT makes the same user listed twice in one
		// request a no-op rather than an error -- a duplicate expresses the same
		// intent as a single mention.
		if _, err := tx.Exec(ctx, `
			INSERT INTO case_watchers (case_id, user_id, email)
			SELECT $1, u.id, LOWER(u.email)
			FROM users u
			WHERE u.id = ANY($2::text[])
			ON CONFLICT (case_id, email) DO NOTHING`, caseID, userIDs); err != nil {
			if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return nil, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
			}
			return nil, fmt.Errorf("insert case watchers: %w", err)
		}
	}

	watchers, err := listCaseWatchers(ctx, tx, caseID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("replace case watchers: commit: %w", err)
	}
	return watchers, nil
}

// unresolvedWatcherIDs returns the subset of ids that name no user, or a user
// with no email address. It preserves the caller's order so the error message
// reads in the order the ids were sent.
func unresolvedWatcherIDs(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, ids []string) ([]string, error) {
	rows, err := q.Query(ctx, `
		SELECT candidate.id
		FROM unnest($1::text[]) WITH ORDINALITY AS candidate(id, ord)
		LEFT JOIN users u ON u.id = candidate.id
		WHERE u.id IS NULL OR COALESCE(TRIM(u.email), '') = ''
		ORDER BY candidate.ord`, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve watch list users: %w", err)
	}
	defer rows.Close()

	var unresolved []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan unresolved watch list user: %w", err)
		}
		unresolved = append(unresolved, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unresolved watch list users: %w", err)
	}
	return unresolved, nil
}
