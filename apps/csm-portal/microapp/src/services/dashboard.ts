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

import { queryOptions } from "@tanstack/react-query";
import type { CaseSearchFiltersDto, CaseSeverity, CaseState } from "@src/types";
import { ALL_SEVERITIES } from "@components/support/config";
import { ASSIGNED_TO_ME_STATES, COMPOSITION_STATES } from "@components/home/config";
import { CASES_AGGREGATE_ENDPOINT } from "@config/endpoints";
import apiClient from "./apiClient";
import { getAllCases, type CaseSearchResult } from "./cases";

// Mirrors the webapp's CaseComposition (useCaseComposition.ts): a 1-D breakdown by severity and a
// 1-D breakdown by state, each over active cases only (closed excluded). The closed count is
// returned separately so a large closed backlog can't skew the active totals.
export interface CaseComposition {
  bySeverity: Record<CaseSeverity, number>;
  byState: Record<CaseState, number>;
  severityTotal: number;
  stateTotal: number;
  closedTotal: number;
}

const EMPTY_RESULT: CaseSearchResult = { items: [], total: 0, limit: 0, offset: 0, hasMore: false };

interface AggregateBucketDto {
  key?: string;
  label?: string;
  count?: number;
}

interface AggregateResponseDto {
  groups?: AggregateBucketDto[];
  othersCount?: number;
  totalRecords?: number;
}

const aggregateCases = async (groupBy: "severity" | "state", filters: CaseSearchFiltersDto) => {
  const { data } = await apiClient.post<AggregateResponseDto>(CASES_AGGREGATE_ENDPOINT, { filters, groupBy });
  return data;
};

const normalizeBucketName = (raw: string | undefined): string =>
  (raw ?? "")
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, "_");

// Maps aggregate buckets onto the known values. Returns null when any non-empty bucket cannot be
// matched (or part of the result was folded into an "others" remainder), so the caller can fall
// back to the exact per-value counts instead of showing wrong numbers.
function bucketsToCounts<T extends string>(
  response: AggregateResponseDto,
  values: readonly T[],
  match: (normalized: string) => T | undefined,
): Record<T, number> | null {
  if ((response.othersCount ?? 0) > 0 || !Array.isArray(response.groups)) return null;
  const counts = Object.fromEntries(values.map((v) => [v, 0])) as Record<T, number>;
  for (const bucket of response.groups) {
    const count = bucket.count ?? 0;
    if (count === 0) continue;
    const value = match(normalizeBucketName(bucket.key)) ?? match(normalizeBucketName(bucket.label));
    if (!value) return null;
    counts[value] += count;
  }
  return counts;
}

// Severity values may arrive decorated (for example "1 - Critical"), so also accept a bucket name
// that contains exactly one severity word.
const severityMatch = (normalized: string): CaseSeverity | undefined => {
  const exact = ALL_SEVERITIES.find((s) => s === normalized);
  if (exact) return exact;
  const contained = ALL_SEVERITIES.filter((s) => normalized.split("_").includes(s));
  return contained.length === 1 ? contained[0] : undefined;
};

const ALL_COMPOSITION_STATES: CaseState[] = [...COMPOSITION_STATES, "closed"];
const stateMatch = (normalized: string): CaseState | undefined => ALL_COMPOSITION_STATES.find((s) => s === normalized);

// Count-only searches (`limit: 1`, read `.total`): one per severity (scoped to active states), one
// per active state (scoped to every severity), plus one for the closed total: 11 requests. Used
// only as the fallback when the aggregate buckets cannot be mapped.
async function fetchCompositionByCounts(): Promise<CaseComposition> {
  const countOf = (filters: CaseSearchFiltersDto): Promise<number> =>
    getAllCases({ filters: { types: ["case"], ...filters }, pagination: { limit: 1 } }).then((r) => r.total);

  const [severityCounts, stateCounts, closedTotal] = await Promise.all([
    Promise.all(
      ALL_SEVERITIES.map((severity) =>
        countOf({ severities: [severity], states: COMPOSITION_STATES }).then((n) => [severity, n] as const),
      ),
    ),
    Promise.all(
      COMPOSITION_STATES.map((state) =>
        countOf({ states: [state], severities: ALL_SEVERITIES }).then((n) => [state, n] as const),
      ),
    ),
    countOf({ states: ["closed"], severities: ALL_SEVERITIES }),
  ]);

  const bySeverity = Object.fromEntries(severityCounts) as Record<CaseSeverity, number>;
  const byState = Object.fromEntries(stateCounts) as Record<CaseState, number>;
  const severityTotal = severityCounts.reduce((sum, [, n]) => sum + n, 0);
  const stateTotal = stateCounts.reduce((sum, [, n]) => sum + n, 0);

  return { bySeverity, byState, severityTotal, stateTotal, closedTotal };
}

// Two aggregate requests (by severity over active states, by state over active plus closed)
// replace the 11 count searches; any response that cannot be mapped cleanly falls back to them.
async function fetchComposition(): Promise<CaseComposition> {
  try {
    const [severityResponse, stateResponse] = await Promise.all([
      aggregateCases("severity", { types: ["case"], states: COMPOSITION_STATES }),
      aggregateCases("state", { types: ["case"], severities: ALL_SEVERITIES, states: ALL_COMPOSITION_STATES }),
    ]);
    const bySeverity = bucketsToCounts(severityResponse, ALL_SEVERITIES, severityMatch);
    const stateCounts = bucketsToCounts(stateResponse, ALL_COMPOSITION_STATES, stateMatch);
    if (bySeverity && stateCounts) {
      const byState = { ...stateCounts, reopened: 0 } as Record<CaseState, number>;
      const severityTotal = Object.values(bySeverity).reduce((sum, n) => sum + n, 0);
      const stateTotal = COMPOSITION_STATES.reduce((sum, state) => sum + byState[state], 0);
      return { bySeverity, byState, severityTotal, stateTotal, closedTotal: stateCounts.closed };
    }
  } catch {
    // Fall through to the per-value counts below.
  }
  return fetchCompositionByCounts();
}

export const dashboard = {
  // Same active-vs-closed split the webapp's dashboard donuts show; staleTime keeps a page
  // revisit from re-firing the requests immediately.
  composition: () =>
    queryOptions({
      queryKey: ["dashboard", "composition"],
      queryFn: fetchComposition,
      staleTime: 60_000,
    }),

  // The signed-in user's own non-closed cases, newest-updated first — mirrors the webapp's
  // MyAssignedCases widget (useGetMyAssignedOpenCases.ts: same assignedUserId/state/type filters,
  // same sort, same 30s staleTime), capped to a short preview (4) rather than paginated — the rest
  // are a "View all" tap away on Support.
  assignedToMe: (userId: string | null) =>
    queryOptions({
      queryKey: ["dashboard", "assigned-to-me", userId],
      queryFn: () => {
        if (!userId) return Promise.resolve(EMPTY_RESULT);
        return getAllCases({
          filters: { types: ["case"], assignedUserIds: [userId], states: ASSIGNED_TO_ME_STATES },
          sortBy: { field: "updatedOn", order: "desc" },
          pagination: { limit: 4 },
        });
      },
      enabled: !!userId,
      staleTime: 30_000,
    }),
};
