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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// SalesEntityPartnerClient lists a customer's partner accounts from REST
// sales/sales-entity-service (customer-search with includePartners).
type SalesEntityPartnerClient interface {
	GetCustomerPartners(ctx context.Context, id string) (string, []salesentity.CustomerPartner, error)
}

// PartnerIngest bundles the dependencies of the partner-relationship
// refresh. It is optional: a service without it answers every
// RefreshPartners call with a no-op, which is how
// CSM_MIGRATION_SALESFORCE_PARTNER_INGEST_ENABLED=false is realised in
// routes.go. That one flag therefore gates every caller — the Account event,
// the partner-contact membership event, the internal endpoint and the retry
// job — because each of them goes through RefreshPartners.
type PartnerIngest struct {
	Partners    repository.AccountPartnerWriteRepository
	SalesEntity SalesEntityPartnerClient
}

func (p *PartnerIngest) enabled() bool {
	return p != nil && p.Partners != nil && p.SalesEntity != nil
}

// PartnerRefresher refreshes one customer's stored partner set from
// Salesforce. The internal endpoint and the retry job use it.
type PartnerRefresher interface {
	RefreshPartners(ctx context.Context, customerSfID string) (domain.SalesforcePartnerRefreshResult, error)
}

// WithPartnerIngest turns on the partner refresh of a service built by one
// of the NewSalesforceEventService constructors and returns it, like
// WithOpportunityIngest.
func WithPartnerIngest(svc SalesforceEventService, ingest PartnerIngest) SalesforceEventService {
	if s, ok := svc.(*salesforceEventService); ok {
		s.partners = &ingest
	}
	return svc
}

// RefreshPartners makes account_relationship's partner links for one
// customer equal to the partner set Salesforce holds (plan §4, decision D8:
// store a copy). Salesforce emits no event when a partner is added or
// removed, so the copy is refreshed from the events that do arrive — the
// customer's own Account events and its partner contacts' membership events
// — and from the internal endpoint.
//
// TODO(salesforce-sync): a nightly sweep that calls RefreshPartners for
// every account with an sf_id. It is the only thing that catches a partner
// added in Salesforce with no related save; until it exists such a link
// waits for the customer's next Account event or partner-contact membership
// event.
//
// The steps: read the partner list from Sales Entity; ensure the customer and
// every partner exist as accounts (EnsureAccount, which ingests a missing one
// when the Account ingest is on — the script's syncCustomer before the link);
// then, in one transaction under an advisory lock on the customer, insert the
// missing forward and reverse rows and delete the ones whose partner left the
// set, recording a SUCCEEDED ledger row (entity account_partners). Any
// failure before the write leaves the stored set as it was: a partner that
// cannot be ensured fails the refresh rather than being dropped, since
// dropping it would delete its existing link. Replaying is harmless — the
// same set yields no change — so there is no duplicate guard.
func (s *salesforceEventService) RefreshPartners(ctx context.Context, customerSfID string) (domain.SalesforcePartnerRefreshResult, error) {
	customerSfID = salesforceID18(customerSfID)
	res := domain.SalesforcePartnerRefreshResult{CustomerSfID: customerSfID, PartnerSfIDs: []string{}}
	if !s.partners.enabled() {
		slog.DebugContext(ctx, "salesforce: partner ingest disabled, not refreshing partners", "accountSfId", customerSfID)
		res.Disabled = true
		return res, nil
	}
	if customerSfID == "" {
		return res, &apierror.ValidationError{Msg: "account sfId is required"}
	}
	if s.support.States == nil {
		return res, errors.New("salesforce: salesforce_ingest_state ledger is not configured")
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityAccountPartners,
		SfID:            customerSfID,
		EventModifiedOn: time.Now().UTC(),
		EventType:       domain.SalesforceEventUpdated,
		Status:          domain.SalesforceIngestSucceeded,
	}

	returnedID, partners, err := s.partners.SalesEntity.GetCustomerPartners(ctx, customerSfID)
	if err != nil {
		s.recordIngestFailed(ctx, state, err)
		return res, err
	}
	if id := strings.TrimSpace(returnedID); id != "" && id != customerSfID {
		customerSfID = id
		res.CustomerSfID, state.SfID = id, id
	}

	customerID, err := s.EnsureAccount(ctx, customerSfID)
	if err != nil {
		s.recordIngestFailed(ctx, state, err)
		return res, err
	}
	partnerIDs := make([]string, 0, len(partners))
	seen := map[string]bool{}
	for _, p := range partners {
		pid := salesforceID18(p.ID)
		if pid == "" || seen[pid] {
			continue
		}
		if strings.EqualFold(pid, customerSfID) {
			slog.WarnContext(ctx, "salesforce: account lists itself as its own partner, ignoring", "accountSfId", customerSfID)
			continue
		}
		seen[pid] = true
		accountID, err := s.EnsureAccount(ctx, pid)
		if err != nil {
			// EnsureAccount's missing-parent NotFoundError already names the
			// partner and starts with the prefix the retry job matches, so it
			// is kept as it is; anything else says which partner failed.
			var nf *apierror.NotFoundError
			if !errors.As(err, &nf) {
				err = fmt.Errorf("partner %s of %s: %w", pid, customerSfID, err)
			}
			s.recordIngestFailed(ctx, state, err)
			return res, err
		}
		partnerIDs = append(partnerIDs, accountID)
		res.PartnerSfIDs = append(res.PartnerSfIDs, pid)
	}

	if isResyncApply(ctx) {
		// The admin resync only adds links; a stored link Salesforce no
		// longer lists is left for the nightly sweep or the next event.
		res.Added, err = s.partners.Partners.AddPartners(ctx, customerSfID, customerID, partnerIDs, state)
	} else {
		res.Added, res.Removed, err = s.partners.Partners.ReplacePartners(ctx, customerSfID, customerID, partnerIDs, state)
	}
	if err != nil {
		s.recordIngestFailed(ctx, state, err)
		return res, err
	}
	slog.InfoContext(ctx, "salesforce: account partners refreshed",
		"accountSfId", customerSfID, "partners", len(partnerIDs), "rowsAdded", res.Added, "rowsRemoved", res.Removed)
	return res, nil
}

