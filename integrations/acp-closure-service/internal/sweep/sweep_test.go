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

package sweep

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/closure"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/notify"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/recipients"
)

func TestProcessProject_NoEndDateIsNoOp(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: nil}

	err := processProject(context.Background(), reader, updater, ntf, time.Now(), proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0", len(ntf.sent))
	}
}

// TestProcessProject_InternalOnlyWindowSkipsCustomerContactLookup covers a
// 90-day window: internal-only per the confirmed audience matrix. Only one
// notify.Send should occur, Recipients.Customer must stay nil, and no
// contact-search calls should happen at all, since the customer side isn't
// consulted for this window.
func TestProcessProject_InternalOnlyWindowSkipsCustomerContactLookup(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			t.Fatal("SearchProjectContacts should not be called for an internal-only window")
			return nil, nil
		},
		searchAccountContactsFn: func(ctx context.Context, accountID string, body []byte) ([]byte, error) {
			t.Fatal("SearchAccountContacts should not be called for an internal-only window")
			return nil, nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89) // fires the 90-day window
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 1 {
		t.Fatalf("ntf.sent = %d, want 1", len(ntf.sent))
	}
	if ntf.sent[0].Recipients.Customer != nil {
		t.Errorf("Recipients.Customer = %v, want nil for an internal-only window", ntf.sent[0].Recipients.Customer)
	}

	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1", len(updater.calls))
	}
	var body struct {
		SuspensionProcessState struct {
			BasedOnSubscriptionEndDate struct {
				EventType string `json:"event_type"`
			} `json:"based_on_subscription_end_date"`
		} `json:"suspensionProcessState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &body); err != nil {
		t.Fatalf("parse update body: %v", err)
	}
	if got := body.SuspensionProcessState.BasedOnSubscriptionEndDate.EventType; got != "90_days_notice" {
		t.Errorf("event_type = %q, want %q", got, "90_days_notice")
	}
}

// TestProcessProject_RecordsIgnoredWhenNotifierDoesNotDeliver verifies that
// recordNoticeSent writes "IGNORED" rather than "SUCCESSFUL" when the
// notifier in use doesn't actually deliver notices (as LoggingNotifier
// never does) — a real Send succeeding is not the same fact as a real email
// having been sent, and the recorded state must not claim otherwise.
func TestProcessProject_RecordsIgnoredWhenNotifierDoesNotDeliver(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89) // fires the 90-day window
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1", len(updater.calls))
	}
	var body struct {
		SuspensionProcessState struct {
			BasedOnSubscriptionEndDate struct {
				ActionSendEmailNotification string `json:"actionSendEmailNotification"`
			} `json:"based_on_subscription_end_date"`
		} `json:"suspensionProcessState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &body); err != nil {
		t.Fatalf("parse update body: %v", err)
	}
	if got := body.SuspensionProcessState.BasedOnSubscriptionEndDate.ActionSendEmailNotification; got != "IGNORED" {
		t.Errorf("actionSendEmailNotification = %q, want %q", got, "IGNORED")
	}
}

