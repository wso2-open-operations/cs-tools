/*
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package service

import (
	"context"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	defaultValidationQueueLimit = 200
	maxValidationQueueLimit     = 1000
	maxValidationValues         = 50
)

// ValidationService backs the plg_registration_validation job's two routes.
type ValidationService interface {
	Queue(ctx context.Context, req domain.ValidationQueueRequest) (domain.ValidationQueueResult, error)
	Apply(ctx context.Context, req domain.ApplyValidationRequest) (domain.ApplyValidationResult, error)
}

type validationService struct {
	repo repository.ValidationRepository
}

// NewValidationService wires a ValidationService over its repository.
func NewValidationService(repo repository.ValidationRepository) ValidationService {
	return &validationService{repo: repo}
}

func (s *validationService) Queue(ctx context.Context, req domain.ValidationQueueRequest) (domain.ValidationQueueResult, error) {
	code := strings.ToUpper(strings.TrimSpace(req.ProductCode))
	if code == "" {
		return domain.ValidationQueueResult{}, invalidField("productCode is required")
	}
	if req.RegisteredAfter.IsZero() {
		return domain.ValidationQueueResult{}, invalidField("registeredAfter is required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = defaultValidationQueueLimit
	}
	if limit > maxValidationQueueLimit {
		limit = maxValidationQueueLimit
	}
	items, err := s.repo.Queue(ctx, code, req.RegisteredAfter, limit)
	if err != nil {
		return domain.ValidationQueueResult{}, err
	}
	return domain.ValidationQueueResult{Items: items}, nil
}

func (s *validationService) Apply(ctx context.Context, req domain.ApplyValidationRequest) (domain.ApplyValidationResult, error) {
	if err := validateUUID("orgPlatformId", req.OrgPlatformID); err != nil {
		return domain.ApplyValidationResult{}, err
	}
	if err := validateSummary(req.Summary); err != nil {
		return domain.ApplyValidationResult{}, err
	}
	if err := validateValues(req.Values); err != nil {
		return domain.ApplyValidationResult{}, err
	}
	if req.Abandon && !sendsExcluded(req.Values) {
		return domain.ApplyValidationResult{}, invalidField(
			"abandon needs " + domain.TaskCodeEligibilityOutcome + " = " + domain.OutcomeExcluded)
	}
	res, err := s.repo.Apply(ctx, req)
	plgAudit(ctx, "apply registration validation", "", err,
		"orgPlatformId", req.OrgPlatformID, "attached", res.Attached,
		"written", len(res.Written), "skipped", len(res.Skipped), "abandoned", res.Abandoned)
	return res, err
}

func sendsExcluded(values []domain.ValidationValue) bool {
	for _, v := range values {
		if v.Code == domain.TaskCodeEligibilityOutcome && v.TextValue != nil && *v.TextValue == domain.OutcomeExcluded {
			return true
		}
	}
	return false
}

func validateSummary(s domain.ActivitySummary) error {
	if s.WindowStart.IsZero() || s.WindowEnd.IsZero() {
		return invalidField("summary.windowStart and summary.windowEnd are required")
	}
	if s.WindowEnd.Before(s.WindowStart) {
		return invalidField("summary.windowEnd is before summary.windowStart")
	}
	if s.EventCount < 0 {
		return invalidField("summary.eventCount must not be negative")
	}
	for action, n := range s.ActionCounts {
		if strings.TrimSpace(action) == "" || n < 0 {
			return invalidField("summary.actionCounts needs non-empty actions and non-negative counts")
		}
	}
	return nil
}

func validateValues(values []domain.ValidationValue) error {
	if len(values) > maxValidationValues {
		return invalidField("too many values")
	}
	seen := make(map[string]bool, len(values))
	for i, v := range values {
		if strings.TrimSpace(v.Code) == "" {
			return invalidField("values[" + plgItoa(i) + "].code is required")
		}
		// The job opens the run; it closes one only by abandoning the pairing.
		if v.Code == domain.TaskCodeClose {
			return invalidField(v.Code + " is closed through abandon only")
		}
		if v.Code == domain.TaskCodeInitiate && (v.BoolValue == nil || !*v.BoolValue) {
			return invalidField(v.Code + " takes boolValue true")
		}
		if seen[v.Code] {
			return invalidField("values repeats " + v.Code)
		}
		seen[v.Code] = true
	}
	return nil
}
