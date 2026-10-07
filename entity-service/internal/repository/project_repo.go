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
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ProjectRepository defines the persistence operations for the project
// table (migration 0014). domain.Project.SubscriptionType is populated
// from project.project_type_id's linked project_type.name (migrations
// 0031/0032) -- the same ServiceNow project "type" reference field
// sn_project_service.go's own snTypeNameToSubscriptionType converts, mirrored
// here as projectTypeNameToSubscriptionType for this data source (see that
// function's own doc comment). ClosureStatus still has no corresponding
// column anywhere in the migrations (it is ServiceNow vocabulary -- e.g.
// "read_only" -- that doesn't match any of project's several different
// closure-state columns), so it is left as its zero value rather than
// guessed at. AgentEnabled/KbReferencesEnabled DO have a clear real-column
// match (account.ai_gen_response_enabled/
// smart_knowledge_base_suggestions_enabled) despite the name difference and
// are populated from them. domain.ProjectAccountRef.Tier is populated from
// account.support_tier (migration 0101) -- see GetProjectByID's own
// comment for the enum-scan cast this column needs.
//
// GetProjectByID also now populates ClosureState/OnboardingStatus (from
// project.wso2_closure_state/onboarding_status, cast ::TEXT the same way
// UpdateProject's own closure-substate columns are), the onboarding date
// trio, and the six query/onboarding hour balances (from project's INTERVAL
// columns via EXTRACT(EPOCH FROM ...)/3600), plus
// ProjectAccountRef.TechnicalOwnerEmail/OwnerEmail (resolved from
// account.technical_owner_id/account_manager_id via a "user" join). See
// GetProjectByID's own query comment for the two mappings flagged as
// unconfirmed assumptions (ConsumedQueryHours <- consumed_duration, and
// OwnerEmail <- account_manager_id).
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
	// comment for exactly which fields and why. Closure sub-states may be Title
	// Case; changing one recomputes wso2_closure_state in the same transaction.
	// updatedBy is the caller's email or internal client id, always written to
	// project.updated_by (and, when HasAgent/HasKbReferences is set,
	// account.updated_by too). Returns a
	// NotFoundError if no such project exists, a ConflictError if
	// HasAgent/HasKbReferences is requested on a project with no linked
	// account (project.account_id IS NULL), or a ValidationError if a closure
	// sub-state value isn't a valid Postgres enum label for that column.
	UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest, updatedBy string) (domain.ProjectUpdateResult, error)
}

type projectRepo struct {
	db *Scoped
}

// NewProjectRepository constructs a ProjectRepository backed by the given scoped connection pool.
func NewProjectRepository(db *Scoped) ProjectRepository {
	return &projectRepo{db: db}
}

// projectClosureColumns selects the overall and sub closure states in ServiceNow's
// Title Case ("Pending Notified"), the compliance violation date as yyyy-MM-dd, then ACP's suspension state.
const projectClosureColumns = `INITCAP(REPLACE(p.wso2_closure_state::TEXT, '_', ' ')),
	INITCAP(REPLACE(p.end_date_closure_state::TEXT, '_', ' ')),
	INITCAP(REPLACE(p.invoice_due_date_closure_state::TEXT, '_', ' ')),
	INITCAP(REPLACE(p.compliance_violation_closure_state::TEXT, '_', ' ')),
	TO_CHAR(p.compliance_violation_date, 'YYYY-MM-DD'),
	p.suspension_process_state`

// accountIsPartnerColumn is NULL without a linked account (alias a), else the
// same classification rule the Salesforce membership mapping uses.
const accountIsPartnerColumn = `CASE WHEN a.id IS NULL THEN NULL
	ELSE COALESCE(LOWER(TRIM(a.classification)) = 'partner', FALSE) END`

// projectSearchOrderBy builds ORDER BY from a whitelist; the service has already
// rejected any other sortBy/sortOrder, so raw input never reaches the SQL.
func projectSearchOrderBy(sortBy, sortOrder string) string {
	if sortBy == "endDate" {
		if sortOrder == "desc" {
			return "p.end_date DESC NULLS LAST, p.id"
		}
		return "p.end_date ASC NULLS LAST, p.id"
	}
	if sortOrder == "asc" {
		return "p.created_on ASC, p.id"
	}
	return "p.created_on DESC, p.id"
}

