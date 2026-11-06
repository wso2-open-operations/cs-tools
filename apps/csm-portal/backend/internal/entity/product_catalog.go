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

package entity

import (
	"context"
	"net/http"
	"net/url"
)

// Product catalogue: the products, versions and release lines CSM publishes to
// the public Support Matrix and Release Matrix, served by entity-service's
// products API. Reads are open to internal callers; every write is re-checked
// by entity-service against the caller's product-catalogue-manager role.

// GetProduct calls GET /products/{id}.
func (c *CustomerEntityClient) GetProduct(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/products/"+url.PathEscape(id), nil)
}

// CreateProduct calls POST /products.
func (c *CustomerEntityClient) CreateProduct(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/products", body)
}

// UpdateProduct calls PATCH /products/{id}.
func (c *CustomerEntityClient) UpdateProduct(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, "/products/"+url.PathEscape(id), body)
}

// CreateProductVersions calls POST /products/{id}/versions: one version with
// all its deployment profiles, created in one transaction.
func (c *CustomerEntityClient) CreateProductVersions(ctx context.Context, productID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/products/"+url.PathEscape(productID)+"/versions", body)
}

// SearchAllProductVersions calls POST /product-versions/search (every product).
func (c *CustomerEntityClient) SearchAllProductVersions(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/product-versions/search", body)
}

// GetProductVersion calls GET /product-versions/{id}.
func (c *CustomerEntityClient) GetProductVersion(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/product-versions/"+url.PathEscape(id), nil)
}

// UpdateProductVersion calls PATCH /product-versions/{id}. Listing-level
// fields apply to every live row of the version's public listing.
func (c *CustomerEntityClient) UpdateProductVersion(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, "/product-versions/"+url.PathEscape(id), body)
}

// RetireProductVersion calls POST /product-versions/{id}/retire, which retires
// every live row of the version's public listing.
func (c *CustomerEntityClient) RetireProductVersion(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/product-versions/"+url.PathEscape(id)+"/retire", nil)
}

// ListReleaseLines calls GET /release-lines with the given (includeRetired) query.
func (c *CustomerEntityClient) ListReleaseLines(ctx context.Context, q url.Values) ([]byte, error) {
	path := "/release-lines"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return c.do(ctx, http.MethodGet, path, nil)
}

// CreateReleaseLine calls POST /release-lines.
func (c *CustomerEntityClient) CreateReleaseLine(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/release-lines", body)
}

// UpdateReleaseLine calls PATCH /release-lines/{id}.
func (c *CustomerEntityClient) UpdateReleaseLine(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, "/release-lines/"+url.PathEscape(id), body)
}

// RetireReleaseLine calls POST /release-lines/{id}/retire.
func (c *CustomerEntityClient) RetireReleaseLine(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/release-lines/"+url.PathEscape(id)+"/retire", nil)
}

// ListCarbonPlatformHistory calls GET /carbon-platform-history.
func (c *CustomerEntityClient) ListCarbonPlatformHistory(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/carbon-platform-history", nil)
}

// ListCarbonKernelReleases calls GET /carbon-kernel-releases.
func (c *CustomerEntityClient) ListCarbonKernelReleases(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/carbon-kernel-releases", nil)
}
