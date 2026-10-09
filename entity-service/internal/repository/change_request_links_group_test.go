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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The assignment group is checked with the rest of the create form's links: a team
// the picker listed that has no "group" row is refused in words naming groupId,
// instead of reaching ServiceNow (a bare 404) or the foreign key (a message that
// quotes the table).

type existsRow struct{ v bool }

func (r existsRow) Scan(dest ...any) error {
	*(dest[0].(*bool)) = r.v
	return nil
}

// groupQueryer answers the one EXISTS query the group check runs and records it.
type groupQueryer struct {
	exists  bool
	err     error
	queries []string
	args    [][]any
}

func (q *groupQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query: no deployments were stated")
}

func (q *groupQueryer) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.queries = append(q.queries, sql)
	q.args = append(q.args, args)
	if q.err != nil {
		return groupErrRow{q.err}
	}
	return existsRow{q.exists}
}

func TestResolveChangeRequestLinks_AssignmentGroup(t *testing.T) {
	const handMade = "DDDDDDDD-0000-4000-8000-000000000102"
	sp := func(s string) *string { return &s }

	t.Run("a group that exists passes, matched on the lower-cased id", func(t *testing.T) {
		q := &groupQueryer{exists: true}
		if _, err := resolveChangeRequestLinks(context.Background(), q, domain.ChangeRequestLinkSelection{AssignmentGroupID: sp(" " + handMade + " ")}, resolveLinkOpts{}); err != nil {
			t.Fatalf("err = %v", err)
		}
		if len(q.queries) != 1 || !strings.Contains(q.queries[0], `FROM "group"`) {
			t.Fatalf("queries = %v, want one lookup in \"group\"", q.queries)
		}
		if got := q.args[0][0]; got != strings.ToLower(handMade) {
			t.Errorf("queried id = %v, want %s", got, strings.ToLower(handMade))
		}
	})

	t.Run("a group that does not exist is refused in words that say why and which field to change", func(t *testing.T) {
		q := &groupQueryer{exists: false}
		_, err := resolveChangeRequestLinks(context.Background(), q, domain.ChangeRequestLinkSelection{AssignmentGroupID: sp(handMade)}, resolveLinkOpts{})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("err = %v, want a ValidationError", err)
		}
		for _, want := range []string{"cannot be used", "not an assignment group in ServiceNow", `"Assignment group"`} {
			if !strings.Contains(ve.Msg, want) {
				t.Errorf("message %q does not contain %q", ve.Msg, want)
			}
		}
		for _, bad := range []string{"table", "assignment_group_id", "groupId", strings.ToLower(handMade)} {
			if strings.Contains(ve.Msg, bad) {
				t.Errorf("message %q shows %q to the person on the form", ve.Msg, bad)
			}
		}
	})

	t.Run("a malformed id is refused without a query", func(t *testing.T) {
		q := &groupQueryer{exists: true}
		_, err := resolveChangeRequestLinks(context.Background(), q, domain.ChangeRequestLinkSelection{AssignmentGroupID: sp("not-a-uuid")}, resolveLinkOpts{})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "groupId") {
			t.Fatalf("err = %v, want a ValidationError naming groupId", err)
		}
		if len(q.queries) != 0 {
			t.Errorf("queries = %v, want none", q.queries)
		}
	})

	t.Run("no group, or a blank one, costs nothing", func(t *testing.T) {
		for _, g := range []*string{nil, sp(""), sp("  ")} {
			q := &groupQueryer{}
			if _, err := resolveChangeRequestLinks(context.Background(), q, domain.ChangeRequestLinkSelection{AssignmentGroupID: g}, resolveLinkOpts{}); err != nil {
				t.Fatalf("group %v: err = %v", g, err)
			}
			if len(q.queries) != 0 {
				t.Errorf("group %v: queries = %v, want none", g, q.queries)
			}
		}
	})

	t.Run("a database failure is not reported as a missing group", func(t *testing.T) {
		boom := errors.New("connection reset")
		q := &groupQueryer{err: boom}
		_, err := resolveChangeRequestLinks(context.Background(), q, domain.ChangeRequestLinkSelection{AssignmentGroupID: sp(handMade)}, resolveLinkOpts{})
		var ve *apierror.ValidationError
		if !errors.Is(err, boom) || errors.As(err, &ve) {
			t.Fatalf("err = %v, want the database error, not a 400", err)
		}
	})
}

type groupErrRow struct{ err error }

func (r groupErrRow) Scan(...any) error { return r.err }
