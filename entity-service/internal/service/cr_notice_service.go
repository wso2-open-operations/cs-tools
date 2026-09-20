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
	"fmt"
	"sort"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// CRNoticeService decides who should be told about a change request, and asks
// csm-notification-service to tell them.
//
// It is the port of two ServiceNow flows -- "CR Approval notifications" with
// its two subflows, and the plan-start-date conversation -- which lived in
// csm-flow-service until the notices were folded into the service that already
// owns these rows.
//
// IT DOES NOT SEND MAIL. It publishes a request carrying an already-resolved
// recipient list; csm-notification-service renders and delivers it. The split
// is deliberate: resolving an audience is a database question about teams,
// project contacts and who last touched the record, and this is the service
// with those tables. Rendering and delivery is a template-and-SMTP question,
// and that is the other one.
type CRNoticeService interface {
	// HandleChange turns one outbox row into zero or more notice requests.
	// Zero is the common case: most change_request updates are not approval
	// transitions or date negotiations.
	HandleChange(ctx context.Context, change repository.OutboxChange) error
}

// crNoticePublisher is the slice of EventPublisherService this policy needs.
// Narrower than the full interface on purpose: publishing a notice must not
// give it the ability to close the producer out from under the rest of the
// service, and a one-method dependency is a one-method fake in tests.
type crNoticePublisher interface {
	Publish(ctx context.Context, eventType events.Type, entityID string, payload json.RawMessage) error
}

type crNoticeService struct {
	repo      repository.CRNoticeRepository
	publisher crNoticePublisher
}

// NewCRNoticeService constructs the change-request notice policy.
func NewCRNoticeService(repo repository.CRNoticeRepository, publisher crNoticePublisher) CRNoticeService {
	return &crNoticeService{repo: repo, publisher: publisher}
}

// CREntityType is the outbox entity_type for a change request -- the table
// name the trigger records, which is what the drainer filters on.
const CREntityType = "change_request"

// Column names as they appear in the outbox diff, which is to_jsonb(NEW) of
// the change_request row and therefore snake_case.
const (
	crColConfirmation = "customer_updated_date_confirmation"
	crColCustomerDate = "customer_updated_on"
	crColState        = "state"
)

// crApprovalStates maps the change-request states the approval flow reacts to
// onto the subject wording and audience its branch used. A state absent from
// this map is not an approval transition and is ignored -- the trigger
// condition, expressed as data rather than a chain of comparisons.
//
//	ServiceNow  state             audience   recipients            subject suffix
//	-4          ASSESS            internal   "Devops Approval"     Request for Approval - Implementation
//	-3          AUTHORIZE         internal   "CAB Approval"        Request for CAB approval - Authorize
//	 0          REVIEW            internal   "Devops Review"       Request for approval - Review
//	 5          CUSTOMER_APPROVAL customer   project contacts      Request for Approval - Implementation
//	 1          CUSTOMER_REVIEW   customer   project contacts      Request for approval - Customer Review
var crApprovalStates = map[string]struct {
	audience events.CRApprovalAudience
	// group is the WSO2 approval group whose members are notified. Empty for
	// a customer-audience branch.
	group string
	// suffix completes "[WSO2 Support] [CR]...(<number>) <suffix>".
	suffix string
}{
	"ASSESS":            {events.CRAudienceInternal, "Devops Approval", "Request for Approval - Implementation"},
	"AUTHORIZE":         {events.CRAudienceInternal, "CAB Approval", "Request for CAB approval - Authorize"},
	"REVIEW":            {events.CRAudienceInternal, "Devops Review", "Request for approval - Review"},
	"CUSTOMER_APPROVAL": {events.CRAudienceCustomer, "", "Request for Approval - Implementation"},
	"CUSTOMER_REVIEW":   {events.CRAudienceCustomer, "", "Request for approval - Customer Review"},
}

