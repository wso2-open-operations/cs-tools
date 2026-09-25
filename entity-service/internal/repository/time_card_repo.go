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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// TimeCardRepository defines the persistence operations for the time_card
// and time_card_approver tables (migration 000039).
type TimeCardRepository interface {
	// SearchTimeCards returns a filtered, sorted, paginated slice of time
	// cards together with the total count of matching rows before
	// pagination. callerEmail is the caller's own resolved identity,
	// threaded down for filters.approverId/approvedById resolution.
	// authProjectIDs is a SEPARATE thing from filters.ProjectIDs -- it
	// restricts by the case's real project (wi.project_id, via the
	// mandatory tc.case_id join), resolved purely from the caller's own
	// AccessScope, never from request input; filters.ProjectIDs stays a
	// caller-supplied business filter on the row's own, optional,
	// independently-set tc.customer_project_id column, which this
	// authorization check deliberately does NOT reuse -- that column can be
	// NULL even when the underlying case has a real project (an empty
	// req.ProjectID on CreateTimeCard leaves it NULL), so filtering
	// authorization on it would incorrectly exclude a caller's own
	// legitimate time cards. nil authProjectIDs means unrestricted
	// (internal caller).
	SearchTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest, callerEmail string, authProjectIDs []string) ([]domain.TimeCardView, int, error)
	// SearchCaseTimeCards returns the same filtered set as SearchTimeCards,
	// grouped and rolled up by case, together with the total count of
	// distinct matching cases before pagination. The returned project comes
	// from the case's own work_item.project_id, not any individual time
	// card's customer_project_id: the latter varies per row (in principle;
	// CreateTimeCard validates it against the case's project when supplied)
	// and grouping by it would fragment one case into multiple summary rows
	// while COUNT(DISTINCT tc.case_id) still counted it once. callerEmail is
	// threaded down for the same reason as SearchTimeCards; authProjectIDs
	// is the same authorization-only filter, identical reasoning.
	SearchCaseTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest, callerEmail string, authProjectIDs []string) ([]domain.CaseTimeCardSummary, int, error)
	// CreateTimeCard inserts a new time card in the "submitted" state,
	// submitted by userID, plus one time_card_approver row per
	// req.ApproverIDs, all in one transaction. When req.ProjectID is
	// non-empty it must match the case's own project (work_item.project_id);
	// an empty req.ProjectID leaves customer_project_id NULL. Returns a
	// ValidationError if req.CaseID does not exist, req.ProjectID does not
	// match the case's project, or any approver id does not exist.
	CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest, userID string) (domain.TimeCardView, error)
	// UpdateTimeCardFields applies req's non-nil editable fields (everything
	// except State/LeadComment, which go through TransitionTimeCardState
	// instead) to the time card identified by req.ID, replacing its approver
	// list wholesale when req.ApproverIDs is non-nil. Only the card's own
	// submitter (actorID) can edit it, and only while it is in the
	// "submitted" state -- both enforced atomically in the UPDATE's own
	// WHERE clause, not as a separate check, to avoid a TOCTOU gap. Returns
	// a ConflictError if the card does not exist, does not belong to
	// actorID, or is not in the "submitted" state (deliberately not
	// distinguished, the same as DeleteTimeCard below, so the response
	// can't be used to enumerate other users' card ids/states).
	UpdateTimeCardFields(ctx context.Context, req domain.UpdateTimeCardRequest, actorID string) (domain.TimeCardView, error)
	// TransitionTimeCardState sets the time card identified by id to state
	// ("approved" or "rejected", validated by the caller), recording
	// actorID as approved_by_id when approving and leadComment (if any)
	// regardless of which transition. Only an eligible approver (a row in
	// time_card_approver for this card) other than the card's own submitter
	// may do this, and only while the card is still "submitted" -- both
	// checked and then acted on inside one transaction (a SELECT ... FOR
	// UPDATE followed by the UPDATE) so a concurrent approver-list edit or a
	// second transition attempt can't slip through between the check and
	// the write. Returns a NotFoundError if id does not exist; a
	// ForbiddenError if actorID is not an eligible approver, or is the
	// card's own submitter (self-approval); a ConflictError if the card is
	// not currently "submitted" (already approved/rejected/processed/
	// recalled).
	TransitionTimeCardState(ctx context.Context, id string, state domain.TimeCardState, leadComment *string, actorID string) (domain.TimeCardView, error)
	// DeleteTimeCard permanently deletes the time card identified by id, but
	// only if it belongs to submitterID and is still in the "submitted"
	// state. Returns a ConflictError otherwise (including if id does not
	// exist at all) -- by the time this is called the caller has already
	// resolved the card, so any mismatch here means it changed concurrently
	// or was never theirs to delete.
	DeleteTimeCard(ctx context.Context, id, submitterID string) error
}

