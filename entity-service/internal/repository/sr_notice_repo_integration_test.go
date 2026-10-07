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

// Exercises SRNoticeRepository against a real Postgres with migrations
// applied: the SR read (SRE team through the account, tags), the assignment,
// and the acknowledgement's comment + state write. Skipped without
// SR_NOTICE_TEST_DSN.
//
//	SR_NOTICE_TEST_DSN=postgres://... go test ./internal/repository/ -run SRNotice

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	srnAccountID = "e1000000-0000-0000-0000-000000000001"
	srnProjectID = "e2000000-0000-0000-0000-000000000001"
	srnTeamID    = "e3000000-0000-0000-0000-000000000001"
	srnSRID      = "e4000000-0000-0000-0000-000000000001"
	srnCaseID    = "e4000000-0000-0000-0000-000000000002"
	srnTagID     = "e5000000-0000-0000-0000-000000000001"
)

func seedSRNoticeFixture(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("SR_NOTICE_TEST_DSN")
	if dsn == "" {
		t.Skip("SR_NOTICE_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM comment WHERE work_item_id IN ($1, $2)`, srnSRID, srnCaseID)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item_tag WHERE work_item_id = $1`, srnSRID)
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id IN ($1, $2)`, srnSRID, srnCaseID)
		_, _ = pool.Exec(ctx, `DELETE FROM tag WHERE id = $1`, srnTagID)
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, srnProjectID)
		_, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, srnAccountID)
		_, _ = pool.Exec(ctx, `DELETE FROM "group" WHERE id = $1`, srnTeamID)
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().UTC()
	steps := []struct {
		scoped bool
		sql    string
		args   []any
	}{
		{false, `INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, $2, $2, 'test', 'test', 'MS/PC SRE Group')`, []any{srnTeamID, now}},
		{false, `INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sre_team_id) VALUES ($1, $2, $2, 'test', 'test', 'SRN Account', 'SRN-ACC-1', $3)`, []any{srnAccountID, now, srnTeamID}},
		{false, `INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, account_id) VALUES ($1, $2, $2, 'test', 'test', 'SRNTEST', 'SRN Project', $3)`, []any{srnProjectID, now, srnAccountID}},
		{false, `INSERT INTO tag (id, created_on, updated_on, name) VALUES ($1, $2, $2, 'DevOps-SM')`, []any{srnTagID, now}},
		// The SR leaves work_item.account_id unset, so the read has to reach
		// the SRE team through the project's account.
		{true, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, description, type, project_id)
			VALUES ($1, $2, $2, 'jane@acme.test', 'jane@acme.test', 'SRN-SR-1', 'SRNTEST-1', 'Rotate the key', 'please', 'SERVICE_REQUEST', $3)`, []any{srnSRID, now, srnProjectID}},
		{true, `INSERT INTO service_request (id, state) VALUES ($1, 'OPEN')`, []any{srnSRID}},
		{true, `INSERT INTO work_item_tag (id, created_on, updated_on, work_item_id, tag_id) VALUES (gen_random_uuid(), $2, $2, $1, $3)`, []any{srnSRID, now, srnTagID}},
		{true, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
			VALUES ($1, $2, $2, 'test', 'test', 'SRN-CS-1', 'SRNTEST-2', 'a plain case', 'CASE', $3)`, []any{srnCaseID, now, srnProjectID}},
	}
	for _, s := range steps {
		var err error
		if s.scoped {
			_, err = scoped.Exec(ctx, s.sql, s.args...)
		} else {
			_, err = pool.Exec(ctx, s.sql, s.args...)
		}
		if err != nil {
			t.Fatalf("seed (%.70s): %v", s.sql, err)
		}
	}
	return pool
}

func TestSRNoticeRepo_EndToEnd(t *testing.T) {
	pool := seedSRNoticeFixture(t)
	scoped := repository.NewScoped(pool)
	repo := repository.NewSRNoticeRepository(scoped)
	ctx := repository.WithSystemIdentity(context.Background())

	if _, ok, err := repo.GetServiceRequest(ctx, srnCaseID); err != nil || ok {
		t.Fatalf("plain case: ok=%v err=%v, want not an SR", ok, err)
	}

	sr, ok, err := repo.GetServiceRequest(ctx, srnSRID)
	if err != nil || !ok {
		t.Fatalf("GetServiceRequest: ok=%v err=%v", ok, err)
	}
	if sr.Number != "SRN-SR-1" || sr.WSO2CaseID != "SRNTEST-1" || sr.State != "OPEN" || sr.ProjectName != "SRN Project" ||
		sr.SRETeamID != srnTeamID || sr.SRETeamName != "MS/PC SRE Group" || sr.AssignmentGroupName != "" ||
		len(sr.Tags) != 1 || sr.Tags[0] != "DevOps-SM" {
		t.Fatalf("GetServiceRequest = %+v", sr)
	}

	if err := repo.AssignToGroup(ctx, srnSRID, srnTeamID, "system"); err != nil {
		t.Fatalf("AssignToGroup: %v", err)
	}
	if err := repo.AssignToGroup(ctx, srnCaseID, srnTeamID, "system"); err == nil {
		t.Errorf("AssignToGroup on a plain case succeeded, want an error")
	}
	commentID, err := repo.Acknowledge(ctx, srnSRID, "system", "received")
	if err != nil || commentID == "" {
		t.Fatalf("Acknowledge: id=%q err=%v", commentID, err)
	}

	sr, _, _ = repo.GetServiceRequest(ctx, srnSRID)
	if sr.AssignmentGroupName != "MS/PC SRE Group" || sr.State != "OPEN" {
		t.Errorf("after assign/ack: group=%q state=%q", sr.AssignmentGroupName, sr.State)
	}
	var typ, content, by string
	if err := scoped.QueryRow(ctx, `SELECT type::text, content, created_by FROM comment WHERE id = $1`, commentID).Scan(&typ, &content, &by); err != nil {
		t.Fatalf("read comment: %v", err)
	}
	if typ != "COMMENT" || content != "received" || by != "system" {
		t.Errorf("comment = %s / %q / %s", typ, content, by)
	}
}
