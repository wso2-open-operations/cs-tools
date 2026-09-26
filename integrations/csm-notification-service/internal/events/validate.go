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

package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"time"
)

// emailPattern is a deliberately loose "does this look like an email
// address" check — local@domain.tld — not full RFC 5322 validation. Good
// enough to catch the actually-costly mistake (a blank or clearly-malformed
// recipient that would burn all of handleAttempts' retries downstream before
// being dropped), without trying to be a real email validator.
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// e164Pattern matches E.164 phone numbers (e.g. "+14155552671") — a leading
// "+", a non-zero first digit, then up to 14 more digits.
var e164Pattern = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// validSLATier mirrors slaengine's own fixed set of tiers — 50%, 75%, 100%
// elapsed. Kept here, not imported from slaengine, since this package is
// the schema/validation layer both slaengine and dispatch depend on, not
// the other way around.
var validSLATier = map[string]bool{"50": true, "75": true, "100": true}

// validRecipients reports whether every entry in recipients looks like an
// email address, and there's at least one. A single malformed entry fails
// the whole event — better to reject once here (and dead-letter it) than
// let the notification clients fail downstream on an address that can never
// succeed.
func validRecipients(recipients []string) bool {
	if len(recipients) == 0 {
		return false
	}
	for _, r := range recipients {
		if !emailPattern.MatchString(r) {
			return false
		}
	}
	return true
}

