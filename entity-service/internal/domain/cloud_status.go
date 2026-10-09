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

package domain

import (
	"strings"
	"time"
)

// Cloud status webhooks
//
// The port of ServiceNow's `Cloud Status Event Notification Flow`. When an
// outage against one of the monitored cloud services begins or ends, the
// public status dashboard is told over a webhook.
//
// The webhook AND the status writes are both ported. The flow rewrote
// cloud_monitor.status for the outage's own CI and for every affected one,
// and cloudStatusService.applyMonitorStatus does the same through
// SetMonitorStatus.
//
// *** THAT WRITE SHARES A COLUMN WITH csm-sync-service. *** While the
// one-time bulk migration is still running, both can write
// cloud_monitor.status. It is not a conflict after cutover -- the sync stops
// existing and outages are written directly to Postgres -- but until then
// the drainer is gated behind CLOUD_STATUS_DRAINER_ENABLED so the write can
// be held back without also disabling the sweep endpoint.

// CloudStatusEvent is the `event` string the webhook body carries.
type CloudStatusEvent string

const (
	// CloudStatusEventOutageBegin is confirmed: the flow's step 15 passes this
	// exact literal.
	CloudStatusEventOutageBegin CloudStatusEvent = "OUTAGE_BEGIN"

	// CloudStatusEventOutageEnd is the completed arm, step 8.
	//
	// *** THE WIRE VALUE FOR THIS ONE IS NOT CONFIRMED. *** Step 8's Event
	// input was never captured from ServiceNow -- only step 15's. The name
	// here is this service's own and is safe; what is a guess is
	// CloudStatusEventWireValue's mapping of it to "outage_end". The
	// dashboard switches on that string, so a wrong guess is a silent
	// no-op on the receiving side, not an error anyone sees. Confirm it
	// against the flow before this goes anywhere near production.
	CloudStatusEventOutageEnd CloudStatusEvent = "OUTAGE_END"
)

// cloudStatusWireValues maps this service's event names to the literals
// ServiceNow puts on the wire. Kept separate from the constants above so the
// storage enum can be renamed freely while the wire contract -- which belongs
// to the dashboard, not to us -- stays pinned.
var cloudStatusWireValues = map[CloudStatusEvent]string{
	CloudStatusEventOutageBegin: "outage_begin",
	CloudStatusEventOutageEnd:   "outage_end", // UNCONFIRMED -- see above.
}

// WireValue returns the literal to send in the webhook body.
func (e CloudStatusEvent) WireValue() string {
	if v, ok := cloudStatusWireValues[e]; ok {
		return v
	}
	return ""
}

// Valid reports whether e is a known event.
func (e CloudStatusEvent) Valid() bool {
	_, ok := cloudStatusWireValues[e]
	return ok
}

// cloudOfferingWireValues maps cloud_monitor.cloud_offering -- a Postgres
// enum, so SCREAMING_SNAKE -- to the slug the status dashboard is addressed
// by. The dashboard routes on this string, and CHOREO_EU -> "choreo-eu" is
// exactly the kind of transformation that silently produces "choreo_eu" and a
// webhook nobody receives.
//
// ALL SEVEN OFFERINGS ARE HANDLED HERE, and that is a deliberate divergence.
// ServiceNow's step-14 script had a five-way lookup -- asgardeo, bijira,
// choreo, devant, moesif -- and returned undefined for choreo-eu and
// agent-manager, posting to a URL of "undefined" with no error raised.
//
// That defect is latent rather than live in ServiceNow: all 31 monitors on
// those two offerings sit outside the 14 services the trigger filters to, so
// the flow never reaches them. It goes live the moment anyone adds a
// choreo-eu or agent-manager service to that filter -- a config change, made
// by someone who has no reason to suspect a hardcoded list exists. Covering
// all seven costs nothing and removes the trap rather than porting it.
var cloudOfferingWireValues = map[string]string{
	"ASGARDEO":      "asgardeo",
	"BIJIRA":        "bijira",
	"CHOREO":        "choreo",
	"DEVANT":        "devant",
	"MOESIF":        "moesif",
	"CHOREO_EU":     "choreo-eu",     // ServiceNow had no URL for this one.
	"AGENT_MANAGER": "agent-manager", // nor this one.
}

