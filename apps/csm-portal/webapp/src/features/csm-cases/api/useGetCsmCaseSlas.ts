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
import { toCaseSla } from "@features/csm-cases/utils/caseSlaMapping";
import type {
  CaseSlaList,
  TaskSlaSearchPayload,
  TaskSlaSearchResponse,
} from "@features/csm-cases/types/csmCases";

const TASK_SLA_PAGE_LIMIT = 100;

/**
 * Lists the SLA records attached to a case.
 *
 * Calls the task-SLA search endpoint scoped to this case's id (a case is a
 * task) and maps each result onto the {@link CaseSla} row model the SLA table
 * renders (see {@link toCaseSla}). Lazy: pass `enabled: false` until the SLA
 * tab is actually selected so the request doesn't fire for every case-detail
 * load.
 */
export function useGetCsmCaseSlas(
  caseId: string | undefined,
  options?: { enabled?: boolean },
): UseQueryResult<CaseSlaList | null, Error> {
  const api = useBackendApi();
  const enabled = (options?.enabled ?? true) && !!caseId;

  return useQuery<CaseSlaList | null, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_SLAS, caseId ?? ""],
    queryFn: async (): Promise<CaseSlaList | null> => {
      if (!caseId) return null;
      const res = await api.post<TaskSlaSearchPayload, TaskSlaSearchResponse>(
        "/task-slas/search",
        {
          filters: { taskIds: [caseId] },
          pagination: { limit: TASK_SLA_PAGE_LIMIT, offset: 0 },
        },
      );
      const slas = (res.taskSlas ?? []).map(toCaseSla);
      return { caseId, count: res.total ?? slas.length, slas };
    },
    enabled,
    staleTime: 30_000,
  });
}
