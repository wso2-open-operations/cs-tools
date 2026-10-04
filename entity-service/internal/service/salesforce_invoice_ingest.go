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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// sf_invoice's bounded text columns (migration 0081) are all VARCHAR(40).
const maxSfInvoiceTextChars = 40

// SalesEntityInvoiceClient fetches one Salesforce Invoice__c from REST
// sales/sales-entity-service.
type SalesEntityInvoiceClient interface {
	GetInvoice(ctx context.Context, id string) (salesentity.Invoice, error)
}

// InvoiceIngest bundles the dependencies of the Invoice__c branch of
// POST /salesforce/events. routes.go attaches it together with the
// Opportunity branch, under CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED;
// a service without it acknowledges invoice envelopes and does nothing.
type InvoiceIngest struct {
	Invoices      repository.SalesforceInvoiceRepository
	Opportunities repository.SalesforceOpportunityLookup
	SalesEntity   SalesEntityInvoiceClient
}

func (i *InvoiceIngest) enabled() bool {
	return i != nil && i.Invoices != nil && i.Opportunities != nil && i.SalesEntity != nil
}

var errInvoiceIngestDisabled = errors.New("salesforce: invoice ingest is disabled")

// InvoiceReingester re-runs the invoice ingest for one Invoice__c id as if an
// UPDATED event had arrived; the delayed-retry job registers it under
// domain.SalesforceIngestEntityInvoice.
type InvoiceReingester interface {
	RetryInvoiceIngest(ctx context.Context, invoiceSfID string) error
}

// WithInvoiceIngest turns on the Invoice__c branch, like WithOpportunityIngest.
func WithInvoiceIngest(svc SalesforceEventService, ingest InvoiceIngest) SalesforceEventService {
	if s, ok := svc.(*salesforceEventService); ok {
		s.invoices = &ingest
	}
	return svc
}

// RetryInvoiceIngest implements InvoiceReingester.
func (s *salesforceEventService) RetryInvoiceIngest(ctx context.Context, invoiceSfID string) error {
	if !s.invoices.enabled() {
		return errInvoiceIngestDisabled
	}
	return s.ingestInvoice(ctx, invoiceSfID, domain.SalesforceEventUpdated)
}

// handleInvoiceEvent is the Invoice__c branch of HandleEvent.
func (s *salesforceEventService) handleInvoiceEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if !s.invoices.enabled() {
		slog.InfoContext(ctx, "salesforce: invoice ingest disabled, ignoring invoice event",
			"eventType", req.EventType, "referenceId", req.ReferenceID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: invoice event", "eventType", req.EventType, "referenceId", req.ReferenceID)
	switch req.EventType {
	case domain.SalesforceEventCreated, domain.SalesforceEventUpdated, domain.SalesforceEventRestored:
		return s.ingestInvoice(ctx, req.ReferenceID, req.EventType)
	case domain.SalesforceEventDeleted:
		return s.deleteInvoice(ctx, req.ReferenceID)
	case domain.SalesforceEventUndefined:
		return &apierror.ValidationError{Msg: "eventType UNDEFINED is not supported"}
	default:
		return &apierror.ValidationError{Msg: "eventType must be CREATED, UPDATED, DELETED, RESTORED, or UNDEFINED"}
	}
}

// ingestInvoice fetches one Invoice__c from POST /invoices/search and writes
// it to sf_invoice by sf_id. Invoices are written only from their own events,
// never from the list embedded in an Opportunity: that list lacks the invoice
// date, the parent id and the original due date, and leaves out the
// auto-created invoices, so replacing from it would null columns and delete
// rows (plan §7). Auto-created invoices ("Auto Created ..." names) are
// written like any other, which is what a single Invoice__c event did in
// ServiceNow.
func (s *salesforceEventService) ingestInvoice(ctx context.Context, sfID, eventType string) error {
	if s.support.States == nil {
		return errors.New("salesforce: salesforce_ingest_state ledger is not configured")
	}
	inv, err := s.invoices.SalesEntity.GetInvoice(ctx, sfID)
	if err != nil {
		return err
	}
	if id := strings.TrimSpace(derefString(inv.ID)); id != "" {
		sfID = id
	}

	skip, eventModifiedOn, err := shouldSkipIngest(ctx, s.support.States, domain.SalesforceIngestEntityInvoice, sfID, eventType, inv.LastModifiedDate)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityInvoice,
		SfID:            sfID,
		EventModifiedOn: eventModifiedOn,
		EventType:       eventType,
		Status:          domain.SalesforceIngestSucceeded,
	}

	row := mapSalesEntityInvoice(ctx, sfID, inv)
	if oppSfID := strings.TrimSpace(derefString(inv.OpportunityID)); oppSfID != "" {
		oppID, err := s.ensureOpportunity(ctx, s.invoices.Opportunities, oppSfID)
		if err != nil {
			s.recordIngestFailed(ctx, state, err)
			return err
		}
		row.OpportunityID = &oppID
	} else {
		slog.WarnContext(ctx, "salesforce: invoice has no opportunityId, writing it without an opportunity", "invoiceSfId", sfID)
	}

	created, err := s.invoices.Invoices.UpsertFromSalesforce(ctx, row, state)
	if err != nil {
		s.recordIngestFailed(ctx, state, err)
		return err
	}
	slog.InfoContext(ctx, "salesforce: invoice ingested", "invoiceSfId", sfID, "created", created, "opportunityId", derefString(row.OpportunityID))
	return nil
}

