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

// resendInvitationCooldown bounds how often the same membership's invitation
// may be re-sent. The resend marker on the payload deliberately bypasses
// csm-notification-service's own already-sent guard, so without a cooldown
// here nothing at all stands between a double-clicked button and a person
// getting a mailbox full of invitations.
const resendInvitationCooldown = 5 * time.Minute

// salesforceMembershipWriteFailureEvent is the event_publish_failures.event_type
// under which a Salesforce write that outlived its transaction is recorded.
// See recordSalesforceOrphan for why that table rather than a new one.
const salesforceMembershipWriteFailureEvent = "salesforce.membership_write"

// SalesEntityMembershipWriteClient is the write half of the REST
// sales/sales-entity-service contract. Every method that creates something is
// preceded by a search: the endpoints are not idempotent, so searching first
// is the only thing that stops a retry (or a previously orphaned record)
// turning into a duplicate Salesforce row.
type SalesEntityMembershipWriteClient interface {
	// GetContact reads a Contact by its Salesforce Id. A membership we
	// already hold ids for is resolved through this rather than through
	// SearchContactByEmail: the linked Contact's own address may differ
	// from the address the membership was invited under (see
	// domain.UserContactAccess.ContactRecordEmail), and searching by the
	// invited address would then miss the real record.
	GetContact(ctx context.Context, id string) (salesentity.Contact, error)
	SearchContactByEmail(ctx context.Context, email string) (salesentity.Contact, bool, error)
	CreateContact(ctx context.Context, in salesentity.CreateContactInput) (salesentity.Contact, error)
	SearchProjectContact(ctx context.Context, projectSfID, contactSfID string) (salesentity.ProjectContact, bool, error)
	CreateProjectContact(ctx context.Context, in salesentity.CreateProjectContactInput) (salesentity.ProjectContact, error)
	UpdateProjectContact(ctx context.Context, membershipSfID string, state *string, roles *[]string) (salesentity.ProjectContact, error)
}

// MembershipWriteDeps bundles what the portal membership writes need.
type MembershipWriteDeps struct {
	Memberships repository.ProjectMembershipRepository
	Steps       repository.OnboardingStepRepository
	SalesEntity SalesEntityMembershipWriteClient
	// Publisher may be nil (Event Hub unconfigured). An invitation then
	// writes both systems and sends no email; a resend, whose only effect is
	// the event, reports 503 instead of silently doing nothing.
	Publisher EventPublisherService
	// Failures may be nil (no pool, which cannot happen for this service,
	// or simply unwired). A Salesforce write that outlived its transaction
	// is then logged rather than recorded.
	Failures EventPublishFailureService
	Access   AccessService
}

type projectMembershipWriteService struct {
	deps MembershipWriteDeps
}

// NewProjectMembershipWriteService constructs a ProjectMembershipWriteService.
//
// THE WRITE ORDERING, in one place, because every method below depends on it:
//
//  1. Open the database transaction, read the project and its account, and
//     the membership that may already be there.
//  2. Write Salesforce inside that transaction, searching before every
//     create so an existing record is adopted rather than duplicated.
//  3. Write the rows through the existing membership upsert.
//  4. Commit.
//
// Salesforce has no transaction: once its write returns it is final. The
// database does, and its rollback is free — so the participant that can be
// undone goes last. Anything that fails before the commit leaves both systems
// exactly as they were, and the caller gets an error. The one residue is a
// commit that fails after Salesforce succeeded, leaving a Salesforce record
// with no row; the search-first rule means the next write adopts it instead
// of making a second, and recordSalesforceOrphan makes it visible meanwhile.
// That same rule is also why an accidental hand edit in Salesforce is
// absorbed rather than duplicated.
func NewProjectMembershipWriteService(deps MembershipWriteDeps) ProjectMembershipWriteService {
	return &projectMembershipWriteService{deps: deps}
}

