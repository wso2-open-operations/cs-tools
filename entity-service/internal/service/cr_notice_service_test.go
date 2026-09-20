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
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const crTestID = "11111111-2222-3333-4444-555555555555"

type fakeCRRepo struct {
	details  repository.CRNoticeDetails
	group    []string
	contacts []string
	groupFor string
}

func (f *fakeCRRepo) ClaimChanges(context.Context, []string, int) ([]repository.OutboxChange, error) {
	return nil, nil
}
func (f *fakeCRRepo) Details(context.Context, string) (repository.CRNoticeDetails, error) {
	return f.details, nil
}
func (f *fakeCRRepo) GroupMemberEmails(_ context.Context, team string) ([]string, error) {
	f.groupFor = team
	return f.group, nil
}
func (f *fakeCRRepo) ProjectContactEmails(context.Context, string) ([]string, error) {
	return f.contacts, nil
}

type published struct {
	Type     events.Type
	EntityID string
	Payload  json.RawMessage
}

type fakePublisher struct{ sent []published }

func (f *fakePublisher) Publish(_ context.Context, t events.Type, id string, p json.RawMessage) error {
	f.sent = append(f.sent, published{t, id, p})
	return nil
}

func crChange(changes map[string]map[string]any, snapshot map[string]any) repository.OutboxChange {
	return repository.OutboxChange{
		ID: 1, EntityType: CREntityType, EntityID: crTestID,
		Changes: changes, Snapshot: snapshot,
	}
}

func stateChange(from, to string) repository.OutboxChange {
	return crChange(
		map[string]map[string]any{crColState: {"from": from, "to": to}},
		map[string]any{crColState: to},
	)
}

func newCRService(repo *fakeCRRepo, pub *fakePublisher) CRNoticeService {
	return NewCRNoticeService(repo, pub)
}

func baseDetails() repository.CRNoticeDetails {
	return repository.CRNoticeDetails{
		Number: "CHG0031234", ProjectID: "proj-1", ProjectName: "Acme",
		RequesterName: "Ada Lovelace",
	}
}

// The five approval states, their audience, group and subject — the table the
// ServiceNow flow branched on.
func TestApprovalNotice_StateTable(t *testing.T) {
	cases := []struct {
		state    string
		audience events.CRApprovalAudience
		group    string
		subject  string
	}{
		{"ASSESS", events.CRAudienceInternal, "Devops Approval", "[WSO2 Support] [CR][MS] (CHG0031234) Request for Approval - Implementation"},
		{"AUTHORIZE", events.CRAudienceInternal, "CAB Approval", "[WSO2 Support] [CR][MS] (CHG0031234) Request for CAB approval - Authorize"},
		{"REVIEW", events.CRAudienceInternal, "Devops Review", "[WSO2 Support] [CR][MS] (CHG0031234) Request for approval - Review"},
		{"CUSTOMER_APPROVAL", events.CRAudienceCustomer, "", "[WSO2 Support] [CR] (CHG0031234) Request for Approval - Implementation"},
		{"CUSTOMER_REVIEW", events.CRAudienceCustomer, "", "[WSO2 Support] [CR] (CHG0031234) Request for approval - Customer Review"},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			repo := &fakeCRRepo{details: baseDetails(), group: []string{"a@wso2.com"}, contacts: []string{"c@acme.com"}}
			pub := &fakePublisher{}
			if err := newCRService(repo, pub).HandleChange(context.Background(), stateChange("NEW", tc.state)); err != nil {
				t.Fatalf("HandleChange: %v", err)
			}
			if len(pub.sent) != 1 {
				t.Fatalf("want 1 notice, got %d", len(pub.sent))
			}
			var got events.CRApprovalRequestedPayload
			if err := json.Unmarshal(pub.sent[0].Payload, &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.Audience != tc.audience {
				t.Errorf("audience = %q, want %q", got.Audience, tc.audience)
			}
			if got.GroupName != tc.group {
				t.Errorf("group = %q, want %q", got.GroupName, tc.group)
			}
			if got.Subject != tc.subject {
				t.Errorf("subject = %q, want %q", got.Subject, tc.subject)
			}
		})
	}
}

