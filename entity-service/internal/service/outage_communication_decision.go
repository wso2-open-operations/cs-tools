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
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// *** PLACEHOLDERS FOR STAKEHOLDERS — see the PR description. ***
//
// Three values in the ServiceNow flow are wrong or unfinished, and the port
// should not carry them forward silently. Each is isolated here so it can be
// changed in one place once somebody answers.
const (
	// ServiceNow hardcodes "Choreo Outage" into the declaration subject, and
	// the flow's trigger has NO cloud filter — so an Asgardeo, Bijira,
	// Devant or Moesif outage goes out titled "Choreo Outage" today.
	//
	// QUESTION FOR STAKEHOLDERS: should the subject name the affected cloud
	// (derivable from the outage's service offering), or stay a fixed
	// product word? Until answered this keeps ServiceNow's literal so the
	// port is a port; swapping it is a one-line change.
	outageCommSubjectPrefix = "Choreo Outage"

	// ServiceNow signs both emails "[Team Name]", a template placeholder that
	// was never filled in and ships with the square brackets. The audience is
	// the SRE team and the sign-off is theirs, so the port says so; decided
	// 2026-10-03 rather than carried over.
	outageCommSignOff = "SRE"
)

// decideOutageCommunication reproduces the two arms of ServiceNow's
// `Outage Communication` flow.
//
// The original is a LONG-RUNNING flow, not a branch: it parks at a Wait For
// Condition until begin is set, sends, parks again until end is set, sends
// again, and ends. Rendered as a sweep the two waits become two guards, and
// the order still matters — an outage cannot be resolved before it has been
// declared, because ServiceNow's resolution arm is only reachable through
// the declaration arm.
func decideOutageCommunication(o domain.OutageForCommunication) domain.OutageCommunicationDecision {
	d := domain.OutageCommunicationDecision{
		OutageID: o.OutageID,
		Number:   o.Number,
		Kind:     domain.OutageCommunicationNone,
	}

	// Step 3's wait. Both clauses, from the STORED encoded query
	// (beginISNOTEMPTY^u_outage_communication=true) rather than from the
	// panel, which renders only the first and leaves an empty second row.
	if !o.OptedIn || o.StartOn == nil {
		d.Reason = "not opted in, or not started"
		return d
	}

	ended := o.EndOn != nil

	switch {
	case !o.AlreadyDeclared:
		// *** STEP 4 EXCLUDES TWO OF THE THREE OUTAGE TYPES. ***
		// Its condition is begin NOT EMPTY ^ type=outage ^ end EMPTY ^
		// flag=true. A DEGRADATION or PLANNED outage clears the wait at
		// step 3 and then fails here, so the ServiceNow instance parks
		// forever and NOTHING is ever sent for it. Recovered from the
		// stored condition and confirmed against the panel.
		//
		// Reproduced deliberately. Widening it would mail on degradations
		// ServiceNow has never mailed on, which is a product change wearing
		// a port's clothes.
		if !strings.EqualFold(o.Type, "OUTAGE") {
			d.Reason = "declaration is type=outage only; this is " + strings.ToLower(o.Type)
			return d
		}
		// And `end is empty`. An outage that arrived already finished is
		// never declared — ServiceNow's step 4 would not match it either.
		if ended {
			d.Reason = "already ended before it was ever declared"
			return d
		}
		d.Kind = domain.OutageCommunicationDeclared
		d.Reason = "Declare Outage"
		d.Subject, d.Body = renderOutageDeclaration(o)
		return d

	case ended && !o.AlreadyResolved:
		// Step 11: `End is not empty`, and nothing else — no type clause and
		// no flag. It is narrower in practice only because it is reachable
		// solely through step 4.
		d.Kind = domain.OutageCommunicationResolved
		d.Reason = "End date is available"
		d.Subject, d.Body = renderOutageResolution(o)
		return d

	default:
		// Declared and still running, or fully resolved. ServiceNow's
		// instance is parked at step 10 or has ended; either way nothing is
		// owed. *** NOTE THERE IS NO UPDATE ARM. *** The "Update" rows in
		// the communication log (155 of them, its largest bucket) belong to
		// `Outage update (Outage table)`, a different flow.
		d.Reason = "nothing owed"
		return d
	}
}

