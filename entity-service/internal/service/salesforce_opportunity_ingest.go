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
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// The VARCHAR limits of sf_opportunity / sf_opportunity_product (migration
// 0080). Salesforce allows longer values for some of these fields; a longer
// value is cut and logged rather than failing the whole event.
const (
	maxSfOpportunityNameChars        = 200
	maxSfOpportunityStageChars       = 40
	maxSfOpportunityEulaVersionChars = 160
	maxSfLineItemSfIDChars           = 40
	maxSfLineItemNameChars           = 200
	maxSfLineItemProductNameChars    = 100
	maxSfLineItemProductCodeChars    = 70
	maxSfLineItemProductDescChars    = 250
	maxSfLineItemProductFamilyChars  = 100
	maxSfLineItemProductSfIDChars    = 40
	maxSfLineItemShortChars          = 40 // product_unit, classification, eng_product_code, environment
)

// defaultEulaVersionDecimal is what the ServiceNow script stores in
// eula_version_decimal when the opportunity has no EULA version.
const defaultEulaVersionDecimal = "3.4"

var eulaVersionDecimalPattern = regexp.MustCompile(`\d+\.\d+`)

// SalesEntityOpportunityClient fetches one Salesforce Opportunity, with its
// line items embedded, from REST sales/sales-entity-service.
type SalesEntityOpportunityClient interface {
	GetOpportunity(ctx context.Context, id string) (salesentity.Opportunity, error)
}

// OpportunityIngest bundles the dependencies of the Opportunity branch of
// POST /salesforce/events. It is optional: a service without it acknowledges
// Opportunity envelopes and does nothing, which is how
// CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED=false is realised in
// routes.go.
type OpportunityIngest struct {
	Opportunities repository.SalesforceOpportunityRepository
	SalesEntity   SalesEntityOpportunityClient
}

func (o *OpportunityIngest) enabled() bool {
	return o != nil && o.Opportunities != nil && o.SalesEntity != nil
}

// errOpportunityIngestDisabled is what an opportunity re-run returns when the
// service was built without OpportunityIngest; the retry job only registers
// the retrier when the flag is on, so seeing it means a wiring mistake.
var errOpportunityIngestDisabled = errors.New("salesforce: opportunity ingest is disabled")

// OpportunityReingester re-runs the Opportunity ingest for one Salesforce
// Opportunity id as if an UPDATED event had arrived. The delayed-retry job
// registers it under domain.SalesforceIngestEntityOpportunity.
type OpportunityReingester interface {
	RetryOpportunityIngest(ctx context.Context, opportunitySfID string) error
}

// WithOpportunityIngest turns on the Opportunity branch of a service built by
// one of the NewSalesforceEventService constructors and returns it. It is a
// separate step, not another constructor, so it composes with the membership
// variant without multiplying constructors.
func WithOpportunityIngest(svc SalesforceEventService, ingest OpportunityIngest) SalesforceEventService {
	if s, ok := svc.(*salesforceEventService); ok {
		s.opportunity = &ingest
	}
	return svc
}

// RetryOpportunityIngest implements OpportunityReingester.
func (s *salesforceEventService) RetryOpportunityIngest(ctx context.Context, opportunitySfID string) error {
	if !s.opportunity.enabled() {
		return errOpportunityIngestDisabled
	}
	return s.ingestOpportunity(ctx, opportunitySfID, domain.SalesforceEventUpdated, true)
}

