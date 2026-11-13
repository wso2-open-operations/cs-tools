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
	"log/slog"
	"strings"
	"time"
)

// EventPoster posts a synthetic Salesforce change event to entity-service's
// existing ingest (POST /salesforce/events), which re-fetches the record from
// the Salesforce service and upserts CSM's copy. Satisfied by *entity.Client.
type EventPoster interface {
	PostSalesforceEvent(ctx context.Context, body []byte) error
}

// Repair configures the repair of CSM's copy: in the pass ACP already makes
// over Salesforce for a project, the values it relies on are compared with
// the copy ACP has, and each record that drifted gets one event. It never
// changes what the run decides and never fails it.
type Repair struct {
	Poster EventPoster
	// MaxEvents bounds the events one run posts (or, in a dry run, would
	// post), so a mass drift cannot flood entity-service.
	MaxEvents int
	// DryRun posts nothing and only counts what it would.
	DryRun bool
}

// The entities an event can name, as entity-service's ingest knows them.
const (
	entityProject = "Project__c"
	entityAccount = "Account"
	entityInvoice = "Invoice__c"
)

type repairCounts struct{ compared, drifted, posted, failed, overCap int }

// repairRun is the state of one run's repair.
type repairRun struct {
	cfg      Repair
	snap     *copySnapshot
	counts   map[string]*repairCounts
	sent     map[string]bool
	attempts int
	warned   bool
}

func newRepairRun(cfg *Repair, snap *copySnapshot) *repairRun {
	if cfg == nil || cfg.Poster == nil || cfg.MaxEvents <= 0 {
		return nil
	}
	return &repairRun{cfg: *cfg, snap: snap, counts: map[string]*repairCounts{}, sent: map[string]bool{}}
}

func (r *repairRun) c(entity string) *repairCounts {
	if r.counts[entity] == nil {
		r.counts[entity] = &repairCounts{}
	}
	return r.counts[entity]
}

// drift records one compared record and, when it drifted, posts its event
// once per run (deduplicated), within the run's bound.
func (r *repairRun) drift(ctx context.Context, entity, sfID string, drifted bool) {
	c := r.c(entity)
	c.compared++
	sfID = strings.TrimSpace(sfID)
	if !drifted || sfID == "" {
		return
	}
	key := entity + "|" + sfID
	if r.sent[key] {
		return
	}
	r.sent[key] = true
	c.drifted++
	if r.attempts >= r.cfg.MaxEvents {
		c.overCap++
		return
	}
	r.attempts++
	if r.cfg.DryRun {
		return
	}
	body, err := json.Marshal(map[string]string{"eventType": "UPDATED", "entity": entity, "referenceId": sfID})
	if err == nil {
		err = r.cfg.Poster.PostSalesforceEvent(ctx, body)
	}
	if err != nil {
		c.failed++
		if !r.warned {
			r.warned = true
			slog.WarnContext(ctx, "acp: a repair event could not be posted; ignored, the run is unaffected (counts in the repair summary)", "err", err)
		}
		return
	}
	c.posted++
}

// dateOf is the date part of a date or datetime, "" when absent.
func dateOf(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 10 {
		s = s[:10]
	}
	return s
}

func timeDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.DateOnly)
}

// project compares the project's start and end dates and its account's
// partner flag between CSM's copy (csm, the fresh re-read) and the live read
// (live, which only a Salesforce details source marks: accountStatusRead).
func (r *repairRun) project(ctx context.Context, csm, live project) {
	if r == nil || !live.accountStatusRead {
		return
	}
	if sf := stringValue(csm.SfID); sf != "" {
		r.drift(ctx, entityProject, sf,
			timeDate(csm.StartDate) != timeDate(live.StartDate) || timeDate(csm.EndDate) != timeDate(live.EndDate))
	}
	if csm.Account != nil && csm.Account.IsPartner != nil && live.Account != nil && live.Account.IsPartner != nil {
		r.drift(ctx, entityAccount, live.Account.ID, *csm.Account.IsPartner != *live.Account.IsPartner)
	}
}

// invoices compares the Salesforce invoices the decision read (kept by the
// live reader) with CSM's copy by Salesforce id: due date, paid date and
// invoice date, or an invoice the copy lacks.
func (r *repairRun) invoices(ctx context.Context, csmReader entityReader, reader entityReader) {
	if r == nil {
		return
	}
	sr, ok := reader.(*salesforceReader)
	if !ok || len(sr.invoices) == 0 {
		return
	}
	index, err := r.snap.bySalesforceID(ctx, csmReader)
	if err != nil {
		if !r.warned {
			r.warned = true
			slog.WarnContext(ctx, "acp: CSM's invoices could not be read for the repair; invoices are not compared this run", "err", err)
		}
		return
	}
	for _, inv := range sr.invoices {
		id := stringValue(inv.ID)
		if id == "" {
			continue
		}
		copyInv, found := index[id]
		drifted := !found ||
			dateOf(stringValue(inv.DueDate)) != dateOf(stringValue(copyInv.InvoicedDueDate)) ||
			dateOf(stringValue(inv.PaidDate)) != dateOf(stringValue(copyInv.InvoicedPaidDate)) ||
			dateOf(stringValue(inv.InvoiceDate)) != dateOf(stringValue(copyInv.InvoiceDate))
		r.drift(ctx, entityInvoice, id, drifted)
	}
}

// log writes the run's one repair INFO: counts by entity, never names or ids.
func (r *repairRun) log(ctx context.Context) {
	if r == nil {
		return
	}
	attrs := []any{"dryRun", r.cfg.DryRun, "maxEvents", r.cfg.MaxEvents}
	for _, e := range []struct{ key, entity string }{{"project", entityProject}, {"account", entityAccount}, {"invoice", entityInvoice}} {
		c := r.c(e.entity)
		attrs = append(attrs,
			e.key+"Compared", c.compared, e.key+"Drifted", c.drifted,
			e.key+"Posted", c.posted, e.key+"Failed", c.failed, e.key+"OverCap", c.overCap)
	}
	slog.InfoContext(ctx, "acp: repair of CSM's copy", attrs...)
}
