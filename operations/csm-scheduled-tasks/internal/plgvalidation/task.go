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

// Package plgvalidation fills a product's registration playbook from its Moesif
// activity. Each run lists the pairings whose activity window is open (or
// closed within the grace period), reads their events, classifies the owner's
// email domain, and writes the values through entity-service, which never
// overwrites an engineer's value.
package plgvalidation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/moesif"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/plgactivity"
)

// Platforms maps a portal product to its Moesif metadata.platform.
var Platforms = map[string]string{"AGENT_PLATFORM": "Agent Manager"}

// Task codes and option codes of the registration playbook.
const (
	codeInitiate = "INITIATE_PLAYBOOK"
	codeClass    = "REGISTERED_EMAIL_CLASSIFICATION"
	codeSession  = "INITIAL_SESSION_DURATION_MINUTES"
	codeLevel    = "INITIAL_ACTIVITY_LEVEL"
	codeOutcome  = "ELIGIBILITY_ASSESSMENT_OUTCOME"
	codeReason   = "ELIGIBILITY_ASSESSMENT_REASON"

	OutcomeEligible = "ELIGIBLE"
	OutcomeMonitor  = "MONITOR"
	OutcomeExcluded = "EXCLUDED"

	LevelHigh     = "HIGH"
	LevelModerate = "MODERATE"
	LevelLow      = "LOW"
	LevelNone     = "NONE"
)

const failuresNamed = 10

// Searcher is the subset of *moesif.Client used here.
type Searcher interface {
	SearchEvents(ctx context.Context, q moesif.Query) ([]moesif.Hit, bool, error)
}

// Entity is the subset of *Client used here.
type Entity interface {
	Queue(ctx context.Context, product string, registeredAfter time.Time, limit int) ([]QueueItem, error)
	Apply(ctx context.Context, orgPlatformID string, req ApplyRequest) (ApplyResult, error)
}

// Options configure a run.
type Options struct {
	Products   []string      // portal product codes, each in Platforms
	Window     time.Duration // activity counted after signup
	Thresholds Thresholds
	Grace      time.Duration // how long a closed window stays in the queue
	DryRun     bool          // derive and log, write nothing
	BatchSize  int           // company ids per Moesif query
	QueueLimit int
	MaxPages   int
	// Directory and Classifier classify the owner's email domain; either nil
	// leaves the classification to engineers.
	Directory  Directory
	Classifier DomainClassifier
}

// Run returns a registry.Task.Handler. Moesif or the queue failing fails the
// run; one pairing failing does not stop the others but fails the run at the end.
func Run(src Searcher, ent Entity, opts Options, now func() time.Time) func(ctx context.Context) error {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 50
	}
	if opts.QueueLimit <= 0 {
		opts.QueueLimit = 1000
	}
	if opts.Thresholds.Eligible <= 0 {
		opts.Thresholds.Eligible = 10
	}
	if opts.Thresholds.Moderate <= 0 {
		opts.Thresholds.Moderate = 5
	}
	return func(ctx context.Context) error {
		// An empty window would read every pairing as inactive and write that.
		if opts.Window <= 0 {
			return errors.New("plg registration validation: the activity window must be positive")
		}
		var failed []string
		for _, product := range opts.Products {
			f, err := runProduct(ctx, src, ent, opts, product, now().UTC())
			if err != nil {
				return err
			}
			failed = append(failed, f...)
		}
		if len(failed) > 0 {
			named := failed[:min(len(failed), failuresNamed)]
			return fmt.Errorf("plg registration validation: %d pairings failed: %s", len(failed), strings.Join(named, ", "))
		}
		return nil
	}
}