// TestProcessProject_CustomerAudienceWindowNotifiesBusinessContact covers a
// 7-day window: both internal and customer per the confirmed audience
// matrix. A project contact with the business-contact role should produce
// TWO separate notices — internal (Customer nil) and customer-facing
// (Customer populated) — since their subject/body genuinely differ, not one
// notice bundling both.
func TestProcessProject_CustomerAudienceWindowNotifiesBusinessContact(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			return []byte(`{"contacts":[{"name":"Bob","email":"bob@customer.example","roles":["business_contact"]}]}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 6) // fires the 7-day window
	proj := project{ID: "p1", Name: "Acme - Subscription", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 2 {
		t.Fatalf("ntf.sent = %d, want 2 (internal + customer)", len(ntf.sent))
	}

	internal, customer := ntf.sent[0], ntf.sent[1]

	if internal.Recipients.Customer != nil {
		t.Errorf("internal notice Recipients.Customer = %v, want nil", internal.Recipients.Customer)
	}
	// The fixture project has an account with no Name set, so the subject
	// correctly omits the " of {AccountName}" clause entirely (regression
	// coverage for the dangling "of " bug lives in TestInternalNoticeSubject).
	const wantInternalSubject = "[ACP] 7 Days Reminder of Project for Acme - Subscription"
	if internal.Subject != wantInternalSubject {
		t.Errorf("internal Subject = %q, want %q", internal.Subject, wantInternalSubject)
	}

	if customer.Recipients.Customer == nil {
		t.Fatal("customer notice Recipients.Customer = nil, want populated")
	}
	if customer.Recipients.Customer.Email != "bob@customer.example" {
		t.Errorf("customer Recipients.Customer.Email = %q, want %q", customer.Recipients.Customer.Email, "bob@customer.example")
	}
	const wantCustomerSubject = "Upcoming Project Suspension Notice - Acme - Subscription"
	if customer.Subject != wantCustomerSubject {
		t.Errorf("customer Subject = %q, want %q", customer.Subject, wantCustomerSubject)
	}
	if customer.ResolvedVia != recipients.ResolvedViaBusinessContact {
		t.Errorf("ResolvedVia = %q, want %q", customer.ResolvedVia, recipients.ResolvedViaBusinessContact)
	}
	if strings.Contains(customer.Body, "Dear ") {
		t.Errorf("customer notice Body has a greeting, want none:\n%s", customer.Body)
	}

	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1", len(updater.calls))
	}
}

// TestProcessProject_RecordsIgnoredWhenOnlyCustomerNoticeWasntDelivered
// covers the exact CodeRabbit-flagged bug: a 7-day window where the
// internal notice genuinely delivers but the customer notice doesn't
// (e.g. filtered out by EmailNotifier's WSO2-only staging safeguard).
// recordNoticeSent must record "IGNORED" for the whole window, not
// "SUCCESSFUL" — the old behavior (keyed off a blanket per-notifier
// Delivers() signal) would have wrongly written "SUCCESSFUL" here, since
// EmailNotifier is, in general, a real-sending notifier, even though this
// specific customer notice never reached anyone.
func TestProcessProject_RecordsIgnoredWhenOnlyCustomerNoticeWasntDelivered(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			return []byte(`{"contacts":[{"name":"Bob","email":"bob@customer.example","roles":["business_contact"]}]}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{
		sendFn: func(ctx context.Context, n notify.Notice) (bool, error) {
			// Internal notice (no Customer) delivers; customer notice
			// (Customer populated) gets filtered out — mirrors a real
			// non-WSO2 customer address in staging.
			return n.Recipients.Customer == nil, nil
		},
	}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 6) // fires the 7-day window
	proj := project{ID: "p1", Name: "Acme - Subscription", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(ntf.sent) != 2 {
		t.Fatalf("ntf.sent = %d, want 2 (internal + customer)", len(ntf.sent))
	}

	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1", len(updater.calls))
	}
	var body struct {
		SuspensionProcessState struct {
			BasedOnSubscriptionEndDate struct {
				ActionSendEmailNotification string `json:"actionSendEmailNotification"`
			} `json:"based_on_subscription_end_date"`
		} `json:"suspensionProcessState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &body); err != nil {
		t.Fatalf("parse update body: %v", err)
	}
	if got := body.SuspensionProcessState.BasedOnSubscriptionEndDate.ActionSendEmailNotification; got != "IGNORED" {
		t.Errorf("actionSendEmailNotification = %q, want %q (customer notice wasn't actually delivered)", got, "IGNORED")
	}
}

// TestProcessProject_CustomerAudienceWindowSendsNoBusinessContactNoticeWhenNoContactFound
// covers the three-tier fallback's last resort: no business contact, no
// primary contact. Both the internal notice (Customer nil — nothing
// resolved) and a separate no-business-contact urgent notice — sent to all
// three internal recipients (Account Owner, Renewal Manager, Technical
// Owner), not the Account Owner alone — must be sent.
func TestProcessProject_CustomerAudienceWindowSendsNoBusinessContactNoticeWhenNoContactFound(t *testing.T) {
	reader := &mockEntityReader{
		getAccountFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{
				"accountManager": {"id": "am-1", "name": "Jordan Perera", "email": "jordan.perera@wso2.example"},
				"renewalAccountManager": {"id": "ram-1", "name": "Sam Jayasuriya", "email": "sam.jayasuriya@wso2.example"},
				"technicalOwner": {"id": "tech-1", "name": "Alex Fernando", "email": "alex.fernando@wso2.example"}
			}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 6) // fires the 7-day window
	startDate := now.AddDate(-1, 0, 0)
	proj := project{
		ID:         "p1",
		Name:       "HFC Subscription - Subscription",
		ProjectKey: "HFCSUBS",
		Account:    &projectAccountRef{ID: "a1"},
		StartDate:  &startDate,
		EndDate:    &endDate,
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 2 {
		t.Fatalf("ntf.sent = %d, want 2 (internal + no-business-contact)", len(ntf.sent))
	}

	internal, nudge := ntf.sent[0], ntf.sent[1]

	if internal.Recipients.Customer != nil {
		t.Errorf("internal Recipients.Customer = %v, want nil", internal.Recipients.Customer)
	}
	if internal.Recipients.AccountOwner.Email != "jordan.perera@wso2.example" {
		t.Errorf("internal Recipients.AccountOwner.Email = %q, want %q", internal.Recipients.AccountOwner.Email, "jordan.perera@wso2.example")
	}

	const wantSubject = "[Urgent] [ACP] No Business Contacts Specified for Project HFC Subscription - Subscription"
	if nudge.Subject != wantSubject {
		t.Errorf("nudge Subject = %q, want %q", nudge.Subject, wantSubject)
	}
	if nudge.Recipients.AccountOwner.Email != "jordan.perera@wso2.example" {
		t.Errorf("nudge Recipients.AccountOwner.Email = %q, want %q", nudge.Recipients.AccountOwner.Email, "jordan.perera@wso2.example")
	}
	if nudge.Recipients.RenewalManager.Email != "sam.jayasuriya@wso2.example" {
		t.Errorf("nudge Recipients.RenewalManager.Email = %q, want %q", nudge.Recipients.RenewalManager.Email, "sam.jayasuriya@wso2.example")
	}
	if nudge.Recipients.TechnicalOwner.Email != "alex.fernando@wso2.example" {
		t.Errorf("nudge Recipients.TechnicalOwner.Email = %q, want %q", nudge.Recipients.TechnicalOwner.Email, "alex.fernando@wso2.example")
	}
	if nudge.Recipients.Customer != nil {
		t.Errorf("nudge Recipients.Customer = %v, want nil", nudge.Recipients.Customer)
	}
	if nudge.Body == "" {
		t.Error("nudge Body is empty, want the no-business-contact template populated")
	}
	wantBodyContains := []string{
		"Project Name: HFC Subscription - Subscription",
		"Project Key: HFCSUBS",
		"Account Owner: Jordan Perera",
	}
	for _, want := range wantBodyContains {
		if !strings.Contains(nudge.Body, want) {
			t.Errorf("nudge Body missing %q, got:\n%s", want, nudge.Body)
		}
	}
}

// TestProcessProject_CustomerAudienceWindowSkipsAccountContactLookupWhenNoAccount
// verifies that a project with no linked account never calls
// SearchAccountContacts. Calling it anyway with an empty account ID would hit
// SearchAccountContacts(ctx, "", ...), which fetchContacts must not do when
// there's no account to search — it should fall through to the
// no-business-contact notice exactly as it does when a real account search
// simply returns no contacts.
func TestProcessProject_CustomerAudienceWindowSkipsAccountContactLookupWhenNoAccount(t *testing.T) {
	reader := &mockEntityReader{
		searchAccountContactsFn: func(ctx context.Context, accountID string, body []byte) ([]byte, error) {
			t.Fatal("SearchAccountContacts should not be called when the project has no linked account")
			return nil, nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 6) // fires the 7-day window
	proj := project{ID: "p1", Account: nil, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 2 {
		t.Fatalf("ntf.sent = %d, want 2 (reminder + no-business-contact)", len(ntf.sent))
	}
	if ntf.sent[0].Recipients.Customer != nil {
		t.Errorf("reminder Recipients.Customer = %v, want nil (no account linked)", ntf.sent[0].Recipients.Customer)
	}
}

// TestProcessProject_CustomerAudienceWindowFetchContactsFailureSendsNothing
// is a regression test for PR #1440 (Sajith Ekanayake): a transient
// fetchContacts failure must not leave the internal notice already sent
// with no corresponding suspensionProcessState record — that combination
// causes a duplicate internal-notice resend on the next sweep, since the
// window still looks "not yet notified". Contact resolution must happen
// before any notice sends for a customer-audience window, exactly as it
// did before this diff, so a fetch failure here sends zero notices.
func TestProcessProject_CustomerAudienceWindowFetchContactsFailureSendsNothing(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			return nil, errors.New("transient network error")
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 6) // fires the 7-day window (customer-audience)
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err == nil {
		t.Fatal("processProject() error = nil, want non-nil")
	}
	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0 (internal notice must not send before contacts are resolved)", len(ntf.sent))
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
}

// TestProcessProject_NotifyFailureBlocksStateWrite verifies that when
// notify.Send fails, no suspensionProcessState write happens — leaving
// lastNoticeWindow unchanged so the same window is retried on the next run,
// with no separate FAILED-marker bookkeeping needed.
func TestProcessProject_NotifyFailureBlocksStateWrite(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{
		sendFn: func(ctx context.Context, n notify.Notice) (bool, error) {
			return false, errors.New("smtp relay unreachable")
		},
	}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89) // fires the 90-day window (internal-only)
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err == nil {
		t.Fatal("processProject() error = nil, want non-nil")
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
}

// TestProcessProject_Day0SuccessfulNotifyThenSuspend verifies the day-0
// ordering: when the final notice email succeeds, suspend is attempted
// afterward — two separate UpdateProject calls, notice-state first.
func TestProcessProject_Day0SuccessfulNotifyThenSuspend(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			return []byte(`{"contacts":[{"name":"Bob","email":"bob@customer.example","roles":["business_contact"]}]}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3) // 3 days past due
	open := "Open"
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate, ClosureState: &open}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 2 {
		t.Fatalf("ntf.sent = %d, want 2 (internal + customer, Customer resolved)", len(ntf.sent))
	}
	if !strings.Contains(ntf.sent[0].Subject, "Project Suspension Notice") {
		t.Errorf("internal Subject = %q, want day-0 suspension wording", ntf.sent[0].Subject)
	}

	if len(updater.calls) != 2 {
		t.Fatalf("updater.calls = %d, want 2 (record notice, then suspend)", len(updater.calls))
	}

	var noticeBody struct {
		SuspensionProcessState struct {
			BasedOnSubscriptionEndDate struct {
				EventType string `json:"event_type"`
			} `json:"based_on_subscription_end_date"`
		} `json:"suspensionProcessState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &noticeBody); err != nil {
		t.Fatalf("parse first update body: %v", err)
	}
	if got := noticeBody.SuspensionProcessState.BasedOnSubscriptionEndDate.EventType; got != "suspend" {
		t.Errorf("first call event_type = %q, want %q", got, "suspend")
	}

	var suspendBody struct {
		EndDateClosureState string `json:"endDateClosureState"`
	}
	if err := json.Unmarshal(updater.calls[1].body, &suspendBody); err != nil {
		t.Fatalf("parse second update body: %v", err)
	}
	if suspendBody.EndDateClosureState != "Suspended" {
		t.Errorf("second call endDateClosureState = %q, want %q", suspendBody.EndDateClosureState, "Suspended")
	}
}

// TestProcessProject_Day0RetrySkipsNotifyWhenAlreadyRecorded covers the
// retry case: a prior run already recorded the terminal marker (email
// done), but suspend itself previously failed. This run should skip notify
// entirely and go straight to suspend.
func TestProcessProject_Day0RetrySkipsNotifyWhenAlreadyRecorded(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			t.Fatal("SearchProjectContacts should not be called when notify is already recorded")
			return nil, nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3)
	open := "Open"
	proj := project{
		ID:                     "p1",
		Account:                &projectAccountRef{ID: "a1"},
		EndDate:                &endDate,
		ClosureState:           &open,
		SuspensionProcessState: []byte(`{"based_on_subscription_end_date":{"event_type":"suspend"}}`),
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0 (notify already recorded)", len(ntf.sent))
	}
	if len(updater.calls) != 1 {
		t.Fatalf("updater.calls = %d, want 1 (suspend only)", len(updater.calls))
	}
	var suspendBody struct {
		EndDateClosureState string `json:"endDateClosureState"`
	}
	if err := json.Unmarshal(updater.calls[0].body, &suspendBody); err != nil {
		t.Fatalf("parse update body: %v", err)
	}
	if suspendBody.EndDateClosureState != "Suspended" {
		t.Errorf("endDateClosureState = %q, want %q", suspendBody.EndDateClosureState, "Suspended")
	}
}

// TestProcessProject_SuspendGuardSkipsAlreadySuspendedProject verifies the
// suspend guard: if endDateClosureState already reads "Suspended" (from this
// run's already-fetched data), no UpdateProject call happens at all for
// suspend. This is the field suspend() itself writes — closureState is a
// separate, derived roll-up field this code never sets, so the guard must
// not be keyed on it (confirmed bug: see EndDateClosureState's doc comment
// in types.go).
func TestProcessProject_SuspendGuardSkipsAlreadySuspendedProject(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3)
	suspended := "Suspended"
	proj := project{
		ID:                     "p1",
		Account:                &projectAccountRef{ID: "a1"},
		EndDate:                &endDate,
		EndDateClosureState:    &suspended,
		SuspensionProcessState: []byte(`{"based_on_subscription_end_date":{"event_type":"suspend"}}`),
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0 (already suspended, no-op)", len(updater.calls))
	}
}

// TestProcessProject_SuspendGuardSkipsWhenEndDateClosureStateIsClosed
// verifies the guard also treats "Closed" as already-handled, not just an
// exact "Suspended" match. Confirmed via a real suspended project fetched
// directly (Postman, project acac149b-eba1-4714-fcf5-f5dabad0cdb1,
// closureState="Suspended" but endDateClosureState="Closed") that this field
// can progress past "Suspended" via a process outside this component — an
// equality check against "Suspended" alone would miss this real case and
// re-suspend indefinitely.
func TestProcessProject_SuspendGuardSkipsWhenEndDateClosureStateIsClosed(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3)
	closed := "Closed"
	proj := project{
		ID:                     "p1",
		Account:                &projectAccountRef{ID: "a1"},
		EndDate:                &endDate,
		EndDateClosureState:    &closed,
		SuspensionProcessState: []byte(`{"based_on_subscription_end_date":{"event_type":"suspend"}}`),
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0 (already closed, no-op)", len(updater.calls))
	}
}

// TestProcessProject_SuspendProceedsWhenEndDateClosureStateIsExplicitlyOpen
// covers the explicit (not just nil) "Open" case — confirmed via real data
// that sibling closure-state dimensions come back as an explicit "Open"
// string, not a null, for an untouched project. This guards against a
// future casing/logic slip in the inequality check landing on the wrong
// side for this specific, real value.
func TestProcessProject_SuspendProceedsWhenEndDateClosureStateIsExplicitlyOpen(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectContactsFn: func(ctx context.Context, projectID string, body []byte) ([]byte, error) {
			return []byte(`{"contacts":[{"name":"Bob","email":"bob@customer.example","roles":["business_contact"]}]}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, -3) // 3 days past due
	open := "Open"
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate, EndDateClosureState: &open}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 2 {
		t.Errorf("updater.calls = %d, want 2 (record notice, then suspend)", len(updater.calls))
	}
}

