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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// CaseTypeRefs is the fixed set of case types (the case-like work_item types),
// as the id/name pairs project metadata and search results both use. Order
// matches migrations/000016_work_item_table.up.sql's work_item_type_enum,
// restricted to the case-like subset.
var CaseTypeRefs = []domain.ReferenceTableItem{
	{ID: "case", Name: "Case"},
	{ID: "engagement", Name: "Engagement"},
	{ID: "security_report_analysis", Name: "Security Report Analysis"},
	{ID: "service_request", Name: "Service Request"},
	{ID: "announcement", Name: "Announcement"},
}

func caseTypeRef(workItemType string) domain.ReferenceTableItem {
	id := strings.ToLower(workItemType)
	for _, t := range CaseTypeRefs {
		if t.ID == id {
			return t
		}
	}
	return domain.ReferenceTableItem{ID: id, Name: workItemType}
}

// SearchScope restricts a search to some projects. Unrestricted means all of
// them; otherwise only ProjectIDs, and an empty list matches nothing (it is
// never treated as "no filter").
//
// ViewerEmail is the resolved caller's own email (populated alongside
// ProjectIDs in AccessService.scopeForUser) -- set here rather than
// re-derived from auth.IdentityFromContext at the repository layer, so
// identity resolution stays in the one place resolveScopeForID's own doc
// comment already designates for it. Only announcement-visibility reads
// (case_repo.go's setAnnouncementVisibility) currently use it; every other
// scoped query still only reads Unrestricted/ProjectIDs. It is left empty
// for an Unrestricted caller resolved from an internal client credential
// with no attached user token -- safe, since Unrestricted alone already
// grants that path full access regardless of email.
type SearchScope struct {
	Unrestricted bool
	ProjectIDs   []string
	ViewerEmail  string
}

// scopePredicate is the single place the "row belongs to one of the caller's
// projects" SQL is spelled. Every scoped query (global search, project/case
// search, project/case by id) builds its scope clause through it, so a future
// change to how scope is matched -- e.g. how an empty ProjectIDs is handled --
// lands once instead of in each hand-written copy. column is the project-id
// column to match (a trusted, code-supplied identifier, never user input);
// argIdx is the placeholder position the caller binds scope.ProjectIDs to.
func scopePredicate(column string, argIdx int) string {
	return fmt.Sprintf("%s = ANY($%d::text[]::uuid[])", column, argIdx)
}

// SearchSortField is a validated sort key for global search.
type SearchSortField string

const (
	SearchSortName      SearchSortField = "name"
	SearchSortCreatedOn SearchSortField = "createdOn"
	SearchSortUpdatedOn SearchSortField = "updatedOn"
)

// GlobalSearchRepository backs POST /search on the Postgres data source.
type GlobalSearchRepository interface {
	// SearchProjects returns projects within scope whose name or key contains
	// query (all of them when query is empty), a page at a time, with the total
	// before pagination.
	SearchProjects(ctx context.Context, scope SearchScope, query string, sortBy SearchSortField, desc bool, pagination domain.Pagination) ([]domain.GlobalSearchProject, int, error)
	// SearchCases does the same for case-like work items within scope, matching
	// number, subject, WSO2 id and description.
	SearchCases(ctx context.Context, scope SearchScope, query string, sortBy SearchSortField, desc bool, pagination domain.Pagination) ([]domain.GlobalSearchCase, int, error)
}

type globalSearchRepo struct {
	db *pgxpool.Pool
}

// NewGlobalSearchRepository constructs a GlobalSearchRepository backed by the given connection pool.
func NewGlobalSearchRepository(db *pgxpool.Pool) GlobalSearchRepository {
	return &globalSearchRepo{db: db}
}

// containsPattern turns user text into an ILIKE "contains" pattern with LIKE
// metacharacters escaped (ESCAPE '\').
func containsPattern(q string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
}

// searchFilter accumulates a WHERE clause and its bound args.
type searchFilter struct {
	where string
	args  []any
}

func (f *searchFilter) add(clause string, val any) {
	f.args = append(f.args, val)
	f.where += fmt.Sprintf(" AND "+clause, len(f.args))
}

func (f *searchFilter) scope(column string, scope SearchScope) {
	if !scope.Unrestricted {
		f.args = append(f.args, scope.ProjectIDs)
		f.where += " AND " + scopePredicate(column, len(f.args))
	}
}

func orderDirection(desc bool) string {
	if desc {
		return "DESC"
	}
	return "ASC"
}