// CloudOfferingSlug converts a stored cloud_offering enum value to the
// dashboard's slug. It returns "" for an unknown value, which callers must
// treat as unroutable rather than posting an empty cloud.
func CloudOfferingSlug(offering string) string {
	return cloudOfferingWireValues[offering]
}

// CloudMonitorStatus is a value of cloud_monitor_status_enum -- what the
// public status dashboard renders for one monitored component.
type CloudMonitorStatus string

const (
	CloudMonitorStatusOperational   CloudMonitorStatus = "OPERATIONAL"
	CloudMonitorStatusMaintenance   CloudMonitorStatus = "MAINTENANCE"
	CloudMonitorStatusDegraded      CloudMonitorStatus = "DEGRADED"
	CloudMonitorStatusPartialOutage CloudMonitorStatus = "PARTIAL_OUTAGE"

	// CloudMonitorStatusMajorOutage exists in the enum and is NEVER written by
	// this port, because the flow never wrote it either: its script mapped the
	// three outage types to 1, 2 and 3, and 4 was unreachable. Declaring it
	// here documents that the omission is known rather than missed.
	CloudMonitorStatusMajorOutage CloudMonitorStatus = "MAJOR_OUTAGE"
)

// StatusForOngoingOutage maps an outage's type to the status its affected
// monitors should show while it is in progress.
//
// THE MAPPING IS THE FLOW'S, TRANSLATED FROM NUMBERS. ServiceNow's script
// returned the raw choice values 1, 2 and 3; the Postgres column is an enum,
// so the same decision reads as names here:
//
//	planned      -> 1 -> MAINTENANCE
//	degradation  -> 2 -> DEGRADED
//	outage       -> 3 -> PARTIAL_OUTAGE
//
// Note the last one: an outage of type "outage" shows as PARTIAL_OUTAGE, not
// MAJOR_OUTAGE. That looks like an off-by-one and is not -- it is what the
// flow did, and changing it would change what the public page says during an
// incident, which is a product decision and not a porting one.
//
// ok is false when the type is missing or unrecognised; see the caller for
// what it does about that, which is NOT what ServiceNow did.
func StatusForOngoingOutage(outageType string) (CloudMonitorStatus, bool) {
	switch strings.ToUpper(strings.TrimSpace(outageType)) {
	case "PLANNED":
		return CloudMonitorStatusMaintenance, true
	case "DEGRADATION":
		return CloudMonitorStatusDegraded, true
	case "OUTAGE":
		return CloudMonitorStatusPartialOutage, true
	default:
		return "", false
	}
}

// CloudMonitorStatusUnknownType is what an ongoing outage with no usable type
// writes instead of nothing.
//
// ServiceNow's script had no default arm: an empty type fell off the end and
// returned undefined, which the Update Record step then wrote. This port will
// not reproduce that, and the choice of what to do instead is forced by which
// way the page should fail.
//
// Skipping the write leaves the monitor showing whatever it showed before --
// for a newly declared outage that is OPERATIONAL, i.e. the public page
// asserts everything is fine in the middle of an incident. Writing DEGRADED
// says something is wrong without claiming to know how badly. On a page whose
// entire purpose is telling customers when something is broken, understating
// is recoverable and a false all-clear is not.
//
// It is logged and counted at every use, because the real fix is the outage
// record having a type.
const CloudMonitorStatusUnknownType = CloudMonitorStatusDegraded