// deleteInvoice is DELETED: a hard delete by sf_id, as ServiceNow plus
// csm-sync-service do today. The id is widened to 18 characters first, the
// form the ingest stored; a never-ingested invoice is acknowledged.
func (s *salesforceEventService) deleteInvoice(ctx context.Context, sfID string) error {
	sfID = salesforceID18(sfID)
	if s.invoices.SalesEntity == nil {
		return errDeleteUnconfirmable
	}
	_, fetchErr := s.invoices.SalesEntity.GetInvoice(ctx, sfID)
	if gone, err := confirmDeletedUpstream(ctx, string(domain.SalesforceIngestEntityInvoice), sfID, fetchErr); err != nil || !gone {
		return err
	}
	modifiedOn, err := s.deletedEventVersion(ctx, domain.SalesforceIngestEntityInvoice, sfID)
	if err != nil {
		return err
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityInvoice,
		SfID:            sfID,
		EventModifiedOn: modifiedOn,
		EventType:       domain.SalesforceEventDeleted,
		Status:          domain.SalesforceIngestSucceeded,
	}
	n, err := s.invoices.Invoices.DeleteBySfID(ctx, sfID, state)
	if err != nil {
		s.recordIngestFailed(ctx, state, err)
		return err
	}
	if n == 0 {
		slog.InfoContext(ctx, "salesforce: DELETED invoice was never ingested, nothing to delete", "invoiceSfId", sfID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: invoice deleted", "invoiceSfId", sfID, "rows", n)
	return nil
}

// mapSalesEntityInvoice maps the Sales Entity record onto sf_invoice's
// columns; OpportunityID is resolved by the caller. The original due date
// falls back to the current one when Salesforce has none, as the ServiceNow
// script did. The three text columns are VARCHAR(40); a longer value is cut
// and logged.
func mapSalesEntityInvoice(ctx context.Context, sfID string, inv salesentity.Invoice) domain.SalesforceInvoiceUpsert {
	text := func(column string, v *string) *string {
		return truncateSfColumn(ctx, "sf_invoice."+column, sfID, optionalPtr(v), maxSfInvoiceTextChars)
	}
	originalDue := inv.OriginalInvoiceDueDate
	if optionalPtr(originalDue) == nil {
		originalDue = inv.DueDate
	}
	return domain.SalesforceInvoiceUpsert{
		SfID:                   sfID,
		Name:                   text("name", inv.Name),
		Description:            text("description", inv.Description),
		Classification:         text("classification", inv.Classification),
		InvoicedAmount:         inv.Amount,
		InvoiceDate:            parseSalesforceDate(ctx, "invoiceDate", sfID, inv.InvoiceDate),
		InvoicedDueDate:        parseSalesforceDate(ctx, "dueDate", sfID, inv.DueDate),
		OriginalInvoiceDueDate: parseSalesforceDate(ctx, "originalInvoiceDueDate", sfID, originalDue),
		InvoicedPaidDate:       parseSalesforceDate(ctx, "paidDate", sfID, inv.PaidDate),
		ServiceStartDate:       parseSalesforceDate(ctx, "serviceStartDate", sfID, inv.ServiceStartDate),
		ServiceEndDate:         parseSalesforceDate(ctx, "serviceEndDate", sfID, inv.ServiceEndDate),
	}
}

// deletedEventVersion is the ledger version a DELETED event records: the
// current time, or the recorded version when that is later. A deleted record
// cannot be fetched, so there is no LastModifiedDate to record, and the
// ledger only moves forward; Salesforce's clock can run ahead of this one, so
// "now" alone could be older than the last UPDATED version and the DELETED
// row would then be dropped. Were that to happen, a later RESTORED (which
// carries the record's pre-delete LastModifiedDate) would be skipped by the
// guard and the row never restored. softDeleteAccount applies the same rule.
func (s *salesforceEventService) deletedEventVersion(ctx context.Context, entity, sfID string) (time.Time, error) {
	modifiedOn := time.Now().UTC()
	if s.support.States == nil {
		return modifiedOn, nil
	}
	st, err := s.support.States.Get(ctx, entity, sfID)
	if err != nil {
		return time.Time{}, err
	}
	if st != nil && st.EventModifiedOn.After(modifiedOn) {
		modifiedOn = st.EventModifiedOn
	}
	return modifiedOn, nil
}

// recordIngestFailed writes a FAILED ledger row best-effort, outside the
// rolled-back transaction, for the opportunity child families (invoice, line
// item); the entity is the one state already names.
func (s *salesforceEventService) recordIngestFailed(ctx context.Context, state domain.UpsertSalesforceIngestStateRequest, cause error) {
	if s.support.States == nil {
		return
	}
	msg := truncateOnboardingStepError(cause.Error())
	state.Status = domain.SalesforceIngestFailed
	state.LastError = &msg
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if _, err := s.support.States.Upsert(recordCtx, state); err != nil {
		slog.ErrorContext(ctx, "salesforce: recording FAILED ingest state also failed", "entity", state.Entity, "sfId", state.SfID, "err", err)
	}
}
