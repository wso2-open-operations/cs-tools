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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Product issue deliveries replayed through the real front door over the real
// repositories, from deliveries captured in the Phase 0 test organisation
// (testdata/github_webhooks, test-organisation data only; the one personal login
// was replaced by "test-user"). Skipped without ENTITY_TEST_DATABASE_URL.
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/service/ -run ProductIssueWebhook

const (
	pwhOwner   = "example-org"
	pwhRepo    = "example-repo"
	pwhNode1   = "I_kwDOVxO0As8AAAABf7upuQ"
	pwhNode2   = "I_kwDOVxO0As8AAAABf7wURQ"
	pwhProduct = "PWH Webhook Product"
)

type pwhDelivery struct {
	ID, Event string
	Raw       []byte
}

func pwhFixture(t *testing.T, prefix string) pwhDelivery {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "github_webhooks", prefix+"-*.json"))
	if len(files) != 1 {
		t.Fatalf("fixture %s: %v", prefix, files)
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		DeliveryID string          `json:"delivery_id"`
		Event      string          `json:"event"`
		Payload    json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	return pwhDelivery{ID: env.DeliveryID, Event: env.Event, Raw: env.Payload}
}

// mutate returns a copy of d under a new delivery id with its payload edited.
func (d pwhDelivery) mutate(t *testing.T, id string, edit func(m map[string]any)) pwhDelivery {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(d.Raw, &m); err != nil {
		t.Fatal(err)
	}
	edit(m)
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return pwhDelivery{ID: id, Event: d.Event, Raw: raw}
}

func (d pwhDelivery) delivery(t *testing.T) Delivery {
	t.Helper()
	var p IssuePayload
	if err := json.Unmarshal(d.Raw, &p); err != nil {
		t.Fatal(err)
	}
	return Delivery{ID: d.ID, Event: d.Event, Payload: p, Raw: d.Raw}
}

type pwhOld struct{ got []Delivery }

func (o *pwhOld) HandleWebhook(_ context.Context, d Delivery) (Outcome, error) {
	o.got = append(o.got, d)
	return Outcome{Action: "old_path"}, nil
}
func (o *pwhOld) CreateServiceRequestFromIssue(context.Context, domain.CreateServiceRequestFromIssueRequest) (domain.CreateServiceRequestFromIssueResponse, error) {
	return domain.CreateServiceRequestFromIssueResponse{}, nil
}
func (o *pwhOld) SetServiceRequestSeverityFromIssue(context.Context, string, string, int, domain.CaseSeverity) (domain.SetIssueSeverityResponse, error) {
	return domain.SetIssueSeverityResponse{}, nil
}

type pwhEnv struct {
	t        *testing.T
	pool     *pgxpool.Pool
	ctx      context.Context
	wi1, wi2 string
	old      *pwhOld
}

func (e *pwhEnv) front(old GithubSyncService, bots ...string) GithubSyncService {
	sc := repository.NewScoped(e.pool)
	return WithProductIssueDeliveries(old, NewProductIssueDeliveries(repository.NewProductIssueSyncRepository(sc), bots))
}

func (e *pwhEnv) send(svc GithubSyncService, d pwhDelivery) (Outcome, error) {
	e.t.Helper()
	return svc.HandleWebhook(e.ctx, d.delivery(e.t))
}

func (e *pwhEnv) ok(svc GithubSyncService, d pwhDelivery) Outcome {
	e.t.Helper()
	out, err := e.send(svc, d)
	if err != nil {
		e.t.Fatalf("delivery %s (%s): %v", d.ID, d.Event, err)
	}
	return out
}

func (e *pwhEnv) count(sql string, args ...any) int {
	e.t.Helper()
	return pwiCount(e.t, e.pool, sql, args...)
}

func (e *pwhEnv) claimed(id string) bool {
	return e.count(`SELECT count(*) FROM github_webhook_delivery WHERE delivery_id = $1`, id) == 1
}

