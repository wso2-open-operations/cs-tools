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

package sweep

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// resolvedInvoice is resolveDueInvoice's result — the "immediate" due
// invoice for a project (the one with the soonest due date, among the
// eligible ones), plus its opportunity's EULA version, which is what
// closure.DecideInvoice/InvoiceSuspendDate need alongside it.
type resolvedInvoice struct {
	ID string
	// Number is what the notice shows as "Invoice Id": the invoice's name
	// (ServiceNow u_name, e.g. US20268838), as legacy shows it. Falls back to
	// ID, the internal record ID, only when the invoice has no name.
	Number             string
	Opportunity        string
	InvoiceDate        time.Time
	DueDate            time.Time
	EULAVersionDecimal float64
	SfID               string
}

// excludedInvoiceClassifications mirrors the legacy fetchDueInvoicesByProject
// exclusion list verbatim (PP/CO/TAM — Cloud support type invoices). Real
// sample data hasn't shown any of these codes yet (CL/LS/PS observed
// instead), but that's staging test data, not confirmed representative of
// production — kept exactly as legacy specifies rather than adjusted
// against unverified staging behavior (see decide_invoice.go's sibling
// eligibility note on the same caution).
var excludedInvoiceClassifications = map[string]bool{"PP": true, "CO": true, "TAM": true}

// resolveDueInvoice finds the "immediate" due invoice for a project —
// ported from the legacy ACPInvoiceUtils.fetchDueInvoicesByProject,
// including its opportunity eligibility check (see eligibleOpportunity).
// Returns (nil, nil) when the project has no eligible due invoice — a
// legitimate, common state, not an error. The returned invoice is the first
// of resolveDueInvoices: the earliest due, which drives the cascade's timing.
func resolveDueInvoice(ctx context.Context, reader entityReader, proj project) (*resolvedInvoice, error) {
	all, err := resolveDueInvoices(ctx, reader, proj)
	if err != nil || len(all) == 0 {
		return nil, err
	}
	return &all[0], nil
}

// resolveDueInvoices returns every eligible due invoice for a project,
// earliest due first, each once. Legacy fetchDueInvoicesByProject returns
// this same list (orderBy u_invoiced_due_date); it decides the cascade's
// timing from the first and passes the whole list to the internal email,
// which shows one box per invoice.
func resolveDueInvoices(ctx context.Context, reader entityReader, proj project) ([]resolvedInvoice, error) {
	links, err := fetchAllProjectOpportunityLinks(ctx, reader, proj.ID)
	if err != nil {
		return nil, err
	}

	var all []resolvedInvoice
	seenOpp := map[string]bool{}
	seenInvoice := map[string]bool{}

	for _, link := range links {
		if link.Opportunity == nil || link.Opportunity.ID == "" || seenOpp[link.Opportunity.ID] {
			continue
		}
		seenOpp[link.Opportunity.ID] = true

		oppRaw, err := reader.GetOpportunity(ctx, link.Opportunity.ID)
		if err != nil {
			return nil, fmt.Errorf("get opportunity %s: %w", link.Opportunity.ID, err)
		}
		var opp opportunityDTO
		if err := json.Unmarshal(oppRaw, &opp); err != nil {
			return nil, fmt.Errorf("parse opportunity %s: %w", link.Opportunity.ID, err)
		}
		if !eligibleOpportunity(opp) {
			continue
		}
		eulaDecimal, err := parseEULAVersionDecimal(opp.EulaVersionDecimal)
		if err != nil {
			return nil, fmt.Errorf("parse eulaVersionDecimal for opportunity %s: %w", link.Opportunity.ID, err)
		}

		invoices, err := fetchAllInvoicesForOpportunity(ctx, reader, link.Opportunity.ID)
		if err != nil {
			return nil, err
		}

		oppName := ""
		if opp.Name != nil {
			oppName = *opp.Name
		}

		for _, inv := range invoices {
			if !eligibleInvoice(inv, proj.StartDate) {
				continue
			}
			dueDate, err := parseInvoiceDate(*inv.InvoicedDueDate)
			if err != nil {
				return nil, fmt.Errorf("parse invoicedDueDate for invoice %s: %w", inv.ID, err)
			}
			invoiceDate := time.Time{}
			if inv.InvoiceDate != nil {
				invoiceDate, err = parseInvoiceDate(*inv.InvoiceDate)
				if err != nil {
					return nil, fmt.Errorf("parse invoiceDate for invoice %s: %w", inv.ID, err)
				}
			}

			if seenInvoice[inv.ID] {
				continue
			}
			seenInvoice[inv.ID] = true
			all = append(all, resolvedInvoice{
				ID:                 inv.ID,
				Number:             invoiceNumber(inv),
				Opportunity:        oppName,
				InvoiceDate:        invoiceDate,
				DueDate:            dueDate,
				EULAVersionDecimal: eulaDecimal,
				SfID:               stringValue(inv.SfID),
			})
		}
	}

	// Stable, so invoices due on the same day keep the order they were found
	// in, as the earlier "first strictly-earlier wins" selection did.
	sort.SliceStable(all, func(i, j int) bool { return all[i].DueDate.Before(all[j].DueDate) })
	return all, nil
}

