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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ProjectAggregateRepository counts projects per grouping value. It is apart
// from ProjectRepository so that interface and its test fakes are unchanged.
type ProjectAggregateRepository interface {
	// AggregateProjects counts the projects SearchProjects would match for
	// req.Filters and scope, per req.GroupBy value, largest first, every
	// bucket returned (the service caps them), plus the matched projects with
	// no value for the grouping.
	AggregateProjects(ctx context.Context, req domain.AggregateProjectsRequest, scope SearchScope) (buckets []domain.AggregateBucket, keyless int, err error)
}

// NewProjectAggregateRepository constructs a ProjectAggregateRepository.
func NewProjectAggregateRepository(db *Scoped) ProjectAggregateRepository {
	return &projectRepo{db: db}
}

// projectAggregateColumns is each grouping's bucket key and display label,
// over the aliases buildProjectSearchWhere and the owner join use. The owner
// label is the search row's owner name, else the email, else the id; a
// region's key ignores case and surrounding spaces, its label keeps one
// spelling the accounts use; the status reads as the search row's does.
var projectAggregateColumns = map[string]struct{ key, label string }{
	domain.ProjectGroupByOnboardingOwner: {"onb.id::TEXT",
		"MIN(COALESCE(onb.name, NULLIF(TRIM(CONCAT_WS(' ', onb.first_name, onb.last_name)), ''), onb.email, onb.id::TEXT))"},
	domain.ProjectGroupByOnboardingStatus: {"INITCAP(REPLACE(p.onboarding_status::TEXT, '_', '-'))",
		"MIN(INITCAP(REPLACE(p.onboarding_status::TEXT, '_', '-')))"},
	domain.ProjectGroupByRegion: {"NULLIF(LOWER(TRIM(a.region)), '')", "MIN(TRIM(a.region))"},
}

// AggregateProjects implements ProjectAggregateRepository.
func (r *projectRepo) AggregateProjects(ctx context.Context, req domain.AggregateProjectsRequest, scope SearchScope) ([]domain.AggregateBucket, int, error) {
	col, ok := projectAggregateColumns[req.GroupBy]
	if !ok {
		return nil, 0, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + req.GroupBy}
	}
	// The same caller identity as SearchProjects, for the same RLS reasons.
	ctx = WithCallerIdentity(ctx, scope)
	where, args, _, err := buildProjectSearchWhere(req.Filters, scope)
	if err != nil {
		return nil, 0, err
	}
	query := fmt.Sprintf(`
		SELECT %s AS bucket, %s AS bucket_label, COUNT(*) AS bucket_count
		FROM project p
		LEFT JOIN project_type pt ON pt.id = p.project_type_id
		LEFT JOIN account a ON a.id = p.account_id
		LEFT JOIN "user" onb ON onb.id = p.onboarding_owner_id
		%s
		GROUP BY 1
		ORDER BY bucket_count DESC, bucket_label, bucket`, col.key, col.label, where)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("aggregate projects: %w", err)
	}
	defer rows.Close()

	var buckets []domain.AggregateBucket
	keyless := 0
	for rows.Next() {
		var key, label *string
		var count int
		if err := rows.Scan(&key, &label, &count); err != nil {
			return nil, 0, fmt.Errorf("scan project bucket: %w", err)
		}
		if key == nil {
			keyless += count
			continue
		}
		b := domain.AggregateBucket{Key: *key, Label: *key, Count: count}
		if label != nil && *label != "" {
			b.Label = *label
		}
		buckets = append(buckets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("aggregate projects: %w", err)
	}
	return buckets, keyless, nil
}
