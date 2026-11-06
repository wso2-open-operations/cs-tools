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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// StandardChangeSuggestionRepository reads what the standard change
// suggestions are computed from (completed change requests, the customers'
// live templates, open proposals) and keeps the dismissals. The clustering is
// the service's (standard_change_suggestion.go); nothing here decides.
type StandardChangeSuggestionRepository interface {
	// Candidates are the change requests completed (Closed or Rollback) since
	// the instant given, not raised from a template, whose project has a
	// customer: of that one customer when accountID is set.
	Candidates(ctx context.Context, accountID string, since time.Time) ([]SuggestionCandidate, error)
	// IncidentsOf maps a change request id to the incidents that name it as
	// their cause (caused_by_id) or their change request (change_request_id).
	IncidentsOf(ctx context.Context, changeRequestIDs []string) (map[string][]domain.EntityRef, error)
	// LiveTemplates are the active and suspended templates of the customers
	// given, with their current version's subject and implementation plan.
	LiveTemplates(ctx context.Context, accountIDs []string) ([]SuggestionTemplate, error)
	// OpenProposalsOf maps a change request id to the open proposals (draft or
	// under review) that started from it or cite it as evidence.
	OpenProposalsOf(ctx context.Context, changeRequestIDs []string) (map[string][]domain.EntityRef, error)
	// Dismissals of the customers given, by account id then key.
	Dismissals(ctx context.Context, accountIDs []string) (map[string]map[string]domain.StandardChangeSuggestionDismissal, error)
	// Dismiss records a dismissal (409 when the key is already dismissed for
	// that customer, 400 when the account does not exist).
	Dismiss(ctx context.Context, accountID, key, reason, actor string) (domain.StandardChangeSuggestionDismissal, error)
	// Restore deletes a dismissal (404 when there is none with that id).
	Restore(ctx context.Context, id string) error
}

// SuggestionCandidate is one completed change request as the clustering reads it.
type SuggestionCandidate struct {
	ID                 string
	Number             string
	Subject            string
	State              string // CLOSED or ROLLBACK
	CompletedOn        time.Time
	ImplementationPlan string
	Risk, Impact       *string
	ClonedFrom         *string
	HasDeviation       bool
	AccountID          string
	AccountName        string
}

// SuggestionTemplate is a live template as the labelling reads it.
type SuggestionTemplate struct {
	ID, Number, Name, State, AccountID string
	Subject, ImplementationPlan        string
}

type standardChangeSuggestionRepo struct {
	db *Scoped
}

// NewStandardChangeSuggestionRepository constructs a StandardChangeSuggestionRepository.
func NewStandardChangeSuggestionRepository(db *Scoped) StandardChangeSuggestionRepository {
	return &standardChangeSuggestionRepo{db: db}
}

