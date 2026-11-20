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

// PublicHolidayRepository's manual rows (migration 0293) against a real
// Postgres with every migration applied. Skipped without
// ENTITY_TEST_DATABASE_URL:
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/repository/ -run PublicHolidayManual

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const manualCountry = "Manualland (test)"

// A day added by hand survives the calendar refresh: the refresh neither
// deletes it (the calendar does not have it) nor retitles it (the calendar has
// the same day under another title), while it still replaces its own rows. A
// manual add over a calendar day takes it over, and a delete removes either.
func TestPublicHolidayManual_SurvivesTheCalendarRefresh(t *testing.T) {
	pool := phPool(t)
	ctx := context.Background()
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM team_schedule_public_holiday WHERE country = $1`, manualCountry)
	}
	cleanup()
	t.Cleanup(cleanup)
	repo := repository.NewPublicHolidayRepository(pool)
	at := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	from := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2027, 12, 31, 0, 0, 0, 0, time.UTC)

	// The calendar's first answer: two days.
	if _, err := repo.ReplaceCountryRange(ctx, manualCountry, from, to, []domain.PublicHolidayDay{
		{Country: manualCountry, Date: "2027-01-14", Title: "Calendar day"},
		{Country: manualCountry, Date: "2027-05-01", Title: "Labour Day"},
	}, at); err != nil {
		t.Fatalf("ReplaceCountryRange: %v", err)
	}
	// By hand: a day the calendar lacks, and a calendar day retitled.
	n, err := repo.UpsertManualHolidays(ctx, manualCountry, []domain.PublicHolidayDay{
		{Country: manualCountry, Date: "2027-02-04", Title: "Independence Day"},
		{Country: manualCountry, Date: "2027-01-14", Title: "Thai Pongal"},
	}, "admin@example.com", at)
	if err != nil || n != 2 {
		t.Fatalf("UpsertManualHolidays = %d, %v; want 2", n, err)
	}
	// The calendar's next answer has neither manual day's title, and drops
	// Labour Day: only its own row goes.
	if _, err := repo.ReplaceCountryRange(ctx, manualCountry, from, to, []domain.PublicHolidayDay{
		{Country: manualCountry, Date: "2027-01-14", Title: "Calendar day again"},
	}, at); err != nil {
		t.Fatalf("ReplaceCountryRange again: %v", err)
	}

	got, err := repo.StoredHolidays(ctx, manualCountry, from, to)
	if err != nil {
		t.Fatalf("StoredHolidays: %v", err)
	}
	type row struct{ date, title, source, by string }
	var rows []row
	for _, d := range got {
		by := ""
		if d.UpdatedBy != nil {
			by = *d.UpdatedBy
		}
		rows = append(rows, row{d.Date, d.Title, d.Source, by})
	}
	want := []row{
		{"2027-01-14", "Thai Pongal", domain.PublicHolidaySourceManual, "admin@example.com"},
		{"2027-02-04", "Independence Day", domain.PublicHolidaySourceManual, "admin@example.com"},
	}
	if len(rows) != len(want) {
		t.Fatalf("stored = %+v, want %+v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}

	countries, err := repo.StoredCountries(ctx)
	if err != nil {
		t.Fatalf("StoredCountries: %v", err)
	}
	found := false
	for _, c := range countries {
		found = found || c == manualCountry
	}
	if !found {
		t.Errorf("StoredCountries = %v, missing %q", countries, manualCountry)
	}

	removed, err := repo.DeleteHoliday(ctx, manualCountry, "2027-02-04")
	if err != nil || !removed {
		t.Fatalf("DeleteHoliday = %v, %v; want true", removed, err)
	}
	removed, err = repo.DeleteHoliday(ctx, manualCountry, "2027-02-04")
	if err != nil || removed {
		t.Fatalf("DeleteHoliday again = %v, %v; want false", removed, err)
	}
	// The search side reads manual rows like any other.
	days, err := repo.HolidaysBetween(ctx, []string{manualCountry}, from, to)
	if err != nil || len(days) != 1 || days[0].Title != "Thai Pongal" {
		t.Fatalf("HolidaysBetween = %+v, %v; want the manual Thai Pongal only", days, err)
	}
}
