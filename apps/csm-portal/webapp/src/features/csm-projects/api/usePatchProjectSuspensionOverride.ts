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

import {
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";

interface ProjectSuspensionOverridePatch {
  suspensionProcessOverridden: boolean;
}

interface ProjectSuspensionOverridePatchResponse {
  message: string;
  project: { id: string; suspensionProcessOverridden?: boolean };
}

/**
 * Exclude a project from (or put it back into) the automatic closure process
 * via `PATCH /projects/{id}` `{suspensionProcessOverridden}` — admin only (the
 * backend answers 403 otherwise; `usePortalAccess().canUpdateProjectSuspensionOverride`
 * only decides whether the control is editable). While the flag is true the
 * automatic process never evaluates the project. Mirrors
 * `usePatchProjectClosureState`: bound to one id up front, invalidates the
 * detail query on success so it refetches the authoritative state.
 */
export function usePatchProjectSuspensionOverride(
  projectId: string | undefined,
): UseMutationResult<ProjectSuspensionOverridePatchResponse, Error, boolean> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<ProjectSuspensionOverridePatchResponse, Error, boolean>({
    mutationFn: async (suspensionProcessOverridden): Promise<ProjectSuspensionOverridePatchResponse> => {
      if (!projectId) {
        throw new Error("Cannot update a project without an id.");
      }
      return api.patch<ProjectSuspensionOverridePatch, ProjectSuspensionOverridePatchResponse>(
        `/projects/${encodeURIComponent(projectId)}`,
        { suspensionProcessOverridden },
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_PROJECT_DETAIL, projectId ?? ""],
      });
    },
  });
}
