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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// currentUserResolver is the one thing the PLG actor resolver needs from the
// user service: the platform user behind the request's validated user token.
// service.UserService satisfies it.
type currentUserResolver interface {
	GetMe(ctx context.Context) (domain.GetUserMeResponse, error)
}

// PlgActorResolver decides which platform user an attributed PLG write is
// recorded against.
//
// The actor is taken from the request's validated identity whenever there is
// one: a caller presenting an x-user-id-token acts as the platform user that
// token resolves to, and may not name anyone else in the body. Only an
// allow-listed internal client (an x-jwt-assertion whose client id is in
// AUTH_INTERNAL_CLIENT_IDS) with no user token -- the portal backend calling
// on behalf of a user it has already authenticated -- may supply the actor as
// the body's actorId. Any other caller is refused: a body field is never on
// its own enough to act as someone.
type PlgActorResolver struct {
	users             currentUserResolver
	internalClientIDs map[string]bool
}

// NewPlgActorResolver builds the resolver. internalClientIDs is
// config.Config.AuthInternalClientIDs.
func NewPlgActorResolver(users currentUserResolver, internalClientIDs map[string]bool) *PlgActorResolver {
	return &PlgActorResolver{users: users, internalClientIDs: internalClientIDs}
}

// ActorID returns the platform user id the write is attributed to, given the
// actorId the request body carried (empty when it carried none).
func (r *PlgActorResolver) ActorID(ctx context.Context, bodyActorID string) (string, error) {
	bodyActorID = strings.TrimSpace(bodyActorID)
	id := auth.IdentityFromContext(ctx)
	switch {
	case id.Validated && id.UserEmail != "":
		if bodyActorID != "" {
			return "", &apierror.ValidationError{Msg: "actorId must not be supplied alongside an x-user-id-token"}
		}
		me, err := r.users.GetMe(ctx)
		if err != nil {
			return "", err
		}
		return me.ID, nil
	case id.Validated && id.ClientID != "" && r.internalClientIDs[id.ClientID]:
		if bodyActorID == "" {
			return "", &apierror.ValidationError{Msg: "actorId is required"}
		}
		return bodyActorID, nil
	default:
		return "", &apierror.ForbiddenError{Msg: "actorId may only be supplied by an authorized internal client"}
	}
}