func runProduct(ctx context.Context, src Searcher, ent Entity, opts Options, product string, now time.Time) ([]string, error) {
	items, err := ent.Queue(ctx, product, now.Add(-opts.Window-opts.Grace), opts.QueueLimit)
	if err != nil {
		return nil, fmt.Errorf("plg registration validation: %s queue: %w", product, err)
	}
	events, err := readEvents(ctx, src, opts, Platforms[product], items, now)
	if err != nil {
		return nil, fmt.Errorf("plg registration validation: %s: read Moesif events: %w", product, err)
	}

	var failed []string
	counts := map[string]int{}
	for _, it := range items {
		end := minTime(it.RegisteredOn.Add(opts.Window), now)
		act := plgactivity.Derive(events[it.CompanyID], it.RegisteredOn, end)
		class := it.EmailClassification
		var newClass *string
		if deref(class) == "" || deref(class) == classUnknown {
			if c, ok := classify(ctx, opts, it); ok {
				class, newClass = &c, &c
				counts["classified"]++
			}
		}
		values, outcome, abandon := Decide(act, class, opts.Thresholds)
		if newClass != nil {
			values = append(values, Value{Code: codeClass, TextValue: newClass})
		}
		counts[outcome]++
		if opts.DryRun {
			slog.InfoContext(ctx, "plg registration validation: dry run", "product", product,
				"orgPlatformId", it.OrgPlatformID, "events", act.EventCount, "actions", act.Actions,
				"classification", deref(class), "outcome", outcome, "abandon", abandon)
			continue
		}
		res, err := ent.Apply(ctx, it.OrgPlatformID, ApplyRequest{Summary: summary(act, it.RegisteredOn, end), Values: values, Abandon: abandon})
		var ae *apierror.Error
		if errors.As(err, &ae) && ae.StatusCode == http.StatusNotFound && strings.Contains(ae.Body, "playbook") {
			slog.WarnContext(ctx, "plg registration validation: no active registration playbook; product skipped", "product", product)
			return failed, nil
		}
		if err != nil {
			slog.ErrorContext(ctx, "plg registration validation: write failed", "orgPlatformId", it.OrgPlatformID, "err", err)
			failed = append(failed, it.OrgPlatformID)
			continue
		}
		if res.Stale {
			// The classification changed since the queue was read; the next run decides again.
			counts["stale"]++
			continue
		}
		counts["written"] += len(res.Written)
		counts["skipped"] += len(res.Skipped)
		if res.Abandoned {
			counts["abandoned"]++
		}
	}
	slog.InfoContext(ctx, "plg registration validation: done", "product", product, "dryRun", opts.DryRun,
		"pairings", len(items), "eligible", counts[OutcomeEligible], "monitor", counts[OutcomeMonitor],
		"excluded", counts[OutcomeExcluded], "abandoned", counts["abandoned"], "stale", counts["stale"], "classified", counts["classified"],
		"valuesWritten", counts["written"],
		"valuesSkipped", counts["skipped"], "failed", len(failed))
	return failed, nil
}

const classUnknown = "UNKNOWN"

// classify reads the owner's email, keeps only its domain and whether it is a
// role mailbox, and asks the AI service about the domain. A personal domain
// with a role or test mailbox is DISPOSABLE. Any failure leaves the pairing
// unclassified until the next run; logs carry the pairing id only.
func classify(ctx context.Context, opts Options, it QueueItem) (string, bool) {
	if opts.Directory == nil || opts.Classifier == nil || it.OwnerUserID == nil {
		return "", false
	}
	email, err := opts.Directory.UserEmail(ctx, *it.OwnerUserID)
	if err != nil {
		slog.WarnContext(ctx, "plg registration validation: owner email lookup failed", "orgPlatformId", it.OrgPlatformID, "err", err)
		return "", false
	}
	domain, role, ok := splitEmail(email)
	if !ok {
		slog.WarnContext(ctx, "plg registration validation: owner email is malformed", "orgPlatformId", it.OrgPlatformID)
		return "", false
	}
	class, err := opts.Classifier.Classify(ctx, domain)
	if err != nil {
		slog.WarnContext(ctx, "plg registration validation: domain classification failed", "orgPlatformId", it.OrgPlatformID, "err", err)
		return "", false
	}
	if class == "PERSONAL" && role {
		class = "DISPOSABLE"
	}
	return class, true
}

