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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// onboardingStepDefaultActor is written to created_by/updated_by when the
// caller supplies no identity of its own. Every caller is another internal
// service (csm-notification-service, the customer portal backend), which
// requireInternalCaller enforces; the label itself carries no identity.
const onboardingStepDefaultActor = "onboarding-step-api"

// maxOnboardingStepEventTypeLen is onboarding_step.event_type's column width
// (migration 000086). A longer value is refused here as a 400 rather than
// reaching Postgres and coming back as a 500 (SQLSTATE 22001).
const maxOnboardingStepEventTypeLen = 64

var validOnboardingStepName = map[domain.OnboardingStepName]bool{
	domain.OnboardingStepIdentity:     true,
	domain.OnboardingStepDatabase:     true,
	domain.OnboardingStepEmail:        true,
	domain.OnboardingStepRegistration: true,
}

var validOnboardingStepStatus = map[domain.OnboardingStepStatus]bool{
	domain.OnboardingStepSucceeded: true,
	domain.OnboardingStepFailed:    true,
	domain.OnboardingStepSkipped:   true,
}

type onboardingStepService struct {
	repo   repository.OnboardingStepRepository
	access AccessService
}

// NewOnboardingStepService constructs an OnboardingStepService. access gates
// every method to internal callers (AUTH_INTERNAL_CLIENT_IDS): onboarding
// steps carry other people's e-mail addresses and Salesforce Ids, and an
// external portal user has no business reading or writing them.
func NewOnboardingStepService(repo repository.OnboardingStepRepository, access AccessService) OnboardingStepService {
	return &onboardingStepService{repo: repo, access: access}
}

// requireInternalCaller rejects anyone whose AccessScope is not Unrestricted,
// i.e. every caller that is not an allow-listed internal service.
func (s *onboardingStepService) requireInternalCaller(ctx context.Context) error {
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: "onboarding steps are only available to internal services"}
	}
	return nil
}

// Upsert implements OnboardingStepService.
func (s *onboardingStepService) Upsert(ctx context.Context, req domain.UpsertOnboardingStepRequest) (domain.OnboardingStep, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.OnboardingStep{}, err
	}
	req.MembershipSfID = strings.TrimSpace(req.MembershipSfID)
	if req.MembershipSfID == "" {
		return domain.OnboardingStep{}, &apierror.ValidationError{Msg: "membershipSfId is required"}
	}
	req.Step = domain.OnboardingStepName(strings.ToUpper(strings.TrimSpace(string(req.Step))))
	if !validOnboardingStepName[req.Step] {
		return domain.OnboardingStep{}, &apierror.ValidationError{Msg: "step must be one of IDENTITY, DATABASE, EMAIL, REGISTRATION"}
	}
	req.Status = domain.OnboardingStepStatus(strings.ToUpper(strings.TrimSpace(string(req.Status))))
	if !validOnboardingStepStatus[req.Status] {
		return domain.OnboardingStep{}, &apierror.ValidationError{Msg: "status must be one of SUCCEEDED, FAILED, SKIPPED"}
	}
	req.EventType = strings.TrimSpace(req.EventType)
	if req.EventType == "" {
		return domain.OnboardingStep{}, &apierror.ValidationError{Msg: "eventType is required"}
	}
	if utf8.RuneCountInString(req.EventType) > maxOnboardingStepEventTypeLen {
		return domain.OnboardingStep{}, &apierror.ValidationError{Msg: fmt.Sprintf("eventType cannot exceed %d characters", maxOnboardingStepEventTypeLen)}
	}
	if req.EventModifiedOn.IsZero() {
		return domain.OnboardingStep{}, &apierror.ValidationError{Msg: "eventModifiedOn is required"}
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		return domain.OnboardingStep{}, &apierror.ValidationError{Msg: "email is required"}
	}
	if req.Status != domain.OnboardingStepFailed {
		// A stale error message must not survive a later success.
		req.LastError = nil
	} else if req.LastError != nil {
		trimmed := truncateOnboardingStepError(*req.LastError)
		req.LastError = &trimmed
	}
	req.ContactSfID = optionalPtr(req.ContactSfID)
	req.ProjectID = optionalPtr(req.ProjectID)
	req.ProjectContactID = optionalPtr(req.ProjectContactID)
	if req.ProjectID != nil {
		if err := validateUUIDs("projectId", []string{*req.ProjectID}); err != nil {
			return domain.OnboardingStep{}, err
		}
	}
	if req.ProjectContactID != nil {
		if err := validateUUIDs("projectContactId", []string{*req.ProjectContactID}); err != nil {
			return domain.OnboardingStep{}, err
		}
	}
	if strings.TrimSpace(req.UpdatedBy) == "" {
		req.UpdatedBy = onboardingStepDefaultActor
	}
	return s.repo.Upsert(ctx, req)
}

// GetByMembership implements OnboardingStepService.
func (s *onboardingStepService) GetByMembership(ctx context.Context, membershipSfID string) (domain.GetOnboardingStepsResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.GetOnboardingStepsResponse{}, err
	}
	membershipSfID = strings.TrimSpace(membershipSfID)
	if membershipSfID == "" {
		return domain.GetOnboardingStepsResponse{}, &apierror.ValidationError{Msg: "membershipSfId is required"}
	}
	steps, err := s.repo.GetByMembership(ctx, membershipSfID)
	if err != nil {
		return domain.GetOnboardingStepsResponse{}, err
	}
	if steps == nil {
		steps = []domain.OnboardingStep{}
	}
	return domain.GetOnboardingStepsResponse{Steps: steps}, nil
}

// Search implements OnboardingStepService.
func (s *onboardingStepService) Search(ctx context.Context, req domain.SearchOnboardingStepsRequest) (domain.SearchOnboardingStepsResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.SearchOnboardingStepsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchOnboardingStepsResponse{}, err
	}
	req.Filters.ProjectID = optionalPtr(req.Filters.ProjectID)
	if req.Filters.ProjectID != nil {
		if err := validateUUIDs("filters.projectId", []string{*req.Filters.ProjectID}); err != nil {
			return domain.SearchOnboardingStepsResponse{}, err
		}
	}
	ids := make([]string, 0, len(req.Filters.MembershipSfIDs))
	for _, id := range req.Filters.MembershipSfIDs {
		if t := strings.TrimSpace(id); t != "" {
			ids = append(ids, t)
		}
	}
	req.Filters.MembershipSfIDs = ids
	for i, st := range req.Filters.Statuses {
		norm := domain.OnboardingStepStatus(strings.ToUpper(strings.TrimSpace(string(st))))
		if !validOnboardingStepStatus[norm] {
			return domain.SearchOnboardingStepsResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("filters.statuses contains unknown status %q", string(st))}
		}
		req.Filters.Statuses[i] = norm
	}

	steps, total, err := s.repo.Search(ctx, req)
	if err != nil {
		return domain.SearchOnboardingStepsResponse{}, err
	}
	if steps == nil {
		steps = []domain.OnboardingStep{}
	}
	return domain.SearchOnboardingStepsResponse{
		Steps:  steps,
		Total:  total,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}
