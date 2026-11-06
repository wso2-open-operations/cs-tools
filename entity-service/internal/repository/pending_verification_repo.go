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
	"golang.org/x/sync/errgroup"
)

// PendingVerificationRepository defines the persistence operations for the
// pending_verification table — see domain.PendingVerification's doc comment
// for what it's for.
type PendingVerificationRepository interface {
	// Create inserts a new entry. WorkItemID must reference an existing
	// work_item row — a foreign-key violation surfaces as a
	// *apierror.ValidationError. A work item that already has an unverified
	// entry (pending_verification_one_unverified_per_work_item, migration
	// 0191) surfaces as a *apierror.ConflictError — there is at most one
	// unverified entry per work item at a time.
	Create(ctx context.Context, req domain.CreatePendingVerificationRequest) (domain.PendingVerification, error)
	// GetWorkItemTypeAndProject returns work_item.type (e.g. "CASE"),
	// work_item.project_id, and -- for a CASE -- "case".state (nil for any
	// other type, via the LEFT JOIN) for workItemID -- used by
	// CreatePendingVerification to enforce the Case-only scope, that the
	// case is actually closed, and the project's VerificationEnabled gate
	// before inserting. Reads through the same Scoped/RLS-protected path as
	// every other work_item query, so a work item that exists but is outside
	// the caller's scope returns *apierror.ValidationError, identical to one
	// that doesn't exist at all. Returns *apierror.ValidationError if the
	// work item has no linked project -- this feature has nothing to check
	// VerificationEnabled against otherwise.
	GetWorkItemTypeAndProject(ctx context.Context, workItemID string) (workItemType string, projectID string, caseState *string, err error)
	// GetProjectIDForEntry returns the project_id of the work item a pending
	// verification entry (by its own id) belongs to -- used by
	// VerifyPendingVerification to enforce the project's VerificationEnabled
	// gate, since that request carries only the entry's id, not a project or
	// work item id directly.
	GetProjectIDForEntry(ctx context.Context, id string) (string, error)
	// GetProjectVerificationEnabled reports whether projectID currently has
	// Pending Verification enabled (project.verification_enabled, coalesced
	// to false when NULL). Create/Search/Verify all check this before acting:
	// RLS enforces project *membership*, not whether the project has this
	// feature turned on at all, so without this a read-only project member
	// could create or verify entries even while the project's own toggle is
	// off.
	GetProjectVerificationEnabled(ctx context.Context, projectID string) (bool, error)
	// Search returns the page of entries matching req.Filters, plus the
	// counts SearchPendingVerificationsResponse needs for the summary tiles
	// and the type-filter dropdown.
	Search(ctx context.Context, req domain.SearchPendingVerificationsRequest) (domain.SearchPendingVerificationsResponse, error)
	// Verify sets verified_on/verified_by for the entry identified by id.
	// Returns a *apierror.NotFoundError if no such entry exists, or if it is
	// already verified (verifying twice is not idempotent here — the
	// service layer surfaces this distinctly from a genuinely missing id).
	Verify(ctx context.Context, req domain.VerifyPendingVerificationRequest) (domain.PendingVerification, error)
}

type pendingVerificationRepo struct {
	db *Scoped
}

// NewPendingVerificationRepository constructs a PendingVerificationRepository
// backed by the given connection pool. work_item/"case" are both RLS-protected
// (see Scoped's own doc comment), so this holds a *Scoped rather than a raw
// *pgxpool.Pool -- every query here joins one or both of those tables.
func NewPendingVerificationRepository(db *Scoped) PendingVerificationRepository {
	return &pendingVerificationRepo{db: db}
}

