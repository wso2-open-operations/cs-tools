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

package service

import (
	"context"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"golang.org/x/sync/errgroup"
)

type globalService struct {
	repo   repository.ReferenceDataRepository
	search repository.GlobalSearchRepository
	access AccessService
}

// NewGlobalService constructs a Postgres-backed GlobalService. GetSystemMetadata
// is populated for real (project types come from the project_type table,
// time zones from the timezone table); see its own doc comment for the one
// field still left empty. GlobalSearch is scoped to what the caller may
// see -- see its own doc comment.
func NewGlobalService(repo repository.ReferenceDataRepository, search repository.GlobalSearchRepository, access AccessService) GlobalService {
	return &globalService{repo: repo, search: search, access: access}
}

// GetSystemMetadata implements GlobalService.
func (s *globalService) GetSystemMetadata(ctx context.Context) (domain.SystemMetadataResponse, error) {
	projectTypes, err := s.repo.ListProjectTypes(ctx)
	if err != nil {
		return domain.SystemMetadataResponse{}, err
	}

	items := make([]domain.ReferenceTableItem, 0, len(projectTypes))
	for _, pt := range projectTypes {
		items = append(items, domain.ReferenceTableItem{ID: pt.ID, Name: pt.Name})
	}

	timeZones, err := s.repo.ListTimeZones(ctx)
	if err != nil {
		return domain.SystemMetadataResponse{}, err
	}
	tzItems := make([]domain.ChoiceListItem, 0, len(timeZones))
	for _, tz := range timeZones {
		tzItems = append(tzItems, domain.ChoiceListItem{ID: tz.Value, Label: tz.Label})
	}

	feedbackEmojis, err := s.repo.ListFeedbackEmojis(ctx)
	if err != nil {
		return domain.SystemMetadataResponse{}, err
	}
	emojiItems := make([]domain.FeedbackEmoji, 0, len(feedbackEmojis))
	for _, e := range feedbackEmojis {
		chips := make([]domain.FeedbackEmojiChip, 0, len(e.Chips))
		for _, c := range e.Chips {
			chips = append(chips, domain.FeedbackEmojiChip{ID: c.ID, Name: c.Name, Value: c.Value})
		}
		emojiItems = append(emojiItems, domain.FeedbackEmoji{
			ID:              e.ID,
			Name:            e.Name,
			Value:           e.Value,
			UnselectedImage: e.UnselectedImage,
			SelectedImage:   e.SelectedImage,
			Chips:           chips,
		})
	}

	return domain.SystemMetadataResponse{ProjectTypes: items, TimeZones: tzItems, FeedbackEmojis: emojiItems}, nil
}

// globalSearchSortFields maps the accepted sortBy.field values to the
// repository's sort keys. One field applies to both tables: "name" is a
// project's name / a case's subject.
var globalSearchSortFields = map[string]repository.SearchSortField{
	"name":      repository.SearchSortName,
	"createdOn": repository.SearchSortCreatedOn,
	"updatedOn": repository.SearchSortUpdatedOn,
}

