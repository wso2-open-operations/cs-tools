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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// noopSLAClockService is a shared do-nothing SLAClockService for the many
// sn_case_*_test.go tests that construct a snCaseService but don't exercise
// SLA behavior — applyCaseStateSLAEffects (sn_case_service.go) calls
// Pause/Resume unconditionally on every genuine state-changing UpdateCase,
// so those tests need a non-nil, non-panicking SLAClockService even though
// they aren't testing SLA clocks themselves.
type noopSLAClockService struct{}

func (noopSLAClockService) RegisterSLAClock(context.Context, domain.RegisterSLAClockRequest) (domain.SLAClock, error) {
	return domain.SLAClock{}, nil
}

func (noopSLAClockService) GetSLAClock(context.Context, string, string) (domain.SLAClock, error) {
	return domain.SLAClock{}, nil
}

func (noopSLAClockService) SetSLAClockTierReached(context.Context, string, string, string, domain.SetSLAClockTierRequest) (domain.SetSLAClockTierReachedResponse, error) {
	return domain.SetSLAClockTierReachedResponse{}, nil
}

func (noopSLAClockService) Pause(context.Context, string, string) (domain.SLAClock, error) {
	return domain.SLAClock{}, nil
}

func (noopSLAClockService) Resume(context.Context, string, string) (domain.SLAClock, error) {
	return domain.SLAClock{}, nil
}
