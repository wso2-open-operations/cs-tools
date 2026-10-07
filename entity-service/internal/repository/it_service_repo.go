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

package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ITServiceRepository defines the read operations for the service table
// (migration 0044), the CMDB service catalogue. Other tables reference it
// (incident/incident_task/change_request/outage/cloud_monitor.service_id,
// service_offering.parent_id), and its group columns (migration 0075)
// reference "group"; this repository only reads it.
type ITServiceRepository interface {
	// SearchITServices returns a filtered, paginated slice of services
	// together with the total count of matching rows before pagination.
	SearchITServices(ctx context.Context, searchQuery string, limit, offset int) ([]domain.ITService, int, error)
}

type itServiceRepo struct {
	db db.Pool
}

// NewITServiceRepository constructs an ITServiceRepository backed by the given connection pool.
func NewITServiceRepository(db db.Pool) ITServiceRepository {
	return &itServiceRepo{db: db}
}

// itServiceBusinessCriticalityFromEnum maps service.business_criticality's
// real service_business_criticality_enum labels (migration 0044) to
// domain.BusinessCriticality. Unlike case_severity_enum, this one already
// matches the domain enum's values 1:1 once case-folded.
var itServiceBusinessCriticalityFromEnum = map[string]domain.BusinessCriticality{
	"MOST_CRITICAL":     domain.BusinessCriticalityMostCritical,
	"SOMEWHAT_CRITICAL": domain.BusinessCriticalitySomewhatCritical,
	"LESS_CRITICAL":     domain.BusinessCriticalityLessCritical,
	"NOT_CRITICAL":      domain.BusinessCriticalityNotCritical,
}

// SearchITServices implements ITServiceRepository.
//
// domain.ITService.Class is mapped from service.category (a free-text
// VARCHAR) -- the same choice product_repo.go's SearchProducts already made
// for product.category -> domain.Product.Class, for the same reason: no
// column named "class" exists, and category is the closest real analogue.
// ServiceClassification (business_service/technology_management_service/
// application_service on the ServiceNow data source) has no corresponding
// column anywhere on service -- category/subcategory are free text, not
// drawn from that three-value set -- so it is always left nil rather than
// guessed at from category's contents.
//
// domain.ITService.SupportGroup is resolved via service.support_group_id
// (migration 0075) LEFT JOINed against "group". That column existed since
// 0075 but was never selected here, so every service came back with no
// support group and the CSM portal's Create Incident page -- which defaults
// the incident's assignment group to the selected service's support group --
// always showed it blank. Support group is the ServiceNow/CSDM field for the
// team that handles a service's incidents; service.assignment_group_id is a
// different, generic CI field (empty for every synced service when checked)
// and is deliberately not used for this. managed_by_group_id/
// approval_group_id/user_group_id from the same migration stay unselected:
// domain.ITService has no field for them.
func (r *itServiceRepo) SearchITServices(ctx context.Context, searchQuery string, limit, offset int) ([]domain.ITService, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	if searchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(searchQuery)
		args = append(args, "%"+escaped+"%")
		where += fmt.Sprintf(" AND (s.name ILIKE $%d ESCAPE '\\' OR s.number ILIKE $%d ESCAPE '\\')", len(args), len(args))
	}

	countQuery := "SELECT COUNT(*) FROM service s " + where
	dataQuery := fmt.Sprintf(
		`SELECT s.id, s.name, s.category, s.business_criticality::TEXT, sg.id, sg.name
		 FROM service s
		 LEFT JOIN "group" sg ON sg.id = s.support_group_id
		 %s
		 ORDER BY s.created_on DESC, s.id
		 LIMIT $%d OFFSET $%d`,
		where, len(args)+1, len(args)+2,
	)
	dataArgs := append(append([]any{}, args...), limit, offset)

	var total int
	var services []domain.ITService

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count services: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query services: %w", err)
		}
		defer rows.Close()

		result := make([]domain.ITService, 0, limit)
		for rows.Next() {
			var (
				id                               string
				name, category                   *string
				businessCriticality              *string
				supportGroupID, supportGroupName *string
			)
			if err := rows.Scan(&id, &name, &category, &businessCriticality, &supportGroupID, &supportGroupName); err != nil {
				return fmt.Errorf("scan service: %w", err)
			}
			item := domain.ITService{ID: id, Name: name, Class: category}
			if businessCriticality != nil {
				if bc, ok := itServiceBusinessCriticalityFromEnum[*businessCriticality]; ok {
					item.BusinessCriticality = &bc
				}
			}
			if supportGroupID != nil {
				item.SupportGroup = &domain.EntityRef{ID: *supportGroupID, Name: stringOrEmpty(supportGroupName)}
			}
			result = append(result, item)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate services: %w", err)
		}
		services = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return services, total, nil
}
