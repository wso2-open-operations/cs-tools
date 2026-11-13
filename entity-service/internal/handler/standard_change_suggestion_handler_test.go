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
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

func TestParseSuggestionsQuery(t *testing.T) {
	req, err := parseSuggestionsQuery(httptest.NewRequest("GET",
		"/standard-change-suggestions?accountId=a&includeIneligible=TRUE&includeDismissed=yes&minClosed=2", nil))
	if err != nil || req.AccountID != "a" || !req.IncludeIneligible || req.IncludeDismissed {
		t.Errorf("request = %+v, %v (a flag is true only for true; the criteria are not read)", req, err)
	}
	if req.Pagination.Limit != 0 || req.Pagination.Offset != 0 {
		t.Errorf("pagination = %+v, want zero (the service's default) when not given", req.Pagination)
	}
}

// limit and offset page the list; anything but a non-negative integer is a 400.
func TestParseSuggestionsQuery_Pagination(t *testing.T) {
	req, err := parseSuggestionsQuery(httptest.NewRequest("GET", "/standard-change-suggestions?limit=25&offset=50", nil))
	if err != nil || req.Pagination.Limit != 25 || req.Pagination.Offset != 50 {
		t.Fatalf("request = %+v, %v; want limit 25, offset 50", req.Pagination, err)
	}
	for _, bad := range []string{"limit=ten", "offset=-1", "limit=-5", "offset=1.5"} {
		_, err := parseSuggestionsQuery(httptest.NewRequest("GET", "/standard-change-suggestions?"+bad, nil))
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want a ValidationError", bad, err)
		}
	}
}
