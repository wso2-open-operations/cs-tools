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
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// holidayTagLeadDays is how far ahead a Public holiday tag is placed: past
// the 14-day notice, so the notice can already say who is tagged.
const holidayTagLeadDays = 21

// holidayReminderLeadDays is how far ahead the reminders look.
const holidayReminderLeadDays = 14

// RotaWriterAuthorizer says whether the caller may change teamKey's rota:
// its lead, its rota manager, a rota admin of its family, or an admin. It is
// the schedule service's own check (ScheduleService.AuthorizeRotaWriter), so
// whoever may remove a member's tag may also confirm the team.
type RotaWriterAuthorizer func(ctx context.Context, teamKey string) error

// HolidayTagPlacer places the coming weeks' Public holiday tags. The public
// holiday refresh runs it after every successful refresh.
type HolidayTagPlacer interface {
	PlaceUpcoming(ctx context.Context) (domain.HolidayTagPlacementResult, error)
}

// HolidayAvailabilityService is working on a public holiday, CRE only.
//
// On a public holiday of their country a CRE member is off: the Team
// Schedule places a Public holiday tag on them (taking the day off the
// timesheet and them off the rota). A team lead or rota manager makes
// somebody available by removing their tag, then confirms the team. A team
// is fully marked when every rota turn it holds that day has somebody
// available and the team is confirmed; the reminders stop then.
type HolidayAvailabilityService interface {
	HolidayTagPlacer
	Search(ctx context.Context, req domain.SearchHolidayAvailabilityRequest) (domain.HolidayAvailabilityResponse, error)
	Confirm(ctx context.Context, teamKey, date string) (domain.HolidayAvailability, error)
	Withdraw(ctx context.Context, teamKey, date string) (domain.HolidayAvailability, error)
	// DueReminders is what the daily reminder task should send today.
	DueReminders(ctx context.Context, req domain.DueHolidayRemindersRequest) (domain.DueHolidayRemindersResponse, error)
	RecordReminder(ctx context.Context, teamKey, date, stage string) error
}

type holidayAvailabilityService struct {
	repo      repository.HolidayAvailabilityRepository
	holidays  repository.PublicHolidayRepository
	access    AccessService
	authorize RotaWriterAuthorizer
	zone      *time.Location
	now       func() time.Time
}

// NewHolidayAvailabilityService builds the service. zone is the rota's own
// calendar ("today"), Asia/Colombo.
func NewHolidayAvailabilityService(repo repository.HolidayAvailabilityRepository, holidays repository.PublicHolidayRepository,
	access AccessService, authorize RotaWriterAuthorizer, zone *time.Location) HolidayAvailabilityService {
	if zone == nil {
		zone = time.UTC
	}
	return &holidayAvailabilityService{repo: repo, holidays: holidays, access: access, authorize: authorize, zone: zone, now: time.Now}
}

