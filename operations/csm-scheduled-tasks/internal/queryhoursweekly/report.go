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

// Package queryhoursweekly is the weekly query-support consumption report
// sub-cron. It holds the task; internal/queryhoursreport holds the
// entity-service client and the wire types it returns.
//
// The split mirrors internal/entitycases and internal/opencases, and exists
// for the same mechanical reason: internal/notify imports the client package
// to render its types, so a task that imports notify cannot live beside them
// without a cycle.
package queryhoursweekly

import (
	"context"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhoursreport"
)

// Subject is the email's subject line, verbatim from the ServiceNow original.
const Subject = "Weekly Query Support Consumption Report"

// ReportFetcher is the subset of *Client this package depends on.
type ReportFetcher interface {
	WeeklyReport(ctx context.Context) (queryhoursreport.Report, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// SendReport returns the sub-cron handler: fetch the report, render it, send
// it.
//
// *** THE To LINE IS DERIVED FROM THE DATA; Cc IS CONFIGURATION. ***
//
// `to` is the SEED, not the audience: DeriveRecipients appends the account
// manager and technical owner of every EXCEEDED account to it. ServiceNow
// does the same, opening with a literal one-address seed and pushing both
// owners per exceeded account.
//
// This was static until 2026-10-01 and the reason it was is worth keeping,
// because it is the reason to re-check the count rather than trust it: the
// ServiceNow copy available for inspection in dev provably was not the one
// sending production's mail — two Cc addresses against the real message's
// three, and a derivation capping near thirty-three recipients where the
// real message reaches about forty-eight. The production action script has
// since been read directly and the rule below is that script's, so the rule
// is no longer a guess. THE COUNT STILL IS. If a run addresses far fewer
// than the live mail, suspect `account.account_manager_id` /
// `technical_owner_id` population before suspecting this function — those
// mirror `u_owner` / `u_technical_owner` through csm-sync-service, and an
// unpopulated column silently shortens the audience instead of failing.
//
// *** WHILE THE SERVICENOW FLOW IS STILL ENABLED, BOTH SYSTEMS SEND. ***
// It also writes sf_opportunity.query_hour_state, so it cannot simply be
// turned off (see docs/choreo-deployment-config.md). Until that is settled a
// derived To means ~48 people receive two reports a week, where a narrow one
// meant a handful did. ALERTS_ENABLED=false, or an empty To for this task,
// is the lever — and an empty To now skips the send for that reason as well
// as the original one.
//
// emailsEnabled is ALERTS_ENABLED. As with the other report tasks, false
// skips the fetch entirely rather than fetching and discarding — and so does
// an empty `to`, since a report nobody receives is work with no consumer.
//
// An empty report is still sent. "No account has exhausted its query hours"
// is a real answer, and a week where the mail simply does not arrive is
// indistinguishable from a week where the job silently broke.
func SendReport(reports ReportFetcher, email EmailSender, to, cc []string, salesforceBaseURL string, emailsEnabled bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled || len(to) == 0 {
			return nil
		}

		report, err := reports.WeeklyReport(ctx)
		if err != nil {
			return fmt.Errorf("queryhoursreport: fetch weekly report: %w", err)
		}

		// Derived AFTER the fetch, because the audience is a function of the
		// report. A week with nothing exceeded addresses the seed alone, and
		// still sends — see the note below on empty reports.
		recipients := DeriveRecipients(report, to)

		body := notify.RenderQueryHoursWeeklyReport(notify.QueryHoursWeeklyReportData{
			Report:            report,
			SalesforceBaseURL: salesforceBaseURL,
		})
		if err := email.SendEmail(ctx, recipients, cc, Subject, body); err != nil {
			return fmt.Errorf("queryhoursreport: send report email: %w", err)
		}
		return nil
	}
}

// wso2Domain is the address suffix ServiceNow filters the derived audience on.
//
// *** THE SERVICENOW TEST IS A SUBSTRING TEST, AND THIS ONE IS NOT. ***
// The action script asks `ref_email.includes("@wso2.com")`, which also admits
// an address like "someone@wso2.community". This uses a suffix test instead:
// the only addresses it therefore drops are ones that are not WSO2 mailboxes,
// and mailing a stranger is the worse failure. Recorded as a deliberate
// divergence rather than left as a silent difference in behaviour.
const wso2Domain = "@wso2.com"

// DeriveRecipients builds the report's To line the way ServiceNow builds it:
// the configured seed, then the account manager and technical owner of every
// account in the EXCEEDED table.
//
// *** ACCOUNTS MERELY GOING TO EXCEED CONTRIBUTE NOBODY. *** In the action
// script the two pushes sit inside `if (acc_data.is_exceeded)`, while the
// close-to-exceed branch appends HTML only. Deriving from both tables is the
// obvious mistake and would widen the audience every week.
//
// Both owners of one account are collected independently, so a single account
// can contribute two addresses — or one, or none.
//
// Addresses are lower-cased before the domain test and before the duplicate
// check, matching `ref_email.toLowerCase()` followed by `arrayUtil.unique`:
// without the fold, "A@wso2.com" and "a@wso2.com" both survive as separate
// recipients. Order is first appearance, so the seed stays at the head and a
// run is reproducible.
//
// The seed is NOT hardcoded here. ServiceNow opens with a literal
// `to_list = ["kalanad@wso2.com"]`; that address belongs in this task's
// SUB_CRON_RECIPIENTS entry, where every other recipient in this component
// lives, so it can be changed without a deploy.
func DeriveRecipients(report queryhoursreport.Report, seed []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(seed)+2*len(report.Exceeded))

	add := func(address string) {
		address = strings.ToLower(strings.TrimSpace(address))
		// An owner reference whose user record carries no email arrives as
		// "" and fails the domain test, which is how ServiceNow drops it too
		// — `"".includes("@wso2.com")` is false.
		if !strings.HasSuffix(address, wso2Domain) || seen[address] {
			return
		}
		seen[address] = true
		out = append(out, address)
	}

	// The seed is exempt from the domain test: it is operator-configured, not
	// derived from account data, and a deployment that wants a non-wso2.com
	// address on this report is making that choice deliberately.
	for _, address := range seed {
		address = strings.ToLower(strings.TrimSpace(address))
		if address == "" || seen[address] {
			continue
		}
		seen[address] = true
		out = append(out, address)
	}

	for _, account := range report.Exceeded {
		add(account.AccountManagerEmail)
		add(account.TechnicalOwnerEmail)
	}

	return out
}
