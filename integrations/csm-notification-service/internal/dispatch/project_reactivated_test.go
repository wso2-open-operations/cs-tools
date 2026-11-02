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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package dispatch

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

const reactivatedTestProjectID = "e3e87599-1bc7-6650-182c-0dc5604bcb68"

func reactivatedRecord(reasons, recipients string) eventbus.Record {
	return eventbus.Record{Value: []byte(`{"type":"project.reactivated","entityId":"` + reactivatedTestProjectID + `","payload":{` +
		`"projectId":"` + reactivatedTestProjectID + `","projectSfId":"a0dE200000CZ9ZNIA1",` +
		`"projectName":"ACP Test Project","projectKey":"APPSUB",` +
		`"accountOwnerName":"Alex Owner","accountOwnerEmail":"owner@example.com",` +
		`"startDate":"2026-10-28","endDate":"2027-10-27",` +
		`"reasons":` + reasons + `,"recipients":` + recipients + `,"reactivatedOn":"2026-10-29T09:30:00Z"}}`)}
}

var reactivatedCC = []string{"lifecycle@example.com", "billing@example.com"}

func TestDispatcher_ProjectReactivated_SendsTheBackToOpenEmail(t *testing.T) {
	email := &mockEmailSender{}
	d := newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}).WithACPReactivation(reactivatedCC, "@example.com")
	rec := reactivatedRecord(`["renewal"]`, `["owner@example.com","tech.owner@example.com"]`)
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(email.calls) != 1 {
		t.Fatalf("emails sent = %d, want 1", len(email.calls))
	}
	got := email.calls[0]
	if got.subject != "Back to Open state : ACP Test Project" {
		t.Errorf("subject = %q", got.subject)
	}
	if !reflect.DeepEqual(got.to, []string{"owner@example.com", "tech.owner@example.com"}) || !reflect.DeepEqual(got.cc, reactivatedCC) {
		t.Errorf("to = %v cc = %v", got.to, got.cc)
	}
	if got.bcc != nil {
		t.Errorf("bcc = %v, want none", got.bcc)
	}
	for _, want := range []string{
		"Dear Alex,",
		reactivatedRenewalLine,
		"ACP Test Project", "APPSUB", "Alex Owner", "2026-10-28", "2027-10-27",
		"https://csm.example/customers/projects/" + reactivatedTestProjectID,
		"Best Regards,<br>WSO2 Team",
		"This automated message was sent by WSO2's support system.",
	} {
		if !strings.Contains(got.htmlBody, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
	if strings.Contains(got.htmlBody, "Expected sent to") {
		t.Error("a real send shows the debug \"Expected sent to\" row")
	}
}

func TestDispatcher_ProjectReactivated_ReasonLines(t *testing.T) {
	cases := map[string]string{
		`["renewal"]`:                reactivatedRenewalLine,
		`["invoice_paid"]`:           reactivatedInvoiceLine,
		`["renewal","invoice_paid"]`: reactivatedBothLine,
		`[]`:                         reactivatedOtherLine,
	}
	for reasons, line := range cases {
		email := &mockEmailSender{}
		d := newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}).WithACPReactivation(nil, "@example.com")
		if err := d.Handle(context.Background(), reactivatedRecord(reasons, `["owner@example.com"]`)); err != nil {
			t.Fatalf("%s: %v", reasons, err)
		}
		if len(email.calls) != 1 || !strings.Contains(email.calls[0].htmlBody, line) {
			t.Errorf("%s: the email does not say %q", reasons, line)
		}
	}
}

func TestDispatcher_ProjectReactivated_CCNeverRepeatsARecipient(t *testing.T) {
	email := &mockEmailSender{}
	d := newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}).
		WithACPReactivation([]string{"OWNER@example.com", "billing@example.com", "billing@example.com"}, "@example.com")
	if err := d.Handle(context.Background(), reactivatedRecord(`["renewal"]`, `["owner@example.com"]`)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(email.calls[0].cc, []string{"billing@example.com"}) {
		t.Errorf("cc = %v", email.calls[0].cc)
	}
}

