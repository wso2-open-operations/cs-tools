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

import { useMutation, type UseMutationResult } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { parseApiResponseMessage } from "@utils/ApiError";

export interface DeploymentUsageImportResponse {
  message?: string;
}

/**
 * Posts a zip file to POST /deployment-usages to import product consumption usage data.
 *
 * @returns {UseMutationResult} Mutation result.
 */
export function usePostDeploymentUsagesImport(): UseMutationResult<
  DeploymentUsageImportResponse,
  Error,
  File
> {
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<DeploymentUsageImportResponse, Error, File>({
    mutationFn: async (zipFile: File): Promise<DeploymentUsageImportResponse> => {
      if (!isSignedIn || isAuthLoading) {
        throw new Error("User must be signed in to import deployment usage data");
      }

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const arrayBuffer = await zipFile.arrayBuffer();
      const response = await authFetch(`${baseUrl}/deployment-usages`, {
        method: "POST",
        headers: { "Content-Type": "application/zip" },
        body: arrayBuffer,
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(parseApiResponseMessage(text, response.status, response.statusText));
      }

      return response.json() as Promise<DeploymentUsageImportResponse>;
    },
  });
}
