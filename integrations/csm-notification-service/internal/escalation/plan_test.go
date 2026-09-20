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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

var triggerAt = time.Date(2026, 9, 6, 11, 0, 0, 0, time.UTC)

func testTrigger(priority string, shift Shift) Trigger {
	return Trigger{
		IncidentID: "11111111-1111-1111-1111-111111111111",
		Number:     "CS0436083",
		WSO2CaseID: "AUTOMATIONTESTSUB-1299",
		Priority:   priority,
		Title:      "Gateway returning 500s in production",
		Account:    "Automation Test Account",
		Team:       "Americas CS Team - Integraion",
		Kind:       TriggerNewIncident,
		At:         triggerAt,
		Routing:    RoutingContext{Product: "WSO2 API Manager", ABTEligible: abtYes(), AssignedCRETeam: "Atlas", Shift: shift},
	}
}

func rec(email, phone string) Recipient {
	return Recipient{Email: email, Name: email, Phone: phone}
}

func fullResolver() StaticResolver {
	return StaticResolver{ByLevel: map[Level][]Recipient{
		Level0: {rec("rota.lead@wso2.com", "+94770000000")},
		Level1: {rec("sub.lead@wso2.com", "+94770000001")},
		Level2: {rec("team.lead@wso2.com", "+94770000002")},
		Level3: {rec("bu.head@wso2.com", "+94770000003")},
		Level4: {rec("cre.head@wso2.com", "+94770000004")},
	}}
}

// The whole P1 ladder, end to end, in milliseconds rather than 44 minutes.
func TestBuildPlan_P1FullLadderDuringRotation(t *testing.T) {
	plan, err := BuildPlan(context.Background(), testTrigger("P1", ShiftLKMorning), DefaultPolicy, fullResolver())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Calls) != 14 {
		t.Fatalf("got %d calls, want 14", len(plan.Calls))
	}

	want := []struct {
		mins  int
		level Level
		email string
	}{
		{6, Level0, "rota.lead@wso2.com"},
		{7, Level0, "rota.lead@wso2.com"},
		{9, Level1, "sub.lead@wso2.com"},
		{11, Level1, "sub.lead@wso2.com"},
		{13, Level1, "sub.lead@wso2.com"},
		{18, Level2, "team.lead@wso2.com"},
		{21, Level2, "team.lead@wso2.com"},
		{24, Level2, "team.lead@wso2.com"},
		{28, Level3, "bu.head@wso2.com"},
		{31, Level3, "bu.head@wso2.com"},
		{34, Level3, "bu.head@wso2.com"},
		{38, Level4, "cre.head@wso2.com"},
		{41, Level4, "cre.head@wso2.com"},
		{44, Level4, "cre.head@wso2.com"},
	}
	for i, w := range want {
		got := plan.Calls[i]
		wantAt := triggerAt.Add(time.Duration(w.mins) * time.Minute)
		if !got.At.Equal(wantAt) || got.Level != w.level || got.Recipient.Email != w.email {
			t.Errorf("call %d: got %s at T+%v to %s; want %s at T+%dm to %s",
				i, got.Level, got.At.Sub(triggerAt), got.Recipient.Email, w.level, w.mins, w.email)
		}
	}
}

// Acknowledgement cancels every call still due, and only those.
func TestPlan_AcknowledgementCancelsRemainingCalls(t *testing.T) {
	plan, _ := BuildPlan(context.Background(), testTrigger("P1", ShiftLKMorning), DefaultPolicy, fullResolver())

	// The sub team lead picks it up 12 minutes in — after LEVEL_1's first two
	// calls, before its third.
	ack := triggerAt.Add(12 * time.Minute)
	delivered, remaining := plan.Delivered(ack), plan.Remaining(ack)

	if len(delivered) != 4 {
		t.Fatalf("delivered %d calls, want 4 (2 at LEVEL_0, 2 at LEVEL_1)", len(delivered))
	}
	if len(remaining) != 10 {
		t.Fatalf("remaining %d calls, want 10 cancelled", len(remaining))
	}
	for _, c := range remaining {
		if c.At.Before(ack) {
			t.Fatalf("call at %v is before acknowledgement and must not be cancelled", c.At)
		}
	}
	// Nobody above LEVEL_1 is ever called.
	for _, c := range delivered {
		if c.Level > Level1 {
			t.Fatalf("%s was called before acknowledgement; ladder escalated too far", c.Level)
		}
	}
}

