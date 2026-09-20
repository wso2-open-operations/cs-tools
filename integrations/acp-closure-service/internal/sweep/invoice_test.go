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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/closure"
)

func testInvoiceProject() project {
	start := time.Date(2025, 9, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, 9, 9, 0, 0, 0, 0, time.UTC)
	return project{
		Name:       "C&S Wholesale Grocers - Subscription",
		ProjectKey: "CSWHOLESALESUB",
		StartDate:  &start,
		EndDate:    &end,
	}
}

func testDueInvoice() dueInvoice {
	due := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	return dueInvoice{
		ID:          "US20268897",
		Opportunity: "C&S - APIM/IS - Y3",
		DueDate:     due,
		SuspendDate: due,
	}
}

// TestCustomerInvoiceNoticeSubject_TerminalMatchesSubscriptionBased confirms
// the one place invoice and subscription customer subjects genuinely
// coincide: the terminal (0-day) text, confirmed identical against
// actual_0_days_invoice_email_customer.html.
func TestCustomerInvoiceNoticeSubject_TerminalMatchesSubscriptionBased(t *testing.T) {
	got := customerInvoiceNoticeSubject(closure.NoticeWindow0, "UAB - EI - Subscription")
	want := customerNoticeSubject(closure.NoticeWindow0, "UAB - EI - Subscription")
	if got != want {
		t.Errorf("customerInvoiceNoticeSubject(terminal) = %q, want %q (same as customerNoticeSubject)", got, want)
	}
	if got != "Project Suspension Notice - UAB - EI - Subscription" {
		t.Errorf("got %q, want the exact confirmed text", got)
	}
}

// TestCustomerInvoiceNoticeSubject_ReminderDiffersFromSubscriptionBased
// covers the one genuine divergence: the reminder (non-terminal) subject is
// a different phrase entirely from subscription-based's "Upcoming Project
// Suspension Notice", confirmed against actual_7_days_invoice_email_cutomer.html
// ("Payment Reminder of Project - X.", trailing period included).
func TestCustomerInvoiceNoticeSubject_ReminderDiffersFromSubscriptionBased(t *testing.T) {
	got := customerInvoiceNoticeSubject(closure.NoticeWindow7, "Robert Bosch Krankenhaus GmbH - Subscription")
	want := "Payment Reminder of Project - Robert Bosch Krankenhaus GmbH - Subscription."
	if got != want {
		t.Errorf("customerInvoiceNoticeSubject(reminder) = %q, want %q", got, want)
	}

	subscriptionSubject := customerNoticeSubject(closure.NoticeWindow7, "Robert Bosch Krankenhaus GmbH - Subscription")
	if got == subscriptionSubject {
		t.Error("invoice reminder subject should not match the subscription-based one — they're confirmed different phrases")
	}
}

// TestInternalInvoiceNoticeBody covers both windows of the internal
// invoice-based body, confirmed verbatim against actual_90_days_invoice_email.png
// (reminder) and actual_0_days_invoice_email.html (suspension) — same
// project fields as the subscription-based body, plus the 3 invoice fields
// that notify.renderInternalInvoiceEmailHTML renders in a nested box.
func TestInternalInvoiceNoticeBody(t *testing.T) {
	proj := testInvoiceProject()
	invoice := testDueInvoice()

	tests := []struct {
		name              string
		window            closure.NoticeWindow
		wantIntroContains string
	}{
		{
			name:              "reminder window: upcoming due invoice wording",
			window:            closure.NoticeWindow90,
			wantIntroContains: "The following project has an upcoming due invoice.",
		},
		{
			name:              "terminal window: suspended due to unpaid invoices wording",
			window:            closure.NoticeWindow0,
			wantIntroContains: "The following project has been suspended due to unpaid invoices.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := internalInvoiceNoticeBody(tt.window, proj, "Lochana De Alwis", invoice)

			if !strings.Contains(got, tt.wantIntroContains) {
				t.Errorf("body missing intro %q\ngot: %s", tt.wantIntroContains, got)
			}
			if !strings.Contains(got, "Project Name: C&S Wholesale Grocers - Subscription") {
				t.Error("body missing Project Name field")
			}
			if !strings.Contains(got, "Project Key: CSWHOLESALESUB") {
				t.Error("body missing Project Key field")
			}
			if !strings.Contains(got, "Account Owner: Lochana De Alwis") {
				t.Error("body missing Account Owner field")
			}
			if !strings.Contains(got, "Start Date: 2025-09-10") {
				t.Error("body missing Start Date field")
			}
			if !strings.Contains(got, "End Date: 2027-09-09") {
				t.Error("body missing End Date field")
			}
			if !strings.Contains(got, "Invoice Id: US20268897") {
				t.Error("body missing Invoice Id field")
			}
			if !strings.Contains(got, "Opportunity: C&S - APIM/IS - Y3") {
				t.Error("body missing Opportunity field")
			}
			if !strings.Contains(got, "Due Date: 2026-10-10") {
				t.Error("body missing Due Date field")
			}
			if !strings.Contains(got, "Best Regards") || !strings.Contains(got, "WSO2 Team") {
				t.Error("body missing the sign-off")
			}

			paragraphs := strings.Split(got, "\n\n")
			if len(paragraphs) != 12 {
				t.Errorf("body has %d paragraphs, want 12 (matching notify.internalInvoiceBodyParagraphCount) — got:\n%s", len(paragraphs), got)
			}
		})
	}
}

// TestCustomerInvoiceNoticeBody covers both windows of the customer-facing
// invoice body, confirmed verbatim against actual_7_days_invoice_email_cutomer.html
// (reminder — also covers 15-day, same template, no day-count-specific
// wording) and actual_0_days_invoice_email_customer.html (suspended).
func TestCustomerInvoiceNoticeBody(t *testing.T) {
	proj := testInvoiceProject()
	invoice := testDueInvoice()

	t.Run("reminder window: gentle payment reminder wording", func(t *testing.T) {
		got := customerInvoiceNoticeBody(closure.NoticeWindow7, proj, invoice)

		if !strings.Contains(got, "This is a gentle reminder that payment for your WSO2 subscription associated with the project C&S Wholesale Grocers - Subscription which is due on 10/10/2026 if not settled.") {
			t.Errorf("body missing the reminder intro sentence with the correct project name/date, got: %s", got)
		}
		if !strings.Contains(got, "billing@wso2.com") {
			t.Error("body missing the billing@wso2.com contact reference")
		}
		if !strings.Contains(got, "Best Regards") || !strings.Contains(got, "WSO2 Team") {
			t.Error("body missing the sign-off")
		}
		// Reminder body must not mention suspension — it's a different
		// window's wording entirely.
		if strings.Contains(got, "suspended") {
			t.Error("reminder body should not mention suspension")
		}
	})

	t.Run("terminal window: past-due suspension wording", func(t *testing.T) {
		got := customerInvoiceNoticeBody(closure.NoticeWindow0, proj, invoice)

		if !strings.Contains(got, "Your WSO2 subscription for C&S Wholesale Grocers - Subscription(CSWHOLESALESUB) is currently past due.") {
			t.Errorf("body missing the suspended intro sentence with the correct project name/key, got: %s", got)
		}
		if !strings.Contains(got, "suspended on 10/10/2026") {
			t.Errorf("body missing the suspend date, got: %s", got)
		}
		if !strings.Contains(got, "billing@wso2.com") {
			t.Error("body missing the billing@wso2.com contact reference")
		}
		if !strings.Contains(got, "Best Regards") || !strings.Contains(got, "WSO2 Team") {
			t.Error("body missing the sign-off")
		}
	})
}
