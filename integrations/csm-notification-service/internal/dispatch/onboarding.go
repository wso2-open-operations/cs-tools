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

// The customer-onboarding handlers (project_contact.invited / project_contact.registered) and their ledger helpers.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/scim"
)

// identityProvisioner abstracts scim.Client for testability — the one
// operation project_contact.invited's identity step needs.
type identityProvisioner interface {
	EnsureExternalUser(ctx context.Context, email, givenName, familyName string) (scim.ExternalUser, error)
}

// onboardingStepRecorder abstracts entity.CustomerEntityClient's
// RecordOnboardingStep for testability — the one write this dispatcher
// makes back to entity-service.
type onboardingStepRecorder interface {
	RecordOnboardingStep(ctx context.Context, req entity.OnboardingStepRequest) error
	// SucceededEmailStep returns the SUCCEEDED EMAIL step on the ledger for
	// the membership, or nil when there is none -- the durable "an
	// invitation has already gone out, for this membership version" check
	// (see Dispatcher.invitationAlreadySent).
	SucceededEmailStep(ctx context.Context, membershipSfID string) (*entity.RecordedOnboardingStep, error)
	// SucceededStep is the same lookup for any step (the Welcome guard).
	SucceededStep(ctx context.Context, membershipSfID string, step entity.OnboardingStep) (*entity.RecordedOnboardingStep, error)
}

// OnboardingConfig is everything handleProjectContactInvited needs beyond
// what NewDispatcher already takes — supplied via Dispatcher.WithOnboarding
// rather than as more NewDispatcher Config fields, since the
// whole feature is optional per deployment (both flags default off) and
// every other event type is untouched by it.
//
// Identity (satisfied by *scim.Client) creates the invitee's Asgardeo user
// when IdentityEnabled (CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED); Email sends the
// invitation when EmailEnabled (CSM_MIGRATION_ONBOARD_EMAIL_ENABLED) — a separate
// emailSender from the Dispatcher's own, because the invitation may go out
// from a different sender address (ONBOARD_EMAIL_FROM) than the case.*
// emails, and notifications.EmailClient binds its From at construction.
// Steps (satisfied by *entity.CustomerEntityClient) records each step's
// outcome on entity-service's onboarding-step ledger, best-effort — see
// recordOnboardingStep — and is also read back for the duplicate-invitation
// check. Recording tolerates a nil Steps, but EmailEnabled does not: with
// no ledger to read there is no duplicate check, so the EMAIL step fails as
// a configuration error rather than sending unguarded. PortalURL is the
// sign-in link the invitation points at (ONBOARD_PORTAL_URL).
type OnboardingConfig struct {
	Identity        identityProvisioner
	Email           emailSender
	Steps           onboardingStepRecorder
	IdentityEnabled bool
	EmailEnabled    bool
	PortalURL       string
	// EmailFrom is the invitation's sender (ONBOARD_EMAIL_FROM); "" means
	// Email's own FromAddress.
	EmailFrom string
	// ReplyTo (EMAIL_REPLY_TO) goes on onboarding emails only.
	ReplyTo []string
}

// WithOnboarding configures handleProjectContactInvited (see
// OnboardingConfig) and returns d for chaining. Not part of NewDispatcher's
// parameter list deliberately: the feature is optional per deployment and
// orthogonal to every other event type.
func (d *Dispatcher) WithOnboarding(cfg OnboardingConfig) *Dispatcher {
	d.onboarding = cfg
	return d
}

