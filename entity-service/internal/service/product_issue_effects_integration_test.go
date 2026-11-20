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
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/engineeringentity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The SLA clocks and the s_dip tag follow a product issue's GitHub state through
// the webhook branch, reconciliation and backfill, over the real repositories.
// Skipped without ENTITY_TEST_DATABASE_URL.
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/service/ -run ProductIssueEffects

var pieCreated = time.Date(2026, 11, 17, 5, 0, 0, 0, time.UTC)

func pieAtMin(m int) time.Time { return pieCreated.Add(time.Duration(m) * time.Minute) }

type pieEnv struct {
	*pwhEnv
	sla   WorkItemSLAEngine
	rule  ProductIssueCaseTagRule
	cases CaseService
}

func pieNewEffects(t *testing.T, pool *pgxpool.Pool) (WorkItemSLAEngine, ProductIssueCaseTagRule, CaseService) {
	t.Helper()
	cases := NewCaseService(repository.NewCaseRepository(repository.NewScoped(pool)), repository.NewUserRepository(pool), nil, internalAccess{}, repository.NewProjectContactRepository(pool))
	return NewWorkItemSLAEngine(repository.NewSLAEngineRepository(repository.NewScoped(pool), "")),
		NewProductIssueCaseTagRule(repository.NewProductIssueCaseTagRepository(repository.NewScoped(pool)), cases), cases
}

// pieSeed seeds the two linked issues (both priority P3, created pieCreated, both
// on the first case). The second is parked as RESOLVED so it does not hold the
// case's s_dip tag open; the first has its clock registered and the tag applied.
func pieSeed(t *testing.T) *pieEnv {
	t.Helper()
	e := &pieEnv{pwhEnv: pwhSeed(t)}
	e.sla, e.rule, e.cases = pieNewEffects(t, e.pool)
	for _, wi := range []string{e.wi1, e.wi2} {
		if _, err := e.pool.Exec(e.ctx, `UPDATE github_issue_link SET github_created_on = $2 WHERE work_item_id = $1::uuid`, wi, pieCreated); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.pool.Exec(e.ctx, `UPDATE github_issue_link SET board_status = 'RESOLVED' WHERE work_item_id = $1::uuid`, e.wi2); err != nil {
		t.Fatal(err)
	}
	if err := e.sla.RegisterWorkItemClocks(e.ctx, e.wi1); err != nil {
		t.Fatal(err)
	}
	if err := e.rule.ApplyCaseTagRuleForWorkItem(e.ctx, e.wi1); err != nil {
		t.Fatal(err)
	}
	if e.sdip() != 1 {
		t.Fatal("the issue's case should have s_dip from creation")
	}
	return e
}

func (e *pieEnv) frontWith(sla WorkItemSLAEngine, rule ProductIssueCaseTagRule) GithubSyncService {
	return WithProductIssueDeliveries(e.old, NewProductIssueDeliveries(repository.NewProductIssueSyncRepository(repository.NewScoped(e.pool)), nil).WithInlineFollowUps().WithWorkItemSLA(sla, rule))
}

func (e *pieEnv) sdip() int {
	return e.count(`SELECT count(*) FROM work_item_tag wit JOIN tag ON tag.id = wit.tag_id WHERE wit.work_item_id = $1::uuid AND lower(tag.name) = 's_dip'`, pwiCaseID)
}

func (e *pieEnv) caseNotes() int {
	return e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, pwiCaseID)
}

type pieClock struct {
	Policy, Stage string
	Start         time.Time
	Paused, End   *time.Time
	Active        bool
	Pause         float64
}

