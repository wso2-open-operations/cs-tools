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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The Support page's "Resolved via Chat (Last 30d)" card and the list it opens
// must both mean "Resolved, and updated in the past 30 days" (a conversation has
// no resolved-on column). The card reads ResolvedConversationsPastThirtyDays, the
// list is SearchConversations with States [resolved] and a StartUpdatedDate; this
// pins that the two agree, and that the bounds behave. Skipped without
// CASE_STATS_TEST_DSN.
const convWindowProject = "66666666-6666-6666-6666-666666666666"

func TestConversationResolvedWindowIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(sys, `DELETE FROM work_item WHERE created_by IN ('conv-window-a@wso2.com', 'conv-window-b@wso2.com')`)
		_, _ = pool.Exec(sys, `DELETE FROM project WHERE id = $1`, convWindowProject)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(sys, sql, args...); err != nil {
			t.Fatalf("seed (%s): %v", sql, err)
		}
	}
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id)
	          VALUES ($1, now(), now(), 'seed', 'seed', 'CONVWIN', 'sf-convwin')`, convWindowProject)

	// state, creator, days since the last update
	rows := []struct {
		state, creator string
		updatedDaysAgo int
	}{
		{"RESOLVED", "conv-window-a@wso2.com", 5},
		{"RESOLVED", "conv-window-a@wso2.com", 45}, // outside the window
		{"RESOLVED", "conv-window-b@wso2.com", 10},
		{"ACTIVE", "conv-window-a@wso2.com", 1}, // recent, but not resolved
	}
	for i, r := range rows {
		id := plcID(950 + i)
		mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, project_id)
		          VALUES ($1, now() - INTERVAL '60 days', now() - make_interval(days => $2), $3, $3, $4, 'conv window', 'CONVERSATION'::work_item_type_enum, $5)`,
			id, r.updatedDaysAgo, r.creator, "CW-"+id[24:], convWindowProject)
		mustExec(`INSERT INTO conversation (id, state) VALUES ($1, $2::conversation_state_enum)`, id, r.state)
	}

	stats := repository.NewProjectStatsRepository(scoped)
	convs := repository.NewConversationRepository(scoped)

	search := func(t *testing.T, f domain.SearchConversationsFilters) int {
		t.Helper()
		f.ProjectIDs = []string{convWindowProject}
		_, total, err := convs.SearchConversations(sys, domain.SearchConversationsRequest{
			Filters:    f,
			SortBy:     domain.ConversationSort{Field: domain.ConversationSortFieldCreatedOn, Order: domain.ConversationSortOrderDesc},
			Pagination: domain.Pagination{Limit: 20},
		}, "")
		if err != nil {
			t.Fatalf("SearchConversations: %v", err)
		}
		return total
	}

	now := time.Now()
	start := now.Add(-30 * 24 * time.Hour)
	resolved := []domain.ConversationState{domain.ConversationStateResolved}

	t.Run("the card and its list agree", func(t *testing.T) {
		card, err := stats.ResolvedConversationsPastThirtyDays(sys, convWindowProject, "")
		if err != nil {
			t.Fatalf("ResolvedConversationsPastThirtyDays: %v", err)
		}
		list := search(t, domain.SearchConversationsFilters{States: resolved, StartUpdatedDate: &start, EndUpdatedDate: &now})
		if card != 2 || list != 2 {
			t.Errorf("card = %d, list = %d, want both 2 (the 45-day-old one is outside the window)", card, list)
		}
	})

	t.Run("without the window every resolved conversation is listed", func(t *testing.T) {
		if got := search(t, domain.SearchConversationsFilters{States: resolved}); got != 3 {
			t.Errorf("resolved list without a window = %d, want 3", got)
		}
	})

	t.Run("a bound can be given on either side", func(t *testing.T) {
		weekAgo := now.Add(-7 * 24 * time.Hour)
		if got := search(t, domain.SearchConversationsFilters{States: resolved, StartUpdatedDate: &start, EndUpdatedDate: &weekAgo}); got != 1 {
			t.Errorf("resolved, updated between 30 and 7 days ago = %d, want 1 (the 10-day-old one)", got)
		}
		if got := search(t, domain.SearchConversationsFilters{States: resolved, EndUpdatedDate: &weekAgo}); got != 2 {
			t.Errorf("resolved, updated more than 7 days ago = %d, want 2", got)
		}
	})

	t.Run("the card honours the creator filter like the state counts do", func(t *testing.T) {
		got, err := stats.ResolvedConversationsPastThirtyDays(sys, convWindowProject, "CONV-WINDOW-A@wso2.com")
		if err != nil {
			t.Fatal(err)
		}
		if got != 1 {
			t.Errorf("resolved in the window created by A (case-insensitive) = %d, want 1", got)
		}
	})
}
