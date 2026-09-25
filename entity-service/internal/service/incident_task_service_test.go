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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubIncidentTaskRepo is a minimal repository.IncidentTaskRepository whose
// methods panic if called -- same convention as stubIncidentRepo.
type stubIncidentTaskRepo struct{}

func (s *stubIncidentTaskRepo) SearchIncidentTasks(context.Context, domain.SearchIncidentTasksRequest, []string, []string) ([]domain.IncidentTask, int, error) {
	panic("not implemented")
}
func (s *stubIncidentTaskRepo) AggregateIncidentTasks(context.Context, domain.SearchIncidentTasksRequest, []string, []string, string, int) (domain.AggregateResponse, error) {
	panic("not implemented")
}
func (s *stubIncidentTaskRepo) GetIncidentTask(context.Context, string) (domain.IncidentTaskDetail, error) {
	panic("not implemented")
}

// TestIncidentTaskService_RequiresInternalCaller is the core regression
// guard for the incident_task authorization gap: SearchIncidentTasks,
// AggregateIncidentTasks, and GetIncidentTask applied no authorization at
// all before this fix. Confirmed live against a real database copy: 100%
// of real incident_task rows have project_id = NULL -- these are internal
// ITIL/ops records, not customer-project-scoped data, so "internal caller
// only" (not project scoping) is the correct fix, mirroring
// incidentService's identical requireInternalCaller pattern.
func TestIncidentTaskService_RequiresInternalCaller(t *testing.T) {
	external := stubAccess{scope: AccessScope{ProjectIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}}

	assertForbidden := func(t *testing.T, err error) {
		t.Helper()
		var fe *apierror.ForbiddenError
		if !errors.As(err, &fe) {
			t.Fatalf("expected *apierror.ForbiddenError, got %T: %v", err, err)
		}
	}

	t.Run("SearchIncidentTasks", func(t *testing.T) {
		svc := NewIncidentTaskService(&stubIncidentTaskRepo{}, external)
		_, err := svc.SearchIncidentTasks(context.Background(), domain.SearchIncidentTasksRequest{Pagination: domain.Pagination{Limit: 10}})
		assertForbidden(t, err)
	})

	t.Run("AggregateIncidentTasks", func(t *testing.T) {
		svc := NewIncidentTaskService(&stubIncidentTaskRepo{}, external)
		_, err := svc.AggregateIncidentTasks(context.Background(), domain.AggregateIncidentTasksRequest{GroupBy: "state"})
		assertForbidden(t, err)
	})

	t.Run("GetIncidentTask", func(t *testing.T) {
		svc := NewIncidentTaskService(&stubIncidentTaskRepo{}, external)
		_, err := svc.GetIncidentTask(context.Background(), "11111111-1111-1111-1111-111111111111")
		assertForbidden(t, err)
	})

	t.Run("an internal caller is not blocked by the gate itself", func(t *testing.T) {
		svc := NewIncidentTaskService(&stubIncidentTaskRepo{}, stubAccess{scope: AccessScope{Unrestricted: true}})
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected the unconfigured stub repo to be reached (and panic) for an internal caller")
			}
		}()
		_, _ = svc.GetIncidentTask(context.Background(), "11111111-1111-1111-1111-111111111111")
	})
}
