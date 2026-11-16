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
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/holidays"
)

const (
	lkUser = "11111111-1111-1111-1111-111111111111"
	inUser = "22222222-2222-2222-2222-222222222222"
	frUser = "33333333-3333-3333-3333-333333333333"
)

type fakePublicHolidayRepo struct {
	people    []domain.SchedulePerson
	indiaKey  string
	stored    map[string][]domain.PublicHolidayDay // country -> days
	locations map[string]string
	replaced  []string // "country:from"
}

// SchedulePeople returns the fake's people, with stored locations applied, limited to ids when given.
func (f *fakePublicHolidayRepo) SchedulePeople(_ context.Context, ids []string) ([]domain.SchedulePerson, error) {
	var out []domain.SchedulePerson
	for _, p := range f.people {
		if loc, ok := f.locations[p.UserID]; ok {
			p.Location = loc
		}
		if len(ids) == 0 {
			out = append(out, p)
			continue
		}
		for _, id := range ids {
			if id == p.UserID {
				out = append(out, p)
			}
		}
	}
	return out, nil
}

// IndiaTeamKey returns the fake's configured India team key.
func (f *fakePublicHolidayRepo) IndiaTeamKey(context.Context) (string, error) { return f.indiaKey, nil }

// UpsertPersonLocation remembers the person's location in the fake.
func (f *fakePublicHolidayRepo) UpsertPersonLocation(_ context.Context, userID, loc string, _ time.Time) error {
	if f.locations == nil {
		f.locations = map[string]string{}
	}
	f.locations[userID] = loc
	return nil
}

// ReplaceCountryRange records the replacement and appends the days to the country's stored holidays.
func (f *fakePublicHolidayRepo) ReplaceCountryRange(_ context.Context, country string, from, _ time.Time, days []domain.PublicHolidayDay, _ time.Time) (int, error) {
	if f.stored == nil {
		f.stored = map[string][]domain.PublicHolidayDay{}
	}
	f.replaced = append(f.replaced, country+":"+from.Format("2006"))
	f.stored[country] = append(f.stored[country], days...)
	return len(days), nil
}

