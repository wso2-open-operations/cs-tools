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
import type { DeploymentDocument } from "@/types/deployments";

function normalizeDocuments(raw: unknown): DeploymentDocument[] {
  if (Array.isArray(raw)) {
    return raw as DeploymentDocument[];
  }
  if (raw && typeof raw === "object" && "attachments" in raw) {
    return (raw as { attachments: DeploymentDocument[] }).attachments ?? [];
  }
  if (raw && typeof raw === "object" && "documents" in raw) {
    return (raw as { documents: DeploymentDocument[] }).documents ?? [];
  }
  return [];
}

/**
 * Fetches deployment documents (GET /deployments/:deploymentId/attachments).
 *
 * @param {string} deploymentId - The deployment ID.
 * @returns {UseQueryResult<DeploymentDocument[], Error>} The query result.
 */
export function useGetDeploymentDocuments(
  deploymentId: string,
): UseQueryResult<DeploymentDocument[], Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<DeploymentDocument[], Error>({
    queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, deploymentId],
    queryFn: async (): Promise<DeploymentDocument[]> => {
      logger.debug(
        `Fetching deployment documents for deployment ID: ${deploymentId}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/deployments/${deploymentId}/attachments`;
        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetDeploymentDocuments] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching deployment documents: ${response.statusText}`,
          );
        }

        const raw = await response.json();
        const documents = normalizeDocuments(raw);
        logger.debug("[useGetDeploymentDocuments] Data received:", documents);
        return documents;
      } catch (error) {
        logger.error("[useGetDeploymentDocuments] Error:", error);
        throw error;
      }
    },
    enabled: !!deploymentId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
