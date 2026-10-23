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
	"time"

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

	// emailRows, when set, replaces the single byEmail match with these rows.
	emailRows []emailUserMatch
	// sfRows are the users the sf_id lookup finds when bySfID is unset: the
	// first one whose sfID equals the queried id (active first, then oldest).
	sfRows []emailUserMatch
	// inserted counts the INSERT INTO "user" statements.
	inserted int
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

func (q *userQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, `INSERT INTO "user"`) {
		q.inserted++
		return userRow{id: "user-new", name: args[1].(string)}
	}
	if q.bySfID {
		return userRow{id: "user-1", name: "jane@acme.com", sf: q.userSf}
	}
	var best *emailUserMatch
	for i := range q.sfRows {
		m := q.sfRows[i]
		if m.sfID != args[0].(string) {
			continue
		}
		if best == nil || (m.active && !best.active) || (m.active == best.active && m.createdOn.Before(best.createdOn)) {
			best = &m
		}
	}
	if best != nil {
		return userRow{id: best.id, name: best.userName, sf: best.sfID}
	}
	return userRow{}
}

func (q *userQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execs = append(q.execs, recordedExec{sql: sql, args: args})
	return pgconn.CommandTag{}, nil
}

func (q *userQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if len(q.emailRows) > 0 {
		return &userRows{rows: q.emailRows}, nil
	}
	if q.byEmail {
		return &userRows{rows: []emailUserMatch{{id: "user-1", userName: "jane@acme.com", sfID: q.userSf, active: true}}}, nil
	}
	return &userRows{}, nil
}

// userRows is the smallest pgx.Rows that can hand back the email matches.
type userRows struct {
	rows []emailUserMatch
	i    int
}

func (r *userRows) Next() bool { r.i++; return r.i <= len(r.rows) }
func (r *userRows) Scan(dest ...any) error {
	m := r.rows[r.i-1]
	*dest[0].(*string), *dest[1].(*string), *dest[2].(*string) = m.id, m.userName, m.sfID
	*dest[3].(*bool), *dest[4].(*time.Time) = m.active, m.createdOn
	return nil
}
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

// userUpdate returns the UPDATE "user" statement the test's querier recorded.
func userUpdate(t *testing.T, q *userQuerier) recordedExec {
	t.Helper()
	for _, e := range q.execs {
		if strings.Contains(e.sql, `UPDATE "user"`) {
			return e
		}
	}
	t.Fatal("no UPDATE \"user\" was run")
	return recordedExec{}
}

func resolveTestUser(t *testing.T, q *userQuerier) (id, userName string, created bool, err error) {
	t.Helper()
	in := domain.SalesforceMembershipUpsert{ContactSfID: "003NEW", ContactEmail: " Jane.Doe@Example.com ", ContactName: "Jane Doe"}
	return upsertMembershipUser(context.Background(), q, in, "test")
}

// TestUpsertMembershipUser_MatchesBySfIDFirst: a row carrying the contact id
// wins even when other rows share the email; the email is not consulted.
func TestUpsertMembershipUser_MatchesBySfIDFirst(t *testing.T) {
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	q := &userQuerier{
		sfRows: []emailUserMatch{{id: "user-b", userName: "uuid-b", sfID: "003NEW", active: false, createdOn: old}},
		emailRows: []emailUserMatch{
			{id: "user-a", userName: "jane.doe@example.com", active: true, createdOn: old},
			{id: "user-b", userName: "uuid-b", sfID: "003NEW", createdOn: old},
		},
	}
	id, name, created, err := resolveTestUser(t, q)
	if err != nil || created || id != "user-b" || name != "uuid-b" {
		t.Fatalf("id = %q, name = %q, created = %v, err = %v; want user-b by sf_id", id, name, created, err)
	}
	for _, e := range q.execs {
		if strings.Contains(e.sql, "pg_advisory_xact_lock") {
			t.Errorf("the email lock is only for the email path, got %+v", e)
		}
	}
}

// TestUpsertMembershipUser_SingleEmailMatchBackfillsSfID: one row for the
// email is used, and its empty sf_id is stamped with the new contact id.
func TestUpsertMembershipUser_SingleEmailMatchBackfillsSfID(t *testing.T) {
	q := &userQuerier{emailRows: []emailUserMatch{{id: "user-a", userName: "jane.doe@example.com", active: true}}}
	id, name, created, err := resolveTestUser(t, q)
	if err != nil || created || id != "user-a" || name != "jane.doe@example.com" {
		t.Fatalf("id = %q, name = %q, created = %v, err = %v", id, name, created, err)
	}
	up := userUpdate(t, q)
	if up.args[0] != "user-a" || up.args[6] != "003NEW" || up.args[4] != "jane.doe@example.com" {
		t.Errorf("update args = %v, want user-a stamped with 003NEW and the normalised email", up.args)
	}
}