// HolidaysBetween returns the stored holidays of the given countries dated in the range, sorted by date.
func (f *fakePublicHolidayRepo) HolidaysBetween(_ context.Context, countries []string, from, to time.Time) ([]domain.PublicHolidayDay, error) {
	var out []domain.PublicHolidayDay
	for _, c := range countries {
		for _, d := range f.stored[c] {
			if d.Date >= from.Format("2006-01-02") && d.Date <= to.Format("2006-01-02") {
				out = append(out, d)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

type fakeCalendar struct {
	byCountry map[string][]holidays.Holiday
	fail      map[string]bool
	calls     []string
}

// Holidays records the call and returns the country's holidays for the year, or an error when the country is set to fail.
func (f *fakeCalendar) Holidays(_ context.Context, country string, from, _ time.Time) ([]holidays.Holiday, error) {
	f.calls = append(f.calls, country+":"+from.Format("2006"))
	if f.fail[country] {
		return nil, errors.New("down")
	}
	var out []holidays.Holiday
	for _, h := range f.byCountry[country] {
		if h.Date[:4] == from.Format("2006") {
			out = append(out, h)
		}
	}
	return out, nil
}

type fakeEmployees map[string]string // email -> location ("!" = error)

// Location returns the email's location, not-found when absent, or an error when the location is "!".
func (f fakeEmployees) Location(_ context.Context, email string) (string, error) {
	loc, ok := f[email]
	if !ok {
		return "", holidays.ErrEmployeeNotFound
	}
	if loc == "!" {
		return "", errors.New("hr down")
	}
	return loc, nil
}

// basePeople returns three test people: two on the nova team and one on the phoenix team.
func basePeople() []domain.SchedulePerson {
	return []domain.SchedulePerson{
		{UserID: lkUser, Email: "lk@example.com", TeamKeys: []string{"nova"}},
		{UserID: inUser, Email: "in@example.com", TeamKeys: []string{"phoenix"}},
		{UserID: frUser, Email: "fr@example.com", TeamKeys: []string{"nova"}},
	}
}

// TestHolidayCountryFor checks that a stored location wins, then the India team, then the Sri Lanka fallback.
func TestHolidayCountryFor(t *testing.T) {
	cases := []struct {
		p       domain.SchedulePerson
		india   string
		country string
		source  string
	}{
		{domain.SchedulePerson{Location: "France", TeamKeys: []string{"phoenix"}}, "phoenix", "France", domain.PublicHolidayCountryFromEmployee},
		{domain.SchedulePerson{TeamKeys: []string{"nova", "phoenix"}}, "phoenix", domain.PublicHolidayCountryIndia, domain.PublicHolidayCountryFromTeam},
		{domain.SchedulePerson{TeamKeys: []string{"nova"}}, "phoenix", domain.PublicHolidayCountrySriLanka, domain.PublicHolidayCountryFromTeam},
		{domain.SchedulePerson{TeamKeys: []string{"phoenix"}}, "", domain.PublicHolidayCountrySriLanka, domain.PublicHolidayCountryFromTeam},
	}
	for i, tc := range cases {
		c, s := holidayCountryFor(tc.p, tc.india)
		if c != tc.country || s != tc.source {
			t.Errorf("case %d: got %s/%s, want %s/%s", i, c, s, tc.country, tc.source)
		}
	}
}

// TestPublicHolidayRefresh_LocationsCountriesAndYears checks that a refresh updates locations, covers the
// right countries and two years, and keeps a year that comes back empty.
func TestPublicHolidayRefresh_LocationsCountriesAndYears(t *testing.T) {
	repo := &fakePublicHolidayRepo{people: basePeople(), indiaKey: "phoenix"}
	cal := &fakeCalendar{byCountry: map[string][]holidays.Holiday{
		"Sri Lanka": {{Date: "2026-01-14", Title: "Thai Pongal"}, {Date: "2027-01-15", Title: "Thai Pongal"}},
		"India":     {{Date: "2026-01-26", Title: "Republic Day"}},
		"France":    {{Date: "2026-07-14", Title: "Fête nationale"}},
	}}
	emp := fakeEmployees{"fr@example.com": "France", "lk@example.com": "!"}
	svc := NewPublicHolidayService(repo, alwaysUnrestrictedAccess{}, cal, emp).(*publicHolidayService)
	svc.now = func() time.Time { return time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC) }

	res, err := svc.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !reflect.DeepEqual(res.Countries, []string{"France", "India", "Sri Lanka"}) {
		t.Errorf("countries %v", res.Countries)
	}
	if !reflect.DeepEqual(res.Years, []int{2026, 2027}) {
		t.Errorf("years %v", res.Years)
	}
	if res.LocationsUpdated != 1 || res.LocationsFailed != 1 || repo.locations[frUser] != "France" {
		t.Errorf("locations: %+v, stored %v", res, repo.locations)
	}
	// India 2027 and France 2027 come back empty: kept, not replaced.
	if res.CountryYearsEmpty != 2 || res.HolidaysStored != 4 {
		t.Errorf("empty=%d stored=%d", res.CountryYearsEmpty, res.HolidaysStored)
	}
	if len(cal.calls) != 6 {
		t.Errorf("calendar calls %v", cal.calls)
	}
	if svc.LastRefresh() == nil {
		t.Error("LastRefresh not recorded")
	}
}

// TestPublicHolidayRefresh_CalendarFailureKeepsStoredDays checks that a failed calendar read counts as a
// failure and leaves the stored days for that country untouched.
func TestPublicHolidayRefresh_CalendarFailureKeepsStoredDays(t *testing.T) {
	repo := &fakePublicHolidayRepo{people: basePeople(), indiaKey: "phoenix",
		stored: map[string][]domain.PublicHolidayDay{"India": {{Country: "India", Date: "2026-01-26", Title: "Republic Day"}}}}
	cal := &fakeCalendar{fail: map[string]bool{"India": true}}
	svc := NewPublicHolidayService(repo, alwaysUnrestrictedAccess{}, cal, nil)
	res, err := svc.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if res.CountryYearsFailed != 2 || len(repo.stored["India"]) != 1 {
		t.Fatalf("failed=%d stored=%v", res.CountryYearsFailed, repo.stored["India"])
	}
	for _, r := range repo.replaced {
		if r[:5] == "India" {
			t.Fatalf("India replaced after a failed read: %v", repo.replaced)
		}
	}
}

// TestPublicHolidayRefresh_UnconfiguredWritesNothing checks that with no calendar or employee source a
// refresh writes nothing and falls back to Sri Lanka.
func TestPublicHolidayRefresh_UnconfiguredWritesNothing(t *testing.T) {
	repo := &fakePublicHolidayRepo{people: basePeople()}
	res, err := NewPublicHolidayService(repo, alwaysUnrestrictedAccess{}, nil, nil).Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if res.CalendarConfigured || res.EmployeesConfigured || len(repo.replaced) != 0 {
		t.Fatalf("got %+v replaced %v", res, repo.replaced)
	}
	// No India team in the catalogue: everyone falls back to Sri Lanka.
	if !reflect.DeepEqual(res.Countries, []string{"Sri Lanka"}) {
		t.Fatalf("countries %v", res.Countries)
	}
}

// TestPublicHolidayRefresh_OneAtATime checks that while a refresh is running, a second one is refused
// with a conflict and a background one does not start.
func TestPublicHolidayRefresh_OneAtATime(t *testing.T) {
	svc := NewPublicHolidayService(&fakePublicHolidayRepo{}, alwaysUnrestrictedAccess{}, nil, nil).(*publicHolidayService)
	svc.running.Lock()
	defer svc.running.Unlock()
	var ce *apierror.ConflictError
	if _, err := svc.Refresh(context.Background()); !errors.As(err, &ce) {
		t.Fatalf("want ConflictError, got %v", err)
	}
	if svc.RefreshInBackground() {
		t.Fatal("RefreshInBackground started while a run was in progress")
	}
}

// TestPublicHolidaySearch_OnlyThePeopleItAppliesTo checks that Search returns each holiday only for the
// people of its country, sorted by date, with where the country came from.
func TestPublicHolidaySearch_OnlyThePeopleItAppliesTo(t *testing.T) {
	repo := &fakePublicHolidayRepo{people: basePeople(), indiaKey: "phoenix",
		locations: map[string]string{frUser: "France"},
		stored: map[string][]domain.PublicHolidayDay{
			"Sri Lanka": {{Country: "Sri Lanka", Date: "2026-11-24", Title: "Il Full Moon Poya Day"}},
			"India":     {{Country: "India", Date: "2026-11-24", Title: "Guru Nanak Jayanti"}},
			"France":    {{Country: "France", Date: "2026-11-11", Title: "Armistice"}},
		}}
	svc := NewPublicHolidayService(repo, alwaysUnrestrictedAccess{}, nil, nil)
	resp, err := svc.Search(context.Background(), domain.SearchSchedulePublicHolidaysRequest{
		From: "2026-11-01", To: "2026-11-30", UserIDs: []string{lkUser, inUser, frUser},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	want := []domain.SchedulePublicHoliday{
		{UserID: frUser, Date: "2026-11-11", Title: "Armistice", Country: "France", CountrySource: domain.PublicHolidayCountryFromEmployee},
		{UserID: lkUser, Date: "2026-11-24", Title: "Il Full Moon Poya Day", Country: "Sri Lanka", CountrySource: domain.PublicHolidayCountryFromTeam},
		{UserID: inUser, Date: "2026-11-24", Title: "Guru Nanak Jayanti", Country: "India", CountrySource: domain.PublicHolidayCountryFromTeam},
	}
	if !reflect.DeepEqual(resp.Holidays, want) || resp.Count != 3 {
		t.Fatalf("got %+v", resp.Holidays)
	}
}

// TestPublicHolidaySearch_Validation checks that Search rejects missing or malformed user ids and bad
// windows, and refuses a restricted caller.
func TestPublicHolidaySearch_Validation(t *testing.T) {
	svc := NewPublicHolidayService(&fakePublicHolidayRepo{}, alwaysUnrestrictedAccess{}, nil, nil)
	bad := []domain.SearchSchedulePublicHolidaysRequest{
		{From: "2026-11-01", To: "2026-11-30"},
		{From: "2026-11-01", To: "2026-11-30", UserIDs: []string{"not-a-uuid"}},
		{From: "2026-11-01", To: "2027-03-30", UserIDs: []string{lkUser}},
		{To: "2026-11-30", UserIDs: []string{lkUser}},
	}
	for i, req := range bad {
		var ve *apierror.ValidationError
		if _, err := svc.Search(context.Background(), req); !errors.As(err, &ve) {
			t.Errorf("case %d: want ValidationError, got %v", i, err)
		}
	}
	if _, err := NewPublicHolidayService(&fakePublicHolidayRepo{}, restrictedAccess{}, nil, nil).Search(context.Background(),
		domain.SearchSchedulePublicHolidaysRequest{From: "2026-11-01", To: "2026-11-02", UserIDs: []string{lkUser}}); err == nil {
		t.Error("restricted caller: want an error")
	}
}
