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
  keepPreviousData,
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type {
  ScheduleAbsenceKind,
  ScheduleTier,
  ScheduleAbsencesResponse,
  ScheduleActivity,
  ScheduleAssignment,
  ScheduleAssignmentsResponse,
  ScheduleCatalogue,
  ScheduleEditMarker,
  ScheduleEditMarkersResponse,
  SearchScheduleAbsencesPayload,
  SearchScheduleAssignmentsPayload,
} from "../types";

/**
 * The catalogue changes when a lead adds a window, which is roughly never
 * within a session -- so it is cached for the session rather than refetched
 * alongside every rota read.
 */
const CATALOGUE_STALE_MS = 30 * 60_000;

/** A rota read is cheap and the roster does change during a shift handover. */
const ROTA_STALE_MS = 60_000;

const QK = {
  catalogue: ["team-schedule", "catalogue"] as const,
  assignments: (p: SearchScheduleAssignmentsPayload) =>
    ["team-schedule", "assignments", p] as const,
  absences: (p: SearchScheduleAbsencesPayload) => ["team-schedule", "absences", p] as const,
  leadTeams: ["team-schedule", "my-lead-teams"] as const,
  editMarkers: (from: string, to: string) =>
    ["team-schedule", "edit-markers", from, to] as const,
};

/**
 * The zones, windows and absence kinds every view needs before it can draw
 * anything. Served as one payload, so one query.
 */
export function useScheduleCatalogue(): UseQueryResult<ScheduleCatalogue, Error> {
  const api = useBackendApi();
  return useQuery<ScheduleCatalogue, Error>({
    queryKey: QK.catalogue,
    queryFn: async () => (await api.get<ScheduleCatalogue>("/team-schedule/catalogue")) ?? {
      zones: [],
      shifts: [],
      absenceKinds: [],
      teams: [],
    },
    staleTime: CATALOGUE_STALE_MS,
  });
}

/** Who is working over a date window. */
export function useScheduleAssignments(
  payload: SearchScheduleAssignmentsPayload,
  enabled = true,
): UseQueryResult<ScheduleAssignmentsResponse, Error> {
  const api = useBackendApi();
  return useQuery<ScheduleAssignmentsResponse, Error>({
    queryKey: QK.assignments(payload),
    queryFn: () =>
      api.post<SearchScheduleAssignmentsPayload, ScheduleAssignmentsResponse>(
        "/team-schedule/assignments/search",
        payload,
      ),
    enabled,
    placeholderData: keepPreviousData,
    staleTime: ROTA_STALE_MS,
  });
}

/** A calendar month as the from/to pair a rota search takes. */
export interface MonthWindow {
  from: string;
  to: string;
}

/** The parts of a query result the page reads -- all a merged read can offer. */
export interface RotaRead<T> {
  data: T | undefined;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
}

/**
 * The rota over several calendar months, read one month per request.
 *
 * One request per month, not one for the whole span: entity-service caps a
 * single read at 70 days, and three months is ninety-odd. It is also the
 * cheaper shape to page through -- each month is cached under the same key a
 * one-month read would use, so stepping the window forward refetches only the
 * month that came into view, not the two already on screen.
 */
export function useScheduleAssignmentsByMonth(
  payload: Omit<SearchScheduleAssignmentsPayload, "from" | "to">,
  months: readonly MonthWindow[],
  enabled = true,
): RotaRead<ScheduleAssignmentsResponse> {
  const api = useBackendApi();
  return useQueries({
    queries: months.map((m) => {
      const p: SearchScheduleAssignmentsPayload = { ...payload, from: m.from, to: m.to };
      return {
        queryKey: QK.assignments(p),
        queryFn: () =>
          api.post<SearchScheduleAssignmentsPayload, ScheduleAssignmentsResponse>(
            "/team-schedule/assignments/search",
            p,
          ),
        enabled,
        // No keepPreviousData: inside useQueries it carries the previous
        // *slot's* data, so a moved window briefly showed another month's rows.
        staleTime: ROTA_STALE_MS,
      };
    }),
    combine: (results): RotaRead<ScheduleAssignmentsResponse> => {
      const failed = results.find((r) => r.isError);
      const assignments = results.flatMap((r) => r.data?.assignments ?? []);
      return {
        data: results.some((r) => r.data)
          ? { assignments, count: assignments.length }
          : undefined,
        isLoading: results.some((r) => r.isLoading),
        isError: Boolean(failed),
        error: failed?.error ?? null,
      };
    },
  });
}

