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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

type stubKBArticleService struct {
	service.KBArticleService

	getResp domain.KBArticle
	getErr  error
}

func (s *stubKBArticleService) GetKBArticle(_ context.Context, _ string) (domain.KBArticle, error) {
	return s.getResp, s.getErr
}

func getKBArticle(t *testing.T, svc service.KBArticleService, id string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /kb-articles/{id}", NewKBArticleHandler(svc).GetKBArticle)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/kb-articles/"+id, nil))
	return rec
}

func TestGetKBArticle_StatusCodes(t *testing.T) {
	tests := []struct {
		name string
		svc  *stubKBArticleService
		want int
	}{
		{"existing article", &stubKBArticleService{getResp: domain.KBArticle{ID: "a1", Title: "T"}}, http.StatusOK},
		{"missing article", &stubKBArticleService{getErr: &apierror.NotFoundError{Msg: "article not found"}}, http.StatusNotFound},
		{"malformed id", &stubKBArticleService{getErr: &apierror.ValidationError{Msg: "id is not a valid UUID"}}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getKBArticle(t, tt.svc, "x").Code; got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}
