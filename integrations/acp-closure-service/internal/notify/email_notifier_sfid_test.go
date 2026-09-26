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

// TestEmailNotifier_Send_InternalNoticeLinksProjectNameWhenSfIDPresent
// covers the real reference email's one structural detail this port was
// missing entirely: local-docs/actual_0_days_invoice_email.html shows the
// internal notice's "Project Name" value as a hyperlink to
// https://wso2.my.salesforce.com/{sfId}, not plain text. ProjectSfID
// present on the Notice must produce that same link.
func TestEmailNotifier_Send_InternalNoticeLinksProjectNameWhenSfIDPresent(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject:     "subject",
		Body:        internalReminderBody(),
		ProjectSfID: "a0d4U00000aUJURQA4",
		Recipients: Recipients{
			AccountOwner: recipients.Contact{Email: "am@wso2.com"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	got := sender.calls[0].htmlBody

	want := `Project Name: <a href="https://wso2.my.salesforce.com/a0d4U00000aUJURQA4" style="color:#2c66bd;font-weight:700;text-decoration:none;" target="_blank">Acme - Subscription</a>`
	if !strings.Contains(got, want) {
		t.Errorf("htmlBody missing the Project Name Salesforce link.\nwant substring: %s\ngot: %s", want, got)
	}
	// The unlinked plain-bold rendering must be gone once a link is used —
	// otherwise the field would render twice.
	if strings.Contains(got, "Project Name: <strong>Acme - Subscription</strong>") {
		t.Error("htmlBody still has the old plain-text Project Name row alongside the link")
	}
}

// TestEmailNotifier_Send_InternalNoticeProjectNameStaysPlainWhenSfIDAbsent
// is the regression guard: a project with no Salesforce ID on file (empty
// ProjectSfID, the zero value — the common case for every existing test
// that doesn't set it) must keep the exact plain-bold rendering this port
// already had, not fail or omit the field.
func TestEmailNotifier_Send_InternalNoticeProjectNameStaysPlainWhenSfIDAbsent(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject: "subject",
		Body:    internalReminderBody(),
		Recipients: Recipients{
			AccountOwner: recipients.Contact{Email: "am@wso2.com"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	got := sender.calls[0].htmlBody

	if !strings.Contains(got, "Project Name: <strong>Acme - Subscription</strong>") {
		t.Error("htmlBody should keep the plain-bold Project Name row when ProjectSfID is empty")
	}
	if strings.Contains(got, "salesforce.com") {
		t.Error("htmlBody should not contain any Salesforce link when ProjectSfID is empty")
	}
}

// TestEmailNotifier_Send_InvoiceNoticeLinksProjectNameWhenSfIDPresent covers
// the invoice-based internal notice's own project-fields box, which uses
// the same field-row rendering as the subscription-based one.
func TestEmailNotifier_Send_InvoiceNoticeLinksProjectNameWhenSfIDPresent(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject:     "subject",
		Body:        invoiceReminderBody(),
		ProjectSfID: "a0d4U00000aUJURQA4",
		Recipients: Recipients{
			AccountOwner: recipients.Contact{Email: "am@wso2.com"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	got := sender.calls[0].htmlBody

	want := `Project Name: <a href="https://wso2.my.salesforce.com/a0d4U00000aUJURQA4"`
	if !strings.Contains(got, want) {
		t.Errorf("htmlBody missing the Project Name Salesforce link in the invoice notice.\nwant substring: %s\ngot: %s", want, got)
	}
	// The invoice-specific fields (Invoice Id/Opportunity/Due Date) are
	// never linked — only the project's own name is.
	if strings.Contains(got, `Invoice Id: <a href`) {
		t.Error("Invoice Id should never be rendered as a Salesforce link")
	}
}

// TestEmailNotifier_Send_NoBusinessContactNoticeLinksProjectNameWhenSfIDPresent
// covers the no-business-contact notice's own, differently-shaped field
// parsing (label lines joined by "\n" inside one paragraph, not one
// paragraph per field).
func TestEmailNotifier_Send_NoBusinessContactNoticeLinksProjectNameWhenSfIDPresent(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject:     "subject",
		Body:        noBusinessContactBody(),
		ProjectSfID: "a0d4U00000aUJURQA4",
		Recipients:  Recipients{AccountOwner: recipients.Contact{Email: "am@wso2.com"}},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	got := sender.calls[0].htmlBody

	want := `Project Name: <a href="https://wso2.my.salesforce.com/a0d4U00000aUJURQA4"`
	if !strings.Contains(got, want) {
		t.Errorf("htmlBody missing the Project Name Salesforce link in the no-business-contact notice.\nwant substring: %s\ngot: %s", want, got)
	}
	// The bolded project-name mention in the warning paragraph (a
	// different occurrence from the field row) must stay plain text, not
	// also become a link — only the structured field row links.
	if !strings.Contains(got, "project <strong>CIB IAM (B2C) - Subscription</strong>. Immediate action") {
		t.Error("the warning paragraph's project name mention should remain plain bold text, not a link")
	}
}

// TestEmailNotifier_Send_CustomerNoticeNeverLinksProjectName confirms the
// Salesforce link is internal-only, matching the real reference: the
// customer-facing email has no Salesforce link anywhere (customers have no
// Salesforce access). Customer notices go through renderEmailHTML, which
// never touches fieldRowHTML at all, but this guards against a future
// refactor accidentally sharing that logic.
func TestEmailNotifier_Send_CustomerNoticeNeverLinksProjectName(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject:     "subject",
		Body:        "Some customer-facing body mentioning Acme - Subscription.",
		ProjectSfID: "a0d4U00000aUJURQA4",
		Recipients: Recipients{
			Customer: &recipients.Contact{Email: "customer@example.com"},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	got := sender.calls[0].htmlBody

	if strings.Contains(got, "salesforce.com") {
		t.Error("customer-facing htmlBody must never contain a Salesforce link")
	}
}
