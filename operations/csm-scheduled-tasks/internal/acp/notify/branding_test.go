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

func TestSetLogoURL_And_SetCompanyName_OverrideCustomerNotice(t *testing.T) {
	t.Cleanup(func() {
		logoURL = wso2LogoURL
		companyName = defaultCompanyName
	})

	SetLogoURL("https://example.test/logo.png")
	SetCompanyName("Acme")

	sender := &mockEmailSender{}
	n := &EmailNotifier{Sender: sender, Logger: discardLogger(), AllowNonWSO2Recipients: true}

	_, err := n.Send(context.Background(), Notice{
		Subject: "subject",
		Body:    "body text",
		Recipients: Recipients{
			AccountOwner: recipients.Contact{Email: "am@wso2.com"},
			Customers:    []recipients.Contact{{Email: "customer@wso2.com"}},
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want nil", err)
	}
	got := sender.calls[0].htmlBody

	if !strings.Contains(got, `src="https://example.test/logo.png"`) {
		t.Error("htmlBody missing the configured logo URL")
	}
	if !strings.Contains(got, `alt="Acme"`) {
		t.Error("htmlBody missing the configured company name as logo alt text")
	}
	if !strings.Contains(got, "This automated message was sent by Acme's support system.") {
		t.Error("htmlBody missing the configured company name in the footer disclaimer")
	}
	if strings.Contains(got, wso2LogoURL) {
		t.Error("htmlBody still carries the default logo URL after SetLogoURL")
	}
}

func TestSetLogoURL_And_SetCompanyName_OverrideInternalNotice(t *testing.T) {
	t.Cleanup(func() {
		logoURL = wso2LogoURL
		companyName = defaultCompanyName
	})

	SetLogoURL("https://example.test/logo.png")
	SetCompanyName("Acme")

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

	if !strings.Contains(got, `src="https://example.test/logo.png"`) {
		t.Error("htmlBody missing the configured logo URL")
	}
	if !strings.Contains(got, `alt="Acme"`) {
		t.Error("htmlBody missing the configured company name as logo alt text")
	}
	if !strings.Contains(got, "This automated message was sent by Acme's support system.") {
		t.Error("htmlBody missing the configured company name in the footer disclaimer")
	}
}

func TestSetLogoURL_EmptyKeepsDefault(t *testing.T) {
	before := logoURL
	SetLogoURL("  ")
	if logoURL != before {
		t.Errorf("empty value must keep the default logo")
	}
}

func TestSetCompanyName_EmptyKeepsDefault(t *testing.T) {
	before := companyName
	SetCompanyName("  ")
	if companyName != before {
		t.Errorf("empty value must keep the current company name")
	}
	if CompanyName() != defaultCompanyName {
		t.Errorf("CompanyName() = %q, want default %q", CompanyName(), defaultCompanyName)
	}
}