func pwhSeed(t *testing.T) *pwhEnv {
	t.Helper()
	pool := pwiPool(t)
	pwiSeed(t, pool)
	ctx := repository.WithSystemIdentity(context.Background())
	e := &pwhEnv{t: t, pool: pool, ctx: ctx, old: &pwhOld{}}
	clean := func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM github_webhook_delivery WHERE delivery_id LIKE 'pwh-%' OR delivery_id IN (
			'98d59bc0-e246-11f1-9854-235a21a70610')`)
		for _, id := range e.deliveryIDs() {
			_, _ = pool.Exec(bg, `DELETE FROM github_webhook_delivery WHERE delivery_id = $1`, id)
		}
		_, _ = pool.Exec(bg, `DELETE FROM product_repo_mapping WHERE product_name = $1`, pwhProduct)
	}
	clean()
	t.Cleanup(clean)
	if _, err := pool.Exec(ctx, `INSERT INTO product_repo_mapping (created_by, updated_by, product_name, abbreviation, owner, repository, github_label, board_project_number)
		VALUES ('test', 'test', $1, 'pwh-abbr', $2, $3, 'product/pwh', 1)`, pwhProduct, strings.ToUpper(pwhOwner), pwhRepo); err != nil {
		t.Fatal(err)
	}
	mk := func(num int, node, title string) string {
		var id string
		err := pool.QueryRow(ctx, `
			INSERT INTO work_item (created_on, updated_on, created_by, updated_by, number, subject, type, account_id, project_id)
			VALUES (NOW(), NOW(), 'test', 'test', next_work_item_number('PRODUCT_ISSUE'), $1, 'PRODUCT_ISSUE', $2, $3) RETURNING id::TEXT`,
			title, pwiAccountID, pwiProjectID).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range []string{
			fmt.Sprintf(`INSERT INTO product_issue (id, priority, category) VALUES ('%s', 'P3', 'Type/Query')`, id),
			fmt.Sprintf(`INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id) VALUES ('%s', '%s', '%s', %d, '%s')`, id, pwhOwner, pwhRepo, num, node),
			fmt.Sprintf(`INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by) VALUES ('%s', '%s', 'AFFECTS', 'MANUAL', 'test', 'test')`, id, pwiCaseID),
		} {
			if _, err := pool.Exec(ctx, q); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	e.wi1 = mk(1, pwhNode1, "seed one")
	e.wi2 = mk(2, pwhNode2, "seed two")
	return e
}

func (e *pwhEnv) deliveryIDs() []string {
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "github_webhooks", "*.json"))
	var ids []string
	for _, f := range files {
		b, _ := os.ReadFile(f)
		var env struct {
			ID string `json:"delivery_id"`
		}
		_ = json.Unmarshal(b, &env)
		ids = append(ids, env.ID)
	}
	return ids
}

type pwhLink struct {
	State, Reason, Raw, Status, Item, LinkState string
	Board                                       *int
	Labels                                      string
	Updated, Synced, Closed                     *time.Time
}

func (e *pwhEnv) link(wi string) pwhLink {
	e.t.Helper()
	var l pwhLink
	var reason, raw, status, item *string
	err := e.pool.QueryRow(e.ctx, `SELECT github_state, state_reason, board_status_raw, board_status, board_item_id, link_state,
		board_project_number, labels::TEXT, github_updated_on, last_synced_on, github_closed_on FROM github_issue_link WHERE work_item_id = $1::uuid`, wi).
		Scan(&l.State, &reason, &raw, &status, &item, &l.LinkState, &l.Board, &l.Labels, &l.Updated, &l.Synced, &l.Closed)
	if err != nil {
		e.t.Fatal(err)
	}
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	l.Reason, l.Raw, l.Status, l.Item = deref(reason), deref(raw), deref(status), deref(item)
	return l
}

type pwhEvent struct {
	From, To, WaitsOn, Raw, Key, Source string
	At                                  time.Time
}

func (e *pwhEnv) events(wi string) []pwhEvent {
	e.t.Helper()
	rows, err := e.pool.Query(e.ctx, `SELECT COALESCE(from_status, ''), to_status, waits_on, COALESCE(raw_status, ''), dedupe_key, source, occurred_at
		FROM work_item_status_event WHERE work_item_id = $1::uuid ORDER BY occurred_at, created_on`, wi)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []pwhEvent
	for rows.Next() {
		var ev pwhEvent
		if err := rows.Scan(&ev.From, &ev.To, &ev.WaitsOn, &ev.Raw, &ev.Key, &ev.Source, &ev.At); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, ev)
	}
	return out
}

func (e *pwhEnv) subject(wi string) string {
	var s string
	if err := e.pool.QueryRow(e.ctx, `SELECT subject FROM work_item WHERE id = $1::uuid`, wi).Scan(&s); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *pwhEnv) product(wi string) (priority, category string) {
	var p, c *string
	if err := e.pool.QueryRow(e.ctx, `SELECT priority, category FROM product_issue WHERE id = $1::uuid`, wi).Scan(&p, &c); err != nil {
		e.t.Fatal(err)
	}
	if p != nil {
		priority = *p
	}
	if c != nil {
		category = *c
	}
	return
}

func titleOf(t *testing.T, d pwhDelivery) string {
	var m struct {
		Issue struct {
			Title string `json:"title"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(d.Raw, &m); err != nil {
		t.Fatal(err)
	}
	return m.Issue.Title
}