// requireInternalCaller gates every method on an allow-listed internal client
// (AUTH_INTERNAL_CLIENT_IDS), exactly as the onboarding-step endpoints do.
//
// A portal END USER must never reach these directly: whether this particular
// customer admin may invite this particular person into this particular
// project is the portal backend's decision, made against the account it has
// already scoped the session to. This service only knows that the caller is
// one of the portal backends, and deliberately does not re-derive that
// authorization from a forwarded user token.
func (s *projectMembershipWriteService) requireInternalCaller(ctx context.Context) error {
	scope, err := s.deps.Access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: "membership writes are only available to internal services"}
	}
	return nil
}

// Invite implements ProjectMembershipWriteService.
func (s *projectMembershipWriteService) Invite(ctx context.Context, projectID string, req domain.CreateProjectMembershipRequest) (domain.ProjectMembership, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ProjectMembership{}, err
	}
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectMembership{}, err
	}
	email, err := normalizeMembershipEmail(req.Email)
	if err != nil {
		return domain.ProjectMembership{}, err
	}
	roles, err := canonicalSalesforceRoles(req.Roles)
	if err != nil {
		return domain.ProjectMembership{}, err
	}
	if len(roles) == 0 {
		// An invitation granting nothing is a mistake, not a valid state:
		// the person would be provisioned an identity and see an empty
		// portal. A role change may legitimately clear the list (see
		// UpdateRoles); a first invitation may not.
		return domain.ProjectMembership{}, &apierror.ValidationError{Msg: "roles must contain at least one role"}
	}

	var written salesforceWriteRecord
	res, err := s.deps.Memberships.UpsertWithin(ctx, projectID, email, func(ctx context.Context, wc repository.MembershipWriteContext) (domain.SalesforceMembershipUpsert, domain.UpsertOnboardingStepRequest, error) {
		state := domain.MembershipStateInvited
		if wc.Existing != nil {
			if !strings.EqualFold(wc.Existing.State, domain.MembershipStateDeactivated) {
				return domain.SalesforceMembershipUpsert{}, domain.UpsertOnboardingStepRequest{},
					&apierror.ConflictError{Msg: "this address is already a contact on the project; change their roles instead"}
			}
			// Bringing a deactivated contact back is a re-invitation, which
			// is a distinct Salesforce state and a distinct email.
			state = domain.MembershipStateReInvited
		}
		in, rec, err := s.writeSalesforce(ctx, wc, salesforceWriteIntent{
			Email:               email,
			FirstName:           strings.TrimSpace(req.FirstName),
			LastName:            strings.TrimSpace(req.LastName),
			IsCsIntegrationUser: req.IsCsIntegrationUser,
			Roles:               roles,
			State:               state,
			SetRoles:            true,
			SetState:            true,
		})
		if err != nil {
			return domain.SalesforceMembershipUpsert{}, domain.UpsertOnboardingStepRequest{}, err
		}
		written = rec
		return in, membershipStep(in, rec), nil
	})
	if err != nil {
		return domain.ProjectMembership{}, s.handleWriteError(ctx, "invite", projectID, email, written, err)
	}

	membership := domain.ProjectMembership{
		ProjectID:        res.ProjectID,
		ProjectContactID: res.ProjectContactID,
		MembershipSfID:   written.MembershipSfID,
		ContactSfID:      written.ContactSfID,
		UserID:           res.UserID,
		Email:            email,
		State:            written.State,
		Roles:            roles,
	}
	// The portal write is the ORIGIN of this invitation, so it is the thing
	// that publishes. The Salesforce echo that follows finds the row already
	// there and stays silent (see the ingest's echo suppression), which is
	// what keeps one invitation to one email.
	s.publishInvited(ctx, membership, written, false)
	return membership, nil
}

