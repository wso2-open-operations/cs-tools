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

// noBusinessContactBody mirrors sweep.go's noBusinessContactBodyTemplate
// exactly (duplicated here rather than imported, same as this file's other
// body-shape helpers — notify must not depend on sweep) — the real
// 5-paragraph shape, with the field lines joined by single "\n" in the
// final paragraph rather than one-per-paragraph like the other notices.
func noBusinessContactBody() string {
	return strings.Join([]string{
		"Internal - Customer Project without Business Contacts",
		"Urgent reminder regarding the project CIB IAM (B2C) - Subscription.",
		"Please note that no Business Contacts was found for the project CIB IAM (B2C) - Subscription. Immediate action is required to address this issue, as without a business contact, the customers will not receive essential notifications regarding their project suspension status. Additionally, this will lead to failures in further automated actions related to project suspension.",
		"Please refer this document to Update Business Contact",
		"Project Name: CIB IAM (B2C) - Subscription\nProject Key: CIBSUB\nAccount Owner: Hasanthi Weerasinghe\nStart Date: 2026-09-01\nEnd Date: 2029-08-31",
	}, "\n\n")
}

// TestEmailNotifier_Send_NoBusinessContactMatchesRealTemplate covers the
// dedicated styling for this notice, confirmed against the real reference
// (business_contact_email.png): a bold red heading above the card, a bold
// blue intro line, the project name bolded and two warning clauses in bold
// red italic within the body paragraph, a real hyperlink to the business
// contact doc, and the same structured field-detail box used elsewhere.
func TestEmailNotifier_Send_NoBusinessContactMatchesRealTemplate(t *testing.T) {
	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject:    "subject",
		Body:       noBusinessContactBody(),
		Recipients: Recipients{AccountOwner: recipients.Contact{Email: "am@wso2.com"}},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("SendEmail calls = %d, want 1", len(sender.calls))
	}
	got := sender.calls[0].htmlBody

	if !strings.Contains(got, "Internal - Customer Project without Business Contacts") {
		t.Error("htmlBody missing the red heading text")
	}
	if !strings.Contains(got, "color:#c62828") {
		t.Error("htmlBody missing the red styling used for the heading and warning clauses")
	}
	if !strings.Contains(got, "Urgent reminder regarding the project CIB IAM (B2C) - Subscription.") {
		t.Error("htmlBody missing the blue intro line")
	}
	if !strings.Contains(got, "<strong>CIB IAM (B2C) - Subscription</strong>") {
		t.Error("htmlBody missing the bolded project name in the warning paragraph")
	}
	if !strings.Contains(got, "customers will not receive essential notifications regarding their project suspension status") {
		t.Error("htmlBody missing the first warning clause")
	}
	if !strings.Contains(got, "failures in further automated actions related to project suspension") {
		t.Error("htmlBody missing the second warning clause")
	}
	if !strings.Contains(got, `href="https://docs.google.com/document/d/1xv-n6XK7E60QZdEb7DrbpDW0UmXs-SqLceLGaqPx68k/edit?tab=t.0#heading=h.58epxafkaoe"`) {
		t.Error("htmlBody missing the real hyperlink to the business contact doc")
	}
	if !strings.Contains(got, ">Update Business Contact<") {
		t.Error("htmlBody missing the hyperlink's visible text")
	}
	if !strings.Contains(got, "Project Name: <strong>CIB IAM (B2C) - Subscription</strong>") {
		t.Error("htmlBody missing the structured Project Name field row")
	}
	if !strings.Contains(got, "Project Key: <strong>CIBSUB</strong>") {
		t.Error("htmlBody missing the structured Project Key field row")
	}
	if !strings.Contains(got, "Account Owner: <strong>Hasanthi Weerasinghe</strong>") {
		t.Error("htmlBody missing the structured Account Owner field row")
	}
	if !strings.Contains(got, "End Date: <strong>2029-08-31</strong>") {
		t.Error("htmlBody missing the structured End Date field row")
	}
	if !strings.Contains(got, wso2LogoURL) {
		t.Error("htmlBody missing the WSO2 logo")
	}
	if !strings.Contains(got, "This automated message was sent by WSO2's support system. Please do not reply to this email.") {
		t.Error("htmlBody missing the standard footer disclaimer")
	}
}
