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
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/viewer"
)

// mockEntityReportsClient implements entityReportsClient. searchCasesPages
// and searchTimeCardsPages let a test return a different response for each
// successive page (keyed by call order), so pagination itself can be
// exercised rather than just a single-page happy path.
type mockEntityReportsClient struct {
	searchProjectsFn func(ctx context.Context, body []byte) ([]byte, error)
	getProjectFn     func(ctx context.Context, id string) ([]byte, error)

	searchCasesPages     [][]byte
	searchCasesCallCount int

	searchTimeCardsPages     [][]byte
	searchTimeCardsCallCount int
}

func (m *mockEntityReportsClient) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	return m.searchProjectsFn(ctx, body)
}

func (m *mockEntityReportsClient) GetProject(ctx context.Context, id string) ([]byte, error) {
	return m.getProjectFn(ctx, id)
}

func (m *mockEntityReportsClient) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	page := m.searchCasesPages[m.searchCasesCallCount]
	m.searchCasesCallCount++
	return page, nil
}

func (m *mockEntityReportsClient) SearchDeployments(ctx context.Context, body []byte) ([]byte, error) {
	return []byte(`{"deployments":[],"total":0}`), nil
}

func (m *mockEntityReportsClient) SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error) {
	return []byte(`{"deployedProducts":[],"total":0}`), nil
}

func (m *mockEntityReportsClient) SearchTimeCards(ctx context.Context, body []byte) ([]byte, error) {
	var req entitySearchTimeCardsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	if len(req.Filters.ProjectIDs) != 1 {
		return nil, errors.New("time-card search must be scoped to one project")
	}
	page := m.searchTimeCardsPages[m.searchTimeCardsCallCount]
	m.searchTimeCardsCallCount++
	return page, nil
}

