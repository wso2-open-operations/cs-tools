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
import { toWorkItemSla } from "@features/csm-sla/utils/workItemSlaMapping";
import type {
  TaskSlaSearchPayload,
  TaskSlaSearchResponse,
  WorkItemSlaList,
} from "@features/csm-sla/types/workItemSla";

const TASK_SLA_PAGE_LIMIT = 50; // entity caps the page size at 50

/**
 * Lists the SLA records attached to a work item (a case or an incident).
 *
 * Calls the task-SLA search endpoint scoped to this work item's id (cases and
 * incidents are both tasks, and the endpoint is agnostic to which) and maps
 * each result onto the {@link WorkItemSla} row model the SLA table renders
 * (see {@link toWorkItemSla}). The query only runs while `workItemId` is set,
 * so callers that mount this hook conditionally (e.g. only while the SLA tab
 * is active) get lazy fetching for free.
 */
export function useGetWorkItemSlas(
  workItemId: string | undefined,
): UseQueryResult<WorkItemSlaList | null, Error> {
  const api = useBackendApi();
  const enabled = !!workItemId;

  return useQuery<WorkItemSlaList | null, Error>({
    queryKey: [ApiQueryKeys.WORK_ITEM_SLAS, workItemId ?? ""],
    queryFn: async (): Promise<WorkItemSlaList | null> => {
      if (!workItemId) return null;
      const slas: WorkItemSlaList["slas"] = [];
      let count = 0;
      // Include historical SLA instances too (severity changes can attach new
      // definitions), so older breached SLA records are not lost after page one.
      for (let page = 0; page < 200; page += 1) {
        const res = await api.post<TaskSlaSearchPayload, TaskSlaSearchResponse>(
          "/slas/search",
          {
            filters: { taskIds: [workItemId] },
            pagination: { limit: TASK_SLA_PAGE_LIMIT, offset: page * TASK_SLA_PAGE_LIMIT },
          },
        );
        const rows = res.slas ?? [];
        slas.push(...rows.map(toWorkItemSla));
        count = res.total ?? slas.length;
        if (rows.length < TASK_SLA_PAGE_LIMIT || slas.length >= count) break;
      }
      return { count, slas };
    },
    enabled,
    staleTime: 30_000,
  });
}
