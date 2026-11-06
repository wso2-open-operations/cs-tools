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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type stubAllocationViewSvc struct {
	gotID     string
	gotPage   domain.Pagination
	gotCreate domain.CreateEngagementAllocationUpdateRequest
	gotSearch domain.SearchEngagementAllocationsRequest
	err       error
	createdID string
}

func (s *stubAllocationViewSvc) Search(_ context.Context, req domain.SearchEngagementAllocationsRequest) (domain.SearchEngagementAllocationsResponse, error) {
	s.gotSearch = req
	return domain.SearchEngagementAllocationsResponse{Allocations: []domain.EngagementAllocationView{}, Total: 0}, s.err
}

func (s *stubAllocationViewSvc) Get(_ context.Context, id string) (domain.EngagementAllocationView, error) {
	s.gotID = id
	return domain.EngagementAllocationView{ID: id}, s.err
}

func (s *stubAllocationViewSvc) ListUpdates(_ context.Context, id string, p domain.Pagination) (domain.ListEngagementAllocationUpdatesResponse, error) {
	s.gotID, s.gotPage = id, p
	return domain.ListEngagementAllocationUpdatesResponse{Updates: []domain.EngagementAllocationUpdate{}}, s.err
}

func (s *stubAllocationViewSvc) CreateUpdate(_ context.Context, id string, req domain.CreateEngagementAllocationUpdateRequest) (domain.EngagementAllocationUpdate, error) {
	s.gotID, s.gotCreate = id, req
	return domain.EngagementAllocationUpdate{ID: "u1"}, s.err
}

// serve routes through a mux so PathValue is populated the way production does.
func serveAllocationView(h *EngagementAllocationViewHandler, method, target, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /customer-engagements/allocations/search", h.SearchAllocations)
	mux.HandleFunc("GET /customer-engagements/allocations/{id}", h.GetAllocation)
	mux.HandleFunc("GET /customer-engagements/allocations/{id}/updates", h.ListAllocationUpdates)
	mux.HandleFunc("POST /customer-engagements/allocations/{id}/updates", h.CreateAllocationUpdate)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(method, target, strings.NewReader(body)))
	return w
}

func TestAllocationViewHandler_Routes(t *testing.T) {
	svc := &stubAllocationViewSvc{}
	h := NewEngagementAllocationViewHandler(svc)

	if w := serveAllocationView(h, "POST", "/customer-engagements/allocations/search",
		`{"pagination":{"limit":5},"filters":{"engineerId":"x","states":["CONFIRMED"]},"sortBy":{"field":"startDate","order":"asc"}}`); w.Code != 200 ||
		svc.gotSearch.Pagination.Limit != 5 || svc.gotSearch.Filters.States[0] != "CONFIRMED" {
		t.Errorf("search: %d %s %+v", w.Code, w.Body, svc.gotSearch)
	}
	if w := serveAllocationView(h, "GET", "/customer-engagements/allocations/abc", ""); w.Code != 200 || svc.gotID != "abc" {
		t.Errorf("get: %d id=%q", w.Code, svc.gotID)
	}
	if w := serveAllocationView(h, "GET", "/customer-engagements/allocations/abc/updates?limit=7&offset=14", ""); w.Code != 200 ||
		svc.gotPage.Limit != 7 || svc.gotPage.Offset != 14 {
		t.Errorf("list: %d page=%+v", w.Code, svc.gotPage)
	}
	w := serveAllocationView(h, "POST", "/customer-engagements/allocations/abc/updates", `{"subject":"s","content":"c"}`)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusCreated || svc.gotCreate.Subject != "s" || body["id"] != "u1" {
		t.Errorf("create: %d %s", w.Code, w.Body)
	}
}

func TestAllocationViewHandler_BadInput(t *testing.T) {
	h := NewEngagementAllocationViewHandler(&stubAllocationViewSvc{})
	for name, w := range map[string]*httptest.ResponseRecorder{
		"unknown body field": serveAllocationView(h, "POST", "/customer-engagements/allocations/abc/updates", `{"subject":"s","content":"c","authorId":"x"}`),
		"malformed search":   serveAllocationView(h, "POST", "/customer-engagements/allocations/search", `{`),
		"limit not a number": serveAllocationView(h, "GET", "/customer-engagements/allocations/abc/updates?limit=many", ""),
	} {
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d", name, w.Code)
		}
	}
}

func TestAllocationViewHandler_ErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{&apierror.ForbiddenError{Msg: "no"}, 403},
		{&apierror.NotFoundError{Msg: "gone"}, 404},
		{&apierror.ValidationError{Msg: "bad"}, 400},
	} {
		h := NewEngagementAllocationViewHandler(&stubAllocationViewSvc{err: tc.err})
		if w := serveAllocationView(h, "POST", "/customer-engagements/allocations/abc/updates", `{"subject":"s","content":"c"}`); w.Code != tc.want {
			t.Errorf("%T: status %d, want %d", tc.err, w.Code, tc.want)
		}
	}
}