// A recipient with no mobile number is logged and skipped — it must not stop
// the level, nor the ladder.
func TestBuildPlan_MissingPhoneIsSkippedNotFatal(t *testing.T) {
	r := StaticResolver{ByLevel: map[Level][]Recipient{
		Level1: {rec("no.number@wso2.com", ""), rec("has.number@wso2.com", "+94770000001")},
		Level2: {rec("team.lead@wso2.com", "+94770000002")},
	}}
	plan, err := BuildPlan(context.Background(), testTrigger("P0", ShiftLK), DefaultPolicy, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, c := range plan.Calls {
		if c.Recipient.Email == "no.number@wso2.com" {
			t.Fatal("a recipient with no phone number must not be called")
		}
	}
	var found bool
	for _, is := range plan.Issues {
		if is.Reason == "NO_NUMBER" && is.Detail == "no.number@wso2.com" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a NO_NUMBER issue recorded for the phoneless recipient")
	}
	// The reachable recipient at the same level is still called, and the
	// ladder still reaches LEVEL_2.
	var sawL1, sawL2 bool
	for _, c := range plan.Calls {
		switch c.Level {
		case Level1:
			sawL1 = true
		case Level2:
			sawL2 = true
		}
	}
	if !sawL1 || !sawL2 {
		t.Fatalf("ladder stalled: LEVEL_1 reached=%v LEVEL_2 reached=%v", sawL1, sawL2)
	}
}

// An entirely unresolvable level is skipped, and the next level still runs.
func TestBuildPlan_EmptyLevelDoesNotStopTheLadder(t *testing.T) {
	r := StaticResolver{ByLevel: map[Level][]Recipient{
		Level3: {rec("bu.head@wso2.com", "+94770000003")},
	}}
	plan, _ := BuildPlan(context.Background(), testTrigger("P2", ShiftLK), DefaultPolicy, r)

	if len(plan.Calls) == 0 {
		t.Fatal("expected LEVEL_3 to still be called")
	}
	for _, c := range plan.Calls {
		if c.Level != Level3 {
			t.Fatalf("unexpected call at %s", c.Level)
		}
	}
	var noRecipients int
	for _, is := range plan.Issues {
		if is.Reason == "NO_RECIPIENTS" {
			noRecipients++
		}
	}
	if noRecipients != 3 { // LEVEL_1, LEVEL_2, LEVEL_4 (LEVEL_0 not scheduled outside a rotation)
		t.Fatalf("got %d NO_RECIPIENTS issues, want 3", noRecipients)
	}
}

// Outside a rotation there is no notification level at all.
func TestBuildPlan_NoLevel0OutsideRotation(t *testing.T) {
	plan, _ := BuildPlan(context.Background(), testTrigger("P1", ShiftLK), DefaultPolicy, fullResolver())
	for _, c := range plan.Calls {
		if c.Level == Level0 {
			t.Fatal("LEVEL_0 must not be called outside a rotation")
		}
	}
	if got := plan.Calls[0]; got.Level != Level1 || !got.At.Equal(triggerAt.Add(6*time.Minute)) {
		t.Fatalf("first call = %s at T+%v, want LEVEL_1 at T+6m", got.Level, got.At.Sub(triggerAt))
	}
}

func TestBuildPlan_UnknownPriorityIsAnError(t *testing.T) {
	if _, err := BuildPlan(context.Background(), testTrigger("P9", ShiftLK), DefaultPolicy, fullResolver()); err == nil {
		t.Fatal("expected an error for an unknown priority")
	}
}

// The voice message must carry the instruction that actually stops the ladder,
// and it differs between the two triggers.
func TestVoiceSpeech_InstructionMatchesTrigger(t *testing.T) {
	newIncident := spokenText(testTrigger("P1", ShiftLK).VoiceSpeech())
	if !strings.Contains(newIncident, "Work In Progress") {
		t.Errorf("new-incident message must instruct a status change:\n%s", newIncident)
	}
	elevated := testTrigger("P1", ShiftLK)
	elevated.Kind = TriggerPriorityElevated
	if msg := spokenText(elevated.VoiceSpeech()); !strings.Contains(msg, "public comment") {
		t.Errorf("elevation message must instruct a public comment:\n%s", msg)
	}
	for _, want := range []string{"WSO2 Support Alert.", "Automation Test Account", "AUTOMATIONTESTSUB-1299"} {
		if !strings.Contains(newIncident, want) {
			t.Errorf("voice message missing %q", want)
		}
	}
}

// A sentence whose only content would be an empty value is skipped, rather
// than spoken as "Account - ." — incidents in this platform carry no account.
func TestVoiceSpeech_SkipsEmptySlots(t *testing.T) {
	tr := testTrigger("P1", ShiftLK)
	tr.Account = ""
	tr.Team = ""
	tr.WSO2CaseID = ""
	tr.Number = "INC0012345"

	spoken := spokenText(tr.VoiceSpeech())
	for _, unwanted := range []string{"Account -", "Team -"} {
		if strings.Contains(spoken, unwanted) {
			t.Errorf("expected no %q sentence when the value is empty:\n%s", unwanted, spoken)
		}
	}
	// The incident number stands in for the missing WSO2 case id.
	if !strings.Contains(spoken, "INC0012345") {
		t.Errorf("expected the incident number as the case reference:\n%s", spoken)
	}
}

// spokenText flattens a Speech into the words a listener would hear, so a test
// can assert on content without asserting on XML.
func spokenText(s notifications.Speech) string {
	var b strings.Builder
	for _, sentence := range s.Sentences {
		for _, part := range sentence.Parts {
			switch {
			case part.Prosody != nil:
				b.WriteString(part.Prosody.Text)
			case part.Emphasis != nil:
				b.WriteString(part.Emphasis.Text)
			case part.Break != nil:
			default:
				b.WriteString(part.Text)
			}
		}
		b.WriteString(" ")
	}
	return b.String()
}

// The execution summary must read like the specification's own work note.
func TestExecutionSummary_MatchesDocumentedFormat(t *testing.T) {
	plan, _ := BuildPlan(context.Background(), testTrigger("P1", ShiftLKMorning), DefaultPolicy, fullResolver())
	ack := triggerAt.Add(12 * time.Minute)
	lines := plan.ExecutionSummary(nil, nil, &ack, "")

	if !strings.HasPrefix(lines[0], "[2026-09-06 11:00:00][OK][Start : Notification Plan - New Case][CS0436083/AUTOMATIONTESTSUB-1299]") {
		t.Errorf("first line does not match the documented header:\n%s", lines[0])
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"[LEVEL_0][OK][Start : Escalation Step]",
		"[LEVEL_1][OK][Call][sub.lead@wso2.com][+94770000001]",
		"[OK][Acknowledged : 10 call(s) cancelled]",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("execution summary missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "LEVEL_2") {
		t.Errorf("summary must stop at acknowledgement, but reached LEVEL_2:\n%s", joined)
	}
}

// countingResolver records how many times each level was resolved, and can
// answer differently on a second call for the same level.
type countingResolver struct {
	inner StaticResolver
	calls map[Level]int
	drift map[Level][]Recipient // returned from the second call onward
}

func (c *countingResolver) Resolve(ctx context.Context, level Level, rc RoutingContext) ([]Recipient, error) {
	c.calls[level]++
	if c.calls[level] > 1 {
		if d, ok := c.drift[level]; ok {
			return d, nil
		}
	}
	return c.inner.Resolve(ctx, level, rc)
}

// A level places several calls but is one escalation step, so its recipients
// are resolved once. Resolving per attempt meant a resolver reading a live
// roster could hand two calls of the same level to two different people — and
// it multiplied the lookups a real resolver has to perform (14 rather than 5
// for a P1 rotation ladder).
func TestBuildPlan_ResolvesOncePerLevelNotPerAttempt(t *testing.T) {
	r := &countingResolver{
		inner: fullResolver(),
		calls: map[Level]int{},
		drift: map[Level][]Recipient{
			Level1: {rec("someone.else@wso2.com", "+94779999999")},
		},
	}
	plan, err := BuildPlan(context.Background(), testTrigger("P1", ShiftLKMorning), DefaultPolicy, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for level, n := range r.calls {
		if n != 1 {
			t.Errorf("%s resolved %d times, want 1", level, n)
		}
	}
	if len(r.calls) != 5 {
		t.Errorf("resolved %d levels, want 5 (LEVEL_0..LEVEL_4 during a rotation)", len(r.calls))
	}
	// Every LEVEL_1 attempt reached the person resolved for the level, not the
	// drifted answer a second lookup would have returned.
	var l1 int
	for _, c := range plan.Calls {
		if c.Level != Level1 {
			continue
		}
		l1++
		if c.Recipient.Email != "sub.lead@wso2.com" {
			t.Errorf("LEVEL_1 attempt %d went to %s; recipients drifted within a level", c.Ordinal, c.Recipient.Email)
		}
	}
	if l1 < 2 {
		t.Fatalf("expected LEVEL_1 to place several calls, got %d", l1)
	}
}

// A level where nobody has a phone number produces no calls, and the
// execution summary still has to report it: the whole point of the
// [LEVEL_n][ERROR][NO_NUMBER][email] line is the level nothing came of.
func TestExecutionSummary_ReportsLevelWithNoCalls(t *testing.T) {
	r := StaticResolver{ByLevel: map[Level][]Recipient{
		Level1: {rec("no.number@wso2.com", ""), rec("also.none@wso2.com", "")},
		Level2: {rec("team.lead@wso2.com", "+94770000002")},
	}}
	plan, err := BuildPlan(context.Background(), testTrigger("P0", ShiftLK), DefaultPolicy, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range plan.Calls {
		if c.Level == Level1 {
			t.Fatal("LEVEL_1 had no reachable recipient; it must place no call")
		}
	}

	summary := strings.Join(plan.ExecutionSummary(nil, nil, nil, ""), "\n")
	for _, want := range []string{
		"[LEVEL_1][OK][Start : Escalation Step]",
		"[LEVEL_1][ERROR][NO_NUMBER][no.number@wso2.com]",
		"[LEVEL_1][ERROR][NO_NUMBER][also.none@wso2.com]",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary is missing %q\n--- summary ---\n%s", want, summary)
		}
	}
	// LEVEL_1 opens before LEVEL_2, so it must be reported before it.
	if i, j := strings.Index(summary, "[LEVEL_1]"), strings.Index(summary, "[LEVEL_2]"); i > j {
		t.Errorf("LEVEL_1 reported after LEVEL_2\n--- summary ---\n%s", summary)
	}
}

// The plain rendering is what a call can actually deliver today, so it must
// carry every field the SSML one does and contain no markup at all.
func TestVoiceMessagePlain_CarriesEveryFieldAndNoMarkup(t *testing.T) {
	trig := testTrigger("P1", ShiftLKMorning)
	msg := trig.VoiceMessagePlain()

	for _, want := range []string{"WSO2 Support Alert", "New Case", "P1", "Automation Test Account", "Americas CS Team - Integraion", "Work In Progress"} {
		if !strings.Contains(msg, want) {
			t.Errorf("plain message is missing %q\n  got: %s", want, msg)
		}
	}
	for _, banned := range []string{"<", ">", "&"} {
		if strings.Contains(msg, banned) {
			t.Errorf("plain message contains markup character %q, which MakeCall would escape and speak aloud\n  got: %s", banned, msg)
		}
	}
	// The case reference is spoken character by character.
	if !strings.Contains(msg, "A U T O M A T I O N") {
		t.Errorf("case reference is not spelled out\n  got: %s", msg)
	}
	// The elevation trigger asks for a comment instead of a status change.
	elevated := trig
	elevated.Kind = TriggerPriorityElevated
	if !strings.Contains(elevated.VoiceMessagePlain(), "public comment") {
		t.Errorf("elevation instruction missing\n  got: %s", elevated.VoiceMessagePlain())
	}
}