// handleProjectContactInvited is the consumer side of the customer
// onboarding flow (Salesforce membership → entity-service's Postgres →
// Asgardeo identity → invitation email → first-access registration): two
// sequential steps, IDENTITY then EMAIL, each recorded on entity-service's
// onboarding-step ledger (recordOnboardingStep) and each behind its own
// deployment flag (OnboardingConfig.IdentityEnabled/EmailEnabled, both
// default off).
//
// Unlike every case.* handler, there is nothing to resolve: the invitee is
// the payload's own single email address (no recipient list, no
// groupByLink, no CC), and the sign-in link is one configured portal URL,
// not a per-recipient case link. Sequential, not two independent
// reactions like handleCaseCreated's email+Chat: the email's wording
// depends on the identity step's answer (a just-created account gets the
// "welcome" template, an account that already existed gets "the project
// was added"), so a failed identity step returns before any email is
// attempted — the invitee must not be told to sign in to an account that
// doesn't exist.
//
// A resend (ProjectContactInvitedPayload.IsResend, set by entity-service
// when an admin presses "Resend invitation") is the one case that skips
// the duplicate-invitation ledger check below and uses the short reminder
// template instead of either of the other two. The identity step is
// unchanged: the SCIM endpoint is create-if-absent, so a resend simply
// finds the account the first invitation created.
//
// An integration user (IsIntegrationUser) never signs in and gets no
// email: both steps are recorded SKIPPED and nothing else happens. A step
// whose flag is off is likewise recorded SKIPPED. A step that fails records
// FAILED with the error text and returns the error, so eventbus.Consumer's
// usual retry/dead-letter path applies; a step that succeeds records
// SUCCEEDED. Recording itself is best-effort and never changes the
// handler's outcome — see recordOnboardingStep.
//
// Retry safety: EnsureExternalUser is idempotent upstream (a repeat is a
// 200), so re-running the identity step is harmless in itself — but it
// would answer existed=true for a user the previous attempt created, and
// the retry's email would then use the wrong wording. Dispatcher.
// identityExisted remembers the first successful answer per record for
// exactly that case; it's released on full success or record.NoMoreRetries
// (never IsFinalAttempt — see recordBaseKey for why a dead-lettered record
// keeps the same key on the DLQ topic). The whole attempt is guarded by a
// per-record claim() (see inflightKey below), so two Handle calls racing on
// the same record cannot both provision or both send: the loser returns an
// error and its retry runs alone, finding the winner's remembered answer.
// No claim/forget tracking is needed for the email itself: nothing after
// SendEmail can fail in a way that triggers a retry (step recording is
// best-effort), so a sent invitation is never re-sent by this handler's own
// retries.
func (d *Dispatcher) handleProjectContactInvited(ctx context.Context, record eventbus.Record, raw json.RawMessage) (retErr error) {
	var p events.ProjectContactInvitedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode project_contact.invited payload: %w", err)
	}
	// Never the invitee's email address — this repo's own "no recipient
	// emails in logs" convention; the membership id is enough to find the
	// row (and its email) on entity-service's ledger.
	logAttrs := []any{"membershipSfId", p.MembershipSfID, "contactSfId", p.ContactSfID, "projectKey", p.ProjectKey}

	if p.IsIntegrationUser {
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepIdentity, entity.OnboardingStepSkipped, nil)
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepSkipped, nil)
		slog.InfoContext(ctx, "dispatch: project_contact.invited for an integration user; identity and email skipped", logAttrs...)
		return nil
	}

	// One attempt at a time per record. Two Handle calls racing on the same
	// record (a consumer-group rebalance, or two of this process's
	// consumers) must not both run the identity or the email step: the
	// loser fails here without touching anything and its retry runs the
	// whole sequence alone, finding whatever the winner remembered. The
	// guard covers the entire attempt, not just the SCIM call — a call that
	// arrived after the memo was written but before the winner's email
	// went out would otherwise send a second invitation. Released whenever
	// this call returns; the memo below outlives it for sequential retries.
	inflightKey := recordBaseKey(record) + "/onboarding"
	if !d.claim(inflightKey) {
		return fmt.Errorf("dispatch: onboarding for membership %s is already in progress", p.MembershipSfID)
	}
	defer d.forget(inflightKey)

	identityKey := recordBaseKey(record) + "/identity"
	defer func() {
		// Drop the remembered identity answer once no retry can ever need
		// it again: the whole record succeeded, or nothing will redeliver
		// its content anywhere (NoMoreRetries — see its doc comment).
		if retErr == nil || record.NoMoreRetries {
			d.forgetIdentityExisted(identityKey)
		}
	}()

	// Step 1 — IDENTITY.
	var existed bool
	// identityKnown: existed is this version's first, trustworthy answer
	// (the identity step ran here, or the memo of an earlier attempt in
	// this process). Only then may the email claim either way.
	var identityKnown bool
	switch {
	case !d.onboarding.IdentityEnabled:
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepIdentity, entity.OnboardingStepSkipped, nil)
		slog.InfoContext(ctx, "dispatch: identity provisioning disabled (CSM_MIGRATION_ONBOARD_IDENTITY_ENABLED != true); skipping", logAttrs...)
	default:
		if remembered, ok := d.rememberedIdentityExisted(identityKey); ok {
			// A previous attempt at this same record already provisioned
			// the identity (and recorded SUCCEEDED); this retry is here for
			// a later step. Reuse its answer rather than asking again — see
			// the doc comment above for why asking again gives the wrong
			// email wording.
			existed = remembered
			identityKnown = true
			slog.InfoContext(ctx, "dispatch: identity already provisioned by an earlier attempt at this record; not repeating", append(logAttrs, "existed", existed)...)
			break
		}
		if d.onboarding.Identity == nil {
			err := fmt.Errorf("dispatch: identity provisioning enabled but no SCIM client configured")
			d.recordOnboardingStep(ctx, p, entity.OnboardingStepIdentity, entity.OnboardingStepFailed, err)
			return err
		}
		// No in-process memo, but this version's identity step may still
		// have succeeded already -- in another replica, or in this process
		// before a restart, after which the email step failed and the
		// record came back. SCIM would then answer existed=true for the
		// account that attempt created. The ledger is the durable record:
		// when it already shows IDENTITY succeeded for this version, the
		// SCIM answer below is still requested (create-if-absent, so the
		// account is guaranteed to exist) but not trusted for wording.
		priorRun, err := d.identityAlreadyProvisioned(ctx, p, logAttrs)
		if err != nil {
			return fmt.Errorf("dispatch: check identity already provisioned for membership %s: %w", p.MembershipSfID, err)
		}
		user, err := d.onboarding.Identity.EnsureExternalUser(ctx, p.Email, p.GivenName, p.FamilyName)
		if err != nil {
			err = fmt.Errorf("dispatch: provision identity for membership %s: %w", p.MembershipSfID, err)
			d.recordOnboardingStep(ctx, p, entity.OnboardingStepIdentity, entity.OnboardingStepFailed, err)
			return err
		}
		existed = user.Existed
		identityKnown = !priorRun
		if identityKnown {
			d.rememberIdentityExisted(identityKey, existed)
		}
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepIdentity, entity.OnboardingStepSucceeded, nil)
		slog.InfoContext(ctx, "dispatch: identity provisioned", append(logAttrs, "asgardeoUserId", user.ID, "existed", existed, "earlierAttemptRecorded", priorRun)...)
	}

	// Step 2 — EMAIL.
	switch {
	case !d.onboarding.EmailEnabled:
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepSkipped, nil)
		slog.InfoContext(ctx, "dispatch: invitation email disabled (CSM_MIGRATION_ONBOARD_EMAIL_ENABLED != true); skipping", logAttrs...)
	case !d.emailSendingEnabled:
		// The service-wide killswitch silences this email the same way it
		// silences every other one here — recorded SKIPPED, not FAILED,
		// since retrying won't change an operator's decision.
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepSkipped, nil)
		slog.InfoContext(ctx, "dispatch: email sending disabled (EMAIL_SENDING_ENABLED=false); not sending invitation", logAttrs...)
	default:
		to := []string{p.Email}
		var intendedFor string
		if d.emailDebugMode {
			if len(d.emailDebugRecipients) == 0 {
				d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepSkipped, nil)
				slog.WarnContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true but EMAIL_DEBUG_RECIPIENTS is empty; not sending invitation", logAttrs...)
				break
			}
			// Same redirect every other email here gets in debug mode: a
			// real send, just to the configured test list instead of the
			// invitee — so a staging deployment can't invite a real
			// customer contact by accident.
			slog.InfoContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true; redirecting invitation to configured debug recipients", append(logAttrs, "debugRecipientCount", len(d.emailDebugRecipients))...)
			intendedFor = p.Email
			to = d.emailDebugRecipients
		}
		if d.onboarding.Email == nil {
			err := fmt.Errorf("dispatch: invitation email enabled but no email client configured")
			d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepFailed, err)
			return err
		}
		if !p.IsResend && d.onboarding.Steps == nil {
			// A resend never reaches the read below, so a missing recorder
			// costs it nothing but the (best-effort) record of the send;
			// failing a deliberate resend over it would be gratuitous.
			//
			// recordOnboardingStep tolerates a nil recorder -- a step
			// outcome nobody can write down is worth a warning, not a
			// failed record. The read below is not that: without it there
			// is no duplicate check at all, and sending anyway is the
			// second invitation this whole block exists to prevent. So it
			// is a configuration error, reported the same way as the two
			// nil checks above, and the record follows the normal
			// retry/DLQ path instead of panicking inside the consumer.
			err := fmt.Errorf("dispatch: invitation email enabled but no onboarding-step ledger configured; cannot check whether an invitation was already sent")
			d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepFailed, err)
			return err
		}

		// Last check before sending: has an invitation for this version of
		// the membership already gone out? (A re-invitation -- the
		// membership moved back into INVITED or RE-INVITED, say after a
		// deactivation -- is a newer version and must send again; see
		// invitationAlreadySent.) The other two guards cannot answer that. The
		// in-process claim above lives for one process, and the ingest's
		// duplicate check only recognises an unchanged Salesforce version,
		// so neither covers a redelivery after a restart, a replay from the
		// dead-letter topic, or the case this was written for -- the
		// customer portal onboarding a contact synchronously, with the
		// Salesforce event for the same contact reaching the ingest
		// afterwards as a new version.
		//
		// It is a read of shared state, not a lock, and the difference
		// matters: two replicas can both read "not sent" before either
		// sends, and a send whose SUCCEEDED write then fails leaves no
		// record for the next delivery to find. Both windows are narrow --
		// the first needs concurrent delivery of one record, which only
		// happens across a consumer-group rebalance, and the second needs
		// the ledger write and the offset commit to fail together -- and
		// the cost of losing either race is one repeated welcome e-mail.
		// Closing them properly needs an atomic reservation in
		// entity-service (claim the EMAIL step, and be told whether you
		// won); see the PR discussion. Until there is a reason to build
		// that, this catches every duplicate we can actually foresee.
		//
		// A failure to read the ledger is not a reason to send, and not a
		// reason to give up either, so it is returned and the record is
		// retried.
		//
		// A resend skips the check entirely. The guard exists to stop an
		// *accidental* second invitation -- every portal invitation also
		// writes the membership back to Salesforce, so the same contact
		// returns through the ingest as an event and would otherwise be
		// invited twice. An admin pressing "Resend invitation" is not
		// that: entity-service republishes this event with isResend set
		// precisely because the invitation should go out again. The EMAIL
		// step is still recorded either way, so the ledger's attemptCount
		// keeps showing how many invitations actually went out.
		if p.IsResend {
			slog.InfoContext(ctx, "dispatch: resend requested; not checking the invitation ledger", logAttrs...)
		} else if sent, err := d.invitationAlreadySent(ctx, p, logAttrs); err != nil {
			return fmt.Errorf("dispatch: check invitation already sent for membership %s: %w", p.MembershipSfID, err)
		} else if sent {
			break
		}

		data := notifications.ProjectContactInvitedEmailData{
			DisplayName: inviteeDisplayName(p.GivenName, p.FamilyName, p.Email),
			Email:       p.Email,
			ProjectName: displayProjectName(p.ProjectName, p.ProjectKey),
			ProjectKey:  p.ProjectKey,
			Roles:       p.Roles,
			PortalURL:   d.onboarding.PortalURL,
			IntendedFor: intendedFor,
		}
		// The "existing" wording only when the identity step actually ran
		// this record and said so. With identity disabled nothing here can
		// know whether an account exists, so the "new" template is sent
		// with AccountCreated=false: it then says neither "an account has
		// been created for you" nor "you already have one", only how to
		// sign in.
		var subject, body string
		switch {
		case p.IsResend:
			// A resend always uses the reminder wording, whatever the
			// identity step answered. By now the account exists (the first
			// invitation, or this record's own identity step, created it),
			// so the "existing" template would tell someone who may never
			// have opened the first email that they already have an
			// account -- and the "new" one would welcome them a second
			// time. The reminder claims neither.
			subject = "Reminder: " + invitationSubject(data.ProjectName)
			body = notifications.RenderProjectContactInvitedReminderEmail(data)
		case identityKnown && existed:
			subject = invitationSubject(data.ProjectName)
			body = notifications.RenderProjectContactInvitedExistingEmail(data)
		default:
			// identityKnown is false with identity disabled, and when an
			// earlier attempt at this version already provisioned the
			// account (see identityAlreadyProvisioned): either way neither
			// "created for you" nor "you already have one" can be claimed.
			data.AccountCreated = identityKnown
			subject = invitationSubject(data.ProjectName)
			body = notifications.RenderProjectContactInvitedNewEmail(data)
		}
		if err := d.onboarding.Email.SendEmailFrom(ctx, d.onboarding.EmailFrom, to, nil, nil, d.onboarding.ReplyTo, subject, body, nil); err != nil {
			err = fmt.Errorf("dispatch: send invitation for membership %s: %w", p.MembershipSfID, err)
			d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepFailed, err)
			return err
		}
		d.recordOnboardingStep(ctx, p, entity.OnboardingStepEmail, entity.OnboardingStepSucceeded, nil)
		slog.InfoContext(ctx, "dispatch: invitation email sent", append(logAttrs, "existingAccount", d.onboarding.IdentityEnabled && existed, "resend", p.IsResend)...)
	}

	return nil
}

