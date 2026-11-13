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
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	coMemberProjectA = "00000000-0000-4000-8000-00000000000a"
	coMemberProjectB = "00000000-0000-4000-8000-00000000000b"
)

// contactSearchFixture is a userService whose repository records which of the
// two searches it was asked for, so a test can prove a customer never reaches
// the directory-wide one.
type contactSearchFixture struct {
	svc            UserService
	directoryCalls int
	contactCalls   int
	gotEmails      []string
	gotProjectIDs  []string
	gotFilters     domain.SearchUsersFilters
}

func newContactSearchFixture(access AccessService, contacts []domain.UserContact) *contactSearchFixture {
	f := &contactSearchFixture{}
	repo := stubUserRepo{
		searchUsers: func(_ context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error) {
			f.directoryCalls++
			f.gotFilters = req.Filters
			return []domain.User{{ID: "u1", Email: "jane.doe@example.com"}}, 1, nil
		},
		searchUserContacts: func(_ context.Context, emails, projectIDs []string) ([]domain.UserContact, error) {
			f.contactCalls++
			f.gotEmails = emails
			f.gotProjectIDs = projectIDs
			return contacts, nil
		},
	}
	f.svc = NewUserService(repo, access)
	return f
}

func coMemberCustomerAccess(projectIDs ...string) AccessService {
	return stubAccess{scope: AccessScope{ProjectIDs: projectIDs, ViewerEmail: "customer@example.com"}}
}

func requireContactSearchValidation(t *testing.T, err error, wantSubstr string) {
	t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if !strings.Contains(ve.Msg, wantSubstr) {
		t.Fatalf("ValidationError %q does not mention %q", ve.Msg, wantSubstr)
	}
}

func TestUserService_SearchUsers_CustomerLooksUpCoMembersByEmail(t *testing.T) {
	f := newContactSearchFixture(coMemberCustomerAccess(coMemberProjectA, coMemberProjectB), []domain.UserContact{
		{ID: "u1", Email: "jane.doe@example.com", Name: "Jane Doe"},
	})
	resp, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{
		Filters: domain.SearchUsersFilters{Emails: []string{" Jane.Doe@example.com ", "john.roe@example.com"}},
	})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if f.directoryCalls != 0 {
		t.Fatal("a customer reached the directory-wide search")
	}
	if want := []string{"Jane.Doe@example.com", "john.roe@example.com"}; !reflect.DeepEqual(f.gotEmails, want) {
		t.Errorf("emails = %v, want trimmed %v", f.gotEmails, want)
	}
	if want := []string{coMemberProjectA, coMemberProjectB}; !reflect.DeepEqual(f.gotProjectIDs, want) {
		t.Errorf("projectIDs = %v, want the caller's own projects %v", f.gotProjectIDs, want)
	}
	if resp.Total != 1 || len(resp.Contacts) != 1 || resp.Limit != 10 || resp.Offset != 0 || resp.HasMore {
		t.Errorf("envelope = %+v", resp)
	}
}

