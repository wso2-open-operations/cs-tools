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
	"sort"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
)

// crApprovalNotice is the port of the ServiceNow flow "CR Approval
// notifications" together with the two subflows it calls, "Send CR Approval
// For Customer" and "Send CR Approval For Internal".
//
// The original triggers on change_request record_update with
//
//	stateCHANGESTO-4^ORstateCHANGESTO-3^ORstateCHANGESTO5^ORstateCHANGESTO0^ORstateCHANGESTO1
//
// and branches five ways, one per state. Each branch resolves an audience and
// sends one email per recipient:
//
//	state              branch            audience   recipients              subject suffix
//	-4 ASSESS          Access approval   internal   "Devops Approval" grp    Request for Approval - Implementation
//	-3 AUTHORIZE       Authorize apprv   internal   "CAB Approval" grp       Request for CAB approval - Authorize
//	 0 REVIEW          Internal Review   internal   "Devops Review" grp      Request for approval - Review
//	 5 CUSTOMER_APPRV  Customer apprv    customer   project contacts         Request for Approval - Implementation
//	 1 CUSTOMER_REVIEW Customer Review   customer   project contacts         Request for approval - Customer Review
//
// The internal subflow additionally derives an owning team from the change
// request's git reference (Choreo / Asgardeo / MS) and puts it in the subject;
// the customer subflow never did.
//
// WHAT THIS DOES NOT DO: send the email. This service is forbidden from
// sending email itself (CLAUDE.md) — it publishes a request and
// csm-notification-service sends it. That consumer does not exist yet; see
// events.TypeCRApprovalRequested.
//
// DELIBERATE DIFFERENCE FROM THE ORIGINAL: ServiceNow sent one email per
// recipient, because its subflows looped (For Each) with the address as the
// To. This publishes ONE event carrying the whole resolved recipient list.
// Nothing about the notice is per-person — same subject, same body — so the
// fan-out was an artefact of Flow Designer's loop, not intent, and preserving
// it would mean publishing N events that each say the same thing.
type crApprovalNotice struct{}

func (crApprovalNotice) Key() string { return "cr_approval_notice" }

// TriggerEntityTypes marks this flow as row-triggered: it fires on a
// change_request row changing, which is what the ServiceNow original's
// "Change Request Updated" trigger meant. Declared here rather than configured
// in main so registering the flow cannot silently fail to drain the one table
// it depends on.
func (crApprovalNotice) TriggerEntityTypes() []string { return []string{crEntityType} }

// crApprovalStates maps the change-request states this flow reacts to onto the
// subject-line wording and audience its branch used. A state absent from this
// map is not an approval transition and the flow ignores it — which is the
// trigger condition, expressed as data rather than a chain of comparisons.
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

// Match is the trigger + condition gate: a change_request whose state CHANGED
// TO one of the five approval states.
//
// "Changed to", not "is": the original's trigger is stateCHANGESTO..., so a
// record updated for some other reason while already sitting in an approval
// state must not re-notify. entity.changed carries the before/after per field,
// which is what makes that expressible here — the state is only a match when
// the payload says it actually moved.
func (crApprovalNotice) Match(evt Event) bool {
	if evt.Envelope.Type != events.TypeEntityChanged {
		return false
	}
	var payload events.EntityChangedPayload
	if err := json.Unmarshal(evt.Envelope.Payload, &payload); err != nil {
		return false
	}
	if payload.EntityType != crEntityType {
		return false
	}
	_, changed := crChangedToApprovalState(payload)
	return changed
}

// crEntityType is the entity.changed entityType for a change request. It is
// the csm-sync-service target-table name, since that is what the outbox keys
// on -- not ServiceNow's own table name, though the two happen to match here.
const crEntityType = "change_request"

// crChangedToApprovalState reports the approval state a change request just
// moved INTO, if it did. A change with no state transition, or a transition to
// a state this flow does not cover (SCHEDULED, CLOSED, ...), yields false.
func crChangedToApprovalState(payload events.EntityChangedPayload) (string, bool) {
	change, ok := payload.Changes["state"]
	if !ok {
		return "", false
	}
	to, ok := change["to"].(string)
	if !ok || to == "" {
		return "", false
	}
	// A no-op update that reports the same value on both sides is not a
	// transition, whatever the change map claims.
	if from, ok := change["from"].(string); ok && from == to {
		return "", false
	}
	if _, covered := crApprovalStates[to]; !covered {
		return "", false
	}
	return to, true
}

