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

// Integration tests for the port of ServiceNow's "Update Child Incidents"
// (resolving an incident resolves its open child incidents, recursively),
// the parentIncidentIds search filter, and the parent-link loop check,
// against a real Postgres with every migration through 0256 applied. Skipped
// unless INCIDENT_TASK_GATE_TEST_DSN is set, like the task cascade's tests.
//
//	INCIDENT_TASK_GATE_TEST_DSN=postgres://... go test ./internal/repository/ -run 'ChildIncident|IncidentParentLink'
package repository_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const childResolvedNote = "Resolved based on resolution of Parent Incident."

// newIncident creates an incident (NEW) under parentID ("" for none).
func newIncident(t *testing.T, pool *pgxpool.Pool, parentID string) string {
	t.Helper()
	req := icRequest()
	if parentID != "" {
		req.ParentIncidentID = &parentID
	}
	resp, err := repository.NewIncidentRepository(repository.NewScoped(pool)).CreateIncident(
		repository.WithSystemIdentity(context.Background()), req, "HIGH", nil, "jane.doe@test.local")
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	return resp.Incident.ID
}

// linkTo is the IncidentLifecycleUpdate.ParentIncidentID that links to parentID.
func linkTo(parentID string) **string {
	p := &parentID
	return &p
}

// parentIncidentOf reads id's stored parent incident, nil when it has none.
func parentIncidentOf(t *testing.T, pool *pgxpool.Pool, id string) *string {
	t.Helper()
	var parent *string
	if err := repository.NewScoped(pool).QueryRow(repository.WithSystemIdentity(context.Background()),
		`SELECT parent_incident_id::TEXT FROM incident WHERE id = $1`, id).Scan(&parent); err != nil {
		t.Fatalf("read parent of %s: %v", id, err)
	}
	return parent
}

// updateIncident applies u as actor and returns the children the cascade resolved.
func updateIncident(t *testing.T, pool *pgxpool.Pool, id string, u repository.IncidentLifecycleUpdate) []repository.ResolvedChildIncident {
	t.Helper()
	children, err := repository.NewIncidentRepository(repository.NewScoped(pool)).UpdateIncidentLifecycle(
		repository.WithSystemIdentity(context.Background()), id, u, "ic-engineer@test.local")
	if err != nil {
		t.Fatalf("update incident %s: %v", id, err)
	}
	return children
}

// resolveAs resolves id with code and notes, resolved by icEngineerID.
func resolveAs(t *testing.T, pool *pgxpool.Pool, id, code, notes string) []repository.ResolvedChildIncident {
	t.Helper()
	return updateIncident(t, pool, id, repository.IncidentLifecycleUpdate{
		State: strp("RESOLVED"), ResolutionCode: &code, ResolutionNotes: &notes, DefaultResolvedByID: strp(icEngineerID),
	})
}

type resolutionRow struct {
	state, code, notes, resolvedBy *string
	resolvedOn                     *time.Time
	withParent                     bool
	updatedBy                      string
	notesByType                    map[string][]string // comment type -> contents
}

func readResolution(t *testing.T, pool *pgxpool.Pool, id string) resolutionRow {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	r := resolutionRow{notesByType: map[string][]string{}}
	if err := scoped.QueryRow(ctx, `
		SELECT inc.state::TEXT, inc.resolution_code::TEXT, inc.close_notes, inc.resolved_by_id::TEXT,
		       inc.resolved_on, inc.resolved_with_parent, wi.updated_by
		FROM incident inc JOIN work_item wi ON wi.id = inc.id WHERE inc.id = $1`, id).
		Scan(&r.state, &r.code, &r.notes, &r.resolvedBy, &r.resolvedOn, &r.withParent, &r.updatedBy); err != nil {
		t.Fatalf("read incident %s: %v", id, err)
	}
	rows, err := scoped.Query(ctx, `SELECT type::TEXT, content FROM comment WHERE work_item_id = $1 AND content = $2`, id, childResolvedNote)
	if err != nil {
		t.Fatalf("read notes of %s: %v", id, err)
	}
	defer rows.Close()
	for rows.Next() {
		var typ, content string
		if err := rows.Scan(&typ, &content); err != nil {
			t.Fatalf("scan note: %v", err)
		}
		r.notesByType[typ] = append(r.notesByType[typ], content)
	}
	return r
}

