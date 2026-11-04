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

package worklogsreport_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogsreport"
)

func d(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// subjectEmail records the whole mail.
type subjectEmail struct {
	sent    int
	to, cc  []string
	subject string
}

func (f *subjectEmail) SendEmail(_ context.Context, to, cc []string, subject, _ string) error {
	f.sent++
	f.to, f.cc, f.subject = to, cc, subject
	return nil
}

var anchor = func() time.Time { return time.Date(2030, 1, 14, 16, 0, 0, 0, time.UTC) }

func TestOverrideWindowReachesTheFetcherExactly(t *testing.T) {
	ov := worklogsreport.Override{From: d("2030-01-07"), To: d("2030-01-15")} // not 7 days
	reports := &fakeReports{}
	email := &subjectEmail{}
	h := worklogsreport.SendWeeklyWith(reports, email, anchor,
		[]string{"configured@example.com"}, nil, worklogreport.SourceConfigured, "", true, ov)
	if err := h(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reports.from.Equal(d("2030-01-07")) || !reports.to.Equal(d("2030-01-15")) {
		t.Errorf("fetched %v..%v", reports.from, reports.to)
	}
	if reports.cadence != worklogreport.CadenceWeekly {
		t.Errorf("cadence %v", reports.cadence)
	}
	if want := "[Manual range] [2030-01-07 - 2030-01-15] CS Worklogs Weekly Report"; email.subject != want {
		t.Errorf("subject %q, want %q", email.subject, want)
	}
	// The audience was not overridden: the configured one is used.
	if !reflect.DeepEqual(email.to, []string{"configured@example.com"}) {
		t.Errorf("to = %v", email.to)
	}

	// Daily, single day.
	reports = &fakeReports{}
	h = worklogsreport.SendDailyWith(reports, email, anchor,
		[]string{"configured@example.com"}, nil, worklogreport.SourceConfigured, "", true,
		worklogsreport.Override{From: d("2030-01-03"), To: d("2030-01-03")})
	if err := h(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reports.from.Equal(d("2030-01-03")) || !reports.to.Equal(d("2030-01-03")) {
		t.Errorf("daily fetched %v..%v", reports.from, reports.to)
	}
}

func TestOverrideRecipientsReplaceConfiguredAudience(t *testing.T) {
	reports := &fakeReports{}
	email := &subjectEmail{}
	ov := worklogsreport.Override{Recipients: []string{"tester@example.com"}, Cc: []string{"tester-cc@example.com"}}
	h := worklogsreport.SendWeeklyWith(reports, email, anchor,
		[]string{"leads@example.com"}, []string{"all@example.com"}, worklogreport.SourceConfigured, "", true, ov)
	if err := h(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(email.to, []string{"tester@example.com"}) || !reflect.DeepEqual(email.cc, []string{"tester-cc@example.com"}) {
		t.Errorf("to=%v cc=%v, want only the override (no union)", email.to, email.cc)
	}
	if !strings.HasPrefix(email.subject, "[Test] [") || !strings.HasSuffix(email.subject, "CS Worklogs Weekly Report") {
		t.Errorf("subject %q lacks the test marker", email.subject)
	}
	// Window untouched: the anchor's Monday-to-Sunday.
	if !reports.from.Equal(d("2030-01-07")) || !reports.to.Equal(d("2030-01-13")) {
		t.Errorf("fetched %v..%v, want the anchor's week", reports.from, reports.to)
	}
}

func TestOverrideRecipientsReplaceDerivedAudience(t *testing.T) {
	reports := &fakeDerived{to: []string{"registry-lead@example.com"}, cc: []string{"registry-list@example.com"}}
	email := &subjectEmail{}
	ov := worklogsreport.Override{Recipients: []string{"tester@example.com"}}
	h := worklogsreport.SendDailyWith(reports, email, anchor,
		[]string{"configured@example.com"}, []string{"configured-cc@example.com"}, worklogreport.SourceDerived, "", true, ov)
	if err := h(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(email.to, []string{"tester@example.com"}) || len(email.cc) != 0 {
		t.Errorf("to=%v cc=%v, want only the override", email.to, email.cc)
	}
}

// A recipient override sends even when the configured To is empty, which
// would otherwise skip the run.
func TestOverrideRecipientsSendWithNoConfiguredAudience(t *testing.T) {
	email := &subjectEmail{}
	h := worklogsreport.SendWeeklyWith(&fakeReports{}, email, anchor, nil, nil, worklogreport.SourceConfigured, "", true,
		worklogsreport.Override{Recipients: []string{"tester@example.com"}})
	if err := h(context.Background()); err != nil || email.sent != 1 {
		t.Fatalf("err=%v sent=%d", err, email.sent)
	}
}

func TestOverrideBothMarkersAndDisabledEmails(t *testing.T) {
	ov := worklogsreport.Override{From: d("2030-01-07"), To: d("2030-01-09"), Recipients: []string{"tester@example.com"}}
	email := &subjectEmail{}
	_ = worklogsreport.SendWeeklyWith(&fakeReports{}, email, anchor, nil, nil, worklogreport.SourceConfigured, "", true, ov)(context.Background())
	if !strings.HasPrefix(email.subject, "[Test][Manual range] [2030-01-07 - 2030-01-09]") {
		t.Errorf("subject %q", email.subject)
	}
	// ALERTS_ENABLED=false still silences an override run.
	email = &subjectEmail{}
	reports := &fakeReports{}
	_ = worklogsreport.SendWeeklyWith(reports, email, anchor, nil, nil, worklogreport.SourceConfigured, "", false, ov)(context.Background())
	if email.sent != 0 || !reports.from.IsZero() {
		t.Errorf("emails disabled but sent=%d", email.sent)
	}
}

// Without an override the handlers are exactly what they were: the subject is
// byte-identical and the Send*With variants with a zero Override match Send*.
func TestNoOverrideChangesNothing(t *testing.T) {
	for _, weekly := range []bool{true, false} {
		a, b := &subjectEmail{}, &subjectEmail{}
		ra, rb := &fakeReports{}, &fakeReports{}
		to, cc := []string{"leads@example.com"}, []string{"all@example.com"}
		if weekly {
			_ = worklogsreport.SendWeekly(ra, a, anchor, to, cc, worklogreport.SourceConfigured, "", true)(context.Background())
			_ = worklogsreport.SendWeeklyWith(rb, b, anchor, to, cc, worklogreport.SourceConfigured, "", true, worklogsreport.Override{})(context.Background())
		} else {
			_ = worklogsreport.SendDaily(ra, a, anchor, to, cc, worklogreport.SourceConfigured, "", true)(context.Background())
			_ = worklogsreport.SendDailyWith(rb, b, anchor, to, cc, worklogreport.SourceConfigured, "", true, worklogsreport.Override{})(context.Background())
		}
		if !reflect.DeepEqual(a, b) || !ra.from.Equal(rb.from) || !ra.to.Equal(rb.to) {
			t.Errorf("weekly=%v: %+v vs %+v", weekly, a, b)
		}
		if strings.Contains(a.subject, "[Test]") || strings.Contains(a.subject, "[Manual range]") || !strings.HasPrefix(a.subject, "[2030-") {
			t.Errorf("scheduled subject altered: %q", a.subject)
		}
	}
	if got := worklogsreport.OverrideSubject("[x] y", worklogsreport.Override{}); got != "[x] y" {
		t.Errorf("zero override changed the subject: %q", got)
	}
}

func TestOverrideValidate(t *testing.T) {
	ok := worklogsreport.Override{From: d("2030-01-01"), To: d("2030-01-01")}
	if err := ok.Validate(worklogreport.CadenceDaily); err != nil {
		t.Errorf("daily single day: %v", err)
	}
	rng := worklogsreport.Override{From: d("2030-01-01"), To: d("2030-01-02")}
	if err := rng.Validate(worklogreport.CadenceDaily); err == nil {
		t.Error("daily range must be refused")
	}
	if err := rng.Validate(worklogreport.CadenceWeekly); err != nil {
		t.Errorf("weekly range: %v", err)
	}
	if err := (worklogsreport.Override{From: d("2030-01-01")}).Validate(worklogreport.CadenceWeekly); err == nil {
		t.Error("half a window must be refused")
	}
	if err := (worklogsreport.Override{Cc: []string{"a@example.com"}}).Validate(worklogreport.CadenceWeekly); err == nil {
		t.Error("cc alone must be refused")
	}
}
