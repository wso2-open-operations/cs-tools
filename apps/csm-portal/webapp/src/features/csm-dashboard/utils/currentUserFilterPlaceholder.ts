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

import { isCaseFieldFilterArray, type WidgetCaseFieldFilterLike } from "./widgetPreviewUrl";

/**
 * Placeholder value a dashboard widget's own filters may carry wherever a
 * per-user value belongs — e.g. `assignedUserId`/`assignedUserIds` — meaning
 * "the signed-in user's own id". Until 2026-08-06 this was resolved
 * server-side (`CurrentUserPlaceholder`/`substituteCurrentUser` in the
 * backend's `internal/dashboard` package, via a `GET /users/me` round trip
 * baked into `GET /dashboards/{id}`); `GET /dashboards/{id}` now returns
 * `Query`/`Slices` verbatim, so this frontend resolves it itself, at the
 * same point (and by the same generic value-substitution approach — the
 * backend's own `substituteCurrentUser` walked the filters object by value,
 * not by a hardcoded field name) `__current_team__` is already resolved
 * client-side (see `teamFilterPlaceholder.ts`).
 */
export const CURRENT_USER_PLACEHOLDER = "__current_user__";

/**
 * Substitutes {@link CURRENT_USER_PLACEHOLDER} wherever it appears in a
 * dashboard widget's filters with the signed-in user's own platform id
 * (`useCurrentUser().user.id`) — handling both filter shapes this app's
 * widgets use (same two shapes `resolveTeamPlaceholder`/
 * `resolveCurrentUserSentinels` already branch on):
 *
 * - the case-search generic field/op/values DSL (`{ filters:
 *   BeCaseFieldFilter[] }`, e.g. `{ field: "assignedUserId", op: "in",
 *   values: ["__current_user__"] }`)
 * - every other resourceType's flat `{ fieldName: string[] | string }`
 *   record (e.g. `{ assignedUserIds: ["__current_user__"] }`)
 *
 * Unlike the case-search DSL's `field`-scoped `resolveTeamPlaceholder` (which
 * only ever looks at `integrationCsTeam`), this walks every field generically
 * — mirroring the backend's own now-removed `substituteCurrentUser`, which
 * substituted the placeholder by VALUE wherever it appeared, with no
 * hardcoded field name — since a widget can put "the signed-in user" in any
 * field a resourceType's search supports (`assignedUserId` for case,
 * `assignedUserIds` for another resourceType's flat filters, and so on), not
 * only the one field name this app's widget configs happen to use today.
 *
 * When `currentUserId` is undefined (the signed-in user's profile hasn't
 * loaded yet), the placeholder is DROPPED rather than sent to the backend
 * literally — same fail-open philosophy as `resolveTeamPlaceholder`: the
 * backend would either reject a non-UUID value with a 400 or, worse, silently
 * match zero rows (a filter that reads as "nothing to see here" rather than
 * "still loading"), where dropping the condition just widens the query back
 * to unfiltered — a visibly-too-broad result a viewer notices, and one that
 * self-corrects on the next render once the user profile loads (the
 * substitution is applied fresh on every call, not cached).
 */
export function resolveCurrentUserPlaceholder(
  filters: Record<string, unknown>,
  currentUserId: string | undefined,
): Record<string, unknown> {
  const fieldFilters = filters.filters;
  if (isCaseFieldFilterArray(fieldFilters)) {
    return resolveCaseFieldFilters(filters, fieldFilters, currentUserId);
  }
  return resolveFlatFilters(filters, currentUserId);
}

function resolveCaseFieldFilters(
  filters: Record<string, unknown>,
  fieldFilters: WidgetCaseFieldFilterLike[],
  currentUserId: string | undefined,
): Record<string, unknown> {
  let changed = false;
  const resolved: WidgetCaseFieldFilterLike[] = [];
  for (const entry of fieldFilters) {
    const values = entry.values;
    if (!values?.includes(CURRENT_USER_PLACEHOLDER)) {
      resolved.push(entry);
      continue;
    }
    changed = true;
    if (currentUserId === undefined) {
      // Drop the entry entirely — see the doc comment above.
      continue;
    }
    resolved.push({
      ...entry,
      values: values.map((v) => (v === CURRENT_USER_PLACEHOLDER ? currentUserId : v)),
    });
  }

  if (!changed) return filters;
  return { ...filters, filters: resolved };
}

function resolveFlatFilters(
  filters: Record<string, unknown>,
  currentUserId: string | undefined,
): Record<string, unknown> {
  let changed = false;
  const resolved: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(filters)) {
    if (Array.isArray(value) && value.every((v) => typeof v === "string")) {
      const strings = value as string[];
      if (!strings.includes(CURRENT_USER_PLACEHOLDER)) {
        resolved[key] = value;
        continue;
      }
      changed = true;
      if (currentUserId === undefined) {
        // Drop the placeholder entry from the array (see the doc comment
        // above); an array left empty by that drop is dropped too, rather
        // than sent as `[]` (the backend rejects an empty values array the
        // same way it would a literal unresolved placeholder).
        const remaining = strings.filter((v) => v !== CURRENT_USER_PLACEHOLDER);
        if (remaining.length > 0) resolved[key] = remaining;
        continue;
      }
      resolved[key] = strings.map((v) => (v === CURRENT_USER_PLACEHOLDER ? currentUserId : v));
      continue;
    }
    if (value === CURRENT_USER_PLACEHOLDER) {
      changed = true;
      if (currentUserId !== undefined) resolved[key] = currentUserId;
      // else: drop the key entirely — see the doc comment above.
      continue;
    }
    resolved[key] = value;
  }

  if (!changed) return filters;
  return resolved;
}
