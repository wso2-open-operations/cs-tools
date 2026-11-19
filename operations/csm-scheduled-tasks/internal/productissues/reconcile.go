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

// Package productissues drives entity-service's daily reconciliation of the
// product issues linked to GitHub (git internals): it brings every linked
// issue's state, labels and board status up to date for the day a webhook was
// missed. entity-service does the work (POST /product-work-items/reconcile, see
// its CLAUDE.md, "Product issues"); this only calls it page after page.
package productissues

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// TaskName is the ledger key of the daily reconciliation. Renaming it orphans
// its history there.
const TaskName = "product_issue_reconcile"

// DefaultSchedule is 01:00 UTC every day (06:30 in Colombo), before the working day.
const DefaultSchedule = "0 1 * * *"

// Reconciler is the entity-service call the task makes: one call of
// POST /product-work-items/reconcile. Satisfied by *entity.Client.
type Reconciler interface {
	ProductIssueReconcile(ctx context.Context, body []byte) ([]byte, error)
}

// maxPages stops a run whose calls never end (an upstream that keeps answering
// with a cursor): 5,000 calls of the limit below, far more than the boards hold.
const maxPages = 5000

// pageLimit is how many board items or linked issues one call works on.
const pageLimit = 200

// maxRateLimitWait is the longest Retry-After the run waits out; a longer one
// fails the run. maxRateLimitWaits bounds how many waits one run makes.
const (
	maxRateLimitWait  = 900 * time.Second
	maxRateLimitWaits = 3
)

// sleepFunc waits d or until ctx ends (tests replace it so they do not sleep).
type sleepFunc func(ctx context.Context, d time.Duration) error

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type request struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}

type response struct {
	Boards              int    `json:"boards"`
	ItemsChecked        int    `json:"itemsChecked"`
	LinksUpdated        int    `json:"linksUpdated"`
	StatusEventsWritten int    `json:"statusEventsWritten"`
	NotOnBoard          int    `json:"notOnBoard"`
	Failed              int    `json:"failed"`
	NextCursor          string `json:"nextCursor"`
	More                bool   `json:"more"`
	RetryAfterSeconds   int    `json:"retryAfterSeconds"`
}

// RunReconcile returns the product_issue_reconcile handler: call entity-service
// from the start while it says more is left, following nextCursor. A call that
// stopped on GitHub's rate limit (retryAfterSeconds) of at most 15 minutes is
// waited out (at most 3 times a run) and the run goes on from the cursor it
// returned; a longer wait, or a fourth one, fails the run with the position it
// reached, so the engine alerts and retries. A run in which boards or issues
// failed also returns an error (what is already up to date costs a retry
// little). entity-service's log names each failure.
func RunReconcile(client Reconciler) func(ctx context.Context) error {
	return runReconcile(client, sleepContext)
}

func runReconcile(client Reconciler, sleep sleepFunc) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var total response
		cursor := ""
		waits := 0
		for page := 0; ; page++ {
			if page >= maxPages {
				return fmt.Errorf("product issue reconcile: still more after %d calls; stopping", maxPages)
			}
			body, err := json.Marshal(request{Cursor: cursor, Limit: pageLimit})
			if err != nil {
				return fmt.Errorf("product issue reconcile: build request: %w", err)
			}
			raw, err := client.ProductIssueReconcile(ctx, body)
			if err != nil {
				return fmt.Errorf("product issue reconcile: call after cursor %q: %w", cursor, err)
			}
			var r response
			if err := json.Unmarshal(raw, &r); err != nil {
				return fmt.Errorf("product issue reconcile: parse the answer after cursor %q: %w", cursor, err)
			}
			total.Boards += r.Boards
			total.ItemsChecked += r.ItemsChecked
			total.LinksUpdated += r.LinksUpdated
			total.StatusEventsWritten += r.StatusEventsWritten
			total.NotOnBoard += r.NotOnBoard
			total.Failed += r.Failed
			if r.RetryAfterSeconds > 0 {
				wait := time.Duration(r.RetryAfterSeconds) * time.Second
				if wait > maxRateLimitWait || waits >= maxRateLimitWaits || !r.More || r.NextCursor == "" {
					return fmt.Errorf("product issue reconcile: GitHub is rate limiting (retry after %ds, %d wait(s) used of %d); stopped at %s with %d items checked, %d links updated, %d failed",
						r.RetryAfterSeconds, waits, maxRateLimitWaits, position(r.NextCursor), total.ItemsChecked, total.LinksUpdated, total.Failed)
				}
				waits++
				slog.WarnContext(ctx, "product issue reconcile: GitHub is rate limiting; waiting",
					"retryAfterSeconds", r.RetryAfterSeconds, "wait", waits, "position", position(r.NextCursor), "itemsChecked", total.ItemsChecked)
				if err := sleep(ctx, wait); err != nil {
					return fmt.Errorf("product issue reconcile: interrupted while waiting for the rate limit at %s with %d items checked: %w",
						position(r.NextCursor), total.ItemsChecked, err)
				}
				cursor = r.NextCursor
				continue
			}
			// more is the only end signal; while it is set the cursor must move on, or
			// the next call would repeat this one until the cap.
			if !r.More {
				break
			}
			if r.NextCursor == "" || r.NextCursor == cursor {
				return fmt.Errorf("product issue reconcile: no progress after %q (more is set but the cursor did not move)", cursor)
			}
			cursor = r.NextCursor
		}
		slog.InfoContext(ctx, "product issue reconcile: finished",
			"boards", total.Boards, "itemsChecked", total.ItemsChecked, "linksUpdated", total.LinksUpdated,
			"statusEventsWritten", total.StatusEventsWritten, "notOnBoard", total.NotOnBoard, "failed", total.Failed)
		return failedError(total.Failed)
	}
}

// position names where a run stands from entity-service's cursor (opaque to this
// task, but base64 JSON with the board's owner "o", number "b" and phase "p"): the
// board and phase when it reads, else the cursor itself.
func position(cursor string) string {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err == nil {
		var c struct {
			Owner string `json:"o"`
			Board int    `json:"b"`
			Phase string `json:"p"`
		}
		if json.Unmarshal(b, &c) == nil && (c.Owner != "" || c.Board != 0) {
			return fmt.Sprintf("board %s/%d (%s phase), cursor %q", c.Owner, c.Board, c.Phase, cursor)
		}
	}
	return fmt.Sprintf("cursor %q", cursor)
}

func failedError(failed int) error {
	if failed == 0 {
		return nil
	}
	return fmt.Errorf("product issue reconcile: %d board(s) or issue(s) failed; see entity-service's log for each error", failed)
}
