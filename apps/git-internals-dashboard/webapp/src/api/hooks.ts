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
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./endpoints";
import type { IssueFilters } from "./types";

/**
 * GET /metrics/overview, optionally scoped to repo/priority; polls every 60s.
 * Keeps the previous filter's data on screen (isPlaceholderData) while a new
 * filter's fetch is in flight, instead of flipping isLoading. staleTime
 * mirrors the backend's overviewCacheTTL (30s, metrics.go), so toggling a
 * filter off and back on inside that window is a pure client cache hit.
 */
export function useOverview(repo?: string, priority?: string) {
  return useQuery({
    queryKey: ["overview", repo, priority],
    queryFn: () => api.getOverview({ repo, priority }),
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
export function useTimeseries(params: { repo?: string; metric?: string; days?: number; groupBy?: string }) {
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

/**
 * Runtime title resolution for a set of issue ids. Titles never touch the
 * DB; they are fetched through the API's GitHub proxy at render time. The
 * stable sorted key makes reordered lists hit the same cache entry.
 */
export function useIssueTitles(ids: number[]) {
  const key = [...ids].sort((a, b) => a - b).join(",");
  return useQuery({
    queryKey: ["issue-titles", key],
    queryFn: () => api.getIssueTitles(ids),
    enabled: ids.length > 0,
    staleTime: 10 * 60_000, // matches the server-side 15-min cache order of magnitude
    retry: 1,
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
  });
}
