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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestCheckOutageSearchLimit(t *testing.T) {
	for _, ok := range []int{-1, 0, 20, maxOutageSearchLimit} {
		if err := checkOutageSearchLimit(ok); err != nil {
			t.Errorf("limit %d: unexpected error %v", ok, err)
		}
	}
	var ve *apierror.ValidationError
	if err := checkOutageSearchLimit(maxOutageSearchLimit + 1); !errors.As(err, &ve) {
		t.Errorf("limit %d: err = %v, want ValidationError", maxOutageSearchLimit+1, err)
	}
}

// TestPgOutageService_SearchCommunications_RefusesOversizedLimit proves the
// cap is applied before the repository is reached (the service here has no
// repository, so reaching it would panic).
func TestPgOutageService_SearchCommunications_RefusesOversizedLimit(t *testing.T) {
	svc := &pgOutageService{}
	_, err := svc.SearchOutageCommunications(context.Background(), domain.SearchOutageCommunicationsRequest{
		OutageID:   authzCaseID,
		Pagination: domain.Pagination{Limit: 10000000},
	})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}
