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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/viewer"
)

func (m *mockReportsClient) GetSLAReport(ctx context.Context, projectID, from, to string) (viewer.SLAReportDetails, error) {
	m.gotProjectID = projectID
	m.gotSLAFrom, m.gotSLATo = from, to
	return m.slaReport, m.slaErr
}

const slaReportRoute = "/generate-sla-report"

func TestGenerateSLAReport_PassesTheReportThrough(t *testing.T) {
	mock := &mockReportsClient{slaReport: viewer.SLAReportDetails{
		ProjectName: "Acme",
		ProjectKey:  "ACME",
		PeriodStart: "2026-01-01",
		PeriodEnd:   "2026-01-31",
		Summary:     []viewer.SLAReportSummaryRow{{Label: "Incident - Critical (P1)", Cases: 2}},
		Cases:       []viewer.SLAReportCase{{CaseNumber: "CS0001", Response: viewer.SLAReportClock{Elapsed: "1 Hour", Status: "Met"}}},
	}}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withUser(httptest.NewRequest(http.MethodGet, slaReportRoute+"?projectId=ACME&from=2026-01-01&to=2026-01-31", nil))
	w := httptest.NewRecorder()

	h.GenerateSLAReport(w, req)

	assertStatus(t, w, http.StatusOK)
	if mock.gotProjectID != "ACME" || mock.gotSLAFrom != "2026-01-01" || mock.gotSLATo != "2026-01-31" {
		t.Errorf("client called with %q %q %q", mock.gotProjectID, mock.gotSLAFrom, mock.gotSLATo)
	}
	got := decodeJSON[viewer.SLAReportDetails](t, w)
	if got.ProjectKey != "ACME" || len(got.Summary) != 1 || got.Cases[0].Response.Status != "Met" {
		t.Errorf("response = %+v, want the mock's report passed through", got)
	}
}

func TestGenerateSLAReport_RejectsABadRequestWithoutCallingTheClient(t *testing.T) {
	cases := []struct {
		name, query string
	}{
		{"no params", ""},
		{"no project", "?from=2026-01-01&to=2026-01-31"},
		{"no from", "?projectId=ACME&to=2026-01-31"},
		{"no to", "?projectId=ACME&from=2026-01-01"},
		{"not a date", "?projectId=ACME&from=abc&to=2026-01-31"},
		{"month 13", "?projectId=ACME&from=2026-13-01&to=2026-01-31"},
		{"time of day", "?projectId=ACME&from=2026-01-01T00:00:00Z&to=2026-01-31"},
		{"from after to", "?projectId=ACME&from=2026-02-01&to=2026-01-31"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockReportsClient{}
			h := NewReportsHandler(mock, viewerAccessGuard)
			req := withUser(httptest.NewRequest(http.MethodGet, slaReportRoute+tc.query, nil))
			w := httptest.NewRecorder()

			h.GenerateSLAReport(w, req)

			assertStatus(t, w, http.StatusBadRequest)
			if mock.gotProjectID != "" {
				t.Error("client should not have been called")
			}
		})
	}
}

