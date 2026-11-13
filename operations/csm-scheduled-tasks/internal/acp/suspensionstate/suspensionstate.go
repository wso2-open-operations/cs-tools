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

// Package suspensionstate translates between the real, confirmed shape of a
// project's suspensionProcessState (a free-form JSON blob written by an
// existing legacy suspension flow, keyed by
// based_on_subscription_end_date/based_on_due_invoices/based_on_compliance —
// confirmed via a real write against the dedicated test project
// 00000000-0000-4000-8000-000000000001) and this component's own
// closure.NoticeWindow. Phase 1 reads/writes based_on_subscription_end_date;
// Phase 2 reads/writes based_on_due_invoices the same way, via its own
// parallel functions below — each track has its own independent
// idempotency signal, and a write to one key always preserves every other
// key byte-for-byte. based_on_compliance is never read or written by this
// package at all; it belongs to a closure reason this team doesn't handle.
package suspensionstate

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/closure"
)

const subscriptionEndDateKey = "based_on_subscription_end_date"

// dueInvoicesKey is Phase 2's own section key — its own independent
// idempotency track, entirely separate from subscriptionEndDateKey.
const dueInvoicesKey = "based_on_due_invoices"

// SuspendEventType is the event_type of a track ACP suspended. It is written
// only together with the suspended closure state, in one update, after
// Salesforce took State__c=Suspended (WithSubscriptionEndDateSuspended,
// WithDueInvoicesSuspended), so the record never exists without the
// suspension: entity-service's reopen reads "suspend" on an Open project as
// a project a person set back to Open, and finishes it.
const SuspendEventType = "suspend"

// SuspendNoticeEventType records that the day-0 notice went out while the
// suspension itself is not done yet (Salesforce or the CSM write still to
// come, or failed). It reads as window 0, so the next run does not send the
// notice again and only retries the suspension. The reopen ignores it.
const SuspendNoticeEventType = "suspend_notice"

// eventTypeToWindow maps the legacy event_type vocabulary observed in
// based_on_subscription_end_date to closure.NoticeWindow.
var eventTypeToWindow = map[string]closure.NoticeWindow{
	"90_days_notice":       closure.NoticeWindow90,
	"60_days_notice":       closure.NoticeWindow60,
	"30_days_notice":       closure.NoticeWindow30,
	"15_days_notice":       closure.NoticeWindow15,
	"7_days_notice":        closure.NoticeWindow7,
	SuspendNoticeEventType: closure.NoticeWindow0,
	SuspendEventType:       closure.NoticeWindow0,
}

// windowToEventType is the event_type a notice for a window is recorded
// with. Window 0 is SuspendNoticeEventType: "suspend" is written only with
// the suspension.
var windowToEventType = map[closure.NoticeWindow]string{
	closure.NoticeWindow90: "90_days_notice",
	closure.NoticeWindow60: "60_days_notice",
	closure.NoticeWindow30: "30_days_notice",
	closure.NoticeWindow15: "15_days_notice",
	closure.NoticeWindow7:  "7_days_notice",
	closure.NoticeWindow0:  SuspendNoticeEventType,
}

// subscriptionEndDateState and dueInvoicesState are identically-shaped —
// each section is just {event_type, ...action results} — but kept as two
// distinct named types (rather than one shared type) so a future field that
// only makes sense for one reason doesn't quietly leak into the other.
type subscriptionEndDateState struct {
	EventType string `json:"event_type"`
}

type dueInvoicesState struct {
	EventType string `json:"event_type"`
}

// LastNoticeWindow extracts closure.NoticeWindow from the
// based_on_subscription_end_date.event_type field of a raw
// suspensionProcessState blob. Returns nil if the blob is empty, the key is
// absent, or event_type is "open" — meaning no prior notice has fired.
func LastNoticeWindow(raw json.RawMessage) (*closure.NoticeWindow, error) {
	return lastNoticeWindowForKey(raw, subscriptionEndDateKey)
}

// LastNoticeWindowForInvoices is LastNoticeWindow's Phase 2 counterpart,
// reading based_on_due_invoices.event_type instead — its own independent
// idempotency track. Same "nil means no prior notice" contract.
func LastNoticeWindowForInvoices(raw json.RawMessage) (*closure.NoticeWindow, error) {
	return lastNoticeWindowForKey(raw, dueInvoicesKey)
}