func TestProcessProject_NothingDueIsNoOp(t *testing.T) {
	reader := &mockEntityReader{}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 200) // far beyond the 90-day window
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(updater.calls) != 0 {
		t.Errorf("updater.calls = %d, want 0", len(updater.calls))
	}
	if len(ntf.sent) != 0 {
		t.Errorf("ntf.sent = %d, want 0", len(ntf.sent))
	}
}

// TestProcessProject_ReminderUsesRealAccountContacts is a regression test
// using the real GetAccount response shape confirmed via direct Postman
// testing against the dedicated test account: a populated accountManager,
// technicalOwner, and renewalAccountManager, all with real emails. The
// reminder notice's Recipients must carry all three, and GetAccount must be
// called with the project's account ID.
func TestProcessProject_ReminderUsesRealAccountContacts(t *testing.T) {
	const realGetAccountResponse = `{
		"id": "f213fdd1-1b4b-a650-a002-c9d3604bcbac",
		"name": "ACP Test Partner Account",
		"technicalOwner": {
			"id": "tech-1",
			"name": "Alex Fernando",
			"email": "alex.fernando@wso2.example"
		},
		"accountManager": {
			"id": "am-1",
			"name": "Jordan Perera",
			"email": "jordan.perera@wso2.example"
		},
		"renewalAccountManager": {
			"id": "ram-1",
			"name": "Sam Jayasuriya",
			"email": "sam.jayasuriya@wso2.example"
		}
	}`

	var gotAccountID string
	reader := &mockEntityReader{
		getAccountFn: func(ctx context.Context, id string) ([]byte, error) {
			gotAccountID = id
			return []byte(realGetAccountResponse), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89) // fires the 90-day (internal-only) window
	proj := project{
		ID:      "p1",
		Account: &projectAccountRef{ID: "f213fdd1-1b4b-a650-a002-c9d3604bcbac"},
		EndDate: &endDate,
	}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}

	if gotAccountID != "f213fdd1-1b4b-a650-a002-c9d3604bcbac" {
		t.Errorf("GetAccount called with id = %q, want the project's account ID", gotAccountID)
	}
	if len(ntf.sent) != 1 {
		t.Fatalf("ntf.sent = %d, want 1", len(ntf.sent))
	}
	got := ntf.sent[0].Recipients
	if got.AccountOwner.Email != "jordan.perera@wso2.example" {
		t.Errorf("AccountOwner.Email = %q, want %q", got.AccountOwner.Email, "jordan.perera@wso2.example")
	}
	if got.RenewalManager.Email != "sam.jayasuriya@wso2.example" {
		t.Errorf("RenewalManager.Email = %q, want %q", got.RenewalManager.Email, "sam.jayasuriya@wso2.example")
	}
	if got.TechnicalOwner.Email != "alex.fernando@wso2.example" {
		t.Errorf("TechnicalOwner.Email = %q, want %q", got.TechnicalOwner.Email, "alex.fernando@wso2.example")
	}
}