// today is the current date on the rota's own calendar, as midnight UTC.
func (s *holidayAvailabilityService) today() time.Time {
	n := s.now().In(s.zone)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// PlaceUpcoming removes tags of cancelled holidays, then places a holiday tag on
// each CRE member for every weekday holiday of their country in the lead window.
func (s *holidayAvailabilityService) PlaceUpcoming(ctx context.Context) (domain.HolidayTagPlacementResult, error) {
	res := domain.HolidayTagPlacementResult{}
	from := s.today()
	to := from.AddDate(0, 0, holidayTagLeadDays)

	removed, err := s.repo.RemoveCancelledHolidayTags(ctx, from.Format(time.DateOnly))
	if err != nil {
		return res, err
	}
	res.Removed = removed

	members, err := s.repo.CREMembers(ctx)
	if err != nil {
		return res, err
	}
	indiaKey, err := s.holidays.IndiaTeamKey(ctx)
	if err != nil {
		return res, err
	}
	byCountry := map[string][]domain.SchedulePerson{}
	for _, p := range members {
		if len(p.TeamKeys) == 0 {
			continue
		}
		c, _ := holidayCountryFor(p, indiaKey)
		byCountry[c] = append(byCountry[c], p)
	}
	countries := make([]string, 0, len(byCountry))
	for c := range byCountry {
		countries = append(countries, c)
	}
	sort.Strings(countries)
	days, err := s.holidays.HolidaysBetween(ctx, countries, from, to)
	if err != nil {
		return res, err
	}
	for _, d := range days {
		day, err := time.Parse(time.DateOnly, d.Date)
		if err != nil || isWeekend(day) {
			// A weekend holiday takes no working day: the weekend rota is
			// marked as every weekend is.
			continue
		}
		for _, p := range byCountry[d.Country] {
			placed, err := s.repo.PlaceHolidayTag(ctx, domain.HolidayTagPlacement{
				UserID: p.UserID, TeamKey: p.TeamKeys[0], Date: d.Date, Country: d.Country, Title: d.Title,
			})
			if err != nil {
				return res, err
			}
			if placed {
				res.Placed++
			} else {
				res.Skipped++
			}
		}
	}
	return res, nil
}

// Search returns each team's availability on every holiday in the requested
// window, for internal staff only.
func (s *holidayAvailabilityService) Search(ctx context.Context, req domain.SearchHolidayAvailabilityRequest) (domain.HolidayAvailabilityResponse, error) {
	empty := domain.HolidayAvailabilityResponse{Holidays: []domain.HolidayAvailability{}}
	if err := RequireInternalCaller(ctx, s.access, "the team schedule is only available to internal staff"); err != nil {
		return empty, err
	}
	from, to, err := parseWindow(req.From, req.To)
	if err != nil {
		return empty, err
	}
	out, err := s.availability(ctx, from.Format(time.DateOnly), to.Format(time.DateOnly), req.TeamKeys, true)
	if err != nil {
		return empty, err
	}
	return domain.HolidayAvailabilityResponse{Holidays: out, Count: len(out)}, nil
}

// availability builds every team's availability on every holiday in
// [from, to] from the placed tags, the rota and the confirmations. withAccess
// fills CanConfirm for the caller.
func (s *holidayAvailabilityService) availability(ctx context.Context, from, to string, teamKeys []string, withAccess bool) ([]domain.HolidayAvailability, error) {
	placed, err := s.repo.PlacedBetween(ctx, from, to, teamKeys)
	if err != nil {
		return nil, err
	}
	type key struct{ team, date string }
	index := map[key]int{}
	out := []domain.HolidayAvailability{}
	teams := map[string]string{}
	for _, p := range placed {
		k := key{strings.ToLower(p.TeamKey), p.Date}
		i, ok := index[k]
		if !ok {
			i = len(out)
			index[k] = i
			out = append(out, domain.HolidayAvailability{
				TeamKey: p.TeamKey, TeamName: p.TeamName, Date: p.Date,
				Titles: []string{}, Countries: []string{},
				Members: []domain.HolidayAvailabilityMember{}, Uncovered: []domain.HolidayAvailabilityTurn{},
			})
			teams[k.team] = p.TeamKey
		}
		h := &out[i]
		if !containsFold(h.Titles, p.Title) {
			h.Titles = append(h.Titles, p.Title)
		}
		if !containsFold(h.Countries, p.Country) {
			h.Countries = append(h.Countries, p.Country)
		}
		m := domain.HolidayAvailabilityMember{UserID: p.UserID, Name: p.Name, Email: p.Email}
		switch {
		case p.AbsenceID != "":
			m.Status, m.AbsenceID = domain.HolidayMemberTagged, p.AbsenceID
		case p.OtherKind != "":
			m.Status, m.AwayKind = domain.HolidayMemberAway, p.OtherKind
		default:
			m.Status = domain.HolidayMemberAvailable
		}
		h.Members = append(h.Members, m)
	}
	if len(out) == 0 {
		return out, nil
	}
	keys := make([]string, 0, len(teams))
	for _, k := range teams {
		keys = append(keys, k)
	}
	turns, err := s.repo.TurnsBetween(ctx, from, to, keys)
	if err != nil {
		return nil, err
	}
	for _, t := range turns {
		i, ok := index[key{strings.ToLower(t.TeamKey), t.Date}]
		if !ok {
			continue
		}
		out[i].Turns++
		if t.Away {
			out[i].Uncovered = append(out[i].Uncovered, domain.HolidayAvailabilityTurn{
				ShiftCode: t.ShiftCode, ShiftLabel: t.ShiftLabel, UserID: t.UserID, Name: t.Name,
			})
		}
	}
	confirmations, err := s.repo.ConfirmationsBetween(ctx, from, to, keys)
	if err != nil {
		return nil, err
	}
	for _, c := range confirmations {
		i, ok := index[key{strings.ToLower(c.TeamKey), c.Date}]
		if !ok {
			continue
		}
		on := c.ConfirmedOn
		out[i].ConfirmedBy, out[i].ConfirmedOn = c.ConfirmedBy, &on
	}
	may := map[string]bool{}
	for i := range out {
		h := &out[i]
		confirmed := h.ConfirmedOn != nil
		switch {
		case len(h.Uncovered) > 0 && confirmed:
			h.Status = domain.HolidayAvailabilityNeedsAttention
		case len(h.Uncovered) > 0:
			h.Status = domain.HolidayAvailabilityNeedsMarking
		case confirmed:
			h.Status = domain.HolidayAvailabilityConfirmed
		default:
			h.Status = domain.HolidayAvailabilityAwaitingConfirmation
		}
		h.FullyMarked = h.Status == domain.HolidayAvailabilityConfirmed
		if withAccess {
			team := strings.ToLower(h.TeamKey)
			ok, seen := may[team]
			if !seen {
				ok = s.mayConfirm(ctx, h.TeamKey)
				may[team] = ok
			}
			h.CanConfirm = ok
		}
	}
	return out, nil
}

// mayConfirm is whether the caller, a person, may confirm teamKey.
func (s *holidayAvailabilityService) mayConfirm(ctx context.Context, teamKey string) bool {
	if auth.IdentityFromContext(ctx).UserEmail == "" || s.authorize == nil {
		return false
	}
	return s.authorize(ctx, teamKey) == nil
}

// one is the availability of teamKey on date, NotFound when no tag was
// placed for any of its members that day.
func (s *holidayAvailabilityService) one(ctx context.Context, teamKey, date string) (domain.HolidayAvailability, error) {
	list, err := s.availability(ctx, date, date, []string{teamKey}, true)
	if err != nil {
		return domain.HolidayAvailability{}, err
	}
	for _, h := range list {
		if strings.EqualFold(h.TeamKey, teamKey) && h.Date == date {
			return h, nil
		}
	}
	return domain.HolidayAvailability{}, &apierror.NotFoundError{
		Msg: fmt.Sprintf("no public holiday is marked for %s on %s", teamKey, date),
	}
}

// changeable checks a confirm or withdraw: a person who may change the
// team's rota, a holiday not yet past.
func (s *holidayAvailabilityService) changeable(ctx context.Context, teamKey, date string) error {
	if err := RequireInternalCaller(ctx, s.access, "the team schedule is only available to internal staff"); err != nil {
		return err
	}
	if auth.IdentityFromContext(ctx).UserEmail == "" {
		return &apierror.ForbiddenError{Msg: "confirming a team's holiday availability needs a user token, not a service credential"}
	}
	day, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return &apierror.ValidationError{Msg: fmt.Sprintf("date %q is not a YYYY-MM-DD date", date)}
	}
	if day.Before(s.today()) {
		return &apierror.ValidationError{Msg: "that holiday has passed"}
	}
	if strings.TrimSpace(teamKey) == "" {
		return &apierror.ValidationError{Msg: "teamKey is required"}
	}
	if s.authorize == nil {
		return &apierror.ForbiddenError{Msg: "confirming a team's holiday availability is not available"}
	}
	if err := s.authorize(ctx, teamKey); err != nil {
		var fe *apierror.ForbiddenError
		if errors.As(err, &fe) {
			return &apierror.ForbiddenError{Msg: fmt.Sprintf(
				"confirming %s's holiday availability needs a lead of that team, its rota manager, a CRE rota admin, or an admin", teamKey)}
		}
		return err
	}
	return nil
}

