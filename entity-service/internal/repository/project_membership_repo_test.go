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
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---- a querier standing in for the transaction ---------------------------

type recordedExec struct {
	sql  string
	args []any
}

// adminQuerier answers syncDerivedAdminRole's aggregate query from an
// in-memory fixture (the memberships the user holds, each with the project
// groups on it) and records every write, so the grant/revoke decision can be
// exercised without a database.
type adminQuerier struct {
	// memberships is one entry per membership the user holds AFTER the
	// membership being processed has been written -- which is the whole point
	// of the rule: the answer is computed from all of them, not from the one
	// in hand.
	memberships []fixtureMembership
	queries     []string
	queryArgs   [][]any
	execs       []recordedExec
	scanErr     error
}

type fixtureMembership struct {
	state string
	admin bool
	// partner marks a membership whose contact belongs to an account other
	// than the project's -- the SQL's `ac.account_id <> p.account_id`. Only
	// such a membership can support partner_admin; the rest support
	// customer_admin.
	partner bool
	userID  string
}

type twoBoolRow struct {
	customerAdmin bool
	partnerAdmin  bool
	err           error
}

func (r twoBoolRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 2 {
		return errors.New("unexpected scan destination")
	}
	c, ok := dest[0].(*bool)
	if !ok {
		return errors.New("unexpected scan destination")
	}
	p, ok := dest[1].(*bool)
	if !ok {
		return errors.New("unexpected scan destination")
	}
	*c, *p = r.customerAdmin, r.partnerAdmin
	return nil
}

func (q *adminQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.queries = append(q.queries, sql)
	q.queryArgs = append(q.queryArgs, args)
	userID, _ := args[0].(string)
	row := twoBoolRow{err: q.scanErr}
	for _, m := range q.memberships {
		if m.userID != userID || !m.admin || m.state == "DEACTIVATED" {
			continue
		}
		if m.partner {
			row.partnerAdmin = true
		} else {
			row.customerAdmin = true
		}
	}
	return row
}

func (q *adminQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execs = append(q.execs, recordedExec{sql: sql, args: args})
	return pgconn.CommandTag{}, nil
}

func (q *adminQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("Query is not used by syncDerivedAdminRole")
}

func (q *adminQuerier) grantedRoles() []string {
	var out []string
	for _, e := range q.execs {
		if strings.Contains(e.sql, "INSERT INTO user_role") {
			out = append(out, e.args[2].(string))
		}
	}
	return out
}

func (q *adminQuerier) revokedRoles() []string {
	for _, e := range q.execs {
		if strings.Contains(e.sql, "DELETE FROM user_role") {
			if roles, ok := e.args[1].([]string); ok {
				return roles
			}
		}
	}
	return nil
}

// ---- the rule ------------------------------------------------------------

