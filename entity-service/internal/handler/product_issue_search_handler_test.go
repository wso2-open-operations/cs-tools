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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

type searchStubSvc struct {
	service.ProductIssueService
	got domain.SearchProductIssuesRequest
	err error
}

func (s *searchStubSvc) Search(_ context.Context, req domain.SearchProductIssuesRequest) (domain.SearchProductIssuesResponse, error) {
	s.got = req
	return domain.SearchProductIssuesResponse{Items: []domain.ProductIssueSummary{{ID: "x", SLA: domain.ProductIssueSLA{State: "NONE"}}}}, s.err
}

func TestProductIssueSearchHandler_NewFields(t *testing.T) {
	post := func(svc *searchStubSvc, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		NewProductIssueHandler(svc).SearchProductWorkItems(w, httptest.NewRequest(http.MethodPost, "/product-work-items/search", strings.NewReader(body)))
		return w
	}
	svc := &searchStubSvc{}
	w := post(svc, `{"filters":{"boardStatuses":["WAITING_ON_CS"],"mine":true,"sort":"pausedSince"}}`)
	if w.Code != http.StatusOK || !svc.got.Filters.Mine || svc.got.Filters.Sort != "pausedSince" || len(svc.got.Filters.BoardStatuses) != 1 {
		t.Fatalf("decode: %d %+v", w.Code, svc.got)
	}
	if !strings.Contains(w.Body.String(), `"sla":{"state":"NONE"`) || !strings.Contains(w.Body.String(), `"nonStandardColumn":false`) {
		t.Errorf("body = %s", w.Body.String())
	}
	// The resolved caller is not a body field.
	svc = &searchStubSvc{}
	post(svc, `{"filters":{"mine":true,"MineEmail":"evil@x.com","mineEmail":"evil@x.com"}}`)
	if svc.got.Filters.MineEmail != "" {
		t.Errorf("MineEmail read from the body: %q", svc.got.Filters.MineEmail)
	}
	for name, body := range map[string]string{
		"mine not a bool":    `{"filters":{"mine":"yes"}}`,
		"sort not a string":  `{"filters":{"sort":3}}`,
		"statuses not array": `{"filters":{"boardStatuses":"WAITING_ON_CS"}}`,
	} {
		if w := post(&searchStubSvc{}, body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, w.Code)
		}
	}
	// A service refusal (mine without a user, bad sort) is a 400 with its message.
	w = post(&searchStubSvc{err: &apierror.ValidationError{Msg: "mine requires a signed-in user."}}, `{"filters":{"mine":true}}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "signed-in user") {
		t.Errorf("service 400: %d %s", w.Code, w.Body.String())
	}
}