// invitationSubject is the subject of the new and existing-account invitations.
func invitationSubject(projectName string) string {
	return "Invitation To Use WSO2 Support for " + projectName
}

// handleProjectContactRegistered sends the Welcome email after a contact's
// first sign-in. Same flags as the invitation; WELCOME_EMAIL guards duplicates.
func (d *Dispatcher) handleProjectContactRegistered(ctx context.Context, record eventbus.Record, raw json.RawMessage) error {
	var p events.ProjectContactRegisteredPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode project_contact.registered payload: %w", err)
	}
	logAttrs := []any{"membershipSfId", p.MembershipSfID, "contactSfId", p.ContactSfID, "projectKey", p.ProjectKey}
	recordStep := func(status entity.OnboardingStepStatus, lastErr error) {
		d.writeOnboardingStep(ctx, entity.OnboardingStepRequest{
			MembershipSfID:  p.MembershipSfID,
			Step:            entity.OnboardingStepWelcomeEmail,
			Status:          status,
			EventType:       string(events.TypeProjectContactRegistered),
			EventModifiedOn: parseEventModifiedOn(p.EventModifiedOn),
			Email:           p.Email,
			ContactSfID:     p.ContactSfID,
		}, lastErr)
	}
	skip := func(msg string) error {
		// A SKIPPED write may replace SUCCEEDED for the same version, so a replay must not overwrite a sent Welcome.
		if d.onboarding.Steps != nil {
			sent, err := d.onboarding.Steps.SucceededStep(ctx, p.MembershipSfID, entity.OnboardingStepWelcomeEmail)
			if err != nil {
				return fmt.Errorf("dispatch: check welcome email already sent for membership %s: %w", p.MembershipSfID, err)
			}
			if sent != nil {
				slog.InfoContext(ctx, "dispatch: welcome email already recorded as sent; not recording a skip", logAttrs...)
				return nil
			}
		}
		recordStep(entity.OnboardingStepSkipped, nil)
		slog.InfoContext(ctx, msg, logAttrs...)
		return nil
	}
	fail := func(err error) error {
		recordStep(entity.OnboardingStepFailed, err)
		return err
	}

	switch {
	case p.IsIntegrationUser:
		return skip("dispatch: project_contact.registered for an integration user; welcome email skipped")
	case !d.onboarding.EmailEnabled:
		return skip("dispatch: welcome email disabled (CSM_MIGRATION_ONBOARD_EMAIL_ENABLED != true); skipping")
	case !d.emailSendingEnabled:
		return skip("dispatch: email sending disabled (EMAIL_SENDING_ENABLED=false); not sending welcome email")
	case d.emailDebugMode && len(d.emailDebugRecipients) == 0:
		return skip("dispatch: EMAIL_DEBUG_MODE=true but EMAIL_DEBUG_RECIPIENTS is empty; not sending welcome email")
	case d.onboarding.Email == nil:
		return fail(fmt.Errorf("dispatch: welcome email enabled but no email client configured"))
	case d.onboarding.Steps == nil:
		return fail(fmt.Errorf("dispatch: welcome email enabled but no onboarding-step ledger configured; cannot check whether it was already sent"))
	}
	to := []string{p.Email}
	var intendedFor string
	if d.emailDebugMode {
		slog.InfoContext(ctx, "dispatch: EMAIL_DEBUG_MODE=true; redirecting welcome email to configured debug recipients", append(logAttrs, "debugRecipientCount", len(d.emailDebugRecipients))...)
		intendedFor = p.Email
		to = d.emailDebugRecipients
	}

	key := recordBaseKey(record) + "/welcome"
	if !d.claim(key) {
		return fmt.Errorf("dispatch: welcome email for membership %s is already in progress", p.MembershipSfID)
	}
	defer d.forget(key)

	// One Welcome per membership: any SUCCEEDED row means it already went out.
	sent, err := d.onboarding.Steps.SucceededStep(ctx, p.MembershipSfID, entity.OnboardingStepWelcomeEmail)
	if err != nil {
		return fmt.Errorf("dispatch: check welcome email already sent for membership %s: %w", p.MembershipSfID, err)
	}
	if sent != nil {
		slog.InfoContext(ctx, "dispatch: welcome email already recorded as sent; not sending again", logAttrs...)
		return nil
	}

	projectName := displayProjectName(p.ProjectName, p.ProjectKey)
	body := notifications.RenderProjectContactRegisteredEmail(notifications.ProjectContactRegisteredEmailData{
		DisplayName: inviteeDisplayName(p.GivenName, p.FamilyName, p.Email),
		ProjectName: projectName,
		ProjectKey:  p.ProjectKey,
		PortalURL:   d.onboarding.PortalURL,
		IntendedFor: intendedFor,
	})
	subject := "Welcome to WSO2 Support for " + projectName
	if err := d.onboarding.Email.SendEmailFrom(ctx, d.onboarding.EmailFrom, to, nil, nil, d.onboarding.ReplyTo, subject, body, nil); err != nil {
		return fail(fmt.Errorf("dispatch: send welcome email for membership %s: %w", p.MembershipSfID, err))
	}
	recordStep(entity.OnboardingStepSucceeded, nil)
	slog.InfoContext(ctx, "dispatch: welcome email sent", logAttrs...)
	return nil
}

