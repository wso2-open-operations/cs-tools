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

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/viewer"
)

// timelogFixture builds an entity mock for one project with the given
// project JSON, cases and time cards (single page each).
func timelogFixture(t *testing.T, projectJSON string, cases []entitySearchCaseView, cards []entityTimeCardView) *mockEntityReportsClient {
	t.Helper()
	return &mockEntityReportsClient{
		searchProjectsFn: func(context.Context, []byte) ([]byte, error) {
			return jsonBytesReports(t, entitySearchProjectsResponse{
				Total: 1, Projects: []entityProjectView{{ID: "proj-uuid-1", Key: "PRJ001"}},
			}), nil
		},
		getProjectFn: func(context.Context, string) ([]byte, error) { return []byte(projectJSON), nil },
		searchCasesPages: [][]byte{
			jsonBytesReports(t, entitySearchCasesResponse{Total: len(cases), Cases: cases}),
		},
		searchTimeCardsPages: [][]byte{
			jsonBytesReports(t, entitySearchTimeCardsResponse{Total: len(cards), TimeCards: cards}),
		},
	}
}

func tlCard(id string, minutes float64, workDate, caseID string, billable bool, state string) entityTimeCardView {
	return entityTimeCardView{
		ID: id, TotalTime: minutes, WorkDate: workDate, HasBillable: billable, State: &state,
		Case: &entityTimeCardRef{ID: caseID},
	}
}

// Entity-service reports a time card's totalTime in minutes: 90 is "1h 30m",
// not "90h 0m".
func TestGetTimeLogBreakdown_TotalTimeIsMinutes(t *testing.T) {
	cases := []entitySearchCaseView{{ID: "c1", Number: "CS1", CreatedOn: "2026-01-01T00:00:00Z"}}
	cards := []entityTimeCardView{
		tlCard("t1", 90, "2026-01-02", "c1", true, "approved"),
		tlCard("t2", 365, "2026-01-03", "c1", false, "pending"),
	}
	c := NewPostgresReportsClient(timelogFixture(t, `{"id":"proj-uuid-1","key":"PRJ001"}`, cases, cards))
	got, err := c.GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{})
	if err != nil {
		t.Fatal(err)
	}
	cr := got.Cases[0]
	if cr.TimeCards[0].Total != "1h 30m" || cr.TimeCards[1].Total != "6h 5m" {
		t.Errorf("card totals = %q, %q; want 1h 30m, 6h 5m", cr.TimeCards[0].Total, cr.TimeCards[1].Total)
	}
	if cr.TotalHours != "7h 35m" {
		t.Errorf("TotalHours = %q, want 7h 35m", cr.TotalHours)
	}
	if cr.ConsumedQueryHours != "1h 30m" {
		t.Errorf("ConsumedQueryHours = %q, want 1h 30m (approved billable only)", cr.ConsumedQueryHours)
	}
}

func TestFormatMinutes(t *testing.T) {
	for _, tc := range []struct {
		min  float64
		want string
	}{{0, "0m"}, {1, "1m"}, {59, "59m"}, {60, "1h 0m"}, {90, "1h 30m"}, {365, "6h 5m"},
		{-90, "-1h 30m"}, {-1, "-1m"}, {-29.6, "-30m"}, {-0.4, "0m"}, {-60, "-1h 0m"}} {
		if got := formatMinutes(tc.min); got != tc.want {
			t.Errorf("formatMinutes(%v) = %q, want %q", tc.min, got, tc.want)
		}
	}
}

func TestFormatHoursMinutes_Signed(t *testing.T) {
	for _, tc := range []struct {
		hours float64
		want  string
	}{{-1.5, "-1h 30m"}, {36.6, "36h 36m"}, {0, "0m"}, {-0.5, "-30m"}} {
		if got := formatHoursMinutes(tc.hours); got != tc.want {
			t.Errorf("formatHoursMinutes(%v) = %q, want %q", tc.hours, got, tc.want)
		}
	}
}

func TestGetTimeLogBreakdown_InvertedEffectivePeriod(t *testing.T) {
	proj := `{"id":"proj-uuid-1","key":"PRJ001","startDate":"2026-01-01","endDate":"2026-03-31"}`
	get := func(p TimeLogPeriod) (viewer.TimeLogBreakdownDetails, error) {
		return NewPostgresReportsClient(periodFixture(t, proj)).GetTimeLogBreakdown(context.Background(), "PRJ001", p)
	}
	for name, p := range map[string]TimeLogPeriod{
		"only from after project end":  {From: "2026-04-15"},
		"only to before project start": {To: "2025-12-15"},
	} {
		if _, err := get(p); !errors.Is(err, viewer.ErrInvalidTimeLogPeriod) {
			t.Errorf("%s: err = %v, want ErrInvalidTimeLogPeriod", name, err)
		}
	}
	for name, p := range map[string]TimeLogPeriod{
		"only from within project":          {From: "2026-03-01"},
		"only to within project":            {To: "2026-02-01"},
		"from equals project end":           {From: "2026-03-31"},
		"allTime with only from after end":  {From: "2026-04-15", AllTime: true},
		"allTime with only to before start": {To: "2025-12-15", AllTime: true},
	} {
		if _, err := get(p); err != nil {
			t.Errorf("%s: unexpected err %v", name, err)
		}
	}
	// A project whose own dates are inverted, with no user bound, stays empty.
	inv := `{"id":"proj-uuid-1","key":"PRJ001","startDate":"2026-05-01","endDate":"2026-03-31"}`
	got, err := NewPostgresReportsClient(periodFixture(t, inv)).GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{})
	if err != nil || len(got.Cases) != 0 {
		t.Errorf("inverted project dates, no bounds: err=%v cases=%d, want nil/0", err, len(got.Cases))
	}
	// With a user bound the inverted effective range is rejected.
	if _, err := NewPostgresReportsClient(periodFixture(t, inv)).GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{From: "2026-04-01"}); !errors.Is(err, viewer.ErrInvalidTimeLogPeriod) {
		t.Errorf("inverted effective with user bound: err = %v", err)
	}
}

