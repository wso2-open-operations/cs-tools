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

// TanStack React Query hooks wrapping the api client's endpoint functions.
import { useEffect, useRef } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./endpoints";
import type { GlobalFilters, IssueFilters, Taxonomy } from "./types";

/**
 * GET /metrics/overview, optionally scoped to repo/priority/abtTeam; polls
 * every 60s. Keeps the previous filter's data on screen (isPlaceholderData)
 * while a new filter's fetch is in flight, instead of flipping isLoading.
 * staleTime mirrors the backend's overviewCacheTTL (30s, metrics.go), so
 * toggling a filter off and back on inside that window is a pure client
 * cache hit.
 */
export function useOverview(filters: GlobalFilters = {}) {
  return useQuery({
    queryKey: ["overview", filters.repo, filters.priority, filters.abtTeam],
    queryFn: () => api.getOverview(filters),
    refetchInterval: 60_000,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}

// Taxonomy changes only on reseed — cache indefinitely, no background refetch.
export function useTaxonomy() {
  return useQuery({
    queryKey: ["taxonomy"],
    queryFn: () => api.getTaxonomy(),
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });
}

/** Returns `false` for every status while taxonomy is loading (no layout shift). */
export function makeIsCsStatus(csStatuses: string[] | undefined) {
  return (status: string | null | undefined): boolean => csStatuses != null && csStatuses.includes(status ?? "");
}

/**
 * GET /metrics/timeseries for the given filters. staleTime mirrors the
 * backend's timeseriesCacheTTL (60s, metrics.go); see useOverview.
 */
export function useTimeseries(params: {
  repo?: string;
  metric?: string;
  days?: number;
  groupBy?: string;
  abtTeam?: string;
}) {
  return useQuery({
    queryKey: ["timeseries", params],
    queryFn: () => api.getTimeseries(params),
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}

/** GET /issues for the given filters. Keeps the previous filter's rows on screen while a new fetch is in flight. */
export function useIssues(filters: IssueFilters) {
  return useQuery({
    queryKey: ["issues", filters],
    queryFn: () => api.listIssues(filters),
    placeholderData: keepPreviousData,
  });
}

/** GET /sync/status; polls every 60s. */
export function useSyncStatus() {
  return useQuery({
    queryKey: ["sync-status"],
    queryFn: () => api.getSyncStatus(),
    refetchInterval: 60_000,
  });
}

/** Latest lastSyncedAt across repos, or null before any repo has synced. */
function newestWatermark(status: { repos: Array<{ lastSyncedAt: string | null }> } | undefined): string | null {
  const times = (status?.repos ?? []).map((r) => r.lastSyncedAt).filter((t): t is string => t != null);
  return times.length === 0 ? null : times.reduce((newest, t) => (t > newest ? t : newest));
}

/**
 * Refreshes the data views when a sync (scheduled, manual, or from another
 * replica) lands new data. Watches the already-polled /sync/status and, when
 * the newest per-repo watermark moves, invalidates the queries that don't poll
 * on their own. The overview keeps its own 60s poll too, because the recompute
 * tick moves SLA states between syncs without touching any watermark. The
 * first load only records the baseline, so opening the page never double-fetches.
 */
export function useRefreshOnSync() {
  const queryClient = useQueryClient();
  const { data } = useSyncStatus();
  const seen = useRef<string | null | undefined>(undefined);

  useEffect(() => {
    if (!data) return;
    const newest = newestWatermark(data);
    const previous = seen.current;
    seen.current = newest;
    if (previous === undefined || previous === newest) return;
    for (const key of ["overview", "issues", "timeseries", "issue"]) {
      void queryClient.invalidateQueries({ queryKey: [key] });
    }
  }, [data, queryClient]);
}

/** GET /issues/{id}; disabled until `enabled` (the row's timeline is expanded). */
export function useIssue(id: number, enabled: boolean) {
  return useQuery({
    queryKey: ["issue", id],
    queryFn: () => api.getIssue(id),
    enabled,
  });
}

/** Triggers POST /sync/runs and invalidates the queries its result affects. */
export function useManualSync() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.postSyncRuns(),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["sync-status"] });
      void queryClient.invalidateQueries({ queryKey: ["overview"] });
      void queryClient.invalidateQueries({ queryKey: ["issues"] });
      void queryClient.invalidateQueries({ queryKey: ["timeseries"] });
    },
    // A refused POST (429 cooldown, 409 in progress) means the status the UI
    // is showing is stale: refetch it so the cooldown countdown is accurate.
    onError: () => {
      void queryClient.invalidateQueries({ queryKey: ["sync-status"] });
    },
  });
}

/** Reserved `status` filter value matching any status not listed in the taxonomy. */
export const OTHER_STATUS = "Other";

/** Maps a raw board status to its configured display name (unchanged when none). */
export function makeStatusLabel(taxonomy: Taxonomy | undefined) {
  const byName = new Map((taxonomy?.statuses ?? []).map((s) => [s.name, s.displayName]));
  return (status: string): string => byName.get(status) || status;
}