// Candidates implements StandardChangeSuggestionRepository. The completion is
// closed_on (0279 stamps it on every completion and backfilled the CSM's own);
// a row completed with none recorded falls back to the work item's last update,
// which is never earlier than its completion.
//
// crvis: internal callers only: GET /standard-change-suggestions is wrapped by internalOnly, and the service requires an internal caller
func (r *standardChangeSuggestionRepo) Candidates(ctx context.Context, accountID string, since time.Time) ([]SuggestionCandidate, error) {
	var account *string
	if accountID != "" {
		account = &accountID
	}
	rows, err := r.db.Query(ctx, `
		SELECT cr.id::text, wi.number, COALESCE(wi.subject, ''), cr.state::text,
		       COALESCE(cr.closed_on, wi.updated_on, wi.created_on),
		       COALESCE(cr.implementation_plan, ''), cr.risk::text, cr.impact::text,
		       cr.cloned_from_change_request_id::text,
		       cr.implementation_deviation IS NOT NULL AND btrim(cr.implementation_deviation) <> '',
		       a.id::text, COALESCE(a.name, '')
		  FROM change_request cr
		  JOIN work_item wi ON wi.id = cr.id AND wi.type = 'CHANGE_REQUEST'
		  JOIN project p ON p.id = wi.project_id
		  JOIN account a ON a.id = p.account_id
		 WHERE cr.state IN ('CLOSED', 'ROLLBACK')
		   AND cr.standard_change_template_version_id IS NULL
		   AND COALESCE(cr.closed_on, wi.updated_on, wi.created_on) >= $1
		   AND ($2::uuid IS NULL OR a.id = $2::uuid)`, since, account)
	if err != nil {
		return nil, fmt.Errorf("standard change suggestions: candidates: %w", err)
	}
	defer rows.Close()
	var out []SuggestionCandidate
	for rows.Next() {
		var c SuggestionCandidate
		if err := rows.Scan(&c.ID, &c.Number, &c.Subject, &c.State, &c.CompletedOn, &c.ImplementationPlan,
			&c.Risk, &c.Impact, &c.ClonedFrom, &c.HasDeviation, &c.AccountID, &c.AccountName); err != nil {
			return nil, fmt.Errorf("standard change suggestions: candidates: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("standard change suggestions: candidates: %w", err)
	}
	return out, nil
}

// linkedRefs runs a query of (change request id, work item id, number) rows
// into a map of change request id to the work items, each once.
func (r *standardChangeSuggestionRepo) linkedRefs(ctx context.Context, what, query string, ids []string) (map[string][]domain.EntityRef, error) {
	out := map[string][]domain.EntityRef{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("standard change suggestions: %s: %w", what, err)
	}
	defer rows.Close()
	for rows.Next() {
		var crID string
		var ref domain.EntityRef
		if err := rows.Scan(&crID, &ref.ID, &ref.Name); err != nil {
			return nil, fmt.Errorf("standard change suggestions: %s: %w", what, err)
		}
		out[crID] = append(out[crID], ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("standard change suggestions: %s: %w", what, err)
	}
	return out, nil
}

// IncidentsOf implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) IncidentsOf(ctx context.Context, ids []string) (map[string][]domain.EntityRef, error) {
	return r.linkedRefs(ctx, "incidents", `
		SELECT DISTINCT l.cr_id::text, wi.id::text, wi.number
		  FROM (SELECT caused_by_id AS cr_id, id FROM incident WHERE caused_by_id = ANY($1::uuid[])
		        UNION
		        SELECT change_request_id, id FROM incident WHERE change_request_id = ANY($1::uuid[])) l
		  JOIN work_item wi ON wi.id = l.id
		 ORDER BY 1, 3`, ids)
}

// OpenProposalsOf implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) OpenProposalsOf(ctx context.Context, ids []string) (map[string][]domain.EntityRef, error) {
	return r.linkedRefs(ctx, "open proposals", `
		SELECT DISTINCT l.cr_id::text, wi.id::text, wi.number
		  FROM (SELECT p.source_change_request_id AS cr_id, p.id FROM standard_change_proposal p
		         WHERE p.source_change_request_id = ANY($1::uuid[])
		           AND p.state IN ('DRAFT', 'CAB_REVIEW', 'AUTHORITY_REVIEW')
		        UNION
		        SELECT e.change_request_id, p.id FROM standard_change_proposal_evidence e
		          JOIN standard_change_proposal p ON p.id = e.proposal_id
		         WHERE e.change_request_id = ANY($1::uuid[])
		           AND p.state IN ('DRAFT', 'CAB_REVIEW', 'AUTHORITY_REVIEW')) l
		  JOIN work_item wi ON wi.id = l.id
		 ORDER BY 1, 3`, ids)
}

// LiveTemplates implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) LiveTemplates(ctx context.Context, accountIDs []string) ([]SuggestionTemplate, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT t.id::text, t.number, t.name, t.state::text, t.account_id::text,
		       COALESCE(v.subject, ''), COALESCE(v.implementation_plan, '')
		  FROM standard_change_template t
		  JOIN standard_change_template_version v ON v.id = t.current_version_id
		 WHERE t.state IN ('ACTIVE', 'SUSPENDED') AND t.account_id = ANY($1::uuid[])
		 ORDER BY t.number`, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("standard change suggestions: templates: %w", err)
	}
	defer rows.Close()
	var out []SuggestionTemplate
	for rows.Next() {
		var t SuggestionTemplate
		if err := rows.Scan(&t.ID, &t.Number, &t.Name, &t.State, &t.AccountID, &t.Subject, &t.ImplementationPlan); err != nil {
			return nil, fmt.Errorf("standard change suggestions: templates: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("standard change suggestions: templates: %w", err)
	}
	return out, nil
}

const suggestionDismissalColumns = `d.id::text, d.account_id::text, d.suggestion_key, d.reason, d.dismissed_on,
	du.id::text, COALESCE(du.name, NULLIF(TRIM(CONCAT_WS(' ', du.first_name, du.last_name)), ''), d.dismissed_by)`

func scanSuggestionDismissal(row pgx.Row) (accountID, key string, d domain.StandardChangeSuggestionDismissal, err error) {
	var byID *string
	var byName string
	if err = row.Scan(&d.ID, &accountID, &key, &d.Reason, &d.DismissedOn, &byID, &byName); err != nil {
		return
	}
	if byID != nil {
		d.DismissedBy = &domain.EntityRef{ID: *byID, Name: byName}
	} else {
		d.DismissedBy = &domain.EntityRef{Name: byName}
	}
	return
}

// Dismissals implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) Dismissals(ctx context.Context, accountIDs []string) (map[string]map[string]domain.StandardChangeSuggestionDismissal, error) {
	out := map[string]map[string]domain.StandardChangeSuggestionDismissal{}
	if len(accountIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `SELECT `+suggestionDismissalColumns+`
		  FROM standard_change_suggestion_dismissal d
		  LEFT JOIN "user" du ON du.id = d.dismissed_by_user_id
		 WHERE d.account_id = ANY($1::uuid[])`, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("standard change suggestions: dismissals: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		account, key, d, err := scanSuggestionDismissal(rows)
		if err != nil {
			return nil, fmt.Errorf("standard change suggestions: dismissals: %w", err)
		}
		if out[account] == nil {
			out[account] = map[string]domain.StandardChangeSuggestionDismissal{}
		}
		out[account][key] = d
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("standard change suggestions: dismissals: %w", err)
	}
	return out, nil
}

// Dismiss implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) Dismiss(ctx context.Context, accountID, key, reason, actor string) (domain.StandardChangeSuggestionDismissal, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (domain.StandardChangeSuggestionDismissal, error) {
		var id string
		err := tx.QueryRow(ctx, `
			INSERT INTO standard_change_suggestion_dismissal (account_id, suggestion_key, reason, dismissed_by_user_id, dismissed_by)
			VALUES ($1::uuid, $2, $3, `+userIDByEmailSQL4+`, $4)
			ON CONFLICT (account_id, suggestion_key) DO NOTHING
			RETURNING id::text`, accountID, key, reason, actor).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StandardChangeSuggestionDismissal{}, &apierror.ConflictError{Msg: "this suggestion is already dismissed for this customer"}
		}
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.StandardChangeSuggestionDismissal{}, &apierror.ValidationError{Msg: "accountId does not refer to an existing customer"}
		}
		if err != nil {
			return domain.StandardChangeSuggestionDismissal{}, fmt.Errorf("dismiss standard change suggestion: %w", err)
		}
		_, _, d, err := scanSuggestionDismissal(tx.QueryRow(ctx, `SELECT `+suggestionDismissalColumns+`
			  FROM standard_change_suggestion_dismissal d
			  LEFT JOIN "user" du ON du.id = d.dismissed_by_user_id
			 WHERE d.id = $1::uuid`, id))
		if err != nil {
			return domain.StandardChangeSuggestionDismissal{}, fmt.Errorf("dismiss standard change suggestion: %w", err)
		}
		return d, nil
	})
}

// userIDByEmailSQL4 is userIDByEmailSQL with the email as $4.
const userIDByEmailSQL4 = `(SELECT u.id FROM "user" u WHERE LOWER(u.email) = LOWER($4) ORDER BY u.id LIMIT 1)`

// Restore implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) Restore(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM standard_change_suggestion_dismissal WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("restore standard change suggestion: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "dismissal not found"}
	}
	return nil
}
