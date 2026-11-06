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
)

type fakeSender struct {
	to, cc  []string
	subject string
	body    string
	calls   int
}

func (f *fakeSender) SendEmail(_ context.Context, to, cc []string, subject, htmlBody string) error {
	f.calls++
	f.to, f.cc, f.subject, f.body = to, cc, subject, htmlBody
	return nil
}

func TestForReportRedirectsToTheDebugRecipients(t *testing.T) {
	inner := &fakeSender{}
	s := ForReport(inner, []string{" debug-one@example.com ", "", "debug-two@example.com"})
	if s == Sender(inner) {
		t.Fatal("ForReport returned the inner sender although debug recipients were given")
	}

	body := "<!DOCTYPE html><html><head><title>x</title></head><body class=\"b\"><p>report</p></body></html>"
	err := s.SendEmail(context.Background(), []string{"lead@example.com", "team@example.com"}, []string{"mgmt@example.com"}, "[Report] Weekly", body)
	if err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	if strings.Join(inner.to, ",") != "debug-one@example.com,debug-two@example.com" {
		t.Errorf("to = %v, want the trimmed debug recipients only", inner.to)
	}
	if len(inner.cc) != 0 {
		t.Errorf("cc = %v, want none: the real audience is never addressed", inner.cc)
	}
	if inner.subject != "[Test copy] [Report] Weekly" {
		t.Errorf("subject = %q", inner.subject)
	}
	notice := "It would have been sent to To: lead@example.com, team@example.com; Cc: mgmt@example.com."
	if !strings.Contains(inner.body, notice) {
		t.Errorf("body lacks the notice %q:\n%s", notice, inner.body)
	}
	if head, tail := strings.Index(inner.body, "<body class=\"b\">"), strings.Index(inner.body, "[Test copy]"); head < 0 || tail < head {
		t.Errorf("the notice must sit just inside <body>, after the head:\n%s", inner.body)
	}
	if !strings.Contains(inner.body, "<p>report</p>") {
		t.Errorf("the report itself is missing from the body:\n%s", inner.body)
	}
}

func TestForReportWithoutDebugRecipientsIsTheSenderItself(t *testing.T) {
	inner := &fakeSender{}
	if s := ForReport(inner, []string{"  ", ""}); s != Sender(inner) {
		t.Fatal("blank debug recipients must leave the sender untouched")
	}
	if s := ForReport(inner, nil); s != Sender(inner) {
		t.Fatal("no debug recipients must leave the sender untouched")
	}
}

func TestTestCopyNoticeWithoutBodyTagGoesFirst(t *testing.T) {
	out := testCopyNotice("<p>plain</p>", []string{"a@example.com"}, nil)
	if !strings.HasPrefix(out, "<p style=") || !strings.HasSuffix(out, "<p>plain</p>") {
		t.Errorf("notice must be prepended to a bodiless fragment:\n%s", out)
	}
	if strings.Contains(out, "Cc:") {
		t.Errorf("no Cc given, none should be named:\n%s", out)
	}
}

type recSender struct {
	to, cc  []string
	subject string
	body    string
}

func (r *recSender) SendEmail(_ context.Context, to, cc []string, subject, body string) error {
	r.to, r.cc, r.subject, r.body = to, cc, subject, body
	return nil
}

func TestForRunNowRedirectsWithCcAndRunNowNotice(t *testing.T) {
	inner := &recSender{}
	s := ForRunNow(inner, []string{" jane.doe@example.com ", ""}, []string{"john.roe@example.com"})
	if err := s.SendEmail(context.Background(), []string{"real@example.com"}, []string{"realcc@example.com"}, "Subj", "<body><p>x</p></body>"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(inner.to, ",") != "jane.doe@example.com" || strings.Join(inner.cc, ",") != "john.roe@example.com" {
		t.Errorf("to=%v cc=%v", inner.to, inner.cc)
	}
	if inner.subject != "[Test copy] Subj" {
		t.Errorf("subject = %q", inner.subject)
	}
	if !strings.Contains(inner.body, "started by hand") || !strings.Contains(inner.body, "real@example.com") ||
		strings.Contains(inner.body, "Debug mail mode") {
		t.Errorf("notice wrong:\n%s", inner.body)
	}
}
