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
import type { BeCaseVolumeTrendResponse } from "@api/backend/types";
import { resolveRelativeDateFilters } from "@features/csm-dashboard/utils/resolveRelativeDateFilters";
import {
  hasCurrentUserPlaceholder,
  resolveCurrentUserPlaceholder,
} from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";
import {
  shouldRetryWidgetFetch,
  withWidgetFetchSlot,
} from "@features/csm-dashboard/utils/widgetFetchConcurrency";
import { plgRegistrationTrendFilters } from "@features/csm-dashboard/config/widgetResourceConfig";
import {
  windowDate,
  type CaseVolumeTrendData,
  type CaseVolumeTrendWindow,
} from "@features/csm-dashboard/api/useCaseVolumeTrendData";

/** Matches entity-service's own default for `/plg/registrations/volume-trend`. */
const DEFAULT_DAYS = 30;

/**
 * Resolves a `plg_registration` `shape: "line"` widget's data via `POST
 * /plg/registrations/volume-trend`: one series, "Registered", counted by
 * registration day (UTC) with the registration search's own filters, in the
 * case volume trend's shape and window rules. One call, as for cases.
 *
 * The widget's flat filters are resolved for relative dates and
 * `__current_user__` first, as `useWidgetData` does for its PLG search, and
 * sent as that search sends them (`plgRegistrationTrendFilters`). The one
 * series is returned as each point's `opened`, with `closed` 0, so
 * `DashboardLineChart` draws it as one line (`series: ["opened"]`, labelled
 * "Registered" by the tile).
 *
 * `fixedWindow` asks for a fixed window instead of the trailing `days`,
 * exactly as `useCaseVolumeTrendData` does.
 */
export function usePlgRegistrationTrendData(
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
  // A fixed window replaces `days` (entity-service refuses both together).
  const range = fixedWindow
    ? {
        startDate: windowDate(fixedWindow.startDate),
        endDate: windowDate(fixedWindow.endDate),
        ...(fixedWindow.bucket && { bucket: fixedWindow.bucket }),
      }
    : { days: days ?? DEFAULT_DAYS };

  const resolvedFilters = resolveCurrentUserPlaceholder(resolveRelativeDateFilters(baseFilters), currentUserId);
  const awaitingCurrentUser = hasCurrentUserPlaceholder(resolvedFilters);
  const body = { ...plgRegistrationTrendFilters(resolvedFilters), ...range };
  // The PLG filters have no team placeholder, but the fetch slot still queues
  // by the selected team, as every widget's does.
  const teamKey = JSON.stringify([selectedTeamCreGroupId, selectedTeamSreGroupId]);

  const query = useQuery({
    queryKey: [ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA, "plg-registration-trend", widgetId, body],
    queryFn: async (): Promise<BeCaseVolumeTrendResponse> => {
      return withWidgetFetchSlot(async (signal) => {
        return api.post<typeof body, BeCaseVolumeTrendResponse>("/plg/registrations/volume-trend", body, {
          signal,
        });
      }, teamKey);
    },
    enabled: enabled && !awaitingCurrentUser,
    retry: (failureCount, error) => shouldRetryWidgetFetch(failureCount, error, true),
    staleTime: 300_000,
  });

  const registered = query.data?.series?.find((s) => s.label === "Registered")?.points ?? [];
  return {
    points: registered.map((p) => ({
      date: p.date,
      ...(p.endDate && { endDate: p.endDate }),
      opened: p.count,
      closed: 0,
    })),
    isLoading: !enabled || awaitingCurrentUser || query.isLoading,
    isError: query.isError,
    // A viewer without PLG access gets a 403; the tile says so instead of
    // an error, as for every other PLG widget.
    isForbidden: query.isError && (query.error as { status?: unknown } | null)?.status === 403,
  };
}