type timeCardRepo struct {
	db *pgxpool.Pool
}

// NewTimeCardRepository constructs a TimeCardRepository backed by the given connection pool.
func NewTimeCardRepository(db *pgxpool.Pool) TimeCardRepository {
	return &timeCardRepo{db: db}
}

const timeCardSelectColumns = `
	tc.id, tc.work_date, tc.is_billable, tc.state::TEXT, tc.issue_complexity::TEXT,
	tc.analyzing_minutes, tc.setting_up_minutes, tc.reproducing_debugging_minutes,
	tc.providing_solution_minutes, tc.patching_minutes, tc.work_log_comment, tc.lead_comment,
	u.id, TRIM(COALESCE(u.name, CONCAT_WS(' ', u.first_name, u.last_name))),
	ab.id, TRIM(COALESCE(ab.name, CONCAT_WS(' ', ab.first_name, ab.last_name))),
	p.id, p.name,
	wi.id, wi.number, wi.subject`

// timeCardFromJoins joins work_item directly (not "case"): time_card.case_id
// now references work_item(id) generically (migration 000039's most recent
// revision), not "case"(id) specifically -- a time card can be logged
// against any case-like work_item type, not just CASE. Only wi.number/
// wi.subject are ever read for the case reference, so no "case"-specific
// column is needed here at all.
const timeCardFromJoins = `
	FROM time_card tc
	JOIN "user" u ON u.id = tc.user_id
	LEFT JOIN "user" ab ON ab.id = tc.approved_by_id
	LEFT JOIN project p ON p.id = tc.customer_project_id
	JOIN work_item wi ON wi.id = tc.case_id`

func scanTimeCardView(row interface{ Scan(...any) error }) (domain.TimeCardView, error) {
	var (
		v                                                      domain.TimeCardView
		workDate                                               *time.Time
		isBillable                                             *bool
		state, issueComplexity                                 *string
		analyzing, settingUp, reproducing, providing, patching int
		userID, userName                                       string
		approvedByID, approvedByName                           *string
		projectID, projectName                                 *string
		caseID, caseNumber, caseSubject                        string
	)
	err := row.Scan(
		&v.ID, &workDate, &isBillable, &state, &issueComplexity,
		&analyzing, &settingUp, &reproducing, &providing, &patching, &v.WorkLogComment, &v.RejectionReason,
		&userID, &userName,
		&approvedByID, &approvedByName,
		&projectID, &projectName,
		&caseID, &caseNumber, &caseSubject,
	)
	if err != nil {
		return domain.TimeCardView{}, err
	}
	// time_card_state_enum/time_card_issue_complexity_enum are UPPER_SNAKE_CASE
	// (migration 000039's most recent revision); domain.TimeCardState's own
	// values, and every caller-supplied issueComplexity string, are lowercase.
	if state != nil {
		lower := strings.ToLower(*state)
		v.State = &lower
	}
	if issueComplexity != nil {
		lower := strings.ToLower(*issueComplexity)
		v.IssueComplexity = &lower
	}

	v.TimeAnalyzing = analyzing
	v.TimeSettingUp = settingUp
	v.TimeReproducingDebugging = reproducing
	v.TimeProvidingSolution = providing
	v.TimePatching = patching
	v.TotalTime = float64(analyzing+settingUp+reproducing+providing+patching) / 60.0
	if isBillable != nil {
		v.HasBillable = *isBillable
	}
	if workDate != nil {
		d := workDate.Format("2006-01-02")
		v.WorkDate = d
		// CreatedOn is deprecated and documented to carry the same value as
		// WorkDate until callers migrate off it -- see TimeCardView's own
		// doc comment.
		v.CreatedOn = d
	}
	v.User = &domain.TimeCardRef{ID: userID, Name: userName}
	if approvedByID != nil {
		name := ""
		if approvedByName != nil {
			name = *approvedByName
		}
		v.ApprovedBy = &domain.TimeCardRef{ID: *approvedByID, Name: name}
	}
	if projectID != nil {
		name := ""
		if projectName != nil {
			name = *projectName
		}
		v.Project = &domain.TimeCardRef{ID: *projectID, Name: name}
	}
	v.Case = &domain.TimeCardCaseRef{ID: caseID, Number: caseNumber, Name: caseSubject}
	return v, nil
}

