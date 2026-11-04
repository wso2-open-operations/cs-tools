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
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// RotaReminderSettingsService is each family's rota reminder settings: read
// by csm-scheduled-tasks at every reminder run and by the Rota Access tab,
// changed on that tab by a CSM admin or the family's rota admin (the people
// who manage that family's rota access).
type RotaReminderSettingsService interface {
	// Get is the family's settings, or nil when none were saved (the
	// reminders then keep their own configuration).
	Get(ctx context.Context, family string) (*domain.RotaReminderSettings, error)
	Save(ctx context.Context, family string, req domain.SaveRotaReminderSettingsRequest) (domain.RotaReminderSettings, error)
}

type rotaReminderSettingsService struct {
	repo       repository.RotaReminderSettingsRepository
	schedule   repository.ScheduleRepository
	access     AccessService
	adminRoles []string
}

// NewRotaReminderSettingsService constructs a RotaReminderSettingsService.
// adminRoles is AUTH_ADMIN_ROLES: the token roles that make a CSM admin.
func NewRotaReminderSettingsService(repo repository.RotaReminderSettingsRepository, schedule repository.ScheduleRepository, access AccessService, adminRoles []string) RotaReminderSettingsService {
	return &rotaReminderSettingsService{repo: repo, schedule: schedule, access: access, adminRoles: adminRoles}
}

// Limits on what is saved: a family has a handful of rotations, so these
// only stop nonsense, never a real setting.
const (
	maxReminderSettingEntries = 20
	maxReminderPeoplePerShift = 20
	maxReminderCopyTo         = 20
)

var (
	reminderShiftCodeRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
	reminderClockRe     = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
)

func reminderSettingsFamily(family string) (string, error) {
	f := strings.ToUpper(strings.TrimSpace(family))
	return f, validReminderFamily(f)
}

func (s *rotaReminderSettingsService) Get(ctx context.Context, family string) (*domain.RotaReminderSettings, error) {
	if err := RequireInternalCaller(ctx, s.access, "rota reminder settings are internal only"); err != nil {
		return nil, err
	}
	f, err := reminderSettingsFamily(family)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, f)
}

// mayChange is whether the caller may change the family's settings: a CSM
// admin, or a rota admin of that family. A person only: the scheduled task
// reads the settings, it never writes them.
func (s *rotaReminderSettingsService) mayChange(ctx context.Context, family string) (string, error) {
	return rotaReminderEditor(ctx, s.access, s.schedule, s.adminRoles, family, "change the %[1]s rota reminder settings")
}

// rotaReminderEditor admits a CSM admin, or a rota admin of the family, as a
// person, and returns their address. what completes "only a CSM admin or the
// CRE rota admin can ..." (%[1]s is the family).
func rotaReminderEditor(ctx context.Context, access AccessService, schedule repository.ScheduleRepository, adminRoles []string, family, what string) (string, error) {
	if err := RequireInternalCaller(ctx, access, "rota reminders are internal only"); err != nil {
		return "", err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" {
		return "", &apierror.ForbiddenError{Msg: "this is done by a person, not a service credential"}
	}
	if holdsAnyRole(id.Roles, adminRoles) {
		return id.UserEmail, nil
	}
	families, err := schedule.RotaAdminFamilies(ctx, id.UserEmail)
	if err != nil {
		return "", err
	}
	if !slices.Contains(families, family) {
		return "", &apierror.ForbiddenError{Msg: fmt.Sprintf("only a CSM admin or the %[1]s rota admin can "+what, family)}
	}
	return id.UserEmail, nil
}

func (s *rotaReminderSettingsService) Save(ctx context.Context, family string, req domain.SaveRotaReminderSettingsRequest) (domain.RotaReminderSettings, error) {
	f, err := reminderSettingsFamily(family)
	if err != nil {
		return domain.RotaReminderSettings{}, err
	}
	actor, err := s.mayChange(ctx, f)
	if err != nil {
		return domain.RotaReminderSettings{}, err
	}
	settings, err := validRotaReminderSettings(f, req)
	if err != nil {
		return domain.RotaReminderSettings{}, err
	}
	// A card from before the per-rotation switches and the weekend copies
	// sends neither: keep what is stored rather than turning every rotation's
	// invitations back on or dropping the weekend copies.
	if req.InviteRotations == nil || req.WeekendCopyTo == nil {
		stored, err := s.repo.Get(ctx, f)
		if err != nil {
			return domain.RotaReminderSettings{}, err
		}
		if stored != nil && req.InviteRotations == nil {
			settings.InviteRotations = stored.InviteRotations
		}
		if stored != nil && req.WeekendCopyTo == nil {
			settings.WeekendCopyTo = stored.WeekendCopyTo
		}
	}
	return s.repo.Save(ctx, actor, settings)
}