// runSearch executes the count and page queries concurrently, each inside
// its own transaction with the caller's identity set via
// runWithCallerIdentity -- required so any table these queries touch that
// carries a caller-scoped row-level-security policy (currently just
// `announcement`, via SearchCases's join) is evaluated correctly. This
// applies unconditionally, including for SearchProjects, which doesn't
// currently need it: the cost is two trivial extra statements per query, and
// it means a future RLS policy on another table this function's callers
// might one day join against needs no further change here.
func runSearch[T any](ctx context.Context, db *pgxpool.Pool, scope SearchScope, countSQL, pageSQL string, f searchFilter, pagination domain.Pagination, scan func(pgx.Rows) (T, error)) ([]T, int, error) {
	pageArgs := append(append([]any{}, f.args...), pagination.Limit, pagination.Offset)
	pageSQL = fmt.Sprintf("%s LIMIT $%d OFFSET $%d", pageSQL, len(f.args)+1, len(f.args)+2)

	var total int
	out := make([]T, 0, pagination.Limit)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return runWithCallerIdentity(egCtx, db, scope, func(tx pgx.Tx) error {
			if err := tx.QueryRow(egCtx, countSQL, f.args...).Scan(&total); err != nil {
				return fmt.Errorf("count: %w", err)
			}
			return nil
		})
	})
	eg.Go(func() error {
		return runWithCallerIdentity(egCtx, db, scope, func(tx pgx.Tx) error {
			rows, err := tx.Query(egCtx, pageSQL, pageArgs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				v, err := scan(rows)
				if err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				out = append(out, v)
			}
			return rows.Err()
		})
	})
	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

var projectSortColumns = map[SearchSortField]string{
	SearchSortName:      "p.name",
	SearchSortCreatedOn: "p.created_on",
	SearchSortUpdatedOn: "p.updated_on",
}

// SearchProjects implements GlobalSearchRepository.
//
// activeChatsCount/actionRequiredCount/outstandingCount are left at 0: what
// they count is defined by ServiceNow-side logic with no Postgres definition
// yet. TODO: define and populate them.
func (r *globalSearchRepo) SearchProjects(ctx context.Context, scope SearchScope, query string, sortBy SearchSortField, desc bool, pagination domain.Pagination) ([]domain.GlobalSearchProject, int, error) {
	if !scope.Unrestricted && len(scope.ProjectIDs) == 0 {
		return []domain.GlobalSearchProject{}, 0, nil
	}

	f := searchFilter{where: "WHERE 1=1"}
	f.scope("p.id", scope)
	if query != "" {
		f.args = append(f.args, containsPattern(query))
		n := len(f.args)
		f.where += fmt.Sprintf(` AND (p.name ILIKE $%d ESCAPE '\' OR p.key ILIKE $%d ESCAPE '\')`, n, n)
	}

	col, ok := projectSortColumns[sortBy]
	if !ok {
		col = projectSortColumns[SearchSortName]
	}
	const from = `
		FROM project p
		LEFT JOIN project_type pt ON pt.id = p.project_type_id
		LEFT JOIN account a ON a.id = p.account_id`
	countSQL := `SELECT COUNT(*) FROM project p ` + f.where
	pageSQL := `SELECT p.id, p.name, p.key, p.description, p.created_on, p.start_date, p.end_date,
	                   p.is_pdp_subscription, p.wso2_closure_state::TEXT,
	                   pt.id, pt.name, a.id, a.name` + from + ` ` + f.where +
		fmt.Sprintf(` ORDER BY %s %s NULLS LAST, p.id`, col, orderDirection(desc))

	return runSearch(ctx, r.db, scope, countSQL, pageSQL, f, pagination, func(rows pgx.Rows) (domain.GlobalSearchProject, error) {
		var (
			p                   domain.GlobalSearchProject
			name, description   *string
			createdOn           time.Time
			startDate, endDate  *time.Time
			pdp                 *bool
			closure             *string
			ptID, ptName        *string
			accountID, acctName *string
		)
		if err := rows.Scan(&p.ID, &name, &p.Key, &description, &createdOn, &startDate, &endDate,
			&pdp, &closure, &ptID, &ptName, &accountID, &acctName); err != nil {
			return p, err
		}
		p.Name = stringOrEmpty(name)
		p.Description = description
		p.CreatedOn = createdOn.UTC().Format(time.RFC3339)
		p.StartDate = dateString(startDate)
		p.EndDate = dateString(endDate)
		p.HasPdpSubscription = pdp != nil && *pdp
		if closure != nil {
			c := strings.ToLower(*closure)
			p.ClosureState = &c
		}
		p.Type = domain.ReferenceTableItem{ID: stringOrEmpty(ptID), Name: stringOrEmpty(ptName)}
		p.Account = domain.ReferenceTableItem{ID: stringOrEmpty(accountID), Name: stringOrEmpty(acctName)}
		return p, nil
	})
}

var caseSortColumns = map[SearchSortField]string{
	SearchSortName:      "wi.subject",
	SearchSortCreatedOn: "wi.created_on",
	SearchSortUpdatedOn: "wi.updated_on",
}

