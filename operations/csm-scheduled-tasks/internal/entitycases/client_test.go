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

package entitycases

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// caseServer serves a token endpoint and /cases/search over n cases,
// returning at most pageCap rows per page regardless of the requested
// limit, and reporting total unless omitTotal. It records every offset
// requested.
func caseServer(t *testing.T, n, pageCap int, omitTotal bool, offsets *[]int) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600}`))
			return
		}
		var req searchCasesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		mu.Lock()
		*offsets = append(*offsets, req.Pagination.Offset)
		mu.Unlock()

		limit := req.Pagination.Limit
		if limit > pageCap {
			limit = pageCap
		}
		var page []searchCaseView
		for i := req.Pagination.Offset; i < n && len(page) < limit; i++ {
			page = append(page, searchCaseView{
				ID: fmt.Sprintf("id-%d", i), Number: fmt.Sprintf("CS%04d", i), State: "open",
				CreatedOn: "2026-09-01T00:00:00Z", UpdatedOn: "2026-09-02 10:00:00",
			})
		}
		resp := map[string]any{"cases": page}
		if !omitTotal {
			resp["total"] = n
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSearch_ShortPagesDoNotSkipRows(t *testing.T) {
	var offsets []int
	srv := caseServer(t, 70, 30, false, &offsets) // server caps pages at 30
	defer srv.Close()

	got, err := newTestClient(t, srv).SearchOpenCasesOlderThan(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 70 {
		t.Fatalf("want all 70 cases, got %d (offsets %v)", len(got), offsets)
	}
	if fmt.Sprint(offsets) != "[0 30 60]" {
		t.Fatalf("offsets must advance by rows returned, got %v", offsets)
	}
}

func TestSearch_MissingTotalKeepsPagingUntilAShortPage(t *testing.T) {
	var offsets []int
	srv := caseServer(t, 120, 50, true, &offsets)
	defer srv.Close()

	got, err := newTestClient(t, srv).SearchCasesInStateCreatedBeforeYesterday(context.Background(), "open")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 120 {
		t.Fatalf("without a total the search must continue to the short page; got %d (offsets %v)", len(got), offsets)
	}
}

func TestSearch_ParsesBothDateLayouts(t *testing.T) {
	var offsets []int
	srv := caseServer(t, 1, 50, false, &offsets)
	defer srv.Close()

	got, err := newTestClient(t, srv).SearchOpenCasesOlderThan(context.Background(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].CreatedOn.Format(time.RFC3339) != "2026-09-01T00:00:00Z" || got[0].UpdatedOn.Format(time.RFC3339) != "2026-09-02T10:00:00Z" {
		t.Fatalf("dates parsed wrong: %+v", got[0])
	}
}

func TestSearch_CapExceededIsAnError(t *testing.T) {
	var offsets []int
	srv := caseServer(t, maxSearchPages*searchPageSize+1, 50, false, &offsets)
	defer srv.Close()

	if _, err := newTestClient(t, srv).SearchOpenCasesOlderThan(context.Background(), time.Hour); err == nil {
		t.Fatal("a result larger than the pagination cap must be an error, not a truncated report")
	}
}
