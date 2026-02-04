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
import { useMockConfig } from "@/providers/MockConfigProvider";
import { mockUserDetails } from "@/models/mockData";
import { type UserDetails } from "@/models/responses";
import { useLogger } from "@/hooks/useLogger";

/**
 * Hook to get user details.
 *
 * @returns {UseQueryResult<UserDetails, Error>} The user details.
 */
const useGetUserDetails = (): UseQueryResult<UserDetails, Error> => {
  const { getIdToken } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();
  const logger = useLogger();

  return useQuery({
    queryKey: ["userDetails", isMockEnabled],
    queryFn: async (): Promise<UserDetails> => {
      logger.debug("[useGetUserDetails] Fetching user details...");

      if (isMockEnabled) {
        logger.debug("[useGetUserDetails] Mock enabled, returning mock data.");
        return Promise.resolve(mockUserDetails);
      }

      try {
        const idToken = await getIdToken();
        const baseUrl = import.meta.env.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        const requestUrl = `${baseUrl}/users/me`;

        logger.debug(`[useGetUserDetails] URL: ${requestUrl}`);

        const response = await fetch(requestUrl, {
          method: "GET",
          headers: {
            Accept: "application/json",
            Authorization: `Bearer ${idToken}`,
            "x-user-id-token": idToken,
          },
        });

        logger.debug(`[useGetUserDetails] Response status: ${response.status}`);

        if (!response.ok) {
          throw new Error(
            `Error fetching user details: ${response.statusText}`,
          );
        }

        const data = await response.json();
        logger.debug("[useGetUserDetails] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetUserDetails] Error:", error);
        throw error;
      }
    },
    enabled: true,
  });
};

export default useGetUserDetails;
