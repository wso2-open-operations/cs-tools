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
	"log/slog"
	"sort"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/suspensionstate"
)

// Screening reasons, one per candidate: the first that applies.
const (
	reasonDate    = "date"    // end date missing or within the horizon
	reasonState   = "state"   // a recorded non-open event on either track
	reasonInvoice = "invoice" // owns an unpaid invoice due within the horizon
	reasonOpen    = "failOpen"
)

// screeningHorizonDays is how far ahead a date makes a project a candidate:
// 90 days (the widest notice window) plus one day of margin for the
// time-of-day and zone arithmetic of closure.daysBetween, which rounds up.
//
// Why the invoice test can use the due date: the live path suspends an
// invoice at its suspend date (closure.InvoiceSuspendDate), which is the due
// date, or, when the grace period applies and the credit period (due date
// minus invoice date) is at most 60 days, invoice date + 60 days, which is
// then not earlier than the due date (due <= date + 60). So a suspend date
// within the horizon implies a due date within it, and no notice window can
// be reached by an invoice whose due date is beyond it.
const screeningHorizonDays = 91

// maxScreeningPages caps the unfiltered invoice listing, which has no filter
// to narrow it: 1,000 pages is 50,000 invoices. Reaching it is a fail-open,
// not a quiet stop.
const maxScreeningPages = 1000

// screenResult is the outcome of screen.
type screenResult struct {
	// candidates maps a project id to the reason it goes through the live
	// path. A project not in it is screened out.
	candidates map[string]string
}

// screen decides which of the listed projects the live path could act on,
// from CSM's copy of the Salesforce data, so the rest are skipped without a
// re-read or a Salesforce call. It must return a superset: a project is a
// candidate when in doubt, and any read problem makes every project a
// candidate (reasonOpen), never fewer. It only decides who to look at; what
// is sent or suspended is still decided from the live read.
//
// The invoice reads use only entity-service's existing search routes, which
// have no due-date or paid filter: all invoices are listed (50 a page) and
// filtered here, then the links of the opportunities that own a relevant
// invoice are read by opportunity. An invoice with no opportunity cannot be
// reached by the live path (it goes project, link, opportunity, invoices of
// that opportunity), so it is ignored. The opportunity's stage, EULA version
// and the invoice's classification are not tested here: the live path does
// that, and a superset keeps the two copies of the rules from drifting.
func screen(ctx context.Context, reader entityReader, projects []project, now time.Time, snap *copySnapshot) screenResult {
	if snap == nil {
		snap = &copySnapshot{}
	}
	horizon := now.AddDate(0, 0, screeningHorizonDays)
	res := screenResult{candidates: map[string]string{}}

	var far []project
	for _, p := range projects {
		switch {
		case p.EndDate == nil || !p.EndDate.After(horizon):
			res.candidates[p.ID] = reasonDate
		case hasRecordedEvent(p):
			res.candidates[p.ID] = reasonState
		default:
			far = append(far, p)
		}
	}
	if len(far) == 0 {
		return res
	}

	byProject, err := projectsWithDueInvoices(ctx, reader, snap, horizon)
	if err != nil {
		slog.WarnContext(ctx, "acp: screening data could not be read completely: every project goes through the live path this run (fail open)", "err", err)
		for _, p := range projects {
			res.candidates[p.ID] = reasonOpen
		}
		return res
	}
	for _, p := range far {
		if byProject[p.ID] {
			res.candidates[p.ID] = reasonInvoice
		}
	}
	return res
}

// hasRecordedEvent reports whether the project's suspensionProcessState
// records anything but "open" for either track. The live path's renewal reset
// (subscriptionRenewed, invoiceRenewed) acts on such a state even far from the
// end date. An unreadable state counts as recorded.
func hasRecordedEvent(p project) bool {
	return suspensionstate.HasRecordedEvent(p.SuspensionProcessState)
}

