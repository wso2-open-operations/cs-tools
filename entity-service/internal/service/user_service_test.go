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

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestUserService_GetMe_ResolvesCallerFromToken proves GetMe decodes the
// caller's email from the x-user-id-token JWT and returns the matching
// Postgres user row, with empty (not fabricated) roles/groups since the
// Postgres data source has no such tables.
func TestUserService_GetMe_ResolvesCallerFromToken(t *testing.T) {
	timezone := "Asia/Colombo"
	repo := stubUserRepo{
		getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
			if email != "jane.doe@example.com" {
				t.Fatalf("GetUserByEmail called with unexpected email: %q", email)
			}
			return domain.User{
				ID:        "11111111-1111-1111-1111-111111111111",
				FirstName: "Jane",
				LastName:  "Doe",
				Email:     email,
				Timezone:  &timezone,
			}, nil
		},
	}
	svc := NewUserService(repo)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe returned error: %v", err)
	}
	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q, want the user's row id", resp.ID)
	}
	if resp.Email != "jane.doe@example.com" {
		t.Errorf("Email = %q, want jane.doe@example.com", resp.Email)
	}
	if resp.FirstName == nil || *resp.FirstName != "Jane" {
		t.Errorf("FirstName = %v, want Jane", resp.FirstName)
	}
	if resp.LastName != "Doe" {
		t.Errorf("LastName = %q, want Doe", resp.LastName)
	}
	if resp.TimeZone == nil || *resp.TimeZone != timezone {
		t.Errorf("TimeZone = %v, want %q", resp.TimeZone, timezone)
	}
	if len(resp.Roles) != 0 {
		t.Errorf("Roles = %v, want empty (Postgres has no roles table)", resp.Roles)
	}
	if len(resp.Groups) != 0 {
		t.Errorf("Groups = %v, want empty (Postgres has no group tables)", resp.Groups)
	}
}

// TestUserService_GetMe_RequiresToken proves a missing x-user-id-token header
// fails as Unauthorized rather than falling through to some other identity.
func TestUserService_GetMe_RequiresToken(t *testing.T) {
	svc := NewUserService(stubUserRepo{})
	ctx := contextWithUserIDToken("")

	_, err := svc.GetMe(ctx)
	if _, ok := err.(*apierror.UnauthorizedError); !ok {
		t.Fatalf("GetMe error = %v (%T), want *apierror.UnauthorizedError", err, err)
	}
}

// TestUserService_GetMe_RejectsMalformedToken proves an undecodable
// x-user-id-token surfaces as a ValidationError, not an opaque failure.
func TestUserService_GetMe_RejectsMalformedToken(t *testing.T) {
	svc := NewUserService(stubUserRepo{})
	ctx := contextWithUserIDToken("not-a-jwt")

	_, err := svc.GetMe(ctx)
	if _, ok := err.(*apierror.ValidationError); !ok {
		t.Fatalf("GetMe error = %v (%T), want *apierror.ValidationError", err, err)
	}
}

// TestUserService_GetMe_PropagatesRepoNotFound proves a token whose email has
// no matching Postgres user surfaces the repository's NotFoundError verbatim.
func TestUserService_GetMe_PropagatesRepoNotFound(t *testing.T) {
	repo := stubUserRepo{
		getUserByEmail: func(context.Context, string) (domain.User, error) {
			return domain.User{}, &apierror.NotFoundError{Msg: "no user found with email: ghost@example.com"}
		},
	}
	svc := NewUserService(repo)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "ghost@example.com"))

	_, err := svc.GetMe(ctx)
	if _, ok := err.(*apierror.NotFoundError); !ok {
		t.Fatalf("GetMe error = %v (%T), want *apierror.NotFoundError", err, err)
	}
}

