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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// StandardChangeSuggestionRepository reads what the standard change
// suggestions are computed from (completed change requests, their incidents,
// open proposals), stores the computed snapshot, and keeps the
// dismissals. The clustering is the service's (standard_change_suggestion.go);
// nothing here decides.
type StandardChangeSuggestionRepository interface {
	// CABDecisionReader: the CAB's rejections and comments, the pushback a
	// suggestion shows (standard_change_cab_history.go).
	CABDecisionReader
	// NextCustomers lists, in id order after the cursor (exclusive; "" is the
	// start), at most limit of the customers a run listed when it started.
	NextCustomers(ctx context.Context, runID, after string, limit int) ([]string, error)
	// Candidates are one customer's change requests completed (Closed or
	// Rollback) since the instant given and not raised from a template.
	Candidates(ctx context.Context, accountID string, since time.Time) ([]SuggestionCandidate, error)
	// IncidentsOf maps a change request id to the incidents that name it as
	// their cause (caused_by_id) or their change request (change_request_id).
	IncidentsOf(ctx context.Context, changeRequestIDs []string) (map[string][]domain.EntityRef, error)
	// OpenProposalsOf maps a change request id to the open proposals (draft or
	// under review) that started from it or cite it as evidence.
	OpenProposalsOf(ctx context.Context, changeRequestIDs []string) (map[string][]domain.EntityRef, error)
	// Dismissals of the customers given (all customers when nil), by account
	// id then key.
	Dismissals(ctx context.Context, accountIDs []string) (map[string]map[string]domain.StandardChangeSuggestionDismissal, error)
	// Dismiss records a dismissal (409 when the key is already dismissed for
	// that customer, 400 when the account does not exist).
	Dismiss(ctx context.Context, accountID, key, reason, actor string) (domain.StandardChangeSuggestionDismissal, error)
	// Restore deletes a dismissal (404 when there is none with that id).
	Restore(ctx context.Context, id string) error

	// StartRun opens a refresh run with the criteria it computes by, and lists
	// in it every customer with at least one candidate (see Candidates)
	// completed since windowStart(the run's start), once, so no page scans for
	// them again. It deletes runs left unpublished for over a day (a job that died).
	StartRun(ctx context.Context, criteria domain.StandardChangeSuggestionCriteria, windowStart func(time.Time) time.Time) (SuggestionRun, error)
	// GetRun reads a run; nil when there is none with that id.
	GetRun(ctx context.Context, id string) (*SuggestionRun, error)
	// SaveCustomer replaces one customer's rows and change-request count in an
	// unpublished run, in one transaction (409 when the run is gone or already
	// published), so a call sent again never counts a customer twice.
	SaveCustomer(ctx context.Context, runID, accountID string, changeRequests int, rows []SuggestionSnapshotRow) error
	// SkipCustomer records that a customer could not be computed in this run
	// (reason, e.g. too large for its deadline): no suggestions, and it is
	// reported as skipped. The customer's cluster keys are carried over from the
	// latest published run (carried_keys, migration 0289), so the next run keeps
	// them (SnapshotKeys). 409 when the run is gone or published.
	SkipCustomer(ctx context.Context, runID, accountID, reason string) error
	// SkippedCustomers are the customers a run skipped (of one customer when
	// accountID is set), with their reasons.
	SkippedCustomers(ctx context.Context, runID, accountID string) ([]domain.StandardChangeSuggestionSkipped, error)
	// PublishRun sums the run's totals, makes it the one readers see and deletes
	// every other run with its rows, in one transaction (409 when it is gone or
	// already published).
	PublishRun(ctx context.Context, id string) (SuggestionRun, error)
	// LatestRun is the newest published run; nil when none has been published.
	LatestRun(ctx context.Context) (*SuggestionRun, error)
	// CustomerCounts are one customer's counts in a run (all zero when the run
	// did not compute it: it had no candidates).
	CustomerCounts(ctx context.Context, runID, accountID string) (domain.StandardChangeSuggestionScan, error)
	// SnapshotRows reads a run's rows: of one customer when accountID is set, and
	// only the eligible ones unless includeIneligible.
	SnapshotRows(ctx context.Context, runID, accountID string, includeIneligible bool) ([]SuggestionSnapshotRow, error)
	// SnapshotKeys is one customer's rows of a run with only their keys and
	// member ids (no suggestion decoded), and the keys a skip carried into the
	// run: what the next refresh carries the clusters' keys over from.
	SnapshotKeys(ctx context.Context, runID, accountID string) ([]SuggestionSnapshotRow, error)
}

