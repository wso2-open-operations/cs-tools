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
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// errContactWriterNotConfigured is what a Contact event returns when the
// service was built with the membership ingest but without the Contact
// writer's store or the ledger: a wiring mistake, never a data problem.
var errContactWriterNotConfigured = errors.New("salesforce: contact writer is not configured (MembershipIngest.Contacts / SalesforceIngestSupport.States)")

// ContactReingester re-runs the Contact writer for one Salesforce Contact id
// as if an UPDATED event had arrived. The delayed-retry job registers it
// under domain.SalesforceIngestEntityContact, so a contact whose account was
// not in CSM yet (a FAILED "account not found" ledger row) is written once
// the account lands.
type ContactReingester interface {
	RetryContactIngest(ctx context.Context, contactSfID string) error
}

// RetryContactIngest implements ContactReingester. It is the whole Contact
// writer — contact write, then the membership fan-out — because the
// memberships that failed for the same missing account need the same second
// chance; each membership keeps its own duplicate guard. A FAILED ledger row
// never blocks the guard, so the re-run goes through to the write.
func (s *salesforceEventService) RetryContactIngest(ctx context.Context, contactSfID string) error {
	if !s.membership.enabled() {
		return errMembershipIngestDisabled
	}
	return s.ingestContact(ctx, contactSfID, domain.SalesforceEventUpdated)
}

