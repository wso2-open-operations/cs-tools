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
  const { data: teams } = useTeams(true, "sre-abt");
  const { data: extraGroups } = useSearchIncidentExtraGroups("", true);
  const search = useSearchGroups(query, enabled);
  const q = query.trim().toLowerCase();

  const data = useMemo(() => {
    const groups: IncidentFilterGroup[] = [];
    const seen = new Set<string>();
    const add = (id: string | undefined, name: string, matchName: boolean): void => {
      if (!id || seen.has(id)) return;
      if (matchName && q && !name.toLowerCase().includes(q)) return;
      seen.add(id);
      groups.push({ id, name });
    };
    for (const t of teams ?? []) {
      if (t.family === "sre-abt") add(t.sreGroupId, t.name, true);
    }
    for (const g of extraGroups ?? []) add(g.id, g.name, true);
    for (const g of search.data ?? []) add(g.id, g.name, false);
    return groups;
  }, [teams, extraGroups, search.data, q]);

  return { data, isFetching: search.isFetching, isError: search.isError };
}

/**
 * Names of the selected assignment groups, by id, for a filter restored from
 * the URL or a saved view: a group the picker has not searched for this session
 * would otherwise show as its raw id. One `GET /groups/{id}` per id, under the
 * same query key as `useGroupDetail`, so a group looked up elsewhere is not
 * fetched again. An id still loading or not found is just missing from the map.
 */
export function useIncidentFilterGroupLabels(ids: string[]): Record<string, string> {
  const api = useBackendApi();
  const results = useQueries({
    queries: ids.map((id) => ({
      queryKey: [ApiQueryKeys.GROUP_DETAILS, id],
      queryFn: (): Promise<BeGroupDetail | null> =>
        api.get<BeGroupDetail>(`/groups/${encodeURIComponent(id)}`),
      staleTime: 60_000,
    })),
  });

  const labels: Record<string, string> = {};
  ids.forEach((id, i) => {
    const name = results[i]?.data?.name;
    if (name) labels[id] = name;
  });
  return labels;
}
