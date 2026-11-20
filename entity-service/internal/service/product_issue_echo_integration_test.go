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

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// A portal comment's echo (the issue_comment created delivery for the comment the
// worker posted) is adopted by the portal comment, not mirrored as a second one.

func pwhCommentBody(t *testing.T, d pwhDelivery) string {
	t.Helper()
	var p struct {
		Comment struct {
			Body string `json:"body"`
		} `json:"comment"`
	}
	if err := json.Unmarshal(d.Raw, &p); err != nil {
		t.Fatal(err)
	}
	return p.Comment.Body
}

func TestProductIssueWebhookIntegration_PortalCommentEchoIsAdopted(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail bool
	}{{"pending command", false}, {"failed command", true}} {
		t.Run(tc.name, func(t *testing.T) {
			e := pwhSeed(t)
			repo := repository.NewGitHubCommandRepository(repository.NewScoped(e.pool))
			echo := pwhFixture(t, "28") // issue_comment created on wi2, comment id 6939407244
			res, err := repo.InsertPortalComment(e.ctx, repository.PortalCommentInput{
				WorkItemID: e.wi2, Content: "from the portal", GitHubBody: pwhCommentBody(t, echo), RequestedBy: "a@wso2.com"})
			if err != nil {
				t.Fatal(err)
			}
			if tc.fail {
				e.pool.Exec(context.Background(), `UPDATE github_command SET status = 'FAILED', last_error = 'x' WHERE id = $1::uuid`, res.Command.ID)
			}
			// No bot logins configured: the guard that would drop it is absent.
			svc := e.front(e.old)
			e.ok(svc, echo)
			if n := e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, e.wi2); n != 1 {
				t.Fatalf("comments = %d, want the portal comment only", n)
			}
			var origin, status string
			var gid *int64
			var login *string
			if err := e.pool.QueryRow(e.ctx, `SELECT r.origin, r.github_comment_id, r.author_login, g.status
				FROM comment_github_ref r JOIN github_command g ON g.comment_id = r.comment_id WHERE r.comment_id = $1::uuid`, res.Comment.ID).Scan(&origin, &gid, &login, &status); err != nil {
				t.Fatal(err)
			}
			if origin != "PORTAL" || gid == nil || *gid != 6939407244 || login == nil || status != "DELIVERED" {
				t.Errorf("ref = %s %v %v command %s", origin, gid, login, status)
			}
			// The same echo again (another delivery) changes nothing.
			e.ok(svc, echo.mutate(t, "pwh-echo-again", func(map[string]any) {}))
			if n := e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, e.wi2); n != 1 {
				t.Errorf("comments after the repeat = %d", n)
			}
			// The worker then has nothing to do for it: it is delivered, not due.
			cmds, _ := repo.ClaimDue(e.ctx, 10)
			for _, c := range cmds {
				if c.ID == res.Command.ID {
					t.Error("an adopted command was claimed again")
				}
			}
		})
	}
}

func TestProductIssueWebhookIntegration_DifferentCommentIsStillMirrored(t *testing.T) {
	e := pwhSeed(t)
	repo := repository.NewGitHubCommandRepository(repository.NewScoped(e.pool))
	res, err := repo.InsertPortalComment(e.ctx, repository.PortalCommentInput{
		WorkItemID: e.wi2, Content: "other", GitHubBody: "a different body", RequestedBy: "a@wso2.com"})
	if err != nil {
		t.Fatal(err)
	}
	e.ok(e.front(e.old), pwhFixture(t, "28"))
	if n := e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, e.wi2); n != 2 {
		t.Errorf("comments = %d, want the portal one and the mirrored one", n)
	}
	var gid *int64
	_ = e.pool.QueryRow(e.ctx, `SELECT github_comment_id FROM comment_github_ref WHERE comment_id = $1::uuid`, res.Comment.ID).Scan(&gid)
	if gid != nil {
		t.Error("a portal comment adopted a comment with another body")
	}
}

// Backfill and reconciliation import comments through the same InsertGitHubComment.
func TestProductIssueEcho_ImportsAdoptToo(t *testing.T) {
	e := pwhSeed(t)
	repo := repository.NewGitHubCommandRepository(repository.NewScoped(e.pool))
	res, err := repo.InsertPortalComment(e.ctx, repository.PortalCommentInput{
		WorkItemID: e.wi1, Content: "imported echo", GitHubBody: "**A** (CS) wrote:\r\n\r\nimported echo", RequestedBy: "a@wso2.com"})
	if err != nil {
		t.Fatal(err)
	}
	scoped := repository.NewScoped(e.pool)
	// Line endings do not matter: GitHub may return CRLF.
	err = scoped.InTx(e.ctx, func(tx pgx.Tx) error {
		id, inserted, err := repository.InsertGitHubComment(e.ctx, tx, repository.GitHubCommentInput{
			WorkItemID: e.wi1, GitHubCommentID: 424242, Author: "portal-bot", Body: "**A** (CS) wrote:\n\nimported echo", CreatedOn: time.Now()})
		if err != nil || inserted || id != res.Comment.ID {
			t.Errorf("import: id=%s inserted=%v err=%v", id, inserted, err)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, e.wi1); n != 1 {
		t.Errorf("comments = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM github_command WHERE id = $1::uuid AND status = 'DELIVERED'`, res.Command.ID); n != 1 {
		t.Error("the command was not marked delivered")
	}
}

// Two writers racing on one GitHub comment id (a webhook and a reconciliation):
// the loser's insert hits the unique index. That is "already recorded", not an error,
// and its transaction stays usable.
func TestProductIssueEcho_RacingMirrorsOfOneCommentIdDoNotFail(t *testing.T) {
	e := pwhSeed(t)
	in := repository.GitHubCommentInput{WorkItemID: e.wi1, GitHubCommentID: 515151, Author: "someone", Body: "racing", CreatedOn: time.Now()}

	txA, err := e.pool.Begin(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer txA.Rollback(e.ctx) //nolint:errcheck
	idA, insertedA, err := repository.InsertGitHubComment(e.ctx, txA, in)
	if err != nil || !insertedA {
		t.Fatalf("first: %v %v", insertedA, err)
	}

	type result struct {
		id       string
		inserted bool
		err      error
		usable   error
	}
	done := make(chan result, 1)
	go func() {
		txB, err := e.pool.Begin(e.ctx)
		if err != nil {
			done <- result{err: err}
			return
		}
		defer txB.Rollback(e.ctx) //nolint:errcheck
		// Blocks on the unique index until A commits, then conflicts.
		id, inserted, err := repository.InsertGitHubComment(e.ctx, txB, in)
		var one int
		usable := txB.QueryRow(e.ctx, `SELECT 1`).Scan(&one) // the transaction must still work
		if err == nil {
			err = txB.Commit(e.ctx)
		}
		done <- result{id, inserted, err, usable}
	}()
	time.Sleep(300 * time.Millisecond)
	if err := txA.Commit(e.ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		if r.err != nil || r.usable != nil || r.inserted || r.id != idA {
			t.Fatalf("loser: %+v (winner comment %s)", r, idA)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the racing insert never returned")
	}
	if n := e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, e.wi1); n != 1 {
		t.Errorf("comments = %d, want 1 (the loser's comment row must not survive)", n)
	}
}
