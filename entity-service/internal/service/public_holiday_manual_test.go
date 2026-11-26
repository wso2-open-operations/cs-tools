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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// signalPlacer counts placement runs and signals each one.
type signalPlacer struct{ ran chan struct{} }

// PlaceUpcoming signals the run and places nothing.
func (p *signalPlacer) PlaceUpcoming(context.Context) (domain.HolidayTagPlacementResult, error) {
	p.ran <- struct{}{}
	return domain.HolidayTagPlacementResult{}, nil
}

// waitPlaced fails the test unless the placer ran within a second.
func waitPlaced(t *testing.T, p *signalPlacer) {
	t.Helper()
	select {
	case <-p.ran:
	case <-time.After(time.Second):
		t.Fatal("the Public holiday tags were not placed after the edit")
	}
}

// manualHolidayService builds the service at 10 Nov 2026 with the given editor
// check and a placer that signals.
func manualHolidayService(repo *fakePublicHolidayRepo, editor HolidayEditorAuthorizer) (PublicHolidayService, *signalPlacer) {
	placer := &signalPlacer{ran: make(chan struct{}, 4)}
	svc := WithHolidayEditorCheck(WithHolidayTagPlacer(NewPublicHolidayService(repo, alwaysUnrestrictedAccess{}, nil, nil), placer), editor)
	svc.(*publicHolidayService).now = func() time.Time { return time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC) }
	return svc, placer
}

func allowEditor(context.Context) error { return nil }

func refuseEditor(context.Context) error { return errNotHolidayEditor }

// A portal or rota admin adds a country's days by hand: they are stored as
// manual rows under the caller's email, and the tags are placed right after.
func TestPublicHolidays_AddStoresManualDaysAndPlacesTags(t *testing.T) {
	repo := &fakePublicHolidayRepo{}
	svc, placer := manualHolidayService(repo, allowEditor)
	resp, err := svc.Add(scheduleCaller("rota.admin@example.com"), domain.AddPublicHolidaysRequest{
		Country: "  Sri Lanka ",
		Holidays: []domain.ManualPublicHoliday{
			{Date: "2026-12-25", Title: " Christmas Day "},
			{Date: "2027-01-14", Title: "Tamil Thai Pongal Day"},
		},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if resp.Stored != 2 {
		t.Errorf("stored = %d, want 2", resp.Stored)
	}
	want := []domain.PublicHolidayDay{
		{Country: "Sri Lanka", Date: "2026-12-25", Title: "Christmas Day"},
		{Country: "Sri Lanka", Date: "2027-01-14", Title: "Tamil Thai Pongal Day"},
	}
	if !reflect.DeepEqual(repo.manual, want) {
		t.Errorf("stored days = %+v, want %+v", repo.manual, want)
	}
	if repo.actor != "rota.admin@example.com" {
		t.Errorf("actor = %q, want the caller's email", repo.actor)
	}
	waitPlaced(t, placer)
}

// Everyone else is refused before anything is stored, and so is every caller
// when no editor check is wired.
func TestPublicHolidays_OnlyEditorsAddOrRemove(t *testing.T) {
	add := domain.AddPublicHolidaysRequest{Country: "India", Holidays: []domain.ManualPublicHoliday{{Date: "2027-01-26", Title: "Republic Day"}}}
	for name, editor := range map[string]HolidayEditorAuthorizer{"a lead": refuseEditor, "no check wired": nil} {
		t.Run(name, func(t *testing.T) {
			repo := &fakePublicHolidayRepo{}
			svc, _ := manualHolidayService(repo, editor)
			var forbidden *apierror.ForbiddenError
			if _, err := svc.Add(scheduleCaller("lead@example.com"), add); !errors.As(err, &forbidden) {
				t.Fatalf("Add: want ForbiddenError, got %v", err)
			}
			if err := svc.Remove(scheduleCaller("lead@example.com"), "India", "2027-01-26"); !errors.As(err, &forbidden) {
				t.Fatalf("Remove: want ForbiddenError, got %v", err)
			}
			if len(repo.manual) != 0 || len(repo.deleted) != 0 {
				t.Errorf("wrote %+v / %+v after a refusal", repo.manual, repo.deleted)
			}
		})
	}
}

// A bad add is refused whole, naming the day, and stores nothing.
func TestPublicHolidays_AddValidation(t *testing.T) {
	day := func(date, title string) []domain.ManualPublicHoliday {
		return []domain.ManualPublicHoliday{{Date: date, Title: title}}
	}
	tooMany := make([]domain.ManualPublicHoliday, maxManualHolidays+1)
	for i := range tooMany {
		tooMany[i] = domain.ManualPublicHoliday{Date: "2027-01-01", Title: "x"}
	}
	cases := map[string]struct {
		req  domain.AddPublicHolidaysRequest
		want string
	}{
		"no country":        {domain.AddPublicHolidaysRequest{Country: " ", Holidays: day("2027-01-01", "x")}, "country is required"},
		"a slash":           {domain.AddPublicHolidaysRequest{Country: "Sri/Lanka", Holidays: day("2027-01-01", "x")}, "slash"},
		"long country":      {domain.AddPublicHolidaysRequest{Country: strings.Repeat("a", maxHolidayCountryLen+1), Holidays: day("2027-01-01", "x")}, "at most"},
		"no days":           {domain.AddPublicHolidaysRequest{Country: "India"}, "holidays is required"},
		"too many":          {domain.AddPublicHolidaysRequest{Country: "India", Holidays: tooMany}, "at most"},
		"not a date":        {domain.AddPublicHolidaysRequest{Country: "India", Holidays: day("26/01/2027", "x")}, "holiday 1"},
		"year typo":         {domain.AddPublicHolidaysRequest{Country: "India", Holidays: day("2207-01-26", "x")}, "outside 2025 to 2028"},
		"no title":          {domain.AddPublicHolidaysRequest{Country: "India", Holidays: day("2027-01-26", "  ")}, "a title is required"},
		"long title":        {domain.AddPublicHolidaysRequest{Country: "India", Holidays: day("2027-01-26", strings.Repeat("t", maxHolidayTitleLen+1))}, "at most"},
		"second day is bad": {domain.AddPublicHolidaysRequest{Country: "India", Holidays: append(day("2027-01-26", "ok"), domain.ManualPublicHoliday{Date: "2027-13-01", Title: "x"})}, "holiday 2"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakePublicHolidayRepo{}
			svc, _ := manualHolidayService(repo, allowEditor)
			_, err := svc.Add(scheduleCaller("admin@example.com"), tc.req)
			var invalid *apierror.ValidationError
			if !errors.As(err, &invalid) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want a ValidationError containing %q, got %v", tc.want, err)
			}
			if len(repo.manual) != 0 {
				t.Errorf("stored %+v after a refusal", repo.manual)
			}
		})
	}
}