// Confirm records the team's availability on a holiday as confirmed, refusing
// while a rota turn that day has nobody available.
func (s *holidayAvailabilityService) Confirm(ctx context.Context, teamKey, date string) (domain.HolidayAvailability, error) {
	if err := s.changeable(ctx, teamKey, date); err != nil {
		return domain.HolidayAvailability{}, err
	}
	h, err := s.one(ctx, teamKey, date)
	if err != nil {
		return h, err
	}
	if n := len(h.Uncovered); n > 0 {
		names := make([]string, 0, n)
		for _, t := range h.Uncovered {
			names = append(names, fmt.Sprintf("%s (%s)", t.ShiftLabel, t.Name))
		}
		return h, &apierror.ConflictError{
			Code: "holiday_turns_uncovered",
			Msg: fmt.Sprintf("%d rota turn(s) on %s have nobody available: %s. Make the person available by removing their tag, or move the turn to somebody who works that day, then confirm.",
				n, date, strings.Join(names, ", ")),
		}
	}
	if _, err := s.repo.Confirm(ctx, h.TeamKey, date, auth.IdentityFromContext(ctx).UserEmail); err != nil {
		return h, err
	}
	return s.one(ctx, teamKey, date)
}

// Withdraw removes a team's confirmation for a holiday, returning NotFound when
// none was recorded.
func (s *holidayAvailabilityService) Withdraw(ctx context.Context, teamKey, date string) (domain.HolidayAvailability, error) {
	if err := s.changeable(ctx, teamKey, date); err != nil {
		return domain.HolidayAvailability{}, err
	}
	ok, err := s.repo.Withdraw(ctx, teamKey, date)
	if err != nil {
		return domain.HolidayAvailability{}, err
	}
	if !ok {
		return domain.HolidayAvailability{}, &apierror.NotFoundError{Msg: fmt.Sprintf("%s's availability on %s is not confirmed", teamKey, date)}
	}
	return s.one(ctx, teamKey, date)
}

