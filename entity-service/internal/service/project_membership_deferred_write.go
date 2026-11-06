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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// DEFERRED SALESFORCE WRITES.
//
// Every portal membership write is half a Postgres transaction and half a
// Salesforce call, and the Salesforce half runs first (see
// NewProjectMembershipWriteService). When sales/sales-entity-service is
// unavailable that ordering means the whole invitation fails with 503 and the
// customer admin can do nothing about it. This file is the alternative for
// the one operation that can live without the Salesforce ids for a while, a
// FIRST invitation:
//
//  1. Invite tries Salesforce synchronously as always. Only when the call
//     fails with ServiceUnavailableError (connection error, timeout, 5xx) does
//     it fall through: the rows are committed with NULL sf_id columns and a
//     salesforce_write_request row (migration 0249) in the same transaction,
//     and salesforce.membership_write.requested -- carrying the admin's
//     intent and the project's and account's Salesforce ids -- is published
//     for sales/sales-entity-service. The admin gets 201 with salesforceSync
//     PENDING. Anything else Salesforce says (a validation refusal, a
//     conflict) still fails the request, because a retry would not change it.
//  2. Sales Entity, the service that owns the Salesforce connection, consumes
//     the command (its own two retry tiers), performs the write search-first
//     so a redelivery adopts rather than duplicates, and publishes
//     salesforce.membership_write.completed or .failed.
//  3. CompleteDeferredSalesforceWrite, driven by the results consumer in
//     internal/server, re-reads the request (settled → acknowledge, which is
//     what makes a duplicate result harmless), reads the records back through
//     Sales Entity's REST API exactly as the Salesforce event ingest does,
//     and in ONE transaction stamps the ids, records the DATABASE step and
//     marks the request SUCCEEDED; only then is project_contact.invited
//     published -- the onboarding ledger that the invitation email and the
//     identity provisioning record into is keyed by the Salesforce membership
//     id, so the email waits for the id. FailDeferredSalesforceWrite marks
//     the request FAILED from a .failed result.
//  4. While the request is PENDING, role changes, deactivation and resend are
//     refused with 409 salesforce_sync_pending: they need the ids. Once it is
//     FAILED (or was never published) role changes and resend answer 409
//     salesforce_sync_failed, and deactivation runs locally, closing the
//     request as CANCELLED: there is no Salesforce half to undo.
//
// The request row is the durable record; the events are only the triggers.
// A request whose result never arrives sits PENDING for an operator, a
// manual retry or a future sweep to re-publish from (and the Salesforce echo
// through Service Bus still stamps the id meanwhile).

// msgSalesforceSyncPending is what the caller is told when a write needs the
// Salesforce ids a still-pending deferred write has not produced yet. 409,
// like the other refusals of this kind, because retrying later WILL help and
// the message says so. apierror.CodeSalesforceSyncPending is its code.
const msgSalesforceSyncPending = "This contact is still being synced to Salesforce. Please try again in a few minutes."

// msgSalesforceSyncFailed is the refusal of a role change or resend on a row
// whose deferred write ended FAILED or was never published: nothing is coming
// that would produce the ids. Removing the contact is allowed (it runs
// locally), which is what the message steers to.
// apierror.CodeSalesforceSyncFailed is its code.
const msgSalesforceSyncFailed = "This contact could not be synced to Salesforce. Remove them from the project and invite them again, or contact WSO2 support."

// deferredWriteState is what a write on a row with no Salesforce id finds in
// the request ledger.
type deferredWriteState int

const (
	// deferredNone: no request at all -- the data-migration gap
	// requireSalesforceLinks reports.
	deferredNone deferredWriteState = iota
	// deferredPending: a published request Sales Entity is working on.
	deferredPending
	// deferredStuck: FAILED, or PENDING but never published; nothing is
	// coming without a re-publish.
	deferredStuck
)

// DeferredSalesforceWriteCompleter is the results-consumer-facing half of the
// deferred write; *projectMembershipWriteService implements it.
type DeferredSalesforceWriteCompleter interface {
	// CompleteDeferredSalesforceWrite finishes one request from Sales
	// Entity's completed result: the ids are stamped on the rows, the
	// DATABASE step recorded, the request marked SUCCEEDED and the
	// invitation published. A nil return acknowledges the record: done, or
	// the request was already settled, or it failed for good and the row
	// says so. A non-nil return means Sales Entity's REST API or the
	// database was unavailable and the caller should retry; lastAttempt
	// says nothing will retry after this call, so the request is closed as
	// FAILED (with the ids in the reason) rather than left PENDING forever.
	CompleteDeferredSalesforceWrite(ctx context.Context, result events.SalesforceMembershipWriteCompletedPayload, lastAttempt bool) error
	// FailDeferredSalesforceWrite marks the request FAILED from Sales
	// Entity's failed result. Idempotent: a settled request is left alone.
	FailDeferredSalesforceWrite(ctx context.Context, failed events.SalesforceMembershipWriteFailedPayload) error
}

