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

package closure

import "time"

// DecideInvoice mirrors Decide, but for the invoice-based closure reason —
// ported from the legacy ACPMainProcess.js's based_on_due_invoices path. It
// runs the exact same 90/60/30/15/7/0 cascade as Decide, just counted down
// to a different anchor date: the one InvoiceSuspendDate computes from the
// invoice's own dates and EULA rule, rather than the subscription end date.
// lastNoticeWindow is this cascade's own idempotency track (the legacy
// based_on_due_invoices state), entirely separate from the subscription
// end-date cascade's.
//
// The caller is responsible for a precondition this function has no way to
// represent, and so must never be called without it already having been
// checked: that this project actually has a due invoice to evaluate. This
// mirrors how Decide itself assumes the caller already confirmed a real
// endDate exists rather than handling "no end date" internally.
//
// isPartner is NOT a precondition of this function — hasPrimaryPartner
// already carries everything DecideInvoice needs to know about partner
// status (see usesGracePeriod). Whether isPartner should additionally
// disable the cascade entirely is the caller's decision, and per legacy's
// calculateEventTypeFromDate (ACPMainProcess.js), that only happens when
// isPartner is true AND hasPrimaryPartner is false — hasPrimaryPartner is
// checked first, unconditionally, and overrides isPartner when true. isPartner
// itself is confirmed (via the real API, not the legacy code's naming) to be
// an account-level flag, not a project-level one — despite the legacy JS
// reading it off projectDetails, which most likely just carried a
// denormalized copy of the account's own flag rather than a genuinely
// independent per-project fact.
func DecideInvoice(
	now, invoiceDate, invoiceDueDate time.Time,
	eulaVersion float64,
	hasPrimaryPartner bool,
	lastNoticeWindow *NoticeWindow,
) Decision {
	suspendDate := InvoiceSuspendDate(invoiceDate, invoiceDueDate, eulaVersion, hasPrimaryPartner)
	return decideFromDaysRemaining(daysBetween(now, suspendDate), lastNoticeWindow)
}

// usesGracePeriod reports whether the EULA-3.3-style 60-day grace period
// (see InvoiceSuspendDate) applies, per the legacy code's precedence: an
// account with a primary partner always uses it, regardless of the
// invoice's own EULA version; otherwise it's decided by the EULA version
// alone. eulaVersion <= 0 (unset/unknown) never uses it on its own — the
// legacy code's "eula_version > 0" guard exists because ServiceNow returns
// 0 for a missing decimal field, not because 0 is a real EULA version.
func usesGracePeriod(eulaVersion float64, hasPrimaryPartner bool) bool {
	return hasPrimaryPartner || (eulaVersion > 0 && eulaVersion <= 3.3)
}

// InvoiceSuspendDate computes the date the 90/60/30/15/7/0 cascade counts
// down to for one due invoice. Exported so callers building the
// customer-facing suspended notice's "suspended on X" text can compute the
// same real anchor date DecideInvoice used, without duplicating this math.
//
// Without the grace period (usesGracePeriod false — EULA 3.4+, no primary
// partner), that's just the invoice due date, no adjustment.
//
// With it: the account must stay active for a minimum of 60 days from the
// invoice date. If the credit period (due date minus invoice date) is
// already longer than that, the real due date is used instead — suspending
// at day 60 would be unfair, since the invoice genuinely isn't due yet.
func InvoiceSuspendDate(invoiceDate, invoiceDueDate time.Time, eulaVersion float64, hasPrimaryPartner bool) time.Time {
	if !usesGracePeriod(eulaVersion, hasPrimaryPartner) {
		return invoiceDueDate
	}

	creditPeriodDays := invoiceDueDate.Sub(invoiceDate).Hours() / 24
	if creditPeriodDays > 60 {
		return invoiceDueDate
	}
	return invoiceDate.AddDate(0, 0, 60)
}