/**
 * Absences over several calendar months, one request per month.
 *
 * An absence spanning a month boundary comes back from both months it
 * touches, so the merge keeps one copy per id -- otherwise it would draw
 * twice on the roster.
 */
export function useScheduleAbsencesByMonth(
  payload: Omit<SearchScheduleAbsencesPayload, "from" | "to">,
  months: readonly MonthWindow[],
  enabled = true,
): RotaRead<ScheduleAbsencesResponse> {
  const api = useBackendApi();
  return useQueries({
    queries: months.map((m) => {
      const p: SearchScheduleAbsencesPayload = { ...payload, from: m.from, to: m.to };
      return {
        queryKey: QK.absences(p),
        queryFn: () =>
          api.post<SearchScheduleAbsencesPayload, ScheduleAbsencesResponse>(
            "/team-schedule/absences/search",
            p,
          ),
        enabled,
        // No keepPreviousData: inside useQueries it carries the previous
        // *slot's* data, so a moved window briefly showed another month's rows.
        staleTime: ROTA_STALE_MS,
      };
    }),
    combine: (results): RotaRead<ScheduleAbsencesResponse> => {
      const failed = results.find((r) => r.isError);
      const byId = new Map<string, ScheduleAbsencesResponse["absences"][number]>();
      for (const r of results) for (const a of r.data?.absences ?? []) byId.set(a.id, a);
      const absences = [...byId.values()];
      return {
        data: results.some((r) => r.data) ? { absences, count: absences.length } : undefined,
        isLoading: results.some((r) => r.isLoading),
        isError: Boolean(failed),
        error: failed?.error ?? null,
      };
    },
  });
}

/**
 * The recorded changes to the rota and leave of the teams a lead leads, over
 * the months on screen, newest first.
 *
 * One read per team per month. Per team because the history is kept, and
 * authorised, per team -- only a team's lead may read it. Per month because
 * entity-service caps a read at 70 days and the roster shows three months: a
 * single read of the whole window is refused.
 *
 * A span of leave that crosses a month boundary comes back from both months,
 * so the merge keeps one copy per id.
 */
export function useTeamActivity(
  teamKeys: readonly string[],
  months: readonly MonthWindow[],
  enabled = true,
): RotaRead<ScheduleActivity[]> {
  const api = useBackendApi();
  return useQueries({
    queries: teamKeys.flatMap((teamKey) =>
      months.map((m) => ({
        queryKey: ["team-schedule", "activity", teamKey, m.from, m.to] as const,
        queryFn: async () =>
          (
            await api.get<{ activity: ScheduleActivity[] }>(
              `/team-schedule/activity?teamKey=${encodeURIComponent(teamKey)}&from=${m.from}&to=${m.to}`,
            )
          )?.activity ?? [],
        enabled,
        staleTime: ROTA_STALE_MS,
      })),
    ),
    combine: (results): RotaRead<ScheduleActivity[]> => {
      const failed = results.find((r) => r.isError);
      const byId = new Map<string, ScheduleActivity>();
      for (const r of results) for (const a of r.data ?? []) byId.set(a.id, a);
      return {
        data: [...byId.values()].sort((a, b) => b.createdOn.localeCompare(a.createdOn)),
        isLoading: results.some((r) => r.isLoading),
        isError: Boolean(failed),
        error: failed?.error ?? null,
      };
    },
  });
}

/** Who is out of the rota over a date window. */
export function useScheduleAbsences(
  payload: SearchScheduleAbsencesPayload,
  enabled = true,
): UseQueryResult<ScheduleAbsencesResponse, Error> {
  const api = useBackendApi();
  return useQuery<ScheduleAbsencesResponse, Error>({
    queryKey: QK.absences(payload),
    queryFn: () =>
      api.post<SearchScheduleAbsencesPayload, ScheduleAbsencesResponse>(
        "/team-schedule/absences/search",
        payload,
      ),
    enabled,
    placeholderData: keepPreviousData,
    staleTime: ROTA_STALE_MS,
  });
}

/**
 * Which teams this reader may edit.
 *
 * Asked once and cached: it changes when somebody is made a lead, not while
 * they are looking at a rota. The page uses it to decide whether to offer an
 * edit control at all -- entity-service refuses the write either way, but a
 * control that always fails is worse than no control.
 */