// buildProjectSearchWhere renders SearchProjects' WHERE clause and its bound
// arguments. It expects aliases p (project), pt (project_type) and a (account).
func buildProjectSearchWhere(req domain.SearchProjectsRequest, scope SearchScope) (string, []any, int, error) {
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

	// key (migration 0014) matches domain.SearchProjectsRequest.ExcludeProjectKeys
	// directly — same column SearchQuery's own ILIKE already matches against
	// above. Exact, case-sensitive per that field's own doc comment.
	if len(req.ExcludeProjectKeys) > 0 {
		where += fmt.Sprintf(" AND p.key <> ALL($%d::text[])", argIdx)
		filterArgs = append(filterArgs, req.ExcludeProjectKeys)
		argIdx++
	}

	// wso2_closure_state_enum's values ('OPEN', 'READ_ONLY', 'CLOSED',
	// 'RESTRICTED', 'SUSPENDED', migration 0014) are the same vocabulary as
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

	// project_type.name (migrations 0031/0032, LEFT JOINed below via
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

	// AccountID was previously documented "ServiceNow data source only" even
	// though project.account_id (migration 000009) is a plain FK already
	// selected/returned by this same query below -- this is what actually
	// applies it as a filter for the Postgres data source too. The service
	// layer validates it's a UUID before this point (project_service.go).
	if req.AccountID != "" {
		where += fmt.Sprintf(" AND p.account_id = $%d::uuid", argIdx)
		filterArgs = append(filterArgs, req.AccountID)
		argIdx++
	}

	// The service validates ClosureStatus as Open/Suspended/Restricted, matching ServiceNow.
	if req.ClosureStatus != "" {
		where += fmt.Sprintf(" AND p.wso2_closure_state::text = $%d", argIdx)
		filterArgs = append(filterArgs, strings.ToUpper(req.ClosureStatus))
		argIdx++
	}

	// endDateFrom/endDateTo: inclusive yyyy-MM-dd bounds (validated by the
	// service); a project with no end date never matches a bound.
	if req.EndDateFrom != "" {
		where += fmt.Sprintf(" AND p.end_date >= $%d::date", argIdx)
		filterArgs = append(filterArgs, req.EndDateFrom)
		argIdx++
	}
	if req.EndDateTo != "" {
		where += fmt.Sprintf(" AND p.end_date <= $%d::date", argIdx)
		filterArgs = append(filterArgs, req.EndDateTo)
		argIdx++
	}

	// onboardingStatus: any of the given ServiceNow labels, compared ignoring
	// case and separators, as the case search's projectOnboardingStatus does.
	if len(req.OnboardingStatus) > 0 {
		labels, err := onboardingStatusEnumLabels("onboardingStatus", req.OnboardingStatus)
		if err != nil {
			return "", nil, argIdx, err
		}
		where += fmt.Sprintf(" AND p.onboarding_status = ANY($%d::text[]::onboarding_status_enum[])", argIdx)
		filterArgs = append(filterArgs, labels)
		argIdx++
	}

	// subRegion: the linked account's sub-region, exact but case-insensitive.
	if sub := strings.TrimSpace(req.SubRegion); sub != "" {
		where += fmt.Sprintf(" AND LOWER(TRIM(a.sub_region)) = LOWER($%d)", argIdx)
		filterArgs = append(filterArgs, sub)
		argIdx++
	}

	return where, filterArgs, argIdx, nil
}

