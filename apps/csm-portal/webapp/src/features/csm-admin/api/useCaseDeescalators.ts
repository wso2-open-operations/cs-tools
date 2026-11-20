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


import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeCaseDeescalator, BeCaseDeescalatorsResponse } from "@api/backend/types";

const PATH = "/case-escalation-access/deescalators";

/**
 * The people an admin named as allowed to de-escalate any case
 * (`GET /case-escalation-access/deescalators`, admin-only server-side). A
 * case's own team leads are not listed: they may de-escalate their team's
 * cases whatever is here.
 */
export function useGetCaseDeescalators(
  enabled = true,
): UseQueryResult<BeCaseDeescalator[], Error> {
  const api = useBackendApi();
  return useQuery<BeCaseDeescalator[], Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_CASE_DEESCALATORS],
    queryFn: async (): Promise<BeCaseDeescalator[]> => {
      const response = await api.get<BeCaseDeescalatorsResponse>(PATH);
      return response?.deescalators ?? [];
    },
    enabled,
    staleTime: 30_000,
  });
}

/**
 * Lets a person de-escalate any case (`PUT .../deescalators/{userId}`) or takes
 * that away (`DELETE`), then refreshes the list and every open case's
 * escalation history, so a case page reflects it without a reload.
 */
export function useChangeCaseDeescalator(
  action: "add" | "remove",
): UseMutationResult<void, Error, string> {
  const api = useBackendApi();
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: async (userId): Promise<void> => {
      const path = `${PATH}/${encodeURIComponent(userId)}`;
      if (action === "add") {
        await api.put<undefined, void>(path, undefined);
      } else {
        await api.delete(path);
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_CASE_DEESCALATORS] });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASE_ESCALATIONS] });
    },
  });
}
