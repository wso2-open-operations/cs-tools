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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// internalCallerCtx is a request context for an internal user: a validated
// identity carrying email, with the Unrestricted scope the request
// middleware would have resolved and attached.
func internalCallerCtx(t *testing.T, email string) context.Context {
	t.Helper()
	return repository.WithCallerIdentity(contextWithUserIDToken(fakeJWTWithEmail(t, email)), AccessScope{Unrestricted: true})
}

// customerCallerCtx is internalCallerCtx for a registered customer contact of
// project "proj-1".
func customerCallerCtx(t *testing.T, email string) context.Context {
	t.Helper()
	return repository.WithCallerIdentity(contextWithUserIDToken(fakeJWTWithEmail(t, email)), AccessScope{ProjectIDs: []string{"proj-1"}, ViewerEmail: email})
}

// projectAccessByEmail answers GetUserProjectAccess from a fixed table:
// "colleague@example.com" is a registered contact on proj-1 (the customer
// caller's project), "invited@example.com" is only invited there, and
// "other-tenant@example.com" is registered on another project.
func projectAccessByEmail(_ context.Context, email string) ([]domain.UserContactAccess, error) {
	switch email {
	case "colleague@example.com":
		return []domain.UserContactAccess{{ProjectID: "proj-1", RegistrationState: "REGISTERED", GrantsCaseAccess: true}}, nil
	case "invited@example.com":
		return []domain.UserContactAccess{{ProjectID: "proj-1", RegistrationState: "INVITED"}}, nil
	case "other-tenant@example.com":
		return []domain.UserContactAccess{{ProjectID: "proj-2", RegistrationState: "REGISTERED", GrantsCaseAccess: true}}, nil
	}
	return nil, nil
}

func TestUserService_CreateUser_RequiresInternalCaller(t *testing.T) {
	req := domain.CreateUserRequest{FirstName: "Jane", Email: "jane.doe@example.com"}
	_, err := NewUserService(stubUserRepo{}).CreateUser(customerCallerCtx(t, "customer@example.com"), req)
	requireForbidden(t, "customer creates user", err)

	_, err = NewUserService(stubUserRepo{}).CreateUser(contextWithUserIDToken(""), req)
	var ue *apierror.UnauthorizedError
	if !errors.As(err, &ue) {
		t.Fatalf("unidentified caller: err = %v (%T), want *apierror.UnauthorizedError", err, err)
	}
}

func TestUserService_GetUser_ScopedCallerVisibility(t *testing.T) {
	repoFor := func(userType domain.UserType, email string) stubUserRepo {
		return stubUserRepo{
			getUserDetail: func(context.Context, string) (domain.UserDetail, error) {
				return domain.UserDetail{ID: userDetailTestID, UserType: userType, Email: email}, nil
			},
			getUserRoles:         func(context.Context, string) ([]string, error) { return nil, nil },
			getUserGroups:        func(context.Context, string) ([]domain.UserGroupRef, error) { return nil, nil },
			getUserProjectAccess: projectAccessByEmail,
		}
	}
	ctx := customerCallerCtx(t, "customer@example.com")
	var nfe *apierror.NotFoundError

	if _, err := NewUserService(repoFor(domain.UserTypeInternal, "eng@example.com")).GetUser(ctx, userDetailTestID); err != nil {
		t.Fatalf("internal user: unexpected error: %v", err)
	}
	got, err := NewUserService(repoFor(domain.UserTypeCustomer, "colleague@example.com")).GetUser(ctx, userDetailTestID)
	if err != nil {
		t.Fatalf("registered colleague: unexpected error: %v", err)
	}
	if got.ProjectAccess != nil {
		t.Fatalf("project access returned to a scoped caller: %+v", got.ProjectAccess)
	}
	for _, email := range []string{"other-tenant@example.com", "invited@example.com", "stranger@example.com"} {
		if _, err := NewUserService(repoFor(domain.UserTypeCustomer, email)).GetUser(ctx, userDetailTestID); !errors.As(err, &nfe) {
			t.Fatalf("%s: err = %v, want NotFoundError", email, err)
		}
	}
	if _, err := NewUserService(repoFor(domain.UserTypeCustomer, "colleague@example.com")).GetUser(contextWithUserIDToken(""), userDetailTestID); err == nil {
		t.Fatal("unidentified caller: want an error")
	}

	// An internal caller still gets project access for a customer user.
	got, err = NewUserService(repoFor(domain.UserTypeCustomer, "colleague@example.com")).GetUser(internalCallerCtx(t, "eng@example.com"), userDetailTestID)
	if err != nil || len(got.ProjectAccess) != 1 {
		t.Fatalf("internal caller: got %+v, %v; want the project access", got.ProjectAccess, err)
	}
}

func TestUserService_SearchUsers_ScopedCallerVisibility(t *testing.T) {
	repo := stubUserRepo{
		searchUsers: func(context.Context, domain.SearchUsersRequest) ([]domain.User, int, error) {
			return []domain.User{
				{ID: "a", UserType: domain.UserTypeInternal, Email: "eng@example.com"},
				{ID: "b", UserType: domain.UserTypeCustomer, Email: "other-tenant@example.com"},
				{ID: "c", UserType: domain.UserTypeCustomer, Email: "colleague@example.com"},
				{ID: "d", UserType: domain.UserTypeCustomer, Email: "invited@example.com"},
			}, 4, nil
		},
		getUserProjectAccess: projectAccessByEmail,
	}
	resp, err := NewUserService(repo).SearchUsers(customerCallerCtx(t, "customer@example.com"), domain.SearchUsersRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Users) != 2 || resp.Users[0].ID != "a" || resp.Users[1].ID != "c" {
		t.Fatalf("customer page = %+v, want the internal user and the registered colleague", resp.Users)
	}

	resp, err = NewUserService(repo).SearchUsers(internalCallerCtx(t, "eng@example.com"), domain.SearchUsersRequest{})
	if err != nil || len(resp.Users) != 4 {
		t.Fatalf("internal caller: got %d users, err %v; want 4, nil", len(resp.Users), err)
	}
	if _, err := NewUserService(repo).SearchUsers(contextWithUserIDToken(""), domain.SearchUsersRequest{}); err == nil {
		t.Fatal("unidentified caller: want an error")
	}
}
