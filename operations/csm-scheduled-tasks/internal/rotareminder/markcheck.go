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
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

// WeekdayMarkCheck returns the handler that warns the rota admins, the evening
// before the weekday reminders go out, about a weekday rotation that has
// fewer people marked than it needs (cfg.WeekdayRequired: "CRE_EVENING" -> 7).
//
// It runs at 17:00 IST the day before rota_weekday_reminder, and checks the
// day that reminder will announce: the next working day after tomorrow
// (Saturday's check is Monday's rota, Wednesday's is Friday's). People on
// leave are not counted (the rota read leaves them out, as the reminder
// does). When every rotation has its people, nothing is sent. The reminders
// themselves are never held back: they go out at 09:00 IST to whoever is
// marked by then, copied to the rota admins (Config.Admins).
//
// The saved on/off switch for the weekday reminders applies: with them off,
// there is nothing to prepare for.
func WeekdayMarkCheck(rota RotaReader, store ReminderStore, email EmailSender, cfg Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !cfg.EmailsEnabled {
			return nil
		}
		cfg, on, err := withSettings(ctx, cfg, false)
		if err != nil || !on {
			return err
		}
		if len(cfg.WeekdayRequired) == 0 {
			slog.InfoContext(ctx, "rota mark check: no weekday rotation has a required headcount; nothing checked", "family", cfg.Family)
			return nil
		}
		// The reminder this check prepares for runs at 09:00 the day after the
		// check's own scheduled day (Config.Anchor, so a retry keeps it), and
		// announces the next working day after that.
		reminderDay := dayOf(cfg.anchor(), cfg.Zone).AddDate(0, 0, 1)
		if !cfg.Now().Before(reminderDay.Add(reminderHour)) {
			// A retry that ran past the reminder: it has gone out already,
			// copied to the rota admins, so a warning now helps nobody.
			slog.WarnContext(ctx, "rota mark check: the weekday reminder has already gone out; nothing checked",
				"reminder", reminderDay.Format(dateLayout))
			return nil
		}
		day := NextWorkingDay(reminderDay)
		date := day.Format(dateLayout)
		// Only rotations the reminders announce are checked: a required code
		// that is not in the weekday shifts would never be marked by anyone
		// the reminders reach, and would warn "0 of N" every evening.
		required := make(map[string]int, len(cfg.WeekdayRequired))
		for code, need := range cfg.WeekdayRequired {
			if cfg.Rotations[code] != "" {
				required[code] = need
			}
		}
		if len(required) == 0 {
			slog.InfoContext(ctx, "rota mark check: no announced weekday rotation has a required headcount; nothing checked", "family", cfg.Family)
			return nil
		}

		cat, err := rota.Catalogue(ctx)
		if err != nil {
			return fmt.Errorf("rotareminder: read the Team Schedule catalogue: %w", err)
		}
		assignments, err := rota.WorkingAssignments(ctx, date, date, cfg.Family)
		if err != nil {
			return fmt.Errorf("rotareminder: read the rota for %s: %w", date, err)
		}
		rostered := selectRostered(cfg, cat, assignments, map[string]time.Time{date: day})
		weekday := cfg
		weekday.Required = required
		gaps := missingSlots(weekday, rostered, []time.Time{day})
		if len(gaps) == 0 {
			slog.InfoContext(ctx, "rota mark check: every weekday rotation is fully marked; nothing sent", "day", date)
			return nil
		}
		slog.InfoContext(ctx, "rota mark check: weekday rotations are not fully marked; telling the rota admins",
			"day", date, "gaps", gapList(cfg, gaps))

		admins, err := store.RotaAdmins(ctx, cfg.Family)
		if err != nil {
			return fmt.Errorf("rotareminder: read the %s rota admins: %w", cfg.Family, err)
		}
		if len(admins) == 0 {
			// No rota admin: the task's alert audience instead, its Cc list
			// when it has no To.
			fallback := cfg.AlertTo
			if len(fallback) == 0 {
				fallback = cfg.AlertCc
			}
			for _, a := range fallback {
				admins = append(admins, teamschedule.RotaAdmin{Email: a})
			}
		}
		msg, ok := buildMarkCheckEmail(cfg, admins, reminderDay, day, gaps)
		if !ok {
			slog.WarnContext(ctx, "rota mark check: the weekday rota is not fully marked and there is nobody to tell", "family", cfg.Family, "day", date)
			return nil
		}
		return send(ctx, email, cfg, msg)
	}
}

// reminderHour is when rota_weekday_reminder runs on its day, in the rota's
// zone: 09:00 (its default schedule, 03:30 UTC in Colombo).
const reminderHour = 9 * time.Hour
