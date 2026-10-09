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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// userQuerier fakes the user lookups upsertMembershipUser runs: by sf_id, then
// by email. It records the UPDATE so a test can read the reactivation flag.
type userQuerier struct {
	bySfID  bool   // the sf_id lookup finds the user
	byEmail bool   // the email lookup finds the user
	userSf  string // that user's stored sf_id
	execs   []recordedExec
}

type userRow struct{ id, name, sf string }

func (r userRow) Scan(dest ...any) error {
	if r.id == "" {
		return pgx.ErrNoRows
	}
	*dest[0].(*string), *dest[1].(*string) = r.id, r.name
	if len(dest) > 2 {
		switch d := dest[2].(type) {
		case *int64:
			*d = 1
		case *string:
			*d = r.sf
		}
	}
	return nil
}

func (q *userQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if q.bySfID {
		return userRow{id: "user-1", name: "jane@acme.com", sf: q.userSf}
	}
	return userRow{}
}

func (q *userQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execs = append(q.execs, recordedExec{sql: sql, args: args})
	return pgconn.CommandTag{}, nil
}

func (q *userQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if q.byEmail {
		return &userRows{rows: []userRow{{"user-1", "jane@acme.com", q.userSf}}}, nil
	}
	return &userRows{}, nil
}

// userRows is the smallest pgx.Rows that can hand back the email matches.
type userRows struct {
	rows []userRow
	i    int
}

func (r *userRows) Next() bool                                   { r.i++; return r.i <= len(r.rows) }
func (r *userRows) Scan(dest ...any) error                       { return r.rows[r.i-1].Scan(dest...) }
func (r *userRows) Close()                                       {}
func (r *userRows) Err() error                                   { return nil }
func (r *userRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *userRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *userRows) Values() ([]any, error)                       { return nil, errors.New("unused") }
func (r *userRows) RawValues() [][]byte                          { return nil }
func (r *userRows) Conn() *pgx.Conn                              { return nil }
func (r *userRows) TypeMap() *pgtype.Map                         { return nil }

// TestUpsertMembershipUserReactivatesOnlyAReplacedContact pins the rule: an
// existing user is set active only when matched by email with a different,
// non-empty sf_id (the person's contact was replaced) and the membership is
// live. The same contact, or a DEACTIVATED membership, never reactivates.
func TestUpsertMembershipUserReactivatesOnlyAReplacedContact(t *testing.T) {
	cases := map[string]struct {
		q    *userQuerier
		st   string
		want bool
	}{
		"replaced contact, re-invited":  {&userQuerier{byEmail: true, userSf: "003OLD"}, domain.MembershipStateReInvited, true},
		"replaced contact, no state":    {&userQuerier{byEmail: true, userSf: "003OLD"}, "", true},
		"replaced contact, deactivated": {&userQuerier{byEmail: true, userSf: "003OLD"}, domain.MembershipStateDeactivated, false},
		"same contact by sf_id":         {&userQuerier{bySfID: true, userSf: "003NEW"}, domain.MembershipStateRegistered, false},
		"email match without sf_id":     {&userQuerier{byEmail: true}, domain.MembershipStateInvited, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := domain.SalesforceMembershipUpsert{ContactSfID: "003NEW", ContactEmail: "jane@acme.com", State: tc.st}
			if _, _, created, err := upsertMembershipUser(context.Background(), tc.q, in, "test"); err != nil || created {
				t.Fatalf("err = %v, created = %v", err, created)
			}
			var got *bool
			for _, e := range tc.q.execs {
				if strings.Contains(e.sql, `UPDATE "user"`) {
					b := e.args[8].(bool)
					got = &b
				}
			}
			if got == nil || *got != tc.want {
				t.Fatalf("reactivate = %v, want %v", got, tc.want)
			}
		})
	}
}
