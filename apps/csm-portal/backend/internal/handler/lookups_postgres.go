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
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// entityProductsClient is the subset of internal/entity.CustomerEntityClient
// needed to list products from Postgres.
type entityProductsClient interface {
	SearchProducts(ctx context.Context, body []byte) ([]byte, error)
}

type entitySearchProductsRequest struct {
	Pagination entityPagination `json:"pagination"`
}

type entityProduct struct {
	Name string `json:"name"`
}

type entitySearchProductsResponse struct {
	Products []entityProduct `json:"products"`
	Total    int             `json:"total"`
}

// postgresLookupsClient implements lookupsClient: GetProductList reads
// from entity-service (Postgres) — entity-service's productService already
// exists natively. GetABTTeamList is NOT migrated: it comes off the backing system's
// sys_user_group table, and entity-service has no team/group domain object
// yet, so it still delegates to the wrapped the backing system client.
type postgresLookupsClient struct {
	entity entityProductsClient
	sn     lookupsClient
}

// NewPostgresLookupsClient builds a postgresLookupsClient. entity is
// the same *entity.CustomerEntityClient every other CS Portal handler uses;
// sn is the existing the backing system client, kept only for ABT team names.
func NewPostgresLookupsClient(entity entityProductsClient, sn lookupsClient) *postgresLookupsClient {
	return &postgresLookupsClient{entity: entity, sn: sn}
}

// entityProductsPageLimit is entity-service's own hard cap (confirmed
// against a real running instance: a limit above 50 is rejected outright
// with "limit cannot exceed 50", the same global cap normalizePagination
// enforces everywhere else in that service) — unlike the legacy-data-source
// implementation, which fetches up to 500 products in one page, this must
// page through in batches of 50. productListPageCap bounds the number of
// pages fetched (25 * 50 = 1250 products) purely as a runaway-loop safety
// net, not an expected real limit.
const (
	entityProductsPageLimit = 50
	productListPageCap      = 25
)

// GetProductList implements lookupsClient.
func (c *postgresLookupsClient) GetProductList(ctx context.Context) ([]string, error) {
	seen := make(map[string]bool)
	products := make([]string, 0, entityProductsPageLimit)

	for page := 0; page < productListPageCap; page++ {
		body, err := json.Marshal(entitySearchProductsRequest{
			Pagination: entityPagination{Limit: entityProductsPageLimit, Offset: page * entityProductsPageLimit},
		})
		if err != nil {
			return nil, fmt.Errorf("marshal entity-service products request: %w", err)
		}
		raw, err := c.entity.SearchProducts(ctx, body)
		if err != nil {
			return nil, err
		}
		var resp entitySearchProductsResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal entity-service products response: %w", err)
		}

		// Trim/dedupe/sort matches the legacy-data-source implementation's
		// own behavior exactly (see internal/servicenow/lookups.go) so
		// callers see no difference in shape.
		for _, p := range resp.Products {
			name := strings.TrimSpace(p.Name)
			if name != "" && !seen[name] {
				seen[name] = true
				products = append(products, name)
			}
		}

		if len(resp.Products) < entityProductsPageLimit || (page+1)*entityProductsPageLimit >= resp.Total {
			break
		}
	}

	sort.Strings(products)
	return products, nil
}

// GetABTTeamList implements lookupsClient by delegating to the wrapped
// The backing system client — see this type's own doc comment for why.
func (c *postgresLookupsClient) GetABTTeamList(ctx context.Context) ([]string, error) {
	return c.sn.GetABTTeamList(ctx)
}
