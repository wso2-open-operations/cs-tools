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

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/scim"
)

type provisionedIdentity struct {
	email, givenName, familyName string
}

// mockIdentityProvisioner stands in for scim.Client: it answers every
// EnsureExternalUser with the configured existed flag or error.
type mockIdentityProvisioner struct {
	existed bool
	err     error
	// block, when non-nil, holds every call open until it is closed, so a
	// test can have several Handle calls in flight on the same record.
	block chan struct{}
	mu    sync.Mutex
	calls []provisionedIdentity
}

func (m *mockIdentityProvisioner) EnsureExternalUser(ctx context.Context, email, givenName, familyName string) (scim.ExternalUser, error) {
	m.mu.Lock()
	m.calls = append(m.calls, provisionedIdentity{email, givenName, familyName})
	m.mu.Unlock()
	if m.block != nil {
		<-m.block
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return scim.ExternalUser{}, m.err
	}
	return scim.ExternalUser{ID: "asgardeo-user-1", UserName: email, Existed: m.existed}, nil
}

// mockStepRecorder stands in for entity.CustomerEntityClient's
// RecordOnboardingStep, keeping every write in order.
type mockStepRecorder struct {
	err error
	// emailAlreadySent / emailSentErr drive the durable duplicate-invitation
	// guard; emailSentChecks counts how often it was consulted.
	emailAlreadySent bool
	emailSentErr     error
	emailSentChecks  int
	// requireLiveContext makes every write fail if its context has already
	// been cancelled, which is how the detached-context test detects a
	// regression rather than relying on timing.
	requireLiveContext bool
	mu                 sync.Mutex
	calls              []entity.OnboardingStepRequest
}

// emailAlreadySent is what EmailAlreadySent answers; emailSentErr makes the
// ledger read itself fail.
func (m *mockStepRecorder) EmailAlreadySent(context.Context, string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emailSentChecks++
	return m.emailAlreadySent, m.emailSentErr
}

func (m *mockStepRecorder) RecordOnboardingStep(ctx context.Context, req entity.OnboardingStepRequest) error {
	if m.requireLiveContext {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("step write received a cancelled context: %w", err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, req)
	return m.err
}

// recordedSteps flattens the recorder's calls to "STEP=STATUS" for
// one-line assertions.
func recordedSteps(r *mockStepRecorder) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	for i, c := range r.calls {
		out[i] = string(c.Step) + "=" + string(c.Status)
	}
	return out
}

func assertSteps(t *testing.T, r *mockStepRecorder, want ...string) {
	t.Helper()
	got := recordedSteps(r)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("recorded steps = %v, want %v", got, want)
	}
}

const invitedMembership = "a0e000000000001AAA"

// invitedRecord builds a project_contact.invited record for jane@acme.com
// on project Acme Cloud; integration flips isIntegrationUser.
func invitedRecord(integration bool) eventbus.Record {
	isIntegration := "false"
	if integration {
		isIntegration = "true"
	}
	return eventbus.Record{Value: []byte(`{"type":"project_contact.invited","entityId":"` + invitedMembership + `","payload":{"membershipSfId":"` + invitedMembership + `","contactSfId":"003000000000001AAA","email":"jane@acme.com","givenName":"Jane","familyName":"Doe","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":["Admin","Portal user"],"isIntegrationUser":` + isIntegration + `,"type":"OWN CONTACT"}}`)}
}

// resentInvitedRecord is the same invitation republished by entity-service
// after an admin pressed "Resend invitation" -- identical to
// invitedRecord(false) but for the isResend marker.
func resentInvitedRecord() eventbus.Record {
	return eventbus.Record{Value: []byte(`{"type":"project_contact.invited","entityId":"` + invitedMembership + `","payload":{"membershipSfId":"` + invitedMembership + `","contactSfId":"003000000000001AAA","email":"jane@acme.com","givenName":"Jane","familyName":"Doe","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":["Admin","Portal user"],"isIntegrationUser":false,"type":"OWN CONTACT","isResend":true}}`)}
}

// newOnboardingDispatcher wires a Dispatcher with every case.* channel
// mocked away and the onboarding feature configured as given.
func newOnboardingDispatcher(identity *mockIdentityProvisioner, email *mockEmailSender, steps *mockStepRecorder, identityEnabled, emailEnabled bool) *Dispatcher {
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{})
	return d.WithOnboarding(OnboardingConfig{
		Identity:        identity,
		Email:           email,
		Steps:           steps,
		IdentityEnabled: identityEnabled,
		EmailEnabled:    emailEnabled,
		PortalURL:       "https://support.wso2.com",
	})
}

