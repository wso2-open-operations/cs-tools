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

package sweep

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

var screenNow = time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)

// screenInvoice is one invoice row of the fake copy.
type screenInvoice struct {
	id, opp, due, paid string
}

// screenFixture is a CSM copy with projects (listed as given), invoices and
// opportunity links, served page by page like entity-service.
type screenFixture struct {
	projects    []string // raw project JSON
	invoices    []screenInvoice
	links       map[string][]string // opportunity id -> project ids
	invoicesErr error
	pageLimit   int // when set, the invoice listing never ends (page cap)
}

func (f *screenFixture) reader() *mockEntityReader {
	r := &mockEntityReader{
		searchProjectsFn: func(context.Context, []byte) ([]byte, error) {
			return []byte(`{"projects":[` + strings.Join(f.projects, ",") + `],"total":` + fmt.Sprint(len(f.projects)) + `,"hasMore":false}`), nil
		},
	}
	r.searchInvoicesFn = func(_ context.Context, body []byte) ([]byte, error) {
		if f.invoicesErr != nil {
			return nil, f.invoicesErr
		}
		var req searchInvoicesRequest
		_ = json.Unmarshal(body, &req)
		if f.pageLimit > 0 { // endless listing
			return []byte(fmt.Sprintf(`{"invoices":[{"id":"x%d"}],"total":0,"hasMore":true}`, req.Pagination.Offset)), nil
		}
		var rows []string
		for _, inv := range f.invoices {
			row := `{"id":"` + inv.id + `","opportunity":{"id":"` + inv.opp + `"}`
			if inv.due != "" {
				row += `,"invoicedDueDate":"` + inv.due + `"`
			}
			if inv.paid != "" {
				row += `,"invoicedPaidDate":"` + inv.paid + `"`
			}
			rows = append(rows, row+`}`)
		}
		end := min(req.Pagination.Offset+req.Pagination.Limit, len(rows))
		start := min(req.Pagination.Offset, len(rows))
		return []byte(`{"invoices":[` + strings.Join(rows[start:end], ",") + `],"total":` + fmt.Sprint(len(rows)) + `,"hasMore":` + fmt.Sprint(end < len(rows)) + `}`), nil
	}
	r.searchProjectOpportunityLinksFn = func(_ context.Context, body []byte) ([]byte, error) {
		var req searchProjectOpportunityLinksRequest
		_ = json.Unmarshal(body, &req)
		var rows []string
		for _, pid := range f.links[req.OpportunityID] {
			rows = append(rows, `{"id":"l","project":{"id":"`+pid+`"},"opportunity":{"id":"`+req.OpportunityID+`"}}`)
		}
		return []byte(`{"links":[` + strings.Join(rows, ",") + `],"total":` + fmt.Sprint(len(rows)) + `,"hasMore":false}`), nil
	}
	return r
}

func screenProject(id string, endInDays int, state string) string {
	end := ""
	if endInDays != -9999 {
		end = `,"endDate":"` + screenNow.AddDate(0, 0, endInDays).Format(time.RFC3339) + `"`
	}
	st := ""
	if state != "" {
		st = `,"suspensionProcessState":` + state
	}
	return `{"id":"` + id + `","suspensionProcessOverridden":false` + end + st + `}`
}

func dateIn(days int) string { return screenNow.AddDate(0, 0, days).Format("2006-01-02") }

const noEndDate = -9999

