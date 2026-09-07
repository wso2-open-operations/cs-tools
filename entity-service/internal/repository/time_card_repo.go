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
)

// TimeCardRepository is the native (Postgres) persistence for time cards.
// SearchCaseTimeCards (the by-case rollup) is intentionally absent: it is a
// ServiceNow-only capability, so the service layer rejects it before reaching
// the repository.
type TimeCardRepository interface {
	// CreateTimeCard inserts a time card (in the submitted state) and its
	// eligible-approver rows atomically, returning the enriched view.
	CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest, submitter string) (domain.TimeCardView, error)
	// GetTimeCardByID returns the enriched view, or a NotFoundError.
	GetTimeCardByID(ctx context.Context, id string) (domain.TimeCardView, error)
	// SearchTimeCards returns a filtered, paginated page of views plus the total
	// count of matching rows before pagination.
	SearchTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) ([]domain.TimeCardView, int, error)
	// UpdateTimeCardFields edits the mutable fields of a time card (and replaces
	// its approver set when ApproverIDs is non-nil). Returns a NotFoundError if
	// the row does not exist.
	UpdateTimeCardFields(ctx context.Context, req domain.UpdateTimeCardRequest) (domain.TimeCardView, error)
	// TransitionTimeCard moves a card to approved or rejected, stamping approved_by
	// (on approve) or rejection_reason (on reject). Returns a NotFoundError if the
	// row does not exist.
	TransitionTimeCard(ctx context.Context, id string, state domain.TimeCardState, approvedBy string, rejectionReason *string) (domain.TimeCardView, error)
	// DeleteTimeCard permanently removes the card (approver rows cascade).
	DeleteTimeCard(ctx context.Context, id string) error
}

type timeCardRepo struct {
	db *pgxpool.Pool
}

// NewTimeCardRepository constructs a TimeCardRepository backed by the pool.
func NewTimeCardRepository(db *pgxpool.Pool) TimeCardRepository {
	return &timeCardRepo{db: db}
}

// querier is the subset of pgxpool.Pool / pgx.Tx used here, so loadView can run
// on either the pool or an open transaction.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// timeCardSelectBase selects every column loadTimeCardRows needs, in order.
// total_time is the sum of the five effort buckets, cast to float8 to match
// domain.TimeCardView.TotalTime.
const timeCardSelectBase = `
	SELECT tc.id, tc.work_date,
	       (tc.time_analyzing + tc.time_setting_up + tc.time_reproducing_debugging +
	        tc.time_providing_solution + tc.time_patching)::float8 AS total_time,
	       tc.is_billable,
	       tc.time_analyzing, tc.time_setting_up, tc.time_reproducing_debugging,
	       tc.time_providing_solution, tc.time_patching,
	       tc.issue_complexity, tc.work_log_comment, tc.rejection_reason, tc.state,
	       su.id, TRIM(su.first_name || ' ' || su.last_name) AS submitter_name,
	       ab.id, TRIM(ab.first_name || ' ' || ab.last_name) AS approver_name,
	       p.id, p.name,
	       c.id, c.subject, c.number
	FROM time_cards tc
	JOIN users su ON su.id = tc.submitter
	LEFT JOIN users ab ON ab.id = tc.approved_by
	JOIN projects p ON p.id = tc.project_id
	JOIN cases c ON c.id = tc.case_id`

