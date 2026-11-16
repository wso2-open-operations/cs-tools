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
)

type mockEntityMTTRReportClient struct {
	getMTTRReportFn func(ctx context.Context, months, days string, creTeamIDs []string) ([]byte, error)
}

func (m *mockEntityMTTRReportClient) GetMTTRReport(ctx context.Context, months, days string, creTeamIDs []string) ([]byte, error) {
	return m.getMTTRReportFn(ctx, months, days, creTeamIDs)
}

const testMTTRTeamID = "55555555-5555-4555-8555-555555555555"

func newMTTRRequest(query string) *http.Request {
	return withUser(httptest.NewRequest(http.MethodGet, "/reports/mttr?"+query, nil))
}

func TestGetMTTRReport(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewMTTRReportHandler(&mockEntityMTTRReportClient{})
		r := httptest.NewRequest(http.MethodGet, "/reports/mttr?months=3", nil)
		w := httptest.NewRecorder()
		h.GetMTTRReport(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects a missing or invalid months without calling upstream", func(t *testing.T) {
		for _, query := range []string{"", "months=", "months=2", "months=03", "days=3", "days=", "months=3&days=7"} {
			client := &mockEntityMTTRReportClient{getMTTRReportFn: func(context.Context, string, string, []string) ([]byte, error) {
				t.Errorf("upstream called for query %q", query)
				return nil, nil
			}}
			h := NewMTTRReportHandler(client)
			w := httptest.NewRecorder()
			h.GetMTTRReport(w, newMTTRRequest(query))
			assertStatus(t, w, http.StatusBadRequest)
			assertContentType(t, w, "application/json")
		}
	})

	t.Run("rejects a malformed creTeamId without calling upstream", func(t *testing.T) {
		client := &mockEntityMTTRReportClient{getMTTRReportFn: func(context.Context, string, string, []string) ([]byte, error) {
			t.Error("upstream called for a malformed creTeamId")
			return nil, nil
		}}
		h := NewMTTRReportHandler(client)
		w := httptest.NewRecorder()
		h.GetMTTRReport(w, newMTTRRequest("months=3&creTeamId=not-a-uuid"))
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards months and creTeamId to upstream and returns the report", func(t *testing.T) {
		var gotMonths string
		var gotTeamIDs []string
		client := &mockEntityMTTRReportClient{
			getMTTRReportFn: func(_ context.Context, months, _ string, creTeamIDs []string) ([]byte, error) {
				gotMonths, gotTeamIDs = months, creTeamIDs
				return []byte(`{"windowMonths":3,"bands":[{"band":"S1","caseCount":2,"avgResolutionHours":12.5}]}`), nil
			},
		}
		h := NewMTTRReportHandler(client)
		w := httptest.NewRecorder()
		h.GetMTTRReport(w, newMTTRRequest("months=3&creTeamId="+testMTTRTeamID))

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if gotMonths != "3" || len(gotTeamIDs) != 1 || gotTeamIDs[0] != testMTTRTeamID {
			t.Errorf("upstream received months=%q creTeamIDs=%v", gotMonths, gotTeamIDs)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["windowMonths"].(float64) != 3 {
			t.Errorf("response = %v", resp)
		}
	})

	t.Run("forwards a 7 day window as days, not months", func(t *testing.T) {
		var gotMonths, gotDays string
		client := &mockEntityMTTRReportClient{
			getMTTRReportFn: func(_ context.Context, months, days string, _ []string) ([]byte, error) {
				gotMonths, gotDays = months, days
				return []byte(`{}`), nil
			},
		}
		h := NewMTTRReportHandler(client)
		w := httptest.NewRecorder()
		h.GetMTTRReport(w, newMTTRRequest("days=7"))
		assertStatus(t, w, http.StatusOK)
		if gotMonths != "" || gotDays != "7" {
			t.Errorf("upstream received months=%q days=%q", gotMonths, gotDays)
		}
	})

	t.Run("no creTeamId means every team", func(t *testing.T) {
		var gotTeamIDs []string
		client := &mockEntityMTTRReportClient{
			getMTTRReportFn: func(_ context.Context, _, _ string, creTeamIDs []string) ([]byte, error) {
				gotTeamIDs = creTeamIDs
				return []byte(`{}`), nil
			},
		}
		h := NewMTTRReportHandler(client)
		w := httptest.NewRecorder()
		h.GetMTTRReport(w, newMTTRRequest("months=12"))
		assertStatus(t, w, http.StatusOK)
		if len(gotTeamIDs) != 0 {
			t.Errorf("creTeamIDs = %v, want none", gotTeamIDs)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve the MTTR report.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityMTTRReportClient{
					getMTTRReportFn: func(context.Context, string, string, []string) ([]byte, error) { return nil, tc.err },
				}
				h := NewMTTRReportHandler(client)
				w := httptest.NewRecorder()
				h.GetMTTRReport(w, newMTTRRequest("months=3"))
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