func TestGenerateTimelogsBreakdownReport_InvalidEffectivePeriodIs400(t *testing.T) {
	h := NewReportsHandler(&mockReportsClient{timelogsErr: viewer.ErrInvalidTimeLogPeriod}, viewerAccessGuard)
	req := withCsEngineerUser(httptest.NewRequest(http.MethodGet, "/generate-timelogs-breakdown-report?projectId=ACME&from=2026-04-15", nil))
	w := httptest.NewRecorder()
	h.GenerateTimelogsBreakdownReport(w, req)
	assertStatus(t, w, http.StatusBadRequest)
}

// Many small cards must sum as raw minutes: 100 x 0.4 min is 40m, whereas
// summing each card's rounded value (0m) would give 0m.
func TestGetTimeLogBreakdown_SumsMinutesWithoutRoundingDrift(t *testing.T) {
	cases := []entitySearchCaseView{{ID: "c1", Number: "CS1", CreatedOn: "2026-01-01T00:00:00Z"}}
	var cards []entityTimeCardView
	for i := 0; i < 100; i++ {
		cards = append(cards, tlCard("t", 0.4, "2026-01-02", "c1", true, "approved"))
	}
	c := NewPostgresReportsClient(timelogFixture(t, `{"id":"proj-uuid-1","key":"PRJ001"}`, cases, cards))
	got, err := c.GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cases[0].TotalHours != "40m" || got.Cases[0].ConsumedQueryHours != "40m" {
		t.Errorf("total/consumed = %q/%q, want 40m/40m", got.Cases[0].TotalHours, got.Cases[0].ConsumedQueryHours)
	}
}

func periodFixture(t *testing.T, project string) *mockEntityReportsClient {
	cases := []entitySearchCaseView{
		{ID: "c-in", Number: "IN", CreatedOn: "2025-12-01T10:00:00Z"},
		{ID: "c-created", Number: "CREATED", CreatedOn: "2026-03-31T23:59:59Z"},
		{ID: "c-out", Number: "OUT", CreatedOn: "2025-12-31T23:59:59Z"},
		{ID: "c-late", Number: "LATE", CreatedOn: "2025-12-01T10:00:00Z"},
	}
	cards := []entityTimeCardView{
		tlCard("a", 60, "2026-01-01", "c-in", true, "approved"),   // first day: in
		tlCard("b", 30, "2025-12-31", "c-in", true, "approved"),   // day before: out
		tlCard("c", 15, "2026-03-31", "c-in", true, "approved"),   // last day: in
		tlCard("d", 45, "2026-04-01", "c-late", true, "approved"), // day after: out
		tlCard("e", 10, "2025-12-31", "c-out", true, "approved"),  // out
	}
	return timelogFixture(t, project, cases, cards)
}

func joinStr(r []string) string {
	out := ""
	for i, s := range r {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func TestGetTimeLogBreakdown_PeriodFiltersInclusiveBounds(t *testing.T) {
	c := NewPostgresReportsClient(periodFixture(t, `{"id":"proj-uuid-1","key":"PRJ001"}`))
	got, err := c.GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{From: "2026-01-01", To: "2026-03-31"})
	if err != nil {
		t.Fatal(err)
	}
	var nums []string
	for _, cr := range got.Cases {
		nums = append(nums, cr.CaseNumber)
	}
	// IN has in-period cards, CREATED was created on the last day, OUT and LATE have neither.
	if joinStr(nums) != "IN,CREATED" {
		t.Fatalf("cases = %v, want IN,CREATED", nums)
	}
	in := got.Cases[0]
	if len(in.TimeCards) != 2 || in.TotalHours != "1h 15m" {
		t.Errorf("IN cards=%d total=%q, want 2 cards and 1h 15m (in-period only)", len(in.TimeCards), in.TotalHours)
	}
	if got.PeriodStart != "2026-01-01" || got.PeriodEnd != "2026-03-31" {
		t.Errorf("period = %q..%q", got.PeriodStart, got.PeriodEnd)
	}
}

func TestGetTimeLogBreakdown_DefaultsToProjectSubscriptionDates(t *testing.T) {
	proj := `{"id":"proj-uuid-1","key":"PRJ001","startDate":"2026-01-01","endDate":"2026-03-31T00:00:00Z"}`
	c := NewPostgresReportsClient(periodFixture(t, proj))
	got, err := c.GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{})
	if err != nil {
		t.Fatal(err)
	}
	if got.PeriodStart != "2026-01-01" || got.PeriodEnd != "2026-03-31" || len(got.Cases) != 2 {
		t.Errorf("period = %q..%q cases = %d, want project dates and 2 cases", got.PeriodStart, got.PeriodEnd, len(got.Cases))
	}
	// An explicit bound overrides only its own side.
	c = NewPostgresReportsClient(periodFixture(t, proj))
	got, _ = c.GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{From: "2026-03-01"})
	if got.PeriodStart != "2026-03-01" || got.PeriodEnd != "2026-03-31" {
		t.Errorf("period = %q..%q, want 2026-03-01..2026-03-31", got.PeriodStart, got.PeriodEnd)
	}
}