// readEvents reads the platform's events for the queued companies, grouped by company id.
func readEvents(ctx context.Context, src Searcher, opts Options, platform string, items []QueueItem, now time.Time) (map[string][]plgactivity.Event, error) {
	byCompany := map[string][]plgactivity.Event{}
	for start := 0; start < len(items); start += opts.BatchSize {
		batch := items[start:min(start+opts.BatchSize, len(items))]
		var ids []string
		from, to := batch[0].RegisteredOn, batch[0].RegisteredOn
		for _, it := range batch {
			if !slices.Contains(ids, it.CompanyID) {
				ids = append(ids, it.CompanyID)
			}
			from = minTime(from, it.RegisteredOn)
			to = maxTime(to, it.RegisteredOn.Add(opts.Window))
		}
		hits, more, err := src.SearchEvents(ctx, moesif.Query{
			From:     from,
			To:       minTime(to, now),
			Filter:   platformFilter(platform, ids),
			Fields:   []string{"company_id", "request.time", "metadata.action"},
			MaxPages: opts.MaxPages,
		})
		if err != nil {
			return nil, err
		}
		if more {
			return nil, errors.New("more events than one query reads; raise PLG_REGISTRATION_VALIDATION_MAX_PAGES")
		}
		for _, h := range hits {
			at, ok := moesif.ParseTime(moesif.Text(h.Source, "request.time"))
			company := moesif.Text(h.Source, "company_id")
			if !ok || company == "" {
				continue
			}
			byCompany[company] = append(byCompany[company], plgactivity.Event{At: at, Action: moesif.Text(h.Source, "metadata.action")})
		}
	}
	return byCompany, nil
}

func platformFilter(platform string, companyIDs []string) map[string]any {
	return map[string]any{"bool": map[string]any{"must": []map[string]any{
		{"term": map[string]any{"metadata.platform.raw": platform}},
		{"terms": map[string]any{"company_id.raw": companyIDs}},
		{"exists": map[string]any{"field": "metadata.action"}},
	}}}
}

// Thresholds split the user's actions in the window into levels.
type Thresholds struct {
	Eligible int // this many or more: ELIGIBLE, HIGH
	Moderate int // this many or more: MODERATE; fewer: LOW, or NONE at zero
}

// Decide turns the activity and the classification into task values. The
// onboarding tasks are left alone. INTERNAL or DISPOSABLE
// excludes and abandons the pairing; CORPORATE or enough actions is ELIGIBLE.
func Decide(act plgactivity.Activity, classification *string, th Thresholds) (values []Value, outcome string, abandon bool) {
	level := LevelNone
	switch {
	case act.Actions >= th.Eligible:
		level = LevelHigh
	case act.Actions >= th.Moderate:
		level = LevelModerate
	case act.Actions > 0:
		level = LevelLow
	}
	values = []Value{
		{Code: codeInitiate, BoolValue: ptr(true)},
		{Code: codeSession, NumberValue: number(act.SessionMinutes)},
		{Code: codeLevel, TextValue: ptr(level)},
	}
	busy := level == LevelHigh
	var reasons []string
	switch class := deref(classification); {
	case class == "INTERNAL":
		outcome, reasons, abandon = OutcomeExcluded, []string{"EXCLUDED_INTERNAL_WSO2_REGISTRATION"}, true
	case class == "DISPOSABLE":
		outcome, reasons, abandon = OutcomeExcluded, []string{"EXCLUDED_DISPOSABLE_EMAIL_ADDRESS"}, true
	case class == "CORPORATE":
		outcome, reasons = OutcomeEligible, []string{"ELIGIBLE_CORPORATE_EMAIL_ADDRESS"}
		if busy {
			reasons = append(reasons, "ELIGIBLE_MEANINGFUL_PRODUCT_ACTIVITY")
		}
	case busy:
		outcome, reasons = OutcomeEligible, []string{"ELIGIBLE_MEANINGFUL_PRODUCT_ACTIVITY"}
	default:
		outcome, reasons = OutcomeMonitor, []string{"MONITOR_MORE_EVIDENCE_NEEDED"}
	}
	values = append(values, Value{Code: codeOutcome, TextValue: ptr(outcome)}, Value{Code: codeReason, CheckedCodes: &reasons})
	return values, outcome, abandon
}

func summary(act plgactivity.Activity, from, to time.Time) Summary {
	return Summary{
		WindowStart: from, WindowEnd: to,
		FirstEventOn: act.FirstEventOn, LastEventOn: act.LastEventOn,
		EventCount: act.EventCount, ActionCounts: act.ActionCounts,
	}
}

func ptr[T any](v T) *T { return &v }

func number(n *int) *float64 {
	if n == nil {
		return nil
	}
	f := float64(*n)
	return &f
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
