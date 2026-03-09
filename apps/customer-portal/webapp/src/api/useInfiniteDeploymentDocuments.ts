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

import { useInfiniteQuery, type InfiniteData } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  DeploymentAttachmentsResponse,
  DeploymentDocument,
} from "@models/responses";

const PAGE_SIZE = 10;

/**
 * Fetches deployment attachments with pagination (GET /deployments/:deploymentId/attachments).
 * Uses infinite query to load all documents across pages.
 *
 * @param {string} deploymentId - The deployment ID.
 * @returns {UseInfiniteQueryResult} Infinite query result with flattened documents.
 */
export function useInfiniteDeploymentDocuments(deploymentId: string) {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useInfiniteQuery<
    DeploymentAttachmentsResponse,
    Error,
    InfiniteData<DeploymentAttachmentsResponse>,
    readonly (string | number)[],
    number
  >({
    queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, deploymentId],
    queryFn: async ({ pageParam }): Promise<DeploymentAttachmentsResponse> => {
      logger.debug(
        `[useInfiniteDeploymentDocuments] Fetching attachments for deployment ${deploymentId}, offset: ${pageParam}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const params = new URLSearchParams({
          limit: String(PAGE_SIZE),
          offset: String(pageParam),
        });
        const requestUrl = `${baseUrl}/deployments/${deploymentId}/attachments?${params}`;
        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useInfiniteDeploymentDocuments] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching deployment attachments: ${response.statusText}`,
          );
        }

        const data: DeploymentAttachmentsResponse = await response.json();
        logger.debug("[useInfiniteDeploymentDocuments] Page received:", data);
        return data;
      } catch (error) {
        logger.error("[useInfiniteDeploymentDocuments] Error:", error);
        throw error;
      }
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      const { offset, limit, totalRecords } = lastPage;
      const nextOffset = offset + limit;
      return nextOffset < totalRecords ? nextOffset : undefined;
    },
    enabled: !!deploymentId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}

/** Flattens all pages of deployment documents into a single array. */
export function flattenDeploymentDocuments(
  data: InfiniteData<DeploymentAttachmentsResponse> | undefined,
): DeploymentDocument[] {
  if (!data?.pages) return [];
  return data.pages.flatMap(
    (page: DeploymentAttachmentsResponse) => page.attachments ?? [],
  );
}