// UpdateRoles implements ProjectMembershipWriteService.
func (s *projectMembershipWriteService) UpdateRoles(ctx context.Context, projectID, email string, req domain.UpdateProjectMembershipRolesRequest) (domain.ProjectMembership, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ProjectMembership{}, err
	}
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectMembership{}, err
	}
	normalized, err := normalizeMembershipEmail(email)
	if err != nil {
		return domain.ProjectMembership{}, err
	}
	roles, err := canonicalSalesforceRoles(req.Roles)
	if err != nil {
		return domain.ProjectMembership{}, err
	}

	var written salesforceWriteRecord
	res, err := s.deps.Memberships.UpsertWithin(ctx, projectID, normalized, func(ctx context.Context, wc repository.MembershipWriteContext) (domain.SalesforceMembershipUpsert, domain.UpsertOnboardingStepRequest, error) {
		if wc.Existing == nil {
			return domain.SalesforceMembershipUpsert{}, domain.UpsertOnboardingStepRequest{},
				&apierror.NotFoundError{Msg: "contact not found on this project"}
		}
		state := wc.Existing.State
		if strings.TrimSpace(state) == "" {
			state = domain.MembershipStateInvited
		}
		in, rec, err := s.writeSalesforce(ctx, wc, salesforceWriteIntent{
			Email:    normalized,
			Roles:    roles,
			State:    state,
			SetRoles: true,
		})
		if err != nil {
			return domain.SalesforceMembershipUpsert{}, domain.UpsertOnboardingStepRequest{}, err
		}
		written = rec
		return in, membershipStep(in, rec), nil
	})
	if err != nil {
		return domain.ProjectMembership{}, s.handleWriteError(ctx, "update-roles", projectID, normalized, written, err)
	}
	return domain.ProjectMembership{
		ProjectID:        res.ProjectID,
		ProjectContactID: res.ProjectContactID,
		MembershipSfID:   written.MembershipSfID,
		ContactSfID:      written.ContactSfID,
		UserID:           res.UserID,
		Email:            normalized,
		State:            written.State,
		Roles:            roles,
	}, nil
}

// Deactivate implements ProjectMembershipWriteService.
func (s *projectMembershipWriteService) Deactivate(ctx context.Context, projectID, email string) error {
	if err := s.requireInternalCaller(ctx); err != nil {
		return err
	}
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return err
	}
	normalized, err := normalizeMembershipEmail(email)
	if err != nil {
		return err
	}

	var written salesforceWriteRecord
	_, err = s.deps.Memberships.UpsertWithin(ctx, projectID, normalized, func(ctx context.Context, wc repository.MembershipWriteContext) (domain.SalesforceMembershipUpsert, domain.UpsertOnboardingStepRequest, error) {
		if wc.Existing == nil {
			return domain.SalesforceMembershipUpsert{}, domain.UpsertOnboardingStepRequest{},
				&apierror.NotFoundError{Msg: "contact not found on this project"}
		}
		// Deactivate, never delete: DEACTIVATED is a real value of the
		// Salesforce State__c picklist and of project_contact_state_enum,
		// and it is what the portal does today. The roles are left exactly
		// as they are -- the derived account-level admin role ignores
		// deactivated memberships, so an admin's last active project going
		// away takes their admin with it without anything being erased.
		in, rec, err := s.writeSalesforce(ctx, wc, salesforceWriteIntent{
			Email:    normalized,
			Roles:    salesforceRolesForGroups(wc.Existing.ProjectGroups),
			Groups:   wc.Existing.ProjectGroups,
			State:    domain.MembershipStateDeactivated,
			SetState: true,
		})
		if err != nil {
			return domain.SalesforceMembershipUpsert{}, domain.UpsertOnboardingStepRequest{}, err
		}
		written = rec
		return in, membershipStep(in, rec), nil
	})
	if err != nil {
		return s.handleWriteError(ctx, "deactivate", projectID, normalized, written, err)
	}
	return nil
}

