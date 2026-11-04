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
	"context"
	"net/http"
)

const userIDHeader = "X-CSM-User-ID"

// maxUserIDLength bounds the header kept for the logs; a longer value is not a
// user id the portal backend sends, so it is dropped rather than logged.
const maxUserIDLength = 128

type userIDKey struct{}

// UserID is an HTTP middleware that stores the X-CSM-User-ID request header,
// the id of the portal user the call is made for, in the context, so the
// ctxHandler adds it to every log record (the token usage line among them).
// The header is for attribution only, never for authorisation: the portal
// backend authenticates the user before it calls this service, and it sends
// the user's id, never their email.
func UserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(userIDHeader)
		if id == "" || len(id) > maxUserIDLength {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, id)))
	})
}

// UserIDFromContext returns the user id stored in ctx, or "" if the request
// carried none (or the UserID middleware was not applied).
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey{}).(string)
	return v
}