// TestUserService_SearchUsers_SortBy proves the Postgres path accepts the same
// sort fields the API advertises (it used to reject any sortBy, so the CSM users
// page, which sends name/asc, got a 400) and passes a valid sort to the
// repository, while a malformed one is still a validation error.
func TestUserService_SearchUsers_SortBy(t *testing.T) {
	tests := []struct {
		name    string
		sort    domain.UserSortBy
		wantErr string
	}{
		{name: "none", sort: domain.UserSortBy{}},
		{name: "name asc (what the CSM page sends)", sort: domain.UserSortBy{Field: "name", Order: "asc"}},
		{name: "createdOn desc", sort: domain.UserSortBy{Field: "createdOn", Order: "desc"}},
		{name: "updatedOn, order omitted", sort: domain.UserSortBy{Field: "updatedOn"}},
		{name: "unknown field", sort: domain.UserSortBy{Field: "email"}, wantErr: "sortBy.field contains invalid value: email"},
		{name: "order without field", sort: domain.UserSortBy{Order: "asc"}, wantErr: "sortBy.order requires sortBy.field to be set"},
		{name: "unknown order", sort: domain.UserSortBy{Field: "name", Order: "up"}, wantErr: "sortBy.order contains invalid value: up"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got domain.UserSortBy
			called := false
			repo := stubUserRepo{
				searchUsers: func(_ context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error) {
					called = true
					got = req.SortBy
					return nil, 0, nil
				},
			}
			_, err := NewUserService(repo).SearchUsers(context.Background(), domain.SearchUsersRequest{SortBy: tt.sort})
			if tt.wantErr != "" {
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) || ve.Msg != tt.wantErr {
					t.Fatalf("err = %v, want ValidationError %q", err, tt.wantErr)
				}
				if called {
					t.Fatal("repository must not be reached for an invalid sort")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !called || got != tt.sort {
				t.Fatalf("repo saw sort %+v (called=%v), want %+v", got, called, tt.sort)
			}
		})
	}
}

