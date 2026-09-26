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
	"fmt"
	"regexp"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// maxScheduleWindowDays bounds a single rota read. The month roster is the
// widest view the UI has, so a little over two months leaves room for a month
// either side of a boundary without letting a client ask for a decade.
const maxScheduleWindowDays = 70

// ScheduleService serves the Team Schedule. It validates the window, applies
// the reading rules the schema deliberately does not encode, and leaves the
// plain data operations to the repository.
type ScheduleService interface {
	Catalogue(ctx context.Context) (domain.ScheduleCatalogue, error)
	SearchAssignments(ctx context.Context, req domain.SearchScheduleAssignmentsRequest) (domain.ScheduleAssignmentsResponse, error)
	SearchAbsences(ctx context.Context, req domain.SearchScheduleAbsencesRequest) (domain.ScheduleAbsencesResponse, error)
	OnDuty(ctx context.Context, at *time.Time) (domain.ScheduleAssignmentsResponse, error)
}

type scheduleService struct {
	repo   repository.ScheduleRepository
	access AccessService
}

// NewScheduleService constructs a ScheduleService over the given repository.
func NewScheduleService(repo repository.ScheduleRepository, access AccessService) ScheduleService {
	return &scheduleService{repo: repo, access: access}
}

// requireInternalCaller rejects anyone whose AccessScope is not Unrestricted.
// The rota is staff data -- who is working, who is on leave and where -- and
// belongs to no project, so there is no narrower scope a customer could be
// given: an internal caller sees all of it and anyone else sees none. Mirrors
// sla_status_service.go's helper of the same name.
func (s *scheduleService) requireInternalCaller(ctx context.Context) error {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: "the team schedule is only available to internal staff"}
	}
	return nil
}

func (s *scheduleService) Catalogue(ctx context.Context) (domain.ScheduleCatalogue, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleCatalogue{}, err
	}
	return s.repo.Catalogue(ctx)
}

// uuidPattern is the canonical 8-4-4-4-12 form user ids are issued in.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// validateUserID refuses a userId that is not a UUID before it reaches a
// uuid column, where Postgres would reject it as a 500 rather than the 400 a
// malformed filter deserves. Empty means "no filter" and passes.
func validateUserID(id string) error {
	if id != "" && !uuidPattern.MatchString(id) {
		return &apierror.ValidationError{Msg: fmt.Sprintf("userId %q is not a UUID", id)}
	}
	return nil
}

// parseWindow validates a from/to pair and returns it normalised. Both dates
// are required: an unbounded rota read would happily return every row in the
// table, which is nobody's intent and a slow way to find that out.
func parseWindow(from, to string) (time.Time, time.Time, error) {
	if from == "" || to == "" {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: "both from and to are required (YYYY-MM-DD)"}
	}
	f, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: fmt.Sprintf("from %q is not a YYYY-MM-DD date", from)}
	}
	t, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: fmt.Sprintf("to %q is not a YYYY-MM-DD date", to)}
	}
	if t.Before(f) {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: "to is before from"}
	}
	if t.Sub(f) > maxScheduleWindowDays*24*time.Hour {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: fmt.Sprintf("window is longer than %d days", maxScheduleWindowDays)}
	}
	return f, t, nil
}

func (s *scheduleService) SearchAssignments(ctx context.Context, req domain.SearchScheduleAssignmentsRequest) (domain.ScheduleAssignmentsResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	if _, _, err := parseWindow(req.From, req.To); err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	if err := validateUserID(req.UserID); err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	if req.Family != "" && req.Family != "CRE" && req.Family != "SRE" {
		return domain.ScheduleAssignmentsResponse{}, &apierror.ValidationError{Msg: "family must be CRE or SRE"}
	}
	rows, err := s.repo.SearchAssignments(ctx, req)
	if err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	return domain.ScheduleAssignmentsResponse{Assignments: rows, Count: len(rows)}, nil
}

func (s *scheduleService) SearchAbsences(ctx context.Context, req domain.SearchScheduleAbsencesRequest) (domain.ScheduleAbsencesResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleAbsencesResponse{}, err
	}
	if _, _, err := parseWindow(req.From, req.To); err != nil {
		return domain.ScheduleAbsencesResponse{}, err
	}
	if err := validateUserID(req.UserID); err != nil {
		return domain.ScheduleAbsencesResponse{}, err
	}
	rows, err := s.repo.SearchAbsences(ctx, req)
	if err != nil {
		return domain.ScheduleAbsencesResponse{}, err
	}
	return domain.ScheduleAbsencesResponse{Absences: rows, Count: len(rows)}, nil
}

// OnDuty answers who is responsible at an instant, defaulting to now. This is
// the lookup an alert escalation needs before it decides who to ring.
func (s *scheduleService) OnDuty(ctx context.Context, at *time.Time) (domain.ScheduleAssignmentsResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	moment := time.Now()
	if at != nil {
		moment = *at
	}
	rows, err := s.repo.OnDutyAt(ctx, moment)
	if err != nil {
		return domain.ScheduleAssignmentsResponse{}, err
	}
	return domain.ScheduleAssignmentsResponse{Assignments: rows, Count: len(rows)}, nil
}
