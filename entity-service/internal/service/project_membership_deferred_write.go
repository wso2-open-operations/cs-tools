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
//     and salesforce.membership_write.requested is published to this
//     service's own topic. The admin gets 201 with salesforceSync PENDING.
//     Anything else Salesforce says -- a validation refusal, a conflict --
//     still fails the request, because a retry would not change it.
//  2. ProcessDeferredSalesforceWrite, driven by the Event Hub consumer in
//     internal/server, replays the stored intent through the SAME
//     writeSalesforce the synchronous path uses: search first, then create or
//     adopt, so a record Salesforce acquired meanwhile (a hand edit, a
//     redelivered attempt that succeeded before its commit) is adopted rather
//     than duplicated. The ids are stamped and the request row marked
//     SUCCEEDED in one transaction, and project_contact.invited is published
//     only then -- the onboarding ledger that the invitation email and the
//     identity provisioning record into is keyed by the Salesforce
//     membership id, so the email waits for the id.
//  3. While the request is PENDING, role changes, deactivation and resend are
//     refused with 409 SALESFORCE_SYNC_PENDING: they need the ids.
//
// The request row is the durable record; the event is only the trigger.
// Event Hub commits the offset after a bounded number of attempts on each
// tier and drops what the dead-letter tier cannot deliver, so a request can
// outlive its event. It then sits PENDING (or FAILED when the consumer saw
// its final attempt) for an operator, a manual retry or a future sweep to
// re-publish from.

// msgSalesforceSyncPending is what the caller is told when a write needs the
// Salesforce ids a still-pending deferred write has not produced yet. 409,
// like the other refusals of this kind, because retrying later WILL help and
// the message says so.
const msgSalesforceSyncPending = "This contact is still being synced to Salesforce. Please try again in a few minutes."

// codeSalesforceSyncPending is the machine-readable companion of
// msgSalesforceSyncPending, so a portal can show its own wording.
const codeSalesforceSyncPending = "SALESFORCE_SYNC_PENDING"

// DeferredSalesforceWriter is the consumer-facing half of the deferred
// write; *projectMembershipWriteService implements it.
type DeferredSalesforceWriter interface {
	// ProcessDeferredSalesforceWrite carries out one salesforce_write_request.
	// A nil return means the record can be acknowledged: the write succeeded,
	// the request was already settled, or it failed for good and the row says
	// so. A non-nil return means Salesforce was still unavailable and the
	// caller should retry per its policy. lastAttempt is the caller saying
	// nothing will retry after this call, so a still-unavailable Salesforce
	// is recorded as FAILED rather than left PENDING forever.
	ProcessDeferredSalesforceWrite(ctx context.Context, requestID string, lastAttempt bool) error
}

// deferralAvailable reports whether this deployment defers at all: the
// request ledger and the command publisher are both wired (see routes.go,
// gated on Config.HasSalesforceDeferredWrites).
func (s *projectMembershipWriteService) deferralAvailable() bool {
	return s.deps.WriteRequests != nil && s.deps.SalesforceWritePublisher != nil
}

// isSalesforceUnavailable is the one error class a deferred write absorbs:
// the sales-entity client maps connection errors, timeouts and 5xx answers to
// ServiceUnavailableError, and so does an answer with no id in it. A
// validation refusal (DownstreamError / ValidationError) or a conflict is
// not an outage and is never deferred.
func isSalesforceUnavailable(err error) bool {
	var sue *apierror.ServiceUnavailableError
	return errors.As(err, &sue)
}

