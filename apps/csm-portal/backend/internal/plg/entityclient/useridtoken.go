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

package entityclient

import "context"

// The caller's own id token, forwarded so entity-service can tell WHO is behind
// a PLG request rather than only which service asked.
//
// Without it entity-service sees nothing but this backend's client-credentials
// token, and its request log reads `callerId=-` on every PLG line — the one
// field that would say who did something. CSM's entity client has forwarded
// this since it was written; PLG's is a separate package that never did.
//
// Duplicating the value rather than reading csm's own context key keeps the
// dependency pointing one way: middleware knows about this package, this
// package knows nothing about middleware. Same arrangement as correlation.go.
const userIDTokenHeader = "x-user-id-token" // #nosec G101 -- header name, not a credential

type userIDTokenKey struct{}

// WithUserIDToken returns a context carrying the token for outgoing requests.
func WithUserIDToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, userIDTokenKey{}, token)
}

func userIDTokenFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDTokenKey{}).(string)
	return v
}
