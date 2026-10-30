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

package notify

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/recipients"
)

// TestEmailNotifier_Send_InvoiceNoticeLinksOpenInSalesforceWhenInvoiceSfIDPresent
// covers the real internal invoice notice's "Open in Salesforce" link
// (local-docs/actual_0_days_invoice_email.html): it sits inside the invoice
// box and points at the invoice's own Salesforce record (an a0I... ID),
// separate from the Project Name link, which points at the project's.
func TestEmailNotifier_Send_InvoiceNoticeLinksOpenInSalesforceWhenInvoiceSfIDPresent(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject:      "subject",
		Body:         invoiceReminderBody(),
		InvoiceSfIDs: []string{"a0IE2000006XBu5MAG"},
		Recipients:   Recipients{AccountOwner: recipients.Contact{Email: "am@wso2.com"}},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	got := sender.calls[0].htmlBody

	want := `<a href="https://wso2.my.salesforce.com/a0IE2000006XBu5MAG" style="color:#2c66bd;font-weight:700;text-decoration:none;" target="_blank">Open in Salesforce</a>`
	if !strings.Contains(got, want) {
		t.Errorf("htmlBody missing the invoice's Open in Salesforce link.\nwant substring: %s\ngot: %s", want, got)
	}
	// The invoice fields themselves stay plain text; the link is separate.
	if !strings.Contains(got, "Invoice Id: <strong>US12345</strong>") {
		t.Error("Invoice Id row should stay plain bold text alongside the separate link")
	}
}

// TestEmailNotifier_Send_InvoiceNoticeHasNoOpenInSalesforceWhenInvoiceSfIDAbsent
// is the regression guard: an invoice with no Salesforce ID on file gets no
// link at all, rather than a link to an empty or broken record URL.
func TestEmailNotifier_Send_InvoiceNoticeHasNoOpenInSalesforceWhenInvoiceSfIDAbsent(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject: "subject",
		Body:    invoiceReminderBody(),
		// The real shape of an invoice with no Salesforce ID: one empty entry.
		InvoiceSfIDs: []string{""},
		Recipients:   Recipients{AccountOwner: recipients.Contact{Email: "am@wso2.com"}},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	got := sender.calls[0].htmlBody
	if !strings.Contains(got, invoiceBoxMarker) {
		t.Errorf("invoice notice without a Salesforce ID should still render its invoice box; got: %s", got)
	}
	if strings.Contains(got, "Open in Salesforce") || strings.Contains(got, "salesforce.com") {
		t.Errorf("htmlBody should have no Salesforce link when InvoiceSfID is empty; got: %s", got)
	}
}

// TestEmailNotifier_Send_CustomerNoticeNeverGetsOpenInSalesforce confirms
// customers, who have no Salesforce access, never get the link: the
// customer-facing shell ignores InvoiceSfID even if it were set.
func TestEmailNotifier_Send_CustomerNoticeNeverGetsOpenInSalesforce(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject:      "subject",
		Body:         "Your invoice US12345 is due.",
		InvoiceSfIDs: []string{"a0IE2000006XBu5MAG"},
		Recipients: Recipients{
			AccountOwner: recipients.Contact{Email: "am@wso2.com"},
			Customers:    []recipients.Contact{{Email: "customer@wso2.com"}},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if got := sender.calls[0].htmlBody; strings.Contains(got, "Open in Salesforce") || strings.Contains(got, "salesforce.com") {
		t.Errorf("customer-facing htmlBody must never contain a Salesforce link; got: %s", got)
	}
}

// TestEmailNotifier_Send_InvoiceNoticeRendersOneBoxPerInvoice covers the
// multi-invoice internal notice: each Invoice Id/Opportunity/Due Date triple
// in the body gets its own box with its own "Open in Salesforce" link, in
// order, as in the real legacy email (actual_invoice_email.png).
func TestEmailNotifier_Send_InvoiceNoticeRendersOneBoxPerInvoice(t *testing.T) {
	body := strings.Replace(invoiceReminderBody(),
		"Due Date: 2026-02-15",
		"Due Date: 2026-02-15\n\nInvoice Id: US99999\n\nOpportunity: Acme - Expansion\n\nDue Date: 2026-03-01", 1)
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject:      "subject",
		Body:         body,
		InvoiceSfIDs: []string{"a0I-first", "a0I-second"},
		Recipients:   Recipients{AccountOwner: recipients.Contact{Email: "am@wso2.com"}},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	got := sender.calls[0].htmlBody

	first := strings.Index(got, "Invoice Id: <strong>US12345</strong>")
	firstLink := strings.Index(got, "https://wso2.my.salesforce.com/a0I-first")
	second := strings.Index(got, "Invoice Id: <strong>US99999</strong>")
	secondLink := strings.Index(got, "https://wso2.my.salesforce.com/a0I-second")
	if first < 0 || firstLink < 0 || second < 0 || secondLink < 0 {
		t.Fatalf("htmlBody missing an invoice or its link\ngot: %s", got)
	}
	if !(first < firstLink && firstLink < second && second < secondLink) {
		t.Errorf("each invoice box should carry its own link, in order\ngot: %s", got)
	}
	if strings.Count(got, "Open in Salesforce") != 2 {
		t.Errorf("want 2 Open in Salesforce links, got %d", strings.Count(got, "Open in Salesforce"))
	}
	if strings.Contains(got, "Since projects need to be due") == false {
		t.Error("closing paragraph missing: the multi-invoice body wasn't recognised as an invoice notice")
	}
}

// TestEmailNotifier_Send_SubscriptionNoticeNeverUsesInvoiceLayout guards the
// renderer's shape detection (CodeRabbit, PR #2085): a subscription notice
// whose project name contains blank lines can have the same paragraph count
// as a two-invoice notice. Without invoice links on the notice it must never
// be drawn with invoice boxes.
func TestEmailNotifier_Send_SubscriptionNoticeNeverUsesInvoiceLayout(t *testing.T) {
	body := strings.Replace(internalReminderBody(),
		"Project Name: Acme - Subscription",
		"Project Name: Acme\n\nA\n\nB\n\nC\n\nD\n\nE\n\nF", 1) // 9 + 6 = 15 paragraphs
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	if _, err := n.Send(context.Background(), Notice{
		Subject:    "subject",
		Body:       body,
		Recipients: Recipients{AccountOwner: recipients.Contact{Email: "am@wso2.com"}},
	}); err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if n := len(strings.Split(body, "\n\n")); n != 15 {
		t.Fatalf("test setup: body has %d paragraphs, want 15 (a two-invoice shape)", n)
	}
	if got := sender.calls[0].htmlBody; strings.Contains(got, invoiceBoxMarker) {
		t.Errorf("subscription notice was rendered with invoice boxes\ngot: %s", got)
	}
}

// invoiceBoxMarker is a style fragment that only invoiceBoxHTML emits.
const invoiceBoxMarker = "background-color:#ffffff;padding:12px 16px;margin-top:8px;"