// canDeferInvite says whether this failed invitation may be committed
// locally instead: deferral is wired, the error is an outage, and this is a
// FIRST invitation. A re-invitation (wc.Existing is a DEACTIVATED row) has
// Salesforce ids already and would be an UPDATE there; deferring it would
// leave the CSM state ahead of Salesforce's for a membership the
// registration flow can already see, so it keeps failing fast.
func (s *projectMembershipWriteService) canDeferInvite(wc repository.MembershipWriteContext, err error) bool {
	return s.deferralAvailable() && wc.Existing == nil && isSalesforceUnavailable(err)
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
// it. A failed publish is recorded by the publisher (event_publish_failures)
// and leaves published_on NULL: the row is still owed, and the operator view
// can tell "never handed to the consumer" from "consumer is retrying".
func (s *projectMembershipWriteService) publishDeferredWrite(ctx context.Context, requestID, projectID, projectContactID string) {
	payload, err := json.Marshal(events.SalesforceMembershipWriteRequestedPayload{
		RequestID:        requestID,
		Operation:        domain.SalesforceWriteOperationInvite,
		ProjectID:        projectID,
		ProjectContactID: projectContactID,
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
	slog.InfoContext(ctx, "membership write: Salesforce unavailable; invitation committed locally and queued for the deferred write",
		"requestId", requestID, "projectId", projectID, "projectContactId", projectContactID)
}

// refuseWhileSyncPending is the 409 a role change, deactivation or resend
// gets while the membership's deferred write is still owed. It only speaks
// up for a row with no Salesforce id AND an open request: a row with no id
// and no request is the data-migration gap requireSalesforceLinks reports.
func (s *projectMembershipWriteService) refuseWhileSyncPending(ctx context.Context, operation string, existing *domain.ProjectMembershipRow) error {
	if existing == nil || s.deps.WriteRequests == nil || strings.TrimSpace(existing.MembershipSfID) != "" {
		return nil
	}
	req, err := s.deps.WriteRequests.PendingForProjectContact(ctx, existing.ProjectContactID)
	if err != nil {
		// Fall through to the ordinary missing-id refusal rather than fail
		// the request on a ledger read: either way the write does not run.
		slog.WarnContext(ctx, "membership write: could not read the salesforce write request ledger", "operation", operation, "projectContactId", existing.ProjectContactID, "err", err)
		return nil
	}
	if req == nil {
		return nil
	}
	slog.InfoContext(ctx, "membership write refused: the membership's Salesforce write is still pending",
		"operation", operation, "projectContactId", existing.ProjectContactID, "requestId", req.ID, "attempts", req.Attempts)
	return &apierror.ConflictError{Msg: msgSalesforceSyncPending, Code: codeSalesforceSyncPending}
}

// ProcessDeferredSalesforceWrite implements DeferredSalesforceWriter.
func (s *projectMembershipWriteService) ProcessDeferredSalesforceWrite(ctx context.Context, requestID string, lastAttempt bool) error {
	if s.deps.WriteRequests == nil {
		return errors.New("deferred salesforce write: the request ledger is not configured")
	}
	req, err := s.deps.WriteRequests.Get(ctx, requestID)
	if err != nil {
		var nfe *apierror.NotFoundError
		if errors.As(err, &nfe) {
			// The row went with its project or membership (ON DELETE
			// CASCADE); there is nothing left to write.
			slog.WarnContext(ctx, "deferred salesforce write: request no longer exists; acknowledging", "requestId", requestID)
			return nil
		}
		return err
	}
	logAttrs := []any{"requestId", req.ID, "projectId", req.ProjectID, "projectContactId", req.ProjectContactID, "attempts", req.Attempts}
	if req.Status != domain.SalesforceWriteRequestPending {
		slog.InfoContext(ctx, "deferred salesforce write: request already settled; acknowledging", append(logAttrs, "status", req.Status)...)
		return nil
	}
	if req.Operation != domain.SalesforceWriteOperationInvite {
		return s.failDeferredWrite(ctx, req, fmt.Errorf("unsupported operation %q", req.Operation), logAttrs)
	}
	var intent domain.SalesforceWriteIntent
	if err := json.Unmarshal(req.Intent, &intent); err != nil {
		return s.failDeferredWrite(ctx, req, fmt.Errorf("decode intent: %w", err), logAttrs)
	}
	roles, err := canonicalSalesforceRoles(intent.Roles)
	if err != nil {
		return s.failDeferredWrite(ctx, req, err, logAttrs)
	}
	if intent.State == "" {
		intent.State = domain.MembershipStateInvited
	}

	var written salesforceWriteRecord
	var adoptedID bool
	res, err := s.deps.Memberships.UpsertWithin(ctx, req.ProjectID, req.Email, func(ctx context.Context, wc repository.MembershipWriteContext) (domain.SalesforceMembershipUpsert, domain.UpsertOnboardingStepRequest, error) {
		if wc.Existing == nil {
			return domain.SalesforceMembershipUpsert{}, domain.UpsertOnboardingStepRequest{},
				&apierror.NotFoundError{Msg: "the membership row this request was deferred for no longer exists"}
		}
		// The project and account links, not the membership's own: that is
		// exactly what a deferred row lacks.
		if err := requireSalesforceLinks(ctx, membershipOpInvite, repository.MembershipWriteContext{Target: wc.Target}); err != nil {
			return domain.SalesforceMembershipUpsert{}, domain.UpsertOnboardingStepRequest{}, err
		}
		if s.deps.Invitations != nil {
			// The checks Invite could not make while Salesforce was down. A
			// refusal here is final: the row stays, the request is FAILED,
			// and the portal shows it.
			if err := s.deps.Invitations.Validate(ctx, wc.Target, req.Email, intent.InviterEmail); err != nil {
				return domain.SalesforceMembershipUpsert{}, domain.UpsertOnboardingStepRequest{}, err
			}
		}
		// A Salesforce echo may have adopted the row first (sf_id already
		// stamped). writeSalesforce then resolves by that id and PATCHes;
		// the request is still completed and the invitation still published,
		// since the echo stays silent for a row whose state did not change.
		adoptedID = strings.TrimSpace(wc.Existing.MembershipSfID) != ""
		in, rec, err := s.writeSalesforce(ctx, wc, salesforceWriteIntent{
			Email:               req.Email,
			FirstName:           strings.TrimSpace(intent.FirstName),
			LastName:            strings.TrimSpace(intent.LastName),
			IsCsIntegrationUser: intent.IsCsIntegrationUser,
			Roles:               roles,
			State:               intent.State,
			SetRoles:            true,
			SetState:            true,
			ValidateContactType: func(isIntegrationUser bool) error {
				if len(roles) == 0 && !isIntegrationUser {
					return &apierror.ValidationError{Msg: "roles must contain at least one role"}
				}
				return nil
			},
		})
		if err != nil {
			return domain.SalesforceMembershipUpsert{}, domain.UpsertOnboardingStepRequest{}, err
		}
		if rec.StoredState == domain.MembershipStateRegistered {
			in.State, rec.State = rec.StoredState, rec.StoredState
		}
		written = rec
		in.CompletesSalesforceWriteRequestID = req.ID
		return in, membershipStep(in, rec), nil
	})
	switch {
	case err == nil:
	case errors.Is(err, repository.ErrMembershipCommitFailed):
		// Salesforce has the record, the commit did not land. The next
		// attempt's search adopts it; meanwhile it is recorded, as the
		// synchronous path does.
		s.recordSalesforceOrphan(ctx, membershipOpInvite, req.ProjectID, req.Email, written, err)
		return s.retryDeferredWriteLater(ctx, req, err, lastAttempt, logAttrs)
	case isSalesforceUnavailable(err):
		return s.retryDeferredWriteLater(ctx, req, err, lastAttempt, logAttrs)
	default:
		return s.failDeferredWrite(ctx, req, err, logAttrs)
	}

	invalidateUser(ctx, s.deps.UserCache, res.UserID, req.Email)
	membership := domain.ProjectMembership{
		ProjectID:        res.ProjectID,
		ProjectContactID: res.ProjectContactID,
		MembershipSfID:   written.MembershipSfID,
		ContactSfID:      written.ContactSfID,
		UserID:           res.UserID,
		Email:            req.Email,
		State:            written.State,
		Roles:            roles,
	}
	slog.InfoContext(ctx, "deferred salesforce write: membership written to Salesforce",
		append(logAttrs, "membershipSfId", written.MembershipSfID, "contactSfId", written.ContactSfID,
			"createdContact", written.CreatedContact, "createdMembership", written.CreatedMembership, "idAlreadyAdopted", adoptedID)...)
	// This is the invitation's origin as far as the notification pipeline is
	// concerned: nothing published project_contact.invited when the rows were
	// committed (no membership id to key the ledger on), and the Salesforce
	// echo stays silent for a row already in this state.
	s.publishInvited(ctx, membership, written, false)
	return nil
}

// retryDeferredWriteLater records one more failed attempt and returns the
// error so the consumer retries -- unless this was the last attempt anyone
// will make, in which case the request is closed as FAILED so it does not
// sit PENDING with nothing behind it. The error is still returned so the
// consumer logs the exhaustion as such.
func (s *projectMembershipWriteService) retryDeferredWriteLater(ctx context.Context, req domain.SalesforceWriteRequest, cause error, lastAttempt bool, logAttrs []any) error {
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if lastAttempt {
		slog.ErrorContext(ctx, "deferred salesforce write: Salesforce still unavailable on the final attempt; request marked FAILED",
			append(logAttrs, "err", cause)...)
		if err := s.deps.WriteRequests.MarkFailed(recordCtx, req.ID, fmt.Errorf("retries exhausted: %w", cause)); err != nil {
			slog.ErrorContext(ctx, "deferred salesforce write: mark request failed", append(logAttrs, "err", err)...)
		}
		return cause
	}
	slog.WarnContext(ctx, "deferred salesforce write: Salesforce unavailable; will retry", append(logAttrs, "err", cause)...)
	if err := s.deps.WriteRequests.RecordAttempt(recordCtx, req.ID, cause); err != nil {
		slog.ErrorContext(ctx, "deferred salesforce write: record attempt", append(logAttrs, "err", err)...)
	}
	return cause
}

// failDeferredWrite closes the request as FAILED for a cause a retry would
// not change, and acknowledges the record (nil). The rows stay as they are:
// the person is a contact in CSM, the portal shows the failed sync, and an
// operator decides.
func (s *projectMembershipWriteService) failDeferredWrite(ctx context.Context, req domain.SalesforceWriteRequest, cause error, logAttrs []any) error {
	slog.ErrorContext(ctx, "deferred salesforce write: Salesforce refused the write; request marked FAILED", append(logAttrs, "err", cause)...)
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.deps.WriteRequests.MarkFailed(recordCtx, req.ID, cause); err != nil {
		slog.ErrorContext(ctx, "deferred salesforce write: mark request failed", append(logAttrs, "err", err)...)
		// Without the FAILED mark the row would stay PENDING and the next
		// redelivery would try again, which for a validation refusal is
		// harmless but pointless; let the consumer retry the mark.
		return err
	}
	return nil
}
