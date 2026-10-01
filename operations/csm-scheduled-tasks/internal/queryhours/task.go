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

package queryhours

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// DefaultStaleFor is how old a project's stored position must be before the
// sweep recomputes it. One hour, matching the default schedule, so each run
// picks up whatever moved since the last one without re-deriving projects
// nothing has touched.
const DefaultStaleFor = time.Hour

// DefaultLimit caps one run. The sweep is resumable — entity-service orders
// by staleness — so the cap costs latency, never coverage.
//
// Sized against the SERVER's deadline, not this client's: entity-service wraps
// its mux in middleware.Timeout(30s), and a sweep that outlives it stops early
// and returns a partial result. 50 projects of aggregate fits comfortably
// inside 30 seconds; anything left over is still the stalest and is picked up
// by the next hourly run.
const DefaultLimit = 50

// Sweeper is the subset of *Client this package depends on.
type Sweeper interface {
	Sweep(ctx context.Context, staleFor time.Duration, limit int) (SweepResult, error)
}

// RecomputeQueryHours returns the sub-cron handler.
//
// Partial failure is reported, not swallowed: a run where some projects
// failed returns an error so the task is marked failed and its
// SUB_CRON_RECIPIENTS are alerted — but only AFTER the successful projects
// have been committed server-side, because entity-service continues past a
// failing project rather than abandoning the sweep. So a bad project costs
// an alert, not the whole run's work.
func RecomputeQueryHours(client Sweeper, staleFor time.Duration, limit int) func(context.Context) error {
	if staleFor <= 0 {
		staleFor = DefaultStaleFor
	}
	if limit <= 0 {
		limit = DefaultLimit
	}

	return func(ctx context.Context) error {
		res, err := client.Sweep(ctx, staleFor, limit)
		if err != nil {
			return fmt.Errorf("query-hour sweep: %w", err)
		}

		slog.Info("query-hour sweep complete",
			"requested", res.Requested,
			"succeeded", res.Succeeded,
			"failed", res.Failed)

		if res.Failed > 0 {
			// Log each failing project before returning: the error string
			// that reaches the alert email is one line, and "which projects"
			// is the first question anyone will ask.
			for projectID, msg := range res.Errors {
				slog.Error("query-hour sweep: project failed",
					"projectId", projectID, "error", msg)
			}
			return fmt.Errorf("query-hour sweep: %d of %d projects failed",
				res.Failed, res.Requested)
		}
		return nil
	}
}
