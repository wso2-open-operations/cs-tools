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
import type { BeSpecialistHandoffTeam, BeSpecialistHandoffTeamsResponse } from "@api/backend/types";

/**
 * The Special Ops teams the "Escalate to specialist team" dialog offers for
 * an incident's service, via `GET /specialist-handoff-teams?serviceId=`.
 * They are configuration, so they change rarely: fetched when the dialog
 * first opens and kept for five minutes.
 */
export function useSpecialistHandoffTeams(
  serviceId: string | undefined,
  enabled: boolean,
): UseQueryResult<BeSpecialistHandoffTeam[], Error> {
  const api = useBackendApi();

  return useQuery<BeSpecialistHandoffTeam[], Error>({
    queryKey: [ApiQueryKeys.SPECIALIST_HANDOFF_TEAMS, serviceId],
    queryFn: async (): Promise<BeSpecialistHandoffTeam[]> =>
      (
        await api.get<BeSpecialistHandoffTeamsResponse>(
          `/specialist-handoff-teams?serviceId=${encodeURIComponent(serviceId ?? "")}`,
        )
      )?.teams ?? [],
    enabled: enabled && !!serviceId,
    staleTime: 5 * 60_000,
  });
}
