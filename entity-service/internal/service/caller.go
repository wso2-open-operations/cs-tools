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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// callerEmail returns the e-mail of the user the request is acting as, taken
// from the identity the auth middleware verified and attached to ctx. It is
// the single place services obtain a caller's e-mail: the raw
// x-user-id-token header is never decoded again here, so a service can only
// act as a user the middleware already validated.
//
// An identity the middleware did not validate is refused outright (the
// middleware was left out of the chain -- a wiring bug, never a deployment
// choice), and a request with no user token is Unauthorized with the same
// message every service used before, so callers and tests see no change in
// the error for that case.
func callerEmail(ctx context.Context) (string, error) {
	id := auth.IdentityFromContext(ctx)
	if !id.Validated {
		return "", &apierror.ServiceUnavailableError{Msg: "the caller cannot be identified: no verified identity on this request"}
	}
	if id.UserEmail == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	return id.UserEmail, nil
}

// optionalCallerEmail is callerEmail for the paths that work with or without
// a user token (a filter that only applies to a signed-in user, a comment
// author recorded best-effort): it returns "" instead of Unauthorized when
// the request carries no user token, and still refuses an unvalidated
// identity.
func optionalCallerEmail(ctx context.Context) (string, error) {
	id := auth.IdentityFromContext(ctx)
	if !id.Validated {
		return "", &apierror.ServiceUnavailableError{Msg: "the caller cannot be identified: no verified identity on this request"}
	}
	return id.UserEmail, nil
}

// resolveCallerScope returns the caller's AccessScope. It goes through access
// when the service has one wired; otherwise it uses the scope the request
// middleware already resolved and attached to ctx, so a service constructed
// without an AccessService still never answers an unidentified caller.
func resolveCallerScope(ctx context.Context, access AccessService) (AccessScope, error) {
	if access != nil {
		return access.ResolveScope(ctx)
	}
	if cached, ok := repository.CallerIdentityFromContext(ctx); ok {
		return cached, nil
	}
	return AccessScope{}, &apierror.UnauthorizedError{Msg: "a user token (x-user-id-token) or an authorized internal client credential is required"}
}