// TestDispatcher_Handle_ProjectContactInvited_ConcurrentHandlesProvisionOnce:
// two Handle calls racing on the same record (a consumer-group rebalance,
// or two of this process's consumers) must not both call SCIM. The claim
// on the identity key lets exactly one through; the other fails and, on
// its retry, reuses the winner's remembered answer. Exactly one email.
func TestDispatcher_Handle_ProjectContactInvited_ConcurrentHandlesProvisionOnce(t *testing.T) {
	identity := &mockIdentityProvisioner{block: make(chan struct{})}
	email, steps := &mockEmailSender{}, &mockStepRecorder{}
	d := newOnboardingDispatcher(identity, email, steps, true, true)
	rec := invitedRecord(false)

	// The first Handle to reach SCIM is held there; the others arrive while
	// it holds the claim and must fail without provisioning.
	const n = 8
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() { errs <- d.Handle(context.Background(), rec) }()
	}
	var failed int
	timeout := time.After(5 * time.Second)
	for failed < n-1 {
		select {
		case err := <-errs:
			if err == nil {
				t.Fatal("a Handle call succeeded while the identity step was still held by another")
			}
			failed++
		case <-timeout:
			t.Fatalf("only %d of %d contenders failed within the timeout", failed, n-1)
		}
	}
	close(identity.block)
	if err := <-errs; err != nil {
		t.Fatalf("the winning Handle returned %v, want nil", err)
	}

	if len(identity.calls) != 1 {
		t.Errorf("SCIM called %d times, want exactly 1", len(identity.calls))
	}
	if len(email.calls) != 1 {
		t.Errorf("emails sent %d, want exactly 1", len(email.calls))
	}
	if len(steps.calls) != 2 {
		t.Errorf("steps recorded %d, want IDENTITY and EMAIL once each (losers record nothing)", len(steps.calls))
	}
	// The winner's success released the claim and the memo; a later
	// redelivery starts clean rather than being stuck behind a stale claim.
	if d.rememberedIdentityExistedForTest(recordBaseKey(rec)+"/identity") || d.claimedForTest(recordBaseKey(rec)+"/onboarding") {
		t.Error("in-flight claim and identity memo must be released after the record succeeded")
	}
}