// "Changed TO", not "is": a record touched while already sitting in an
// approval state must not re-notify.
func TestApprovalNotice_OnlyOnTransition(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change repository.OutboxChange
	}{
		{"no state in the diff", crChange(map[string]map[string]any{"description": {"from": "a", "to": "b"}}, nil)},
		{"same value both sides", stateChange("ASSESS", "ASSESS")},
		{"uncovered state", stateChange("ASSESS", "SCHEDULED")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeCRRepo{details: baseDetails(), group: []string{"a@wso2.com"}}
			pub := &fakePublisher{}
			if err := newCRService(repo, pub).HandleChange(context.Background(), tc.change); err != nil {
				t.Fatalf("HandleChange: %v", err)
			}
			if len(pub.sent) != 0 {
				t.Fatalf("published %d notices, want none", len(pub.sent))
			}
		})
	}
}

func TestTeamFromGitReference(t *testing.T) {
	for ref, want := range map[string]string{
		"https://github.com/wso2/choreo-apis/pull/1": "Choreo",
		"git@github.com:wso2/ASGARDEO-x.git":         "Asgardeo",
		"https://github.com/wso2/something":          "MS",
		"":                                           "MS",
	} {
		if got := crTeamFromGitReference(ref); got != want {
			t.Errorf("crTeamFromGitReference(%q) = %q, want %q", ref, got, want)
		}
	}
}

// Nobody to tell is a real state, not an error, and must publish nothing.
func TestApprovalNotice_NoRecipientsPublishesNothing(t *testing.T) {
	repo := &fakeCRRepo{details: baseDetails()} // empty group and contacts
	pub := &fakePublisher{}
	if err := newCRService(repo, pub).HandleChange(context.Background(), stateChange("NEW", "ASSESS")); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	if len(pub.sent) != 0 {
		t.Fatalf("published %d notices, want none", len(pub.sent))
	}
}

// A record deleted between the outbox row and the read is a silent no-op.
func TestApprovalNotice_DeletedRecord(t *testing.T) {
	repo := &fakeCRRepo{group: []string{"a@wso2.com"}} // zero details => Number ""
	pub := &fakePublisher{}
	if err := newCRService(repo, pub).HandleChange(context.Background(), stateChange("NEW", "ASSESS")); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	if len(pub.sent) != 0 {
		t.Fatalf("published %d notices, want none", len(pub.sent))
	}
}

