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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeReminderRequestRepo struct {
	created, reported string
	claimed           bool
	cooldown          time.Duration
}

func (f *fakeReminderRequestRepo) Create(_ context.Context, actor, family, kind string, test bool, cooldown time.Duration) (domain.RotaReminderRequest, error) {
	f.created, f.cooldown = actor+" "+family+" "+kind, cooldown
	if test {
		f.created += " test"
	}
	return domain.RotaReminderRequest{Family: family, Kind: kind, Status: "QUEUED", RequestedBy: actor}, nil
}

func (f *fakeReminderRequestRepo) List(context.Context, string, int) ([]domain.RotaReminderRequest, error) {
	return []domain.RotaReminderRequest{}, nil
}

func (f *fakeReminderRequestRepo) Claim(context.Context, string, string, time.Duration) (*domain.RotaReminderRequest, error) {
	f.claimed = true
	return nil, nil
}

func (f *fakeReminderRequestRepo) Report(_ context.Context, _, family, id, status, result string) (domain.RotaReminderRequest, error) {
	f.reported = family + " " + id + " " + status + " " + result
	return domain.RotaReminderRequest{ID: id, Status: status, Result: result}, nil
}

func newReminderRequests(repo *fakeReminderRequestRepo, families ...string) RotaReminderRequestService {
	return NewRotaReminderRequestService(repo, &rotaAdminOf{families: families}, alwaysUnrestrictedAccess{}, []string{"admin"})
}

// Asking: a CSM admin or the family's rota admin, by a person; the kind is
// weekday or weekend; the ten-minute gap is asked of the repository.
func TestRotaReminderRequest_WhoMayAsk(t *testing.T) {
	cases := []struct {
		name     string
		ctx      context.Context
		families []string
		family   string
		ok       bool
	}{
		{"CSM admin", callerWithRoles("boss@example.com", "admin"), nil, "CRE", true},
		{"CRE rota admin", callerWithRoles("ann@example.com"), []string{"CRE"}, "cre", true},
		{"SRE rota admin", callerWithRoles("sam@example.com"), []string{"SRE"}, "CRE", false},
		{"anybody else", callerWithRoles("joe@example.com", "cs_engineer"), nil, "CRE", false},
		{"service credential", serviceCtx(), []string{"CRE"}, "CRE", false},
	}
	for _, c := range cases {
		repo := &fakeReminderRequestRepo{}
		_, err := newReminderRequests(repo, c.families...).Create(c.ctx, c.family, domain.CreateRotaReminderRequest{Kind: "Weekend"})
		var forbidden *apierror.ForbiddenError
		switch {
		case c.ok && (err != nil || repo.created == "" || repo.cooldown != 10*time.Minute):
			t.Errorf("%s: err %v, created %q, cooldown %v", c.name, err, repo.created, repo.cooldown)
		case !c.ok && (!errors.As(err, &forbidden) || repo.created != ""):
			t.Errorf("%s: err %v, created %q", c.name, err, repo.created)
		}
	}
	repo := &fakeReminderRequestRepo{}
	var invalid *apierror.ValidationError
	if _, err := newReminderRequests(repo, "CRE").Create(callerWithRoles("ann@example.com"), "CRE", domain.CreateRotaReminderRequest{Kind: "monthly"}); !errors.As(err, &invalid) || repo.created != "" {
		t.Errorf("bad kind: %v, created %q", err, repo.created)
	}
	// Send now offers the Morning and the Evening apart; weekday (both) stays
	// accepted for a portal not yet updated.
	for _, kind := range []string{"morning", "Evening", "weekday", "weekend"} {
		repo := &fakeReminderRequestRepo{}
		if _, err := newReminderRequests(repo, "CRE").Create(callerWithRoles("ann@example.com"), "CRE", domain.CreateRotaReminderRequest{Kind: kind}); err != nil || repo.created != "ann@example.com CRE "+strings.ToLower(kind) {
			t.Errorf("kind %q: err %v, created %q", kind, err, repo.created)
		}
	}
}

// Claiming and reporting are the scheduled task's alone; a report is SENT or
// FAILED, with a one-line result.
func TestRotaReminderRequest_TheTaskClaimsAndReports(t *testing.T) {
	repo := &fakeReminderRequestRepo{}
	svc := newReminderRequests(repo, "CRE")
	user := callerWithRoles("boss@example.com", "admin")
	var forbidden *apierror.ForbiddenError
	if _, err := svc.Claim(user, "CRE"); !errors.As(err, &forbidden) || repo.claimed {
		t.Errorf("a person claimed: %v", err)
	}
	if _, err := svc.Claim(serviceCtx(), "CRE"); err != nil || !repo.claimed {
		t.Errorf("the task's claim: %v", err)
	}
	const id = "11111111-1111-4111-8111-111111111111"
	if _, err := svc.Report(user, "CRE", id, domain.ReportRotaReminderRequest{Status: "SENT"}); !errors.As(err, &forbidden) {
		t.Errorf("a person reported: %v", err)
	}
	var invalid *apierror.ValidationError
	for _, bad := range []struct{ id, status string }{{"not-a-uuid", "SENT"}, {id, "QUEUED"}} {
		if _, err := svc.Report(serviceCtx(), "CRE", bad.id, domain.ReportRotaReminderRequest{Status: bad.status}); !errors.As(err, &invalid) {
			t.Errorf("report %+v: %v", bad, err)
		}
	}
	if _, err := svc.Report(serviceCtx(), "cre", id, domain.ReportRotaReminderRequest{Status: "failed", Result: "mail\nservice   down"}); err != nil ||
		repo.reported != "CRE "+id+" FAILED mail service down" {
		t.Errorf("reported %q: %v", repo.reported, err)
	}
}

// A test has its own, shorter gap.
func TestRotaReminderRequest_ATestIsItsOwnRequest(t *testing.T) {
	repo := &fakeReminderRequestRepo{}
	if _, err := newReminderRequests(repo, "CRE").Create(callerWithRoles("ann@example.com"), "CRE", domain.CreateRotaReminderRequest{Kind: "weekday", Test: true}); err != nil {
		t.Fatal(err)
	}
	if repo.created != "ann@example.com CRE weekday test" || repo.cooldown != 2*time.Minute {
		t.Errorf("created %q with cooldown %v", repo.created, repo.cooldown)
	}
}
