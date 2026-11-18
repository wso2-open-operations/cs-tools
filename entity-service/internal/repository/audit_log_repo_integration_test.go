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
// AuditLogRepository against a real Postgres with migration 0295 applied.
// Skipped without ENTITY_TEST_DATABASE_URL:
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/repository/ -run AuditLog

package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func newAuditLogTestRepo(t *testing.T) repository.AuditLogRepository {
	t.Helper()
	url := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return repository.NewAuditLogRepository(repository.NewScoped(pool))
}

func TestAuditLogRepository_CreatePartitionsIsRepeatable(t *testing.T) {
	repo := newAuditLogTestRepo(t)
	ctx := context.Background()

	if _, err := repo.CreatePartitions(ctx, 3); err != nil {
		t.Fatalf("first create: %v", err)
	}
	created, err := repo.CreatePartitions(ctx, 3)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if created != 0 {
		t.Errorf("second create made %d partitions, want 0", created)
	}
}

func TestAuditLogRepository_HealthyAfterCreatingPartitions(t *testing.T) {
	repo := newAuditLogTestRepo(t)
	ctx := context.Background()

	if _, err := repo.CreatePartitions(ctx, 3); err != nil {
		t.Fatalf("create: %v", err)
	}
	problems, err := repo.PartitionHealth(ctx, 3, 18)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if len(problems) != 0 {
		t.Errorf("health problems = %v, want none", problems)
	}
}

func TestAuditLogRepository_ReconcileRuns(t *testing.T) {
	repo := newAuditLogTestRepo(t)

	if _, err := repo.Reconcile(context.Background(), false); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}
