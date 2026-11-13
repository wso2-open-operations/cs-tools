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
	"errors"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// AccountRefresher re-reads one account from Salesforce and writes it whatever
// the ledger records. Satisfied by the Salesforce event service.
type AccountRefresher interface {
	RefreshAccount(ctx context.Context, sfID string) error
}

// RefreshAccount writes the account from its current Salesforce record with
// the duplicate guard off. An Account event for a version the ledger already
// holds is skipped, so replaying events cannot fill a column added after the
// account was last written (primary_partner_sf_id, migration 0291); this can.
// Incoming events keep the guard. The partner links are not refreshed here
// (POST /salesforce/accounts/{sfId}/refresh-partners does that).
func (s *salesforceEventService) RefreshAccount(ctx context.Context, sfID string) error {
	sfID = strings.TrimSpace(sfID)
	if sfID == "" {
		return &apierror.ValidationError{Msg: "account sfId is required"}
	}
	if s.repo == nil {
		return errors.New("salesforce: account ingest is not configured")
	}
	return s.upsertAccount(ctx, sfID, domain.SalesforceEventUpdated, false)
}

// AccountRefreshService backs POST /salesforce/accounts/{sfId}/refresh.
type AccountRefreshService interface {
	Refresh(ctx context.Context, sfID string) error
}

type accountRefreshService struct {
	refresher AccountRefresher
	access    AccessService
}

// NewAccountRefreshService constructs the internal endpoint's service. Every
// call is restricted to internal callers (an Unrestricted AccessScope), as for
// the partner refresh: it makes this service read Salesforce and write accounts.
func NewAccountRefreshService(refresher AccountRefresher, access AccessService) AccountRefreshService {
	return &accountRefreshService{refresher: refresher, access: access}
}

// Refresh implements AccountRefreshService.
func (a *accountRefreshService) Refresh(ctx context.Context, sfID string) error {
	scope, err := a.access.ResolveScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Unrestricted {
		return &apierror.ForbiddenError{Msg: "account refresh is only available to internal services"}
	}
	if strings.TrimSpace(sfID) == "" {
		return &apierror.ValidationError{Msg: "sfId is required"}
	}
	return a.refresher.RefreshAccount(ctx, sfID)
}
