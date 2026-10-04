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
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"time"
)

//go:embed templates/ledger_alert.html
var ledgerAlertTemplateRaw string

var ledgerAlertTemplate = bakeLogo(ledgerAlertTemplateRaw)

// LedgerFailure is one row of the ledger alert: a task whose claim, success
// record or failure record could not be written to entity-service during a
// tick. Stage is engine.Stage's string form ("claim", "complete", "fail");
// Error is an already-summarised, log-safe description — see
// engine.summarizeError — never a raw response body.
type LedgerFailure struct {
	Task  string
	Stage string
	Error string
}

// LedgerAlertData holds every value substituted into the "ledger error"
// HTML email template. One email per tick carries every ledger-stage
// failure that tick produced (see engine.Engine.Tick), so six tasks
// against one unreachable entity-service produce one message, not six.
type LedgerAlertData struct {
	TickTime string
	Failures []LedgerFailure
	// Omitted is how many further failures were cut from Failures to keep
	// the email bounded; zero renders nothing.
	Omitted int
}

// RenderLedgerAlertEmail fills in the "ledger error" HTML email template.
// [YEAR] is today's year, the same footer-only detail RenderAlertEmail's
// own doc comment describes.
func RenderLedgerAlertEmail(data LedgerAlertData) string {
	var rows strings.Builder
	for _, f := range data.Failures {
		fmt.Fprintf(&rows,
			`<tr>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed; vertical-align:top;"><strong>%s</strong></td>`+
				`<td style="padding:8px 10px; border-bottom:1px solid #e8eaed; vertical-align:top;">%s</td>`+
				`<td class="errorBlock" style="padding:8px 10px; border-bottom:1px solid #e8eaed; font-family:'Courier New', monospace; font-size:12px;">%s</td>`+
				`</tr>`,
			escapeHTML(f.Task), escapeHTML(f.Stage), escapeMultiline(f.Error))
	}
	omitted := ""
	if data.Omitted > 0 {
		omitted = fmt.Sprintf(`<p style="margin:0 0 24px; font-size:13px; color:#8a8f98;">%d further failure(s) omitted from this email; see the run's logs.</p>`, data.Omitted)
	}
	replacer := strings.NewReplacer(
		"<!-- [TICK_TIME] -->", escapeHTML(data.TickTime),
		"<!-- [FAILURE_COUNT] -->", strconv.Itoa(len(data.Failures)+data.Omitted),
		"<!-- [FAILURE_ROWS] -->", rows.String(),
		"<!-- [OMITTED_NOTE] -->", omitted,
		"<!-- [YEAR] -->", strconv.Itoa(time.Now().Year()),
	)
	return replacer.Replace(ledgerAlertTemplate)
}