// ingestContact is the Contact writer (CREATED / UPDATED / RESTORED): fetch
// the contact once, write its "user" and account_contact rows and its
// contact-derived roles (writeContact), then re-run the membership upsert
// for each of its memberships with the contact already in hand, so
// project_contact and the groups follow without a second contact read per
// membership.
//
// A failed contact write returns before the fan-out: the envelope is
// redelivered and the whole thing runs again. The fan-out itself processes
// every membership and returns the first error, as before.
func (s *salesforceEventService) ingestContact(ctx context.Context, contactSfID, eventType string) error {
	if s.membership.Contacts == nil || s.support.States == nil {
		return errContactWriterNotConfigured
	}
	contact, err := s.membership.SalesEntity.GetContact(ctx, contactSfID)
	if err != nil {
		return err
	}
	if id := strings.TrimSpace(derefString(contact.ID)); id != "" {
		contactSfID = id
	}
	if err := s.writeContact(ctx, contactSfID, eventType, contact); err != nil {
		return err
	}

	var firstErr error
	for _, m := range contact.Memberships {
		id := strings.TrimSpace(derefString(m.ID))
		if id == "" {
			continue
		}
		if err := s.ingestMembership(ctx, id, eventType, &contact); err != nil {
			slog.ErrorContext(ctx, "salesforce: contact event: membership upsert failed", "contactSfId", contactSfID, "membershipSfId", id, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// writeContact is the contact half of ingestContact. A contact version the
// ledger already holds is skipped (the fan-out still runs: a membership that
// failed last time must get its redelivery). A contact with no account is
// acknowledged and not written — account_contact cannot exist without one —
// with a warning.
func (s *salesforceEventService) writeContact(ctx context.Context, contactSfID, eventType string, contact salesentity.Contact) error {
	skip, eventModifiedOn, err := shouldSkipIngest(ctx, s.support.States, domain.SalesforceIngestEntityContact, contactSfID, eventType, contact.LastModifiedDate)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityContact,
		SfID:            contactSfID,
		EventModifiedOn: eventModifiedOn,
		EventType:       eventType,
		Status:          domain.SalesforceIngestSucceeded,
	}

	accountSfID := contactAccountSfID(contact)
	if accountSfID == "" {
		slog.WarnContext(ctx, "salesforce: contact has no account, user and account_contact not written", "contactSfId", contactSfID)
		return nil
	}
	// Parent before child. A NotFoundError (the account is not in CSM and
	// the Account ingest is off) fails the event so Service Bus redelivers
	// it; the ledger records why.
	accountID, err := s.EnsureAccount(ctx, accountSfID)
	if err != nil {
		s.recordContactIngestFailed(ctx, state, err)
		return err
	}

	in, err := buildContactUpsert(contact, contactSfID, accountID, accountSfID)
	if err != nil {
		s.recordContactIngestFailed(ctx, state, err)
		return err
	}
	res, err := s.membership.Contacts.Upsert(ctx, in, state)
	if err != nil {
		s.recordContactIngestFailed(ctx, state, err)
		return err
	}
	slog.InfoContext(ctx, "salesforce: contact ingested",
		"contactSfId", contactSfID, "accountSfId", accountSfID, "userId", res.UserID, "accountContactId", res.AccountContactID,
		"createdUser", res.CreatedUser, "createdAccountContact", res.CreatedAccountContact,
		"deactivatedAccountContacts", res.DeactivatedAccountContacts, "isAccountAdmin", res.IsAccountAdmin)
	return nil
}

// deactivateContact is the Contact writer's DELETED branch: confirm the
// contact is gone upstream (a deleted record is hidden from reads, so the
// lookup answers "not found"), soft delete by sf_id, and a DELETED ledger row
// so a later RESTORED is not skipped as a duplicate.
func (s *salesforceEventService) deactivateContact(ctx context.Context, contactSfID string) error {
	if s.membership.Contacts == nil || s.support.States == nil {
		return errContactWriterNotConfigured
	}
	if s.membership.SalesEntity == nil {
		return errDeleteUnconfirmable
	}
	_, fetchErr := s.membership.SalesEntity.GetContact(ctx, contactSfID)
	if gone, err := confirmDeletedUpstream(ctx, string(domain.SalesforceIngestEntityContact), contactSfID, fetchErr); err != nil || !gone {
		return err
	}
	found, err := s.membership.Contacts.DeactivateBySfID(ctx, contactSfID, domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityContact,
		SfID:            contactSfID,
		EventModifiedOn: time.Now().UTC(),
		EventType:       domain.SalesforceEventDeleted,
		Status:          domain.SalesforceIngestSucceeded,
	})
	if err != nil {
		return err
	}
	if !found {
		slog.InfoContext(ctx, "salesforce: DELETED contact was never ingested, nothing to deactivate", "contactSfId", contactSfID)
	}
	return nil
}

// recordContactIngestFailed writes a FAILED ledger row best-effort so the
// failure is visible; the original error is what HandleEvent returns
// regardless of whether this write succeeds.
func (s *salesforceEventService) recordContactIngestFailed(ctx context.Context, state domain.UpsertSalesforceIngestStateRequest, cause error) {
	msg := truncateOnboardingStepError(cause.Error())
	state.Status = domain.SalesforceIngestFailed
	state.LastError = &msg
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if _, err := s.support.States.Upsert(recordCtx, state); err != nil {
		slog.ErrorContext(ctx, "salesforce: recording FAILED contact ingest also failed", "contactSfId", state.SfID, "err", err)
	}
}

// adminRoleBasis is the repository.AdminRoleBasisFunc the membership DELETED
// path hands DeactivateBySfID: it reads the contact (the membership is gone
// from Salesforce, the contact is not) for its account classification and
// isCsAdmin. A contact that cannot be read — deleted together with its
// memberships, or Sales Entity down — reports !ok, and the admin role is left
// for the next event to re-derive rather than revoked on a guess.
func (s *salesforceEventService) adminRoleBasis(ctx context.Context, contactSfID string) (repository.AdminRoleBasis, bool) {
	contact, err := s.membership.SalesEntity.GetContact(ctx, contactSfID)
	if err != nil {
		slog.WarnContext(ctx, "salesforce: membership DELETED: contact unreadable, admin role not re-derived", "contactSfId", contactSfID, "err", err)
		return repository.AdminRoleBasis{}, false
	}
	isIntegration := contact.IsCsIntegrationUser != nil && *contact.IsCsIntegrationUser
	roles := mapGlobalRoles(contact.Account, isIntegration)
	if roles.AdminRole == "" {
		return repository.AdminRoleBasis{}, false
	}
	return repository.AdminRoleBasis{
		AdminRole: roles.AdminRole,
		Managed:   roles.ManagedAdmin,
		IsCsAdmin: contact.IsCsAdmin != nil && *contact.IsCsAdmin,
	}, true
}

// contactAccountSfID is the contact's parent account Id: the flat accountId,
// else the nested account.id.
func contactAccountSfID(contact salesentity.Contact) string {
	if id := strings.TrimSpace(derefString(contact.AccountID)); id != "" {
		return id
	}
	if contact.Account != nil {
		return strings.TrimSpace(derefString(contact.Account.ID))
	}
	return ""
}

// buildContactUpsert translates a Sales Entity Contact into the Contact
// writer's request. accountID is the CSM account EnsureAccount resolved.
func buildContactUpsert(contact salesentity.Contact, contactSfID, accountID, accountSfID string) (domain.SalesforceContactUpsert, error) {
	email := strings.ToLower(strings.TrimSpace(derefString(contact.Email)))
	if email == "" {
		return domain.SalesforceContactUpsert{}, &apierror.ValidationError{Msg: "contact " + contactSfID + " has no email"}
	}
	name := strings.TrimSpace(derefString(contact.Name))
	first, last := strings.TrimSpace(derefString(contact.FirstName)), strings.TrimSpace(derefString(contact.LastName))
	if first == "" && last == "" && name != "" {
		first, last = splitDisplayName(name)
	}
	isIntegration := contact.IsCsIntegrationUser != nil && *contact.IsCsIntegrationUser
	roles := mapGlobalRoles(contact.Account, isIntegration)
	return domain.SalesforceContactUpsert{
		ContactSfID:         contactSfID,
		Email:               email,
		Name:                name,
		FirstName:           first,
		LastName:            last,
		AccountID:           accountID,
		AccountSfID:         accountSfID,
		IsPrimaryContact:    contact.IsPrimaryContact,
		IsCsAdmin:           contact.IsCsAdmin != nil && *contact.IsCsAdmin,
		IsCsIntegrationUser: isIntegration,
		GlobalRoles:         roles.Grant,
		ManagedGlobalRoles:  roles.ManagedGlobal,
		ManagedAdminRoles:   roles.ManagedAdmin,
		AdminRoleName:       roles.AdminRole,
	}, nil
}
