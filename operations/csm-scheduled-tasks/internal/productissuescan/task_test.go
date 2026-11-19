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

package productissuescan

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/productissuereport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/schedule"
)

type dashCall struct {
	period, date string
	products     []string
}

type fakeFetcher struct {
	calls     []dashCall
	leads     []productissuereport.Lead
	leadsErr  error
	dashErr   func(products []string) error
	dashboard productissuereport.Dashboard
	echo      func(requested []string) []string
}

func (f *fakeFetcher) Dashboard(_ context.Context, period, date string, products []string) (productissuereport.Dashboard, error) {
	f.calls = append(f.calls, dashCall{period, date, products})
	d := f.dashboard
	d.Products = products
	if f.echo != nil {
		d.Products = f.echo(products)
	}
	if f.dashErr != nil {
		if err := f.dashErr(products); err != nil {
			return productissuereport.Dashboard{}, err
		}
	}
	return d, nil
}

func (f *fakeFetcher) Recipients(context.Context) ([]productissuereport.Lead, error) {
	return f.leads, f.leadsErr
}

type sent struct {
	to, cc  []string
	subject string
	body    string
}

type fakeSender struct {
	mails []sent
	err   func(to []string) error
}

func (f *fakeSender) SendEmail(_ context.Context, to, cc []string, subject, body string) error {
	if f.err != nil {
		if err := f.err(to); err != nil {
			return err
		}
	}
	f.mails = append(f.mails, sent{to, cc, subject, body})
	return nil
}

func ist(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func weekDash() productissuereport.Dashboard {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	return productissuereport.Dashboard{
		PeriodStart: time.Date(2026, 11, 9, 0, 0, 0, 0, loc),
		PeriodEnd:   time.Date(2026, 11, 16, 0, 0, 0, 0, loc),
	}
}

func anchorCtx(a time.Time) context.Context { return schedule.WithPeriod(context.Background(), a) }

func TestReportDate(t *testing.T) {
	cases := []struct {
		name    string
		cadence Cadence
		anchor  time.Time
		want    string
	}{
		{"weekly Monday 09:00 IST", Weekly, time.Date(2026, 11, 16, 3, 30, 0, 0, time.UTC), "2026-11-09"},
		{"weekly retry later the same week", Weekly, time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC), "2026-11-09"},
		// Sunday 21:00 UTC is already Monday 02:30 in Kolkata.
		{"weekly Sunday evening UTC is Monday IST", Weekly, time.Date(2026, 11, 15, 21, 0, 0, 0, time.UTC), "2026-11-09"},
		// Sunday 18:00 UTC is still Sunday 23:30 in Kolkata: the previous week.
		{"weekly Sunday 18:00 UTC is Sunday IST", Weekly, time.Date(2026, 11, 15, 18, 0, 0, 0, time.UTC), "2026-11-02"},
		{"monthly the 1st", Monthly, time.Date(2026, 12, 1, 3, 30, 0, 0, time.UTC), "2026-11-01"},
		{"monthly across the year", Monthly, time.Date(2027, 1, 1, 3, 30, 0, 0, time.UTC), "2026-12-01"},
		// 30 Nov 20:00 UTC is 1 Dec 01:30 in Kolkata.
		{"monthly midnight edge", Monthly, time.Date(2026, 11, 30, 20, 0, 0, 0, time.UTC), "2026-11-01"},
		{"monthly 18:00 UTC last day is still that month", Monthly, time.Date(2026, 11, 30, 18, 0, 0, 0, time.UTC), "2026-10-01"},
	}
	for _, c := range cases {
		if got := ReportDate(c.cadence, c.anchor); got != c.want {
			t.Errorf("%s: ReportDate = %s, want %s", c.name, got, c.want)
		}
	}
	_ = ist(t)
}