// TestDispatcher_Handle_ProjectContactInvited_ConcurrentHandleDuringEmailSendsOnce:
// the guard covers the whole attempt, not just the SCIM call. A second
// Handle that arrives after the identity memo is written but while the
// first is still sending the email must fail without sending a second one.
func TestDispatcher_Handle_ProjectContactInvited_ConcurrentHandleDuringEmailSendsOnce(t *testing.T) {
	identity, steps := &mockIdentityProvisioner{}, &mockStepRecorder{}
	email := &mockEmailSender{block: make(chan struct{})}
	d := newOnboardingDispatcher(identity, email, steps, true, true)
	rec := invitedRecord(false)

	first := make(chan error, 1)
	go func() { first <- d.Handle(context.Background(), rec) }()
	// Wait until the first call is inside SendEmail (identity done, memo
	// written), then race it with a second call.
	deadline := time.After(5 * time.Second)
	for {
		identity.mu.Lock()
		n := len(identity.calls)
		identity.mu.Unlock()
		if n == 1 && d.rememberedIdentityExistedForTest(recordBaseKey(rec)+"/identity") {
			break
		}
		select {
		case <-deadline:
			t.Fatal("first Handle never reached the email step")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if err := d.Handle(context.Background(), rec); err == nil {
		t.Fatal("second Handle succeeded while the first was still sending; it must fail without sending")
	}
	close(email.block)
	if err := <-first; err != nil {
		t.Fatalf("first Handle returned %v, want nil", err)
	}
	if len(identity.calls) != 1 || len(email.calls) != 1 {
		t.Errorf("scim=%d emails=%d, want exactly one of each", len(identity.calls), len(email.calls))
	}
}

// TestDispatcher_Handle_ProjectContactInvited_SenderOverride: EmailFrom is
// passed through to the shared email client so the invitation can use its
// own sender without a second client.
func TestDispatcher_Handle_ProjectContactInvited_SenderOverride(t *testing.T) {
	identity, email, steps := &mockIdentityProvisioner{}, &mockEmailSender{}, &mockStepRecorder{}
	d := newOnboardingDispatcher(identity, email, steps, true, true)
	d.onboarding.EmailFrom = "invitations@wso2.com"
	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(email.calls) != 1 || email.calls[0].from != "invitations@wso2.com" {
		t.Errorf("email from = %q, want the configured onboarding sender", email.calls[0].from)
	}
}

// TestDispatcher_Handle_ProjectContactInvited_IntegrationUser: a machine
// account never signs in and gets no email — both steps SKIPPED, no SCIM
// call, no SendEmail, and a nil return so the record is acknowledged.
func TestDispatcher_Handle_ProjectContactInvited_IntegrationUser(t *testing.T) {
	identity, email, steps := &mockIdentityProvisioner{}, &mockEmailSender{}, &mockStepRecorder{}
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	if err := d.Handle(context.Background(), invitedRecord(true)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(identity.calls) != 0 || len(email.calls) != 0 {
		t.Errorf("integration user reached an outbound client: scim=%d email=%d", len(identity.calls), len(email.calls))
	}
	assertSteps(t, steps, "IDENTITY=SKIPPED", "EMAIL=SKIPPED")
}

// TestDispatcher_Handle_ProjectContactInvited_NewUser is the happy path with
// both flags on: SCIM is called with the payload's identity, the "welcome"
// template goes to the invitee alone (no CC/BCC), and both steps are
// SUCCEEDED, identity first.
func TestDispatcher_Handle_ProjectContactInvited_NewUser(t *testing.T) {
	identity, email, steps := &mockIdentityProvisioner{existed: false}, &mockEmailSender{}, &mockStepRecorder{}
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(identity.calls) != 1 || identity.calls[0] != (provisionedIdentity{"jane@acme.com", "Jane", "Doe"}) {
		t.Errorf("EnsureExternalUser calls = %+v, want one call with the payload's email/givenName/familyName", identity.calls)
	}
	if len(email.calls) != 1 {
		t.Fatalf("sent %d emails, want 1", len(email.calls))
	}
	sent := email.calls[0]
	if len(sent.to) != 1 || sent.to[0] != "jane@acme.com" || len(sent.bcc) != 0 {
		t.Errorf("to = %v, bcc = %v, want the invitee alone", sent.to, sent.bcc)
	}
	if !strings.Contains(sent.subject, "Welcome") || !strings.Contains(sent.subject, "Acme Cloud") {
		t.Errorf("subject = %q, want the welcome wording naming the project", sent.subject)
	}
	for _, want := range []string{"A WSO2 account has been created for you", "Jane Doe", "Acme Cloud", "ACMECLOUD", "Admin, Portal user", `href="https://support.wso2.com"`} {
		if !strings.Contains(sent.htmlBody, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
	if strings.Contains(sent.htmlBody, "You already have a WSO2 account") {
		t.Error("body uses the existing-account wording for a just-created user")
	}
	assertSteps(t, steps, "IDENTITY=SUCCEEDED", "EMAIL=SUCCEEDED")

	// Every step write carries what the ledger needs to place it.
	for _, c := range steps.calls {
		if c.MembershipSfID != invitedMembership || c.Email != "jane@acme.com" || c.ContactSfID != "003000000000001AAA" ||
			c.EventType != "project_contact.invited" || c.EventModifiedOn.IsZero() || c.LastError != "" {
			t.Errorf("step write = %+v, want membership/email/contact/eventType/eventModifiedOn set and no lastError", c)
		}
	}
}

// TestDispatcher_Handle_ProjectContactInvited_ExistingUser: SCIM says the
// account already existed, so the "project added to your account" wording
// goes out instead of the welcome.
func TestDispatcher_Handle_ProjectContactInvited_ExistingUser(t *testing.T) {
	identity, email, steps := &mockIdentityProvisioner{existed: true}, &mockEmailSender{}, &mockStepRecorder{}
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(email.calls) != 1 {
		t.Fatalf("sent %d emails, want 1", len(email.calls))
	}
	sent := email.calls[0]
	if !strings.Contains(sent.subject, "has been added to your account") {
		t.Errorf("subject = %q, want the existing-account wording", sent.subject)
	}
	if !strings.Contains(sent.htmlBody, "You already have a WSO2 account") || strings.Contains(sent.htmlBody, "A WSO2 account has been created for you") {
		t.Error("body does not use the existing-account wording")
	}
	assertSteps(t, steps, "IDENTITY=SUCCEEDED", "EMAIL=SUCCEEDED")
}

// TestDispatcher_Handle_ProjectContactInvited_IdentityFailure: a SCIM
// failure records IDENTITY=FAILED with the error text, returns the error
// (so the consumer retries / dead-letters), and never reaches the email —
// the invitee must not be told to sign in to an account that doesn't exist.
func TestDispatcher_Handle_ProjectContactInvited_IdentityFailure(t *testing.T) {
	identity := &mockIdentityProvisioner{err: errors.New("upstream returned 500: asgardeo unavailable")}
	email, steps := &mockEmailSender{}, &mockStepRecorder{}
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	err := d.Handle(context.Background(), invitedRecord(false))
	if err == nil || !strings.Contains(err.Error(), "asgardeo unavailable") {
		t.Fatalf("Handle() error = %v, want the SCIM failure propagated", err)
	}
	if len(email.calls) != 0 {
		t.Errorf("sent %d emails after a failed identity step, want 0", len(email.calls))
	}
	assertSteps(t, steps, "IDENTITY=FAILED")
	if got := steps.calls[0].LastError; !strings.Contains(got, "asgardeo unavailable") {
		t.Errorf("lastError = %q, want the SCIM error text", got)
	}
}

// TestDispatcher_Handle_ProjectContactInvited_EmailFailureThenRetry: the
// email failing records EMAIL=FAILED and returns the error; the retry must
// not re-provision the identity (SCIM would now say existed=true) and must
// still send the "welcome" wording the invitee has never received.
func TestDispatcher_Handle_ProjectContactInvited_EmailFailureThenRetry(t *testing.T) {
	identity := &mockIdentityProvisioner{existed: false}
	email := &mockEmailSender{err: errors.New("smtp down")}
	steps := &mockStepRecorder{}
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	err := d.Handle(context.Background(), invitedRecord(false))
	if err == nil || !strings.Contains(err.Error(), "smtp down") {
		t.Fatalf("first Handle() error = %v, want the email failure propagated", err)
	}
	assertSteps(t, steps, "IDENTITY=SUCCEEDED", "EMAIL=FAILED")
	if got := steps.calls[1].LastError; !strings.Contains(got, "smtp down") {
		t.Errorf("EMAIL lastError = %q, want the send error text", got)
	}

	// The retry: SCIM would now answer existed=true for the user the first
	// attempt created — the handler must not ask it again.
	identity.existed = true
	email.err = nil
	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("retry Handle() error = %v", err)
	}
	if len(identity.calls) != 1 {
		t.Errorf("EnsureExternalUser called %d times across the retry, want 1", len(identity.calls))
	}
	if len(email.calls) != 2 {
		t.Fatalf("sent %d emails across both attempts, want 2 (one failed, one succeeded)", len(email.calls))
	}
	if !strings.Contains(email.calls[1].htmlBody, "A WSO2 account has been created for you") {
		t.Error("retry used the existing-account wording for a user the first attempt created")
	}
	assertSteps(t, steps, "IDENTITY=SUCCEEDED", "EMAIL=FAILED", "EMAIL=SUCCEEDED")

	// Full success releases the remembered answer.
	if _, ok := d.rememberedIdentityExisted(recordBaseKey(invitedRecord(false)) + "/identity"); ok {
		t.Error("identityExisted still holds the record's key after a full success")
	}
}

// TestDispatcher_Handle_ProjectContactInvited_NoMoreRetriesReleasesMemo: a
// record that will never be redelivered must not leak its remembered
// identity answer, even though it's still failing.
func TestDispatcher_Handle_ProjectContactInvited_NoMoreRetriesReleasesMemo(t *testing.T) {
	identity := &mockIdentityProvisioner{}
	email := &mockEmailSender{err: errors.New("smtp down")}
	d := newOnboardingDispatcher(identity, email, &mockStepRecorder{}, true, true)

	record := invitedRecord(false)
	record.NoMoreRetries = true
	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("Handle() = nil, want the email failure")
	}
	if _, ok := d.rememberedIdentityExisted(recordBaseKey(record) + "/identity"); ok {
		t.Error("identityExisted still holds the record's key after NoMoreRetries")
	}
}

// TestDispatcher_Handle_ProjectContactInvited_BothFlagsOff: the default
// deployment shape — nothing outbound at all, both steps SKIPPED, nil.
func TestDispatcher_Handle_ProjectContactInvited_BothFlagsOff(t *testing.T) {
	identity, email, steps := &mockIdentityProvisioner{}, &mockEmailSender{}, &mockStepRecorder{}
	d := newOnboardingDispatcher(identity, email, steps, false, false)

	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(identity.calls) != 0 || len(email.calls) != 0 {
		t.Errorf("flags off but an outbound client was called: scim=%d email=%d", len(identity.calls), len(email.calls))
	}
	assertSteps(t, steps, "IDENTITY=SKIPPED", "EMAIL=SKIPPED")
}

// TestDispatcher_Handle_ProjectContactInvited_IdentityOffUsesNewWording:
// with identity disabled nothing can say whether the account exists, so
// the email uses the "new" template in its neutral form — it must not
// claim an account was created, nor that one already exists.
func TestDispatcher_Handle_ProjectContactInvited_IdentityOffUsesNewWording(t *testing.T) {
	identity, email, steps := &mockIdentityProvisioner{existed: true}, &mockEmailSender{}, &mockStepRecorder{}
	d := newOnboardingDispatcher(identity, email, steps, false, true)

	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(identity.calls) != 0 {
		t.Errorf("SCIM called %d times with identity disabled", len(identity.calls))
	}
	if len(email.calls) != 1 {
		t.Fatalf("want exactly one email, got %d", len(email.calls))
	}
	body := email.calls[0].htmlBody
	if !strings.Contains(body, "Sign in with your email address") ||
		strings.Contains(body, "A WSO2 account has been created for you") ||
		strings.Contains(body, "You already have a WSO2 account") {
		t.Error("with identity disabled the email must use the neutral wording and make no claim about the account")
	}
	if !strings.Contains(email.calls[0].subject, "You have been given access to") {
		t.Errorf("subject = %q, want the neutral subject", email.calls[0].subject)
	}
	assertSteps(t, steps, "IDENTITY=SKIPPED", "EMAIL=SUCCEEDED")
}

// TestDispatcher_Handle_ProjectContactInvited_StepRecordingFailureIsBestEffort:
// the ledger being down must not turn a successful onboarding into a
// retried (and re-sent) record.
func TestDispatcher_Handle_ProjectContactInvited_StepRecordingFailureIsBestEffort(t *testing.T) {
	identity, email := &mockIdentityProvisioner{}, &mockEmailSender{}
	steps := &mockStepRecorder{err: errors.New("entity-service 503")}
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v, want nil despite the recorder failing", err)
	}
	if len(identity.calls) != 1 || len(email.calls) != 1 {
		t.Errorf("scim=%d email=%d, want both steps to have run once", len(identity.calls), len(email.calls))
	}
	assertSteps(t, steps, "IDENTITY=SUCCEEDED", "EMAIL=SUCCEEDED")
}

// TestDispatcher_Handle_ProjectContactInvited_StepRecordingFailureDoesNotMaskPrimaryError:
// when the step itself failed, the caller sees that error — not the
// recorder's.
func TestDispatcher_Handle_ProjectContactInvited_StepRecordingFailureDoesNotMaskPrimaryError(t *testing.T) {
	identity := &mockIdentityProvisioner{err: errors.New("scim exploded")}
	steps := &mockStepRecorder{err: errors.New("entity-service 503")}
	d := newOnboardingDispatcher(identity, &mockEmailSender{}, steps, true, true)

	err := d.Handle(context.Background(), invitedRecord(false))
	if err == nil || !strings.Contains(err.Error(), "scim exploded") || strings.Contains(err.Error(), "entity-service 503") {
		t.Fatalf("Handle() error = %v, want the SCIM error alone", err)
	}
}

// TestDispatcher_Handle_ProjectContactInvited_NamelessInviteeUsesEmailLocalPart:
// Salesforce doesn't require a first name; the email must still address
// the reader by something recognisably theirs rather than a blank.
func TestDispatcher_Handle_ProjectContactInvited_NamelessInviteeUsesEmailLocalPart(t *testing.T) {
	email := &mockEmailSender{}
	d := newOnboardingDispatcher(&mockIdentityProvisioner{}, email, &mockStepRecorder{}, true, true)

	record := eventbus.Record{Value: []byte(`{"type":"project_contact.invited","entityId":"` + invitedMembership + `","payload":{"membershipSfId":"` + invitedMembership + `","contactSfId":"","email":"ops.team@acme.com","givenName":"","familyName":"","projectName":"","projectKey":"ACMECLOUD","roles":[],"isIntegrationUser":false,"type":"OWN CONTACT"}}`)}
	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(email.calls) != 1 {
		t.Fatalf("sent %d emails, want 1", len(email.calls))
	}
	if !strings.Contains(email.calls[0].htmlBody, "ops.team") {
		t.Error("body does not fall back to the email's local part as the display name")
	}
	if !strings.Contains(email.calls[0].subject, "ACMECLOUD") {
		t.Errorf("subject = %q, want the project key when the name is empty", email.calls[0].subject)
	}
	if strings.Contains(email.calls[0].htmlBody, "Your role") {
		t.Error("body renders a roles line for a membership with no roles")
	}
}

// TestDispatcher_Handle_ProjectContactInvited_Killswitch: EMAIL_SENDING_ENABLED
// silences the invitation like every other email here — recorded SKIPPED,
// not FAILED, and the identity step still runs.
func TestDispatcher_Handle_ProjectContactInvited_Killswitch(t *testing.T) {
	identity, email, steps := &mockIdentityProvisioner{}, &mockEmailSender{}, &mockStepRecorder{}
	d := NewDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}, &mockLinkResolver{}, false, false, nil, true, "", "").
		WithOnboarding(OnboardingConfig{Identity: identity, Email: email, Steps: steps, IdentityEnabled: true, EmailEnabled: true, PortalURL: "https://support.wso2.com"})

	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(identity.calls) != 1 || len(email.calls) != 0 {
		t.Errorf("scim=%d email=%d, want identity provisioned and no email", len(identity.calls), len(email.calls))
	}
	assertSteps(t, steps, "IDENTITY=SUCCEEDED", "EMAIL=SKIPPED")
}

