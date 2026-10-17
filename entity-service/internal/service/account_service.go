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

// Package service is declared in interfaces.go.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type accountService struct {
	repo repository.AccountRepository
}

// NewAccountService constructs an AccountService backed by the given repository.
func NewAccountService(repo repository.AccountRepository) AccountService {
	return &accountService{repo: repo}
}

// SearchAccounts implements AccountService.
func (s *accountService) SearchAccounts(ctx context.Context, req domain.SearchAccountsRequest) (domain.SearchAccountsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAccountsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchAccountsResponse{}, err
	}
	rows, total, err := s.repo.SearchAccounts(ctx, req)
	if err != nil {
		return domain.SearchAccountsResponse{}, err
	}

	accounts := make([]domain.AccountView, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, accountRowToView(row))
	}

	return domain.SearchAccountsResponse{
		Accounts: accounts,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(accounts) < total,
	}, nil
}

// GetAccountByID implements AccountService.
func (s *accountService) GetAccountByID(ctx context.Context, id string) (domain.AccountDetail, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AccountDetail{}, err
	}
	row, err := s.repo.GetAccountByID(ctx, id)
	if err != nil {
		return domain.AccountDetail{}, err
	}
	return accountRowToDetail(row), nil
}

// GetAccountHealthDetail implements AccountService.
func (s *accountService) GetAccountHealthDetail(ctx context.Context, id string) (domain.AccountHealthDetail, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AccountHealthDetail{}, err
	}
	row, err := s.repo.GetAccountHealthDetail(ctx, id)
	if err != nil {
		return domain.AccountHealthDetail{}, err
	}
	projects := make([]domain.ProjectHealthDetail, 0, len(row.Projects))
	for _, p := range row.Projects {
		detail := p.Detail
		detail.GoLiveStatus = accountGoLiveStatus(p.GoLiveState, row.GoLiveMonths, row.ActivationDate)
		projects = append(projects, detail)
	}
	return domain.AccountHealthDetail{AccountName: row.AccountName, CustomerProjects: projects}, nil
}

// UpdateAccountTeams implements AccountService.
func (s *accountService) UpdateAccountTeams(ctx context.Context, req domain.UpdateAccountTeamsRequest) (domain.AccountDetail, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.AccountDetail{}, err
	}
	if req.CreTeamID == nil && req.SreTeamID == nil {
		return domain.AccountDetail{}, &apierror.ValidationError{Msg: "at least one of creTeamId or sreTeamId must be provided"}
	}
	ids := []string{}
	if req.CreTeamID != nil {
		ids = append(ids, *req.CreTeamID)
	}
	if req.SreTeamID != nil {
		ids = append(ids, *req.SreTeamID)
	}
	if err := validateUUIDs("creTeamId/sreTeamId", ids); err != nil {
		return domain.AccountDetail{}, err
	}

	row, err := s.repo.UpdateAccountTeams(ctx, req.ID, req.CreTeamID, req.SreTeamID)
	if err != nil {
		return domain.AccountDetail{}, err
	}
	return accountRowToDetail(row), nil
}

// accountTeamRef builds an EntityRef from a joined "group" id/name pair,
// returning nil when the id is absent (no team linked) -- same nil
// convention as accountPersonRef, just for a group rather than a user.
func accountTeamRef(id, name *string) *domain.EntityRef {
	if id == nil || *id == "" {
		return nil
	}
	var teamName string
	if name != nil {
		teamName = *name
	}
	return &domain.EntityRef{ID: *id, Name: teamName}
}

// accountPersonRef builds a PersonRef from a joined person id/name/email triple,
// returning nil when the id is absent (no owner/manager linked).
func accountPersonRef(id, name, email *string) *domain.PersonRef {
	if id == nil || *id == "" {
		return nil
	}
	var personName string
	if name != nil {
		personName = *name
	}
	return &domain.PersonRef{ID: *id, Name: personName, Email: email}
}