// deferralAvailable reports whether this deployment defers at all: the
// request ledger and the command publisher are both wired (see routes.go,
// gated on Config.HasSalesforceDeferredWrites).
func (s *projectMembershipWriteService) deferralAvailable() bool {
	return s.deps.WriteRequests != nil && s.deps.SalesforceWritePublisher != nil
}

// isSalesforceUnavailable is the one error class a deferred write absorbs:
// the sales-entity client maps connection errors and 5xx answers to
// ServiceUnavailableError, and so does an answer with no id in it. A
// validation refusal (DownstreamError / ValidationError) or a conflict is
// not an outage and is never deferred.
//
// A TIMEOUT of the Sales Entity call counts as an outage too, with one
// condition: the admin's own request must still be alive (ctx.Err() == nil).
// The client returns a deadline error unwrapped so a caller that has gone
// away is not mistaken for a downstream fault; but when Salesforce is down,
// sales-entity-service retries its own Salesforce calls with backoff for
// twenty-odd seconds before answering 5xx, which is longer than the client's
// timeout -- so what entity-service actually sees during a Salesforce outage
// is a timeout, not a 503 (observed on the local stack). A timeout with the
// request still open is therefore the outage signature, and is deferred.
func isSalesforceUnavailable(ctx context.Context, err error) bool {
	var sue *apierror.ServiceUnavailableError
	if errors.As(err, &sue) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return true
	}
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout() && ctx.Err() == nil
}

// canDeferInvite says whether this failed invitation may be committed
// locally instead: deferral is wired, the error is an outage, and this is a
// FIRST invitation. A re-invitation (wc.Existing is a DEACTIVATED row) has
// Salesforce ids already and would be an UPDATE there; deferring it would
// leave the CSM state ahead of Salesforce's for a membership the
// registration flow can already see, so it keeps failing fast.
func (s *projectMembershipWriteService) canDeferInvite(ctx context.Context, wc repository.MembershipWriteContext, err error) bool {
	return s.deferralAvailable() && wc.Existing == nil && isSalesforceUnavailable(ctx, err)
}

// deferredInviteUpsert is the database half of an invitation whose
// Salesforce half did not run: the same rows the synchronous path writes,
// with no Salesforce ids, as an own contact of the project's account (which
// is what an invitation asks for; a partner contact's true account is
// resolved when the write reaches Salesforce, and that upsert corrects the
// type and roles). The request row is written alongside, in the transaction.
func deferredInviteUpsert(wc repository.MembershipWriteContext, intent domain.SalesforceWriteIntent) domain.SalesforceMembershipUpsert {
	groups, _ := mapProjectGroups(intent.Roles)
	accountSfID := wc.Target.AccountSfID
	roles := mapGlobalRoles(&salesentity.ContactAccount{ID: &accountSfID}, intent.IsCsIntegrationUser)
	name := strings.TrimSpace(strings.TrimSpace(intent.FirstName) + " " + strings.TrimSpace(intent.LastName))
	return domain.SalesforceMembershipUpsert{
		State:               intent.State,
		Type:                domain.MembershipTypeOwnContact,
		Email:               intent.Email,
		Actor:               domain.PortalMembershipWriteActor,
		ProjectID:           wc.Target.ProjectID,
		ContactEmail:        intent.Email,
		ContactName:         name,
		ContactFirstName:    strings.TrimSpace(intent.FirstName),
		ContactLastName:     strings.TrimSpace(intent.LastName),
		ContactAccountSfID:  wc.Target.AccountSfID,
		IsCsIntegrationUser: intent.IsCsIntegrationUser,
		ProjectSfID:         wc.Target.ProjectSfID,
		ProjectKey:          wc.Target.ProjectKey,
		GlobalRoles:         roles.Grant,
		ManagedGlobalRoles:  roles.ManagedGlobal,
		ManagedAdminRoles:   roles.ManagedAdmin,
		AdminRoleName:       roles.AdminRole,
		ProjectGroups:       groups,
		DeferredSalesforceWrite: &domain.DeferredSalesforceWrite{
			Operation: domain.SalesforceWriteOperationInvite,
			Intent:    intent,
		},
	}
}