// projectsWithDueInvoices returns the ids of the projects that own, through a
// link, an unpaid invoice with a due date on or before horizon.
func projectsWithDueInvoices(ctx context.Context, reader entityReader, snap *copySnapshot, horizon time.Time) (map[string]bool, error) {
	opps, err := opportunitiesWithDueInvoices(ctx, reader, snap, horizon)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(opps))
	for id := range opps {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := map[string]bool{}
	for _, oppID := range ids {
		offset, pages := 0, 0
		for {
			reqBody, err := json.Marshal(searchProjectOpportunityLinksRequest{
				Pagination:    pagination{Limit: pageSize, Offset: offset},
				OpportunityID: oppID,
			})
			if err != nil {
				return nil, fmt.Errorf("build links request: %w", err)
			}
			raw, err := reader.SearchProjectOpportunityLinks(ctx, reqBody)
			if err != nil {
				return nil, fmt.Errorf("search links of an opportunity at offset %d: %w", offset, err)
			}
			var page searchProjectOpportunityLinksResponse
			if err := json.Unmarshal(raw, &page); err != nil {
				return nil, fmt.Errorf("parse links at offset %d: %w", offset, err)
			}
			for _, l := range page.Links {
				if l.Project != nil && l.Project.ID != "" {
					out[l.Project.ID] = true
				}
			}
			offset += len(page.Links)
			if len(page.Links) == 0 || !page.HasMore || (page.Total > 0 && offset >= page.Total) {
				break
			}
			if pages++; pages >= maxSearchPages {
				return nil, fmt.Errorf("links search still reports more pages after %d pages", maxSearchPages)
			}
		}
	}
	return out, nil
}

// copySnapshot is one run's read of CSM's copy of every invoice, shared by
// screening and the repair so the listing is made once. It is read lazily and
// a failure is kept, so a failed listing is not retried within the run.
type copySnapshot struct {
	loaded   bool
	invoices []invoiceDTO
	err      error
	bySfID   map[string]invoiceDTO
}

// all returns every invoice of the copy. The listing has no filter, so it is
// paged 50 at a time up to maxScreeningPages; a listing that cannot be shown
// complete (a page cap reached, or fewer distinct invoices than the reported
// total, since offset paging over a changing table can skip a row) is an
// error.
func (c *copySnapshot) all(ctx context.Context, reader entityReader) ([]invoiceDTO, error) {
	if c.loaded {
		return c.invoices, c.err
	}
	c.loaded = true
	seen := map[string]bool{}
	offset, pages, total := 0, 0, 0
	for {
		reqBody, err := json.Marshal(searchInvoicesRequest{Pagination: pagination{Limit: pageSize, Offset: offset}})
		if err != nil {
			c.err = fmt.Errorf("build invoices request: %w", err)
			return nil, c.err
		}
		raw, err := reader.SearchInvoices(ctx, reqBody)
		if err != nil {
			c.err = fmt.Errorf("search invoices at offset %d: %w", offset, err)
			return nil, c.err
		}
		var page searchInvoicesResponse
		if err := json.Unmarshal(raw, &page); err != nil {
			c.err = fmt.Errorf("parse invoices at offset %d: %w", offset, err)
			return nil, c.err
		}
		for _, inv := range page.Invoices {
			seen[inv.ID] = true
		}
		c.invoices = append(c.invoices, page.Invoices...)
		total = page.Total
		offset += len(page.Invoices)
		if len(page.Invoices) == 0 || !page.HasMore {
			break
		}
		if pages++; pages >= maxScreeningPages {
			c.err = fmt.Errorf("invoice listing still reports more pages after %d pages", maxScreeningPages)
			return nil, c.err
		}
	}
	if len(seen) < total {
		c.err = fmt.Errorf("invoice listing returned %d distinct invoices of a reported %d", len(seen), total)
		return nil, c.err
	}
	return c.invoices, nil
}

// bySalesforceID indexes the copy's invoices by their Salesforce id.
func (c *copySnapshot) bySalesforceID(ctx context.Context, reader entityReader) (map[string]invoiceDTO, error) {
	all, err := c.all(ctx, reader)
	if err != nil {
		return nil, err
	}
	if c.bySfID == nil {
		c.bySfID = map[string]invoiceDTO{}
		for _, inv := range all {
			if id := stringValue(inv.SfID); id != "" {
				c.bySfID[id] = inv
			}
		}
	}
	return c.bySfID, nil
}

// opportunitiesWithDueInvoices returns the ids of the opportunities that own
// an unpaid invoice due on or before horizon. An invoice whose due date does
// not parse is an error (the caller fails open). An invoice with no due date
// is ignored, as the live path does (eligibleInvoice).
func opportunitiesWithDueInvoices(ctx context.Context, reader entityReader, snap *copySnapshot, horizon time.Time) (map[string]bool, error) {
	all, err := snap.all(ctx, reader)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, inv := range all {
		if inv.InvoicedDueDate == nil || *inv.InvoicedDueDate == "" {
			continue
		}
		if inv.InvoicedPaidDate != nil && *inv.InvoicedPaidDate != "" {
			continue
		}
		due, err := parseInvoiceDate(*inv.InvoicedDueDate)
		if err != nil {
			return nil, fmt.Errorf("an invoice has an unparseable due date: %w", err)
		}
		if !due.After(horizon) && inv.Opportunity != nil && inv.Opportunity.ID != "" {
			out[inv.Opportunity.ID] = true
		}
	}
	return out, nil
}
