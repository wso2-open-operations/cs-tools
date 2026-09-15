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

package flows

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
)

// planDateEvent builds an entity.changed for a change_request where one column
// moved. Snapshot keys are snake_case, as the outbox trigger writes them.
func planDateEvent(t *testing.T, column, from, to, state string) Event {
	t.Helper()
	payload := events.EntityChangedPayload{
		EntityType: "change_request",
		EntityID:   "cr-1",
		Changes:    map[string]map[string]any{column: {"from": from, "to": to}},
		Snapshot:   map[string]any{"state": state},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return Event{Envelope: events.Envelope{
		Type: events.TypeEntityChanged, EntityID: "cr-1", Payload: raw,
	}}
}

func planDateDeps(rec *fakeRecipients, crs *fakeCRs, prod *fakeProducer) Deps {
	return Deps{Recipients: rec, ChangeRequests: crs, Producer: prod}
}

func runPlanDate(t *testing.T, evt Event, deps Deps, prod *fakeProducer) *events.CRPlanDateNoticePayload {
	t.Helper()
	if err := (crPlanDateNotice{}).Run(context.Background(), evt, deps); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(prod.published) == 0 {
		return nil
	}
	var env events.Envelope
	if err := json.Unmarshal(prod.published[0], &env); err != nil {
		t.Fatalf("not an envelope: %v", err)
	}
	if env.Type != events.TypeCRPlanDateNotice {
		t.Fatalf("type = %q", env.Type)
	}
	var n events.CRPlanDateNoticePayload
	if err := json.Unmarshal(env.Payload, &n); err != nil {
		t.Fatalf("not a plan-date notice: %v", err)
	}
	return &n
}

// TestCRPlanDateNotice_Match pins both triggers, and the cases that look like
// them but are not.
func TestCRPlanDateNotice_Match(t *testing.T) {
	f := crPlanDateNotice{}
	tests := []struct {
		name   string
		evt    Event
		expect bool
	}{
		{"customer moves the date while awaiting customer approval",
			planDateEvent(t, "customer_updated_on", "", "2026-10-01T00:00:00Z", "CUSTOMER_APPROVAL"), true},
		{"same move in any other state is not the original's trigger",
			planDateEvent(t, "customer_updated_on", "", "2026-10-01T00:00:00Z", "IMPLEMENT"), false},
		{"WSO2 agrees", planDateEvent(t, "customer_updated_date_confirmation", "", "AGREE", "CUSTOMER_APPROVAL"), true},
		{"WSO2 disagrees", planDateEvent(t, "customer_updated_date_confirmation", "AGREE", "DISAGREE", "CUSTOMER_APPROVAL"), true},
		{"the confirmation being CLEARED is not an answer",
			planDateEvent(t, "customer_updated_date_confirmation", "AGREE", "", "CUSTOMER_APPROVAL"), false},
		{"a no-op write that reports the same value both sides",
			planDateEvent(t, "customer_updated_date_confirmation", "AGREE", "AGREE", "CUSTOMER_APPROVAL"), false},
		{"an unrelated column", planDateEvent(t, "git_reference", "a", "b", "CUSTOMER_APPROVAL"), false},
		{"a state change, which is the OTHER flow's business",
			planDateEvent(t, "state", "NEW", "ASSESS", "ASSESS"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f.Match(tt.evt); got != tt.expect {
				t.Errorf("Match = %v, want %v", got, tt.expect)
			}
		})
	}
}

// TestCRPlanDateNotice_ClearingIsSilent is worth its own test: "CR change start
// plan date notifications" blanks the confirmation every time a customer moves
// the date. If that blanking counted as an answer, every proposal would also
// mail the customer that WSO2 had responded.
func TestCRPlanDateNotice_ClearingIsSilent(t *testing.T) {
	prod := &fakeProducer{}
	evt := planDateEvent(t, "customer_updated_date_confirmation", "AGREE", "", "CUSTOMER_APPROVAL")
	if n := runPlanDate(t, evt, planDateDeps(&fakeRecipients{}, &fakeCRs{}, prod), prod); n != nil {
		t.Fatalf("published %+v, want nothing", n)
	}
}

// TestCRPlanDateNotice_CustomerProposed covers the internal turn, including the
// actor gate the original expresses as sys_updated_byNOT LIKE@wso2.com.
func TestCRPlanDateNotice_CustomerProposed(t *testing.T) {
	base := ChangeRequestDetails{
		Number: "CHG0031234", ProjectID: "proj-9", ProjectName: "Acme Cloud",
		ShortDescription: "Upgrade the gateway", Description: "Full details here",
		ActorName: "Perera Nimal",
	}

	t.Run("a customer edit notifies Devops Approval", func(t *testing.T) {
		rec := &fakeRecipients{group: map[string][]string{"Devops Approval": {"devops@wso2.com"}}}
		prod := &fakeProducer{}
		evt := planDateEvent(t, "customer_updated_on", "", "2026-10-01T00:00:00Z", "CUSTOMER_APPROVAL")

		n := runPlanDate(t, evt, planDateDeps(rec, &fakeCRs{details: base}, prod), prod)
		if n == nil {
			t.Fatal("published nothing")
		}
		if rec.askedFor != "group:Devops Approval" {
			t.Errorf("resolved %q", rec.askedFor)
		}
		if want := "[WSO2 Support] [CR] (CHG0031234) Customer has updated the plan start date"; n.Subject != want {
			t.Errorf("subject = %q, want %q", n.Subject, want)
		}
		if n.Kind != events.CRPlanDateCustomerProposed || n.Audience != events.CRAudienceInternal {
			t.Errorf("kind=%q audience=%q", n.Kind, n.Audience)
		}
		if n.ActorName != "Perera Nimal" {
			t.Errorf("actorName = %q, want last name first", n.ActorName)
		}
	})

	t.Run("a WSO2 edit notifies nobody", func(t *testing.T) {
		wso2 := base
		wso2.ActorIsWSO2 = true
		rec := &fakeRecipients{group: map[string][]string{"Devops Approval": {"devops@wso2.com"}}}
		prod := &fakeProducer{}
		evt := planDateEvent(t, "customer_updated_on", "", "2026-10-01T00:00:00Z", "CUSTOMER_APPROVAL")

		if n := runPlanDate(t, evt, planDateDeps(rec, &fakeCRs{details: wso2}, prod), prod); n != nil {
			t.Fatalf("published %+v — the WSO2 team editing its own plan is not news to the team", n)
		}
	})
}

// TestCRPlanDateNotice_WSO2Answers covers both customer-facing turns and pins
// the two subject lines verbatim, including the original's own grammar.
func TestCRPlanDateNotice_WSO2Answers(t *testing.T) {
	details := ChangeRequestDetails{
		Number: "CHG0031234", ProjectID: "proj-9", ProjectName: "Acme Cloud",
		ActorName: "Silva Kasun", ActorIsWSO2: true,
	}
	tests := []struct {
		name, to, wantSubject string
		wantKind              events.CRPlanDateKind
	}{
		{"accepted", "AGREE",
			"[WSO2 Support] [CR] (CHG0031234) Accepted the plan start date",
			events.CRPlanDateAccepted},
		{"rejected", "DISAGREE",
			"[WSO2 Support] [CR] (CHG0031234) Reject the proposed plan start date",
			events.CRPlanDateRejected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &fakeRecipients{contacts: map[string][]string{"proj-9": {"contact@acme.example"}}}
			prod := &fakeProducer{}
			evt := planDateEvent(t, "customer_updated_date_confirmation", "", tt.to, "CUSTOMER_APPROVAL")

			n := runPlanDate(t, evt, planDateDeps(rec, &fakeCRs{details: details}, prod), prod)
			if n == nil {
				t.Fatal("published nothing")
			}
			if rec.askedFor != "project:proj-9" {
				t.Errorf("resolved %q, want the project's contacts", rec.askedFor)
			}
			if n.Subject != tt.wantSubject {
				t.Errorf("subject = %q, want %q", n.Subject, tt.wantSubject)
			}
			if n.Kind != tt.wantKind || n.Audience != events.CRAudienceCustomer {
				t.Errorf("kind=%q audience=%q", n.Kind, n.Audience)
			}
			if n.GroupName != "" {
				t.Errorf("groupName = %q, want empty on a customer notice", n.GroupName)
			}
			// The WSO2 actor gate applies only to the customer-proposed turn --
			// an answer is BY WSO2 by definition, so it must not be filtered.
			if len(n.Recipients) == 0 {
				t.Error("no recipients: the actor gate must not apply to WSO2's own answer")
			}
		})
	}
}

// TestCRPlanDateNotice_NotRegistered: same double-fire guard as the approval
// flow. All five ServiceNow flows behind this are still active.
func TestCRPlanDateNotice_NotRegistered(t *testing.T) {
	for _, f := range All() {
		if f.Key() == (crPlanDateNotice{}).Key() {
			t.Fatal("cr_plan_date_notice is registered: registering it requires disabling " +
				"'CR change start plan date notifications' and '[CR] WSO2 response for plan " +
				"start date change' in ServiceNow in the same commit")
		}
	}
}
