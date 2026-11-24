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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const makeInternalTestClientID = "csm-portal-backend-client"

// viaCSMPortalForUsers mirrors viaCSMPortal (case_escalation_access_service_test.go):
// csm-portal-backend's own client id plus the forwarded signed-in person.
func viaCSMPortalForUsers(email string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email, ClientID: makeInternalTestClientID})
}

func newMakeInternalUserService(repo stubUserRepo) UserService {
	return NewUserService(repo, alwaysUnrestrictedAccess{}, makeInternalTestClientID)
}

// A call from any other client, or with no signed-in person, is refused
// before the repository is ever reached -- the admin decision was already
// made by csm-portal-backend's own PermAdmin gate, so this service only
// needs to know the call genuinely came from there.
func TestMakeUserInternal_IsForTheCSMPortalBackendOnly(t *testing.T) {
	called := false
	repo := stubUserRepo{
		getUserDetail: func(context.Context, string) (domain.UserDetail, error) {
			called = true
			return domain.UserDetail{Email: "staff@wso2.com"}, nil
		},
		makeUserInternal: func(context.Context, string, string) (domain.UserType, error) {
			called = true
			return domain.UserTypeInternal, nil
		},
	}
	svc := newMakeInternalUserService(repo)
	for name, ctx := range map[string]context.Context{
		"another client, even with a signed-in person": auth.WithIdentity(context.Background(), auth.Identity{
			Validated: true, UserEmail: "admin@example.com", ClientID: "some-other-client",
		}),
		"no signed-in user": auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: makeInternalTestClientID}),
	} {
		_, err := svc.MakeUserInternal(ctx, userDetailTestID)
		wantForbidden(t, err, name)
	}
	if called {
		t.Fatal("a refused caller reached the repository")
	}
}

// With no CSM_PORTAL_BACKEND_CLIENT_ID configured, nobody is let through.
func TestMakeUserInternal_RefusesEveryoneWithoutAConfiguredClientID(t *testing.T) {
	svc := NewUserService(stubUserRepo{}, alwaysUnrestrictedAccess{}, "")
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "admin@example.com"})
	_, err := svc.MakeUserInternal(ctx, userDetailTestID)
	wantForbidden(t, err, "make internal with no configured client id")
}

// A non-@wso2.com target is refused before any role is touched -- an
// external/customer address must never become internal, regardless of who
// asks.
func TestMakeUserInternal_RequiresAWso2EmailTarget(t *testing.T) {
	granted := false
	repo := stubUserRepo{
		getUserDetail: func(context.Context, string) (domain.UserDetail, error) {
			return domain.UserDetail{Email: "customer@acme.com"}, nil
		},
		makeUserInternal: func(context.Context, string, string) (domain.UserType, error) {
			granted = true
			return domain.UserTypeInternal, nil
		},
	}
	svc := newMakeInternalUserService(repo)
	_, err := svc.MakeUserInternal(viaCSMPortalForUsers("admin@wso2.com"), userDetailTestID)
	wantInvalid(t, err, "non-wso2 target")
	if granted {
		t.Fatal("roles were synced for a non-@wso2.com target")
	}
}

// The happy path: a @wso2.com target through csm-portal-backend grants
// "internal" via the repository and returns the resulting user_type.
func TestMakeUserInternal_GrantsInternalForAWso2Target(t *testing.T) {
	var gotUserID, gotActor string
	repo := stubUserRepo{
		getUserDetail: func(context.Context, string) (domain.UserDetail, error) {
			return domain.UserDetail{Email: "staff@wso2.com"}, nil
		},
		makeUserInternal: func(_ context.Context, userID, actor string) (domain.UserType, error) {
			gotUserID, gotActor = userID, actor
			return domain.UserTypeInternal, nil
		},
	}
	svc := newMakeInternalUserService(repo)
	resp, err := svc.MakeUserInternal(viaCSMPortalForUsers("admin@wso2.com"), userDetailTestID)
	if err != nil {
		t.Fatalf("MakeUserInternal: %v", err)
	}
	if gotUserID != userDetailTestID {
		t.Fatalf("repo called with userID %q, want %q", gotUserID, userDetailTestID)
	}
	if gotActor != "admin@wso2.com" {
		t.Fatalf("repo called with actor %q, want the signed-in person's email", gotActor)
	}
	if resp.ID != userDetailTestID || resp.UserType != domain.UserTypeInternal {
		t.Fatalf("response = %+v, want id=%s userType=internal", resp, userDetailTestID)
	}
}

// A malformed id is a ValidationError before the repository is reached.
func TestMakeUserInternal_RejectsAMalformedID(t *testing.T) {
	called := false
	repo := stubUserRepo{
		getUserDetail: func(context.Context, string) (domain.UserDetail, error) {
			called = true
			return domain.UserDetail{}, nil
		},
	}
	svc := newMakeInternalUserService(repo)
	_, err := svc.MakeUserInternal(viaCSMPortalForUsers("admin@wso2.com"), "not-a-uuid")
	wantInvalid(t, err, "malformed id")
	if called {
		t.Fatal("repository reached with a malformed id")
	}
}