// crPlanDateInternalGroup is the approval group the original's internal subflow
// hardcoded by sys_id (509dc4c51b08d250a002c9d3604bcbfa). By name here, for the
// same reason as the approval groups: a sys_id means nothing after the
// migration, and the name is what a reader recognises.
const crPlanDateInternalGroup = "Devops Approval"

// crPlanDateTurn is which half of the plan-start-date conversation an outbox
// row represents.
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

// HandleChange implements CRNoticeService.
//
// Both notices can in principle fire from one row change, so both are offered
// the row. In practice they are mutually exclusive -- a write that moves the
// state is not the write that moves the date -- but nothing enforces that, and
// silently dropping the second would be a bug that only shows under a
// combined update.
func (s *crNoticeService) HandleChange(ctx context.Context, change repository.OutboxChange) error {
	if change.EntityType != CREntityType {
		return nil
	}
	// Both branches run even if the first fails, and the errors are joined.
	// The drainer has already claimed the row, so returning early would not
	// retry the branch that was skipped -- it would drop it, and a combined
	// state-and-date update would silently lose its plan-date notice.
	return errors.Join(
		s.approvalNotice(ctx, change),
		s.planDateNotice(ctx, change),
	)
}

// approvalNotice publishes the "this change request is waiting on you" notice
// when the state moved INTO one of the five approval states.
//
// "Changed to", not "is": the original's trigger is stateCHANGESTO..., so a
// record updated for some other reason while already sitting in an approval
// state must not re-notify. The outbox diff carries before and after per
// column, which is what makes that expressible.
func (s *crNoticeService) approvalNotice(ctx context.Context, change repository.OutboxChange) error {
	state, ok := crChangedToApprovalState(change)
	if !ok {
		return nil
	}
	branch := crApprovalStates[state]

	// Read the record, not the snapshot. The snapshot is only change_request's
	// own columns, and everything the notice says -- the number, the
	// requester, the project -- lives on work_item or a join away. This is
	// also what the ServiceNow original did: its subflows read fields off the
	// triggering record, not off a diff.
	details, err := s.repo.Details(ctx, change.EntityID)
	if err != nil {
		return fmt.Errorf("crnotice: read change request %s: %w", change.EntityID, err)
	}
	if details.Number == "" {
		// Deleted between the outbox row being written and now. A notice about
		// a record that no longer exists is a silent no-op.
		return nil
	}

	notice := events.CRApprovalRequestedPayload{
		ChangeRequestID: change.EntityID,
		Number:          details.Number,
		State:           state,
		Audience:        branch.audience,
		RequesterName:   details.RequesterName,
		ProjectName:     details.ProjectName,
		ProjectID:       details.ProjectID,
	}

	var recipients []string
	if branch.audience == events.CRAudienceInternal {
		notice.Team = crTeamFromGitReference(details.GitReference)
		notice.GroupName = branch.group
		notice.Subject = crSubject(details.Number, branch.suffix, notice.Team)
		recipients, err = s.repo.GroupMemberEmails(ctx, branch.group)
	} else {
		notice.Subject = crSubject(details.Number, branch.suffix, "")
		recipients, err = s.repo.ProjectContactEmails(ctx, details.ProjectID)
	}
	if err != nil {
		return fmt.Errorf("crnotice: resolve %s recipients: %w", branch.audience, err)
	}

	recipients = normaliseAddresses(recipients)
	if len(recipients) == 0 {
		// Nobody to tell. Not an error: an approval group with no members, or
		// a project with no contacts, is a real state, and failing here would
		// retry the row forever against something no retry can fix.
		return nil
	}
	notice.Recipients = recipients

	return s.publish(ctx, events.TypeCRApprovalRequested, change.EntityID, notice)
}

