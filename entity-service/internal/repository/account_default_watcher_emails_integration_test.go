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

// AccountDefaultWatcherEmails: the account's technical owner, secondary
// technical owner, account manager, renewal account manager and customer
// success manager, plus the project's own onboarding owner while (and only
// while) the project's onboarding_status is IN_PROGRESS. Runs against a real
// Postgres. Skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run AccountDefaultWatcherEmailsIntegration

package repository_test

import (
	"context"
	"sort"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	adweUserTOW  = "92000000-0000-0000-0000-000000000001"
	adweUserSTOW = "92000000-0000-0000-0000-000000000002"
	adweUserAM   = "92000000-0000-0000-0000-000000000003"
	adweUserRAM  = "92000000-0000-0000-0000-000000000004"
	adweUserCSM  = "92000000-0000-0000-0000-000000000005"
	adweUserOnb  = "92000000-0000-0000-0000-000000000006"

	adweAccount = "92000000-0000-0000-0000-0000000000a1"

	adweProjInProgress = "92000000-0000-0000-0000-0000000000b1"
	adweProjCompleted  = "92000000-0000-0000-0000-0000000000b2"
	adweProjNoOwner    = "92000000-0000-0000-0000-0000000000b3"
	adweProjNoAccount  = "92000000-0000-0000-0000-0000000000b4"
)

func TestAccountDefaultWatcherEmailsIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())

	userIDs := []string{adweUserTOW, adweUserSTOW, adweUserAM, adweUserRAM, adweUserCSM, adweUserOnb}
	projIDs := []string{adweProjInProgress, adweProjCompleted, adweProjNoOwner, adweProjNoAccount}
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = ANY($1::uuid[])`, projIDs)
		_, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, adweAccount)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = ANY($1::uuid[])`, userIDs)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email) VALUES
		($1, now(), now(), 'adwe-tow', 'Tina Owner', 'tow@example.test'),
		($2, now(), now(), 'adwe-stow', 'Sam Secondary', 'stow@example.test'),
		($3, now(), now(), 'adwe-am', 'Alex Manager', 'am@example.test'),
		($4, now(), now(), 'adwe-ram', 'Rae Renewal', 'ram@example.test'),
		($5, now(), now(), 'adwe-csm', 'Casey Success', 'csm@example.test'),
		($6, now(), now(), 'adwe-onb', 'Oni Onboarder', 'onboarder@example.test')`,
		adweUserTOW, adweUserSTOW, adweUserAM, adweUserRAM, adweUserCSM, adweUserOnb)

	mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id,
			technical_owner_id, secondary_technical_owner_id, account_manager_id, renewal_account_manager_id, customer_success_manager_id)
		VALUES ($1, now(), now(), 't', 't', 'ADWE Account', 'ACC-ADWE-1', 'SF-ADWE-1', $2, $3, $4, $5, $6)`,
		adweAccount, adweUserTOW, adweUserSTOW, adweUserAM, adweUserRAM, adweUserCSM)

	insertProject := func(id, key string, onboardingStatus *string, onboardingOwner *string) {
		mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, sf_id, account_id, onboarding_status, onboarding_owner_id)
			VALUES ($1, now(), now(), 't', 't', $2, $2, 'sf-'||$2, $3, $4::onboarding_status_enum, $5)`,
			id, key, adweAccount, onboardingStatus, onboardingOwner)
	}
	inProgress := "IN_PROGRESS"
	completed := "COMPLETED"
	onboardingOwner := adweUserOnb
	insertProject(adweProjInProgress, "ADWEINPROG", &inProgress, &onboardingOwner)
	insertProject(adweProjCompleted, "ADWECOMPLETE", &completed, &onboardingOwner)
	insertProject(adweProjNoOwner, "ADWENOOWNER", &inProgress, nil)
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, sf_id)
		VALUES ($1, now(), now(), 't', 't', 'ADWENOACCT', 'ADWENOACCT', 'sf-adwenoacct')`, adweProjNoAccount)

	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	sorted := func(ss []string) []string {
		out := append([]string(nil), ss...)
		sort.Strings(out)
		return out
	}
	eq := func(t *testing.T, got []string, want ...string) {
		t.Helper()
		g, w := sorted(got), sorted(want)
		if len(g) != len(w) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range g {
			if g[i] != w[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	}

	t.Run("onboarding IN_PROGRESS includes the onboarding owner alongside the four stakeholders and the CSM", func(t *testing.T) {
		got, err := repo.AccountDefaultWatcherEmails(ctx, adweProjInProgress)
		if err != nil {
			t.Fatalf("AccountDefaultWatcherEmails: %v", err)
		}
		eq(t, got, "tow@example.test", "stow@example.test", "am@example.test", "ram@example.test", "csm@example.test", "onboarder@example.test")
	})

	t.Run("onboarding not IN_PROGRESS excludes the onboarding owner even though one is set", func(t *testing.T) {
		got, err := repo.AccountDefaultWatcherEmails(ctx, adweProjCompleted)
		if err != nil {
			t.Fatalf("AccountDefaultWatcherEmails: %v", err)
		}
		eq(t, got, "tow@example.test", "stow@example.test", "am@example.test", "ram@example.test", "csm@example.test")
	})

	t.Run("onboarding IN_PROGRESS with no onboarding owner set still returns the account stakeholders", func(t *testing.T) {
		got, err := repo.AccountDefaultWatcherEmails(ctx, adweProjNoOwner)
		if err != nil {
			t.Fatalf("AccountDefaultWatcherEmails: %v", err)
		}
		eq(t, got, "tow@example.test", "stow@example.test", "am@example.test", "ram@example.test", "csm@example.test")
	})

	t.Run("project with no linked account returns an empty slice, not an error", func(t *testing.T) {
		got, err := repo.AccountDefaultWatcherEmails(ctx, adweProjNoAccount)
		if err != nil {
			t.Fatalf("AccountDefaultWatcherEmails: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got %v, want empty", got)
		}
	})

	t.Run("a project id that does not exist returns an empty slice, not an error", func(t *testing.T) {
		got, err := repo.AccountDefaultWatcherEmails(ctx, "00000000-0000-0000-0000-000000000000")
		if err != nil {
			t.Fatalf("AccountDefaultWatcherEmails: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got %v, want empty", got)
		}
	})
}
