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

import { useMemo } from "react";
import { useQueries } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeGroupDetail } from "@api/backend/types";
import { useSearchGroups, useSearchIncidentExtraGroups } from "@api/useSearchGroups";
import { useTeams } from "@features/csm-dashboard/api/useTeams";

export interface IncidentFilterGroup {
  id: string;
  name: string;
}

export interface IncidentFilterGroupsResult {
  data: IncidentFilterGroup[];
  isFetching: boolean;
  isError: boolean;
}

/**
 * The incidents list's "Assignment group" filter options. The groups incidents
 * are usually assigned to come first: the `sre-abt` teams (by their SRE group
 * id) and the extra groups an incident may carry (the security operations
 * group), as the filter offered before. Every other group the type-ahead group
 * search (`POST /groups/search`) finds follows, so an incident assigned to any
 * group can be filtered. Deduped by id; what is typed narrows the first part by
 * name and is the search's own query.
 *
 * The first part is loaded whether or not the picker is open, so a selected
 * SRE team or extra group is always labelled; a stable module-level hook so it
 * can be handed to `AsyncEntityMultiSelect`'s `useSearch`. A failed or loading
 * request just yields fewer options: neither part waits on the other.
 */
export function useSearchIncidentFilterGroups(
  query: string,
  enabled: boolean,
): IncidentFilterGroupsResult {
  const pinned = usePinnedIncidentFilterGroups();
  const search = useSearchGroups(query, enabled);
  // While a new query is in flight the search keeps the previous query's
  // results as placeholder data; listing them would offer groups that do not
  // match what is now typed, so only the pinned part shows until it returns.
  const searchData = search.isPlaceholderData ? undefined : search.data;
  const q = query.trim().toLowerCase();

  const data = useMemo(() => {
    const groups: IncidentFilterGroup[] = [];
    const seen = new Set<string>();
    for (const g of pinned) {
      seen.add(g.id);
      if (!q || g.name.toLowerCase().includes(q)) groups.push(g);
    }
    for (const g of searchData ?? []) {
      if (!g.id || seen.has(g.id)) continue;
      seen.add(g.id);
      groups.push({ id: g.id, name: g.name });
    }
    return groups;
  }, [pinned, searchData, q]);

  return { data, isFetching: search.isFetching, isError: search.isError };
}

/**
 * The groups the filter always lists first: the `sre-abt` teams (by their SRE
 * group id) and the incident extra groups, deduped by id, the team's name
 * winning. Shared by the options and the labels so a pinned group is named the
 * same way in both.
 */
function usePinnedIncidentFilterGroups(): IncidentFilterGroup[] {
  const { data: teams } = useTeams(true, "sre-abt");
  const { data: extraGroups } = useSearchIncidentExtraGroups("", true);
  return useMemo(() => {
    const groups: IncidentFilterGroup[] = [];
    const seen = new Set<string>();
    const add = (id: string | undefined, name: string): void => {
      if (!id || seen.has(id)) return;
      seen.add(id);
      groups.push({ id, name });
    };
    for (const t of teams ?? []) {
      if (t.family === "sre-abt") add(t.sreGroupId, t.name);
    }
    for (const g of extraGroups ?? []) add(g.id, g.name);
    return groups;
  }, [teams, extraGroups]);
}

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Builds the id → name map from the lookups; a stable module-level function
 * so `useQueries` re-runs it only when a lookup's result changes. */
function combineGroupNames(
  results: { data?: BeGroupDetail | null }[],
): Record<string, string> {
  const names: Record<string, string> = {};
  for (const r of results) if (r.data?.id && r.data.name) names[r.data.id] = r.data.name;
  return names;
}

/**
 * Names of the selected assignment groups, by id, for a filter restored from
 * the URL or a saved view: a group the picker has not searched for this session
 * would otherwise show as its raw id. A pinned group (SRE team or extra group)
 * is named from the list the filter already loads; only the rest are looked up,
 * one `GET /groups/{id}` each, under the same query key as `useGroupDetail` so
 * a group looked up elsewhere is not fetched again. An id that is not a UUID
 * (e.g. a hand-edited URL) is never looked up, and a failed lookup is not
 * retried. An id still loading or not found is just missing from the map. The
 * map keeps its identity until a name in it changes.
 */
export function useIncidentFilterGroupLabels(ids: string[]): Record<string, string> {
  const api = useBackendApi();
  const pinned = usePinnedIncidentFilterGroups();
  const pinnedNames = useMemo(() => new Map(pinned.map((g) => [g.id, g.name])), [pinned]);
  const lookupIds = ids.filter((id) => !pinnedNames.has(id) && UUID_RE.test(id));

  const looked = useQueries({
    queries: lookupIds.map((id) => ({
      queryKey: [ApiQueryKeys.GROUP_DETAILS, id],
      queryFn: (): Promise<BeGroupDetail | null> =>
        api.get<BeGroupDetail>(`/groups/${encodeURIComponent(id)}`),
      staleTime: 60_000,
      retry: false,
    })),
    combine: combineGroupNames,
  });

  const idsKey = ids.join("\u0000");
  return useMemo(() => {
    const labels: Record<string, string> = {};
    for (const id of idsKey ? idsKey.split("\u0000") : []) {
      const name = pinnedNames.get(id) ?? looked[id];
      if (name) labels[id] = name;
    }
    return labels;
  }, [idsKey, pinnedNames, looked]);
}
