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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.
package repository

import (
	"context"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// lockTimeout is how long a deletion waits for a lock before giving up. The next
// daily run tries again.
const lockTimeout = "2s"

// AuditLogRepository runs the audit log's upkeep functions (migration 0295).
// They are SECURITY DEFINER, so the application role needs EXECUTE on them and
// nothing else on the audit tables.
type AuditLogRepository interface {
	// DeleteExpiredPartitions drops the monthly partitions that ended at least
	// retentionMonths ago and returns their names.
	DeleteExpiredPartitions(ctx context.Context, retentionMonths int) ([]string, error)
	// DeleteExpiredDefaultRows deletes the catch-all partition's rows older than
	// retentionMonths, which dropping partitions cannot reach, and returns how many.
	DeleteExpiredDefaultRows(ctx context.Context, retentionMonths int) (int64, error)
	// CreatePartitions creates the missing monthly partitions and returns how
	// many it created.
	CreatePartitions(ctx context.Context, monthsAhead int) (int, error)
	// PartitionHealth returns one message per problem; none means healthy. It
	// reports a catch-all row and fewer than monthsAhead future monthly
	// partitions.
	PartitionHealth(ctx context.Context, monthsAhead int) ([]string, error)
	// Reconcile compares the registry with the triggers that exist. With repair,
	// missing or disabled triggers are put back.
	Reconcile(ctx context.Context, repair bool) ([]domain.AuditTriggerProblem, error)
}

type auditLogRepository struct {
	db *Scoped
}

// NewAuditLogRepository constructs the repository over the scoped pool.
func NewAuditLogRepository(db *Scoped) AuditLogRepository {
	return &auditLogRepository{db: db}
}

// CreatePartitions implements AuditLogRepository.
func (r *auditLogRepository) CreatePartitions(ctx context.Context, monthsAhead int) (int, error) {
	var created int
	err := r.db.QueryRow(WithSystemIdentity(ctx), `SELECT audit_create_partitions($1)`, monthsAhead).Scan(&created)
	if err != nil {
		return 0, fmt.Errorf("audit log: create partitions: %w", err)
	}
	return created, nil
}

// DeleteExpiredPartitions implements AuditLogRepository.
func (r *auditLogRepository) DeleteExpiredPartitions(ctx context.Context, retentionMonths int) ([]string, error) {
	rows, err := r.db.Query(WithSystemIdentity(ctx), `SELECT audit_delete_expired_partitions($1, $2)`, retentionMonths, lockTimeout)
	if err != nil {
		return nil, fmt.Errorf("audit log: delete expired partitions: %w", err)
	}
	defer rows.Close()

	var deleted []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("audit log: scan deleted partition: %w", err)
		}
		deleted = append(deleted, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit log: delete expired partitions: %w", err)
	}
	return deleted, nil
}

// DeleteExpiredDefaultRows implements AuditLogRepository.
func (r *auditLogRepository) DeleteExpiredDefaultRows(ctx context.Context, retentionMonths int) (int64, error) {
	var deleted int64
	err := r.db.QueryRow(WithSystemIdentity(ctx), `SELECT audit_delete_expired_default_rows($1, $2)`, retentionMonths, lockTimeout).Scan(&deleted)
	if err != nil {
		return 0, fmt.Errorf("audit log: delete expired default rows: %w", err)
	}
	return deleted, nil
}

// PartitionHealth implements AuditLogRepository.
func (r *auditLogRepository) PartitionHealth(ctx context.Context, monthsAhead int) ([]string, error) {
	rows, err := r.db.Query(WithSystemIdentity(ctx), `SELECT problem FROM audit_partition_health($1)`, monthsAhead)
	if err != nil {
		return nil, fmt.Errorf("audit log: partition health: %w", err)
	}
	defer rows.Close()

	var problems []string
	for rows.Next() {
		var problem string
		if err := rows.Scan(&problem); err != nil {
			return nil, fmt.Errorf("audit log: scan partition health: %w", err)
		}
		problems = append(problems, problem)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit log: partition health: %w", err)
	}
	return problems, nil
}

// Reconcile implements AuditLogRepository.
func (r *auditLogRepository) Reconcile(ctx context.Context, repair bool) ([]domain.AuditTriggerProblem, error) {
	rows, err := r.db.Query(WithSystemIdentity(ctx), `SELECT schema_name, table_name, problem FROM audit_reconcile($1)`, repair)
	if err != nil {
		return nil, fmt.Errorf("audit log: reconcile: %w", err)
	}
	defer rows.Close()

	var problems []domain.AuditTriggerProblem
	for rows.Next() {
		var p domain.AuditTriggerProblem
		if err := rows.Scan(&p.Schema, &p.Table, &p.Problem); err != nil {
			return nil, fmt.Errorf("audit log: scan reconcile: %w", err)
		}
		problems = append(problems, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit log: reconcile: %w", err)
	}
	return problems, nil
}