func dateOnlyOrNil(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func boolOrFalse(b *bool) bool {
	return b != nil && *b
}

// accountIsPartner applies the Salesforce membership mapping's partner rule
// (classification "Partner") to the account row.
func accountIsPartner(classification *string) *bool {
	v := classification != nil && strings.EqualFold(strings.TrimSpace(*classification), salesforceAccountClassificationPartner)
	return &v
}

// accountRowCommonFields maps the fields shared between the search view and
// the account detail response. SupportTier and ArrToday are not available
// in the Postgres schema and are always nil.
func accountRowCommonFields(row repository.AccountRow) (classification string, technicalOwner, accountManager, renewalAccountManager, customerSuccessManager *domain.PersonRef, creTeam, sreTeam *domain.EntityRef) {
	if row.Classification != nil {
		classification = *row.Classification
	}
	technicalOwner = accountPersonRef(row.TechnicalOwnerID, row.TechnicalOwnerName, row.TechnicalOwnerEmail)
	accountManager = accountPersonRef(row.AccountManagerID, row.AccountManagerName, row.AccountManagerEmail)
	renewalAccountManager = accountPersonRef(row.RenewalAccountManagerID, row.RenewalAccountManagerName, row.RenewalAccountManagerEmail)
	customerSuccessManager = accountPersonRef(row.CustomerSuccessManagerID, row.CustomerSuccessManagerName, row.CustomerSuccessManagerEmail)
	creTeam = accountTeamRef(row.CreTeamID, row.CreTeamName)
	sreTeam = accountTeamRef(row.SreTeamID, row.SreTeamName)
	return
}

func accountRowToView(row repository.AccountRow) domain.AccountView {
	classification, technicalOwner, accountManager, renewalAccountManager, customerSuccessManager, creTeam, sreTeam := accountRowCommonFields(row)
	createdBy := row.CreatedBy

	return domain.AccountView{
		ID:                     row.ID,
		Name:                   row.Name,
		Number:                 row.Number,
		Classification:         classification,
		Pod:                    row.Pod,
		SfID:                   row.SfID,
		Region:                 row.Region,
		SubRegion:              row.SubRegion,
		Country:                row.Country,
		City:                   row.City,
		DriveLocation:          row.DriveLocation,
		SupportTier:            nil,
		ArrToday:               nil,
		TechnicalOwner:         technicalOwner,
		AccountManager:         accountManager,
		RenewalAccountManager:  renewalAccountManager,
		CustomerSuccessManager: customerSuccessManager,
		CreTeam:                creTeam,
		SreTeam:                sreTeam,
		ActivationDate:         dateOnlyOrNil(row.ActivationDate),
		DeactivationDate:       dateOnlyOrNil(row.DeactivationDate),
		HasAgent:               boolOrFalse(row.HasAgent),
		HasKbReferences:        boolOrFalse(row.HasKbReferences),
		CreatedOn:              row.CreatedOn.Format(time.RFC3339),
		CreatedBy:              &createdBy,
		UpdatedOn:              row.UpdatedOn.Format(time.RFC3339),
		IsPartner:              accountIsPartner(row.Classification),
		HasPrimaryPartner:      &row.HasPrimaryPartner,
		HealthSummary:          accountHealthSummaryFromRow(row),
	}
}

// accountHealthSummaryFromRow maps the health columns a health-summary search
// read onto the response shape, or returns nil when the search did not ask
// for them.
func accountHealthSummaryFromRow(row repository.AccountRow) *domain.AccountSearchHealthSummary {
	h := row.Health
	if h == nil {
		return nil
	}
	return &domain.AccountSearchHealthSummary{
		GoLiveStatus:           accountGoLiveStatus(h.GoLiveState, h.GoLiveMonths, row.ActivationDate),
		HasRecentCases:         h.HasRecentCases,
		HasEolProduct:          h.HasEolProduct,
		HasAbandonedMigrations: h.HasAbandonedMigrations,
		HasMigrationDelays:     h.HasMigrationDelays,
		HasRecentEscalations:   h.HasRecentEscalations,
		NoSupportCases6mo:      !h.HasRecentCases,
		HealthStatus:           h.HealthStatus,
	}
}

// accountGoLiveStatus builds the display status for the state the search
// computed. Pending has four variants, told apart by the activation date and
// the whole months elapsed since it (nil months: no date, or one in the
// future).
func accountGoLiveStatus(state string, months *int, activation *time.Time) domain.AccountGoLiveStatus {
	switch state {
	case "success":
		return domain.AccountGoLiveStatus{Status: "Has Gone Live", State: "success"}
	case "failure":
		return domain.AccountGoLiveStatus{Status: "Hasn't Gone Live in 6 months", IsRisk: true, State: "failure"}
	}
	switch {
	case activation == nil:
		return domain.AccountGoLiveStatus{Status: "N/A (No Start Date)", State: "pending"}
	case months == nil:
		return domain.AccountGoLiveStatus{Status: "N/A (Activation Date in Future)", State: "pending"}
	case *months == 0:
		return domain.AccountGoLiveStatus{Status: "Recently Activated (Less than a month)", State: "pending"}
	case *months == 1:
		return domain.AccountGoLiveStatus{Status: "Hasn't Gone Live Since 1 month", State: "pending"}
	default:
		return domain.AccountGoLiveStatus{Status: fmt.Sprintf("Hasn't Gone Live Since %d months", *months), State: "pending"}
	}
}

func accountRowToDetail(row repository.AccountRow) domain.AccountDetail {
	classification, technicalOwner, accountManager, renewalAccountManager, customerSuccessManager, creTeam, sreTeam := accountRowCommonFields(row)
	createdBy := row.CreatedBy

	return domain.AccountDetail{
		ID:                     row.ID,
		Name:                   row.Name,
		Number:                 row.Number,
		Classification:         classification,
		Pod:                    row.Pod,
		SfID:                   row.SfID,
		Region:                 row.Region,
		Country:                row.Country,
		City:                   row.City,
		DriveLocation:          row.DriveLocation,
		SupportTier:            nil,
		ArrToday:               nil,
		TechnicalOwner:         technicalOwner,
		AccountManager:         accountManager,
		RenewalAccountManager:  renewalAccountManager,
		CustomerSuccessManager: customerSuccessManager,
		CreTeam:                creTeam,
		SreTeam:                sreTeam,
		ActivationDate:         dateOnlyOrNil(row.ActivationDate),
		DeactivationDate:       dateOnlyOrNil(row.DeactivationDate),
		HasAgent:               boolOrFalse(row.HasAgent),
		HasKbReferences:        boolOrFalse(row.HasKbReferences),
		CreatedOn:              row.CreatedOn.Format(time.RFC3339),
		CreatedBy:              &createdBy,
		UpdatedOn:              row.UpdatedOn.Format(time.RFC3339),
		IsPartner:              accountIsPartner(row.Classification),
		HasPrimaryPartner:      &row.HasPrimaryPartner,
	}
}
