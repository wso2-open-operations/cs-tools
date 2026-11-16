/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { keepPreviousData, useQueries } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type {
  SchedulePublicHoliday,
  SchedulePublicHolidaysResponse,
  SearchSchedulePublicHolidaysPayload,
} from "../types";
import { chunk, splitWindow } from "../utils/engagementAllocations";

/** Holidays change once a day at most (the server refreshes daily). */
const HOLIDAYS_STALE_MS = 30 * 60_000;

export interface PublicHolidaysRead {
  holidays: SchedulePublicHoliday[];
  isError: boolean;
}

/**
 * Public holidays for a set of engineers over a date window, read-only, cut
 * into the same batches and windows as the allocation read (500 ids, 70 days
 * a request). Decoration on the roster: a failed piece leaves out its own
 * shading and nothing else.
 */
export function usePublicHolidays(
  userIds: readonly string[],
  from: string,
  to: string,
  enabled = true,
): PublicHolidaysRead {
  const api = useBackendApi();
  const payloads: SearchSchedulePublicHolidaysPayload[] = chunk(userIds).flatMap((ids) =>
    splitWindow(from, to).map((w) => ({ userIds: ids, from: w.from, to: w.to })),
  );
  return useQueries({
    queries: payloads.map((p) => ({
      queryKey: ["team-schedule", "public-holidays", p] as const,
      queryFn: () =>
        api.post<SearchSchedulePublicHolidaysPayload, SchedulePublicHolidaysResponse>(
          "/team-schedule/public-holidays/search",
          p,
        ),
      enabled,
      placeholderData: keepPreviousData,
      staleTime: HOLIDAYS_STALE_MS,
    })),
    combine: (results): PublicHolidaysRead => {
      const byKey = new Map<string, SchedulePublicHoliday>();
      for (const r of results) for (const h of r.data?.holidays ?? []) byKey.set(`${h.userId}|${h.date}`, h);
      return { holidays: [...byKey.values()], isError: results.some((r) => r.isError) };
    },
  });
}