export function useMyLeadTeams(): UseQueryResult<string[], Error> {
  const api = useBackendApi();
  return useQuery<string[], Error>({
    queryKey: QK.leadTeams,
    queryFn: async () => {
      const r = await api.get<{ teamKeys?: string[] }>("/team-schedule/my-lead-teams");
      return r?.teamKeys ?? [];
    },
    staleTime: 5 * 60 * 1000,
  });
}

/** Everything a write touches, so the views redraw from the server rather than
 *  from an optimistic guess about what the server did. */
function invalidateRota(qc: ReturnType<typeof useQueryClient>): void {
  void qc.invalidateQueries({ queryKey: ["team-schedule", "assignments"] });
  void qc.invalidateQueries({ queryKey: ["team-schedule", "absences"] });
  void qc.invalidateQueries({ queryKey: ["team-schedule", "activity"] });
  // The marks on the roster come from here. Without this they keep the last
  // answer for a minute, so a lead who has just changed a cell and left edit
  // mode sees a grid saying nobody has touched it.
  void qc.invalidateQueries({ queryKey: ["team-schedule", "edit-markers"] });
}

export interface CreateAssignmentPayload {
  userId: string;
  teamKey: string;
  shiftCode: string;
  rotaDate: string;
  tier?: string | null;
  isOnCall?: boolean | null;
  note?: string | null;
}

/** Put somebody on a window. Lead only; the server decides. */
export function useCreateAssignment(): UseMutationResult<
  ScheduleAssignment,
  Error,
  CreateAssignmentPayload
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<ScheduleAssignment, Error, CreateAssignmentPayload>({
    mutationFn: (payload) =>
      api.post<CreateAssignmentPayload, ScheduleAssignment>("/team-schedule/assignments", payload),
    onSuccess: () => invalidateRota(qc),
  });
}

export interface UpdateAssignmentPayload {
  id: string;
  userId?: string;
  tier?: string;
  isOnCall?: boolean;
  note?: string;
}

/** Change who holds a slot. */
export function useUpdateAssignment(): UseMutationResult<
  ScheduleAssignment,
  Error,
  UpdateAssignmentPayload
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<ScheduleAssignment, Error, UpdateAssignmentPayload>({
    mutationFn: ({ id, ...body }) =>
      api.patch<Omit<UpdateAssignmentPayload, "id">, ScheduleAssignment>(
        `/team-schedule/assignments/${encodeURIComponent(id)}`,
        body,
      ),
    onSuccess: () => invalidateRota(qc),
  });
}

/** Take somebody off a slot. */
export function useDeleteAssignment(): UseMutationResult<
  unknown,
  Error,
  { id: string; note?: string }
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<unknown, Error, { id: string; note?: string }>({
    mutationFn: ({ id, note }) =>
      api.del(
        `/team-schedule/assignments/${encodeURIComponent(id)}` +
          (note ? `?note=${encodeURIComponent(note)}` : ""),
      ),
    onSuccess: () => invalidateRota(qc),
  });
}

export interface ApplyRangePayload {
  userId: string;
  teamKey: string;
  shiftCode: string;
  from: string;
  to: string;
  note?: string;
  /** L1, L2 or L3 on an escalation window that leaves the tier to the
   *  person. Omitted takes the window's own. */
  tier?: ScheduleTier;
  /** With an empty shiftCode, clears only this zone's turn that day. */
  zoneCode?: string;
}

export interface ApplyRangeResult {
  applied: number;
  skipped: number;
  skippedDates: string[];
}

/**
 * Set one engineer to one window across a span of days.
 *
 * One call for the whole span rather than one per day: the server does it in a
 * transaction, so a range cannot end up half applied, and the answer says how
 * many days were actually set -- which is routinely fewer than were asked for,
 * because a weekday rotation skips the weekend inside the range.
 */
export function useApplyRange(): UseMutationResult<ApplyRangeResult, Error, ApplyRangePayload> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<ApplyRangeResult, Error, ApplyRangePayload>({
    mutationFn: (payload) =>
      api.post<ApplyRangePayload, ApplyRangeResult>("/team-schedule/assignments/apply", payload),
    onSuccess: () => invalidateRota(qc),
  });
}

export interface ApplyAbsencePayload {
  userId: string;
  teamKey: string;
  /** Empty brings them back over the span instead of marking it. */
  kindCode: string;
  from: string;
  to: string;
  note?: string;
  /** Who an allocation is for: the customer, or the product team for RnD.
   *  The server keeps it only with an allocation kind. */
  allocatedTo?: string;
}

