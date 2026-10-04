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

package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type stubCurrentUser struct {
	id    string
	err   error
	calls int
}

func (s *stubCurrentUser) GetMe(context.Context) (domain.GetUserMeResponse, error) {
	s.calls++
	return domain.GetUserMeResponse{ID: s.id}, s.err
}

func TestPlgActorResolver(t *testing.T) {
	const (
		internalClient = "internal-client"
		tokenUser      = "11111111-1111-1111-1111-111111111111"
		bodyActor      = "22222222-2222-2222-2222-222222222222"
	)
	var (
		validation *apierror.ValidationError
		forbidden  *apierror.ForbiddenError
	)

	cases := []struct {
		name      string
		identity  auth.Identity
		body      string
		want      string
		wantErr   any // pointer to the expected *apierror type, or nil
		wantGetMe int
	}{
		{"user token acts as the token's user", auth.Identity{Validated: true, UserEmail: "jane.doe@example.com"}, "", tokenUser, nil, 1},
		{"user token may not name another actor", auth.Identity{Validated: true, UserEmail: "jane.doe@example.com"}, bodyActor, "", &validation, 0},
		{"internal client supplies the actor", auth.Identity{Validated: true, ClientID: internalClient}, bodyActor, bodyActor, nil, 0},
		{"internal client must supply one", auth.Identity{Validated: true, ClientID: internalClient}, "  ", "", &validation, 0},
		{"unknown client is refused", auth.Identity{Validated: true, ClientID: "someone-else"}, bodyActor, "", &forbidden, 0},
		{"no identity is refused", auth.Identity{Validated: true}, bodyActor, "", &forbidden, 0},
		{"unvalidated identity is refused", auth.Identity{ClientID: internalClient}, bodyActor, "", &forbidden, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users := &stubCurrentUser{id: tokenUser}
			r := NewPlgActorResolver(users, map[string]bool{internalClient: true})
			got, err := r.ActorID(auth.WithIdentity(context.Background(), tc.identity), tc.body)
			switch want := tc.wantErr.(type) {
			case nil:
				if err != nil || got != tc.want {
					t.Fatalf("ActorID = %q, %v; want %q, nil", got, err, tc.want)
				}
			case **apierror.ValidationError:
				if !errors.As(err, want) {
					t.Fatalf("err = %v, want a ValidationError", err)
				}
			case **apierror.ForbiddenError:
				if !errors.As(err, want) {
					t.Fatalf("err = %v, want a ForbiddenError", err)
				}
			}
			if users.calls != tc.wantGetMe {
				t.Fatalf("GetMe called %d times, want %d", users.calls, tc.wantGetMe)
			}
		})
	}
}

func TestPlgActorResolver_PropagatesTheUserLookupFailure(t *testing.T) {
	nfe := &apierror.NotFoundError{Msg: "user not found"}
	r := NewPlgActorResolver(&stubCurrentUser{err: nfe}, nil)
	_, err := r.ActorID(auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "jane.doe@example.com"}), "")
	if !errors.Is(err, nfe) {
		t.Fatalf("err = %v, want the lookup's own error", err)
	}
}
