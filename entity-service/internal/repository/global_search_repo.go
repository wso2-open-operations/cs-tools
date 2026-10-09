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
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// CaseTypeRefs is the fixed set of case types (the case-like work_item types),
// as the id/name pairs project metadata and search results both use. Order
// matches migrations/0021_work_item_table.sql's work_item_type_enum,
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
	// HasInternalAccess is true whenever the caller's email has an active
	// INTERNAL "user" row, even when Unrestricted is false because the same
	// email ALSO has an active EXTERNAL row (accessService.scopeForUser's
	// own "external wins" rule for data-visibility scoping -- "less access,
	// never more, when the data is ambiguous"). That rule is about which
	// projects/cases a caller may LIST, a different question from "is this
	// person WSO2 staff" -- a caller this field is true for is still legitimate
	// internal staff and must not be treated as an external customer by
	// callers asking that second question (see caseService.UpdateCase's own
	// resolution-fields requirement, the one place this is read as of this
	// field's introduction). Never true from an Unrestricted:true internal-
	// client-id/system-identity scope -- those paths have no "user" row to
	// check at all; callers that also want to treat such a caller as internal
	// should check Unrestricted separately, as UpdateCase does.
	HasInternalAccess bool
	// ViaCustomerPortal is true when the request came from the customer portal's
	// backend (AccessClientConfig.CustomerPortalBackendClientID), whoever the signed-in
	// user is -- including WSO2 staff looking at a customer's project. The data scope
	// is unchanged (a staff user is still Unrestricted); what it changes is which
	// numbers a card shows: the customer portal's lists show every change request the
	// caller can see, so its change-request counts use the customer grouping
	// (crOutstandingStatesFor) for staff too, or a staff user's card and list disagree.
	ViaCustomerPortal bool
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
	// ProjectActivityCounts returns, for each of projectIDs, the three counts
	// a project row shows in the customer portal's project list. Every
	// project in projectIDs is present in the result, at zero when nothing
	// counts. See ProjectActivityStates for what each count is made of.
	ProjectActivityCounts(ctx context.Context, scope SearchScope, projectIDs []string, states ProjectActivityStates) (map[string]ProjectActivityCounts, error)
}

type globalSearchRepo struct {
	db *Scoped
	// vis narrows the change request counts to what a customer may see, the
	// same rule the project stats apply (change_request_visibility.go).
	vis CRVisibility
}

