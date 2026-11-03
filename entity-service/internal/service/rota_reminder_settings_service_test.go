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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeReminderSettingsRepo struct {
	saved *domain.RotaReminderSettings
	actor string
}

func (f *fakeReminderSettingsRepo) Get(context.Context, string) (*domain.RotaReminderSettings, error) {
	return f.saved, nil
}

func (f *fakeReminderSettingsRepo) Save(_ context.Context, actor string, s domain.RotaReminderSettings) (domain.RotaReminderSettings, error) {
	f.saved, f.actor = &s, actor
	return s, nil
}

func yes() *bool { b := true; return &b }

func goodReminderSettings() domain.SaveRotaReminderSettingsRequest {
	return domain.SaveRotaReminderSettingsRequest{
		WeekdayEnabled: yes(), WeekendEnabled: yes(), InvitesEnabled: yes(),
		WeekendRequired: map[string]int{"CRE_WEEKEND": 3, "CRE_WEEKEND_NIGHT": 1},
		Windows: map[string]domain.ReminderWindow{
			"CRE_WEEKEND_NIGHT":    {Start: "08:00", End: "18:00", TimeZone: "America/Los_Angeles"},
			"CRE_WEEKEND_NIGHT_OC": {Start: "08:00", End: "18:00", TimeZone: "America/Los_Angeles"},
		},
		CopyTo: []string{" Lead@Example.com ", "lead@example.com"},
	}
}

func newReminderSettings(repo *fakeReminderSettingsRepo, families ...string) RotaReminderSettingsService {
	return NewRotaReminderSettingsService(repo, &rotaAdminOf{families: families}, alwaysUnrestrictedAccess{}, []string{"admin"})
}

// A CSM admin may change any family; a rota admin only their own; anybody
// else, and the scheduled task's service credential, may not.
func TestRotaReminderSettings_WhoMayChange(t *testing.T) {
	cases := []struct {
		name     string
		ctx      context.Context
		families []string
		family   string
		ok       bool
	}{
		{"CSM admin", callerWithRoles("boss@example.com", "admin"), nil, "SRE", true},
		{"CRE rota admin, CRE", callerWithRoles("ann@example.com"), []string{"CRE"}, "cre", true},
		{"CRE rota admin, SRE", callerWithRoles("ann@example.com"), []string{"CRE"}, "SRE", false},
		{"nobody in particular", callerWithRoles("joe@example.com", "cs_engineer"), nil, "CRE", false},
		{"service credential", serviceCtx(), []string{"CRE"}, "CRE", false},
	}
	for _, c := range cases {
		repo := &fakeReminderSettingsRepo{}
		_, err := newReminderSettings(repo, c.families...).Save(c.ctx, c.family, goodReminderSettings())
		var forbidden *apierror.ForbiddenError
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case !c.ok && !errors.As(err, &forbidden):
			t.Errorf("%s: err = %v, want forbidden", c.name, err)
		case !c.ok && repo.saved != nil:
			t.Errorf("%s: saved although refused", c.name)
		}
	}
}

// What is stored: the family upper-cased, the actor recorded, addresses
// trimmed, lower-cased and once each.
func TestRotaReminderSettings_SavesNormalised(t *testing.T) {
	repo := &fakeReminderSettingsRepo{}
	out, err := newReminderSettings(repo, "CRE").Save(callerWithRoles("ann@example.com"), "cre", goodReminderSettings())
	if err != nil {
		t.Fatal(err)
	}
	if out.Family != "CRE" || repo.actor != "ann@example.com" || len(out.CopyTo) != 1 || out.CopyTo[0] != "lead@example.com" ||
		out.WeekendRequired["CRE_WEEKEND"] != 3 || out.Windows["CRE_WEEKEND_NIGHT_OC"].TimeZone != "America/Los_Angeles" {
		t.Errorf("saved %+v as %q", out, repo.actor)
	}
}

// The per-rotation invitation switches are stored as sent; a card that sends
// none (one from before they existed) keeps what is stored instead of turning
// every rotation's invitations back on.
func TestRotaReminderSettings_InviteRotations(t *testing.T) {
	repo := &fakeReminderSettingsRepo{}
	svc := newReminderSettings(repo, "CRE")
	req := goodReminderSettings()
	req.InviteRotations = map[string]bool{"CRE_MORNING": true, "CRE_WEEKEND": false}
	out, err := svc.Save(callerWithRoles("ann@example.com"), "CRE", req)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.InviteRotations) != 2 || !out.InviteRotations["CRE_MORNING"] || out.InviteRotations["CRE_WEEKEND"] {
		t.Fatalf("saved %v", out.InviteRotations)
	}

	older := goodReminderSettings()
	older.InviteRotations = nil
	out, err = svc.Save(callerWithRoles("ann@example.com"), "CRE", older)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := out.InviteRotations["CRE_WEEKEND"]; !ok || v {
		t.Errorf("an older card's save changed the switches to %v", out.InviteRotations)
	}

	cleared := goodReminderSettings()
	cleared.InviteRotations = map[string]bool{}
	if out, err = svc.Save(callerWithRoles("ann@example.com"), "CRE", cleared); err != nil || len(out.InviteRotations) != 0 {
		t.Errorf("an empty map is a save of no exceptions: %v, %v", out.InviteRotations, err)
	}
}