func TestProductIssueWebhookIntegration_IssueAndBoardLifecycle(t *testing.T) {
	e := pwhSeed(t)
	svc := e.front(e.old)

	// --- issues: opened, labeled -> subject, labels, priority and category ---
	opened := pwhFixture(t, "02")
	if out := e.ok(svc, opened); out.Action != "product_issue_opened" {
		t.Fatalf("opened = %+v", out)
	}
	if got := e.subject(e.wi1); got != titleOf(t, opened) {
		t.Errorf("subject = %q, want the issue title", got)
	}
	if p, c := e.product(e.wi1); p != "P2" || c != "Type/Incident" {
		t.Errorf("priority/category = %q/%q", p, c)
	}
	l := e.link(e.wi1)
	if l.Synced == nil || l.Updated == nil || !strings.Contains(l.Labels, "Origin/CS") || l.State != "OPEN" {
		t.Errorf("link after opened = %+v", l)
	}
	e.ok(svc, pwhFixture(t, "03"))
	if len(e.old.got) != 0 {
		t.Fatal("a linked delivery reached the customer path")
	}

	// --- board: item created before the board is known records nothing, and no claim ---
	created := pwhFixture(t, "06")
	if out := e.ok(svc, created); out.Skipped == "" || e.claimed(created.ID) {
		t.Errorf("cold created: %+v claimed=%v", out, e.claimed(created.ID))
	}
	// Status Open (the automation bot), then moves by a person.
	open := pwhFixture(t, "07")
	e.ok(svc, open)
	l = e.link(e.wi1)
	if l.Raw != "Open" || l.Status != "OPEN" || l.Item != "307799589" || l.Board == nil || *l.Board != 1 {
		t.Errorf("board after Open = %+v", l)
	}
	evs := e.events(e.wi1)
	if len(evs) != 1 || evs[0].From != "" || evs[0].To != "OPEN" || evs[0].WaitsOn != "PRODUCT_TEAM" || evs[0].Key != open.ID ||
		evs[0].Source != "WEBHOOK" || evs[0].At.UTC().Format(time.RFC3339) != "2026-11-17T05:20:26Z" {
		t.Errorf("events after Open = %+v", evs)
	}
	// Now the board is known, the created event records the item and board.
	e.ok(svc, pwhFixture(t, "06").mutate(t, "pwh-created-warm", func(m map[string]any) {}))
	if !e.claimed("pwh-created-warm") {
		t.Error("a created item on the known tracking board should be recorded")
	}

	woc := pwhFixture(t, "10")
	e.ok(svc, woc)
	if l = e.link(e.wi1); l.Raw != "WOC" || l.Status != "WAITING_ON_CS" {
		t.Errorf("board after WOC = %+v", l)
	}
	evs = e.events(e.wi1)
	if n := len(evs); n != 2 || evs[1].From != "OPEN" || evs[1].To != "WAITING_ON_CS" || evs[1].WaitsOn != "CS" || evs[1].Raw != "WOC" {
		t.Errorf("events after WOC = %+v", evs)
	}
	// A replayed delivery is refused and changes nothing.
	if _, err := e.send(svc, woc); !errors.Is(err, repository.ErrDeliverySeen) {
		t.Errorf("replay = %v, want ErrDeliverySeen", err)
	}
	if n := len(e.events(e.wi1)); n != 2 {
		t.Errorf("replay wrote events: %d", n)
	}
	e.ok(svc, pwhFixture(t, "11"))
	e.ok(svc, pwhFixture(t, "12"))
	if l = e.link(e.wi1); l.Status != "IN_PROGRESS" {
		t.Errorf("board after In Progress = %+v", l)
	}
	// A Labels field change on the board is not a Status change: ignored, not claimed.
	lab := pwhFixture(t, "13")
	if out := e.ok(svc, lab); out.Skipped == "" || e.claimed(lab.ID) {
		t.Errorf("labels field: %+v", out)
	}
	// An unknown column is OTHER; a different board is ignored.
	other := pwhFixture(t, "18").mutate(t, "pwh-other", func(m map[string]any) {
		m["changes"].(map[string]any)["field_value"].(map[string]any)["to"].(map[string]any)["name"] = "Pending Patch Queue"
	})
	e.ok(svc, other)
	if l = e.link(e.wi1); l.Status != "OTHER" || l.Raw != "Pending Patch Queue" {
		t.Errorf("board after unknown column = %+v", l)
	}
	if evs = e.events(e.wi1); evs[len(evs)-1].WaitsOn != "PRODUCT_TEAM" || evs[len(evs)-1].To != "OTHER" {
		t.Errorf("OTHER event = %+v", evs[len(evs)-1])
	}
	before := len(e.events(e.wi1))
	elsewhere := pwhFixture(t, "21").mutate(t, "pwh-other-board", func(m map[string]any) {
		m["changes"].(map[string]any)["field_value"].(map[string]any)["project_number"] = 7
	})
	if out := e.ok(svc, elsewhere); out.Skipped == "" || e.claimed("pwh-other-board") || len(e.events(e.wi1)) != before || e.link(e.wi1).Status != "OTHER" {
		t.Errorf("a non-tracking board changed the issue: %+v", out)
	}
	// Out of order: an older Status change is kept as history but does not move the link.
	e.ok(svc, pwhFixture(t, "18").mutate(t, "pwh-old-move", func(m map[string]any) {
		m["projects_v2_item"].(map[string]any)["updated_at"] = "2026-11-17T05:00:00Z"
		m["changes"].(map[string]any)["field_value"].(map[string]any)["to"].(map[string]any)["name"] = "Open"
	}))
	if l = e.link(e.wi1); l.Status != "OTHER" {
		t.Errorf("an out-of-order move overwrote the board status: %+v", l)
	}
	if evs = e.events(e.wi1); evs[0].Key != "pwh-old-move" || evs[0].From != "" {
		t.Errorf("an older move with no board event before it should have no from_status: %+v", evs[0])
	}
	e.ok(svc, pwhFixture(t, "18").mutate(t, "pwh-resolved", func(m map[string]any) {
		m["changes"].(map[string]any)["field_value"].(map[string]any)["to"].(map[string]any)["name"] = "  resolved "
	}))
	if l = e.link(e.wi1); l.Status != "RESOLVED" {
		t.Errorf("a status name is matched without case and spaces: %+v", l)
	}

	// --- labels: unlabeled/labeled -> priority P1; an older delivery is stale ---
	e.ok(svc, pwhFixture(t, "15"))
	e.ok(svc, pwhFixture(t, "16"))
	if p, _ := e.product(e.wi1); p != "P1" {
		t.Errorf("priority = %q, want P1", p)
	}
	stale := pwhFixture(t, "09")
	if out := e.ok(svc, stale); out.Action != "product_issue_stale" {
		t.Errorf("stale edit = %+v", out)
	}
	if got := e.subject(e.wi1); got != titleOf(t, pwhFixture(t, "16")) {
		t.Errorf("a stale delivery changed the subject to %q", got)
	}
	if p, _ := e.product(e.wi1); p != "P1" {
		t.Errorf("a stale delivery changed the priority to %q", p)
	}
	// A new title, longer than the column, is cut.
	long := pwhFixture(t, "17").mutate(t, "pwh-long", func(m map[string]any) {
		m["issue"].(map[string]any)["title"] = strings.Repeat("é", 300)
		m["issue"].(map[string]any)["updated_at"] = "2026-11-17T05:22:15Z"
	})
	e.ok(svc, long)
	if got := e.subject(e.wi1); len([]rune(got)) != 255 {
		t.Errorf("subject length = %d, want 255", len([]rune(got)))
	}

	// --- closed / reopened: link state, reason, date and a status event each ---
	e.ok(svc, pwhFixture(t, "19"))
	l = e.link(e.wi1)
	if l.State != "CLOSED" || l.Reason != "completed" || l.Closed == nil {
		t.Errorf("after closed = %+v", l)
	}
	evs = e.events(e.wi1)
	if last := evs[len(evs)-1]; last.To != "CLOSED" || last.From != "RESOLVED" || last.WaitsOn != "NOBODY" || last.Raw != "" || last.Key != pwhFixture(t, "19").ID {
		t.Errorf("closed event = %+v", last)
	}
	e.ok(svc, pwhFixture(t, "20"))
	l = e.link(e.wi1)
	if l.State != "OPEN" || l.Closed != nil {
		t.Errorf("after reopened = %+v", l)
	}
	evs = e.events(e.wi1)
	if last := evs[len(evs)-1]; last.To != "REOPENED" || last.From != "CLOSED" {
		t.Errorf("reopened event = %+v", last)
	}

	// --- a link that was never set: work_item.github_* stays NULL ---
	if n := e.count(`SELECT count(*) FROM work_item WHERE id = $1::uuid AND (github_owner IS NOT NULL OR github_repository IS NOT NULL OR github_issue_number IS NOT NULL)`, e.wi1); n != 0 {
		t.Error("work_item.github_* was set")
	}

	// --- deleted: link DELETED, history kept, later events ignored ---
	e.ok(svc, pwhFixture(t, "17").mutate(t, "pwh-deleted", func(m map[string]any) { m["action"] = "deleted" }))
	if l = e.link(e.wi1); l.LinkState != "DELETED" || len(e.events(e.wi1)) == 0 {
		t.Errorf("after deleted = %+v", l)
	}
	if out := e.ok(svc, pwhFixture(t, "17").mutate(t, "pwh-after-delete", func(m map[string]any) {})); out.Skipped == "" || e.claimed("pwh-after-delete") {
		t.Errorf("event after delete: %+v", out)
	}
	// --- transferred: the other issue's link is marked ---
	e.ok(svc, pwhFixture(t, "31").mutate(t, "pwh-transferred", func(m map[string]any) { m["action"] = "transferred" }))
	if l = e.link(e.wi2); l.LinkState != "TRANSFERRED" {
		t.Errorf("after transferred = %+v", l)
	}
}

