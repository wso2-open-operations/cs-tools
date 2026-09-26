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

	"github.com/wso2-open-operations/cs-tools/integrations/acp-closure-service/internal/recipients"
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
		Subject:     "subject",
		Body:        invoiceReminderBody(),
		InvoiceSfID: "a0IE2000006XBu5MAG",
		Recipients:  Recipients{AccountOwner: recipients.Contact{Email: "am@wso2.com"}},
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
		Subject:    "subject",
		Body:       invoiceReminderBody(),
		Recipients: Recipients{AccountOwner: recipients.Contact{Email: "am@wso2.com"}},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	got := sender.calls[0].htmlBody
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
		Subject:     "subject",
		Body:        "Your invoice US12345 is due.",
		InvoiceSfID: "a0IE2000006XBu5MAG",
		Recipients: Recipients{
			AccountOwner: recipients.Contact{Email: "am@wso2.com"},
			Customer:     &recipients.Contact{Email: "customer@wso2.com"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if got := sender.calls[0].htmlBody; strings.Contains(got, "Open in Salesforce") || strings.Contains(got, "salesforce.com") {
		t.Errorf("customer-facing htmlBody must never contain a Salesforce link; got: %s", got)
	}
}
