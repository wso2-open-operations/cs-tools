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

package escalation

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// Trigger is what starts a ladder: an incident being created, or its priority
// being elevated.
type Trigger struct {
	IncidentID string
	// Number is the human-readable reference used in the voice message and
	// the execution summary, e.g. "INC0012345".
	Number string
	// WSO2CaseID is the platform's own identifier, e.g. "WSO2-1000".
	WSO2CaseID string
	// Priority keys the timing policy; accepts P-notation or severity labels.
	Priority string
	Title    string
	Account  string
	Team     string
	// Kind distinguishes the two triggers, which changes the instruction the
	// voice message gives (see VoiceMessage).
	Kind TriggerKind
	// At is when the trigger happened — every planned call is an offset from
	// this, never from plan time, so a delayed consume does not shift the
	// ladder later than the specification intends.
	At time.Time
	// Routing selects recipients per level.
	Routing RoutingContext
}

// TriggerKind is which of the two events started the ladder.
type TriggerKind string

const (
	TriggerNewIncident      TriggerKind = "New Case"
	TriggerPriorityElevated TriggerKind = "Priority Elevation"
)

// PlannedCall is one concrete call: who, when, at which level.
type PlannedCall struct {
	Level     Level
	Ordinal   int
	At        time.Time
	Recipient Recipient
	// index is this call's position in Plan.Calls, set while the summary is
	// grouping them so it can look the call up in the engine's placed flags.
	// Unexported: it is an artefact of that grouping, not part of the plan,
	// and must not reach the stored JSON where it would go stale.
	index int
}

// PlanIssue records something the plan could not do but which must not abort
// it — the specification's execution summary logs these and carries on.
type PlanIssue struct {
	Level Level
	// At is when the level would have opened. Recorded even though no call
	// came of it: the execution summary orders a level by this, which is the
	// only way a level whose every recipient was unreachable still appears.
	At     time.Time
	Reason string // e.g. "NO_NUMBER", "NO_RECIPIENTS", "RESOLVE_FAILED"
	Detail string // an email, or a resolver error
}

// Plan is a fully expanded ladder for one incident.
type Plan struct {
	Trigger Trigger
	Calls   []PlannedCall
	Issues  []PlanIssue
	// InitialWait is the priority's own pre-ladder delay (section 7.0's
	// "Initial waiting time"), kept so ExecutionSummary can report it the way
	// section 11.0's work note does.
	InitialWait time.Duration
}

// BuildPlan expands a trigger into every call the ladder would place, with
// recipients resolved once per level.
//
// Once per level, not once per attempt: a level places several calls, and a
// resolver reading a live rotation roster could legitimately answer two
// attempts of the same level differently. The specification escalates to a
// level's people, so the level is the unit that gets resolved; every attempt
// of that level then calls the same set.
//
// Level 0 is included only when the incident was reported during a rotation
// (section 3.0). A level that resolves to nobody, or whose resolver fails, is
// recorded as an issue and skipped — never fatal, because one unreachable
// level must not stop the ladder reaching the next one.
//
// A recipient with no phone number yields a NO_NUMBER issue and no call,
// matching the execution summary's own
// [LEVEL_n][ERROR][NO_NUMBER][email] line.
func BuildPlan(ctx context.Context, t Trigger, policies map[string]PriorityPolicy, r Resolver) (Plan, error) {
	policy, ok := Lookup(policies, t.Priority)
	if !ok {
		return Plan{}, fmt.Errorf("escalation: no policy for priority %q", t.Priority)
	}

	// Group the flat schedule by level, keeping levels in the order they open.
	var order []Level
	attemptsByLevel := map[Level][]Attempt{}
	for _, a := range Schedule(policy, t.Routing.HasNotificationLevel()) {
		if _, seen := attemptsByLevel[a.Level]; !seen {
			order = append(order, a.Level)
		}
		attemptsByLevel[a.Level] = append(attemptsByLevel[a.Level], a)
	}

	plan := Plan{Trigger: t, InitialWait: policy.InitialWait}
	for _, level := range order {
		attempts := attemptsByLevel[level]
		opensAt := t.At.Add(attempts[0].After)

		recipients, err := r.Resolve(ctx, level, t.Routing)
		if err != nil {
			plan.Issues = append(plan.Issues, PlanIssue{
				Level: level, At: opensAt, Reason: "RESOLVE_FAILED", Detail: err.Error(),
			})
			continue
		}
		if len(recipients) == 0 {
			plan.Issues = append(plan.Issues, PlanIssue{Level: level, At: opensAt, Reason: "NO_RECIPIENTS"})
			continue
		}

		reachable := make([]Recipient, 0, len(recipients))
		for _, rec := range recipients {
			if rec.Phone == "" {
				plan.Issues = append(plan.Issues, PlanIssue{
					Level: level, At: opensAt, Reason: "NO_NUMBER", Detail: rec.Email,
				})
				continue
			}
			reachable = append(reachable, rec)
		}

		for _, a := range attempts {
			for _, rec := range reachable {
				plan.Calls = append(plan.Calls, PlannedCall{
					Level:     level,
					Ordinal:   a.Ordinal,
					At:        t.At.Add(a.After),
					Recipient: rec,
				})
			}
		}
	}

	sort.SliceStable(plan.Calls, func(i, j int) bool { return plan.Calls[i].At.Before(plan.Calls[j].At) })
	return plan, nil
}

