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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// linkQuerier is an in-memory sf_opportunity_link keyed by link_sf_id.
type linkQuerier struct {
	links      map[string]domain.SalesforceOpportunityLinkUpsert
	statements []string
	ledger     []string
	derived    []string // project ids the onboarding rule ran for
	hours      []string // project ids the project hours hook ran for
}

func (q *linkQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.statements = append(q.statements, sql)
	switch sql {
	case deriveOnboardingForProjectQuery:
		q.derived = append(q.derived, args[1].(string))
		return pgconn.NewCommandTag("UPDATE 0"), nil
	case insertSfOpportunityLinkQuery:
		q.put(args)
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	case deleteSfOpportunityLinkQuery:
		if _, ok := q.links[args[0].(string)]; !ok {
			return pgconn.NewCommandTag("DELETE 0"), nil
		}
		delete(q.links, args[0].(string))
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (q *linkQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, fmt.Errorf("unexpected Query")
}

func (q *linkQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.statements = append(q.statements, sql)
	if sql == projectHoursStateQuery {
		// The project hours hook; an unknown project is a no-op.
		q.hours = append(q.hours, args[0].(string))
		return scanRow{err: pgx.ErrNoRows}
	}
	if sql == previousLinkProjectsQuery {
		prev := []string{}
		if l, ok := q.links[args[0].(string)]; ok && l.ProjectID != "" {
			prev = append(prev, l.ProjectID)
		}
		return scanRow{vals: []any{prev}}
	}
	if sql == updateSfOpportunityLinkQuery {
		if _, ok := q.links[args[4].(string)]; !ok {
			return scanRow{err: pgx.ErrNoRows}
		}
		q.put(args)
		return scanRow{vals: []any{"link-row", int64(1)}}
	}
	if strings.Contains(sql, "INSERT INTO salesforce_ingest_state") {
		q.ledger = append(q.ledger, args[3].(string))
		return scanRow{}
	}
	return scanRow{err: fmt.Errorf("unexpected QueryRow: %.60s", sql)}
}

func (q *linkQuerier) put(args []any) {
	id := args[4].(string)
	q.links[id] = domain.SalesforceOpportunityLinkUpsert{LinkSfID: id, Number: args[1].(*string), OpportunityID: args[2].(string), ProjectID: args[3].(string)}
}

func linkState(eventType string) domain.UpsertSalesforceIngestStateRequest {
	return domain.UpsertSalesforceIngestStateRequest{
		Entity: domain.SalesforceIngestEntityLinkedOpportunity, SfID: "a3Uxx", EventModifiedOn: time.Unix(1, 0).UTC(),
		EventType: eventType, Status: domain.SalesforceIngestSucceeded,
	}
}

// TestWriteSfOpportunityLink_InsertThenUpdate: the first write inserts, the
// second updates the same link_sf_id (new project), each with a ledger row
// after an advisory lock.
func TestWriteSfOpportunityLink_InsertThenUpdate(t *testing.T) {
	q := &linkQuerier{links: map[string]domain.SalesforceOpportunityLinkUpsert{}}
	num := "LO-1"
	created, err := writeSfOpportunityLink(context.Background(), q, domain.SalesforceOpportunityLinkUpsert{LinkSfID: "a3Uxx", Number: &num, OpportunityID: "opp", ProjectID: "p1"}, linkState("CREATED"))
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	created, err = writeSfOpportunityLink(context.Background(), q, domain.SalesforceOpportunityLinkUpsert{LinkSfID: "a3Uxx", Number: &num, OpportunityID: "opp", ProjectID: "p2"}, linkState("UPDATED"))
	if err != nil || created || len(q.links) != 1 || q.links["a3Uxx"].ProjectID != "p2" {
		t.Errorf("created=%v err=%v links=%+v", created, err, q.links)
	}
	if !strings.Contains(q.statements[0], "pg_advisory_xact_lock") || !reflect.DeepEqual(q.ledger, []string{"CREATED", "UPDATED"}) {
		t.Errorf("first statement %.40s ledger %v", q.statements[0], q.ledger)
	}
	// Insert derives p1; the move to p2 derives p2 and then the previous owner p1.
	if want := []string{"p1", "p2", "p1"}; !reflect.DeepEqual(q.derived, want) {
		t.Errorf("derived projects = %v, want %v", q.derived, want)
	}
	// Project hours follow the same owners, after the onboarding derivation.
	if want := []string{"p1", "p2", "p1"}; !reflect.DeepEqual(q.hours, want) {
		t.Errorf("project hours recomputed for %v, want %v", q.hours, want)
	}
}

func TestDeleteSfOpportunityLink(t *testing.T) {
	q := &linkQuerier{links: map[string]domain.SalesforceOpportunityLinkUpsert{"a3Uxx": {}}}
	n, err := deleteSfOpportunityLink(context.Background(), q, "a3Uxx", linkState("DELETED"))
	if err != nil || n != 1 || len(q.links) != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	n, err = deleteSfOpportunityLink(context.Background(), q, "a3Uxx", linkState("DELETED"))
	if err != nil || n != 0 || len(q.ledger) != 2 {
		t.Errorf("second delete n=%d err=%v ledger=%v", n, err, q.ledger)
	}
	if deleteSfOpportunityLinkQuery != `DELETE FROM sf_opportunity_link WHERE link_sf_id = $1` {
		t.Errorf("delete query = %q", deleteSfOpportunityLinkQuery)
	}
}

func TestSfOpportunityLinkSQL_ColumnList(t *testing.T) {
	want := []string{"number", "opportunity_id", "project_id", "sync_time_stamp", "updated_by", "updated_on"}
	if got := setColumns(updateSfOpportunityLinkQuery); !reflect.DeepEqual(got, want) {
		t.Errorf("link UPDATE columns = %v, want %v", got, want)
	}
}