func TestNormaliseAddresses(t *testing.T) {
	got := normaliseAddresses([]string{" B@WSO2.com ", "a@wso2.com", "", "b@wso2.com"})
	want := []string{"a@wso2.com", "b@wso2.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// THE PRECEDENCE RULE. The write that moves the date also clears the standing
// confirmation, so one outbox row carries both columns. Reading the
// confirmation first would see it cleared and report "no answer", silently
// swallowing every proposal that arrives while an answer is standing.
func TestPlanDate_DateChangeWinsOverClearedConfirmation(t *testing.T) {
	change := crChange(
		map[string]map[string]any{
			crColCustomerDate: {"from": nil, "to": "2026-10-01"},
			crColConfirmation: {"from": "AGREE", "to": ""},
		},
		map[string]any{crColState: crStateCustomerApproval},
	)
	if turn := crPlanDateTurnOf(change); turn != crTurnCustomerProposed {
		t.Fatalf("turn = %v, want crTurnCustomerProposed", turn)
	}
}

func TestPlanDate_Turns(t *testing.T) {
	cases := []struct {
		name   string
		change repository.OutboxChange
		want   crPlanDateTurn
	}{
		{"agree", crChange(map[string]map[string]any{crColConfirmation: {"from": "", "to": "AGREE"}}, nil), crTurnWSO2Accepted},
		{"disagree", crChange(map[string]map[string]any{crColConfirmation: {"from": "", "to": "DISAGREE"}}, nil), crTurnWSO2Rejected},
		{"unknown answer", crChange(map[string]map[string]any{crColConfirmation: {"from": "", "to": "MAYBE"}}, nil), crTurnNone},
		{
			"date moved outside CUSTOMER_APPROVAL",
			crChange(map[string]map[string]any{crColCustomerDate: {"from": nil, "to": "2026-10-01"}}, map[string]any{crColState: "SCHEDULED"}),
			crTurnNone,
		},
		{"unrelated column", crChange(map[string]map[string]any{"description": {"from": "a", "to": "b"}}, nil), crTurnNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := crPlanDateTurnOf(tc.change); got != tc.want {
				t.Errorf("turn = %v, want %v", got, tc.want)
			}
		})
	}
}

// sys_updated_byNOT LIKE@wso2.com — a WSO2 user moving the date is the team
// editing its own plan, which is not news to the team.
func TestPlanDate_WSO2ActorSuppressesCustomerProposed(t *testing.T) {
	d := baseDetails()
	d.ActorIsWSO2 = true
	repo := &fakeCRRepo{details: d, group: []string{"a@wso2.com"}}
	pub := &fakePublisher{}
	change := crChange(
		map[string]map[string]any{crColCustomerDate: {"from": nil, "to": "2026-10-01"}},
		map[string]any{crColState: crStateCustomerApproval},
	)
	if err := newCRService(repo, pub).HandleChange(context.Background(), change); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	if len(pub.sent) != 0 {
		t.Fatalf("published %d notices, want none", len(pub.sent))
	}
}

func TestPlanDate_CustomerProposedNotifiesInternalGroup(t *testing.T) {
	repo := &fakeCRRepo{details: baseDetails(), group: []string{"dev@wso2.com"}}
	pub := &fakePublisher{}
	change := crChange(
		map[string]map[string]any{crColCustomerDate: {"from": nil, "to": "2026-10-01"}},
		map[string]any{crColState: crStateCustomerApproval},
	)
	if err := newCRService(repo, pub).HandleChange(context.Background(), change); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	if len(pub.sent) != 1 || pub.sent[0].Type != events.TypeCRPlanDateNotice {
		t.Fatalf("want 1 plan-date notice, got %#v", pub.sent)
	}
	if repo.groupFor != crPlanDateInternalGroup {
		t.Errorf("resolved group %q, want %q", repo.groupFor, crPlanDateInternalGroup)
	}
	var got events.CRPlanDateNoticePayload
	if err := json.Unmarshal(pub.sent[0].Payload, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Kind != events.CRPlanDateCustomerProposed {
		t.Errorf("kind = %q, want %q", got.Kind, events.CRPlanDateCustomerProposed)
	}
	if got.Subject != "[WSO2 Support] [CR] (CHG0031234) Customer has updated the plan start date" {
		t.Errorf("subject = %q", got.Subject)
	}
}

// A row change that is neither a state transition nor a date turn is ignored
// entirely — most change_request updates are exactly that.
func TestHandleChange_IgnoresUnrelatedEntityType(t *testing.T) {
	repo := &fakeCRRepo{details: baseDetails(), group: []string{"a@wso2.com"}}
	pub := &fakePublisher{}
	c := stateChange("NEW", "ASSESS")
	c.EntityType = "work_item"
	if err := newCRService(repo, pub).HandleChange(context.Background(), c); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	if len(pub.sent) != 0 {
		t.Fatalf("published %d notices, want none", len(pub.sent))
	}
}

// failingPublisher fails the first publish and succeeds afterwards, which is
// what a combined state-and-date update meets when the approval branch errors.
type failingPublisher struct {
	calls int
	sent  []published
}

func (f *failingPublisher) Publish(_ context.Context, t events.Type, id string, p json.RawMessage) error {
	f.calls++
	if f.calls == 1 {
		return errStub
	}
	f.sent = append(f.sent, published{t, id, p})
	return nil
}

var errStub = errors.New("publish failed")

// One write can move the state AND the date -- the migration that shifts the
// plan start date does exactly that. A failure in the approval branch must not
// cost the plan-date notice: the drainer has already claimed the row, so a
// dropped branch is dropped for good.
func TestHandleChange_SecondBranchRunsAfterFirstFails(t *testing.T) {
	repo := &fakeCRRepo{details: baseDetails(), group: []string{"dev@wso2.com"}, contacts: []string{"c@acme.com"}}
	pub := &failingPublisher{}

	change := crChange(
		map[string]map[string]any{
			crColState:        {"from": "NEW", "to": crStateCustomerApproval},
			crColCustomerDate: {"from": nil, "to": "2026-12-01"},
		},
		map[string]any{crColState: crStateCustomerApproval},
	)

	err := NewCRNoticeService(repo, pub).HandleChange(context.Background(), change)
	if err == nil {
		t.Fatal("want the approval branch's error to surface, got nil")
	}
	if pub.calls != 2 {
		t.Fatalf("publisher called %d times, want 2 (both branches attempted)", pub.calls)
	}
	if len(pub.sent) != 1 || pub.sent[0].Type != events.TypeCRPlanDateNotice {
		t.Fatalf("plan-date notice was lost; sent = %#v", pub.sent)
	}
}