func TestProductIssueWebhookIntegration_Comments(t *testing.T) {
	e := pwhSeed(t)
	svc := e.front(e.old)
	queueBefore := e.count(`SELECT count(*) FROM github_outbound_queue`)

	created := pwhFixture(t, "28")
	e.ok(svc, created)
	var cid, typ, by string
	var content *string
	err := e.pool.QueryRow(e.ctx, `SELECT c.id::TEXT, c.type::TEXT, c.created_by, c.content FROM comment c JOIN comment_github_ref r ON r.comment_id = c.id
		WHERE r.github_comment_id = 6939407244 AND r.origin = 'GITHUB' AND r.author_login = 'test-user' AND c.work_item_id = $1::uuid`, e.wi2).Scan(&cid, &typ, &by, &content)
	if err != nil {
		t.Fatalf("mirrored comment: %v", err)
	}
	if typ != "WORK_NOTE" || by != "test-user" || content == nil || *content == "" {
		t.Errorf("comment = %s %s %v", typ, by, content)
	}
	if n := e.count(`SELECT count(*) FROM github_outbound_queue`); n != queueBefore {
		t.Errorf("mirroring a comment queued %d outbound rows", n-queueBefore)
	}
	// The same GitHub comment again under another delivery id is not a second comment.
	e.ok(svc, created.mutate(t, "pwh-created-again", func(m map[string]any) {}))
	if n := e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, e.wi2); n != 1 {
		t.Errorf("comments = %d, want 1", n)
	}
	// It is in the product issue conversation read model.
	d, err := repository.NewProductIssueRepository(repository.NewScoped(e.pool)).GetDetail(e.ctx, e.wi2)
	if err != nil || len(d.Conversation) != 1 || d.Conversation[0].AuthorLogin == nil || *d.Conversation[0].AuthorLogin != "test-user" ||
		d.Conversation[0].Author != "test-user" || d.Conversation[0].Origin == nil || *d.Conversation[0].Origin != "GITHUB" {
		t.Fatalf("conversation = %+v %v", d.Conversation, err)
	}

	// Edited with the same text changes nothing; with new text it updates and keeps the old.
	e.ok(svc, pwhFixture(t, "29"))
	if n := e.count(`SELECT count(*) FROM comment_edit_history WHERE comment_id = $1::uuid`, cid); n != 0 {
		t.Errorf("unchanged edit wrote history: %d", n)
	}
	e.ok(svc, pwhFixture(t, "29").mutate(t, "pwh-edited", func(m map[string]any) {
		m["comment"].(map[string]any)["body"] = "rewritten"
		m["comment"].(map[string]any)["updated_at"] = "2026-11-17T09:00:00Z"
		m["sender"].(map[string]any)["login"] = "editor-login"
	}))
	if n := e.count(`SELECT count(*) FROM comment WHERE id = $1::uuid AND content = 'rewritten' AND last_edited_at IS NOT NULL`, cid); n != 1 {
		t.Error("the edit did not update the comment")
	}
	if n := e.count(`SELECT count(*) FROM comment_edit_history WHERE comment_id = $1::uuid AND body = $2`, cid, *content); n != 1 {
		t.Error("the previous text was not kept")
	}
	if n := e.count(`SELECT count(*) FROM comment_edit_history WHERE comment_id = $1::uuid AND edited_by = 'editor-login'`, cid); n != 1 {
		t.Error("edited_by should be the delivery's sender, not the comment's author")
	}
	// Deleted: soft-deleted on both tables and gone from the conversation.
	e.ok(svc, pwhFixture(t, "30"))
	if n := e.count(`SELECT count(*) FROM comment c JOIN comment_github_ref r ON r.comment_id = c.id WHERE c.id = $1::uuid AND c.deleted_at IS NOT NULL AND r.deleted_on IS NOT NULL`, cid); n != 1 {
		t.Error("the comment was not soft-deleted")
	}
	if d, _ = repository.NewProductIssueRepository(repository.NewScoped(e.pool)).GetDetail(e.ctx, e.wi2); len(d.Conversation) != 0 {
		t.Errorf("deleted comment still in the conversation: %+v", d.Conversation)
	}
	if n := e.count(`SELECT count(*) FROM github_outbound_queue`); n != queueBefore {
		t.Errorf("comment events queued %d outbound rows", n-queueBefore)
	}
	// An edit of a comment never seen is recorded as it reads now.
	e.ok(svc, pwhFixture(t, "29").mutate(t, "pwh-unseen", func(m map[string]any) {
		m["comment"].(map[string]any)["id"] = 7000000001
		m["comment"].(map[string]any)["body"] = "never seen"
	}))
	if n := e.count(`SELECT count(*) FROM comment_github_ref WHERE github_comment_id = 7000000001`); n != 1 {
		t.Error("an unseen edited comment was not recorded")
	}
}