// workingDaysBefore is the day n working days (Monday to Friday) before d.
func workingDaysBefore(d time.Time, n int) time.Time {
	for n > 0 {
		d = d.AddDate(0, 0, -1)
		if !isWeekend(d) {
			n--
		}
	}
	return d
}

// holidayReminderStage is the reminder stage due on today for a holiday on
// day: the latest one whose day has come, "" before the 14-day notice and
// from the holiday on. A stage whose day passed unsent is not caught up; the
// one due now says everything it would have.
func holidayReminderStage(today, day time.Time) string {
	if !today.Before(day) {
		return ""
	}
	stages := []struct {
		name string
		on   time.Time
	}{
		{domain.HolidayReminderNotice, day.AddDate(0, 0, -holidayReminderLeadDays)},
		{domain.HolidayReminderWeek, day.AddDate(0, 0, -7)},
		{domain.HolidayReminderTwoDays, workingDaysBefore(day, 2)},
		{domain.HolidayReminderOneDay, workingDaysBefore(day, 1)},
	}
	due := ""
	for _, st := range stages {
		if !st.on.After(today) {
			due = st.name
		}
	}
	return due
}

// validHolidayStage reports whether stage is one of the known reminder stages.
func validHolidayStage(stage string) bool {
	switch stage {
	case domain.HolidayReminderNotice, domain.HolidayReminderWeek, domain.HolidayReminderTwoDays, domain.HolidayReminderOneDay:
		return true
	}
	return false
}

