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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeProductRepo captures the request it was called with so tests can
// assert on what the service layer forwards to the repository, without a
// live Postgres connection.
type fakeProductRepo struct {
	gotReq domain.SearchProductsRequest
}

func (f *fakeProductRepo) SearchProducts(_ context.Context, req domain.SearchProductsRequest) ([]domain.Product, int, error) {
	f.gotReq = req
	return []domain.Product{}, 0, nil
}

func (f *fakeProductRepo) GetProduct(context.Context, string) (*domain.Product, error) {
	return nil, nil
}

// TestProductService_SearchProducts_EmptyClassIsUnfiltered guards against
// reintroducing the ServiceNow-only "product_model" default on the
// Postgres-backed path: the products table already stores one row per
// distinct product, so an empty Class must reach the repository unchanged
// (empty), not be defaulted to any value.
func TestProductService_SearchProducts_EmptyClassIsUnfiltered(t *testing.T) {
	repo := &fakeProductRepo{}
	svc := NewProductService(repo)

	_, err := svc.SearchProducts(context.Background(), domain.SearchProductsRequest{
		Pagination: domain.Pagination{Limit: 20, Offset: 0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotReq.Class != "" {
		t.Fatalf("expected empty class to reach repo unfiltered, got %q", repo.gotReq.Class)
	}
}

// TestProductService_SearchProducts_ExplicitClassForwarded verifies an
// explicit, valid class value reaches the repository unchanged so it can be
// applied as a SQL predicate.
func TestProductService_SearchProducts_ExplicitClassForwarded(t *testing.T) {
	for _, class := range []string{"software", "service"} {
		t.Run(class, func(t *testing.T) {
			repo := &fakeProductRepo{}
			svc := NewProductService(repo)

			_, err := svc.SearchProducts(context.Background(), domain.SearchProductsRequest{
				Pagination: domain.Pagination{Limit: 20, Offset: 0},
				Class:      class,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.gotReq.Class != class {
				t.Fatalf("expected class %q forwarded to repo, got %q", class, repo.gotReq.Class)
			}
		})
	}
}

// TestProductService_SearchProducts_RejectsUnknownClass ensures an invalid
// class value never reaches the repository (and so can never be
// interpolated into the SQL predicate).
func TestProductService_SearchProducts_RejectsUnknownClass(t *testing.T) {
	repo := &fakeProductRepo{}
	svc := NewProductService(repo)

	_, err := svc.SearchProducts(context.Background(), domain.SearchProductsRequest{
		Pagination: domain.Pagination{Limit: 20, Offset: 0},
		Class:      "product_model", // valid for the SN path, not for Postgres
	})
	if err == nil {
		t.Fatal("expected validation error for unknown class, got nil")
	}
}
