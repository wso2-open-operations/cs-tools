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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/viewer"
)

type mockReportsClient struct {
	timelogs     viewer.TimeLogBreakdownDetails
	timelogsErr  error
	csReport     viewer.CSReportDetails
	csErr        error
	gotProjectID string
}

func (m *mockReportsClient) GetTimeLogBreakdown(ctx context.Context, projectID string) (viewer.TimeLogBreakdownDetails, error) {
	m.gotProjectID = projectID
	return m.timelogs, m.timelogsErr
}

func (m *mockReportsClient) GetProjectReportDetails(ctx context.Context, projectID, from, to string) (viewer.CSReportDetails, error) {
	m.gotProjectID = projectID
	return m.csReport, m.csErr
}

func TestGenerateTimelogsBreakdownReport_Success(t *testing.T) {
	mock := &mockReportsClient{timelogs: viewer.TimeLogBreakdownDetails{
		ProjectName: "Acme",
		ProjectKey:  "ACME",
		Cases:       []viewer.TimeLogBreakdownCase{{CaseNumber: "CS0001", TotalHours: "1h 30m"}},
	}}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withUser(httptest.NewRequest(http.MethodGet, "/generate-timelogs-breakdown-report?projectId=ACME", nil))
	w := httptest.NewRecorder()

	h.GenerateTimelogsBreakdownReport(w, req)

	assertStatus(t, w, http.StatusOK)
	if mock.gotProjectID != "ACME" {
		t.Errorf("client called with projectID=%q, want ACME", mock.gotProjectID)
	}
	got := decodeJSON[viewer.TimeLogBreakdownDetails](t, w)
	if got.ProjectName != "Acme" || got.ProjectKey != "ACME" || len(got.Cases) != 1 || got.Cases[0].CaseNumber != "CS0001" {
		t.Errorf("response = %+v, want the mock's report passed through", got)
	}
}

func TestGenerateTimelogsBreakdownReport_RequiresProjectID(t *testing.T) {
	mock := &mockReportsClient{}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withUser(httptest.NewRequest(http.MethodGet, "/generate-timelogs-breakdown-report", nil))
	w := httptest.NewRecorder()

	h.GenerateTimelogsBreakdownReport(w, req)

	assertStatus(t, w, http.StatusBadRequest)
	if mock.gotProjectID != "" {
		t.Error("client should not have been called without a projectId")
	}
}

func TestGenerateTimelogsBreakdownReport_AllowsCsEngineer(t *testing.T) {
	mock := &mockReportsClient{timelogs: viewer.TimeLogBreakdownDetails{ProjectKey: "ACME"}}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withCsEngineerUser(httptest.NewRequest(http.MethodGet, "/generate-timelogs-breakdown-report?projectId=ACME", nil))
	w := httptest.NewRecorder()

	h.GenerateTimelogsBreakdownReport(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestGetReportDetails_RequiresRange(t *testing.T) {
	mock := &mockReportsClient{}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withUser(httptest.NewRequest(http.MethodGet, "/report-details?projectSysId=ACME", nil))
	w := httptest.NewRecorder()

	h.GetReportDetails(w, req)

	assertStatus(t, w, http.StatusBadRequest)
	if mock.gotProjectID != "" {
		t.Error("client should not have been called without from and to")
	}
}

func TestGetReportDetails_RejectsBadDate(t *testing.T) {
	mock := &mockReportsClient{}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withUser(httptest.NewRequest(http.MethodGet, "/report-details?projectSysId=ACME&from=abc&to=2026-01-31", nil))
	w := httptest.NewRecorder()

	h.GetReportDetails(w, req)

	assertStatus(t, w, http.StatusBadRequest)
	if mock.gotProjectID != "" {
		t.Error("client should not have been called for an invalid from")
	}
}

func TestGetReportDetails_NotFound(t *testing.T) {
	mock := &mockReportsClient{csErr: viewer.ErrProjectNotFound}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withUser(httptest.NewRequest(http.MethodGet, "/report-details?projectSysId=missing&from=2026-01-01&to=2026-01-31", nil))
	w := httptest.NewRecorder()

	h.GetReportDetails(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestGenerateTimelogsBreakdownReport_RejectsMissingViewerAccess(t *testing.T) {
	mock := &mockReportsClient{}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := httptest.NewRequest(http.MethodGet, "/generate-timelogs-breakdown-report?projectId=ACME", nil)
	// Authenticated but holds no role granting PermViewerAccess.
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{Email: "nobody@example.com", UserID: "u-nobody"}))
	w := httptest.NewRecorder()

	h.GenerateTimelogsBreakdownReport(w, req)

	assertStatus(t, w, http.StatusForbidden)
	if mock.gotProjectID != "" {
		t.Error("client should not have been called for a caller without viewer access")
	}
}

func TestGenerateTimelogsBreakdownReport_NotFound(t *testing.T) {
	mock := &mockReportsClient{timelogsErr: viewer.ErrProjectNotFound}
	h := NewReportsHandler(mock, viewerAccessGuard)
	req := withUser(httptest.NewRequest(http.MethodGet, "/generate-timelogs-breakdown-report?projectId=p1", nil))
	w := httptest.NewRecorder()

	h.GenerateTimelogsBreakdownReport(w, req)

	assertStatus(t, w, http.StatusNotFound)
}
