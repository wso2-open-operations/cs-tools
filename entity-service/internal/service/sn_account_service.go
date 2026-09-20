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
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snAccountsResponse mirrors the Choreo POST /accounts/search response.
type snAccountsResponse struct {
	Accounts     []snAccount `json:"accounts"`
	TotalRecords int         `json:"totalRecords"`
	Offset       int         `json:"offset"`
	Limit        int         `json:"limit"`
}

type snSupportTier struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type snPersonRef struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email"`
}

type snAccount struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	SupportTier      *snSupportTier `json:"supportTier"`
	Classification   string         `json:"classification"`
	Pod              *string        `json:"pod"`
	SfID             *string        `json:"sfId"`
	Region           *string        `json:"region"`
	ArrToday         *string        `json:"arrToday"`
	ActivationDate   string         `json:"activationDate"`
	DeactivationDate *string        `json:"deactivationDate"`
	TechnicalOwner   *snPersonRef   `json:"technicalOwner"`
	// Owner is Ballerina's own wire key and stays "owner" on the wire even
	// though the entity-service's own output renames this concept to
	// "accountManager" (see snAccountCommonFields / snAccountToDomain).
	Owner                 *snPersonRef `json:"owner"`
	RenewalAccountManager *snPersonRef `json:"renewalAccountManager"`
	// CreTeam and SreTeam resolve the account's CRE/SRE group refs. Per the project
	// owner's resolution, SN's u_integration_cs_team maps to CreTeam and u_sre_team maps
	// to SreTeam -- both refs to sys_user_group. Same mapping as the case service's
	// snCaseAccount.CreTeam/SreTeam.
	CreTeam         *snCaseEntityRef `json:"creTeam"`
	SreTeam         *snCaseEntityRef `json:"sreTeam"`
	HasAgent        bool             `json:"hasAgent"`
	HasKbReferences bool             `json:"hasKbReferences"`
	CreatedOn       string           `json:"createdOn"`
	CreatedBy       *string          `json:"createdBy"`
	UpdatedOn       string           `json:"updatedOn"`
	// Partner and PrimaryPartnerAccountID are the raw ServiceNow
	// customer_account.partner/u_primary_partner_account_id fields, merged in by the
	// Ballerina entity-service from a separate Table API read (not part of the scoped
	// app's own /accounts response). Naming/derivation into IsPartner/HasPrimaryPartner
	// happens in this layer -- see snAccountCommonFields.
	Partner                 *bool            `json:"partner"`
	PrimaryPartnerAccountID *snCaseEntityRef `json:"primaryPartnerAccountId"`
}