// getTimeCardByID fetches the full view (including approvers) for one time
// card. Shared by CreateTimeCard/UpdateTimeCardFields/TransitionTimeCardState
// so all three return the exact same shape SearchTimeCards does.
func (r *timeCardRepo) getTimeCardByID(ctx context.Context, id string) (domain.TimeCardView, error) {
	query := "SELECT " + timeCardSelectColumns + " " + timeCardFromJoins + " WHERE tc.id = $1"
	v, err := scanTimeCardView(r.db.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TimeCardView{}, &apierror.NotFoundError{Msg: "time card not found"}
	}
	if err != nil {
		return domain.TimeCardView{}, fmt.Errorf("get time card by id: %w", err)
	}

	approversByCard, err := r.getApprovers(ctx, []string{id})
	if err != nil {
		return domain.TimeCardView{}, err
	}
	v.Approvers = approversByCard[id]
	return v, nil
}

// getApprovers batch-fetches the approver list for every id in timeCardIDs,
// avoiding one query per card.
func (r *timeCardRepo) getApprovers(ctx context.Context, timeCardIDs []string) (map[string][]domain.TimeCardRef, error) {
	out := map[string][]domain.TimeCardRef{}
	if len(timeCardIDs) == 0 {
		return out, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT tca.time_card_id, u.id, TRIM(COALESCE(u.name, CONCAT_WS(' ', u.first_name, u.last_name)))
		FROM time_card_approver tca
		JOIN "user" u ON u.id = tca.approver_id
		WHERE tca.time_card_id = ANY($1::uuid[])
		ORDER BY tca.created_on, tca.id`, timeCardIDs)
	if err != nil {
		return nil, fmt.Errorf("list time card approvers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var timeCardID, userID, userName string
		if err := rows.Scan(&timeCardID, &userID, &userName); err != nil {
			return nil, fmt.Errorf("scan time card approver: %w", err)
		}
		out[timeCardID] = append(out[timeCardID], domain.TimeCardRef{ID: userID, Name: userName})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate time card approvers: %w", err)
	}
	return out, nil
}

// timeCardWhereClause builds the shared WHERE clause + args used by both
// SearchTimeCards and SearchCaseTimeCards, so the two can never drift out of
// sync on which cards a given filter set matches. authProjectIDs is kept
// separate from f.ProjectIDs -- see SearchTimeCards's own doc comment for
// why: it restricts by the case's real project (wi.project_id) rather than
// the row's own optional, independently-set tc.customer_project_id column,
// and is resolved purely from the caller's AccessScope, never from f. nil
// means unrestricted (internal caller) -- no filter added.
func timeCardWhereClause(f *domain.SearchTimeCardsFilters, authProjectIDs []string) (string, []any) {
	where := "WHERE 1=1"
	args := []any{}
	if len(authProjectIDs) > 0 {
		args = append(args, authProjectIDs)
		where += fmt.Sprintf(" AND wi.project_id = ANY($%d::uuid[])", len(args))
	}
	if f == nil {
		return where, args
	}

	argIdx := len(args) + 1
	add := func(clause string, val any) {
		where += fmt.Sprintf(" AND "+clause, argIdx)
		args = append(args, val)
		argIdx++
	}

	if len(f.ProjectIDs) > 0 {
		add("tc.customer_project_id = ANY($%d::uuid[])", f.ProjectIDs)
	}
	if f.CaseID != nil {
		add("tc.case_id = $%d", *f.CaseID)
	}
	if f.UserID != nil {
		add("tc.user_id = $%d", *f.UserID)
	}
	if len(f.UserIDs) > 0 {
		add("tc.user_id = ANY($%d::uuid[])", f.UserIDs)
	}
	if f.ApproverID != nil {
		// Their own cards are always excluded -- see SearchTimeCardsFilters.ApproverID's own doc comment.
		where += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM time_card_approver tca WHERE tca.time_card_id = tc.id AND tca.approver_id = $%d) AND tc.user_id <> $%d", argIdx, argIdx)
		args = append(args, *f.ApproverID)
		argIdx++
	}
	if f.ApprovedByID != nil {
		add("tc.approved_by_id = $%d", *f.ApprovedByID)
	}
	if f.StartDate != nil {
		// ::text::date, not a direct ::date cast: pgx v5's date codec has no
		// encode plan for a raw Go string once the server infers the
		// parameter's OID as `date` (which a direct cast does) -- casting
		// through text first keeps the parameter bound as text (matching a
		// Go string's own default codec), with the date conversion then
		// happening server-side. See the identical fix in
		// change_request_repo.go's PlannedStartOn/PlannedEndOn handling.
		add("tc.work_date >= $%d::text::date", *f.StartDate)
	}
	if f.EndDate != nil {
		add("tc.work_date <= $%d::text::date", *f.EndDate)
	}
	if len(f.States) > 0 {
		// time_card_state_enum's labels are UPPER_SNAKE_CASE; domain.TimeCardState's
		// own values are lowercase.
		states := make([]string, len(f.States))
		for i, st := range f.States {
			states[i] = strings.ToUpper(string(st))
		}
		// ::text[] before ::time_card_state_enum[]: this repository never
		// registers time_card_state_enum/_time_card_state_enum with pgx, so
		// binding a []string directly to ANY($n::time_card_state_enum[])
		// has no encode plan for that array OID. Casting through text[]
		// first keeps the parameter bound as pgx's default []string codec,
		// with the enum conversion happening server-side -- same fix as
		// every other enum column in this file, just for an array bind.
		add("tc.state = ANY($%d::text[]::time_card_state_enum[])", states)
	}
	return where, args
}

// SearchTimeCards implements TimeCardRepository.
func (r *timeCardRepo) SearchTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest, _ string, authProjectIDs []string) ([]domain.TimeCardView, int, error) {
	where, args := timeCardWhereClause(req.Filters, authProjectIDs)

	sortCol := "tc.updated_on"
	if req.SortBy.Field == domain.TimeCardSortFieldWorkDate {
		sortCol = "tc.work_date"
	}
	sortDir := "DESC"
	if req.SortBy.Order == domain.TimeCardSortOrderAsc {
		sortDir = "ASC"
	}

	countQuery := "SELECT COUNT(*) " + timeCardFromJoins + " " + where
	dataQuery := fmt.Sprintf("SELECT %s %s %s ORDER BY %s %s, tc.id LIMIT $%d OFFSET $%d",
		timeCardSelectColumns, timeCardFromJoins, where, sortCol, sortDir, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var views []domain.TimeCardView

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count time cards: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query time cards: %w", err)
		}
		defer rows.Close()

		out := make([]domain.TimeCardView, 0, req.Pagination.Limit)
		for rows.Next() {
			v, err := scanTimeCardView(rows)
			if err != nil {
				return fmt.Errorf("scan time card: %w", err)
			}
			out = append(out, v)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate time cards: %w", err)
		}
		views = out
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	ids := make([]string, len(views))
	for i, v := range views {
		ids[i] = v.ID
	}
	approversByCard, err := r.getApprovers(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range views {
		views[i].Approvers = approversByCard[views[i].ID]
	}

	return views, total, nil
}

// SearchCaseTimeCards implements TimeCardRepository.
func (r *timeCardRepo) SearchCaseTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest, _ string, authProjectIDs []string) ([]domain.CaseTimeCardSummary, int, error) {
	where, args := timeCardWhereClause(req.Filters, authProjectIDs)

	countQuery := fmt.Sprintf(`SELECT COUNT(DISTINCT tc.case_id) FROM time_card tc JOIN work_item wi ON wi.id = tc.case_id %s`, where)

	dataQuery := fmt.Sprintf(`
		SELECT wi.id, wi.number, wi.subject, wi.created_on, wi.updated_on, wi.created_by, wi.updated_by,
		       p.id, p.name,
		       COALESCE(SUM(tc.analyzing_minutes + tc.setting_up_minutes + tc.reproducing_debugging_minutes + tc.providing_solution_minutes + tc.patching_minutes), 0) AS total_minutes,
		       COUNT(tc.id) AS total_count,
		       COALESCE(SUM(CASE WHEN tc.is_billable THEN tc.analyzing_minutes + tc.setting_up_minutes + tc.reproducing_debugging_minutes + tc.providing_solution_minutes + tc.patching_minutes ELSE 0 END), 0) AS billable_minutes,
		       COUNT(*) FILTER (WHERE tc.is_billable) AS billable_count,
		       COALESCE(SUM(CASE WHEN NOT COALESCE(tc.is_billable, false) THEN tc.analyzing_minutes + tc.setting_up_minutes + tc.reproducing_debugging_minutes + tc.providing_solution_minutes + tc.patching_minutes ELSE 0 END), 0) AS non_billable_minutes,
		       COUNT(*) FILTER (WHERE NOT COALESCE(tc.is_billable, false)) AS non_billable_count
		FROM time_card tc
		JOIN work_item wi ON wi.id = tc.case_id
		LEFT JOIN project p ON p.id = wi.project_id
		%s
		GROUP BY wi.id, wi.number, wi.subject, wi.created_on, wi.updated_on, wi.created_by, wi.updated_by, p.id, p.name
		ORDER BY wi.updated_on DESC, wi.id
		LIMIT $%d OFFSET $%d`, where, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var summaries []domain.CaseTimeCardSummary

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count case time cards: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query case time cards: %w", err)
		}
		defer rows.Close()

		out := make([]domain.CaseTimeCardSummary, 0, req.Pagination.Limit)
		for rows.Next() {
			var (
				caseID, caseNumber, caseSubject      string
				caseCreatedOn, caseUpdatedOn         time.Time
				createdBy, updatedBy                 string
				projectID, projectName               *string
				totalMinutes, totalCount             int
				billableMinutes, billableCount       int
				nonBillableMinutes, nonBillableCount int
			)
			if err := rows.Scan(
				&caseID, &caseNumber, &caseSubject, &caseCreatedOn, &caseUpdatedOn, &createdBy, &updatedBy,
				&projectID, &projectName,
				&totalMinutes, &totalCount, &billableMinutes, &billableCount, &nonBillableMinutes, &nonBillableCount,
			); err != nil {
				return fmt.Errorf("scan case time card: %w", err)
			}

			createdOnStr := caseCreatedOn.UTC().Format(time.RFC3339)
			summary := domain.CaseTimeCardSummary{
				Case: domain.CaseTimeCardCaseRef{
					ID:        caseID,
					Number:    caseNumber,
					Name:      caseSubject,
					UpdatedOn: caseUpdatedOn.UTC().Format(time.RFC3339),
					CreatedOn: &createdOnStr,
					CreatedBy: &createdBy,
					UpdatedBy: &updatedBy,
				},
				TotalTime:   float64(totalMinutes) / 60.0,
				TotalCount:  totalCount,
				Billable:    domain.CaseTimeCardBillingInfo{TotalTime: float64(billableMinutes) / 60.0, Count: billableCount},
				NonBillable: domain.CaseTimeCardBillingInfo{TotalTime: float64(nonBillableMinutes) / 60.0, Count: nonBillableCount},
			}
			if projectID != nil {
				name := ""
				if projectName != nil {
					name = *projectName
				}
				summary.Case.Project = &domain.EntityRef{ID: *projectID, Name: name}
			}
			out = append(out, summary)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate case time cards: %w", err)
		}
		summaries = out
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return summaries, total, nil
}

// CreateTimeCard implements TimeCardRepository.
func (r *timeCardRepo) CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest, userID string) (domain.TimeCardView, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.TimeCardView{}, fmt.Errorf("create time card: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// The case's own project is work_item.project_id -- case_id now
	// references work_item(id) generically (migration 000039's most recent
	// revision), not "case"(id) specifically, so this looks up work_item
	// directly rather than joining through "case".
	// time_card.customer_project_id is a separate, independently-settable
	// column, so without this check a caller could attach an unrelated
	// existing project to a case's time card. Validate a supplied
	// req.ProjectID against it in this same transaction; when none is
	// supplied, leave customer_project_id NULL (unchanged behavior) rather
	// than auto-filling it in.
	// The type filter matters, not just style: case_id's FK is into
	// work_item(id) generically, with no type constraint of its own, so
	// without this a time card could be logged against a CHANGE_REQUEST or
	// INCIDENT id -- caseLikeWorkItemTypes (case_repo.go) is the same
	// case/engagement/service_request/security_report_analysis/announcement
	// set every other case-scoped query in this codebase restricts to.
	var caseProjectID *string
	err = tx.QueryRow(ctx, `SELECT project_id FROM work_item WHERE id = $1 AND type = ANY(`+caseLikeWorkItemTypes+`)`, req.CaseID).Scan(&caseProjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TimeCardView{}, &apierror.ValidationError{Msg: "case not found: " + req.CaseID}
	}
	if err != nil {
		return domain.TimeCardView{}, fmt.Errorf("look up case project: %w", err)
	}
	if req.ProjectID != "" && (caseProjectID == nil || *caseProjectID != req.ProjectID) {
		return domain.TimeCardView{}, &apierror.ValidationError{Msg: "projectId must match the case's own project"}
	}

	// 'SUBMITTED' (not 'submitted') and issue_complexity's ::text::enum cast:
	// time_card_state_enum/time_card_issue_complexity_enum are UPPER_SNAKE_CASE
	// (migration 000039's most recent revision). The ::text::enum cast on
	// issue_complexity -- not a direct ::enum cast -- avoids the same pgx v5
	// codec issue this file's date fields already work around: once the
	// server infers a parameter's OID as a custom enum type, pgx has no
	// binary encode plan for a raw Go string.
	const insertQuery = `
		INSERT INTO time_card (
			id, created_on, updated_on, created_by, updated_by,
			case_id, customer_project_id, user_id, work_date, is_billable, state,
			issue_complexity, work_log_comment,
			analyzing_minutes, setting_up_minutes, reproducing_debugging_minutes,
			providing_solution_minutes, patching_minutes
		) VALUES (
			gen_random_uuid(), NOW(), NOW(), $1, $1,
			$2, $3, $1, $4::text::date, $5, 'SUBMITTED',
			$6::text::time_card_issue_complexity_enum, $7, $8, $9, $10, $11, $12
		) RETURNING id`

	var issueComplexity *string
	if req.IssueComplexity != nil {
		upper := strings.ToUpper(*req.IssueComplexity)
		issueComplexity = &upper
	}

	var id string
	err = tx.QueryRow(ctx, insertQuery,
		userID, req.CaseID, nullIfEmpty(req.ProjectID), req.Date, req.IsBillable,
		issueComplexity, req.WorkLogComment,
		req.TimeAnalyzing, req.TimeSettingUp, req.TimeReproducingDebugging, req.TimeProvidingSolution, req.TimePatching,
	).Scan(&id)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.TimeCardView{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
		}
		return domain.TimeCardView{}, fmt.Errorf("insert time card: %w", err)
	}

	for _, approverID := range req.ApproverIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO time_card_approver (id, created_on, updated_on, created_by, updated_by, time_card_id, approver_id)
			 VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3)`,
			userID, id, approverID,
		); err != nil {
			if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return domain.TimeCardView{}, &apierror.ValidationError{Msg: "one or more approver IDs do not exist: " + pgErr.Detail}
			}
			return domain.TimeCardView{}, fmt.Errorf("insert time card approver: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.TimeCardView{}, fmt.Errorf("create time card: commit tx: %w", err)
	}

	return r.getTimeCardByID(ctx, id)
}

