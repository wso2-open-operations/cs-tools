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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// maxEngagementAllocationUsers bounds how many people one read may ask about.
const maxEngagementAllocationUsers = 500

// SearchEngagementAllocations returns the confirmed customer engagement
// allocations of the requested people over the window. Internal callers only,
// like the rest of the team schedule.
func (s *scheduleService) SearchEngagementAllocations(ctx context.Context, req domain.SearchScheduleEngagementAllocationsRequest) (domain.ScheduleEngagementAllocationsResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleEngagementAllocationsResponse{}, err
	}
	if _, _, err := parseWindow(req.From, req.To); err != nil {
		return domain.ScheduleEngagementAllocationsResponse{}, err
	}
	if len(req.UserIDs) == 0 {
		return domain.ScheduleEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: "userIds is required"}
	}
	if len(req.UserIDs) > maxEngagementAllocationUsers {
		return domain.ScheduleEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: "too many userIds"}
	}
	for _, id := range req.UserIDs {
		if id == "" {
			return domain.ScheduleEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: "userIds must not contain an empty id"}
		}
		if err := validateUserID(id); err != nil {
			return domain.ScheduleEngagementAllocationsResponse{}, err
		}
	}
	rows, err := s.repo.SearchEngagementAllocations(ctx, req)
	if err != nil {
		return domain.ScheduleEngagementAllocationsResponse{}, err
	}
	if rows == nil {
		rows = []domain.ScheduleEngagementAllocation{}
	}
	return domain.ScheduleEngagementAllocationsResponse{Allocations: rows, Count: len(rows)}, nil
}

// DismissEngagementAllocation implements ScheduleService.
//
// The allocation comes from the allocation app and is never edited here: a
// removal records the days it is taken off the person's roster, so the next
// sync keeps it, and Restore puts days back. The span is cut to the
// allocation's own dates; only weekdays are recorded (the roster draws none
// on a weekend). Whoever may change that person's leave and allocations on
// the team (the team's lead or rota marker, the family's rota admin) may do
// it, the same check as marking a tag.
func (s *scheduleService) DismissEngagementAllocation(ctx context.Context, req domain.DismissScheduleEngagementAllocationRequest) (domain.DismissScheduleEngagementAllocationResponse, error) {
	req.ID = strings.TrimSpace(req.ID)
	req.TeamKey = strings.TrimSpace(req.TeamKey)
	if req.ID == "" || req.TeamKey == "" || req.From == "" || req.To == "" {
		return domain.DismissScheduleEngagementAllocationResponse{}, &apierror.ValidationError{
			Msg: "id, teamKey, from and to are all required",
		}
	}
	if err := validateUserID(req.ID); err != nil {
		return domain.DismissScheduleEngagementAllocationResponse{}, &apierror.ValidationError{Msg: "id must be a UUID"}
	}
	from, to, err := parseWindow(req.From, req.To)
	if err != nil {
		return domain.DismissScheduleEngagementAllocationResponse{}, err
	}
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.DismissScheduleEngagementAllocationResponse{}, err
	}
	al, err := s.repo.EngagementAllocationOnRoster(ctx, req.ID)
	if err != nil {
		return domain.DismissScheduleEngagementAllocationResponse{}, err
	}
	// Cut to the allocation's own dates (ISO dates compare as strings).
	f, t := from.Format("2006-01-02"), to.Format("2006-01-02")
	if f < al.StartDate {
		f = al.StartDate
	}
	if t > al.EndDate {
		t = al.EndDate
	}
	if f > t {
		return domain.DismissScheduleEngagementAllocationResponse{}, &apierror.ValidationError{
			Msg: "those days are outside the allocation's dates",
		}
	}
	if _, err := s.requireRotaWriterOverSpan(ctx, req.TeamKey, al.UserID, f, t, true); err != nil {
		return domain.DismissScheduleEngagementAllocationResponse{}, err
	}
	req.From, req.To = f, t
	days, err := s.repo.DismissEngagementAllocation(ctx, al, req, auth.IdentityFromContext(ctx).UserEmail)
	if err != nil {
		return domain.DismissScheduleEngagementAllocationResponse{}, err
	}
	return domain.DismissScheduleEngagementAllocationResponse{Days: days}, nil
}
