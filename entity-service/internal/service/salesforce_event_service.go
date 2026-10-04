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
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// maxAccountPhoneChars is account.phone's width (VARCHAR(64) since
// migration 0096). A longer phone keeps the stored value rather than failing
// the event.
const maxAccountPhoneChars = 64

// accountColumnLimits is the width of every other bounded VARCHAR account
// column the Account ingest writes (migration 0012). A Salesforce value longer
// than its column is written as NULL with a warning, so one oversized picklist
// value cannot fail the whole event. name is not listed: it is NOT NULL, and
// Salesforce caps Account.Name at 255 characters, which is its width here.
var accountColumnLimits = map[string]int{
	"industry":             255,
	"region":               100,
	"global_pod":           100,
	"sales_region":         100,
	"sub_region":           100,
	"life_cycle":           50,
	"naics_industry":       100,
	"sub_industry":         255,
	"classification":       255,
	"street":               255,
	"city":                 100,
	"state_province":       100,
	"postal_code":          20,
	"country":              100,
	"lost_reason":          255,
	"account_vertical":     255,
	"lost_reason_category": 255,
}

// SalesEntityCustomerClient fetches a REST sales/sales-entity-service Customer by Salesforce Account Id.
type SalesEntityCustomerClient interface {
	GetCustomer(ctx context.Context, id string) (salesentity.Customer, error)
}

// SalesEntityMembershipClient fetches the Salesforce records the membership
// ingest needs from REST sales/sales-entity-service.
type SalesEntityMembershipClient interface {
	GetProjectContact(ctx context.Context, id string) (salesentity.ProjectContact, error)
	GetContact(ctx context.Context, id string) (salesentity.Contact, error)
}

// MembershipIngest bundles the dependencies of the Project_Contact__c /
// Contact branch of POST /salesforce/events. It is optional: a
// salesforceEventService built without it acknowledges those entities and
// does nothing (the pre-existing behaviour), which is how
// CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED=false is realised in routes.go.
type MembershipIngest struct {
	Memberships repository.ProjectMembershipRepository
	Steps       repository.OnboardingStepRepository
	SalesEntity SalesEntityMembershipClient
	// Contacts is the Contact writer's store ("user" and account_contact of
	// a Contact, with or without memberships). Contact events need it and
	// SalesforceIngestSupport.States; Project_Contact__c events do not.
	Contacts repository.SalesforceContactRepository
	// Publisher may be nil (Event Hub unconfigured): project_contact.invited
	// is then not published, the database write still happens.
	Publisher EventPublisherService
}

func (m *MembershipIngest) enabled() bool {
	return m != nil && m.Memberships != nil && m.Steps != nil && m.SalesEntity != nil
}

// errMembershipIngestDisabled is what a membership re-run returns when the
// service was built without MembershipIngest; the retry job never asks in
// that case, so seeing it means a wiring mistake.
var errMembershipIngestDisabled = errors.New("salesforce: membership ingest is disabled")

// AccountLookup resolves a CSM account id by Salesforce Account Id. It is a
// read-only slice of repository.AccountRepository, kept separate so
// EnsureAccount can read accounts even when the Account ingest — the write
// side, s.repo — is off.
type AccountLookup interface {
	LookupAccountIDBySfID(ctx context.Context, sfID string) (*string, error)
}

// SalesforceIngestSupport bundles the dependencies every ingest family
// shares regardless of which family flags are on: the account lookup behind
// EnsureAccount and the salesforce_ingest_state ledger behind
// shouldSkipIngest. Both may be nil; each caller that needs one says so.
type SalesforceIngestSupport struct {
	Accounts AccountLookup
	States   repository.SalesforceIngestStateRepository
	// Projects is the read side of EnsureProject, flag-independent like
	// Accounts (salesforce_project_ingest.go).
	Projects ProjectLookup
}

type salesforceEventService struct {
	repo       repository.AccountRepository
	se         SalesEntityCustomerClient
	support    SalesforceIngestSupport
	membership *MembershipIngest
	// opportunity is set by WithOpportunityIngest (salesforce_opportunity_ingest.go).
	opportunity *OpportunityIngest
	// project is set by WithProjectIngest (salesforce_project_ingest.go).
	project *ProjectIngest
	// linkedOpportunity is set by WithLinkedOpportunityIngest
	// (salesforce_linked_opportunity_ingest.go).
	linkedOpportunity *LinkedOpportunityIngest
	// partners is set by WithPartnerIngest (salesforce_partner_ingest.go).
	partners *PartnerIngest
	// invoices is set by WithInvoiceIngest (salesforce_invoice_ingest.go).
	invoices *InvoiceIngest
	// lineItems is set by WithOpportunityLineItemIngest
	// (salesforce_opportunity_line_item_ingest.go).
	lineItems *OpportunityLineItemIngest
}