// TestUserService_SearchUsers_ActiveFilterReachesRepository is the
// regression guard for a real bug: POST /users/search with an active
// filter (e.g. {roleIds: ["timecard_approver"], active: true}, the Time
// Tracking tab's own approver search) 400'd unconditionally on this data
// source, even though "user".is_active is a real, already-read column.
// Proves both true and false reach the repository rather than being
// rejected.
func TestUserService_SearchUsers_ActiveFilterReachesRepository(t *testing.T) {
	for _, active := range []bool{true, false} {
		t.Run(fmt.Sprintf("active=%v", active), func(t *testing.T) {
			var got *bool
			repo := stubUserRepo{
				searchUsers: func(_ context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error) {
					got = req.Filters.Active
					return nil, 0, nil
				},
			}
			active := active
			req := domain.SearchUsersRequest{Filters: domain.SearchUsersFilters{Active: &active}}
			if _, err := NewUserService(repo).SearchUsers(context.Background(), req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil || *got != active {
				t.Fatalf("repo saw Active = %v, want %v", got, active)
			}
		})
	}
}

// TestUserService_SearchUsers_UserIDsGroupFiltersReachRepository is the
// regression guard for another instance of the same bug: userIds, groupIds
// and groupNames were rejected outright on this data source, even though
// they're a plain u.id = ANY(...) and an EXISTS against team_member/team --
// both already used elsewhere (e.g. GetUserGroups). Proves all three reach
// the repository rather than being rejected.
func TestUserService_SearchUsers_UserIDsGroupFiltersReachRepository(t *testing.T) {
	userID := "11111111-1111-1111-1111-111111111111"
	groupID := "22222222-2222-2222-2222-222222222222"
	var got domain.SearchUsersFilters
	repo := stubUserRepo{
		searchUsers: func(_ context.Context, req domain.SearchUsersRequest) ([]domain.User, int, error) {
			got = req.Filters
			return nil, 0, nil
		},
	}
	req := domain.SearchUsersRequest{Filters: domain.SearchUsersFilters{
		UserIDs:    []string{userID},
		GroupIDs:   []string{groupID},
		GroupNames: []string{"CAB Approval"},
	}}
	if _, err := NewUserService(repo).SearchUsers(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.UserIDs) != 1 || got.UserIDs[0] != userID {
		t.Fatalf("repo saw UserIDs = %v, want [%s]", got.UserIDs, userID)
	}
	if len(got.GroupIDs) != 1 || got.GroupIDs[0] != groupID {
		t.Fatalf("repo saw GroupIDs = %v, want [%s]", got.GroupIDs, groupID)
	}
	if len(got.GroupNames) != 1 || got.GroupNames[0] != "CAB Approval" {
		t.Fatalf("repo saw GroupNames = %v, want [CAB Approval]", got.GroupNames)
	}
}

// TestUserService_SearchUsers_UserIDsGroupFilters_RejectsMalformedUUID proves
// userIds/groupIds still validate as UUIDs before reaching the repository --
// only the blanket "unsupported on Postgres" rejection was removed.
func TestUserService_SearchUsers_UserIDsGroupFilters_RejectsMalformedUUID(t *testing.T) {
	tests := []struct {
		name    string
		filters domain.SearchUsersFilters
	}{
		{name: "userIds", filters: domain.SearchUsersFilters{UserIDs: []string{"not-a-uuid"}}},
		{name: "groupIds", filters: domain.SearchUsersFilters{GroupIDs: []string{"not-a-uuid"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := stubUserRepo{
				searchUsers: func(context.Context, domain.SearchUsersRequest) ([]domain.User, int, error) {
					t.Fatal("repository should not be called for a malformed uuid")
					return nil, 0, nil
				},
			}
			req := domain.SearchUsersRequest{Filters: tt.filters}
			_, err := NewUserService(repo).SearchUsers(context.Background(), req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
		})
	}
}

const userDetailTestID = "11111111-1111-1111-1111-111111111111"

func TestUserService_GetUser(t *testing.T) {
	staff := domain.UserDetail{ID: userDetailTestID, Name: "Sam Staff", Email: "sam@example.com", UserType: domain.UserTypeInternal, Active: true}
	customer := domain.UserDetail{ID: userDetailTestID, Name: "Cy Customer", Email: "cy@customer.example", UserType: domain.UserTypeCustomer, Active: true}
	access := []domain.UserContactAccess{{ProjectID: "p1", ProjectKey: "ACME", RegistrationState: "REGISTERED", GrantsCaseAccess: true, Roles: []string{"PORTAL_USER"}}}

	t.Run("staff gets roles and groups but no project access lookup", func(t *testing.T) {
		repo := stubUserRepo{
			getUserDetail: func(context.Context, string) (domain.UserDetail, error) { return staff, nil },
			getUserRoles:  func(context.Context, string) ([]string, error) { return []string{"admin"}, nil },
			getUserGroups: func(context.Context, string) ([]domain.UserGroupRef, error) {
				return []domain.UserGroupRef{{ID: "t1", Name: "CAB Approval"}}, nil
			},
			// getUserProjectAccess left nil: calling it panics, proving it is skipped.
		}
		got, err := NewUserService(repo).GetUser(context.Background(), userDetailTestID)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Roles) != 1 || got.Roles[0] != "admin" || len(got.Groups) != 1 || got.ProjectAccess != nil {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("customer also gets project access, looked up by their email", func(t *testing.T) {
		var lookedUp string
		repo := stubUserRepo{
			getUserDetail: func(context.Context, string) (domain.UserDetail, error) { return customer, nil },
			getUserProjectAccess: func(_ context.Context, email string) ([]domain.UserContactAccess, error) {
				lookedUp = email
				return access, nil
			},
		}
		got, err := NewUserService(repo).GetUser(context.Background(), userDetailTestID)
		if err != nil {
			t.Fatal(err)
		}
		if lookedUp != "cy@customer.example" || len(got.ProjectAccess) != 1 || !got.ProjectAccess[0].GrantsCaseAccess {
			t.Errorf("lookedUp=%q got=%+v", lookedUp, got.ProjectAccess)
		}
	})

	t.Run("malformed id is a validation error and never reaches the repository", func(t *testing.T) {
		_, err := NewUserService(stubUserRepo{}).GetUser(context.Background(), "not-a-uuid")
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("err = %v, want ValidationError", err)
		}
	})

	t.Run("unknown user is not found", func(t *testing.T) {
		repo := stubUserRepo{getUserDetail: func(context.Context, string) (domain.UserDetail, error) {
			return domain.UserDetail{}, &apierror.NotFoundError{Msg: "no user"}
		}}
		_, err := NewUserService(repo).GetUser(context.Background(), userDetailTestID)
		var nf *apierror.NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("err = %v, want NotFoundError", err)
		}
	})

	t.Run("a lookup failure is an error, not a silently partial profile", func(t *testing.T) {
		boom := errors.New("db down")
		repo := stubUserRepo{
			getUserDetail: func(context.Context, string) (domain.UserDetail, error) { return staff, nil },
			getUserRoles:  func(context.Context, string) ([]string, error) { return nil, boom },
		}
		if _, err := NewUserService(repo).GetUser(context.Background(), userDetailTestID); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
	})
}

func TestUserService_CreateUser(t *testing.T) {
	validReq := domain.CreateUserRequest{FirstName: "Jane", LastName: "Doe", Email: "jane.doe@example.com"}

	t.Run("resolves the acting caller from the token and forwards it as actor", func(t *testing.T) {
		var gotReq domain.CreateUserRequest
		var gotActor string
		repo := stubUserRepo{
			createUser: func(_ context.Context, req domain.CreateUserRequest, actor string) (domain.User, error) {
				gotReq, gotActor = req, actor
				return domain.User{ID: userDetailTestID, Email: req.Email}, nil
			},
		}
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "admin@example.com"))
		got, err := NewUserService(repo).CreateUser(ctx, validReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotActor != "admin@example.com" {
			t.Errorf("actor = %q, want the caller's own email", gotActor)
		}
		if gotReq.Email != validReq.Email || gotReq.FirstName != validReq.FirstName || gotReq.LastName != validReq.LastName {
			t.Errorf("repo saw %+v, want %+v", gotReq, validReq)
		}
		if got.ID != userDetailTestID {
			t.Errorf("got = %+v, want the repository's row echoed back", got)
		}
	})

	t.Run("requires a token, same as GetMe", func(t *testing.T) {
		_, err := NewUserService(stubUserRepo{}).CreateUser(contextWithUserIDToken(""), validReq)
		if _, ok := err.(*apierror.UnauthorizedError); !ok {
			t.Fatalf("err = %v (%T), want *apierror.UnauthorizedError", err, err)
		}
	})

	t.Run("rejects a missing or malformed email before reaching the repository", func(t *testing.T) {
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "admin@example.com"))
		for name, req := range map[string]domain.CreateUserRequest{
			"empty":     {FirstName: "Jane", LastName: "Doe"},
			"malformed": {FirstName: "Jane", LastName: "Doe", Email: "not-an-email"},
		} {
			t.Run(name, func(t *testing.T) {
				_, err := NewUserService(stubUserRepo{}).CreateUser(ctx, req)
				if _, ok := err.(*apierror.ValidationError); !ok {
					t.Fatalf("err = %v (%T), want *apierror.ValidationError", err, err)
				}
			})
		}
	})

	t.Run("requires at least a first or last name", func(t *testing.T) {
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "admin@example.com"))
		_, err := NewUserService(stubUserRepo{}).CreateUser(ctx, domain.CreateUserRequest{Email: "jane.doe@example.com"})
		if _, ok := err.(*apierror.ValidationError); !ok {
			t.Fatalf("err = %v (%T), want *apierror.ValidationError", err, err)
		}
	})

	t.Run("propagates the repository's conflict on a duplicate email", func(t *testing.T) {
		repo := stubUserRepo{
			createUser: func(context.Context, domain.CreateUserRequest, string) (domain.User, error) {
				return domain.User{}, &apierror.ConflictError{Msg: "a user with this email already exists: jane.doe@example.com"}
			},
		}
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "admin@example.com"))
		_, err := NewUserService(repo).CreateUser(ctx, validReq)
		if _, ok := err.(*apierror.ConflictError); !ok {
			t.Fatalf("err = %v (%T), want *apierror.ConflictError", err, err)
		}
	})
}
