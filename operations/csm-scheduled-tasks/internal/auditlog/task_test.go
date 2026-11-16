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
package auditlog

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeMaintainer struct {
	result Result
	err    error
}

func (f fakeMaintainer) RunMaintenance(context.Context) (Result, error) { return f.result, f.err }

func TestMaintenance_SucceedsWhenNothingIsWrong(t *testing.T) {
	err := Maintenance(fakeMaintainer{result: Result{PartitionsCreated: 1}})(context.Background())
	if err != nil {
		t.Fatalf("a clean run should succeed, got %v", err)
	}
}

func TestMaintenance_FailsOnEachKindOfProblem(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   string
	}{
		{"health problem", Result{HealthProblems: []string{"rows landed in the catch-all partition"}}, "catch-all partition"},
		{"unresolved trigger", Result{UnresolvedTriggers: []TriggerProblem{{"public", "project", "registered table was renamed or dropped"}}}, "public.project is still wrong"},
		{"repaired trigger", Result{RepairedTriggers: []TriggerProblem{{"public", "account", "audit trigger is disabled"}}}, "public.account was repaired"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Maintenance(fakeMaintainer{result: tc.result})(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestMaintenance_NamesEveryProblemInOneFailure(t *testing.T) {
	result := Result{
		HealthProblems:     []string{"too few future months"},
		UnresolvedTriggers: []TriggerProblem{{"public", "project", "registered table has no audit trigger"}},
	}

	err := Maintenance(fakeMaintainer{result: result})(context.Background())
	if err == nil || !strings.Contains(err.Error(), "2 problem(s)") {
		t.Fatalf("error = %v, want both problems counted", err)
	}
}

func TestMaintenance_ReturnsTheCallError(t *testing.T) {
	boom := errors.New("boom")

	err := Maintenance(fakeMaintainer{err: boom})(context.Background())

	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap the call error", err)
	}
}
