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

package timecards_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/timecards"
)

func newClient(t *testing.T, mux *http.ServeMux) *timecards.Client {
	t.Helper()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, err := timecards.NewClient(timecards.Config{BaseURL: srv.URL + "/", TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSearchProjectsPagesAndSendsTheQuery(t *testing.T) {
	var offsets []int
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/search", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SearchQuery string `json:"searchQuery"`
			Pagination  struct{ Limit, Offset int }
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.SearchQuery != "PRJ-1" || req.Pagination.Limit != 50 || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("request = %+v, auth %q", req, r.Header.Get("Authorization"))
		}
		offsets = append(offsets, req.Pagination.Offset)
		more := req.Pagination.Offset == 0
		_, _ = fmt.Fprintf(w, `{"projects":[{"id":"id-%d","key":"K%d","name":null}],"hasMore":%t}`, req.Pagination.Offset, req.Pagination.Offset, more)
	})
	got, err := newClient(t, mux).SearchProjects(context.Background(), "PRJ-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "id-0" || got[1].ID != "id-50" || len(offsets) != 2 {
		t.Fatalf("got %+v offsets %v", got, offsets)
	}
}

func TestProjectContactsPages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/id-1/contacts/search", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Pagination struct{ Limit, Offset int } }
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Pagination.Offset == 0 {
			_, _ = w.Write([]byte(`{"contacts":[{"email":"jane.doe@example.com","isCsIntegrationUser":false}],"total":2}`))
			return
		}
		_, _ = w.Write([]byte(`{"contacts":[{"email":"bot@example.com","isCsIntegrationUser":true}],"total":2}`))
	})
	got, err := newClient(t, mux).ProjectContacts(context.Background(), "id-1")
	if err != nil || len(got) != 2 || !got[1].IsCsIntegrationUser {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestReportDecodesContractShapeAndSendsTheRange(t *testing.T) {
	var gotQuery, gotMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/id-1/time-cards/report", func(w http.ResponseWriter, r *http.Request) {
		gotQuery, gotMethod = r.URL.RawQuery, r.Method
		_, _ = w.Write([]byte(`{"project":{"id":"id-1","key":"PRJ-1","name":null},"from":"2030-04-01","to":"2030-04-30",
		"totals":{"totalMinutes":90,"billableMinutes":60,"nonBillableMinutes":30,"patchingMinutes":15},
		"cases":[{"caseId":"c1","caseNumber":"CS0000001","externalCaseId":null,"caseType":"service_request",
		"totalMinutes":90,"billableMinutes":60,"nonBillableMinutes":30,"patchingMinutes":15}]}`))
	})
	got, err := newClient(t, mux).Report(context.Background(), "id-1", "2030-04-01", "2030-04-30")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotQuery != "from=2030-04-01&to=2030-04-30" {
		t.Errorf("%s ?%s", gotMethod, gotQuery)
	}
	if got.Project.Key != "PRJ-1" || got.Project.Name != "" || got.Totals.PatchingMinutes != 15 ||
		len(got.Cases) != 1 || got.Cases[0].ExternalCaseID != "" || got.Cases[0].CaseType != "service_request" {
		t.Errorf("decoded %+v", got)
	}
}

func TestReportErrorIsATypedStatusError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/id-1/time-cards/report", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusNotFound)
	})
	_, err := newClient(t, mux).Report(context.Background(), "id-1", "2030-04-01", "2030-04-30")
	var ae *apierror.Error
	if !errors.As(err, &ae) || ae.StatusCode != http.StatusNotFound {
		t.Fatalf("err = %v", err)
	}
}
