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

import { useAsgardeo } from "@asgardeo/react";
import { useQuery } from "@tanstack/react-query";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { InstanceSearchRequest } from "@features/project-details/types/usage";
import type { InstancesResponse } from "@features/project-details/types/usage";

/**
 * Searches all instances for a project.
 *
 * @param {string | undefined} projectId - The project ID.
 * @param {InstanceSearchRequest} payload - Optional filters and pagination.
 * @returns {UseQueryResult<InstancesResponse>} React Query result.
 */
export default function usePostProjectInstancesSearch(
  projectId: string | undefined,
  payload: InstanceSearchRequest = {},
) {
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();

  const authFetch = useAuthApiClient();

  return useQuery<InstancesResponse>({
    queryKey: [ApiQueryKeys.PROJECT_INSTANCES_SEARCH, projectId, payload],
    queryFn: async () => {
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL ?? "";
      const response = await authFetch(
        `${baseUrl}/projects/${projectId}/instances/search`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
      );
      if (!response.ok) {
        throw new Error(
          `Failed to search project instances: ${response.status}`,
        );
      }
      return response.json() as Promise<InstancesResponse>;
    },
    enabled: !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
