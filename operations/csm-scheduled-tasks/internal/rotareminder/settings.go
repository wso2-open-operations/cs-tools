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
	"maps"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

// withSettings is cfg with the family's saved reminder settings applied, and
// whether this kind of reminder (weekday, or weekend) is on. The switches on
// the Rota Access tab are the only way to turn the automatic reminders on:
// with nothing saved for the family, none is sent. "Send now" (cfg.Manual) is
// asked for by hand and is sent whatever the switches say, with the
// configured values where nothing is saved. A failure to read the settings
// fails the run, so it is retried rather than sent with settings somebody
// turned off.
func withSettings(ctx context.Context, cfg Config, weekend bool) (Config, bool, error) {
	if cfg.Settings == nil {
		return cfg, true, nil
	}
	s, err := cfg.Settings.ReminderSettingsFor(ctx, cfg.Family)
	if err != nil {
		return cfg, false, fmt.Errorf("rotareminder: read the %s reminder settings: %w", cfg.Family, err)
	}
	if s == nil {
		if !cfg.Manual {
			slog.InfoContext(ctx, "rota reminder: not turned on on the Rota Access tab; nothing sent", "task", cfg.Task, "family", cfg.Family)
			return cfg, false, nil
		}
		return cfg, true, nil
	}
	on := s.WeekdayEnabled
	if weekend {
		on = s.WeekendEnabled
	}
	// "Send now" is asked for by hand: the on/off switches are for the
	// automatic sends, so they do not stop it.
	if !on && !cfg.Manual {
		slog.InfoContext(ctx, "rota reminder: turned off on the Rota Access tab; nothing sent", "task", cfg.Task, "family", cfg.Family)
		return cfg, false, nil
	}
	// The card can turn invitations off, never on where the deployment has
	// them off (ROTA_REMINDER_INVITES_ENABLED, or no organizer): a save
	// cannot override a setting nobody changed on the card.
	cfg.Invites = cfg.Invites && s.InvitesEnabled
	// Saved minimums and hours override the configured ones code by code; a
	// code the saved settings do not name keeps the configured value, so a
	// save that left a rotation untouched never replaces the deployment's
	// setting for it.
	if len(s.WeekendRequired) > 0 {
		required := maps.Clone(cfg.Required)
		if required == nil {
			required = map[string]int{}
		}
		maps.Copy(required, s.WeekendRequired)
		cfg.Required = required
	}
	if len(s.Windows) > 0 {
		saved, err := settingsWindows(s.Windows)
		if err != nil {
			return cfg, false, fmt.Errorf("rotareminder: the saved %s reminder windows: %w", cfg.Family, err)
		}
		windows := maps.Clone(cfg.Windows)
		if windows == nil {
			windows = map[string]LocalWindow{}
		}
		maps.Copy(windows, saved)
		cfg.Windows = windows
	}
	cfg.Copy = without(append(append([]string{}, cfg.Copy...), s.CopyTo...), nil)
	return cfg, true, nil
}

// settingsWindows reads the saved windows as ParseWindows reads the
// environment's, with the same checks.
func settingsWindows(saved map[string]teamschedule.ReminderWindow) (map[string]LocalWindow, error) {
	out := map[string]LocalWindow{}
	for code, w := range saved {
		one, err := ParseWindows(fmt.Sprintf("%s=%s-%s %s", code, w.Start, w.End, w.TimeZone))
		if err != nil {
			return nil, err
		}
		out[code] = one[code]
	}
	return out, nil
}