// lastNoticeWindowForKey is the shared implementation behind LastNoticeWindow
// and LastNoticeWindowForInvoices — both sections use the identical
// {event_type: "..."} shape and the identical eventTypeToWindow vocabulary,
// only the key differs.
// HasRecordedEvent reports whether raw records any event_type other than
// "open" (or none) for either track: a notice, a suspension, or an event type
// this package does not know. An unparseable state counts as recorded, so a
// caller screening on it errs toward looking at the project.
func HasRecordedEvent(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var blob map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blob); err != nil {
		return true
	}
	for _, key := range []string{subscriptionEndDateKey, dueInvoicesKey} {
		section, ok := blob[key]
		if !ok {
			continue
		}
		var state struct {
			EventType string `json:"event_type"`
		}
		if err := json.Unmarshal(section, &state); err != nil {
			return true
		}
		if state.EventType != "" && state.EventType != "open" {
			return true
		}
	}
	return false
}

func lastNoticeWindowForKey(raw json.RawMessage, key string) (*closure.NoticeWindow, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var blob map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blob); err != nil {
		return nil, fmt.Errorf("suspensionstate: parse blob: %w", err)
	}

	section, ok := blob[key]
	if !ok {
		return nil, nil
	}

	var state struct {
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(section, &state); err != nil {
		return nil, fmt.Errorf("suspensionstate: parse %s: %w", key, err)
	}

	window, ok := eventTypeToWindow[state.EventType]
	if !ok {
		return nil, nil
	}
	return &window, nil
}

// SuspensionCompleted reports whether based_on_subscription_end_date holds a
// completed suspension: event_type "suspend" and no failed suspension action.
// The legacy process re-ran an event only when its type changed or an action
// had failed, so a project reopened by a person is left alone while the saved
// event is still "suspend". ACP's own record has no action result and counts as
// completed: it is written only after the suspension itself succeeded. A blob
// that is empty, lacks the key, or holds another event_type reports false.
func SuspensionCompleted(raw json.RawMessage) (bool, error) {
	return suspensionCompletedForKey(raw, subscriptionEndDateKey)
}

// SuspensionCompletedForInvoices is SuspensionCompleted for
// based_on_due_invoices.
func SuspensionCompletedForInvoices(raw json.RawMessage) (bool, error) {
	return suspensionCompletedForKey(raw, dueInvoicesKey)
}

func suspensionCompletedForKey(raw json.RawMessage, key string) (bool, error) {
	if len(raw) == 0 {
		return false, nil
	}

	var blob map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blob); err != nil {
		return false, fmt.Errorf("suspensionstate: parse blob: %w", err)
	}
	section, ok := blob[key]
	if !ok {
		return false, nil
	}

	var state struct {
		EventType            string          `json:"event_type"`
		ActionSuspendProject json.RawMessage `json:"actionSuspendProject"`
	}
	if err := json.Unmarshal(section, &state); err != nil {
		return false, fmt.Errorf("suspensionstate: parse %s: %w", key, err)
	}
	if state.EventType != SuspendEventType {
		return false, nil
	}
	// actionSuspendProject is the legacy result of the suspension action. Absent
	// (or null) counts as completed; a value must be "SUCCESSFUL".
	result := string(state.ActionSuspendProject)
	return result == "" || result == "null" || result == `"SUCCESSFUL"`, nil
}

// WithSubscriptionEndDateState returns a copy of raw with only the
// based_on_subscription_end_date key replaced by the given window and
// action results. based_on_due_invoices, based_on_compliance, and any other
// keys present in raw are preserved byte-for-byte — they are never
// unmarshaled into a typed structure and re-serialized, only carried through
// as raw JSON, so nothing about their formatting or content can drift.
func WithSubscriptionEndDateState(raw json.RawMessage, window closure.NoticeWindow, actions map[string]string) (json.RawMessage, error) {
	return withState(raw, subscriptionEndDateKey, window, actions)
}

// WithDueInvoicesState is WithSubscriptionEndDateState's Phase 2
// counterpart — replaces only based_on_due_invoices, preserving
// based_on_subscription_end_date, based_on_compliance, and any other key
// byte-for-byte, with the same guarantee in the opposite direction Phase 1's
// own preservation test covers.
func WithDueInvoicesState(raw json.RawMessage, window closure.NoticeWindow, actions map[string]string) (json.RawMessage, error) {
	return withState(raw, dueInvoicesKey, window, actions)
}

// WithSubscriptionEndDateOpen returns a copy of raw with
// based_on_subscription_end_date set back to {"event_type":"open"}: no
// notice sent for the current term. Legacy saved the recomputed event on every run,
// so a date that moved out past the recorded window started the reminders
// over. Other keys are preserved byte-for-byte, as in
// WithSubscriptionEndDateState.
func WithSubscriptionEndDateOpen(raw json.RawMessage) (json.RawMessage, error) {
	return withSection(raw, subscriptionEndDateKey, map[string]string{"event_type": "open"})
}

