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

// invoiceReminderBody is the invoice-based internal notice's real 12-
// paragraph shape, confirmed against actual_90_days_invoice_email.png and
// actual_0_days_invoice_email.html: greeting, intro, the same 5 project
// fields the subscription-based notice has, then 3 more invoice-specific
// fields (Invoice Id/Opportunity/Due Date) that render in their own nested
// box in the real reference examples, then closing, signoff.
func invoiceReminderBody() string {
	return strings.Join([]string{
		"Dear Jordan Perera",
		"The following project has an upcoming due invoice. Please find the details below.",
		"Project Name: Acme - Subscription",
		"Project Key: ACMESUB",
		"Account Owner: Jordan Perera",
		"Start Date: 2025-01-01",
		"End Date: 2026-01-01",
		"Invoice Id: US12345",
		"Opportunity: Acme - APIM - Y1",
		"Due Date: 2026-02-15",
		"Since projects need to be due on the due date of each invoice, kindly take the remedial actions to avoid any disruptions of subscription support. We appreciate your understanding and your prompt attention to this matter.",
		"Best Regards,\nWSO2 Team",
	}, "\n\n")
}

// TestEmailNotifier_Send_InternalInvoiceNoticeNestsInvoiceFields covers the
// one real structural difference invoice-based internal notices have from
// the subscription-based ones: Invoice Id/Opportunity/Due Date render in
// their own nested box inside the main detail box, not as three more flat
// rows alongside the project fields.
func TestEmailNotifier_Send_InternalInvoiceNoticeNestsInvoiceFields(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject: "subject",
		Body:    invoiceReminderBody(),
		Recipients: Recipients{
			AccountOwner: recipients.Contact{Email: "am@wso2.com"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("SendEmail calls = %d, want 1", len(sender.calls))
	}
	got := sender.calls[0].htmlBody

	// The project fields still render as before.
	if !strings.Contains(got, "Project Name: <strong>Acme - Subscription</strong>") {
		t.Error("htmlBody missing the structured Project Name field row")
	}
	if !strings.Contains(got, "End Date: <strong>2026-01-01</strong>") {
		t.Error("htmlBody missing the structured End Date field row")
	}

	// The invoice fields render too...
	if !strings.Contains(got, "Invoice Id: <strong>US12345</strong>") {
		t.Error("htmlBody missing the structured Invoice Id field row")
	}
	if !strings.Contains(got, "Due Date: <strong>2026-02-15</strong>") {
		t.Error("htmlBody missing the structured Due Date field row")
	}

	// ...inside their own nested box, not just more rows in the outer one.
	if !strings.Contains(got, "background-color:#ffffff;padding:12px 16px;margin-top:8px;") {
		t.Error("htmlBody missing the nested invoice-fields box styling — invoice fields should be visually nested inside the main detail box, matching the real reference examples")
	}

	if !strings.Contains(got, "Dear Jordan Perera") {
		t.Error("htmlBody missing the greeting")
	}
	if !strings.Contains(got, "Best Regards") || !strings.Contains(got, "WSO2 Team") {
		t.Error("htmlBody missing the sign-off")
	}
	if !strings.Contains(got, wso2LogoURL) {
		t.Error("htmlBody missing the WSO2 logo")
	}
}
