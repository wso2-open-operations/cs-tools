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

	"github.com/jackc/pgx/v5/pgxpool"

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
	db *pgxpool.Pool
}

// NewConfigurationItemService constructs the Postgres-backed service.
func NewConfigurationItemService(db *pgxpool.Pool) ConfigurationItemService {
	return &pgConfigurationItemService{db: db}
}

// maxSearchTerms caps how many words of a picker query become conditions, so
// a pasted paragraph cannot turn into a query with hundreds of ILIKEs. Eight
// is more than any offering name needs to be told apart.
const maxSearchTerms = 8

// likeEscaper makes a typed word match itself under ILIKE ... ESCAPE '\':
// without it "_" matches any character and "%" any run of them.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// configurationItemSearch is the WHERE clause, its arguments and the ORDER BY
// for a picker query, built apart from the database so it can be tested.
type configurationItemSearch struct {
	where     string // "" or " WHERE ...", over service_offering so and service s
	whereArgs []any
	orderBy   string // " ORDER BY ...", numbering its arguments after whereArgs
	orderArgs []any
}

// newConfigurationItemSearch builds the picker's search.
//
// *** EVERY WORD MUST MATCH, IN ANY ORDER, AND THE PARENT SERVICE COUNTS. ***
// The picker shows only the first 20 results (the webapp asks for 20), and
// the query used to be one ILIKE '%whole text%' over the offering name, in
// name order. Offering names repeat their product's words ("Integration CP
// ... - US" next to older offerings that merely mention "Integration"), so a
// product name alone filled the 20 with whatever sorted first, and an
// engineer could not reach the item they wanted. Now each word narrows the
// list ("integration automation eu"), and a word may also match the parent
// service's name, which is where a plane is spelled out ("Control Plane").
//
// Ranking keeps the old behaviour for an exact name: a name that starts with
// the query as typed comes first, then one that contains it, then the rest,
// each in name order. It ranks on the whole query, spacing normalised, even
// past the eighth word the filter stops at: a full name such as "Integration
// CP Automation Creation and Build Service Offering - US" has ten, and its EU
// twin would otherwise rank level with it and sort first.
func newConfigurationItemSearch(query string) configurationItemSearch {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return configurationItemSearch{orderBy: " ORDER BY so.name, so.id"}
	}
	phrase := likeEscaper.Replace(strings.Join(terms, " "))
	if len(terms) > maxSearchTerms {
		terms = terms[:maxSearchTerms]
	}
	search := configurationItemSearch{}
	conds := make([]string, 0, len(terms))
	for _, t := range terms {
		search.whereArgs = append(search.whereArgs, "%"+likeEscaper.Replace(t)+"%")
		// service_offering.name may be NULL; the parent's name can still match.
		conds = append(conds, fmt.Sprintf(
			`(COALESCE(so.name, '') || ' ' || COALESCE(s.name, '')) ILIKE $%d ESCAPE '\'`, len(search.whereArgs)))
	}
	search.where = " WHERE " + strings.Join(conds, " AND ")

	search.orderArgs = []any{phrase + "%", "%" + phrase + "%"}
	n := len(search.whereArgs)
	search.orderBy = fmt.Sprintf(` ORDER BY CASE WHEN so.name ILIKE $%d ESCAPE '\' THEN 0`+
		` WHEN so.name ILIKE $%d ESCAPE '\' THEN 1 ELSE 2 END, so.name, so.id`, n+1, n+2)
	return search
}

// SearchConfigurationItems implements ConfigurationItemService for Postgres.
func (s *pgConfigurationItemService) SearchConfigurationItems(ctx context.Context,
	req domain.SearchConfigurationItemsRequest) (domain.SearchConfigurationItemsResponse, error) {

	query := ""
	if req.Filters != nil {
		query = req.Filters.SearchQuery
	}
	search := newConfigurationItemSearch(query)

	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*)
  FROM service_offering so
  LEFT JOIN service s ON s.id = so.parent_id`+search.where, search.whereArgs...).Scan(&total); err != nil {
		return domain.SearchConfigurationItemsResponse{}, fmt.Errorf("count configuration items: %w", err)
	}

	limit := req.Pagination.Limit
	if limit <= 0 {
		limit = 20
	}
	args := append(append(append([]any{}, search.whereArgs...), search.orderArgs...), limit, req.Pagination.Offset)
	q := fmt.Sprintf(`
SELECT so.id::text, so.name, s.name
  FROM service_offering so
  LEFT JOIN service s ON s.id = so.parent_id%s%s
 LIMIT $%d OFFSET $%d`, search.where, search.orderBy, len(args)-1, len(args))

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