// invitationAlreadySent reports whether the ledger already records an
// invitation sent for this version of the membership, logging why when it
// does. It is the durable duplicate-invitation guard of
// handleProjectContactInvited.
//
// The ledger keeps one EMAIL row per membership, so "is there a SUCCEEDED
// EMAIL row" alone cannot tell a duplicate from a re-invitation: the first
// invitation would block every later one forever. What tells them apart is
// the membership version. Salesforce emits several UPDATED events per save
// and events are redelivered, but all of those carry the same (or, for a
// delayed delivery, an older) LastModifiedDate; a genuine re-invitation is a
// new save and carries a newer one. So the invitation counts as already
// sent only when the SUCCEEDED row's eventModifiedOn is not before the
// event's -- the same "not older" rule entity-service's step upsert applies.
//
// When either timestamp is missing or unparseable the versions cannot be
// compared, and this falls back to the conservative answer: any SUCCEEDED
// row means sent. A lost re-invitation can be recovered with a resend; a
// duplicate e-mail cannot be taken back.
func (d *Dispatcher) invitationAlreadySent(ctx context.Context, p events.ProjectContactInvitedPayload, logAttrs []any) (bool, error) {
	step, err := d.onboarding.Steps.SucceededEmailStep(ctx, p.MembershipSfID)
	if err != nil {
		return false, err
	}
	if step == nil {
		return false, nil
	}
	recordedOn, recordedOK := parseLedgerTime(step.EventModifiedOn)
	eventOn, eventOK := parseLedgerTime(p.EventModifiedOn)
	if !recordedOK || !eventOK {
		slog.WarnContext(ctx, "dispatch: cannot compare invitation versions (missing or unparseable eventModifiedOn); an invitation is already recorded as sent, so not sending again",
			append(logAttrs, "eventModifiedOn", p.EventModifiedOn, "recordedEventModifiedOn", step.EventModifiedOn)...)
		return true, nil
	}
	// Postgres keeps microseconds, so a timestamp with finer precision (only
	// the processing-time fallback has one) comes back rounded; compare at
	// the precision the ledger can actually hold, or a redelivery of that
	// same event could look newer than its own record.
	eventOn = eventOn.Truncate(time.Microsecond)
	if recordedOn.Before(eventOn) {
		slog.InfoContext(ctx, "dispatch: invitation recorded as sent for an older membership version; this is a re-invitation, sending",
			append(logAttrs, "eventModifiedOn", eventOn, "recordedEventModifiedOn", recordedOn)...)
		return false, nil
	}
	slog.InfoContext(ctx, "dispatch: invitation already recorded as sent for this membership version; not sending again",
		append(logAttrs, "eventModifiedOn", eventOn, "recordedEventModifiedOn", recordedOn)...)
	return true, nil
}

