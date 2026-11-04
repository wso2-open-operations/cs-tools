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
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// RotaReminderRequestService is "Send now" on the Rota Access tab: a CSM
// admin or the family's rota admin asks for a rota reminder, and
// csm-scheduled-tasks (which has no address to call) claims it at its next
// run, sends it and reports how it went.
type RotaReminderRequestService interface {
	Create(ctx context.Context, family string, req domain.CreateRotaReminderRequest) (domain.RotaReminderRequest, error)
	// List is the family's latest requests, newest first, for the card.
	List(ctx context.Context, family string) ([]domain.RotaReminderRequest, error)
	// Claim and Report are the scheduled task's, by its service credential.
	Claim(ctx context.Context, family string) (*domain.RotaReminderRequest, error)
	Report(ctx context.Context, family, id string, req domain.ReportRotaReminderRequest) (domain.RotaReminderRequest, error)
}

type rotaReminderRequestService struct {
	repo       repository.RotaReminderRequestRepository
	schedule   repository.ScheduleRepository
	access     AccessService
	adminRoles []string
}

// NewRotaReminderRequestService constructs a RotaReminderRequestService.
func NewRotaReminderRequestService(repo repository.RotaReminderRequestRepository, schedule repository.ScheduleRepository, access AccessService, adminRoles []string) RotaReminderRequestService {
	return &rotaReminderRequestService{repo: repo, schedule: schedule, access: access, adminRoles: adminRoles}
}

const (
	// reminderRequestCooldown is how soon the same reminder can be asked for
	// again: a second press should not email the whole rota twice.
	reminderRequestCooldown = 10 * time.Minute
	// reminderTestCooldown is the same for a test, which emails only the
	// person asking.
	reminderTestCooldown = 2 * time.Minute
	// reminderRequestStale is when a request claimed but never reported is
	// claimed again (the run that took it died). Well beyond any run, and
	// beyond the driver's hourly tick, so a slow run's request is never
	// claimed again while it is still being sent.
	reminderRequestStale  = 2 * time.Hour
	reminderRequestListed = 5
	maxReminderResult     = 500
	// reminderRequestActor names the scheduled task in the audit trail.
	reminderRequestActor = "csm-scheduled-tasks"
)

func validReminderRequestKind(kind string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(kind))
	switch k {
	case "weekday", "morning", "evening", "weekend":
	default:
		return "", &apierror.ValidationError{Msg: "kind must be morning, evening, weekend or weekday"}
	}
	return k, nil
}

func (s *rotaReminderRequestService) Create(ctx context.Context, family string, req domain.CreateRotaReminderRequest) (domain.RotaReminderRequest, error) {
	f, err := reminderSettingsFamily(family)
	if err != nil {
		return domain.RotaReminderRequest{}, err
	}
	actor, err := rotaReminderEditor(ctx, s.access, s.schedule, s.adminRoles, f, "send the %[1]s rota reminders")
	if err != nil {
		return domain.RotaReminderRequest{}, err
	}
	kind, err := validReminderRequestKind(req.Kind)
	if err != nil {
		return domain.RotaReminderRequest{}, err
	}
	cooldown := reminderRequestCooldown
	if req.Test {
		cooldown = reminderTestCooldown
	}
	return s.repo.Create(ctx, actor, f, kind, req.Test, cooldown)
}

func (s *rotaReminderRequestService) List(ctx context.Context, family string) ([]domain.RotaReminderRequest, error) {
	if err := RequireInternalCaller(ctx, s.access, "rota reminders are internal only"); err != nil {
		return nil, err
	}
	f, err := reminderSettingsFamily(family)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, f, reminderRequestListed)
}

// requireTask admits an internal service credential and nobody else.
func (s *rotaReminderRequestService) requireTask(ctx context.Context) error {
	if err := RequireInternalCaller(ctx, s.access, "rota reminders are internal only"); err != nil {
		return err
	}
	if auth.IdentityFromContext(ctx).UserEmail != "" {
		return &apierror.ForbiddenError{Msg: "rota reminder requests are claimed by the scheduled tasks' service credential, not a user"}
	}
	return nil
}

func (s *rotaReminderRequestService) Claim(ctx context.Context, family string) (*domain.RotaReminderRequest, error) {
	if err := s.requireTask(ctx); err != nil {
		return nil, err
	}
	f, err := reminderSettingsFamily(family)
	if err != nil {
		return nil, err
	}
	return s.repo.Claim(ctx, reminderRequestActor, f, reminderRequestStale)
}

func (s *rotaReminderRequestService) Report(ctx context.Context, family, id string, req domain.ReportRotaReminderRequest) (domain.RotaReminderRequest, error) {
	if err := s.requireTask(ctx); err != nil {
		return domain.RotaReminderRequest{}, err
	}
	f, err := reminderSettingsFamily(family)
	if err != nil {
		return domain.RotaReminderRequest{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.RotaReminderRequest{}, err
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if status != "SENT" && status != "FAILED" {
		return domain.RotaReminderRequest{}, &apierror.ValidationError{Msg: "status must be SENT or FAILED"}
	}
	result := strings.Join(strings.Fields(req.Result), " ")
	if utf8.RuneCountInString(result) > maxReminderResult {
		result = string([]rune(result)[:maxReminderResult])
	}
	return s.repo.Report(ctx, reminderRequestActor, f, id, status, result)
}
