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

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// These run against a real Postgres with the migrations applied. Skipped
// without GITHUB_SYNC_TEST_DSN, so an ordinary `go test ./...` stays hermetic.
//
//	GITHUB_SYNC_TEST_DSN=postgres://... go test ./internal/repository/ -run Integration
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("GITHUB_SYNC_TEST_DSN")
	if dsn == "" {
		t.Skip("GITHUB_SYNC_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seed(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`DELETE FROM github_webhook_delivery`,
		`DELETE FROM account_github_repo`,
		`INSERT INTO account (id,created_on,updated_on,created_by,updated_by,name,number,sf_id)
		 VALUES ('11111111-0000-4000-8000-000000000001',now(),now(),'t','t','Choreo Customer','ACC-GH-1','SF-GH-1')
		 ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO account_github_repo (id,created_by,updated_by,account_id,owner,repository)
		 VALUES (gen_random_uuid(),'t','t','11111111-0000-4000-8000-000000000001','WSO2','Choreo')
		 ON CONFLICT (account_id) DO NOTHING`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("seed %q: %v", s[:40], err)
		}
	}
}

// GitHub routes case-insensitively while preserving the case a repository was
// created with. ServiceNow compared exactly, so "Choreo" never matched
// "choreo" and those change requests got a literal "NULL" assignment group.
func TestIntegration_RepoMappingIsCaseInsensitive(t *testing.T) {
	pool := testPool(t)
	seed(t, pool)
	repo := repository.NewGithubSyncRepository(pool)
	ctx := context.Background()

	for _, tc := range [][2]string{
		{"WSO2", "Choreo"}, {"wso2", "choreo"}, {"WsO2", "ChOrEo"},
	} {
		got, err := repo.RepoMapping(ctx, tc[0], tc[1])
		if err != nil {
			t.Fatalf("RepoMapping(%v): %v", tc, err)
		}
		if got == nil {
			t.Fatalf("RepoMapping(%s/%s) = nil, want the account mapping", tc[0], tc[1])
		}
		if got.AccountName != "Choreo Customer" {
			t.Fatalf("account = %q", got.AccountName)
		}
	}
}

// An unmapped repository is ordinary, not an error: the table is the
// allow-list.
func TestIntegration_UnmappedRepoIsNilNotError(t *testing.T) {
	pool := testPool(t)
	seed(t, pool)
	got, err := repository.NewGithubSyncRepository(pool).
		RepoMapping(context.Background(), "someone", "unmapped")
	if err != nil {
		t.Fatalf("RepoMapping: %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

// The idempotency claim: the first attempt takes it, a redelivery is refused,
// and releasing lets a genuine retry back in.
func TestIntegration_DeliveryClaim(t *testing.T) {
	pool := testPool(t)
	seed(t, pool)
	repo := repository.NewGithubSyncRepository(pool)
	ctx := context.Background()

	if err := repo.ClaimDelivery(ctx, "delivery-1", "issues", "labeled"); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	err := repo.ClaimDelivery(ctx, "delivery-1", "issues", "labeled")
	if !errors.Is(err, repository.ErrDeliverySeen) {
		t.Fatalf("second claim = %v, want ErrDeliverySeen", err)
	}

	if err := repo.ReleaseDelivery(ctx, "delivery-1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := repo.ClaimDelivery(ctx, "delivery-1", "issues", "labeled"); err != nil {
		t.Fatalf("claim after release: %v", err)
	}
}

// An empty action must store NULL rather than an empty string, so the column
// means "no action" rather than "an action that is blank".
func TestIntegration_DeliveryWithoutAction(t *testing.T) {
	pool := testPool(t)
	seed(t, pool)
	repo := repository.NewGithubSyncRepository(pool)
	ctx := context.Background()

	if err := repo.ClaimDelivery(ctx, "delivery-2", "ping", ""); err != nil {
		t.Fatalf("claim: %v", err)
	}
	var isNull bool
	if err := pool.QueryRow(ctx,
		`SELECT action IS NULL FROM github_webhook_delivery WHERE delivery_id='delivery-2'`).
		Scan(&isNull); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !isNull {
		t.Fatal("empty action stored as '' rather than NULL")
	}
}