// pendingVerificationColumns is the column list shared by every query that
// returns a full row, kept in one place so Create/Verify/Search can't drift
// out of sync with scanPendingVerification's field order. severity is
// LEFT-joined from "case" — the only extension table with a severity column
// — and is NULL for every non-Case record type, matching
// domain.PendingVerification.Severity being optional.
const pendingVerificationColumns = `pv.id, pv.work_item_id, wi.number, wi.wso2_id, wi.subject, c.severity::TEXT, wi.type::TEXT,
	pv.added_reason::TEXT, pv.previous_status::TEXT, pv.added_on, pv.added_by,
	pv.verified_on, pv.verified_by, pv.note`

const pendingVerificationFromJoin = `
	FROM pending_verification pv
	JOIN work_item wi ON pv.work_item_id = wi.id
	LEFT JOIN "case" c ON wi.id = c.id`

// pendingVerificationCountFromJoin is pendingVerificationFromJoin without the
// "case" join -- used by the count and type-count queries in Search below,
// neither of which ever selects c.severity (only the data query does). The
// "case" join is a no-op for them: paying for it per row, including every
// pre-dedup row the VerifiedOnly/DISTINCT ON branches scan before collapsing
// to one row per work_item_id, buys nothing.
const pendingVerificationCountFromJoin = `
	FROM pending_verification pv
	JOIN work_item wi ON pv.work_item_id = wi.id`

// pendingVerificationSeverityFromEnum maps "case".severity's raw
// case_severity_enum label ('S0'..'S4') to domain.CaseSeverity's own
// lowercase word (catastrophic..low) via the same caseSeverityFromEnum
// table case_repo.go's GetCaseByID/SearchCases already use for their own
// Severity field -- this table was previously left unmapped, returning the
// raw DB label directly, which disagreed with every other case severity
// response in this API. nil (not the raw value) when unmapped, matching
// GetCaseByID's own "unmapped means absent" posture rather than surfacing
// a non-domain value as if it were a real one.
func pendingVerificationSeverityFromEnum(raw *string) *string {
	if raw == nil {
		return nil
	}
	mapped, ok := caseSeverityFromEnum[*raw]
	if !ok {
		return nil
	}
	s := string(mapped)
	return &s
}

func scanPendingVerification(row pgx.Row) (domain.PendingVerification, error) {
	var pv domain.PendingVerification
	var internalID *string
	var severity *string
	var previousStatus *string
	if err := row.Scan(
		&pv.ID, &pv.WorkItemID, &pv.RecordNumber, &internalID, &pv.RecordTitle, &severity, &pv.RecordType,
		&pv.AddedReason, &previousStatus, &pv.AddedOn, &pv.AddedBy,
		&pv.VerifiedOn, &pv.VerifiedBy, &pv.Note,
	); err != nil {
		return domain.PendingVerification{}, err
	}
	pv.InternalID = internalID
	pv.Severity = pendingVerificationSeverityFromEnum(severity)
	if previousStatus != nil {
		s := domain.PendingVerificationPreviousStatus(*previousStatus)
		pv.PreviousStatus = &s
	}
	return pv, nil
}

