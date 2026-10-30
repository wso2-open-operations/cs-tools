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

// Package notify defines the shape of an ACP notice and a logging
// implementation of sending one. Real email sending is not implemented on
// either side yet — deferred pending message-queue design (per the 2026-07-17
// meeting notes) — so LoggingNotifier is not a temporary stand-in for this
// component specifically; it is genuinely the only option available today.
package notify

import (
	"context"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/closure"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/recipients"
)

// Recipients is the structured set of people one Notice should reach.
// AccountOwner (the Account Manager), RenewalManager, and TechnicalOwner are
// always populated for a day-count reminder — an individual Contact's Email
// may legitimately be "" per recipients.AccountManagerEmail's existing
// convention (role assigned but no email on file, or no role assigned at
// all), which is not an error state. Customers is empty except on a resolved
// 15/7/0-window notice, where it holds every resolved customer contact
// (see recipients.ResolveCustomerContacts); it is also empty on the separate
// no-business-contact notice.
type Recipients struct {
	AccountOwner   recipients.Contact
	RenewalManager recipients.Contact
	TechnicalOwner recipients.Contact
	Customers      []recipients.Contact
}

// IsCustomerFacing reports whether r is for a customer-facing notice: one
// with at least one customer contact. Internal notices and the
// no-business-contact nudge have none.
func (r Recipients) IsCustomerFacing() bool {
	return len(r.Customers) > 0
}

// Notice is everything a Notifier needs to send (or, today, log) one ACP
// notification. There is no more Kind field distinguishing
// internal/customer/am_nudge audiences — that distinction is now implied by
// Subject's wording and which Recipients fields are populated, per Chamara's
// request that the log stop saying "internal"/"external" explicitly.
type Notice struct {
	ProjectID   string
	ProjectName string
	ProjectKey  string
	// ProjectSfID is the project's Salesforce record ID, when known — used
	// only by EmailNotifier to render the internal notice's "Project Name"
	// field as a hyperlink to https://wso2.my.salesforce.com/{ProjectSfID},
	// matching a real reference email. Empty when absent (no Salesforce ID
	// on file), in which case that field renders as plain text like every
	// other field row — never used for customer-facing notices, which
	// don't get this link at all (confirmed absent from the real
	// customer-facing reference email).
	ProjectSfID string
	// InvoiceSfIDs are the listed invoices' own Salesforce record IDs (a0I...
	// IDs, distinct from ProjectSfID), one per invoice box in Body and in
	// the same order, set only on the internal invoice notice. EmailNotifier
	// renders each as that box's "Open in Salesforce" link, matching the real
	// reference email. An empty entry, or a missing one, means no link for
	// that box — and customer-facing notices never carry any (customers have
	// no Salesforce access).
	InvoiceSfIDs []string
	StartDate    time.Time
	EndDate      time.Time
	Window       closure.NoticeWindow
	// Subject is the notice's title line — one of five templates depending
	// on notice type and window (see sweep.go's internalNoticeSubject/
	// customerNoticeSubject for the exact wording): the internal day-count
	// reminder (90/60/30/15/7, [ACP]-prefixed — every internal window, not
	// just 90/60/30), the internal day-0 suspension notice (also
	// [ACP]-prefixed, distinct wording since there's no "days remaining"
	// left to report), the customer day-count/day-0 notices (never
	// [ACP]-prefixed), or the no-business-contact notice's fixed
	// "[Urgent] [ACP] No Business Contacts Specified for Project
	// {ProjectName}".
	Subject string
	// Body is the notice's full email body — populated for every notice
	// type today (day-count reminder, day-0 suspension, customer notice,
	// no-business-contact urgent notice all have their own template).
	Body       string
	Recipients Recipients
	// ResolvedVia records which tier of the three-tier customer-contact
	// fallback was attempted (see recipients.ResolveCustomerContacts). Left
	// at its zero value ("") only when the fallback was never attempted at
	// all — an internal-only 90/60/30 notice. It IS set on the
	// no-business-contact notice too, to recipients.ResolvedViaNone — the
	// fallback was attempted there, it just found nothing; that's a
	// different, more specific fact than "never attempted," worth keeping
	// distinct in the log.
	ResolvedVia recipients.ResolvedVia
}

// LoggingNotifier logs that a notice would have been sent instead of
// sending it. The log names the project and says how many recipients the
// notice has, never who they are: no email address, no name (staff or
// customer) and no body, masked or not, since logs must hold no personal
// data in any mode (Rashmika's review of PR #2134). The subject stays: it
// names only the project and the customer company.
type LoggingNotifier struct {
	Logger *slog.Logger
}

// Send logs the notice and always succeeds. Reports delivered=false always
// — it only logs what would have been sent, it never actually delivers a
// notice to anyone.
func (n *LoggingNotifier) Send(ctx context.Context, notice Notice) (bool, error) {
	to, cc := recipientsToToCC(notice.Recipients)
	n.Logger.InfoContext(ctx, "notice",
		"subject", notice.Subject,
		"window", notice.Window,
		"projectID", notice.ProjectID,
		"projectName", notice.ProjectName,
		"projectKey", notice.ProjectKey,
		"startDate", notice.StartDate,
		"endDate", notice.EndDate,
		"toCount", len(to),
		"ccCount", len(cc),
		"customerCount", len(notice.Recipients.Customers),
		"resolvedVia", notice.ResolvedVia,
	)
	return false, nil
}
