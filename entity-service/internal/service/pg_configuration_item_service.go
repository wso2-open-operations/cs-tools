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
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// pgConfigurationItemService is the Postgres implementation of
// ConfigurationItemService.
//
// *** IT SEARCHES service_offering, NOT A GENERIC CMDB TABLE. *** The outage
// API documents configurationItemId as having to resolve to a
// service_offering-class item -- it is the publish switch, and publication is
// decided by whether the offering has a cloud_monitor row. Returning anything
// broader would let the form offer items an outage can never publish from,
// and the create would then fail at write time instead of never being
// offered.
type pgConfigurationItemService struct {
	db db.Pool
}

// NewConfigurationItemService constructs the Postgres-backed service.
func NewConfigurationItemService(db db.Pool) ConfigurationItemService {
	return &pgConfigurationItemService{db: db}
}

// SearchConfigurationItems implements ConfigurationItemService for Postgres.
func (s *pgConfigurationItemService) SearchConfigurationItems(ctx context.Context,
	req domain.SearchConfigurationItemsRequest) (domain.SearchConfigurationItemsResponse, error) {

	args := []any{}
	clause := ""
	if req.Filters != nil && strings.TrimSpace(req.Filters.SearchQuery) != "" {
		args = append(args, "%"+strings.TrimSpace(req.Filters.SearchQuery)+"%")
		clause = " WHERE so.name ILIKE $1"
	}

	var total int
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM service_offering so"+clause, args...).Scan(&total); err != nil {
		return domain.SearchConfigurationItemsResponse{}, fmt.Errorf("count configuration items: %w", err)
	}

	limit := req.Pagination.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit, req.Pagination.Offset)
	q := fmt.Sprintf(`
SELECT so.id::text, so.name, s.name
  FROM service_offering so
  LEFT JOIN service s ON s.id = so.parent_id%s
 ORDER BY so.name, so.id
 LIMIT $%d OFFSET $%d`, clause, len(args)-1, len(args))

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return domain.SearchConfigurationItemsResponse{}, fmt.Errorf("search configuration items: %w", err)
	}
	defer rows.Close()

	items := []domain.ConfigurationItem{}
	for rows.Next() {
		var ci domain.ConfigurationItem
		var parent *string
		if err := rows.Scan(&ci.ID, &ci.Name, &parent); err != nil {
			return domain.SearchConfigurationItemsResponse{}, fmt.Errorf("scan configuration item: %w", err)
		}
		// Description carries the parent service, which is how an engineer
		// tells "API Insights - US" from "API Insights - EU" in the picker.
		ci.Description = parent
		class := "service_offering"
		ci.Class = &class
		items = append(items, ci)
	}
	if err := rows.Err(); err != nil {
		return domain.SearchConfigurationItemsResponse{}, err
	}

	return domain.SearchConfigurationItemsResponse{
		ConfigurationItems: items, Total: total, Offset: req.Pagination.Offset, Limit: limit,
	}, nil
}