// scanTimeCardRow scans one timeCardSelectBase row into a view (approvers are
// loaded separately by attachApprovers).
func scanTimeCardRow(row pgx.Row) (domain.TimeCardView, error) {
	var (
		v                               domain.TimeCardView
		workDate                        time.Time
		issueComplexity, workLogComment *string
		rejectionReason, state          *string
		submitterID, submitterName      string
		approvedByID, approvedByName    *string
		projectID, projectName          string
		caseID, caseSubject, caseNumber string
	)
	if err := row.Scan(
		&v.ID, &workDate, &v.TotalTime, &v.HasBillable,
		&v.TimeAnalyzing, &v.TimeSettingUp, &v.TimeReproducingDebugging,
		&v.TimeProvidingSolution, &v.TimePatching,
		&issueComplexity, &workLogComment, &rejectionReason, &state,
		&submitterID, &submitterName,
		&approvedByID, &approvedByName,
		&projectID, &projectName,
		&caseID, &caseSubject, &caseNumber,
	); err != nil {
		return domain.TimeCardView{}, err
	}
	dateStr := workDate.Format("2006-01-02")
	v.WorkDate = dateStr
	v.CreatedOn = dateStr
	v.IssueComplexity = issueComplexity
	v.WorkLogComment = workLogComment
	v.RejectionReason = rejectionReason
	v.State = state
	v.User = &domain.TimeCardRef{ID: submitterID, Name: submitterName}
	v.Project = &domain.TimeCardRef{ID: projectID, Name: projectName}
	v.Case = &domain.TimeCardCaseRef{ID: caseID, Name: caseSubject, Number: caseNumber}
	if approvedByID != nil {
		name := ""
		if approvedByName != nil {
			name = *approvedByName
		}
		v.ApprovedBy = &domain.TimeCardRef{ID: *approvedByID, Name: name}
	}
	return v, nil
}

// attachApprovers loads the eligible approvers for every card in views and
// attaches them. Runs on q so it can share an open transaction.
func attachApprovers(ctx context.Context, q querier, views []domain.TimeCardView) error {
	if len(views) == 0 {
		return nil
	}
	ids := make([]string, len(views))
	for i := range views {
		ids[i] = views[i].ID
	}
	rows, err := q.Query(ctx, `
		SELECT tca.time_card_id, u.id, TRIM(u.first_name || ' ' || u.last_name)
		FROM time_card_approvers tca
		JOIN users u ON u.id = tca.approver_id
		WHERE tca.time_card_id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("load time card approvers: %w", err)
	}
	defer rows.Close()
	byCard := map[string][]domain.TimeCardRef{}
	for rows.Next() {
		var cardID, uID, uName string
		if err := rows.Scan(&cardID, &uID, &uName); err != nil {
			return fmt.Errorf("scan time card approver: %w", err)
		}
		byCard[cardID] = append(byCard[cardID], domain.TimeCardRef{ID: uID, Name: uName})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate time card approvers: %w", err)
	}
	for i := range views {
		if a := byCard[views[i].ID]; len(a) > 0 {
			views[i].Approvers = a
		}
	}
	return nil
}

// loadTimeCardByID loads one enriched view (with approvers) on q.
func (r *timeCardRepo) loadTimeCardByID(ctx context.Context, q querier, id string) (domain.TimeCardView, error) {
	v, err := scanTimeCardRow(q.QueryRow(ctx, timeCardSelectBase+"\n\tWHERE tc.id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TimeCardView{}, &apierror.NotFoundError{Msg: "time card not found"}
	}
	if err != nil {
		return domain.TimeCardView{}, fmt.Errorf("load time card: %w", err)
	}
	views := []domain.TimeCardView{v}
	if err := attachApprovers(ctx, q, views); err != nil {
		return domain.TimeCardView{}, err
	}
	return views[0], nil
}

// GetTimeCardByID implements TimeCardRepository.
func (r *timeCardRepo) GetTimeCardByID(ctx context.Context, id string) (domain.TimeCardView, error) {
	return r.loadTimeCardByID(ctx, r.db, id)
}

// CreateTimeCard implements TimeCardRepository.
func (r *timeCardRepo) CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest, submitter string) (domain.TimeCardView, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.TimeCardView{}, fmt.Errorf("create time card: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO time_cards (
			case_id, project_id, submitter, work_date, state, is_billable,
			issue_complexity, work_log_comment,
			time_analyzing, time_setting_up, time_reproducing_debugging,
			time_providing_solution, time_patching)
		VALUES ($1, $2, $3, $4::date, 'submitted', $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`,
		req.CaseID, req.ProjectID, submitter, req.Date, req.IsBillable,
		req.IssueComplexity, req.WorkLogComment,
		req.TimeAnalyzing, req.TimeSettingUp, req.TimeReproducingDebugging,
		req.TimeProvidingSolution, req.TimePatching,
	).Scan(&id)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.TimeCardView{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
		}
		return domain.TimeCardView{}, fmt.Errorf("insert time card: %w", err)
	}
	if err := insertApprovers(ctx, tx, id, req.ApproverIDs); err != nil {
		return domain.TimeCardView{}, err
	}
	v, err := r.loadTimeCardByID(ctx, tx, id)
	if err != nil {
		return domain.TimeCardView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.TimeCardView{}, fmt.Errorf("create time card: commit: %w", err)
	}
	return v, nil
}

