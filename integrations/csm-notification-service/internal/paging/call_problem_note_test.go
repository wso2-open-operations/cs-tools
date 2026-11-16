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

package paging

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// callNotePlan is one rung with one healthy recipient and one broken one.
func callNotePlan(broken Recipient) Plan {
	return Plan{
		Trigger: testTrigger("P1", ShiftLKMorning),
		Calls: []PlannedCall{
			{Level: Level1, Ordinal: 1, At: triggerAt, Recipient: Recipient{Email: "jane.doe@example.com", Phone: "+15550100000"}},
			{Level: Level1, Ordinal: 1, At: triggerAt, Recipient: broken},
		},
	}
}

const callProblemsHeadline = "Call problems: 1 recipient(s) could not be called (see ERROR lines)"

func TestWorkNote_EmptyNumberIsAnErrorNotOK(t *testing.T) {
	// A call+chat rung: the empty-number recipient stays in the plan, and the
	// engine used to count the call as placed.
	plan := callNotePlan(Recipient{Email: "no.phone@example.com"})
	note := plan.WorkNote([]bool{true, true}, nil, nil, "")
	t.Logf("note:\n%s", note)

	if !strings.Contains(note, callProblemsHeadline) {
		t.Errorf("missing headline:\n%s", note)
	}
	if !strings.Contains(note, "[LEVEL_1][ERROR][NO_NUMBER][no.phone@example.com]") {
		t.Errorf("missing NO_NUMBER error line:\n%s", note)
	}
	if strings.Contains(note, "[OK][Call][no.phone@example.com]") {
		t.Errorf("an empty number was reported as a placed call:\n%s", note)
	}
	if !strings.Contains(note, "[LEVEL_1][OK][Call][jane.doe@example.com][+15550100000]") {
		t.Errorf("healthy call line changed:\n%s", note)
	}
	if i, j := strings.Index(note, callProblemsHeadline), strings.Index(note, "Execution Summary:"); i < 0 || i > j {
		t.Errorf("headline must come before the summary:\n%s", note)
	}
}

func TestWorkNote_RecordedFailureReasons(t *testing.T) {
	cases := []struct{ name, reason, want string }{
		{"not E.164", PhoneInvalidFormat, "[LEVEL_1][ERROR][INVALID_PHONE_FORMAT][bad.phone@example.com]"},
		{"provider rejection", "REJECTED_400_21219", "[LEVEL_1][ERROR][CALL_FAILED][bad.phone@example.com][REJECTED_400_21219]"},
		{"no status", "CALL_UNCONFIRMED", "[LEVEL_1][ERROR][CALL_FAILED][bad.phone@example.com][CALL_UNCONFIRMED]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := callNotePlan(Recipient{Email: "bad.phone@example.com", PhoneRejected: PhoneInvalidFormat})
			note := plan.WorkNote([]bool{true, false}, []string{"", c.reason}, nil, "")
			t.Logf("note:\n%s", note)
			if !strings.Contains(note, c.want) {
				t.Errorf("missing %q:\n%s", c.want, note)
			}
			if !strings.Contains(note, callProblemsHeadline) {
				t.Errorf("missing headline:\n%s", note)
			}
		})
	}
}

func TestWorkNote_HealthyCallsUnchanged(t *testing.T) {
	plan := callNotePlan(Recipient{Email: "john.roe@example.com", Phone: "+15550100001"})
	note := plan.WorkNote([]bool{true, true}, nil, nil, "")
	if strings.Contains(note, "Call problems") || strings.Contains(note, "[ERROR]") {
		t.Errorf("a healthy note must carry no problem lines:\n%s", note)
	}
	if !strings.Contains(note, "[LEVEL_1][OK][Call][john.roe@example.com][+15550100001]") {
		t.Errorf("healthy call line missing:\n%s", note)
	}
}

func TestBuildPlan_DroppedInvalidNumberIsNamed(t *testing.T) {
	res := StaticResolver{ByLevel: map[Level][]Recipient{Level1: {
		{Email: "bad.phone@example.com", PhoneRejected: PhoneInvalidFormat},
	}}}
	plan, _ := BuildPlan(context.Background(), testTrigger("P1", ShiftLKMorning), DefaultPolicy, res, ChannelCall)
	var found bool
	for _, is := range plan.Issues {
		if is.Reason == PhoneInvalidFormat && is.Detail == "bad.phone@example.com" {
			found = true
		}
	}
	if !found {
		t.Errorf("no INVALID_PHONE_FORMAT issue: %+v", plan.Issues)
	}
}

