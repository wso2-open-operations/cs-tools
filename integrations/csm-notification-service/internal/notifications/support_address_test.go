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

// restoreSupportAddress puts the baked onboarding templates and the current
// address back after a test that changes them.
func restoreSupportAddress(t *testing.T) {
	t.Helper()
	tmpls := footerIconTemplates()
	orig := make([]string, len(tmpls))
	for i, p := range tmpls {
		orig[i] = *p
	}
	addr, mailto := supportAddress, supportMailto
	t.Cleanup(func() {
		for i, p := range tmpls {
			*p = orig[i]
		}
		supportAddress, supportMailto = addr, mailto
	})
}

func onboardingBodies() map[string]string {
	d := ProjectContactInvitedEmailData{DisplayName: "Jane Doe", Email: "jane.doe@example.com", ProjectName: "Sample", ProjectKey: "SAMPLE"}
	return map[string]string{
		"new":        RenderProjectContactInvitedNewEmail(d),
		"existing":   RenderProjectContactInvitedExistingEmail(d),
		"reminder":   RenderProjectContactInvitedReminderEmail(d),
		"registered": RenderProjectContactRegisteredEmail(ProjectContactRegisteredEmailData{DisplayName: "Jane Doe", ProjectName: "Sample", ProjectKey: "SAMPLE"}),
	}
}

func TestSupportAddress_DefaultInEveryOnboardingEmail(t *testing.T) {
	restoreSupportAddress(t)
	if SupportAddress() != "support@wso2.com" {
		t.Fatalf("SupportAddress() = %q, want the default", SupportAddress())
	}
	for name, body := range onboardingBodies() {
		if !strings.Contains(body, "mailto:support@wso2.com") || !strings.Contains(body, ">support@wso2.com<") {
			t.Errorf("%s email does not carry the default support address as text and link", name)
		}
	}
}

func TestSetSupportAddress_RewritesTextAndLinkInEveryOnboardingEmail(t *testing.T) {
	restoreSupportAddress(t)
	if err := SetSupportAddress(" help@example.com "); err != nil {
		t.Fatalf("SetSupportAddress: %v", err)
	}
	if SupportAddress() != "help@example.com" {
		t.Errorf("SupportAddress() = %q, want the configured address", SupportAddress())
	}
	for name, body := range onboardingBodies() {
		if !strings.Contains(body, "mailto:help@example.com") || !strings.Contains(body, ">help@example.com<") {
			t.Errorf("%s email does not carry the configured address as text and link", name)
		}
		if strings.Contains(body, "support@wso2.com") {
			t.Errorf("%s email still carries the default address", name)
		}
	}
}

func TestSetSupportAddress_CanBeChangedAgain(t *testing.T) {
	restoreSupportAddress(t)
	_ = SetSupportAddress("first@example.com")
	_ = SetSupportAddress("second@example.com")
	for name, body := range onboardingBodies() {
		if strings.Contains(body, "first@example.com") || !strings.Contains(body, "mailto:second@example.com") {
			t.Errorf("%s email did not move to the second address", name)
		}
	}
}

func TestSetSupportAddress_EmptyKeepsTheDefault(t *testing.T) {
	restoreSupportAddress(t)
	if err := SetSupportAddress("   "); err != nil {
		t.Fatalf("an empty value must not be an error: %v", err)
	}
	if SupportAddress() != "support@wso2.com" {
		t.Errorf("SupportAddress() = %q, want the default", SupportAddress())
	}
}

func TestSetSupportAddress_RefusesAnythingButOnePlainAddress(t *testing.T) {
	for _, bad := range []string{"not-an-address", "a@b@example.com", "@example.com", "help@", "help me@example.com", "help@example.com, other@example.com", "<help@example.com>", `help"@example.com`, "help..desk@example.com", ".help@example.com", "help.@example.com", "Jane Doe <jane.doe@example.com>"} {
		t.Run(bad, func(t *testing.T) {
			restoreSupportAddress(t)
			before := onboardingBodies()["new"]
			if err := SetSupportAddress(bad); err == nil {
				t.Errorf("%q was accepted", bad)
			}
			if SupportAddress() != "support@wso2.com" || onboardingBodies()["new"] != before {
				t.Errorf("a refused value must leave the default in place")
			}
		})
	}
}

// The error is logged at startup, so it must not repeat the rejected value,
// which could be a name and an address.
func TestSetSupportAddress_TheErrorNamesNoValue(t *testing.T) {
	restoreSupportAddress(t)
	err := SetSupportAddress("Jane Doe <jane.doe@example.com>")
	if err == nil {
		t.Fatal("a display name with an address was accepted")
	}
	for _, leak := range []string{"Jane", "jane.doe", "example.com"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("the error repeats part of the rejected value (%q): %v", leak, err)
		}
	}
}

// The mailto destination is percent-encoded for a URI separately from the
// visible text: a "?" in the part before the "@" would otherwise start the
// link's query instead of staying in the recipient.
func TestSetSupportAddress_TheLinkIsEncodedAndTheTextIsNot(t *testing.T) {
	restoreSupportAddress(t)
	if err := SetSupportAddress("unlikely?address@example.com"); err != nil {
		t.Fatalf("SetSupportAddress: %v", err)
	}
	for name, body := range onboardingBodies() {
		if !strings.Contains(body, "mailto:unlikely%3Faddress@example.com") {
			t.Errorf("%s email: the mailto link is not percent-encoded", name)
		}
		if !strings.Contains(body, ">unlikely?address@example.com<") {
			t.Errorf("%s email: the visible text is not the plain address", name)
		}
		if strings.Contains(body, "mailto:unlikely?address") {
			t.Errorf("%s email: the link still has a bare \"?\"", name)
		}
	}
}

// A new address that contains the old one is not mangled by the replacement.
func TestSetSupportAddress_ANewAddressContainingTheOldOneIsKeptWhole(t *testing.T) {
	restoreSupportAddress(t)
	if err := SetSupportAddress("x-support@wso2.com"); err != nil {
		t.Fatalf("SetSupportAddress: %v", err)
	}
	for name, body := range onboardingBodies() {
		if !strings.Contains(body, "mailto:x-support@wso2.com") || !strings.Contains(body, ">x-support@wso2.com<") {
			t.Errorf("%s email: the address was mangled", name)
		}
		if strings.Contains(body, "x-x-support") {
			t.Errorf("%s email: the address was replaced twice", name)
		}
	}
}