// DueReminders lists the holiday reminders due today that have not been sent,
// with each team's recipients. The notice always goes; follow-ups only while
// the team is not fully marked.
func (s *holidayAvailabilityService) DueReminders(ctx context.Context, req domain.DueHolidayRemindersRequest) (domain.DueHolidayRemindersResponse, error) {
	resp := domain.DueHolidayRemindersResponse{Reminders: []domain.DueHolidayReminder{}}
	if err := RequireInternalCaller(ctx, s.access, "the team schedule is only available to internal staff"); err != nil {
		return resp, err
	}
	today := s.today()
	if req.Today != "" {
		d, err := time.Parse(time.DateOnly, req.Today)
		if err != nil {
			return resp, &apierror.ValidationError{Msg: fmt.Sprintf("today %q is not a YYYY-MM-DD date", req.Today)}
		}
		today = d
	}
	from := today.AddDate(0, 0, 1).Format(time.DateOnly)
	to := today.AddDate(0, 0, holidayReminderLeadDays).Format(time.DateOnly)
	list, err := s.availability(ctx, from, to, nil, false)
	if err != nil {
		return resp, err
	}
	sent, err := s.repo.SentStages(ctx, from, to)
	if err != nil {
		return resp, err
	}
	recipients := map[string][]domain.HolidayReminderRecipient{}
	for _, h := range list {
		day, err := time.Parse(time.DateOnly, h.Date)
		if err != nil {
			continue
		}
		stage := holidayReminderStage(today, day)
		if stage == "" {
			continue
		}
		// The notice always goes; the follow-ups only while the team is
		// not fully marked.
		if stage != domain.HolidayReminderNotice && h.FullyMarked {
			continue
		}
		if containsFold(sent[repository.HolidayStageKey(h.TeamKey, h.Date)], stage) {
			continue
		}
		team := strings.ToLower(h.TeamKey)
		who, ok := recipients[team]
		if !ok {
			who, err = s.repo.ReminderRecipients(ctx, h.TeamKey)
			if err != nil {
				return resp, err
			}
			recipients[team] = who
		}
		resp.Reminders = append(resp.Reminders, domain.DueHolidayReminder{
			Stage: stage, DaysBefore: int(day.Sub(today).Hours() / 24), Availability: h, Recipients: who,
		})
	}
	return resp, nil
}

// RecordReminder validates the team, date and stage, then records that the
// reminder for that stage was sent.
func (s *holidayAvailabilityService) RecordReminder(ctx context.Context, teamKey, date, stage string) error {
	if err := RequireInternalCaller(ctx, s.access, "the team schedule is only available to internal staff"); err != nil {
		return err
	}
	if strings.TrimSpace(teamKey) == "" {
		return &apierror.ValidationError{Msg: "teamKey is required"}
	}
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return &apierror.ValidationError{Msg: fmt.Sprintf("date %q is not a YYYY-MM-DD date", date)}
	}
	if !validHolidayStage(stage) {
		return &apierror.ValidationError{Msg: fmt.Sprintf("stage %q is not one of notice, week, two_days, one_day", stage)}
	}
	return s.repo.RecordReminder(ctx, teamKey, date, stage)
}

// placeAfterRefresh runs the placer after a refresh, logging what it did. A
// failure is logged, never the refresh's: the holidays are stored either way.
func placeAfterRefresh(ctx context.Context, placer HolidayTagPlacer) *domain.HolidayTagPlacementResult {
	if placer == nil {
		return nil
	}
	res, err := placer.PlaceUpcoming(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "public holidays: placing the holiday tags failed", "err", err)
		return nil
	}
	slog.InfoContext(ctx, "public holidays: holiday tags placed", "placed", res.Placed, "skipped", res.Skipped, "removed", res.Removed)
	return &res
}