// GlobalSearch implements GlobalService. It searches projects and cases the
// caller is allowed to see (AccessService.ResolveScope): a customer's results
// are limited to projects they are a registered contact of and the cases in
// them, while internal users and "internal" system clients see everything. A
// caller with no resolvable access gets an error, never an unscoped result.
//
// Defaults, when the request doesn't say: projects sort by name ascending and
// cases by updatedOn descending; an explicit sortBy.field applies to both, with
// a default order of ascending for "name" and descending for the date fields.
// projectsPagination/casesPagination default to the standard page size.
//
// activeChatsCount/actionRequiredCount/outstandingCount on each project are
// the figures the project's own dashboard shows -- see fillProjectActivityCounts.
func (s *globalService) GlobalSearch(ctx context.Context, req domain.GlobalSearchRequest) (domain.GlobalSearchResponse, error) {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return domain.GlobalSearchResponse{}, err
	}

	var query string
	var tables []string
	if req.Filters != nil {
		query, tables = req.Filters.SearchQuery, req.Filters.Tables
		if err := validateSearchQuery(query); err != nil {
			return domain.GlobalSearchResponse{}, err
		}
		for _, t := range tables {
			if !validGlobalSearchTables[t] {
				return domain.GlobalSearchResponse{}, &apierror.ValidationError{Msg: "filters.tables contains invalid value: " + t}
			}
		}
	}

	sortField, desc, err := resolveGlobalSearchSort(req.SortBy)
	if err != nil {
		return domain.GlobalSearchResponse{}, err
	}

	projectsPage, casesPage := domain.Pagination{}, domain.Pagination{}
	if req.ProjectsPagination != nil {
		projectsPage = *req.ProjectsPagination
	}
	if req.CasesPagination != nil {
		casesPage = *req.CasesPagination
	}
	if err := normalizePagination(&projectsPage); err != nil {
		return domain.GlobalSearchResponse{}, err
	}
	if err := normalizePagination(&casesPage); err != nil {
		return domain.GlobalSearchResponse{}, err
	}

	wantProjects, wantCases := len(tables) == 0, len(tables) == 0
	for _, t := range tables {
		wantProjects = wantProjects || t == "projects"
		wantCases = wantCases || t == "cases"
	}

	resp := domain.GlobalSearchResponse{
		Query:    query,
		Projects: []domain.GlobalSearchProject{},
		Cases:    []domain.GlobalSearchCase{},
	}

	eg, egCtx := errgroup.WithContext(ctx)
	if wantProjects {
		eg.Go(func() error {
			field, d := projectSort(req.SortBy, sortField, desc)
			projects, total, err := s.search.SearchProjects(egCtx, scope, query, field, d, projectsPage)
			if err != nil {
				return err
			}
			if err := s.fillProjectActivityCounts(egCtx, scope, projects); err != nil {
				return err
			}
			resp.Projects, resp.ProjectsTotal = projects, total
			return nil
		})
	}
	if wantCases {
		eg.Go(func() error {
			field, d := caseSort(req.SortBy, sortField, desc)
			cases, total, err := s.search.SearchCases(egCtx, scope, query, field, d, casesPage)
			resp.Cases, resp.CasesTotal = cases, total
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return domain.GlobalSearchResponse{}, err
	}
	return resp, nil
}

// fillProjectActivityCounts sets activeChatsCount, actionRequiredCount and
// outstandingCount on each project of a result page, for the project list.
// They are the numbers the project's own dashboard shows, from the same state
// groupings the project stats use, so the two cannot drift:
//
//   - outstanding: cases, service requests, engagements and security report
//     analyses that are not closed (an item with no state of its own type is
//     not counted, as on the dashboard: no list can show it), plus the change
//     requests that are in motion for this caller (crOutstandingStatesFor).
//   - action required: those waiting on the customer -- cases in Awaiting
//     Info or Solution Proposed, change requests in Customer Approval or
//     Customer Review.
//   - active chats: conversations in an active state.
//
// One caveat the dashboard does not have: it also narrows by what the caller's
// role may see (hasSR, hasCR, ...), which is not known per project here, so a
// user without access to a type still has its items in the totals.
//
// A failure leaves the three at zero and is logged, never returned: the list
// is the portal's way into a project, and a lookup that decorates it must not
// be able to take it down (the same posture as GetProjectStats' instance
// count). The one exception is the request itself running out of time or being
// cancelled: that is returned, so the request fails as it would have had the
// search itself run into the deadline. Swallowing it would answer 200 with
// every count at zero and nothing in the log, indistinguishable from a real
// zero.
func (s *globalService) fillProjectActivityCounts(ctx context.Context, scope AccessScope, projects []domain.GlobalSearchProject) error {
	if len(projects) == 0 {
		return nil
	}
	ids := make([]string, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}
	counts, err := s.search.ProjectActivityCounts(ctx, scope, ids, repository.ProjectActivityStates{
		CaseClosed:         []string{caseStateClosed},
		CaseActionRequired: caseStatsActionRequiredStates,
		CROutstanding:      crOutstandingStatesFor(scope),
		CRActionRequired:   crActionRequiredStates,
		ChatActive:         conversationActiveStates,
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			// The request is over (deadline, cancelled, or a sibling of the
			// search failed): let it fail rather than answer with zeros.
			return ctxErr
		}
		slog.WarnContext(ctx, "global search: project counts degraded to zero",
			"projects", len(ids), "error", err)
		return nil
	}
	for i := range projects {
		c := counts[projects[i].ID]
		projects[i].ActiveChatsCount = c.ActiveChats
		projects[i].ActionRequiredCount = c.ActionRequired
		projects[i].OutstandingCount = c.Outstanding
	}
	return nil
}

// resolveGlobalSearchSort validates sortBy and returns the chosen field and
// direction. The field is empty when the caller named none, in which case each
// table falls back to its own default (see projectSort/caseSort).
func resolveGlobalSearchSort(sortBy *domain.GlobalSearchSort) (repository.SearchSortField, bool, error) {
	if sortBy == nil || (sortBy.Field == "" && sortBy.Order == "") {
		return "", false, nil
	}
	if sortBy.Order != "" && !validGlobalSearchSortOrder[sortBy.Order] {
		return "", false, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + sortBy.Order}
	}
	if sortBy.Field == "" {
		return "", sortBy.Order == "desc", nil
	}
	field, ok := globalSearchSortFields[sortBy.Field]
	if !ok {
		return "", false, &apierror.ValidationError{Msg: "sortBy.field must be one of: name, createdOn, updatedOn"}
	}
	desc := field != repository.SearchSortName
	if sortBy.Order != "" {
		desc = sortBy.Order == "desc"
	}
	return field, desc, nil
}

// projectSort/caseSort apply each table's default when the caller named no
// field, but still honour an explicit order on its own.
func projectSort(sortBy *domain.GlobalSearchSort, field repository.SearchSortField, desc bool) (repository.SearchSortField, bool) {
	if field != "" {
		return field, desc
	}
	return repository.SearchSortName, sortBy != nil && sortBy.Order == "desc"
}

func caseSort(sortBy *domain.GlobalSearchSort, field repository.SearchSortField, desc bool) (repository.SearchSortField, bool) {
	if field != "" {
		return field, desc
	}
	return repository.SearchSortUpdatedOn, sortBy == nil || sortBy.Order != "asc"
}
