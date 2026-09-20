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
// existing ServiceNow suspension flow, keyed by
// based_on_subscription_end_date/based_on_due_invoices/based_on_compliance —
// confirmed via a real write against the dedicated test project
// e3e87599-1bc7-6650-182c-0dc5604bcb68) and this component's own
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

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/closure"
)

const subscriptionEndDateKey = "based_on_subscription_end_date"

// dueInvoicesKey is Phase 2's own section key — its own independent
// idempotency track, entirely separate from subscriptionEndDateKey.
const dueInvoicesKey = "based_on_due_invoices"

// eventTypeToWindow maps the legacy event_type vocabulary observed in
// based_on_subscription_end_date to closure.NoticeWindow.
var eventTypeToWindow = map[string]closure.NoticeWindow{
	"90_days_notice": closure.NoticeWindow90,
	"60_days_notice": closure.NoticeWindow60,
	"30_days_notice": closure.NoticeWindow30,
	"15_days_notice": closure.NoticeWindow15,
	"7_days_notice":  closure.NoticeWindow7,
	"suspend":        closure.NoticeWindow0,
}

// windowToEventType is the inverse of eventTypeToWindow, used when writing a
// new based_on_subscription_end_date state back.
var windowToEventType = map[closure.NoticeWindow]string{
	closure.NoticeWindow90: "90_days_notice",
	closure.NoticeWindow60: "60_days_notice",
	closure.NoticeWindow30: "30_days_notice",
	closure.NoticeWindow15: "15_days_notice",
	closure.NoticeWindow7:  "7_days_notice",
	closure.NoticeWindow0:  "suspend",
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

// withState is the shared implementation behind WithSubscriptionEndDateState
// and WithDueInvoicesState — both sections are written identically, only the
// key differs.
func withState(raw json.RawMessage, key string, window closure.NoticeWindow, actions map[string]string) (json.RawMessage, error) {
	blob := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &blob); err != nil {
			return nil, fmt.Errorf("suspensionstate: parse blob: %w", err)
		}
	}

	eventType, ok := windowToEventType[window]
	if !ok {
		return nil, fmt.Errorf("suspensionstate: no event_type mapping for window %d", window)
	}

	section := map[string]string{"event_type": eventType}
	for action, result := range actions {
		section[action] = result
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