func TestProductIssueWebhookIntegration_BotLoginsAndAutomation(t *testing.T) {
	e := pwhSeed(t)
	svc := e.front(e.old, "test-user", "github-project-automation[bot]")

	// The listed bot's comment is dropped, unclaimed.
	c := pwhFixture(t, "08")
	if out := e.ok(svc, c); out.Skipped == "" || e.claimed(c.ID) || e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, e.wi1) != 0 {
		t.Errorf("bot comment: %+v", out)
	}
	// The project automation account is never ignored, even listed: its board event
	// and its comment are both applied.
	e.ok(svc, pwhFixture(t, "07"))
	if l := e.link(e.wi1); l.Status != "OPEN" {
		t.Errorf("the automation's Status change was ignored: %+v", l)
	}
	e.ok(svc, c.mutate(t, "pwh-automation-comment", func(m map[string]any) {
		m["sender"].(map[string]any)["login"] = "github-project-automation[bot]"
	}))
	if n := e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, e.wi1); n != 1 {
		t.Errorf("the automation's comment was ignored: %d", n)
	}
	// The bot's issue events (labels, close) are not comment loops and are applied.
	e.ok(svc, pwhFixture(t, "19"))
	if l := e.link(e.wi1); l.State != "CLOSED" {
		t.Errorf("an issues event from a listed login was ignored: %+v", l)
	}
}