// TestDispatcher_Handle_ProjectContactInvited_DebugModeRedirects: debug
// mode sends the invitation to the test list, never to the real contact.
func TestDispatcher_Handle_ProjectContactInvited_DebugModeRedirects(t *testing.T) {
	email, steps := &mockEmailSender{}, &mockStepRecorder{}
	d := NewDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}, &mockLinkResolver{}, true, true, []string{"debug@wso2.com"}, true, "", "").
		WithOnboarding(OnboardingConfig{Identity: &mockIdentityProvisioner{}, Email: email, Steps: steps, IdentityEnabled: true, EmailEnabled: true, PortalURL: "https://support.wso2.com"})

	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(email.calls) != 1 || len(email.calls[0].to) != 1 || email.calls[0].to[0] != "debug@wso2.com" {
		t.Errorf("email calls = %+v, want one send redirected to the debug list", email.calls)
	}
	assertSteps(t, steps, "IDENTITY=SUCCEEDED", "EMAIL=SUCCEEDED")
}

// TestDispatcher_Handle_ProjectContactInvited_RejectsInvalidPayload: the
// validation boundary applies here like everywhere else — a payload with
// no email can't be onboarded and is an error, not a SKIPPED pair.
func TestDispatcher_Handle_ProjectContactInvited_RejectsInvalidPayload(t *testing.T) {
	steps := &mockStepRecorder{}
	d := newOnboardingDispatcher(&mockIdentityProvisioner{}, &mockEmailSender{}, steps, true, true)

	record := eventbus.Record{Value: []byte(`{"type":"project_contact.invited","entityId":"` + invitedMembership + `","payload":{"membershipSfId":"` + invitedMembership + `","email":"","projectName":"Acme Cloud"}}`)}
	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("Handle() = nil, want a validation error")
	}
	if len(steps.calls) != 0 {
		t.Errorf("recorded %d steps for an invalid payload, want 0", len(steps.calls))
	}
}

