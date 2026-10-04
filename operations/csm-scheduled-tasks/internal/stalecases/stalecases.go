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

// Package stalecases is the "cases open too long" report sub-cron: it finds
// every case that's been open for more than a configured threshold and
// emails a report of them. This is the first sub-cron in this component to
// send an email on success rather than only on failure — see this
// component's own CLAUDE.md ("Per-task report emails") for why that
// wasn't built as a generic engine feature: the report itself is sent from
// inside SendReport's returned handler, using recipients the caller supplies
// directly, not through engine.Engine's failure-alert path at all.
package stalecases

import (
	"context"
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

// CaseSearcher is the subset of *entitycases.Client this package depends on.
type CaseSearcher interface {
	SearchOpenCasesOlderThan(ctx context.Context, olderThan time.Duration) ([]entitycases.Case, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// SendReport returns a registry.Task.Handler that finds every case open for
// more than olderThan and emails to/cc a report of them. Recipients are
// exactly this task's own SUB_CRON_RECIPIENTS entry (see
// cmd/server/main.go) — deliberately not merged with the standing
// ALERT_RECIPIENTS audience, which is about failure alerting, not report
// distribution; a deployment that wants the same people getting both
// configures the same addresses in both places.
//
// emailsEnabled is cmd/server/main.go's ALERTS_ENABLED, the same global kill
// switch engine.Engine.AlertsEnabled uses — despite the env var's name, it
// silences every email this component sends, not just failure alerts, since
// this report is the other kind. When false, the returned handler still
// succeeds (the ledger records success, the task doesn't retry) but sends
// nothing.
//
// If emailsEnabled is false or to is empty, the returned handler skips the
// case search entirely and returns nil (success) without contacting
// entity-service at all — nothing would be done with the result anyway, so
// there's no reason to spend an entity-service query on every tick, in a
// deployment that hasn't configured this task's recipients yet or has gone
// quiet.
//
// guard makes the report at most once per period (see internal/reportguard):
// without it, a run whose ledger record-back failed after the e-mail went
// out is reclaimed later and mails the identical report again. The whole
// search-render-send runs inside the guard, so a period already sent costs
// no query either. nil sends unguarded.
func SendReport(cases CaseSearcher, email EmailSender, guard SendGuard, olderThan time.Duration, to, cc []string, emailsEnabled bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled || len(to) == 0 {
			return nil
		}
		send := func(ctx context.Context) error {
			found, err := cases.SearchOpenCasesOlderThan(ctx, olderThan)
			if err != nil {
				return fmt.Errorf("stalecases: search open cases: %w", err)
			}

			thresholdDays := int(olderThan.Hours() / 24)
			subject := fmt.Sprintf("[Action-Required][Report] Cases open for more than %d days", thresholdDays)
			body := notify.RenderStaleCasesReport(notify.StaleCasesReportData{
				ThresholdDays: thresholdDays,
				Cases:         found,
			})
			if err := email.SendEmail(ctx, to, cc, subject, body); err != nil {
				return fmt.Errorf("stalecases: send report email: %w", err)
			}
			return nil
		}
		if guard == nil {
			return send(ctx)
		}
		return guard.Once(ctx, send)
	}
}

// SendGuard is the subset of *reportguard.Guard this package depends on.
type SendGuard interface {
	Once(ctx context.Context, send func(ctx context.Context) error) error
}