// orNone renders an empty routing field as "none" rather than a blank gap —
// an absent product or team is itself a section 12.0 erroneous scenario, so
// the record should say so plainly.
func orNone(v string) string {
	if v == "" {
		return "none"
	}
	return v
}

// LevelsClimbed lists the rungs this plan will actually reach, in order. It is
// the shape of the ladder for this particular incident, which differs by rule:
// a level with no reachable recipient is absent entirely, and LEVEL_0 exists
// only for some rotations.
func (p Plan) LevelsClimbed() []string {
	seen := map[Level]bool{}
	var out []string
	for _, c := range p.Calls {
		if !seen[c.Level] {
			seen[c.Level] = true
			out = append(out, c.Level.String())
		}
	}
	return out
}

// Remaining returns the calls still due at or after `from` — what a ladder
// would go on to place. Acknowledging an incident cancels exactly this set.
func (p Plan) Remaining(from time.Time) []PlannedCall {
	out := make([]PlannedCall, 0, len(p.Calls))
	for _, c := range p.Calls {
		if !c.At.Before(from) {
			out = append(out, c)
		}
	}
	return out
}

// Delivered returns the calls that would already have been placed before
// `at` — used to report what actually happened when a ladder is cancelled.
func (p Plan) Delivered(at time.Time) []PlannedCall {
	out := make([]PlannedCall, 0, len(p.Calls))
	for _, c := range p.Calls {
		if c.At.Before(at) {
			out = append(out, c)
		}
	}
	return out
}

// caseRef is the reference the voice message reads out: the platform's own
// WSO2 case id when there is one, else the incident's human-readable number.
//
// Section 10.0 reads caserecord.u_wso2_case_id, which exists on ServiceNow's
// CSM case records. Incidents in this platform carry no such field (see
// domain.IncidentView in entity-service), so the incident number is the
// closest equivalent and is what a recipient can actually search for.
func (t Trigger) caseRef() string {
	if t.WSO2CaseID != "" {
		return t.WSO2CaseID
	}
	return t.Number
}

// instruction is the closing line of the voice message, which differs by
// trigger exactly as section 10.0's template does.
//
// quoted reproduces the specification's own punctuation — it writes the state
// as 'Work In Progress' — and is right inside SSML, where the quotes sit in
// markup a speech engine parses. The plain document is not parsed: it is
// escaped into XML character data, so an apostrophe arrives as &#39; and a
// text-to-speech voice may well pronounce it. The two messages therefore
// differ by exactly these two characters, which is why this takes a flag
// rather than the callers sharing one string.
func (t Trigger) instruction(quoted bool) string {
	if t.Kind != TriggerNewIncident {
		return "Add a public comment to stop further notifications."
	}
	if quoted {
		return "Update the ticket status to 'Work In Progress' to stop further notifications."
	}
	return "Update the ticket status to Work In Progress to stop further notifications."
}

