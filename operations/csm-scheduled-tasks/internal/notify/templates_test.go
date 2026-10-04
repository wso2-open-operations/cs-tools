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

func TestEscapeMultiline(t *testing.T) {
	got := EscapeMultiline("a <b> & c\nd — é")
	want := "a &lt;b&gt; &amp; c<br>d &#8212; &#233;"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderOutageNotification_LogoAndEscaping(t *testing.T) {
	out := RenderOutageNotification(OutageNotificationData{PhaseWord: "Declared", Number: "OUT0001", Message: "Outage — <x>"})
	if strings.Contains(out, "[LOGO_SRC]") || !strings.Contains(out, wso2LogoURL) {
		t.Fatal("the logo placeholder must be replaced by the logo URL")
	}
	if !strings.Contains(out, "Outage &#8212; &lt;x&gt;") {
		t.Fatalf("the message must be escaped, got %q", out)
	}
}

func TestEveryTemplateHasItsLogoBaked(t *testing.T) {
	for name, tpl := range map[string]string{
		"alert": alertTemplate, "stale": staleCasesReportTemplate, "open": openCasesReportTemplate,
		"outage": outageNotificationTemplate, "ledger": ledgerAlertTemplate,
	} {
		if strings.Contains(tpl, "[LOGO_SRC]") {
			t.Errorf("%s template still has its logo placeholder", name)
		}
	}
}

func TestRenderLedgerAlertEmail(t *testing.T) {
	out := RenderLedgerAlertEmail(LedgerAlertData{
		TickTime: "2026-10-02T10:00:00Z",
		Failures: []LedgerFailure{{Task: "a<b", Stage: "claim", Error: "entity-service returned HTTP 503"}},
		Omitted:  2,
	})
	for _, want := range []string{"a&lt;b", "claim", "entity-service returned HTTP 503", "<strong>3</strong>", "2 further failure(s) omitted"} {
		if !strings.Contains(out, want) {
			t.Errorf("ledger alert should contain %q", want)
		}
	}
}
