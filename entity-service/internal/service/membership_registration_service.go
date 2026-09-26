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
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// membershipRegistrationSalesClient is the narrow write contract the
// registration flip needs: clear the contact's lockout flag, then move the
// membership to REGISTERED. Deliberately narrower than
// SalesEntityMembershipWriteClient (which the portal write path uses and
// which can also create records) so this path cannot create anything in
// Salesforce -- it only ever flips an existing membership.
// *salesentity.Client satisfies both.
type membershipRegistrationSalesClient interface {
	// UpdateContactLockout clears or sets the Contact's "Locked Out
	// [ Service Now ]" flag (Salesforce Contact.State__c).
	UpdateContactLockout(ctx context.Context, contactSfID string, lockedOut bool) error
	// UpdateProjectContactState writes the membership's State__c and returns
	// the record as Salesforce persisted it (a zero record when
	// sales-entity-service could not re-read it -- still a success).
	UpdateProjectContactState(ctx context.Context, membershipSfID, state string) (salesentity.ProjectContact, error)
}

type membershipRegistrationService struct {
	repo  repository.MembershipRegistrationRepository
	sales membershipRegistrationSalesClient
	// events is the same SalesforceEventService POST /salesforce/events is
	// backed by. Re-ingesting through it (rather than through a second copy
	// of the mapping) is what keeps the Postgres row, the DATABASE onboarding
	// step and the invited-event decision identical to a real Salesforce
	// envelope -- see RegisterInvitedMemberships step 4.
	events SalesforceEventService
	steps  repository.OnboardingStepRepository
}

// NewMembershipRegistrationService constructs a MembershipRegistrationService. events must be the
// membership-ingest-enabled SalesforceEventService (routes.go only builds this
// service when it is), otherwise the Salesforce flip would happen with no
// matching database write.
func NewMembershipRegistrationService(
	repo repository.MembershipRegistrationRepository,
	sales membershipRegistrationSalesClient,
	events SalesforceEventService,
	steps repository.OnboardingStepRepository,
) MembershipRegistrationService {
	return &membershipRegistrationService{repo: repo, sales: sales, events: events, steps: steps}
}

// RegisterInvitedMemberships implements MembershipRegistrationService.
func (s *membershipRegistrationService) RegisterInvitedMemberships(ctx context.Context) error {
	// Same identity resolution as GET /users/me: the already-validated
	// x-user-id-token's email claim. No token is a 401; an undecodable one a
	// 400 -- the convention userService.GetMe already set.
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}

	memberships, err := s.repo.InvitedMembershipsByEmail(ctx, email)
	if err != nil {
		return err
	}
	// The fast path, and by a wide margin the common one: the portal calls
	// this on every profile load and a user is only ever INVITED once per
	// membership. Nothing is logged and Salesforce is never touched.
	if len(memberships) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "register memberships: flipping invited memberships to REGISTERED", "membershipCount", len(memberships))

	var firstErr error
	failed := 0
	for _, m := range memberships {
		if err := s.registerMembership(ctx, m, email); err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
			// Deliberately no email in this line: membership and contact ids
			// identify the record without putting a customer's address in the
			// logs.
			slog.ErrorContext(ctx, "register memberships: could not register a membership",
				"membershipSfId", m.MembershipSfID, "contactSfId", m.ContactSfID, "err", err)
		}
	}
	// A partial success is a success: the memberships that did flip are
	// registered, and the rest are retried on the caller's next profile load,
	// since they are still INVITED. Only a total failure is reported, with
	// the first error so its own status mapping survives.
	if failed == len(memberships) {
		return firstErr
	}
	return nil
}

// registerMembership performs the whole per-membership attempt and records its
// single REGISTRATION onboarding step. The three writes are guarded as one
// unit: any of them failing leaves the step FAILED, since the membership is
// only really registered when Salesforce has been flipped and Postgres has
// caught up.
func (s *membershipRegistrationService) registerMembership(ctx context.Context, m repository.InvitedMembership, email string) error {
	persisted, err := s.flipMembership(ctx, m)
	s.recordRegistrationStep(ctx, m, email, persisted, err)
	return err
}