// VoiceSpeech renders section 10.0's alert as a structured SSML document, in
// the order and with the pauses the specification's own template produces.
//
// It returns a tree rather than a string on purpose. An earlier version built
// the SSML by concatenation and could not be delivered at all: Twilio's <Say>
// document is marshaled from a typed struct (correctly — it is what stops
// caller-supplied text injecting a different TwiML verb), so a markup string
// handed to it arrives escaped and is read aloud tag by tag. Handing the call
// path a tree keeps both properties at once — real nested XML on the wire, and
// text that can never become markup. See internal/notifications/ssml.go.
//
// Sentences whose only content would be an empty value are skipped rather than
// spoken as "Account - .": incidents have no account field in this platform,
// and a half-empty sentence sounds like a fault on the line.
func (t Trigger) VoiceSpeech() notifications.Speech {
	sentence := func(parts ...notifications.SpeechPart) notifications.Sentence {
		return notifications.Sentence{Parts: parts}
	}
	sentences := []notifications.Sentence{
		sentence(notifications.Say("WSO2 Support Alert.")),
		sentence(notifications.Say(fmt.Sprintf("Trigger Type - %s.", t.Kind))),
	}
	if t.Priority != "" {
		sentences = append(sentences, sentence(notifications.Say(fmt.Sprintf("Priority - %s.", t.Priority))))
	}
	if t.Account != "" {
		sentences = append(sentences, sentence(notifications.Say(fmt.Sprintf("Account - %s.", t.Account))))
	}
	if ref := t.caseRef(); ref != "" {
		sentences = append(sentences, sentence(
			notifications.Say("Case number - "),
			notifications.Pause("500ms"),
			notifications.Say(" "),
			notifications.Spell("90%", ref),
			notifications.Say(" ."),
		))
	}
	if t.Team != "" {
		sentences = append(sentences, sentence(notifications.Say(fmt.Sprintf("Team - %s.", t.Team))))
	}
	sentences = append(sentences,
		sentence(notifications.Stress("moderate", " "+t.instruction(true))),
		sentence(notifications.Pause("1s")),
	)
	return notifications.Speech{Sentences: sentences}
}