// wantResolvedWithParent checks a child the cascade resolved: the parent's
// code, notes as given, the parent's resolver, now, the flag, the actor on
// work_item, and the note once, as a comment.
func wantResolvedWithParent(t *testing.T, name string, r resolutionRow, code, notes string) {
	t.Helper()
	if deref(r.state) != "RESOLVED" || deref(r.code) != code || deref(r.notes) != notes ||
		deref(r.resolvedBy) != icEngineerID || r.resolvedOn == nil || !r.withParent || r.updatedBy != "ic-engineer@test.local" {
		t.Errorf("%s: state=%s code=%s notes=%q resolved_by=%s resolved_on=%v with_parent=%v updated_by=%s; want RESOLVED/%s/%q/%s/set/true/ic-engineer@test.local",
			name, deref(r.state), deref(r.code), deref(r.notes), deref(r.resolvedBy), r.resolvedOn, r.withParent, r.updatedBy, code, notes, icEngineerID)
	}
	if len(r.notesByType["COMMENT"]) != 1 || len(r.notesByType["WORK_NOTE"]) != 0 {
		t.Errorf("%s: cascade notes = %v, want one COMMENT and no WORK_NOTE", name, r.notesByType)
	}
}

// TestChildIncidentCascade_ResolvesOpenChildren: resolving the parent
// resolves its New, On Hold and In Progress children and, through them, a
// grandchild. A child keeps a code and close notes of its own and takes the
// parent's otherwise. Resolved, Closed and Canceled children are left exactly
// as they were.
func TestChildIncidentCascade_ResolvesOpenChildren(t *testing.T) {
	pool := incidentTaskGatePool(t)
	parent := newIncident(t, pool, "")
	updateIncident(t, pool, parent, repository.IncidentLifecycleUpdate{State: strp("IN_PROGRESS")})

	childNew := newIncident(t, pool, parent)
	childHold := newIncident(t, pool, parent)
	updateIncident(t, pool, childHold, repository.IncidentLifecycleUpdate{State: strp("ON_HOLD"), ResolutionNotes: strp("child's own notes")})
	childOwnCode := newIncident(t, pool, parent)
	updateIncident(t, pool, childOwnCode, repository.IncidentLifecycleUpdate{State: strp("IN_PROGRESS"), ResolutionCode: strp("DUPLICATE")})
	grandchild := newIncident(t, pool, childNew)
	updateIncident(t, pool, grandchild, repository.IncidentLifecycleUpdate{State: strp("IN_PROGRESS")})

	childResolved := newIncident(t, pool, parent)
	resolveAs(t, pool, childResolved, "FALSE_ALARM", "resolved on its own")
	childClosed := newIncident(t, pool, parent)
	resolveAs(t, pool, childClosed, "FALSE_ALARM", "closed on its own")
	updateIncident(t, pool, childClosed, repository.IncidentLifecycleUpdate{State: strp("CLOSED")})
	childCanceled := newIncident(t, pool, parent)
	updateIncident(t, pool, childCanceled, repository.IncidentLifecycleUpdate{State: strp("CANCELED")})
	untouched := map[string]resolutionRow{}
	for _, id := range []string{childResolved, childClosed, childCanceled} {
		untouched[id] = readResolution(t, pool, id)
	}

	got := resolveAs(t, pool, parent, "SOLVED_WORK_AROUND", "parent notes")

	type summary struct{ id, prevState, prevCode, code string }
	var gotSummary []summary
	for _, c := range got {
		if c.Number == "" {
			t.Errorf("cascaded child %s has no number", c.ID)
		}
		prevCode := ""
		if c.PreviousResolutionCode != nil {
			prevCode = *c.PreviousResolutionCode
		}
		gotSummary = append(gotSummary, summary{c.ID, c.PreviousState, prevCode, deref(c.ResolutionCode)})
	}
	want := []summary{
		{childNew, "NEW", "", "SOLVED_WORKAROUND"},
		{childHold, "ON_HOLD", "", "SOLVED_WORKAROUND"},
		{childOwnCode, "IN_PROGRESS", "DUPLICATE", "DUPLICATE"},
		{grandchild, "IN_PROGRESS", "", "SOLVED_WORKAROUND"},
	}
	byID := func(s []summary) { sort.Slice(s, func(i, j int) bool { return s[i].id < s[j].id }) }
	byID(gotSummary)
	byID(want)
	if len(gotSummary) != len(want) {
		t.Fatalf("cascaded children = %+v, want %+v", gotSummary, want)
	}
	for i := range want {
		if gotSummary[i] != want[i] {
			t.Errorf("cascaded child = %+v, want %+v", gotSummary[i], want[i])
		}
	}

	wantResolvedWithParent(t, "New child", readResolution(t, pool, childNew), "SOLVED_WORK_AROUND", "parent notes")
	wantResolvedWithParent(t, "On Hold child", readResolution(t, pool, childHold), "SOLVED_WORK_AROUND", "child's own notes")
	wantResolvedWithParent(t, "child with its own code", readResolution(t, pool, childOwnCode), "DUPLICATE", "parent notes")
	// The grandchild follows its own parent, which took the top parent's notes.
	wantResolvedWithParent(t, "grandchild", readResolution(t, pool, grandchild), "SOLVED_WORK_AROUND", "parent notes")

	if p := readResolution(t, pool, parent); p.withParent || len(p.notesByType) != 0 {
		t.Errorf("parent: with_parent=%v cascade notes=%v, want false and none", p.withParent, p.notesByType)
	}
	for id, before := range untouched {
		after := readResolution(t, pool, id)
		if deref(after.state) != deref(before.state) || deref(after.code) != deref(before.code) || deref(after.notes) != deref(before.notes) ||
			after.withParent || len(after.notesByType) != 0 || !equalTimes(after.resolvedOn, before.resolvedOn) {
			t.Errorf("terminal child %s changed: %+v -> %+v", id, before, after)
		}
	}

	view, err := repository.NewIncidentRepository(repository.NewScoped(pool)).GetIncidentByID(repository.WithSystemIdentity(context.Background()), childNew)
	if err != nil {
		t.Fatalf("GetIncidentByID: %v", err)
	}
	if !view.ResolvedWithParent || view.ParentIncident == nil || view.ParentIncident.ID != parent {
		t.Errorf("child view: resolvedWithParent=%v parentIncident=%+v, want true and the parent", view.ResolvedWithParent, view.ParentIncident)
	}
}

