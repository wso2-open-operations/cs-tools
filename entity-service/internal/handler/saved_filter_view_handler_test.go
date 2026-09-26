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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

type stubSavedFilterViewService struct {
	service.SavedFilterViewService

	listResp domain.SavedFilterViewList
	listErr  error

	saveResp domain.SavedFilterViewList
	saveErr  error

	deleteResp domain.SavedFilterViewList
	deleteErr  error

	reorderResp domain.SavedFilterViewList
	reorderErr  error
}

func (s *stubSavedFilterViewService) List(_ context.Context, _ domain.SavedFilterListKey) (domain.SavedFilterViewList, error) {
	return s.listResp, s.listErr
}

func (s *stubSavedFilterViewService) Save(_ context.Context, _ domain.SaveSavedFilterViewRequest) (domain.SavedFilterViewList, error) {
	return s.saveResp, s.saveErr
}

func (s *stubSavedFilterViewService) Delete(_ context.Context, _ domain.SavedFilterListKey, _ string) (domain.SavedFilterViewList, error) {
	return s.deleteResp, s.deleteErr
}

func (s *stubSavedFilterViewService) Reorder(_ context.Context, _ domain.ReorderSavedFilterViewRequest) (domain.SavedFilterViewList, error) {
	return s.reorderResp, s.reorderErr
}

func TestSavedFilterViewHandler_List_Success(t *testing.T) {
	h := NewSavedFilterViewHandler(&stubSavedFilterViewService{
		listResp: domain.SavedFilterViewList{Views: []domain.SavedFilterView{{Name: "Open", Qs: "states=open"}}},
	})
	req := httptest.NewRequest(http.MethodGet, "/users/me/saved-filter-views?listKey=cases", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got domain.SavedFilterViewList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Views) != 1 || got.Views[0].Name != "Open" {
		t.Fatalf("unexpected body: %+v", got)
	}
}

func TestSavedFilterViewHandler_List_ValidationError(t *testing.T) {
	h := NewSavedFilterViewHandler(&stubSavedFilterViewService{
		listErr: &apierror.ValidationError{Msg: "listKey must be one of cases, incidents, change_requests, problems"},
	})
	req := httptest.NewRequest(http.MethodGet, "/users/me/saved-filter-views?listKey=nope", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSavedFilterViewHandler_Save_Success(t *testing.T) {
	h := NewSavedFilterViewHandler(&stubSavedFilterViewService{
		saveResp: domain.SavedFilterViewList{Views: []domain.SavedFilterView{{Name: "Mine", Qs: "q=1"}}},
	})
	body, _ := json.Marshal(domain.SaveSavedFilterViewRequest{ListKey: domain.SavedFilterListKeyCases, Name: "Mine", Qs: "q=1"})
	req := httptest.NewRequest(http.MethodPatch, "/users/me/saved-filter-views", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Save(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// savedFilterViewMux uses the same method patterns as routes.go when
// savedFilterViewHandler is registered. Go's ServeMux then answers PUT with
// 405. The published contract does not require 404 for that method.
func savedFilterViewMux(h *SavedFilterViewHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/me/saved-filter-views", h.List)
	mux.HandleFunc("PATCH /users/me/saved-filter-views", h.Save)
	mux.HandleFunc("DELETE /users/me/saved-filter-views", h.Delete)
	mux.HandleFunc("POST /users/me/saved-filter-views/reorder", h.Reorder)
	return mux
}

func TestSavedFilterViewRoutes_PatchReachesSaveAndPutIsMethodNotAllowed(t *testing.T) {
	h := NewSavedFilterViewHandler(&stubSavedFilterViewService{
		saveResp: domain.SavedFilterViewList{Views: []domain.SavedFilterView{{Name: "Mine", Qs: "q=1"}}},
	})
	mux := savedFilterViewMux(h)
	body, _ := json.Marshal(domain.SaveSavedFilterViewRequest{ListKey: domain.SavedFilterListKeyCases, Name: "Mine", Qs: "q=1"})

	patchRec := httptest.NewRecorder()
	mux.ServeHTTP(patchRec, httptest.NewRequest(http.MethodPatch, "/users/me/saved-filter-views", bytes.NewReader(body)))
	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d, want 200: %s", patchRec.Code, patchRec.Body.String())
	}

	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, httptest.NewRequest(http.MethodPut, "/users/me/saved-filter-views", bytes.NewReader(body)))
	if putRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT = %d, want 405", putRec.Code)
	}
}

func TestSavedFilterViewHandler_Delete_Success(t *testing.T) {
	h := NewSavedFilterViewHandler(&stubSavedFilterViewService{
		deleteResp: domain.SavedFilterViewList{Views: []domain.SavedFilterView{}},
	})
	req := httptest.NewRequest(http.MethodDelete, "/users/me/saved-filter-views?listKey=cases&name=Mine", nil)
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSavedFilterViewHandler_Reorder_Success(t *testing.T) {
	h := NewSavedFilterViewHandler(&stubSavedFilterViewService{
		reorderResp: domain.SavedFilterViewList{Views: []domain.SavedFilterView{{Name: "A", Qs: "q=a"}}},
	})
	body, _ := json.Marshal(domain.ReorderSavedFilterViewRequest{
		ListKey: domain.SavedFilterListKeyCases, Name: "A", Direction: domain.SavedFilterMoveDown,
	})
	req := httptest.NewRequest(http.MethodPost, "/users/me/saved-filter-views/reorder", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Reorder(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
