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

import type { BeWidgetResourceType } from "@api/backend/types";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useWidgetData } from "@features/csm-dashboard/api/useWidgetData";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import { resolveTeamPlaceholder } from "@features/csm-dashboard/utils/teamFilterPlaceholder";
import { resolveRelativeDateFilters } from "@features/csm-dashboard/utils/resolveRelativeDateFilters";
import {
  hasCurrentUserPlaceholder,
  resolveCurrentUserPlaceholder,
} from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";
import { resolveWidgetText } from "@features/csm-dashboard/utils/widgetTextPlaceholder";
import { CS_OVERVIEW_REFETCH_INTERVAL_MS } from "@features/csm-dashboard/utils/wallboardMetricStyle";

export interface WallboardTileDataInput {
  widgetId: string;
  displayName: string;
  resourceType: BeWidgetResourceType;
  filters: Record<string, unknown>;
  selectedTeamCreGroupId?: string | string[];
  selectedTeamSreGroupId?: string | string[];
  selectedTeamLabel?: string;
}

/** Which of the three mutually-exclusive render states a wallboard tile is
 * in. `loading` — the first fetch is in flight, or the query is deferred
 * waiting on the signed-in user's own id. `error` — the fetch failed AND
 * there's nothing cached to fall back on. `value` — show `total`, which
 * includes a *stale* `total` held through a transient background-refetch
 * failure (deliberately not "error": a real number a minute old beats a
 * dash on a display glanced at from across a room). */
export type WallboardTileState = "loading" | "error" | "value";

export interface WallboardTileData {
  /** Last known count — `0` until the first fetch resolves, and kept
   * through a failed background refetch (see {@link WallboardTileState}). */
  total: number;
  /** `displayName` with any team-name text placeholder resolved. */
  resolvedDisplayName: string;
  state: WallboardTileState;
  /** The resource-tab URL to wrap the tile in as a link, or `undefined`
   * when it shouldn't be a link right now — no href builder for this
   * resourceType, or the tile is loading / errored / awaiting the current
   * user (so a skeleton or dash is never a click target). */
  linkHref: string | undefined;
}

/**
 * The data + derived state every CS Overview wallboard count-tile needs,
 * shared by `WallboardStatTile` (the glow-capable tiles) and
 * `WallboardSecondaryStat` (CRE's plain secondary row). Those two differ
 * only in styling — the `useCurrentUser` + `useWidgetData` wiring, the
 * `__current_user__` deferral, the last-known-value-on-error handling and
 * the href/link-guard logic are identical, and were previously duplicated
 * line-for-line in both.
 */
export function useWallboardTileData({
  widgetId,
  displayName,
  resourceType,
  filters,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  selectedTeamLabel,
}: WallboardTileDataInput): WallboardTileData {
  const { user } = useCurrentUser();
  const currentUserId = user?.id;
  const awaitingCurrentUser =
    currentUserId === undefined && hasCurrentUserPlaceholder(filters);

  const { data, isLoading, isError } = useWidgetData({
    widgetId,
    resourceType,
    filters,
    shape: "count",
    enabled: !awaitingCurrentUser,
    selectedTeamCreGroupId,
    selectedTeamSreGroupId,
    currentUserId,
    refetchIntervalMs: CS_OVERVIEW_REFETCH_INTERVAL_MS,
  });

  const config = WIDGET_RESOURCE_CONFIG[resourceType];
  const resolvedFilters = resolveCurrentUserPlaceholder(
    resolveRelativeDateFilters(
      resolveTeamPlaceholder(filters, selectedTeamCreGroupId, selectedTeamSreGroupId),
    ),
    currentUserId,
  );
  const href = config ? config.buildHref(resolvedFilters) : undefined;

  const state: WallboardTileState =
    isLoading || awaitingCurrentUser ? "loading" : isError && !data ? "error" : "value";

  // A disabled query (`enabled: !awaitingCurrentUser`) reports `isLoading`
  // false in TanStack Query v5 — that flag is "actively fetching", not
  // "deferred, never fetched" — so `awaitingCurrentUser` has to be checked
  // here too, or a still-loading tile whose filters carry the unresolved
  // `__current_user__` placeholder could get wrapped in a link built from
  // it.
  const linkable = !!href && !isLoading && !isError && !awaitingCurrentUser;

  return {
    total: data?.total ?? 0,
    resolvedDisplayName: resolveWidgetText(displayName, selectedTeamLabel) ?? displayName,
    state,
    linkHref: linkable ? href : undefined,
  };
}
