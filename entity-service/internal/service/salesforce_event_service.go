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
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

const maxAccountPhoneChars = 20

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
	// Publisher may be nil (Event Hub unconfigured): project_contact.invited
	// is then not published, the database write still happens.
	Publisher EventPublisherService
}

func (m *MembershipIngest) enabled() bool {
	return m != nil && m.Memberships != nil && m.Steps != nil && m.SalesEntity != nil
}

type salesforceEventService struct {
	repo       repository.AccountRepository
	se         SalesEntityCustomerClient
	membership *MembershipIngest
}

// NewSalesforceEventService constructs a SalesforceEventService that ingests
// Account events only; Project_Contact__c and Contact envelopes are
// acknowledged and ignored.
func NewSalesforceEventService(repo repository.AccountRepository, se SalesEntityCustomerClient) SalesforceEventService {
	return &salesforceEventService{repo: repo, se: se}
}

// NewSalesforceEventServiceWithMembershipIngest additionally ingests
// Project_Contact__c and Contact envelopes — see salesforce_membership_ingest.go.
func NewSalesforceEventServiceWithMembershipIngest(repo repository.AccountRepository, se SalesEntityCustomerClient, ingest MembershipIngest) SalesforceEventService {
	return &salesforceEventService{repo: repo, se: se, membership: &ingest}
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
	default:
		// Other Salesforce objects are acknowledged and ignored: a 400 would
		// make ASB retry the envelope forever.
		return nil
	}
	if req.EventType == domain.SalesforceEventUndefined {
		return &apierror.ValidationError{Msg: "eventType UNDEFINED is not supported"}
	}

	switch req.EventType {
	case domain.SalesforceEventCreated, domain.SalesforceEventUpdated, domain.SalesforceEventRestored:
		return s.upsertAccount(ctx, req.ReferenceID)
	case domain.SalesforceEventDeleted:
		return s.repo.SoftDeleteBySfID(ctx, req.ReferenceID)
	default:
		return &apierror.ValidationError{Msg: "eventType must be CREATED, UPDATED, DELETED, RESTORED, or UNDEFINED"}
	}
}

func (s *salesforceEventService) upsertAccount(ctx context.Context, sfID string) error {
	cust, err := s.se.GetCustomer(ctx, sfID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(derefString(cust.Name)) == "" {
		return &apierror.ServiceUnavailableError{Msg: "sales/sales-entity-service customer is missing Name"}
	}

	row := mapSalesEntityCustomer(cust)
	row.TechnicalOwnerID, err = s.lookupOwner(ctx, derefString(cust.TechnicalOwner))
	if err != nil {
		return err
	}
	row.SecondaryTechnicalOwnerID = nil
	return s.repo.UpsertFromSalesforce(ctx, row)
}

func (s *salesforceEventService) lookupOwner(ctx context.Context, email string) (*string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil
	}
	return s.repo.LookupUserIDByEmail(ctx, email)
}

func mapSalesEntityCustomer(cust salesentity.Customer) domain.SalesforceAccountUpsert {
	phone, keepExistingPhone := mapPhone(derefString(cust.Phone))
	return domain.SalesforceAccountUpsert{
		SfID:              cust.ID,
		Name:              strings.TrimSpace(derefString(cust.Name)),
		Number:            cust.ID,
		Industry:          optionalPtr(cust.Industry),
		Region:            optionalPtr(cust.Region),
		GlobalPod:         optionalPtr(cust.GlobalPod),
		Phone:             phone,
		KeepExistingPhone: keepExistingPhone,
		SalesRegion:       optionalPtr(cust.SalesRegions),
		SubRegion:         optionalPtr(cust.SubRegion),
		AccountVertical:   nil,
		LifeCycle:         optionalPtr(cust.Status),
		NAICSIndustry:     optionalPtr(cust.NAICSIndustry),
		SubIndustry:       optionalPtr(cust.SubIndustry),
		Classification:    optionalPtr(cust.AccountClassification),
	}
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
