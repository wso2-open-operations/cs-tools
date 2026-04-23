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
import type { InstanceMetricsRequest } from "@features/project-details/types/usage";
import type { InstanceUsageResponse } from "@features/project-details/types/usage";

/**
 * Searches instance usages for a project within a date range.
 *
 * @param {string | undefined} projectId - The project ID.
 * @param {InstanceMetricsRequest} payload - Required date range filters.
 * @returns {UseQueryResult<InstanceUsageResponse>} React Query result.
 */
export default function usePostProjectInstancesUsagesSearch(
  projectId: string | undefined,
  payload: InstanceMetricsRequest,
) {
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();

  const authFetch = useAuthApiClient();

  return useQuery<InstanceUsageResponse>({
    queryKey: [ApiQueryKeys.PROJECT_INSTANCE_USAGES, projectId, payload],
    queryFn: async () => {
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL ?? "";
      const response = await authFetch(
        `${baseUrl}/projects/${projectId}/instances/usages/search`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
      );
      if (!response.ok) {
        throw new Error(
          `Failed to search project instance usages: ${response.status}`,
        );
      }
      return response.json() as Promise<InstanceUsageResponse>;
    },
    enabled: !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
