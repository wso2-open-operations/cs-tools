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
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// OnboardingDates is the parsed form of ProjectUpdateRequest's four onboarding
// dates; a nil field was not supplied.
type OnboardingDates struct {
	Start, GoLivePlan, GoLive, Expiry *time.Time
}

// ParseOnboardingDates parses the request's onboarding dates ("YYYY-MM-DD") and
// rejects, within the request alone, a go-live plan, go-live or expiry date
// earlier than the start date. A malformed or impossible date is a
// ValidationError. The repository additionally compares against the stored
// start date.
func ParseOnboardingDates(req domain.ProjectUpdateRequest) (OnboardingDates, error) {
	var d OnboardingDates
	for _, f := range []struct {
		field string
		in    *string
		out   **time.Time
	}{
		{"onboardingStartDate", req.OnboardingStartDate, &d.Start},
		{"onboardingGoLivePlanDate", req.OnboardingGoLivePlanDate, &d.GoLivePlan},
		{"onboardingGoLiveDate", req.OnboardingGoLiveDate, &d.GoLive},
		{"onboardingExpiryDate", req.OnboardingExpiryDate, &d.Expiry},
	} {
		if f.in == nil {
			continue
		}
		t, err := time.Parse(time.DateOnly, *f.in)
		if err != nil {
			return d, &apierror.ValidationError{Msg: fmt.Sprintf("%s must be a date in YYYY-MM-DD format", f.field)}
		}
		*f.out = &t
	}
	return d, d.checkOrder(d.Start)
}

// checkOrder rejects a supplied plan/go-live/expiry date earlier than start.
func (d OnboardingDates) checkOrder(start *time.Time) error {
	if start == nil {
		return nil
	}
	for _, f := range []struct {
		field string
		t     *time.Time
	}{
		{"onboardingGoLivePlanDate", d.GoLivePlan},
		{"onboardingGoLiveDate", d.GoLive},
		{"onboardingExpiryDate", d.Expiry},
	} {
		if f.t != nil && f.t.Before(*start) {
			return &apierror.ValidationError{Msg: f.field + " must not be earlier than onboardingStartDate"}
		}
	}
	return nil
}