func TestProductIssueWebhookIntegration_UnlinkedTakesTheOldPath(t *testing.T) {
	e := pwhSeed(t)

	moved := func(id, ev string) pwhDelivery {
		d := pwhFixture(t, ev).mutate(t, id, func(m map[string]any) {
			m["repository"].(map[string]any)["name"] = "customer-repo"
		})
		return d
	}
	// A customer repository's issue and comment, and a board item for an unlinked issue,
	// reach the old path with the delivery untouched, and the branch records nothing.
	for _, d := range []pwhDelivery{
		moved("pwh-cust-issue", "02"), moved("pwh-cust-comment", "08"),
		pwhFixture(t, "07").mutate(t, "pwh-cust-board", func(m map[string]any) {
			m["projects_v2_item"].(map[string]any)["content_node_id"] = "I_unlinked"
		}),
		pwhFixture(t, "08").mutate(t, "pwh-cust-number", func(m map[string]any) { m["issue"].(map[string]any)["number"] = 99 }),
	} {
		old := &pwhOld{}
		svc := e.front(old)
		out, err := e.send(svc, d)
		if err != nil || out.Action != "old_path" || len(old.got) != 1 {
			t.Fatalf("%s: out=%+v err=%v old calls=%d", d.ID, out, err, len(old.got))
		}
		if got := old.got[0]; got.ID != d.ID || got.Event != d.Event || string(got.Raw) != string(d.Raw) || got.Payload.Repository.Name != d.delivery(t).Payload.Repository.Name {
			t.Errorf("%s: the old path got a changed delivery", d.ID)
		}
		if e.claimed(d.ID) {
			t.Errorf("%s: the branch claimed a delivery that is not its own", d.ID)
		}
	}
	// Owner and repository match without case.
	if out := e.ok(e.front(&pwhOld{}), pwhFixture(t, "02").mutate(t, "pwh-case", func(m map[string]any) {
		r := m["repository"].(map[string]any)
		r["name"] = strings.ToUpper(pwhRepo)
		r["owner"].(map[string]any)["login"] = strings.ToUpper(pwhOwner)
	})); out.Action != "product_issue_opened" {
		t.Errorf("case-insensitive match: %+v", out)
	}
	// An event type the branch does not know is not its business.
	old := &pwhOld{}
	if out := e.ok(e.front(old), pwhFixture(t, "02").mutate(t, "pwh-other-event", func(m map[string]any) {}).withEvent("pull_request")); out.Action != "old_path" {
		t.Errorf("unknown event: %+v", out)
	}
	// With the customer sync off, an unlinked delivery is skipped and nothing runs.
	if out := e.ok(e.front(nil), moved("pwh-sync-off", "02")); out.Skipped == "" || e.claimed("pwh-sync-off") {
		t.Errorf("sync off: %+v", out)
	}
	// The claim is part of the write: a failed write leaves none, so GitHub's retry
	// (same delivery id) runs, and a replay after that is the seen outcome.
	bad := pwhFixture(t, "08").mutate(t, "pwh-retry", func(m map[string]any) { m["comment"].(map[string]any)["body"] = "a\u0000b" })
	if _, err := e.send(e.front(&pwhOld{}), bad); err == nil || e.claimed("pwh-retry") {
		t.Fatalf("failed write: err=%v claimed=%v", err, e.claimed("pwh-retry"))
	}
	good := bad.mutate(t, "pwh-retry", func(m map[string]any) { m["comment"].(map[string]any)["body"] = "fine" })
	if out := e.ok(e.front(&pwhOld{}), good); out.Action != "product_issue_comment_created" || !e.claimed("pwh-retry") {
		t.Errorf("retry: %+v", out)
	}
	if _, err := e.send(e.front(&pwhOld{}), good); !errors.Is(err, repository.ErrDeliverySeen) {
		t.Errorf("replay after the retry = %v, want ErrDeliverySeen", err)
	}
	if n := e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, e.wi1); n != 1 {
		t.Errorf("comments = %d, want 1", n)
	}
}

