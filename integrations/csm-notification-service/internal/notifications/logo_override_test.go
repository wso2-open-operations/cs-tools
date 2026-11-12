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
