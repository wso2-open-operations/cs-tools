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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

type stubSLAEngineClient struct {
	searchLimit, searchOffset, searchSource string
	searchResult                            []byte
	searchErr                               error
	recalcCalled                            bool
	recalcErr                               error
}

func (s *stubSLAEngineClient) SearchActiveSLAStatusesManage(_ context.Context, limit, offset, source string) ([]byte, error) {
	s.searchLimit, s.searchOffset, s.searchSource = limit, offset, source
	return s.searchResult, s.searchErr
}

func (s *stubSLAEngineClient) RequestSLARecalculation(context.Context) ([]byte, error) {
	s.recalcCalled = true
	return nil, s.recalcErr
}

func TestSLAEngineHandler_SearchActiveSLAStatusesManage_RequiresUser(t *testing.T) {
	h := NewSLAEngineHandler(&stubSLAEngineClient{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sla-status/manage", nil)

	h.SearchActiveSLAStatusesManage(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSLAEngineHandler_SearchActiveSLAStatusesManage_ForwardsQueryAndResult(t *testing.T) {
	client := &stubSLAEngineClient{searchResult: []byte(`{"statuses":[]}`)}
	h := NewSLAEngineHandler(client)
	rec := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodGet, "/sla-status/manage?limit=10&offset=5&source=csm", nil))

	h.SearchActiveSLAStatusesManage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if client.searchLimit != "10" || client.searchOffset != "5" || client.searchSource != "csm" {
		t.Errorf("query not forwarded: limit=%q offset=%q source=%q", client.searchLimit, client.searchOffset, client.searchSource)
	}
	if rec.Body.String() != `{"statuses":[]}` {
		t.Errorf("body = %q, want the client's raw result passed through", rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
}

func TestSLAEngineHandler_SearchActiveSLAStatusesManage_UpstreamErrorMapped(t *testing.T) {
	client := &stubSLAEngineClient{searchErr: &apierror.Error{StatusCode: http.StatusForbidden}}
	h := NewSLAEngineHandler(client)
	rec := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodGet, "/sla-status/manage", nil))

	h.SearchActiveSLAStatusesManage(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestSLAEngineHandler_RequestRecalculation_RequiresUser(t *testing.T) {
	h := NewSLAEngineHandler(&stubSLAEngineClient{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sla-engine/recalculate", nil)

	h.RequestRecalculation(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSLAEngineHandler_RequestRecalculation_CallsEntityAndReturnsNoContent(t *testing.T) {
	client := &stubSLAEngineClient{}
	h := NewSLAEngineHandler(client)
	rec := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost, "/sla-engine/recalculate", nil))

	h.RequestRecalculation(rec, req)

	if !client.recalcCalled {
		t.Error("RequestSLARecalculation was not called")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestSLAEngineHandler_RequestRecalculation_UpstreamErrorMapped(t *testing.T) {
	client := &stubSLAEngineClient{recalcErr: &apierror.Error{StatusCode: http.StatusServiceUnavailable}}
	h := NewSLAEngineHandler(client)
	rec := httptest.NewRecorder()
	req := withUser(httptest.NewRequest(http.MethodPost, "/sla-engine/recalculate", nil))

	h.RequestRecalculation(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
