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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// These tests pin the one exemption UpdateCase makes for the auto-closure
// workflow: closing a ticket that sits in Awaiting Info (no staff table lists
// that move) when, and only when, the close is made by the workflow itself.

var autoclosureMachineAccess = stubAccess{scope: AccessScope{Unrestricted: true}}

func closeRequest() domain.UpdateCaseRequest {
	closed := domain.CaseStateClosed
	return domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &closed}
}

func TestUpdateCase_AwaitingInfoToClosedIsRefusedForOrdinaryCallers(t *testing.T) {
	f := newCustomerReplyFixture("case", domain.CaseStateAwaitingInfo)
	svc := NewCaseService(f.repo, f.userRepo, nil, autoclosureMachineAccess, nil)

	_, err := svc.UpdateCase(context.Background(), closeRequest())
	if err == nil || !strings.Contains(err.Error(), "state cannot move") {
		t.Fatalf("err = %v, want the illegal-transition refusal", err)
	}
	if moves := f.movesTo(); len(moves) != 0 {
		t.Errorf("a refused move still reached the repository: %v", moves)
	}
}

func TestUpdateCase_AutoclosureMayCloseAwaitingInfoAsTheSystemUser(t *testing.T) {
	for _, from := range []domain.CaseState{domain.CaseStateAwaitingInfo, domain.CaseStateSolutionProposed} {
		t.Run(string(from), func(t *testing.T) {
			f := newCustomerReplyFixture("case", from)
			svc := NewCaseService(f.repo, f.userRepo, nil, autoclosureMachineAccess, nil)
			ctx := withAutoclosureActor(context.Background(), "system@example.com")

			if _, err := svc.UpdateCase(ctx, closeRequest()); err != nil {
				t.Fatalf("UpdateCase: %v", err)
			}
			if moves := f.movesTo(); len(moves) != 1 || moves[0] != domain.CaseStateClosed {
				t.Fatalf("moves = %v, want one move to closed", moves)
			}
			acts := f.activities()
			if len(acts) != 1 || acts[0].actor != "system@example.com" || acts[0].newValue != "Closed" {
				t.Errorf("activity = %+v, want the close attributed to the system user", acts)
			}
		})
	}
}

func TestUpdateCase_AutoclosureExemptionIsNarrow(t *testing.T) {
	ctx := withAutoclosureActor(context.Background(), "system@example.com")
	wip := domain.CaseStateWorkInProgress
	for _, tc := range []struct {
		name string
		from domain.CaseState
		req  domain.UpdateCaseRequest
	}{
		// Only a close is exempt, and only out of the two waiting states.
		{"open to closed", domain.CaseStateOpen, closeRequest()},
		{"awaiting info to work in progress", domain.CaseStateAwaitingInfo, domain.UpdateCaseRequest{ID: testDeploymentUUID, State: &wip}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCustomerReplyFixture("case", tc.from)
			svc := NewCaseService(f.repo, f.userRepo, nil, autoclosureMachineAccess, nil)
			_, err := svc.UpdateCase(ctx, tc.req)
			if err == nil || !strings.Contains(err.Error(), "state cannot move") {
				t.Fatalf("err = %v, want the illegal-transition refusal", err)
			}
		})
	}
}
