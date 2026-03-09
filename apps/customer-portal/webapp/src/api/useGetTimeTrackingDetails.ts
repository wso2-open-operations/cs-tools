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
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { TimeTrackingDetailsResponse } from "@models/responses";

/**
 * Custom hook to fetch project time tracking details.
 *
 * @param {string} projectId - The ID of the project to fetch time tracking details for.
 * @returns {UseQueryResult<TimeTrackingDetailsResponse, Error>} The query result object.
 */
export default function useGetTimeTrackingDetails(
  projectId: string,
): UseQueryResult<TimeTrackingDetailsResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<TimeTrackingDetailsResponse, Error>({
    queryKey: [ApiQueryKeys.TIME_TRACKING_DETAILS, projectId],
    queryFn: async (): Promise<TimeTrackingDetailsResponse> => {
      logger.debug(
        `Fetching time tracking details for project ID: ${projectId}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/timetracking`;

        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetTimeTrackingDetails] Response status for ${projectId}: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching time tracking details: ${response.statusText}`,
          );
        }

        const data: TimeTrackingDetailsResponse = await response.json();
        logger.debug("[useGetTimeTrackingDetails] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetTimeTrackingDetails] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}