func TestSendUsesAnchorFromContextThenFallback(t *testing.T) {
	f := &fakeFetcher{dashboard: weekDash()}
	s := &fakeSender{}
	opt := Options{Cadence: Weekly, To: []string{"all@x.invalid"}, EmailsEnabled: true,
		Anchor: func() time.Time { return time.Date(2026, 11, 23, 3, 30, 0, 0, time.UTC) }}
	// With the engine's period in the context it wins over the fallback, and a
	// later retry of the same period asks for the same date.
	h := Send(f, s, opt)
	if err := h(anchorCtx(time.Date(2026, 11, 16, 3, 30, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}
	if err := h(anchorCtx(time.Date(2026, 11, 16, 3, 30, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}
	if f.calls[0].date != "2026-11-09" || f.calls[1].date != "2026-11-09" || f.calls[0].period != "week" {
		t.Errorf("calls = %+v", f.calls)
	}
	// No period in the context: the Anchor fallback.
	f.calls = nil
	if err := h(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.calls[0].date != "2026-11-16" {
		t.Errorf("fallback date = %s", f.calls[0].date)
	}
}

func TestSendMonthlySubjectAndPeriod(t *testing.T) {
	loc := ist(t)
	f := &fakeFetcher{dashboard: productissuereport.Dashboard{
		PeriodStart: time.Date(2026, 11, 1, 0, 0, 0, 0, loc), PeriodEnd: time.Date(2026, 12, 1, 0, 0, 0, 0, loc)}}
	s := &fakeSender{}
	h := Send(f, s, Options{Cadence: Monthly, To: []string{"all@x.invalid"}, EmailsEnabled: true})
	if err := h(anchorCtx(time.Date(2026, 12, 1, 3, 30, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}
	if f.calls[0].period != "month" || f.calls[0].date != "2026-11-01" {
		t.Errorf("call = %+v", f.calls[0])
	}
	if s.mails[0].subject != "Product issues: monthly report, November 2026" {
		t.Errorf("subject = %q", s.mails[0].subject)
	}
}

func TestAudiences(t *testing.T) {
	f := &fakeFetcher{dashboard: weekDash(), leads: []productissuereport.Lead{
		{Email: "all@x.invalid", ProductNames: []string{"API Manager"}}, // also in the configured to-list
		{Email: "lead2@x.invalid", ProductNames: []string{"API Manager", "Identity Server"}},
		{Email: "  ", ProductNames: []string{"X"}},
		{Email: "noproducts@x.invalid"},
	}}
	s := &fakeSender{}
	h := Send(f, s, Options{Cadence: Weekly, To: []string{"all@x.invalid"}, Cc: []string{"cc@x.invalid"}, EmailsEnabled: true})
	if err := h(anchorCtx(time.Date(2026, 11, 16, 3, 30, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}
	if len(s.mails) != 3 || len(f.calls) != 3 {
		t.Fatalf("mails=%d calls=%d", len(s.mails), len(f.calls))
	}
	if len(f.calls[0].products) != 0 || s.mails[0].to[0] != "all@x.invalid" || s.mails[0].cc[0] != "cc@x.invalid" ||
		s.mails[0].subject != "Product issues: weekly report, 9-15 Nov 2026" {
		t.Errorf("all-products mail = %+v call %+v", s.mails[0], f.calls[0])
	}
	// The lead who is also a configured recipient still gets their own report.
	if s.mails[1].to[0] != "all@x.invalid" || len(s.mails[1].cc) != 0 ||
		s.mails[1].subject != "Product issues: weekly report, 9-15 Nov 2026 (API Manager)" ||
		strings.Join(f.calls[1].products, "|") != "API Manager" {
		t.Errorf("lead 1 mail = %+v call %+v", s.mails[1], f.calls[1])
	}
	if s.mails[2].subject != "Product issues: weekly report, 9-15 Nov 2026 (API Manager, Identity Server)" ||
		strings.Join(f.calls[2].products, "|") != "API Manager|Identity Server" || s.mails[2].to[0] != "lead2@x.invalid" {
		t.Errorf("lead 2 mail = %+v call %+v", s.mails[2], f.calls[2])
	}
}

func TestEmptyToSkipsOnlyTheConfiguredAudience(t *testing.T) {
	f := &fakeFetcher{dashboard: weekDash(), leads: []productissuereport.Lead{{Email: "lead@x.invalid", ProductNames: []string{"A"}}}}
	s := &fakeSender{}
	h := Send(f, s, Options{Cadence: Weekly, EmailsEnabled: true})
	if err := h(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(s.mails) != 1 || s.mails[0].to[0] != "lead@x.invalid" || len(f.calls) != 1 {
		t.Errorf("mails = %+v", s.mails)
	}
}

func TestDisabledDoesNothing(t *testing.T) {
	f := &fakeFetcher{leads: []productissuereport.Lead{{Email: "l@x.invalid", ProductNames: []string{"A"}}}}
	s := &fakeSender{}
	if err := Send(f, s, Options{Cadence: Weekly, To: []string{"a@x.invalid"}})(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 || len(s.mails) != 0 {
		t.Error("emails disabled must not fetch or send")
	}
}

func TestOneFailureDoesNotStopTheOthers(t *testing.T) {
	f := &fakeFetcher{dashboard: weekDash(), leads: []productissuereport.Lead{
		{Email: "bad@x.invalid", ProductNames: []string{"Bad"}},
		{Email: "good@x.invalid", ProductNames: []string{"Good"}},
	}}
	s := &fakeSender{err: func(to []string) error {
		if to[0] == "bad@x.invalid" {
			return errors.New("smtp down")
		}
		return nil
	}}
	h := Send(f, s, Options{Cadence: Weekly, To: []string{"all@x.invalid"}, EmailsEnabled: true})
	err := h(context.Background())
	if err == nil || !strings.Contains(err.Error(), "smtp down") {
		t.Fatalf("err = %v", err)
	}
	if len(s.mails) != 2 {
		t.Errorf("sent %d, want the all-products and the good lead", len(s.mails))
	}
	if strings.Contains(err.Error(), "bad@x.invalid") {
		t.Error("error must not carry the lead's address")
	}

	// A failing all-products fetch does not stop the leads either.
	f2 := &fakeFetcher{dashboard: weekDash(), leads: f.leads, dashErr: func(p []string) error {
		if len(p) == 0 {
			return errors.New("boom")
		}
		return nil
	}}
	s2 := &fakeSender{}
	if err := Send(f2, s2, Options{Cadence: Weekly, To: []string{"all@x.invalid"}, EmailsEnabled: true})(context.Background()); err == nil {
		t.Error("want the error returned")
	}
	if len(s2.mails) != 2 {
		t.Errorf("leads sent = %d, want 2", len(s2.mails))
	}
}

func TestRecipientsFailureStillSendsConfiguredAndReturnsError(t *testing.T) {
	f := &fakeFetcher{dashboard: weekDash(), leadsErr: errors.New("leads down")}
	s := &fakeSender{}
	err := Send(f, s, Options{Cadence: Weekly, To: []string{"all@x.invalid"}, EmailsEnabled: true})(context.Background())
	if err == nil || len(s.mails) != 1 {
		t.Errorf("err=%v mails=%d", err, len(s.mails))
	}
}

func TestNotFoundIsLoggedAndNil(t *testing.T) {
	nf := &apierror.Error{StatusCode: http.StatusNotFound}
	f := &fakeFetcher{dashErr: func([]string) error { return nf }, leads: []productissuereport.Lead{{Email: "l@x.invalid", ProductNames: []string{"A"}}}}
	s := &fakeSender{}
	if err := Send(f, s, Options{Cadence: Weekly, To: []string{"a@x.invalid"}, EmailsEnabled: true})(context.Background()); err != nil {
		t.Errorf("404 must be nil, got %v", err)
	}
	if len(s.mails) != 0 || len(f.calls) != 1 {
		t.Errorf("a feature-off 404 must stop at the first call: calls=%d mails=%d", len(f.calls), len(s.mails))
	}

	// 404 on the recipients route with no configured audience.
	f2 := &fakeFetcher{leadsErr: nf}
	if err := Send(f2, s, Options{Cadence: Weekly, EmailsEnabled: true})(context.Background()); err != nil {
		t.Errorf("404 recipients must be nil, got %v", err)
	}
}

func TestCommaInProductNameIsRefusedWithoutRetry(t *testing.T) {
	f := &fakeFetcher{dashboard: weekDash(), leads: []productissuereport.Lead{{Email: "l@x.invalid", ProductNames: []string{"Open, Banking"}}}}
	s := &fakeSender{}
	err := Send(f, s, Options{Cadence: Weekly, EmailsEnabled: true})(context.Background())
	if err != nil || len(f.calls) != 0 || len(s.mails) != 0 {
		t.Errorf("err=%v calls=%d mails=%d", err, len(f.calls), len(s.mails))
	}
}

func TestDebugRedirectCoversEveryAudience(t *testing.T) {
	f := &fakeFetcher{dashboard: weekDash(), leads: []productissuereport.Lead{{Email: "lead@x.invalid", ProductNames: []string{"A"}}}}
	raw := &fakeSender{}
	debug := notify.ForReport(raw, []string{"debug@x.invalid"})
	if err := Send(f, debug, Options{Cadence: Weekly, To: []string{"all@x.invalid"}, EmailsEnabled: true})(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(raw.mails) != 2 {
		t.Fatalf("mails = %d", len(raw.mails))
	}
	for _, m := range raw.mails {
		if len(m.to) != 1 || m.to[0] != "debug@x.invalid" || !strings.HasPrefix(m.subject, "[Test copy] Product issues: weekly report") {
			t.Errorf("not redirected: %+v", m)
		}
	}
}

func TestPermanentLeadFailureIsNotReturnedAndOthersSend(t *testing.T) {
	f := &fakeFetcher{dashboard: weekDash(), leads: []productissuereport.Lead{
		{Email: "comma@x.invalid", ProductNames: []string{"Open, Banking"}},
		{Email: "blank@x.invalid", ProductNames: []string{"  ", ""}},
		{Email: "forbidden@x.invalid", ProductNames: []string{"Forbidden"}},
		{Email: "good@x.invalid", ProductNames: []string{" Good "}},
	}, dashErr: func(p []string) error {
		if len(p) == 1 && p[0] == "Forbidden" {
			return &apierror.Error{StatusCode: 403}
		}
		return nil
	}}
	s := &fakeSender{}
	if err := Send(f, s, Options{Cadence: Weekly, EmailsEnabled: true})(context.Background()); err != nil {
		t.Fatalf("permanent failures must not be returned: %v", err)
	}
	if len(s.mails) != 1 || s.mails[0].to[0] != "good@x.invalid" || f.calls[len(f.calls)-1].products[0] != "Good" {
		t.Errorf("mails = %+v calls = %+v", s.mails, f.calls)
	}
}

func TestTransientFailuresAreReturned(t *testing.T) {
	for _, code := range []int{408, 429, 500, 503} {
		f := &fakeFetcher{dashboard: weekDash(), leads: []productissuereport.Lead{{Email: "l@x.invalid", ProductNames: []string{"A"}}},
			dashErr: func(p []string) error { return &apierror.Error{StatusCode: code} }}
		if err := Send(f, &fakeSender{}, Options{Cadence: Weekly, EmailsEnabled: true})(context.Background()); err == nil {
			t.Errorf("%d must be returned", code)
		}
	}
	// A permanent 4xx on the recipients fetch is not returned.
	f := &fakeFetcher{leadsErr: &apierror.Error{StatusCode: 400}}
	if err := Send(f, &fakeSender{}, Options{Cadence: Weekly, EmailsEnabled: true})(context.Background()); err != nil {
		t.Errorf("400 recipients: %v", err)
	}
	// A permanent 4xx on the all-products report is not returned either.
	f = &fakeFetcher{dashErr: func([]string) error { return &apierror.Error{StatusCode: 400} }}
	if err := Send(f, &fakeSender{}, Options{Cadence: Weekly, To: []string{"a@x.invalid"}, EmailsEnabled: true})(context.Background()); err != nil {
		t.Errorf("400 report: %v", err)
	}
}

func TestScopeEchoMustMatch(t *testing.T) {
	// Case and order are ignored.
	f := &fakeFetcher{dashboard: weekDash(), leads: []productissuereport.Lead{{Email: "l@x.invalid", ProductNames: []string{"A", "B"}}},
		echo: func([]string) []string { return []string{"b", "a"} }}
	s := &fakeSender{}
	if err := Send(f, s, Options{Cadence: Weekly, EmailsEnabled: true})(context.Background()); err != nil || len(s.mails) != 1 {
		t.Fatalf("err=%v mails=%d", err, len(s.mails))
	}
	// A lead's report whose echo differs is not sent, and not returned.
	f.echo = func([]string) []string { return []string{"A"} }
	s = &fakeSender{}
	if err := Send(f, s, Options{Cadence: Weekly, EmailsEnabled: true})(context.Background()); err != nil || len(s.mails) != 0 {
		t.Errorf("mismatch: err=%v mails=%d", err, len(s.mails))
	}
	// The all-products report must echo no products.
	f = &fakeFetcher{dashboard: weekDash(), echo: func([]string) []string { return []string{"A"} }}
	s = &fakeSender{}
	if err := Send(f, s, Options{Cadence: Weekly, To: []string{"a@x.invalid"}, EmailsEnabled: true})(context.Background()); err != nil || len(s.mails) != 0 {
		t.Errorf("all-products mismatch: err=%v mails=%d", err, len(s.mails))
	}
}
