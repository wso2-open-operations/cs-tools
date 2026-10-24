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

package repository

import (
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestParseOnboardingDates(t *testing.T) {
	str := func(s string) *string { return &s }
	d, err := ParseOnboardingDates(domain.ProjectUpdateRequest{OnboardingStartDate: str("2026-03-01"), OnboardingGoLiveDate: str("2026-03-01")})
	if err != nil || d.Start == nil || d.GoLive == nil || d.GoLivePlan != nil || d.Expiry != nil {
		t.Fatalf("same-day pair: %+v, %v", d, err)
	}
	if d, err = ParseOnboardingDates(domain.ProjectUpdateRequest{}); err != nil || d != (OnboardingDates{}) {
		t.Fatalf("empty: %+v, %v", d, err)
	}
	for name, req := range map[string]domain.ProjectUpdateRequest{
		"malformed":    {OnboardingStartDate: str("2026-3-1x")},
		"empty string": {OnboardingExpiryDate: str("")},
		"order":        {OnboardingStartDate: str("2026-03-02"), OnboardingGoLiveDate: str("2026-03-01")},
	} {
		var ve *apierror.ValidationError
		if _, err := ParseOnboardingDates(req); !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want ValidationError", name, err)
		}
	}
}
