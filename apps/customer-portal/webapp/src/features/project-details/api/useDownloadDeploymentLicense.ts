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
import { useLogger } from "@hooks/useLogger";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import type { DownloadDeploymentLicenseVariables } from "@features/project-details/types/projectDetailsApi";
import type { DeploymentLicense } from "@features/project-details/types/deployments";
import { parseApiResponseMessage } from "@utils/ApiError";

/**
 * Hook to download a license for a deployment (POST /projects/:projectId/deployments/:deploymentId/license).
 * Downloads the license as a JSON file.
 *
 * @returns {UseMutationResult<void, Error, DownloadDeploymentLicenseVariables>} Mutation result.
 */
export function useDownloadDeploymentLicense(): UseMutationResult<
  void,
  Error,
  DownloadDeploymentLicenseVariables
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const { showError } = useErrorBanner();

  const downloadJsonFile = (data: DeploymentLicense, filename: string) => {
    const jsonString = JSON.stringify(data, null, 2);
    const blob = new Blob([jsonString], { type: "application/json" });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  };

  return useMutation<void, Error, DownloadDeploymentLicenseVariables>({
    mutationFn: async ({
      projectId,
      deploymentId,
      deploymentName,
    }: DownloadDeploymentLicenseVariables): Promise<void> => {
      try {
        logger.debug("[useDownloadDeploymentLicense] Request:", {
          projectId,
          deploymentId,
        });

        if (!isSignedIn || isAuthLoading) {
          throw new Error(
            "User must be signed in to download deployment license",
          );
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/deployments/${deploymentId}/license`;
        const response = await authFetch(requestUrl, {
          method: "POST",
        });

        logger.debug(
          "[useDownloadDeploymentLicense] Response status:",
          response.status,
        );

        if (!response.ok) {
          const text = await response.text();
          throw new Error(parseApiResponseMessage(text, response.status, response.statusText));
        }

        const licenseData: DeploymentLicense = await response.json();
        const filename = `subscription-${deploymentName}.json`;
        downloadJsonFile(licenseData, filename);

        logger.debug(
          "[useDownloadDeploymentLicense] License downloaded successfully:",
          filename,
        );
      } catch (error) {
        logger.error("[useDownloadDeploymentLicense] Error:", error);
        throw error;
      }
    },
    onError: (error) => {
      const errorMessage =
        error instanceof Error ? error.message : "Failed to download license";
      showError(errorMessage);
    },
  });
}