// handleOpportunityEvent is the Opportunity branch of HandleEvent.
func (s *salesforceEventService) handleOpportunityEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if !s.opportunity.enabled() {
		slog.InfoContext(ctx, "salesforce: opportunity ingest disabled, ignoring opportunity event",
			"eventType", req.EventType, "referenceId", req.ReferenceID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: opportunity event", "eventType", req.EventType, "referenceId", req.ReferenceID)

	switch req.EventType {
	case domain.SalesforceEventCreated, domain.SalesforceEventUpdated, domain.SalesforceEventRestored:
		return s.ingestOpportunity(ctx, req.ReferenceID, req.EventType, true)
	case domain.SalesforceEventDeleted:
		return s.deleteOpportunity(ctx, req.ReferenceID)
	case domain.SalesforceEventUndefined:
		return &apierror.ValidationError{Msg: "eventType UNDEFINED is not supported"}
	default:
		return &apierror.ValidationError{Msg: "eventType must be CREATED, UPDATED, DELETED, RESTORED, or UNDEFINED"}
	}
}

// ingestOpportunity fetches one Opportunity from sales-entity-service and
// writes it, with its embedded line items, to sf_opportunity and
// sf_opportunity_product. It is idempotent: every row is resolved by its
// Salesforce id, and a replay whose LastModifiedDate is not newer than the
// ledger's is skipped. Sales Entity does not return lastModifiedDate for
// opportunities yet, so until it does the guard is skipped (with a warning)
// and the upsert simply runs again. guard=false turns the duplicate guard
// off, for ensureOpportunity's write of a row known to be missing.
func (s *salesforceEventService) ingestOpportunity(ctx context.Context, sfID, eventType string, guard bool) error {
	if s.support.States == nil {
		return errors.New("salesforce: salesforce_ingest_state ledger is not configured")
	}
	opp, err := s.opportunity.SalesEntity.GetOpportunity(ctx, sfID)
	if err != nil {
		return err
	}
	// Sales Entity may answer with the 18-character form of a 15-character
	// referenceId; the stored sf_id is whatever Salesforce returns.
	if strings.TrimSpace(opp.ID) != "" {
		sfID = strings.TrimSpace(opp.ID)
	}

	eventModifiedOn, ok := parseSalesforceLastModified(opp.LastModifiedDate)
	if !ok {
		eventModifiedOn = time.Now().UTC()
	}
	if guard {
		var skip bool
		skip, eventModifiedOn, err = shouldSkipIngest(ctx, s.support.States, domain.SalesforceIngestEntityOpportunity, sfID, eventType, opp.LastModifiedDate)
		if err != nil {
			return err
		}
		if skip {
			return nil
		}
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityOpportunity,
		SfID:            sfID,
		EventModifiedOn: eventModifiedOn,
		EventType:       eventType,
		Status:          domain.SalesforceIngestSucceeded,
	}

	row := mapSalesEntityOpportunity(ctx, opp)
	row.SfID = sfID
	if customerID := strings.TrimSpace(opp.CustomerID); customerID != "" {
		accountID, err := s.EnsureAccount(ctx, customerID)
		if err != nil {
			// EnsureAccount's NotFoundError already carries the
			// "account not found for sfId" prefix the retry job matches.
			s.recordOpportunityFailed(ctx, state, err)
			return err
		}
		row.AccountID = &accountID
	} else {
		slog.WarnContext(ctx, "salesforce: opportunity has no customerId, writing it without an account", "opportunitySfId", sfID)
	}

	res, err := s.opportunity.Opportunities.UpsertFromSalesforce(ctx, row, state)
	if err != nil {
		s.recordOpportunityFailed(ctx, state, err)
		return err
	}
	slog.InfoContext(ctx, "salesforce: opportunity ingested",
		"opportunitySfId", sfID, "opportunityId", res.OpportunityID, "created", res.Created,
		"lineItemsWritten", res.LineItemsWritten, "lineItemsDeleted", res.LineItemsDeleted,
		"eulaVersionSent", opp.EulaVersion.Present)
	return nil
}

// deleteOpportunity is DELETED: a hard delete by sf_id, as ServiceNow plus
// csm-sync-service do today. The foreign keys cascade the line items and the
// project links and null out the invoices. There is nothing to fetch — the
// record is gone from Salesforce — so the ledger version is the current time,
// or the recorded version when that is later (deletedEventVersion).
//
// The ingest stores the 18-character Id Sales Entity returns, so a
// 15-character referenceId is widened to that form first: the delete, the
// advisory lock and the ledger row are then all keyed on the same Id the
// ingest used, instead of the delete matching no row and being acknowledged.
func (s *salesforceEventService) deleteOpportunity(ctx context.Context, sfID string) error {
	sfID = salesforceID18(sfID)
	if s.opportunity.SalesEntity == nil {
		return errDeleteUnconfirmable
	}
	_, fetchErr := s.opportunity.SalesEntity.GetOpportunity(ctx, sfID)
	if gone, err := confirmDeletedUpstream(ctx, string(domain.SalesforceIngestEntityOpportunity), sfID, fetchErr); err != nil || !gone {
		return err
	}
	modifiedOn, err := s.deletedEventVersion(ctx, domain.SalesforceIngestEntityOpportunity, sfID)
	if err != nil {
		return err
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityOpportunity,
		SfID:            sfID,
		EventModifiedOn: modifiedOn,
		EventType:       domain.SalesforceEventDeleted,
		Status:          domain.SalesforceIngestSucceeded,
	}
	n, err := s.opportunity.Opportunities.DeleteBySfID(ctx, sfID, state)
	if err != nil {
		s.recordOpportunityFailed(ctx, state, err)
		return err
	}
	if n == 0 {
		slog.InfoContext(ctx, "salesforce: DELETED opportunity was never ingested, nothing to delete", "opportunitySfId", sfID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: opportunity deleted", "opportunitySfId", sfID, "rows", n)
	return nil
}

// salesforceID18 returns the 18-character, case-insensitive form of a
// 15-character Salesforce record Id: the three extra characters encode which
// of the 15 are upper case, five per character, as Salesforce computes them.
// Anything that is not a 15-character alphanumeric Id is returned trimmed and
// otherwise unchanged.
func salesforceID18(id string) string {
	id = strings.TrimSpace(id)
	if len(id) != 15 {
		return id
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	suffix := make([]byte, 3)
	for chunk := 0; chunk < 3; chunk++ {
		bits := 0
		for i := 0; i < 5; i++ {
			c := id[chunk*5+i]
			switch {
			case c >= 'A' && c <= 'Z':
				bits |= 1 << i
			case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			default:
				return id
			}
		}
		suffix[chunk] = alphabet[bits]
	}
	return id + string(suffix)
}

// recordOpportunityFailed writes a FAILED ledger row best-effort, outside the
// rolled-back transaction, so the failure is visible and the delayed-retry
// job can re-run a missing-account failure. The original error is what
// HandleEvent returns regardless.
func (s *salesforceEventService) recordOpportunityFailed(ctx context.Context, state domain.UpsertSalesforceIngestStateRequest, cause error) {
	if s.support.States == nil {
		return
	}
	msg := truncateOnboardingStepError(cause.Error())
	state.Status = domain.SalesforceIngestFailed
	state.LastError = &msg
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if _, err := s.support.States.Upsert(recordCtx, state); err != nil {
		slog.ErrorContext(ctx, "salesforce: recording FAILED opportunity ingest state also failed", "opportunitySfId", state.SfID, "err", err)
	}
}

// mapSalesEntityOpportunity maps the Sales Entity record onto the columns
// the ingest owns. AccountID is resolved by the caller. type and
// engagementCode are deliberately not mapped: sf_opportunity.type and
// engagement_code stay ServiceNow-derived until decided otherwise.
func mapSalesEntityOpportunity(ctx context.Context, opp salesentity.Opportunity) domain.SalesforceOpportunityUpsert {
	sfID := strings.TrimSpace(opp.ID)
	keepEula, eulaVersion, eulaDecimal := deriveEulaVersion(opp.EulaVersion)
	row := domain.SalesforceOpportunityUpsert{
		SfID:               sfID,
		Name:               truncateSfColumn(ctx, "sf_opportunity.name", sfID, optionalPtr(opp.Name), maxSfOpportunityNameChars),
		Stage:              truncateSfColumn(ctx, "sf_opportunity.stage", sfID, optionalPtr(opp.StageName), maxSfOpportunityStageChars),
		IsWon:              opp.IsWon,
		CloseDate:          parseSalesforceDate(ctx, "supportAccountEndDateRollUp", sfID, opp.SupportAccountEndDateRollUp),
		KeepExistingEula:   keepEula,
		EulaVersion:        truncateSfColumn(ctx, "sf_opportunity.eula_version", sfID, eulaVersion, maxSfOpportunityEulaVersionChars),
		EulaVersionDecimal: eulaDecimal,
	}
	if !opp.EulaVersion.Present {
		slog.WarnContext(ctx, "salesforce: sales entity sent no eulaVersion for the opportunity, keeping the stored EULA version", "opportunitySfId", sfID)
	}

	row.LineItems = make([]domain.SalesforceOpportunityLineItemUpsert, 0, len(opp.SubscriptionLineItems))
	seen := map[string]int{}
	for _, li := range opp.SubscriptionLineItems {
		id := strings.TrimSpace(derefString(li.ID))
		if id == "" {
			slog.WarnContext(ctx, "salesforce: opportunity line item has no id, skipping it", "opportunitySfId", sfID)
			continue
		}
		if utf8.RuneCountInString(id) > maxSfLineItemSfIDChars {
			slog.WarnContext(ctx, "salesforce: opportunity line item id is longer than the column, skipping it", "opportunitySfId", sfID, "lineItemSfId", id)
			continue
		}
		mapped := mapSalesEntityLineItem(ctx, id, li)
		// A repeated id would be written twice under one lock; the last wins.
		if i, dup := seen[id]; dup {
			row.LineItems[i] = mapped
			continue
		}
		seen[id] = len(row.LineItems)
		row.LineItems = append(row.LineItems, mapped)
	}
	return row
}

func mapSalesEntityLineItem(ctx context.Context, id string, li salesentity.SubscriptionLineItem) domain.SalesforceOpportunityLineItemUpsert {
	col := func(name string, v *string, max int) *string {
		return truncateSfColumn(ctx, "sf_opportunity_product."+name, id, optionalPtr(v), max)
	}
	out := domain.SalesforceOpportunityLineItemUpsert{
		LineItemSfID:     id,
		Name:             col("name", li.Name, maxSfLineItemNameChars),
		Quantity:         li.Quantity,
		ServiceStartDate: parseSalesforceDate(ctx, "serviceStartDate", id, li.ServiceStartDate),
		ServiceEndDate:   parseSalesforceDate(ctx, "serviceEndDate", id, li.ServiceEndDate),
		Classification:   col("classification", li.Classification, maxSfLineItemShortChars),
		Environment:      col("environment", li.Environment, maxSfLineItemShortChars),
		TotalPrice:       li.TotalPrice,
	}
	if p := li.Product; p != nil {
		out.ProductName = col("product_name", p.Name, maxSfLineItemProductNameChars)
		out.ProductCode = col("product_code", p.ProductCode, maxSfLineItemProductCodeChars)
		out.ProductDescription = col("product_description", p.Description, maxSfLineItemProductDescChars)
		out.ProductFamily = col("product_family", p.Family, maxSfLineItemProductFamilyChars)
		out.ProductUnit = col("product_unit", p.ProductUnit, maxSfLineItemShortChars)
		out.EngProductCode = col("eng_product_code", p.EngProductCode, maxSfLineItemShortChars)
		out.ProductSfID = col("product_sf_id", p.ID, maxSfLineItemProductSfIDChars)
	}
	return out
}

// deriveEulaVersion reproduces the ServiceNow script's EULA derivation:
// eula_version_decimal is the first \d+\.\d+ in eulaVersion, and 3.4 when
// eulaVersion is empty. A version with no decimal in it stores a NULL
// decimal (the script left the column untouched there, a quirk that would
// keep a decimal disagreeing with the new version). When Sales Entity does
// not send the key at all, both columns are kept.
func deriveEulaVersion(v salesentity.OptionalString) (keep bool, version, decimal *string) {
	if !v.Present {
		return true, nil, nil
	}
	version = optionalPtr(v.Value)
	if version == nil {
		d := defaultEulaVersionDecimal
		return false, nil, &d
	}
	if m := eulaVersionDecimalPattern.FindString(*version); m != "" {
		return false, version, &m
	}
	return false, version, nil
}

// parseSalesforceDate reads a Salesforce date ("2027-03-31", or a datetime
// whose first ten characters are the date) as a DATE value. Empty is NULL;
// an unparseable value is NULL and logged.
func parseSalesforceDate(ctx context.Context, field, sfID string, v *string) *time.Time {
	s := strings.TrimSpace(derefString(v))
	if s == "" {
		return nil
	}
	if len(s) > 10 {
		s = s[:10]
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		slog.WarnContext(ctx, "salesforce: unparseable date, writing NULL", "field", field, "sfId", sfID, "value", derefString(v))
		return nil
	}
	return &t
}

// truncateSfColumn cuts v to max characters (Postgres VARCHAR counts
// characters, so runes here) and logs when it had to.
func truncateSfColumn(ctx context.Context, column, sfID string, v *string, max int) *string {
	if v == nil || utf8.RuneCountInString(*v) <= max {
		return v
	}
	slog.WarnContext(ctx, "salesforce: value longer than its column, truncating", "column", column, "sfId", sfID,
		"length", utf8.RuneCountInString(*v), "max", max)
	cut := string([]rune(*v)[:max])
	return &cut
}