func TestScreen_CandidateReasons(t *testing.T) {
	for _, tc := range []struct {
		name       string
		project    string
		invoices   []screenInvoice
		wantReason string // "" = screened out
	}{
		{"a: no end date", screenProject("p1", noEndDate, ""), nil, reasonDate},
		{"a: end date within the horizon", screenProject("p1", 80, ""), nil, reasonDate},
		{"a: end date in the past", screenProject("p1", -30, ""), nil, reasonDate},
		{"a: end date exactly at the horizon edge", screenProject("p1", screeningHorizonDays, ""), nil, reasonDate},
		{"b: recorded notice far from the end date", screenProject("p1", 400, `{"based_on_subscription_end_date":{"event_type":"30_days_notice"}}`), nil, reasonState},
		{"b: recorded invoice event far from the end date", screenProject("p1", 400, `{"based_on_due_invoices":{"event_type":"60_days_notice"}}`), nil, reasonState},
		{"b: unknown event type", screenProject("p1", 400, `{"based_on_due_invoices":{"event_type":"something_new"}}`), nil, reasonState},
		{"b: unparseable state", screenProject("p1", 400, `"not-an-object"`), nil, reasonState},
		{"far, state open only: screened out", screenProject("p1", 400, `{"based_on_subscription_end_date":{"event_type":"open"}}`), nil, ""},
		{"far, nothing recorded, no invoices: screened out", screenProject("p1", 400, ""), nil, ""},
		{"c: unpaid invoice due in 30 days", screenProject("p1", 400, ""), []screenInvoice{{"i1", "o1", dateIn(30), ""}}, reasonInvoice},
		{"c: unpaid invoice already overdue", screenProject("p1", 400, ""), []screenInvoice{{"i1", "o1", dateIn(-200), ""}}, reasonInvoice},
		{"paid invoice only: screened out", screenProject("p1", 400, ""), []screenInvoice{{"i1", "o1", dateIn(30), dateIn(-1)}}, ""},
		{"unpaid invoice due in 120 days: screened out", screenProject("p1", 400, ""), []screenInvoice{{"i1", "o1", dateIn(120), ""}}, ""},
		{"invoice with no due date: screened out", screenProject("p1", 400, ""), []screenInvoice{{"i1", "o1", "", ""}}, ""},
		{"due invoice of an opportunity linked to another project", screenProject("p1", 400, ""), []screenInvoice{{"i1", "o2", dateIn(30), ""}}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &screenFixture{projects: []string{tc.project}, invoices: tc.invoices,
				links: map[string][]string{"o1": {"p1"}, "o2": {"p-other"}}}
			reader := f.reader()
			var listed searchProjectsResponse
			raw, _ := reader.SearchProjects(context.Background(), nil)
			if err := json.Unmarshal(raw, &listed); err != nil {
				t.Fatal(err)
			}
			got := screen(context.Background(), reader, listed.Projects, screenNow).candidates["p1"]
			if got != tc.wantReason {
				t.Errorf("reason = %q, want %q", got, tc.wantReason)
			}
		})
	}
}

