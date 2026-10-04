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

// maxSfOpportunityLinkNumberChars is sf_opportunity_link.number's width
// (migration 0080).
const maxSfOpportunityLinkNumberChars = 40

// SalesEntityLinkedOpportunityClient fetches one Salesforce
// Linked_Opportunity__c from REST sales/sales-entity-service.
type SalesEntityLinkedOpportunityClient interface {
	GetLinkedOpportunity(ctx context.Context, id string) (salesentity.LinkedOpportunity, error)
}

// LinkedOpportunityIngest bundles the dependencies of the
// Linked_Opportunity__c branch of POST /salesforce/events. It belongs to the
// Opportunity family (CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED)
// and needs the Opportunity ingest too, which it runs inline for a link
// whose opportunity is not in CSM yet.
type LinkedOpportunityIngest struct {
	Links       repository.SalesforceOpportunityLinkRepository
	SalesEntity SalesEntityLinkedOpportunityClient
}

// linkedOpportunityEnabled reports whether the link branch can run: its own
// dependencies and the Opportunity ingest it calls inline.
func (s *salesforceEventService) linkedOpportunityEnabled() bool {
	l := s.linkedOpportunity
	return l != nil && l.Links != nil && l.SalesEntity != nil && s.opportunity.enabled()
}

// errLinkedOpportunityIngestDisabled is what a link re-run returns when the
// service was built without the link branch; seeing it means a wiring
// mistake.
var errLinkedOpportunityIngestDisabled = errors.New("salesforce: linked opportunity ingest is disabled")

// LinkedOpportunityReingester re-runs the link ingest for one Salesforce
// Linked_Opportunity__c id as if an UPDATED event had arrived. The
// delayed-retry job registers it under
// domain.SalesforceIngestEntityLinkedOpportunity.
type LinkedOpportunityReingester interface {
	RetryLinkedOpportunityIngest(ctx context.Context, linkSfID string) error
}

// WithLinkedOpportunityIngest turns on the Linked_Opportunity__c branch of a
// service and returns it. It takes effect only together with
// WithOpportunityIngest.
func WithLinkedOpportunityIngest(svc SalesforceEventService, ingest LinkedOpportunityIngest) SalesforceEventService {
	if s, ok := svc.(*salesforceEventService); ok {
		s.linkedOpportunity = &ingest
	}
	return svc
}

// RetryLinkedOpportunityIngest implements LinkedOpportunityReingester.
func (s *salesforceEventService) RetryLinkedOpportunityIngest(ctx context.Context, linkSfID string) error {
	if !s.linkedOpportunityEnabled() {
		return errLinkedOpportunityIngestDisabled
	}
	return s.ingestLinkedOpportunity(ctx, linkSfID, domain.SalesforceEventUpdated)
}

