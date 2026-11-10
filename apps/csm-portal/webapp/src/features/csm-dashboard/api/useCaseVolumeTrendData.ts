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
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeCaseVolumeTrendResponse, BeLineBucket } from "@api/backend/types";
import { resolveRelativeDatePlaceholderToUtcDate } from "@utils/resolveRelativeDatePlaceholder";
import {
  hasTeamPlaceholder,
  resolveTeamPlaceholder,
} from "@features/csm-dashboard/utils/teamFilterPlaceholder";
import { resolveRelativeDateFilters } from "@features/csm-dashboard/utils/resolveRelativeDateFilters";
import {
  hasCurrentUserPlaceholder,
  resolveCurrentUserPlaceholder,
} from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";
import {
  shouldRetryWidgetFetch,
  withWidgetFetchSlot,
} from "@features/csm-dashboard/utils/widgetFetchConcurrency";

/** Matches the backend's own default (see entity-service's `/cases/volume-trend`). */
const DEFAULT_DAYS = 30;

/** One point, after the two named series in {@link BeCaseVolumeTrendResponse}
 * are merged by date into a single row — the shape `DashboardLineChart`'s
 * underlying chart library wants (one data array, one line per series key). */
export interface CaseVolumeTrendChartPoint {
  /** `YYYY-MM-DD`, UTC; a week or month point's first day in the window. */
  date: string;
  /** A week or month point's last day in the window; absent for a day. */
  endDate?: string;
  opened: number;
  closed: number;
}

/** A `shape: "line"` widget's fixed window (`BeDashboardWidget.startDate`/
 * `endDate`/`bucket`), sent instead of `days`. */
export interface CaseVolumeTrendWindow {
  startDate: string;
  endDate: string;
  bucket?: BeLineBucket;
}

/** A window bound as the plain date the endpoint takes: a relative date
 * placeholder becomes a UTC calendar day, because the endpoint counts by UTC
 * day (so "this week" starts on the same Monday for every viewer, and
 * "today" is never a day the service has not reached yet); a plain date is
 * sent as it is. */
export function windowDate(value: string): string {
  return resolveRelativeDatePlaceholderToUtcDate(value, new Date()) ?? value;
}

export interface CaseVolumeTrendData {
  points: CaseVolumeTrendChartPoint[];
  isLoading: boolean;
  isError: boolean;
}

/**
 * Resolves a `shape: "line"` widget's data via `POST /cases/volume-trend` —
 * the fixed two-series (Opened/Closed) Case Volume trend chart. Unlike
 * `shape: "pie"`/`"bar"` (`useWidgetPieData`/`useWidgetGroupByData`, one
 * search per slice/bucket), this is always exactly one call: the backend
 * itself computes both series' day buckets in one round trip (and caches
 * them — see entity-service's own CLAUDE.md), so there's nothing for this
 * hook to fan out.
 *
 * `widget.query` is forwarded verbatim as this request's own `filters` —
 * the same case-search DSL every other `case`-resourceType widget's `query`
 * already is (see `BeDashboardWidget.query`'s own doc comment) — resolved
 * for the `__current_team__`/`__current_user__`/relative-date placeholders
 * first, exactly like `useWidgetData` does for `case`-resourceType widgets.
 *
 * `fixedWindow` (a widget's `startDate`/`endDate`/`bucket`) asks for a fixed
 * window instead of the trailing `days`: one point per day, week or month,
 * a week or month point carrying its last day as `endDate`.
 */
export function useCaseVolumeTrendData(
  widgetId: string,
  baseFilters: Record<string, unknown>,
  days: number | undefined,
  selectedTeamCreGroupId?: string | string[],
  selectedTeamSreGroupId?: string | string[],
  currentUserId?: string,
  enabled = true,
  fixedWindow?: CaseVolumeTrendWindow,
): CaseVolumeTrendData {
  const api = useBackendApi();
  // A fixed window replaces `days` (the backend refuses both together).
  const range = fixedWindow
    ? {
        startDate: windowDate(fixedWindow.startDate),
        endDate: windowDate(fixedWindow.endDate),
        ...(fixedWindow.bucket && { bucket: fixedWindow.bucket }),
      }
    : { days: days ?? DEFAULT_DAYS };

  const resolvedFilters = resolveCurrentUserPlaceholder(
    resolveRelativeDateFilters(
      resolveTeamPlaceholder(baseFilters, selectedTeamCreGroupId, selectedTeamSreGroupId),
    ),
    currentUserId,
  );
  const awaitingCurrentUser = hasCurrentUserPlaceholder(resolvedFilters);
  const teamKey = JSON.stringify([selectedTeamCreGroupId, selectedTeamSreGroupId]);
  const isTeamIndependent = !hasTeamPlaceholder(baseFilters);

  const query = useQuery({
    queryKey: [
      ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA,
      "case-volume-trend",
      widgetId,
      resolvedFilters,
      range,
    ],
    queryFn: async (): Promise<BeCaseVolumeTrendResponse> => {
      return withWidgetFetchSlot(async (signal) => {
        return api.post<
          { filters: Record<string, unknown> } & typeof range,
          BeCaseVolumeTrendResponse
        >("/cases/volume-trend", { filters: resolvedFilters, ...range }, { signal });
      }, teamKey);
    },
    enabled: enabled && !awaitingCurrentUser,
    retry: (failureCount, error) => shouldRetryWidgetFetch(failureCount, error, isTeamIndependent),
    staleTime: 300_000,
  });

  // Opened/Closed always carry the identical set of dates (the backend
  // fills every day in the window with a 0-count point rather than omitting
  // it — see CaseVolumeTrendSeries.points' own doc comment), so a plain
  // index-aligned zip is safe; this still falls back to a by-date merge in
  // case a future backend ever returns the two series out of step.
  const series = query.data?.series ?? [];
  const opened = series.find((s) => s.label === "Opened")?.points ?? [];
  const closed = series.find((s) => s.label === "Closed")?.points ?? [];
  const closedByDate = new Map(closed.map((p) => [p.date, p.count]));
  const points: CaseVolumeTrendChartPoint[] = opened.map((p) => ({
    date: p.date,
    ...(p.endDate && { endDate: p.endDate }),
    opened: p.count,
    closed: closedByDate.get(p.date) ?? 0,
  }));

  return {
    points,
    isLoading: !enabled || awaitingCurrentUser || query.isLoading,
    isError: query.isError,
  };
}