func jsonBytesReports(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// TestGetTimeLogBreakdown_PagesThroughAllCasesAndTimeCards verifies the
// fix for the finding that this report silently stopped at 50 cases and 50
// time cards per case: a project with more than entitySearchPageLimit cases,
// and a case with more than entitySearchPageLimit time cards, must both be
// fully paged through rather than truncated at the first page.
func TestGetTimeLogBreakdown_PagesThroughAllCasesAndTimeCards(t *testing.T) {
	entity := &mockEntityReportsClient{
		searchProjectsFn: func(context.Context, []byte) ([]byte, error) {
			return jsonBytesReports(t, entitySearchProjectsResponse{
				Total:    1,
				Projects: []entityProjectView{{ID: "proj-uuid-1", Key: "PRJ001"}},
			}), nil
		},
		getProjectFn: func(context.Context, string) ([]byte, error) {
			return jsonBytesReports(t, entityProjectDetailsView{
				ID: "proj-uuid-1", Name: "Acme Project", Key: "PRJ001", SubscriptionType: "Standard",
			}), nil
		},
		// entitySearchPageLimit (50) + 1 = 51 cases total, split across two pages.
		searchCasesPages: [][]byte{
			jsonBytesReports(t, entitySearchCasesResponse{
				Total: 51,
				Cases: makeCaseViews(0, 50, "case"),
			}),
			jsonBytesReports(t, entitySearchCasesResponse{
				Total: 51,
				Cases: makeCaseViews(50, 1, "case"),
			}),
		},
		// One project search, two pages. Cards land on the first case except the
		// last card, which lands on the last case, so grouping is checked too.
		searchTimeCardsPages: [][]byte{
			jsonBytesReports(t, entitySearchTimeCardsResponse{
				Total:     60,
				TimeCards: makeTimeCardViews(50, 1.0, caseIDFor(0, "case")),
			}),
			jsonBytesReports(t, entitySearchTimeCardsResponse{
				Total: 60,
				TimeCards: append(
					makeTimeCardViews(9, 1.0, caseIDFor(0, "case")),
					entityTimeCardView{ID: "tc-last", TotalTime: 1, WorkDate: "2026-01-01", Case: &entityTimeCardRef{ID: caseIDFor(50, "case")}},
				),
			}),
		},
	}

	c := NewPostgresReportsClient(entity)
	got, err := c.GetTimeLogBreakdown(context.Background(), "PRJ001")
	if err != nil {
		t.Fatalf("GetTimeLogBreakdown: %v", err)
	}
	if entity.searchTimeCardsCallCount != 2 {
		t.Fatalf("time-card searches = %d, want 2 (one project search, not one per case)", entity.searchTimeCardsCallCount)
	}
	if len(got.Cases) != 51 {
		t.Fatalf("len(Cases) = %d, want 51 (all cases across both pages, not truncated at 50)", len(got.Cases))
	}
	if len(got.Cases[0].TimeCards) != 59 || len(got.Cases[50].TimeCards) != 1 {
		t.Fatalf("time cards grouped as %d and %d, want 59 on the first case and 1 on the last", len(got.Cases[0].TimeCards), len(got.Cases[50].TimeCards))
	}
	if got.Cases[0].CaseSysID != caseIDFor(0, "case") {
		t.Fatalf("CaseSysID = %q", got.Cases[0].CaseSysID)
	}
}

func caseIDFor(i int, prefix string) string {
	return prefix + "-uuid-" + strconv.Itoa(i)
}

// makeCaseViews builds n views with globally-unique ids/numbers starting at
// start, so a second page picks up right where a first page of size start left off.
func makeCaseViews(start, n int, prefix string) []entitySearchCaseView {
	views := make([]entitySearchCaseView, n)
	for i := 0; i < n; i++ {
		idx := start + i
		views[i] = entitySearchCaseView{ID: caseIDFor(idx, prefix), Number: prefix + strconv.Itoa(idx), InternalID: "WSO2-" + strconv.Itoa(idx)}
	}
	return views
}

func makeTimeCardViews(n int, hours float64, caseID string) []entityTimeCardView {
	views := make([]entityTimeCardView, n)
	for i := 0; i < n; i++ {
		views[i] = entityTimeCardView{
			ID: "tc-" + strconv.Itoa(i), TotalTime: hours, WorkDate: "2026-01-01",
			Case: &entityTimeCardRef{ID: caseID},
		}
	}
	return views
}

func TestBuildCSReportCountsQueriesAndIncidents(t *testing.T) {
	total := 10.0
	consumed := 4.0
	question := "question"
	project := entityProjectDetailsView{
		Name: "Acme", Key: "ACME", SubscriptionType: "standard",
		Account:            entityProjectAccountRef{Name: "Acme Inc", Tier: "gold"},
		TotalQueryHours:    &total,
		ConsumedQueryHours: &consumed,
	}
	cases := []entitySearchCaseView{
		{ID: "c1", Number: "CS1", Type: "case", CreatedOn: "2026-03-02T00:00:00Z"},
		{ID: "c2", Number: "CS2", Type: "engagement", IssueType: &question, CreatedOn: "2026-03-15T00:00:00Z"},
	}

	got := buildCSReport(project, cases, []viewer.ProjectDeployment{})
	if got.SubscriptionDetails.SupportTier != "gold" || got.SubscriptionDetails.ConsumedQueryHours != "4" {
		t.Fatalf("subscription = %+v", got.SubscriptionDetails)
	}
	if len(got.CasesRecords) != 2 || got.CasesRecords[1].CaseType != "Query" {
		t.Fatalf("cases = %+v", got.CasesRecords)
	}
	if len(got.MonthlyCounts) != 1 || got.MonthlyCounts[0].Counts.IncidentCount != 1 || got.MonthlyCounts[0].Counts.QueryCount != 1 {
		t.Fatalf("monthly = %+v", got.MonthlyCounts)
	}
	if len(got.SlaDetails.SlaRecords) != 0 {
		t.Fatalf("sla records = %+v, want none", got.SlaDetails.SlaRecords)
	}
}

func TestFormatHoursMinutes(t *testing.T) {
	cases := []struct {
		hours float64
		want  string
	}{
		{0, "0m"},
		{0.5, "30m"},
		{1, "1h 0m"},
		{1.5, "1h 30m"},
		{2.0833, "2h 5m"}, // 125 minutes
		{0.0083, "0m"},    // under half a minute rounds down
		{0.0084, "1m"},    // just over half a minute rounds up
	}
	for _, tc := range cases {
		if got := formatHoursMinutes(tc.hours); got != tc.want {
			t.Errorf("formatHoursMinutes(%v) = %q, want %q", tc.hours, got, tc.want)
		}
	}
}

func TestGetTimeLogBreakdown_AcceptsProjectUUID(t *testing.T) {
	const projectID = "90269291-b2c3-46cb-9214-ee52c70f861d"
	mock := &mockEntityReportsClient{
		searchProjectsFn: func(context.Context, []byte) ([]byte, error) {
			t.Fatal("SearchProjects must not be called for a project UUID")
			return nil, nil
		},
		getProjectFn: func(_ context.Context, id string) ([]byte, error) {
			if id != projectID {
				t.Errorf("GetProject id = %q, want %q", id, projectID)
			}
			return []byte(`{"id":"` + projectID + `","name":"Solstice Platforms Production","key":"GEN02-PROJ1","subscriptionType":"managed_cloud_subscription"}`), nil
		},
		searchCasesPages: [][]byte{[]byte(`{"cases":[],"total":0}`)},
	}
	result, err := NewPostgresReportsClient(mock).GetTimeLogBreakdown(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetTimeLogBreakdown: %v", err)
	}
	if result.ProjectKey != "GEN02-PROJ1" || result.ProjectName != "Solstice Platforms Production" {
		t.Errorf("project = %q / %q", result.ProjectName, result.ProjectKey)
	}
}

func TestGetTimeLogBreakdown_UnknownProjectUUIDIsNotFound(t *testing.T) {
	mock := &mockEntityReportsClient{
		getProjectFn: func(context.Context, string) ([]byte, error) {
			return nil, &apierror.Error{StatusCode: http.StatusNotFound, Body: `{"message":"project not found"}`}
		},
	}
	_, err := NewPostgresReportsClient(mock).GetTimeLogBreakdown(context.Background(), "90269291-b2c3-46cb-9214-ee52c70f861d")
	if !errors.Is(err, viewer.ErrProjectNotFound) {
		t.Fatalf("err = %v, want viewer.ErrProjectNotFound", err)
	}
}
