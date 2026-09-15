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
	"fmt"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
)

// crPlanDateNotice ports the plan-start-date negotiation: the ServiceNow flows
// "CR change start plan date notifications" and "[CR] WSO2 response for plan
// start date change", with the two subflows they call.
//
// It is one conversation with two turns, which is why it is one flow here
// rather than four:
//
//	customer moves the proposed date          -> tell Devops Approval
//	WSO2 answers Agree                        -> tell the project's contacts
//	WSO2 answers Disagree                     -> tell the project's contacts
//
// The two turns key off different columns, so Match distinguishes them by which
// column changed rather than by state.
//
// WHAT IS DELIBERATELY NOT HERE. "CR change start plan date notifications"
// sends nothing despite its name -- its only action clears
// u_confirm_customer_updated_date so a previous answer does not stand against a
// newly proposed date. That is a WRITE to change_request, and this service is
// read-only by design (internal/store's package doc). Same for "[CR] Fields
// Changes Comments on the SR", whose only action writes a comment onto the
// linked task. Both belong wherever change_request writes end up living; a flow
// that publishes notices must not be the thing that quietly also mutates rows.
type crPlanDateNotice struct{}

func (crPlanDateNotice) Key() string { return "cr_plan_date_notice" }

func (crPlanDateNotice) TriggerEntityTypes() []string { return []string{crEntityType} }

// Column names as the outbox trigger writes them -- to_jsonb(NEW) of
// change_request, so the database's own snake_case.
const (
	crColCustomerUpdatedOn = "customer_updated_on"
	crColConfirmation      = "customer_updated_date_confirmation"
	crColState             = "state"
)

// crPlanDateTurn is which half of the conversation an event represents.
type crPlanDateTurn int

const (
	crTurnNone crPlanDateTurn = iota
	// crTurnCustomerProposed: the customer moved the date. Internal audience.
	crTurnCustomerProposed
	// crTurnWSO2Accepted / crTurnWSO2Rejected: WSO2 answered. Customer audience.
	crTurnWSO2Accepted
	crTurnWSO2Rejected
)

// crStateCustomerApproval is ServiceNow state 5, the only state in which the
// original's trigger accepts a customer date change.
const crStateCustomerApproval = "CUSTOMER_APPROVAL"

// Match decides which turn this is, from the changed columns alone.
//
// The two ServiceNow triggers are:
//
//	u_customer_updatedVALCHANGES^state=5^sys_updated_byNOT LIKE@wso2.com
//	u_confirm_customer_updated_dateVALCHANGES
//
// Two of those three clauses are expressible here; the third is not. Match must
// stay pure, and "who changed it" lives on work_item, not in the outbox
// snapshot -- so the actor gate is applied in Run, once the record has been
// read. A customer-proposed turn therefore MATCHES on a WSO2 edit and then
// publishes nothing, which is the correct outcome by a slightly longer route.
func (f crPlanDateNotice) Match(evt Event) bool {
	_, ok := f.turn(evt)
	return ok
}

func (crPlanDateNotice) turn(evt Event) (crPlanDateTurn, bool) {
	if evt.Envelope.Type != events.TypeEntityChanged {
		return crTurnNone, false
	}
	var payload events.EntityChangedPayload
	if err := json.Unmarshal(evt.Envelope.Payload, &payload); err != nil {
		return crTurnNone, false
	}
	if payload.EntityType != crEntityType {
		return crTurnNone, false
	}

	// WSO2's answer takes precedence: if both columns moved in one write, the
	// answer is the newer fact and the proposal it answers is implied.
	if to, changed := crChangedTo(payload, crColConfirmation); changed {
		switch to {
		case "AGREE":
			return crTurnWSO2Accepted, true
		case "DISAGREE":
			return crTurnWSO2Rejected, true
		}
		// Cleared, or some third value: not an answer, so not a notice. This is
		// the path "CR change start plan date notifications" creates when it
		// blanks the field, and it must stay silent or every proposal would
		// also mail the customer.
		return crTurnNone, false
	}

	if _, changed := crChangedTo(payload, crColCustomerUpdatedOn); changed {
		// state=5 in the original. Read from the snapshot rather than the diff:
		// the state need not have changed, it needs to BE CUSTOMER_APPROVAL.
		if crSnapshotString(payload.Snapshot, crColState) != crStateCustomerApproval {
			return crTurnNone, false
		}
		return crTurnCustomerProposed, true
	}
	return crTurnNone, false
}

// crChangedTo reports a column's new value if that column changed at all.
// Unlike crChangedToApprovalState it does not require the value to be a
// non-empty string -- a date being set for the first time is a change, and so
// is one being cleared.
func crChangedTo(payload events.EntityChangedPayload, column string) (string, bool) {
	change, ok := payload.Changes[column]
	if !ok {
		return "", false
	}
	to, _ := change["to"].(string)
	if from, ok := change["from"].(string); ok && from == to {
		return "", false
	}
	return to, true
}

