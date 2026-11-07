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

package longopenreport_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/longopenreport"
)

const reportJSON = `{
  "generatedAt": "2026-11-03T01:30:00Z",
  "olderThanDays": 30,
  "createdBefore": "2026-10-03T18:30:00Z",
  "cases": [
    {"number": "CS0000001", "internalId": "WSO2-1001", "createdOn": "2026-08-01T04:05:06Z",
     "updatedOn": "2026-10-30T10:00:00Z", "state": "work_in_progress", "severity": "high",
     "assignedEngineer": "Jane Doe", "csTeam": "Team Alpha"}
  ],
  "onboardingCases": [],
  "casesTotal": 1500,
  "onboardingCasesTotal": 0,
  "casesTruncated": true,
  "onboardingCasesTruncated": false
}`

func newTestClient(t *testing.T, handler http.HandlerFunc) *longopenreport.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/reports/long-open-cases", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, err := longopenreport.NewClient(longopenreport.Config{
		BaseURL: srv.URL + "/", TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestReportDecodesContractShape(t *testing.T) {
	var gotAuth, gotQuery, gotMethod string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotQuery, gotMethod = r.Header.Get("Authorization"), r.URL.RawQuery, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reportJSON))
	})

	report, err := c.Report(context.Background(), 30)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if gotMethod != http.MethodGet || gotQuery != "olderThanDays=30" || gotAuth != "Bearer test-token" {
		t.Errorf("request method=%q query=%q auth=%q", gotMethod, gotQuery, gotAuth)
	}
	if report.OlderThanDays != 30 || report.CasesTotal != 1500 || !report.CasesTruncated || report.OnboardingCasesTruncated {
		t.Errorf("envelope decoded wrongly: %+v", report)
	}
	if len(report.Cases) != 1 || len(report.OnboardingCases) != 0 {
		t.Fatalf("tables: cases=%d onboarding=%d", len(report.Cases), len(report.OnboardingCases))
	}
	row := report.Cases[0]
	if row.Number != "CS0000001" || row.InternalID != "WSO2-1001" || row.State != "work_in_progress" ||
		row.Severity != "high" || row.AssignedEngineer != "Jane Doe" || row.CsTeam != "Team Alpha" ||
		row.CreatedOn.Year() != 2026 || row.UpdatedOn.Month() != 10 {
		t.Errorf("row decoded wrongly: %+v", row)
	}
}

func TestReportNon200IsAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	})
	_, err := c.Report(context.Background(), 30)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("err = %v, want an apierror with 502", err)
	}
}

func TestReportBadJSON(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{not json")) })
	if _, err := c.Report(context.Background(), 30); err == nil {
		t.Fatal("expected a decode error")
	}
}

func TestNewClientRejectsPlainHTTP(t *testing.T) {
	if _, err := longopenreport.NewClient(longopenreport.Config{BaseURL: "http://example.com", TokenURL: "https://example.com/t"}); err == nil {
		t.Error("an http base URL must be refused")
	}
	if _, err := longopenreport.NewClient(longopenreport.Config{BaseURL: "https://example.com", TokenURL: "http://example.com/t"}); err == nil {
		t.Error("an http token URL must be refused")
	}
}

func TestOpenBeforeYesterday(t *testing.T) {
	var gotQuery string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"variant":"open-before-yesterday","olderThanDays":1,
		  "createdBefore":"2026-11-05T18:30:00Z",
		  "cases":[{"number":"CS0000001","internalId":"ID-1","state":"open","severity":"high",
		    "assignedEngineer":"Jane Doe","accountName":"Example Corp"},
		    {"number":"CS0000002","state":"open"}],
		  "onboardingCases":[],"casesTotal":7,"onboardingCasesTotal":0,
		  "casesTruncated":true,"onboardingCasesTruncated":false}`))
	})
	rep, err := c.OpenBeforeYesterday(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "variant=open-before-yesterday" {
		t.Errorf("query = %q", gotQuery)
	}
	if rep.Variant != "open-before-yesterday" || len(rep.Cases) != 2 || rep.CasesTotal != 7 || !rep.CasesTruncated {
		t.Errorf("report = %+v", rep)
	}
	if rep.Cases[0].AccountName != "Example Corp" || rep.Cases[1].AccountName != "" {
		t.Errorf("account names = %q, %q", rep.Cases[0].AccountName, rep.Cases[1].AccountName)
	}
}
