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
import { useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";

export interface TeamOption {
  id: string;
  name: string;
}

interface SearchTeamsResponse {
  teams: TeamOption[];
  total: number;
}

/**
 * Lists the organisation's teams via the existing, already-deployed
 * `POST /teams/search` reference endpoint (backed by CSM_TEAM_REGISTRY) --
 * not a KB-specific endpoint. Used to populate the optional Team picker on
 * article creation and the Team filter on "All".
 */
export function useSearchTeams(): UseQueryResult<TeamOption[], Error> {
  const api = useBackendApi();
  return useQuery<TeamOption[], Error>({
    queryKey: [ApiQueryKeys.CSM_KB_ARTICLES, "teams"],
    queryFn: async () => {
      const res = await api.post<{ filters: Record<string, never>; pagination: { limit: number; offset: number } }, SearchTeamsResponse>(
        "/teams/search",
        { filters: {}, pagination: { limit: 100, offset: 0 } },
      );
      return res.teams ?? [];
    },
    staleTime: 5 * 60_000,
  });
}
