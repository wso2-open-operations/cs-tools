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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

// LocalWindow is a rotation's hours on its own clock, used by the reminders
// instead of the Team Schedule's: the Americas Weekend is worked 08:00-18:00
// in Los Angeles, which is a different span of IST before and after US
// daylight saving ends.
type LocalWindow struct {
	// Start and End are minutes after midnight. An End not after Start ends
	// on the next day.
	Start, End int
	Zone       *time.Location
}

// on is the window on a rota day: the day is read on the window's own clock.
func (lw LocalWindow) on(rotaDate string) (time.Time, time.Time, bool) {
	d, err := time.Parse(dateLayout, rotaDate)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	at := func(day time.Time, minutes int) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), minutes/60, minutes%60, 0, 0, lw.Zone)
	}
	endDay := d
	if lw.End <= lw.Start {
		endDay = d.AddDate(0, 0, 1)
	}
	return at(d, lw.Start), at(endDay, lw.End), true
}

// ParseWindows reads "CODE=HH:MM-HH:MM Zone, ...", for example
// "CRE_WEEKEND_NIGHT=08:00-18:00 America/Los_Angeles". Empty is no override.
func ParseWindows(raw string) (map[string]LocalWindow, error) {
	out := map[string]LocalWindow{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		code, rest, ok := strings.Cut(entry, "=")
		hours, zoneName, ok2 := strings.Cut(strings.TrimSpace(rest), " ")
		from, to, ok3 := strings.Cut(hours, "-")
		code = strings.TrimSpace(code)
		if !ok || !ok2 || !ok3 || code == "" {
			return nil, fmt.Errorf("%q is not CODE=HH:MM-HH:MM Zone", entry)
		}
		zone, err := time.LoadLocation(strings.TrimSpace(zoneName))
		if err != nil {
			return nil, fmt.Errorf("%q: %w", entry, err)
		}
		start, err1 := clockMinutes(from)
		end, err2 := clockMinutes(to)
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("%q: hours must be HH:MM-HH:MM", entry)
		}
		if start == end {
			return nil, fmt.Errorf("%q: the window has no length", entry)
		}
		out[code] = LocalWindow{Start: start, End: end, Zone: zone}
	}
	return out, nil
}

func clockMinutes(s string) (int, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}

// applyWindows gives every assignment on an overridden rotation that
// rotation's own hours.
func applyWindows(cfg Config, rostered []teamschedule.Assignment) {
	for i, a := range rostered {
		lw, ok := cfg.Windows[a.ShiftCode]
		if !ok {
			continue
		}
		if s, e, ok := lw.on(a.RotaDate); ok {
			rostered[i].StartsAt, rostered[i].EndsAt = s, e
		}
	}
}

// americasClock lists the rotations worked on the Americas clock: the
// Americas Weekend and the Americas On-call beside it. Their hours are told
// on Los Angeles time whatever zone their window is saved in, so a window
// entered in IST still reads as the Americas team's hours.
var americasClock = map[string]bool{"CRE_WEEKEND_NIGHT": true, "CRE_WEEKEND_NIGHT_OC": true}

// losAngeles is America/Los_Angeles, or nil when the zone database lacks it.
var losAngeles, _ = time.LoadLocation("America/Los_Angeles")

// clockOf is the zone a rotation is described in: Los Angeles for the
// Americas rotations, else its own window's, when it has one, else the rota's.
func clockOf(cfg Config, code string) *time.Location {
	if americasClock[code] && losAngeles != nil {
		return losAngeles
	}
	if lw, ok := cfg.Windows[code]; ok && lw.Zone != nil {
		return lw.Zone
	}
	return cfg.Zone
}
