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

package dispatch

// The incident.created handler.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// handleIncidentCreated has exactly one reaction — a voice call — unlike
// this file's other two-reaction handlers: incident.created no longer has
// a Google Chat alert at all, per explicit product direction (an incident
// pages on-call directly; a separate Chat post was redundant with that).
// entity-service's own IncidentCreatedPayload.Product field is still
// accepted on the wire (decode-compatibility, unused) but no longer read
// here — see that field's own doc comment.
//
// CallTo falls back to Dispatcher.defaultOnCallNumber when the payload's
// own value is empty — a publisher that has no way to determine on-call
// rotations itself (e.g. entity-service) can omit it entirely; events.Validate
// allows this. A publisher that does know the right number per incident can
// still supply it and takes precedence over the default. If the resolved
// number is still empty (payload and default both unset), the call is
// skipped (logged, treated as succeeded) instead of calling MakeCall with
// an empty destination — that would just return a real error, which would
// otherwise burn all of eventbus.Consumer's retries and dead-letter an
// incident whose only problem is a missing operator default, not a
// transient failure.
//
// callSendingEnabled gates the whole call step (CALL_SENDING_ENABLED): when
// false, this logs what would have been called instead of calling, and
// still marks it "done" so a disabled call doesn't retry forever — the same
// log-only shape sendPerGroup's email sending used to have before
// EMAIL_DEBUG_MODE replaced it with a redirect-to-a-test-list behavior (see
// sendPerGroup's doc comment); calls have no equivalent debug-recipient
// concept, so this keeps the simpler disable-entirely shape.
//
// With only one channel left, there's no cross-channel release race to
// guard against the way beginRecord/endRecord exists for elsewhere in this
// file — this call's own callOwned already fully determines whether it's
// safe to release, the same reasoning handleCaseAcknowledged's own doc
// comment gives for its own single-channel shape. claim/forget keys on this
// specific record (recordBaseKey, unique per event content, not per Kafka
// delivery — see its doc comment): eventbus.Consumer retries this whole
// function on any error, and without tracking, a transient failure would
// resend an already-succeeded call on every retry too.
func (d *Dispatcher) handleIncidentCreated(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.IncidentCreatedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode incident.created payload: %w", err)
	}

	callTo := p.CallTo
	if callTo == "" {
		callTo = d.defaultOnCallNumber
	}

	callKey := recordBaseKey(record) + "/call"
	callOwned := d.claim(callKey)
	var callErr error
	if callOwned {
		switch {
		case !d.callSendingEnabled:
			slog.InfoContext(ctx, "dispatch: call sending disabled (CALL_SENDING_ENABLED=false); not calling", "to", maskPhone(callTo))
		case callTo == "":
			slog.WarnContext(ctx, "dispatch: no callTo for incident.created (payload and INCIDENT_DEFAULT_CALL_TO both empty); skipping call")
		default:
			message := fmt.Sprintf("New incident: %s. %s", p.Title, p.ShortDescription)
			callErr = d.call.MakeCall(ctx, callTo, message)
			if callErr != nil {
				d.forget(callKey)
				callOwned = false
			}
		}
	}

	// Deliberately just callOwned, not "|| record.NoMoreRetries" — see
	// handleCaseAcknowledged's own doc comment for why that would be wrong
	// with only one channel/claim total: whichever call actually owns it
	// is the only call that will ever release it.
	if callOwned {
		d.forget(callKey)
	}
	return callErr
}

// maskPhone redacts all but the last 4 characters of an E.164 phone number
// for logging — this repo's own convention is to log only ids and sanitised
// summaries, not raw PII, and a phone number is PII the same way a recipient
// email address is (see internal/recipientlinks' own equivalent reasoning).
// A number with 4 or fewer characters (never valid E.164, but defensive
// against a malformed default) is masked entirely rather than echoed as-is.
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return strings.Repeat("*", len(phone))
	}
	return strings.Repeat("*", len(phone)-4) + phone[len(phone)-4:]
}