// Removing a stored day deletes it and places the tags again (which takes the
// day's untouched tags away); a day that is not stored is a 404.
func TestPublicHolidays_Remove(t *testing.T) {
	repo := &fakePublicHolidayRepo{}
	svc, placer := manualHolidayService(repo, allowEditor)
	if err := svc.Remove(scheduleCaller("admin@example.com"), " Sri Lanka ", "2026-12-25"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !reflect.DeepEqual(repo.deleted, []string{"Sri Lanka|2026-12-25"}) {
		t.Errorf("deleted = %v", repo.deleted)
	}
	waitPlaced(t, placer)

	repo.missing = true
	var notFound *apierror.NotFoundError
	if err := svc.Remove(scheduleCaller("admin@example.com"), "Sri Lanka", "2026-12-26"); !errors.As(err, &notFound) {
		t.Fatalf("a day not stored: want NotFoundError, got %v", err)
	}
	var invalid *apierror.ValidationError
	if err := svc.Remove(scheduleCaller("admin@example.com"), "Sri Lanka", "Christmas"); !errors.As(err, &invalid) {
		t.Fatalf("not a date: want ValidationError, got %v", err)
	}
}

// The list offers the stored countries plus the two the teams fall back to,
// says whether the caller may edit, and refuses a window over a year.
func TestPublicHolidays_List(t *testing.T) {
	repo := &fakePublicHolidayRepo{
		countries: []string{"France", "Sri Lanka"},
		listed: []domain.StoredPublicHoliday{
			{Country: "Sri Lanka", Date: "2026-12-25", Title: "Christmas Day", Source: domain.PublicHolidaySourceManual},
			{Country: "France", Date: "2026-07-14", Title: "Fête nationale", Source: domain.PublicHolidaySourceCalendar},
		},
	}
	editor, _ := manualHolidayService(repo, allowEditor)
	resp, err := editor.List(scheduleCaller("admin@example.com"), domain.ListPublicHolidaysRequest{From: "2026-01-01", To: "2026-12-31", Country: "Sri Lanka"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !resp.CanEdit {
		t.Error("canEdit = false for an editor")
	}
	if !reflect.DeepEqual(resp.Countries, []string{"France", "India", "Sri Lanka"}) {
		t.Errorf("countries = %v", resp.Countries)
	}
	if len(resp.Holidays) != 1 || resp.Holidays[0].Title != "Christmas Day" {
		t.Errorf("holidays = %+v, want Sri Lanka's only", resp.Holidays)
	}

	reader, _ := manualHolidayService(repo, refuseEditor)
	resp, err = reader.List(scheduleCaller("engineer@example.com"), domain.ListPublicHolidaysRequest{From: "2026-01-01", To: "2026-12-31"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp.CanEdit {
		t.Error("canEdit = true for a reader who may not edit")
	}

	var invalid *apierror.ValidationError
	if _, err := reader.List(scheduleCaller("engineer@example.com"), domain.ListPublicHolidaysRequest{From: "2026-01-01", To: "2027-01-02"}); !errors.As(err, &invalid) {
		t.Fatalf("a window over a year: want ValidationError, got %v", err)
	}
	if _, err := reader.List(scheduleCaller("engineer@example.com"), domain.ListPublicHolidaysRequest{From: "2026-12-31", To: "2026-01-01"}); !errors.As(err, &invalid) {
		t.Fatalf("to before from: want ValidationError, got %v", err)
	}
}

// The holiday editor check admits a portal admin and a rota admin of any
// family, and refuses a lead who is neither and a service credential.
func TestHolidayEditorCheck(t *testing.T) {
	cases := map[string]struct {
		repo  *fakeScheduleRepo
		email string
		ok    bool
	}{
		"portal admin":       {&fakeScheduleRepo{portalAdmin: true}, "admin@example.com", true},
		"SME rota admin":     {&fakeScheduleRepo{adminFamilies: []string{"SME"}}, "sme.admin@example.com", true},
		"CRE lead":           {&fakeScheduleRepo{leadsTeam: true, leadFamilies: []string{"CRE"}}, "lead@example.com", false},
		"service credential": {&fakeScheduleRepo{portalAdmin: true}, "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			check := HolidayEditorCheck(NewScheduleService(tc.repo, alwaysUnrestrictedAccess{}))
			err := check(scheduleCaller(tc.email))
			if tc.ok && err != nil {
				t.Fatalf("want allowed, got %v", err)
			}
			var forbidden *apierror.ForbiddenError
			if !tc.ok && !errors.As(err, &forbidden) {
				t.Fatalf("want ForbiddenError, got %v", err)
			}
		})
	}
}