// SuggestionRun is one refresh run (standard_change_suggestion_run).
type SuggestionRun struct {
	ID          string
	StartedOn   time.Time
	CompletedOn *time.Time
	Criteria    domain.StandardChangeSuggestionCriteria
	// Counts: Customers, ChangeRequests, Clusters, Eligible, Skipped, summed at publish.
	Counts domain.StandardChangeSuggestionScan
}

// SuggestionSnapshotRow is one stored suggestion.
type SuggestionSnapshotRow struct {
	AccountID  string
	Key        string
	Eligible   bool
	MemberIDs  []string
	Suggestion domain.StandardChangeSuggestion
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

type standardChangeSuggestionRepo struct {
	db *Scoped
}

// NewStandardChangeSuggestionRepository constructs a StandardChangeSuggestionRepository.
func NewStandardChangeSuggestionRepository(db *Scoped) StandardChangeSuggestionRepository {
	return &standardChangeSuggestionRepo{db: db}
}

// candidateFilter is what makes a change request a candidate: completed (Closed
// or Rollback) since $1, not raised from a template, with a customer. The
// completion is closed_on (0283 stamps it on every completion and backfilled the
// CSM's own); a row completed with none recorded falls back to the work item's
// last update, which is never earlier than its completion.
const candidateFilter = `
		  FROM change_request cr
		  JOIN work_item wi ON wi.id = cr.id AND wi.type = 'CHANGE_REQUEST'
		  JOIN project p ON p.id = wi.project_id
		  JOIN account a ON a.id = p.account_id
		 WHERE cr.state IN ('CLOSED', 'ROLLBACK')
		   AND cr.standard_change_template_version_id IS NULL
		   AND COALESCE(cr.closed_on, wi.updated_on, wi.created_on) >= $1`

// NextCustomers implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) NextCustomers(ctx context.Context, runID, after string, limit int) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT account_id::text FROM standard_change_suggestion_run_customer
		 WHERE run_id = $1::uuid AND ($2 = '' OR account_id > $2::uuid)
		 ORDER BY account_id
		 LIMIT $3`, runID, after, limit)
	if err != nil {
		return nil, fmt.Errorf("standard change suggestions: customers: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("standard change suggestions: customers: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("standard change suggestions: customers: %w", err)
	}
	return out, nil
}

// Candidates implements StandardChangeSuggestionRepository.
//
// crvis: internal callers only: POST /standard-change-suggestions/refresh is wrapped by serviceOnly, and the service requires an internal caller
func (r *standardChangeSuggestionRepo) Candidates(ctx context.Context, accountID string, since time.Time) ([]SuggestionCandidate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cr.id::text, wi.number, COALESCE(wi.subject, ''), cr.state::text,
		       COALESCE(cr.closed_on, wi.updated_on, wi.created_on),
		       COALESCE(cr.implementation_plan, ''), cr.risk::text, cr.impact::text,
		       cr.cloned_from_change_request_id::text,
		       cr.implementation_deviation IS NOT NULL AND btrim(cr.implementation_deviation) <> '',
		       a.id::text, COALESCE(a.name, '')`+candidateFilter+`
		   AND a.id = $2::uuid`, since, accountID)
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
		           AND p.state IN ('DRAFT', 'AUTHORITY_REVIEW')
		        UNION
		        SELECT e.change_request_id, p.id FROM standard_change_proposal_evidence e
		          JOIN standard_change_proposal p ON p.id = e.proposal_id
		         WHERE e.change_request_id = ANY($1::uuid[])
		           AND p.state IN ('DRAFT', 'AUTHORITY_REVIEW')) l
		  JOIN work_item wi ON wi.id = l.id
		 ORDER BY 1, 3`, ids)
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
	if accountIDs != nil && len(accountIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `SELECT `+suggestionDismissalColumns+`
		  FROM standard_change_suggestion_dismissal d
		  LEFT JOIN "user" du ON du.id = d.dismissed_by_user_id
		 WHERE $1::uuid[] IS NULL OR d.account_id = ANY($1::uuid[])`, accountIDs)
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

// runStaleAfter: an unpublished run older than this is a job that died.
const runStaleAfter = 24 * time.Hour

var errRunGone = &apierror.ConflictError{Msg: "this refresh run is no longer open (published, or replaced by another): start a new one"}

const suggestionRunColumns = `id::text, started_on, completed_on, criteria, customers, change_requests, clusters, eligible, skipped`

func scanSuggestionRun(row pgx.Row) (SuggestionRun, error) {
	var run SuggestionRun
	var criteria []byte
	err := row.Scan(&run.ID, &run.StartedOn, &run.CompletedOn, &criteria,
		&run.Counts.Customers, &run.Counts.ChangeRequests, &run.Counts.Clusters, &run.Counts.Eligible, &run.Counts.Skipped)
	if err != nil {
		return run, err
	}
	if err := json.Unmarshal(criteria, &run.Criteria); err != nil {
		return run, fmt.Errorf("decode criteria: %w", err)
	}
	return run, nil
}

// StartRun implements StandardChangeSuggestionRepository.
//
// crvis: internal callers only: POST /standard-change-suggestions/refresh is wrapped by serviceOnly, and the service requires an internal caller
func (r *standardChangeSuggestionRepo) StartRun(ctx context.Context, criteria domain.StandardChangeSuggestionCriteria, windowStart func(time.Time) time.Time) (SuggestionRun, error) {
	raw, err := json.Marshal(criteria)
	if err != nil {
		return SuggestionRun{}, fmt.Errorf("start suggestion run: %w", err)
	}
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (SuggestionRun, error) {
		if _, err := tx.Exec(ctx, `DELETE FROM standard_change_suggestion_run
			WHERE completed_on IS NULL AND started_on < NOW() - $1::interval`, runStaleAfter.String()); err != nil {
			return SuggestionRun{}, fmt.Errorf("start suggestion run: clear stale runs: %w", err)
		}
		run, err := scanSuggestionRun(tx.QueryRow(ctx, `INSERT INTO standard_change_suggestion_run (criteria)
			VALUES ($1::jsonb) RETURNING `+suggestionRunColumns, raw))
		if err != nil {
			return SuggestionRun{}, fmt.Errorf("start suggestion run: %w", err)
		}
		// The customers to compute, read once for the whole run.
		if _, err := tx.Exec(ctx, `
			INSERT INTO standard_change_suggestion_run_customer (run_id, account_id)
			SELECT DISTINCT $2::uuid, a.id`+candidateFilter, windowStart(run.StartedOn), run.ID); err != nil {
			return SuggestionRun{}, fmt.Errorf("start suggestion run: list the customers: %w", err)
		}
		return run, nil
	})
}

// GetRun implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) GetRun(ctx context.Context, id string) (*SuggestionRun, error) {
	run, err := scanSuggestionRun(r.db.QueryRow(ctx, `SELECT `+suggestionRunColumns+`
		FROM standard_change_suggestion_run WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read suggestion run: %w", err)
	}
	return &run, nil
}