// ExecutionSummary renders the plan in section 11.0's work-note format, so a
// dry run, a live run and a cancelled run all read identically.
//
// The format is reproduced line for line from the specification's own example,
// including its two inconsistent bracket orders — the escalation-step line puts
// the level before [OK], every other line puts it after. That is how the
// documented sample reads, and anyone diffing a real work note against the
// document will expect it.
//
// failed is parallel to Plan.Calls as well: a non-empty entry is the reason a
// call was rejected by the provider and given up on. It is listed under its
// attempt as [LEVEL_n][ERROR][CALL_FAILED][email][reason] — the same shape
// section 11.0 uses for a recipient with no number, because to the person
// reading the note both mean the same thing: this rung tried and this person
// was not reached.
//
// placed is parallel to Plan.Calls and says which calls were actually dialled.
// Pass it whenever it is known — the engine always knows — and the summary
// reports only those. Passing nil falls back to "everything scheduled before
// cancelledAt", which is all a caller without an engine (cmd/ladder-harness)
// can say.
//
// The distinction is not cosmetic. A cancellation and a call due at the same
// instant race, and the cancellation wins: the wake entries are dropped before
// that tick places anything. Reporting by scheduled time alone therefore wrote
// a work note claiming a call that was retired microseconds earlier and never
// happened — an operational record has to say what was done, not what was
// planned.
//
// cancelledAt, when non-nil, truncates the summary at an acknowledgement,
// followed by a closing line counting what was dropped. reason names the
// gesture that stopped it — which matters because there are two, and which one
// fired says whether somebody changed the status or left a public comment.
// Empty defaults to "Acknowledged", the wording the documented sample uses.
func (p Plan) ExecutionSummary(placed []bool, failed []string, cancelledAt *time.Time, reason string) []string {
	const stamp = "2006-01-02 15:04:05"
	lines := []string{
		fmt.Sprintf("[%s][OK][Start : Notification Plan - %s][%s/%s]",
			p.Trigger.At.Format(stamp), p.Trigger.Kind, p.Trigger.Number, p.Trigger.WSO2CaseID),
	}
	if p.InitialWait > 0 {
		waitedUntil := p.Trigger.At.Add(p.InitialWait)
		if cancelledAt == nil || waitedUntil.Before(*cancelledAt) {
			lines = append(lines, fmt.Sprintf("[%s][OK][Initial Waiting Time: %d Minutes]",
				waitedUntil.Format(stamp), int(p.InitialWait.Minutes())))
		}
	}

	// Order the levels by when each opens, taking that time from the level's
	// issues as readily as from its calls. Driving this off p.Calls alone hid
	// a whole level whenever none of its recipients had a phone number — the
	// one case the summary's NO_NUMBER line exists to report.
	type block struct {
		level  Level
		opensA time.Time
		calls  []PlannedCall
		issues []PlanIssue
	}
	blocks := map[Level]*block{}
	var order []Level
	at := func(level Level, t time.Time) *block {
		b, ok := blocks[level]
		if !ok {
			b = &block{level: level, opensA: t}
			blocks[level] = b
			order = append(order, level)
		}
		if t.Before(b.opensA) {
			b.opensA = t
		}
		return b
	}
	for _, is := range p.Issues {
		b := at(is.Level, is.At)
		b.issues = append(b.issues, is)
	}
	for i, c := range p.Calls {
		c.index = i
		b := at(c.Level, c.At)
		b.calls = append(b.calls, c)
	}
	sort.SliceStable(order, func(i, j int) bool {
		bi, bj := blocks[order[i]], blocks[order[j]]
		if !bi.opensA.Equal(bj.opensA) {
			return bi.opensA.Before(bj.opensA)
		}
		return bi.level < bj.level
	})

	for _, level := range order {
		b := blocks[level]
		if cancelledAt != nil && !b.opensA.Before(*cancelledAt) {
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s][%s][OK][Start : Escalation Step]", b.opensA.Format(stamp), b.level))
		for _, is := range b.issues {
			lines = append(lines, fmt.Sprintf("[%s][%s][ERROR][%s][%s]",
				is.At.Format(stamp), is.Level, is.Reason, is.Detail))
		}

		// One "Notification Attempt" line per attempt, then that attempt's
		// own calls — section 11.0 groups the per-recipient call lines under
		// the attempt that placed them, not flat under the level.
		for _, ord := range attemptOrdinals(b.calls) {
			var due time.Time
			var attempted []PlannedCall
			for _, c := range b.calls {
				if c.Ordinal != ord {
					continue
				}
				if !wasPlaced(placed, c, cancelledAt) && failureOf(failed, c) == "" {
					continue
				}
				if due.IsZero() || c.At.Before(due) {
					due = c.At
				}
				attempted = append(attempted, c)
			}
			if len(attempted) == 0 {
				continue
			}
			lines = append(lines, fmt.Sprintf("[%s][OK][%s][Start : Notification Attempt]",
				due.Format(stamp), b.level))
			for _, c := range attempted {
				if why := failureOf(failed, c); why != "" {
					lines = append(lines, fmt.Sprintf("[%s][%s][ERROR][CALL_FAILED][%s][%s]",
						c.At.Format(stamp), c.Level, c.Recipient.Email, why))
					continue
				}
				lines = append(lines, fmt.Sprintf("[%s][%s][OK][Call][%s][%s]",
					c.At.Format(stamp), c.Level, c.Recipient.Email, c.Recipient.Phone))
			}
		}
	}

	if cancelledAt != nil {
		if reason == "" {
			reason = "Acknowledged"
		}
		// Counted the same way the lines above are listed, or the two
		// disagree: a call due at the very instant of the cancellation is
		// retired before it is placed, so it is neither dialled nor "still
		// scheduled", and counting by time alone lost it from both totals.
		dropped := 0
		for i := range p.Calls {
			c := p.Calls[i].withIndex(i)
			if !wasPlaced(placed, c, cancelledAt) && failureOf(failed, c) == "" {
				dropped++
			}
		}
		lines = append(lines, fmt.Sprintf("[%s][OK][%s : %d call(s) cancelled]",
			cancelledAt.Format(stamp), reason, dropped))
	}
	return lines
}