// SearchCases implements GlobalSearchRepository. state and severity are the
// raw enum labels ("OPEN", "S2") used as both id and label, the same
// vocabulary project metadata offers for those lists. severity lives only on
// "case", so it is nil for the other case-like types.
func (r *globalSearchRepo) SearchCases(ctx context.Context, scope SearchScope, query string, sortBy SearchSortField, desc bool, pagination domain.Pagination) ([]domain.GlobalSearchCase, int, error) {
	if !scope.Unrestricted && len(scope.ProjectIDs) == 0 {
		return []domain.GlobalSearchCase{}, 0, nil
	}

	f := searchFilter{where: `WHERE wi.type = ANY(` + caseLikeWorkItemTypes + `)`}
	f.scope("wi.project_id", scope)
	// Without this, an ANNOUNCEMENT-typed row the caller can't see under
	// migration 000085's RLS policy would still surface here with its
	// subject/description intact (both live on the unprotected work_item
	// table) and only its state nulled out -- the exact leak this repo's
	// case_repo.go counterpart already guards against. A self-contained
	// EXISTS, not "ann.id IS NULL" (case_repo.go's version): countSQL below
	// has no announcement join at all to reference an ann alias against,
	// unlike pageSQL, so this must work standalone in both.
	f.where += ` AND NOT (wi.type = 'ANNOUNCEMENT' AND NOT EXISTS (SELECT 1 FROM announcement rls_ann WHERE rls_ann.id = wi.id))`
	if query != "" {
		f.args = append(f.args, containsPattern(query))
		n := len(f.args)
		f.where += fmt.Sprintf(` AND (wi.number ILIKE $%[1]d ESCAPE '\' OR wi.subject ILIKE $%[1]d ESCAPE '\'
			OR wi.wso2_id ILIKE $%[1]d ESCAPE '\' OR wi.description ILIKE $%[1]d ESCAPE '\')`, n)
	}

	col, ok := caseSortColumns[sortBy]
	if !ok {
		col = caseSortColumns[SearchSortUpdatedOn]
	}
	const from = `
		FROM work_item wi
		LEFT JOIN "case" c ON c.id = wi.id` + caseLikeJoins + `
		LEFT JOIN project p ON p.id = wi.project_id
		LEFT JOIN account a ON a.id = COALESCE(wi.account_id, p.account_id)
		LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id`
	countSQL := `SELECT COUNT(*) FROM work_item wi ` + f.where
	pageSQL := `SELECT wi.id, wi.wso2_id, wi.number, wi.subject, wi.description, wi.created_on, wi.created_by, wi.updated_on,
	                   p.id, p.name, wi.type::TEXT, ` + caseLikeStateColumn + `, c.severity::TEXT,
	                   ae.id, COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')),
	                   a.id, a.name` + from + ` ` + f.where +
		fmt.Sprintf(` ORDER BY %s %s NULLS LAST, wi.id`, col, orderDirection(desc))

	return runSearch(ctx, r.db, scope, countSQL, pageSQL, f, pagination, func(rows pgx.Rows) (domain.GlobalSearchCase, error) {
		var (
			cs                       domain.GlobalSearchCase
			internalID, title, descr *string
			createdOn, updatedOn     time.Time
			projID, projName         *string
			wiType, state, severity  *string
			engID, engName           *string
			accountID, accountName   *string
		)
		if err := rows.Scan(&cs.ID, &internalID, &cs.Number, &title, &descr, &createdOn, &cs.CreatedBy, &updatedOn,
			&projID, &projName, &wiType, &state, &severity,
			&engID, &engName, &accountID, &accountName); err != nil {
			return cs, err
		}
		cs.InternalID = stringOrEmpty(internalID)
		cs.Title = title
		cs.Description = descr
		cs.CreatedOn = createdOn.UTC().Format(time.RFC3339)
		cs.UpdatedOn = updatedOn.UTC().Format(time.RFC3339)
		if projID != nil {
			cs.Project = &domain.ReferenceTableItem{ID: *projID, Name: stringOrEmpty(projName)}
		}
		if wiType != nil {
			t := caseTypeRef(*wiType)
			cs.CaseType = &t
		}
		if state != nil {
			cs.State = &domain.ChoiceListItem{ID: *state, Label: *state}
		}
		if severity != nil {
			cs.Severity = &domain.ChoiceListItem{ID: *severity, Label: *severity}
		}
		if engID != nil {
			cs.AssignedEngineer = &domain.ReferenceTableItem{ID: *engID, Name: stringOrEmpty(engName)}
		}
		cs.Account = domain.ReferenceTableItem{ID: stringOrEmpty(accountID), Name: stringOrEmpty(accountName)}
		return cs, nil
	})
}

// dateString formats a DATE column as YYYY-MM-DD, or nil when NULL.
func dateString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}