func equalTimes(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// TestChildIncidentCascade_OnlyOnResolve: closing or canceling a parent
// straight from an open state, and re-opening a resolved one, leave the
// children as they are.
func TestChildIncidentCascade_OnlyOnResolve(t *testing.T) {
	pool := incidentTaskGatePool(t)

	closed := newIncident(t, pool, "")
	closedChild := newIncident(t, pool, closed)
	if got := updateIncident(t, pool, closed, repository.IncidentLifecycleUpdate{
		State: strp("CLOSED"), ResolutionCode: strp("SOLVED_PERMANENTLY"), ResolutionNotes: strp("closed directly"),
	}); len(got) != 0 || incidentState(t, pool, closedChild) != "NEW" {
		t.Errorf("close: cascaded %+v, child state %s; want nothing and NEW", got, incidentState(t, pool, closedChild))
	}

	canceled := newIncident(t, pool, "")
	canceledChild := newIncident(t, pool, canceled)
	if got := updateIncident(t, pool, canceled, repository.IncidentLifecycleUpdate{State: strp("CANCELED")}); len(got) != 0 ||
		incidentState(t, pool, canceledChild) != "NEW" {
		t.Errorf("cancel: cascaded %+v, child state %s; want nothing and NEW", got, incidentState(t, pool, canceledChild))
	}

	reopened := newIncident(t, pool, "")
	reopenedChild := newIncident(t, pool, reopened)
	if got := resolveAs(t, pool, reopened, "SOLVED_PERMANENTLY", "fixed"); len(got) != 1 {
		t.Fatalf("resolve: cascaded %+v, want the one child", got)
	}
	if got := updateIncident(t, pool, reopened, repository.IncidentLifecycleUpdate{State: strp("IN_PROGRESS")}); len(got) != 0 ||
		incidentState(t, pool, reopenedChild) != "RESOLVED" {
		t.Errorf("re-open: cascaded %+v, child state %s; want nothing and RESOLVED", got, incidentState(t, pool, reopenedChild))
	}
	// Resolving it again finds the child already Resolved: nothing to do.
	if got := resolveAs(t, pool, reopened, "SOLVED_PERMANENTLY", "fixed again"); len(got) != 0 {
		t.Errorf("second resolve: cascaded %+v, want nothing", got)
	}
}

// TestChildIncidentCascade_OwnResolveClearsTheFlag: a child resolved with its
// parent, re-opened and then resolved on its own is no longer marked.
func TestChildIncidentCascade_OwnResolveClearsTheFlag(t *testing.T) {
	pool := incidentTaskGatePool(t)
	parent := newIncident(t, pool, "")
	child := newIncident(t, pool, parent)
	resolveAs(t, pool, parent, "SOLVED_PERMANENTLY", "fixed")
	if !readResolution(t, pool, child).withParent {
		t.Fatal("child not marked resolved with parent")
	}
	updateIncident(t, pool, child, repository.IncidentLifecycleUpdate{State: strp("IN_PROGRESS")})
	resolveAs(t, pool, child, "FALSE_ALARM", "its own reason")
	if r := readResolution(t, pool, child); r.withParent || deref(r.code) != "FALSE_ALARM" {
		t.Errorf("after its own resolve: with_parent=%v code=%s, want false/FALSE_ALARM", r.withParent, deref(r.code))
	}
}

// TestChildIncidentCascade_SearchByParentIncident: parentIncidentIds finds
// the children through incident.parent_incident_id; parentIds (the generic
// work_item.parent_id) does not.
func TestChildIncidentCascade_SearchByParentIncident(t *testing.T) {
	pool := incidentTaskGatePool(t)
	parent := newIncident(t, pool, "")
	a, b := newIncident(t, pool, parent), newIncident(t, pool, parent)
	newIncident(t, pool, a) // a grandchild is not a child
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	search := func(f domain.SearchIncidentsFilters) []string {
		t.Helper()
		views, total, err := repo.SearchIncidents(ctx, domain.SearchIncidentsRequest{
			Filters: f, Pagination: domain.Pagination{Limit: 50},
		}, nil, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("SearchIncidents: %v", err)
		}
		var ids []string
		for _, v := range views {
			ids = append(ids, deref(v.ID))
			if v.ParentIncident == nil || v.ParentIncident.ID != parent {
				t.Errorf("row %s parentIncident = %+v, want %s", deref(v.ID), v.ParentIncident, parent)
			}
		}
		if total != len(ids) {
			t.Errorf("total = %d, rows = %d", total, len(ids))
		}
		sort.Strings(ids)
		return ids
	}
	want := []string{a, b}
	sort.Strings(want)
	if got := search(domain.SearchIncidentsFilters{ParentIncidentIDs: []string{parent}}); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("parentIncidentIds: got %v, want %v", got, want)
	}
	if got := search(domain.SearchIncidentsFilters{ParentIDs: []string{parent}}); len(got) != 0 {
		t.Errorf("parentIds matched %v; it is work_item.parent_id, not parent_incident_id", got)
	}
}

