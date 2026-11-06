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
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

const (
	// maxAllocationUpdateSubject is the subject column's width.
	maxAllocationUpdateSubject = 512
	// maxAllocationUpdateContent is the length the update's source field allowed.
	maxAllocationUpdateContent = 8000
)

// allocationStates are the values of engagement_allocation_state_enum.
var allocationStates = map[string]bool{
	"TENTATIVE": true, "READY_FOR_CLEARANCE": true, "CONFIRMED": true, "CLEARANCE_IN_PROGRESS": true,
	"ALLOCATION_CANCELLED": true, "ACCEPTED_BY_CONSULTANT": true, "REJECTED_OTHER": true,
	"REJECTED_BY_CONSULTANT": true, "CONFIRMED_VISA_PENDING": true, "REJECTED_VISA_ISSUES": true,
}

// EngagementAllocationViewService serves the allocation list, detail and
// updates, and lets the allocated engineer post an update. Internal staff only.
type EngagementAllocationViewService interface {
	Search(ctx context.Context, req domain.SearchEngagementAllocationsRequest) (domain.SearchEngagementAllocationsResponse, error)
	Get(ctx context.Context, id string) (domain.EngagementAllocationView, error)
	ListUpdates(ctx context.Context, id string, p domain.Pagination) (domain.ListEngagementAllocationUpdatesResponse, error)
	// CreateUpdate posts a published, internal update as the caller, who must
	// be the engineer the allocation is for.
	CreateUpdate(ctx context.Context, id string, req domain.CreateEngagementAllocationUpdateRequest) (domain.EngagementAllocationUpdate, error)
}

type engagementAllocationViewService struct {
	repo   repository.EngagementAllocationViewRepository
	access AccessService
	now    func() time.Time
}

// NewEngagementAllocationViewService constructs the service.
func NewEngagementAllocationViewService(repo repository.EngagementAllocationViewRepository, access AccessService) EngagementAllocationViewService {
	return &engagementAllocationViewService{repo: repo, access: access, now: time.Now}
}

func (s *engagementAllocationViewService) requireInternalCaller(ctx context.Context) error {
	return RequireInternalCaller(ctx, s.access, "engagement allocations are only available to internal staff")
}

func validDate(field, v string) error {
	if v == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", v); err != nil {
		return &apierror.ValidationError{Msg: field + " must be a date (YYYY-MM-DD)"}
	}
	return nil
}

// Search implements EngagementAllocationViewService.
func (s *engagementAllocationViewService) Search(ctx context.Context, req domain.SearchEngagementAllocationsRequest) (domain.SearchEngagementAllocationsResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.SearchEngagementAllocationsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchEngagementAllocationsResponse{}, err
	}
	f := &req.Filters
	f.SearchQuery = strings.TrimSpace(f.SearchQuery)
	f.AccountName = strings.TrimSpace(f.AccountName)
	if len(f.SearchQuery) > maxSearchQueryLen || len(f.AccountName) > maxSearchQueryLen {
		return domain.SearchEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: "search text is too long"}
	}
	// Checked in a fixed order so the message names the same field every time.
	for _, c := range []struct{ field, id string }{{"accountId", f.AccountID}, {"engineerId", f.EngineerID}, {"engagementId", f.EngagementID}} {
		if c.id != "" && !validate.IsUUID(c.id) {
			return domain.SearchEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: c.field + " must be a UUID"}
		}
	}
	for _, st := range f.States {
		if !allocationStates[st] {
			return domain.SearchEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: "unknown allocation state " + st}
		}
	}
	if err := validDate("from", f.From); err != nil {
		return domain.SearchEngagementAllocationsResponse{}, err
	}
	if err := validDate("to", f.To); err != nil {
		return domain.SearchEngagementAllocationsResponse{}, err
	}
	if f.From != "" && f.To != "" && f.To < f.From {
		return domain.SearchEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: "to must not be before from"}
	}
	if req.SortBy.Field != "" && !repository.IsEngagementAllocationSortField(req.SortBy.Field) {
		return domain.SearchEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: "unknown sortBy.field " + req.SortBy.Field}
	}
	if o := strings.ToLower(req.SortBy.Order); o != "" && o != "asc" && o != "desc" {
		return domain.SearchEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: "sortBy.order must be asc or desc"}
	}
	rows, total, err := s.repo.SearchAllocations(ctx, req)
	if err != nil {
		return domain.SearchEngagementAllocationsResponse{}, err
	}
	if rows == nil {
		rows = []domain.EngagementAllocationView{}
	}
	return domain.SearchEngagementAllocationsResponse{
		Allocations: rows, Total: total, Limit: req.Pagination.Limit, Offset: req.Pagination.Offset,
		HasMore: req.Pagination.Offset+len(rows) < total,
	}, nil
}