// NewSalesforceEventService constructs a SalesforceEventService that ingests
// Account events only; Project_Contact__c and Contact envelopes are
// acknowledged and ignored. A nil repo turns the Account branch off too.
func NewSalesforceEventService(repo repository.AccountRepository, se SalesEntityCustomerClient, support SalesforceIngestSupport) SalesforceEventService {
	return &salesforceEventService{repo: repo, se: se, support: support}
}

// NewSalesforceEventServiceWithMembershipIngest additionally ingests
// Project_Contact__c and Contact envelopes — see salesforce_membership_ingest.go.
func NewSalesforceEventServiceWithMembershipIngest(repo repository.AccountRepository, se SalesEntityCustomerClient, support SalesforceIngestSupport, ingest MembershipIngest) SalesforceEventService {
	return &salesforceEventService{repo: repo, se: se, support: support, membership: &ingest}
}

// EnsureAccount returns the CSM id of the account with this Salesforce Account
// Id, for a child ingest (contact, project, opportunity) that needs its parent
// row before it can write its own. The account is looked up by sf_id; when it
// is absent and the Account ingest is on (s.repo != nil) the ordinary Account
// upsert runs first — fetch from Sales Entity, write by natural key — and the
// lookup is repeated. When it is absent and the Account ingest is off, the
// account can only arrive through the ServiceNow sync, so the result is a
// NotFoundError: the caller fails its event, Service Bus redelivers it, and
// the delayed-retry job re-runs it once the parent has landed. Idempotent —
// a second call for a present account is one SELECT.
func (s *salesforceEventService) EnsureAccount(ctx context.Context, sfID string) (string, error) {
	sfID = strings.TrimSpace(sfID)
	if sfID == "" {
		return "", &apierror.ValidationError{Msg: "account sfId is required"}
	}
	// The write-side repository can read too; support.Accounts is what
	// keeps reads possible when the Account ingest is off.
	lookup := s.support.Accounts
	if lookup == nil && s.repo != nil {
		lookup = s.repo
	}
	if lookup == nil {
		return "", errors.New("salesforce: account lookup is not configured")
	}
	id, err := lookup.LookupAccountIDBySfID(ctx, sfID)
	if err != nil {
		return "", err
	}
	if id != nil {
		return *id, nil
	}
	if s.repo == nil {
		// The prefix is what repository.IsMissingParentError (and so the
		// delayed-retry job) matches, and the same text the membership
		// ingest uses, so every child family's FAILED row is re-run.
		return "", &apierror.NotFoundError{Msg: fmt.Sprintf("account not found for sfId %q", sfID)}
	}
	slog.InfoContext(ctx, "salesforce: parent account not in CSM yet, ingesting it first", "accountSfId", sfID)
	// The duplicate guard is off here: the row is known to be missing, so a
	// ledger row saying this version was written must not stop the write.
	if err := s.upsertAccount(ctx, sfID, domain.SalesforceEventUpdated, false); err != nil {
		return "", err
	}
	id, err = lookup.LookupAccountIDBySfID(ctx, sfID)
	if err != nil {
		return "", err
	}
	if id == nil {
		return "", fmt.Errorf("salesforce: account %s was upserted but cannot be read back by sf_id", sfID)
	}
	return *id, nil
}

