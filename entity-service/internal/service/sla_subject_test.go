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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// A policy with no stop condition, never cancelled: only the terminal safety
// net can end its clock.
func neverEndingPolicy() repository.SLAPolicyRef {
	return repository.SLAPolicyRef{ID: "p1", Name: "never ending", Target: "RESOLUTION", Source: "CSM", WhenToCancel: "NEVER"}
}

func TestReevaluateClocks_IncidentSafetyNetFiresOffLoadedFactsState(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.activeByCaseTarget["inc-1|RESOLUTION"] = neverEndingPolicy()
	svc := NewSLAEngineService(repo).(*slaEngineService)

	svc.reevaluateClocks(context.Background(), incidentSLASubject{id: "inc-1", facts: repository.IncidentFacts{State: "CLOSED"}})

	if len(repo.completed) != 1 || repo.completed[0] != "inc-1|RESOLUTION" {
		t.Fatalf("completed = %v, want [inc-1|RESOLUTION]: an incident whose loaded state is CLOSED is terminal", repo.completed)
	}
}

func TestReevaluateClocks_CaseSafetyNetNeedsTheTerminalFlag(t *testing.T) {
	// A case that is CLOSED in the database but re-evaluated for a field change
	// (no state override) must not have its clocks force-completed.
	repo := newFakeFullSLAEngineRepo()
	repo.activeByCaseTarget["case-1|RESOLUTION"] = neverEndingPolicy()
	svc := NewSLAEngineService(repo).(*slaEngineService)

	svc.reevaluateClocks(context.Background(), caseSLASubject{id: "case-1", facts: repository.CaseFacts{State: "CLOSED"}, terminal: false})
	if len(repo.completed) != 0 {
		t.Fatalf("completed = %v, want none: case terminal flag is false", repo.completed)
	}

	svc.reevaluateClocks(context.Background(), caseSLASubject{id: "case-1", facts: repository.CaseFacts{State: "CLOSED"}, terminal: true})
	if len(repo.completed) != 1 || repo.completed[0] != "case-1|RESOLUTION" {
		t.Fatalf("completed = %v, want [case-1|RESOLUTION]", repo.completed)
	}
}
