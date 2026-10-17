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

package rotareminder

import (
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

// shiftEmail is one window's email.
type shiftEmail struct {
	to, cc  []string
	subject string
	body    string
}

// buildEmail writes the email for one window: to the people on it, with the
// rota table (the whole weekend's for a weekend window). ok is false when
// nobody on the window has an address.
func buildEmail(cfg Config, w *window, rostered []teamschedule.Assignment, wholeRota bool, today time.Time) (shiftEmail, bool) {
	var named []string
	var to []string
	seen := map[string]bool{}
	start, end := w.people[0].StartsAt, w.people[0].EndsAt
	for _, a := range w.people {
		if a.StartsAt.Before(start) {
			start = a.StartsAt
		}
		if a.EndsAt.After(end) {
			end = a.EndsAt
		}
		addr := strings.TrimSpace(a.Engineer.Email)
		if addr == "" || seen[strings.ToLower(addr)] {
			continue
		}
		seen[strings.ToLower(addr)] = true
		named = append(named, a.Engineer.Name)
		to = append(to, addr)
	}
	if len(to) == 0 {
		return shiftEmail{}, false
	}

	title := windowTitle(cfg, w.base) // "Morning Rotation"
	inSentence := strings.Replace(title, " Rotation", " rotation", 1)
	when := windowText(start, end, cfg.Zone)       // "06:00–09:00 IST"
	date := w.day.Format("Monday, 2 January 2006") // "Tuesday, 13 October 2026"

	notice := fmt.Sprintf("This is an advance notice that you are rostered for the %s on %s, %s.", inSentence, date, when)
	if w.day.Equal(today) {
		// Only a retry that went out on the window's own day, before it began.
		notice = fmt.Sprintf("This is a reminder that you are rostered for the %s today, %s, %s.", inSentence, date, when)
	}
	if w.day.Equal(today.AddDate(0, 0, 1)) {
		notice = fmt.Sprintf("This is a reminder that you are rostered for the %s tomorrow, %s, %s.", inSentence, date, when)
	}

	data := notify.RotaReminderData{
		Greeting:    greeting(named),
		Intro:       notice,
		ScheduleURL: cfg.ScheduleURL,
	}
	if wholeRota {
		data.Columns = []string{"Name", "Day", "Rotation"}
		for _, a := range rostered {
			data.Rows = append(data.Rows, []string{a.Engineer.Name, dateOf(a.RotaDate, cfg.Zone).Format("Monday"), cfg.Rotations[a.ShiftCode]})
		}
	} else {
		data.Columns = []string{"Name", "Rotation"}
		for _, a := range w.people {
			data.Rows = append(data.Rows, []string{a.Engineer.Name, cfg.Rotations[a.ShiftCode]})
		}
	}

	email := shiftEmail{
		to:      to,
		cc:      without(cfg.Copy, to),
		subject: fmt.Sprintf("[%s Rota] %s – %s (%s)", cfg.Family, title, date, when),
	}
	if len(cfg.RedirectTo) > 0 {
		data.IntendedFor = append(append([]string{}, email.to...), email.cc...)
	}
	email.body = notify.RenderRotaReminder(data)

	return email, true
}

// windowTitle names a window in title case: "Morning Rotation", "LK Weekend
// Rotation", "America Weekend Rotation". The name is the base shift's, so a
// window holding only an on-call person is still named for its shift.
func windowTitle(cfg Config, base string) string {
	name := cfg.Rotations[base]
	if name == "" {
		name = cfg.Rotations[base+"_OC"]
	}
	if strings.HasSuffix(strings.ToLower(name), "rotation") {
		return name
	}
	return name + " Rotation"
}

// greeting addresses an email by the names of the people it goes to: by full
// name when it goes to one person, "Dear all," when it goes to several.
func greeting(names []string) string {
	if len(names) == 1 && strings.TrimSpace(names[0]) != "" {
		return "Dear " + strings.TrimSpace(names[0]) + ","
	}
	return "Dear all,"
}

// windowText writes a shift window in the rota's zone: "06:00–09:00 IST", or,
// past midnight, "21:00 IST to 06:00 IST on Sunday".
func windowText(start, end time.Time, zone *time.Location) string {
	s, e := start.In(zone), end.In(zone)
	z := zoneName(s)
	if s.YearDay() == e.YearDay() && s.Year() == e.Year() {
		return fmt.Sprintf("%s–%s %s", s.Format("15:04"), e.Format("15:04"), z)
	}
	return fmt.Sprintf("%s %s to %s %s on %s", s.Format("15:04"), z, e.Format("15:04"), z, e.Format("Monday"))
}

// dateOf is a rota day (YYYY-MM-DD) in zone.
func dateOf(rotaDate string, zone *time.Location) time.Time {
	d, err := time.ParseInLocation(dateLayout, rotaDate, zone)
	if err != nil {
		return time.Time{}
	}
	return d
}

// zoneName is the name people use for t's zone. Go's tz database names
// Colombo's offset "+0530"; the office calls it IST.
func zoneName(t time.Time) string {
	switch t.Location().String() {
	case "Asia/Colombo", "Asia/Kolkata":
		return "IST"
	}
	return t.Format("MST")
}