// Run resolves the branch's audience and publishes one approval-notice request.
func (f crApprovalNotice) Run(ctx context.Context, evt Event, deps Deps) error {
	var payload events.EntityChangedPayload
	if err := json.Unmarshal(evt.Envelope.Payload, &payload); err != nil {
		return fmt.Errorf("cr_approval_notice: decode entity.changed: %w", err)
	}
	state, ok := crChangedToApprovalState(payload)
	if !ok {
		// Match already gated this; a mismatch here means the payload changed
		// between the two calls, which cannot happen for one record.
		return nil
	}
	branch := crApprovalStates[state]

	// Read the record rather than the snapshot. The snapshot is only the
	// changed table's own columns, and everything the notice says -- the
	// number, the requester, the project -- lives elsewhere. This is also what
	// the ServiceNow original did: its subflows read fields off the triggering
	// record, not off a diff.
	if deps.ChangeRequests == nil {
		return fmt.Errorf("cr_approval_notice: change request reader is not configured")
	}
	details, err := deps.ChangeRequests.ChangeRequestDetails(ctx, payload.EntityID)
	if err != nil {
		return fmt.Errorf("cr_approval_notice: read change request %s: %w", payload.EntityID, err)
	}

	notice := events.CRApprovalRequestedPayload{
		ChangeRequestID: payload.EntityID,
		Number:          details.Number,
		State:           state,
		Audience:        branch.audience,
		RequesterName:   details.RequesterName,
		ProjectName:     details.ProjectName,
		ProjectID:       details.ProjectID,
	}

	if branch.audience == events.CRAudienceInternal {
		notice.Team = crTeamFromGitReference(details.GitReference)
		notice.GroupName = branch.group
		notice.Subject = crSubject(details.Number, branch.suffix, notice.Team)
	} else {
		notice.Subject = crSubject(details.Number, branch.suffix, "")
	}

	recipients, err := f.resolveRecipients(ctx, deps, branch.audience, branch.group, details.ProjectID)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		// Nobody to tell. Not an error: an approval group with no members, or
		// a project with no contacts, is a real state, and failing here would
		// retry the whole record forever against something no retry can fix.
		//
		// Checked BEFORE the debug-recipient swap on purpose: a deployment
		// pointed at a test mailbox must still stay silent about an event that
		// would have notified nobody, or it produces mail the real deployment
		// never would.
		return nil
	}
	notice.Recipients = applyDebugRecipients(recipients, deps.EmailDebugRecipients)

	body, err := json.Marshal(events.Envelope{
		Type:     events.TypeCRApprovalRequested,
		EntityID: payload.EntityID,
		Payload:  mustMarshal(notice),
	})
	if err != nil {
		return fmt.Errorf("cr_approval_notice: encode notice: %w", err)
	}
	// Keyed by the change request id so every notice about one CR stays
	// ordered on the same partition, matching entity-service's own convention.
	if err := deps.Producer.Publish(ctx, []byte(payload.EntityID), body); err != nil {
		return fmt.Errorf("cr_approval_notice: publish notice: %w", err)
	}
	return nil
}

// resolveRecipients returns the addresses this branch notifies: an approval
// group's members for an internal branch, the customer project's contacts for
// a customer one. Addresses are lower-cased, de-duplicated and sorted so the
// published payload is stable for the same input.
func (crApprovalNotice) resolveRecipients(
	ctx context.Context, deps Deps,
	audience events.CRApprovalAudience, group, projectID string,
) ([]string, error) {
	if deps.Recipients == nil {
		return nil, fmt.Errorf("cr_approval_notice: recipient store is not configured")
	}
	var (
		addrs []string
		err   error
	)
	switch audience {
	case events.CRAudienceInternal:
		addrs, err = deps.Recipients.GroupMemberEmails(ctx, group)
	case events.CRAudienceCustomer:
		addrs, err = deps.Recipients.ProjectContactEmails(ctx, projectID)
	default:
		return nil, fmt.Errorf("cr_approval_notice: unknown audience %q", audience)
	}
	if err != nil {
		return nil, fmt.Errorf("cr_approval_notice: resolve %s recipients: %w", audience, err)
	}
	return normaliseAddresses(addrs), nil
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

// crTeamFromGitReference derives the owning team the internal subflow put in
// its subject line. The original branched on the change request's git
// reference with If / Else If / Else, assigning Choreo, Asgardeo, then MS as
// the fallback — so an unrecognised or absent reference is MS by design, not
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

// normaliseAddresses lower-cases, drops blanks, de-duplicates and sorts.
// One person in two approval groups, or listed twice on a project, must be
// notified once.
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

// applyDebugRecipients swaps a resolved audience for the configured debug list,
// when one is configured. Real resolution has already happened by the time this
// is called, so a broken entity-service lookup still surfaces as an error
// rather than being hidden behind the override.
//
// An empty real audience stays empty regardless: a deployment pointed at a test
// mailbox must stay silent about an event that would have notified nobody, or
// it produces mail the real deployment never would.
func applyDebugRecipients(resolved, debug []string) []string {
	if len(resolved) == 0 || len(debug) == 0 {
		return resolved
	}
	return normaliseAddresses(debug)
}

// mustMarshal encodes a payload that cannot fail to encode — every field is a
// string or a []string. A failure here is a programming error, not a runtime
// condition, so it yields an empty payload the caller's own Marshal reports
// rather than a panic in a consumer goroutine.
func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return b
}

var _ Flow = crApprovalNotice{}
