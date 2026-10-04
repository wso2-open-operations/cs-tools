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

// Whether the signed-in user should see the SPL (Support Portal Lite)
// section at all — the "can this caller reach SPL" audience gate, distinct
// from permissionsContext.ts's usePermissions()'s fine-grained action gates
// and from usePortalView.ts's "which nav lands by default" choice.
//
// Reads a portal role off `GET /users/me` (the same server-authoritative
// `roles` array usePortalView reads it from — see
// `internal/handler/access.go`'s `AccessConfig`), not a client-side
// identity-provider groups decode.
//
// Checks plain "viewer", unconditionally — it does NOT exclude callers who
// also hold cs_engineer. This is deliberately broader than
// usePortalView.ts's routing choice: a CS engineer who also holds viewer
// still *defaults* to the CS/ABT nav (see usePortalView.ts's cs_engineer
// precedence), but isn't hard-blocked from an SPL screen reached directly.
// The audience question here — does this caller have any business in SPL
// at all — and the nav-default question there are answered separately on
// purpose; keep them that way rather than merging the two checks. See
// internal/handler/access.go's PermSPLAccess for the matching backend
// grant, which must stay in sync with this hook's role check.
//
// Real enforcement is server-side: every /spl/* route on the Go backend
// re-checks PermSPLAccess (internal/handler/access.go), currently granted
// by the same Viewer role this hook checks. A caller who reaches an SPL
// screen without the role sees a 403 from every call it makes, same as
// any other tampered/stale-claim scenario in this app.

import { useMemo } from "react";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { devBypassAccessCheck } from "@config/devFlags";

export interface Access {
  /** False until the signed-in user's profile has been resolved — hold gated UI until it clears. */
  ready: boolean;
  hasAccess: boolean;
}

// Must stay byte-for-byte in sync with the backend's AccessGuard portalRoles
// key (apps/csm-portal/backend/internal/handler/access.go) -- see this
// file's own top-of-file comment for why this check is unconditional
// (no cs_engineer exclusion) unlike usePortalView.ts's nav-default choice.
const SPL_AUDIENCE_ROLE = "viewer";

export function useAccess(): Access {
  let roles: string[] | undefined;
  let isLoading = false;
  try {
    // useCurrentUser always runs its useContext before it can throw, so the
    // hook order is identical on every render — same pattern as
    // usePortalAccess/usePortalView, which this mirrors.
    const ctx = useCurrentUser();
    roles = ctx.user?.roles;
    isLoading = ctx.isLoading;
  } catch {
    roles = undefined;
  }

  return useMemo<Access>(() => {
    // TEMPORARY / LOCAL DEV ONLY — see authConfig.ts's devBypassAccessCheck.
    // Short-circuits SPL's own audience gate so the section shows up even
    // when the signed-in account has no portal roles provisioned yet.
    if (devBypassAccessCheck) return { ready: true, hasAccess: true };
    if (isLoading) return { ready: false, hasAccess: false };
    return { ready: true, hasAccess: (roles ?? []).includes(SPL_AUDIENCE_ROLE) };
  }, [roles, isLoading]);
}
