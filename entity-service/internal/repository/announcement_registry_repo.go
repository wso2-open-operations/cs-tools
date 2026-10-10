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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// AnnouncementRegistryRepository reads every announcement case matching a
// search in ONE query, for the CSM announcement registry.
//
// The registry groups every matching announcement into batches client-side,
// so it needs the whole matching set, not a page. Reading it through
// CaseRepository.SearchCases meant one request per 50 rows (that search caps a
// page at 50), each repeating the same full scan, sort and COUNT: about 100
// queries per registry load for ~5,000 announcements. Under row-level
// security that is slow, because work_item's policy runs before the type
// filter and enum equality is not leakproof, so the (type, updated_on) index
// cannot be used and every page scans the whole table.
type AnnouncementRegistryRepository interface {
	// SearchAnnouncementCases returns every case matching req, newest-updated
	// first, carrying only the fields the registry reads. It returns
	// ErrTooManyRegistryRows when more than maxRows match, rather than a
	// silently truncated list.
	SearchAnnouncementCases(ctx context.Context, req domain.SearchCasesRequest, scope SearchScope, maxRows int) ([]domain.SearchCaseView, error)

	// SearchAnnouncementRegistryRows returns one page of the grouped registry:
	// the announcement cases matching req, collapsed into one row per
	// published announcement request (a "batch") plus one row per case no
	// published request owns, newest first. Grouping, ordering and paging all
	// happen in SQL, so nothing is capped and nothing is truncated.
	// req.Pagination must already be normalized. The response's Limit and
	// Offset are echoed from it.
	SearchAnnouncementRegistryRows(ctx context.Context, req domain.SearchCasesRequest, scope SearchScope) (domain.SearchAnnouncementRegistryRowsResponse, error)
}

// ErrTooManyRegistryRows is returned by SearchAnnouncementCases when the
// result would exceed maxRows.
var ErrTooManyRegistryRows = errors.New("too many matching announcements")

type announcementRegistryRepo struct {
	db *Scoped
}

// NewAnnouncementRegistryRepository returns an AnnouncementRegistryRepository
// backed by the Scoped pool, so the caller's identity is stamped on the query
// exactly as for every other protected read.
func NewAnnouncementRegistryRepository(db *Scoped) AnnouncementRegistryRepository {
	return &announcementRegistryRepo{db: db}
}

