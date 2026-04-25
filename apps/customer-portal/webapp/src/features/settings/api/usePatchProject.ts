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
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import {
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { PatchProjectRequest } from "@features/project-hub/types/projects";
import { parseApiResponseMessage } from "@utils/ApiError";

export interface PatchProjectResponse {
  id: string;
}

/**
 * Patches AI Chat Assistant (Novera) settings for a project (PATCH /projects/:projectId).
 *
 * @param {string} projectId - Project ID.
 * @returns {UseMutationResult<PatchProjectResponse, Error, PatchProjectRequest>} Mutation result.
 */
export function usePatchProject(
  projectId: string,
): UseMutationResult<PatchProjectResponse, Error, PatchProjectRequest> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<PatchProjectResponse, Error, PatchProjectRequest>({
    mutationFn: async (
      body: PatchProjectRequest,
    ): Promise<PatchProjectResponse> => {
      logger.debug("[usePatchProject] Request:", { projectId, body });

      if (!isSignedIn || isAuthLoading) {
        throw new Error("User must be signed in to update a project");
      }

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const response = await authFetch(`${baseUrl}/projects/${projectId}`, {
        method: "PATCH",
        body: JSON.stringify(body),
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(parseApiResponseMessage(text, response.status, response.statusText));
      }

      const data: PatchProjectResponse = await response.json();
      logger.debug("[usePatchProject] Project updated:", data);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.PROJECT_DETAILS, projectId],
      });
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.PROJECTS],
      });
    },
  });
}