// Validate decodes raw as t's matching payload type (rejecting unknown
// fields) and checks its required fields are non-empty. This is the only
// validation boundary this service has left: callers (csm-portal-backend,
// customer-portal-backend) publish directly to the event bus themselves —
// this service never sees a request before the record is already on the
// topic — so dispatch.Dispatcher.Handle calls this before rendering/sending
// anything, rather than a since-removed HTTP handler validating before
// publish.
//
// This is deliberately duplicated per type rather than done via reflection —
// each type's required fields are exactly the ones its Render* function in
// internal/notifications needs. entityID is the envelope's own EntityID —
// for the three case.* types that carry their own CaseID, it must match:
// EntityID is the Kafka partition key (see Envelope's doc comment), so a
// payload whose CaseID disagrees with it would have been keyed under the
// wrong case's partition, breaking that other case's ordering guarantee.
func Validate(entityID string, t Type, raw json.RawMessage) error {
	switch t {
	case TypeCaseCreated:
		var p CaseCreatedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		// Priority is deliberately NOT required here: entity-service only
		// ever sets a severity (and so a Priority) for type=="case" — every
		// other case type (service_request/security_report_analysis/
		// announcement/engagement) genuinely has none, by design, not a
		// data-quality bug (see entity-service's own validateCreateCaseRequest).
		// Requiring it unconditionally used to reject case.created outright
		// for every one of those types, before it ever reached dispatch's
		// own CaseType branching — no email or Chat alert ever went out for
		// them as a result. RenderCaseCreatedEmail already renders an empty
		// Priority as a blank value with no ill effect, and
		// SendSecurityReportAnalysisAlert/SendCaseCreatedAlert both already
		// omit their severity-derived line entirely when it's empty.
		if p.ReporterName == "" || p.ProjectName == "" || p.ProjectID == "" || p.CaseID == "" || p.CaseTitle == "" ||
			p.CaseType == "" || p.CreatedAt == "" || p.Description == "" ||
			!validRecipients(p.Recipients) {
			return fmt.Errorf("events: missing required field for %s", t)
		}
		if p.CaseID != entityID {
			return fmt.Errorf("events: payload caseId %q does not match entityId %q", p.CaseID, entityID)
		}
	case TypeCommentAdded:
		var p CommentAddedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.Name == "" || p.ProjectID == "" || p.CaseID == "" || p.CaseTitle == "" || p.CaseComment == "" ||
			p.CommentID == "" || !validRecipients(p.Recipients) {
			return fmt.Errorf("events: missing required field for %s", t)
		}
		if p.CaseID != entityID {
			return fmt.Errorf("events: payload caseId %q does not match entityId %q", p.CaseID, entityID)
		}
	case TypeStatusChanged:
		var p StatusChangedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.ProjectID == "" || p.CaseID == "" || p.NewStatus == "" || !validRecipients(p.Recipients) {
			return fmt.Errorf("events: missing required field for %s", t)
		}
		if p.CaseID != entityID {
			return fmt.Errorf("events: payload caseId %q does not match entityId %q", p.CaseID, entityID)
		}
	case TypeCaseAssigned:
		var p CaseAssignedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.AssigneeName == "" || p.AssigneeEmail == "" || p.ProjectID == "" || p.CaseID == "" ||
			!validRecipients(p.Recipients) {
			return fmt.Errorf("events: missing required field for %s", t)
		}
		if p.CaseID != entityID {
			return fmt.Errorf("events: payload caseId %q does not match entityId %q", p.CaseID, entityID)
		}
	case TypeCaseAcknowledged:
		var p CaseAcknowledgedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		// No Recipients check here — unlike every other case.* type, this
		// one is Chat-only (see CaseAcknowledgedPayload's own doc comment),
		// so there's no recipient list to validate.
		if p.CaseID == "" || p.AcknowledgerName == "" {
			return fmt.Errorf("events: missing required field for %s", t)
		}
		if p.CaseID != entityID {
			return fmt.Errorf("events: payload caseId %q does not match entityId %q", p.CaseID, entityID)
		}
	case TypeSeverityChanged:
		var p SeverityChangedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.ProjectID == "" || p.CaseID == "" || p.OldSeverity == "" || p.NewSeverity == "" ||
			p.OldSeverity == p.NewSeverity || !validRecipients(p.Recipients) {
			return fmt.Errorf("events: missing or invalid required field for %s", t)
		}
		if p.CaseID != entityID {
			return fmt.Errorf("events: payload caseId %q does not match entityId %q", p.CaseID, entityID)
		}
	case TypeIncidentCreated:
		var p IncidentCreatedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		// entityID is required here (unlike its role for the case.* types
		// above, where it's checked against the payload's own CaseID
		// instead): dispatch.handleIncidentCreated builds the Chat alert's
		// portal link directly from it (recipientlinks.Resolver.IncidentLink),
		// so an empty entityID would produce a broken link on an otherwise
		// "valid" event rather than being caught here.
		if entityID == "" || p.Title == "" || p.ShortDescription == "" {
			return fmt.Errorf("events: missing required field for %s", t)
		}
		// Product and CallTo are optional: a publisher that can't determine
		// which Chat space or on-call number applies (e.g. entity-service)
		// may omit them, and dispatch substitutes its own configured
		// defaults. A non-empty CallTo must still be a valid E.164 number —
		// this only relaxes "absent," not "malformed."
		if p.CallTo != "" && !e164Pattern.MatchString(p.CallTo) {
			return fmt.Errorf("events: %s callTo %q is not a valid E.164 phone number", t, p.CallTo)
		}
	case TypeSLATierReached:
		var p SLATierReachedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.CaseID == "" || p.ClockType == "" || !validSLATier[p.Tier] {
			return fmt.Errorf("events: missing or invalid required field for %s", t)
		}
		if p.CaseID != entityID {
			return fmt.Errorf("events: payload caseId %q does not match entityId %q", p.CaseID, entityID)
		}
	case TypeCaseBillableStatusChanged:
		var p CaseBillableStatusChangedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.CaseID == "" {
			return fmt.Errorf("events: missing required field for %s", t)
		}
		if p.CaseID != entityID {
			return fmt.Errorf("events: payload caseId %q does not match entityId %q", p.CaseID, entityID)
		}
	case TypeCRPlanDateNotice:
		var p CRPlanDateNoticePayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.ChangeRequestID == "" || p.Number == "" || p.Subject == "" || p.Kind == "" {
			return fmt.Errorf("events: missing required field for %s", t)
		}
		if p.ChangeRequestID != entityID {
			return fmt.Errorf("events: payload changeRequestId %q does not match entityId %q", p.ChangeRequestID, entityID)
		}
		// Kind and Audience are not independent: the kind decides who the
		// notice is addressed to, and the audience decides the portal link
		// and whether recipients go in To or BCC. A mismatched pair sends
		// customer wording to an internal group, or puts an internal
		// audience's addresses where a customer can read them.
		switch {
		case p.Kind == "customer_proposed" && p.Audience == "internal":
		case p.Kind == "accepted" && p.Audience == "customer":
		case p.Kind == "rejected" && p.Audience == "customer":
		default:
			return fmt.Errorf("events: %s has kind %q that does not go with audience %q", t, p.Kind, p.Audience)
		}
		if !validRecipients(p.Recipients) {
			return fmt.Errorf("events: invalid recipients for %s", t)
		}
	case TypeCRApprovalRequested:
		var p CRApprovalRequestedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		// Subject and recipients are what makes this sendable at all: the
		// flow builds the subject (reproducing ServiceNow's per-branch
		// wording) and resolves the audience, and a notice missing either is
		// one this service cannot repair by retrying.
		if p.ChangeRequestID == "" || p.Number == "" || p.State == "" || p.Subject == "" {
			return fmt.Errorf("events: missing required field for %s", t)
		}
		if p.ChangeRequestID != entityID {
			return fmt.Errorf("events: payload changeRequestId %q does not match entityId %q", p.ChangeRequestID, entityID)
		}
		if p.Audience != "internal" && p.Audience != "customer" {
			return fmt.Errorf("events: %s has unknown audience %q", t, p.Audience)
		}
		if !validRecipients(p.Recipients) {
			return fmt.Errorf("events: invalid recipients for %s", t)
		}
	case TypeProjectContactInvited:
		var p ProjectContactInvitedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		// Only the two values no step can proceed without are required:
		// MembershipSfID keys every onboarding-step write, and Email is
		// both the Asgardeo userName and the invitation's recipient.
		// GivenName/FamilyName are optional (Salesforce doesn't require a
		// first name; dispatch falls back to the email's local part), and
		// ProjectName/ProjectKey/Roles/Type are display-only, and
		// IsResend is a marker dispatch acts on, valid either way —
		// an absent one is simply a first invitation.
		if p.MembershipSfID == "" || !emailPattern.MatchString(p.Email) {
			return fmt.Errorf("events: missing or invalid required field for %s", t)
		}
		if p.MembershipSfID != entityID {
			return fmt.Errorf("events: payload membershipSfId %q does not match entityId %q", p.MembershipSfID, entityID)
		}
		if p.EventModifiedOn != "" {
			if _, err := time.Parse(time.RFC3339Nano, p.EventModifiedOn); err != nil {
				return fmt.Errorf("events: eventModifiedOn %q is not RFC 3339: %w", p.EventModifiedOn, err)
			}
		}
	default:
		return fmt.Errorf("events: unknown event type %q", t)
	}
	return nil
}

// decodeStrict unmarshals raw into v, rejecting any field not present in v's
// struct definition, or any trailing value after the first — raw is always
// exactly one JSON value in today's only call path (env.Payload, extracted
// by the outer Unmarshal in dispatch.Dispatcher.Handle, which already
// rejects trailing garbage on the envelope itself), but this stays defensive
// against a future caller passing something less strictly bounded.
func decodeStrict(raw json.RawMessage, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("events: unexpected trailing data after payload")
	}
	return nil
}
