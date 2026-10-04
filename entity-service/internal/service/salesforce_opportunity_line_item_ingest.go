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
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// SalesEntityOpportunityLineItemClient fetches one Salesforce
// OpportunityLineItem from REST sales/sales-entity-service.
type SalesEntityOpportunityLineItemClient interface {
	GetOpportunityLineItem(ctx context.Context, id string) (salesentity.SubscriptionLineItem, error)
}

// OpportunityLineItemIngest bundles the dependencies of the standalone
// OpportunityLineItem branch of POST /salesforce/events. routes.go attaches
// it with the Opportunity branch, under
// CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED; a service without it
// acknowledges line item envelopes and does nothing.
type OpportunityLineItemIngest struct {
	LineItems     repository.SalesforceOpportunityLineItemRepository
	Opportunities repository.SalesforceOpportunityLookup
	SalesEntity   SalesEntityOpportunityLineItemClient
}

func (o *OpportunityLineItemIngest) enabled() bool {
	return o != nil && o.LineItems != nil && o.Opportunities != nil && o.SalesEntity != nil
}

var errOpportunityLineItemIngestDisabled = errors.New("salesforce: opportunity line item ingest is disabled")

// OpportunityLineItemReingester re-runs the standalone line item ingest as
// if an UPDATED event had arrived; the delayed-retry job registers it under
// domain.SalesforceIngestEntityOpportunityLineItem.
type OpportunityLineItemReingester interface {
	RetryOpportunityLineItemIngest(ctx context.Context, lineItemSfID string) error
}

// WithOpportunityLineItemIngest turns on the standalone OpportunityLineItem
// branch, like WithOpportunityIngest.
func WithOpportunityLineItemIngest(svc SalesforceEventService, ingest OpportunityLineItemIngest) SalesforceEventService {
	if s, ok := svc.(*salesforceEventService); ok {
		s.lineItems = &ingest
	}
	return svc
}

// RetryOpportunityLineItemIngest implements OpportunityLineItemReingester.
func (s *salesforceEventService) RetryOpportunityLineItemIngest(ctx context.Context, lineItemSfID string) error {
	if !s.lineItems.enabled() {
		return errOpportunityLineItemIngestDisabled
	}
	return s.ingestOpportunityLineItem(ctx, lineItemSfID, domain.SalesforceEventUpdated)
}

// handleOpportunityLineItemEvent is the OpportunityLineItem branch of HandleEvent.
func (s *salesforceEventService) handleOpportunityLineItemEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if !s.lineItems.enabled() {
		slog.InfoContext(ctx, "salesforce: opportunity line item ingest disabled, ignoring line item event",
			"eventType", req.EventType, "referenceId", req.ReferenceID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: opportunity line item event", "eventType", req.EventType, "referenceId", req.ReferenceID)
	switch req.EventType {
	case domain.SalesforceEventCreated, domain.SalesforceEventUpdated, domain.SalesforceEventRestored:
		return s.ingestOpportunityLineItem(ctx, req.ReferenceID, req.EventType)
	case domain.SalesforceEventDeleted:
		return s.deleteOpportunityLineItem(ctx, req.ReferenceID)
	case domain.SalesforceEventUndefined:
		return &apierror.ValidationError{Msg: "eventType UNDEFINED is not supported"}
	default:
		return &apierror.ValidationError{Msg: "eventType must be CREATED, UPDATED, DELETED, RESTORED, or UNDEFINED"}
	}
}

// ingestOpportunityLineItem fetches one line item from
// POST /opportunity-line-items/search and upserts it by line_item_sf_id
// under its opportunity, which is ingested first when missing. The column
// mapping is the derived path's (mapSalesEntityLineItem), so both paths
// write the same values.
func (s *salesforceEventService) ingestOpportunityLineItem(ctx context.Context, sfID, eventType string) error {
	if s.support.States == nil {
		return errors.New("salesforce: salesforce_ingest_state ledger is not configured")
	}
	li, err := s.lineItems.SalesEntity.GetOpportunityLineItem(ctx, sfID)
	if err != nil {
		return err
	}
	if id := strings.TrimSpace(derefString(li.ID)); id != "" {
		sfID = id
	}
	if utf8.RuneCountInString(sfID) > maxSfLineItemSfIDChars {
		slog.WarnContext(ctx, "salesforce: opportunity line item id is longer than the column, not writing it", "lineItemSfId", sfID)
		return nil
	}
	oppSfID := strings.TrimSpace(derefString(li.OpportunityID))
	if oppSfID == "" {
		// Salesforce requires OpportunityId; a record without one is a Sales
		// Entity fault worth retrying, not a line item to store unparented.
		return &apierror.ServiceUnavailableError{Msg: "sales/sales-entity-service opportunity line item " + sfID + " has no opportunityId"}
	}

	skip, eventModifiedOn, err := shouldSkipIngest(ctx, s.support.States, domain.SalesforceIngestEntityOpportunityLineItem, sfID, eventType, li.LastModifiedDate)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityOpportunityLineItem,
		SfID:            sfID,
		EventModifiedOn: eventModifiedOn,
		EventType:       eventType,
		Status:          domain.SalesforceIngestSucceeded,
	}

	oppSfID = salesforceID18(oppSfID)
	oppID, err := s.ensureOpportunity(ctx, s.lineItems.Opportunities, oppSfID)
	if err != nil {
		s.recordIngestFailed(ctx, state, err)
		return err
	}
	row := mapSalesEntityLineItem(ctx, sfID, li)
	created, err := s.lineItems.LineItems.UpsertFromSalesforce(ctx, oppSfID, oppID, row, state)
	if err != nil {
		s.recordIngestFailed(ctx, state, err)
		return err
	}
	slog.InfoContext(ctx, "salesforce: opportunity line item ingested",
		"lineItemSfId", sfID, "opportunitySfId", oppSfID, "opportunityId", oppID, "created", created)
	return nil
}

// deleteOpportunityLineItem is DELETED: a hard delete by line_item_sf_id
// (widened to 18 characters), as ServiceNow plus csm-sync-service do today;
// a never-ingested line item is acknowledged.
func (s *salesforceEventService) deleteOpportunityLineItem(ctx context.Context, sfID string) error {
	sfID = salesforceID18(sfID)
	if s.lineItems.SalesEntity == nil {
		return errDeleteUnconfirmable
	}
	_, fetchErr := s.lineItems.SalesEntity.GetOpportunityLineItem(ctx, sfID)
	if gone, err := confirmDeletedUpstream(ctx, string(domain.SalesforceIngestEntityOpportunityLineItem), sfID, fetchErr); err != nil || !gone {
		return err
	}
	modifiedOn, err := s.deletedEventVersion(ctx, domain.SalesforceIngestEntityOpportunityLineItem, sfID)
	if err != nil {
		return err
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityOpportunityLineItem,
		SfID:            sfID,
		EventModifiedOn: modifiedOn,
		EventType:       domain.SalesforceEventDeleted,
		Status:          domain.SalesforceIngestSucceeded,
	}
	n, err := s.lineItems.LineItems.DeleteByLineItemSfID(ctx, sfID, state)
	if err != nil {
		s.recordIngestFailed(ctx, state, err)
		return err
	}
	if n == 0 {
		slog.InfoContext(ctx, "salesforce: DELETED opportunity line item was never ingested, nothing to delete", "lineItemSfId", sfID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: opportunity line item deleted", "lineItemSfId", sfID, "rows", n)
	return nil
}