// renderOutageDeclaration is step 5's email.
//
// The body is derived fields plus fixed prose and carries NO outage message
// text — which is why step 6 logs the rendered content separately rather
// than pointing at a field on the outage.
func renderOutageDeclaration(o domain.OutageForCommunication) (subject, body string) {
	subject = fmt.Sprintf("%s %s on %s", outageCommSubjectPrefix, o.Number, o.ShortDescription)

	var b strings.Builder
	b.WriteString("Hello Team,\n\n")
	// ServiceNow interpolates the raw enum here, so a degradation would read
	// "We want to inform you of a degradation" — moot in practice, since the
	// declaration arm only ever runs for type=outage.
	fmt.Fprintf(&b, "We want to inform you of a %s currently affecting our services. "+
		"Please find the key details below:\n\n", strings.ToLower(o.Type))
	b.WriteString("Outage Summary:\n")
	// "Incident Number" is ServiceNow's label and it is bound to the
	// OUTAGE's number, not an incident's. The label is wrong; the binding is
	// right. Kept verbatim so the two systems' mails read identically.
	fmt.Fprintf(&b, "  Incident Number: %s\n", o.Number)
	fmt.Fprintf(&b, "  Title : %s\n", o.ShortDescription)
	fmt.Fprintf(&b, "  Start Time: %s\n", formatOutageInstant(o.StartOn))
	fmt.Fprintf(&b, "  Impact: %s\n", o.Impact)
	fmt.Fprintf(&b, "  Current Status: %s\n", o.State)
	fmt.Fprintf(&b, "  Description: %s\n\n", o.ShortDescription)
	b.WriteString("Next Steps: Our team is prioritizing resolution efforts and will keep you updated with progress.\n\n")
	b.WriteString("Thank you for your support and cooperation as we work to restore services as quickly as possible.\n\n")
	b.WriteString("Best regards,\n")
	b.WriteString(outageCommSignOff + "\n")
	return subject, b.String()
}

// renderOutageResolution is step 12's email.
//
// *** THE SUBJECT IS THE DECLARATION'S, REUSED. *** ServiceNow ferries it
// through a field on the incident purely so the two mails thread: step 8
// writes the subject there, step 12 reads it back. The port keeps the
// behaviour and drops the mechanism — the subject is already on the
// declaration's log row, so no incident column and no incident write are
// needed. If that row is somehow missing, fall back to re-deriving it
// rather than sending a blank subject.
func renderOutageResolution(o domain.OutageForCommunication) (subject, body string) {
	subject = o.DeclaredSubject
	if subject == "" {
		subject = fmt.Sprintf("%s %s on %s", outageCommSubjectPrefix, o.Number, o.ShortDescription)
	}

	var b strings.Builder
	b.WriteString("Hi Team,\n\n")
	b.WriteString("We're pleased to inform you that the outage has been fully resolved. Here are the final details:\n\n")
	b.WriteString("Resolution Summary:\n")
	fmt.Fprintf(&b, "  Incident Number : %s\n", o.Number)
	fmt.Fprintf(&b, "  Title : %s\n", o.ShortDescription)
	fmt.Fprintf(&b, "  Outage Duration: %s\n", formatOutageDuration(o.DurationSeconds))
	fmt.Fprintf(&b, "  Start Time: %s\n", formatOutageInstant(o.StartOn))
	fmt.Fprintf(&b, "  End Time: %s\n", formatOutageInstant(o.EndOn))
	fmt.Fprintf(&b, "  Overall Impact: %s\n", o.Impact)
	fmt.Fprintf(&b, "  Description: %s\n\n", o.ShortDescription)
	// *** THIS SENTENCE PROMISES SOMETHING NOTHING DELIVERS. *** No step in
	// this flow sends an RCA; the RCA notifier is a sibling flow that is
	// INACTIVE. The communication log does hold 13 "RCA" rows, so it ran at
	// some point and does not now.
	//
	// QUESTION FOR STAKEHOLDERS: keep the promise and port the RCA flow, or
	// drop the sentence? Kept verbatim for now so the port matches what is
	// being sent today.
	b.WriteString("We are currently conducting a Root Cause Analysis (RCA) to identify the factors " +
		"behind this incident and implement measures to prevent future occurrences. " +
		"The RCA findings will be shared with the team as soon as they are available.\n\n")
	b.WriteString("Thank you for your patience and cooperation throughout this incident.\n\n")
	b.WriteString("Best regards,\n")
	b.WriteString(outageCommSignOff + "\n")
	return subject, b.String()
}

// formatOutageInstant renders a timestamp for the email body.
//
// *** THE FORMAT IS A DIVERGENCE, AND A DELIBERATE ONE. *** ServiceNow
// interpolates a glide_date_time pill, which renders in the INSTANCE's date
// format and the CALLING USER's timezone — so the same outage reads
// differently depending on who the flow ran as. That is not reproducible
// from Go and is not worth reproducing: these mails go to one group, and a
// stable, unambiguous instant is strictly better than a locale-dependent
// one. RFC 3339 in UTC, stated here rather than discovered later.
//
// Empty rather than "0001-01-01" for a nil: the resolution mail renders
// End Time, and an outage can reach it with the field unset if the row
// changed under the sweep.
func formatOutageInstant(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
