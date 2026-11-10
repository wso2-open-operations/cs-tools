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

// The skill management revamp's SQL against a live PostgreSQL (migrations
// 0282-0286): categories, skill requests with their approval and the
// requester's auto-added skill, and the directory's ALL / ANY criteria.
// Uses skill_repo_integration_test.go's fixtures; skipped without
// ENTITY_TEST_DATABASE_URL.

package repository_test

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// skDefaultScaleA makes fixture scale A the default for the test and puts the
// database's own default back afterwards.
func skDefaultScaleA(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	var prev *string
	_ = pool.QueryRow(ctx, `SELECT id::text FROM skill_level_type WHERE is_default`).Scan(&prev)
	skExec(t, pool, `UPDATE skill_level_type SET is_default = FALSE WHERE is_default`)
	skExec(t, pool, `UPDATE skill_level_type SET is_default = TRUE WHERE id = $1`, skScaleA)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `UPDATE skill_level_type SET is_default = FALSE WHERE id = $1`, skScaleA)
		if prev != nil && *prev != skScaleA {
			_, _ = pool.Exec(ctx, `UPDATE skill_level_type SET is_default = TRUE WHERE id = $1`, *prev)
		}
	})
}

func TestSkillIntegration_RequestCreateApproveAddsToProfile(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	skDefaultScaleA(t, pool)
	ctx := context.Background()

	// An active skill's name cannot be requested.
	_, outcome, err := repo.CreateSkillRequest(ctx, skJane, "sktest go", domain.CreateSkillRequestRequest{Name: "sktest Go", LevelID: skLvlA50}, skJaneEmail)
	if err != nil || outcome != domain.SkillOutcomeSkillExists {
		t.Fatalf("request for an existing skill = %v, %v", outcome, err)
	}
	req, outcome, err := repo.CreateSkillRequest(ctx, skJane, "sktest ballerina",
		domain.CreateSkillRequestRequest{Name: "sktest Ballerina", CategoryID: ptr(skCatLeaf), LevelID: skLvlA50}, skJaneEmail)
	if err != nil || outcome != domain.SkillOutcomeDone || req.Status != domain.SkillRequestPending {
		t.Fatalf("create request = %+v, %v, %v", req, outcome, err)
	}
	// A second pending request for the same name is pointed at the first.
	dup, outcome, err := repo.CreateSkillRequest(ctx, skLead, "sktest ballerina",
		domain.CreateSkillRequestRequest{Name: "SKTEST  Ballerina", LevelID: skLvlA20}, skLeadEmail)
	if err != nil || outcome != domain.SkillOutcomePendingExists || dup == nil || dup.ID != req.ID {
		t.Fatalf("duplicate request = %+v, %v, %v", dup, outcome, err)
	}
	chk, err := repo.CheckSkillRequest(ctx, "sktest ballerina")
	if err != nil || chk.PendingRequest == nil || chk.PendingRequest.ID != req.ID {
		t.Fatalf("check = %+v, %v", chk, err)
	}
	if n, _ := repo.CountPendingSkillRequests(ctx); n < 1 {
		t.Fatalf("pending count = %d", n)
	}

	approved, outcome, err := repo.ApproveSkillRequest(ctx, domain.SkillApproval{
		RequestID: req.ID, Mode: domain.SkillApproveCreate, Name: "sktest Ballerina", CategoryID: ptr(skCatLeaf),
		ReviewerID: skLead, Actor: skLeadEmail, Comment: ptr("welcome"),
	})
	if err != nil || outcome != domain.SkillOutcomeDone {
		t.Fatalf("approve = %v, %v", outcome, err)
	}
	if approved.Status != domain.SkillRequestApproved || approved.Resolution == nil || *approved.Resolution != domain.SkillResolutionCreated ||
		approved.ResolvedSkill == nil || approved.AutoAddNote == nil || *approved.AutoAddNote != domain.SkillAutoAddAdded ||
		approved.AutoAddedLevel == nil || approved.AutoAddedLevel.ID != skLvlA50 || approved.ReviewedBy == nil || approved.ReviewedBy.ID != skLead {
		t.Fatalf("approved request = %+v", approved)
	}
	newSkill := approved.ResolvedSkill.ID
	if n := skCount(t, pool, `SELECT count(*) FROM skill WHERE id = $1 AND active AND level_type_id = $2`, newSkill, skScaleA); n != 1 {
		t.Fatalf("created skill rows = %d", n)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill_category_link WHERE skill_id = $1 AND category_id = $2`, newSkill, skCatLeaf); n != 1 {
		t.Fatalf("created skill category links = %d", n)
	}
	// On the requester's profile, with history naming the request and the reviewer.
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2 AND active AND skill_level_id = $3`, skJane, newSkill, skLvlA50); n != 1 {
		t.Fatalf("auto-added rows = %d", n)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill_history WHERE user_id = $1 AND skill_id = $2 AND action = 'ADDED' AND skill_request_id = $3 AND created_by = $4 AND skill_name_snapshot = 'sktest Ballerina'`,
		skJane, newSkill, req.ID, skLeadEmail); n != 1 {
		t.Fatalf("auto-add history rows = %d", n)
	}
	// History shows the name the skill had then, not after a later rename.
	skExec(t, pool, `UPDATE skill SET name = 'sktest Ballerina Lang' WHERE id = $1`, newSkill)
	hist, _, err := repo.ListUserSkillHistory(ctx, skJane, newSkill, 10, 0)
	if err != nil || len(hist) != 1 || hist[0].SkillName != "sktest Ballerina" {
		t.Fatalf("history after rename = %+v, %v", hist, err)
	}
	skExec(t, pool, `UPDATE skill SET name = 'sktest Ballerina' WHERE id = $1`, newSkill)
	// Deciding it again is refused and changes nothing.
	again, outcome, err := repo.RejectSkillRequest(ctx, req.ID, skLead, "late", skLeadEmail)
	if err != nil || outcome != domain.SkillOutcomeRequestNotPending || again.Status != domain.SkillRequestApproved {
		t.Fatalf("reject after approve = %+v, %v, %v", again, outcome, err)
	}
	// The name is now taken, so the same request cannot be made again.
	_, outcome, _ = repo.CreateSkillRequest(ctx, skLead, "sktest ballerina", domain.CreateSkillRequestRequest{Name: "sktest Ballerina", LevelID: skLvlA20}, skLeadEmail)
	if outcome != domain.SkillOutcomeSkillExists {
		t.Fatalf("request after approval = %v", outcome)
	}
}

func TestSkillIntegration_ApproveMapsTheLevelAndHandlesExistingAndReactivate(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	skDefaultScaleA(t, pool)
	ctx := context.Background()

	// Jane already holds Go: approving "existing" Go leaves her level alone.
	if _, err := repo.AddUserSkill(ctx, skJane, skGo, skLvlA100, skJaneEmail); err != nil {
		t.Fatal(err)
	}
	r1, _, _ := repo.CreateSkillRequest(ctx, skJane, "sktest golang", domain.CreateSkillRequestRequest{Name: "sktest Golang", LevelID: skLvlA20}, skJaneEmail)
	got, outcome, err := repo.ApproveSkillRequest(ctx, domain.SkillApproval{RequestID: r1.ID, Mode: domain.SkillApproveExisting, SkillID: skGo, ReviewerID: skLead, Actor: skLeadEmail})
	if err != nil || outcome != domain.SkillOutcomeDone || *got.AutoAddNote != domain.SkillAutoAddAlreadyHeld || got.AutoAddedLevel != nil {
		t.Fatalf("existing, already held = %+v, %v, %v", got, outcome, err)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM user_skill WHERE user_id = $1 AND skill_id = $2 AND skill_level_id = $3`, skJane, skGo, skLvlA100); n != 1 {
		t.Fatal("an already-held level was changed")
	}

	// Reactivating Old (inactive, scale A) refuses "existing" and accepts "reactivate".
	r2, _, _ := repo.CreateSkillRequest(ctx, skLead, "sktest old", domain.CreateSkillRequestRequest{Name: "sktest Old", LevelID: skLvlA50}, skLeadEmail)
	_, outcome, _ = repo.ApproveSkillRequest(ctx, domain.SkillApproval{RequestID: r2.ID, Mode: domain.SkillApproveExisting, SkillID: skOld, ReviewerID: skLead, Actor: skLeadEmail})
	if outcome != domain.SkillOutcomeSkillNotActive {
		t.Fatalf("existing on an inactive skill = %v", outcome)
	}
	got, outcome, err = repo.ApproveSkillRequest(ctx, domain.SkillApproval{RequestID: r2.ID, Mode: domain.SkillApproveReactivate, SkillID: skOld, ReviewerID: skJane, Actor: skJaneEmail})
	if err != nil || outcome != domain.SkillOutcomeDone || *got.Resolution != domain.SkillResolutionReactivated || *got.AutoAddNote != domain.SkillAutoAddAdded {
		t.Fatalf("reactivate = %+v, %v, %v", got, outcome, err)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill WHERE id = $1 AND active`, skOld); n != 1 {
		t.Fatal("the skill was not reactivated")
	}

	// A request on scale A approved as Rust (scale B): the level is mapped onto
	// scale B (the closest level at or below the asked value).
	r3, _, _ := repo.CreateSkillRequest(ctx, skLead, "sktest rustlang", domain.CreateSkillRequestRequest{Name: "sktest Rustlang", LevelID: skLvlA50}, skLeadEmail)
	got, outcome, err = repo.ApproveSkillRequest(ctx, domain.SkillApproval{RequestID: r3.ID, Mode: domain.SkillApproveExisting, SkillID: skRust, ReviewerID: skJane, Actor: skJaneEmail})
	if err != nil || outcome != domain.SkillOutcomeDone || *got.AutoAddNote != domain.SkillAutoAddLevelAdjusted || got.AutoAddedLevel.ID != skLvlB20 {
		t.Fatalf("mapped level = %+v, %v, %v", got, outcome, err)
	}
	// A reviewer's level of another scale is refused and nothing is approved.
	r4, _, _ := repo.CreateSkillRequest(ctx, skJane, "sktest x", domain.CreateSkillRequestRequest{Name: "sktest X", LevelID: skLvlA50}, skJaneEmail)
	_, outcome, _ = repo.ApproveSkillRequest(ctx, domain.SkillApproval{RequestID: r4.ID, Mode: domain.SkillApproveCreate, Name: "sktest X", CategoryID: ptr(skCatLeaf), LevelID: ptr(skLvlB20), ReviewerID: skLead, Actor: skLeadEmail})
	if outcome != domain.SkillOutcomeLevelNotInScale {
		t.Fatalf("off-scale override = %v", outcome)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill WHERE name = 'sktest X'`); n != 0 {
		t.Fatal("a refused approval left a skill behind")
	}
	if r, _ := repo.GetSkillRequest(ctx, r4.ID); r.Status != domain.SkillRequestPending {
		t.Fatalf("a refused approval changed the request: %s", r.Status)
	}
	// Withdraw: only the requester's own.
	if _, outcome, _ := repo.WithdrawSkillRequest(ctx, r4.ID, skLead, skLeadEmail); outcome != domain.SkillOutcomeNotFound {
		t.Fatalf("withdraw someone else's = %v", outcome)
	}
	if w, outcome, _ := repo.WithdrawSkillRequest(ctx, r4.ID, skJane, skJaneEmail); outcome != domain.SkillOutcomeDone || w.Status != domain.SkillRequestWithdrawn {
		t.Fatalf("withdraw = %v", outcome)
	}
}