// ResendInvitation implements ProjectMembershipWriteService.
func (s *projectMembershipWriteService) ResendInvitation(ctx context.Context, projectID, email string) error {
	if err := s.requireInternalCaller(ctx); err != nil {
		return err
	}
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return err
	}
	normalized, err := normalizeMembershipEmail(email)
	if err != nil {
		return err
	}
	row, err := s.deps.Memberships.GetMembershipByEmail(ctx, projectID, normalized)
	if err != nil {
		return err
	}
	// Only an OUTSTANDING invitation can be re-sent. REGISTERED means they
	// already accepted it and DEACTIVATED means they should not receive one
	// at all -- re-sending either would confuse the recipient or hand a
	// deactivated person a working sign-up link.
	//
	// RE-INVITED counts as outstanding, not as "already re-sent": it is the
	// state Invite writes when it brings a deactivated contact back, which
	// is a fresh invitation rather than a second copy of one. If the
	// notification service was down when that event was published, the
	// person sits in RE-INVITED having received nothing, and a resend is
	// exactly the retry they need. The cooldown below bounds it either way.
	if !strings.EqualFold(row.State, domain.MembershipStateInvited) &&
		!strings.EqualFold(row.State, domain.MembershipStateReInvited) {
		return &apierror.ConflictError{Msg: "only a contact in state INVITED or RE-INVITED can have their invitation re-sent"}
	}
	if strings.TrimSpace(row.MembershipSfID) == "" {
		return &apierror.ConflictError{Msg: "this contact has no Salesforce membership to re-send an invitation for"}
	}
	if err := s.enforceResendCooldown(ctx, row.MembershipSfID); err != nil {
		return err
	}
	if s.deps.Publisher == nil {
		// Unlike an invitation, whose database and Salesforce writes are the
		// substance of the call, a resend IS the event. Reporting success
		// with no publisher would be reporting an email nobody will send.
		return &apierror.ServiceUnavailableError{Msg: "invitation e-mails are not configured on this deployment"}
	}

	payload, err := json.Marshal(events.ProjectContactInvitedPayload{
		MembershipSfID:    row.MembershipSfID,
		ContactSfID:       row.ContactSfID,
		Email:             row.Email,
		GivenName:         row.FirstName,
		FamilyName:        row.LastName,
		ProjectName:       row.ProjectName,
		ProjectKey:        row.ProjectKey,
		Roles:             salesforceRolesForGroups(row.ProjectGroups),
		IsIntegrationUser: row.IsIntegrationUser,
		Type:              row.Type,
		Resend:            true,
	})
	if err != nil {
		return fmt.Errorf("encode project_contact.invited resend payload: %w", err)
	}
	pubCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishInvitedTimeout)
	defer cancel()
	if err := s.deps.Publisher.Publish(pubCtx, events.TypeProjectContactInvited, row.MembershipSfID, payload); err != nil {
		slog.ErrorContext(ctx, "membership write: re-publish project_contact.invited", "membershipSfId", row.MembershipSfID, "err", err)
		return &apierror.ServiceUnavailableError{Msg: "the invitation could not be queued; please try again"}
	}
	return nil
}

// enforceResendCooldown refuses a resend inside resendInvitationCooldown of
// the last one, measured from the EMAIL step's updatedOn in the onboarding
// ledger -- the record of when an invitation was actually sent, written by
// csm-notification-service itself rather than guessed at here. No EMAIL step
// means none has been sent yet, so there is nothing to wait for.
func (s *projectMembershipWriteService) enforceResendCooldown(ctx context.Context, membershipSfID string) error {
	steps, err := s.deps.Steps.GetByMembership(ctx, membershipSfID)
	if err != nil {
		return err
	}
	for _, st := range steps {
		if st.Step != domain.OnboardingStepEmail {
			continue
		}
		since := time.Since(st.UpdatedOn)
		if since < resendInvitationCooldown {
			wait := (resendInvitationCooldown - since).Round(time.Second)
			return &apierror.TooManyRequestsError{Msg: fmt.Sprintf("an invitation was sent recently; try again in %s", wait)}
		}
	}
	return nil
}

// salesforceWriteIntent is what one portal call wants Salesforce to end up
// saying about a membership.
type salesforceWriteIntent struct {
	Email     string
	FirstName string
	LastName  string
	// IsCsIntegrationUser is only consulted when this write creates the
	// Salesforce contact; an existing contact's own flag wins.
	IsCsIntegrationUser bool
	// Roles are canonical Salesforce Role__c labels.
	Roles []string
	// Groups overrides the project_group set derived from Roles. Only the
	// deactivate path sets it, to keep a membership's stored groups exactly
	// as they are while its state moves.
	Groups []string
	State  string
	// SetRoles/SetState select which fields an UPDATE of an existing
	// Salesforce membership actually PATCHes, so a role change does not
	// restate the state and a deactivation does not restate the roles.
	SetRoles bool
	SetState bool
}