func TestInviteeDisplayName(t *testing.T) {
	tests := []struct {
		given, family, email, want string
	}{
		{"Jane", "Doe", "jane@acme.com", "Jane Doe"},
		{"Jane", "", "jane@acme.com", "Jane"},
		{"", "Doe", "jane@acme.com", "Doe"},
		{"", "", "jane.doe@acme.com", "jane.doe"},
		{"  ", "  ", "jane@acme.com", "jane"},
		{"", "", "no-at-sign", "no-at-sign"},
	}
	for _, tt := range tests {
		if got := inviteeDisplayName(tt.given, tt.family, tt.email); got != tt.want {
			t.Errorf("inviteeDisplayName(%q, %q, %q) = %q, want %q", tt.given, tt.family, tt.email, got, tt.want)
		}
	}
}

// rememberedIdentityExistedForTest / claimedForTest peek at the
// dispatcher's idempotency state for the concurrency test above.
func (d *Dispatcher) rememberedIdentityExistedForTest(key string) bool {
	_, ok := d.rememberedIdentityExisted(key)
	return ok
}

func (d *Dispatcher) claimedForTest(key string) bool {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	return d.done[key]
}

// TestDispatcher_Handle_ProjectContactInvited_SkipsWhenTheLedgerSaysSent is
// the durable guard: a membership whose EMAIL step is already SUCCEEDED --
// because an earlier delivery sent it, or because the customer portal
// onboarded the contact synchronously before the Salesforce event arrived --
// must not be invited a second time. Identity still runs: SCIM is
// create-if-absent, so it is harmless and keeps the step honest.
func TestDispatcher_Handle_ProjectContactInvited_SkipsWhenTheLedgerSaysSent(t *testing.T) {
	identity, email := &mockIdentityProvisioner{}, &mockEmailSender{}
	steps := &mockStepRecorder{emailAlreadySent: true}
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}
	if len(email.calls) != 0 {
		t.Errorf("sent %d emails, want none: the ledger already records one", len(email.calls))
	}
	if steps.emailSentChecks != 1 {
		t.Errorf("ledger consulted %d times, want exactly 1", steps.emailSentChecks)
	}
}

