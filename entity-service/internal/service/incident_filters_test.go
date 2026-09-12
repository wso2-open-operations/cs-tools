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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestParseIncidentFieldFilters_StateGroupAndBusinessService(t *testing.T) {
	groupID := "11111111-1111-1111-1111-111111111111"
	serviceID := "22222222-2222-2222-2222-222222222222"

	parsed, err := ParseIncidentFieldFilters([]domain.IncidentFieldFilter{
		{Field: "state", Op: "in", Values: []string{"NEW", "IN_PROGRESS"}},
		{Field: "assignmentGroupId", Op: "in", Values: []string{groupID}},
		{Field: "businessServiceId", Op: "in", Values: []string{serviceID}},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.AssignmentGroupIDs) != 1 || parsed.AssignmentGroupIDs[0] != groupID {
		t.Errorf("AssignmentGroupIDs = %v, want [%s]", parsed.AssignmentGroupIDs, groupID)
	}
	if len(parsed.BusinessServiceIDs) != 1 || parsed.BusinessServiceIDs[0] != serviceID {
		t.Errorf("BusinessServiceIDs = %v, want [%s]", parsed.BusinessServiceIDs, serviceID)
	}
}

func TestParseIncidentFieldFilters_CreatedOnRelativeDate(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	parsed, err := ParseIncidentFieldFilters([]domain.IncidentFieldFilter{
		{Field: "createdOn", Op: "gte", Values: []string{"__daysAgo:90__"}},
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.StartCreatedDate == nil {
		t.Fatal("StartCreatedDate = nil, want a resolved time")
	}
	want := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	if !parsed.StartCreatedDate.Equal(want) {
		t.Errorf("StartCreatedDate = %v, want %v", parsed.StartCreatedDate, want)
	}
	if parsed.EndCreatedDate != nil {
		t.Errorf("EndCreatedDate = %v, want nil", parsed.EndCreatedDate)
	}
}

func TestParseIncidentFieldFilters_SlaViolated(t *testing.T) {
	parsed, err := ParseIncidentFieldFilters([]domain.IncidentFieldFilter{
		{Field: "slaViolated", Op: "eq", Values: []string{"true"}},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.SlaViolated == nil || !*parsed.SlaViolated {
		t.Fatalf("SlaViolated = %v, want pointer to true", parsed.SlaViolated)
	}

	parsed, err = ParseIncidentFieldFilters([]domain.IncidentFieldFilter{
		{Field: "slaViolated", Op: "eq", Values: []string{"false"}},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.SlaViolated == nil || *parsed.SlaViolated {
		t.Fatalf("SlaViolated = %v, want pointer to false", parsed.SlaViolated)
	}
}

func TestParseIncidentFieldFilters_MadeSla(t *testing.T) {
	parsed, err := ParseIncidentFieldFilters([]domain.IncidentFieldFilter{
		{Field: "madeSla", Op: "eq", Values: []string{"true"}},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.MadeSla == nil || !*parsed.MadeSla {
		t.Fatalf("MadeSla = %v, want pointer to true", parsed.MadeSla)
	}

	parsed, err = ParseIncidentFieldFilters([]domain.IncidentFieldFilter{
		{Field: "madeSla", Op: "eq", Values: []string{"false"}},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.MadeSla == nil || *parsed.MadeSla {
		t.Fatalf("MadeSla = %v, want pointer to false", parsed.MadeSla)
	}
}

func TestParseIncidentFieldFilters_ProductName(t *testing.T) {
	parsed, err := ParseIncidentFieldFilters([]domain.IncidentFieldFilter{
		{Field: "productName", Op: "in", Values: []string{"API Manager", "Choreo"}},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"API Manager", "Choreo"}
	if len(parsed.ProductNames) != len(want) {
		t.Fatalf("ProductNames = %v, want %v", parsed.ProductNames, want)
	}
	for i, v := range want {
		if parsed.ProductNames[i] != v {
			t.Errorf("ProductNames[%d] = %q, want %q", i, parsed.ProductNames[i], v)
		}
	}
}

func TestParseIncidentFieldFilters_Rejections(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name    string
		filters []domain.IncidentFieldFilter
	}{
		{
			name:    "unsupported field",
			filters: []domain.IncidentFieldFilter{{Field: "priority", Op: "in", Values: []string{"1"}}},
		},
		{
			name:    "unsupported op",
			filters: []domain.IncidentFieldFilter{{Field: "state", Op: "eq", Values: []string{"NEW"}}},
		},
		{
			name:    "empty values",
			filters: []domain.IncidentFieldFilter{{Field: "state", Op: "in", Values: []string{}}},
		},
		{
			name:    "invalid state value",
			filters: []domain.IncidentFieldFilter{{Field: "state", Op: "in", Values: []string{"BOGUS"}}},
		},
		{
			name:    "invalid assignmentGroupId UUID",
			filters: []domain.IncidentFieldFilter{{Field: "assignmentGroupId", Op: "in", Values: []string{"not-a-uuid"}}},
		},
		{
			name:    "createdOn with unsupported op",
			filters: []domain.IncidentFieldFilter{{Field: "createdOn", Op: "in", Values: []string{"2026-01-01"}}},
		},
		{
			name:    "createdOn with unparseable value",
			filters: []domain.IncidentFieldFilter{{Field: "createdOn", Op: "gte", Values: []string{"not-a-date"}}},
		},
		{
			name:    "slaViolated with unsupported op",
			filters: []domain.IncidentFieldFilter{{Field: "slaViolated", Op: "in", Values: []string{"true"}}},
		},
		{
			name:    "slaViolated with non-boolean value",
			filters: []domain.IncidentFieldFilter{{Field: "slaViolated", Op: "eq", Values: []string{"yes"}}},
		},
		{
			name:    "slaViolated with more than one value",
			filters: []domain.IncidentFieldFilter{{Field: "slaViolated", Op: "eq", Values: []string{"true", "false"}}},
		},
		{
			name:    "madeSla with unsupported op",
			filters: []domain.IncidentFieldFilter{{Field: "madeSla", Op: "in", Values: []string{"true"}}},
		},
		{
			name:    "madeSla with non-boolean value",
			filters: []domain.IncidentFieldFilter{{Field: "madeSla", Op: "eq", Values: []string{"yes"}}},
		},
		{
			name:    "madeSla with more than one value",
			filters: []domain.IncidentFieldFilter{{Field: "madeSla", Op: "eq", Values: []string{"true", "false"}}},
		},
		{
			name:    "productName with unsupported op",
			filters: []domain.IncidentFieldFilter{{Field: "productName", Op: "eq", Values: []string{"Choreo"}}},
		},
		{
			name:    "productName with empty values",
			filters: []domain.IncidentFieldFilter{{Field: "productName", Op: "in", Values: []string{}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseIncidentFieldFilters(tt.filters, now); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}
