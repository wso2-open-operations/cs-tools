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

// Integration test: GetUserByEmail (what GET /users/me resolves the caller
// through) returns "user".phone, and nil when the column is NULL. Same DSN and
// skip-when-unset pattern as user_by_ids_integration_test.go (reuses
// caseStatsPool):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run UserByEmailPhone

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestUserByEmailPhone(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := context.Background()

	const (
		withPhoneID = "3b222222-0000-0000-0000-000000000001"
		noPhoneID   = "3b222222-0000-0000-0000-000000000002"
	)
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = ANY($1)`, []string{withPhoneID, noPhoneID})
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name, first_name, last_name, email, phone)
	    VALUES ($1, now(), now(), 'me-phone-test-1', 'Jane', 'Doe', 'jane.doe@example.com', '+15555550123'),
	           ($2, now(), now(), 'me-phone-test-2', 'John', 'Doe', 'john.doe@example.com', NULL)`,
		withPhoneID, noPhoneID); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	repo := repository.NewUserRepository(pool)

	u, err := repo.GetUserByEmail(ctx, "jane.doe@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail (with phone): %v", err)
	}
	if u.Phone == nil || *u.Phone != "+15555550123" {
		t.Errorf("Phone = %v, want +15555550123", u.Phone)
	}

	u, err = repo.GetUserByEmail(ctx, "john.doe@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail (NULL phone): %v", err)
	}
	if u.Phone != nil {
		t.Errorf("Phone = %q, want nil for a NULL column", *u.Phone)
	}
}

// TestUpdateUserProfile proves the PATCH /users/me write: each field is
// independent (absent leaves it untouched), phone can be set and cleared, and
// the returned state is what was stored.
func TestUpdateUserProfile(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := context.Background()

	const id = "3b222222-0000-0000-0000-000000000003"
	cleanup := func() { _, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, id) }
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name, first_name, last_name, email)
	    VALUES ($1, now(), now(), 'me-phone-test-3', 'Jane', 'Doe', 'jane.doe.patch@example.com')`, id); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	repo := repository.NewUserRepository(pool)
	p := func(s string) *string { return &s }

	got, err := repo.UpdateUserProfile(ctx, id, nil, p("+15555550123"))
	if err != nil || got.Phone == nil || *got.Phone != "+15555550123" || got.Timezone != nil {
		t.Fatalf("phone only: %+v, %v", got, err)
	}
	got, err = repo.UpdateUserProfile(ctx, id, nil, nil)
	if err != nil || got.Phone == nil || *got.Phone != "+15555550123" {
		t.Fatalf("no-op update must keep phone: %+v, %v", got, err)
	}
	got, err = repo.UpdateUserProfile(ctx, id, nil, p(""))
	if err != nil || got.Phone != nil {
		t.Fatalf("empty phone must clear to NULL: %+v, %v", got, err)
	}
	if _, err := repo.UpdateUserProfile(ctx, "3b222222-0000-0000-0000-0000000000ff", nil, p("1")); err == nil {
		t.Fatal("unknown user: want NotFoundError")
	}
}
