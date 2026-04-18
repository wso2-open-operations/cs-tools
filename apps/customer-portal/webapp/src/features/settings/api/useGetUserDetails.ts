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
import type { UserDetails } from "@features/settings/types/users";
import { useLogger } from "@hooks/useLogger";
import { AUTH_NOT_READY_ERROR_MESSAGE } from "@constants/apiConstants";

/**
 * Hook to get user details.
 *
 * @returns {UseQueryResult<UserDetails, Error>} The user details.
 */
const useGetUserDetails = (): UseQueryResult<UserDetails, Error> => {
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const logger = useLogger();

  return useQuery({
    queryKey: ["userDetails"],
    queryFn: async (): Promise<UserDetails> => {
      logger.debug("[useGetUserDetails] Fetching user details...");

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }
        const requestUrl = `${baseUrl}/users/me`;

        logger.debug(`[useGetUserDetails] URL: ${requestUrl}`);

        const response = await authFetch(requestUrl, {
          method: "GET",
          cache: "no-store",
        });

        logger.debug(`[useGetUserDetails] Response status: ${response.status}`);

        if (!response.ok) {
          const err = new Error(
            `Error fetching user details: ${response.status} ${response.statusText}`,
          ) as Error & { status?: number };
          err.status = response.status;
          throw err;
        }

        const data = (await response.json()) as Record<string, unknown>;
        logger.debug("[useGetUserDetails] Data received:", data);
        const tzRaw = data.timeZone ?? data.timezone ?? data.time_zone;
        const timeZone =
          typeof tzRaw === "string"
            ? tzRaw
            : tzRaw != null
              ? String(tzRaw)
              : "";
        return { ...data, timeZone } as UserDetails;
      } catch (error) {
        if (
          error instanceof Error &&
          error.message.includes(AUTH_NOT_READY_ERROR_MESSAGE)
        ) {
          logger.warn("[useGetUserDetails] Auth not ready yet; will retry.");
        } else {
          logger.error("[useGetUserDetails] Error:", error);
        }
        throw error;
      }
    },
    enabled: isSignedIn && !isAuthLoading,
    retry: (failureCount, error) => {
      if (failureCount >= 3) {
        return false;
      }
      if (
        error instanceof Error &&
        error.message.includes(AUTH_NOT_READY_ERROR_MESSAGE)
      ) {
        return true;
      }
      const status = (error as Error & { status?: number }).status;
      if (typeof status === "number") {
        if (status >= 500) {
          return true;
        }
        if (status >= 400 && status < 500) {
          return false;
        }
      }
      if (error instanceof TypeError) {
        return true;
      }
      return false;
    },
    retryDelay: (attemptIndex) => Math.min(400 * 2 ** attemptIndex, 3000),
    staleTime: 0,
    refetchOnWindowFocus: false,
    refetchOnMount: true,
  });
};

export default useGetUserDetails;
