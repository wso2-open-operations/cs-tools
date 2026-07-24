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
import type { BeTaskDetail, BeUpdateTaskPayload } from "@api/backend/types";

/**
 * Update a single task via `PATCH /tasks/{id}` — state, assignee
 * (`assignedToEmail`), or due date (`dueDate`). The backend requires
 * **exactly one** of those fields per call (see {@link BeUpdateTaskPayload}).
 * `caseId` is optional and used only to also invalidate the owning case's
 * tasks list (e.g. so a state change updates {@link TaskDetailDialog} and the
 * {@link TasksWidget} row together); omit it when the caller only has the
 * task id.
 */
export function useUpdateTask(
  taskId: string | undefined,
  caseId?: string,
): UseMutationResult<BeTaskDetail, Error, BeUpdateTaskPayload> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeTaskDetail, Error, BeUpdateTaskPayload>({
    mutationFn: async (input): Promise<BeTaskDetail> => {
      if (!taskId) {
        throw new Error("Cannot update a task without an id.");
      }
      return api.patch<BeUpdateTaskPayload, BeTaskDetail>(
        `/tasks/${encodeURIComponent(taskId)}`,
        input,
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.TASK_DETAILS, taskId ?? ""],
      });
      if (caseId) {
        queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.CASE_TASKS, caseId],
        });
      }
    },
  });
}