// WithDueInvoicesOpen is WithSubscriptionEndDateOpen for
// based_on_due_invoices.
func WithDueInvoicesOpen(raw json.RawMessage) (json.RawMessage, error) {
	return withSection(raw, dueInvoicesKey, map[string]string{"event_type": "open"})
}

// WithSubscriptionEndDateSuspended returns a copy of raw with
// based_on_subscription_end_date recording ACP's suspension: event_type
// "suspend" and suspended_on (YYYY-MM-DD), keeping the section's action
// results (the day-0 notice's actionSendEmailNotification). It is sent in the
// same update as endDateClosureState. Other keys are preserved byte-for-byte.
func WithSubscriptionEndDateSuspended(raw json.RawMessage, suspendedOn string) (json.RawMessage, error) {
	return withSuspended(raw, subscriptionEndDateKey, suspendedOn)
}

// WithDueInvoicesSuspended is WithSubscriptionEndDateSuspended for
// based_on_due_invoices, sent with invoiceDueDateClosureState.
func WithDueInvoicesSuspended(raw json.RawMessage, suspendedOn string) (json.RawMessage, error) {
	return withSuspended(raw, dueInvoicesKey, suspendedOn)
}

// withSuspended is the shared implementation behind
// WithSubscriptionEndDateSuspended and WithDueInvoicesSuspended. The
// section's string fields are carried over; a field that is not a string
// (none is written by this package) is dropped.
func withSuspended(raw json.RawMessage, key, suspendedOn string) (json.RawMessage, error) {
	section := map[string]string{}
	if len(raw) > 0 {
		var blob map[string]json.RawMessage
		if err := json.Unmarshal(raw, &blob); err != nil {
			return nil, fmt.Errorf("suspensionstate: parse blob: %w", err)
		}
		if existing, ok := blob[key]; ok {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(existing, &fields); err == nil {
				for name, value := range fields {
					var str string
					if json.Unmarshal(value, &str) == nil {
						section[name] = str
					}
				}
			}
		}
	}
	section["event_type"] = SuspendEventType
	section["suspended_on"] = suspendedOn
	return withSection(raw, key, section)
}

// withState is the shared implementation behind WithSubscriptionEndDateState
// and WithDueInvoicesState — both sections are written identically, only the
// key differs.
func withState(raw json.RawMessage, key string, window closure.NoticeWindow, actions map[string]string) (json.RawMessage, error) {
	eventType, ok := windowToEventType[window]
	if !ok {
		return nil, fmt.Errorf("suspensionstate: no event_type mapping for window %d", window)
	}

	section := map[string]string{"event_type": eventType}
	for action, result := range actions {
		section[action] = result
	}
	return withSection(raw, key, section)
}

// withSection returns a copy of raw with only key replaced by section.
func withSection(raw json.RawMessage, key string, section map[string]string) (json.RawMessage, error) {
	blob := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &blob); err != nil {
			return nil, fmt.Errorf("suspensionstate: parse blob: %w", err)
		}
	}
	// A JSON null (suspension_process_state NULL on the CSM database: a project
	// that never had a notice) decodes into a nil map; start a new blob.
	if blob == nil {
		blob = map[string]json.RawMessage{}
	}

	sectionRaw, err := json.Marshal(section)
	if err != nil {
		return nil, fmt.Errorf("suspensionstate: marshal %s: %w", key, err)
	}
	blob[key] = sectionRaw

	out, err := marshalWithoutHTMLEscaping(blob)
	if err != nil {
		return nil, fmt.Errorf("suspensionstate: marshal blob: %w", err)
	}
	return out, nil
}

// marshalWithoutHTMLEscaping is json.Marshal, minus one behavior that would
// otherwise break the byte-for-byte preservation this package promises:
// encoding/json's default encoder HTML-escapes '<', '>', and '&' in *any*
// output it produces — including bytes coming from an untouched
// json.RawMessage value that was never semantically changed (confirmed via
// a real CodeRabbit review finding, PR #1657: a project name or note
// containing one of those characters would otherwise get silently rewritten
// on every write to an unrelated section). json.Encoder.SetEscapeHTML(false)
// disables exactly that step; Encode also appends a trailing newline
// json.Marshal doesn't, trimmed here to keep this a drop-in replacement.
func marshalWithoutHTMLEscaping(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