// Create implements PendingVerificationRepository.
func (r *pendingVerificationRepo) Create(ctx context.Context, req domain.CreatePendingVerificationRequest) (domain.PendingVerification, error) {
	query := `
		INSERT INTO pending_verification (work_item_id, added_reason, previous_status, added_by, note)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	var id string
	err := r.db.QueryRow(ctx, query, req.WorkItemID, req.AddedReason, req.PreviousStatus, req.AddedBy, req.Note).Scan(&id)
	if err != nil {
		// IsRLSPolicyViolation: the pending_verification_write WITH CHECK
		// policy (migration 100025) rejects a work_item_id outside the
		// caller's own project scope -- mapped the same way as the
		// foreign-key check just below (never reveal existence to a caller
		// outside scope). In practice CreatePendingVerification's own
		// GetWorkItemType pre-check already catches this earlier via
		// work_item's own RLS, so this branch mainly guards a direct API
		// caller or a future call site that skips that pre-check.
		if IsRLSPolicyViolation(err) {
			return domain.PendingVerification{}, &apierror.ValidationError{Msg: "workItemId does not reference an existing record"}
		}
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign_key_violation on work_item_id
				return domain.PendingVerification{}, &apierror.ValidationError{Msg: "workItemId does not reference an existing record"}
			case "23505": // unique_violation on pending_verification_one_unverified_per_work_item
				return domain.PendingVerification{}, &apierror.ConflictError{Msg: "this case already has an unverified pending verification entry"}
			}
		}
		return domain.PendingVerification{}, fmt.Errorf("create pending_verification: %w", err)
	}

	return r.getByID(ctx, id)
}

// GetWorkItemTypeAndProject implements PendingVerificationRepository.
func (r *pendingVerificationRepo) GetWorkItemTypeAndProject(ctx context.Context, workItemID string) (string, string, *string, error) {
	var workItemType string
	var projectID, caseState *string
	// "case" is LEFT-joined, not INNER: a non-CASE work item (rejected right
	// after this call returns) must still resolve a row here rather than
	// being misreported as "does not exist".
	err := r.db.QueryRow(ctx, `SELECT wi.type::TEXT, wi.project_id, c.state::TEXT FROM work_item wi LEFT JOIN "case" c ON wi.id = c.id WHERE wi.id = $1`, workItemID).Scan(&workItemType, &projectID, &caseState)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil, &apierror.ValidationError{Msg: "workItemId does not reference an existing record"}
		}
		return "", "", nil, fmt.Errorf("get work_item type and project: %w", err)
	}
	if projectID == nil {
		return "", "", nil, &apierror.ValidationError{Msg: "workItemId has no linked project; Pending Verification requires one"}
	}
	return workItemType, *projectID, caseState, nil
}

// GetProjectIDForEntry implements PendingVerificationRepository.
func (r *pendingVerificationRepo) GetProjectIDForEntry(ctx context.Context, id string) (string, error) {
	var projectID *string
	query := `SELECT wi.project_id FROM pending_verification pv JOIN work_item wi ON wi.id = pv.work_item_id WHERE pv.id = $1`
	err := r.db.QueryRow(ctx, query, id).Scan(&projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", &apierror.NotFoundError{Msg: "pending_verification not found"}
		}
		return "", fmt.Errorf("get project id for pending_verification entry: %w", err)
	}
	if projectID == nil {
		return "", &apierror.ValidationError{Msg: "this entry's work item has no linked project; Pending Verification requires one"}
	}
	return *projectID, nil
}

// GetProjectVerificationEnabled implements PendingVerificationRepository.
func (r *pendingVerificationRepo) GetProjectVerificationEnabled(ctx context.Context, projectID string) (bool, error) {
	var enabled bool
	err := r.db.QueryRow(ctx, `SELECT COALESCE(verification_enabled, false) FROM project WHERE id = $1`, projectID).Scan(&enabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, &apierror.ValidationError{Msg: "projectId does not reference an existing record"}
		}
		return false, fmt.Errorf("get project verification_enabled: %w", err)
	}
	return enabled, nil
}

func (r *pendingVerificationRepo) getByID(ctx context.Context, id string) (domain.PendingVerification, error) {
	query := `SELECT ` + pendingVerificationColumns + pendingVerificationFromJoin + ` WHERE pv.id = $1`
	pv, err := scanPendingVerification(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PendingVerification{}, &apierror.NotFoundError{Msg: "pending_verification not found"}
		}
		return domain.PendingVerification{}, fmt.Errorf("get pending_verification: %w", err)
	}
	return pv, nil
}

// Verify implements PendingVerificationRepository.
func (r *pendingVerificationRepo) Verify(ctx context.Context, req domain.VerifyPendingVerificationRequest) (domain.PendingVerification, error) {
	query := `
		UPDATE pending_verification
		SET verified_on = NOW(), verified_by = $2
		WHERE id = $1 AND verified_on IS NULL
		RETURNING id`

	var id string
	err := r.db.QueryRow(ctx, query, req.ID, req.VerifiedBy).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PendingVerification{}, &apierror.NotFoundError{Msg: "pending_verification not found, or already verified"}
		}
		return domain.PendingVerification{}, fmt.Errorf("verify pending_verification: %w", err)
	}

	return r.getByID(ctx, id)
}

// buildSearchWhere builds the shared WHERE clause (and its args) for every
// branch of Search below. includeTypeFilter is false when computing the
// type-filter dropdown's own per-type counts, since those must reflect every
// other active filter without being narrowed by the type filter itself.
func buildSearchWhere(f domain.PendingVerificationSearchFilters, includeTypeFilter bool) (string, []any) {
	// Pending Verification is scoped to Case only (product decision,
	// 2026-10-06) -- hardcoded, not driven by f.WorkItemTypes, which stays on
	// the wire only for backward compatibility with callers still sending it
	// (see PendingVerificationSearchFilters' own doc comment).
	where := "WHERE wi.project_id = $1 AND wi.type = 'CASE'::work_item_type_enum"
	args := []any{f.ProjectID}
	argIdx := 2

	// Skipped entirely when VerifiedOnly is set -- see
	// dedupeAddedReasonClause's own doc comment for why: it must be applied
	// AFTER Search's DISTINCT ON dedup, not here, or it can make dedup pick
	// the wrong cycle.
	if f.AddedReason != nil && !f.VerifiedOnly {
		where += fmt.Sprintf(" AND pv.added_reason = $%d::pending_verification_added_reason_enum", argIdx)
		args = append(args, *f.AddedReason)
		argIdx++
	}

	// VerifiedOnly deliberately does NOT filter on pv.verified_on here -- the
	// dedupe branches in Search need every row (verified or not) to find each
	// work item's true latest cycle by added_on, then filter to verified-only
	// AFTER that, in their own outer query. Filtering here would let an older
	// verified round win DISTINCT ON's per-work-item slot over a newer,
	// still-unverified round for the same case (see Search's own doc
	// comment).
	if !f.VerifiedOnly && !f.IncludeVerified {
		where += " AND pv.verified_on IS NULL"
	}
	if f.WorkItemID != nil {
		where += fmt.Sprintf(" AND pv.work_item_id = $%d", argIdx)
		args = append(args, *f.WorkItemID)
		argIdx++
	}
	if includeTypeFilter && len(f.WorkItemTypes) > 0 {
		where += fmt.Sprintf(" AND wi.type = ANY($%d::work_item_type_enum[])", argIdx)
		args = append(args, f.WorkItemTypes)
		argIdx++
	}
	if f.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(f.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (wi.number ILIKE $%d ESCAPE '\\' OR wi.subject ILIKE $%d ESCAPE '\\' OR wi.description ILIKE $%d ESCAPE '\\')", argIdx, argIdx, argIdx)
		args = append(args, pattern)
		argIdx++
	}
	return where, args
}

// dedupeAddedReasonClause returns the SQL fragment (its one placeholder
// numbered right after argOffset existing args) and the argument to append,
// for applying AddedReason to a VerifiedOnly (dedupe) query's OUTER
// selection -- i.e. AFTER DISTINCT ON has already picked each work item's
// true latest cycle. buildSearchWhere deliberately excludes AddedReason from
// the inner WHERE when VerifiedOnly is set, for the same reason it already
// excludes verified_on: filtering by reason BEFORE dedup can remove a case's
// real latest (still-unverified, different-reason) cycle from DISTINCT ON's
// candidate set entirely, letting it pick a stale, already-verified older
// cycle instead -- showing a case in the Verified tab, in the wrong reason
// bucket, whose true current cycle isn't verified at all. Returns ("", nil)
// when no AddedReason filter applies.
func dedupeAddedReasonClause(f domain.PendingVerificationSearchFilters, argOffset int) (string, []any) {
	if f.AddedReason == nil {
		return "", nil
	}
	// added_reason is the raw enum column in the count/type-counts queries'
	// "latest" CTEs, but TEXT in the data query's subquery (pendingVerificationColumns
	// casts it, aliased to the same name) -- comparing as ::TEXT on both sides
	// is valid for either shape, where comparing the bare enum literal against
	// a TEXT column is not (Postgres has no implicit text = enum operator).
	return fmt.Sprintf(" AND added_reason::TEXT = $%d", argOffset+1), []any{string(*f.AddedReason)}
}

// Search implements PendingVerificationRepository.
func (r *pendingVerificationRepo) Search(ctx context.Context, req domain.SearchPendingVerificationsRequest) (domain.SearchPendingVerificationsResponse, error) {
	where, args := buildSearchWhere(req.Filters, true)
	typeCountsWhere, typeCountsArgs := buildSearchWhere(req.Filters, false)
	dedupe := req.Filters.VerifiedOnly

	var entries []domain.PendingVerification
	var total, autoClosedCount, manualCount int
	var typeCounts []domain.PendingVerificationTypeCount

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		var dataQuery string
		var dataArgs []any
		if dedupe {
			// One row per work_item_id — its LATEST cycle (by added_on, not
			// by verified_on) — so a case verified more than once (VerifiedOnly's
			// whole reason to exist: the Verified tab, post
			// re-add-after-verified) shows as a single card, not one per
			// historical round. Ordering DISTINCT ON by added_on (rather than
			// verified_on) is deliberate: it must pick each work item's
			// current cycle regardless of whether that cycle happens to be
			// verified yet, so the outer "WHERE verified_on IS NOT NULL" below
			// can correctly OMIT a case whose latest cycle is still pending —
			// even though an older, already-verified cycle exists for the
			// same case. Without this, a case re-added for a second round
			// would wrongly keep showing in the Verified tab via its stale
			// first-round row. The outer re-sort by verified_on DESC is then
			// just display order, unrelated to which row DISTINCT ON picked.
			// addedReasonClause (see dedupeAddedReasonClause) is applied here,
			// in this same outer WHERE, for the identical reason.
			addedReasonClause, addedReasonArgs := dedupeAddedReasonClause(req.Filters, len(args))
			dataQuery = fmt.Sprintf(`
				SELECT * FROM (
					SELECT DISTINCT ON (pv.work_item_id) %s
					%s %s
					ORDER BY pv.work_item_id, pv.added_on DESC
				) latest
				WHERE verified_on IS NOT NULL%s
				ORDER BY verified_on DESC
				LIMIT $%d OFFSET $%d`,
				pendingVerificationColumns, pendingVerificationFromJoin, where, addedReasonClause,
				len(args)+len(addedReasonArgs)+1, len(args)+len(addedReasonArgs)+2)
			dataArgs = append(append(append([]any{}, args...), addedReasonArgs...), req.Pagination.Limit, req.Pagination.Offset)
		} else {
			dataQuery = fmt.Sprintf(`
				SELECT %s %s %s
				ORDER BY pv.added_on DESC
				LIMIT $%d OFFSET $%d`,
				pendingVerificationColumns, pendingVerificationFromJoin, where, len(args)+1, len(args)+2)
			dataArgs = append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)
		}

		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query pending_verifications: %w", err)
		}
		defer rows.Close()

		result := make([]domain.PendingVerification, 0, req.Pagination.Limit)
		for rows.Next() {
			pv, err := scanPendingVerification(rows)
			if err != nil {
				return fmt.Errorf("scan pending_verification: %w", err)
			}
			result = append(result, pv)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate pending_verifications: %w", err)
		}
		entries = result
		return nil
	})

	eg.Go(func() error {
		var countQuery string
		var countArgs []any
		if dedupe {
			// Same one-row-per-work-item LATEST-cycle set as the data query
			// above (see its own doc comment for why DISTINCT ON orders by
			// added_on, not verified_on), filtered to verified cycles only,
			// so total/autoClosedCount/manualCount agree with what the rows
			// themselves show — each work item's count attributed to its
			// own latest cycle's own addedReason, and a case whose latest
			// cycle isn't verified yet is excluded entirely. addedReasonClause
			// (see dedupeAddedReasonClause) is applied in this same outer
			// WHERE, after dedup, for the identical reason the data query above
			// applies it there.
			addedReasonClause, addedReasonArgs := dedupeAddedReasonClause(req.Filters, len(args))
			countQuery = fmt.Sprintf(`
				WITH latest AS (
					SELECT DISTINCT ON (pv.work_item_id) pv.added_reason, pv.verified_on
					%s %s
					ORDER BY pv.work_item_id, pv.added_on DESC
				)
				SELECT COUNT(*),
				       COUNT(*) FILTER (WHERE added_reason = 'AUTO_CLOSED'),
				       COUNT(*) FILTER (WHERE added_reason = 'MANUAL')
				FROM latest
				WHERE verified_on IS NOT NULL%s`, pendingVerificationCountFromJoin, where, addedReasonClause)
			countArgs = append(append([]any{}, args...), addedReasonArgs...)
		} else {
			countQuery = fmt.Sprintf(`
				SELECT COUNT(*),
				       COUNT(*) FILTER (WHERE pv.added_reason = 'AUTO_CLOSED'),
				       COUNT(*) FILTER (WHERE pv.added_reason = 'MANUAL')
				%s %s`, pendingVerificationCountFromJoin, where)
			countArgs = args
		}
		if err := r.db.QueryRow(egCtx, countQuery, countArgs...).Scan(&total, &autoClosedCount, &manualCount); err != nil {
			return fmt.Errorf("count pending_verifications: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		var typeCountsQuery string
		var typeCountsQueryArgs []any
		if dedupe {
			// added_reason has to be selected here too (it wasn't before) so
			// dedupeAddedReasonClause's outer filter below has a column to
			// apply itself to, same reasoning as the data/count queries above.
			addedReasonClause, addedReasonArgs := dedupeAddedReasonClause(req.Filters, len(typeCountsArgs))
			typeCountsQuery = fmt.Sprintf(`
				WITH latest AS (
					SELECT DISTINCT ON (pv.work_item_id) wi.type, pv.added_reason, pv.verified_on
					%s %s
					ORDER BY pv.work_item_id, pv.added_on DESC
				)
				SELECT type::TEXT, COUNT(*) FROM latest WHERE verified_on IS NOT NULL%s GROUP BY type`,
				pendingVerificationCountFromJoin, typeCountsWhere, addedReasonClause)
			typeCountsQueryArgs = append(append([]any{}, typeCountsArgs...), addedReasonArgs...)
		} else {
			typeCountsQuery = fmt.Sprintf(`
				SELECT wi.type::TEXT, COUNT(*)
				%s %s
				GROUP BY wi.type`, pendingVerificationCountFromJoin, typeCountsWhere)
			typeCountsQueryArgs = typeCountsArgs
		}
		rows, err := r.db.Query(egCtx, typeCountsQuery, typeCountsQueryArgs...)
		if err != nil {
			return fmt.Errorf("count pending_verifications by type: %w", err)
		}
		defer rows.Close()

		result := make([]domain.PendingVerificationTypeCount, 0)
		for rows.Next() {
			var tc domain.PendingVerificationTypeCount
			if err := rows.Scan(&tc.RecordType, &tc.Count); err != nil {
				return fmt.Errorf("scan pending_verification type count: %w", err)
			}
			result = append(result, tc)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate pending_verification type counts: %w", err)
		}
		typeCounts = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return domain.SearchPendingVerificationsResponse{}, err
	}

	return domain.SearchPendingVerificationsResponse{
		PendingVerifications: entries,
		Total:                total,
		AutoClosedCount:      autoClosedCount,
		ManualCount:          manualCount,
		TypeCounts:           typeCounts,
		Limit:                req.Pagination.Limit,
		Offset:               req.Pagination.Offset,
		HasMore:              req.Pagination.Offset+len(entries) < total,
	}, nil
}