// salesforceWriteRecord is what the Salesforce half actually did, kept so the
// response can carry the ids and so an orphaned write can be recorded.
type salesforceWriteRecord struct {
	ContactSfID       string
	MembershipSfID    string
	State             string
	Roles             []string
	CreatedContact    bool
	CreatedMembership bool
	// The rest is what project_contact.invited needs, captured here because
	// the Salesforce contact and the project are both already in hand at
	// that point and neither is worth re-reading after the commit.
	GivenName         string
	FamilyName        string
	ProjectName       string
	ProjectKey        string
	Type              string
	IsIntegrationUser bool
	// LastModifiedOn is the Salesforce record's own LastModifiedDate after
	// the write, used as the onboarding step's eventModifiedOn so the echo
	// of this very write is recognised as not newer.
	LastModifiedOn time.Time
}

// writeSalesforce is the Salesforce half of every portal membership write,
// run inside the database transaction. It searches before it creates,
// always: the create endpoints are not idempotent, so adopting an existing
// record is the only thing that keeps a retry, a hand edit in Salesforce, or
// a previously orphaned record from becoming a duplicate.
//
// The contact is resolved BY ID whenever the membership already carries one.
// The invited address and the linked Contact's own Email are not the same
// field and do drift apart on real rows (domain.UserContactAccess compares
// them for exactly that reason), so resolving a known membership by address
// can miss its Contact -- and a miss here is not a harmless retry: it would
// create a second Contact, then a second Project_Contact__c, leaving the
// real membership untouched while the role change or deactivation appears to
// have succeeded. A by-id read that fails for any reason falls back to the
// address search, so every self-healing path this had before still works.
func (s *projectMembershipWriteService) writeSalesforce(ctx context.Context, wc repository.MembershipWriteContext, intent salesforceWriteIntent) (domain.SalesforceMembershipUpsert, salesforceWriteRecord, error) {
	var rec salesforceWriteRecord

	var contact salesentity.Contact
	var found bool
	var err error
	if wc.Existing != nil && strings.TrimSpace(wc.Existing.ContactSfID) != "" {
		linkedID := strings.TrimSpace(wc.Existing.ContactSfID)
		contact, err = s.deps.SalesEntity.GetContact(ctx, linkedID)
		if err != nil {
			// The id we hold no longer resolves (hand-deleted in Salesforce,
			// or the read itself failed). Fall through to the address search
			// and, if that misses too, to the create -- the behaviour before
			// the by-id lookup existed.
			slog.WarnContext(ctx, "membership write: linked Salesforce contact could not be read by id, falling back to the address search",
				"contactSfId", linkedID, "err", err)
		} else {
			found = true
		}
	}
	if !found {
		contact, found, err = s.deps.SalesEntity.SearchContactByEmail(ctx, intent.Email)
		if err != nil {
			return domain.SalesforceMembershipUpsert{}, rec, err
		}
	}
	if !found {
		first, last := intent.FirstName, intent.LastName
		if first == "" && last == "" {
			// Salesforce requires a LastName on a Contact. Falling back to
			// the local part of the address keeps an invitation that carries
			// no name at all working, rather than failing it outright.
			first, last = splitDisplayName(localPartOf(intent.Email))
			if last == "" {
				first, last = "", localPartOf(intent.Email)
			}
		}
		contact, err = s.deps.SalesEntity.CreateContact(ctx, salesentity.CreateContactInput{
			FirstName:           first,
			LastName:            last,
			Email:               intent.Email,
			AccountID:           wc.Target.AccountSfID,
			IsCsIntegrationUser: intent.IsCsIntegrationUser,
		})
		if err != nil {
			return domain.SalesforceMembershipUpsert{}, rec, err
		}
		rec.CreatedContact = true
	}
	rec.ContactSfID = strings.TrimSpace(derefString(contact.ID))
	if rec.ContactSfID == "" {
		return domain.SalesforceMembershipUpsert{}, rec, &apierror.ServiceUnavailableError{Msg: "sales/sales-entity-service returned a contact with no id"}
	}

	membership, mFound, err := s.deps.SalesEntity.SearchProjectContact(ctx, wc.Target.ProjectSfID, rec.ContactSfID)
	if err != nil {
		return domain.SalesforceMembershipUpsert{}, rec, err
	}
	switch {
	case !mFound:
		membership, err = s.deps.SalesEntity.CreateProjectContact(ctx, salesentity.CreateProjectContactInput{
			ProjectID: wc.Target.ProjectSfID,
			ContactID: rec.ContactSfID,
			State:     intent.State,
			Role:      intent.Roles,
		})
		if err != nil {
			return domain.SalesforceMembershipUpsert{}, rec, err
		}
		rec.CreatedMembership = true
	default:
		var state *string
		var roles *[]string
		if intent.SetState {
			state = &intent.State
		}
		if intent.SetRoles {
			r := intent.Roles
			roles = &r
		}
		if state != nil || roles != nil {
			// PATCH answers with the record it re-read, or 200 with an empty
			// body when only that re-read failed. An empty body is a zero
			// value, not an error: the write landed either way, so the id we
			// already hold is still the right one.
			updated, uerr := s.deps.SalesEntity.UpdateProjectContact(ctx, membership.ID, state, roles)
			if uerr != nil {
				return domain.SalesforceMembershipUpsert{}, rec, uerr
			}
			if strings.TrimSpace(updated.ID) != "" {
				membership = updated
			}
		}
	}
	rec.MembershipSfID = strings.TrimSpace(membership.ID)
	if rec.MembershipSfID == "" {
		return domain.SalesforceMembershipUpsert{}, rec, &apierror.ServiceUnavailableError{Msg: "sales/sales-entity-service returned a membership with no id"}
	}
	rec.State = intent.State
	rec.Roles = intent.Roles
	if modified, ok := parseSalesforceLastModified(membership.LastModifiedDate); ok {
		rec.LastModifiedOn = modified
	} else {
		rec.LastModifiedOn = time.Now().UTC()
	}

	// An existing Salesforce contact may belong to a different account than
	// the project's -- that is exactly what a partner contact is. A contact
	// we just created belongs to the project's account by construction.
	accountSfID := wc.Target.AccountSfID
	membershipType := domain.MembershipTypeOwnContact
	if contact.Account != nil && strings.TrimSpace(derefString(contact.Account.ID)) != "" {
		accountSfID = strings.TrimSpace(derefString(contact.Account.ID))
		if !strings.EqualFold(accountSfID, wc.Target.AccountSfID) {
			membershipType = domain.MembershipTypePartnerContact
		}
	}

	groups := intent.Groups
	if groups == nil {
		groups, _ = mapProjectGroups(intent.Roles)
	}
	// The Salesforce value wins for a contact that already existed. For one
	// this call just CREATED, POST /contacts is only contracted to return an
	// id, so a response that omits isCsIntegrationUser must not be read as
	// "false" -- that would hand a machine account an Asgardeo identity,
	// global roles and an invitation e-mail. Fall back to what we asked for.
	isIntegrationUser := contact.IsCsIntegrationUser != nil && *contact.IsCsIntegrationUser
	if rec.CreatedContact && contact.IsCsIntegrationUser == nil {
		isIntegrationUser = intent.IsCsIntegrationUser
	}
	globalRoles, managed, adminRole := mapGlobalRoles(membershipType, isIntegrationUser)

	first, last := strings.TrimSpace(derefString(contact.FirstName)), strings.TrimSpace(derefString(contact.LastName))
	if first == "" && last == "" {
		first, last = intent.FirstName, intent.LastName
	}
	name := strings.TrimSpace(derefString(contact.Name))
	if first == "" && last == "" && name != "" {
		first, last = splitDisplayName(name)
	}

	rec.GivenName, rec.FamilyName = first, last
	rec.ProjectName, rec.ProjectKey = wc.Target.ProjectName, wc.Target.ProjectKey
	rec.Type = membershipType
	rec.IsIntegrationUser = isIntegrationUser

	return domain.SalesforceMembershipUpsert{
		MembershipSfID:      rec.MembershipSfID,
		State:               intent.State,
		Type:                membershipType,
		Email:               intent.Email,
		Actor:               domain.PortalMembershipWriteActor,
		ProjectID:           wc.Target.ProjectID,
		ContactSfID:         rec.ContactSfID,
		ContactEmail:        intent.Email,
		ContactName:         name,
		ContactFirstName:    first,
		ContactLastName:     last,
		ContactAccountSfID:  accountSfID,
		IsCsAdmin:           contact.IsCsAdmin != nil && *contact.IsCsAdmin,
		IsCsIntegrationUser: isIntegrationUser,
		ProjectSfID:         wc.Target.ProjectSfID,
		ProjectKey:          wc.Target.ProjectKey,
		GlobalRoles:         globalRoles,
		ManagedAdminRoles:   managed,
		AdminRoleName:       adminRole,
		ProjectGroups:       groups,
	}, rec, nil
}

