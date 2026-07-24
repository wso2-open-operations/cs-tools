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

import { keepPreviousData, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeIncident,
  BeIncidentSearchPayload,
  BeIncidentSearchResponse,
} from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const INCIDENT_SEARCH_LIMIT = 20;

/**
 * Type-ahead incident search (`POST /incidents/search`, `filters.searchQuery`)
 * for the problem create form's "Primary incident" picker. `useSearchIncidents`
 * already wraps this endpoint but takes the full paginated-listing payload/
 * result shape used by `IncidentsTab`; this wraps the same endpoint in the
 * `(query, enabled) => {data, isFetching, isError}` shape `AsyncEntitySelect`
 * expects instead — same template as `useSearchGroups`/`useSearchUsersByName`.
 * Disabled until the caller has typed something. Filters out any result
 * without an id (the `BeIncident.id` field is nullable) so every option
 * `AsyncEntitySelect` renders is guaranteed to have one.
 */
export function useSearchIncidentsForSelect(
  query: string,
  enabled: boolean,
): UseQueryResult<BeIncident[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeIncident[], Error>({
    queryKey: [ApiQueryKeys.INCIDENTS_SEARCH_FOR_SELECT, q],
    queryFn: async (): Promise<BeIncident[]> => {
      const res = await api.post<BeIncidentSearchPayload, BeIncidentSearchResponse>(
        "/incidents/search",
        { filters: { searchQuery: q }, pagination: { offset: 0, limit: INCIDENT_SEARCH_LIMIT } },
      );
      return (res.incidents ?? []).filter((i) => !!i.id);
    },
    enabled: enabled && q.length > 0,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}
