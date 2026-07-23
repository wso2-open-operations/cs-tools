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
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeCreateCaseTaskPayload, BeTaskDetail } from "@api/backend/types";

/**
 * Create a task on a case via `POST /cases/{caseId}/tasks` (ServiceNow data
 * source only; the caller surfaces a rejection on another source). On
 * success, invalidates this case's tasks list so the new row shows without a
 * manual refetch.
 */
export function useCreateCaseTask(
  caseId: string | undefined,
): UseMutationResult<BeTaskDetail, Error, BeCreateCaseTaskPayload> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeTaskDetail, Error, BeCreateCaseTaskPayload>({
    mutationFn: async (input): Promise<BeTaskDetail> => {
      if (!caseId) {
        throw new Error("Cannot create a task without a case id.");
      }
      return api.post<BeCreateCaseTaskPayload, BeTaskDetail>(
        `/cases/${encodeURIComponent(caseId)}/tasks`,
        input,
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_TASKS, caseId ?? ""],
      });
    },
  });
}
