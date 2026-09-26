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

package slaengine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func newTestEntityClient(t *testing.T, apiSrv *httptest.Server) *EntityClient {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "token_type": "Bearer", "expires_in": 3600})
	}))
	t.Cleanup(tokenSrv.Close)
	return NewEntityClient(EntityConfig{BaseURL: apiSrv.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})
}

// TestFetchAllActiveSLAStatuses_PagesUntilExhausted verifies the client
// keeps requesting pages until it has seen every row the server reports as
// total, matching entity-service's listPageSize-per-request contract.
func TestFetchAllActiveSLAStatuses_PagesUntilExhausted(t *testing.T) {
	const total = listPageSize + 5
	var gotOffsets []int
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		gotOffsets = append(gotOffsets, offset)

		remaining := total - offset
		pageSize := remaining
		if pageSize > listPageSize {
			pageSize = listPageSize
		}
		statuses := make([]SLAStatus, pageSize)
		for i := range statuses {
			statuses[i] = SLAStatus{CaseID: strconv.Itoa(offset + i), ClockType: "response"}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(searchSLAStatusResponse{Statuses: statuses, Total: total, Limit: listPageSize, Offset: offset})
	}))
	defer apiSrv.Close()

	c := newTestEntityClient(t, apiSrv)
	got, err := c.FetchAllActiveSLAStatuses(t.Context())
	if err != nil {
		t.Fatalf("FetchAllActiveSLAStatuses() error = %v, want nil", err)
	}
	if len(got) != total {
		t.Fatalf("len(got) = %d, want %d", len(got), total)
	}
	if want := []int{0, listPageSize}; len(gotOffsets) != len(want) || gotOffsets[0] != want[0] || gotOffsets[1] != want[1] {
		t.Errorf("requested offsets = %v, want %v", gotOffsets, want)
	}
}

func TestFetchAllActiveSLAStatuses_EmptyResult(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(searchSLAStatusResponse{Statuses: nil, Total: 0})
	}))
	defer apiSrv.Close()

	c := newTestEntityClient(t, apiSrv)
	got, err := c.FetchAllActiveSLAStatuses(t.Context())
	if err != nil {
		t.Fatalf("FetchAllActiveSLAStatuses() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}