// UpdateTimeCardFields implements TimeCardRepository.
func (r *timeCardRepo) UpdateTimeCardFields(ctx context.Context, req domain.UpdateTimeCardRequest, actorID string) (domain.TimeCardView, error) {
	sets := []string{"updated_on = NOW()"}
	args := []any{}
	argIdx := 1

	add := func(assignment string, val any) {
		sets = append(sets, fmt.Sprintf(assignment, argIdx))
		args = append(args, val)
		argIdx++
	}

	add("updated_by = $%d", actorID)
	// actorArg is reused below in the WHERE clause's ownership check
	// (AND user_id = $actorArg) -- the same bound value backing SET
	// updated_by above, referenced a second time by its existing
	// placeholder rather than appended again.
	actorArg := argIdx - 1
	if req.Date != nil {
		add("work_date = $%d::text::date", *req.Date)
	}
	if req.IsBillable != nil {
		add("is_billable = $%d", *req.IsBillable)
	}
	if req.IssueComplexity != nil {
		add("issue_complexity = $%d::text::time_card_issue_complexity_enum", strings.ToUpper(*req.IssueComplexity))
	}
	if req.WorkLogComment != nil {
		add("work_log_comment = $%d", *req.WorkLogComment)
	}
	if req.TimeAnalyzing != nil {
		add("analyzing_minutes = $%d", *req.TimeAnalyzing)
	}
	if req.TimeSettingUp != nil {
		add("setting_up_minutes = $%d", *req.TimeSettingUp)
	}
	if req.TimeReproducingDebugging != nil {
		add("reproducing_debugging_minutes = $%d", *req.TimeReproducingDebugging)
	}
	if req.TimeProvidingSolution != nil {
		add("providing_solution_minutes = $%d", *req.TimeProvidingSolution)
	}
	if req.TimePatching != nil {
		add("patching_minutes = $%d", *req.TimePatching)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.TimeCardView{}, fmt.Errorf("update time card: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	args = append(args, req.ID)
	// user_id = $actorArg is the ownership check: without it, any
	// authenticated caller could edit any other user's submitted time card
	// (an IDOR) purely by knowing its id. Only the submitter may edit their
	// own card while it's submitted -- matching DeleteTimeCard's own
	// ownership guard below.
	query := fmt.Sprintf(`UPDATE time_card SET %s WHERE id = $%d AND user_id = $%d AND state = 'SUBMITTED' RETURNING id`, strings.Join(sets, ", "), argIdx, actorArg)

	var id string
	err = tx.QueryRow(ctx, query, args...).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TimeCardView{}, &apierror.ConflictError{Msg: "time card is not editable (it may not exist, may not belong to you, or is no longer in the submitted state)"}
	}
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.TimeCardView{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
		}
		return domain.TimeCardView{}, fmt.Errorf("update time card fields: %w", err)
	}

	if req.ApproverIDs != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM time_card_approver WHERE time_card_id = $1`, id); err != nil {
			return domain.TimeCardView{}, fmt.Errorf("clear time card approvers: %w", err)
		}
		for _, approverID := range req.ApproverIDs {
			if _, err := tx.Exec(ctx,
				`INSERT INTO time_card_approver (id, created_on, updated_on, created_by, updated_by, time_card_id, approver_id)
				 VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3)`,
				actorID, id, approverID,
			); err != nil {
				if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
					return domain.TimeCardView{}, &apierror.ValidationError{Msg: "one or more approver IDs do not exist: " + pgErr.Detail}
				}
				return domain.TimeCardView{}, fmt.Errorf("insert time card approver: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.TimeCardView{}, fmt.Errorf("update time card: commit tx: %w", err)
	}

	return r.getTimeCardByID(ctx, id)
}

// TransitionTimeCardState implements TimeCardRepository.
func (r *timeCardRepo) TransitionTimeCardState(ctx context.Context, id string, state domain.TimeCardState, leadComment *string, actorID string) (domain.TimeCardView, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.TimeCardView{}, fmt.Errorf("transition time card state: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the row and check eligibility AND current state before writing
	// anything: only an approver on this specific card, other than its own
	// submitter, may transition it, and only while it is still "submitted"
	// -- without that state check, an eligible approver could re-approve/
	// reject an already approved/rejected/processed/recalled card. FOR
	// UPDATE holds the lock across both statements in this transaction,
	// closing the gap a plain check-then-UPDATE would leave for a
	// concurrent approver-list edit (or a second transition attempt) to
	// race through.
	var submitterID string
	var currentState *string
	var isApprover bool
	err = tx.QueryRow(ctx, `
		SELECT tc.user_id, tc.state::TEXT, EXISTS (
			SELECT 1 FROM time_card_approver tca WHERE tca.time_card_id = tc.id AND tca.approver_id = $2
		)
		FROM time_card tc WHERE tc.id = $1 FOR UPDATE`, id, actorID,
	).Scan(&submitterID, &currentState, &isApprover)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TimeCardView{}, &apierror.NotFoundError{Msg: "time card not found"}
	}
	if err != nil {
		return domain.TimeCardView{}, fmt.Errorf("check time card approver eligibility: %w", err)
	}
	if !isApprover || submitterID == actorID {
		return domain.TimeCardView{}, &apierror.ForbiddenError{Msg: "only an eligible approver, other than the submitter, may approve or reject this time card"}
	}
	// time_card_state_enum is UPPER_SNAKE_CASE; domain.TimeCardStateSubmitted
	// is lowercase.
	if currentState == nil || strings.ToUpper(*currentState) != strings.ToUpper(string(domain.TimeCardStateSubmitted)) {
		return domain.TimeCardView{}, &apierror.ConflictError{Msg: "time card is not in the submitted state (it may already have been approved, rejected, processed, or recalled)"}
	}

	// $2's ::text::enum cast on SET (not a direct ::enum cast) avoids the
	// same pgx v5 codec issue this file's date fields already work around
	// -- see CreateTimeCard's own comment on this. The CASE WHEN comparison
	// stays a bare text comparison against the same (already-uppercased)
	// $2 value, matching case_repo.go's updateCaseQuery's identical pattern
	// for case_state_enum.
	const query = `
		UPDATE time_card
		SET state = $2::text::time_card_state_enum,
		    lead_comment = COALESCE($3, lead_comment),
		    approved_by_id = CASE WHEN $2 = 'APPROVED' THEN $4::uuid ELSE approved_by_id END,
		    updated_on = NOW(),
		    updated_by = $4
		WHERE id = $1
		RETURNING id`

	var returnedID string
	if err := tx.QueryRow(ctx, query, id, strings.ToUpper(string(state)), leadComment, actorID).Scan(&returnedID); err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.TimeCardView{}, &apierror.ValidationError{Msg: pgErr.Detail}
		}
		return domain.TimeCardView{}, fmt.Errorf("transition time card state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.TimeCardView{}, fmt.Errorf("transition time card state: commit tx: %w", err)
	}

	return r.getTimeCardByID(ctx, returnedID)
}

// DeleteTimeCard implements TimeCardRepository.
func (r *timeCardRepo) DeleteTimeCard(ctx context.Context, id, submitterID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM time_card WHERE id = $1 AND user_id = $2 AND state = 'SUBMITTED'`, id, submitterID)
	if err != nil {
		return fmt.Errorf("delete time card: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.ConflictError{Msg: "time card cannot be deleted (it may not exist, may not belong to you, or is no longer in the submitted state)"}
	}
	return nil
}