// TestDispatcher_Handle_ProjectContactInvited_LedgerUnreadableDoesNotSend:
// if the ledger cannot be read we do not know whether an invitation went
// out, so the record is retried rather than risking a second one.
func TestDispatcher_Handle_ProjectContactInvited_LedgerUnreadableDoesNotSend(t *testing.T) {
	identity, email := &mockIdentityProvisioner{}, &mockEmailSender{}
	steps := &mockStepRecorder{emailSentErr: errors.New("entity-service unavailable")}
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	err := d.Handle(context.Background(), invitedRecord(false))
	if err == nil {
		t.Fatal("Handle() = nil, want an error so the consumer retries")
	}
	if len(email.calls) != 0 {
		t.Errorf("sent %d emails, want none while the ledger is unreadable", len(email.calls))
	}
}

// TestDispatcher_Handle_ProjectContactInvited_LedgerNotConsultedWhenEmailIsOff
// keeps the guard off the path it cannot affect: with the email step
// disabled there is nothing to suppress.
func TestDispatcher_Handle_ProjectContactInvited_LedgerNotConsultedWhenEmailIsOff(t *testing.T) {
	identity, email, steps := &mockIdentityProvisioner{}, &mockEmailSender{}, &mockStepRecorder{}
	d := newOnboardingDispatcher(identity, email, steps, true, false)

	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if steps.emailSentChecks != 0 {
		t.Errorf("ledger consulted %d times with the email step off, want 0", steps.emailSentChecks)
	}
}