// WorkNote renders the execution summary as the work note section 11.0
// specifies, heading and all, ready to PATCH onto the incident.
func (p Plan) WorkNote(placed []bool, failed []string, cancelledAt *time.Time, reason string) string {
	const stamp = "2006-01-02 15:04:05"
	var b strings.Builder
	b.WriteString("Execution Summary Of the Escalation Flow\n\n")
	b.WriteString(fmt.Sprintf("Incident Created/Priority Updated time: %s\n",
		p.Trigger.At.Format(stamp)))
	// Which section 5.0 row selected these recipients, in the permanent
	// record rather than only in a log line that ages out. "Why did this page
	// the Americas leads and not ours" is answerable from the incident itself.
	b.WriteString(fmt.Sprintf("Notification path: %s (shift %s, product %s, team %s, ABT-eligible %s)\n\n",
		p.Trigger.Routing.Rule(),
		orNone(string(p.Trigger.Routing.Shift)),
		orNone(p.Trigger.Routing.Product),
		orNone(p.Trigger.Routing.AssignedCRETeam),
		p.Trigger.Routing.ABTEligibility()))
	b.WriteString("Execution Summary:\n\n")
	b.WriteString(strings.Join(p.ExecutionSummary(placed, failed, cancelledAt, reason), "\n"))
	return b.String()
}

// withIndex returns a copy carrying its position in Plan.Calls, so wasPlaced
// can look it up in the engine's flags.
func (c PlannedCall) withIndex(i int) PlannedCall {
	c.index = i
	return c
}

// failureOf is the recorded rejection reason for a call, or "".
func failureOf(failed []string, c PlannedCall) string {
	if c.index < len(failed) {
		return failed[c.index]
	}
	return ""
}

// wasPlaced reports whether a call should appear in the summary as dialled.
// With placed flags it is simply what the engine recorded; without them, the
// best available approximation is "scheduled before the cancellation".
func wasPlaced(placed []bool, c PlannedCall, cancelledAt *time.Time) bool {
	if placed != nil {
		return c.index < len(placed) && placed[c.index]
	}
	return cancelledAt == nil || c.At.Before(*cancelledAt)
}

// attemptOrdinals lists the distinct attempt ordinals present in calls, in
// ascending order.
func attemptOrdinals(calls []PlannedCall) []int {
	seen := map[int]bool{}
	var out []int
	for _, c := range calls {
		if !seen[c.Ordinal] {
			seen[c.Ordinal] = true
			out = append(out, c.Ordinal)
		}
	}
	sort.Ints(out)
	return out
}

// VoiceMessagePlain renders the same alert as plain sentences, with no markup
// at all. It carries exactly the information VoiceSpeech does, minus the pauses
// and prosody, and is safe to pass to MakeCall: escaping plain text changes
// nothing about how it is spoken.
//
// Kept as the fallback for a deployment that would rather not depend on SSML
// support, and as the readable form the ladder harness prints.
func (t Trigger) VoiceMessagePlain() string {
	// spacedRef reads an identifier out as separated characters, which is the
	// plain-text stand-in for the SSML version's slowed prosody: a case id
	// spoken at normal speed is the part listeners most often mishear.
	parts := []string{
		"WSO2 Support Alert.",
		fmt.Sprintf("Trigger type, %s.", t.Kind),
	}
	if t.Priority != "" {
		parts = append(parts, fmt.Sprintf("Priority, %s.", t.Priority))
	}
	if t.Account != "" {
		parts = append(parts, fmt.Sprintf("Account, %s.", t.Account))
	}
	if ref := t.caseRef(); ref != "" {
		parts = append(parts, fmt.Sprintf("Case number, %s.", spacedRef(ref)))
	}
	if t.Team != "" {
		parts = append(parts, fmt.Sprintf("Team, %s.", t.Team))
	}
	return strings.Join(append(parts, t.instruction(false)), " ")
}

// spacedRef separates a reference's characters so a text-to-speech voice reads
// it out character by character instead of trying to pronounce it as a word.
func spacedRef(ref string) string {
	var b strings.Builder
	for i, r := range ref {
		if i > 0 {
			b.WriteByte(' ')
		}
		if r == '-' {
			b.WriteString("dash")
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