// HandleEvent implements SalesforceEventService.
func (s *salesforceEventService) HandleEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if strings.TrimSpace(req.EventType) == "" {
		return &apierror.ValidationError{Msg: "eventType is required"}
	}
	if strings.TrimSpace(req.Entity) == "" {
		return &apierror.ValidationError{Msg: "entity is required"}
	}
	if strings.TrimSpace(req.ReferenceID) == "" {
		return &apierror.ValidationError{Msg: "referenceId is required"}
	}
	req.EventType = strings.TrimSpace(req.EventType)
	req.Entity = strings.TrimSpace(req.Entity)
	req.ReferenceID = strings.TrimSpace(req.ReferenceID)
	switch {
	case strings.EqualFold(req.Entity, domain.SalesforceEntityAccount):
		// handled below
	case strings.EqualFold(req.Entity, domain.SalesforceEntityProjectContact),
		strings.EqualFold(req.Entity, domain.SalesforceEntityProjectContactAlt):
		return s.handleProjectContactEvent(ctx, req)
	case strings.EqualFold(req.Entity, domain.SalesforceEntityContact):
		return s.handleContactEvent(ctx, req)
	case strings.EqualFold(req.Entity, domain.SalesforceEntityOpportunity):
		return s.handleOpportunityEvent(ctx, req)
	case strings.EqualFold(req.Entity, domain.SalesforceEntityProject):
		return s.handleProjectEvent(ctx, req)
	case strings.EqualFold(req.Entity, domain.SalesforceEntityLinkedOpportunity),
		strings.EqualFold(req.Entity, domain.SalesforceEntityLinkedOpportunityAlt):
		return s.handleLinkedOpportunityEvent(ctx, req)
	case strings.EqualFold(req.Entity, domain.SalesforceEntityInvoice),
		strings.EqualFold(req.Entity, domain.SalesforceEntityInvoiceAlt):
		return s.handleInvoiceEvent(ctx, req)
	case strings.EqualFold(req.Entity, domain.SalesforceEntityOpportunityLineItem):
		return s.handleOpportunityLineItemEvent(ctx, req)
	default:
		// Other Salesforce objects are acknowledged and ignored: a 400 would
		// make ASB retry the envelope forever.
		return nil
	}
	if s.repo == nil {
		slog.InfoContext(ctx, "salesforce: account ingest disabled, ignoring account event",
			"eventType", req.EventType, "referenceId", req.ReferenceID)
		return nil
	}
	if req.EventType == domain.SalesforceEventUndefined {
		return &apierror.ValidationError{Msg: "eventType UNDEFINED is not supported"}
	}

	switch req.EventType {
	case domain.SalesforceEventCreated, domain.SalesforceEventUpdated, domain.SalesforceEventRestored:
		if err := s.upsertAccount(ctx, req.ReferenceID, req.EventType, true); err != nil {
			return err
		}
		return s.refreshPartnersAfterAccountEvent(ctx, req.ReferenceID)
	case domain.SalesforceEventDeleted:
		return s.softDeleteAccount(ctx, req.ReferenceID)
	default:
		return &apierror.ValidationError{Msg: "eventType must be CREATED, UPDATED, DELETED, RESTORED, or UNDEFINED"}
	}
}

// softDeleteAccount is the DELETED branch of the Account ingest. A deleted
// record cannot be read from Sales Entity, so there is no LastModifiedDate
// and no duplicate guard: the soft delete is idempotent (a repeat keeps the
// first deleted_on). The ledger row is stamped with the current time, or the
// recorded version when that is later, so the DELETED row always replaces
// the one before it (the ledger only moves forward) and a later RESTORED —
// which carries an older LastModifiedDate — is still let through by the
// guard's DELETED rule.
func (s *salesforceEventService) softDeleteAccount(ctx context.Context, sfID string) error {
	if s.se == nil {
		return errDeleteUnconfirmable
	}
	_, fetchErr := s.se.GetCustomer(ctx, sfID)
	if gone, err := confirmDeletedUpstream(ctx, string(domain.SalesforceIngestEntityAccount), sfID, fetchErr); err != nil || !gone {
		return err
	}
	modifiedOn := time.Now().UTC()
	if s.support.States != nil {
		st, err := s.support.States.Get(ctx, domain.SalesforceIngestEntityAccount, sfID)
		if err != nil {
			return err
		}
		if st != nil && st.EventModifiedOn.After(modifiedOn) {
			modifiedOn = st.EventModifiedOn
		}
	}
	found, err := s.repo.SoftDeleteBySfID(ctx, sfID, domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityAccount,
		SfID:            sfID,
		EventModifiedOn: modifiedOn,
		EventType:       domain.SalesforceEventDeleted,
		Status:          domain.SalesforceIngestSucceeded,
	})
	if err != nil {
		return err
	}
	if !found {
		slog.InfoContext(ctx, "salesforce: DELETED account was never ingested, nothing to mark", "accountSfId", sfID)
	}
	return nil
}

