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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// cveFilterStub records the requests it receives and answers with a canned error.
type cveFilterStub struct {
	got []entity.SearchProductVulnerabilitiesRequest
	err error
}

func (s *cveFilterStub) SearchProductVulnerabilities(_ context.Context, req entity.SearchProductVulnerabilitiesRequest) (entity.SearchProductVulnerabilitiesResponse, error) {
	s.got = append(s.got, req)
	return entity.SearchProductVulnerabilitiesResponse{}, s.err
}

func (s *cveFilterStub) GetProductVulnerability(context.Context, string) (entity.ProductVulnerabilityView, error) {
	return entity.ProductVulnerabilityView{}, nil
}

func postVulnSearch(h *ProductVulnerabilityHandler, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/products/vulnerabilities/search", strings.NewReader(body))
	r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{UserID: "user-1"}))
	w := httptest.NewRecorder()
	h.SearchProductVulnerabilities(w, r)
	return w
}

func TestSearchProductVulnerabilities_CveIDFilter(t *testing.T) {
	long := strings.Repeat("C", 101)
	exact := strings.Repeat("C", 100)
	tests := []struct {
		name        string
		cveID       string
		upstreamErr error
		wantStatus  int
		wantForward bool
	}{
		{"forwarded unchanged", " cve-2024-0001 ", nil, http.StatusOK, true},
		{"exactly 100 characters", exact, nil, http.StatusOK, true},
		{"over 100 characters", long, nil, http.StatusBadRequest, false},
		{"surrounding whitespace not counted", " " + exact + " ", nil, http.StatusOK, true},
		{
			"upstream validation error maps through",
			"CVE-2024-0001",
			apierror.NewUpstreamError(http.StatusBadRequest, []byte(`{"message":"cveId is not supported"}`)),
			http.StatusBadRequest,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &cveFilterStub{err: tt.upstreamErr}
			h := NewProductVulnerabilityHandler(stub)
			w := postVulnSearch(h, `{"filters":{"cveId":"`+tt.cveID+`"},"pagination":{"limit":50,"offset":0}}`)
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", w.Code, tt.wantStatus, w.Body)
			}
			if tt.wantForward != (len(stub.got) == 1) {
				t.Fatalf("upstream calls = %d, wantForward %v", len(stub.got), tt.wantForward)
			}
			if tt.wantForward {
				f := stub.got[0].Filters
				if f == nil || f.CveID != tt.cveID {
					t.Errorf("forwarded filters = %+v, want CveID %q", f, tt.cveID)
				}
			}
			if tt.upstreamErr != nil && !strings.Contains(w.Body.String(), "cveId is not supported") {
				t.Errorf("upstream message not passed through: %s", w.Body)
			}
		})
	}
}
