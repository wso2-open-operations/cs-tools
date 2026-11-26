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

/**
 * Scopes the portal's access token carries. `roles` is what makes the IdP put
 * the user's roles claim in the token; the backend authorises every request
 * from it (see the backend README's "Access control"), so without this scope a
 * user holds no portal role.
 */
export const AUTH_SCOPES = ["openid", "email", "groups", "profile", "roles"] as const;

/**
 * Client id a host application exchanges its own token for when the portal is
 * loaded as a remote: the portal's own sign-in client id,
 * `window.config.CSM_PORTAL_AUTH_CLIENT_ID`. Read at call time, and directly
 * rather than through authConfig.ts so importing this never trips that
 * module's missing-config check.
 */
export function tokenExchangeClientId(): string {
  return window.config?.CSM_PORTAL_AUTH_CLIENT_ID ?? "";
}