func TestUserService_SearchUsers_CustomerResponseCarriesOnlyIDEmailName(t *testing.T) {
	f := newContactSearchFixture(coMemberCustomerAccess(coMemberProjectA), []domain.UserContact{
		{ID: "u1", Email: "jane.doe@example.com", Name: "Jane Doe"},
	})
	resp, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{
		Filters: domain.SearchUsersFilters{Emails: []string{"jane.doe@example.com"}},
	})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	body, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"users":[{"id":"u1","email":"jane.doe@example.com","name":"Jane Doe"}],"total":1,"limit":10,"offset":0,"hasMore":false}`
	if string(body) != want {
		t.Fatalf("body = %s\nwant   %s", body, want)
	}
	for _, key := range []string{"phone", "timezone", "roles", "userType", "userName", "firstName", "lastName"} {
		if strings.Contains(string(body), `"`+key+`"`) {
			t.Errorf("non-internal response carries %q", key)
		}
	}
}

func TestUserService_SearchUsers_CustomerNoMatchIsEmptyArrayNotNull(t *testing.T) {
	f := newContactSearchFixture(coMemberCustomerAccess(coMemberProjectA), nil)
	resp, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{
		Filters: domain.SearchUsersFilters{Emails: []string{"nobody@example.com"}},
	})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	body, _ := json.Marshal(resp)
	if !strings.HasPrefix(string(body), `{"users":[],"total":0`) {
		t.Fatalf("body = %s, want an empty users array", body)
	}
}

func TestUserService_SearchUsers_CustomerRefusesEveryOtherFilter(t *testing.T) {
	yes := true
	tests := map[string]domain.SearchUsersFilters{
		"searchQuery":                {Emails: []string{"jane.doe@example.com"}, SearchQuery: "jane"},
		"roleIds":                    {Emails: []string{"jane.doe@example.com"}, RoleIDs: []domain.UserRole{"admin"}},
		"userNames":                  {Emails: []string{"jane.doe@example.com"}, UserNames: []string{"jane"}},
		"userIds":                    {Emails: []string{"jane.doe@example.com"}, UserIDs: []string{coMemberProjectA}},
		"groupIds":                   {Emails: []string{"jane.doe@example.com"}, GroupIDs: []string{coMemberProjectA}},
		"teamIds":                    {Emails: []string{"jane.doe@example.com"}, TeamIDs: []string{coMemberProjectA}},
		"groupNames":                 {Emails: []string{"jane.doe@example.com"}, GroupNames: []string{"team"}},
		"active":                     {Emails: []string{"jane.doe@example.com"}, Active: &yes},
		"userType":                   {Emails: []string{"jane.doe@example.com"}, UserType: domain.UserTypeInternal},
		"no filter":                  {},
		"searchQuery without emails": {SearchQuery: "a"},
	}
	for name, filters := range tests {
		t.Run(name, func(t *testing.T) {
			f := newContactSearchFixture(coMemberCustomerAccess(coMemberProjectA), nil)
			_, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{Filters: filters})
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
			if f.directoryCalls+f.contactCalls != 0 {
				t.Fatal("the repository was reached for a refused request")
			}
		})
	}
	// The refusal says why.
	f := newContactSearchFixture(coMemberCustomerAccess(coMemberProjectA), nil)
	_, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{Filters: domain.SearchUsersFilters{SearchQuery: "x"}})
	requireContactSearchValidation(t, err, "only look up users by email")
}

func TestUserService_SearchUsers_CustomerEmailCountBounds(t *testing.T) {
	many := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("user%d@example.com", i)
		}
		return out
	}
	f := newContactSearchFixture(coMemberCustomerAccess(coMemberProjectA), nil)
	if _, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{Filters: domain.SearchUsersFilters{Emails: many(50)}}); err != nil {
		t.Fatalf("50 emails: %v", err)
	}
	_, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{Filters: domain.SearchUsersFilters{Emails: many(51)}})
	requireContactSearchValidation(t, err, "more than 50")
	_, err = f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{Filters: domain.SearchUsersFilters{Emails: []string{"  "}}})
	requireContactSearchValidation(t, err, "empty")
	if f.contactCalls != 1 {
		t.Errorf("contact lookups = %d, want only the valid 50-email request", f.contactCalls)
	}
}

func TestUserService_SearchUsers_CustomerPaginatesContacts(t *testing.T) {
	contacts := []domain.UserContact{{ID: "1"}, {ID: "2"}, {ID: "3"}}
	f := newContactSearchFixture(coMemberCustomerAccess(coMemberProjectA), contacts)
	resp, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{
		Pagination: domain.Pagination{Limit: 2, Offset: 1},
		Filters:    domain.SearchUsersFilters{Emails: []string{"a@example.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Contacts) != 2 || resp.Contacts[0].ID != "2" || resp.Total != 3 || resp.HasMore {
		t.Errorf("resp = %+v", resp)
	}
	resp, _ = f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{
		Pagination: domain.Pagination{Limit: 1},
		Filters:    domain.SearchUsersFilters{Emails: []string{"a@example.com"}},
	})
	if !resp.HasMore || len(resp.Contacts) != 1 {
		t.Errorf("first page = %+v, want hasMore", resp)
	}
}

func TestUserService_SearchUsers_UnresolvedIdentityIsRefusedNotServerError(t *testing.T) {
	for name, resolveErr := range map[string]error{
		"forbidden":    &apierror.ForbiddenError{Msg: "no active user"},
		"unauthorized": &apierror.UnauthorizedError{Msg: "no identity"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newContactSearchFixture(stubAccess{err: resolveErr}, nil)
			_, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{
				Filters: domain.SearchUsersFilters{Emails: []string{"jane.doe@example.com"}},
			})
			if !errors.Is(err, resolveErr) {
				t.Fatalf("err = %v, want the resolver's own refusal %v", err, resolveErr)
			}
			if f.directoryCalls+f.contactCalls != 0 {
				t.Fatal("the repository was reached with an unresolved identity")
			}
		})
	}
}

func TestUserService_SearchUsers_NoAccessServiceFailsClosed(t *testing.T) {
	f := newContactSearchFixture(nil, nil)
	_, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{})
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) || f.directoryCalls != 0 {
		t.Fatalf("err = %v, directoryCalls = %d, want Forbidden and no repository call", err, f.directoryCalls)
	}
}

func TestUserService_SearchUsers_InternalCallerIsUnchanged(t *testing.T) {
	f := newContactSearchFixture(stubAccess{scope: AccessScope{Unrestricted: true}}, nil)
	filters := domain.SearchUsersFilters{SearchQuery: "jane", RoleIDs: []domain.UserRole{"admin"}, UserType: domain.UserTypeInternal}
	resp, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{Filters: filters})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if f.contactCalls != 0 || f.directoryCalls != 1 {
		t.Fatalf("directoryCalls = %d, contactCalls = %d", f.directoryCalls, f.contactCalls)
	}
	if !reflect.DeepEqual(f.gotFilters, filters) {
		t.Errorf("filters = %+v, want them forwarded untouched", f.gotFilters)
	}
	if len(resp.Users) != 1 || resp.Contacts != nil {
		t.Errorf("resp = %+v, want the full user shape", resp)
	}
	body, _ := json.Marshal(resp)
	if !strings.Contains(string(body), `"phone"`) || !strings.Contains(string(body), `"roles"`) {
		t.Errorf("internal body lost its full user shape: %s", body)
	}
}

func TestUserService_SearchUsers_UserTypeFilterValidation(t *testing.T) {
	for _, tc := range []struct {
		in      domain.UserType
		wantErr bool
	}{
		{"", false}, {domain.UserTypeInternal, false}, {domain.UserTypeCustomer, false},
		{domain.UserTypeSystem, false}, {domain.UserTypeExternal, false}, {"INTERNAL", true}, {"staff", true},
	} {
		f := newContactSearchFixture(stubAccess{scope: AccessScope{Unrestricted: true}}, nil)
		_, err := f.svc.SearchUsers(context.Background(), domain.SearchUsersRequest{Filters: domain.SearchUsersFilters{UserType: tc.in}})
		if tc.wantErr {
			requireContactSearchValidation(t, err, "userType")
			if f.directoryCalls != 0 {
				t.Errorf("%q: repository reached", tc.in)
			}
		} else if err != nil {
			t.Errorf("%q: %v", tc.in, err)
		}
	}
}