// TestUpsertMembershipUser_MultipleEmailMatchesPickDeterministically: several
// rows for one email never fail the membership; the active row wins, then the
// one with a sf_id, then the oldest, whatever order the database returns them.
func TestUpsertMembershipUser_MultipleEmailMatchesPickDeterministically(t *testing.T) {
	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(24 * time.Hour)
	cases := map[string]struct {
		rows []emailUserMatch
		want string
	}{
		"active beats sf_id and age": {[]emailUserMatch{
			{id: "old-sf-inactive", userName: "u1", sfID: "003OLD", active: false, createdOn: t1},
			{id: "new-nosf-active", userName: "u2", active: true, createdOn: t2},
		}, "new-nosf-active"},
		"sf_id beats age among active": {[]emailUserMatch{
			{id: "old-nosf", userName: "u1", active: true, createdOn: t1},
			{id: "new-sf", userName: "u2", sfID: "003OLD", active: true, createdOn: t2},
		}, "new-sf"},
		"oldest breaks the tie": {[]emailUserMatch{
			{id: "newer", userName: "u1", sfID: "003A", active: true, createdOn: t2},
			{id: "older", userName: "u2", sfID: "003B", active: true, createdOn: t1},
		}, "older"},
		"id breaks a full tie": {[]emailUserMatch{
			{id: "b", userName: "u1", active: true, createdOn: t1},
			{id: "a", userName: "u2", active: true, createdOn: t1},
		}, "a"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, rows := range [][]emailUserMatch{tc.rows, {tc.rows[1], tc.rows[0]}} {
				q := &userQuerier{emailRows: rows}
				id, _, created, err := resolveTestUser(t, q)
				if err != nil || created || id != tc.want {
					t.Fatalf("id = %q, created = %v, err = %v; want %q", id, created, err, tc.want)
				}
				if up := userUpdate(t, q); up.args[0] != tc.want || up.args[6] != "003NEW" {
					t.Errorf("update args = %v, want %s stamped with 003NEW", up.args, tc.want)
				}
			}
		})
	}
}

// TestUpsertMembershipUser_ChosenRowReplacedContactReactivates: picking
// among duplicates reuses the replaced-contact rule: the chosen row's old,
// different sf_id marks a replacement and reactivates it.
func TestUpsertMembershipUser_ChosenRowReplacedContactReactivates(t *testing.T) {
	q := &userQuerier{emailRows: []emailUserMatch{
		{id: "user-b", userName: "uuid-b", sfID: "003OLD", active: false, createdOn: time.Now()},
		{id: "user-a", userName: "jane.doe@example.com", sfID: "003OTHER", active: false, createdOn: time.Now().Add(-time.Hour)},
	}}
	if _, _, _, err := resolveTestUser(t, q); err != nil {
		t.Fatal(err)
	}
	if got := userUpdate(t, q).args[8]; got != true {
		t.Errorf("reactivate = %v, want true", got)
	}
}

// TestUpsertMembershipUser_NoMatchCreates: with no row by sf_id or email a
// user is inserted, after the email lock.
func TestUpsertMembershipUser_NoMatchCreates(t *testing.T) {
	q := &userQuerier{}
	id, name, created, err := resolveTestUser(t, q)
	if err != nil || !created || id != "user-new" || name != "jane.doe@example.com" || q.inserted != 1 {
		t.Fatalf("id = %q, name = %q, created = %v, inserted = %d, err = %v", id, name, created, q.inserted, err)
	}
	if len(q.execs) == 0 || !strings.Contains(q.execs[0].sql, "pg_advisory_xact_lock") ||
		q.execs[0].args[0] != "user-email|jane.doe@example.com" {
		t.Errorf("execs = %+v, want the email lock first", q.execs)
	}
}

// TestUpsertMembershipUser_CreateDoesNotDuplicateAnExistingEmail: a new
// contact id that matches no sf_id still finds the person by email, however
// the existing row is spelled, and inserts nothing.
func TestUpsertMembershipUser_CreateDoesNotDuplicateAnExistingEmail(t *testing.T) {
	q := &userQuerier{emailRows: []emailUserMatch{{id: "user-b", userName: "uuid-b", sfID: "003OLD", active: false}}}
	id, _, created, err := resolveTestUser(t, q)
	if err != nil || created || id != "user-b" || q.inserted != 0 {
		t.Fatalf("id = %q, created = %v, inserted = %d, err = %v; want user-b reused", id, created, q.inserted, err)
	}
}
