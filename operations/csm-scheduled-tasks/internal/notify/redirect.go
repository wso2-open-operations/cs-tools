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
	"log/slog"
	"strings"
)

// Sender is the one method every report-style task sends through. *Client
// is one; ForReport wraps one.
type Sender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// ForReport returns the Sender a report task should send through. With no
// debug recipients it is inner itself, unchanged. With them, it is debug
// mail mode for that one report: every email goes to the debug recipients
// instead of its intended To/Cc, with "[Test copy]" prefixed to the
// subject and a notice just inside <body> naming who it would have gone
// to. It is how a report is exercised in a production deployment, real
// data and all, without reaching its real audience -- per task, so the
// migration scan can be on trial while the stale cases report and every
// failure alert go out as normal. Configured per task as the "debug" list
// in SUB_CRON_RECIPIENTS (see cmd/server/main.go).
func ForReport(inner Sender, debugRecipients []string) Sender {
	debug := cleanAddresses(debugRecipients)
	if len(debug) == 0 {
		return inner
	}
	return &redirected{inner: inner, debug: debug}
}

// ForRunNow returns the Sender for a manual run-now that replaces a
// report's audience: every email goes to to (and cc) instead of its
// intended To/Cc, with "[Test copy]" prefixed to the subject and a notice
// just inside <body> naming who it would have gone to. Unlike ForReport it
// is not debug mail mode and can address a Cc. The real audience is named
// in the body and never addressed.
func ForRunNow(inner Sender, to, cc []string) Sender {
	return &redirected{inner: inner, debug: cleanAddresses(to), cc: cleanAddresses(cc), runNow: true}
}

type redirected struct {
	inner Sender
	debug []string
	// cc and runNow are set only by ForRunNow.
	cc     []string
	runNow bool
}

func (r *redirected) SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error {
	// The real audience is named in the body and never addressed. The log
	// line is the audit trail of what was redirected; the addresses
	// themselves stay out of the logs.
	if r.runNow {
		slog.Info("run-now: redirecting a report to the given recipients",
			"subject", subject, "intendedTo", len(to), "intendedCc", len(cc), "to", len(r.debug), "cc", len(r.cc))
		return r.inner.SendEmail(ctx, r.debug, r.cc, "[Test copy] "+subject,
			testCopyNoticeFor(htmlBody, to, cc, "This run was started by hand with its recipients replaced."))
	}
	slog.Info("debug mail mode: redirecting a report to its debug recipients",
		"subject", subject, "intendedTo", len(to), "intendedCc", len(cc), "debugRecipients", len(r.debug))
	return r.inner.SendEmail(ctx, r.debug, nil, "[Test copy] "+subject, testCopyNotice(htmlBody, to, cc))
}

// cleanAddresses trims the addresses and drops blanks, so a trailing
// comma in configuration never becomes an empty recipient.
func cleanAddresses(in []string) []string {
	var out []string
	for _, a := range in {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}

// testCopyNotice prepends the debug-mode notice to htmlBody: who the
// email would have gone to. It goes just inside <body> when the body has
// one, so a full HTML document keeps its head and styles, and at the very
// start otherwise.
func testCopyNotice(htmlBody string, to, cc []string) string {
	return testCopyNoticeFor(htmlBody, to, cc, "Debug mail mode is on for this report.")
}

// testCopyNoticeFor is testCopyNotice with the sentence that says why this
// is a test copy.
func testCopyNoticeFor(htmlBody string, to, cc []string, reason string) string {
	intended := "To: " + strings.Join(to, ", ")
	if len(to) == 0 {
		intended = "no To address"
	}
	if len(cc) > 0 {
		intended += "; Cc: " + strings.Join(cc, ", ")
	}
	notice := `<p style="margin:0; padding:12px 16px; background-color:#fff4e5; color:#7a4100; font-family:Arial, Helvetica, sans-serif; font-size:13px;"><b>[Test copy]</b> ` + escapeHTML(reason) + ` It would have been sent to ` +
		escapeHTML(intended) + `.</p>` + "\n"
	if i := strings.Index(strings.ToLower(htmlBody), "<body"); i >= 0 {
		if j := strings.Index(htmlBody[i:], ">"); j >= 0 {
			at := i + j + 1
			return htmlBody[:at] + "\n" + notice + htmlBody[at:]
		}
	}
	return notice + htmlBody
}
