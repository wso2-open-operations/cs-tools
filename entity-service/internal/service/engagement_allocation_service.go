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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// cycleStartDateLayout is the only accepted form of the cycleStartDate query
// parameter. A bare date, not a timestamp: the underlying columns are DATE,
// and accepting an instant would invite a timezone to creep into a boundary
// that has none.
const cycleStartDateLayout = "2006-01-02"

type engagementAllocationService struct {
	repo repository.EngagementAllocationRepository
}

// NewEngagementAllocationService constructs an EngagementAllocationService.
func NewEngagementAllocationService(repo repository.EngagementAllocationRepository) EngagementAllocationService {
	return &engagementAllocationService{repo: repo}
}

// StatusUpdateReminderRecipients validates cycleStartDate and returns the
// people who owe an update for that cycle.
func (s *engagementAllocationService) StatusUpdateReminderRecipients(ctx context.Context, cycleStartDate string) (domain.StatusUpdateReminderResponse, error) {
	if cycleStartDate == "" {
		return domain.StatusUpdateReminderResponse{}, &apierror.ValidationError{Msg: "cycleStartDate is required"}
	}
	// time.Parse with a date-only layout yields midnight UTC, which is what
	// the DATE columns compare against.
	cycleStart, err := time.Parse(cycleStartDateLayout, cycleStartDate)
	if err != nil {
		return domain.StatusUpdateReminderResponse{}, &apierror.ValidationError{Msg: "cycleStartDate must be a date in YYYY-MM-DD form"}
	}

	recipients, err := s.repo.StatusUpdateReminderRecipients(ctx, cycleStart)
	if err != nil {
		return domain.StatusUpdateReminderResponse{}, err
	}
	return domain.StatusUpdateReminderResponse{
		CycleStartDate: cycleStartDate,
		Count:          len(recipients),
		Recipients:     recipients,
	}, nil
}
