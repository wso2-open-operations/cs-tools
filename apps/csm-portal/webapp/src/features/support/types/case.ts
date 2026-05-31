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

export type DeploymentTypeKey =
  | "primary_production"
  | "staging"
  | "qa"
  | "stress"
  | "uat"
  | "development";

export interface Deployment {
  id: string;
  projectId: string;
  name: string;
  type: DeploymentTypeKey;
  description?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
}

export interface SearchDeploymentsRequest {
  pagination?: { limit?: number; offset?: number };
  searchQuery?: string;
  projectIds?: string[];
  deploymentTypeKeys?: DeploymentTypeKey[];
}

export interface SearchDeploymentsResponse {
  deployments: Deployment[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

export type ProductClass = "software" | "service";

export interface Product {
  id: string;
  name: string;
  class: ProductClass;
  createdAt: string;
  updatedAt: string;
}

export interface SearchProductsResponse {
  products: Product[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

export interface DeployedProduct {
  id: string;
  deploymentId: string;
  productId: string;
  productVersionId: string;
  createdAt: string;
  updatedAt: string;
}

export interface SearchDeployedProductsResponse {
  deployedProducts: DeployedProduct[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

/**
 * Severity keys used in the case create payload. The numeric values mirror
 * the backend's severityKey contract (P0 = highest, P4 = lowest).
 */
export const CASE_SEVERITY = {
  P0: 0,
  P1: 1,
  P2: 2,
  P3: 3,
  P4: 4,
} as const;

export type CaseSeverityLabel = keyof typeof CASE_SEVERITY;
export type CaseSeverityKey = (typeof CASE_SEVERITY)[CaseSeverityLabel];

export interface CaseCreatePayload {
  type: "DEFAULT_CASE";
  projectId: string;
  deploymentId: string;
  deployedProductId?: string;
  title: string;
  description: string;
  severityKey: CaseSeverityKey;
}