// Get implements EngagementAllocationViewService.
func (s *engagementAllocationViewService) Get(ctx context.Context, id string) (domain.EngagementAllocationView, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.EngagementAllocationView{}, err
	}
	if err := requireAllocationID(id); err != nil {
		return domain.EngagementAllocationView{}, err
	}
	return s.repo.AllocationByID(ctx, id)
}

// requireAllocationID refuses an id that is not a UUID before it reaches a uuid column.
func requireAllocationID(id string) error {
	if !validate.IsUUID(id) {
		return &apierror.ValidationError{Msg: "allocation id must be a UUID"}
	}
	return nil
}

// ListUpdates implements EngagementAllocationViewService.
func (s *engagementAllocationViewService) ListUpdates(ctx context.Context, id string, p domain.Pagination) (domain.ListEngagementAllocationUpdatesResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ListEngagementAllocationUpdatesResponse{}, err
	}
	if err := requireAllocationID(id); err != nil {
		return domain.ListEngagementAllocationUpdatesResponse{}, err
	}
	if err := normalizePagination(&p); err != nil {
		return domain.ListEngagementAllocationUpdatesResponse{}, err
	}
	// A missing allocation is a 404, not an empty list.
	if _, err := s.repo.AllocationByID(ctx, id); err != nil {
		return domain.ListEngagementAllocationUpdatesResponse{}, err
	}
	rows, total, err := s.repo.ListUpdates(ctx, id, p.Limit, p.Offset)
	if err != nil {
		return domain.ListEngagementAllocationUpdatesResponse{}, err
	}
	if rows == nil {
		rows = []domain.EngagementAllocationUpdate{}
	}
	return domain.ListEngagementAllocationUpdatesResponse{
		Updates: rows, Total: total, Limit: p.Limit, Offset: p.Offset, HasMore: p.Offset+len(rows) < total,
	}, nil
}

// weekStart is the Monday of the week t falls in, as YYYY-MM-DD (UTC).
func weekStart(t time.Time) string {
	t = t.UTC()
	back := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -back).Format("2006-01-02")
}

// CreateUpdate implements EngagementAllocationViewService.
//
// The caller is identified by the validated email of the user token, which is
// resolved to the platform user record; that platform id is what is compared
// with the allocation's engineer. The identity provider's user id is a
// different id space and is never compared with a platform id.
func (s *engagementAllocationViewService) CreateUpdate(ctx context.Context, id string, req domain.CreateEngagementAllocationUpdateRequest) (domain.EngagementAllocationUpdate, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.EngagementAllocationUpdate{}, err
	}
	if err := requireAllocationID(id); err != nil {
		return domain.EngagementAllocationUpdate{}, err
	}
	subject, content := strings.TrimSpace(req.Subject), strings.TrimSpace(req.Content)
	if subject == "" {
		return domain.EngagementAllocationUpdate{}, &apierror.ValidationError{Msg: "subject is required"}
	}
	if utf8.RuneCountInString(subject) > maxAllocationUpdateSubject {
		return domain.EngagementAllocationUpdate{}, &apierror.ValidationError{Msg: "subject must be at most 512 characters"}
	}
	if content == "" {
		return domain.EngagementAllocationUpdate{}, &apierror.ValidationError{Msg: "content is required"}
	}
	if utf8.RuneCountInString(content) > maxAllocationUpdateContent {
		return domain.EngagementAllocationUpdate{}, &apierror.ValidationError{Msg: "content must be at most 8000 characters"}
	}

	alloc, err := s.repo.AllocationByID(ctx, id)
	if err != nil {
		return domain.EngagementAllocationUpdate{}, err
	}
	const notAllocated = "only the engineer allocated can post an update on this allocation"
	email := auth.IdentityFromContext(ctx).UserEmail
	if email == "" || alloc.Engineer == nil {
		return domain.EngagementAllocationUpdate{}, &apierror.ForbiddenError{Msg: notAllocated}
	}
	caller, err := s.repo.UserByEmail(ctx, email)
	if err != nil {
		return domain.EngagementAllocationUpdate{}, err
	}
	if caller == nil || !strings.EqualFold(caller.ID, alloc.Engineer.ID) {
		return domain.EngagementAllocationUpdate{}, &apierror.ForbiddenError{Msg: notAllocated}
	}

	up, err := s.repo.CreateUpdate(ctx, repository.EngagementAllocationUpdateWrite{
		AllocationID:   alloc.ID,
		EngagementID:   alloc.Engagement.ID,
		AuthorID:       caller.ID,
		Subject:        subject,
		Content:        content,
		State:          "PUBLISHED",
		Frequency:      "WEEKLY",
		Scope:          "INDIVIDUAL",
		Visibility:     "INTERNAL",
		CycleStartDate: weekStart(s.now()),
		ActorEmail:     email,
	})
	if err != nil {
		return domain.EngagementAllocationUpdate{}, err
	}
	up.Author = caller
	return up, nil
}
