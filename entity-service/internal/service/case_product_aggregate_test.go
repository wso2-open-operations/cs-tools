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
	"errors"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestCaseService_AggregateCases_ByProduct: the relational service passes a
// product grouping to the repository and returns its buckets as they are,
// names as keys.
func TestCaseService_AggregateCases_ByProduct(t *testing.T) {
	repo := &stubCaseRepo{aggregateCases: func(_ context.Context, _ domain.SearchCasesRequest, groupBy string) ([]domain.AggregateBucket, error) {
		if groupBy != "product" {
			t.Errorf("groupBy = %q, want product", groupBy)
		}
		return []domain.AggregateBucket{{Key: "WSO2 API Manager", Label: "WSO2 API Manager", Count: 4}, {Key: "WSO2 Identity Server", Label: "WSO2 Identity Server", Count: 2}}, nil
	}}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "product"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Groups) != 2 || resp.Groups[0].Key != "WSO2 API Manager" || resp.TotalRecords != 6 {
		t.Errorf("got %+v, want the two product buckets and a total of 6", resp)
	}
}

// TestSNCaseService_AggregateCases_RefusesProduct: the ServiceNow data
// source has no product grouping, so it refuses one before any call.
func TestSNCaseService_AggregateCases_RefusesProduct(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(http.ResponseWriter, *http.Request) {
		t.Fatalf("ServiceNow must not be called")
	})
	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, nil, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "product"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || ve.Msg != `groupBy "product" is not supported by this data source` {
		t.Errorf("err = %v, want the product grouping refusal", err)
	}
}
