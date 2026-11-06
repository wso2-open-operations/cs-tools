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

package standardchangesuggestions

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const (
	// PageSize is how many customers one call asks entity-service to compute
	// (its own default); a call short of time returns fewer.
	PageSize = 50
	// MaxPages bounds one run: 2,000 pages is far more customers than exist
	// (each page computes at least one). Reaching it fails the run and
	// publishes nothing, leaving the previous snapshot in place.
	MaxPages = 2000
)

type refresher interface {
	Refresh(ctx context.Context, req Request) (Response, error)
}

// Run returns a registry.Task.Handler that computes a new snapshot: it starts a
// run, asks for page after page while entity-service says there is more, and
// the last page publishes the run.
//
// It returns an error when a call fails, when a page makes no progress (more is
// set but the cursor did not move), or at MaxPages, so the engine alerts and
// retries. A retry starts a new run; the one left unfinished is never read and
// is deleted when a later run is published. Readers keep the previous snapshot
// until then, so a failed run costs freshness, never a half-written snapshot.
func Run(client refresher) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		started := time.Now()
		req := Request{Limit: PageSize}
		customers, changeRequests, suggestions := 0, 0, 0
		for page := 1; ; page++ {
			if page > MaxPages {
				return fmt.Errorf("standard change suggestions refresh: still more after %d pages; nothing was published", MaxPages)
			}
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("standard change suggestions refresh: %w", err)
			}
			resp, err := client.Refresh(ctx, req)
			if err != nil {
				return fmt.Errorf("standard change suggestions refresh: page %d: %w; nothing was published", page, err)
			}
			customers += resp.Customers
			changeRequests += resp.ChangeRequests
			suggestions += resp.Suggestions
			if !resp.More {
				totals := Totals{Customers: customers, ChangeRequests: changeRequests}
				if resp.Totals != nil {
					totals = *resp.Totals
				}
				slog.InfoContext(ctx, "standard change suggestions refresh: published", "runId", resp.RunID,
					"computedOn", resp.ComputedOn, "pages", page, "customers", totals.Customers,
					"changeRequests", totals.ChangeRequests, "clusters", totals.Clusters, "eligible", totals.Eligible,
					"suggestionsComputed", suggestions, "durationMs", time.Since(started).Milliseconds())
				return nil
			}
			if resp.RunID == "" || resp.NextCursor == "" || resp.NextCursor == req.Cursor || (req.RunID != "" && resp.RunID != req.RunID) {
				return fmt.Errorf("standard change suggestions refresh: no progress on page %d (more is set but the run or cursor did not move on); nothing was published", page)
			}
			req = Request{RunID: resp.RunID, Cursor: resp.NextCursor, Limit: PageSize}
		}
	}
}