// PartnerReingester re-runs a FAILED partner refresh; the delayed-retry job
// registers it under domain.SalesforceIngestEntityAccountPartners.
type PartnerReingester interface {
	RetryPartnerRefresh(ctx context.Context, customerSfID string) error
}

// RetryPartnerRefresh re-runs a FAILED partner refresh for the delayed-retry
// job (registered under domain.SalesforceIngestEntityAccountPartners).
func (s *salesforceEventService) RetryPartnerRefresh(ctx context.Context, customerSfID string) error {
	_, err := s.RefreshPartners(ctx, customerSfID)
	return err
}

// refreshPartnersAfterAccountEvent is the Account event's partner refresh.
// It runs after every successful Account CREATED/UPDATED/RESTORED, including
// one the duplicate guard skipped: a partner change does not move the
// account's LastModifiedDate, so a replayed Account event is as good a
// moment to pick it up as a new one. Its error is returned, so Service Bus
// redelivers the event; the replay's account write is then skipped by the
// guard and only the refresh runs again.
//
// An account that is still not in CSM after the upsert (a customer with no
// name, which upsertAccount acknowledges without writing) has no partner
// links to hold, so the refresh is skipped rather than made to fail.
func (s *salesforceEventService) refreshPartnersAfterAccountEvent(ctx context.Context, sfID string) error {
	if !s.partners.enabled() {
		return nil
	}
	lookup := s.support.Accounts
	if lookup == nil && s.repo != nil {
		lookup = s.repo
	}
	if lookup == nil {
		return errors.New("salesforce: account lookup is not configured")
	}
	id, err := lookup.LookupAccountIDBySfID(ctx, sfID)
	if err != nil {
		return err
	}
	if id == nil {
		id, err = lookup.LookupAccountIDBySfID(ctx, salesforceID18(sfID))
		if err != nil {
			return err
		}
	}
	if id == nil {
		slog.InfoContext(ctx, "salesforce: account not in CSM after its event, not refreshing partners", "accountSfId", sfID)
		return nil
	}
	_, err = s.RefreshPartners(ctx, sfID)
	return err
}

// refreshPartnersForMembership is the membership ingest's partner refresh:
// when the membership's contact belongs to a different account than the
// project (a partner contact on a customer's project — the ServiceNow
// script's condition), the project account's partner set is refreshed.
//
// It is best-effort. The membership is the event's subject and its outcome
// must not depend on the partner read; and a failed refresh cannot be left
// to redelivery, because the membership guard would skip the replay before
// reaching here. A failure is logged and recorded FAILED in the ledger
// (RefreshPartners does that), where the retry job and the dashboard see it.
func (s *salesforceEventService) refreshPartnersForMembership(ctx context.Context, membershipSfID string, pc salesentity.ProjectContact) {
	if !s.partners.enabled() || pc.Contact == nil || pc.Subscription == nil {
		return
	}
	contactAccount := salesforceID18(derefString(pc.Contact.CustomerID))
	projectAccount := salesforceID18(derefString(pc.Subscription.CustomerID))
	if contactAccount == "" || projectAccount == "" || strings.EqualFold(contactAccount, projectAccount) {
		return
	}
	if _, err := s.RefreshPartners(ctx, projectAccount); err != nil {
		slog.WarnContext(ctx, "salesforce: partner refresh after a partner-contact membership failed",
			"membershipSfId", membershipSfID, "accountSfId", projectAccount, "contactAccountSfId", contactAccount, "err", err)
	}
}

// PartnerRefreshService backs POST /salesforce/accounts/{sfId}/refresh-partners.
type PartnerRefreshService interface {
	Refresh(ctx context.Context, customerSfID string) (domain.SalesforcePartnerRefreshResult, error)
}

type partnerRefreshService struct {
	refresher PartnerRefresher
	access    AccessService
}

// NewPartnerRefreshService constructs the internal endpoint's service. Every
// call is restricted to internal callers (an Unrestricted AccessScope), as for
// onboarding steps: it makes this service read Salesforce and write accounts.
func NewPartnerRefreshService(refresher PartnerRefresher, access AccessService) PartnerRefreshService {
	return &partnerRefreshService{refresher: refresher, access: access}
}

// Refresh implements PartnerRefreshService.
func (p *partnerRefreshService) Refresh(ctx context.Context, customerSfID string) (domain.SalesforcePartnerRefreshResult, error) {
	scope, err := p.access.ResolveScope(ctx)
	if err != nil {
		return domain.SalesforcePartnerRefreshResult{}, err
	}
	if !scope.Unrestricted {
		return domain.SalesforcePartnerRefreshResult{}, &apierror.ForbiddenError{Msg: "partner refresh is only available to internal services"}
	}
	if strings.TrimSpace(customerSfID) == "" {
		return domain.SalesforcePartnerRefreshResult{}, &apierror.ValidationError{Msg: "sfId is required"}
	}
	return p.refresher.RefreshPartners(ctx, customerSfID)
}