func crSnapshotString(snap map[string]any, key string) string {
	if snap == nil {
		return ""
	}
	if v, ok := snap[key].(string); ok {
		return v
	}
	return ""
}

// Run resolves the turn's audience and publishes one notice.
func (f crPlanDateNotice) Run(ctx context.Context, evt Event, deps Deps) error {
	turn, ok := f.turn(evt)
	if !ok {
		return nil
	}
	var payload events.EntityChangedPayload
	if err := json.Unmarshal(evt.Envelope.Payload, &payload); err != nil {
		return fmt.Errorf("cr_plan_date_notice: decode entity.changed: %w", err)
	}
	if deps.ChangeRequests == nil {
		return fmt.Errorf("cr_plan_date_notice: change request reader is not configured")
	}
	details, err := deps.ChangeRequests.ChangeRequestDetails(ctx, payload.EntityID)
	if err != nil {
		return fmt.Errorf("cr_plan_date_notice: read change request %s: %w", payload.EntityID, err)
	}
	if details.Number == "" {
		// Deleted between the outbox row and now. Nothing to say about it.
		return nil
	}

	notice := events.CRPlanDateNoticePayload{
		ChangeRequestID:  payload.EntityID,
		Number:           details.Number,
		ProjectID:        details.ProjectID,
		ProjectName:      details.ProjectName,
		ShortDescription: details.ShortDescription,
		Description:      details.Description,
		ActorName:        details.ActorName,
	}

	var recipients []string
	switch turn {
	case crTurnCustomerProposed:
		// sys_updated_byNOT LIKE@wso2.com. Applied here rather than in Match
		// because the actor is not in the outbox snapshot -- see Match's note.
		// A WSO2 user moving the date is the WSO2 team editing its own plan,
		// which is not news to the team.
		if details.ActorIsWSO2 {
			return nil
		}
		notice.Kind = events.CRPlanDateCustomerProposed
		notice.Audience = events.CRAudienceInternal
		notice.GroupName = crPlanDateInternalGroup
		notice.Subject = crPlanDateSubject(details.Number, "Customer has updated the plan start date")
		recipients, err = deps.Recipients.GroupMemberEmails(ctx, crPlanDateInternalGroup)

	case crTurnWSO2Accepted:
		notice.Kind = events.CRPlanDateAccepted
		notice.Audience = events.CRAudienceCustomer
		notice.Subject = crPlanDateSubject(details.Number, "Accepted the plan start date")
		recipients, err = deps.Recipients.ProjectContactEmails(ctx, details.ProjectID)

	case crTurnWSO2Rejected:
		notice.Kind = events.CRPlanDateRejected
		notice.Audience = events.CRAudienceCustomer
		notice.Subject = crPlanDateSubject(details.Number, "Reject the proposed plan start date")
		recipients, err = deps.Recipients.ProjectContactEmails(ctx, details.ProjectID)
	}
	if err != nil {
		return fmt.Errorf("cr_plan_date_notice: resolve %s recipients: %w", notice.Audience, err)
	}

	recipients = normaliseAddresses(recipients)
	if len(recipients) == 0 {
		// Same reasoning as cr_approval_notice: an empty group or a project
		// with no contacts is a real state, and checked BEFORE the debug swap
		// so a test deployment stays as silent as the real one would be.
		return nil
	}
	notice.Recipients = applyDebugRecipients(recipients, deps.EmailDebugRecipients)

	body, err := json.Marshal(events.Envelope{
		Type:     events.TypeCRPlanDateNotice,
		EntityID: payload.EntityID,
		Payload:  mustMarshal(notice),
	})
	if err != nil {
		return fmt.Errorf("cr_plan_date_notice: encode notice: %w", err)
	}
	if err := deps.Producer.Publish(ctx, []byte(payload.EntityID), body); err != nil {
		return fmt.Errorf("cr_plan_date_notice: publish notice: %w", err)
	}
	return nil
}

// crPlanDateInternalGroup is the approval group the original's internal subflow
// hardcoded by sys_id (509dc4c51b08d250a002c9d3604bcbfa). By name here, for the
// same reason as cr_approval_notice: a sys_id means nothing after the migration.
const crPlanDateInternalGroup = "Devops Approval"

// crPlanDateSubject renders "[WSO2 Support] [CR] (<number>) <suffix>". The
// plan-date notices never carry the owning team, unlike the internal approval
// ones -- not an oversight, it is what the originals do.
func crPlanDateSubject(number, suffix string) string {
	return fmt.Sprintf("[WSO2 Support] [CR] (%s) %s", number, suffix)
}

var _ Flow = crPlanDateNotice{}
