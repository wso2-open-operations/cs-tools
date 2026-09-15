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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesforce"
)

const maxAccountPhoneChars = 20

// SalesforceAccountClient fetches a Salesforce Account by Id.
type SalesforceAccountClient interface {
	GetAccount(ctx context.Context, id string) (salesforce.Account, error)
}

type salesforceEventService struct {
	repo repository.AccountRepository
	sf   SalesforceAccountClient
}

// NewSalesforceEventService constructs a SalesforceEventService.
func NewSalesforceEventService(repo repository.AccountRepository, sf SalesforceAccountClient) SalesforceEventService {
	return &salesforceEventService{repo: repo, sf: sf}
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
	if !strings.EqualFold(req.Entity, domain.SalesforceEntityAccount) {
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
	acct, err := s.sf.GetAccount(ctx, sfID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(acct.Name) == "" {
		return &apierror.ServiceUnavailableError{Msg: "salesforce account is missing Name"}
	}

	row := mapSalesforceAccount(acct)
	row.TechnicalOwnerID, err = s.lookupOwner(ctx, acct.TechnicalOwner)
	if err != nil {
		return err
	}
	row.SecondaryTechnicalOwnerID, err = s.lookupOwner(ctx, acct.TechnicalOwner2)
	if err != nil {
		return err
	}
	return s.repo.UpsertFromSalesforce(ctx, row)
}

func (s *salesforceEventService) lookupOwner(ctx context.Context, email string) (*string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil
	}
	return s.repo.LookupUserIDByEmail(ctx, email)
}

func mapSalesforceAccount(acct salesforce.Account) domain.SalesforceAccountUpsert {
	number := strings.TrimSpace(acct.AccountNumber)
	if number == "" {
		number = acct.ID
	}
	phone, keepExistingPhone := mapPhone(acct.Phone)
	return domain.SalesforceAccountUpsert{
		SfID:              acct.ID,
		Name:              strings.TrimSpace(acct.Name),
		Number:            number,
		Industry:          optionalString(acct.Industry),
		Region:            optionalString(acct.Region),
		GlobalPod:         optionalString(acct.GlobalPOD),
		Phone:             phone,
		KeepExistingPhone: keepExistingPhone,
		SalesRegion:       optionalString(acct.SalesRegions),
		SubRegion:         optionalString(acct.SubRegion),
		AccountVertical:   optionalString(acct.AccountVertical),
		LifeCycle:         optionalString(acct.AccountStatus),
		NAICSIndustry:     optionalString(acct.NAICSIndustry),
		SubIndustry:       optionalString(acct.SubIndustry),
		Classification:    optionalString(acct.AccountClassification),
	}
}

func optionalString(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
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