// identityAlreadyProvisioned reports whether the ledger already holds a
// SUCCEEDED IDENTITY step for this membership version -- the durable
// "an earlier attempt, in any process, already provisioned this account"
// signal that the in-process memo cannot give after a restart or on
// another replica. Same version rule as invitationAlreadySent; when either
// timestamp is missing or unparseable any SUCCEEDED row counts, since the
// cost of a false positive is only the neutral wording.
//
// Consulted only when its answer can change something: the invitation
// email is enabled and this is not a resend (a resend uses the reminder
// wording regardless), and a ledger is configured. A failed read is
// returned, so the record is retried rather than guessing.
func (d *Dispatcher) identityAlreadyProvisioned(ctx context.Context, p events.ProjectContactInvitedPayload, logAttrs []any) (bool, error) {
	if !d.onboarding.EmailEnabled || p.IsResend || d.onboarding.Steps == nil {
		return false, nil
	}
	step, err := d.onboarding.Steps.SucceededStep(ctx, p.MembershipSfID, entity.OnboardingStepIdentity)
	if err != nil {
		return false, err
	}
	if step == nil {
		return false, nil
	}
	recordedOn, recordedOK := parseLedgerTime(step.EventModifiedOn)
	eventOn, eventOK := parseLedgerTime(p.EventModifiedOn)
	if recordedOK && eventOK && recordedOn.Before(eventOn.Truncate(time.Microsecond)) {
		// Provisioned for an older version (a re-invitation): this
		// version's identity step is new, its answer is trustworthy.
		return false, nil
	}
	slog.InfoContext(ctx, "dispatch: identity already recorded as provisioned for this membership version; invitation will use the neutral wording", logAttrs...)
	return true, nil
}

