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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type stubGroupSearchService struct {
	got domain.SearchGroupsRequest
}

func (s *stubGroupSearchService) SearchGroups(_ context.Context, req domain.SearchGroupsRequest) (domain.SearchGroupsResponse, error) {
	s.got = req
	return domain.SearchGroupsResponse{Groups: []domain.Group{}, Total: 0, Limit: 10}, nil
}

// incidentExtraGroupsOnly is decoded from the request body into the service
// request; omitted, it is false.
func TestSearchGroups_DecodesIncidentExtraGroupsOnly(t *testing.T) {
	for name, c := range map[string]struct {
		body string
		want bool
	}{
		"true":    {`{"filters":{"incidentExtraGroupsOnly":true,"searchQuery":"soc"},"pagination":{"limit":10,"offset":0}}`, true},
		"false":   {`{"filters":{"incidentExtraGroupsOnly":false},"pagination":{"limit":10,"offset":0}}`, false},
		"omitted": {`{"filters":{"supportGroupsOnly":true},"pagination":{"limit":10,"offset":0}}`, false},
	} {
		svc := &stubGroupSearchService{}
		mux := http.NewServeMux()
		mux.HandleFunc("POST /groups/search", NewGroupHandler(svc).SearchGroups)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/groups/search", strings.NewReader(c.body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", name, rec.Code, rec.Body)
		}
		if svc.got.Filters == nil || svc.got.Filters.IncidentExtraGroupsOnly != c.want {
			t.Errorf("%s: filters = %+v, want IncidentExtraGroupsOnly %v", name, svc.got.Filters, c.want)
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s: response: %v", name, err)
		}
		if _, ok := resp["groups"]; !ok {
			t.Errorf("%s: response lost the groups field: %s", name, rec.Body)
		}
	}
}