// upsertAccount is the CREATED/UPDATED/RESTORED branch of the Account
// ingest (and EnsureAccount's write): read the Customer from Sales Entity,
// skip it when the ledger already holds this version (guard), otherwise map
// it and write the account row and a SUCCEEDED ledger row in one transaction.
//
// A customer with no name is acknowledged, not retried: account.name is NOT
// NULL and no redelivery can fix the record, so the event is logged and
// recorded FAILED in the ledger instead of dead-lettering. A failed write is
// recorded FAILED too, and its error returned so Service Bus redelivers.
func (s *salesforceEventService) upsertAccount(ctx context.Context, sfID, eventType string, guard bool) error {
	cust, err := s.se.GetCustomer(ctx, sfID)
	if err != nil {
		return err
	}
	// The Salesforce id is stored as both sf_id and account.number (NOT NULL,
	// UNIQUE), so a record without one could only be inserted as an empty
	// number, once.
	if strings.TrimSpace(cust.ID) == "" {
		return &apierror.ServiceUnavailableError{Msg: "sales/sales-entity-service customer is missing id"}
	}

	// eventModifiedOn is the version the ledger records; shouldSkipIngest
	// derives the same value (the current time when lastModifiedDate is
	// missing), and warns about a missing one.
	eventModifiedOn, ok := parseSalesforceLastModified(cust.LastModifiedDate)
	if !ok {
		eventModifiedOn = time.Now().UTC()
	}
	if guard && s.support.States != nil {
		var skip bool
		skip, eventModifiedOn, err = shouldSkipIngest(ctx, s.support.States, domain.SalesforceIngestEntityAccount, sfID, eventType, cust.LastModifiedDate)
		if err != nil {
			return err
		}
		if skip {
			return nil
		}
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityAccount,
		SfID:            sfID,
		EventModifiedOn: eventModifiedOn,
		EventType:       eventType,
		Status:          domain.SalesforceIngestSucceeded,
	}

	if strings.TrimSpace(derefString(cust.Name)) == "" {
		slog.WarnContext(ctx, "salesforce: customer has no name, account not written", "accountSfId", sfID, "eventType", eventType)
		s.recordAccountIngestFailed(ctx, state, errors.New("sales/sales-entity-service customer is missing Name"))
		return nil
	}

	row := mapSalesEntityCustomer(ctx, cust)
	people := []struct {
		role  string
		email *string
		dest  **string
	}{
		{"owner", ownerEmail(cust.Owner), &row.AccountManagerID},
		{"technicalOwner", cust.TechnicalOwner, &row.TechnicalOwnerID},
		{"csm", cust.CsmEmail, &row.CustomerSuccessManagerID},
		{"secondaryTechnicalOwner", cust.SecondaryTechnicalOwner, &row.SecondaryTechnicalOwnerID},
		{"renewalManager", ownerEmail(cust.RenewalManager), &row.RenewalAccountManagerID},
	}
	for _, p := range people {
		*p.dest, err = s.lookupOwner(ctx, sfID, p.role, derefString(p.email))
		if err != nil {
			s.recordAccountIngestFailed(ctx, state, err)
			return err
		}
	}
	if err := s.repo.UpsertFromSalesforce(ctx, row, state); err != nil {
		s.recordAccountIngestFailed(ctx, state, err)
		return err
	}
	s.requeueChildrenOf(ctx, repository.MissingParent{Kind: repository.MissingParentAccount, SfID: strings.TrimSpace(cust.ID)})
	return nil
}

// recordAccountIngestFailed writes a FAILED ledger row best-effort so the
// onboarding dashboard can see the account is stuck; the caller's outcome
// does not depend on whether this write succeeds.
func (s *salesforceEventService) recordAccountIngestFailed(ctx context.Context, state domain.UpsertSalesforceIngestStateRequest, cause error) {
	if s.support.States == nil {
		return
	}
	msg := truncateOnboardingStepError(cause.Error())
	state.Status = domain.SalesforceIngestFailed
	state.LastError = &msg
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if _, err := s.support.States.Upsert(recordCtx, state); err != nil {
		slog.ErrorContext(ctx, "salesforce: recording FAILED account ingest state also failed", "accountSfId", state.SfID, "err", err)
	}
}

