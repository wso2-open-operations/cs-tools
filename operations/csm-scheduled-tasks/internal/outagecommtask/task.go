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

// Package outagecommtask is the outage-communication sub-cron: the SRE-facing
// declaration and resolution emails. internal/outagecomm holds the
// entity-service client; the split mirrors outagenotify/outagenotifytask.
package outagecommtask

import (
	"context"
	"errors"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/outagecomm"
)

// Sweeper is the subset of *outagecomm.Client this package depends on.
type Sweeper interface {
	Sweep(ctx context.Context, limit int) (outagecomm.SweepResult, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// SendCommunications returns the sub-cron handler: sweep, then send one email
// per decision.
//
// *** ONE EMAIL PER OUTAGE, NOT A DIGEST. *** The legacy workflow sends a
// separate message per outage per phase, and outage mail is read as it arrives.
// Batching would change what an on-call reader sees at the moment it matters.
//
// *** A SEND THAT FAILS IS LOST, NOT RETRIED. *** entity-service writes the
// communication-log row before returning the decision, so the next sweep will
// not offer it again. Recording after sending would instead re-send whenever
// the recording failed, and a duplicate announcement to a standing group is
// worse than a gap the on-call notices anyway. Every failure is still
// reported, so the task fails and alerts — the gap is visible, just not
// silently repaired by re-mailing everyone.
//
// One bad recipient does not abandon the rest: every decision is attempted
// and the failures are joined, the same rule internal/announcementpublish uses.
//
// emailsEnabled is ALERTS_ENABLED. As everywhere else here, false — or an
// empty `to` — skips the sweep ENTIRELY rather than sweeping and discarding.
// That matters more here than for a report: sweeping writes log rows, so
// running it with nowhere to deliver would consume announcements nobody ever
// receives and leave those outages permanently marked as declared.
func SendCommunications(sweeper Sweeper, email EmailSender, to, cc []string, emailsEnabled bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled || len(to) == 0 {
			return nil
		}

		res, err := sweeper.Sweep(ctx, 0)
		if err != nil {
			return fmt.Errorf("outagecommtask: sweep: %w", err)
		}

		var failures []error
		for _, d := range res.Decisions {
			if err := email.SendEmail(ctx, to, cc, d.Subject, renderBody(d)); err != nil {
				failures = append(failures, fmt.Errorf("outage %s (%s): %w", d.Number, d.Kind, err))
			}
		}
		return errors.Join(failures...)
	}
}

// renderBody wraps entity-service's plain-text body for email.
//
// *** THE BODY ARRIVES AS TEXT AND IS ESCAPED, NOT INTERPOLATED. *** It
// carries an outage's short description and impact, which are operator-typed
// free text on a mirrored record. Dropping that into HTML unescaped would let
// a stray `<` silently swallow the rest of the mail, and anything worse if
// the field ever carried markup. Escape first, then turn newlines into
// breaks — in that order, so the breaks survive. notify.EscapeMultiline does
// both, and also writes every non-ASCII rune as a numeric character
// reference: the e-mail service does not reliably carry raw multi-byte
// characters, so an em dash or accented name typed into an outage would
// otherwise arrive as "?" — the same treatment every other e-mail here gets.
//
// Deliberately not a shared template: unlike the internal notifier, whose
// three bodies are one fixed sentence apiece, these two are full messages
// rendered upstream. The presentation decision belongs where the content is.
func renderBody(d outagecomm.Decision) string {
	withBreaks := notify.EscapeMultiline(d.Body)
	return `<div style="font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;font-size:14px;line-height:1.6;color:#17191e">` +
		withBreaks +
		`</div>`
}