// PendingCloudStatusWebhook is one webhook this service has decided is owed to
// the dashboard and not yet seen delivered.
//
// The cloud is resolved once, when the row is recorded, and carried here as
// stored rather than re-derived on each attempt: an outage's configuration
// item can be corrected after the fact, and a retry must repeat the original
// post rather than quietly become a different one.
type PendingCloudStatusWebhook struct {
	ID       string `json:"id"`
	OutageID string `json:"outageId"`
	// Number is carried for logging and for the operator reading a failure.
	// It is not part of the webhook body.
	Number string `json:"number"`
	// Event is this service's own name for the transition. It is carried for
	// logs, for the operator reading a failure, and for the API contract --
	// NOT for the wire. Sending it would put "OUTAGE_BEGIN" in the webhook
	// where the dashboard expects "outage_begin".
	Event CloudStatusEvent `json:"event"`

	// WireEvent is the literal the webhook body must carry.
	//
	// It exists as its own field, rather than Event being translated in
	// place, because the two are different contracts that happen to look
	// alike: Event belongs to this API and may be renamed freely; WireEvent
	// belongs to the status dashboard and may not be touched without
	// coordinating with it. Collapsing them invites a rename here from
	// silently changing what a third party receives.
	//
	// A live cross-service run is what caught this: the delivering task was
	// posting the enum name because nothing translated it, and every test
	// missed it because their fixtures already held the lowercase form.
	WireEvent string `json:"wireEvent"`

	Cloud string `json:"cloud"`
	// Timestamp is the outage's own begin or end instant -- whichever this
	// event is about -- not the moment of sending.
	//
	// ServiceNow sent `new Date()` here, i.e. send-time. That is a divergence
	// on paper and a fidelity IMPROVEMENT in practice: the flow fired
	// synchronously on the record update, so its send-time was within a second
	// of the transition and the two were interchangeable. This port decides on
	// a sweep and delivers on a later tick, so send-time could be minutes off,
	// and a retried webhook hours. Sending the instant the outage actually
	// changed keeps the dashboard agreeing with what ServiceNow DID, rather
	// than with what its code literally said.
	Timestamp    string `json:"timestamp"`
	AttemptCount int    `json:"attemptCount"`
	LastError    string `json:"lastError,omitempty"`
	// ClaimToken is the attempt this row was handed out under. Send it back
	// with the outcome (RecordCloudStatusDeliveryRequest.ClaimToken) so the
	// report is fenced to this attempt.
	ClaimToken string `json:"claimToken,omitempty"`
	// CreatedOn orders a claimed batch; not part of the API.
	CreatedOn time.Time `json:"-"`
}

// PendingCloudStatusWebhooksResponse is the body of the pending-webhook read.
//
// THE READ CLAIMS WHAT IT RETURNS. Every row comes back with an attempt already
// started under its ClaimToken, so the caller must post it and report the
// outcome; nobody else can post it meanwhile (migration 0213).
type PendingCloudStatusWebhooksResponse struct {
	Count    int                         `json:"count"`
	Webhooks []PendingCloudStatusWebhook `json:"webhooks"`
	// UnknownOutcome counts webhooks whose last attempt ended with no known
	// outcome (a timeout after sending, or a sender that never reported).
	// They are never posted again automatically -- the dashboard may already
	// have them -- and need someone to check the status page.
	UnknownOutcome int `json:"unknownOutcome"`
}

// CloudStatusSweepResponse reports what one decision sweep recorded.
type CloudStatusSweepResponse struct {
	// Scanned is how many in-scope outages the sweep considered.
	Scanned int `json:"scanned"`
	// Recorded is how many new transitions it owed the dashboard. Zero is the
	// steady state; a sweep that records nothing is working correctly.
	Recorded int `json:"recorded"`
	// SkippedNoCloud counts in-scope outages whose configuration item has no
	// cloud monitor, and which therefore cannot be routed. ServiceNow's flow
	// failed its whole execution on this; this port skips the outage and keeps
	// going, so the count is the only way to notice.
	SkippedNoCloud int `json:"skippedNoCloud"`
	// MonitorsUpdated is how many cloud_monitor rows actually changed status.
	// 0 in the steady state: the write skips monitors already showing the
	// target value, so a repeated sweep is a read and no writes.
	MonitorsUpdated int64 `json:"monitorsUpdated"`
	// UnknownOutageType counts ongoing outages whose type could not be mapped
	// and which therefore fell back to a default severity. Non-zero means an
	// outage record is missing its type and the public page is showing a
	// guess -- fix the record, not this service.
	UnknownOutageType int `json:"unknownOutageType"`
}

// RecordCloudStatusDeliveryRequest reports the outcome of one webhook attempt.
type RecordCloudStatusDeliveryRequest struct {
	ID        string `json:"-"`
	Delivered bool   `json:"delivered"`
	Error     string `json:"error,omitempty"`
	// Unknown: the request was sent and no answer came back, so the
	// dashboard may have it. The webhook is then never posted again
	// automatically. Requires delivered=false and an error.
	Unknown bool `json:"unknown,omitempty"`
	// ClaimToken is the token the webhook was handed out under; when given,
	// the report is accepted only for that attempt.
	ClaimToken string `json:"claimToken,omitempty"`
}

// ClaimCloudStatusWebhookRequest starts the attempt to post a webhook that was
// published as outage.status_page_due, under the token it carried.
type ClaimCloudStatusWebhookRequest struct {
	ID         string `json:"-"`
	ClaimToken string `json:"claimToken"`
}
