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

func TestCasePagingSubject(t *testing.T) {
	if got, want := CasePagingSubject("S0", "CS0099001"), "[Case Alerts] S0 CS0099001 not acknowledged"; got != want {
		t.Errorf("subject = %q, want %q", got, want)
	}
	if got, want := CasePagingRungLine("S0", 1, "Atlas ABT Team Lead"), "S0 alert at L1 for Atlas ABT Team Lead"; got != want {
		t.Errorf("rung line = %q, want %q", got, want)
	}
	if got, want := CasePagingRungLine("S2", 3, " "), "S2 alert at L3"; got != want {
		t.Errorf("rung line with no audience = %q, want %q", got, want)
	}
}

func TestRenderCasePagingEmail(t *testing.T) {
	body := RenderCasePagingEmail(CasePagingEmailData{
		Greeting:        "ABT Team Leads",
		RungLine:        "S1 alert at L2 for all ABT Team Leads",
		Opening:         "Case CS0099001 (S1) is still in open state and response SLA remaining time is 42m.",
		CaseNumber:      "CS0099001",
		WSO2CaseID:      "WSO2-1000",
		Title:           "Gateway <down> & out",
		SeverityCode:    "S1",
		Severity:        "S1 \u2013 Critical",
		Team:            "Atlas",
		Rung:            2,
		EscalationLevel: "L2 \u00b7 All ABT Team Leads",
		CreatedAt:       "2026-10-11 14:05 IST",
		Waiting:         "15m",
		Notified:        []string{"Atlas Lead (Atlas ABT Team Lead), 14:05 IST"},
		NextEscalation:  "CRE Head in 10m",
		CaseURL:         "https://csm.example/cases/abc",
	})
	for _, want := range []string{
		">Case Alert Notification</td>",                             // the heading (agreed 2026-10-17)
		`color:#7a2e04;">S1 alert at L2 for all ABT Team Leads</p>`, // S1's callout
		"background-color:#e8590c;",                                 // S1's bar
		"Hi ABT Team Leads,",
		">Case number</td>", ">CS0099001</td>",
		">WSO2 case ID</td>", ">WSO2-1000</td>",
		">Short description</td>", ">Gateway &lt;down&gt; &amp; out</td>",
		"color:#c2410c;\">S1 &#8211; Critical</td>",
		"color:#b45309;\">L2 &#183; All ABT Team Leads</td>",
		">Already notified</td>", "Atlas Lead (Atlas ABT Team Lead), 14:05 IST",
		"CRE Head in 10m",
		`href="https://csm.example/cases/abc"`, ">View case</a>",
		wso2LogoURL,
		"WSO2 LLC. All rights reserved.",
		"</html>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body has no %q", want)
		}
	}
	// An empty value drops its row, the debug line is absent on a real send,
	// and no placeholder survives.
	for _, unwanted := range []string{">Account<", "Expected sent to", "<!-- ["} {
		if strings.Contains(body, unwanted) {
			t.Errorf("body has %q", unwanted)
		}
	}
	// ASCII only on the wire: the email service mangles raw multi-byte text.
	for i, r := range body {
		if r > 127 {
			t.Fatalf("non-ASCII %q at byte %d", r, i)
		}
	}
}

func TestRenderCasePagingEmail_NoLinkDebugLineAndUnknownSeverity(t *testing.T) {
	body := RenderCasePagingEmail(CasePagingEmailData{Greeting: "Cs Head", CaseNumber: "CS1", SeverityCode: "S9", IntendedFor: "cs.head@example.com"})
	if strings.Contains(body, "View case") {
		t.Error("a button was rendered with no case URL")
	}
	if !strings.Contains(body, "Expected sent to: cs.head@example.com") {
		t.Error("a debug-redirected send does not say who it was meant for")
	}
	if !strings.Contains(body, "background-color:#868e96;") {
		t.Error("an unknown severity did not fall back to the neutral colours")
	}
}