// TestProcessProject_ReminderHasEmptyAccountOwnerEmailWhenNoAccountManager
// covers the legitimate-absence case: an account with no accountManager
// assigned at all (nested key entirely missing, not just empty). The
// reminder notice must still be sent, with an empty AccountOwner.Email —
// this is not an error, per recipients.AccountManagerEmail's contract.
func TestProcessProject_ReminderHasEmptyAccountOwnerEmailWhenNoAccountManager(t *testing.T) {
	reader := &mockEntityReader{
		getAccountFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"id": "a1", "name": "Some Account"}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	endDate := now.AddDate(0, 0, 89)
	proj := project{ID: "p1", Account: &projectAccountRef{ID: "a1"}, EndDate: &endDate}

	err := processProject(context.Background(), reader, updater, ntf, now, proj)
	if err != nil {
		t.Fatalf("processProject() error = %v, want nil", err)
	}
	if len(ntf.sent) != 1 {
		t.Fatalf("ntf.sent = %d, want 1", len(ntf.sent))
	}
	if got := ntf.sent[0].Recipients.AccountOwner.Email; got != "" {
		t.Errorf("AccountOwner.Email = %q, want \"\" (no account manager assigned)", got)
	}
}

// TestInternalNoticeSubject covers the always-[ACP]-prefixed internal
// subject template directly, confirmed against real examples from Chamara:
// every window (90 through 0) gets the prefix — it marks "internal
// audience," not "90/60/30 window" specifically (an earlier version of this
// logic had that backwards). Day-0 uses distinct "Project Suspension
// Notice" wording; every other window uses "N Days Reminder".
func TestInternalNoticeSubject(t *testing.T) {
	tests := []struct {
		name        string
		window      closure.NoticeWindow
		projectName string
		accountName string
		want        string
	}{
		{
			name:        "90-day window",
			window:      90,
			projectName: "TICKETNETWORK - Subscription",
			accountName: "TicketNetwork",
			want:        "[ACP] 90 Days Reminder of Project for TICKETNETWORK - Subscription of TicketNetwork",
		},
		{
			name:        "60-day window",
			window:      60,
			projectName: "P",
			accountName: "A",
			want:        "[ACP] 60 Days Reminder of Project for P of A",
		},
		{
			name:        "30-day window",
			window:      30,
			projectName: "P",
			accountName: "A",
			want:        "[ACP] 30 Days Reminder of Project for P of A",
		},
		{
			name:        "15-day window is still [ACP]-prefixed",
			window:      15,
			projectName: "SSC ICT - Subscription",
			accountName: "SSC-ICT",
			want:        "[ACP] 15 Days Reminder of Project for SSC ICT - Subscription of SSC-ICT",
		},
		{
			name:        "7-day window is still [ACP]-prefixed",
			window:      7,
			projectName: "APIM & Integration - Subscription",
			accountName: "Department of Science & Technology (DOST)",
			want:        "[ACP] 7 Days Reminder of Project for APIM & Integration - Subscription of Department of Science & Technology (DOST)",
		},
		{
			name:        "day-0 uses suspension wording, not days-remaining",
			window:      0,
			projectName: "Kotak Insurance - Subscription",
			accountName: "Kotak Life Insurance company Ltd",
			want:        "[ACP] Project Suspension Notice of Kotak Insurance - Subscription of Kotak Life Insurance company Ltd",
		},
		{
			// Regression test, PR #1440 review (Sajith Ekanayake): a
			// project with no linked account previously produced a
			// dangling "...of " with a trailing space and nothing after
			// it, in a real outbound email subject.
			name:        "no linked account: omits the dangling \" of \" clause entirely",
			window:      90,
			projectName: "Solo Project - Subscription",
			accountName: "",
			want:        "[ACP] 90 Days Reminder of Project for Solo Project - Subscription",
		},
		{
			name:        "no linked account, day-0: omits the dangling \" of \" clause entirely",
			window:      0,
			projectName: "Solo Project - Subscription",
			accountName: "",
			want:        "[ACP] Project Suspension Notice of Solo Project - Subscription",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := internalNoticeSubject(tt.window, tt.projectName, tt.accountName); got != tt.want {
				t.Errorf("internalNoticeSubject(%v, %q, %q) = %q, want %q", tt.window, tt.projectName, tt.accountName, got, tt.want)
			}
		})
	}
}

