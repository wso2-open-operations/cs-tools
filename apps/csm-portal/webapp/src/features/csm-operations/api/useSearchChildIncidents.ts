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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeIncident,
  BeIncidentSearchPayload,
  BeIncidentSearchResponse,
} from "@api/backend/types";

const CHILD_INCIDENTS_PAGE_LIMIT = BE_MAX_PAGE_LIMIT;
/** Safety bound on the paging loop, as in `useCsmIncidentComments.ts`. */
const MAX_CHILD_INCIDENT_PAGES = 200;

/**
 * Prefix of every child-incidents query. It sits under the incidents list
 * key, so the list invalidation every incident PATCH already does covers it.
 */
export const CHILD_INCIDENTS_QUERY_KEY = [ApiQueryKeys.INCIDENTS, "child-incidents"] as const;

export interface ChildIncidentRow {
  id: string;
  number?: string;
  subject: string;
  priority?: string | null;
  state?: string | null;
  assigneeName?: string;
}

export interface ChildIncidentsResult {
  incidents: ChildIncidentRow[];
  total: number;
}

/**
 * The child incidents of one incident: those whose `parentIncident` is it
 * (`POST /incidents/search` with `filters.parentIncidentIds`). Not the generic
 * `parentIds`, which matches the work item parent a case or change request
 * can also hold. Pages through every result, since the parent page counts
 * the open ones for the resolve dialog. Disabled until an id is provided.
 */
export function useSearchChildIncidents(
  parentIncidentId: string | undefined,
): UseQueryResult<ChildIncidentsResult, Error> {
  const api = useBackendApi();

  return useQuery<ChildIncidentsResult, Error>({
    queryKey: [...CHILD_INCIDENTS_QUERY_KEY, parentIncidentId ?? ""],
    queryFn: async (): Promise<ChildIncidentsResult> => {
      const rows: BeIncident[] = [];
      for (let page = 0; page < MAX_CHILD_INCIDENT_PAGES; page += 1) {
        const res = await api.post<BeIncidentSearchPayload, BeIncidentSearchResponse>(
          "/incidents/search",
          {
            filters: { parentIncidentIds: [parentIncidentId as string] },
            pagination: {
              offset: page * CHILD_INCIDENTS_PAGE_LIMIT,
              limit: CHILD_INCIDENTS_PAGE_LIMIT,
            },
          },
        );
        const pageRows = res.incidents ?? [];
        rows.push(...pageRows);
        // Incident search has no `hasMore`: a short page or the reported
        // total ends it.
        if (pageRows.length < CHILD_INCIDENTS_PAGE_LIMIT || rows.length >= (res.total ?? 0)) break;
      }
      const incidents = rows
        .filter((i): i is typeof i & { id: string } => !!i.id)
        .map((i) => ({
          id: i.id,
          number: i.number ?? undefined,
          subject: i.subject ?? "(no subject)",
          priority: i.priority,
          state: i.state,
          assigneeName: i.assignedTo?.name ?? undefined,
        }));
      return { incidents, total: incidents.length };
    },
    enabled: !!parentIncidentId,
    staleTime: 30_000,
  });
}
