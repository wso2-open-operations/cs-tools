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

import { useQuery } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { BeUserSearchByEmailResponse } from "@api/backend/types";
import {
  requestUserIdByEmail,
  type ResolvedUser,
} from "@features/csm-users/api/userEmailResolutionLoader";
import { isPlausibleEmail } from "@features/csm-users/utils/isPlausibleEmail";

/**
 * `emails` search results carry a `oneOf` shape (postgres `User` vs
 * ServiceNow `SnUser`), but `id`/`email` are common to both — that's all this
 * lookup needs.
 */
async function fetchUserIdsByEmail(
  api: ReturnType<typeof useBackendApi>,
  emails: string[],
): Promise<Map<string, ResolvedUser>> {
  const res = await api.post<
    { filters: { emails: string[] } },
    BeUserSearchByEmailResponse
  >("/users/search", { filters: { emails } });
  const map = new Map<string, ResolvedUser>();
  for (const u of res.users ?? []) {
    if (u.id && u.email) {
      map.set(u.email.trim().toLowerCase(), {
        id: u.id,
        isInternal: u.userType === "internal",
      });
    }
  }
  return map;
}

/**
 * Resolves a user's canonical id from their email, for a {@link UserReference}
 * that arrived with `id: null` (or from a pre-canonical-reference backend that
 * only ever sent an email). Never blocks rendering: while unresolved (or the
 * lookup fails) this returns `undefined` and the caller renders plain text —
 * see `UserRefLink`.
 *
 * - `knownId` (already-present) short-circuits with no network call.
 * - An email that doesn't look like an email (`"system"`, a bot name, ...) is
 *   never looked up.
 * - Requests for different emails made in a short window are coalesced into
 *   one `POST /users/search` call by `requestUserIdByEmail`; the result for
 *   *this* email is then cached under its own react-query key, so every other
 *   `UserRefLink` for the same person — on this page or later in the session —
 *   reuses it instead of re-fetching.
 */
export function useResolvedUserId(
  email: string | undefined,
  knownId?: string | null,
): string | undefined {
  return useResolvedUser(email, knownId).id;
}

/**
 * {@link useResolvedUserId} plus whether the person is an internal user. With
 * `alsoNeedInternal` the lookup also runs for a reference whose id is already
 * known, because the id alone does not say who is staff.
 */
export function useResolvedUser(
  email: string | undefined,
  knownId?: string | null,
  alsoNeedInternal = false,
): { id: string | undefined; isInternal: boolean | undefined } {
  const api = useBackendApi();
  const normalizedEmail = email?.trim().toLowerCase();
  const shouldResolve =
    (!knownId || alsoNeedInternal) && isPlausibleEmail(normalizedEmail);

  const { data } = useQuery<ResolvedUser | null>({
    queryKey: ["csm-user-id-by-email", normalizedEmail ?? ""],
    queryFn: () =>
      requestUserIdByEmail(normalizedEmail as string, (batch) =>
        fetchUserIdsByEmail(api, batch),
      ),
    enabled: shouldResolve,
    // The mapping is effectively static (an email changing owners is a rare
    // administrative event, not something a case-detail view needs to react
    // to live) — cache aggressively so the same person resolves once per
    // session rather than once per page visit.
    staleTime: 5 * 60_000,
    gcTime: 30 * 60_000,
    // A confirmed-empty result is cached by the loader itself (resolves to
    // `null`, which react-query treats as a normal, cacheable value); a
    // network failure rejects instead (see userEmailResolutionLoader), so
    // this bounded retry only covers genuine transient errors.
    retry: 1,
  });

  return {
    id: knownId || data?.id || undefined,
    isInternal: data?.isInternal,
  };
}