// snAccountSearchPayload is the Choreo POST /accounts/search request body.
type snAccountSearchPayload struct {
	Filters    snAccountFilters    `json:"filters,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snAccountFilters struct {
	SearchQuery    string `json:"searchQuery,omitempty"`
	Active         *bool  `json:"active,omitempty"`
	Pod            string `json:"pod,omitempty"`
	Classification string `json:"classification,omitempty"`
}

type snAccountService struct {
	client *integrationservice.Client
}

// NewServiceNowAccountService constructs an SNAccountService backed by the Choreo API.
func NewServiceNowAccountService(client *integrationservice.Client) SNAccountService {
	return &snAccountService{client: client}
}

func (s *snAccountService) SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) (domain.SearchAccountsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAccountsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchAccountsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snAccountSearchPayload{
		Filters: snAccountFilters{
			SearchQuery:    req.Filters.SearchQuery,
			Active:         req.Filters.Active,
			Pod:            req.Filters.Pod,
			Classification: req.Filters.Classification,
		},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/accounts/search", token, payload)
	if err != nil {
		return domain.SearchAccountsResponse{}, err
	}

	var snResp snAccountsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchAccountsResponse{}, fmt.Errorf("sn accounts: parse response: %w", err)
	}

	accounts := make([]domain.AccountView, 0, len(snResp.Accounts))
	for _, a := range snResp.Accounts {
		accounts = append(accounts, snAccountToDomain(a))
	}

	return domain.SearchAccountsResponse{
		Accounts: accounts,
		Total:    snResp.TotalRecords,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(accounts) < snResp.TotalRecords,
	}, nil
}

func (s *snAccountService) GetAccountByID(ctx context.Context, id string) (domain.AccountDetail, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AccountDetail{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/accounts/"+uuidToSysid(id), token)
	if err != nil {
		return domain.AccountDetail{}, err
	}

	var a snAccount
	if err := json.Unmarshal(raw, &a); err != nil {
		return domain.AccountDetail{}, fmt.Errorf("sn accounts: parse account response: %w", err)
	}

	return snAccountToDetail(a), nil
}

func nilIfEmpty(s *string) *string {
	if s != nil && *s == "" {
		return nil
	}
	return s
}

// snHasPrimaryPartner derives AccountView/AccountDetail.HasPrimaryPartner from the raw
// primaryPartnerAccountId reference: nil when the key is entirely absent (Ballerina never
// had a value for this account -- e.g. Postgres data source, or ServiceNow's own Table API
// row omitted the column), true when present with a real id, false when present but the
// reference is empty. In practice the Ballerina layer's own displayValuePairToReference
// already collapses "present but empty" into "entirely absent" before this ever reaches the
// wire, so false is not currently observable from live ServiceNow data -- but this function
// still handles it correctly in case that upstream behavior ever changes.
func snHasPrimaryPartner(ref *snCaseEntityRef) *bool {
	if ref == nil {
		return nil
	}
	has := ref.ID != ""
	return &has
}

func snAccountCommonFields(a snAccount) (deactivationDate *string, technicalOwner, accountManager, renewalAccountManager *domain.PersonRef, creTeam, sreTeam *domain.EntityRef) {
	deactivationDate = nilIfEmpty(a.DeactivationDate)
	if a.TechnicalOwner != nil && a.TechnicalOwner.ID != "" {
		technicalOwner = &domain.PersonRef{ID: sysidToUUID(a.TechnicalOwner.ID), Name: a.TechnicalOwner.Name, Email: nilIfEmpty(a.TechnicalOwner.Email)}
	}
	if a.Owner != nil && a.Owner.ID != "" {
		accountManager = &domain.PersonRef{ID: sysidToUUID(a.Owner.ID), Name: a.Owner.Name, Email: nilIfEmpty(a.Owner.Email)}
	}
	if a.RenewalAccountManager != nil && a.RenewalAccountManager.ID != "" {
		renewalAccountManager = &domain.PersonRef{ID: sysidToUUID(a.RenewalAccountManager.ID), Name: a.RenewalAccountManager.Name, Email: nilIfEmpty(a.RenewalAccountManager.Email)}
	}
	// CreTeam/SreTeam (see snAccount.CreTeam/SreTeam doc comment) pass through once
	// non-empty, same as the case service's account-embedded team refs.
	if a.CreTeam != nil {
		if id := sysidToUUID(a.CreTeam.ID); id != "" {
			creTeam = &domain.EntityRef{ID: id, Name: a.CreTeam.Name}
		}
	}
	if a.SreTeam != nil {
		if id := sysidToUUID(a.SreTeam.ID); id != "" {
			sreTeam = &domain.EntityRef{ID: id, Name: a.SreTeam.Name}
		}
	}
	return
}

func snAccountToDomain(a snAccount) domain.AccountView {
	deactivationDate, technicalOwner, accountManager, renewalAccountManager, creTeam, sreTeam := snAccountCommonFields(a)

	var supportTier *string
	if a.SupportTier != nil && a.SupportTier.Label != "" {
		supportTier = &a.SupportTier.Label
	}

	return domain.AccountView{
		ID:                    sysidToUUID(a.ID),
		Name:                  a.Name,
		Classification:        a.Classification,
		Pod:                   nilIfEmpty(a.Pod),
		SfID:                  nilIfEmpty(a.SfID),
		Region:                nilIfEmpty(a.Region),
		SupportTier:           supportTier,
		ArrToday:              nilIfEmpty(a.ArrToday),
		TechnicalOwner:        technicalOwner,
		AccountManager:        accountManager,
		RenewalAccountManager: renewalAccountManager,
		CreTeam:               creTeam,
		SreTeam:               sreTeam,
		ActivationDate:        nilIfEmpty(&a.ActivationDate),
		DeactivationDate:      deactivationDate,
		HasAgent:              a.HasAgent,
		HasKbReferences:       a.HasKbReferences,
		CreatedOn:             a.CreatedOn,
		CreatedBy:             nilIfEmpty(a.CreatedBy),
		UpdatedOn:             a.UpdatedOn,
		IsPartner:             a.Partner,
		HasPrimaryPartner:     snHasPrimaryPartner(a.PrimaryPartnerAccountID),
	}
}

func snAccountToDetail(a snAccount) domain.AccountDetail {
	deactivationDate, technicalOwner, accountManager, renewalAccountManager, creTeam, sreTeam := snAccountCommonFields(a)

	var supportTier *domain.SNSupportTierRef
	if a.SupportTier != nil && a.SupportTier.ID != "" {
		supportTier = &domain.SNSupportTierRef{
			ID:    sysidToUUID(a.SupportTier.ID),
			Label: a.SupportTier.Label,
		}
	}

	return domain.AccountDetail{
		ID:                    sysidToUUID(a.ID),
		Name:                  a.Name,
		Classification:        a.Classification,
		Pod:                   nilIfEmpty(a.Pod),
		SfID:                  nilIfEmpty(a.SfID),
		Region:                nilIfEmpty(a.Region),
		SupportTier:           supportTier,
		ArrToday:              nilIfEmpty(a.ArrToday),
		TechnicalOwner:        technicalOwner,
		AccountManager:        accountManager,
		RenewalAccountManager: renewalAccountManager,
		CreTeam:               creTeam,
		SreTeam:               sreTeam,
		ActivationDate:        nilIfEmpty(&a.ActivationDate),
		DeactivationDate:      deactivationDate,
		HasAgent:              a.HasAgent,
		HasKbReferences:       a.HasKbReferences,
		CreatedOn:             a.CreatedOn,
		CreatedBy:             nilIfEmpty(a.CreatedBy),
		UpdatedOn:             a.UpdatedOn,
		IsPartner:             a.Partner,
		HasPrimaryPartner:     snHasPrimaryPartner(a.PrimaryPartnerAccountID),
	}
}

// snAccountContactsResponse mirrors the Choreo POST /accounts/{id}/contacts/search response.
type snAccountContactsResponse struct {
	Contacts     []snAccountContact `json:"contacts"`
	TotalRecords int                `json:"totalRecords"`
	Offset       int                `json:"offset"`
	Limit        int                `json:"limit"`
}

type snAccountContact struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	IsPrimary bool   `json:"isPrimary"`
}

type snAccountContactService struct {
	client *integrationservice.Client
}

// NewServiceNowAccountContactService constructs an AccountContactService backed by the Choreo API.
func NewServiceNowAccountContactService(client *integrationservice.Client) AccountContactService {
	return &snAccountContactService{client: client}
}

// SearchAccountContacts implements AccountContactService. Supported by the ServiceNow
// data source only; there is no Postgres fallback.
func (s *snAccountContactService) SearchAccountContacts(ctx context.Context, accountID string, req domain.SearchAccountContactsRequest) (domain.SearchAccountContactsResponse, error) {
	if err := validateUUIDs("id", []string{accountID}); err != nil {
		return domain.SearchAccountContactsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAccountContactsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchAccountContactsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snContactSearchPayload{
		Filters:    snContactFilters{SearchQuery: req.Filters.SearchQuery},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/accounts/"+uuidToSysid(accountID)+"/contacts/search", token, payload)
	if err != nil {
		return domain.SearchAccountContactsResponse{}, err
	}

	var snResp snAccountContactsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchAccountContactsResponse{}, fmt.Errorf("sn account contacts: parse response: %w", err)
	}

	contacts := make([]domain.AccountContact, 0, len(snResp.Contacts))
	for _, c := range snResp.Contacts {
		contacts = append(contacts, domain.AccountContact{
			Name:      c.Name,
			Email:     c.Email,
			IsPrimary: c.IsPrimary,
		})
	}

	return domain.SearchAccountContactsResponse{
		Contacts: contacts,
		Total:    snResp.TotalRecords,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}
