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
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestValidateSearchExcludeFields(t *testing.T) {
	for _, ok := range [][]string{nil, {}, {"description"}, {"description", "description"}} {
		if err := validateSearchExcludeFields(ok); err != nil {
			t.Errorf("%v: unexpected error %v", ok, err)
		}
	}
	for _, bad := range [][]string{{"subject"}, {"description", "bogus"}, {""}, {"Description"}} {
		err := validateSearchExcludeFields(bad)
		var ve *apierror.ValidationError
		if err == nil || !asValidationError(err, &ve) {
			t.Errorf("%v: want a validation error, got %v", bad, err)
		}
	}
}

// An unsupported value is a 400 before the repository is ever reached.
func TestCaseService_SearchCases_RejectsUnknownExcludeField(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	_, err := svc.SearchCases(ctx, domain.SearchCasesRequest{ExcludeFields: []string{"subject"}})
	var ve *apierror.ValidationError
	if err == nil || !asValidationError(err, &ve) {
		t.Fatalf("expected a validation error, got %v", err)
	}
}

// The flag reaches the repository only when asked for, and a request without
// it is passed through untouched (the default response does not change).
func TestCaseService_SearchCases_ExcludeFieldsReachesRepository(t *testing.T) {
	desc := "<p>body</p>"
	var gotReq domain.SearchCasesRequest
	repo := &stubCaseRepo{
		searchCases: func(ctx context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error) {
			gotReq = req
			return []domain.SearchCaseView{{ID: "c1", Description: &desc}}, 1, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.SearchCases(ctx, domain.SearchCasesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if gotReq.ExcludesField(domain.SearchExcludeFieldDescription) {
		t.Error("default request must not exclude description")
	}
	if resp.Cases[0].Description == nil || *resp.Cases[0].Description != desc {
		t.Errorf("default response must keep the description, got %v", resp.Cases[0].Description)
	}

	if _, err := svc.SearchCases(ctx, domain.SearchCasesRequest{ExcludeFields: []string{"description"}}); err != nil {
		t.Fatal(err)
	}
	if !gotReq.ExcludesField(domain.SearchExcludeFieldDescription) {
		t.Error("excludeFields did not reach the repository")
	}
}

// The ServiceNow-backed service cannot leave the field out of its upstream
// response, so it drops it; without the flag the description is returned.
func TestSNCaseService_SearchCases_ExcludeDescription(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cases/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cases":  []map[string]any{{"id": "0123456789abcdef0123456789abcdef", "number": "CS0000001", "title": "t", "description": "<p>body</p>"}},
			"total":  1,
			"offset": 0,
			"limit":  20,
		})
	})
	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, nil, nil, "", nil)
	ctx := contextWithUserIDToken("token")

	resp, err := svc.SearchCases(ctx, domain.SearchCasesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Cases) != 1 || resp.Cases[0].Description == nil || *resp.Cases[0].Description != "<p>body</p>" {
		t.Fatalf("default response must keep the description, got %+v", resp.Cases)
	}

	resp, err = svc.SearchCases(ctx, domain.SearchCasesRequest{ExcludeFields: []string{"description"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Cases[0].Description != nil {
		t.Errorf("description must be omitted, got %q", *resp.Cases[0].Description)
	}
	if _, err := svc.SearchCases(ctx, domain.SearchCasesRequest{ExcludeFields: []string{"bogus"}}); err == nil {
		t.Error("expected a validation error for an unknown excludeFields value")
	}
}