export interface ApplyAbsenceResult {
  created: number;
  removed: number;
  trimmed: number;
}

/**
 * Mark one engineer away across a span, or bring them back over it.
 *
 * Separate from `useApplyRange` because leave is a separate table, not a rota
 * window: it is stored as a span rather than resolved day by day, so a weekend
 * inside the range is covered too. The server reconciles whatever is already
 * there -- shortening a stretch of leave the span clips rather than cancelling
 * it -- so the caller does not have to read the absences first.
 */
export function useApplyAbsence(): UseMutationResult<ApplyAbsenceResult, Error, ApplyAbsencePayload> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<ApplyAbsenceResult, Error, ApplyAbsencePayload>({
    mutationFn: (payload) =>
      api.post<ApplyAbsencePayload, ApplyAbsenceResult>("/team-schedule/absences/apply", payload),
    onSuccess: () => invalidateRota(qc),
  });
}

/**
 * Remove one absence whole -- every day of it, and an open-ended one too.
 *
 * Not the same as clearing a span with `useApplyAbsence`: that needs dates, and
 * a standing allocation "until further notice" has no end date to name. This is
 * what the picker's Remove does, so one click takes the whole thing away.
 */
export function useDeleteAbsence(): UseMutationResult<
  unknown,
  Error,
  { id: string; note?: string }
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<unknown, Error, { id: string; note?: string }>({
    mutationFn: ({ id, note }) =>
      api.del(
        `/team-schedule/absences/${encodeURIComponent(id)}` +
          (note ? `?note=${encodeURIComponent(note)}` : ""),
      ),
    onSuccess: () => invalidateRota(qc),
  });
}

export interface CreateAbsenceKindPayload {
  shortCode: string;
  label: string;
  bucket: "LEAVE" | "ALLOCATION";
  colourToken: string;
}

/**
 * Add a leave or allocation tag to the shared catalogue. Every team sees it
 * once it exists, so the catalogue is refetched rather than patched locally.
 */
export function useCreateAbsenceKind(): UseMutationResult<
  ScheduleAbsenceKind,
  Error,
  CreateAbsenceKindPayload
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<ScheduleAbsenceKind, Error, CreateAbsenceKindPayload>({
    mutationFn: (payload) =>
      api.post<CreateAbsenceKindPayload, ScheduleAbsenceKind>("/team-schedule/absence-kinds", payload),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: QK.catalogue });
    },
  });
}

/**
 * Delete a tag a lead added. The server refuses one of the catalogue's own,
 * or one still in use, so a failure here is worth showing as it is.
 */
export function useDeleteAbsenceKind(): UseMutationResult<unknown, Error, string> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<unknown, Error, string>({
    mutationFn: (code) => api.del(`/team-schedule/absence-kinds/${encodeURIComponent(code)}`),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: QK.catalogue });
    },
  });
}

/**
 * Which roster cells somebody changed by hand.
 *
 * Fetched a month at a time, like the rota itself: a single read is capped at
 * 70 days and the roster spans three months, so one request for the whole span
 * is refused. The months are the same ones the rota was fetched in, so the two
 * share a cache key shape and a month already on screen is not asked for twice.
 *
 * Its own query rather than part of the rota's, and deliberately so: the grid
 * renders from the rota alone and picks these up when they arrive. A
 * three-month grid is slow enough to build without waiting on a second call
 * before anything can be drawn, and a missing mark is a far smaller problem
 * than a late page.
 *
 * Only edits made by a person come back -- a handful of rows a month, against
 * twelve thousand assignments -- so the payload stays small however wide the
 * grid gets.
 */
export function useScheduleEditMarkers(
  months: readonly MonthWindow[],
  enabled: boolean,
): { markers: ScheduleEditMarker[] } {
  const api = useBackendApi();
  return useQueries({
    queries: months.map((m) => ({
      queryKey: QK.editMarkers(m.from, m.to),
      queryFn: async () => {
        const r = await api.get<ScheduleEditMarkersResponse>(
          `/team-schedule/edit-markers?from=${m.from}&to=${m.to}`,
        );
        return r ?? { markers: [], count: 0 };
      },
      enabled,
      staleTime: ROTA_STALE_MS,
    })),
    combine: (results) => ({
      markers: results.flatMap((r) => r.data?.markers ?? []),
    }),
  });
}
