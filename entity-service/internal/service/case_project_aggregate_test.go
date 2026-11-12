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

// TestCaseService_AggregateCases_ByProject: the relational service passes a
// project grouping to the repository and returns its buckets as they are,
// ids as keys and names as labels.
func TestCaseService_AggregateCases_ByProject(t *testing.T) {
	repo := &stubCaseRepo{aggregateCases: func(_ context.Context, _ domain.SearchCasesRequest, groupBy string) ([]domain.AggregateBucket, error) {
		if groupBy != "project" {
			t.Errorf("groupBy = %q, want project", groupBy)
		}
		return []domain.AggregateBucket{{Key: "p-1", Label: "Example Corp Production", Count: 5}, {Key: "p-2", Label: "EXCORP-PROJ-2", Count: 1}}, nil
	}}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "project"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Groups) != 2 || resp.Groups[0].Key != "p-1" || resp.Groups[0].Label != "Example Corp Production" || resp.TotalRecords != 6 {
		t.Errorf("got %+v, want the two project buckets and a total of 6", resp)
	}
}

// TestSNCaseService_AggregateCases_RefusesProject: the ServiceNow data
// source has no project grouping, so it refuses one before any call.
func TestSNCaseService_AggregateCases_RefusesProject(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(http.ResponseWriter, *http.Request) {
		t.Fatalf("ServiceNow must not be called")
	})
	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, nil, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "project"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || ve.Msg != `groupBy "project" is not supported by this data source` {
		t.Errorf("err = %v, want the project grouping refusal", err)
	}
}
