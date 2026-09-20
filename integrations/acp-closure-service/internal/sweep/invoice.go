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

package sweep

import (
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/closure"
)

// dueInvoice holds the invoice-specific fields the invoice-based notice
// bodies need. No invoice-fetching client exists yet (blocked on the
// Opportunity/Invoice API work requested in epic #3004) — this is kept as
// a small local struct so the notice-content work below can be written and
// tested now, independent of that. Whatever eventually resolves this from
// the real API is expected to populate it.
type dueInvoice struct {
	ID          string
	Opportunity string
	// DueDate is the invoice's own due date — always what's shown in the
	// "Due Date:" field, in both the internal and customer-facing bodies,
	// regardless of any grace period.
	DueDate time.Time
	// SuspendDate is the actual date the invoice cascade suspends on —
	// closure.invoiceSuspendDate's result, which can be later than DueDate
	// under the EULA-3.3-style 60-day grace period. Equals DueDate when no
	// grace period applies. Only the customer-facing suspended (0-day)
	// body's prose references this — the internal body and the "Due Date:"
	// field everywhere else always show the real DueDate.
	SuspendDate time.Time
}

// customerInvoiceNoticeSubject builds the invoice-based customer-facing
// notice's subject line (15/7/0 only, same audience windows as
// subscription-based — see needsCustomerAudience). The terminal (0-day)
// text is identical to customerNoticeSubject's, confirmed against
// actual_0_days_invoice_email_customer.html — but the reminder
// (non-terminal) text is a genuinely different phrase, confirmed against
// actual_7_days_invoice_email_cutomer.html: "Payment Reminder of Project",
// not "Upcoming Project Suspension Notice". The same reminder text covers
// both 15-day and 7-day, same as customerNoticeSubject's own reminder
// branch covers both without varying by exact window.
func customerInvoiceNoticeSubject(window closure.NoticeWindow, projectName string) string {
	if window.IsTerminal() {
		return fmt.Sprintf("Project Suspension Notice - %s", projectName)
	}
	return fmt.Sprintf("Payment Reminder of Project - %s.", projectName)
}

// internalInvoiceReminderBodyTemplate is the day-count (90/60/30/15/7)
// invoice-based internal notice body, confirmed verbatim against
// actual_90_days_invoice_email.png. Same project fields as
// internalReminderBodyTemplate (%[1]s-%[5]s), plus 3 invoice-specific
// fields (%[6]s-%[8]s) that notify.renderInternalInvoiceEmailHTML renders
// in their own nested box.
const internalInvoiceReminderBodyTemplate = `Dear %[1]s

The following project has an upcoming due invoice. Please find the details below.

Project Name: %[2]s

Project Key: %[3]s

Account Owner: %[1]s

Start Date: %[4]s

End Date: %[5]s

Invoice Id: %[6]s

Opportunity: %[7]s

Due Date: %[8]s

Since projects need to be due on the due date of each invoice, kindly take the remedial actions to avoid any disruptions of subscription support. We appreciate your understanding and your prompt attention to this matter.

Best Regards,
WSO2 Team`

// internalInvoiceSuspensionBodyTemplate is the day-0 invoice-based internal
// notice body, confirmed verbatim against actual_0_days_invoice_email.html
// — past tense ("has been suspended due to unpaid invoices"), same field
// shape as internalInvoiceReminderBodyTemplate otherwise.
const internalInvoiceSuspensionBodyTemplate = `Dear %[1]s

The following project has been suspended due to unpaid invoices. Kindly request that you take the appropriate action to reinitiate the suspended support account.

Project Name: %[2]s

Project Key: %[3]s

Account Owner: %[1]s

Start Date: %[4]s

End Date: %[5]s

Invoice Id: %[6]s

Opportunity: %[7]s

Due Date: %[8]s

Since the project is suspended, kindly take the remedial actions to reinstate the subscription support. We appreciate your prompt attention to this matter.

Best Regards,
WSO2 Team`

// internalInvoiceNoticeBody builds the invoice-based internal notice body.
// The internal subject line needs no invoice-specific variant — confirmed
// identical to internalNoticeSubject's text — so only the body differs
// here; callers use the existing internalNoticeSubject directly.
func internalInvoiceNoticeBody(window closure.NoticeWindow, proj project, accountOwnerName string, invoice dueInvoice) string {
	template := internalInvoiceReminderBodyTemplate
	if window.IsTerminal() {
		template = internalInvoiceSuspensionBodyTemplate
	}
	return fmt.Sprintf(template,
		accountOwnerName, proj.Name, proj.ProjectKey, formatDate(proj.StartDate), formatDate(proj.EndDate),
		invoice.ID, invoice.Opportunity, formatDate(&invoice.DueDate))
}

// customerInvoicePaymentReminderBodyTemplate is the 15/7-day customer-facing
// invoice body, confirmed verbatim against
// actual_7_days_invoice_email_cutomer.html — covers both windows, same
// template, no day-count-specific wording, mirroring how
// customerUpcomingSuspensionBodyTemplate already covers both 15 and 7 for
// subscription-based.
const customerInvoicePaymentReminderBodyTemplate = `This is a gentle reminder that payment for your WSO2 subscription associated with the project %[1]s which is due on %[2]s if not settled. To ensure uninterrupted access to your support services, we kindly request that the invoice be settled on or before the due date.

We understand that delays can occasionally occur, and we are here to assist you in resolving any issues related to this payment. If you have any questions or require assistance, please do not hesitate to reach out to your WSO2 Account Manager. If the payment has already been made, we sincerely apologize for any inconvenience. Kindly notify us by sending the payment details, including the payment date to billing@wso2.com so we can update our records.

Your cooperation is greatly appreciated in ensuring the continued, uninterrupted WSO2 services. We greatly appreciate your prompt attention to this matter and your continued partnership with WSO2.

Best Regards,
WSO2 Team`

// customerInvoiceSuspendedBodyTemplate is the day-0 customer-facing invoice
// body, confirmed verbatim against actual_0_days_invoice_email_customer.html
// — past tense, references SuspendDate (the actual grace-period-adjusted
// suspend date), not the raw invoice due date.
const customerInvoiceSuspendedBodyTemplate = `Your WSO2 subscription for %[1]s(%[2]s) is currently past due. As a result, your project was suspended on %[3]s and support portal has been temporarily paused. This also means your license to access product updates and the WSO2 support portal has been temporarily paused.

To restore your services, please complete the outstanding payment and send your confirmation to billing@wso2.com with the relevant payment information to ensure a swift resolution.
If you have already settled this invoice, please provide the transaction details so we can reactivate your account immediately.

We appreciate your prompt attention to this matter. Our goal is to ensure the continuity of your WSO2 services, and we remain committed to assisting you.

Best Regards,
WSO2 Team`

// customerInvoiceNoticeBody builds the invoice-based customer-facing notice
// body.
func customerInvoiceNoticeBody(window closure.NoticeWindow, proj project, invoice dueInvoice) string {
	if window.IsTerminal() {
		return fmt.Sprintf(customerInvoiceSuspendedBodyTemplate, proj.Name, proj.ProjectKey, formatDateUS(&invoice.SuspendDate))
	}
	return fmt.Sprintf(customerInvoicePaymentReminderBodyTemplate, proj.Name, formatDateUS(&invoice.DueDate))
}
