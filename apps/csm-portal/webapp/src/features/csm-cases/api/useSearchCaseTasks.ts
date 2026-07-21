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
import type { BeCaseTasksSearchPayload, BeListCaseTasksResponse } from "@api/backend/types";

/**
 * Page size for the case-scoped tasks list. Tasks are a lightweight sub-item
 * (no multi-stage lifecycle like call requests / change requests), so a
 * single wide-ish page is expected to cover the vast majority of cases
 * without needing pagination controls in the UI yet.
 */
const CASE_TASKS_PAGE_LIMIT = 20;

/**
 * Lists the tasks attached to a case via `POST /cases/{caseId}/tasks/search`.
 * Only runs while `caseId` is set, so callers that mount this hook
 * conditionally (e.g. only while the Tasks tab is active) get lazy fetching.
 */
export function useSearchCaseTasks(
  caseId: string | undefined,
): UseQueryResult<BeListCaseTasksResponse | null, Error> {
  const api = useBackendApi();

  return useQuery<BeListCaseTasksResponse | null, Error>({
    queryKey: [ApiQueryKeys.CASE_TASKS, caseId ?? ""],
    queryFn: async (): Promise<BeListCaseTasksResponse | null> => {
      if (!caseId) return null;
      return api.post<BeCaseTasksSearchPayload, BeListCaseTasksResponse>(
        `/cases/${encodeURIComponent(caseId)}/tasks/search`,
        { pagination: { limit: CASE_TASKS_PAGE_LIMIT, offset: 0 } },
      );
    },
    enabled: !!caseId,
    staleTime: 10_000,
  });
}
