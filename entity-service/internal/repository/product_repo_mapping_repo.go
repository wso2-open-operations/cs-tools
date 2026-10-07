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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ProductRepoMappingRepository reads product_repo_mapping.
type ProductRepoMappingRepository struct {
	db db.Pool
}

// NewProductRepoMappingRepository returns a repository bound to db.
func NewProductRepoMappingRepository(db db.Pool) *ProductRepoMappingRepository {
	return &ProductRepoMappingRepository{db: db}
}

// ListActive returns every active mapping. The table is a small catalogue.
func (r *ProductRepoMappingRepository) ListActive(ctx context.Context) ([]domain.ProductRepoMapping, error) {
	rows, err := r.db.Query(ctx, `
		SELECT product_name, abbreviation, owner, repository, github_label
		FROM product_repo_mapping
		WHERE is_active = TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ProductRepoMapping
	for rows.Next() {
		var m domain.ProductRepoMapping
		if err := rows.Scan(&m.ProductName, &m.Abbreviation, &m.Owner, &m.Repository, &m.GithubLabel); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
