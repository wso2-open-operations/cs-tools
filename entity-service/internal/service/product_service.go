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

// Package service is declared in interfaces.go.
package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// validProductClass enumerates the class values the products table accepts.
// Unlike the ServiceNow-backed search, an empty Class here means "no
// filter" — the products table already stores one row per distinct
// product, so there is no version-duplication problem to default around.
var validProductClass = map[domain.ProductClass]bool{
	domain.ProductClassSoftware: true,
	domain.ProductClassService:  true,
}

type productService struct {
	repo repository.ProductRepository
}

// NewProductService constructs a ProductService backed by the given repository.
func NewProductService(repo repository.ProductRepository) ProductService {
	return &productService{repo: repo}
}

// maxProductsLimit bounds a page of POST /products/search, so the product
// catalogue (about 80 products) reads in one page.
const maxProductsLimit = 500

// SearchProducts implements ProductService.
func (s *productService) SearchProducts(ctx context.Context, req domain.SearchProductsRequest) (domain.SearchProductsResponse, error) {
	if err := normalizePaginationMax(&req.Pagination, maxProductsLimit); err != nil {
		return domain.SearchProductsResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchProductsResponse{}, err
	}
	if req.Class != "" && !validProductClass[domain.ProductClass(req.Class)] {
		return domain.SearchProductsResponse{}, &apierror.ValidationError{Msg: "invalid class"}
	}

	products, total, err := s.repo.SearchProducts(ctx, req)
	if err != nil {
		return domain.SearchProductsResponse{}, err
	}

	return domain.SearchProductsResponse{
		Products: products,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(products) < total,
	}, nil
}
