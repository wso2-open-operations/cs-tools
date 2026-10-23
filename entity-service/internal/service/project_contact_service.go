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
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type projectContactService struct {
	repo   repository.ProjectContactRepository
	access AccessService
	// writes is the deferred Salesforce write ledger; nil leaves
	// SalesforceSync empty (see WithProjectContactSalesforceSync).
	writes repository.SalesforceWriteRequestRepository
}

// NewProjectContactService constructs a ProjectContactService backed by Postgres.
func NewProjectContactService(repo repository.ProjectContactRepository, access AccessService) ProjectContactService {
	return &projectContactService{repo: repo, access: access}
}

// WithProjectContactSalesforceSync makes the contact reads report
// domain.ProjectContact.SalesforceSync from the deferred Salesforce write
// ledger (see project_membership_deferred_write.go). Optional: without it,
// or when svc is not this package's implementation, the field stays empty.
func WithProjectContactSalesforceSync(svc ProjectContactService, writes repository.SalesforceWriteRequestRepository) ProjectContactService {
	if impl, ok := svc.(*projectContactService); ok && writes != nil {
		impl.writes = writes
	}
	return svc
}

// attachSalesforceSync fills SalesforceSync for the contacts that have a
// deferred write recorded. Best-effort: a ledger read failure is logged and
// the list is served without the field rather than not at all.
func (s *projectContactService) attachSalesforceSync(ctx context.Context, rows []repository.ProjectContactRow, contacts []domain.ProjectContact) {
	if s.writes == nil || len(rows) == 0 {
		return
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ProjectContactID != "" {
			ids = append(ids, row.ProjectContactID)
		}
	}
	statuses, err := s.writes.SyncStatusForProjectContacts(ctx, ids)
	if err != nil {
		slog.WarnContext(ctx, "project contacts: could not read the Salesforce sync status", "err", err)
		return
	}
	for i, row := range rows {
		if status, ok := statuses[row.ProjectContactID]; ok && i < len(contacts) {
			st := status
			contacts[i].SalesforceSync = &st
		}
	}
}

// registeredProjectContactState is the state the access check requires.
const registeredProjectContactState = "REGISTERED"

func projectContactRowToDomain(row repository.ProjectContactRow) domain.ProjectContact {
	pc := domain.ProjectContact{
		Email:             row.Email,
		RegistrationState: row.RegistrationState,
		// NotificationsEnabled has no backing column anywhere in this
		// schema (project_contact, migration 0027, carries no preference
		// column). Defaulted true rather than false: an invited/registered
		// contact is assumed opted-in until a real preference column
		// exists, matching this schema's general "absence means the
		// permissive default" posture elsewhere (e.g. nullable boolean
		// flags read as false only when explicitly set).
		NotificationsEnabled: true,
		IsCsIntegrationUser:  row.IsSystemUser,
		Roles:                row.Roles,
		AccountRoles:         row.AccountRoles,
	}
	if pc.Roles == nil {
		pc.Roles = []string{}
	}
	if pc.AccountRoles == nil {
		pc.AccountRoles = []string{}
	}
	if row.ResolvedName != nil && *row.ResolvedName != "" {
		name := *row.ResolvedName
		pc.Name = &name
	}
	if row.ResolvedUserID != nil {
		id := *row.ResolvedUserID
		pc.ID = &id
		// CustomerContactPresent: a "user" row was resolved via
		// account_contact.user_name at all -- see ProjectContactRow's own
		// doc comment.
		pc.CustomerContactPresent = true
		// GrantsCaseAccess mirrors AccessRepository.RegisteredProjectIDs: the
		// user's email matches the invited address and the row is REGISTERED.
		if row.ResolvedEmail != nil && strings.EqualFold(*row.ResolvedEmail, row.Email) &&
			row.RegistrationState == registeredProjectContactState {
			pc.GrantsCaseAccess = true
		}
		// Email is the linked user's address, else the invited one (as on ServiceNow).
		if row.ResolvedEmail != nil && *row.ResolvedEmail != "" {
			pc.Email = *row.ResolvedEmail
		}
	}
	return pc
}

// SearchProjectContacts implements ProjectContactService.
func (s *projectContactService) SearchProjectContacts(ctx context.Context, projectID string, req domain.SearchProjectContactsRequest) (domain.SearchProjectContactsResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}

	callerEmail, err := resolveContactCaller(ctx, s.access)
	if err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}

	rows, total, err := s.repo.SearchProjectContacts(ctx, projectID, req, callerEmail)
	if err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}

	contacts := make([]domain.ProjectContact, 0, len(rows))
	for _, row := range rows {
		contacts = append(contacts, projectContactRowToDomain(row))
	}
	s.attachSalesforceSync(ctx, rows, contacts)

	return domain.SearchProjectContactsResponse{
		Contacts: contacts,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}

// GetProjectContact implements ProjectContactService.
func (s *projectContactService) GetProjectContact(ctx context.Context, projectID, contactID string) (domain.ProjectContact, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectContact{}, err
	}
	if err := validateUUIDs("contactId", []string{contactID}); err != nil {
		return domain.ProjectContact{}, err
	}

	callerEmail, err := resolveContactCaller(ctx, s.access)
	if err != nil {
		return domain.ProjectContact{}, err
	}

	row, err := s.repo.GetProjectContactByUserID(ctx, projectID, contactID, callerEmail)
	if err != nil {
		return domain.ProjectContact{}, err
	}

	contacts := []domain.ProjectContact{projectContactRowToDomain(row)}
	s.attachSalesforceSync(ctx, []repository.ProjectContactRow{row}, contacts)
	return contacts[0], nil
}