// TestIncidentParentLink_RejectsLoops: an incident cannot be its own parent,
// nor take one of its descendants as parent; any other incident is fine.
func TestIncidentParentLink_RejectsLoops(t *testing.T) {
	pool := incidentTaskGatePool(t)
	top := newIncident(t, pool, "")
	mid := newIncident(t, pool, top)
	bottom := newIncident(t, pool, mid)
	other := newIncident(t, pool, "")
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	for name, tc := range map[string]struct{ id, parent, want string }{
		"itself":     {top, top, "cannot be the incident itself"},
		"child":      {top, mid, "cannot be " + incidentNumber(t, pool, mid)},
		"grandchild": {top, bottom, "cannot be " + incidentNumber(t, pool, bottom)},
	} {
		_, err := repo.UpdateIncidentLifecycle(ctx, tc.id, repository.IncidentLifecycleUpdate{ParentIncidentID: linkTo(tc.parent)}, "ic-engineer@test.local")
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || !strings.Contains(ve.Msg, tc.want) {
			t.Errorf("%s: got %v, want a ValidationError containing %q", name, err, tc.want)
		}
	}
	var parentOfTop *string
	if err := repository.NewScoped(pool).QueryRow(ctx, `SELECT parent_incident_id::TEXT FROM incident WHERE id = $1`, top).Scan(&parentOfTop); err != nil || parentOfTop != nil {
		t.Errorf("refused links left top's parent = %v (err %v), want none", deref(parentOfTop), err)
	}

	updateIncident(t, pool, top, repository.IncidentLifecycleUpdate{ParentIncidentID: linkTo(other)})
	updateIncident(t, pool, bottom, repository.IncidentLifecycleUpdate{ParentIncidentID: linkTo(top)}) // re-parenting up the chain is fine
}

