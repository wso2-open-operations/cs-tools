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

export type DeploymentType = "primary_production" | "staging" | "qa" | "stress" | "uat" | "development";

export interface DeploymentDto {
  id: string;
  projectId: string;
  name: string;
  type: DeploymentType;
}

export interface DeploymentSearchPayloadDto {
  pagination?: { offset?: number; limit?: number };
  searchQuery?: string;
  projectIds?: string[];
  deploymentTypes?: DeploymentType[];
}

export interface DeploymentSearchResponseDto {
  deployments: DeploymentDto[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

export interface DeployedProductEntityRefDto {
  id: string;
  name: string;
}

// The search response embeds the resolved product + version objects (not just their raw ids —
// unlike what the OpenAPI doc's DeployedProduct schema documents), so the human-readable label
// can be built straight from this without a second lookup, matching the webapp's
// useDeployedProductOptions.ts.
export interface DeployedProductSearchItemDto {
  id: string;
  product?: DeployedProductEntityRefDto;
  version?: DeployedProductEntityRefDto | null;
}

export interface DeployedProductSearchPayloadDto {
  pagination?: { offset?: number; limit?: number };
}

export interface DeployedProductSearchResponseDto {
  deployedProducts: DeployedProductSearchItemDto[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}
