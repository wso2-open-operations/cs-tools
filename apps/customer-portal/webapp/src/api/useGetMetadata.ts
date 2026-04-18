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
import { useAuthApiClient } from "@utils/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { PortalMetadataResponse } from "@features/project-hub/types/projects";

/**
 * Fetches global metadata used by customer portal (time zones, etc).
 *
 * @returns {UseQueryResult<PortalMetadataResponse, Error>} Metadata query result.
 */
export default function useGetMetadata(): UseQueryResult<
  PortalMetadataResponse,
  Error
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<PortalMetadataResponse, Error>({
    queryKey: [ApiQueryKeys.METADATA],
    queryFn: async (): Promise<PortalMetadataResponse> => {
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const response = await authFetch(`${baseUrl}/metadata`, {
        method: "GET",
      });
      if (!response.ok) {
        throw new Error(`Error fetching metadata: ${response.statusText}`);
      }

      const data = (await response.json()) as PortalMetadataResponse;
      logger.debug("[useGetMetadata] Data received", {
        timeZonesCount: data.timeZones?.length ?? 0,
      });
      return data;
    },
    enabled: isSignedIn && !isAuthLoading,
    staleTime: 10 * 60 * 1000,
  });
}