// publishDeferredWrite publishes the command for a request row the
// transaction just committed, and stamps published_on when Event Hub took
// it. The payload carries the whole intent and the Salesforce ids of the
// project and account, because Sales Entity cannot read this database. A
// failed publish is recorded by the publisher (event_publish_failures) and
// leaves published_on NULL: the row is still owed, and the operator view can
// tell "never handed to Sales Entity" from "Sales Entity is retrying".
func (s *projectMembershipWriteService) publishDeferredWrite(ctx context.Context, requestID, projectID, projectContactID string, intent domain.SalesforceWriteIntent, target domain.MembershipWriteTarget) {
	payload, err := json.Marshal(events.SalesforceMembershipWriteRequestedPayload{
		RequestID:           requestID,
		Operation:           domain.SalesforceWriteOperationInvite,
		ProjectID:           projectID,
		ProjectContactID:    projectContactID,
		ProjectSfID:         target.ProjectSfID,
		AccountSfID:         target.AccountSfID,
		Email:               intent.Email,
		FirstName:           intent.FirstName,
		LastName:            intent.LastName,
		Roles:               intent.Roles,
		State:               intent.State,
		IsCsIntegrationUser: intent.IsCsIntegrationUser,
		InviterEmail:        intent.InviterEmail,
		AllowedAccountSfIDs: intent.AllowedAccountSfIDs,
		RequestedOn:         time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		slog.ErrorContext(ctx, "membership write: encode salesforce.membership_write.requested payload", "requestId", requestID, "err", err)
		return
	}
	pubCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishInvitedTimeout)
	defer cancel()
	if err := s.deps.SalesforceWritePublisher.Publish(pubCtx, events.TypeSalesforceMembershipWriteRequested, requestID, payload); err != nil {
		slog.ErrorContext(ctx, "membership write: publish salesforce.membership_write.requested; the request stays unpublished",
			"requestId", requestID, "projectContactId", projectContactID, "err", err)
		return
	}
	if err := s.deps.WriteRequests.MarkPublished(pubCtx, requestID); err != nil {
		slog.ErrorContext(ctx, "membership write: mark salesforce write request published", "requestId", requestID, "err", err)
	}
	slog.InfoContext(ctx, "membership write: Salesforce unavailable; invitation committed locally and handed to sales-entity-service",
		"requestId", requestID, "projectId", projectID, "projectContactId", projectContactID)
}

// deferredWrite reads the latest request for a row with no Salesforce id and
// says what state the deferred write is in. A row with an id, or no ledger,
// or a ledger read failure, answers deferredNone: the caller then falls
// through to the ordinary missing-id refusal, so a ledger problem never
// lets a write run without ids.
func (s *projectMembershipWriteService) deferredWrite(ctx context.Context, operation string, existing *domain.ProjectMembershipRow) (*domain.SalesforceWriteRequest, deferredWriteState) {
	if existing == nil || s.deps.WriteRequests == nil || strings.TrimSpace(existing.MembershipSfID) != "" {
		return nil, deferredNone
	}
	req, err := s.deps.WriteRequests.LatestForProjectContact(ctx, existing.ProjectContactID)
	if err != nil {
		slog.WarnContext(ctx, "membership write: could not read the salesforce write request ledger", "operation", operation, "projectContactId", existing.ProjectContactID, "err", err)
		return nil, deferredNone
	}
	if req == nil {
		return nil, deferredNone
	}
	switch {
	case req.Status == domain.SalesforceWriteRequestPending && req.PublishedOn != nil:
		return req, deferredPending
	case req.Status == domain.SalesforceWriteRequestPending || req.Status == domain.SalesforceWriteRequestFailed:
		return req, deferredStuck
	default:
		// SUCCEEDED with no id on the row (the row was re-created?), or
		// CANCELLED: the generic refusal applies.
		return req, deferredNone
	}
}