func TestSkillIntegration_ConcurrentDecisionsDecideOnce(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	skDefaultScaleA(t, pool)
	ctx := context.Background()
	r, _, _ := repo.CreateSkillRequest(ctx, skJane, "sktest race", domain.CreateSkillRequestRequest{Name: "sktest Race", CategoryID: ptr(skCatLeaf), LevelID: skLvlA20}, skJaneEmail)

	var wg sync.WaitGroup
	outcomes := make([]domain.SkillOutcome, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, outcomes[0], _ = repo.ApproveSkillRequest(ctx, domain.SkillApproval{RequestID: r.ID, Mode: domain.SkillApproveCreate, Name: "sktest Race", CategoryID: ptr(skCatLeaf), ReviewerID: skLead, Actor: skLeadEmail})
	}()
	go func() {
		defer wg.Done()
		_, outcomes[1], _ = repo.RejectSkillRequest(ctx, r.ID, skSreLead, "no", "sre@example.test")
	}()
	wg.Wait()
	done := 0
	for _, o := range outcomes {
		if o == domain.SkillOutcomeDone {
			done++
		} else if o != domain.SkillOutcomeRequestNotPending {
			t.Fatalf("unexpected outcome %v", o)
		}
	}
	if done != 1 {
		t.Fatalf("decided %d times", done)
	}
}

