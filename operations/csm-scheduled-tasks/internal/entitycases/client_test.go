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

package entitycases_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
)

func newSearchClient(t *testing.T, handler http.HandlerFunc) *entitycases.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/cases/search", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, err := entitycases.NewClient(entitycases.Config{
		BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestSearchOpenCasesFiltersByTypeAndDecodesIt(t *testing.T) {
	var got struct {
		Filters struct {
			Filters []struct {
				Field  string   `json:"field"`
				Op     string   `json:"op"`
				Values []string `json:"values"`
			} `json:"filters"`
		} `json:"filters"`
	}
	c := newSearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cases":[{"id":"00000000-0000-0000-0000-000000000001","number":"CS0000001",` +
			`"internalId":"ABC-1","type":"service_request","state":"open",` +
			`"createdOn":"2026-08-01T04:05:06Z","updatedOn":"2026-08-02T04:05:06Z"}],"total":1}`))
	})

	cases, err := c.SearchCasesInStateCreatedBeforeYesterday(context.Background(), "open")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(cases) != 1 || cases[0].Type != "service_request" || cases[0].ID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("decoded wrongly: %+v", cases)
	}
	var typeFilter []string
	for _, f := range got.Filters.Filters {
		if f.Field == "type" {
			if f.Op != "in" {
				t.Errorf("type filter op = %q", f.Op)
			}
			typeFilter = f.Values
		}
	}
	if !reflect.DeepEqual(typeFilter, []string{"case", "security_report_analysis", "service_request"}) {
		t.Errorf("type filter values = %v", typeFilter)
	}
}

func TestSearchOpenCasesErrorsOverCap(t *testing.T) {
	c := newSearchClient(t, func(w http.ResponseWriter, _ *http.Request) {
		rows := make([]string, 50)
		for i := range rows {
			rows[i] = `{"id":"00000000-0000-0000-0000-000000000001","number":"CS0000001","type":"case","state":"open",` +
				`"createdOn":"2026-08-01T04:05:06Z","updatedOn":"2026-08-02T04:05:06Z"}`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"cases":[%s],"total":5000}`, strings.Join(rows, ","))
	})
	if _, err := c.SearchCasesInStateCreatedBeforeYesterday(context.Background(), "open"); err == nil {
		t.Fatalf("a result set over the pagination cap must be an error, not a partial list")
	}
}
