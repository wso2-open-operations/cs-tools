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
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ProjectRepository defines the persistence operations for the project
// table (migration 000009). domain.Project.SubscriptionType is populated
// from project.project_type_id's linked project_type.name (migrations
// 000026/000027) -- the same ServiceNow project "type" reference field
// sn_project_service.go's own snTypeNameToSubscriptionType converts, mirrored
// here as projectTypeNameToSubscriptionType for this data source (see that
// function's own doc comment). ClosureStatus and domain.ProjectAccountRef.Tier
// still have no corresponding column anywhere in the migrations (ClosureStatus
// is ServiceNow vocabulary -- e.g. "read_only" -- that doesn't match any of
// project's several different closure-state columns; account has no
// tier-like column at all), so they are left as their zero value rather than
// guessed at. AgentEnabled/KbReferencesEnabled DO have a clear real-column
// match (account.ai_gen_response_enabled/
// smart_knowledge_base_suggestions_enabled) despite the name difference and
// are populated from them.
type ProjectRepository interface {
	// SearchProjects returns a filtered, paginated slice of projects together
	// with the total count of matching rows before pagination, narrowed to
	// scope.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchProjects(ctx context.Context, req domain.SearchProjectsRequest, scope SearchScope) ([]domain.Project, int, error)
	// GetProjectByID returns the enriched project detail with the linked account,
	// or a NotFoundError if no such project exists OR it exists but scope
	// excludes it (existence is never revealed to a caller who can't see it).
	GetProjectByID(ctx context.Context, id string, scope SearchScope) (domain.ProjectDetailsView, error)
	// UpdateProject applies the subset of domain.ProjectUpdateRequest that has
	// a real Postgres column -- see service.pgProjectUpdateService's own doc
	// comment for exactly which fields and why. updatedBy is the caller's
	// resolved email, always written to project.updated_by (and, when
	// HasAgent/HasKbReferences is set, account.updated_by too). Returns a
	// NotFoundError if no such project exists, a ConflictError if
	// HasAgent/HasKbReferences is requested on a project with no linked
	// account (project.account_id IS NULL), or a ValidationError if a closure
	// sub-state value isn't a valid Postgres enum label for that column.
	UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest, updatedBy string) (domain.ProjectUpdateResult, error)
}

type projectRepo struct {
	db *pgxpool.Pool
}

// NewProjectRepository constructs a ProjectRepository backed by the given connection pool.
func NewProjectRepository(db *pgxpool.Pool) ProjectRepository {
	return &projectRepo{db: db}
}