// fetchAllProjectOpportunityLinks pages through /project-opportunity-links/search
// for one project until every link is collected, following the same
// pagination pattern Run uses for /projects/search. Neither this nor
// fetchAllInvoicesForOpportunity specified a page limit before this fix
// (CodeRabbit, PR #1933) — a project or opportunity with more rows than a
// single page would silently have the rest ignored, possibly missing a
// more-overdue eligible invoice on a later page.
func fetchAllProjectOpportunityLinks(ctx context.Context, reader entityReader, projectID string) ([]projectOpportunityLinkDTO, error) {
	var all []projectOpportunityLinkDTO
	offset := 0
	for {
		reqBody, err := json.Marshal(searchProjectOpportunityLinksRequest{
			Pagination: pagination{Limit: pageSize, Offset: offset},
			ProjectID:  projectID,
		})
		if err != nil {
			return nil, fmt.Errorf("build project-opportunity-links search request: %w", err)
		}
		raw, err := reader.SearchProjectOpportunityLinks(ctx, reqBody)
		if err != nil {
			return nil, fmt.Errorf("search project-opportunity links at offset %d: %w", offset, err)
		}
		var page searchProjectOpportunityLinksResponse
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("parse project-opportunity links at offset %d: %w", offset, err)
		}
		all = append(all, page.Links...)
		// Stop once "total" rows are read even if hasMore stays true, so a
		// misbehaving upstream can't keep this loop going until the job
		// timeout (security assessment 2026-10-08).
		if len(page.Links) == 0 || !page.HasMore || (page.Total > 0 && len(all) >= page.Total) {
			break
		}
		offset += pageSize
	}
	return all, nil
}

// fetchAllInvoicesForOpportunity pages through /invoices/search for one
// opportunity until every invoice is collected — see
// fetchAllProjectOpportunityLinks's doc comment for why this matters.
func fetchAllInvoicesForOpportunity(ctx context.Context, reader entityReader, opportunityID string) ([]invoiceDTO, error) {
	var all []invoiceDTO
	offset := 0
	for {
		reqBody, err := json.Marshal(searchInvoicesRequest{
			Pagination:    pagination{Limit: pageSize, Offset: offset},
			OpportunityID: opportunityID,
		})
		if err != nil {
			return nil, fmt.Errorf("build invoices search request: %w", err)
		}
		raw, err := reader.SearchInvoices(ctx, reqBody)
		if err != nil {
			return nil, fmt.Errorf("search invoices for opportunity %s at offset %d: %w", opportunityID, offset, err)
		}
		var page searchInvoicesResponse
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("parse invoices for opportunity %s at offset %d: %w", opportunityID, offset, err)
		}
		all = append(all, page.Invoices...)
		// Stop once "total" rows are read even if hasMore stays true, so a
		// misbehaving upstream can't keep this loop going until the job
		// timeout (security assessment 2026-10-08).
		if len(page.Invoices) == 0 || !page.HasMore || (page.Total > 0 && len(all) >= page.Total) {
			break
		}
		offset += pageSize
	}
	return all, nil
}

// closedWonStage is the only opportunity stage whose invoices count —
// the exact literal legacy compares u_stage against.
const closedWonStage = "50 - Closed Won"

// eligibleOpportunity mirrors the legacy fetchDueInvoicesByProject
// eligibility check exactly: stage is "50 - Closed Won", and the text EULA
// field is non-null and not "Customer contract". A null or any other stage
// is ineligible, as in legacy's equality check. See CLAUDE.md ("Only Closed
// Won opportunities' invoices count"). Kept literal (see
// excludedInvoiceClassifications' doc comment) rather than adjusted against
// unverified staging data.
func eligibleOpportunity(opp opportunityDTO) bool {
	return opp.Stage != nil && *opp.Stage == closedWonStage &&
		opp.EulaVersion != nil && *opp.EulaVersion != "Customer contract"
}

// eligibleInvoice mirrors the legacy fetchDueInvoicesByProject invoice
// filters: unpaid, not an excluded classification, not the
// "Auto Created PS" placeholder name, and due on/after the project's start
// date (when both are known). An invoice with no due date at all can't be
// scheduled against, so it's excluded too — the legacy query implicitly
// required this by filtering/sorting on the field directly.
func eligibleInvoice(inv invoiceDTO, projectStart *time.Time) bool {
	if inv.InvoicedDueDate == nil || *inv.InvoicedDueDate == "" {
		return false
	}
	if inv.InvoicedPaidDate != nil && *inv.InvoicedPaidDate != "" {
		return false
	}
	if inv.Classification != nil && excludedInvoiceClassifications[*inv.Classification] {
		return false
	}
	if inv.Name != nil && *inv.Name == "Auto Created PS" {
		return false
	}
	if projectStart != nil {
		due, err := parseInvoiceDate(*inv.InvoicedDueDate)
		if err == nil && due.Before(*projectStart) {
			return false
		}
	}
	return true
}

// parseInvoiceDate parses an Invoice date field — date-only ("2026-09-01"),
// not the full RFC3339 timestamps project.StartDate/EndDate use.
func parseInvoiceDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// parseEULAVersionDecimal parses Opportunity.EulaVersionDecimal — a string
// on the wire (confirmed via the real API, not a float like its name might
// suggest) — into the float64 closure.DecideInvoice's math needs. nil
// (absent) parses to 0, matching usesGracePeriod's own "eulaVersion <= 0
// means unset" convention rather than erroring on an absent-but-otherwise-
// eligible opportunity.
func parseEULAVersionDecimal(s *string) (float64, error) {
	if s == nil || *s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(*s, 64)
}

// invoiceNumber returns the value the notice shows as "Invoice Id". Legacy
// shows the invoice's name (ServiceNow u_name, e.g. US20268838). The API's
// "id" is an internal record ID nobody recognises, and the Salesforce ID is
// used only for the "Open in Salesforce" link. Falls back to the record ID
// only when there's no name, so the field is never blank.
func invoiceNumber(inv invoiceDTO) string {
	if inv.Name != nil && strings.TrimSpace(*inv.Name) != "" {
		return strings.TrimSpace(*inv.Name)
	}
	return inv.ID
}
