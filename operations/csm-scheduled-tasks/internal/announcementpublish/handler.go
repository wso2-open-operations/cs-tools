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

package announcementpublish

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// dueClient is the subset of *Client (the real HTTP client in this same
// package) PublishDue depends on — declared here, mirroring
// internal/housekeeping's own Client interface of the same name/reasoning,
// so a test can substitute a fake without spinning up real HTTP. Unexported
// (unlike housekeeping.Client) only because its concrete implementation
// lives in this same package — a caller outside the package still passes a
// *Client value, it just can't name this interface type itself.
type dueClient interface {
	SearchDueIDs(ctx context.Context) ([]string, error)
	AutoPublish(ctx context.Context, id string) error
}

// PublishDue returns a registry.Task.Handler that finds every
// announcement_requests row whose scheduled_on has arrived and calls
// entity-service's AutoPublish for each one in turn — see that service
// method's own doc comment for the actual fan-out logic, which runs
// entirely server-side in entity-service, not here.
//
// Every due id is attempted independently: one id's failure doesn't stop the
// rest — an announcement stuck on a downstream outage must not also delay
// every other unrelated scheduled announcement. Errors from every attempted
// id are joined into one summary so the engine's own alert email names every
// request that's still stuck, not just the first one found.
//
// Idempotent per period, exactly as registry.Task.Handler requires: a due
// id whose AutoPublish call returns a 409 (not yet fully delivered this
// pass — a partial case-creation/tag failure) is expected, not a bug — it
// simply stays in the "due" result set and gets retried on the very next
// tick, resuming from wherever entity-service's own delivery ledger left
// off. There is no other state kept between ticks in this package at all.
func PublishDue(client dueClient) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		ids, err := client.SearchDueIDs(ctx)
		if err != nil {
			return fmt.Errorf("announcementpublish: search due requests: %w", err)
		}
		if len(ids) == 0 {
			return nil
		}

		var errs []error
		for _, id := range ids {
			if err := client.AutoPublish(ctx, id); err != nil {
				slog.WarnContext(ctx, "announcementpublish: auto-publish attempt did not complete", "id", id, "err", err)
				errs = append(errs, fmt.Errorf("request %s: %w", id, err))
				continue
			}
			slog.InfoContext(ctx, "announcementpublish: auto-published a scheduled announcement request", "id", id)
		}
		if len(errs) > 0 {
			return errors.Join(errs...)
		}
		return nil
	}
}