func TestVoiceNotifier_ProblemsAreErrors(t *testing.T) {
	plan := Plan{Trigger: testTrigger("P1", ShiftLKMorning)}
	call := func(r Recipient) PlannedCall { return PlannedCall{Level: Level1, Ordinal: 1, Recipient: r} }

	t.Run("empty number", func(t *testing.T) {
		_, err := voiceNotifier{calls: &fakeCaller{}}.Deliver(context.Background(), plan, call(Recipient{Email: "no.phone@example.com"}))
		if !isPermanent(err) || permanentReason(err) != PhoneNoNumber {
			t.Errorf("want permanent NO_NUMBER, got %v", err)
		}
	})
	t.Run("dropped as not E.164", func(t *testing.T) {
		_, err := voiceNotifier{calls: &fakeCaller{}}.Deliver(context.Background(), plan, call(Recipient{PhoneRejected: PhoneInvalidFormat}))
		if permanentReason(err) != PhoneInvalidFormat {
			t.Errorf("want INVALID_PHONE_FORMAT, got %v", err)
		}
	})
	t.Run("provider error keeps its code", func(t *testing.T) {
		f := &fakeCaller{err: &apierror.Error{StatusCode: 400, Body: `{"code": 21219, "message": "x"}`}}
		_, err := voiceNotifier{calls: f}.Deliver(context.Background(), plan, call(Recipient{Phone: "+15550100000"}))
		if !isPermanent(err) || permanentReason(err) != "REJECTED_400_21219" {
			t.Errorf("want REJECTED_400_21219, got %v", err)
		}
	})
	t.Run("empty status", func(t *testing.T) {
		_, err := voiceNotifier{calls: statusCaller{}}.Deliver(context.Background(), plan, call(Recipient{Phone: "+15550100000"}))
		if permanentReason(err) != "CALL_UNCONFIRMED" {
			t.Errorf("want CALL_UNCONFIRMED, got %v", err)
		}
	})
	t.Run("healthy", func(t *testing.T) {
		d, err := voiceNotifier{calls: &fakeCaller{}}.Deliver(context.Background(), plan, call(Recipient{Phone: "+15550100000"}))
		if err != nil || d.Status != "queued" {
			t.Errorf("healthy call changed: %+v %v", d, err)
		}
	})
}

type statusCaller struct{}

func (statusCaller) MakeCall(context.Context, string, string) (notifications.Call, error) {
	return notifications.Call{}, nil
}
func (statusCaller) MakeSSMLCall(context.Context, string, notifications.Speech) (notifications.Call, error) {
	return notifications.Call{}, nil
}

func TestProfilePhoneResolver_RemembersDroppedInvalidNumber(t *testing.T) {
	ctx := context.Background()
	inner := StaticResolver{ByLevel: map[Level][]Recipient{Level0: {
		{Email: "bad.phone@example.com"}, {Email: "ok@example.com"}, {Email: "none@example.com"},
	}}}
	lookup := &fakeLookup{numbers: map[string]string{
		"bad.phone@example.com": "0770000001", "ok@example.com": "+15550100000"}}
	got, err := NewProfilePhoneResolver(inner, lookup).Resolve(ctx, Level0, RoutingContext{Shift: ShiftLK})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Recipient{}
	for _, r := range got {
		by[r.Email] = r
	}
	if r := by["bad.phone@example.com"]; r.Phone != "" || r.PhoneRejected != PhoneInvalidFormat {
		t.Errorf("dropped number not remembered: %+v", r)
	}
	if r := by["ok@example.com"]; r.Phone == "" || r.PhoneRejected != "" {
		t.Errorf("valid number mis-flagged: %+v", r)
	}
	if r := by["none@example.com"]; r.PhoneRejected != "" {
		t.Errorf("a missing number is NO_NUMBER, not invalid: %+v", r)
	}
}

func TestWorkNote_ChatOnlyRecipientWithoutNumberIsNotACallProblem(t *testing.T) {
	plan := callNotePlan(Recipient{Email: "no.phone@example.com"})
	plan.Channel = ChannelChat
	note := plan.WorkNote([]bool{true, true}, nil, nil, "")
	t.Logf("note:\n%s", note)
	if strings.Contains(note, "Call problems") || strings.Contains(note, "[ERROR]") || strings.Contains(note, "NO_NUMBER") {
		t.Errorf("a delivered chat card was reported as a call problem:\n%s", note)
	}
	if strings.Contains(note, "[OK][Call][no.phone@example.com]") {
		t.Errorf("nothing was dialled, yet a call line was printed:\n%s", note)
	}
}