// TestDispatcher_Handle_ProjectContactInvited_ResendBypassesTheLedgerGuard:
// a deliberate resend is the one case the duplicate guard must not stop.
// The ledger says an invitation already succeeded -- which is exactly what
// a resend expects to find -- and the email still goes out, with the EMAIL
// step recorded again so the ledger's attempt count keeps counting.
func TestDispatcher_Handle_ProjectContactInvited_ResendBypassesTheLedgerGuard(t *testing.T) {
	identity, email := &mockIdentityProvisioner{existed: true}, &mockEmailSender{}
	steps := &mockStepRecorder{emailAlreadySent: true}
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	if err := d.Handle(context.Background(), resentInvitedRecord()); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(email.calls) != 1 {
		t.Fatalf("sent %d emails, want 1: a resend must send even though the ledger records one", len(email.calls))
	}
	if steps.emailSentChecks != 0 {
		t.Errorf("ledger consulted %d times on a resend, want 0", steps.emailSentChecks)
	}
	assertSteps(t, steps, "IDENTITY=SUCCEEDED", "EMAIL=SUCCEEDED")
}

// TestDispatcher_Handle_ProjectContactInvited_ResendUsesReminderWording:
// by the time a resend goes out the account exists, so the unguarded logic
// would reach for the "you already have a WSO2 account" template -- which
// reads as nonsense to someone who never received the first email. A
// resend gets the reminder instead, and neither of the other two.
func TestDispatcher_Handle_ProjectContactInvited_ResendUsesReminderWording(t *testing.T) {
	identity, email, steps := &mockIdentityProvisioner{existed: true}, &mockEmailSender{}, &mockStepRecorder{}
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	if err := d.Handle(context.Background(), resentInvitedRecord()); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(email.calls) != 1 {
		t.Fatalf("sent %d emails, want 1", len(email.calls))
	}
	sent := email.calls[0]
	if !strings.Contains(sent.subject, "Reminder") || !strings.Contains(sent.subject, "Acme Cloud") {
		t.Errorf("subject = %q, want the reminder wording naming the project", sent.subject)
	}
	if !strings.Contains(sent.htmlBody, "Here is your invitation to the project") {
		t.Error("body does not use the reminder wording")
	}
	for _, deny := range []string{"A WSO2 account has been created for you", "You already have a WSO2 account"} {
		if strings.Contains(sent.htmlBody, deny) {
			t.Errorf("resend body contains %q, which belongs to another variant", deny)
		}
	}
}