func TestDispatcher_ProjectReactivated_DebugModeRedirects(t *testing.T) {
	email := &mockEmailSender{}
	d := NewDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}, &mockLinkResolver{}, true, true, []string{"debug@example.com"}, true, "", nil).
		WithACPReactivation(reactivatedCC, "@example.com")
	if err := d.Handle(context.Background(), reactivatedRecord(`["renewal"]`, `["owner@example.com"]`)); err != nil {
		t.Fatal(err)
	}
	got := email.calls[0]
	if !reflect.DeepEqual(got.to, []string{"debug@example.com"}) || got.cc != nil {
		t.Errorf("to = %v cc = %v, want the debug inbox alone", got.to, got.cc)
	}
	if !strings.Contains(got.htmlBody, "Expected sent to: owner@example.com, lifecycle@example.com, billing@example.com") {
		t.Error("the debug copy does not say who it was meant for")
	}
}

func TestDispatcher_ProjectReactivated_SendsNothing(t *testing.T) {
	t.Run("sending off", func(t *testing.T) {
		email := &mockEmailSender{}
		d := NewDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}, &mockLinkResolver{}, false, false, nil, true, "", nil)
		if err := d.Handle(context.Background(), reactivatedRecord(`["renewal"]`, `["owner@example.com"]`)); err != nil || len(email.calls) != 0 {
			t.Errorf("err = %v sent = %d", err, len(email.calls))
		}
	})
	t.Run("no recipients", func(t *testing.T) {
		email := &mockEmailSender{}
		d := newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}).WithACPReactivation(nil, "@example.com")
		if err := d.Handle(context.Background(), reactivatedRecord(`["renewal"]`, `[]`)); err != nil || len(email.calls) != 0 {
			t.Errorf("err = %v sent = %d", err, len(email.calls))
		}
	})
	t.Run("debug mode without debug recipients", func(t *testing.T) {
		email := &mockEmailSender{}
		d := NewDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}, &mockLinkResolver{}, true, true, nil, true, "", nil)
		if err := d.Handle(context.Background(), reactivatedRecord(`["renewal"]`, `["owner@example.com"]`)); err != nil || len(email.calls) != 0 {
			t.Errorf("err = %v sent = %d", err, len(email.calls))
		}
	})
}

func TestDispatcher_ProjectReactivated_SendFailureIsRetried(t *testing.T) {
	email := &mockEmailSender{err: errors.New("email service down")}
	d := newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}).WithACPReactivation(nil, "@example.com")
	rec := reactivatedRecord(`["renewal"]`, `["owner@example.com"]`)
	if err := d.Handle(context.Background(), rec); err == nil {
		t.Fatal("a failed send was not returned, so the record would not be retried")
	}
	// The retry sends it once the email service is back.
	email.err = nil
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if len(email.calls) != 2 {
		t.Errorf("send attempts = %d, want 2", len(email.calls))
	}
}

func TestGreetingFirstName(t *testing.T) {
	for in, want := range map[string]string{"Alex Owner": "Alex", "  Sam  ": "Sam", "": "Team", "  ": "Team"} {
		if got := greetingFirstName(in); got != want {
			t.Errorf("%q = %q, want %q", in, got, want)
		}
	}
}

// fakeReactivationLedger is the durable "sent" record over memory.
type fakeReactivationLedger struct {
	state    map[string]string // key -> "sending" | "sent"
	err      error
	released []string
}

func (l *fakeReactivationLedger) Reserve(_ context.Context, key string) (bool, bool, error) {
	if l.err != nil {
		return false, false, l.err
	}
	switch l.state[key] {
	case reactivationSent:
		return false, true, nil
	case reactivationSending:
		return false, false, nil
	}
	l.state[key] = reactivationSending
	return true, false, nil
}

func (l *fakeReactivationLedger) MarkSent(_ context.Context, key string) error {
	l.state[key] = reactivationSent
	return nil
}

func (l *fakeReactivationLedger) Release(_ context.Context, key string) error {
	l.released = append(l.released, key)
	if l.state[key] == reactivationSending {
		delete(l.state, key)
	}
	return nil
}