// refuseUnsynced is the 409 a role change or resend gets on a row whose
// deferred write is PENDING (try again later) or stuck (remove and re-invite).
// Both need the Salesforce ids. A row with no id and no request keeps the
// generic missing-id refusal from requireSalesforceLinks.
func (s *projectMembershipWriteService) refuseUnsynced(ctx context.Context, operation string, existing *domain.ProjectMembershipRow) error {
	req, state := s.deferredWrite(ctx, operation, existing)
	switch state {
	case deferredPending:
		slog.InfoContext(ctx, "membership write refused: the membership's Salesforce write is still pending",
			"operation", operation, "projectContactId", existing.ProjectContactID, "requestId", req.ID, "attempts", req.Attempts)
		return &apierror.ConflictError{Msg: msgSalesforceSyncPending, Code: apierror.CodeSalesforceSyncPending}
	case deferredStuck:
		slog.InfoContext(ctx, "membership write refused: the membership's Salesforce write failed or was never published",
			"operation", operation, "projectContactId", existing.ProjectContactID, "requestId", req.ID, "status", req.Status, "published", req.PublishedOn != nil)
		return &apierror.ConflictError{Msg: msgSalesforceSyncFailed, Code: apierror.CodeSalesforceSyncFailed}
	}
	return nil
}

// localDeactivationUpsert is the database half of removing a contact whose
// deferred write is stuck: the row and its groups as they are, state
// DEACTIVATED, no Salesforce ids (there are none), and the request closed as
// CANCELLED in the same transaction so nothing picks it up later.
func localDeactivationUpsert(wc repository.MembershipWriteContext, existing *domain.ProjectMembershipRow, email, requestID string) domain.SalesforceMembershipUpsert {
	accountSfID := wc.Target.AccountSfID
	roles := mapGlobalRoles(&salesentity.ContactAccount{ID: &accountSfID}, existing.IsIntegrationUser)
	return domain.SalesforceMembershipUpsert{
		State:                           domain.MembershipStateDeactivated,
		Type:                            existing.Type,
		Email:                           email,
		Actor:                           domain.PortalMembershipWriteActor,
		ProjectID:                       wc.Target.ProjectID,
		ContactEmail:                    email,
		ContactFirstName:                existing.FirstName,
		ContactLastName:                 existing.LastName,
		ContactAccountSfID:              wc.Target.AccountSfID,
		IsCsIntegrationUser:             existing.IsIntegrationUser,
		ProjectSfID:                     wc.Target.ProjectSfID,
		ProjectKey:                      wc.Target.ProjectKey,
		GlobalRoles:                     roles.Grant,
		ManagedGlobalRoles:              roles.ManagedGlobal,
		ManagedAdminRoles:               roles.ManagedAdmin,
		AdminRoleName:                   roles.AdminRole,
		ProjectGroups:                   existing.ProjectGroups,
		CancelsSalesforceWriteRequestID: requestID,
	}
}

// isTerminalCompletionError says a completion failure a retry would not
// change: the row is gone, the record Sales Entity reported does not add up,
// or the rows refuse the write. Everything else (Sales Entity REST or the
// database unavailable) is retried by the consumer.
func isTerminalCompletionError(err error) bool {
	var nfe *apierror.NotFoundError
	var ve *apierror.ValidationError
	var ce *apierror.ConflictError
	var de *apierror.DownstreamError
	return errors.As(err, &nfe) || errors.As(err, &ve) || errors.As(err, &ce) || errors.As(err, &de)
}

