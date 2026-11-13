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
// Package plgsignups lands WSO2 Cloud signups read from Moesif in the PLG
// portal. A registration event names the user's entry product; each later
// subscription event names the next product they try. One registration is
// sent per event, by company id, user id and product only.
//
// Each run reads a window of events. Ingest is idempotent, so the overlap
// between runs only refreshes what is stored, and a failed run is covered by
// the next one as long as the outage is shorter than the window.
package plgsignups

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/moesif"
)

// Defaults for the events read and where they hold the product.
var (
	DefaultEvents = []string{
		"cloud-core.org-account-management.register-organization",
		"cloud-core.billing.select-subscription-plan",
	}
	DefaultProductFields = []string{"metadata.initiating_product", "metadata.product"}
)

// successStatuses are the ingest outcomes of a registration that landed.
var successStatuses = map[string]bool{"created": true, "updated": true, "unchanged": true}

// Searcher is the subset of *moesif.Client used here.
type Searcher interface {
	SearchEvents(ctx context.Context, q moesif.Query) ([]moesif.Hit, bool, error)
}

// Entity is the subset of *Client used here.
type Entity interface {
	Ingest(ctx context.Context, regs []Registration) ([]Result, error)
	RecordFailure(ctx context.Context, f Failure) error
}

// Options configure a run.
type Options struct {
	Events        []string
	ProductFields []string
	Window        time.Duration
	BatchSize     int
	MaxPages      int
}

// event is one signup, reduced to what the portal needs.
type event struct {
	id, action, userID, companyID, product string
	at                                     time.Time
}

// Sync returns a registry.Task.Handler that lands the signups of the last
// opts.Window. Rejected events are recorded, not returned as errors; an
// unreachable Moesif or entity-service fails the run so the engine retries it.
func Sync(src Searcher, ent Entity, opts Options, now func() time.Time) func(ctx context.Context) error {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}
	return func(ctx context.Context) error {
		to := now().UTC()
		from := to.Add(-opts.Window)
		hits, more, err := src.SearchEvents(ctx, moesif.Query{
			From:     from,
			To:       to,
			Filter:   moesif.TermsFilter("action_name.raw", opts.Events),
			Fields:   append([]string{"request.time", "action_name", "user_id", "company_id"}, opts.ProductFields...),
			MaxPages: opts.MaxPages,
		})
		if err != nil {
			return fmt.Errorf("plg signups: read Moesif events: %w", err)
		}

		var (
			regs     []Registration
			owners   []event
			failures []Failure
		)
		read := 0
		for _, h := range hits {
			e := toEvent(h, opts.ProductFields)
			// Moesif applies the window; this guards against one it did not.
			if !e.at.IsZero() && (e.at.Before(from) || !e.at.Before(to)) {
				continue
			}
			read++
			if reason := invalid(e); reason != "" {
				failures = append(failures, failureFor(e, reason))
				continue
			}
			regs = append(regs, Registration{
				CreatedOn:         e.at.Format("2006-01-02T15:04:05.000Z07:00"),
				CompanyID:         e.companyID,
				UserID:            e.userID,
				InitiatingProduct: strings.ToLower(e.product),
			})
			owners = append(owners, e)
		}

		accepted := 0
		for start := 0; start < len(regs); start += opts.BatchSize {
			end := min(start+opts.BatchSize, len(regs))
			results, err := ent.Ingest(ctx, regs[start:end])
			if err != nil {
				return fmt.Errorf("plg signups: ingest: %w", err)
			}
			if len(results) != end-start {
				return fmt.Errorf("plg signups: ingest answered %d results for %d registrations", len(results), end-start)
			}
			for i, r := range results {
				if successStatuses[strings.ToLower(r.Status)] {
					accepted++
					continue
				}
				failures = append(failures, failureFor(owners[start+i],
					fmt.Sprintf("entity-service rejected it (%s): %s", r.Status, r.Message)))
			}
		}

		// Every failure is attempted: one refused record must not hide the rest.
		var recordErrs []error
		for _, f := range failures {
			if err := ent.RecordFailure(ctx, f); err != nil {
				recordErrs = append(recordErrs, err)
			}
		}
		slog.InfoContext(ctx, "plg signups: synced", "read", read, "accepted", accepted, "failed", len(failures),
			"unrecorded", len(recordErrs))
		if len(recordErrs) > 0 {
			return fmt.Errorf("plg signups: record %d of %d failures: %w", len(recordErrs), len(failures), errors.Join(recordErrs...))
		}

		if more {
			return errors.New("plg signups: the window holds more events than one run reads; " +
				"raise PLG_MOESIF_SYNC_MAX_PAGES or shorten PLG_MOESIF_SYNC_WINDOW_MINUTES")
		}
		return nil
	}
}

func toEvent(h moesif.Hit, productFields []string) event {
	e := event{
		id:        h.ID,
		action:    moesif.Text(h.Source, "action_name"),
		userID:    moesif.Text(h.Source, "user_id"),
		companyID: moesif.Text(h.Source, "company_id"),
	}
	e.at, _ = moesif.ParseTime(moesif.Text(h.Source, "request.time"))
	for _, f := range productFields {
		if e.product = moesif.Text(h.Source, f); e.product != "" {
			break
		}
	}
	return e
}

func invalid(e event) string {
	switch {
	case e.companyID == "":
		return "the event has no company id"
	case e.product == "":
		return "the event names no product"
	case e.at.IsZero():
		return "the event has no readable time"
	}
	return ""
}

// failureFor records an event by its ids only, never personal data.
func failureFor(e event, reason string) Failure {
	payload, _ := json.Marshal(map[string]string{
		"companyId":   e.companyID,
		"userId":      e.userID,
		"actionName":  e.action,
		"product":     e.product,
		"requestTime": formatTime(e.at),
	})
	return Failure{EventID: e.id, EventType: e.action, ReceivedAt: formatTime(e.at), Payload: payload, Failure: reason}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}