// TestDispatcher_Handle_ProjectContactInvited_WithoutResendTheGuardStillHolds
// is the other half of the pair above: the same ledger state, the same
// dispatcher, only the marker missing -- and nothing is sent.
func TestDispatcher_Handle_ProjectContactInvited_WithoutResendTheGuardStillHolds(t *testing.T) {
	identity, email := &mockIdentityProvisioner{existed: true}, &mockEmailSender{}
	steps := &mockStepRecorder{emailAlreadySent: true}
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	if err := d.Handle(context.Background(), invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(email.calls) != 0 {
		t.Errorf("sent %d emails without the resend marker, want none", len(email.calls))
	}
	if steps.emailSentChecks != 1 {
		t.Errorf("ledger consulted %d times, want exactly 1", steps.emailSentChecks)
	}
}

// TestDispatcher_Handle_ProjectContactInvited_NoLedgerConfiguredDoesNotSend
// covers a misconfiguration the other two nil checks already cover for the
// SCIM and email clients: the email step enabled with no onboarding-step
// recorder at all. Recording a step tolerates that, but the duplicate check
// cannot -- and reaching it with a nil recorder used to panic inside the
// consumer goroutine. It must be an ordinary failed record instead, with no
// unguarded invitation going out.
func TestDispatcher_Handle_ProjectContactInvited_NoLedgerConfiguredDoesNotSend(t *testing.T) {
	identity, email := &mockIdentityProvisioner{}, &mockEmailSender{}
	// Deliberately not newOnboardingDispatcher: Steps has to be a nil
	// interface, not a typed nil *mockStepRecorder.
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).
		WithOnboarding(OnboardingConfig{
			Identity:        identity,
			Email:           email,
			IdentityEnabled: true,
			EmailEnabled:    true,
			PortalURL:       "https://support.wso2.com",
		})

	err := d.Handle(context.Background(), invitedRecord(false))
	if err == nil {
		t.Fatal("Handle() = nil, want a configuration error so the record is retried")
	}
	if !strings.Contains(err.Error(), "onboarding-step ledger") {
		t.Errorf("Handle() error = %v, want it to name the missing ledger", err)
	}
	if len(email.calls) != 0 {
		t.Errorf("sent %d emails, want none with no ledger to check them against", len(email.calls))
	}
}

// TestDispatcher_Handle_ProjectContactInvited_ResendNeedsNoLedger is the
// other side of that guard: a resend never reads the ledger, so a missing
// recorder must not stop it. The send is deliberate; the only thing lost is
// the best-effort record of it, which recordOnboardingStep already tolerates.
func TestDispatcher_Handle_ProjectContactInvited_ResendNeedsNoLedger(t *testing.T) {
	identity, email := &mockIdentityProvisioner{existed: true}, &mockEmailSender{}
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).
		WithOnboarding(OnboardingConfig{
			Identity:        identity,
			Email:           email,
			IdentityEnabled: true,
			EmailEnabled:    true,
			PortalURL:       "https://support.wso2.com",
		})

	if err := d.Handle(context.Background(), resentInvitedRecord()); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(email.calls) != 1 {
		t.Fatalf("sent %d emails, want 1: a resend does not depend on the ledger", len(email.calls))
	}
}

// TestDispatcher_Handle_ProjectContactInvited_LedgerWriteSurvivesCancellation
// pins the detached recording context. A shutdown cancels the handler's
// context, and the EMAIL=SUCCEEDED write must still happen: losing it while
// the offset commit is also lost is precisely how one invitation becomes
// two.
func TestDispatcher_Handle_ProjectContactInvited_LedgerWriteSurvivesCancellation(t *testing.T) {
	identity, email, steps := &mockIdentityProvisioner{}, &mockEmailSender{}, &mockStepRecorder{}
	steps.requireLiveContext = true
	d := newOnboardingDispatcher(identity, email, steps, true, true)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancelled the instant the email is sent, standing in for a shutdown
	// landing between the send and the write that records it.
	email.onSend = cancel

	if err := d.Handle(ctx, invitedRecord(false)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(email.calls) != 1 {
		t.Fatalf("emails sent = %d, want 1", len(email.calls))
	}
	assertSteps(t, steps, "IDENTITY=SUCCEEDED", "EMAIL=SUCCEEDED")
}
