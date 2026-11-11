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
	"errors"
	"log/slog"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/holidays"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// maxPublicHolidayUsers bounds one search, like the allocation search.
const maxPublicHolidayUsers = 500

// publicHolidayRefreshTimeout bounds one whole refresh run: one employee
// lookup per member plus two calendar reads per country.
const publicHolidayRefreshTimeout = 10 * time.Minute

// HolidayCalendar is the calendar-events service as the refresh reads it;
// *holidays.CalendarClient satisfies it.
type HolidayCalendar interface {
	Holidays(ctx context.Context, country string, from, to time.Time) ([]holidays.Holiday, error)
}

// EmployeeLocations is the employee service as the refresh reads it;
// *holidays.EmployeeClient satisfies it.
type EmployeeLocations interface {
	Location(ctx context.Context, email string) (string, error)
}

// PublicHolidayService serves the Team Schedule's public holidays: the
// roster's read, which only ever reads the local copy, and the refresh that
// fills that copy from the calendar-events and employee services.
//
// A holiday is not leave and moves no shift by itself. For CRE members the
// refresh then places a Public holiday tag on each holiday (see
// HolidayAvailabilityService): that tag takes the day off the timesheet and
// the member off the rota until a lead removes it.
type PublicHolidayService interface {
	Search(ctx context.Context, req domain.SearchSchedulePublicHolidaysRequest) (domain.SchedulePublicHolidaysResponse, error)
	// Refresh fetches the members' locations and the current and next
	// year's holidays of every country in use. Safe to re-run; a run already
	// in progress makes this a ConflictError.
	Refresh(ctx context.Context) (domain.PublicHolidayRefreshResult, error)
	// RefreshInBackground starts Refresh on its own context and returns at
	// once; false when one is already running.
	RefreshInBackground() bool
	// LastRefresh is the last finished run, nil before the first.
	LastRefresh() *domain.PublicHolidayRefreshResult
}

type publicHolidayService struct {
	repo      repository.PublicHolidayRepository
	access    AccessService
	calendar  HolidayCalendar
	employees EmployeeLocations
	now       func() time.Time

	// placer, when set, places the coming weeks' Public holiday tags after
	// every successful refresh (WithHolidayTagPlacer).
	placer HolidayTagPlacer

	running sync.Mutex
	mu      sync.Mutex
	last    *domain.PublicHolidayRefreshResult
}

// WithHolidayTagPlacer makes every successful refresh place the coming weeks'
// Public holiday tags (CRE). Applied after construction so the constructor's
// callers keep compiling; a service it does not recognise is returned
// unchanged.
func WithHolidayTagPlacer(svc PublicHolidayService, placer HolidayTagPlacer) PublicHolidayService {
	if s, ok := svc.(*publicHolidayService); ok {
		s.placer = placer
	}
	return svc
}

// NewPublicHolidayService builds the service. calendar and employees may be
// nil (unconfigured): the read still works from the stored copy, and the
// team fallback decides every country.
func NewPublicHolidayService(repo repository.PublicHolidayRepository, access AccessService, calendar HolidayCalendar, employees EmployeeLocations) PublicHolidayService {
	return &publicHolidayService{repo: repo, access: access, calendar: calendar, employees: employees, now: time.Now}
}

// holidayCountryFor is whose holidays a person keeps: their employee-service
// location, else their team's country -- India for the India team, Sri
// Lanka for every other team.
func holidayCountryFor(p domain.SchedulePerson, indiaTeamKey string) (country, source string) {
	if p.Location != "" {
		return p.Location, domain.PublicHolidayCountryFromEmployee
	}
	if indiaTeamKey != "" && slices.Contains(p.TeamKeys, indiaTeamKey) {
		return domain.PublicHolidayCountryIndia, domain.PublicHolidayCountryFromTeam
	}
	return domain.PublicHolidayCountrySriLanka, domain.PublicHolidayCountryFromTeam
}

