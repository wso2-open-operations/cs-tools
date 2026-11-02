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
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

// RequestQueue is the subset of *teamschedule.Client that hands out the
// "Send now" requests made on the Rota Access tab.
type RequestQueue interface {
	ClaimReminderRequest(ctx context.Context, family string) (*teamschedule.ReminderRequest, error)
	ReportReminderRequest(ctx context.Context, family, id, status, result string) error
}

// manualPerRun bounds the requests one run sends, so a backlog cannot keep a
// run going; the rest wait for the next.
const manualPerRun = 5

// ManualReminder returns the handler of rota_manual_reminder: at every run it
// claims the family's waiting "Send now" requests and sends each with the
// automatic reminders' own code -- weekday is the next working day's windows,
// morning and evening one of them (the Morning with its on-call, or the
// Evening), weekend the coming weekend (this one, from a Saturday or a Sunday), sent
// even when not fully marked, as on Friday. Each request is reported SENT or
// FAILED with a short result for the card. A request that cannot be sent is
// reported, not retried: somebody pressed a button and should see why. Only a
// failure to claim or report fails the run.
func ManualReminder(rota RotaReader, store ReminderStore, queue RequestQueue, email EmailSender, weekdayCfg, weekendCfg Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		for i := 0; i < manualPerRun; i++ {
			req, err := queue.ClaimReminderRequest(ctx, weekendCfg.Family)
			if err != nil {
				return fmt.Errorf("rotareminder: claim a rota reminder request: %w", err)
			}
			if req == nil {
				return nil
			}
			status, result := "SENT", ""
			weekday, weekend := weekdayCfg, weekendCfg
			if req.Test {
				weekday, weekend = asTest(weekday, req.RequestedBy), asTest(weekend, req.RequestedBy)
			}
			tally, err := sendManual(ctx, rota, store, email, weekday, weekend, req.Kind)
			switch {
			case err != nil:
				status, result = "FAILED", err.Error()
			case tally.Failed > 0:
				// Some went out, so the run did not fail; the person who
				// pressed the button must still see that some did not.
				result = fmt.Sprintf("Sent %d of %d emails; %d could not be sent (the task's alert recipients were told who missed out).",
					tally.Sent, tally.Sent+tally.Failed, tally.Failed)
			case tally.Sent == 0:
				result = "Nothing was sent: " + tally.reason() + "."
			case tally.Sent == 1:
				result = "Sent 1 email."
			default:
				result = fmt.Sprintf("Sent %d emails.", tally.Sent)
			}
			if req.Test && err == nil && tally.Sent > 0 && tally.Failed == 0 {
				result = strings.TrimSuffix(result, ".") + " to " + req.RequestedBy + " (test)."
			}
			slog.InfoContext(ctx, "rota reminder: sent by hand", "kind", req.Kind, "test", req.Test, "requestedBy", req.RequestedBy, "status", status, "result", result)
			if err := queue.ReportReminderRequest(ctx, weekendCfg.Family, req.ID, status, result); err != nil {
				return fmt.Errorf("rotareminder: report rota reminder request %s: %w", req.ID, err)
			}
		}
		return nil
	}
}

// sendManual sends one request and tallies the reminder emails that went out
// and those that did not.
func sendManual(ctx context.Context, rota RotaReader, store ReminderStore, email EmailSender, weekdayCfg, weekendCfg Config, kind string) (*Tally, error) {
	tally := &Tally{}
	switch kind {
	case "weekday", "morning", "evening":
		cfg := weekdayCfg
		if !cfg.EmailsEnabled {
			return tally, errors.New("emails are turned off in this environment (ALERTS_ENABLED)")
		}
		if kind != "weekday" {
			cfg.Rotations = windowRotations(cfg.Rotations, kind)
			if len(cfg.Rotations) == 0 {
				return tally, fmt.Errorf("no %s rotation is set up for the weekday reminders (ROTA_REMINDER_WEEKDAY_SHIFTS)", kind)
			}
		}
		cfg.Manual, cfg.Anchor, cfg.Tally = true, cfg.Now, tally
		return tally, WeekdayReminder(rota, email, cfg)(ctx)
	case "weekend":
		cfg := weekendCfg
		if !cfg.EmailsEnabled {
			return tally, errors.New("emails are turned off in this environment (ALERTS_ENABLED)")
		}
		now := cfg.Now()
		cfg.Manual, cfg.Tally = true, tally
		cfg.Anchor = func() time.Time { return manualWeekendAnchor(now, cfg.Zone) }
		return tally, weekend(ctx, rota, store, email, cfg, true)
	}
	return tally, fmt.Errorf("unknown reminder kind %q", kind)
}

// windowRotations keeps the weekday rotations of one window: "morning" is the
// shift whose code ends in _MORNING with its on-call (<CODE>_OC), "evening" the
// one ending in _EVENING with its on-call, so "Send now" can send one window's
// reminder on its own. The names stay as configured.
func windowRotations(rotations map[string]string, kind string) map[string]string {
	suffix := "_" + strings.ToUpper(kind)
	out := map[string]string{}
	for code, name := range rotations {
		if strings.HasSuffix(strings.TrimSuffix(code, "_OC"), suffix) {
			out[code] = name
		}
	}
	return out
}

// manualWeekendAnchor is the day a "Send now" weekend is counted from: today,
// except from a Saturday or a Sunday, when it is the Friday before, so the
// weekend is this one and not the next.
func manualWeekendAnchor(now time.Time, zone *time.Location) time.Time {
	switch day := dayOf(now, zone); day.Weekday() {
	case time.Saturday:
		return day.AddDate(0, 0, -1)
	case time.Sunday:
		return day.AddDate(0, 0, -2)
	default:
		return now
	}
}

// Tally is what a run did with the rota's own reminder emails, counted where
// each is sent: a weekday window's email by its final outcome (after it is
// sent again without its invitation, sendWindowEmail), the weekend summary
// and each weekend invitation. The rota admins' email, cancellations and
// alerts are not reminders and are not counted. Its methods do nothing on a
// nil Tally, so a scheduled run, which has none, calls them unchecked.
type Tally struct {
	Sent, Failed int
	reasons      []string
}

// count records one reminder email's outcome.
func (t *Tally) count(err error) {
	if t == nil {
		return
	}
	if err == nil {
		t.Sent++
	} else {
		t.Failed++
	}
}

// skipped notes why something was not sent.
func (t *Tally) skipped(reason string) {
	if t == nil || slices.Contains(t.reasons, reason) {
		return
	}
	t.reasons = append(t.reasons, reason)
}

// reason is why nothing was sent, for the card.
func (t *Tally) reason() string {
	if len(t.reasons) == 0 {
		return "there was nothing to send"
	}
	return strings.Join(t.reasons, "; ")
}

// longDay writes a rota day for the card as the emails do (dayList):
// "Tuesday, 20 October 2026".
func longDay(d time.Time) string { return dayList([]time.Time{d}) }

// asTest is cfg for "Send a test to me": everything, alerts included, goes
// only to the person who asked, and nothing is copied or recorded.
func asTest(cfg Config, to string) Config {
	cfg.Test = true
	cfg.RedirectTo = []string{to}
	cfg.Copy = nil
	cfg.AlertTo, cfg.AlertCc = []string{to}, nil
	return cfg
}
