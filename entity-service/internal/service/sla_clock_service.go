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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// validSLAClockTier mirrors the three columns SLAClockRepository knows how
// to write — kept here (not in the repository) so an invalid tier is
// rejected as a 400 before it ever reaches a query.
var validSLAClockTier = map[string]bool{"50": true, "75": true, "100": true}

// validSLATierStatus is the allow-list for SetSLAClockTierRequest.Status —
// see domain.SLATierStatus's doc comment for why this is an enum with
// exactly one member today rather than a bare boolean.
var validSLATierStatus = map[domain.SLATierStatus]bool{domain.SLATierStatusReached: true}

type slaClockService struct {
	repo repository.SLAClockRepository
}

// NewSLAClockService constructs an SLAClockService backed by the given
// repository.
func NewSLAClockService(repo repository.SLAClockRepository) SLAClockService {
	return &slaClockService{repo: repo}
}

// RegisterSLAClock implements SLAClockService.
func (s *slaClockService) RegisterSLAClock(ctx context.Context, req domain.RegisterSLAClockRequest) (domain.SLAClock, error) {
	if req.CaseID == "" {
		return domain.SLAClock{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if req.ClockType == "" {
		return domain.SLAClock{}, &apierror.ValidationError{Msg: "clockType is required"}
	}
	if req.StartedAt.IsZero() {
		return domain.SLAClock{}, &apierror.ValidationError{Msg: "startedAt is required"}
	}
	if req.DueAt.IsZero() {
		return domain.SLAClock{}, &apierror.ValidationError{Msg: "dueAt is required"}
	}
	if !req.DueAt.After(req.StartedAt) {
		return domain.SLAClock{}, &apierror.ValidationError{Msg: "dueAt must be after startedAt"}
	}
	return s.repo.Register(ctx, req)
}

// GetSLAClock implements SLAClockService.
func (s *slaClockService) GetSLAClock(ctx context.Context, caseID, clockType string) (domain.SLAClock, error) {
	if caseID == "" {
		return domain.SLAClock{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if clockType == "" {
		return domain.SLAClock{}, &apierror.ValidationError{Msg: "clockType is required"}
	}
	return s.repo.Get(ctx, caseID, clockType)
}

// SetSLAClockTierReached implements SLAClockService.
func (s *slaClockService) SetSLAClockTierReached(ctx context.Context, caseID, clockType, tier string, req domain.SetSLAClockTierRequest) (domain.SetSLAClockTierReachedResponse, error) {
	if caseID == "" {
		return domain.SetSLAClockTierReachedResponse{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if clockType == "" {
		return domain.SetSLAClockTierReachedResponse{}, &apierror.ValidationError{Msg: "clockType is required"}
	}
	if !validSLAClockTier[tier] {
		return domain.SetSLAClockTierReachedResponse{}, &apierror.ValidationError{Msg: "tier must be one of 50, 75, 100"}
	}
	if !validSLATierStatus[req.Status] {
		return domain.SetSLAClockTierReachedResponse{}, &apierror.ValidationError{Msg: `status must be "reached"`}
	}
	reachedAt, alreadyReached, err := s.repo.SetTierReachedIfUnset(ctx, caseID, clockType, tier)
	if err != nil {
		return domain.SetSLAClockTierReachedResponse{}, err
	}
	return domain.SetSLAClockTierReachedResponse{ReachedOn: reachedAt, AlreadyReached: alreadyReached}, nil
}

// Pause implements SLAClockService.
func (s *slaClockService) Pause(ctx context.Context, caseID, clockType string) (domain.SLAClock, error) {
	if caseID == "" {
		return domain.SLAClock{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if clockType == "" {
		return domain.SLAClock{}, &apierror.ValidationError{Msg: "clockType is required"}
	}
	return s.repo.SetPaused(ctx, caseID, clockType, true)
}

// Resume implements SLAClockService.
func (s *slaClockService) Resume(ctx context.Context, caseID, clockType string) (domain.SLAClock, error) {
	if caseID == "" {
		return domain.SLAClock{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if clockType == "" {
		return domain.SLAClock{}, &apierror.ValidationError{Msg: "clockType is required"}
	}
	return s.repo.SetPaused(ctx, caseID, clockType, false)
}
