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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
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

type snAccount struct {
	ID               string  `json:"id"`
	SfID             string  `json:"sfId"`
	Name             string  `json:"name"`
	SupportTier      string  `json:"supportTier"`
	Region           *string `json:"region"`
	ActivationDate   string  `json:"activationDate"`
	DeactivationDate *string `json:"deactivationDate"`
	OwnerID          string  `json:"ownerId"`
	TechnicalOwnerID *string `json:"technicalOwnerId"`
	HasAgent         bool    `json:"hasAgent"`
	HasKbReferences  bool    `json:"hasKbReferences"`
	CreatedOn        string  `json:"createdOn"`
	UpdatedOn        string  `json:"updatedOn"`
}

// snAccountSearchPayload is the Choreo POST /accounts/search request body.
type snAccountSearchPayload struct {
	Filters    snAccountFilters    `json:"filters,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snAccountFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
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
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchSNAccountsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchSNAccountsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snAccountSearchPayload{
		Filters:    snAccountFilters{SearchQuery: req.SearchQuery},
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

	accounts := make([]domain.SNAccount, 0, len(snResp.Accounts))
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

func (s *snAccountService) GetAccountByID(ctx context.Context, id string) (domain.SNAccount, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SNAccount{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SNAccount{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	raw, err := s.client.Get(ctx, "/accounts/"+uuidToSysid(id), token)
	if err != nil {
		return domain.SNAccount{}, err
	}

	var a snAccount
	if err := json.Unmarshal(raw, &a); err != nil {
		return domain.SNAccount{}, fmt.Errorf("sn accounts: parse account response: %w", err)
	}

	return snAccountToDomain(a), nil
}

func snAccountToDomain(a snAccount) domain.SNAccount {
	var deactivationDate *string
	if a.DeactivationDate != nil && *a.DeactivationDate != "" {
		deactivationDate = a.DeactivationDate
	}

	var technicalOwnerID *string
	if a.TechnicalOwnerID != nil && *a.TechnicalOwnerID != "" {
		uid := sysidToUUID(*a.TechnicalOwnerID)
		technicalOwnerID = &uid
	}

	return domain.SNAccount{
		ID:                  sysidToUUID(a.ID),
		SfID:                a.SfID,
		Name:                a.Name,
		Tier:                strings.ToLower(a.SupportTier),
		Region:              a.Region,
		ActivationDate:      a.ActivationDate,
		DeactivationDate:    deactivationDate,
		OwnerID:             sysidToUUID(a.OwnerID),
		TechnicalOwnerID:    technicalOwnerID,
		AgentEnabled:        a.HasAgent,
		KbReferencesEnabled: a.HasKbReferences,
		CreatedOn:           a.CreatedOn,
		UpdatedOn:           a.UpdatedOn,
	}
}