// A far project with nothing due is screened out with no re-read and no
// Salesforce-backed call; one with an invoice due soon goes through the live
// path, which re-reads it.
func TestRun_ScreenedOutProjectCostsNoLiveCalls(t *testing.T) {
	f := &screenFixture{
		projects: []string{screenProject("far", 400, ""), screenProject("invoiced", 400, "")},
		invoices: []screenInvoice{{"i1", "o1", dateIn(30), ""}},
		links:    map[string][]string{"o1": {"invoiced"}},
	}
	reader := f.reader()
	result, err := RunWith(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, &mockNotifier{}, screenNow, "", nil, Options{Screening: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ProjectsScreenedOut != 1 || result.ProjectsEvaluated != 1 || result.ScreenCandidates[reasonInvoice] != 1 {
		t.Errorf("result = %+v, want 1 screened out, 1 evaluated by invoice", result)
	}
	if strings.Join(reader.getProjectCalls, ",") != "invoiced" {
		t.Errorf("re-reads = %v, want only the candidate", reader.getProjectCalls)
	}
	if len(reader.getAccountCalls) != 0 && reader.getAccountCalls[0] == "far" {
		t.Errorf("the screened-out project was read live")
	}
}

func TestRun_ScreeningOffIsUnchanged(t *testing.T) {
	f := &screenFixture{projects: []string{screenProject("far", 400, ""), screenProject("near", 10, "")}}
	reader := f.reader()
	result, err := RunWith(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, delivering(), screenNow, "", nil, Options{})
	if err != nil || result.ProjectsScreenedOut != 0 || result.ProjectsEvaluated != 2 || result.ScreenCandidates != nil {
		t.Errorf("err=%v result=%+v, want both evaluated and no screening", err, result)
	}
	if len(reader.getProjectCalls) != 2 || len(reader.searchInvoicesCalls) != 0 {
		t.Errorf("re-reads=%v invoice reads=%d, want 2 and 0", reader.getProjectCalls, len(reader.searchInvoicesCalls))
	}
}

// A recorded state far from the end date still reaches the renewal reset.
func TestRun_ScreeningKeepsTheRenewalReset(t *testing.T) {
	f := &screenFixture{projects: []string{screenProject("renewed", 390, `{"based_on_subscription_end_date":{"event_type":"30_days_notice"}}`)}}
	reader := f.reader()
	updater := &mockProjectUpdater{}
	result, err := RunWith(context.Background(), reader, CSMDetails(reader), updater, &mockNotifier{}, screenNow, "", nil, Options{Screening: true})
	if err != nil || result.ProjectsScreenedOut != 0 || result.ScreenCandidates[reasonState] != 1 {
		t.Fatalf("err=%v result=%+v", err, result)
	}
	if got := savedEventTypes(t, updater.calls)["based_on_subscription_end_date"]; got != "open" {
		t.Errorf("end-date event_type = %q, want the reset to open; calls %+v", got, updater.calls)
	}
}

// Any read problem makes every project a candidate and logs a warning.
func TestRun_ScreeningFailsOpen(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    *screenFixture
	}{
		{"read error", &screenFixture{invoicesErr: errors.New("entity-service unavailable")}},
		{"page cap", &screenFixture{pageLimit: 1}},
		{"incomplete listing", &screenFixture{invoices: []screenInvoice{{"i1", "o1", dateIn(500), ""}}}},
		{"unparseable due date", &screenFixture{invoices: []screenInvoice{{"i1", "o1", "not-a-date", ""}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.f.projects = []string{screenProject("a", 400, ""), screenProject("b", 500, "")}
			if tc.name == "incomplete listing" {
				// Reports a total of 2 but lists one invoice.
				r := tc.f.reader()
				orig := r.searchInvoicesFn
				r.searchInvoicesFn = func(c context.Context, b []byte) ([]byte, error) {
					raw, err := orig(c, b)
					return bytes.Replace(raw, []byte(`"total":1`), []byte(`"total":2`), 1), err
				}
				runFailOpen(t, r)
				return
			}
			runFailOpen(t, tc.f.reader())
		})
	}
}

func runFailOpen(t *testing.T, reader *mockEntityReader) {
	t.Helper()
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(old)

	result, err := RunWith(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, &mockNotifier{}, screenNow, "", nil, Options{Screening: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ProjectsScreenedOut != 0 || result.ProjectsEvaluated != 2 || result.ScreenCandidates[reasonOpen] != 2 {
		t.Errorf("result = %+v, want every project live (fail open)", result)
	}
	if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "fail open") {
		t.Errorf("no fail-open WARN in the log: %s", logs.String())
	}
}

// A single-project run is never screened.
func TestRun_ScreeningNeverAppliesToASingleProject(t *testing.T) {
	f := &screenFixture{projects: []string{screenProject("far", 400, "")}}
	reader := f.reader()
	reader.getProjectFn = func(context.Context, string) ([]byte, error) { return []byte(screenProject("far", 400, "")), nil }
	result, err := RunWith(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, &mockNotifier{}, screenNow, "far", nil, Options{Screening: true})
	if err != nil || result.ProjectsEvaluated != 1 || result.ProjectsScreenedOut != 0 || result.ScreenCandidates != nil || len(reader.searchInvoicesCalls) != 0 {
		t.Errorf("err=%v result=%+v invoiceReads=%d", err, result, len(reader.searchInvoicesCalls))
	}
}

// Screened-out projects are not evaluated and do not use the daily limit.
func TestRun_ScreenedOutProjectsDoNotConsumeTheLimit(t *testing.T) {
	f := &screenFixture{projects: []string{
		screenProject("far1", 400, ""), screenProject("far2", 400, ""),
		screenProject("due1", 89, ""), screenProject("due2", 89, ""),
	}}
	reader := f.reader()
	result, err := RunWith(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, delivering(), screenNow, "", nil, Options{Screening: true, Limit: 1})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ProjectsScreenedOut != 2 || result.ProjectsActed != 1 || result.NotEvaluatedByLimit != 1 {
		t.Errorf("screenedOut=%d acted=%d notEvaluated=%d, want 2/1/1", result.ProjectsScreenedOut, result.ProjectsActed, result.NotEvaluatedByLimit)
	}
}

// Screening off restores today's behaviour exactly: the same projects, the
// same writes and notices, no invoice listing, as with screening on and
// nothing screened out.
func TestRun_ScreeningOffMatchesScreeningOnWhenNothingIsScreenedOut(t *testing.T) {
	run := func(screening bool) (Result, *mockProjectUpdater, *mockNotifier, *mockEntityReader) {
		f := &screenFixture{projects: []string{screenProject("a", 10, ""), screenProject("b", 89, "")}}
		reader := f.reader()
		updater, ntf := &mockProjectUpdater{}, delivering()
		result, err := RunWith(context.Background(), reader, CSMDetails(reader), updater, ntf, screenNow, "", nil, Options{Screening: screening})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return result, updater, ntf, reader
	}
	off, offUp, offNtf, offReader := run(false)
	on, onUp, onNtf, _ := run(true)
	if off.ProjectsEvaluated != on.ProjectsEvaluated || off.ProjectsActed != on.ProjectsActed || len(offUp.calls) != len(onUp.calls) || len(offNtf.sent) != len(onNtf.sent) {
		t.Errorf("off %+v (%d writes, %d notices) differs from on %+v (%d, %d)", off, len(offUp.calls), len(offNtf.sent), on, len(onUp.calls), len(onNtf.sent))
	}
	if len(offReader.searchInvoicesCalls) != 0 {
		t.Error("screening off listed invoices")
	}
}
