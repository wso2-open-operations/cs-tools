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

package middleware

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/entityclient"
)

// ForwardUserIDToken carries the caller's own id token across the hop into
// entity-service.
//
// WHY THIS EXISTS. csm-portal's Auth middleware already puts the token in the
// request context — it reads the incoming x-user-id-token, or falls back to the
// bearer token when the gateway supplied none — but it stores it under
// internal/entity's context key, which only internal/entity's own client reads.
// PLG's client is a different package with keys of its own, so the value was
// present on every PLG request and never looked at.
//
// The effect was that entity-service saw only this backend's client-credentials
// token on PLG calls. Its request log takes callerId from a validated
// x-user-id-token and falls back to the client id, so every PLG line read
// `callerId=-`: the one field that says who performed an action was blank for
// the whole feature, while CSM routes beside it recorded it correctly.
//
// MOUNTED OUTSIDE ResolveIdentity, deliberately, exactly as
// ForwardCorrelationID is. That middleware makes an entity-service call of its
// own to resolve the caller, and it is the first thing to run on a PLG request;
// wrapping it means that call is attributed too, rather than leaving the single
// most frequent PLG request to entity-service as the one hop still anonymous.
//
// A request with no token passes through untouched rather than being given a
// fabricated one. Auth has already supplied one by this point, so an empty value
// means the chain was assembled wrongly, and inventing something would hide that
// rather than surface it.
func ForwardUserIDToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := entity.UserIDTokenFromContext(r.Context())
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(entityclient.WithUserIDToken(r.Context(), token)))
	})
}
