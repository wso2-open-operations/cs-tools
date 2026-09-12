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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestParseProblemFieldFilters_StateAndAssignmentGroup(t *testing.T) {
	groupID := "11111111-1111-1111-1111-111111111111"

	parsed, err := ParseProblemFieldFilters([]domain.ProblemFieldFilter{
		{Field: "state", Op: "in", Values: []string{"NEW", "FIX_IN_PROGRESS", "CLOSED"}},
		{Field: "assignmentGroupId", Op: "in", Values: []string{groupID}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantStates := []int{101, 104, 107}
	if len(parsed.StateKeys) != len(wantStates) {
		t.Fatalf("StateKeys = %v, want %v", parsed.StateKeys, wantStates)
	}
	for i, k := range wantStates {
		if parsed.StateKeys[i] != k {
			t.Errorf("StateKeys[%d] = %d, want %d", i, parsed.StateKeys[i], k)
		}
	}
	if len(parsed.AssignmentGroupIDs) != 1 || parsed.AssignmentGroupIDs[0] != groupID {
		t.Errorf("AssignmentGroupIDs = %v, want [%s]", parsed.AssignmentGroupIDs, groupID)
	}
}

func TestParseProblemFieldFilters_AssignedUserId(t *testing.T) {
	userID := "33333333-3333-3333-3333-333333333333"

	parsed, err := ParseProblemFieldFilters([]domain.ProblemFieldFilter{
		{Field: "assignedUserId", Op: "in", Values: []string{userID}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.AssignedUserIDs) != 1 || parsed.AssignedUserIDs[0] != userID {
		t.Errorf("AssignedUserIDs = %v, want [%s]", parsed.AssignedUserIDs, userID)
	}
}

func TestParseProblemFieldFilters_Rejections(t *testing.T) {
	tests := []struct {
		name    string
		filters []domain.ProblemFieldFilter
	}{
		{
			name:    "unsupported field",
			filters: []domain.ProblemFieldFilter{{Field: "priority", Op: "in", Values: []string{"1"}}},
		},
		{
			name:    "unsupported op",
			filters: []domain.ProblemFieldFilter{{Field: "state", Op: "eq", Values: []string{"NEW"}}},
		},
		{
			name:    "invalid state value",
			filters: []domain.ProblemFieldFilter{{Field: "state", Op: "in", Values: []string{"BOGUS_STATE"}}},
		},
		{
			name:    "empty values",
			filters: []domain.ProblemFieldFilter{{Field: "state", Op: "in", Values: nil}},
		},
		{
			name:    "invalid assignmentGroupId uuid",
			filters: []domain.ProblemFieldFilter{{Field: "assignmentGroupId", Op: "in", Values: []string{"not-a-uuid"}}},
		},
		{
			name:    "invalid assignedUserId uuid",
			filters: []domain.ProblemFieldFilter{{Field: "assignedUserId", Op: "in", Values: []string{"not-a-uuid"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseProblemFieldFilters(tt.filters)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var v *apierror.ValidationError
			if !errors.As(err, &v) {
				t.Errorf("error = %v, want *apierror.ValidationError", err)
			}
		})
	}
}
