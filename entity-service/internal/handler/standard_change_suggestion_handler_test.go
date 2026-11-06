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
		"/standard-change-suggestions?accountId=a&includeIneligible=TRUE&includeDismissed=yes&minClosed=2&windowMonths=%206%20", nil))
	if err != nil {
		t.Fatal(err)
	}
	if req.AccountID != "a" || !req.IncludeIneligible || req.IncludeDismissed {
		t.Errorf("request = %+v (a flag is true only for true)", req)
	}
	if req.MinClosed == nil || *req.MinClosed != 2 || req.WindowMonths == nil || *req.WindowMonths != 6 || req.MinSuccessPercent != nil {
		t.Errorf("overrides = %v %v %v", req.MinClosed, req.WindowMonths, req.MinSuccessPercent)
	}
	_, err = parseSuggestionsQuery(httptest.NewRequest("GET", "/standard-change-suggestions?planSimilarityPercent=half", nil))
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || ve.Msg != "planSimilarityPercent must be an integer" {
		t.Errorf("err = %v, want a 400 naming planSimilarityPercent", err)
	}
}