// TestCustomerNoticeSubject covers the customer-facing subject template,
// confirmed against real examples: never [ACP]-prefixed, never names the
// account, future ("Upcoming") tense before day-0 and past tense at day-0.
func TestCustomerNoticeSubject(t *testing.T) {
	tests := []struct {
		name        string
		window      closure.NoticeWindow
		projectName string
		want        string
	}{
		{
			name:        "15-day window",
			window:      15,
			projectName: "SSC ICT - Subscription",
			want:        "Upcoming Project Suspension Notice - SSC ICT - Subscription",
		},
		{
			name:        "7-day window",
			window:      7,
			projectName: "SSC ICT - Subscription",
			want:        "Upcoming Project Suspension Notice - SSC ICT - Subscription",
		},
		{
			name:        "day-0 uses past tense, no \"Upcoming\"",
			window:      0,
			projectName: "Kotak Insurance - Subscription",
			want:        "Project Suspension Notice - Kotak Insurance - Subscription",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := customerNoticeSubject(tt.window, tt.projectName); got != tt.want {
				t.Errorf("customerNoticeSubject(%v, %q) = %q, want %q", tt.window, tt.projectName, got, tt.want)
			}
		})
	}
}

// TestInternalNoticeBody covers the internal body templates directly,
// confirmed verbatim against real examples: the greeting always names the
// Account Manager (not whichever recipient happens to read their own copy),
// and day-0 uses distinct "already suspended" wording asking for
// reinstatement rather than renewal.
func TestInternalNoticeBody(t *testing.T) {
	startDate := time.Date(2024, 11, 10, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	proj := project{
		Name:       "TICKETNETWORK - Subscription",
		ProjectKey: "TICKETNETWORKPROD",
		StartDate:  &startDate,
		EndDate:    &endDate,
	}

	t.Run("day-count window", func(t *testing.T) {
		got := internalNoticeBody(90, proj, "Lochana De Alwis")
		wantContains := []string{
			"Dear Lochana De Alwis",
			"non renewed contract",
			"Project Name: TICKETNETWORK - Subscription",
			"Project Key: TICKETNETWORKPROD",
			"Account Owner: Lochana De Alwis",
			"Start Date: 2024-11-10",
			"End Date: 2026-11-09",
			"Best Regards,\nWSO2 Team",
		}
		for _, want := range wantContains {
			if !strings.Contains(got, want) {
				t.Errorf("body missing %q, got:\n%s", want, got)
			}
		}
	})

	t.Run("day-0 window uses suspension wording", func(t *testing.T) {
		got := internalNoticeBody(0, proj, "Rajat Mehta")
		wantContains := []string{
			"Dear Rajat Mehta",
			"has been suspended",
			"reinitiate the suspended support account",
			"Account Owner: Rajat Mehta",
		}
		for _, want := range wantContains {
			if !strings.Contains(got, want) {
				t.Errorf("body missing %q, got:\n%s", want, got)
			}
		}
		if strings.Contains(got, "non renewed contract. Please find") {
			t.Errorf("day-0 body contains day-count wording, got:\n%s", got)
		}
	})
}

// TestCustomerNoticeBody covers the customer body templates directly:
// future tense with US-formatted (01/02/2006) dates before day-0, past
// tense at day-0, and no greeting in either case.
func TestCustomerNoticeBody(t *testing.T) {
	endDate := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	proj := project{Name: "SSC ICT - Subscription", EndDate: &endDate}

	t.Run("day-count window uses future tense and US date format", func(t *testing.T) {
		got := customerNoticeBody(15, proj)
		if strings.Contains(got, "Dear ") {
			t.Errorf("body has a greeting, want none:\n%s", got)
		}
		wantContains := []string{
			"will be suspended on 08/24/2026",
			"on or before 08/24/2026",
		}
		for _, want := range wantContains {
			if !strings.Contains(got, want) {
				t.Errorf("body missing %q, got:\n%s", want, got)
			}
		}
	})

	t.Run("day-0 window uses past tense", func(t *testing.T) {
		got := customerNoticeBody(0, proj)
		if !strings.Contains(got, "was suspended 08/24/2026") {
			t.Errorf("body missing past-tense suspension wording, got:\n%s", got)
		}
	})
}