// LatestRun implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) LatestRun(ctx context.Context) (*SuggestionRun, error) {
	run, err := scanSuggestionRun(r.db.QueryRow(ctx, `SELECT `+suggestionRunColumns+`
		FROM standard_change_suggestion_run WHERE completed_on IS NOT NULL
		ORDER BY completed_on DESC LIMIT 1`))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the published suggestion run: %w", err)
	}
	return &run, nil
}

// lockOpenRun locks an unpublished run's row for the rest of tx (errRunGone when
// it is gone or published), so a publish and a save of the same run serialise.
func lockOpenRun(ctx context.Context, tx pgx.Tx, id string) error {
	var open bool
	err := tx.QueryRow(ctx, `SELECT completed_on IS NULL FROM standard_change_suggestion_run WHERE id = $1::uuid FOR UPDATE`, id).Scan(&open)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !open) {
		return errRunGone
	}
	return err
}

// SaveCustomer implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) SaveCustomer(ctx context.Context, runID, accountID string, changeRequests int, rows []SuggestionSnapshotRow) error {
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		if err := lockOpenRun(ctx, tx, runID); err != nil {
			return err
		}
		// A customer already saved in this run (a call sent again) is replaced.
		if _, err := tx.Exec(ctx, `DELETE FROM standard_change_suggestion_snapshot WHERE run_id = $1::uuid AND account_id = $2::uuid`, runID, accountID); err != nil {
			return fmt.Errorf("save suggestions: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO standard_change_suggestion_run_customer (run_id, account_id, change_requests)
			VALUES ($1::uuid, $2::uuid, $3)
			ON CONFLICT (run_id, account_id) DO UPDATE SET change_requests = EXCLUDED.change_requests, skipped_reason = NULL, carried_keys = NULL`,
			runID, accountID, changeRequests); err != nil {
			return fmt.Errorf("save suggestions: %w", err)
		}
		for _, row := range rows {
			raw, err := json.Marshal(row.Suggestion)
			if err != nil {
				return fmt.Errorf("save suggestions: encode: %w", err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO standard_change_suggestion_snapshot (run_id, account_id, suggestion_key, eligible, member_ids, suggestion)
				VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid[], $6::jsonb)`,
				runID, accountID, row.Key, row.Eligible, row.MemberIDs, raw); err != nil {
				return fmt.Errorf("save suggestions: %w", err)
			}
		}
		return nil
	})
}

