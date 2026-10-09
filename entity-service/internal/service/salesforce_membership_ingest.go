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

package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// maxOnboardingStepErrorChars bounds onboarding_step.last_error, counted in
// characters (runes) so a cut never splits a multi-byte sequence.
const maxOnboardingStepErrorChars = 1000

// truncateOnboardingStepError shortens msg to maxOnboardingStepErrorChars
// characters on a rune boundary.
func truncateOnboardingStepError(msg string) string {
	if utf8.RuneCountInString(msg) <= maxOnboardingStepErrorChars {
		return msg
	}
	return string([]rune(msg)[:maxOnboardingStepErrorChars])
}

// publishInvitedTimeout bounds the Event Hub round trip so it cannot eat the
// request's own timeout; the database write has already committed by then.
const publishInvitedTimeout = 5 * time.Second

// handleProjectContactEvent is the Project_Contact__c branch of HandleEvent.
func (s *salesforceEventService) handleProjectContactEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if !s.membership.enabled() {
		slog.InfoContext(ctx, "salesforce: membership ingest disabled, ignoring project contact event",
			"eventType", req.EventType, "entity", req.Entity, "referenceId", req.ReferenceID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: project contact event", "eventType", req.EventType, "entity", req.Entity, "referenceId", req.ReferenceID)

	switch req.EventType {
	case domain.SalesforceEventCreated, domain.SalesforceEventUpdated, domain.SalesforceEventRestored:
		return s.ingestMembership(ctx, req.ReferenceID, req.EventType, nil)
	case domain.SalesforceEventDeleted:
		found, affected, err := s.membership.Memberships.DeactivateBySfID(ctx, req.ReferenceID, s.adminRoleBasis)
		if err != nil {
			return err
		}
		for _, u := range affected {
			invalidateUser(ctx, s.membership.UserCache, u.ID, u.Email)
		}
		if !found {
			slog.InfoContext(ctx, "salesforce: DELETED project contact was never ingested, nothing to deactivate", "referenceId", req.ReferenceID)
		}
		return nil
	case domain.SalesforceEventUndefined:
		return &apierror.ValidationError{Msg: "eventType UNDEFINED is not supported"}
	default:
		return &apierror.ValidationError{Msg: "eventType must be CREATED, UPDATED, DELETED, RESTORED, or UNDEFINED"}
	}
}

// handleContactEvent is the Contact branch of HandleEvent. CREATED, UPDATED
// and RESTORED run the Contact writer (salesforce_contact_ingest.go), which
// writes the contact's "user" and account_contact rows whether or not it has
// a membership, and then re-run the membership upsert for each of its
// memberships so project_contact and the groups follow. DELETED deactivates
// the contact's rows without fetching it (Salesforce hides deleted records
// from reads); its memberships are deleted in Salesforce too, each with its
// own event.
func (s *salesforceEventService) handleContactEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if !s.membership.enabled() {
		slog.InfoContext(ctx, "salesforce: membership ingest disabled, ignoring contact event",
			"eventType", req.EventType, "entity", req.Entity, "referenceId", req.ReferenceID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: contact event", "eventType", req.EventType, "entity", req.Entity, "referenceId", req.ReferenceID)

	switch req.EventType {
	case domain.SalesforceEventCreated, domain.SalesforceEventUpdated, domain.SalesforceEventRestored:
		return s.ingestContact(ctx, req.ReferenceID, req.EventType)
	case domain.SalesforceEventDeleted:
		return s.deactivateContact(ctx, req.ReferenceID)
	case domain.SalesforceEventUndefined:
		return &apierror.ValidationError{Msg: "eventType UNDEFINED is not supported"}
	default:
		return &apierror.ValidationError{Msg: "eventType must be CREATED, UPDATED, DELETED, RESTORED, or UNDEFINED"}
	}
}

// ingestMembership fetches one Project_Contact__c (and its Contact) from
// sales-entity-service and writes it to Postgres. It is idempotent: the
// repository resolves every row by natural key, and a replay whose
// LastModifiedDate is not newer than the recorded DATABASE step is skipped.
//
// known is the Contact when a Contact event drives the upsert (nil
// otherwise). It is used instead of a second fetch when it is the
// membership's own contact, and its LastModifiedDate joins the guard: a
// contact edit does not touch the membership's LastModifiedDate, so the
// guard compares the later of the two.
func (s *salesforceEventService) ingestMembership(ctx context.Context, membershipSfID, eventType string, known *salesentity.Contact) error {
	pc, err := s.membership.SalesEntity.GetProjectContact(ctx, membershipSfID)
	if err != nil {
		return err
	}
	if pc.Contact == nil || strings.TrimSpace(derefString(pc.Contact.ID)) == "" {
		return &apierror.ServiceUnavailableError{Msg: "sales/sales-entity-service project contact " + membershipSfID + " has no linked contact"}
	}
	if pc.Subscription == nil || (strings.TrimSpace(derefString(pc.Subscription.Key)) == "" && strings.TrimSpace(derefString(pc.Subscription.ID)) == "") {
		return &apierror.ServiceUnavailableError{Msg: "sales/sales-entity-service project contact " + membershipSfID + " has no linked project"}
	}
	contactSfID := strings.TrimSpace(derefString(pc.Contact.ID))
	// A partner contact's membership refreshes the project account's partner
	// links on every exit, duplicate or not (salesforce_partner_ingest.go).
	defer s.refreshPartnersForMembership(ctx, membershipSfID, pc)
	if known != nil && !strings.EqualFold(strings.TrimSpace(derefString(known.ID)), contactSfID) {
		// Not this membership's contact as spelled here (a 15/18-character
		// Id pair, or Salesforce moved it between the two reads): read the
		// one the membership names rather than guess.
		known = nil
	}
	var contactModified *string
	if known != nil {
		contactModified = known.LastModifiedDate
	}

	// Duplicate-event guard. Salesforce emits several UPDATED events per save
	// and the portal replays the envelope after its own write, so the same
	// membership version arrives more than once. eventModifiedOn is the
	// record's LastModifiedDate (or the contact's, when a Contact event
	// carries a later one); when unparseable the guard is skipped and
	// the (idempotent) upsert simply runs again. A step last touched by a
	// DELETED event never counts: an undelete (RESTORED) keeps the record's
	// LastModifiedDate, and the row must leave DEACTIVATED.
	eventModifiedOn, hasModified := parseSalesforceLastModified(pc.LastModifiedDate)
	if contactOn, ok := parseSalesforceLastModified(contactModified); ok && (!hasModified || contactOn.After(eventModifiedOn)) {
		eventModifiedOn, hasModified = contactOn, true
	}
	if !hasModified {
		slog.WarnContext(ctx, "salesforce: project contact has no parseable lastModifiedDate, skipping duplicate guard",
			"membershipSfId", membershipSfID, "lastModifiedDate", derefString(pc.LastModifiedDate))
		eventModifiedOn = time.Now().UTC()
	} else {
		steps, err := s.membership.Steps.GetByMembership(ctx, membershipSfID)
		if err != nil {
			return err
		}
		for _, st := range steps {
			if st.Step == domain.OnboardingStepDatabase && st.Status == domain.OnboardingStepSucceeded &&
				st.EventType != string(domain.SalesforceEventDeleted) && !st.EventModifiedOn.Before(eventModifiedOn) {
				slog.InfoContext(ctx, "salesforce: project contact version already ingested, skipping",
					"membershipSfId", membershipSfID, "eventModifiedOn", eventModifiedOn, "recordedOn", st.EventModifiedOn)
				return nil
			}
		}
	}

	var contact salesentity.Contact
	if known != nil {
		contact = *known
	} else {
		contact, err = s.membership.SalesEntity.GetContact(ctx, contactSfID)
		if err != nil {
			return err
		}
	}

	in, ignored, err := buildMembershipUpsert(pc, contact)
	if err != nil {
		return err
	}
	if len(ignored) > 0 {
		// One line per membership. The six labels with no CSM group are
		// dropped by decision (D2); anything else is a vocabulary gap.
		byDecision, unknown := splitIgnoredRoles(ignored)
		if len(unknown) > 0 {
			slog.WarnContext(ctx, "salesforce: project contact carries roles the ingest does not map",
				"membershipSfId", membershipSfID, "unknownRoles", unknown, "ignoredRoles", byDecision)
		} else {
			slog.InfoContext(ctx, "salesforce: project contact carries roles CSM does not store",
				"membershipSfId", membershipSfID, "ignoredRoles", byDecision)
		}
	}

	step := domain.UpsertOnboardingStepRequest{
		MembershipSfID:  membershipSfID,
		Step:            domain.OnboardingStepDatabase,
		Status:          domain.OnboardingStepSucceeded,
		EventType:       eventType,
		EventModifiedOn: eventModifiedOn,
		Email:           in.Email,
		ContactSfID:     &contactSfID,
		UpdatedBy:       domain.SalesforceSyncActor,
	}
	res, err := s.membership.Memberships.Upsert(ctx, in, step)
	if err != nil && s.ensureMissingProjectForMembership(ctx, err, membershipSfID, in.ProjectSfID) {
		res, err = s.membership.Memberships.Upsert(ctx, in, step)
	}
	if err != nil {
		s.recordDatabaseStepFailed(ctx, step, err)
		return err
	}
	invalidateUser(ctx, s.membership.UserCache, res.UserID, in.Email)
	slog.InfoContext(ctx, "salesforce: project contact ingested",
		"membershipSfId", membershipSfID, "state", in.State, "projectId", res.ProjectID, "projectContactId", res.ProjectContactID,
		"createdUser", res.CreatedUser, "createdAccountContact", res.CreatedAccountContact, "createdProjectContact", res.CreatedProjectContact)

	// ECHO SUPPRESSION. Publish only when this event MOVED the membership
	// into an invited state — the row was created here, or its stored state
	// was something else before this upsert overwrote it.
	//
	// Every portal membership write also writes Salesforce, and every
	// Salesforce write comes back to us through the Service Bus subscriber
	// as an ordinary CREATED/UPDATED envelope. The portal has already
	// written the new state by the time its echo lands, so the echo finds
	// PreviousState equal to the state it carries and stays silent.
	// Publishing there would have csm-notification-service send a SECOND
	// invitation email for the one invitation the customer admin sent.
	//
	// The gate is the TRANSITION, not the row insert. Row creation alone is
	// not a reliable echo signal: a re-invitation made in Salesforce moves
	// an existing DEACTIVATED row to RE-INVITED, so nothing is created and
	// the person would never be told — the previous state is what tells
	// that apart from our own write returning. A genuinely
	// Salesforce-originated first invitation (or the historical backfill)
	// still creates the row and still publishes. A membership that lands
	// REGISTERED publishes only when it is new (see addedAsRegistered).
	invited := in.State == domain.MembershipStateInvited || in.State == domain.MembershipStateReInvited
	movedIntoInvited := res.CreatedProjectContact || !strings.EqualFold(res.PreviousState, in.State)
	if (invited && movedIntoInvited) || addedAsRegistered(in.State, eventType, res, pc.CreatedDate) {
		s.publishProjectContactInvited(ctx, in, pc, eventModifiedOn, hasModified)
	} else if invited {
		slog.InfoContext(ctx, "salesforce: membership already in this state, not re-publishing project_contact.invited",
			"membershipSfId", membershipSfID, "state", in.State)
	}
	// Welcome only on INVITED/RE-INVITED -> REGISTERED: echoes, replays and
	// backfilled rows (created already REGISTERED) stay silent.
	if strings.EqualFold(in.State, domain.MembershipStateRegistered) && !res.CreatedProjectContact && wasInvitedState(res.PreviousState) {
		s.publishProjectContactRegistered(ctx, in, pc, eventModifiedOn, hasModified)
	}
	return nil
}

// newMembershipWindow bounds how old a Salesforce record created REGISTERED
// may be and still count as a new membership rather than a backfill.
const newMembershipWindow = 24 * time.Hour

// addedAsRegistered reports a NEW membership that Salesforce saved REGISTERED
// because the contact is already unlocked (has signed in before).
func addedAsRegistered(state, eventType string, res domain.SalesforceMembershipUpsertResult, createdDate *string) bool {
	if !strings.EqualFold(state, domain.MembershipStateRegistered) || eventType == domain.SalesforceEventRestored {
		return false
	}
	if !res.CreatedProjectContact {
		return strings.EqualFold(res.PreviousState, domain.MembershipStateDeactivated)
	}
	created, ok := parseSalesforceLastModified(createdDate)
	return ok && time.Since(created) < newMembershipWindow
}

func wasInvitedState(state string) bool {
	return strings.EqualFold(state, domain.MembershipStateInvited) || strings.EqualFold(state, domain.MembershipStateReInvited)
}

// publishProjectContactRegistered emits project_contact.registered for the
// Welcome email. Failures are logged, like publishProjectContactInvited.
func (s *salesforceEventService) publishProjectContactRegistered(ctx context.Context, in domain.SalesforceMembershipUpsert, pc salesentity.ProjectContact, eventModifiedOn time.Time, hasModified bool) {
	ctx, cancelNotify := detachedNotifyContext(ctx)
	defer cancelNotify()
	if s.membership.Publisher == nil {
		return
	}
	modifiedOn := ""
	if hasModified {
		modifiedOn = eventModifiedOn.UTC().Format(time.RFC3339Nano)
	}
	payload, err := json.Marshal(events.ProjectContactRegisteredPayload{
		MembershipSfID:    in.MembershipSfID,
		ContactSfID:       in.ContactSfID,
		Email:             in.Email,
		GivenName:         in.ContactFirstName,
		FamilyName:        in.ContactLastName,
		ProjectName:       strings.TrimSpace(derefString(pc.Subscription.Name)),
		ProjectKey:        in.ProjectKey,
		IsIntegrationUser: in.IsCsIntegrationUser,
		EventModifiedOn:   modifiedOn,
	})
	if err != nil {
		slog.ErrorContext(ctx, "salesforce: encode project_contact.registered payload", "membershipSfId", in.MembershipSfID, "err", err)
		return
	}
	pubCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishInvitedTimeout)
	defer cancel()
	if err := s.membership.Publisher.Publish(pubCtx, events.TypeProjectContactRegistered, in.MembershipSfID, payload); err != nil {
		slog.ErrorContext(ctx, "salesforce: publish project_contact.registered", "membershipSfId", in.MembershipSfID, "err", err)
	}
}

// recordDatabaseStepFailed writes DATABASE=FAILED best-effort so the
// onboarding dashboard can see the membership is stuck; the original error
// is what HandleEvent returns regardless of whether this write succeeds.
func (s *salesforceEventService) recordDatabaseStepFailed(ctx context.Context, step domain.UpsertOnboardingStepRequest, cause error) {
	msg := truncateOnboardingStepError(cause.Error())
	step.Status = domain.OnboardingStepFailed
	step.LastError = &msg
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if _, err := s.membership.Steps.Upsert(recordCtx, step); err != nil {
		slog.ErrorContext(ctx, "salesforce: recording DATABASE=FAILED onboarding step also failed", "membershipSfId", step.MembershipSfID, "err", err)
	}
}

// publishProjectContactInvited emits project_contact.invited so
// csm-notification-service can provision the Asgardeo identity and send the
// invitation. Failures are logged, never returned: the database write is
// already committed and EventPublisherService records the failure durably.
func (s *salesforceEventService) publishProjectContactInvited(ctx context.Context, in domain.SalesforceMembershipUpsert, pc salesentity.ProjectContact, eventModifiedOn time.Time, hasModified bool) {
	ctx, cancelNotify := detachedNotifyContext(ctx)
	defer cancelNotify()
	if s.membership.Publisher == nil {
		return
	}
	modifiedOn := ""
	if hasModified {
		modifiedOn = eventModifiedOn.UTC().Format(time.RFC3339Nano)
	}
	roles := pc.Roles
	if len(roles) == 0 {
		roles = splitSalesforceRoles(derefString(pc.Role))
	}
	if roles == nil {
		roles = []string{}
	}
	payload, err := json.Marshal(events.ProjectContactInvitedPayload{
		MembershipSfID:    in.MembershipSfID,
		ContactSfID:       in.ContactSfID,
		Email:             in.Email,
		GivenName:         in.ContactFirstName,
		FamilyName:        in.ContactLastName,
		ProjectName:       strings.TrimSpace(derefString(pc.Subscription.Name)),
		ProjectKey:        in.ProjectKey,
		Roles:             roles,
		IsIntegrationUser: in.IsCsIntegrationUser,
		Type:              in.Type,
		EventModifiedOn:   modifiedOn,
	})
	if err != nil {
		slog.ErrorContext(ctx, "salesforce: encode project_contact.invited payload", "membershipSfId", in.MembershipSfID, "err", err)
		return
	}
	pubCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishInvitedTimeout)
	defer cancel()
	if err := s.membership.Publisher.Publish(pubCtx, events.TypeProjectContactInvited, in.MembershipSfID, payload); err != nil {
		slog.ErrorContext(ctx, "salesforce: publish project_contact.invited", "membershipSfId", in.MembershipSfID, "err", err)
	}
}