// parseLedgerTime parses an RFC 3339 eventModifiedOn. An empty, malformed or
// zero value (entity-service marshals an unset time.Time as
// 0001-01-01T00:00:00Z) is reported as absent.
func parseLedgerTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil || ts.IsZero() {
		return time.Time{}, false
	}
	return ts.UTC(), true
}

// recordOnboardingStep writes one step's outcome to entity-service's
// onboarding-step ledger (PUT /onboarding-steps/{membershipSfId}/{step}),
// best-effort: a failure to record is logged at ERROR and otherwise
// ignored. It is synchronous on purpose — the volume is a handful of
// invitations a day and the two writes per record keep IDENTITY recorded
// before EMAIL is attempted — but bounded by recordOnboardingStepTimeout
// so a slow ledger cannot hold the record for long. It must never mask the primary outcome — a step that genuinely
// succeeded must not turn into a retried (and, for email, re-sent) record
// because the ledger was briefly unreachable, and a step that failed must
// return its own error, not the ledger's. lastErr, when non-nil, becomes
// the row's lastError (entity-service keeps it only for a FAILED status).
//
// EventModifiedOn is the payload's eventModifiedOn — the Salesforce
// LastModifiedDate of the membership version this event describes. entity-
// service only applies a step write whose eventModifiedOn is not older than
// the row's stored one, so a delayed delivery of an older invitation cannot
// overwrite the outcome recorded for a newer one, while retries of the same
// version (equal timestamps) still land. Only when the payload carries no
// timestamp (entity-service could not parse the Salesforce date) does this
// fall back to the processing time.
func (d *Dispatcher) recordOnboardingStep(ctx context.Context, p events.ProjectContactInvitedPayload, step entity.OnboardingStep, status entity.OnboardingStepStatus, lastErr error) {
	d.writeOnboardingStep(ctx, entity.OnboardingStepRequest{
		MembershipSfID:  p.MembershipSfID,
		Step:            step,
		Status:          status,
		EventType:       string(events.TypeProjectContactInvited),
		EventModifiedOn: parseEventModifiedOn(p.EventModifiedOn),
		Email:           p.Email,
		ContactSfID:     p.ContactSfID,
	}, lastErr)
}