// planDateNotice publishes one turn of the plan-start-date conversation.
//
// The two ServiceNow triggers are:
//
//	u_customer_updatedVALCHANGES^state=5^sys_updated_byNOT LIKE@wso2.com
//	u_confirm_customer_updated_dateVALCHANGES
//
// Two of those three clauses are decidable from the diff alone; the third is
// not -- "who changed it" lives on work_item, not in the change_request
// snapshot -- so the actor gate is applied below, once the record has been
// read.
func (s *crNoticeService) planDateNotice(ctx context.Context, change repository.OutboxChange) error {
	turn := crPlanDateTurnOf(change)
	if turn == crTurnNone {
		return nil
	}

	details, err := s.repo.Details(ctx, change.EntityID)
	if err != nil {
		return fmt.Errorf("crnotice: read change request %s: %w", change.EntityID, err)
	}
	if details.Number == "" {
		return nil
	}

	notice := events.CRPlanDateNoticePayload{
		ChangeRequestID:  change.EntityID,
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
		// sys_updated_byNOT LIKE@wso2.com. A WSO2 user moving the date is the
		// team editing its own plan, which is not news to the team.
		//
		// KNOWN GAP: this is work_item.updated_by read at drain time, not the
		// actor who made THIS change. change_request has no updated_by of its
		// own, so there is nothing truer to read -- the outbox snapshot is
		// to_jsonb(NEW) of change_request and simply does not contain one. Two
		// consequences: a write that touches change_request without touching
		// work_item leaves the previous work_item editor standing in as the
		// actor, and a later edit between the trigger firing and this read
		// replaces them. Either can suppress a genuine customer proposal or
		// admit a WSO2 one.
		//
		// Closing it needs csm-sync to carry ServiceNow's sys_updated_by onto
		// change_request, after which this reads the actor off the snapshot and
		// the question of when it was read stops existing. Tracked separately;
		// the approval notices do not use the actor at all, so only this one
		// branch is affected.
		if details.ActorIsWSO2 {
			return nil
		}
		notice.Kind = events.CRPlanDateCustomerProposed
		notice.Audience = events.CRAudienceInternal
		notice.GroupName = crPlanDateInternalGroup
		notice.Subject = crPlanDateSubject(details.Number, "Customer has updated the plan start date")
		recipients, err = s.repo.GroupMemberEmails(ctx, crPlanDateInternalGroup)

	case crTurnWSO2Accepted:
		notice.Kind = events.CRPlanDateAccepted
		notice.Audience = events.CRAudienceCustomer
		notice.Subject = crPlanDateSubject(details.Number, "Accepted the plan start date")
		recipients, err = s.repo.ProjectContactEmails(ctx, details.ProjectID)

	case crTurnWSO2Rejected:
		notice.Kind = events.CRPlanDateRejected
		notice.Audience = events.CRAudienceCustomer
		notice.Subject = crPlanDateSubject(details.Number, "Reject the proposed plan start date")
		recipients, err = s.repo.ProjectContactEmails(ctx, details.ProjectID)
	}
	if err != nil {
		return fmt.Errorf("crnotice: resolve %s recipients: %w", notice.Audience, err)
	}

	recipients = normaliseAddresses(recipients)
	if len(recipients) == 0 {
		return nil
	}
	notice.Recipients = recipients

	return s.publish(ctx, events.TypeCRPlanDateNotice, change.EntityID, notice)
}

func (s *crNoticeService) publish(ctx context.Context, t events.Type, entityID string, notice any) error {
	payload, err := json.Marshal(notice)
	if err != nil {
		return fmt.Errorf("crnotice: encode %s payload: %w", t, err)
	}
	if err := s.publisher.Publish(ctx, t, entityID, payload); err != nil {
		return fmt.Errorf("crnotice: publish %s: %w", t, err)
	}
	return nil
}

