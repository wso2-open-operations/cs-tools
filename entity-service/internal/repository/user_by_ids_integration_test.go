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

// This is an integration test: it runs UserService.GetUsersByIDs over the real
// user repository against a live PostgreSQL instance, regression-testing a
// failure Postgres itself produces. "user".id is a UUID column, so a lookup
// that binds a non-UUID value ("invalid input syntax for type uuid") fails the
// whole query, and with it the lookup of every valid id in the same batch --
// which a fake repository cannot reproduce. Same DSN and skip-when-unset
// pattern as project_case_stats_repo_integration_test.go (package
// repository_test, reuses caseStatsPool):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run UsersByIDs

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const usersByIDsUserID = "3b111111-0000-0000-0000-000000000001"

// TestUsersByIDsToleratesNonUUIDIDs is the original failure: a batch that mixes
// a real user id with the free-text values a caller can legitimately send
// (an email address, an empty author id, a system name) must still resolve the
// real user, not fail as a whole.
func TestUsersByIDsToleratesNonUUIDIDs(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, usersByIDsUserID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name, first_name, last_name, email)
	                             VALUES ($1, now(), now(), 'users-by-ids-test', 'Ada', 'Lovelace', 'ada@example.com')`,
		usersByIDsUserID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	svc := service.NewUserService(repository.NewUserRepository(pool))

	resp, err := svc.GetUsersByIDs(ctx, []string{
		usersByIDsUserID,
		"someone@example.com",
		"",
		"system",
	})
	if err != nil {
		t.Fatalf("GetUsersByIDs: %v", err)
	}
	if len(resp.Users) != 1 || resp.Users[0].ID != usersByIDsUserID {
		t.Fatalf("Users = %+v, want exactly the one seeded user %s", resp.Users, usersByIDsUserID)
	}
	if resp.Users[0].FirstName != "Ada" || resp.Users[0].LastName != "Lovelace" {
		t.Errorf("user name = %q %q, want Ada Lovelace", resp.Users[0].FirstName, resp.Users[0].LastName)
	}

	// A batch with nothing that can match answers empty, without an error.
	resp, err = svc.GetUsersByIDs(ctx, []string{"someone@example.com", ""})
	if err != nil {
		t.Fatalf("GetUsersByIDs (no valid ids): %v", err)
	}
	if len(resp.Users) != 0 {
		t.Errorf("Users = %+v, want none", resp.Users)
	}
}
