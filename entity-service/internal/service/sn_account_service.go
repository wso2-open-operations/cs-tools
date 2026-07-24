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
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snAccount struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	SupportTier      *snSupportTier `json:"supportTier"`
	Classification   string         `json:"classification"`
	Pod              *string        `json:"pod"`
	Region           *string        `json:"region"`
	ArrToday         *string        `json:"arrToday"`
	ActivationDate   string         `json:"activationDate"`
	DeactivationDate *string        `json:"deactivationDate"`
	TechnicalOwner   *snPersonRef   `json:"technicalOwner"`
	Owner            *snPersonRef   `json:"owner"`
	HasAgent         bool           `json:"hasAgent"`
	HasKbReferences  bool           `json:"hasKbReferences"`
	CreatedOn        string         `json:"createdOn"`
	CreatedBy        *string        `json:"createdBy"`
	UpdatedOn        string         `json:"updatedOn"`
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

func (s *snAccountService) SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) (domain.SearchSNAccountsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchSNAccountsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchSNAccountsResponse{}, err
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
		return domain.SearchSNAccountsResponse{}, err
	}

	var snResp snAccountsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchSNAccountsResponse{}, fmt.Errorf("sn accounts: parse response: %w", err)
	}

	accounts := make([]domain.SNAccountView, 0, len(snResp.Accounts))
	for _, a := range snResp.Accounts {
		accounts = append(accounts, snAccountToDomain(a))
	}

	return domain.SearchSNAccountsResponse{
		Accounts: accounts,
		Total:    snResp.TotalRecords,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(accounts) < snResp.TotalRecords,
	}, nil
}

func (s *snAccountService) GetAccountByID(ctx context.Context, id string) (domain.SNAccountDetail, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SNAccountDetail{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/accounts/"+uuidToSysid(id), token)
	if err != nil {
		return domain.SNAccountDetail{}, err
	}

	var a snAccount
	if err := json.Unmarshal(raw, &a); err != nil {
		return domain.SNAccountDetail{}, fmt.Errorf("sn accounts: parse account response: %w", err)
	}

	return snAccountToDetail(a), nil
}

func nilIfEmpty(s *string) *string {
	if s != nil && *s == "" {
		return nil
	}
	return s
}

func snAccountCommonFields(a snAccount) (deactivationDate *string, technicalOwner, owner *domain.EntityRef) {
	deactivationDate = nilIfEmpty(a.DeactivationDate)
	if a.TechnicalOwner != nil && a.TechnicalOwner.ID != "" {
		technicalOwner = &domain.EntityRef{ID: sysidToUUID(a.TechnicalOwner.ID), Name: a.TechnicalOwner.Name}
	}
	if a.Owner != nil && a.Owner.ID != "" {
		owner = &domain.EntityRef{ID: sysidToUUID(a.Owner.ID), Name: a.Owner.Name}
	}
	return
}

func snAccountToDomain(a snAccount) domain.SNAccountView {
	deactivationDate, technicalOwner, owner := snAccountCommonFields(a)

	var supportTier *string
	if a.SupportTier != nil && a.SupportTier.Label != "" {
		supportTier = &a.SupportTier.Label
	}

	return domain.SNAccountView{
		ID:               sysidToUUID(a.ID),
		Name:             a.Name,
		Classification:   a.Classification,
		Pod:              nilIfEmpty(a.Pod),
		Region:           nilIfEmpty(a.Region),
		SupportTier:      supportTier,
		ArrToday:         nilIfEmpty(a.ArrToday),
		TechnicalOwner:   technicalOwner,
		Owner:            owner,
		ActivationDate:   a.ActivationDate,
		DeactivationDate: deactivationDate,
		HasAgent:         a.HasAgent,
		HasKbReferences:  a.HasKbReferences,
		CreatedOn:        a.CreatedOn,
		CreatedBy:        nilIfEmpty(a.CreatedBy),
		UpdatedOn:        a.UpdatedOn,
	}
}

func snAccountToDetail(a snAccount) domain.SNAccountDetail {
	deactivationDate, technicalOwner, owner := snAccountCommonFields(a)

	var supportTier *domain.SNSupportTierRef
	if a.SupportTier != nil && a.SupportTier.ID != "" {
		supportTier = &domain.SNSupportTierRef{
			ID:    sysidToUUID(a.SupportTier.ID),
			Label: a.SupportTier.Label,
		}
	}

	return domain.SNAccountDetail{
		ID:               sysidToUUID(a.ID),
		Name:             a.Name,
		Classification:   a.Classification,
		Pod:              nilIfEmpty(a.Pod),
		Region:           nilIfEmpty(a.Region),
		SupportTier:      supportTier,
		ArrToday:         nilIfEmpty(a.ArrToday),
		TechnicalOwner:   technicalOwner,
		Owner:            owner,
		ActivationDate:   a.ActivationDate,
		DeactivationDate: deactivationDate,
		HasAgent:         a.HasAgent,
		HasKbReferences:  a.HasKbReferences,
		CreatedOn:        a.CreatedOn,
		CreatedBy:        nilIfEmpty(a.CreatedBy),
		UpdatedOn:        a.UpdatedOn,
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
