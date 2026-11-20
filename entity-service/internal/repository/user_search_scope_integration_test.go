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

// The POST /users/search scoping against a live PostgreSQL: who may search the
// directory, what a customer may resolve (co-members by email, three fields),
// and that the userType filter reaches the real enum column. The caller is
// resolved by the real access service over the real access repository, so
// "shares a project" is the rule production applies, not a stub's. Skipped
// unless ENTITY_TEST_DATABASE_URL is set; apply every migration first (see
// schedule_repo_integration_test.go):
//
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run UserSearchScope ./internal/repository/

package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// Fixture ids, distinct from every other integration file's.
const (
	usAccount  = "75e5ea00-0000-4000-8000-0000000000a0"
	usProjectA = "75e5ea00-0000-4000-8000-0000000000b1"
	usProjectB = "75e5ea00-0000-4000-8000-0000000000b2"

	usCaller   = "75e5ea00-0000-4000-8000-0000000000c1"
	usCoMember = "75e5ea00-0000-4000-8000-0000000000c2"
	usOutsider = "75e5ea00-0000-4000-8000-0000000000c3"
	usInvited  = "75e5ea00-0000-4000-8000-0000000000c4"
	usStaff    = "75e5ea00-0000-4000-8000-0000000000c5"

	usCallerEmail   = "ussearch.caller@example.com"
	usCoMemberEmail = "ussearch.comember@example.com"
	usOutsiderEmail = "ussearch.outsider@example.com"
	usInvitedEmail  = "ussearch.invited@example.com"
	usStaffEmail    = "ussearch.staff@example.com"

	usPortalClient = "us-customer-portal-client"
	usM2MClient    = "us-m2m-client"
)

var usUsers = []string{usCaller, usCoMember, usOutsider, usInvited, usStaff}

func usExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("seed (%.60s): %v", sql, err)
	}
}

func usRemoveFixtures(pool *pgxpool.Pool) {
	ctx := context.Background()
	for _, q := range []string{
		`DELETE FROM project_contact WHERE project_id IN ('` + usProjectA + `','` + usProjectB + `')`,
		`DELETE FROM account_contact WHERE account_id = '` + usAccount + `'`,
		`DELETE FROM project WHERE id IN ('` + usProjectA + `','` + usProjectB + `')`,
		`DELETE FROM account WHERE id = '` + usAccount + `'`,
		`DELETE FROM "user" WHERE id = ANY(ARRAY['` + strings.Join(usUsers, `','`) + `']::uuid[])`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			panic("user search fixture cleanup: " + err.Error())
		}
	}
}

// newUserSearchScope connects and rebuilds this file's fixtures: two projects;
// the caller, a REGISTERED co-member and an INVITED contact on project A; an
// outsider on project B only; and a staff user. Mixed-case email on the
// co-member proves the match ignores case.
func newUserSearchScope(t *testing.T) service.UserService {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	usRemoveFixtures(pool)
	t.Cleanup(func() { usRemoveFixtures(pool) })

	usExec(t, pool, `INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number)
		VALUES ($1, now(), now(), 't', 't', 'User Search Account', 'ACC-USSEARCH')`, usAccount)
	for i, p := range []string{usProjectA, usProjectB} {
		usExec(t, pool, `INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, account_id)
			VALUES ($1, now(), now(), 't', 't', $2, $3)`, p, fmt.Sprintf("USSEARCH-%d", i), usAccount)
	}

	seedUser := func(id, email, userType, name, phone string) {
		usExec(t, pool, `INSERT INTO "user" (id, created_on, updated_on, user_name, name, first_name, last_name, email, is_active, user_type, phone)
			VALUES ($1, now(), now(), lower($2), NULLIF($3, ''), 'First', 'Last', $2, TRUE, $4::user_type_enum, $5)`,
			id, email, name, userType, phone)
		// An insert fires the trigger that derives user_type from role
		// membership (migration 0011) and these users hold no role, so pin the
		// type afterwards; updating user_type itself fires nothing.
		usExec(t, pool, `UPDATE "user" SET user_type = $2::user_type_enum WHERE id = $1`, id, userType)
	}
	seedUser(usCaller, usCallerEmail, "EXTERNAL", "Jane Caller", "+15550100001")
	seedUser(usCoMember, "UsSearch.CoMember@Example.com", "EXTERNAL", "John Roe", "+15550100002")
	seedUser(usOutsider, usOutsiderEmail, "EXTERNAL", "Outside Person", "+15550100003")
	seedUser(usInvited, usInvitedEmail, "EXTERNAL", "", "+15550100004")
	seedUser(usStaff, usStaffEmail, "INTERNAL", "Staff Person", "+15550100005")

	contactSeq := 0
	seedContact := func(project, email, state string) {
		contactSeq++
		acID := fmt.Sprintf("75e5ea00-0000-4000-8000-0000000001%02d", contactSeq)
		usExec(t, pool, `INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
			VALUES ($1, now(), now(), 't', 't', $2, $3)`, acID, email, usAccount)
		usExec(t, pool, `INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, state, account_contact_id, project_id)
			VALUES (gen_random_uuid(), now(), now(), 't', 't', $1, $2::project_contact_state_enum, $3, $4)`,
			email, state, acID, project)
	}
	seedContact(usProjectA, usCallerEmail, "REGISTERED")
	seedContact(usProjectA, strings.ToLower("UsSearch.CoMember@Example.com"), "REGISTERED")
	seedContact(usProjectA, usInvitedEmail, "INVITED")
	seedContact(usProjectB, usOutsiderEmail, "REGISTERED")

	access := service.NewAccessService(repository.NewAccessRepository(pool), service.AccessClientConfig{
		M2MClientIDs:                  map[string]bool{usM2MClient: true},
		CustomerPortalBackendClientID: usPortalClient,
	})
	return service.NewUserService(repository.NewUserRepository(pool), access)
}

func usCustomerCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: usCallerEmail, ClientID: usPortalClient})
}

func usStaffCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: usStaffEmail})
}

func usM2MCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: usM2MClient})
}

func usEmailReq(emails ...string) domain.SearchUsersRequest {
	return domain.SearchUsersRequest{Filters: domain.SearchUsersFilters{Emails: emails}}
}

// usBody is the response exactly as it goes on the wire.
func usBody(t *testing.T, resp domain.SearchUsersResponse) map[string]any {
	t.Helper()
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func usEmails(t *testing.T, body map[string]any) []string {
	t.Helper()
	var out []string
	for _, u := range body["users"].([]any) {
		out = append(out, u.(map[string]any)["email"].(string))
	}
	slices.Sort(out)
	return out
}

func TestUserSearchScopeIntegration_CustomerGetsCoMemberAsThreeFieldsOnly(t *testing.T) {
	svc := newUserSearchScope(t)
	resp, err := svc.SearchUsers(usCustomerCtx(), usEmailReq("ussearch.comember@example.com"))
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	body := usBody(t, resp)
	users := body["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("users = %v, want the one co-member", users)
	}
	got := users[0].(map[string]any)
	want := map[string]any{"id": usCoMember, "email": "UsSearch.CoMember@Example.com", "name": "John Roe"}
	if len(got) != len(want) {
		t.Fatalf("user keys = %v, want exactly id, email, name", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
	raw, _ := json.Marshal(resp)
	for _, leak := range []string{"phone", "timezone", "roles", "userType", "+1555"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("customer response leaks %q: %s", leak, raw)
		}
	}
	if body["total"] != float64(1) || body["hasMore"] != false {
		t.Errorf("envelope = %v", body)
	}
}

func TestUserSearchScopeIntegration_NonCoMembersLookLikeNoSuchUser(t *testing.T) {
	svc := newUserSearchScope(t)
	for name, email := range map[string]string{
		"registered on another project only": usOutsiderEmail,
		"invited, not registered":            usInvitedEmail,
		"staff, not a contact":               usStaffEmail,
		"no such user at all":                "ussearch.nobody@example.com",
	} {
		resp, err := svc.SearchUsers(usCustomerCtx(), usEmailReq(email))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		raw, _ := json.Marshal(resp)
		if got := usEmails(t, usBody(t, resp)); len(got) != 0 {
			t.Errorf("%s: got %v, want no rows", name, got)
		}
		// Identical bytes to "no such user", so existence cannot be inferred.
		unknown, _ := svc.SearchUsers(usCustomerCtx(), usEmailReq("ussearch.nobody@example.com"))
		unknownRaw, _ := json.Marshal(unknown)
		if string(raw) != string(unknownRaw) {
			t.Errorf("%s: %s differs from the no-such-user answer %s", name, raw, unknownRaw)
		}
	}
}

func TestUserSearchScopeIntegration_MixedEmailsReturnOnlyCoMembers(t *testing.T) {
	svc := newUserSearchScope(t)
	resp, err := svc.SearchUsers(usCustomerCtx(), usEmailReq(
		usOutsiderEmail, "USSEARCH.COMEMBER@EXAMPLE.COM", usCallerEmail, usStaffEmail, "ussearch.nobody@example.com"))
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	want := []string{"UsSearch.CoMember@Example.com", usCallerEmail}
	slices.Sort(want)
	if got := usEmails(t, usBody(t, resp)); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v (the caller and the co-member, matched ignoring case)", got, want)
	}
}

func TestUserSearchScopeIntegration_CustomerBroadFiltersAreRefused(t *testing.T) {
	svc := newUserSearchScope(t)
	many := make([]string, 51)
	for i := range many {
		many[i] = fmt.Sprintf("ussearch.%d@example.com", i)
	}
	for name, filters := range map[string]domain.SearchUsersFilters{
		"searchQuery":    {SearchQuery: "ussearch"},
		"roleIds":        {RoleIDs: []domain.UserRole{"admin"}},
		"emails+query":   {Emails: []string{usCoMemberEmail}, SearchQuery: "ussearch"},
		"emails+roleIds": {Emails: []string{usCoMemberEmail}, RoleIDs: []domain.UserRole{"admin"}},
		"emails+type":    {Emails: []string{usCoMemberEmail}, UserType: domain.UserTypeInternal},
		"51 emails":      {Emails: many},
		"nothing":        {},
	} {
		_, err := svc.SearchUsers(usCustomerCtx(), domain.SearchUsersRequest{Filters: filters})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want a 400 ValidationError", name, err)
		}
	}
}