func TestSkillIntegration_CategoryAdmin(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	ctx := context.Background()

	// A child under a leaf with a skill filed directly under it is refused.
	_, outcome, err := repo.CreateCategory(ctx, domain.CreateSkillCategoryRequest{Name: "sktest under leaf", ParentID: ptr(skCatLeaf)}, skActor)
	if err != nil || outcome != domain.SkillOutcomeCategoryInvalid {
		t.Fatalf("child of a filed leaf = %v, %v", outcome, err)
	}
	child, outcome, err := repo.CreateCategory(ctx, domain.CreateSkillCategoryRequest{Name: "sktest child", ParentID: ptr(skCatOther)}, skActor)
	if err != nil || outcome != domain.SkillOutcomeDone || !child.IsLowestCategory || child.CategoryPath == nil || *child.CategoryPath != "sktest other / sktest child" {
		t.Fatalf("create child = %+v, %v, %v", child, outcome, err)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill_category WHERE id = $1 AND NOT is_lowest_category`, skCatOther); n != 1 {
		t.Fatal("the parent is still a leaf")
	}
	_, outcome, _ = repo.CreateCategory(ctx, domain.CreateSkillCategoryRequest{Name: "SKTEST Child ", ParentID: ptr(skCatOther)}, skActor)
	if outcome != domain.SkillOutcomeNameTaken {
		t.Fatalf("duplicate sibling = %v", outcome)
	}
	// A rename rewrites the paths under it.
	if _, outcome, err := repo.UpdateCategory(ctx, skCatOther, domain.UpdateSkillCategoryRequest{Name: ptr("sktest renamed")}, skActor); err != nil || outcome != domain.SkillOutcomeDone {
		t.Fatalf("rename = %v, %v", outcome, err)
	}
	if n := skCount(t, pool, `SELECT count(*) FROM skill_category WHERE id = $1 AND category_path = 'sktest renamed / sktest child'`, child.ID); n != 1 {
		t.Fatal("the child's path was not rewritten")
	}
	// Deactivation is refused while an active child or skill is under it.
	_, outcome, _ = repo.UpdateCategory(ctx, skCatOther, domain.UpdateSkillCategoryRequest{Active: ptr(false)}, skActor)
	if outcome != domain.SkillOutcomeCategoryInUse {
		t.Fatalf("deactivate with an active child = %v", outcome)
	}
	_, outcome, _ = repo.UpdateCategory(ctx, skCatLeaf, domain.UpdateSkillCategoryRequest{Active: ptr(false)}, skActor)
	if outcome != domain.SkillOutcomeCategoryInUse {
		t.Fatalf("deactivate with an active skill = %v", outcome)
	}
	if _, outcome, _ := repo.UpdateCategory(ctx, child.ID, domain.UpdateSkillCategoryRequest{Active: ptr(false)}, skActor); outcome != domain.SkillOutcomeDone {
		t.Fatalf("deactivate an empty leaf = %v", outcome)
	}
	// Hidden from the plain list, shown to a manager's.
	var plain, all bool
	for _, inc := range []bool{false, true} {
		cats, err := repo.ListCategories(ctx, inc)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cats {
			if c.ID == child.ID {
				if inc {
					all = true
				} else {
					plain = true
				}
			}
		}
	}
	if plain || !all {
		t.Fatalf("inactive category listed: plain %v, manager %v", plain, all)
	}
	if ok, _ := repo.CategoriesAreActiveLeaves(ctx, []string{child.ID}); ok {
		t.Fatal("an inactive category accepted as a filing place")
	}
	if ok, _ := repo.CategoriesAreActiveLeaves(ctx, []string{skCatTop}); ok {
		t.Fatal("a parent category accepted as a filing place")
	}
}

func TestSkillIntegration_DirectoryAllAndAny(t *testing.T) {
	repo, _ := newSkillIntegration(t)
	ctx := context.Background()
	// Jane: Go Expert (100), Rust Basic (20). Lead: Go Beginner (20).
	for _, w := range []struct{ user, skill, level string }{{skJane, skGo, skLvlA100}, {skJane, skRust, skLvlB20}, {skLead, skGo, skLvlA20}} {
		if res, err := repo.AddUserSkill(ctx, w.user, w.skill, w.level, skActor); err != nil || res.Outcome != domain.SkillOutcomeDone {
			t.Fatalf("seed %v: %v %v", w, res.Outcome, err)
		}
	}
	search := func(match string, crit ...domain.SkillCriterion) map[string]int {
		t.Helper()
		items, total, err := repo.SearchSkillDirectory(ctx, domain.SearchSkillDirectoryRequest{
			SearchQuery: "skills@example.test", Criteria: crit, Match: match, Pagination: domain.Pagination{Limit: 50},
		})
		if err != nil || total != len(items) {
			t.Fatalf("directory: %d/%d %v", len(items), total, err)
		}
		out := map[string]int{}
		for _, p := range items {
			out[p.UserID] = p.MatchedCount
			if len(p.Skills) == 0 {
				t.Fatalf("%s listed without skills", p.UserID)
			}
		}
		return out
	}
	// Go ≥ 50 AND Rust (any level): only Jane.
	got := search("ALL", domain.SkillCriterion{SkillID: skGo, MinLevelValue: 50}, domain.SkillCriterion{SkillID: skRust})
	if len(got) != 1 || got[skJane] != 2 {
		t.Fatalf("ALL = %v", got)
	}
	// Go ≥ 50 OR Rust: Jane only (the lead's Go is below 50).
	got = search("ANY", domain.SkillCriterion{SkillID: skGo, MinLevelValue: 50}, domain.SkillCriterion{SkillID: skRust})
	if len(got) != 1 {
		t.Fatalf("ANY high = %v", got)
	}
	// Go at any level: both.
	got = search("ALL", domain.SkillCriterion{SkillID: skGo})
	if len(got) != 2 || got[skLead] != 1 {
		t.Fatalf("Go any level = %v", got)
	}
	// No criteria: everyone with a skill.
	if got = search("ALL"); len(got) != 2 {
		t.Fatalf("no criteria = %v", got)
	}
	// Category filter: Go is filed under the leaf, under top: both match top.
	items, _, err := repo.SearchSkillDirectory(ctx, domain.SearchSkillDirectoryRequest{CategoryIDs: []string{skCatTop}, SearchQuery: "skills@example.test", Pagination: domain.Pagination{Limit: 50}})
	if err != nil || len(items) != 2 {
		t.Fatalf("category filter = %d, %v", len(items), err)
	}
	// A removed skill no longer matches.
	if _, err := repo.RemoveUserSkill(ctx, skLead, skGo, skActor); err != nil {
		t.Fatal(err)
	}
	if got = search("ALL", domain.SkillCriterion{SkillID: skGo}); len(got) != 1 {
		t.Fatalf("after removal = %v", got)
	}
	p, err := repo.GetUserSkillProfile(ctx, skJane)
	if err != nil || p == nil || len(p.Skills) != 2 {
		t.Fatalf("profile = %+v, %v", p, err)
	}
	if p, _ := repo.GetUserSkillProfile(ctx, skExt); p != nil {
		t.Fatal("an external user's profile was returned")
	}
}

func TestSkillIntegration_ScaleChangeIsAllowedOnceEveryHolderRemovedIt(t *testing.T) {
	repo, _ := newSkillIntegration(t)
	ctx := context.Background()
	if _, err := repo.AddUserSkill(ctx, skJane, skGo, skLvlA50, skActor); err != nil {
		t.Fatal(err)
	}
	res, err := repo.UpdateSkill(ctx, skGo, domain.UpdateSkillRequest{LevelTypeID: ptr(skScaleB)}, skActor)
	if err != nil || res.Outcome != domain.SkillOutcomeLevelsInUse {
		t.Fatalf("scale change while held = %v, %v", res.Outcome, err)
	}
	// Removing keeps the row (soft delete), but it no longer holds the scale.
	if _, err := repo.RemoveUserSkill(ctx, skJane, skGo, skActor); err != nil {
		t.Fatal(err)
	}
	res, err = repo.UpdateSkill(ctx, skGo, domain.UpdateSkillRequest{LevelTypeID: ptr(skScaleB)}, skActor)
	if err != nil || res.Outcome != domain.SkillOutcomeDone {
		t.Fatalf("scale change after removal = %v, %v", res.Outcome, err)
	}
	// Adding it back takes a level of the new scale.
	add, err := repo.AddUserSkill(ctx, skJane, skGo, skLvlB20, skActor)
	if err != nil || add.Outcome != domain.SkillOutcomeDone || add.Row.Level.ID != skLvlB20 {
		t.Fatalf("re-add on the new scale = %+v, %v", add, err)
	}
}

func TestSkillIntegration_TheOnlyScaleIsTheDefaultWhenNoneIsMarked(t *testing.T) {
	repo, pool := newSkillIntegration(t)
	ctx := context.Background()
	if n := skCount(t, pool, `SELECT count(*) FROM skill_level_type WHERE id NOT IN ($1, $2)`, skScaleA, skScaleB); n != 0 {
		t.Skip("the database has scales of its own; the one-scale fallback cannot be shown here")
	}
	skExec(t, pool, `UPDATE skill_level_type SET is_default = FALSE WHERE is_default`)
	if _, ok, err := repo.DefaultLevelTypeID(ctx); err != nil || ok {
		t.Fatalf("two unmarked scales gave a default: %v %v", ok, err)
	}
	skExec(t, pool, `DELETE FROM skill_level_type WHERE id = $1`, skScaleB)
	id, ok, err := repo.DefaultLevelTypeID(ctx)
	if err != nil || !ok || id != skScaleA {
		t.Fatalf("the only scale = %q, %v, %v", id, ok, err)
	}
}

var _ repository.SkillRepository = repository.NewSkillRepository(nil)
