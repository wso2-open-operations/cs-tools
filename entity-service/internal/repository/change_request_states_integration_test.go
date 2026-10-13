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

package repository_test

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The additive state report of the two writes that can move a change request
// (ChangeRequestStates: the state the transaction FOUND under the row lock and
// the state it COMMITTED), which the dual-write mirror compares to send the
// previous system every move PostgreSQL makes. Same harness as
// TestChangeRequestFlowIntegration_* (crFlow, DSN-gated by
// CHANGE_REQUEST_TEST_DSN, run as a superuser and as the non-superuser
// csm_app).

func wantStates(t *testing.T, what string, got repository.ChangeRequestStates, prior, committed string) {
	t.Helper()
	str := func(s *string) string {
		if s == nil {
			return "<NULL>"
		}
		return *s
	}
	if str(got.Prior) != prior || str(got.Committed) != committed {
		t.Fatalf("%s: states = %s -> %s, want %s -> %s", what, str(got.Prior), str(got.Committed), prior, committed)
	}
	if got.Moved() != (prior != committed) {
		t.Fatalf("%s: Moved() = %v for %s -> %s", what, got.Moved(), prior, committed)
	}
}

func TestChangeRequestStatesIntegration_PatchReportsTheStateFoundAndCommitted(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	// Request Approval on a Normal change checks that somebody can give the
	// CAB approval later.
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)

	// A field edit: the state is read, under the lock, and did not move.
	cr, st, err := f.repo.PatchChangeRequestStates(f.sys, id, domain.PatchChangeRequestRequest{Title: sp("renamed")}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		t.Fatalf("title: %v", err)
	}
	if cr.Subject == nil || *cr.Subject != "renamed" {
		t.Fatalf("subject = %v, the detail returned is still the committed one", cr.Subject)
	}
	wantStates(t, "a title", st, "NEW", "NEW")

	// Request Approval on a Normal change: New -> Assess.
	assess := domain.ChangeRequestStateAssess
	_, st, err = f.repo.PatchChangeRequestStates(f.sys, id, domain.PatchChangeRequestRequest{State: &assess}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		t.Fatalf("Request Approval: %v", err)
	}
	wantStates(t, "Request Approval", st, "NEW", "ASSESS")

	// A resend of the state the change is in: found and committed agree.
	_, st, err = f.repo.PatchChangeRequestStates(f.sys, id, domain.PatchChangeRequestRequest{State: &assess}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		t.Fatalf("resend: %v", err)
	}
	wantStates(t, "a resend of assess", st, "ASSESS", "ASSESS")

	// A refused request reports nothing (and writes nothing).
	implement := domain.ChangeRequestStateImplement
	if _, st, err = f.repo.PatchChangeRequestStates(f.sys, id, domain.PatchChangeRequestRequest{State: &implement}, crFlowEmail(crFlowCreatorID)); err == nil {
		t.Fatal("Assess -> Implement was accepted")
	}
	wantStates(t, "a refused jump", st, "<NULL>", "<NULL>")
	f.expect(id, "after the refusal", "ASSESS", "canceled")

	// Cancel: Assess -> Canceled; and the plain PatchChangeRequest still works
	// (it is the same transaction without the report).
	canceled := domain.ChangeRequestStateCanceled
	if _, err := f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{State: &canceled}, crFlowEmail(crFlowCreatorID)); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	f.expect(id, "after Cancel", "CANCELED")
}

func TestChangeRequestStatesIntegration_RequestApprovalReportsTheStateTheTypeChose(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	// A Standard change has no internal approval to wait for: Request
	// Approval ({state: "assess"}) writes Scheduled, and that is what is
	// reported -- the committed state, not the requested one.
	id := f.create(domain.ChangeRequestTypeStandard, crFlowGroupID)
	assess := domain.ChangeRequestStateAssess
	_, st, err := f.repo.PatchChangeRequestStates(f.sys, id, domain.PatchChangeRequestRequest{State: &assess}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		t.Fatalf("Request Approval (Standard): %v", err)
	}
	wantStates(t, "Request Approval on a Standard change", st, "NEW", "SCHEDULED")
}

func TestChangeRequestStatesIntegration_DecisionReportsTheCascade(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.requestApproval(id)

	// A peer's rejection resolves the stage without moving the change.
	approvalID, st, err := f.repo.DecideChangeRequestApprovalStates(f.sys, id, crFlowPeerAID, "rejected", crFlowEmail(crFlowPeerAID))
	if err != nil {
		t.Fatalf("peer rejection: %v", err)
	}
	if approvalID == "" {
		t.Fatal("no approval id returned")
	}
	wantStates(t, "a peer rejection", st, "ASSESS", "ASSESS")

	// Fresh change: the peer approval cascades Assess -> Authorize, the CAB
	// approval Authorize -> Scheduled.
	id = f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.requestApproval(id)
	_, st, err = f.repo.DecideChangeRequestApprovalStates(f.sys, id, crFlowPeerAID, "approved", crFlowEmail(crFlowPeerAID))
	if err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	wantStates(t, "the peer approval", st, "ASSESS", "AUTHORIZE")
	_, st, err = f.repo.DecideChangeRequestApprovalStates(f.sys, id, crCABMemberUserID1, "approved", crFlowEmail(crCABMemberUserID1))
	if err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	wantStates(t, "the CAB approval", st, "AUTHORIZE", "SCHEDULED")
	f.expect(id, "after CAB approval", "SCHEDULED", "implement", "canceled")

	// A decision the caller has no pending row for reports nothing.
	_, st, err = f.repo.DecideChangeRequestApprovalStates(f.sys, id, crFlowOutsiderID, "approved", crFlowEmail(crFlowOutsiderID))
	if err == nil {
		t.Fatal("a decision with no pending approval was accepted")
	}
	wantStates(t, "a refused decision", st, "<NULL>", "<NULL>")
}

func TestChangeRequestStatesIntegration_CustomerAnswerReportsTheMove(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)

	// The customer's own approval through the PATCH: Customer Approval ->
	// Scheduled, reported by the same report a staff PATCH gives.
	_, st, err := f.repo.PatchChangeRequestStates(asContact(crScopeUserA1), id, domain.PatchChangeRequestRequest{IsCustomerApproved: boolp(true)}, crFlowEmail(crScopeUserA1))
	if err != nil {
		t.Fatalf("the customer's approval: %v", err)
	}
	wantStates(t, "the customer's approval", st, "CUSTOMER_APPROVAL", "SCHEDULED")
}

func TestChangeRequestStatesIntegration_MigratedRowWithANullState(t *testing.T) {
	// A row migrated from the previous system can carry no state at all. The
	// report says so (nil), a field edit keeps it, and Request Approval moves
	// it (NULL is New for the PATCH).
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.setState(id, "")

	_, st, err := f.repo.PatchChangeRequestStates(f.sys, id, domain.PatchChangeRequestRequest{Title: sp("renamed")}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		t.Fatalf("title on a NULL state: %v", err)
	}
	wantStates(t, "a title on a NULL state", st, "<NULL>", "<NULL>")

	assess := domain.ChangeRequestStateAssess
	_, st, err = f.repo.PatchChangeRequestStates(f.sys, id, domain.PatchChangeRequestRequest{State: &assess}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		t.Fatalf("Request Approval on a NULL state: %v", err)
	}
	wantStates(t, "Request Approval on a NULL state", st, "<NULL>", "ASSESS")
}
