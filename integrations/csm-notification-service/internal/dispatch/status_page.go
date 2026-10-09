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

package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/statuspage"
)

// handleStatusPageDue posts one cloud status webhook AT MOST ONCE and reports
// the outcome. No duplicates by construction: it posts only after winning the
// claim the event carried (a redelivered or late event loses it); one attempt,
// never an error back to the consumer (no consumer retry or DLQ replay); a
// post with no answer is reported unknown, which is never re-sent; a lost
// report leaves the attempt to expire as unknown. csm-scheduled-tasks retries
// only definite failures.
func (d *Dispatcher) handleStatusPageDue(ctx context.Context, raw json.RawMessage) error {
	var p events.OutageStatusPageDuePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("dispatch: decode %s payload: %w", events.TypeOutageStatusPageDue, err)
	}
	if d.statusPageReports == nil {
		slog.WarnContext(ctx, "dispatch: status page not configured; leaving outage.status_page_due to the scheduled task",
			"webhookId", p.WebhookID, "outageId", p.OutageID)
		return nil
	}
	if err := d.statusPageReports.ClaimCloudStatusWebhook(ctx, p.WebhookID, p.ClaimToken); err != nil {
		if errors.Is(err, entity.ErrWebhookNotClaimable) {
			slog.InfoContext(ctx, "dispatch: status page webhook already delivered or taken over; not posting",
				"webhookId", p.WebhookID, "number", p.Number)
			return nil
		}
		// Not claimed, so not posted: the reservation expires to the scheduled task.
		slog.ErrorContext(ctx, "dispatch: claiming status page webhook failed; leaving it to the scheduled task",
			"webhookId", p.WebhookID, "err", err)
		return nil
	}
	outcome := entity.CloudStatusDelivery{Delivered: true}
	if d.statusPage == nil {
		outcome = entity.CloudStatusDelivery{Error: "status page webhooks are not configured on csm-notification-service"}
	} else if err := d.statusPage.Post(ctx, p.Cloud, p.Event, p.Timestamp); err != nil {
		outcome = entity.CloudStatusDelivery{Error: err.Error(), Unknown: statuspage.IsUnknownOutcome(err)}
	}
	if err := d.statusPageReports.RecordCloudStatusDelivery(ctx, p.WebhookID, p.ClaimToken, outcome); err != nil {
		slog.ErrorContext(ctx, "dispatch: reporting status page outcome failed; it will read as unknown and not be re-sent",
			"webhookId", p.WebhookID, "delivered", outcome.Delivered, "err", err)
		return nil
	}
	switch {
	case outcome.Delivered:
		slog.InfoContext(ctx, "dispatch: status page webhook delivered",
			"webhookId", p.WebhookID, "number", p.Number, "cloud", p.Cloud, "event", p.Event)
	case outcome.Unknown:
		slog.ErrorContext(ctx, "dispatch: status page webhook outcome unknown; not re-sent, check the status page",
			"webhookId", p.WebhookID, "number", p.Number, "cloud", p.Cloud, "err", outcome.Error)
	default:
		slog.WarnContext(ctx, "dispatch: status page webhook failed; left for the scheduled task",
			"webhookId", p.WebhookID, "number", p.Number, "cloud", p.Cloud, "err", outcome.Error)
	}
	return nil
}
