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

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { SlaCalendar, SlaWorkingDay } from "../types";

const SLA_CALENDARS_KEY = ["sla", "calendars"] as const;
const slaCalendarKey = (id: string) => [...SLA_CALENDARS_KEY, id] as const;

/** Every SLA calendar — exactly two exist (24x7, WSO2 Working Days, seeded
 *  by entity-service migration 0246); this API has no create/retire route
 *  (see this feature's own plan file, "Calendars are two seeded rows"). */
export function useSlaCalendars(): UseQueryResult<SlaCalendar[], Error> {
  const api = useBackendApi();
  return useQuery<SlaCalendar[], Error>({
    queryKey: SLA_CALENDARS_KEY,
    queryFn: async () => {
      const res = await api.get<{ calendars: SlaCalendar[] }>("/sla/calendars");
      return res?.calendars ?? [];
    },
    staleTime: 60 * 1000,
  });
}

export function useSlaCalendar(id: string | undefined): UseQueryResult<SlaCalendar, Error> {
  const api = useBackendApi();
  return useQuery<SlaCalendar, Error>({
    queryKey: slaCalendarKey(id ?? ""),
    queryFn: async () => {
      const res = await api.get<SlaCalendar>(`/sla/calendars/${encodeURIComponent(id ?? "")}`);
      if (res === null) throw new Error(`GET /sla/calendars/${id} returned 404`);
      return res;
    },
    enabled: Boolean(id),
  });
}

function invalidateCalendar(qc: ReturnType<typeof useQueryClient>, id: string): void {
  void qc.invalidateQueries({ queryKey: SLA_CALENDARS_KEY });
  void qc.invalidateQueries({ queryKey: slaCalendarKey(id) });
}

/** Replaces a calendar's whole 7-day working week in one call (R23). */
export function useUpdateSlaCalendarWorkingWeek(): UseMutationResult<
  SlaCalendar,
  Error,
  { id: string; workingWeek: SlaWorkingDay[] }
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<SlaCalendar, Error, { id: string; workingWeek: SlaWorkingDay[] }>({
    mutationFn: ({ id, workingWeek }) =>
      api.patch<{ workingWeek: SlaWorkingDay[] }, SlaCalendar>(`/sla/calendars/${encodeURIComponent(id)}`, {
        workingWeek,
      }),
    onSettled: (_data, _error, vars) => invalidateCalendar(qc, vars.id),
  });
}

export interface AddSlaHolidayInput {
  calendarId: string;
  startDate: string;
  endDate: string;
  name: string;
}

export function useAddSlaHoliday(): UseMutationResult<SlaCalendar, Error, AddSlaHolidayInput> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<SlaCalendar, Error, AddSlaHolidayInput>({
    mutationFn: ({ calendarId, ...body }) =>
      api.post<Omit<AddSlaHolidayInput, "calendarId">, SlaCalendar>(
        `/sla/calendars/${encodeURIComponent(calendarId)}/holidays`,
        body,
      ),
    onSettled: (_data, _error, vars) => invalidateCalendar(qc, vars.calendarId),
  });
}

export function useDeleteSlaHoliday(): UseMutationResult<
  void,
  Error,
  { calendarId: string; holidayId: string }
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<void, Error, { calendarId: string; holidayId: string }>({
    mutationFn: ({ calendarId, holidayId }) =>
      api.delete(`/sla/calendars/${encodeURIComponent(calendarId)}/holidays/${encodeURIComponent(holidayId)}`),
    onSettled: (_data, _error, vars) => invalidateCalendar(qc, vars.calendarId),
  });
}
