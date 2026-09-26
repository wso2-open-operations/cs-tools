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
package repository

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

func TestOnboardingStatusEnumLabels(t *testing.T) {
	// The dashboard sends ServiceNow's spellings; each must land on the enum
	// label, regardless of case or separator.
	got, err := onboardingStatusEnumLabels([]string{"Not-Started", "In-Progress", "Completed", "OnHold", "Not-Applicable", "Expired", "Cancelled", " on_hold ", "IN PROGRESS"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"NOT_STARTED", "IN_PROGRESS", "COMPLETED", "ON_HOLD", "NOT_APPLICABLE", "EXPIRED", "CANCELLED", "ON_HOLD", "IN_PROGRESS"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("labels = %v, want %v", got, want)
	}

	// An unknown value must fail loudly: silently matching nothing would
	// widen a notIn.
	_, err = onboardingStatusEnumLabels([]string{"Completed", "Bogus"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *apierror.ValidationError", err)
	}
}

// Every enum label onboardingStatusLabels can produce must exist in the
// migration's onboarding_status_enum, or a valid filter would fail at query time.
func TestOnboardingStatusLabelsMatchMigration(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/000009_projects_table.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range onboardingStatusLabels {
		if !strings.Contains(string(raw), "'"+label+"'") {
			t.Errorf("%s is not an onboarding_status_enum label in migration 000009", label)
		}
	}
}