// SkipCustomer implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) SkipCustomer(ctx context.Context, runID, accountID, reason string) error {
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		if err := lockOpenRun(ctx, tx, runID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM standard_change_suggestion_snapshot WHERE run_id = $1::uuid AND account_id = $2::uuid`, runID, accountID); err != nil {
			return fmt.Errorf("skip customer: %w", err)
		}
		// The keys of the latest published run: its rows, or what it carried
		// itself when it skipped this customer too.
		if _, err := tx.Exec(ctx, `
			WITH latest AS (
				SELECT id FROM standard_change_suggestion_run
				 WHERE completed_on IS NOT NULL ORDER BY completed_on DESC LIMIT 1
			), keys AS (
				SELECT COALESCE(
				  (SELECT jsonb_agg(jsonb_build_object('key', s.suggestion_key, 'memberIds', to_jsonb(s.member_ids)) ORDER BY s.id)
				     FROM standard_change_suggestion_snapshot s
				    WHERE s.run_id = (SELECT id FROM latest) AND s.account_id = $2::uuid),
				  (SELECT c.carried_keys FROM standard_change_suggestion_run_customer c
				    WHERE c.run_id = (SELECT id FROM latest) AND c.account_id = $2::uuid)) AS carried
			)
			INSERT INTO standard_change_suggestion_run_customer (run_id, account_id, change_requests, skipped_reason, carried_keys)
			SELECT $1::uuid, $2::uuid, NULL, $3, carried FROM keys
			ON CONFLICT (run_id, account_id) DO UPDATE
			   SET change_requests = NULL, skipped_reason = EXCLUDED.skipped_reason, carried_keys = EXCLUDED.carried_keys`,
			runID, accountID, reason); err != nil {
			return fmt.Errorf("skip customer: %w", err)
		}
		return nil
	})
}

// SkippedCustomers implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) SkippedCustomers(ctx context.Context, runID, accountID string) ([]domain.StandardChangeSuggestionSkipped, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.account_id::text, COALESCE(a.name, ''), c.skipped_reason
		  FROM standard_change_suggestion_run_customer c
		  LEFT JOIN account a ON a.id = c.account_id
		 WHERE c.run_id = $1::uuid AND c.skipped_reason IS NOT NULL
		   AND ($2 = '' OR c.account_id = $2::uuid)
		 ORDER BY 2, 1`, runID, accountID)
	if err != nil {
		return nil, fmt.Errorf("read skipped customers: %w", err)
	}
	defer rows.Close()
	out := []domain.StandardChangeSuggestionSkipped{}
	for rows.Next() {
		var sk domain.StandardChangeSuggestionSkipped
		if err := rows.Scan(&sk.Customer.ID, &sk.Customer.Name, &sk.Reason); err != nil {
			return nil, fmt.Errorf("read skipped customers: %w", err)
		}
		out = append(out, sk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read skipped customers: %w", err)
	}
	return out, nil
}

// PublishRun implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) PublishRun(ctx context.Context, id string) (SuggestionRun, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (SuggestionRun, error) {
		if err := lockOpenRun(ctx, tx, id); err != nil {
			return SuggestionRun{}, err
		}
		run, err := scanSuggestionRun(tx.QueryRow(ctx, `UPDATE standard_change_suggestion_run r SET completed_on = NOW(),
			    customers = (SELECT COUNT(*) FROM standard_change_suggestion_run_customer c WHERE c.run_id = r.id AND c.change_requests IS NOT NULL),
			    skipped = (SELECT COUNT(*) FROM standard_change_suggestion_run_customer c WHERE c.run_id = r.id AND c.skipped_reason IS NOT NULL),
			    change_requests = (SELECT COALESCE(SUM(change_requests), 0) FROM standard_change_suggestion_run_customer c WHERE c.run_id = r.id),
			    clusters = (SELECT COUNT(*) FROM standard_change_suggestion_snapshot s WHERE s.run_id = r.id),
			    eligible = (SELECT COUNT(*) FROM standard_change_suggestion_snapshot s WHERE s.run_id = r.id AND s.eligible)
			WHERE r.id = $1::uuid RETURNING `+suggestionRunColumns, id))
		if err != nil {
			return SuggestionRun{}, fmt.Errorf("publish suggestion run: %w", err)
		}
		// Every other run goes, published or not, with its rows (ON DELETE CASCADE):
		// readers switch at this commit.
		if _, err := tx.Exec(ctx, `DELETE FROM standard_change_suggestion_run WHERE id <> $1::uuid`, id); err != nil {
			return SuggestionRun{}, fmt.Errorf("publish suggestion run: drop older runs: %w", err)
		}
		return run, nil
	})
}