func TestGetTimeLogBreakdown_NoProjectDatesMeansAllTime(t *testing.T) {
	c := NewPostgresReportsClient(periodFixture(t, `{"id":"proj-uuid-1","key":"PRJ001"}`))
	got, _ := c.GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{})
	if got.PeriodStart != "" || got.PeriodEnd != "" || len(got.Cases) != 4 {
		t.Errorf("period = %q..%q cases = %d, want unbounded and 4 cases", got.PeriodStart, got.PeriodEnd, len(got.Cases))
	}
}

func TestGetTimeLogBreakdown_AllTimeIgnoresProjectDates(t *testing.T) {
	proj := `{"id":"proj-uuid-1","key":"PRJ001","startDate":"2026-01-01","endDate":"2026-03-31"}`
	c := NewPostgresReportsClient(periodFixture(t, proj))
	got, _ := c.GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{AllTime: true})
	if got.PeriodStart != "" || got.PeriodEnd != "" || len(got.Cases) != 4 {
		t.Errorf("period = %q..%q cases = %d, want unbounded and 4 cases", got.PeriodStart, got.PeriodEnd, len(got.Cases))
	}
}

func TestGetTimeLogBreakdown_ProjectCardDisplay(t *testing.T) {
	proj := `{"id":"proj-uuid-1","key":"PRJ001","totalQueryHours":40,"remainingQueryHours":36.6}`
	c := NewPostgresReportsClient(periodFixture(t, proj))
	got, _ := c.GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{})
	if got.TotalQueryHours != "40h 0m" || got.RemainingQueryHours != "36h 36m" {
		t.Errorf("card = %q / %q, want 40h 0m / 36h 36m", got.TotalQueryHours, got.RemainingQueryHours)
	}
	c = NewPostgresReportsClient(periodFixture(t, `{"id":"proj-uuid-1","key":"PRJ001","totalQueryHours":40,"remainingQueryHours":-1.5}`))
	got, _ = c.GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{})
	if got.RemainingQueryHours != "-1h 30m" {
		t.Errorf("negative remaining = %q, want -1h 30m", got.RemainingQueryHours)
	}
	c = NewPostgresReportsClient(periodFixture(t, `{"id":"proj-uuid-1","key":"PRJ001"}`))
	got, _ = c.GetTimeLogBreakdown(context.Background(), "PRJ001", TimeLogPeriod{})
	if got.TotalQueryHours != "" || got.RemainingQueryHours != "" {
		t.Errorf("missing hours must stay empty, got %q / %q", got.TotalQueryHours, got.RemainingQueryHours)
	}
}

func TestGenerateTimelogsBreakdownReport_PeriodParams(t *testing.T) {
	do := func(query string) (*mockReportsClient, *httptest.ResponseRecorder) {
		mock := &mockReportsClient{}
		h := NewReportsHandler(mock, viewerAccessGuard)
		req := withCsEngineerUser(httptest.NewRequest(http.MethodGet, "/generate-timelogs-breakdown-report?projectId=ACME"+query, nil))
		w := httptest.NewRecorder()
		h.GenerateTimelogsBreakdownReport(w, req)
		return mock, w
	}
	mock, w := do("&from=2026-01-01&to=2026-03-31")
	assertStatus(t, w, http.StatusOK)
	if mock.gotPeriod != (TimeLogPeriod{From: "2026-01-01", To: "2026-03-31"}) {
		t.Errorf("period = %+v", mock.gotPeriod)
	}
	mock, w = do("&allTime=true")
	assertStatus(t, w, http.StatusOK)
	if !mock.gotPeriod.AllTime {
		t.Errorf("allTime not passed: %+v", mock.gotPeriod)
	}
	for _, q := range []string{"&from=2026-13-01", "&to=yesterday", "&from=2026-04-01&to=2026-03-31", "&allTime=maybe"} {
		mock, w = do(q)
		assertStatus(t, w, http.StatusBadRequest)
		if mock.gotProjectID != "" {
			t.Errorf("%s: upstream must not be called on a 400", q)
		}
	}
}