// buildMembershipUpsert translates the two sales-entity-service records into
// the repository's write request, applying the role mapping: project groups
// from the membership's roles, global roles from the contact's account
// classification. ignored lists Salesforce roles that map to nothing.
func buildMembershipUpsert(pc salesentity.ProjectContact, contact salesentity.Contact) (domain.SalesforceMembershipUpsert, []string, error) {
	state, err := normalizeMembershipState(derefString(pc.State))
	if err != nil {
		return domain.SalesforceMembershipUpsert{}, nil, err
	}

	email := strings.ToLower(strings.TrimSpace(pc.Email))
	contactEmail := strings.ToLower(strings.TrimSpace(derefString(contact.Email)))
	if contactEmail == "" {
		contactEmail = strings.ToLower(strings.TrimSpace(derefString(pc.Contact.Email)))
	}
	if email == "" {
		email = contactEmail
	}
	if contactEmail == "" {
		contactEmail = email
	}
	if email == "" {
		return domain.SalesforceMembershipUpsert{}, nil, &apierror.ValidationError{Msg: "project contact " + pc.ID + " has no email"}
	}

	roles := pc.Roles
	if len(roles) == 0 {
		roles = splitSalesforceRoles(derefString(pc.Role))
	}
	isCsAdmin := contact.IsCsAdmin != nil && *contact.IsCsAdmin
	isIntegration := contact.IsCsIntegrationUser != nil && *contact.IsCsIntegrationUser
	membershipType := strings.TrimSpace(derefString(pc.Type))

	// customer/partner (and which admin role applies) follow the contact's
	// account classification, the same basis the Contact writer uses, not
	// the membership type.
	globalRoles := mapGlobalRoles(contact.Account, isIntegration)
	groups, ignored := mapProjectGroups(roles)

	name := strings.TrimSpace(derefString(contact.Name))
	if name == "" {
		name = strings.TrimSpace(derefString(pc.Contact.Name))
	}
	first, last := strings.TrimSpace(derefString(contact.FirstName)), strings.TrimSpace(derefString(contact.LastName))
	if first == "" && last == "" && name != "" {
		first, last = splitDisplayName(name)
	}

	accountSfID := strings.TrimSpace(derefString(pc.Contact.CustomerID))
	if accountSfID == "" && contact.Account != nil {
		accountSfID = strings.TrimSpace(derefString(contact.Account.ID))
	}

	return domain.SalesforceMembershipUpsert{
		MembershipSfID:      pc.ID,
		State:               state,
		Type:                membershipType,
		Email:               email,
		ContactSfID:         strings.TrimSpace(derefString(pc.Contact.ID)),
		ContactEmail:        contactEmail,
		ContactName:         name,
		ContactFirstName:    first,
		ContactLastName:     last,
		ContactAccountSfID:  accountSfID,
		IsCsAdmin:           isCsAdmin,
		IsCsIntegrationUser: isIntegration,
		IsPrimaryContact:    contact.IsPrimaryContact,
		ProjectSfID:         strings.TrimSpace(derefString(pc.Subscription.ID)),
		ProjectKey:          strings.TrimSpace(derefString(pc.Subscription.Key)),
		GlobalRoles:         globalRoles.Grant,
		ManagedGlobalRoles:  globalRoles.ManagedGlobal,
		ManagedAdminRoles:   globalRoles.ManagedAdmin,
		AdminRoleName:       globalRoles.AdminRole,
		ProjectGroups:       groups,
	}, ignored, nil
}

// splitDisplayName is the fallback when the Contact carries only a full
// name: the last word is the family name, the rest the given name.
func splitDisplayName(name string) (first, last string) {
	parts := strings.Fields(name)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
	}
}