// flipMembership performs the Salesforce writes and the re-ingest, in the one
// order that works.
//
// ORDER IS LOAD-BEARING. Salesforce's SN_T_Project_Contact trigger recomputes
// Project_Contact__c.State__c from the Contact's lockout flag on EVERY save
// (locked out -> INVITED, not locked out -> REGISTERED, unless the membership
// is DEACTIVATED). Setting the state first and clearing the flag second would
// have the trigger overwrite REGISTERED back to INVITED on the state write's
// own save. Do not reorder these two calls.
func (s *membershipRegistrationService) flipMembership(ctx context.Context, m repository.InvitedMembership) (salesentity.ProjectContact, error) {
	if err := s.sales.UpdateContactLockout(ctx, m.ContactSfID, false); err != nil {
		// The state update is skipped entirely: with the flag still set the
		// trigger would just revert it, so the call would be a no-op that
		// looked like a success.
		return salesentity.ProjectContact{}, err
	}
	persisted, err := s.sales.UpdateProjectContactState(ctx, m.MembershipSfID, domain.MembershipStateRegistered)
	if err != nil {
		return salesentity.ProjectContact{}, err
	}

	// Re-ingest so Postgres matches Salesforce before this request returns,
	// rather than whenever the ASB envelope for the same save arrives. This
	// goes through the same SalesforceEventService entry point the ASB
	// subscriber posts to, with the envelope Salesforce itself would have
	// emitted, so the mapping, the DATABASE step and the duplicate guard are
	// literally the same code -- none of it is reimplemented here.
	if err := s.events.HandleEvent(ctx, domain.SalesforceEventRequest{
		EventType:   domain.SalesforceEventUpdated,
		Entity:      domain.SalesforceEntityProjectContact,
		ReferenceID: m.MembershipSfID,
	}); err != nil {
		return persisted, err
	}
	return persisted, nil
}

// recordRegistrationStep writes REGISTRATION = SUCCEEDED / FAILED for the
// membership, best-effort: the Salesforce flip has already happened by the
// time this runs, so a failure to record it is logged, never returned.
// eventModifiedOn is the persisted record's own LastModifiedDate when
// Salesforce gave one back, so the row is stamped with the same Salesforce
// version the ingest's DATABASE step is.
func (s *membershipRegistrationService) recordRegistrationStep(
	ctx context.Context, m repository.InvitedMembership, email string,
	persisted salesentity.ProjectContact, cause error,
) {
	eventModifiedOn, ok := parseSalesforceLastModified(persisted.LastModifiedDate)
	if !ok {
		eventModifiedOn = time.Now().UTC()
	}
	contactSfID := m.ContactSfID
	step := domain.UpsertOnboardingStepRequest{
		MembershipSfID:  m.MembershipSfID,
		Step:            domain.OnboardingStepRegistration,
		Status:          domain.OnboardingStepSucceeded,
		EventType:       domain.SalesforceEventUpdated,
		EventModifiedOn: eventModifiedOn,
		Email:           email,
		ContactSfID:     &contactSfID,
		UpdatedBy:       domain.SalesforceSyncActor,
	}
	if cause != nil {
		msg := truncateOnboardingStepError(cause.Error())
		step.Status = domain.OnboardingStepFailed
		step.LastError = &msg
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordRegistrationStepTimeout)
	defer cancel()
	if _, err := s.steps.Upsert(recordCtx, step); err != nil {
		slog.ErrorContext(ctx, "register memberships: recording the REGISTRATION onboarding step failed",
			"membershipSfId", m.MembershipSfID, "status", step.Status, "err", err)
	}
}

// recordRegistrationStepTimeout bounds the step write the same way
// recordDatabaseStepFailed bounds its own: the Salesforce side is already
// committed, so this must not be what makes the request time out.
const recordRegistrationStepTimeout = 3 * time.Second