// writeOnboardingStep is recordOnboardingStep's event-agnostic core.
func (d *Dispatcher) writeOnboardingStep(ctx context.Context, req entity.OnboardingStepRequest, lastErr error) {
	if d.onboarding.Steps == nil {
		slog.WarnContext(ctx, "dispatch: no onboarding-step recorder configured; step outcome not recorded",
			"membershipSfId", req.MembershipSfID, "step", req.Step, "status", req.Status)
		return
	}
	if lastErr != nil {
		req.LastError = lastErr.Error()
	}
	// Detached from the handler's context on purpose. A shutdown or a
	// consumer-group rebalance cancels ctx, and it would cancel this write
	// too -- losing the EMAIL=SUCCEEDED row for an e-mail that has already
	// gone out. The same shutdown is likely to lose the offset commit, so
	// the record comes back on restart, finds no record of the send, and
	// invites the person twice. One cause, both failures, which is exactly
	// the coincidence the ledger check relies on being rare. The timeout
	// still bounds it, so a hung ledger cannot hold the handler.
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordOnboardingStepTimeout)
	defer cancel()
	if err := d.onboarding.Steps.RecordOnboardingStep(recordCtx, req); err != nil {
		slog.ErrorContext(ctx, "dispatch: failed to record onboarding step; continuing",
			"membershipSfId", req.MembershipSfID, "step", req.Step, "status", req.Status, "err", apierror.Summary(err))
		return
	}
	slog.InfoContext(ctx, "dispatch: onboarding step recorded", "membershipSfId", req.MembershipSfID, "step", req.Step, "status", req.Status)
}