// NewGlobalSearchRepository constructs a GlobalSearchRepository backed by the given connection pool.
// vis is optional: none given is the zero CRVisibility, no strict-visibility cutover.
func NewGlobalSearchRepository(db *Scoped, vis ...CRVisibility) GlobalSearchRepository {
	return &globalSearchRepo{db: db, vis: firstCRVisibility(vis)}
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

// runSearch executes the count and page queries concurrently, each through
// Scoped so the caller's identity (stamped onto ctx from the explicit scope
// parameter below) is set correctly -- required so any table these queries
// touch that carries a caller-scoped row-level-security policy (currently
// `announcement` and, once work_item's own RLS is in play, SearchCases'
// join more broadly) is evaluated correctly. This applies unconditionally,
// including for SearchProjects, which doesn't currently need it: the cost
// is negligible, and it means a future RLS policy on another table this
// function's callers might one day join against needs no further change
// here.
func runSearch[T any](ctx context.Context, db *Scoped, scope SearchScope, countSQL, pageSQL string, f searchFilter, pagination domain.Pagination, scan func(pgx.Rows) (T, error)) ([]T, int, error) {
	// WithCallerIdentity from the explicit scope parameter, not whatever
	// identity ctx already carries -- same convention as case_repo.go's
	// GetCaseByID/SearchCases, which take an identical explicit parameter
	// for the same reason.
	ctx = WithCallerIdentity(ctx, scope)

	pageArgs := append(append([]any{}, f.args...), pagination.Limit, pagination.Offset)
	pageSQL = fmt.Sprintf("%s LIMIT $%d OFFSET $%d", pageSQL, len(f.args)+1, len(f.args)+2)

	var total int
	out := make([]T, 0, pagination.Limit)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := db.QueryRow(egCtx, countSQL, f.args...).Scan(&total); err != nil {
			return fmt.Errorf("count: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		rows, err := db.Query(egCtx, pageSQL, pageArgs...)
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
// activeChatsCount/actionRequiredCount/outstandingCount are left at 0 here:
// they are filled in by the caller from ProjectActivityCounts, for just the
// page of projects this returns.
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
	// No f.scope("wi.project_id", scope) call here any more -- work_item's
	// own RLS policy (migration 0147) already applies the identical
	// is_project_member check to every statement this repository's Scoped
	// connection issues, so a second hand-written copy would only be a
	// second place for the two to drift. The early return above still
	// short-circuits the round trip for a scoped caller with zero
	// registered projects; it's an optimization, not the enforcement.
	//
	// See announcementVisibilityLeakGuard's own doc comment (case_repo.go).
	// Applies to both countSQL and pageSQL below, via this same f.where.
	f.where += " AND " + announcementLeakGuardFor(scope)
	// Planner hint for external callers only (see viewerProjectHint); RLS
	// remains the authorization boundary.
	f.where += viewerProjectHint("wi", scope)
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

// projectActivityCaseTypes are the work item types a project's Action Required
// and Outstanding counts are made of, besides change requests: the four the
// project dashboard combines (apps/customer-portal DashboardPage's
// combinedCaseTypes). Announcements are left out there, so they are here.
var projectActivityCaseTypes = []string{"CASE", "SERVICE_REQUEST", "ENGAGEMENT", "SECURITY_REPORT_ANALYSIS"}

// ProjectActivityStates names the states that make an item count, so the
// state vocabulary stays with the service that owns it (the project stats
// service's constants) and this repository only does the counting.
//
//   - Outstanding = case-like items in any state but CaseClosed + change
//     requests in CROutstanding.
//   - Action Required = case-like items in CaseActionRequired + change
//     requests in CRActionRequired.
//   - Active Chats = conversations in ChatActive.
//
// Case-like items are the types in projectActivityCaseTypes. A state list may
// be empty, which counts nothing for that part (CaseClosed empty counts every
// case-like item as outstanding).
//
// Outstanding is "not closed" rather than "in one of these states" on purpose:
// that is how the project dashboard's Outstanding tile counts
// (projectCaseStatsService: "active and outstanding are the same set, every
// state except CLOSED"), so a state added to the enum is outstanding without
// being listed anywhere. It counts only an item that HAS a state of its own
// type (caseLikeOwnStateColumn): a case-like work item whose extension row is
// missing, or belongs to another type, which real synced data has, is in no
// list filtered by state, and the dashboard tile leaves it out for the same
// reason (the tile used to count it and read higher than the list behind it).
type ProjectActivityStates struct {
	CaseClosed         []string
	CaseActionRequired []string
	CROutstanding      []string
	CRActionRequired   []string
	ChatActive         []string
}

// ProjectActivityCounts is one project's row of ProjectActivityCounts.
type ProjectActivityCounts struct {
	ActiveChats    int
	ActionRequired int
	Outstanding    int
}

// ProjectActivityCounts implements GlobalSearchRepository. It is three
// grouped queries for the whole page of projects, not three per project, and
// they run concurrently.
//
// Each count is read exactly as the project's own dashboard reads it, so a
// project's row in the list agrees with its dashboard tiles: the same tables,
// the same states (passed in by the service from the stats constants), and for
// a customer the same visibility, i.e. row-level security through Scoped (the
// identity comes from scope, as in runSearch) plus, for change requests, the
// designation rule (CRVisibility), which row-level security does not apply.
//
// If any query fails the first error is returned and no partial map with it:
// a count that silently stayed zero is indistinguishable from a real zero, so
// the caller decides what a missing count is worth.
func (r *globalSearchRepo) ProjectActivityCounts(ctx context.Context, scope SearchScope, projectIDs []string, states ProjectActivityStates) (map[string]ProjectActivityCounts, error) {
	out := make(map[string]ProjectActivityCounts, len(projectIDs))
	for _, id := range projectIDs {
		out[id] = ProjectActivityCounts{}
	}
	if len(projectIDs) == 0 {
		return out, nil
	}
	ctx = WithCallerIdentity(ctx, scope)

	var (
		mu sync.Mutex
		eg errgroup.Group
	)
	add := func(projectID string, fn func(*ProjectActivityCounts)) {
		mu.Lock()
		defer mu.Unlock()
		c, ok := out[projectID]
		if !ok {
			// A row for a project that was not asked for cannot happen with
			// the ANY($1) filter; ignore it rather than invent an entry.
			return
		}
		fn(&c)
		out[projectID] = c
	}

	eg.Go(func() error {
		return r.perChunk(ctx, scope, projectIDs, func(q activityQuerier, ids []string) error {
			// The state is the one of the item's own type (caseLikeOwnStateColumn), the
			// one a list filtered by state matches. An item with none is NULL here and
			// satisfies neither condition, so it is not counted (see
			// ProjectActivityStates): a bare <> ALL is NULL for it, which is what drops it.
			notClosed := caseLikeOwnStateColumn + ` <> ALL($3::text[])`
			rows, err := q.Query(ctx, `
				SELECT wi.project_id::text,
				       COUNT(*) FILTER (WHERE `+notClosed+`),
				       COUNT(*) FILTER (WHERE `+caseLikeOwnStateColumn+` = ANY($4::text[]))
				  FROM work_item wi
				  LEFT JOIN "case" c ON c.id = wi.id`+caseLikeJoins+`
				 WHERE wi.project_id = ANY($1::text[]::uuid[])
				   AND wi.type = ANY($2::work_item_type_enum[])
				   AND (`+notClosed+` OR `+caseLikeOwnStateColumn+` = ANY($4::text[]))
				 GROUP BY 1`,
				ids, projectActivityCaseTypes, nonNil(states.CaseClosed), nonNil(states.CaseActionRequired))
			if err != nil {
				return fmt.Errorf("project activity counts: cases: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var id string
				var outstanding, actionRequired int
				if err := rows.Scan(&id, &outstanding, &actionRequired); err != nil {
					return fmt.Errorf("project activity counts: scan cases: %w", err)
				}
				add(id, func(c *ProjectActivityCounts) {
					c.Outstanding += outstanding
					c.ActionRequired += actionRequired
				})
			}
			return rows.Err()
		})
	})

	if len(states.CROutstanding)+len(states.CRActionRequired) > 0 {
		eg.Go(func() error {
			return r.perChunk(ctx, scope, projectIDs, func(q activityQuerier, ids []string) error {
				// The state arguments come first so the visibility fragment's own
				// placeholders continue after them (andClause numbers from the
				// length of the args it is given).
				visSQL, args := r.vis.andClause(ctx, "wi", "cr",
					[]any{ids, nonNil(states.CROutstanding), nonNil(states.CRActionRequired)})
				rows, err := q.Query(ctx, `
					SELECT wi.project_id::text,
					       COUNT(*) FILTER (WHERE cr.state::TEXT = ANY($2::text[])),
					       COUNT(*) FILTER (WHERE cr.state::TEXT = ANY($3::text[]))
					  FROM work_item wi
					  JOIN change_request cr ON cr.id = wi.id
					 WHERE wi.project_id = ANY($1::text[]::uuid[])
					   AND (cr.state::TEXT = ANY($2::text[]) OR cr.state::TEXT = ANY($3::text[]))`+visSQL+`
					 GROUP BY 1`, args...)
				if err != nil {
					return fmt.Errorf("project activity counts: change requests: %w", err)
				}
				defer rows.Close()
				for rows.Next() {
					var id string
					var outstanding, actionRequired int
					if err := rows.Scan(&id, &outstanding, &actionRequired); err != nil {
						return fmt.Errorf("project activity counts: scan change requests: %w", err)
					}
					add(id, func(c *ProjectActivityCounts) {
						c.Outstanding += outstanding
						c.ActionRequired += actionRequired
					})
				}
				return rows.Err()
			})
		})
	}

	if len(states.ChatActive) > 0 {
		eg.Go(func() error {
			return r.perChunk(ctx, scope, projectIDs, func(q activityQuerier, ids []string) error {
				rows, err := q.Query(ctx, `
					SELECT wi.project_id::text, COUNT(*)
					  FROM work_item wi
					  JOIN conversation conv ON conv.id = wi.id
					 WHERE wi.project_id = ANY($1::text[]::uuid[])
					   AND conv.state::TEXT = ANY($2::text[])
					 GROUP BY 1`, ids, states.ChatActive)
				if err != nil {
					return fmt.Errorf("project activity counts: conversations: %w", err)
				}
				defer rows.Close()
				for rows.Next() {
					var id string
					var n int
					if err := rows.Scan(&id, &n); err != nil {
						return fmt.Errorf("project activity counts: scan conversations: %w", err)
					}
					add(id, func(c *ProjectActivityCounts) { c.ActiveChats += n })
				}
				return rows.Err()
			})
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

// activityQuerier is what the count queries need: a Scoped (one round trip
// that carries the caller's identity) or a pgx.Tx that already carries it.
type activityQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// projectActivityChunkSize is how many projects one narrowed statement covers.
// Measured on a staging-like copy (409k work items, a customer registered on
// the 50 heaviest projects): counting all 50 in one go took ~430 ms, as chunks
// of 25 ~240 ms, of 10 ~120 ms, of 5 ~90 ms. Ten is the portal's default page
// size, so the common page is one chunk, and the gain from 5 does not pay for
// the extra round trips.
const projectActivityChunkSize = 10

// narrowViewerProjectIDsSQL narrows app.viewer_project_ids, the list every
// row-level-security policy compares a row's project against, to the part of it
// that is in $1 (the projects about to be counted).
//
// WHY: each policy re-parses that setting's text into a uuid[] for every row it
// checks, so the cost per row grows with the length of the viewer's whole list.
// For a caller registered on 50 projects that made the counts ~4x slower per
// item than for the same page narrowed to its own projects.
//
// WHY IT CANNOT WIDEN ACCESS: it is an intersection, computed in the database,
// with the list Scoped established a statement earlier in the SAME transaction
// (from project_contact, REGISTERED). A project in $1 that the viewer is not a
// member of is not in the list and stays invisible; nothing here reads or
// trusts a caller-supplied project as membership. It is set_config(..., true),
// so it reverts at COMMIT/ROLLBACK and cannot leak to the next statement on the
// connection. An internal caller's list is '{}' and is never narrowed (see
// perChunk), and is_internal short-circuits every policy before the list is
// read anyway. Every row the narrowed policies reject is a row the query's own
// `project_id = ANY($1)` would have excluded: for the rows the query asks for,
// the result is the same as without it.
//
// It must be its own statement ahead of the guarded query, never inlined into
// it (the planner may evaluate a policy before a non-leakproof set_config; see
// setCallerIdentity).
const narrowViewerProjectIDsSQL = `SELECT set_config('app.viewer_project_ids', COALESCE((
	SELECT array_agg(p)::text
	  FROM unnest(NULLIF(current_setting('app.viewer_project_ids', true), '')::uuid[]) AS p
	 WHERE p = ANY($1::text[]::uuid[])
), '{}'), true)`

// perChunk runs fn for projectIDs under the caller's row-level security.
//
// A caller registered on more than projectActivityChunkSize projects (a partner)
// is served in chunks of that size, each in its own Scoped transaction whose
// viewer project list is narrowed to the chunk (narrowViewerProjectIDsSQL). Any
// other caller -- an internal one, or a customer on a handful of projects, whose
// list is already short -- takes the single-statement path exactly as before, with
// no narrowing and no extra round trips.
//
// scope.ProjectIDs is used only to decide whether narrowing is worth doing, never
// as the membership that is enforced: that stays the database's.
func (r *globalSearchRepo) perChunk(ctx context.Context, scope SearchScope, projectIDs []string, fn func(q activityQuerier, ids []string) error) error {
	if scope.Unrestricted || len(scope.ProjectIDs) <= projectActivityChunkSize {
		return fn(r.db, projectIDs)
	}
	for start := 0; start < len(projectIDs); start += projectActivityChunkSize {
		chunk := projectIDs[start:min(start+projectActivityChunkSize, len(projectIDs))]
		err := r.db.InTx(ctx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, narrowViewerProjectIDsSQL, chunk); err != nil {
				return fmt.Errorf("narrow viewer project ids: %w", err)
			}
			return fn(tx, chunk)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// nonNil turns a nil slice into an empty one, so it binds as an empty
// text[] rather than a NULL (= ANY(NULL) is NULL, which would silently make a
// FILTER match nothing for the wrong reason).
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
