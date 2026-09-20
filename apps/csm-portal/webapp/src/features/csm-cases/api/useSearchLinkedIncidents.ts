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
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeIncidentSearchPayload, BeIncidentSearchResponse } from "@api/backend/types";

const LINKED_INCIDENTS_LIMIT = 20;

export interface LinkedIncidentRow {
  id: string;
  number?: string;
  subject: string;
  priority?: string | null;
  state?: string | null;
  assigneeName?: string;
}

export interface LinkedIncidentsResult {
  incidents: LinkedIncidentRow[];
  total: number;
}

/**
 * Incidents that are SN-side *children* of this case — i.e. incidents whose
 * `parent` field points at this case (the opposite direction from
 * `LinkedIncidentWidget`'s `c.parentCase`, which shows the case pointing at
 * an incident as ITS parent). Calls `POST /incidents/search` filtered by
 * `parentIds`, mirroring `useSearchChildCases`'s "reuse the existing
 * cross-entity search endpoint rather than a dedicated one" pattern. Disabled
 * until a case id is provided.
 */
export function useSearchLinkedIncidents(
  caseId: string | undefined,
): UseQueryResult<LinkedIncidentsResult, Error> {
  const api = useBackendApi();

  return useQuery<LinkedIncidentsResult, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_LINKED_INCIDENTS, caseId ?? ""],
    queryFn: async (): Promise<LinkedIncidentsResult> => {
      const res = await api.post<BeIncidentSearchPayload, BeIncidentSearchResponse>(
        "/incidents/search",
        {
          filters: { parentIds: [caseId as string] },
          pagination: { offset: 0, limit: LINKED_INCIDENTS_LIMIT },
        },
      );
      return {
        incidents: (res.incidents ?? [])
          .filter((i): i is typeof i & { id: string } => !!i.id)
          .map((i) => ({
            id: i.id,
            number: i.number ?? undefined,
            subject: i.subject ?? "(no subject)",
            priority: i.priority,
            state: i.state,
            assigneeName: i.assignedTo?.name ?? undefined,
          })),
        total: res.total ?? 0,
      };
    },
    enabled: !!caseId,
    staleTime: 30_000,
  });
}