// TestChildIncidentCascade_DepthCapRollsBack: a chain deeper than the cap
// refuses the resolve, naming the first child past it, and nothing is saved
// -- neither the parent nor the children above the cap.
func TestChildIncidentCascade_DepthCapRollsBack(t *testing.T) {
	pool := incidentTaskGatePool(t)
	top := newIncident(t, pool, "")
	chain := []string{top}
	for i := 0; i < 11; i++ {
		chain = append(chain, newIncident(t, pool, chain[len(chain)-1]))
	}
	code, notes := "SOLVED_PERMANENTLY", "fixed"
	_, err := repository.NewIncidentRepository(repository.NewScoped(pool)).UpdateIncidentLifecycle(
		repository.WithSystemIdentity(context.Background()), top, repository.IncidentLifecycleUpdate{
			State: strp("RESOLVED"), ResolutionCode: &code, ResolutionNotes: &notes,
		}, "ic-engineer@test.local")
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, incidentNumber(t, pool, chain[11])) {
		t.Fatalf("got %v, want a ValidationError naming the 11th-level child %s", err, incidentNumber(t, pool, chain[11]))
	}
	for i, id := range chain {
		if r := readResolution(t, pool, id); deref(r.state) != "NEW" || r.withParent || len(r.notesByType) != 0 {
			t.Errorf("level %d: state=%s with_parent=%v notes=%v after the refused resolve, want it untouched", i, deref(r.state), r.withParent, r.notesByType)
		}
	}
}

// TestChildIncidentCascade_UnlinkClearsTheParent: a ParentIncidentID holding
// nil (the edit dialog's cleared Parent incident picker) unlinks the incident,
// and a nil ParentIncidentID leaves the link alone.
func TestChildIncidentCascade_UnlinkClearsTheParent(t *testing.T) {
	pool := incidentTaskGatePool(t)
	parent := newIncident(t, pool, "")
	child := newIncident(t, pool, parent)

	subject := "renamed"
	updateIncident(t, pool, child, repository.IncidentLifecycleUpdate{Subject: &subject})
	if got := parentIncidentOf(t, pool, child); got == nil || *got != parent {
		t.Fatalf("an update without ParentIncidentID changed the parent to %v, want %s", deref(got), parent)
	}

	updateIncident(t, pool, child, repository.IncidentLifecycleUpdate{ParentIncidentID: new(*string)})
	if got := parentIncidentOf(t, pool, child); got != nil {
		t.Fatalf("unlink left parent = %s, want none", *got)
	}

	// Resolving the former parent no longer touches the unlinked incident.
	code, notes := "SOLVED_PERMANENTLY", "fixed"
	if resolved := updateIncident(t, pool, parent, repository.IncidentLifecycleUpdate{
		State: strp("RESOLVED"), ResolutionCode: &code, ResolutionNotes: &notes,
	}); len(resolved) != 0 {
		t.Fatalf("resolving the former parent resolved %d children, want 0", len(resolved))
	}
}
