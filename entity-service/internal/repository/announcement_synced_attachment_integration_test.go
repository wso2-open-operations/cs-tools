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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// An announcement's files can also reach Postgres through the sync, which mirrors
// ServiceNow's attachment records of every work item into work_item_attachment
// (migration 0085). That table has no row-level security of its own, and
// work_item's policy only asks for project membership, so it knows nothing about an
// announcement a contact is not cleared for (a security announcement and a General
// Access contact, migration 100010). The two reads that merge those rows into a case's
// attachment list and activity feed therefore have to hide them the way every other
// case-level read hides the announcement itself (announcementVisibilityLeakGuard).
//
// Skipped without ANNOUNCEMENT_VISIBILITY_TEST_DSN, like the other announcement
// integration tests; run as a non-superuser so row-level security is in force.
func TestAnnouncementSyncedAttachmentVisibilityIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	seedAnnouncementVisibilityFixtures(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := context.Background()

	const (
		syncedGeneral  = "f1111111-0000-4000-8000-000000000001"
		syncedSecurity = "f1111111-0000-4000-8000-000000000002"
	)
	seed := func(id, workItemID, name string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `INSERT INTO work_item_attachment
				(id, created_on, updated_on, created_by, updated_by, name, content_type, work_item_id, size_bytes, state)
			VALUES ($1, NOW(), NOW(), 'sync', 'sync', $3, 'application/pdf', $2, 100, 'AVAILABLE')
			ON CONFLICT (id) DO NOTHING`, id, workItemID, name); err != nil {
			t.Fatalf("seed synced attachment: %v", err)
		}
	}
	seed(syncedGeneral, avGeneralID, "synced-general.pdf")
	seed(syncedSecurity, avSecurityID, "synced-security.pdf")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM work_item_attachment WHERE id IN ($1, $2)`, syncedGeneral, syncedSecurity)
	})

	member := func(email string) repository.SearchScope {
		return repository.SearchScope{ProjectIDs: []string{avProjectID}, ViewerEmail: email}
	}
	listed := func(scope repository.SearchScope, id string) int {
		t.Helper()
		_, total, err := repo.SearchCaseAttachments(repository.WithCallerIdentity(context.Background(), scope), id, domain.Pagination{Limit: 50})
		if err != nil {
			t.Fatalf("SearchCaseAttachments(%s): %v", id, err)
		}
		return total
	}
	inFeed := func(scope repository.SearchScope, id string) int {
		t.Helper()
		activity, _, err := repo.SearchCaseActivities(repository.WithCallerIdentity(context.Background(), scope),
			domain.SearchCaseActivitiesRequest{CaseID: id, Pagination: domain.Pagination{Limit: 50}})
		var notFound *apierror.NotFoundError
		if errors.As(err, &notFound) {
			// A case the caller may not see is not found, which shows them nothing.
			return 0
		}
		if err != nil {
			t.Fatalf("SearchCaseActivities(%s): %v", id, err)
		}
		n := 0
		for _, a := range activity {
			if a.Type == domain.ActivityTypeAttachment {
				n++
			}
		}
		return n
	}

	for _, tc := range []struct {
		name         string
		scope        repository.SearchScope
		wantGeneral  int
		wantSecurity int
	}{
		{"General Access sees the general announcement's synced file only", member("av-general@test.local"), 1, 0},
		{"Lead User Group sees the general announcement's synced file only", member("av-lead@test.local"), 1, 0},
		{"Security Only sees both", member("av-secure-only@test.local"), 1, 1},
		{"Full Access sees both", member("av-full-access@test.local"), 1, 1},
		{"an unknown email sees nothing (fails closed)", member("nobody@nowhere.local"), 0, 0},
		{"an internal caller sees both", repository.SearchScope{Unrestricted: true}, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := listed(tc.scope, avGeneralID); got != tc.wantGeneral {
				t.Errorf("attachment list, general announcement = %d, want %d", got, tc.wantGeneral)
			}
			if got := listed(tc.scope, avSecurityID); got != tc.wantSecurity {
				t.Errorf("attachment list, security announcement = %d, want %d", got, tc.wantSecurity)
			}
			if got := inFeed(tc.scope, avGeneralID); got != tc.wantGeneral {
				t.Errorf("activity feed, general announcement = %d attachment entries, want %d", got, tc.wantGeneral)
			}
			if got := inFeed(tc.scope, avSecurityID); got != tc.wantSecurity {
				t.Errorf("activity feed, security announcement = %d attachment entries, want %d", got, tc.wantSecurity)
			}
		})
	}
}