// SearchAnnouncementCases implements AnnouncementRegistryRepository.
//
// It reuses buildCaseSearchWhere and caseSearchJoins, so filters mean exactly
// what they mean in /cases/search, and the same ORDER BY as the registry used
// before (updatedOn descending, id as tie-break). The select list is cut down
// to what the registry reads, so Postgres drops the joins nothing references.
func (r *announcementRegistryRepo) SearchAnnouncementCases(ctx context.Context, req domain.SearchCasesRequest, scope SearchScope, maxRows int) ([]domain.SearchCaseView, error) {
	// Every row below is labelled an announcement, so refuse a request that
	// does not filter to announcements only (the service forces this; a future
	// caller that skips the service must not get other case types mislabelled).
	if len(req.Parsed.Types) != 1 || !strings.EqualFold(req.Parsed.Types[0], "announcement") {
		return nil, fmt.Errorf("announcement registry search requires a type filter of exactly announcement, got %v", req.Parsed.Types)
	}
	// Same explicit identity stamp as caseRepo.SearchCases.
	ctx = WithCallerIdentity(ctx, scope)
	where, args, argIdx, err := buildCaseSearchWhere(req, scope)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(
		`SELECT wi.id, wi.number, wi.wso2_id, wi.subject, `+caseLikeStateColumn+`,
		        wi.created_on, wi.updated_on, wi.created_by,
		        p.id, p.name
		 FROM work_item wi %s %s
		 ORDER BY wi.updated_on DESC NULLS LAST, wi.id
		 LIMIT $%d`,
		caseSearchJoins, where, argIdx,
	)
	args = append(args, maxRows+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query announcement registry cases: %w", err)
	}
	defer rows.Close()

	out := make([]domain.SearchCaseView, 0, 256)
	for rows.Next() {
		var (
			cv                   domain.SearchCaseView
			internalID           *string
			subject              string
			state                *string
			createdAt, updatedAt time.Time
			creatorEmail         string
			projID, projName     *string
		)
		if err := rows.Scan(&cv.ID, &cv.Number, &internalID, &subject, &state,
			&createdAt, &updatedAt, &creatorEmail, &projID, &projName); err != nil {
			return nil, fmt.Errorf("scan announcement registry case: %w", err)
		}
		cv.InternalID = stringOrEmpty(internalID)
		cv.Type = "announcement"
		cv.Subject = &subject
		if state != nil {
			lower := strings.ToLower(*state)
			cv.State = &lower
		}
		cv.CreatedOn = createdAt.UTC().Format(time.RFC3339)
		cv.UpdatedOn = updatedAt.UTC().Format(time.RFC3339)
		// Same as SearchCases: the projection carries only the creator's
		// email, so the reference keeps a null id and empty name.
		cv.CreatedBy = domain.NewUserReference("", creatorEmail, "")
		if projID != nil {
			cv.Project = &domain.EntityRef{ID: *projID, Name: stringOrEmpty(projName)}
		}
		out = append(out, cv)
		if len(out) > maxRows {
			return nil, ErrTooManyRegistryRows
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate announcement registry cases: %w", err)
	}
	return out, nil
}

// registryUUIDPattern gates the text -> uuid cast of a published_case_ids
// element. It is applied inside a CASE, which Postgres evaluates in order, so
// a malformed element yields NULL (no owner, no member) instead of failing the
// whole query.
const registryUUIDPattern = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`

// registryPublishedCaseIDsArray reads published_case_ids as an array, or an
// empty one when the column is NULL or not an array (jsonb_array_elements_text
// raises on a non-array).
const registryPublishedCaseIDsArray = `CASE WHEN jsonb_typeof(ar.published_case_ids) = 'array' THEN ar.published_case_ids ELSE '[]'::jsonb END`

// registryGroupsCTE is the head of the page query. Its final CTE, firsts, holds one row per registry row (id and updated_on of the
// group's first matching case, request_id when the group is a batch).
// %[1]s is caseSearchJoins and %[2]s the WHERE of the matching cases.
//
// Ownership: a case belongs to the PUBLISHED request listing its id in
// published_case_ids. If two published requests list the same case (should
// not happen), the OLDEST request (created_on, then the greater id) owns it.
// That is the request the previous Go grouping ended up with, because it
// walked the requests newest first and let the last one overwrite the map.
//
// Order: the previous Go grouping walked the cases in updated_on DESC NULLS
// LAST, id order and emitted a group where it first met one of its cases. So
// a group sits at the position of its first matching case in that order,
// which is the group's maximum updated_on (id breaking ties). firsts keeps
// exactly that case per group and the page query orders by it. Same order,
// stated without a loop.
//
// The OFFSET 0 in owners is an optimisation fence, not a limit: it stops the
// planner pulling the subquery up, so the regex check and uuid cast run once
// per published id. Without it the CASE is inlined and evaluated again for the
// IS NOT NULL filter and for the sort key.
//
// matched stays narrow (id, updated_on): it is the only part that scales with
// the number of announcements, and the page's display columns are joined back
// for the few rows returned.
const registryGroupsCTE = `
WITH matched AS (
	SELECT wi.id, wi.updated_on
	FROM work_item wi %[1]s %[2]s
), owners AS (
	SELECT DISTINCT ON (m.case_id) m.case_id, ar.id AS request_id
	FROM announcement_requests ar
	CROSS JOIN LATERAL (
		SELECT CASE WHEN e.cid ~ '` + registryUUIDPattern + `' THEN e.cid::uuid END AS case_id
		FROM jsonb_array_elements_text(` + registryPublishedCaseIDsArray + `) AS e(cid)
		OFFSET 0
	) m
	WHERE ar.state = 'published' AND m.case_id IS NOT NULL
	ORDER BY m.case_id, ar.created_on, ar.id DESC
), tagged AS (
	SELECT matched.id, matched.updated_on, o.request_id,
	       (o.request_id IS NOT NULL) AS is_batch, COALESCE(o.request_id, matched.id) AS group_key
	FROM matched LEFT JOIN owners o ON o.case_id = matched.id
), firsts AS (
	SELECT DISTINCT ON (is_batch, group_key) id, updated_on, request_id
	FROM tagged
	ORDER BY is_batch, group_key, updated_on DESC NULLS LAST, id
)`

// SearchAnnouncementRegistryRows implements AnnouncementRegistryRepository.
func (r *announcementRegistryRepo) SearchAnnouncementRegistryRows(ctx context.Context, req domain.SearchCasesRequest, scope SearchScope) (domain.SearchAnnouncementRegistryRowsResponse, error) {
	if len(req.Parsed.Types) != 1 || !strings.EqualFold(req.Parsed.Types[0], "announcement") {
		return domain.SearchAnnouncementRegistryRowsResponse{}, fmt.Errorf("announcement registry search requires a type filter of exactly announcement, got %v", req.Parsed.Types)
	}
	// Internal callers only. The service already enforces this; the repository
	// refuses on its own too, so a future caller that skips the service cannot
	// read the registry with a customer scope.
	if !scope.Unrestricted {
		return domain.SearchAnnouncementRegistryRowsResponse{}, &apierror.ForbiddenError{Msg: "the announcement registry is only available to internal callers"}
	}
	ctx = WithCallerIdentity(ctx, scope)
	where, args, argIdx, err := buildCaseSearchWhere(req, scope)
	if err != nil {
		return domain.SearchAnnouncementRegistryRowsResponse{}, err
	}
	head := fmt.Sprintf(registryGroupsCTE, caseSearchJoins, where)

	pageQuery := head + fmt.Sprintf(`, tot AS (
	SELECT COUNT(*) AS total FROM firsts
), page AS (
	SELECT id, updated_on, request_id
	FROM firsts
	ORDER BY updated_on DESC NULLS LAST, id
	LIMIT $%d OFFSET $%d
)
SELECT tot.total, page.id, page.request_id,
       req.subject, req.created_by, req.created_by_email, req.created_on, req.updated_on,
       req.resolved_project_count, req.announcement_type::TEXT,
       jsonb_array_length(CASE WHEN jsonb_typeof(req.published_case_ids) = 'array' THEN req.published_case_ids ELSE '[]'::jsonb END),
       wi.id, wi.number, wi.wso2_id, wi.subject, `+caseLikeStateColumn+`,
       wi.created_on, wi.updated_on, wi.created_by, p.name
FROM tot
LEFT JOIN page ON TRUE
LEFT JOIN work_item wi ON wi.id = page.id
%s
LEFT JOIN announcement_requests req ON req.id = page.request_id
ORDER BY page.updated_on DESC NULLS LAST, page.id`, argIdx, argIdx+1, caseSearchJoins)
	pageArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	resp := domain.SearchAnnouncementRegistryRowsResponse{
		Rows:   []domain.AnnouncementRegistryRow{},
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}
	rows, err := r.db.Query(ctx, pageQuery, pageArgs...)
	if err != nil {
		return resp, fmt.Errorf("query announcement registry rows: %w", err)
	}
	defer rows.Close()

	var batchIDs []string
	batchIdx := map[string]int{}
	for rows.Next() {
		var (
			total                              int
			pageID, requestID                  *string
			reqSubject, reqCreatedBy, reqEmail *string
			reqCreatedOn, reqUpdatedOn         *time.Time
			reqProjectCount                    *int
			reqType                            *string
			reqMemberCount                     *int
			caseID, number                     *string
			wso2ID                             *string
			subject                            *string
			state                              *string
			createdOn, updatedOn               *time.Time
			creator                            *string
			projectName                        *string
		)
		if err := rows.Scan(&total, &pageID, &requestID,
			&reqSubject, &reqCreatedBy, &reqEmail, &reqCreatedOn, &reqUpdatedOn,
			&reqProjectCount, &reqType, &reqMemberCount,
			&caseID, &number, &wso2ID, &subject, &state,
			&createdOn, &updatedOn, &creator, &projectName); err != nil {
			return resp, fmt.Errorf("scan announcement registry row: %w", err)
		}
		resp.Total = total
		// An empty page still yields one row, carrying only the total.
		if pageID == nil {
			continue
		}
		if requestID != nil {
			row := domain.AnnouncementRegistryRow{
				Kind:                   domain.AnnouncementRegistryRowKindBatch,
				Subject:                stringOrEmpty(reqSubject),
				CreatedBy:              stringOrEmpty(reqCreatedBy),
				AnnouncementRequestID:  *requestID,
				IsSecurityAnnouncement: stringOrEmpty(reqType) == "SECURITY",
			}
			if email := stringOrEmpty(reqEmail); email != "" {
				row.CreatedBy = email
			}
			if reqCreatedOn != nil {
				row.CreatedOn = reqCreatedOn.UTC().Format(time.RFC3339)
			}
			if reqUpdatedOn != nil {
				row.UpdatedOn = reqUpdatedOn.UTC().Format(time.RFC3339)
			}
			if reqProjectCount != nil {
				row.ProjectCount = *reqProjectCount
			} else if reqMemberCount != nil {
				row.ProjectCount = *reqMemberCount
			}
			batchIdx[*requestID] = len(resp.Rows)
			batchIDs = append(batchIDs, *requestID)
			resp.Rows = append(resp.Rows, row)
			continue
		}
		row := domain.AnnouncementRegistryRow{
			Kind:        domain.AnnouncementRegistryRowKindCase,
			Subject:     stringOrEmpty(subject),
			CreatedBy:   stringOrEmpty(creator),
			CreatedOn:   timeOrEmpty(createdOn),
			UpdatedOn:   timeOrEmpty(updatedOn),
			CaseID:      *pageID,
			CaseNumber:  stringOrEmpty(number),
			WSO2CaseID:  stringOrEmpty(wso2ID),
			ProjectName: stringOrEmpty(projectName),
		}
		if state != nil {
			row.State = strings.ToLower(*state)
		}
		resp.Rows = append(resp.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return resp, fmt.Errorf("iterate announcement registry rows: %w", err)
	}
	rows.Close()

	// Never true for an empty page, whatever the offset and total.
	resp.HasMore = len(resp.Rows) > 0 && resp.Offset+len(resp.Rows) < resp.Total

	if len(batchIDs) > 0 {
		if err := r.fillBatchMembers(ctx, scope, batchIDs, batchIdx, resp.Rows); err != nil {
			return resp, err
		}
	}
	return resp, nil
}

// fillBatchMembers lists the members of the page's batch rows: every id in
// the request's published_case_ids, in that order, whatever the search
// filters were (a filter picks which rows appear, it never shrinks a batch),
// skipping ids with no announcement case row visible to the caller.
func (r *announcementRegistryRepo) fillBatchMembers(ctx context.Context, scope SearchScope, batchIDs []string, batchIdx map[string]int, out []domain.AnnouncementRegistryRow) error {
	query := `SELECT ar.id, wi.id, wi.number, wi.wso2_id, p.name
FROM announcement_requests ar
CROSS JOIN LATERAL (
	SELECT CASE WHEN e.cid ~ '` + registryUUIDPattern + `' THEN e.cid::uuid END AS case_id, e.ord
	FROM jsonb_array_elements_text(` + registryPublishedCaseIDsArray + `) WITH ORDINALITY AS e(cid, ord)
) mem
JOIN work_item wi ON wi.id = mem.case_id AND wi.type = 'ANNOUNCEMENT' AND ` + announcementLeakGuardFor(scope) + `
LEFT JOIN project p ON p.id = wi.project_id
WHERE ar.id = ANY($1::uuid[])
ORDER BY ar.id, mem.ord`
	rows, err := r.db.Query(ctx, query, batchIDs)
	if err != nil {
		return fmt.Errorf("query announcement registry batch members: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			requestID, caseID, number string
			wso2ID, projectName       *string
		)
		if err := rows.Scan(&requestID, &caseID, &number, &wso2ID, &projectName); err != nil {
			return fmt.Errorf("scan announcement registry batch member: %w", err)
		}
		i, ok := batchIdx[requestID]
		if !ok {
			continue
		}
		out[i].Cases = append(out[i].Cases, domain.AnnouncementRegistryCaseMember{
			CaseID:      caseID,
			CaseNumber:  number,
			WSO2CaseID:  stringOrEmpty(wso2ID),
			ProjectName: stringOrEmpty(projectName),
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate announcement registry batch members: %w", err)
	}
	return nil
}

// timeOrEmpty formats t as RFC 3339 UTC, or "" for a NULL.
func timeOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
