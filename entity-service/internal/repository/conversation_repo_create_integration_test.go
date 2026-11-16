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

// Regression test for migration 0190 and ConversationRepository.
// CreateConversation: a registered project contact can start a conversation
// under their own (non-internal) identity, and a stranger is refused. Runs
// against a real Postgres with 0190 applied. Skipped without
// CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ConversationCreateIntegration

package repository_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	ccProjectID = "74000000-0000-0000-0000-000000000001"
	ccAccountID = "74111111-0000-0000-0000-000000000001"
	ccContactID = "74222222-0000-0000-0000-000000000001"
	ccMember    = "cc-member@test.local"
	ccStranger  = "cc-stranger@test.local"
)

// seedConversationCreateFixture creates one project with one REGISTERED
// contact, ccMember.
func seedConversationCreateFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE project_id = $1`, ccProjectID)
		_, _ = pool.Exec(ctx, `DELETE FROM project_contact WHERE project_id = $1`, ccProjectID)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, ccProjectID)
		_, _ = pool.Exec(ctx, `DELETE FROM account_contact WHERE id = $1`, ccContactID)
		_, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, ccAccountID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}
	now := time.Now().UTC()

	mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
		VALUES ($1, $2, $2, 'test', 'test', 'CC Test Account', 'CC-ACC-1', 'sf-cc-acc-1')`, ccAccountID, now)
	mustExec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'CC Test Contact', $3)`, ccContactID, now, ccAccountID)
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, account_id)
		VALUES ($1, $2, $2, 'test', 'test', 'CCTEST', 'sf-cctest', $3)`, ccProjectID, now, ccAccountID)
	mustExec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		VALUES (gen_random_uuid(), $1, $1, 'test', 'test', $2, $3, $4, 'REGISTERED')`,
		now, ccMember, ccContactID, ccProjectID)
}

func ccInput(createdBy string) repository.CreateConversationInput {
	msg := "My API gateway returns 502 after upgrading to 4.3.0"
	return repository.CreateConversationInput{
		ProjectID:      ccProjectID,
		Subject:        msg,
		InitialMessage: msg,
		CreatedBy:      createdBy,
		State:          domain.ConversationStateActive,
	}
}

func TestConversationCreateIntegration_MemberCreatesAndReadsBack(t *testing.T) {
	pool := caseStatsPool(t)
	seedConversationCreateFixture(t, pool)
	repo := repository.NewConversationRepository(repository.NewScoped(pool))
	ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: ccMember})

	created, err := repo.CreateConversation(ctx, ccInput(ccMember))
	if err != nil {
		t.Fatalf("CreateConversation as a registered project member: %v", err)
	}
	if !strings.HasPrefix(created.Number, "CHAT") {
		t.Errorf("number = %q, want a next_work_item_number('CONVERSATION') value (CHAT...)", created.Number)
	}

	got, err := repo.GetConversation(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetConversation(%s): %v", created.ID, err)
	}
	if got.State == nil || *got.State != string(domain.ConversationStateActive) {
		t.Errorf("state = %v, want ACTIVE", got.State)
	}
	if got.InitialMessage == nil || *got.InitialMessage != ccInput(ccMember).InitialMessage {
		t.Errorf("initialMessage = %v, want the first message", got.InitialMessage)
	}

	// The conversation must accept comments, which is how its history is
	// persisted.
	comments := repository.NewCommentRepository(repository.NewScoped(pool))
	if _, err := comments.CreateComment(ctx, created.ID, domain.ReferenceTypeConversation, "COMMENT", "hello", ccMember); err != nil {
		t.Fatalf("CreateComment on the new conversation: %v", err)
	}
}

func TestConversationCreateIntegration_SuppliedIdentityIsKept(t *testing.T) {
	pool := caseStatsPool(t)
	seedConversationCreateFixture(t, pool)
	repo := repository.NewConversationRepository(repository.NewScoped(pool))
	ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: ccMember})

	in := ccInput(ccMember)
	in.ID = "74333333-0000-0000-0000-000000000001"
	in.Number = "CHAT-CC-TEST-1"
	created, err := repo.CreateConversation(ctx, in)
	if err != nil {
		t.Fatalf("CreateConversation with a supplied id/number: %v", err)
	}
	if created.ID != in.ID || created.Number != in.Number {
		t.Errorf("id/number = %q/%q, want %q/%q", created.ID, created.Number, in.ID, in.Number)
	}
}

func TestConversationCreateIntegration_StrangerIsForbidden(t *testing.T) {
	pool := caseStatsPool(t)
	seedConversationCreateFixture(t, pool)
	repo := repository.NewConversationRepository(repository.NewScoped(pool))
	ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: ccStranger})

	_, err := repo.CreateConversation(ctx, ccInput(ccStranger))
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("CreateConversation as a non-member: err = %v, want a ForbiddenError", err)
	}
}
