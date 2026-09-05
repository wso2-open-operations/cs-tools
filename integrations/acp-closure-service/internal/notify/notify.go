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

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/closure"
	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/recipients"
)

// Recipients is the structured set of people one Notice should reach.
// AccountOwner (the Account Manager), RenewalManager, and TechnicalOwner are
// always populated for a day-count reminder — an individual Contact's Email
// may legitimately be "" per recipients.AccountManagerEmail's existing
// convention (role assigned but no email on file, or no role assigned at
// all), which is not an error state. Customer is nil except on a resolved
// 15/7/0-window notice; it is also nil (not a zero-value Contact) on the
// separate no-business-contact notice, which names only an Account Owner.
type Recipients struct {
	AccountOwner   recipients.Contact
	RenewalManager recipients.Contact
	TechnicalOwner recipients.Contact
	Customer       *recipients.Contact
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
	StartDate   time.Time
	EndDate     time.Time
	Window      closure.NoticeWindow
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
	// fallback was attempted (see recipients.ResolveCustomerContact). Left
	// at its zero value ("") only when the fallback was never attempted at
	// all — an internal-only 90/60/30 notice. It IS set on the
	// no-business-contact notice too, to recipients.ResolvedViaNone — the
	// fallback was attempted there, it just found nothing; that's a
	// different, more specific fact than "never attempted," worth keeping
	// distinct in the log.
	ResolvedVia recipients.ResolvedVia
}

// LoggingNotifier logs what would have been sent instead of sending it.
type LoggingNotifier struct {
	Logger *slog.Logger
}

// Send logs the notice and always succeeds. Reports delivered=false always
// — it only logs what would have been sent, it never actually delivers a
// notice to anyone.
func (n *LoggingNotifier) Send(ctx context.Context, notice Notice) (bool, error) {
	attrs := []any{
		"subject", notice.Subject,
		"window", notice.Window,
		"projectID", notice.ProjectID,
		"projectName", notice.ProjectName,
		"projectKey", notice.ProjectKey,
		"startDate", notice.StartDate,
		"endDate", notice.EndDate,
		"accountOwner", notice.Recipients.AccountOwner.Email,
		"accountOwnerName", notice.Recipients.AccountOwner.Name,
		"renewalManager", notice.Recipients.RenewalManager.Email,
		"renewalManagerName", notice.Recipients.RenewalManager.Name,
		"technicalOwner", notice.Recipients.TechnicalOwner.Email,
		"technicalOwnerName", notice.Recipients.TechnicalOwner.Name,
		"resolvedVia", notice.ResolvedVia,
	}
	if notice.Recipients.Customer != nil {
		attrs = append(attrs, "customer", notice.Recipients.Customer.Email, "customerName", notice.Recipients.Customer.Name)
	}
	if notice.Body != "" {
		attrs = append(attrs, "body", notice.Body)
	}

	n.Logger.InfoContext(ctx, "notice", attrs...)
	return false, nil
}