// SearchProjects implements ProjectRepository.
func (r *projectRepo) SearchProjects(ctx context.Context, req domain.SearchProjectsRequest, scope SearchScope) ([]domain.Project, int, error) {
	// WithCallerIdentity from the explicit scope parameter -- see
	// CaseRepository.GetCaseByID's identical stamp. The per-project case count
	// below reads work_item (RLS-protected), so it is counted through the same
	// identity that scopes the project list: all of it for an internal caller,
	// only what a customer's own project membership lets them see otherwise.
	ctx = WithCallerIdentity(ctx, scope)
	where, filterArgs, argIdx, err := buildProjectSearchWhere(req, scope)
	if err != nil {
		return nil, 0, err
	}

	countQuery := "SELECT COUNT(*) FROM project p LEFT JOIN project_type pt ON pt.id = p.project_type_id LEFT JOIN account a ON a.id = p.account_id " + where

	dataQuery := fmt.Sprintf(
		`SELECT p.id, p.account_id, p.sf_id, p.name, p.key, pt.name,
		        p.start_date, p.end_date, p.created_on, p.updated_on,
		        `+projectClosureColumns+`,
		        INITCAP(REPLACE(p.onboarding_status::TEXT, '_', '-')),
		        a.id, a.name, a.region, a.sub_region, `+accountIsPartnerColumn+`,
		        (SELECT COUNT(*) FROM work_item wi
		           LEFT JOIN "case" c ON c.id = wi.id`+caseLikeJoins+`
		          WHERE wi.project_id = p.id
		            AND wi.type = ANY(`+caseLikeWorkItemTypes+`)
		            AND `+caseLikeStateColumn+` IS DISTINCT FROM 'CLOSED')
		 FROM project p
		 LEFT JOIN project_type pt ON pt.id = p.project_type_id
		 LEFT JOIN account a ON a.id = p.account_id
		 %s
		 ORDER BY %s
		 LIMIT $%d OFFSET $%d`,
		where, projectSearchOrderBy(req.SortBy, req.SortOrder), argIdx, argIdx+1,
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
			// sf_id is NOT NULL per migration 0014, but real data has since
			// proven that constraint isn't actually enforced (the same gap
			// GetCaseByID's own InternalID doc comment describes for
			// wso2_id) -- a non-pointer scan here panicked "cannot scan NULL
			// into *string" the moment such a row reached this query. Scanned
			// into a nullable temp var and defaulted to "" below rather than
			// widening domain.Project.SfID to *string, so this stays a
			// narrow fix at the one place real data violates the schema's
			// own declared constraint, not a wider contract change every
			// other reader of Project.SfID would also have to handle.
			var sfID, aID, aName *string
			var acct domain.ProjectSearchAccountRef
			if err := rows.Scan(
				&p.ID, &p.AccountID, &sfID, &p.Name, &p.Key, &projectTypeName,
				&p.StartDate, &p.EndDate, &p.CreatedOn, &p.UpdatedOn,
				&p.ClosureState, &p.EndDateClosureState, &p.InvoiceDueDateClosureState,
				&p.ComplianceViolationClosureState, &p.ComplianceViolationDate, &p.SuspensionProcessState,
				&p.OnboardingStatus,
				&aID, &aName, &acct.Region, &acct.SubRegion, &acct.IsPartner,
				&p.ActiveCasesCount,
			); err != nil {
				return fmt.Errorf("scan project: %w", err)
			}
			if sfID != nil {
				p.SfID = *sfID
			}
			if aID != nil {
				acct.ID, acct.Name = *aID, stringOrEmpty(aName)
				p.Account = &acct
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
	// WithCallerIdentity from the explicit scope parameter, as in SearchProjects.
	ctx = WithCallerIdentity(ctx, scope)
	var v domain.ProjectDetailsView
	// account.ai_gen_response_enabled/smart_knowledge_base_suggestions_enabled
	// are nullable BOOLEAN columns, but ProjectAccountRef.AgentEnabled/
	// KbReferencesEnabled are plain (non-pointer) bool -- scan into *bool
	// and treat a NULL column as false, not an error.
	var agentEnabled, kbReferencesEnabled *bool
	// account is a LEFT JOIN, not an INNER JOIN: project.account_id
	// (migration 0014) is nullable and genuinely NULL on live data (14 of
	// 1956 rows) -- an INNER JOIN here used to make every such project
	// invisible (zero rows -> misreported as 404 "project not found"), the
	// same class of false-404 GetCaseByID had for its own optional joins
	// before that was fixed (see this file's "Case-like work_item types"
	// history). aID/aName are scanned nullable for the same reason;
	// ActivationDate/Region are already pointer fields on ProjectAccountRef
	// so they tolerate NULL (whether from a real account or a LEFT JOIN
	// producing no row at all) without a separate local var.
	var aID, aName *string
	// aNumber is the same class of already-present-but-unselected gap
	// account_repo.go's own comment describes for account.number -- kept as
	// its own local var (rather than folded into aID/aName above) since only
	// this one column needed the fix, not the whole account.* group.
	var aNumber *string
	// account.support_tier (migration 0101) is support_tier_enum, not a plain
	// VARCHAR -- like every other enum column this repository package scans
	// into a Go string (see e.g. this file's wso2_closure_state/
	// onboarding_status casts above), it needs its own ::TEXT cast in the
	// query below or the scan fails with no custom enum types registered in
	// this connection's pgx type map. ProjectAccountRef.Tier is a plain
	// (non-pointer) string, so this scans into a *string local and defaults
	// to "" via stringOrEmpty, same pattern as agentEnabled/kbReferencesEnabled
	// above.
	var supportTier *string
	// project_type is a LEFT JOIN for the same reason account is: a project
	// with no project_type_id set (or one pointing at a deleted row) must
	// still resolve, just with SubscriptionType/HasSr left at their zero
	// value below.
	var projectTypeName *string
	// sf_id is nullable in production data (migration 0095 dropped NOT NULL).
	var sfID *string
	// has_service_request_write_access (migration 0130) is a direct port of
	// ServiceNow's ProjectTypeFeatureManager.FEATURE_MATRIX (see
	// reference_data_repo.go's own doc comment) -- reusing it here is what
	// makes HasSr answer the same question on this data source that
	// ServiceNow's own ProjectDeploymentClassification.isSREnabledForProject
	// answers on that one, keyed off the same project type, rather than
	// leaving HasSr at its Go zero value (false) for every project
	// regardless of type, which is what this data source used to do.
	var hasSr *bool
	// tou/amu: this view's Account.OwnerEmail/TechnicalOwnerEmail were
	// previously left at their zero value unconditionally -- both are real
	// columns' worth of data, just not this project's own; they're the
	// linked account's own technical owner and account manager.
	//
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
		        a.id, a.name, a.number, a.activation_date, a.region,
		        a.ai_gen_response_enabled, a.smart_knowledge_base_suggestions_enabled,
		        a.support_tier::TEXT,
		        pt.name, pt.has_service_request_write_access,
		        -- wso2_closure_state/onboarding_status (migration 0014) are stored
		        -- SCREAMING_SNAKE_CASE ('SUSPENDED', 'NOT_STARTED'), but the documented
		        -- response vocabulary isn't -- and the two don't even share a separator:
		        -- ClosureState is space-separated Title Case ("Suspended", "Read Only"),
		        -- OnboardingStatus is hyphen-separated ("Not-Started", "In-Progress"),
		        -- confirmed against a real ServiceNow payload and Postgres's own INITCAP
		        -- behavior for both before picking these. A bare ::TEXT cast leaves both
		        -- uppercase, matching neither.
		        `+projectClosureColumns+`,
		        INITCAP(REPLACE(p.onboarding_status::TEXT, '_', '-')),
		        `+accountIsPartnerColumn+`,
		        p.onboarding_go_live_plan_date, p.onboarding_go_live_date, p.onboarding_expiry_date,
		        EXTRACT(EPOCH FROM p.total_query_duration) / 3600,
		        EXTRACT(EPOCH FROM p.remaining_query_duration) / 3600,
		        -- ASSUMPTION, not confirmed against a live payload: project.consumed_duration
		        -- has no "query" in its name, but it sits in the csm-sync repo's SN mapping
		        -- file right next to total_query_duration/remaining_query_duration (mapped
		        -- from ServiceNow's u_consumed_hours, next to u_total_query_hour/
		        -- u_remaining_query_hours -- the onboarding trio below is separately and
		        -- explicitly named u_total_onboarding_hours etc). Treated here as
		        -- ConsumedQueryHours on that basis. Verify against a real payload before
		        -- trusting it further.
		        EXTRACT(EPOCH FROM p.consumed_duration) / 3600,
		        EXTRACT(EPOCH FROM p.total_onboarding_duration) / 3600,
		        EXTRACT(EPOCH FROM p.consumed_onboarding_duration) / 3600,
		        EXTRACT(EPOCH FROM p.remaining_onboarding_duration) / 3600,
		        tou.email, amu.email
		 FROM project p
		 LEFT JOIN account a ON p.account_id = a.id
		 LEFT JOIN project_type pt ON pt.id = p.project_type_id
		 -- account.technical_owner_id/account_manager_id (migration 0012) are UUID FKs
		 -- into "user"(id); resolved to email here the same way deployment_repo.go/
		 -- other repos in this file resolve a *_by column to a display value.
		 -- ASSUMPTION, not confirmed against a live payload: account_manager_id is
		 -- mapped to ProjectAccountRef.OwnerEmail ("the account owner") purely from field
		 -- naming. account also has customer_success_manager_id/technical_owner_id/
		 -- secondary_technical_owner_id/renewal_account_manager_id, any of which could
		 -- plausibly be "owner" -- sn_project_service.go's own OwnerEmail is a bare
		 -- passthrough of Ballerina's snProjectAccount.OwnerEmail with no further
		 -- ServiceNow field name recorded in this codebase to confirm the mapping
		 -- against. Verify against a real payload before trusting it further.
		 LEFT JOIN "user" tou ON tou.id = a.technical_owner_id
		 LEFT JOIN "user" amu ON amu.id = a.account_manager_id
		 WHERE p.id = $1`+scopeClause, scopeArgs...,
	).Scan(
		&v.ID, &sfID, &v.Name, &v.Key,
		&v.StartDate, &v.EndDate, &v.CreatedOn, &v.UpdatedOn,
		&aID, &aName, &aNumber, &v.Account.ActivationDate, &v.Account.Region,
		&agentEnabled, &kbReferencesEnabled,
		&supportTier,
		&projectTypeName, &hasSr,
		&v.ClosureState, &v.EndDateClosureState, &v.InvoiceDueDateClosureState,
		&v.ComplianceViolationClosureState, &v.ComplianceViolationDate, &v.SuspensionProcessState,
		&v.OnboardingStatus, &v.Account.IsPartner,
		&v.GoLivePlanDate, &v.GoLiveDate, &v.OnboardingExpiryDate,
		&v.TotalQueryHours, &v.RemainingQueryHours,
		&v.ConsumedQueryHours,
		&v.TotalOnboardingHours, &v.ConsumedOnboardingHours, &v.RemainingOnboardingHours,
		&v.Account.TechnicalOwnerEmail, &v.Account.OwnerEmail,
	)
	// v.Account.Tier is sourced from account.support_tier (migration 0101).
	// ServiceNow's own u_support_tier/u_support_timezone fields are swapped on
	// the production tenant (a historical relabeling bug); the sync's own
	// mapping already corrects that per-environment before the value ever
	// reaches this column (source_by_env in
	// operations/csm-sync-service/configs/mappings/customer_account.yaml), so
	// this column holds the real tier by the time it's read here -- no
	// swap-awareness needed on this side.
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
	if aNumber != nil {
		v.Account.Number = *aNumber
	}
	v.SfID = stringOrEmpty(sfID)
	v.Account.AgentEnabled = agentEnabled != nil && *agentEnabled
	v.Account.KbReferencesEnabled = kbReferencesEnabled != nil && *kbReferencesEnabled
	v.Account.Tier = stringOrEmpty(supportTier)
	if projectTypeName != nil {
		v.SubscriptionType = projectTypeNameToSubscriptionType(*projectTypeName)
	}
	v.HasSr = hasSr != nil && *hasSr
	return v, nil
}

// UpdateProject implements ProjectRepository.
func (r *projectRepo) UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest, updatedBy string) (domain.ProjectUpdateResult, error) {
	if err := validateClosureFields(req); err != nil {
		return domain.ProjectUpdateResult{}, err
	}
	// WithSystemIdentity: this writes only project and account, neither
	// RLS-protected, and carries no caller-visible rows. The service already
	// authorized the caller (resolveUpdatedBy); this only gives Scoped the
	// identity it requires, including for an allow-listed client the identity
	// middleware could not resolve to a scope.
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (domain.ProjectUpdateResult, error) {
		return updateProjectTx(ctx, tx, id, req, updatedBy)
	})
}

// updateProjectTx is UpdateProject's body, run inside tx.
func updateProjectTx(ctx context.Context, tx pgx.Tx, id string, req domain.ProjectUpdateRequest, updatedBy string) (domain.ProjectUpdateResult, error) {
	// Lock the row first (mirrors CaseRepository.UpdateCase's own
	// lock-before-read reasoning) so the account_id this reads is accurate
	// even under a concurrent update to the same project, and so a
	// nonexistent id is caught as NotFoundError before either UPDATE below
	// runs.
	var accountID *string
	err := tx.QueryRow(ctx, `SELECT account_id FROM project WHERE id = $1 FOR UPDATE`, id).Scan(&accountID)
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
	// columns (migration 0012), the same mapping GetProjectByID already
	// reads from -- see this file's own doc comment. project.account_id is
	// nullable (14 of 1956 rows on live data, per GetProjectByID's own doc
	// comment), so a project with no linked account can't take these fields.
	if (req.HasAgent != nil || req.HasKbReferences != nil) && accountID == nil {
		return domain.ProjectUpdateResult{}, &apierror.ConflictError{Msg: "project has no linked account; hasAgent/hasKbReferences cannot be set"}
	}

	// project.updated_on/updated_by is bumped on every successful call, even one
	// that only touches the linked account below.
	var res domain.ProjectUpdateResult
	err = tx.QueryRow(ctx, `
		UPDATE project
		SET end_date_closure_state = COALESCE($2::end_date_closure_state_enum, end_date_closure_state),
		    invoice_due_date_closure_state = COALESCE($3::invoice_due_date_closure_state_enum, invoice_due_date_closure_state),
		    compliance_violation_closure_state = COALESCE($4::compliance_violation_closure_state_enum, compliance_violation_closure_state),
		    suspension_process_state = COALESCE($6::jsonb, suspension_process_state),
		    updated_on = NOW(),
		    updated_by = $5
		WHERE id = $1
		RETURNING id, updated_on, updated_by`,
		id, closureEnumLabel(req.EndDateClosureState), closureEnumLabel(req.InvoiceDueDateClosureState),
		closureEnumLabel(req.ComplianceViolationClosureState), updatedBy, suspensionStateArg(req.SuspensionProcessState),
	).Scan(&res.ID, &res.UpdatedOn, &res.UpdatedBy)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "22P02" {
			// The enum type name in pgErr.Message is logged, never returned.
			slog.WarnContext(ctx, "update project: invalid enum value", "projectId", id, "error", pgErr.Message)
			return domain.ProjectUpdateResult{}, &apierror.ValidationError{Msg: "endDateClosureState, invoiceDueDateClosureState, or complianceViolationClosureState contains an unrecognized value"}
		}
		return domain.ProjectUpdateResult{}, fmt.Errorf("update project: %w", err)
	}

	// Recompute the overall state in this tx, standing in for ServiceNow's
	// "Update WSO2 Closure State" business rule.
	if req.EndDateClosureState != nil || req.InvoiceDueDateClosureState != nil || req.ComplianceViolationClosureState != nil {
		if _, err := tx.Exec(ctx, `UPDATE project SET wso2_closure_state = `+wso2ClosureStateRule+` WHERE id = $1`, id); err != nil {
			return domain.ProjectUpdateResult{}, fmt.Errorf("update project: recompute closure state: %w", err)
		}
	}
	var complianceDate *string
	err = tx.QueryRow(ctx, `SELECT `+projectClosureColumns+` FROM project p WHERE p.id = $1`, id).Scan(
		&res.ClosureState, &res.EndDateClosureState, &res.InvoiceDueDateClosureState,
		&res.ComplianceViolationClosureState, &complianceDate, &res.SuspensionProcessState,
	)
	if err != nil {
		return domain.ProjectUpdateResult{}, fmt.Errorf("update project: read closure states: %w", err)
	}

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

	return res, nil
}

// suspensionStateArg passes the object as text so pgx sends it as jsonb input; nil keeps the stored value.
func suspensionStateArg(v json.RawMessage) *string {
	if v == nil {
		return nil
	}
	s := string(v)
	return &s
}

// wso2ClosureStateRule derives project.wso2_closure_state from the three sub-states.
// It matches 1,939 of 1,940 dev rows that carry a closure state.
const wso2ClosureStateRule = `(CASE
	WHEN end_date_closure_state::TEXT IN ('CLOSED', 'SUSPENDED')
	  OR invoice_due_date_closure_state::TEXT LIKE 'SUSPENDED%'
	  OR compliance_violation_closure_state::TEXT = 'SUSPENDED' THEN 'SUSPENDED'
	WHEN end_date_closure_state::TEXT = 'RESTRICTED'
	  OR invoice_due_date_closure_state::TEXT LIKE 'RESTRICTED%' THEN 'RESTRICTED'
	ELSE 'OPEN' END)::wso2_closure_state_enum`

// closureEnumLabel maps a ServiceNow closure value ("Pending Notified") to its enum
// label (PENDING_NOTIFIED), the same snake_upper rule csm-sync applies.
func closureEnumLabel(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.ReplaceAll(strings.ToLower(*v), "&", " and ")
	s = strings.Trim(nonAlnumRun.ReplaceAllString(s, "_"), "_")
	s = strings.ToUpper(s)
	return &s
}

var nonAlnumRun = regexp.MustCompile(`[^a-z0-9]+`)

// Enum labels of the three closure sub-state columns (migrations 0014, 0175).
var (
	endDateClosureLabels = []string{"OPEN", "NOTIFIED", "CLOSURE_NOTICES", "RESTRICTED", "CLOSED", "SUSPENDED",
		"PENDING_NOTIFIED", "PENDING_CLOSURE_NOTICES", "PENDING_CLOSED", "PENDING_RESTRICTED"}
	invoiceDueDateClosureLabels = []string{"OPEN", "NOTIFIED", "NOTICED", "RESTRICTED", "SUSPENDED",
		"PENDING_NOTIFIED", "PENDING_SUSPENDED", "PENDING_NOTICED", "PENDING_RESTRICTED",
		"NOTIFIED_AND_PREVIOUSLY_PAID", "NOTICED_AND_PREVIOUSLY_PAID", "RESTRICTED_AND_PREVIOUSLY_PAID",
		"SUSPENDED_AND_PREVIOUSLY_PAID", "PENDING_NOTIFIED_AND_PREVIOUSLY_PAID", "PENDING_NOTICED_AND_PREVIOUSLY_PAID",
		"PENDING_RESTRICTED_AND_PREVIOUSLY_PAID", "PENDING_SUSPENDED_AND_PREVIOUSLY_PAID"}
	complianceViolationClosureLabels = []string{"OPEN", "SUSPENDED"}
)

// validateClosureFields rejects a PATCH closure value with no enum label,
// naming the value and the accepted values in the Title Case reads return.
func validateClosureFields(req domain.ProjectUpdateRequest) error {
	fields := []struct {
		name   string
		value  *string
		labels []string
	}{
		{"endDateClosureState", req.EndDateClosureState, endDateClosureLabels},
		{"invoiceDueDateClosureState", req.InvoiceDueDateClosureState, invoiceDueDateClosureLabels},
		{"complianceViolationClosureState", req.ComplianceViolationClosureState, complianceViolationClosureLabels},
	}
	for _, f := range fields {
		if f.value == nil || slices.Contains(f.labels, *closureEnumLabel(f.value)) {
			continue
		}
		names := make([]string, len(f.labels))
		for i, l := range f.labels {
			names[i] = closureDisplayName(l)
		}
		sort.Strings(names)
		return apierror.InvalidValue(f.name, *f.value, "closure state", names)
	}
	return nil
}

// closureDisplayName renders an enum label the way reads return it:
// PENDING_NOTIFIED -> "Pending Notified".
func closureDisplayName(label string) string {
	words := strings.Split(strings.ToLower(label), "_")
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// projectTypeNameToSubscriptionType converts a project_type.name label (e.g.
// "Cloud Support", migrations 0031/0032) to the domain SubscriptionType
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
