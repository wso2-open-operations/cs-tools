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

package allocationreminder

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/engagementallocations"
)

// TestLastCycleStart pins the week boundary. Every day of one week must
// resolve to the SAME previous Monday: the task normally runs Monday 00:00,
// but a retry can land any day after that, and a retry must not silently ask
// about a different cycle than the attempt it is retrying.
func TestLastCycleStart(t *testing.T) {
	// 2026-09-21 is a Monday; 2026-09-14 is the Monday before it.
	want := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name string
		now  time.Time
	}{
		{"monday, the scheduled slot", time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)},
		{"monday later in the day", time.Date(2026, 9, 21, 23, 59, 59, 0, time.UTC)},
		{"tuesday retry", time.Date(2026, 9, 22, 6, 0, 0, 0, time.UTC)},
		{"wednesday retry", time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)},
		{"thursday retry", time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)},
		{"friday retry", time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)},
		{"saturday retry", time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)},
		{"sunday retry, the last moment before the next cycle", time.Date(2026, 9, 27, 23, 59, 59, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := lastCycleStart(tc.now); !got.Equal(want) {
				t.Fatalf("lastCycleStart(%s) = %s, want %s",
					tc.now.Format(time.RFC3339), got.Format(time.DateOnly), want.Format(time.DateOnly))
			}
		})
	}

	// The following Monday must roll over to the next cycle, otherwise the
	// reminder would ask about the same week forever.
	nextMonday := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	wantNext := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	if got := lastCycleStart(nextMonday); !got.Equal(wantNext) {
		t.Fatalf("lastCycleStart(next monday) = %s, want %s",
			got.Format(time.DateOnly), wantNext.Format(time.DateOnly))
	}
}

type fakeSource struct {
	recipients []engagementallocations.Recipient
	err        error
	calls      int
	gotCycle   time.Time
}

func (f *fakeSource) StatusUpdateReminderRecipients(_ context.Context, cycleStart time.Time) ([]engagementallocations.Recipient, error) {
	f.calls++
	f.gotCycle = cycleStart
	return f.recipients, f.err
}

type sentEmail struct {
	to      []string
	cc      []string
	subject string
	body    string
}

type fakeSender struct {
	sent   []sentEmail
	failOn map[string]error
}

func (f *fakeSender) SendEmail(_ context.Context, to, cc []string, subject, htmlBody string) error {
	if len(to) == 1 {
		if err, ok := f.failOn[to[0]]; ok {
			return err
		}
	}
	f.sent = append(f.sent, sentEmail{to: to, cc: cc, subject: subject, body: htmlBody})
	return nil
}

// portalURL is what a configured deployment passes; the link it produces is
// asserted in TestSendRemindersLinksThePortal below.
const portalURL = "https://csm.example.test"

func recipients(emails ...string) []engagementallocations.Recipient {
	out := make([]engagementallocations.Recipient, 0, len(emails))
	for _, e := range emails {
		out = append(out, engagementallocations.Recipient{UserID: "id-" + e, Email: e, Name: e})
	}
	return out
}

// TestSendRemindersOneEmailPerRecipient is the behaviour that differs most
// visibly from the ServiceNow original, which put every address in one To
// line and so told everybody who else was behind.
func TestSendRemindersOneEmailPerRecipient(t *testing.T) {
	src := &fakeSource{recipients: recipients("a@wso2.com", "b@wso2.com", "c@wso2.com")}
	sender := &fakeSender{}

	if err := SendReminders(src, sender, portalURL, true)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sent) != 3 {
		t.Fatalf("sent %d emails, want 3", len(sender.sent))
	}
	for _, e := range sender.sent {
		if len(e.to) != 1 {
			t.Errorf("email addressed to %d recipients, want exactly 1: %v", len(e.to), e.to)
		}
		if len(e.cc) != 0 {
			t.Errorf("email carried a Cc (%v); nobody should be copied on someone else's reminder", e.cc)
		}
		if e.subject != subject {
			t.Errorf("subject = %q, want %q", e.subject, subject)
		}
		if !strings.Contains(e.body, "weekly update") {
			t.Errorf("body does not look like the reminder template: %.80s", e.body)
		}
	}
}

// TestSendRemindersAsksAboutLastWeek guards the wiring between the cycle
// calculation and the lookup.
func TestSendRemindersAsksAboutLastWeek(t *testing.T) {
	src := &fakeSource{}
	if err := SendReminders(src, &fakeSender{}, portalURL, true)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.calls != 1 {
		t.Fatalf("looked up recipients %d times, want 1", src.calls)
	}
	if want := lastCycleStart(time.Now().UTC()); !src.gotCycle.Equal(want) {
		t.Fatalf("asked about cycle %s, want %s",
			src.gotCycle.Format(time.DateOnly), want.Format(time.DateOnly))
	}
}