// validRotaReminderSettings checks a save and returns the settings to store,
// addresses trimmed, lower-cased and without repeats.
func validRotaReminderSettings(family string, req domain.SaveRotaReminderSettingsRequest) (domain.RotaReminderSettings, error) {
	bad := func(msg string, args ...any) (domain.RotaReminderSettings, error) {
		return domain.RotaReminderSettings{}, &apierror.ValidationError{Msg: fmt.Sprintf(msg, args...)}
	}
	if req.WeekdayEnabled == nil || req.WeekendEnabled == nil || req.InvitesEnabled == nil {
		return bad("weekdayEnabled, weekendEnabled and invitesEnabled are required")
	}
	out := domain.RotaReminderSettings{
		Family:          family,
		WeekdayEnabled:  *req.WeekdayEnabled,
		WeekendEnabled:  *req.WeekendEnabled,
		InvitesEnabled:  *req.InvitesEnabled,
		WeekendRequired: map[string]int{},
		Windows:         map[string]domain.ReminderWindow{},
		InviteRotations: map[string]bool{},
		CopyTo:          []string{},
		WeekendCopyTo:   []string{},
	}
	if len(req.WeekendRequired) > maxReminderSettingEntries {
		return bad("weekendRequired has more than %d shifts", maxReminderSettingEntries)
	}
	for code, n := range req.WeekendRequired {
		if !reminderShiftCodeRe.MatchString(code) {
			return bad("weekendRequired: %q is not a shift code", code)
		}
		if n < 0 || n > maxReminderPeoplePerShift {
			return bad("weekendRequired: %s must be 0 to %d people", code, maxReminderPeoplePerShift)
		}
		out.WeekendRequired[code] = n
	}
	if len(req.Windows) > maxReminderSettingEntries {
		return bad("windows has more than %d shifts", maxReminderSettingEntries)
	}
	for code, w := range req.Windows {
		if !reminderShiftCodeRe.MatchString(code) {
			return bad("windows: %q is not a shift code", code)
		}
		if !reminderClockRe.MatchString(w.Start) || !reminderClockRe.MatchString(w.End) {
			return bad("windows: %s start and end must be HH:MM, 24-hour", code)
		}
		if w.Start == w.End {
			return bad("windows: %s starts and ends at the same time", code)
		}
		zone := strings.TrimSpace(w.TimeZone)
		if zone == "" || len(zone) > 64 {
			return bad("windows: %s needs a time zone", code)
		}
		if _, err := time.LoadLocation(zone); err != nil || zone == "Local" {
			return bad("windows: %s: %q is not a time zone", code, zone)
		}
		out.Windows[code] = domain.ReminderWindow{Start: w.Start, End: w.End, TimeZone: zone}
	}
	if len(req.InviteRotations) > maxReminderSettingEntries {
		return bad("inviteRotations has more than %d rotations", maxReminderSettingEntries)
	}
	for code, on := range req.InviteRotations {
		if !reminderShiftCodeRe.MatchString(code) {
			return bad("inviteRotations: %q is not a shift code", code)
		}
		out.InviteRotations[code] = on
	}
	var err error
	if out.CopyTo, err = reminderAddresses("copyTo", req.CopyTo); err != nil {
		return domain.RotaReminderSettings{}, err
	}
	if out.WeekendCopyTo, err = reminderAddresses("weekendCopyTo", req.WeekendCopyTo); err != nil {
		return domain.RotaReminderSettings{}, err
	}
	return out, nil
}

// reminderAddresses checks a list of copied addresses and returns them
// trimmed, lower-cased and without repeats. field names the list in a refusal.
func reminderAddresses(field string, in []string) ([]string, error) {
	out := []string{}
	if len(in) > maxReminderCopyTo {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("%s has more than %d addresses", field, maxReminderCopyTo)}
	}
	for _, a := range in {
		a = strings.ToLower(strings.TrimSpace(a))
		if !emailRE.MatchString(a) || len(a) > 254 {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("%s: %q is not an email address", field, a)}
		}
		if !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out, nil
}