// TestSyncDerivedAdminRole covers the account-level rule that replaced the
// revoke bug: admin is a project role, and the account-level role is derived
// from EVERY membership the user holds -- so processing one non-admin
// membership can no longer strip an admin who still holds it elsewhere.
func TestSyncDerivedAdminRole(t *testing.T) {
	const user = "user-1"
	cases := []struct {
		name        string
		memberships []fixtureMembership
		adminRole   string
		isCsAdmin   bool
		wantAdmin   bool
		wantGrant   []string
		wantRevoke  []string
	}{
		{
			name:        "admin on the only project grants the account role",
			memberships: []fixtureMembership{{userID: user, state: "INVITED", admin: true}},
			adminRole:   globalRoleCustomerAdminName,
			wantAdmin:   true,
			wantGrant:   []string{globalRoleCustomerAdminName},
			wantRevoke:  []string{globalRolePartnerAdminName},
		},
		{
			name: "admin on ANY project under the account is enough",
			memberships: []fixtureMembership{
				{userID: user, state: "REGISTERED", admin: false},
				{userID: user, state: "REGISTERED", admin: true},
			},
			adminRole:  globalRoleCustomerAdminName,
			wantAdmin:  true,
			wantGrant:  []string{globalRoleCustomerAdminName},
			wantRevoke: []string{globalRolePartnerAdminName},
		},
		{
			name: "losing admin on one project keeps it while another still grants it",
			memberships: []fixtureMembership{
				// The membership just processed lost its Admin role...
				{userID: user, state: "REGISTERED", admin: false},
				// ...but this one still carries it, so the account role stays.
				{userID: user, state: "INVITED", admin: true},
			},
			adminRole:  globalRoleCustomerAdminName,
			wantAdmin:  true,
			wantGrant:  []string{globalRoleCustomerAdminName},
			wantRevoke: []string{globalRolePartnerAdminName},
		},
		{
			name: "no admin on any live membership revokes both admin roles",
			memberships: []fixtureMembership{
				{userID: user, state: "REGISTERED", admin: false},
				{userID: user, state: "INVITED", admin: false},
			},
			adminRole:  globalRoleCustomerAdminName,
			wantAdmin:  false,
			wantRevoke: []string{globalRoleCustomerAdminName, globalRolePartnerAdminName},
		},
		{
			name:        "a deactivated membership's admin role does not count",
			memberships: []fixtureMembership{{userID: user, state: "DEACTIVATED", admin: true}},
			adminRole:   globalRoleCustomerAdminName,
			wantAdmin:   false,
			wantRevoke:  []string{globalRoleCustomerAdminName, globalRolePartnerAdminName},
		},
		{
			name:        "the contact's own isCsAdmin flag grants it with no admin membership",
			memberships: []fixtureMembership{{userID: user, state: "REGISTERED", admin: false}},
			adminRole:   globalRoleCustomerAdminName,
			isCsAdmin:   true,
			wantAdmin:   true,
			wantGrant:   []string{globalRoleCustomerAdminName},
			wantRevoke:  []string{globalRolePartnerAdminName},
		},
		{
			name:        "a partner contact's ADMIN membership grants partner_admin, and customer_admin is revoked",
			memberships: []fixtureMembership{{userID: user, state: "INVITED", admin: true, partner: true}},
			adminRole:   globalRolePartnerAdminName,
			wantAdmin:   true,
			wantGrant:   []string{globalRolePartnerAdminName},
			wantRevoke:  []string{globalRoleCustomerAdminName},
		},
		{
			// The cross-account case. Admin on their OWN account's project
			// earns customer_admin; the partner membership being processed
			// carries no ADMIN role, so it earns nothing -- and must not
			// trade the role they did earn for one they did not.
			name: "a non-admin partner membership neither grants partner_admin nor revokes an earned customer_admin",
			memberships: []fixtureMembership{
				{userID: user, state: "REGISTERED", admin: true},
				{userID: user, state: "INVITED", admin: false, partner: true},
			},
			adminRole:  globalRolePartnerAdminName,
			wantAdmin:  false,
			wantGrant:  []string{globalRoleCustomerAdminName},
			wantRevoke: []string{globalRolePartnerAdminName},
		},
		{
			// Both are earned on their own evidence, so both are held.
			name: "admin on both an own and a partner project holds both roles",
			memberships: []fixtureMembership{
				{userID: user, state: "REGISTERED", admin: true},
				{userID: user, state: "REGISTERED", admin: true, partner: true},
			},
			adminRole: globalRoleCustomerAdminName,
			wantAdmin: true,
			wantGrant: []string{globalRoleCustomerAdminName, globalRolePartnerAdminName},
		},
		{
			name:        "an integration user has no admin role to decide at all",
			memberships: []fixtureMembership{{userID: user, state: "INVITED", admin: true}},
			adminRole:   "",
			wantAdmin:   false,
		},
	}

	managed := []string{globalRoleCustomerAdminName, globalRolePartnerAdminName}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &adminQuerier{memberships: tc.memberships}
			got, err := syncDerivedAdminRole(context.Background(), q, user, tc.adminRole, managed, tc.isCsAdmin, "test-actor")
			if err != nil {
				t.Fatalf("syncDerivedAdminRole: %v", err)
			}
			if got != tc.wantAdmin {
				t.Errorf("isAdmin = %v, want %v", got, tc.wantAdmin)
			}
			if grant := q.grantedRoles(); !equalStrings(grant, tc.wantGrant) {
				t.Errorf("granted = %v, want %v", grant, tc.wantGrant)
			}
			if revoke := q.revokedRoles(); !equalStrings(revoke, tc.wantRevoke) {
				t.Errorf("revoked = %v, want %v", revoke, tc.wantRevoke)
			}
			if tc.adminRole == "" && len(q.queries) != 0 {
				t.Error("an integration user must not even be queried")
			}
		})
	}
}

// TestSyncDerivedAdminRoleQueriesEveryMembership pins the shape of the rule's
// one query: a single aggregate over the USER's memberships, keyed only on
// the user id, splitting them by whether the contact's account is the
// project's. Binding the membership being processed would reintroduce the bug
// this replaced -- the decision must not depend on which membership happens
// to be in hand.
func TestSyncDerivedAdminRoleQueriesEveryMembership(t *testing.T) {
	q := &adminQuerier{memberships: []fixtureMembership{{userID: "user-1", state: "INVITED", admin: true}}}
	if _, err := syncDerivedAdminRole(context.Background(), q, "user-1", globalRoleCustomerAdminName,
		[]string{globalRoleCustomerAdminName, globalRolePartnerAdminName}, false, "test-actor"); err != nil {
		t.Fatal(err)
	}
	if len(q.queries) != 1 {
		t.Fatalf("queries = %d, want exactly 1", len(q.queries))
	}
	sql := q.queries[0]
	for _, want := range []string{
		"bool_or", "ac.account_id = p.account_id", "ac.account_id <> p.account_id",
		"project_contact", "project_contact_group", "project_role", "ADMIN", "DEACTIVATED",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("query must mention %q:\n%s", want, sql)
		}
	}
	if args := q.queryArgs[0]; len(args) != 1 || args[0] != "user-1" {
		t.Errorf("query args = %v, want only the user id", args)
	}
}

// TestSyncDerivedAdminRolePropagatesQueryFailure asserts a failed derivation
// aborts the write rather than silently deciding "not an admin", which would
// revoke a real admin's role on a transient error.
func TestSyncDerivedAdminRolePropagatesQueryFailure(t *testing.T) {
	q := &adminQuerier{scanErr: errors.New("boom")}
	if _, err := syncDerivedAdminRole(context.Background(), q, "user-1", globalRoleCustomerAdminName,
		[]string{globalRoleCustomerAdminName}, false, "test-actor"); err == nil {
		t.Fatal("want an error")
	}
	if len(q.execs) != 0 {
		t.Error("no role may be granted or revoked when the derivation failed")
	}
}

// The two role names, aliased to the repository's own constants so a rename
// cannot leave the fixtures testing yesterday's vocabulary.
const (
	globalRoleCustomerAdminName = globalRoleCustomerAdmin
	globalRolePartnerAdminName  = globalRolePartnerAdmin
)

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