func TestWorkNote_HeldCallWithoutNumberIsNotACallProblem(t *testing.T) {
	plan := callNotePlan(Recipient{Email: "no.phone@example.com"})
	plan.Channel = ChannelBoth
	plan.Calls[1].HoldCall = true
	note := plan.WorkNote([]bool{true, true}, nil, nil, "")
	if strings.Contains(note, "Call problems") || strings.Contains(note, "[ERROR]") {
		t.Errorf("a deliberately held call was reported as a problem:\n%s", note)
	}
	// The same entry not held is still a genuine problem.
	plan.Calls[1].HoldCall = false
	note = plan.WorkNote([]bool{true, true}, nil, nil, "")
	if !strings.Contains(note, callProblemsHeadline) || !strings.Contains(note, "[ERROR][NO_NUMBER][no.phone@example.com]") {
		t.Errorf("unheld empty-number call must stay a problem:\n%s", note)
	}
}

func TestWorkNote_HeadlineCountsRecipientsWithoutEmail(t *testing.T) {
	plan := Plan{
		Trigger: testTrigger("P1", ShiftLKMorning),
		Channel: ChannelCall,
		Calls: []PlannedCall{
			{Level: Level1, Ordinal: 1, At: triggerAt, Recipient: Recipient{UserID: "00000000-0000-0000-0000-000000000001"}},
			{Level: Level1, Ordinal: 1, At: triggerAt, Recipient: Recipient{UserID: "00000000-0000-0000-0000-000000000002"}},
			{Level: Level1, Ordinal: 1, At: triggerAt, Recipient: Recipient{}},
		},
	}
	note := plan.WorkNote([]bool{true, true, true}, nil, nil, "")
	if !strings.Contains(note, "Call problems: 3 recipient(s) could not be called") {
		t.Errorf("recipients without an email collapsed in the headline:\n%s", note)
	}
}

func TestWorkNote_ChatOnlyFailureIsVisibleButNotACallProblem(t *testing.T) {
	plan := callNotePlan(Recipient{Email: "no.phone@example.com"})
	plan.Channel = ChannelChat
	note := plan.WorkNote([]bool{true, false}, []string{"", "NO_CHAT_SPACE"}, nil, "")
	t.Logf("note:\n%s", note)
	if strings.Contains(note, "Call problems") {
		t.Errorf("a chat failure triggered the call-problems headline:\n%s", note)
	}
	if !strings.Contains(note, "[LEVEL_1][ERROR][CALL_FAILED][no.phone@example.com][NO_CHAT_SPACE]") {
		t.Errorf("the chat failure line must stay visible:\n%s", note)
	}
}

func TestWorkNote_ChatFailureDoesNotAddToCallProblemCount(t *testing.T) {
	// Entry 1 is a genuine call failure; entry 2 intends no call (held) and
	// only its chat part failed. Only entry 1 is counted.
	plan := callNotePlan(Recipient{Email: "held@example.com", Phone: "+15550100002"})
	plan.Channel = ChannelBoth
	plan.Calls[1].HoldCall = true
	plan.Calls = append(plan.Calls, PlannedCall{Level: Level1, Ordinal: 1, At: triggerAt,
		Recipient: Recipient{Email: "bad.call@example.com", Phone: "+15550100003"}})
	note := plan.WorkNote([]bool{true, false, false}, []string{"", "NO_CHAT_SPACE", "REJECTED_400_21219"}, nil, "")
	t.Logf("note:\n%s", note)
	if !strings.Contains(note, callProblemsHeadline) {
		t.Errorf("the genuine call failure must be counted once:\n%s", note)
	}
	if !strings.Contains(note, "[CALL_FAILED][held@example.com][NO_CHAT_SPACE]") {
		t.Errorf("the chat failure line must stay visible:\n%s", note)
	}
}

func TestWorkNote_CallPlusChatRungWithOnlyChatFailedIsNotACallProblem(t *testing.T) {
	// The call went through; the card's error is recorded nowhere as a call
	// failure (placeVia returns it as cardErr), so Failed is empty.
	plan := callNotePlan(Recipient{Email: "john.roe@example.com", Phone: "+15550100001"})
	plan.Channel = ChannelBoth
	note := plan.WorkNote([]bool{true, true}, nil, nil, "")
	if strings.Contains(note, "Call problems") || strings.Contains(note, "[ERROR]") {
		t.Errorf("a delivered call was reported as a problem:\n%s", note)
	}
}

func TestWorkNote_GenuineCallFailureStillCounts(t *testing.T) {
	plan := callNotePlan(Recipient{Email: "bad.phone@example.com", Phone: "+15550100001"})
	plan.Channel = ChannelBoth
	note := plan.WorkNote([]bool{true, false}, []string{"", "REJECTED_400_21219"}, nil, "")
	if !strings.Contains(note, callProblemsHeadline) {
		t.Errorf("a genuine call failure must be counted:\n%s", note)
	}
}