func (e *pieEnv) clocks(wi string) []pieClock {
	e.t.Helper()
	rs, err := e.pool.Query(e.ctx, `
		SELECT sp.name, s.stage::TEXT, s.start_on, s.paused_on, s.end_on, COALESCE(s.is_active, FALSE),
		       COALESCE(EXTRACT(EPOCH FROM s.pause_duration), 0)::float8
		  FROM sla s JOIN sla_policy sp ON sp.id = s.sla_policy_id
		 WHERE s.work_item_id = $1::uuid ORDER BY s.created_on, s.id`, wi)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rs.Close()
	var out []pieClock
	for rs.Next() {
		var c pieClock
		if err := rs.Scan(&c.Policy, &c.Stage, &c.Start, &c.Paused, &c.End, &c.Active, &c.Pause); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

func (e *pieEnv) last(wi string) pieClock {
	e.t.Helper()
	cs := e.clocks(wi)
	if len(cs) == 0 {
		e.t.Fatal("no clock")
	}
	return cs[len(cs)-1]
}

func pieTimeIs(got *time.Time, want time.Time) bool { return got != nil && got.Equal(want) }

// pieBoard is a board Status move of the issue behind node to the named column.
func pieBoard(t *testing.T, id, node, status string, at time.Time) pwhDelivery {
	return pwhFixture(t, "10").mutate(t, id, func(m map[string]any) {
		it := m["projects_v2_item"].(map[string]any)
		it["content_node_id"], it["updated_at"] = node, at.Format(time.RFC3339)
		m["changes"].(map[string]any)["field_value"].(map[string]any)["to"].(map[string]any)["name"] = status
	})
}

// pieIssue is an issues delivery for issue number (action closed, reopened, ...) at
// the time, with the labels (a P3 issue: Priority/Medium).
func pieIssue(t *testing.T, id, action string, number int, at time.Time, labels ...string) pwhDelivery {
	return pwhFixture(t, "20").mutate(t, id, func(m map[string]any) {
		m["action"] = action
		is := m["issue"].(map[string]any)
		is["number"], is["updated_at"] = number, at.Format(time.RFC3339)
		if action == "closed" {
			is["state"], is["closed_at"] = "closed", at.Format(time.RFC3339)
		} else {
			is["state"], is["closed_at"] = "open", nil
		}
		var ls []any
		for _, l := range labels {
			ls = append(ls, map[string]any{"name": l})
		}
		is["labels"] = ls
	})
}

var pieP3 = []string{"Origin/CS", "Priority/Medium"}

func TestProductIssueEffectsIntegration_WebhookLifecycle(t *testing.T) {
	e := pieSeed(t)
	svc := e.frontWith(e.sla, e.rule)
	if c := e.last(e.wi1); c.Policy != "Product Issue P3" || c.Stage != "IN_PROGRESS" || !c.Start.Equal(pieCreated) {
		t.Fatalf("seed clock = %+v", c)
	}

	e.ok(svc, pieBoard(t, "pwh-pie-open", pwhNode1, "Open", pieAtMin(1)))
	if c := e.last(e.wi1); c.Stage != "IN_PROGRESS" || c.Paused != nil {
		t.Errorf("after Open = %+v", c)
	}
	// Waiting on the customer pauses the clock at the event's own time.
	e.ok(svc, pieBoard(t, "pwh-pie-woc", pwhNode1, "WOC", pieAtMin(10)))
	if c := e.last(e.wi1); c.Stage != "PAUSED" || !pieTimeIs(c.Paused, pieAtMin(10)) {
		t.Errorf("after WOC = %+v", c)
	}
	if e.sdip() != 1 {
		t.Error("s_dip was removed while the issue is still open")
	}
	// Back in progress resumes it.
	e.ok(svc, pieBoard(t, "pwh-pie-ip", pwhNode1, "In Progress", pieAtMin(20)))
	if c := e.last(e.wi1); c.Stage != "IN_PROGRESS" || c.Paused != nil || c.Pause <= 0 {
		t.Errorf("after In Progress = %+v", c)
	}
	// Resolved completes it and the case no longer waits on a product team.
	e.ok(svc, pieBoard(t, "pwh-pie-res", pwhNode1, "Resolved", pieAtMin(30)))
	if c := e.last(e.wi1); c.Stage != "ACHIEVED" || !pieTimeIs(c.End, pieAtMin(30)) || c.Active {
		t.Errorf("after Resolved = %+v", c)
	}
	if e.sdip() != 0 {
		t.Error("s_dip should be removed once the issue is resolved")
	}

	// Back on the board, then closed and reopened on GitHub.
	e.ok(svc, pieBoard(t, "pwh-pie-ip2", pwhNode1, "In Progress", pieAtMin(40)))
	if c := e.last(e.wi1); !c.Active || c.End != nil {
		t.Errorf("after the board reopen = %+v", c)
	}
	if e.sdip() != 1 {
		t.Error("s_dip should return when the issue is worked on again")
	}
	e.ok(svc, pieIssue(t, "pwh-pie-close", "closed", 1, pieAtMin(50), pieP3...))
	if c := e.last(e.wi1); c.Stage != "ACHIEVED" || !pieTimeIs(c.End, pieAtMin(50)) {
		t.Errorf("after GitHub close = %+v", c)
	}
	if e.sdip() != 0 {
		t.Error("s_dip should be removed when GitHub closes the issue")
	}
	before := len(e.clocks(e.wi1))
	e.ok(svc, pieIssue(t, "pwh-pie-reopen", "reopened", 1, pieAtMin(55), pieP3...))
	c := e.last(e.wi1)
	if len(e.clocks(e.wi1)) != before+1 || !c.Active || c.End != nil || !c.Start.Equal(pieCreated) {
		t.Errorf("after GitHub reopen = %+v (clocks before %d, now %d)", c, before, len(e.clocks(e.wi1)))
	}
	if e.sdip() != 1 {
		t.Error("s_dip should return when GitHub reopens the issue")
	}
}

func TestProductIssueEffectsIntegration_OutOfOrderBoardDeliveryEqualsInOrder(t *testing.T) {
	e := pieSeed(t)
	if _, err := e.pool.Exec(e.ctx, `UPDATE github_issue_link SET board_status = NULL WHERE work_item_id = $1::uuid`, e.wi2); err != nil {
		t.Fatal(err)
	}
	if err := e.sla.RegisterWorkItemClocks(e.ctx, e.wi2); err != nil {
		t.Fatal(err)
	}
	svc := e.frontWith(e.sla, e.rule)
	moves := []struct {
		name   string
		status string
		min    int
	}{{"open", "Open", 1}, {"woc", "WOC", 10}, {"ip", "In Progress", 20}, {"res", "Resolved", 30}}
	for _, m := range moves { // in order, on the first issue
		e.ok(svc, pieBoard(t, "pwh-pie-a-"+m.name, pwhNode1, m.status, pieAtMin(m.min)))
	}
	for _, i := range []int{0, 1, 3, 2} { // In Progress arrives last, on the second
		m := moves[i]
		e.ok(svc, pieBoard(t, "pwh-pie-b-"+m.name, pwhNode2, m.status, pieAtMin(m.min)))
	}
	a, b := e.clocks(e.wi1), e.clocks(e.wi2)
	if len(a) != len(b) || len(a) == 0 {
		t.Fatalf("clocks %+v vs %+v", a, b)
	}
	for i := range a {
		if a[i].Policy != b[i].Policy || a[i].Stage != b[i].Stage || !a[i].Start.Equal(b[i].Start) || a[i].Active != b[i].Active ||
			(a[i].Paused == nil) != (b[i].Paused == nil) || (a[i].End == nil) != (b[i].End == nil) ||
			(a[i].End != nil && !a[i].End.Equal(*b[i].End)) || a[i].Pause != b[i].Pause {
			t.Errorf("clock %d differs:\n in order     %+v\n out of order %+v", i, a[i], b[i])
		}
	}
	if c := e.last(e.wi2); c.Stage != "ACHIEVED" || !pieTimeIs(c.End, pieAtMin(30)) || c.Pause != 600 {
		t.Errorf("out of order final clock = %+v (want ACHIEVED at +30m with the 10 minute pause)", c)
	}
	if e.link(e.wi2).Status != "RESOLVED" {
		t.Errorf("link = %+v", e.link(e.wi2))
	}
}

func TestProductIssueEffectsIntegration_PriorityChange(t *testing.T) {
	e := pieSeed(t)
	svc := e.frontWith(e.sla, e.rule)
	// An edit that leaves the priority alone adds nothing.
	e.ok(svc, pieIssue(t, "pwh-pie-edit", "edited", 1, pieAtMin(5), pieP3...))
	if n := len(e.clocks(e.wi1)); n != 1 {
		t.Fatalf("an edit without a priority change left %d clocks", n)
	}
	e.ok(svc, pieIssue(t, "pwh-pie-p1", "labeled", 1, pieAtMin(15), "Origin/CS", "Priority/Critical(P1)"))
	cs := e.clocks(e.wi1)
	if len(cs) != 2 {
		t.Fatalf("clocks = %+v", cs)
	}
	if cs[0].Policy != "Product Issue P3" || cs[0].Stage != "CANCELLED" || !pieTimeIs(cs[0].End, pieAtMin(15)) {
		t.Errorf("old clock = %+v", cs[0])
	}
	if cs[1].Policy != "Product Issue P1" || cs[1].Stage != "IN_PROGRESS" || !cs[1].Start.Equal(pieCreated) || !cs[1].Active {
		t.Errorf("new clock = %+v", cs[1])
	}
	// An older delivery is stale and changes nothing.
	e.ok(svc, pieIssue(t, "pwh-pie-stale", "unlabeled", 1, pieAtMin(2), pieP3...))
	if n := len(e.clocks(e.wi1)); n != 2 {
		t.Errorf("a stale delivery changed the clocks: %d", n)
	}
}

func TestProductIssueEffectsIntegration_DuplicateDeliveryChangesNothing(t *testing.T) {
	e := pieSeed(t)
	svc := e.frontWith(e.sla, e.rule)
	woc := pieBoard(t, "pwh-pie-dup", pwhNode1, "WOC", pieAtMin(10))
	e.ok(svc, woc)
	clocks, notes, events, tags := e.clocks(e.wi1), e.caseNotes(), len(e.events(e.wi1)), e.sdip()
	if _, err := e.send(svc, woc); !errors.Is(err, repository.ErrDeliverySeen) {
		t.Fatalf("replay = %v", err)
	}
	if fmt.Sprint(e.clocks(e.wi1)) != fmt.Sprint(clocks) || e.caseNotes() != notes || len(e.events(e.wi1)) != events || e.sdip() != tags {
		t.Error("a duplicate delivery changed the clocks, notes, events or tags")
	}
	// The same move under another delivery id is a second event with the same
	// outcome: no extra clock and no extra work note.
	e.ok(svc, pieBoard(t, "pwh-pie-dup2", pwhNode1, "WOC", pieAtMin(10)))
	if len(e.clocks(e.wi1)) != len(clocks) || e.caseNotes() != notes {
		t.Error("repeating a move added rows or notes")
	}
}

type pieFailSLA struct{ calls int }

func (f *pieFailSLA) RegisterWorkItemClocks(context.Context, string) error {
	f.calls++
	return errors.New("boom")
}
func (f *pieFailSLA) ApplyWorkItemStatusEffects(context.Context, string, time.Time, bool) error {
	f.calls++
	return errors.New("boom")
}
func (f *pieFailSLA) ReviseWorkItemClocks(context.Context, string, time.Time) error {
	f.calls++
	return errors.New("boom")
}
func (f *pieFailSLA) RecalculateWorkItemClocks(context.Context, string) error {
	f.calls++
	return errors.New("boom")
}

type pieFailTags struct{ calls int }

func (f *pieFailTags) ApplyCaseTagRuleForCase(context.Context, string) error {
	f.calls++
	return errors.New("boom")
}
func (f *pieFailTags) ApplyCaseTagRuleForWorkItem(context.Context, string) error {
	f.calls++
	return errors.New("boom")
}

func TestProductIssueEffectsIntegration_FollowUpErrorsDoNotFailTheDelivery(t *testing.T) {
	e := pieSeed(t)
	sla, tags := &pieFailSLA{}, &pieFailTags{}
	svc := e.frontWith(sla, tags)
	e.ok(svc, pieBoard(t, "pwh-pie-err1", pwhNode1, "WOC", pieAtMin(10)))
	e.ok(svc, pieIssue(t, "pwh-pie-err3", "labeled", 1, pieAtMin(20), "Origin/CS", "Priority/Critical(P1)"))
	e.ok(svc, pieIssue(t, "pwh-pie-err2", "closed", 1, pieAtMin(30), "Origin/CS", "Priority/Critical(P1)"))
	if sla.calls != 3 || tags.calls < 2 {
		t.Errorf("follow-ups ran sla=%d tags=%d times", sla.calls, tags.calls)
	}
	if !e.claimed("pwh-pie-err1") || !e.claimed("pwh-pie-err3") || e.link(e.wi1).State != "CLOSED" || len(e.events(e.wi1)) != 2 {
		t.Errorf("the writes did not stand: link %+v", e.link(e.wi1))
	}
	// A deleted issue runs the tag rule and no SLA step.
	slaCalls, tagCalls := sla.calls, tags.calls
	e.ok(svc, pieIssue(t, "pwh-pie-err4", "deleted", 1, pieAtMin(40), pieP3...))
	if e.link(e.wi1).LinkState != "DELETED" || sla.calls != slaCalls || tags.calls != tagCalls+1 {
		t.Errorf("deleted: link %+v sla %d->%d tags %d->%d", e.link(e.wi1), slaCalls, sla.calls, tagCalls, tags.calls)
	}
}

// --- reconciliation and backfill ---

func TestProductIssueEffectsIntegration_ReconcileRebuildsClocksAndConvergesTags(t *testing.T) {
	e := rcSeed(t)
	sla, rule, _ := pieNewEffects(t, e.pool)
	a := e.mkLinked(1, "seed A", "Open")
	e.exec(`UPDATE product_issue SET priority = 'P3' WHERE id = $1::uuid`, a)
	e.exec(`UPDATE github_issue_link SET github_created_on = $2 WHERE work_item_id = $1::uuid`, a, pieCreated)
	e.seedEvent(a, "Open", "wh-a0", pieAtMin(1))
	if err := sla.RegisterWorkItemClocks(e.ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := rule.ApplyCaseTagRuleForWorkItem(e.ctx, a); err != nil {
		t.Fatal(err)
	}
	sdip := func() int {
		return e.count(`SELECT count(*) FROM work_item_tag wit JOIN tag ON tag.id = wit.tag_id WHERE wit.work_item_id = $1::uuid AND lower(tag.name) = 's_dip'`, pwiCaseID)
	}
	if sdip() != 1 {
		t.Fatal("s_dip missing before the run")
	}
	resolvedAt := pieAtMin(120)
	eng := &fakeSyncEng{}
	eng.items = func(_ string, board int, _ string) (engineeringentity.ProjectItemsPage, error) {
		if board != rcBoard {
			return engineeringentity.ProjectItemsPage{}, nil
		}
		it := boardItem("I_rc1", 1, "Resolved")
		it.Content.Repository, it.Content.Title, it.Content.Labels = rcOwner+"/"+rcRepo, "seed A", []string{"Origin/CS", "Priority/Medium"}
		it.Content.UpdatedAt = "2026-11-18T10:00:00Z"
		return engineeringentity.ProjectItemsPage{Items: []engineeringentity.ProjectItem{it}}, nil
	}
	eng.history = func(int) ([]engineeringentity.ProjectStatusHistory, error) {
		return []engineeringentity.ProjectStatusHistory{{ProjectNumber: rcBoard, CurrentStatus: strp("Resolved"), Changes: []engineeringentity.ProjectStatusChange{
			{EventNodeID: "e-res", PreviousStatus: strp("Open"), NewStatus: strp("Resolved"), OccurredAt: resolvedAt.Format(time.RFC3339)},
		}}}, nil
	}
	svc := WithWorkItemSLA(e.service(eng), sla, rule)
	resp, err := svc.Reconcile(e.ctx, domain.ProductIssueReconcileRequest{})
	if err != nil || resp.StatusEventsWritten != 1 || resp.Failed != 0 {
		t.Fatalf("reconcile = %+v %v", resp, err)
	}
	pe := &pieEnv{pwhEnv: e.pwhEnv}
	if c := pe.last(a); c.Stage != "ACHIEVED" || !pieTimeIs(c.End, resolvedAt) || c.Active {
		t.Errorf("clock after reconcile = %+v", c)
	}
	if sdip() != 0 {
		t.Error("s_dip should be removed by the reconcile of a resolved issue")
	}
	// A run that finds nothing new changes nothing.
	rows := fmt.Sprint(pe.clocks(a))
	if _, err := svc.Reconcile(e.ctx, domain.ProductIssueReconcileRequest{}); err != nil || fmt.Sprint(pe.clocks(a)) != rows {
		t.Errorf("a second run changed the clocks: %v", err)
	}
}

func TestProductIssueEffectsIntegration_Backfill(t *testing.T) {
	e := rcSeed(t)
	sla, rule, _ := pieNewEffects(t, e.pool)
	closedAt := "2026-11-03T10:00:00Z"
	footer := "WSO2 case id : PWI-WSO2-1\nCase number : PWI-CASE-1"
	open := withBody(issue(201, "Priority/High(P2)"), footer)
	closed := withBody(issue(202, "Priority/High(P2)"), footer)
	closed.State, closed.ClosedAt, closed.UpdatedAt, closed.StateReason = "closed", &closedAt, closedAt, strp("completed")
	eng := &fakeSyncEng{}
	eng.list = func(q engineeringentity.ListIssueDetailsQuery) ([]engineeringentity.IssueDetail, error) {
		if q.Page > 1 {
			return nil, nil
		}
		if q.State == "closed" {
			return []engineeringentity.IssueDetail{closed}, nil
		}
		return []engineeringentity.IssueDetail{open}, nil
	}
	eng.comments = func(int, int) ([]engineeringentity.IssueComment, error) { return nil, nil }
	eng.history = func(int) ([]engineeringentity.ProjectStatusHistory, error) { return nil, nil }
	svc := WithWorkItemSLA(e.service(eng), sla, rule)
	req := domain.ProductIssueBackfillRequest{Owner: rcOwner, Repository: rcRepo, DryRun: true, MaxIssues: 100, SinceDays: 3650}
	slaRows := func() int {
		return e.count(`SELECT count(*) FROM sla s JOIN github_issue_link l ON l.work_item_id = s.work_item_id WHERE lower(l.owner) = $1`, rcOwner)
	}
	sdip := func() int {
		return e.count(`SELECT count(*) FROM work_item_tag wit JOIN tag ON tag.id = wit.tag_id WHERE wit.work_item_id = $1::uuid AND lower(tag.name) = 's_dip'`, pwiCaseID)
	}
	if _, err := svc.Backfill(e.ctx, req); err != nil || slaRows() != 0 || sdip() != 0 {
		t.Fatalf("dry run: rows=%d s_dip=%d err=%v", slaRows(), sdip(), err)
	}
	req.DryRun = false
	resp, err := svc.Backfill(e.ctx, req)
	if err != nil || resp.CreatedMatched != 2 || resp.Failed != 0 {
		t.Fatalf("backfill = %+v %v", resp, err)
	}
	pe := &pieEnv{pwhEnv: e.pwhEnv}
	wi := func(n int) string {
		var id string
		if err := e.pool.QueryRow(e.ctx, `SELECT work_item_id::TEXT FROM github_issue_link WHERE lower(owner) = $1 AND issue_number = $2`, rcOwner, n).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	if cs := pe.clocks(wi(202)); len(cs) != 1 || cs[0].Stage != "ACHIEVED" || cs[0].Active || !cs[0].Start.Equal(time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("resolved issue clocks = %+v", cs)
	}
	if cs := pe.clocks(wi(201)); len(cs) != 1 || cs[0].Stage != "IN_PROGRESS" {
		t.Errorf("open issue clocks = %+v", cs)
	}
	if sdip() != 1 {
		t.Error("the open backfilled issue should put s_dip on its case")
	}
	// A second run registers nothing new.
	if _, err := svc.Backfill(e.ctx, req); err != nil || slaRows() != 2 {
		t.Errorf("second run: rows=%d err=%v", slaRows(), err)
	}
	_ = strings.ToLower
}

func TestProductIssueEffectsIntegration_GitHubReopenOnResolvedCardReAddsTag(t *testing.T) {
	e := pieSeed(t)
	svc := e.frontWith(e.sla, e.rule)
	e.ok(svc, pieBoard(t, "pwh-pier-res", pwhNode1, "Resolved", pieAtMin(10)))
	if e.sdip() != 0 || e.last(e.wi1).Stage != "ACHIEVED" {
		t.Fatalf("after Resolved: s_dip=%d clock=%+v", e.sdip(), e.last(e.wi1))
	}
	e.ok(svc, pieIssue(t, "pwh-pier-close", "closed", 1, pieAtMin(20), pieP3...))
	if e.sdip() != 0 {
		t.Error("s_dip should stay off after the GitHub close")
	}
	before := len(e.clocks(e.wi1))
	// Reopened on GitHub while the card still sits on Resolved.
	e.ok(svc, pieIssue(t, "pwh-pier-reopen", "reopened", 1, pieAtMin(30), pieP3...))
	if l := e.link(e.wi1); l.Status != "RESOLVED" || l.State != "OPEN" {
		t.Fatalf("link = %+v", l)
	}
	c := e.last(e.wi1)
	if len(e.clocks(e.wi1)) != before+1 || !c.Active || c.End != nil || !c.Start.Equal(pieCreated) {
		t.Errorf("after reopen: %+v (clocks %d -> %d)", c, before, len(e.clocks(e.wi1)))
	}
	if e.sdip() != 1 {
		t.Error("s_dip should be re-added by a GitHub reopen on a Resolved card")
	}
	// A later board move to Resolved ends it again.
	e.ok(svc, pieBoard(t, "pwh-pier-res2", pwhNode1, "Resolved", pieAtMin(40)))
	if e.sdip() != 0 || e.last(e.wi1).Stage != "ACHIEVED" {
		t.Errorf("after Resolved again: s_dip=%d clock=%+v", e.sdip(), e.last(e.wi1))
	}
}

// A state flip that arrives without a close or reopen event (a labeled or edited
// delivery after a missed closed one) still moves the clock and the tag.
func TestProductIssueEffectsIntegration_StateFlipWithoutAnEventRebuilds(t *testing.T) {
	e := pieSeed(t)
	svc := e.frontWith(e.sla, e.rule)
	closed := pieIssue(t, "pwh-pie-flip", "closed", 1, pieAtMin(20), pieP3...)
	flip := closed.mutate(t, "pwh-pie-flip2", func(m map[string]any) { m["action"] = "edited" })
	e.ok(svc, flip)
	if l := e.link(e.wi1); l.State != "CLOSED" || len(e.events(e.wi1)) != 0 {
		t.Fatalf("link %+v, events %+v", l, e.events(e.wi1))
	}
	if c := e.last(e.wi1); c.Stage != "ACHIEVED" || c.Active {
		t.Errorf("clock after the flip = %+v", c)
	}
	if e.sdip() != 0 {
		t.Error("s_dip should be removed once the issue is closed")
	}
}

type pieBlockSLA struct {
	pieFailSLA
	started, release chan struct{}
	done             chan struct{}
}

func (b *pieBlockSLA) ApplyWorkItemStatusEffects(context.Context, string, time.Time, bool) error {
	b.calls++
	close(b.started)
	<-b.release
	close(b.done)
	return nil
}

// The webhook returns without waiting for the follow-up; the follow-up still runs.
func TestProductIssueEffectsIntegration_FollowUpIsAsynchronousOnTheWebhookPath(t *testing.T) {
	e := pieSeed(t)
	sla := &pieBlockSLA{started: make(chan struct{}), release: make(chan struct{}), done: make(chan struct{})}
	d := NewProductIssueDeliveries(repository.NewProductIssueSyncRepository(repository.NewScoped(e.pool)), nil).WithWorkItemSLA(sla, nil)
	svc := WithProductIssueDeliveries(e.old, d)
	e.ok(svc, pieBoard(t, "pwh-pie-async", pwhNode1, "WOC", pieAtMin(10)))
	// The delivery has returned and its write stands while the follow-up is still blocked.
	select {
	case <-sla.started:
	case <-time.After(10 * time.Second):
		t.Fatal("the follow-up never started")
	}
	select {
	case <-sla.done:
		t.Fatal("the follow-up finished before it was released")
	default:
	}
	if !e.claimed("pwh-pie-async") || e.link(e.wi1).Status != "WAITING_ON_CS" {
		t.Error("the write should stand while the follow-up runs")
	}
	close(sla.release)
	d.WaitFollowUps()
	select {
	case <-sla.done:
	default:
		t.Error("the follow-up did not finish")
	}
}

// --- reconcile convergence ---

func (e *rcEnv) reconcileItemOf(priorityLabel string) *fakeSyncEng {
	eng := &fakeSyncEng{}
	eng.items = func(_ string, board int, _ string) (engineeringentity.ProjectItemsPage, error) {
		if board != rcBoard {
			return engineeringentity.ProjectItemsPage{}, nil
		}
		it := boardItem("I_rc1", 1, "Open")
		it.Content.Repository, it.Content.Title = rcOwner+"/"+rcRepo, "seed A"
		it.Content.Labels = []string{"Origin/CS", priorityLabel}
		it.Content.UpdatedAt = "2026-11-10T10:00:00Z"
		return engineeringentity.ProjectItemsPage{Items: []engineeringentity.ProjectItem{it}}, nil
	}
	return eng
}

func (e *rcEnv) seedClocked(sla WorkItemSLAEngine, priority string) string {
	a := e.mkLinked(1, "seed A", "Open")
	e.exec(`UPDATE product_issue SET priority = $2 WHERE id = $1::uuid`, a, priority)
	e.exec(`UPDATE github_issue_link SET github_created_on = $2 WHERE work_item_id = $1::uuid`, a, pieCreated)
	e.seedEvent(a, "Open", "wh-a0", pieAtMin(1))
	if err := sla.RegisterWorkItemClocks(e.ctx, a); err != nil {
		e.t.Fatal(err)
	}
	return a
}

func TestProductIssueEffectsIntegration_ReconcileCancelsTheOldPriorityClock(t *testing.T) {
	e := rcSeed(t)
	sla, rule, _ := pieNewEffects(t, e.pool)
	a := e.seedClocked(sla, "P2")
	pe := &pieEnv{pwhEnv: e.pwhEnv}
	if c := pe.clocks(a); len(c) != 1 || c[0].Policy != "Product Issue P2" {
		t.Fatalf("seed clocks = %+v", c)
	}
	svc := WithWorkItemSLA(e.service(e.reconcileItemOf("Priority/Critical(P1)")), sla, rule)
	if resp, err := svc.Reconcile(e.ctx, domain.ProductIssueReconcileRequest{}); err != nil || resp.Failed != 0 {
		t.Fatalf("reconcile = %+v %v", resp, err)
	}
	cs := pe.clocks(a)
	open := 0
	for _, c := range cs {
		if c.Active {
			open++
		}
	}
	if len(cs) != 2 || cs[0].Policy != "Product Issue P2" || cs[0].Stage != "CANCELLED" || cs[1].Policy != "Product Issue P1" || !cs[1].Active || open != 1 {
		t.Errorf("clocks after the priority change = %+v", cs)
	}
}

// Nothing changed on GitHub, but the stored clock or tag is wrong (a follow-up that
// failed or timed out): a reconcile run repairs it.
func TestProductIssueEffectsIntegration_ReconcileRepairsUnchangedIssues(t *testing.T) {
	e := rcSeed(t)
	sla, rule, cases := pieNewEffects(t, e.pool)
	a := e.seedClocked(sla, "P3")
	pe := &pieEnv{pwhEnv: e.pwhEnv}
	svc := WithWorkItemSLA(e.service(e.reconcileItemOf("Priority/Medium")), sla, rule)
	run := func(label string, wantChanged bool) {
		t.Helper()
		resp, err := svc.Reconcile(e.ctx, domain.ProductIssueReconcileRequest{})
		if err != nil || resp.Failed != 0 || (resp.LinksUpdated > 0) != wantChanged {
			t.Fatalf("%s: %+v %v", label, resp, err)
		}
	}
	sdip := func() int {
		return e.count(`SELECT count(*) FROM work_item_tag wit JOIN tag ON tag.id = wit.tag_id WHERE wit.work_item_id = $1::uuid AND lower(tag.name) = 's_dip'`, pwiCaseID)
	}
	run("first run settles the labels", true)
	if sdip() != 1 || len(pe.clocks(a)) != 1 {
		t.Fatalf("after the first run: s_dip=%d clocks=%+v", sdip(), pe.clocks(a))
	}

	// The clock row is gone.
	e.exec(`DELETE FROM sla WHERE work_item_id = $1::uuid`, a)
	run("repairing a missing clock", false)
	if cs := pe.clocks(a); len(cs) != 1 || cs[0].Policy != "Product Issue P3" || cs[0].Stage != "IN_PROGRESS" || !cs[0].Start.Equal(pieCreated) {
		t.Errorf("missing clock not restored: %+v", cs)
	}
	// A stale PAUSED clock on an issue that is in progress.
	e.exec(`UPDATE sla SET stage = 'PAUSED', paused_on = $2 WHERE work_item_id = $1::uuid`, a, pieAtMin(30))
	run("repairing a stale pause", false)
	if c := pe.last(a); c.Stage != "IN_PROGRESS" || c.Paused != nil {
		t.Errorf("stale pause not repaired: %+v", c)
	}
	// s_dip removed by hand while the issue is open.
	var tagID string
	if err := e.pool.QueryRow(e.ctx, `SELECT tag.id::text FROM work_item_tag wit JOIN tag ON tag.id = wit.tag_id WHERE wit.work_item_id = $1::uuid AND lower(tag.name) = 's_dip' LIMIT 1`, pwiCaseID).Scan(&tagID); err != nil {
		t.Fatal(err)
	}
	if err := cases.RemoveCaseTagAs(e.ctx, pwiCaseID, tagID, ctrPerson); err != nil || sdip() != 0 {
		t.Fatalf("remove tag: %v s_dip=%d", err, sdip())
	}
	run("repairing the tag", false)
	if sdip() != 1 {
		t.Error("s_dip should be re-added while the issue is open")
	}
}