// SnapshotRows implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) SnapshotRows(ctx context.Context, runID, accountID string, includeIneligible bool) ([]SuggestionSnapshotRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT account_id::text, suggestion_key, eligible, member_ids::text[], suggestion
		  FROM standard_change_suggestion_snapshot
		 WHERE run_id = $1::uuid
		   AND ($2 = '' OR account_id = $2::uuid)
		   AND ($3 OR eligible)
		 ORDER BY id`, runID, accountID, includeIneligible)
	if err != nil {
		return nil, fmt.Errorf("read suggestions: %w", err)
	}
	defer rows.Close()
	var out []SuggestionSnapshotRow
	for rows.Next() {
		var row SuggestionSnapshotRow
		var raw []byte
		if err := rows.Scan(&row.AccountID, &row.Key, &row.Eligible, &row.MemberIDs, &raw); err != nil {
			return nil, fmt.Errorf("read suggestions: %w", err)
		}
		if err := json.Unmarshal(raw, &row.Suggestion); err != nil {
			return nil, fmt.Errorf("read suggestions: decode: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read suggestions: %w", err)
	}
	return out, nil
}

// SnapshotKeys implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) SnapshotKeys(ctx context.Context, runID, accountID string) ([]SuggestionSnapshotRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT key, member_ids FROM (
			SELECT 0 AS src, s.id AS ord, s.suggestion_key AS key, s.member_ids::text[] AS member_ids
			  FROM standard_change_suggestion_snapshot s
			 WHERE s.run_id = $1::uuid AND s.account_id = $2::uuid
			UNION ALL
			SELECT 1, e.ord, e.value ->> 'key',
			       ARRAY(SELECT jsonb_array_elements_text(e.value -> 'memberIds'))
			  FROM standard_change_suggestion_run_customer c,
			       jsonb_array_elements(COALESCE(c.carried_keys, '[]'::jsonb)) WITH ORDINALITY AS e(value, ord)
			 WHERE c.run_id = $1::uuid AND c.account_id = $2::uuid
		) k ORDER BY src, ord`, runID, accountID)
	if err != nil {
		return nil, fmt.Errorf("read suggestion keys: %w", err)
	}
	defer rows.Close()
	var out []SuggestionSnapshotRow
	for rows.Next() {
		row := SuggestionSnapshotRow{AccountID: accountID}
		if err := rows.Scan(&row.Key, &row.MemberIDs); err != nil {
			return nil, fmt.Errorf("read suggestion keys: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read suggestion keys: %w", err)
	}
	return out, nil
}

// CustomerCounts implements StandardChangeSuggestionRepository.
func (r *standardChangeSuggestionRepo) CustomerCounts(ctx context.Context, runID, accountID string) (domain.StandardChangeSuggestionScan, error) {
	var out domain.StandardChangeSuggestionScan
	err := r.db.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM standard_change_suggestion_run_customer WHERE run_id = $1::uuid AND account_id = $2::uuid AND change_requests IS NOT NULL),
		       (SELECT COALESCE(SUM(change_requests), 0) FROM standard_change_suggestion_run_customer WHERE run_id = $1::uuid AND account_id = $2::uuid),
		       (SELECT COUNT(*) FROM standard_change_suggestion_snapshot WHERE run_id = $1::uuid AND account_id = $2::uuid),
		       (SELECT COUNT(*) FROM standard_change_suggestion_snapshot WHERE run_id = $1::uuid AND account_id = $2::uuid AND eligible),
		       (SELECT COUNT(*) FROM standard_change_suggestion_run_customer WHERE run_id = $1::uuid AND account_id = $2::uuid AND skipped_reason IS NOT NULL)`,
		runID, accountID).Scan(&out.Customers, &out.ChangeRequests, &out.Clusters, &out.Eligible, &out.Skipped)
	if err != nil {
		return out, fmt.Errorf("read suggestion counts: %w", err)
	}
	return out, nil
}