// insertApprovers inserts the eligible-approver rows for a card. Duplicates in
// the input are ignored (ON CONFLICT DO NOTHING).
func insertApprovers(ctx context.Context, q querier, cardID string, approverIDs []string) error {
	for _, aid := range approverIDs {
		if _, err := q.Exec(ctx, `
			INSERT INTO time_card_approvers (time_card_id, approver_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`, cardID, aid); err != nil {
			if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return &apierror.ValidationError{Msg: "approver does not exist: " + aid}
			}
			return fmt.Errorf("insert time card approver: %w", err)
		}
	}
	return nil
}

// UpdateTimeCardFields implements TimeCardRepository.
func (r *timeCardRepo) UpdateTimeCardFields(ctx context.Context, req domain.UpdateTimeCardRequest) (domain.TimeCardView, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.TimeCardView{}, fmt.Errorf("update time card: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	set := []string{"updated_at = NOW()"}
	args := []any{req.ID}
	n := 1
	add := func(expr string, val any) { n++; set = append(set, fmt.Sprintf(expr, n)); args = append(args, val) }
	if req.Date != nil {
		add("work_date = $%d::date", *req.Date)
	}
	if req.IsBillable != nil {
		add("is_billable = $%d", *req.IsBillable)
	}
	if req.IssueComplexity != nil {
		add("issue_complexity = $%d", *req.IssueComplexity)
	}
	if req.WorkLogComment != nil {
		add("work_log_comment = $%d", *req.WorkLogComment)
	}
	if req.LeadComment != nil {
		add("lead_comment = $%d", *req.LeadComment)
	}
	if req.TimeAnalyzing != nil {
		add("time_analyzing = $%d", *req.TimeAnalyzing)
	}
	if req.TimeSettingUp != nil {
		add("time_setting_up = $%d", *req.TimeSettingUp)
	}
	if req.TimeReproducingDebugging != nil {
		add("time_reproducing_debugging = $%d", *req.TimeReproducingDebugging)
	}
	if req.TimeProvidingSolution != nil {
		add("time_providing_solution = $%d", *req.TimeProvidingSolution)
	}
	if req.TimePatching != nil {
		add("time_patching = $%d", *req.TimePatching)
	}

	tag, err := tx.Exec(ctx, "UPDATE time_cards SET "+strings.Join(set, ", ")+" WHERE id = $1", args...)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return domain.TimeCardView{}, &apierror.ValidationError{Msg: "time buckets must not be negative"}
		}
		return domain.TimeCardView{}, fmt.Errorf("update time card: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.TimeCardView{}, &apierror.NotFoundError{Msg: "time card not found"}
	}
	if req.ApproverIDs != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM time_card_approvers WHERE time_card_id = $1`, req.ID); err != nil {
			return domain.TimeCardView{}, fmt.Errorf("clear time card approvers: %w", err)
		}
		if err := insertApprovers(ctx, tx, req.ID, req.ApproverIDs); err != nil {
			return domain.TimeCardView{}, err
		}
	}
	v, err := r.loadTimeCardByID(ctx, tx, req.ID)
	if err != nil {
		return domain.TimeCardView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.TimeCardView{}, fmt.Errorf("update time card: commit: %w", err)
	}
	return v, nil
}

// TransitionTimeCard implements TimeCardRepository.
func (r *timeCardRepo) TransitionTimeCard(ctx context.Context, id string, state domain.TimeCardState, approvedBy string, rejectionReason *string) (domain.TimeCardView, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.TimeCardView{}, fmt.Errorf("transition time card: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var approvedByArg any
	var rejectionArg any
	if state == domain.TimeCardStateApproved {
		approvedByArg = approvedBy
	}
	if state == domain.TimeCardStateRejected {
		rejectionArg = rejectionReason
	}
	tag, err := tx.Exec(ctx, `
		UPDATE time_cards
		SET state = $2::time_card_state_enum,
		    approved_by = $3,
		    rejection_reason = $4,
		    updated_at = NOW()
		WHERE id = $1`, id, string(state), approvedByArg, rejectionArg)
	if err != nil {
		return domain.TimeCardView{}, fmt.Errorf("transition time card: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.TimeCardView{}, &apierror.NotFoundError{Msg: "time card not found"}
	}
	v, err := r.loadTimeCardByID(ctx, tx, id)
	if err != nil {
		return domain.TimeCardView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.TimeCardView{}, fmt.Errorf("transition time card: commit: %w", err)
	}
	return v, nil
}

// DeleteTimeCard implements TimeCardRepository.
func (r *timeCardRepo) DeleteTimeCard(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM time_cards WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete time card: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "time card not found"}
	}
	return nil
}

// SearchTimeCards implements TimeCardRepository.
func (r *timeCardRepo) SearchTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) ([]domain.TimeCardView, int, error) {
	where := []string{"1 = 1"}
	args := []any{}
	n := 0
	add := func(expr string, val any) { n++; where = append(where, fmt.Sprintf(expr, n)); args = append(args, val) }

	if f := req.Filters; f != nil {
		if len(f.ProjectIDs) > 0 {
			add("tc.project_id = ANY($%d)", f.ProjectIDs)
		}
		if f.CaseID != nil {
			add("tc.case_id = $%d", *f.CaseID)
		}
		if f.UserID != nil {
			add("tc.submitter = $%d", *f.UserID)
		}
		if len(f.UserIDs) > 0 {
			add("tc.submitter = ANY($%d)", f.UserIDs)
		}
		if f.ApprovedByID != nil {
			add("tc.approved_by = $%d", *f.ApprovedByID)
		}
		if f.ApproverID != nil {
			// eligible approver of the card, never the card's own submitter
			n++
			where = append(where, fmt.Sprintf("tc.id IN (SELECT time_card_id FROM time_card_approvers WHERE approver_id = $%d) AND tc.submitter <> $%d", n, n))
			args = append(args, *f.ApproverID)
		}
		if f.StartDate != nil {
			add("tc.work_date >= $%d::date", *f.StartDate)
		}
		if f.EndDate != nil {
			add("tc.work_date <= $%d::date", *f.EndDate)
		}
		if len(f.States) > 0 {
			states := make([]string, len(f.States))
			for i, s := range f.States {
				states[i] = string(s)
			}
			add("tc.state = ANY($%d::time_card_state_enum[])", states)
		}
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM time_cards tc WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count time cards: %w", err)
	}

	orderCol := "tc.updated_at"
	if req.SortBy.Field == domain.TimeCardSortFieldWorkDate {
		orderCol = "tc.work_date"
	}
	orderDir := "DESC"
	if req.SortBy.Order == domain.TimeCardSortOrderAsc {
		orderDir = "ASC"
	}
	limitArg := n + 1
	offsetArg := n + 2
	query := timeCardSelectBase + "\n\tWHERE " + whereSQL +
		fmt.Sprintf("\n\tORDER BY %s %s\n\tLIMIT $%d OFFSET $%d", orderCol, orderDir, limitArg, offsetArg)
	args = append(args, req.Pagination.Limit, req.Pagination.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search time cards: %w", err)
	}
	defer rows.Close()
	views := []domain.TimeCardView{}
	for rows.Next() {
		v, err := scanTimeCardRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan time card: %w", err)
		}
		views = append(views, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate time cards: %w", err)
	}
	if err := attachApprovers(ctx, r.db, views); err != nil {
		return nil, 0, err
	}
	return views, total, nil
}