func TestGenerateSLAReport_AcceptsASingleDayRange(t *testing.T) {
	mock := &mockReportsClient{}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withUser(httptest.NewRequest(http.MethodGet, slaReportRoute+"?projectId=ACME&from=2026-01-15&to=2026-01-15", nil))
	w := httptest.NewRecorder()

	h.GenerateSLAReport(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestGenerateSLAReport_AllowsCsEngineer(t *testing.T) {
	mock := &mockReportsClient{}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withCsEngineerUser(httptest.NewRequest(http.MethodGet, slaReportRoute+"?projectId=ACME&from=2026-01-01&to=2026-01-31", nil))
	w := httptest.NewRecorder()

	h.GenerateSLAReport(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestGenerateSLAReport_RejectsACallerWithoutAccess(t *testing.T) {
	mock := &mockReportsClient{}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := httptest.NewRequest(http.MethodGet, slaReportRoute+"?projectId=ACME&from=2026-01-01&to=2026-01-31", nil)
	// Authenticated but holds no role that can read the portal.
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{Email: "nobody@example.com", UserID: "u-nobody"}))
	w := httptest.NewRecorder()

	h.GenerateSLAReport(w, req)

	assertStatus(t, w, http.StatusForbidden)
	if mock.gotProjectID != "" {
		t.Error("client should not have been called for a caller without access")
	}
}

func TestGenerateSLAReport_UnknownProjectIsNotFound(t *testing.T) {
	mock := &mockReportsClient{slaErr: viewer.ErrProjectNotFound}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withUser(httptest.NewRequest(http.MethodGet, slaReportRoute+"?projectId=missing&from=2026-01-01&to=2026-01-31", nil))
	w := httptest.NewRecorder()

	h.GenerateSLAReport(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestGenerateSLAReport_UpstreamFailureDoesNotLeak(t *testing.T) {
	mock := &mockReportsClient{slaErr: &apierror.Error{StatusCode: http.StatusInternalServerError, Body: `{"message":"pq: relation sla_live secret detail"}`}}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withUser(httptest.NewRequest(http.MethodGet, slaReportRoute+"?projectId=ACME&from=2026-01-01&to=2026-01-31", nil))
	w := httptest.NewRecorder()

	h.GenerateSLAReport(w, req)

	assertStatus(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "sla_live") || strings.Contains(w.Body.String(), "secret") {
		t.Errorf("the upstream body leaked: %s", w.Body.String())
	}
}

const slaReportTestUUID = "90269291-b2c3-46cb-9214-ee52c70f861d"

func slaReportProjectJSON() []byte {
	return []byte(`{"id":"` + slaReportTestUUID + `","name":"Acme Production","key":"ACME"}`)
}

func TestPostgresGetSLAReport_ReshapesTheEntityResponse(t *testing.T) {
	mock := &mockEntityReportsClient{
		getProjectFn: func(_ context.Context, id string) ([]byte, error) {
			if id != slaReportTestUUID {
				t.Errorf("GetProject id = %q", id)
			}
			return slaReportProjectJSON(), nil
		},
		getProjectSLAReportFn: func(_ context.Context, id, from, to string) ([]byte, error) {
			if id != slaReportTestUUID || from != "2026-01-01" || to != "2026-01-31" {
				t.Errorf("entity call = %s %s %s", id, from, to)
			}
			return []byte(`{
				"project":{"id":"` + slaReportTestUUID + `","key":"ACME","name":"Acme Production"},
				"from":"2026-01-01","to":"2026-01-31",
				"summary":[{"label":"Incident - Critical (P1)","caseType":"incident","severity":"S1","cases":2,
					"response":{"elapsed":"2 Hours","target":"4 Hours","met":1,"missed":1,"running":0},
					"workaround":{"elapsed":"—","target":"Not applicable","met":0,"missed":0,"running":0},
					"resolution":{"elapsed":"—","target":"Not applicable","met":0,"missed":0,"running":0}}],
				"cases":[{"caseId":"c1","caseNumber":"CS1","caseType":"Incident","severity":"Critical (P1)","state":"Open",
					"openedOn":"2026-01-05T08:00:00Z",
					"response":{"elapsed":"1 Hour","status":"Met"},
					"workaround":{"elapsed":"—","status":""},
					"resolution":{"elapsed":"—","status":""}}]}`), nil
		},
	}

	got, err := NewPostgresReportsClient(mock).GetSLAReport(context.Background(), slaReportTestUUID, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("GetSLAReport: %v", err)
	}
	if got.ProjectName != "Acme Production" || got.ProjectKey != "ACME" || got.PeriodStart != "2026-01-01" || got.PeriodEnd != "2026-01-31" {
		t.Errorf("header = %+v", got)
	}
	if len(got.Summary) != 1 || got.Summary[0].Response.Target != "4 Hours" || got.Summary[0].Response.Missed != 1 || got.Summary[0].Cases != 2 {
		t.Errorf("summary = %+v", got.Summary)
	}
	if len(got.Cases) != 1 || got.Cases[0].CaseNumber != "CS1" || got.Cases[0].Response.Status != "Met" {
		t.Errorf("cases = %+v", got.Cases)
	}
}

func TestPostgresGetSLAReport_EmptyListsStayEmptyArrays(t *testing.T) {
	mock := &mockEntityReportsClient{
		getProjectFn: func(context.Context, string) ([]byte, error) { return slaReportProjectJSON(), nil },
		getProjectSLAReportFn: func(context.Context, string, string, string) ([]byte, error) {
			return []byte(`{"project":{"id":"x","key":"ACME","name":null},"from":"2026-01-01","to":"2026-01-31"}`), nil
		},
	}
	got, err := NewPostgresReportsClient(mock).GetSLAReport(context.Background(), slaReportTestUUID, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("GetSLAReport: %v", err)
	}
	if got.Summary == nil || got.Cases == nil {
		t.Errorf("summary and cases must be [] and not null: %+v", got)
	}
	// a null project name falls back to the one resolved first
	if got.ProjectName != "Acme Production" {
		t.Errorf("project name = %q", got.ProjectName)
	}
}

func TestPostgresGetSLAReport_UnknownProjectUUIDIsNotFound(t *testing.T) {
	mock := &mockEntityReportsClient{
		getProjectFn: func(context.Context, string) ([]byte, error) {
			return nil, &apierror.Error{StatusCode: http.StatusNotFound, Body: `{"message":"project not found"}`}
		},
	}
	_, err := NewPostgresReportsClient(mock).GetSLAReport(context.Background(), slaReportTestUUID, "2026-01-01", "2026-01-31")
	if !errors.Is(err, viewer.ErrProjectNotFound) {
		t.Fatalf("err = %v, want viewer.ErrProjectNotFound", err)
	}
}

func TestPostgresGetSLAReport_EntityNotFoundIsNotFound(t *testing.T) {
	mock := &mockEntityReportsClient{
		getProjectFn: func(context.Context, string) ([]byte, error) { return slaReportProjectJSON(), nil },
		getProjectSLAReportFn: func(context.Context, string, string, string) ([]byte, error) {
			return nil, &apierror.Error{StatusCode: http.StatusNotFound, Body: `{"message":"project not found"}`}
		},
	}
	_, err := NewPostgresReportsClient(mock).GetSLAReport(context.Background(), slaReportTestUUID, "2026-01-01", "2026-01-31")
	if !errors.Is(err, viewer.ErrProjectNotFound) {
		t.Fatalf("err = %v, want viewer.ErrProjectNotFound", err)
	}
}

func TestPostgresGetSLAReport_ABadEntityBodyIsAnError(t *testing.T) {
	mock := &mockEntityReportsClient{
		getProjectFn: func(context.Context, string) ([]byte, error) { return slaReportProjectJSON(), nil },
		getProjectSLAReportFn: func(context.Context, string, string, string) ([]byte, error) {
			return []byte(`not json`), nil
		},
	}
	if _, err := NewPostgresReportsClient(mock).GetSLAReport(context.Background(), slaReportTestUUID, "2026-01-01", "2026-01-31"); err == nil {
		t.Fatal("expected an error for a body that is not JSON")
	}
}

func TestPostgresGetSLAReport_AcceptsAProjectKey(t *testing.T) {
	mock := &mockEntityReportsClient{
		searchProjectsFn: func(context.Context, []byte) ([]byte, error) {
			return []byte(`{"projects":[{"id":"` + slaReportTestUUID + `","name":"Acme Production","key":"ACME"}],"total":1}`), nil
		},
		getProjectFn: func(context.Context, string) ([]byte, error) { return slaReportProjectJSON(), nil },
		getProjectSLAReportFn: func(_ context.Context, id, _, _ string) ([]byte, error) {
			if id != slaReportTestUUID {
				t.Errorf("the key must be resolved to the project id, got %q", id)
			}
			return []byte(`{"project":{"id":"x","key":"ACME","name":"Acme Production"},"from":"2026-01-01","to":"2026-01-31","summary":[],"cases":[]}`), nil
		},
	}
	if _, err := NewPostgresReportsClient(mock).GetSLAReport(context.Background(), "ACME", "2026-01-01", "2026-01-31"); err != nil {
		t.Fatalf("GetSLAReport: %v", err)
	}
}