// lookupOwner resolves a person reference on the account (owner, technical
// owner, CSM, renewal manager) to a "user" id by email. An email with no
// matching user writes NULL and logs a warning: WSO2 staff reach "user" only
// through the ServiceNow sys_user sync today, so a missing one is a data gap
// to see, not a reason to fail the event.
func (s *salesforceEventService) lookupOwner(ctx context.Context, sfID, role, email string) (*string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil
	}
	id, err := s.repo.LookupUserIDByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if id == nil {
		slog.WarnContext(ctx, "salesforce: account person reference does not match a CSM user, writing NULL",
			"accountSfId", sfID, "role", role, "email", email)
	}
	return id, nil
}

func ownerEmail(u *salesentity.CustomerUser) *string {
	if u == nil {
		return nil
	}
	return u.Email
}

// mapSalesEntityCustomer maps the Customer's own fields; the person
// references (user ids) are resolved by upsertAccount.
func mapSalesEntityCustomer(ctx context.Context, cust salesentity.Customer) domain.SalesforceAccountUpsert {
	phone, keepExistingPhone := mapPhone(derefString(cust.Phone))
	addr := cust.Address
	if addr == nil {
		addr = &salesentity.CustomerAddress{}
	}
	bounded := func(column string, v *string) *string {
		return boundedAccountColumn(ctx, cust.ID, column, v)
	}
	return domain.SalesforceAccountUpsert{
		SfID:              cust.ID,
		Name:              strings.TrimSpace(derefString(cust.Name)),
		Number:            cust.ID,
		Industry:          bounded("industry", cust.Industry),
		Region:            bounded("region", cust.Region),
		GlobalPod:         bounded("global_pod", cust.GlobalPod),
		Phone:             phone,
		KeepExistingPhone: keepExistingPhone,
		SalesRegion:       bounded("sales_region", cust.SalesRegions),
		SubRegion:         bounded("sub_region", cust.SubRegion),
		LifeCycle:         bounded("life_cycle", cust.Status),
		NAICSIndustry:     bounded("naics_industry", cust.NAICSIndustry),
		SubIndustry:       bounded("sub_industry", cust.SubIndustry),
		Classification:    bounded("classification", cust.AccountClassification),
		Street:            bounded("street", addr.BillingStreet),
		City:              bounded("city", addr.BillingCity),
		StateProvince:     bounded("state_province", addr.BillingState),
		PostalCode:        bounded("postal_code", addr.BillingPostalCode),
		Country:           bounded("country", addr.BillingCountry),
		ActivationDate:    salesforceDate(ctx, cust.ID, "activation_date", cust.ActivationDate),
		LostDate:          salesforceDate(ctx, cust.ID, "lost_date", cust.LostDate),
		LostReason:        bounded("lost_reason", cust.LostReason),

		AccountVertical:    bounded("account_vertical", cust.AccountVertical),
		LostReasonCategory: bounded("lost_reason_category", cust.LostReasonCategory),
		DeactivationDate:   salesforceDate(ctx, cust.ID, "deactivation_date", cust.DeactivationDate),
	}
}

// boundedAccountColumn trims v and returns nil for an empty value or one
// longer than the column (accountColumnLimits), logging the latter.
func boundedAccountColumn(ctx context.Context, sfID, column string, v *string) *string {
	out := optionalPtr(v)
	if out == nil {
		return nil
	}
	if limit, ok := accountColumnLimits[column]; ok && utf8.RuneCountInString(*out) > limit {
		slog.WarnContext(ctx, "salesforce: account value is longer than its column, writing NULL",
			"accountSfId", sfID, "column", column, "limit", limit, "length", utf8.RuneCountInString(*out))
		return nil
	}
	return out
}

// salesforceDate parses a Salesforce date field ("2006-01-02"). A value
// that does not parse is logged and written as NULL.
func salesforceDate(ctx context.Context, sfID, column string, v *string) *time.Time {
	raw := optionalPtr(v)
	if raw == nil {
		return nil
	}
	d, err := time.Parse(time.DateOnly, *raw)
	if err != nil {
		slog.WarnContext(ctx, "salesforce: account date does not parse, writing NULL",
			"accountSfId", sfID, "column", column, "value", *raw)
		return nil
	}
	return &d
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func optionalPtr(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func mapPhone(phone string) (*string, bool) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil, false
	}
	if utf8.RuneCountInString(phone) > maxAccountPhoneChars {
		return nil, true
	}
	return &phone, false
}