// membershipStep is the DATABASE onboarding step a portal write records, in
// the same transaction as the rows themselves.
func membershipStep(in domain.SalesforceMembershipUpsert, rec salesforceWriteRecord) domain.UpsertOnboardingStepRequest {
	contactSfID := in.ContactSfID
	return domain.UpsertOnboardingStepRequest{
		MembershipSfID: in.MembershipSfID,
		Step:           domain.OnboardingStepDatabase,
		Status:         domain.OnboardingStepSucceeded,
		EventType:      domain.PortalMembershipWriteEventType,
		// Stamped with the Salesforce record's own LastModifiedDate, not
		// with now(): the echo of this write carries that same value, and
		// the ingest's duplicate guard then recognises it as not newer and
		// leaves the row alone.
		EventModifiedOn: rec.LastModifiedOn,
		Email:           in.Email,
		ContactSfID:     &contactSfID,
		UpdatedBy:       domain.PortalMembershipWriteActor,
	}
}

// publishInvited emits project_contact.invited for a membership this service
// just invited or re-invited. Failures are logged, never returned: both
// systems have already been written, and EventPublisherService records the
// failure durably.
func (s *projectMembershipWriteService) publishInvited(ctx context.Context, m domain.ProjectMembership, rec salesforceWriteRecord, resend bool) {
	if s.deps.Publisher == nil {
		return
	}
	if m.State != domain.MembershipStateInvited && m.State != domain.MembershipStateReInvited {
		return
	}
	payload, err := json.Marshal(events.ProjectContactInvitedPayload{
		MembershipSfID:    m.MembershipSfID,
		ContactSfID:       m.ContactSfID,
		Email:             m.Email,
		GivenName:         rec.GivenName,
		FamilyName:        rec.FamilyName,
		ProjectName:       rec.ProjectName,
		ProjectKey:        rec.ProjectKey,
		Roles:             m.Roles,
		IsIntegrationUser: rec.IsIntegrationUser,
		Type:              rec.Type,
		Resend:            resend,
	})
	if err != nil {
		slog.ErrorContext(ctx, "membership write: encode project_contact.invited payload", "membershipSfId", m.MembershipSfID, "err", err)
		return
	}
	pubCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishInvitedTimeout)
	defer cancel()
	if err := s.deps.Publisher.Publish(pubCtx, events.TypeProjectContactInvited, m.MembershipSfID, payload); err != nil {
		slog.ErrorContext(ctx, "membership write: publish project_contact.invited", "membershipSfId", m.MembershipSfID, "err", err)
	}
}