func (d pwhDelivery) withEvent(ev string) pwhDelivery { d.Event = ev; return d }

func TestProductIssueWebhookIntegration_SameSecondDeliveryCannotReopenAClosedIssue(t *testing.T) {
	e := pwhSeed(t)
	svc := e.front(e.old)
	closed := pwhFixture(t, "19")
	e.ok(svc, closed)
	var m struct {
		Issue struct {
			UpdatedAt string `json:"updated_at"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(closed.Raw, &m); err != nil {
		t.Fatal(err)
	}
	// Same updated_at as the close, but its payload (read before the close landed)
	// still says open and carries a new priority label.
	e.ok(svc, pwhFixture(t, "16").mutate(t, "pwh-same-second", func(p map[string]any) {
		is := p["issue"].(map[string]any)
		is["updated_at"] = m.Issue.UpdatedAt
		is["state"] = "open"
		is["labels"] = []any{map[string]any{"name": "Origin/CS"}, map[string]any{"name": "Priority/High(P2)"}}
	}))
	l := e.link(e.wi1)
	if l.State != "CLOSED" || l.Reason != "completed" || l.Closed == nil {
		t.Errorf("a same-second labeled delivery changed the state: %+v", l)
	}
	if p, _ := e.product(e.wi1); p != "P2" || !strings.Contains(l.Labels, "Priority/High(P2)") {
		t.Errorf("a same-second delivery should still update labels and priority: %q %s", p, l.Labels)
	}
	// A same-second reopened delivery may change it.
	e.ok(svc, pwhFixture(t, "20").mutate(t, "pwh-same-second-reopen", func(p map[string]any) {
		p["issue"].(map[string]any)["updated_at"] = m.Issue.UpdatedAt
	}))
	if l = e.link(e.wi1); l.State != "OPEN" || l.Closed != nil {
		t.Errorf("a reopened delivery must change the state: %+v", l)
	}
}

func TestProductIssueWebhookIntegration_DeleteBeforeCreateLeavesATombstone(t *testing.T) {
	e := pwhSeed(t)
	svc := e.front(e.old)
	deleted := pwhFixture(t, "30")
	if out := e.ok(svc, deleted); out.Action != "product_issue_comment_deleted" || out.Skipped != "" {
		t.Fatalf("delete of an unknown comment = %+v", out)
	}
	if n := e.count(`SELECT count(*) FROM comment c JOIN comment_github_ref r ON r.comment_id = c.id
		WHERE r.github_comment_id = 6939407244 AND c.work_item_id = $1::uuid AND c.deleted_at IS NOT NULL AND r.deleted_on IS NOT NULL
		AND c.content IS NOT NULL AND c.content <> ''`, e.wi2); n != 1 {
		t.Fatal("no tombstone for the deleted comment")
	}
	// The created delivery that arrives after it is skipped.
	if out := e.ok(svc, pwhFixture(t, "28")); out.Skipped == "" {
		t.Errorf("late created = %+v, want skipped", out)
	}
	if n := e.count(`SELECT count(*) FROM comment WHERE work_item_id = $1::uuid`, e.wi2); n != 1 {
		t.Errorf("comments = %d, want the tombstone only", n)
	}
	d, err := repository.NewProductIssueRepository(repository.NewScoped(e.pool)).GetDetail(e.ctx, e.wi2)
	if err != nil || len(d.Conversation) != 0 {
		t.Errorf("a tombstone shows in the conversation: %+v %v", d.Conversation, err)
	}
}

func TestProductIssueWebhookIntegration_OutOfOrderMoveKeepsItsOwnFromStatus(t *testing.T) {
	e := pwhSeed(t)
	svc := e.front(e.old)
	e.ok(svc, pwhFixture(t, "07")) // Open at 05:20:26
	e.ok(svc, pwhFixture(t, "12")) // In Progress at 05:22:07
	e.ok(svc, pwhFixture(t, "18").mutate(t, "pwh-between", func(m map[string]any) {
		m["projects_v2_item"].(map[string]any)["updated_at"] = "2026-11-17T05:21:00Z"
		m["changes"].(map[string]any)["field_value"].(map[string]any)["to"].(map[string]any)["name"] = "WOC"
	}))
	if l := e.link(e.wi1); l.Status != "IN_PROGRESS" {
		t.Errorf("the older move moved the link: %+v", l)
	}
	var from *string
	if err := e.pool.QueryRow(e.ctx, `SELECT from_status FROM work_item_status_event WHERE dedupe_key = 'pwh-between'`).Scan(&from); err != nil {
		t.Fatal(err)
	}
	if from == nil || *from != "OPEN" {
		t.Errorf("from_status = %v, want OPEN (the board event before it), not the link's current status", from)
	}
}

func TestProductIssueWebhookIntegration_BoardEventChecks(t *testing.T) {
	e := pwhSeed(t)
	svc := e.front(e.old)
	// A Status field that is not a single select is not the board column.
	notSelect := pwhFixture(t, "07").mutate(t, "pwh-text-field", func(m map[string]any) {
		m["changes"].(map[string]any)["field_value"].(map[string]any)["field_type"] = "text"
	})
	if out := e.ok(svc, notSelect); out.Skipped == "" || e.claimed(notSelect.ID) || e.link(e.wi1).Raw != "" {
		t.Errorf("a non single-select Status field was applied: %+v", out)
	}
	// No organisation: the check cannot be made, so the event is refused.
	noOrg := pwhFixture(t, "07").mutate(t, "pwh-no-org", func(m map[string]any) { delete(m, "organization") })
	if out := e.ok(svc, noOrg); out.Skipped == "" || e.claimed(noOrg.ID) || e.link(e.wi1).Raw != "" {
		t.Errorf("a board event without an organisation was applied: %+v", out)
	}
	// The unchanged fixture is applied.
	if e.ok(svc, pwhFixture(t, "07")); e.link(e.wi1).Raw != "Open" {
		t.Error("the board event should be applied")
	}
}

func TestProductIssueWebhookIntegration_RealCustomerSyncBehindTheFrontDoor(t *testing.T) {
	e := pwhSeed(t)
	sc := repository.NewScoped(e.pool)
	real := NewGithubSyncServiceWriting(repository.NewGithubSyncRepository(sc), repository.NewGithubMutationRepository(sc),
		&fakeGhClient{}, "wso2-integration-bot", DefaultGithubLabels())
	front := e.front(real)
	for _, fx := range []string{"02", "08"} {
		customer := func(id string) pwhDelivery {
			return pwhFixture(t, fx).mutate(t, id, func(m map[string]any) {
				m["repository"].(map[string]any)["name"] = "customer-repo"
			})
		}
		wantOut, wantErr := e.send(real, customer("pwh-direct-"+fx))
		gotOut, gotErr := e.send(front, customer("pwh-front-"+fx))
		if gotOut != wantOut || (gotErr == nil) != (wantErr == nil) || (wantErr != nil && gotErr.Error() != wantErr.Error()) {
			t.Errorf("fixture %s: through the front door = %+v, %v; direct = %+v, %v", fx, gotOut, gotErr, wantOut, wantErr)
		}
		if gotOut == (Outcome{}) && gotErr == nil {
			t.Errorf("fixture %s: the customer sync reported nothing", fx)
		}
	}
}
