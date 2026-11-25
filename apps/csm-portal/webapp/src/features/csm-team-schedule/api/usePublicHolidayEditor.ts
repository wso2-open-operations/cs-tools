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

import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { AddPublicHolidaysPayload, PublicHolidaysList } from "../types";

const listKey = (from: string, to: string) => ["team-schedule", "public-holiday-editor", from, to] as const;

/**
 * After a change, the editor's list, the roster's shading and the holiday
 * availability panel all read the holidays again.
 */
function refreshHolidays(qc: QueryClient): void {
  void qc.invalidateQueries({ queryKey: ["team-schedule", "public-holiday-editor"] });
  void qc.invalidateQueries({ queryKey: ["team-schedule", "public-holidays"] });
  void qc.invalidateQueries({ queryKey: ["team-schedule", "holiday-availability"] });
}

/** The stored holidays in a window (every country), with canEdit and the countries in use. */
export function usePublicHolidayList(from: string, to: string): UseQueryResult<PublicHolidaysList, Error> {
  const api = useBackendApi();
  return useQuery<PublicHolidaysList, Error>({
    queryKey: listKey(from, to),
    queryFn: async () =>
      (await api.get<PublicHolidaysList>(
        `/team-schedule/public-holidays?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
      )) ?? { holidays: [], countries: [], canEdit: false },
    staleTime: 60 * 1000,
  });
}

/**
 * Adds one country's holidays by hand. A refusal (400 naming the day, 403 who
 * may) surfaces as the error's message.
 */
export function useAddPublicHolidays(): UseMutationResult<{ stored: number }, Error, AddPublicHolidaysPayload> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<{ stored: number }, Error, AddPublicHolidaysPayload>({
    mutationFn: (body) => api.post<AddPublicHolidaysPayload, { stored: number }>("/team-schedule/public-holidays", body),
    onSettled: () => refreshHolidays(qc),
  });
}

/** Removes one stored day. */
export function useRemovePublicHoliday(): UseMutationResult<void, Error, { country: string; date: string }> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<void, Error, { country: string; date: string }>({
    mutationFn: ({ country, date }) =>
      api.delete(`/team-schedule/public-holidays/${encodeURIComponent(country)}/${encodeURIComponent(date)}`),
    onSettled: () => refreshHolidays(qc),
  });
}