func TestUserSearchScopeIntegration_UnresolvedIdentityIsForbidden(t *testing.T) {
	svc := newUserSearchScope(t)
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "ussearch.stranger@example.com", ClientID: usPortalClient})
	_, err := svc.SearchUsers(ctx, usEmailReq(usCoMemberEmail))
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("err = %v, want 403 ForbiddenError", err)
	}
}

func TestUserSearchScopeIntegration_InternalCallerKeepsFullDirectoryAndUserTypeFilter(t *testing.T) {
	svc := newUserSearchScope(t)
	all := []string{usCallerEmail, "UsSearch.CoMember@Example.com", usOutsiderEmail, usInvitedEmail, usStaffEmail}
	slices.Sort(all)

	// Free-text search, which a customer is refused, works and returns the full shape.
	resp, err := svc.SearchUsers(usStaffCtx(), domain.SearchUsersRequest{
		Pagination: domain.Pagination{Limit: 50},
		Filters:    domain.SearchUsersFilters{SearchQuery: "ussearch."},
	})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if got := usEmails(t, usBody(t, resp)); !slices.Equal(got, all) {
		t.Errorf("got %v, want %v", got, all)
	}
	var staff *domain.User
	for i := range resp.Users {
		if resp.Users[i].ID == usStaff {
			staff = &resp.Users[i]
		}
	}
	// Phone is deliberately absent: the list read never selected it.
	if staff == nil || staff.UserType != domain.UserTypeInternal || staff.Roles == nil || staff.UserName != usStaffEmail {
		t.Errorf("internal caller lost the full user shape: %+v", staff)
	}

	// userType narrows to the one type, for each accepted spelling.
	for ut, want := range map[domain.UserType][]string{
		domain.UserTypeInternal: {usStaffEmail},
		domain.UserTypeCustomer: {usCallerEmail, "UsSearch.CoMember@Example.com", usOutsiderEmail, usInvitedEmail},
		domain.UserTypeExternal: {usCallerEmail, "UsSearch.CoMember@Example.com", usOutsiderEmail, usInvitedEmail},
	} {
		slices.Sort(want)
		r, err := svc.SearchUsers(usStaffCtx(), domain.SearchUsersRequest{
			Pagination: domain.Pagination{Limit: 50},
			Filters:    domain.SearchUsersFilters{SearchQuery: "ussearch.", UserType: ut},
		})
		if err != nil {
			t.Fatalf("userType %q: %v", ut, err)
		}
		if got := usEmails(t, usBody(t, r)); !slices.Equal(got, want) {
			t.Errorf("userType %q: got %v, want %v", ut, got, want)
		}
	}

	// Without userType the request behaves exactly as before.
	r, err := svc.SearchUsers(usStaffCtx(), usEmailReq(usOutsiderEmail))
	if err != nil || len(r.Users) != 1 || r.Users[0].ID != usOutsider {
		t.Errorf("internal email lookup = %+v, %v", r, err)
	}
}

func TestUserSearchScopeIntegration_ServiceCredentialIsUnchanged(t *testing.T) {
	svc := newUserSearchScope(t)
	resp, err := svc.SearchUsers(usM2MCtx(), usEmailReq(usOutsiderEmail, usStaffEmail))
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if got := usEmails(t, usBody(t, resp)); !slices.Equal(got, []string{usOutsiderEmail, usStaffEmail}) {
		t.Errorf("got %v: a service credential must see users outside any project", got)
	}
	if len(resp.Users) != 2 || resp.Users[0].UserName == "" || resp.Users[0].Roles == nil {
		t.Errorf("service credential lost the full user shape: %+v", resp.Users)
	}
	// And free-text search is still open to it.
	r, err := svc.SearchUsers(usM2MCtx(), domain.SearchUsersRequest{Filters: domain.SearchUsersFilters{SearchQuery: "ussearch.staff"}})
	if err != nil || len(r.Users) != 1 {
		t.Errorf("service credential search = %+v, %v", r, err)
	}
}