// CompleteDeferredSalesforceWrite implements DeferredSalesforceWriteCompleter.
func (s *projectMembershipWriteService) CompleteDeferredSalesforceWrite(ctx context.Context, result events.SalesforceMembershipWriteCompletedPayload, lastAttempt bool) error {
	if s.deps.WriteRequests == nil {
		return errors.New("deferred salesforce write: the request ledger is not configured")
	}
	req, err := s.deps.WriteRequests.Get(ctx, result.RequestID)
	if err != nil {
		var nfe *apierror.NotFoundError
		if errors.As(err, &nfe) {
			slog.WarnContext(ctx, "deferred salesforce write: completed result for a request that no longer exists; acknowledging", "requestId", result.RequestID)
			return nil
		}
		return err
	}
	logAttrs := []any{"requestId", req.ID, "projectId", req.ProjectID, "projectContactId", req.ProjectContactID,
		"membershipSfId", result.MembershipSfID, "contactSfId", result.ContactSfID, "attempts", req.Attempts}
	if req.Status != domain.SalesforceWriteRequestPending {
		slog.InfoContext(ctx, "deferred salesforce write: request already settled; acknowledging the result", append(logAttrs, "status", req.Status)...)
		return nil
	}
	if strings.TrimSpace(result.MembershipSfID) == "" || strings.TrimSpace(result.ContactSfID) == "" {
		return s.failDeferredWrite(ctx, req, errors.New("completed result carries no Salesforce ids"), false, logAttrs)
	}

	// The records as Salesforce holds them, read back the way the Salesforce
	// event ingest reads an echo: the Sales Entity REST contract, so the
	// upsert is built by the same mapping (buildMembershipUpsert) and the
	// rows end up exactly as a synchronous write would have left them.
	pc, err := s.deps.SalesEntity.GetProjectContact(ctx, result.MembershipSfID)
	if err != nil {
		return s.completionProblem(ctx, req, result, fmt.Errorf("read membership %s: %w", result.MembershipSfID, err), lastAttempt, logAttrs)
	}
	contact, err := s.deps.SalesEntity.GetContact(ctx, result.ContactSfID)
	if err != nil {
		return s.completionProblem(ctx, req, result, fmt.Errorf("read contact %s: %w", result.ContactSfID, err), lastAttempt, logAttrs)
	}
	if pc.Contact == nil || strings.TrimSpace(derefString(pc.Contact.ID)) == "" {
		pc.Contact = &salesentity.ProjectContactContact{ID: &result.ContactSfID, Email: contact.Email, Name: contact.Name}
	}
	if pc.Subscription == nil {
		pc.Subscription = &salesentity.ProjectContactSubscription{}
	}
	in, _, err := buildMembershipUpsert(pc, contact)
	if err != nil {
		return s.failDeferredWrite(ctx, req, err, false, logAttrs)
	}
	in.ProjectID = req.ProjectID
	in.Actor = domain.PortalMembershipWriteActor
	in.CompletesSalesforceWriteRequestID = req.ID
	// Stamp the row this request was deferred for, even when Salesforce put
	// the contact on another account than the project's: the plain lookups
	// would otherwise miss it and insert a second membership.
	in.AdoptProjectContactID = req.ProjectContactID
	if strings.TrimSpace(in.Email) == "" {
		in.Email, in.ContactEmail = req.Email, req.Email
	}
	modified, ok := parseSalesforceLastModified(pc.LastModifiedDate)
	if !ok {
		modified = time.Now().UTC()
	}

	var target domain.MembershipWriteTarget
	res, err := s.deps.Memberships.UpsertWithin(ctx, req.ProjectID, req.Email, func(ctx context.Context, wc repository.MembershipWriteContext) (domain.SalesforceMembershipUpsert, domain.UpsertOnboardingStepRequest, error) {
		if wc.Existing == nil {
			return domain.SalesforceMembershipUpsert{}, domain.UpsertOnboardingStepRequest{},
				&apierror.NotFoundError{Msg: "the membership row this request was deferred for no longer exists"}
		}
		target = wc.Target
		contactSfID := in.ContactSfID
		return in, domain.UpsertOnboardingStepRequest{
			MembershipSfID:  in.MembershipSfID,
			Step:            domain.OnboardingStepDatabase,
			Status:          domain.OnboardingStepSucceeded,
			EventType:       domain.PortalMembershipWriteEventType,
			EventModifiedOn: modified,
			Email:           in.Email,
			ContactSfID:     &contactSfID,
			UpdatedBy:       domain.PortalMembershipWriteActor,
		}, nil
	})
	if err != nil {
		return s.completionProblem(ctx, req, result, err, lastAttempt, logAttrs)
	}

	invalidateUser(ctx, s.deps.UserCache, res.UserID, in.Email)
	membership := domain.ProjectMembership{
		ProjectID:        res.ProjectID,
		ProjectContactID: res.ProjectContactID,
		MembershipSfID:   in.MembershipSfID,
		ContactSfID:      in.ContactSfID,
		UserID:           res.UserID,
		Email:            in.Email,
		State:            in.State,
		Roles:            result.Roles,
	}
	rec := salesforceWriteRecord{
		ContactSfID:       in.ContactSfID,
		MembershipSfID:    in.MembershipSfID,
		State:             in.State,
		Roles:             result.Roles,
		CreatedContact:    result.ContactCreated,
		CreatedMembership: result.MembershipCreated,
		StoredState:       in.State,
		GivenName:         in.ContactFirstName,
		FamilyName:        in.ContactLastName,
		ProjectName:       target.ProjectName,
		ProjectKey:        target.ProjectKey,
		Type:              in.Type,
		IsIntegrationUser: in.IsCsIntegrationUser,
		LastModifiedOn:    modified,
	}
	slog.InfoContext(ctx, "deferred salesforce write: request completed; ids stamped",
		append(logAttrs, "state", in.State, "contactCreated", result.ContactCreated, "membershipCreated", result.MembershipCreated)...)
	// This is the invitation's origin as far as the notification pipeline is
	// concerned: nothing published project_contact.invited when the rows were
	// committed (no membership id to key the ledger on), and the Salesforce
	// echo stays silent for a row already in this state.
	s.publishInvited(ctx, membership, rec, false)
	return nil
}

