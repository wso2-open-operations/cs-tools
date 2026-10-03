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

// Package outagenotifytask is the internal-stakeholder outage notification
// sub-cron. internal/outagenotify holds the entity-service client; the split
// mirrors entitycases/opencases and exists because internal/notify imports the
// client package for its types.
package outagenotifytask

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/outagenotify"
)

// Sweeper is the subset of *outagenotify.Client this package depends on.
type Sweeper interface {
	Sweep(ctx context.Context, limit int) (outagenotify.SweepResult, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// phaseWord turns the decision kind into the word the banner shows.
func phaseWord(kind string) string {
	switch kind {
	case "DECLARED":
		return "Declared"
	case "RESOLVED":
		return "Resolved"
	case "UPDATE":
		return "Update"
	default:
		return kind
	}
}

// SendNotices returns the sub-cron handler: sweep, then send one email per
// decision.
//
// *** ONE EMAIL PER OUTAGE, NOT ONE DIGEST. *** The ServiceNow flow sends a
// separate message per outage per phase, and outage mail is read as it
// arrives; batching several outages into one notice would change what an
// on-call reader sees at the moment it matters.
//
// *** A SEND THAT FAILS IS LOST, NOT RETRIED, AND THAT IS THE DESIGN. ***
// entity-service records each decision as sent before returning it, so the
// next sweep will not offer it again. The alternative — record after sending —
// re-sends the notice whenever the recording fails, and a duplicate outage
// declaration to the whole internal audience is worse than a missed one. Every
// failure is still reported, so the task fails, alerts, and the gap is
// visible; it is just not silently repaired by re-mailing everyone.
//
// One bad recipient does not abandon the rest: every decision is attempted and
// the failures are joined, the same rule internal/allocationreminder uses.
//
// emailsEnabled is ALERTS_ENABLED. As with every other task here, false — or
// an empty `to` — skips the sweep entirely rather than sweeping and
// discarding. That matters more here than elsewhere: sweeping marks decisions
// as sent, so running it with nowhere to deliver would consume notices nobody
// ever receives.
//
// portalBaseURL is CSM_PORTAL_WEB_BASE_URL; each email links to the outage's
// portal page under it, or carries no link when it is empty.
func SendNotices(sweeper Sweeper, email EmailSender, to, cc []string, emailsEnabled bool, portalBaseURL string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled || len(to) == 0 {
			return nil
		}

		res, err := sweeper.Sweep(ctx, 0)
		if err != nil {
			return fmt.Errorf("outagenotifytask: sweep: %w", err)
		}

		var failures []error
		for _, d := range res.Decisions {
			body := notify.RenderOutageNotification(notify.OutageNotificationData{
				PhaseWord: phaseWord(d.Kind),
				Number:    d.Number,
				Message:   d.Body,
				Link:      notify.OutageLink(portalBaseURL, d.OutageID),
			})
			if err := email.SendEmail(ctx, to, cc, d.Subject, body); err != nil {
				failures = append(failures, fmt.Errorf("outage %s (%s): %w", d.Number, d.Kind, err))
			}
		}

		// Server-side recording failures are the sweep's own, already logged
		// there, but they belong in this task's error too — otherwise a run
		// where nothing could be recorded looks like a quiet success.
		if len(res.Errors) > 0 {
			ids := make([]string, 0, len(res.Errors))
			for id := range res.Errors {
				ids = append(ids, id)
			}
			failures = append(failures, fmt.Errorf("entity-service could not record %d outage(s): %s",
				len(res.Errors), strings.Join(ids, ", ")))
		}
		return errors.Join(failures...)
	}
}