// The weekend-only copies are stored normalised; a card that sends none keeps
// what is stored.
func TestRotaReminderSettings_WeekendCopyTo(t *testing.T) {
	repo := &fakeReminderSettingsRepo{}
	svc := newReminderSettings(repo, "CRE")
	req := goodReminderSettings()
	req.WeekendCopyTo = []string{" Head@Example.com ", "head@example.com"}
	out, err := svc.Save(callerWithRoles("ann@example.com"), "CRE", req)
	if err != nil || len(out.WeekendCopyTo) != 1 || out.WeekendCopyTo[0] != "head@example.com" {
		t.Fatalf("saved %v, %v", out.WeekendCopyTo, err)
	}
	older := goodReminderSettings()
	older.WeekendCopyTo = nil
	if out, err = svc.Save(callerWithRoles("ann@example.com"), "CRE", older); err != nil || len(out.WeekendCopyTo) != 1 {
		t.Errorf("an older card's save changed the weekend copies to %v, %v", out.WeekendCopyTo, err)
	}
}

func TestRotaReminderSettings_RefusesNonsense(t *testing.T) {
	cases := map[string]func(r *domain.SaveRotaReminderSettingsRequest){
		"missing switch":  func(r *domain.SaveRotaReminderSettingsRequest) { r.InvitesEnabled = nil },
		"bad shift code":  func(r *domain.SaveRotaReminderSettingsRequest) { r.WeekendRequired["cre weekend"] = 1 },
		"too many people": func(r *domain.SaveRotaReminderSettingsRequest) { r.WeekendRequired["CRE_WEEKEND"] = 21 },
		"negative people": func(r *domain.SaveRotaReminderSettingsRequest) { r.WeekendRequired["CRE_WEEKEND"] = -1 },
		"bad clock": func(r *domain.SaveRotaReminderSettingsRequest) {
			r.Windows["X"] = domain.ReminderWindow{Start: "8am", End: "18:00", TimeZone: "UTC"}
		},
		"no length": func(r *domain.SaveRotaReminderSettingsRequest) {
			r.Windows["X"] = domain.ReminderWindow{Start: "08:00", End: "08:00", TimeZone: "UTC"}
		},
		"unknown zone": func(r *domain.SaveRotaReminderSettingsRequest) {
			r.Windows["X"] = domain.ReminderWindow{Start: "08:00", End: "18:00", TimeZone: "Mars/Base"}
		},
		"server's own clock": func(r *domain.SaveRotaReminderSettingsRequest) {
			r.Windows["X"] = domain.ReminderWindow{Start: "08:00", End: "18:00", TimeZone: "Local"}
		},
		"bad address":         func(r *domain.SaveRotaReminderSettingsRequest) { r.CopyTo = []string{"not an address"} },
		"bad weekend address": func(r *domain.SaveRotaReminderSettingsRequest) { r.WeekendCopyTo = []string{"nope"} },
		"bad invite rotation": func(r *domain.SaveRotaReminderSettingsRequest) {
			r.InviteRotations = map[string]bool{"cre morning": false}
		},
	}
	for name, spoil := range cases {
		req := goodReminderSettings()
		spoil(&req)
		repo := &fakeReminderSettingsRepo{}
		var invalid *apierror.ValidationError
		if _, err := newReminderSettings(repo, "CRE").Save(callerWithRoles("ann@example.com"), "CRE", req); !errors.As(err, &invalid) || repo.saved != nil {
			t.Errorf("%s: err = %v, saved = %v", name, err, repo.saved != nil)
		}
	}
	var invalid *apierror.ValidationError
	if _, err := newReminderSettings(&fakeReminderSettingsRepo{}, "CRE").Get(serviceCtx(), "OPS"); !errors.As(err, &invalid) {
		t.Errorf("unknown family: %v", err)
	}
}

// The scheduled task reads with its service credential; nothing saved is nil.
func TestRotaReminderSettings_TheTaskReads(t *testing.T) {
	s, err := newReminderSettings(&fakeReminderSettingsRepo{}).Get(serviceCtx(), "CRE")
	if err != nil || s != nil {
		t.Errorf("got %+v, %v", s, err)
	}
}