// completionProblem sorts a completion failure: terminal → FAILED and
// acknowledged; otherwise one more attempt is recorded and the error returned
// so the consumer retries, unless this was the last attempt anyone will make,
// in which case the request is closed as FAILED naming the Salesforce ids it
// could not stamp (the Salesforce echo through Service Bus still stamps them
// when it arrives; the request row then just needs an operator).
func (s *projectMembershipWriteService) completionProblem(ctx context.Context, req domain.SalesforceWriteRequest, result events.SalesforceMembershipWriteCompletedPayload, cause error, lastAttempt bool, logAttrs []any) error {
	if isTerminalCompletionError(cause) {
		return s.failDeferredWrite(ctx, req, cause, false, logAttrs)
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if lastAttempt {
		slog.ErrorContext(ctx, "deferred salesforce write: could not complete the request on the final attempt; request marked FAILED", append(logAttrs, "err", cause)...)
		if err := s.deps.WriteRequests.MarkFailed(recordCtx, req.ID,
			fmt.Errorf("completion failed after retries (Salesforce has contact %s, membership %s): %w", result.ContactSfID, result.MembershipSfID, cause), true); err != nil {
			slog.ErrorContext(ctx, "deferred salesforce write: mark request failed", append(logAttrs, "err", err)...)
		}
		return cause
	}
	slog.WarnContext(ctx, "deferred salesforce write: could not complete the request; will retry", append(logAttrs, "err", cause)...)
	if err := s.deps.WriteRequests.RecordAttempt(recordCtx, req.ID, cause); err != nil {
		slog.ErrorContext(ctx, "deferred salesforce write: record attempt", append(logAttrs, "err", err)...)
	}
	return cause
}

// FailDeferredSalesforceWrite implements DeferredSalesforceWriteCompleter.
func (s *projectMembershipWriteService) FailDeferredSalesforceWrite(ctx context.Context, failed events.SalesforceMembershipWriteFailedPayload) error {
	if s.deps.WriteRequests == nil {
		return errors.New("deferred salesforce write: the request ledger is not configured")
	}
	req, err := s.deps.WriteRequests.Get(ctx, failed.RequestID)
	if err != nil {
		var nfe *apierror.NotFoundError
		if errors.As(err, &nfe) {
			slog.WarnContext(ctx, "deferred salesforce write: failed result for a request that no longer exists; acknowledging", "requestId", failed.RequestID)
			return nil
		}
		return err
	}
	logAttrs := []any{"requestId", req.ID, "projectId", req.ProjectID, "projectContactId", req.ProjectContactID, "terminal", failed.Terminal}
	if req.Status != domain.SalesforceWriteRequestPending {
		slog.InfoContext(ctx, "deferred salesforce write: request already settled; acknowledging the failed result", append(logAttrs, "status", req.Status)...)
		return nil
	}
	reason := strings.TrimSpace(failed.Reason)
	if reason == "" {
		reason = "sales-entity-service reported the write as failed"
	}
	// Terminal (a refusal) is not retryable; an outage that outlived the
	// retries is. The row keeps the distinction for the sweep / operator.
	return s.failDeferredWrite(ctx, req, errors.New(reason), !failed.Terminal, logAttrs)
}

// failDeferredWrite closes the request as FAILED and acknowledges the record
// (nil). The rows stay as they are: the person is a contact in CSM, the
// portal shows the failed sync, and an operator decides.
func (s *projectMembershipWriteService) failDeferredWrite(ctx context.Context, req domain.SalesforceWriteRequest, cause error, retryable bool, logAttrs []any) error {
	slog.ErrorContext(ctx, "deferred salesforce write: request marked FAILED", append(logAttrs, "retryable", retryable, "err", cause)...)
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.deps.WriteRequests.MarkFailed(recordCtx, req.ID, cause, retryable); err != nil {
		slog.ErrorContext(ctx, "deferred salesforce write: mark request failed", append(logAttrs, "err", err)...)
		// Without the FAILED mark the row would stay PENDING; let the
		// consumer retry the mark.
		return err
	}
	return nil
}