// Search returns the stored public holidays that apply to the requested people
// in the window, each with the country it was chosen by and where that came from.
func (s *publicHolidayService) Search(ctx context.Context, req domain.SearchSchedulePublicHolidaysRequest) (domain.SchedulePublicHolidaysResponse, error) {
	empty := domain.SchedulePublicHolidaysResponse{Holidays: []domain.SchedulePublicHoliday{}}
	if err := RequireInternalCaller(ctx, s.access, "the team schedule is only available to internal staff"); err != nil {
		return empty, err
	}
	from, to, err := parseWindow(req.From, req.To)
	if err != nil {
		return empty, err
	}
	if len(req.UserIDs) == 0 {
		return empty, &apierror.ValidationError{Msg: "userIds is required"}
	}
	if len(req.UserIDs) > maxPublicHolidayUsers {
		return empty, &apierror.ValidationError{Msg: "too many userIds"}
	}
	for _, id := range req.UserIDs {
		if id == "" {
			return empty, &apierror.ValidationError{Msg: "userIds must not contain an empty id"}
		}
		if err := validateUserID(id); err != nil {
			return empty, err
		}
	}
	people, err := s.repo.SchedulePeople(ctx, req.UserIDs)
	if err != nil {
		return empty, err
	}
	indiaKey, err := s.repo.IndiaTeamKey(ctx)
	if err != nil {
		return empty, err
	}
	type who struct{ userID, source string }
	byCountry := map[string][]who{}
	for _, p := range people {
		c, src := holidayCountryFor(p, indiaKey)
		byCountry[c] = append(byCountry[c], who{p.UserID, src})
	}
	countries := make([]string, 0, len(byCountry))
	for c := range byCountry {
		countries = append(countries, c)
	}
	sort.Strings(countries)
	days, err := s.repo.HolidaysBetween(ctx, countries, from, to)
	if err != nil {
		return empty, err
	}
	out := []domain.SchedulePublicHoliday{}
	for _, d := range days {
		for _, w := range byCountry[d.Country] {
			out = append(out, domain.SchedulePublicHoliday{
				UserID: w.userID, Date: d.Date, Title: d.Title, Country: d.Country, CountrySource: w.source,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].UserID < out[j].UserID
	})
	return domain.SchedulePublicHolidaysResponse{Holidays: out, Count: len(out)}, nil
}

// Refresh runs one holiday refresh and records its result, refusing with a
// conflict when another refresh is already running.
func (s *publicHolidayService) Refresh(ctx context.Context) (domain.PublicHolidayRefreshResult, error) {
	if !s.running.TryLock() {
		return domain.PublicHolidayRefreshResult{}, &apierror.ConflictError{Msg: "a public holiday refresh is already running"}
	}
	defer s.running.Unlock()
	return s.refreshAndRecord(ctx)
}

// refresh updates stored member locations, works out the countries in use, and
// replaces each country's holidays for this and next year from the calendar.
// A failed or empty country-year keeps what is stored.
func (s *publicHolidayService) refresh(ctx context.Context) (domain.PublicHolidayRefreshResult, error) {
	now := s.now().UTC()
	res := domain.PublicHolidayRefreshResult{
		CalendarConfigured:  s.calendar != nil,
		EmployeesConfigured: s.employees != nil,
		Countries:           []string{},
		Years:               []int{},
		RanAt:               now,
	}
	people, err := s.repo.SchedulePeople(ctx, nil)
	if err != nil {
		return res, err
	}
	res.People = len(people)

	if s.employees != nil {
		for i := range people {
			p := &people[i]
			if p.Email == "" {
				continue
			}
			loc, err := s.employees.Location(ctx, p.Email)
			if err != nil {
				if ctx.Err() != nil {
					return res, ctx.Err()
				}
				if !errors.Is(err, holidays.ErrEmployeeNotFound) {
					res.LocationsFailed++
					slog.WarnContext(ctx, "public holidays: employee location lookup failed", "userId", p.UserID, "err", err)
				}
				continue // keep the stored location, or the team fallback
			}
			if loc == "" || loc == p.Location {
				continue
			}
			if err := s.repo.UpsertPersonLocation(ctx, p.UserID, loc, now); err != nil {
				return res, err
			}
			p.Location = loc
			res.LocationsUpdated++
		}
	}

	indiaKey, err := s.repo.IndiaTeamKey(ctx)
	if err != nil {
		return res, err
	}
	set := map[string]bool{
		// The two countries the teams fall back to are always kept fresh.
		domain.PublicHolidayCountrySriLanka: true,
	}
	if indiaKey != "" {
		set[domain.PublicHolidayCountryIndia] = true
	}
	for _, p := range people {
		c, _ := holidayCountryFor(p, indiaKey)
		set[c] = true
	}
	for c := range set {
		res.Countries = append(res.Countries, c)
	}
	sort.Strings(res.Countries)
	res.Years = []int{now.Year(), now.Year() + 1}

	if s.calendar == nil {
		return res, nil
	}
	for _, country := range res.Countries {
		for _, year := range res.Years {
			from := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
			hs, err := s.calendar.Holidays(ctx, country, from, to)
			if err != nil {
				if ctx.Err() != nil {
					return res, ctx.Err()
				}
				res.CountryYearsFailed++
				slog.WarnContext(ctx, "public holidays: calendar read failed; keeping the stored days", "country", country, "year", year, "err", err)
				continue
			}
			if len(hs) == 0 {
				// Not published yet, or a transient empty answer: never let
				// it wipe a year that was stored.
				res.CountryYearsEmpty++
				continue
			}
			days := make([]domain.PublicHolidayDay, 0, len(hs))
			for _, h := range hs {
				days = append(days, domain.PublicHolidayDay{Country: country, Date: h.Date, Title: h.Title})
			}
			n, err := s.repo.ReplaceCountryRange(ctx, country, from, to, days, now)
			if err != nil {
				return res, err
			}
			res.HolidaysStored += n
		}
	}
	return res, nil
}

// RefreshInBackground starts a refresh in a goroutine and returns true, or
// returns false without starting one when a refresh is already running.
func (s *publicHolidayService) RefreshInBackground() bool {
	if !s.running.TryLock() {
		return false
	}
	go func() {
		defer s.running.Unlock()
		ctx, cancel := context.WithTimeout(repository.WithSystemIdentity(context.Background()), publicHolidayRefreshTimeout)
		defer cancel()
		_, err := s.refreshAndRecord(ctx)
		s.logRun(ctx, err)
	}()
	return true
}

// refreshAndRecord runs a refresh and, when it succeeds, places holiday tags
// and keeps the result as the last refresh.
func (s *publicHolidayService) refreshAndRecord(ctx context.Context) (domain.PublicHolidayRefreshResult, error) {
	res, err := s.refresh(ctx)
	if err == nil {
		res.Tags = placeAfterRefresh(ctx, s.placer)
		s.mu.Lock()
		s.last = &res
		s.mu.Unlock()
	}
	return res, err
}

// logRun logs the outcome of a background refresh: the error, or a summary of
// the last stored result.
func (s *publicHolidayService) logRun(ctx context.Context, err error) {
	if err != nil {
		slog.ErrorContext(ctx, "public holidays: refresh failed", "err", err)
		return
	}
	if last := s.LastRefresh(); last != nil {
		slog.InfoContext(ctx, "public holidays: refreshed",
			"people", last.People, "locationsUpdated", last.LocationsUpdated, "locationsFailed", last.LocationsFailed,
			"countries", last.Countries, "stored", last.HolidaysStored, "failed", last.CountryYearsFailed, "empty", last.CountryYearsEmpty)
	}
}

// LastRefresh returns a copy of the most recent successful refresh result, or
// nil when none has completed yet.
func (s *publicHolidayService) LastRefresh() *domain.PublicHolidayRefreshResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		return nil
	}
	cp := *s.last
	return &cp
}

// RunPublicHolidayRefresh refreshes once after firstAfter, then every
// interval, until ctx ends. Every replica runs it; a run is idempotent.
func RunPublicHolidayRefresh(ctx context.Context, svc PublicHolidayService, firstAfter, interval time.Duration) {
	timer := time.NewTimer(firstAfter)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		runCtx, cancel := context.WithTimeout(repository.WithSystemIdentity(ctx), publicHolidayRefreshTimeout)
		res, err := svc.Refresh(runCtx)
		cancel()
		switch {
		case err != nil:
			slog.Error("public holidays: daily refresh failed", "err", err)
		default:
			slog.Info("public holidays: daily refresh done",
				"people", res.People, "locationsUpdated", res.LocationsUpdated, "countries", res.Countries,
				"stored", res.HolidaysStored, "failed", res.CountryYearsFailed, "empty", res.CountryYearsEmpty)
		}
		timer.Reset(interval)
	}
}