// recordOnboardingStepTimeout bounds one best-effort ledger write.
const recordOnboardingStepTimeout = 5 * time.Second

// parseEventModifiedOn returns the payload's Salesforce timestamp, or
// the processing time when the payload has none (events.Validate has already
// rejected a malformed one).
func parseEventModifiedOn(eventModifiedOn string) time.Time {
	if eventModifiedOn != "" {
		if ts, err := time.Parse(time.RFC3339Nano, eventModifiedOn); err == nil {
			return ts.UTC()
		}
	}
	return time.Now().UTC()
}

// rememberIdentityExisted/rememberedIdentityExisted/forgetIdentityExisted
// are Dispatcher.identityExisted's accessors — see that field's doc comment.
func (d *Dispatcher) rememberIdentityExisted(key string, existed bool) {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	d.identityExisted[key] = existed
}

func (d *Dispatcher) rememberedIdentityExisted(key string) (existed, ok bool) {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	existed, ok = d.identityExisted[key]
	return existed, ok
}

func (d *Dispatcher) forgetIdentityExisted(key string) {
	d.doneMu.Lock()
	defer d.doneMu.Unlock()
	delete(d.identityExisted, key)
}

// inviteeDisplayName is how the invitation addresses its reader: the
// Salesforce given and family names joined, or — since Salesforce doesn't
// require a first name and test data frequently has neither — the email's
// local part (the part before "@"), which is at least recognisably theirs.
func inviteeDisplayName(givenName, familyName, email string) string {
	if name := strings.TrimSpace(strings.TrimSpace(givenName) + " " + strings.TrimSpace(familyName)); name != "" {
		return name
	}
	if local, _, ok := strings.Cut(email, "@"); ok && local != "" {
		return local
	}
	return email
}

// displayProjectName is the project as the invitation names it: the
// Salesforce project name, falling back to its key, then to a generic
// phrase, so neither the subject nor the body ever has an empty slot.
func displayProjectName(projectName, projectKey string) string {
	if projectName != "" {
		return projectName
	}
	if projectKey != "" {
		return projectKey
	}
	return "your project"
}
