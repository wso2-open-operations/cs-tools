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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubDeployedProductRepo is a minimal repository.DeployedProductRepository
// whose SearchDeployedProducts panics if called: tests using it prove the
// PostgreSQL-backed service rejects an unsupported ProductCategories filter
// before ever reaching the repository, not merely that the repository
// ignores it.
type stubDeployedProductRepo struct {
	searchDeployedProducts         func(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error)
	searchProjectsByProductVersion func(ctx context.Context, req domain.SearchProjectsByProductVersionRequest, excludeClosureStates []string, excludeSubscriptionTypes []domain.SubscriptionType) ([]domain.EntityRef, int, error)
}

func (s *stubDeployedProductRepo) SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error) {
	if s.searchDeployedProducts != nil {
		return s.searchDeployedProducts(ctx, req)
	}
	panic("SearchDeployedProducts called unexpectedly: the productCategories rejection should have short-circuited before reaching the repository")
}

func (s *stubDeployedProductRepo) SearchDeployedProductMetrics(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductMetricsResponse, error) {
	panic("SearchDeployedProductMetrics not stubbed")
}

func (s *stubDeployedProductRepo) SearchDeployedProductUsageCounts(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductUsageCountsResponse, error) {
	panic("SearchDeployedProductUsageCounts not stubbed")
}

func (s *stubDeployedProductRepo) SearchProjectsByProductVersion(ctx context.Context, req domain.SearchProjectsByProductVersionRequest, excludeClosureStates []string, excludeSubscriptionTypes []domain.SubscriptionType) ([]domain.EntityRef, int, error) {
	if s.searchProjectsByProductVersion != nil {
		return s.searchProjectsByProductVersion(ctx, req, excludeClosureStates, excludeSubscriptionTypes)
	}
	panic("SearchProjectsByProductVersion called unexpectedly")
}

func TestDeployedProductService_SearchDeployedProducts_RejectsProductCategories(t *testing.T) {
	svc := NewDeployedProductService(&stubDeployedProductRepo{})

	_, err := svc.SearchDeployedProducts(context.Background(), domain.SearchDeployedProductsRequest{
		ProductCategories: []string{"pdp"},
	})

	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestDeployedProductService_SearchDeployedProducts_NoProductCategoriesReachesRepository(t *testing.T) {
	called := false
	svc := NewDeployedProductService(&stubDeployedProductRepo{
		searchDeployedProducts: func(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error) {
			called = true
			return nil, 0, nil
		},
	})

	_, err := svc.SearchDeployedProducts(context.Background(), domain.SearchDeployedProductsRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected SearchDeployedProducts to reach the repository when ProductCategories is empty")
	}
}

func TestDeployedProductService_SearchProjectsByProductVersion_RejectsInvalidProductID(t *testing.T) {
	svc := NewDeployedProductService(&stubDeployedProductRepo{})

	_, err := svc.SearchProjectsByProductVersion(context.Background(), domain.SearchProjectsByProductVersionRequest{
		ProductID:        "not-a-uuid",
		ProductVersionID: "22222222-2222-2222-2222-222222222222",
	})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestDeployedProductService_SearchProjectsByProductVersion_RejectsInvalidProductVersionID(t *testing.T) {
	svc := NewDeployedProductService(&stubDeployedProductRepo{})

	_, err := svc.SearchProjectsByProductVersion(context.Background(), domain.SearchProjectsByProductVersionRequest{
		ProductID:        "11111111-1111-1111-1111-111111111111",
		ProductVersionID: "not-a-uuid",
	})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestDeployedProductService_SearchProjectsByProductVersion_AppliesMandatoryExclusions
// proves the service passes the SAME package-level mandatory exclusion
// policy the ServiceNow implementation uses (mandatoryExcludeClosureStates/
// mandatoryExcludeSubscriptionTypes) through to the repository, rather than
// leaving it to the caller or silently dropping it -- the whole point of
// this being a fixed policy, not a request field.
func TestDeployedProductService_SearchProjectsByProductVersion_AppliesMandatoryExclusions(t *testing.T) {
	var gotClosureStates []string
	var gotSubscriptionTypes []domain.SubscriptionType
	var gotReq domain.SearchProjectsByProductVersionRequest

	svc := NewDeployedProductService(&stubDeployedProductRepo{
		searchProjectsByProductVersion: func(ctx context.Context, req domain.SearchProjectsByProductVersionRequest, excludeClosureStates []string, excludeSubscriptionTypes []domain.SubscriptionType) ([]domain.EntityRef, int, error) {
			gotReq = req
			gotClosureStates = excludeClosureStates
			gotSubscriptionTypes = excludeSubscriptionTypes
			return []domain.EntityRef{{ID: "p1", Name: "Project One"}}, 1, nil
		},
	})

	resp, err := svc.SearchProjectsByProductVersion(context.Background(), domain.SearchProjectsByProductVersionRequest{
		ProductID:        "11111111-1111-1111-1111-111111111111",
		ProductVersionID: "22222222-2222-2222-2222-222222222222",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.Projects) != 1 || resp.Projects[0].ID != "p1" {
		t.Fatalf("expected the repository's project to pass through, got %+v", resp.Projects)
	}
	if resp.Total != 1 || resp.Limit != gotReq.Pagination.Limit || resp.Offset != gotReq.Pagination.Offset {
		t.Fatalf("expected pagination fields to echo the normalized request, got %+v (req pagination %+v)", resp, gotReq.Pagination)
	}
	if len(gotClosureStates) != len(mandatoryExcludeClosureStates) {
		t.Fatalf("expected mandatoryExcludeClosureStates to be passed through, got %v", gotClosureStates)
	}
	if len(gotSubscriptionTypes) != len(mandatoryExcludeSubscriptionTypes) {
		t.Fatalf("expected mandatoryExcludeSubscriptionTypes to be passed through, got %v", gotSubscriptionTypes)
	}
}