// TestSendRemindersDisabled: ALERTS_ENABLED=false must silence this the way
// it silences every other email this component sends, without failing the
// task or even querying.
func TestSendRemindersDisabled(t *testing.T) {
	src := &fakeSource{recipients: recipients("a@wso2.com")}
	sender := &fakeSender{}

	if err := SendReminders(src, sender, portalURL, false)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.calls != 0 {
		t.Errorf("queried recipients %d times while disabled, want 0", src.calls)
	}
	if len(sender.sent) != 0 {
		t.Errorf("sent %d emails while disabled, want 0", len(sender.sent))
	}
}

// TestSendRemindersNobodyOwing: an empty audience is a normal week, not a
// failure, and must not send anything.
func TestSendRemindersNobodyOwing(t *testing.T) {
	sender := &fakeSender{}
	if err := SendReminders(&fakeSource{}, sender, portalURL, true)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("sent %d emails with nobody owing, want 0", len(sender.sent))
	}
}

// TestSendRemindersPartialFailure is the one that matters for retries: one
// bad address must not cost everyone else their reminder, because the ledger
// will retry the WHOLE task and re-mail whoever already succeeded.
func TestSendRemindersPartialFailure(t *testing.T) {
	src := &fakeSource{recipients: recipients("a@wso2.com", "broken@wso2.com", "c@wso2.com")}
	sender := &fakeSender{failOn: map[string]error{"broken@wso2.com": errors.New("mailbox unavailable")}}

	err := SendReminders(src, sender, portalURL, true)(context.Background())
	if err == nil {
		t.Fatal("expected an error when a send fails, got nil")
	}
	if len(sender.sent) != 2 {
		t.Fatalf("delivered %d emails, want 2 — a single failure must not abandon the rest", len(sender.sent))
	}
	if !strings.Contains(err.Error(), "broken@wso2.com") {
		t.Errorf("error does not name the failing recipient, so the alert is not actionable: %v", err)
	}
	if !strings.Contains(err.Error(), "1 of 3") {
		t.Errorf("error does not report the failure count: %v", err)
	}
}

// TestSendRemindersLookupFailure: if the audience cannot be resolved, the task
// must fail rather than quietly send nothing — silence would look identical to
// a week where nobody owed an update.
func TestSendRemindersLookupFailure(t *testing.T) {
	src := &fakeSource{err: errors.New("entity-service unavailable")}
	sender := &fakeSender{}

	err := SendReminders(src, sender, portalURL, true)(context.Background())
	if err == nil {
		t.Fatal("expected an error when the lookup fails, got nil")
	}
	if len(sender.sent) != 0 {
		t.Errorf("sent %d emails despite a failed lookup", len(sender.sent))
	}
}

// TestSendRemindersLinksThePortal pins the navigation steps to the CSM Portal.
// The ServiceNow original told people to log into Agent Workspace and open
// "My Allocations", which they can no longer do — a regression to that wording
// would send every recipient somewhere they cannot reach.
func TestSendRemindersLinksThePortal(t *testing.T) {
	src := &fakeSource{recipients: recipients("a@wso2.com")}
	sender := &fakeSender{}

	if err := SendReminders(src, sender, portalURL, true)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("sent %d emails, want 1", len(sender.sent))
	}
	body := sender.sent[0].body
	if strings.Contains(body, "ServiceNow") {
		t.Error("body still mentions ServiceNow; recipients cannot log in there any more")
	}
	if strings.Contains(body, "My Allocations") {
		t.Error(`body still names "My Allocations", which does not exist in the CSM Portal`)
	}
	if !strings.Contains(body, portalURL+"/engagements") {
		t.Errorf("body does not link the portal engagements page (%s/engagements)", portalURL)
	}
}

// TestSendRemindersWithoutPortalURL: an unconfigured deployment must still
// send a readable email, not one with a dangling href.
func TestSendRemindersWithoutPortalURL(t *testing.T) {
	src := &fakeSource{recipients: recipients("a@wso2.com")}
	sender := &fakeSender{}

	if err := SendReminders(src, sender, "", true)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body := sender.sent[0].body
	if strings.Contains(body, "<!-- [ENGAGEMENTS_LINK] -->") {
		t.Error("placeholder left unsubstituted in the sent body")
	}
	if strings.Contains(body, "/engagements") {
		t.Error("rendered a link despite no portal URL being configured")
	}
	if !strings.Contains(body, "Engagements") {
		t.Error("the Engagements step vanished entirely without a portal URL")
	}
}
