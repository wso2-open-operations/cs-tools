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

// ServiceOfferingRepository defines the read operations for service_offering
// (migration 000049), a shared-PK-free standalone table with an optional
// parent_id FK into service (migration 000048).
type ServiceOfferingRepository interface {
	// SearchServiceOfferings returns a filtered, paginated slice of service
	// offerings together with the total count of matching rows before
	// pagination.
	SearchServiceOfferings(ctx context.Context, serviceIDs []string, searchQuery string, limit, offset int) ([]domain.ServiceOffering, int, error)
}

type serviceOfferingRepo struct {
	db *pgxpool.Pool
}

// NewServiceOfferingRepository constructs a ServiceOfferingRepository backed by the given connection pool.
func NewServiceOfferingRepository(db *pgxpool.Pool) ServiceOfferingRepository {
	return &serviceOfferingRepo{db: db}
}

// SearchServiceOfferings implements ServiceOfferingRepository.
func (r *serviceOfferingRepo) SearchServiceOfferings(ctx context.Context, serviceIDs []string, searchQuery string, limit, offset int) ([]domain.ServiceOffering, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	if len(serviceIDs) > 0 {
		args = append(args, serviceIDs)
		where += fmt.Sprintf(" AND so.parent_id = ANY($%d::uuid[])", len(args))
	}
	if searchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(searchQuery)
		args = append(args, "%"+escaped+"%")
		where += fmt.Sprintf(" AND so.name ILIKE $%d ESCAPE '\\'", len(args))
	}

	countQuery := "SELECT COUNT(*) FROM service_offering so " + where
	dataQuery := fmt.Sprintf(
		`SELECT so.id, so.name, s.id, s.name
		 FROM service_offering so
		 LEFT JOIN service s ON s.id = so.parent_id
		 %s
		 ORDER BY so.name, so.id LIMIT $%d OFFSET $%d`,
		where, len(args)+1, len(args)+2,
	)
	dataArgs := append(append([]any{}, args...), limit, offset)

	var total int
	var offerings []domain.ServiceOffering

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count service offerings: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query service offerings: %w", err)
		}
		defer rows.Close()

		result := make([]domain.ServiceOffering, 0, limit)
		for rows.Next() {
			var (
				id, name       string
				svcID, svcName *string
			)
			if err := rows.Scan(&id, &name, &svcID, &svcName); err != nil {
				return fmt.Errorf("scan service offering: %w", err)
			}
			item := domain.ServiceOffering{ID: id, Name: name}
			if svcID != nil {
				item.Service = &domain.ServiceOfferingServiceRef{ID: *svcID, Name: stringOrEmpty(svcName)}
			}
			result = append(result, item)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate service offerings: %w", err)
		}
		offerings = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return offerings, total, nil
}
