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

// Shape of the OIDC ID token claims the UI relies on. The IdP SDK exposes the
// decoded claims as `any`; this interface narrows it for type-safe access.
export interface IdTokenClaims {
  sub?: string;
  email?: string;
  given_name?: string;
  family_name?: string;
  name?: string;
  preferred_username?: string;
  username?: string;
  // Either of the standard OIDC claims may carry the profile picture URL,
  // depending on the IdP. `profile` is the URL of the user's profile page in
  // strict OIDC, but in practice IdPs (Asgardeo, Google, etc.) use it for the
  // avatar image; `picture` is the canonical OIDC field. Read both.
  profile?: string;
  picture?: string;
  groups?: string[];
  org_name?: string;
  org_handle?: string;
  org_id?: string;
}

export interface ResolvedUserInfo {
  fullName: string;
  email: string;
  avatarUrl?: string;
  orgName?: string;
  orgHandle?: string;
  groups: string[];
}

export function resolveUserInfo(user: unknown): ResolvedUserInfo {
  const c = (user ?? {}) as IdTokenClaims;
  const fullName =
    c.name ||
    [c.given_name, c.family_name].filter(Boolean).join(" ").trim() ||
    c.preferred_username ||
    c.username ||
    c.email ||
    "Signed in";

  return {
    fullName,
    email: c.email ?? "",
    avatarUrl: c.picture || c.profile,
    orgName: c.org_name,
    orgHandle: c.org_handle,
    groups: Array.isArray(c.groups) ? c.groups : [],
  };
}

export function initialsOf(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
  return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase();
}