// handleWriteError turns the repository's error into the one the caller sees,
// recording the single case where Salesforce moved and the database did not.
func (s *projectMembershipWriteService) handleWriteError(ctx context.Context, operation, projectID, email string, rec salesforceWriteRecord, err error) error {
	if errors.Is(err, repository.ErrMembershipCommitFailed) {
		s.recordSalesforceOrphan(ctx, operation, projectID, email, rec, err)
		return &apierror.ServiceUnavailableError{Msg: "the change could not be saved; please try again"}
	}
	return err
}

// recordSalesforceOrphan durably records a Salesforce write whose transaction
// then failed to commit, so it can be retried rather than lost.
//
// It reuses event_publish_failures rather than adding a table of its own. The
// shape already fits exactly -- a type, the id the record is about, the JSON
// of what should have happened, the error, and a resolved_on for when someone
// has dealt with it -- and it already has a repository, a service, a search
// endpoint and a resolve endpoint, so the backlog is visible the day this
// ships instead of needing its own API first. The alternative, a
// salesforce_write_failures table, would have been the same five columns with
// a second copy of all of that around them. Nothing drains this automatically
// yet; recording it is the whole of the commitment here.
func (s *projectMembershipWriteService) recordSalesforceOrphan(ctx context.Context, operation, projectID, email string, rec salesforceWriteRecord, cause error) {
	entityID := rec.MembershipSfID
	if entityID == "" {
		entityID = email
	}
	slog.ErrorContext(ctx, "membership write: Salesforce was written but the transaction did not commit",
		"operation", operation, "projectId", projectID, "membershipSfId", rec.MembershipSfID, "contactSfId", rec.ContactSfID, "err", cause)
	if s.deps.Failures == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"operation":         operation,
		"projectId":         projectID,
		"email":             email,
		"membershipSfId":    rec.MembershipSfID,
		"contactSfId":       rec.ContactSfID,
		"state":             rec.State,
		"roles":             rec.Roles,
		"createdContact":    rec.CreatedContact,
		"createdMembership": rec.CreatedMembership,
	})
	if err != nil {
		slog.ErrorContext(ctx, "membership write: encode orphaned-write payload", "membershipSfId", rec.MembershipSfID, "err", err)
		return
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if _, err := s.deps.Failures.CreateEventPublishFailure(recordCtx, domain.CreateEventPublishFailureRequest{
		EventType: salesforceMembershipWriteFailureEvent,
		EntityID:  entityID,
		Payload:   payload,
		Error:     truncateOnboardingStepError(cause.Error()),
	}); err != nil {
		slog.ErrorContext(ctx, "membership write: recording the orphaned Salesforce write also failed", "membershipSfId", rec.MembershipSfID, "err", err)
	}
}