// crChangedToApprovalState reports the approval state a change request just
// moved INTO, if it did. A change with no state transition, or one into a
// state these notices do not cover (SCHEDULED, CLOSED, ...), yields false.
func crChangedToApprovalState(change repository.OutboxChange) (string, bool) {
	to, changed := crChangedTo(change, crColState)
	if !changed || to == "" {
		return "", false
	}
	if _, covered := crApprovalStates[to]; !covered {
		return "", false
	}
	return to, true
}

// crPlanDateTurnOf decides which turn of the date conversation a row change is.
//
// A DATE CHANGE IS READ FIRST, AND THE ORDER MATTERS. The migration that moves
// the date clears the confirmation in the same write, so the outbox sees one
// row whose diff contains BOTH columns. Reading the confirmation first would
// see it cleared and report "no answer" -- silently swallowing every proposal
// that arrives while an answer is already standing.
func crPlanDateTurnOf(change repository.OutboxChange) crPlanDateTurn {
	if _, changed := crChangedTo(change, crColCustomerDate); changed {
		// state=5: the only state in which the original accepts a customer
		// date change. Read from the snapshot, since the state need not have
		// changed in this write.
		if crSnapshotString(change.Snapshot, crColState) != crStateCustomerApproval {
			return crTurnNone
		}
		return crTurnCustomerProposed
	}
	if to, changed := crChangedTo(change, crColConfirmation); changed {
		switch to {
		case "AGREE":
			return crTurnWSO2Accepted
		case "DISAGREE":
			return crTurnWSO2Rejected
		}
	}
	return crTurnNone
}

// crChangedTo reports what a column changed to, and whether it changed at all.
// Unlike crChangedToApprovalState it does not require a non-empty value: a date
// being set for the first time is a change, and so is one being cleared.
func crChangedTo(change repository.OutboxChange, column string) (string, bool) {
	diff, ok := change.Changes[column]
	if !ok {
		return "", false
	}
	to, _ := diff["to"].(string)
	// A no-op update reporting the same value on both sides is not a
	// transition, whatever the diff claims.
	if from, ok := diff["from"].(string); ok && from == to {
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

// crSubject renders the branch's subject line. The two shapes are the
// original's verbatim: the internal subflow interpolated the owning team, the
// customer one did not.
//
//	internal: [WSO2 Support] [CR][Choreo] (CHG0031234) Request for approval - Review
//	customer: [WSO2 Support] [CR] (CHG0031234) Request for approval - Customer Review
func crSubject(number, suffix, team string) string {
	teamPart := ""
	if team != "" {
		teamPart = "[" + team + "]"
	}
	return fmt.Sprintf("[WSO2 Support] [CR]%s (%s) %s", teamPart, number, suffix)
}

// crPlanDateSubject renders "[WSO2 Support] [CR] (<number>) <suffix>". The
// plan-date notices never carry the owning team, unlike the internal approval
// ones -- not an oversight, it is what the originals do.
func crPlanDateSubject(number, suffix string) string {
	return fmt.Sprintf("[WSO2 Support] [CR] (%s) %s", number, suffix)
}

// crTeamFromGitReference derives the owning team the internal subflow put in
// its subject line. The original branched on the change request's git
// reference with If / Else If / Else, assigning Choreo, Asgardeo, then MS as
// the fallback -- so an unrecognised or absent reference is MS by design, not
// by accident.
func crTeamFromGitReference(ref string) string {
	switch {
	case strings.Contains(strings.ToLower(ref), "choreo"):
		return "Choreo"
	case strings.Contains(strings.ToLower(ref), "asgardeo"):
		return "Asgardeo"
	default:
		return "MS"
	}
}

// normaliseAddresses lower-cases, drops blanks, de-duplicates and sorts. One
// person in two approval groups, or listed twice on a project, is notified
// once, and the published payload is stable for the same input.
func normaliseAddresses(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, a := range in {
		addr := strings.ToLower(strings.TrimSpace(a))
		if addr == "" || seen[addr] {
			continue
		}
		seen[addr] = true
		out = append(out, addr)
	}
	sort.Strings(out)
	return out
}