// A redelivery of a notice already sent (a restart or rebalance before the
// offset was committed) is not sent again; one being sent by another attempt
// is retried; a failed send gives the reservation back so the retry sends.
func TestDispatcher_ProjectReactivated_LedgerStopsDuplicates(t *testing.T) {
	email := &mockEmailSender{}
	ledger := &fakeReactivationLedger{state: map[string]string{}}
	d := newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}).
		WithACPReactivation(nil, "@example.com").WithACPReactivationLedger(ledger)
	rec := reactivatedRecord(`["renewal"]`, `["owner@example.com"]`)
	key := reactivatedTestProjectID + "/2026-10-29T09:30:00Z"

	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if len(email.calls) != 1 || ledger.state[key] != reactivationSent {
		t.Fatalf("sent=%d ledger=%q", len(email.calls), ledger.state[key])
	}
	// Redelivered after a restart: the in-process claim is gone, the ledger
	// still knows.
	d2 := newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}).
		WithACPReactivation(nil, "@example.com").WithACPReactivationLedger(ledger)
	if err := d2.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if len(email.calls) != 1 {
		t.Errorf("a redelivery sent the email again: sent=%d", len(email.calls))
	}

	// Being sent by another attempt: retried, not sent.
	ledger.state[key] = reactivationSending
	if err := d2.Handle(context.Background(), rec); err == nil || len(email.calls) != 1 {
		t.Errorf("err=%v sent=%d, want an error to retry and no send", err, len(email.calls))
	}

	// A failed send releases the reservation; the retry sends.
	delete(ledger.state, key)
	email.err = errors.New("email service down")
	if err := d2.Handle(context.Background(), rec); err == nil {
		t.Fatal("a failed send was not returned")
	}
	if len(ledger.released) != 1 || ledger.state[key] != "" {
		t.Errorf("released=%v state=%q, want the reservation given back", ledger.released, ledger.state[key])
	}
	email.err = nil
	if err := d2.Handle(context.Background(), rec); err != nil || ledger.state[key] != reactivationSent {
		t.Errorf("retry: err=%v state=%q", err, ledger.state[key])
	}

	// A ledger that cannot be read is retried, never a reason to send.
	before := len(email.calls)
	failing := &fakeReactivationLedger{state: map[string]string{}, err: errors.New("redis down")}
	d3 := newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}).
		WithACPReactivation(nil, "@example.com").WithACPReactivationLedger(failing)
	if err := d3.Handle(context.Background(), rec); err == nil || len(email.calls) != before {
		t.Errorf("err=%v sent=%d, want an error and no send", err, len(email.calls)-before)
	}
}

// A concurrent delivery of the same record returns nil without sending: an
// error would be retried after the winner released the claim, and send again.
func TestDispatcher_ProjectReactivated_ConcurrentDeliverySendsOnce(t *testing.T) {
	email := &mockEmailSender{block: make(chan struct{})}
	d := newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}).WithACPReactivation(nil, "@example.com")
	rec := reactivatedRecord(`["renewal"]`, `["owner@example.com"]`)
	done := make(chan error, 1)
	go func() { done <- d.Handle(context.Background(), rec) }()
	// Wait until the first call holds the claim.
	for !d.claimedForTest(recordBaseKey(rec) + "/project-reactivated") {
		runtime.Gosched()
	}
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Errorf("the losing delivery returned %v, which would be retried", err)
	}
	close(email.block)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(email.calls) != 1 {
		t.Errorf("sent=%d, want 1", len(email.calls))
	}
}

// Recipients that are not addresses, or not internal, are dropped and the
// rest still get the email; with none left the CC lists get it in To.
func TestDispatcher_ProjectReactivated_RecipientsAreFilteredNotRejected(t *testing.T) {
	email := &mockEmailSender{}
	d := newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}).WithACPReactivation(reactivatedCC, "@example.com")
	rec := reactivatedRecord(`["renewal"]`, `["owner@example.com","Tech Owner","tech@customer.example","OWNER@example.com"]`)
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if len(email.calls) != 1 || !reflect.DeepEqual(email.calls[0].to, []string{"owner@example.com"}) {
		t.Fatalf("to = %v, want only the internal, well-formed address once", email.calls)
	}

	email = &mockEmailSender{}
	d = newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{}).WithACPReactivation(reactivatedCC, "@example.com")
	if err := d.Handle(context.Background(), reactivatedRecord(`["renewal"]`, `["tech@customer.example"]`)); err != nil {
		t.Fatal(err)
	}
	if len(email.calls) != 1 || !reflect.DeepEqual(email.calls[0].to, reactivatedCC) || email.calls[0].cc != nil {
		t.Errorf("with no internal recipient the CC lists must be in To: %+v", email.calls)
	}
}
