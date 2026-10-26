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
	"regexp"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

var slaTimeOfDayRE = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// SLACalendarService is the admin-facing surface of sla_calendar (R22-R27).
// There is no create/retire route -- only the two calendars migration 0246
// seeds exist (see this change's own plan file, "Calendars are two seeded
// rows") -- so this only edits a working week and manages a holiday list.
type SLACalendarService interface {
	List(ctx context.Context) ([]domain.SLACalendar, error)
	Get(ctx context.Context, id string) (domain.SLACalendar, error)
	UpdateWorkingWeek(ctx context.Context, id string, req domain.UpdateSLACalendarWorkingWeekRequest) (domain.SLACalendar, error)
	AddHoliday(ctx context.Context, calendarID string, req domain.AddSLAHolidayRequest) (domain.SLACalendar, error)
	DeleteHoliday(ctx context.Context, calendarID, holidayID string) error
}

type slaCalendarService struct {
	repo       repository.SLACalendarRepository
	access     AccessService
	adminRoles []string
}

// NewSLACalendarService constructs an SLACalendarService. adminRoles is
// AUTH_ADMIN_ROLES.
func NewSLACalendarService(repo repository.SLACalendarRepository, access AccessService, adminRoles []string) SLACalendarService {
	return &slaCalendarService{repo: repo, access: access, adminRoles: adminRoles}
}

func (s *slaCalendarService) List(ctx context.Context) ([]domain.SLACalendar, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *slaCalendarService) Get(ctx context.Context, id string) (domain.SLACalendar, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLACalendar{}, err
	}
	if id == "" {
		return domain.SLACalendar{}, &apierror.ValidationError{Msg: "calendar id is required"}
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SLACalendar{}, err
	}
	return s.repo.Get(ctx, id)
}

// validateWorkingWeek enforces R23: exactly one row per day of the week
// (0 = Sunday .. 6 = Saturday), each day named at most once, and a working
// day must carry a start and end time in HH:MM, with start before end.
func validateWorkingWeek(days []domain.SLAWorkingDay) error {
	if len(days) != 7 {
		return &apierror.ValidationError{Msg: "workingWeek must have exactly 7 entries, one per day of the week"}
	}
	seen := map[int]bool{}
	for _, d := range days {
		if d.Weekday < 0 || d.Weekday > 6 {
			return &apierror.ValidationError{Msg: "workingWeek.weekday must be 0 (Sunday) through 6 (Saturday)"}
		}
		if seen[d.Weekday] {
			return &apierror.ValidationError{Msg: "workingWeek must name each day of the week exactly once"}
		}
		seen[d.Weekday] = true
		if !d.IsWorking {
			continue
		}
		if d.StartTime == nil || d.EndTime == nil || !slaTimeOfDayRE.MatchString(*d.StartTime) || !slaTimeOfDayRE.MatchString(*d.EndTime) {
			return &apierror.ValidationError{Msg: "a working day needs startTime and endTime as HH:MM"}
		}
		if *d.StartTime >= *d.EndTime {
			return &apierror.ValidationError{Msg: "a working day's startTime must be before its endTime"}
		}
	}
	return nil
}

func (s *slaCalendarService) UpdateWorkingWeek(ctx context.Context, id string, req domain.UpdateSLACalendarWorkingWeekRequest) (domain.SLACalendar, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLACalendar{}, err
	}
	if id == "" {
		return domain.SLACalendar{}, &apierror.ValidationError{Msg: "calendar id is required"}
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SLACalendar{}, err
	}
	if err := validateWorkingWeek(req.WorkingWeek); err != nil {
		return domain.SLACalendar{}, err
	}
	return s.repo.UpdateWorkingWeek(ctx, id, req.WorkingWeek, slaActorEmail(ctx))
}

func (s *slaCalendarService) AddHoliday(ctx context.Context, calendarID string, req domain.AddSLAHolidayRequest) (domain.SLACalendar, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLACalendar{}, err
	}
	if calendarID == "" {
		return domain.SLACalendar{}, &apierror.ValidationError{Msg: "calendar id is required"}
	}
	if err := validateUUIDs("calendarId", []string{calendarID}); err != nil {
		return domain.SLACalendar{}, err
	}
	if req.StartDate == "" || req.EndDate == "" {
		return domain.SLACalendar{}, &apierror.ValidationError{Msg: "startDate and endDate are required"}
	}
	// Parsed, not string-compared: a string comparison only orders values
	// that already share the YYYY-MM-DD layout, so a malformed date (e.g.
	// "2026-1-5" or "tomorrow") would otherwise pass this check and only
	// fail later at the repository's own "$2::date" cast -- a 500, not the
	// 400 a bad request deserves.
	start, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return domain.SLACalendar{}, &apierror.ValidationError{Msg: "startDate must be YYYY-MM-DD"}
	}
	end, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return domain.SLACalendar{}, &apierror.ValidationError{Msg: "endDate must be YYYY-MM-DD"}
	}
	if end.Before(start) {
		return domain.SLACalendar{}, &apierror.ValidationError{Msg: "endDate must not be before startDate"}
	}
	if req.Name == "" {
		return domain.SLACalendar{}, &apierror.ValidationError{Msg: "name is required"}
	}
	return s.repo.AddHoliday(ctx, calendarID, req, slaActorEmail(ctx))
}

func (s *slaCalendarService) DeleteHoliday(ctx context.Context, calendarID, holidayID string) error {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return err
	}
	if calendarID == "" || holidayID == "" {
		return &apierror.ValidationError{Msg: "calendar id and holiday id are required"}
	}
	if err := validateUUIDs("calendarId/holidayId", []string{calendarID, holidayID}); err != nil {
		return err
	}
	return s.repo.DeleteHoliday(ctx, calendarID, holidayID)
}
