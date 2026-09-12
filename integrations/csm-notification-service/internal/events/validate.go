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
		if p.ReporterName == "" || p.ProjectName == "" || p.ProjectID == "" || p.CaseID == "" || p.CaseTitle == "" ||
			p.CaseType == "" || p.Priority == "" || p.CreatedAt == "" || p.Description == "" ||
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
	case TypeCaseMentioned:
		var p CaseMentionedPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.MentionerName == "" || p.ProjectID == "" || p.CaseID == "" || p.CaseComment == "" ||
			p.CommentID == "" || !validRecipients(p.Recipients) {
			return fmt.Errorf("events: missing required field for %s", t)
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
	case TypeSLAClockRegister:
		var p SLAClockRegisterPayload
		if err := decodeStrict(raw, &p); err != nil {
			return err
		}
		if p.CaseID == "" || len(p.Durations) == 0 || p.CaseTitle == "" {
			return fmt.Errorf("events: missing required field for %s", t)
		}
		for clockType, dur := range p.Durations {
			if clockType == "" || dur == "" {
				return fmt.Errorf("events: %s durations must have non-empty clock types and values", t)
			}
			// A duration time.ParseDuration can't parse would otherwise only
			// surface deep inside slaengine's own registration loop, which
			// logs and skips that one clockType rather than failing the
			// whole record — for a payload whose durations are ALL
			// unparsable, that means the record is silently marked handled
			// with no clock ever registered and no retry/DLQ visibility.
			// Rejecting it here instead makes it retried and dead-lettered
			// like any other malformed record.
			if d, err := time.ParseDuration(dur); err != nil || d <= 0 {
				return fmt.Errorf("events: %s duration %q for clock type %q is not a valid positive duration", t, dur, clockType)
			}
		}
		// Each AvoidWeekendDueDate entry must name a clock type Durations
		// actually has an entry for — same "one bad entry fails the whole
		// event" posture validRecipients uses, rather than silently
		// ignoring a typo'd/stale clock-type name.
		for _, clockType := range p.AvoidWeekendDueDate {
			if _, ok := p.Durations[clockType]; !ok {
				return fmt.Errorf("events: %s avoidWeekendDueDate entry %q does not match any durations clock type", t, clockType)
			}
		}
		if p.CaseID != entityID {
			return fmt.Errorf("events: payload caseId %q does not match entityId %q", p.CaseID, entityID)
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
