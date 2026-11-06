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

package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// UpdateIncidentLifecycle's record fields against a real database: the
// edit dialog's save under dual-write. Skipped unless
// ENTITY_TEST_DATABASE_URL is set.

const (
	iuIncidentID = "48888888-0000-0000-0000-000000000001"
	iuWatcherID  = "48888888-0000-0000-0000-000000000002"
	iuGroupID    = "48888888-0000-0000-0000-000000000003"
	iuServiceID  = "48888888-0000-0000-0000-000000000004"
	iuMissingID  = "48888888-0000-0000-0000-0000000000ff"
)

// iuPriorityFor is the service's impact x urgency lookup, repeated here so
// the repository test does not import the service package.
func iuPriorityFor(impact, urgency string) string {
	rank := map[string]int{"HIGH": 0, "MEDIUM": 1, "LOW": 2}
	return [...]string{"CRITICAL", "HIGH", "MODERATE", "LOW", "PLANNING"}[rank[impact]+rank[urgency]]
}

func newIncidentUpdateIntegrationRepo(t *testing.T) (IncidentRepository, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	cleanup := func() {
		mustExec(t, pool, `DELETE FROM work_item_watcher WHERE work_item_id = $1`, iuIncidentID)
		mustExec(t, pool, `DELETE FROM work_item WHERE id = $1`, iuIncidentID)
		mustExec(t, pool, `DELETE FROM service WHERE id = $1`, iuServiceID)
		mustExec(t, pool, `DELETE FROM "group" WHERE id = $1`, iuGroupID)
		mustExec(t, pool, `DELETE FROM "user" WHERE id = $1`, iuWatcherID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec(t, pool, `INSERT INTO "user" (id, created_on, updated_on, user_name, email) VALUES ($1, NOW(), NOW(), 'iu.watcher', 'iu.watcher@example.com')`, iuWatcherID)
	mustExec(t, pool, `INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, NOW(), NOW(), 't', 't', 'IU Group')`, iuGroupID)
	mustExec(t, pool, `INSERT INTO service (id, created_on, updated_on, created_by, updated_by, name, number) VALUES ($1, NOW(), NOW(), 't', 't', 'IU Service', 'SVC-IU-1')`, iuServiceID)
	mustExec(t, pool, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		VALUES ($1, NOW(), NOW(), 't', 't', 'INC-IU-1', 'before', 'INCIDENT')`, iuIncidentID)
	mustExec(t, pool, `INSERT INTO incident (id, category, subcategory_id, impact, urgency, priority, environment)
		SELECT $1, 'NETWORK', id, 'LOW', 'LOW', 'PLANNING', 'staging'
		FROM incident_subcategory WHERE category = 'NETWORK' AND value = 'dns'`, iuIncidentID)
	return NewIncidentRepository(NewScoped(pool)), pool
}

type iuRow struct {
	subject, category, impact, urgency, priority, contactType string
	subcategory, environment, group, service                  *string
	watchers                                                  int
}

func readIncidentUpdateRow(t *testing.T, pool *pgxpool.Pool) iuRow {
	t.Helper()
	var r iuRow
	if err := pool.QueryRow(context.Background(), `
		SELECT wi.subject, inc.category::text, inc.impact::text, inc.urgency::text, inc.priority::text,
		       COALESCE(inc.contact_type::text, ''), sc.value, inc.environment,
		       wi.assignment_group_id::text, inc.service_id::text,
		       (SELECT count(*) FROM work_item_watcher w WHERE w.work_item_id = wi.id)
		FROM work_item wi JOIN incident inc ON inc.id = wi.id
		LEFT JOIN incident_subcategory sc ON sc.id = inc.subcategory_id
		WHERE wi.id = $1`, iuIncidentID).Scan(&r.subject, &r.category, &r.impact, &r.urgency, &r.priority,
		&r.contactType, &r.subcategory, &r.environment, &r.group, &r.service, &r.watchers); err != nil {
		t.Fatalf("read incident: %v", err)
	}
	return r
}

func iuStr(v string) *string { return &v }

func TestIncidentUpdateIntegration_RecordFields(t *testing.T) {
	repo, pool := newIncidentUpdateIntegrationRepo(t)
	ctx := WithSystemIdentity(context.Background())

	// Impact up, urgency unchanged (LOW): ServiceNow's lookup gives MODERATE.
	// A new category whose list lacks the current subcategory clears it.
	var cleared *string
	if _, err := repo.UpdateIncidentLifecycle(ctx, iuIncidentID, IncidentLifecycleUpdate{
		Subject:           iuStr("after"),
		Impact:            iuStr("HIGH"),
		Category:          iuStr("SECURITY"),
		ContactType:       iuStr("SITE_247"),
		AssignmentGroupID: iuStr(iuGroupID),
		ServiceID:         iuStr(iuServiceID),
		Environment:       &cleared,
		WatchList:         &[]string{iuWatcherID},
		PriorityFor:       iuPriorityFor,
	}, "tester@example.com"); err != nil {
		t.Fatalf("first update: %v", err)
	}
	r := readIncidentUpdateRow(t, pool)
	if r.subject != "after" || r.impact != "HIGH" || r.urgency != "LOW" || r.priority != "MODERATE" {
		t.Errorf("subject/impact/urgency/priority = %s/%s/%s/%s, want after/HIGH/LOW/MODERATE", r.subject, r.impact, r.urgency, r.priority)
	}
	if r.category != "SECURITY" || r.subcategory != nil {
		t.Errorf("category=%s subcategory=%v, want SECURITY and the old NETWORK subcategory cleared", r.category, r.subcategory)
	}
	if r.contactType != "SITE_24_7" {
		t.Errorf("contact type = %q, want SITE_24_7", r.contactType)
	}
	if r.environment != nil {
		t.Errorf("environment = %q, want cleared", *r.environment)
	}
	if r.group == nil || *r.group != iuGroupID || r.service == nil || *r.service != iuServiceID || r.watchers != 1 {
		t.Errorf("group=%v service=%v watchers=%d", r.group, r.service, r.watchers)
	}

	// A subcategory is looked up under the current category; an explicit
	// priority stands when impact and urgency are unchanged; an empty watch
	// list clears it.
	env := "production"
	envPtr := &env
	if _, err := repo.UpdateIncidentLifecycle(ctx, iuIncidentID, IncidentLifecycleUpdate{
		SubcategoryValue: iuStr("Phishing"),
		Priority:         iuStr("CRITICAL"),
		Environment:      &envPtr,
		WatchList:        &[]string{},
		PriorityFor:      iuPriorityFor,
	}, "tester@example.com"); err != nil {
		t.Fatalf("second update: %v", err)
	}
	r = readIncidentUpdateRow(t, pool)
	if r.subcategory == nil || *r.subcategory != "Phishing" || r.priority != "CRITICAL" ||
		r.environment == nil || *r.environment != "production" || r.watchers != 0 {
		t.Errorf("subcategory=%v priority=%s environment=%v watchers=%d", r.subcategory, r.priority, r.environment, r.watchers)
	}

	// Validation: a subcategory of another category, and a group that does not exist.
	for name, tc := range map[string]struct {
		u    IncidentLifecycleUpdate
		want string
	}{
		"subcategory of another category": {IncidentLifecycleUpdate{SubcategoryValue: iuStr("dns")}, "does not belong to category SECURITY"},
		"unknown assignment group":        {IncidentLifecycleUpdate{AssignmentGroupID: iuStr(iuMissingID)}, "assignmentGroupId does not identify an existing group"},
	} {
		_, err := repo.UpdateIncidentLifecycle(ctx, iuIncidentID, tc.u, "tester@example.com")
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || !strings.Contains(ve.Msg, tc.want) {
			t.Errorf("%s: got %v, want a ValidationError containing %q", name, err, tc.want)
		}
	}
	if r2 := readIncidentUpdateRow(t, pool); r2.subcategory == nil || *r2.subcategory != "Phishing" {
		t.Errorf("a rejected update changed the record: subcategory=%v", r2.subcategory)
	}
}

// The detail returns the watch list a PATCH wrote: the list's bulk edit adds
// to and removes from the list it reads, so an empty read would replace the
// stored watchers instead of adding to them.
func TestIncidentUpdateIntegration_DetailReadsWatchList(t *testing.T) {
	repo, _ := newIncidentUpdateIntegrationRepo(t)
	ctx := WithSystemIdentity(context.Background())

	v, err := repo.GetIncidentByID(ctx, iuIncidentID)
	if err != nil {
		t.Fatalf("GetIncidentByID: %v", err)
	}
	if v.WatchList == nil || len(v.WatchList) != 0 {
		t.Fatalf("watch list before any write = %#v, want an empty list (never null)", v.WatchList)
	}

	if _, err := repo.UpdateIncidentLifecycle(ctx, iuIncidentID, IncidentLifecycleUpdate{
		WatchList: &[]string{iuWatcherID},
	}, "tester@example.com"); err != nil {
		t.Fatalf("add watcher: %v", err)
	}
	v, err = repo.GetIncidentByID(ctx, iuIncidentID)
	if err != nil {
		t.Fatalf("GetIncidentByID: %v", err)
	}
	if len(v.WatchList) != 1 || v.WatchList[0].ID != iuWatcherID || v.WatchList[0].Email != "iu.watcher@example.com" {
		t.Fatalf("watch list = %#v, want the one watcher the update wrote", v.WatchList)
	}
	if v.WatchList[0].Name != "iu.watcher" {
		t.Errorf("watcher name = %q, want the user name when the user has no name", v.WatchList[0].Name)
	}

	if _, err := repo.UpdateIncidentLifecycle(ctx, iuIncidentID, IncidentLifecycleUpdate{
		WatchList: &[]string{},
	}, "tester@example.com"); err != nil {
		t.Fatalf("clear watch list: %v", err)
	}
	if v, err = repo.GetIncidentByID(ctx, iuIncidentID); err != nil || len(v.WatchList) != 0 {
		t.Errorf("watch list after clearing = %#v (err %v), want empty", v.WatchList, err)
	}
}
