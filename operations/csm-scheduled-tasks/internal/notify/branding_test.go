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
	"strings"
	"testing"
)

func TestSetLogoURL_ReplacesEveryBakedLogo(t *testing.T) {
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

	SetLogoURL("https://example.test/logo.png")

	if !strings.Contains(alertTemplate, "https://example.test/logo.png") || strings.Contains(alertTemplate, wso2LogoURL) {
		t.Errorf("logo not replaced in alertTemplate")
	}
	if !strings.Contains(worklogsReportTemplate, "https://example.test/logo.png") || strings.Contains(worklogsReportTemplate, wso2LogoURL) {
		t.Errorf("logo not replaced in worklogsReportTemplate")
	}
}

func TestSetLogoURL_EmptyKeepsDefault(t *testing.T) {
	before := alertTemplate
	SetLogoURL("  ")
	if alertTemplate != before {
		t.Errorf("empty value must keep the default logo")
	}
}

func TestSetCompanyName_ReplacesEveryBakedPhrase(t *testing.T) {
	tmpls := logoTemplates()
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

	SetCompanyName("Acme")

	if CompanyName() != "Acme" {
		t.Errorf("CompanyName() = %q, want %q", CompanyName(), "Acme")
	}
	if !strings.Contains(alertTemplate, "&copy; <!-- [YEAR] --> Acme LLC. All Rights Reserved.") {
		t.Errorf("copyright line not replaced in alertTemplate")
	}
	if !strings.Contains(productRegressionReportTemplate, "Acme Case Id") {
		t.Errorf("\"Case Id\" column header not replaced in productRegressionReportTemplate")
	}
	if !strings.Contains(staleCasesReportTemplate, "Acme Case ID") {
		t.Errorf("\"Case ID\" column header not replaced in staleCasesReportTemplate")
	}
	if !strings.Contains(migrationCaseReportTemplate, "Acme ID</th>") || !strings.Contains(migrationCaseReportTemplate, "Acme Product</th>") {
		t.Errorf("\"ID\"/\"Product\" column headers not replaced in migrationCaseReportTemplate")
	}
	// The Apache license header every raw template carries as a leading HTML
	// comment also contains the literal text "WSO2 LLC" -- unrelated to the
	// footer copyright line and never touched by SetCompanyName. Assert the
	// specific footer phrase, not a blanket absence of "WSO2 LLC" anywhere.
	if strings.Contains(alertTemplate, "&copy; <!-- [YEAR] --> WSO2 LLC.") {
		t.Errorf("stale default copyright phrase still present in alertTemplate")
	}

	// humanizeState's "wso2" -> "WSO2" mapping is deliberately NOT routed
	// through CompanyName -- see that function's own doc comment for why
	// (internal-only report audience, no branding reason to vary it).
	if humanized := humanizeState("waiting_on_wso2"); humanized != "Waiting On WSO2" {
		t.Errorf("humanizeState(waiting_on_wso2) = %q, want unchanged %q", humanized, "Waiting On WSO2")
	}
}

func TestSetCompanyName_EmptyKeepsDefault(t *testing.T) {
	before := alertTemplate
	SetCompanyName("  ")
	if alertTemplate != before {
		t.Errorf("empty value must keep the default company name")
	}
	if CompanyName() != defaultCompanyName {
		t.Errorf("CompanyName() = %q, want default %q", CompanyName(), defaultCompanyName)
	}
}
