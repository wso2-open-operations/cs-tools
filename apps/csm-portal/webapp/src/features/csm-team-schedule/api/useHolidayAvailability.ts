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

import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { HolidayAvailability, HolidayAvailabilityResponse } from "../types";

const KEY = ["team-schedule", "holiday-availability"] as const;

/**
 * Each CRE team's availability on each public holiday from `from` to `to`
 * (CRE only; the server places the Public holiday tags three weeks ahead).
 */
export function useHolidayAvailability(from: string, to: string, enabled = true): UseQueryResult<HolidayAvailabilityResponse> {
  const api = useBackendApi();
  return useQuery({
    queryKey: [...KEY, from, to],
    queryFn: () =>
      api.post<{ from: string; to: string }, HolidayAvailabilityResponse>("/team-schedule/holiday-availability/search", {
        from,
        to,
      }),
    enabled,
    staleTime: 60_000,
  });
}

export interface HolidayConfirmationInput {
  teamKey: string;
  date: string;
  /** true: confirm; false: withdraw the confirmation. */
  confirm: boolean;
}

/** Confirm (or withdraw) a team's availability on a holiday. The server
 *  refuses a confirm, in words, while a rota turn has nobody available. */
export function useHolidayConfirmation(): UseMutationResult<HolidayAvailability, Error, HolidayConfirmationInput> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<HolidayAvailability, Error, HolidayConfirmationInput>({
    mutationFn: async ({ teamKey, date, confirm }) => {
      const path = `/team-schedule/holiday-availability/${encodeURIComponent(teamKey)}/${encodeURIComponent(date)}/confirmation`;
      if (confirm) return api.put<Record<string, never>, HolidayAvailability>(path, {});
      const withdrawn = await api.del<HolidayAvailability>(path);
      if (!withdrawn) throw new Error("The confirmation could not be withdrawn.");
      return withdrawn;
    },
    onSettled: () => qc.invalidateQueries({ queryKey: KEY }),
  });
}