// SearchProjects implements ProjectRepository.
func (r *projectRepo) SearchProjects(ctx context.Context, req domain.SearchProjectsRequest, scope SearchScope) ([]domain.Project, int, error) {
	filterArgs := []any{}
	argIdx := 1

	// p/pt aliases (rather than the unaliased "project" this query used
	// before project_type joined in) are required the moment a second table
	// with its own id/name columns is in scope -- every column reference
	// below is qualified accordingly, including inside scopePredicate/
	// SearchQuery's ILIKE, which used to be able to say plain "id"/"name".
	where := "WHERE 1=1"

	// See CaseRepository.SearchCases's identical scope clause for why this is
	// independent of any project filter the request itself may carry.
	if !scope.Unrestricted {
		where += " AND " + scopePredicate("p.id", argIdx)
		filterArgs = append(filterArgs, scope.ProjectIDs)
		argIdx++
	}

	if req.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (p.name ILIKE $%d ESCAPE '\\' OR p.key ILIKE $%d ESCAPE '\\')", argIdx, argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	// key (migration 000009) matches domain.SearchProjectsRequest.ExcludeProjectKeys
	// directly — same column SearchQuery's own ILIKE already matches against
	// above. Exact, case-sensitive per that field's own doc comment.
	if len(req.ExcludeProjectKeys) > 0 {
		where += fmt.Sprintf(" AND p.key <> ALL($%d::text[])", argIdx)
		filterArgs = append(filterArgs, req.ExcludeProjectKeys)
		argIdx++
	}

	// wso2_closure_state_enum's values ('OPEN', 'READ_ONLY', 'CLOSED',
	// 'RESTRICTED', 'SUSPENDED', migration 000009) are the same vocabulary as
	// ExcludeClosureStates' ServiceNow-sourced values ("Open"/"Suspended"/
	// "Restricted"), just differently cased, so this upper-cases the caller's
	// values rather than requiring them to match casing they have no way to
	// know. A NULL wso2_closure_state never matches any exclude value (a
	// project with no recorded closure state can't be excluded by one).
	if len(req.ExcludeClosureStates) > 0 {
		upper := make([]string, len(req.ExcludeClosureStates))
		for i, s := range req.ExcludeClosureStates {
			upper[i] = strings.ToUpper(s)
		}
		where += fmt.Sprintf(" AND (p.wso2_closure_state IS NULL OR p.wso2_closure_state::text <> ALL($%d::text[]))", argIdx)
		filterArgs = append(filterArgs, upper)
		argIdx++
	}

	// project_type.name (migrations 000026/000027, LEFT JOINed below via
	// p.project_type_id) holds the raw ServiceNow project "type" label (e.g.
	// "Cloud Support") -- normalized in SQL the same way
	// snTypeNameToSubscriptionType normalizes it in Go
	// (lower-cased, spaces to underscores) so it can be compared directly
	// against the caller's already-lowercase-underscore SubscriptionType
	// values without needing a reverse mapping back to ServiceNow's display
	// casing. A NULL pt.name (no project_type_id set, or an orphaned
	// reference) never matches any exclude value, same NULL-permissive
	// semantics as ExcludeClosureStates above. An unrecognized label (e.g.
	// "Regular", "Cloud Support - Platformer" -- both real project_type rows
	// with no SubscriptionType match) simply never equals any requested
	// value either, so it's never excluded by this filter -- consistent with
	// snTypeNameToSubscriptionType's own "never fails" resilience.
	if len(req.ExcludeSubscriptionTypes) > 0 {
		types := make([]string, len(req.ExcludeSubscriptionTypes))
		for i, t := range req.ExcludeSubscriptionTypes {
			types[i] = string(t)
		}
		where += fmt.Sprintf(" AND (pt.name IS NULL OR lower(replace(pt.name, ' ', '_')) <> ALL($%d::text[]))", argIdx)
		filterArgs = append(filterArgs, types)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM project p LEFT JOIN project_type pt ON pt.id = p.project_type_id " + where

	dataQuery := fmt.Sprintf(
		`SELECT p.id, p.account_id, p.sf_id, p.name, p.key, pt.name,
		        p.start_date, p.end_date, p.created_on, p.updated_on
		 FROM project p
		 LEFT JOIN project_type pt ON pt.id = p.project_type_id
		 %s
		 ORDER BY p.created_on DESC, p.id
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var projects []domain.Project

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count projects: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query projects: %w", err)
		}
		defer rows.Close()

		result := make([]domain.Project, 0, req.Pagination.Limit)
		for rows.Next() {
			var p domain.Project
			// account_id/start_date/end_date are nullable (migration
			// 000009); pt.name is nullable via the LEFT JOIN (no
			// project_type_id set, or set to a row that no longer exists).
			// domain.Project.SubscriptionType is a non-pointer field though
			// (its zero value, "", already means "unknown/unset" -- no
			// separate pointer needed the way AccountID/StartDate/EndDate
			// need one), so it's scanned into a *string temp var and
			// converted below rather than scanned directly. A non-pointer
			// scan here used to error "cannot scan NULL into *time.Time" the
			// moment any of the 13-14 (of 1956) rows with a NULL date
			// reached this query -- same class of bug this guards against
			// for pt.name too.
			var projectTypeName *string
			if err := rows.Scan(
				&p.ID, &p.AccountID, &p.SfID, &p.Name, &p.Key, &projectTypeName,
				&p.StartDate, &p.EndDate, &p.CreatedOn, &p.UpdatedOn,
			); err != nil {
				return fmt.Errorf("scan project: %w", err)
			}
			if projectTypeName != nil {
				p.SubscriptionType = projectTypeNameToSubscriptionType(*projectTypeName)
			}
			// ClosureStatus still has no real column -- see this
			// repository's own doc comment.
			result = append(result, p)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate projects: %w", err)
		}
		projects = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

// GetProjectByID implements ProjectRepository.
func (r *projectRepo) GetProjectByID(ctx context.Context, id string, scope SearchScope) (domain.ProjectDetailsView, error) {
	var v domain.ProjectDetailsView
	// account.ai_gen_response_enabled/smart_knowledge_base_suggestions_enabled
	// are nullable BOOLEAN columns, but ProjectAccountRef.AgentEnabled/
	// KbReferencesEnabled are plain (non-pointer) bool -- scan into *bool
	// and treat a NULL column as false, not an error.
	var agentEnabled, kbReferencesEnabled *bool
	// account is a LEFT JOIN, not an INNER JOIN: project.account_id
	// (migration 000009) is nullable and genuinely NULL on live data (14 of
	// 1956 rows) -- an INNER JOIN here used to make every such project
	// invisible (zero rows -> misreported as 404 "project not found"), the
	// same class of false-404 GetCaseByID had for its own optional joins
	// before that was fixed (see this file's "Case-like work_item types"
	// history). aID/aName are scanned nullable for the same reason;
	// ActivationDate/Region are already pointer fields on ProjectAccountRef
	// so they tolerate NULL (whether from a real account or a LEFT JOIN
	// producing no row at all) without a separate local var.
	var aID, aName *string
	// project_type is a LEFT JOIN for the same reason account is: a project
	// with no project_type_id set (or one pointing at a deleted row) must
	// still resolve, just with SubscriptionType left at its zero value below.
	var projectTypeName *string
	// Same "existence never revealed to a caller who can't see it" reasoning
	// as CaseRepository.GetCaseByID.
	scopeClause, scopeArgs := "", []any{id}
	if !scope.Unrestricted {
		scopeClause = " AND " + scopePredicate("p.id", 2)
		scopeArgs = append(scopeArgs, scope.ProjectIDs)
	}
	err := r.db.QueryRow(ctx,
		`SELECT p.id, p.sf_id, p.name, p.key,
		        p.start_date, p.end_date, p.created_on, p.updated_on,
		        a.id, a.name, a.activation_date, a.region,
		        a.ai_gen_response_enabled, a.smart_knowledge_base_suggestions_enabled,
		        pt.name
		 FROM project p
		 LEFT JOIN account a ON p.account_id = a.id
		 LEFT JOIN project_type pt ON pt.id = p.project_type_id
		 WHERE p.id = $1`+scopeClause, scopeArgs...,
	).Scan(
		&v.ID, &v.SfID, &v.Name, &v.Key,
		&v.StartDate, &v.EndDate, &v.CreatedOn, &v.UpdatedOn,
		&aID, &aName, &v.Account.ActivationDate, &v.Account.Region,
		&agentEnabled, &kbReferencesEnabled,
		&projectTypeName,
	)
	// v.Account.Tier still has no real column -- see this repository's own
	// doc comment; left at its zero value.
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProjectDetailsView{}, &apierror.NotFoundError{Msg: "project not found"}
	}
	if err != nil {
		return domain.ProjectDetailsView{}, fmt.Errorf("get project by id: %w", err)
	}
	if aID != nil {
		v.Account.ID = *aID
	}
	if aName != nil {
		v.Account.Name = *aName
	}
	v.Account.AgentEnabled = agentEnabled != nil && *agentEnabled
	v.Account.KbReferencesEnabled = kbReferencesEnabled != nil && *kbReferencesEnabled
	if projectTypeName != nil {
		v.SubscriptionType = projectTypeNameToSubscriptionType(*projectTypeName)
	}
	return v, nil
}

// UpdateProject implements ProjectRepository.
func (r *projectRepo) UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest, updatedBy string) (domain.ProjectUpdateResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.ProjectUpdateResult{}, fmt.Errorf("update project: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the row first (mirrors CaseRepository.UpdateCase's own
	// lock-before-read reasoning) so the account_id this reads is accurate
	// even under a concurrent update to the same project, and so a
	// nonexistent id is caught as NotFoundError before either UPDATE below
	// runs.
	var accountID *string
	err = tx.QueryRow(ctx, `SELECT account_id FROM project WHERE id = $1 FOR UPDATE`, id).Scan(&accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProjectUpdateResult{}, &apierror.NotFoundError{Msg: "project not found"}
	}
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "22P02" {
			return domain.ProjectUpdateResult{}, &apierror.ValidationError{Msg: "id is not a valid UUID: " + id}
		}
		return domain.ProjectUpdateResult{}, fmt.Errorf("update project: lock row: %w", err)
	}

	// HasAgent/HasKbReferences map onto the linked account's
	// ai_gen_response_enabled/smart_knowledge_base_suggestions_enabled
	// columns (migration 000008), the same mapping GetProjectByID already
	// reads from -- see this file's own doc comment. project.account_id is
	// nullable (14 of 1956 rows on live data, per GetProjectByID's own doc
	// comment), so a project with no linked account can't take these fields.
	if (req.HasAgent != nil || req.HasKbReferences != nil) && accountID == nil {
		return domain.ProjectUpdateResult{}, &apierror.ConflictError{Msg: "project has no linked account; hasAgent/hasKbReferences cannot be set"}
	}

	// project.updated_on/updated_by is bumped unconditionally on every
	// successful call (even one that only touches the linked account below)
	// so the response's UpdatedOn/UpdatedBy always reflects when this PATCH
	// happened, matching the ServiceNow-mode contract's own always-present
	// UpdatedOn/UpdatedBy. ClosureState is deliberately never selected or
	// returned here -- see pgProjectUpdateService.UpdateProject's own doc
	// comment for why this mode leaves wso2_closure_state alone rather than
	// reimplementing the ServiceNow business rule that derives it.
	var res domain.ProjectUpdateResult
	var endDateState, invoiceState, complianceState *string
	err = tx.QueryRow(ctx, `
		UPDATE project
		SET end_date_closure_state = COALESCE($2::end_date_closure_state_enum, end_date_closure_state),
		    invoice_due_date_closure_state = COALESCE($3::invoice_due_date_closure_state_enum, invoice_due_date_closure_state),
		    compliance_violation_closure_state = COALESCE($4::compliance_violation_closure_state_enum, compliance_violation_closure_state),
		    updated_on = NOW(),
		    updated_by = $5
		WHERE id = $1
		RETURNING id, updated_on, updated_by,
		          end_date_closure_state::TEXT, invoice_due_date_closure_state::TEXT, compliance_violation_closure_state::TEXT`,
		id, req.EndDateClosureState, req.InvoiceDueDateClosureState, req.ComplianceViolationClosureState, updatedBy,
	).Scan(&res.ID, &res.UpdatedOn, &res.UpdatedBy, &endDateState, &invoiceState, &complianceState)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "22P02" {
			// pgErr.Message (e.g. `invalid input value for enum
			// end_date_closure_state_enum: "Pending Notified"`) is logged in
			// full for debugging but never returned to the caller verbatim --
			// it embeds the internal enum type name, a schema implementation
			// detail this API's response shouldn't leak. The caller only
			// needs to know which of the three fields it sent was bad.
			slog.WarnContext(ctx, "update project: invalid enum value", "projectId", id, "error", pgErr.Message)
			return domain.ProjectUpdateResult{}, &apierror.ValidationError{Msg: "endDateClosureState, invoiceDueDateClosureState, or complianceViolationClosureState contains an unrecognized value"}
		}
		return domain.ProjectUpdateResult{}, fmt.Errorf("update project: %w", err)
	}
	res.EndDateClosureState = endDateState
	res.InvoiceDueDateClosureState = invoiceState
	res.ComplianceViolationClosureState = complianceState

	if req.HasAgent != nil || req.HasKbReferences != nil {
		// RowsAffected is checked, not just the error, because the account
		// row itself (not just the project row locked above) could be
		// deleted by a concurrent transaction between the project SELECT ...
		// FOR UPDATE and this UPDATE -- accountID is a stale reference at
		// that point, tx.Exec returns no error, and without this check the
		// transaction would commit as a silent partial success: caller gets
		// 200, hasAgent/hasKbReferences never actually changed.
		tag, err := tx.Exec(ctx, `
			UPDATE account
			SET ai_gen_response_enabled = COALESCE($2, ai_gen_response_enabled),
			    smart_knowledge_base_suggestions_enabled = COALESCE($3, smart_knowledge_base_suggestions_enabled),
			    updated_on = NOW(),
			    updated_by = $4
			WHERE id = $1`,
			*accountID, req.HasAgent, req.HasKbReferences, updatedBy,
		)
		if err != nil {
			return domain.ProjectUpdateResult{}, fmt.Errorf("update project: update linked account: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return domain.ProjectUpdateResult{}, fmt.Errorf("update project: linked account %s disappeared under transaction", *accountID)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ProjectUpdateResult{}, fmt.Errorf("update project: commit tx: %w", err)
	}

	// ClosureState/SuspensionProcessState stay nil -- see this method's own
	// doc comment (ClosureState) and pgProjectUpdateService.UpdateProject's
	// (SuspensionProcessState is rejected before this method is ever called).
	return res, nil
}

// projectTypeNameToSubscriptionType converts a project_type.name label (e.g.
// "Cloud Support", migrations 000026/000027) to the domain SubscriptionType
// enum (e.g. "cloud_support") -- the same transform
// sn_project_service.go's snTypeNameToSubscriptionType applies to the same
// underlying ServiceNow field, duplicated here rather than shared because
// the repository package cannot import the service package (the reverse
// import already exists). Never fails, same as that function: an
// unrecognized label (a real project_type row like "Regular" or "Cloud
// Support - Platformer" with no SubscriptionType match, or a future label
// added on the ServiceNow side) still returns a best-effort derived value
// instead of erroring -- callers that filter against a known
// SubscriptionType value simply never match it, rather than the whole query
// failing.
func projectTypeNameToSubscriptionType(name string) domain.SubscriptionType {
	return domain.SubscriptionType(strings.ToLower(strings.ReplaceAll(name, " ", "_")))
}