// handleLinkedOpportunityEvent is the Linked_Opportunity__c branch of HandleEvent.
func (s *salesforceEventService) handleLinkedOpportunityEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if !s.linkedOpportunityEnabled() {
		slog.InfoContext(ctx, "salesforce: opportunity ingest disabled, ignoring linked opportunity event",
			"eventType", req.EventType, "entity", req.Entity, "referenceId", req.ReferenceID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: linked opportunity event", "eventType", req.EventType, "entity", req.Entity, "referenceId", req.ReferenceID)

	switch req.EventType {
	case domain.SalesforceEventCreated, domain.SalesforceEventUpdated, domain.SalesforceEventRestored:
		return s.ingestLinkedOpportunity(ctx, req.ReferenceID, req.EventType)
	case domain.SalesforceEventDeleted:
		return s.deleteLinkedOpportunity(ctx, req.ReferenceID)
	case domain.SalesforceEventUndefined:
		return &apierror.ValidationError{Msg: "eventType UNDEFINED is not supported"}
	default:
		return &apierror.ValidationError{Msg: "eventType must be CREATED, UPDATED, DELETED, RESTORED, or UNDEFINED"}
	}
}

// ingestLinkedOpportunity fetches one Linked_Opportunity__c and writes it to
// sf_opportunity_link by link_sf_id, after resolving both parents by their
// Salesforce ids: the opportunity (ensureOpportunity: ingested inline when
// missing, the ServiceNow script's _initOpportunity) and the project (EnsureProject: an
// inline ingest when project inserts are allowed, else a NotFoundError the
// delayed-retry job re-runs).
func (s *salesforceEventService) ingestLinkedOpportunity(ctx context.Context, sfID, eventType string) error {
	if s.support.States == nil {
		return errors.New("salesforce: salesforce_ingest_state ledger is not configured")
	}
	link, err := s.linkedOpportunity.SalesEntity.GetLinkedOpportunity(ctx, sfID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(link.ID) != "" {
		sfID = strings.TrimSpace(link.ID)
	}

	skip, eventModifiedOn, err := shouldSkipIngest(ctx, s.support.States, domain.SalesforceIngestEntityLinkedOpportunity, sfID, eventType, link.LastModifiedDate)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityLinkedOpportunity,
		SfID:            sfID,
		EventModifiedOn: eventModifiedOn,
		EventType:       eventType,
		Status:          domain.SalesforceIngestSucceeded,
	}

	oppSfID := strings.TrimSpace(derefString(link.OpportunityID))
	projectSfID := strings.TrimSpace(derefString(link.ProjectID))
	if oppSfID == "" || projectSfID == "" {
		// Both are master-detail in Salesforce, so this is a data error no
		// redelivery can fix: recorded FAILED and acknowledged.
		cause := fmt.Errorf("salesforce linked opportunity has no opportunityId (%q) or projectId (%q)", oppSfID, projectSfID)
		slog.WarnContext(ctx, "salesforce: linked opportunity is missing a parent id, not written", "linkSfId", sfID, "err", cause)
		s.recordLinkedOpportunityFailed(ctx, state, cause)
		return nil
	}

	oppID, err := s.ensureOpportunity(ctx, s.linkedOpportunity.Links, oppSfID)
	if err != nil {
		s.recordLinkedOpportunityFailed(ctx, state, err)
		return err
	}
	projectID, err := s.EnsureProject(ctx, projectSfID)
	if err != nil {
		s.recordLinkedOpportunityFailed(ctx, state, err)
		// A keyless parent project (D5) is fixed in Salesforce, not by
		// redelivery: recorded FAILED above and acknowledged.
		return ackRefusedProject(err)
	}

	row := domain.SalesforceOpportunityLinkUpsert{
		LinkSfID:      sfID,
		Number:        truncateSfColumn(ctx, "sf_opportunity_link.number", sfID, optionalPtr(link.Name), maxSfOpportunityLinkNumberChars),
		OpportunityID: oppID,
		ProjectID:     projectID,
	}
	created, err := s.linkedOpportunity.Links.UpsertFromSalesforce(ctx, row, state)
	if err != nil {
		s.recordLinkedOpportunityFailed(ctx, state, err)
		return err
	}
	slog.InfoContext(ctx, "salesforce: linked opportunity ingested",
		"linkSfId", sfID, "number", derefString(row.Number), "opportunityId", oppID, "projectId", projectID, "created", created)
	return nil
}

// deleteLinkedOpportunity is DELETED: a hard delete by link_sf_id, as
// ServiceNow plus csm-sync-service do today, with a DELETED ledger row
// stamped by deletedEventVersion. The
// 15-character referenceId is widened to the 18-character form the ingest
// stores first. A link never ingested is acknowledged.
func (s *salesforceEventService) deleteLinkedOpportunity(ctx context.Context, sfID string) error {
	sfID = salesforceID18(sfID)
	if s.linkedOpportunity.SalesEntity == nil {
		return errDeleteUnconfirmable
	}
	_, fetchErr := s.linkedOpportunity.SalesEntity.GetLinkedOpportunity(ctx, sfID)
	if gone, err := confirmDeletedUpstream(ctx, string(domain.SalesforceIngestEntityLinkedOpportunity), sfID, fetchErr); err != nil || !gone {
		return err
	}
	modifiedOn, err := s.deletedEventVersion(ctx, domain.SalesforceIngestEntityLinkedOpportunity, sfID)
	if err != nil {
		return err
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityLinkedOpportunity,
		SfID:            sfID,
		EventModifiedOn: modifiedOn,
		EventType:       domain.SalesforceEventDeleted,
		Status:          domain.SalesforceIngestSucceeded,
	}
	n, err := s.linkedOpportunity.Links.DeleteByLinkSfID(ctx, sfID, state)
	if err != nil {
		s.recordLinkedOpportunityFailed(ctx, state, err)
		return err
	}
	if n == 0 {
		slog.InfoContext(ctx, "salesforce: DELETED linked opportunity was never ingested, nothing to delete", "linkSfId", sfID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: linked opportunity deleted", "linkSfId", sfID, "rows", n)
	return nil
}

// recordLinkedOpportunityFailed writes a FAILED ledger row best-effort,
// outside the rolled-back transaction.
func (s *salesforceEventService) recordLinkedOpportunityFailed(ctx context.Context, state domain.UpsertSalesforceIngestStateRequest, cause error) {
	if s.support.States == nil {
		return
	}
	msg := truncateOnboardingStepError(cause.Error())
	state.Status = domain.SalesforceIngestFailed
	state.LastError = &msg
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if _, err := s.support.States.Upsert(recordCtx, state); err != nil {
		slog.ErrorContext(ctx, "salesforce: recording FAILED linked opportunity ingest state also failed", "linkSfId", state.SfID, "err", err)
	}
}
