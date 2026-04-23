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

import type {
  DeployedProductsResponse,
  SelectedDeploymentProduct,
} from "@features/project-details/types/deployments";

/**
 * Type guard for DeployedProductsResponse.
 *
 * @param {unknown} payload - The payload to check.
 * @returns {payload is DeployedProductsResponse} True if the payload is a DeployedProductsResponse.
 */
export function isDeployedProductsResponse(
  payload: unknown,
): payload is DeployedProductsResponse {
  return (
    typeof payload === "object" &&
    payload !== null &&
    Array.isArray((payload as { deployedProducts?: unknown }).deployedProducts)
  );
}

/**
 * Clamps the current page index to valid range for a known page count.
 *
 * @param page - One-based page from UI.
 * @param totalPages - Total pages (0 when unknown or empty).
 * @returns Clamped one-based page.
 */
export function clampDeploymentsPage(page: number, totalPages: number): number {
  if (totalPages <= 0) {
    return 1;
  }
  return Math.min(Math.max(page, 1), totalPages);
}

/**
 * Query string for service request creation from a selected deployment product.
 *
 * @param selected - Selected deployment and product row.
 * @returns URLSearchParams for the create-SR route.
 */
export function buildServiceRequestCreateSearchParams(
  selected: SelectedDeploymentProduct,
): URLSearchParams {
  return new URLSearchParams({
    deploymentId: selected.deploymentId,
    productId: selected.productItemId,
  });
}
