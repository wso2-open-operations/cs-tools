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
import { isMockMode, useBackendApi } from "@api/backend/client";
import type { BeCase, BeCaseCreatePayload } from "@api/backend/types";

const MOCK_LATENCY_MS = 200;

/**
 * Create a case via `POST /cases`. Returns the created case. The mutation
 * also invalidates project-scoped case searches so list views refresh.
 *
 * In MOCK mode the mutation resolves with a fabricated case using the input
 * payload; nothing is persisted server-side.
 */
export function usePostCsmCase(): UseMutationResult<
  BeCase,
  Error,
  BeCaseCreatePayload
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeCase, Error, BeCaseCreatePayload>({
    mutationFn: async (input): Promise<BeCase> => {
      if (isMockMode()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        const now = new Date().toISOString();
        return {
          id: `mock-${Math.random().toString(36).slice(2, 10)}`,
          number: `CS-MOCK-${Math.floor(Math.random() * 9000) + 1000}`,
          projectId: input.projectId,
          deploymentId: input.deploymentId,
          deployedProductId: input.deployedProductId,
          subject: input.subject,
          description: input.description,
          priority: input.priority,
          issueType: input.issueType,
          state: "open",
          createdAt: now,
          updatedAt: now,
        };
      }
      return api.post<BeCaseCreatePayload, BeCase>("/cases", input);
    },
    onSuccess: (created) => {
      // Invalidate any project-scoped search for the case's project.
      if (created.projectId) {
        queryClient.invalidateQueries({
          queryKey: [
            ApiQueryKeys.BACKEND_PROJECT_CASES_SEARCH,
            created.projectId,
          ],
        });
      }
      // And the cross-project CSM cases list (fan-out aggregation).
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASES] });
    },
  });
}