// normalizeMembershipEmail lower-cases and trims the address a membership is
// keyed by. Every layer below compares addresses case-insensitively, so
// normalizing once here keeps the response, the row and the Salesforce record
// spelling it the same way.
func normalizeMembershipEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", &apierror.ValidationError{Msg: "email is required"}
	}
	at := strings.Index(email, "@")
	if at <= 0 || at == len(email)-1 || strings.ContainsAny(email, " \t\r\n") {
		return "", &apierror.ValidationError{Msg: "email is not a valid address"}
	}
	return email, nil
}

func localPartOf(email string) string {
	if at := strings.Index(email, "@"); at > 0 {
		return email[:at]
	}
	return email
}

// canonicalSalesforceRoles maps the caller's role strings onto Salesforce's
// own picklist spelling, rejecting anything Salesforce would not accept.
//
// The ingest logs an unknown role and carries on, which is right for a record
// Salesforce already holds -- refusing it would strip a real membership over
// a vocabulary gap. It is wrong here: this call is asking us to WRITE the
// value, and Salesforce would reject the picklist entry mid-transaction, so a
// 400 naming the role is both earlier and clearer.
func canonicalSalesforceRoles(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, r := range raw {
		key := strings.ToLower(strings.TrimSpace(r))
		if key == "" {
			continue
		}
		label, ok := validSalesforceRoles[key]
		if !ok {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("roles contains unknown role %q", strings.TrimSpace(r))}
		}
		if seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	return out, nil
}
