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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ITServiceRepository defines the read operations for the standalone
// service table (migration 000048). service has no FK to any other table in
// this schema (project/deployment/deployed_product/work_item all reference
// it nowhere) -- it exists purely as a searchable catalogue today.
type ITServiceRepository interface {
	// SearchITServices returns a filtered, paginated slice of services
	// together with the total count of matching rows before pagination.
	SearchITServices(ctx context.Context, searchQuery string, limit, offset int) ([]domain.ITService, int, error)
}

type itServiceRepo struct {
	db *pgxpool.Pool
}

// NewITServiceRepository constructs an ITServiceRepository backed by the given connection pool.
func NewITServiceRepository(db *pgxpool.Pool) ITServiceRepository {
	return &itServiceRepo{db: db}
}

// itServiceBusinessCriticalityFromEnum maps service.business_criticality's
// real service_business_criticality_enum labels (migration 000048) to
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
func (r *itServiceRepo) SearchITServices(ctx context.Context, searchQuery string, limit, offset int) ([]domain.ITService, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	if searchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(searchQuery)
		args = append(args, "%"+escaped+"%")
		where += fmt.Sprintf(" AND (name ILIKE $%d ESCAPE '\\' OR number ILIKE $%d ESCAPE '\\')", len(args), len(args))
	}

	countQuery := "SELECT COUNT(*) FROM service " + where
	dataQuery := fmt.Sprintf(
		`SELECT id, name, category, business_criticality::TEXT
		 FROM service %s
		 ORDER BY created_on DESC, id
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
				id                  string
				name, category      *string
				businessCriticality *string
			)
			if err := rows.Scan(&id, &name, &category, &businessCriticality); err != nil {
				return fmt.Errorf("scan service: %w", err)
			}
			item := domain.ITService{ID: id, Name: name, Class: category}
			if businessCriticality != nil {
				if bc, ok := itServiceBusinessCriticalityFromEnum[*businessCriticality]; ok {
					item.BusinessCriticality = &bc
				}
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
