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

// Package opencases is the "cases still in Open state" report sub-cron: it
// finds every case created before yesterday that has never moved out of the
// initial "open" state, and emails a report of them. Distinct from
// internal/stalecases (any non-closed state, 30 days) — this one flags a
// narrower, more urgent signal: a case nobody has even started working on
// yet. Same shape otherwise: see internal/stalecases's own package doc
// comment for why this report is sent from inside the handler itself, not
// through engine.Engine's failure-alert path.
package opencases

import (
	"context"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

// openState is entity-service's exact CaseState value this report queries —
// see entity-service's own domain.CaseStateOpen. Unlike
// stalecases.SendReport's "not closed" filter, this is an exact match: a
// case that has since moved to work_in_progress (or any other non-open,
// non-closed state) no longer belongs in this specific report.
const openState = "open"

// CaseSearcher is the subset of *entitycases.Client this package depends on.
type CaseSearcher interface {
	SearchCasesInStateCreatedBeforeYesterday(ctx context.Context, state string) ([]entitycases.Case, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// SendReport returns a registry.Task.Handler that finds every case created
// before yesterday and still in "open" state, and emails to/cc a report of
// them. Recipients are exactly this task's own SUB_CRON_RECIPIENTS entry
// (see cmd/server/main.go) — same reasoning as stalecases.SendReport's own
// doc comment: unrelated to the standing ALERT_RECIPIENTS failure-alert
// audience.
//
// emailsEnabled is cmd/server/main.go's ALERTS_ENABLED — see
// stalecases.SendReport's own doc comment for why this silences report
// emails too, not just failure alerts.
//
// If emailsEnabled is false or to is empty, the returned handler skips the
// case search entirely and returns nil (success) — same reasoning as
// stalecases.SendReport.
//
// guard makes the report at most once per period — see
// stalecases.SendReport's doc comment. nil sends unguarded.
func SendReport(cases CaseSearcher, email EmailSender, guard SendGuard, to, cc []string, emailsEnabled bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled || len(to) == 0 {
			return nil
		}
		send := func(ctx context.Context) error {
			found, err := cases.SearchCasesInStateCreatedBeforeYesterday(ctx, openState)
			if err != nil {
				return fmt.Errorf("opencases: search open cases: %w", err)
			}

			subject := "[Action-Required][Report] Cases created before yesterday still in Open state"
			body := notify.RenderOpenCasesReport(notify.OpenCasesReportData{
				Cases: found,
			})
			if err := email.SendEmail(ctx, to, cc, subject, body); err != nil {
				return fmt.Errorf("opencases: send report email: %w", err)
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
