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

package notifications

import (
	"strings"
	"testing"
)

func TestSetLogoURLs_ReplacesEveryBakedLogo(t *testing.T) {
	tmpls := logoTemplates()
	orig := make([]string, len(tmpls))
	for i, p := range tmpls {
		orig[i] = *p
	}
	t.Cleanup(func() {
		for i, p := range tmpls {
			*p = orig[i]
		}
	})

	SetLogoURLs("https://example.test/light.png", "https://example.test/dark.png")

	if !strings.Contains(outageNotificationTemplate, "https://example.test/light.png") ||
		strings.Contains(outageNotificationTemplate, wso2LogoURL) {
		t.Errorf("light logo not replaced")
	}
	if !strings.Contains(projectContactRegisteredTemplate, "https://example.test/dark.png") ||
		strings.Contains(projectContactRegisteredTemplate, wso2LogoWhiteURL) {
		t.Errorf("white logo not replaced")
	}
}

func TestSetLogoURLs_EmptyKeepsDefaults(t *testing.T) {
	before := outageNotificationTemplate
	SetLogoURLs("  ", "")
	if outageNotificationTemplate != before {
		t.Errorf("empty values must keep the default logos")
	}
}

func TestSetFooterIconURL_ReplacesEveryBakedIcon(t *testing.T) {
	tmpls := footerIconTemplates()
	orig := make([]string, len(tmpls))
	for i, p := range tmpls {
		orig[i] = *p
	}
	t.Cleanup(func() {
		for i, p := range tmpls {
			*p = orig[i]
		}
	})

	SetFooterIconURL("https://example.test/icon.png")

	if !strings.Contains(projectContactRegisteredTemplate, "https://example.test/icon.png") ||
		strings.Contains(projectContactRegisteredTemplate, wso2FooterIconURL) {
		t.Errorf("footer icon not replaced")
	}
}

func TestSetFooterIconURL_EmptyKeepsDefault(t *testing.T) {
	before := projectContactRegisteredTemplate
	SetFooterIconURL("  ")
	if projectContactRegisteredTemplate != before {
		t.Errorf("empty value must keep the default footer icon")
	}
}

func TestSetTemplateCompanyName_ReplacesEveryBakedPhrase(t *testing.T) {
	tmpls := companyNameTemplates()
	orig := make([]string, len(tmpls))
	for i, p := range tmpls {
		orig[i] = *p
	}
	t.Cleanup(func() {
		for i, p := range tmpls {
			*p = orig[i]
		}
		companyName = defaultCompanyName
		companyNameHTML = defaultCompanyName
	})

	SetTemplateCompanyName("Acme")

	if CompanyName() != "Acme" {
		t.Errorf("CompanyName() = %q, want %q", CompanyName(), "Acme")
	}
	if !strings.Contains(caseCreatedTemplate, "This message was sent by Acme's support system.") ||
		strings.Contains(caseCreatedTemplate, "WSO2's support system") {
		t.Errorf("footer disclaimer not replaced in caseCreatedTemplate")
	}
	if !strings.Contains(projectReactivatedTemplate, "This automated message was sent by Acme's support system.") {
		t.Errorf("reactivated-email disclaimer not replaced")
	}
	if !strings.Contains(projectReactivatedTemplate, "Best Regards,<br>Acme Team") {
		t.Errorf("Best Regards signoff not replaced")
	}
	if !strings.Contains(projectContactRegisteredTemplate, "Cheers!<br>The Acme Team") {
		t.Errorf("Cheers signoff not replaced")
	}
	// Note: every template's raw file also carries an Apache license header
	// ("Copyright (c) 2026 WSO2 LLC. ...") as a leading HTML comment, which
	// also contains the literal text "WSO2 LLC" — unrelated to the footer
	// copyright line and never touched by SetTemplateCompanyName. So these
	// checks assert the specific footer phrase, not a blanket absence of
	// "WSO2 LLC" anywhere in the template.
	if !strings.Contains(projectContactRegisteredTemplate, "&copy; 2026 Acme LLC. All rights reserved.") {
		t.Errorf("copyright line not replaced in projectContactRegisteredTemplate")
	}
	if !strings.Contains(casePagingTemplate, "&copy; <!-- [YEAR] --> Acme LLC. All rights reserved.") {
		t.Errorf("copyright line (YEAR-placeholder variant) not replaced in casePagingTemplate")
	}
	if !strings.Contains(outageNotificationTemplate, "&copy; <!-- [YEAR] --> Acme LLC. All Rights Reserved.") {
		t.Errorf("copyright line (capitalized variant) not replaced in outageNotificationTemplate")
	}
	// Must not touch the logo alt text or "... Support" titles — only the
	// disclaimer/signoff/copyright phrases above.
	if !strings.Contains(projectContactRegisteredTemplate, `alt="WSO2"`) {
		t.Errorf("unrelated bare \"WSO2\" occurrence (logo alt text) must be left untouched")
	}
}

func TestSetTemplateCompanyName_EmptyKeepsDefault(t *testing.T) {
	before := caseCreatedTemplate
	SetTemplateCompanyName("  ")
	if caseCreatedTemplate != before {
		t.Errorf("empty value must keep the default company name")
	}
	if CompanyName() != defaultCompanyName {
		t.Errorf("CompanyName() = %q, want default %q", CompanyName(), defaultCompanyName)
	}
}
