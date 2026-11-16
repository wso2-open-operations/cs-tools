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

package slabreachscan

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/slabreachreport"
)

type fakeFetcher struct {
	report   slabreachreport.Report
	err      error
	calls    int
	gotDefs  []string
	gotStage []string
}

func (f *fakeFetcher) Report(_ context.Context, definitions, stages []string) (slabreachreport.Report, error) {
	f.calls++
	f.gotDefs, f.gotStage = definitions, stages
	return f.report, f.err
}

type fakeSender struct {
	calls   int
	to, cc  []string
	subject string
	body    string
}

func (f *fakeSender) SendEmail(_ context.Context, to, cc []string, subject, htmlBody string) error {
	f.calls++
	f.to, f.cc, f.subject, f.body = to, cc, subject, htmlBody
	return nil
}

func defs() []string { return DefaultDefinitions() }

// TestSendReportSkipsWithoutRecipients: with no "to" the task must not
// even reach entity-service -- no report to send is no reason to query.
func TestSendReportSkipsWithoutRecipients(t *testing.T) {
	fetch, send := &fakeFetcher{}, &fakeSender{}
	if err := SendReport(fetch, send, defs(), DefaultStages(), nil, nil, "", true)(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if fetch.calls != 0 || send.calls != 0 {
		t.Fatalf("fetch=%d send=%d, want 0 and 0", fetch.calls, send.calls)
	}
}

// TestSendReportSkipsWhenEmailsDisabled covers ALERTS_ENABLED=false.
func TestSendReportSkipsWhenEmailsDisabled(t *testing.T) {
	fetch, send := &fakeFetcher{}, &fakeSender{}
	if err := SendReport(fetch, send, defs(), DefaultStages(), []string{"a@wso2.com"}, nil, "", false)(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if fetch.calls != 0 || send.calls != 0 {
		t.Fatalf("fetch=%d send=%d, want 0 and 0", fetch.calls, send.calls)
	}
}

// TestSendReportSendsAnEmptyWeek is the deliberate difference from the
// regression reports: a clean week is the result this report exists to
// show, so it is still sent.
func TestSendReportSendsAnEmptyWeek(t *testing.T) {
	fetch := &fakeFetcher{report: slabreachreport.Report{
		PeriodStart: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
	}}
	send := &fakeSender{}
	if err := SendReport(fetch, send, defs(), DefaultStages(), []string{"a@wso2.com"}, nil, "", true)(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if send.calls != 1 {
		t.Fatalf("send calls = %d, want 1 even with no breaches", send.calls)
	}
}

// TestSendReportSubjectAndAudience pins the subject word for word and
// checks the configured audience is what reaches the sender.
func TestSendReportSubjectAndAudience(t *testing.T) {
	fetch := &fakeFetcher{report: slabreachreport.Report{
		CRE: []slabreachreport.Row{{Team: "Atlas", CaseNumber: "CS01", Definition: "P1 - Response (Open Source)"}},
	}}
	send := &fakeSender{}
	to := []string{"to@wso2.com"}
	cc := []string{"cc@wso2.com"}
	if err := SendReport(fetch, send, defs(), DefaultStages(), to, cc, "", true)(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if send.subject != "Weekly SLA Breach Report" {
		t.Fatalf("subject = %q", send.subject)
	}
	if len(send.to) != 1 || send.to[0] != "to@wso2.com" || len(send.cc) != 1 || send.cc[0] != "cc@wso2.com" {
		t.Fatalf("to=%v cc=%v", send.to, send.cc)
	}
	if !strings.Contains(send.body, "CS01") {
		t.Fatalf("body is missing the case number")
	}
}

// TestSendReportPassesFiltersThrough: the ten definitions and the stages
// must reach entity-service unchanged.
func TestSendReportPassesFiltersThrough(t *testing.T) {
	fetch, send := &fakeFetcher{}, &fakeSender{}
	if err := SendReport(fetch, send, defs(), DefaultStages(), []string{"a@wso2.com"}, nil, "", true)(context.Background()); err != nil {
		t.Fatalf("SendReport: %v", err)
	}
	if len(fetch.gotDefs) != 10 {
		t.Fatalf("definitions = %d, want the 10 the source report named", len(fetch.gotDefs))
	}
	if len(fetch.gotStage) != 2 || fetch.gotStage[0] != "PAUSED" || fetch.gotStage[1] != "COMPLETED" {
		t.Fatalf("stages = %v", fetch.gotStage)
	}
}

// TestSendReportRefusesWithoutDefinitions: entity-service rejects an
// empty list, so fail here rather than 400 on every tick.
func TestSendReportRefusesWithoutDefinitions(t *testing.T) {
	fetch, send := &fakeFetcher{}, &fakeSender{}
	err := SendReport(fetch, send, nil, DefaultStages(), []string{"a@wso2.com"}, nil, "", true)(context.Background())
	if err == nil {
		t.Fatal("want an error with no definitions configured")
	}
	if fetch.calls != 0 || send.calls != 0 {
		t.Fatalf("fetch=%d send=%d, want 0 and 0", fetch.calls, send.calls)
	}
}

// TestSendReportPropagatesFetchFailure keeps a failed period retryable
// rather than silently sending nothing.
func TestSendReportPropagatesFetchFailure(t *testing.T) {
	fetch := &fakeFetcher{err: errors.New("boom")}
	send := &fakeSender{}
	if err := SendReport(fetch, send, defs(), DefaultStages(), []string{"a@wso2.com"}, nil, "", true)(context.Background()); err == nil {
		t.Fatal("want the fetch error propagated")
	}
	if send.calls != 0 {
		t.Fatalf("send calls = %d, want 0", send.calls)
	}
}

// TestParseList covers the override parsing both overrides rely on.
func TestParseList(t *testing.T) {
	if got := ParseList("  "); got != nil {
		t.Fatalf("blank = %#v, want nil so the default is kept", got)
	}
	got := ParseList(" PAUSED , ,COMPLETED ")
	if len(got) != 2 || got[0] != "PAUSED" || got[1] != "COMPLETED" {
		t.Fatalf("got %#v", got)
	}
}
