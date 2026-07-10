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

import { queryOptions } from "@tanstack/react-query";
import { DEPLOYMENTS_SEARCH_ENDPOINT, DEPLOYMENT_PRODUCTS_SEARCH_ENDPOINT } from "@config/endpoints";
import type { DeploymentSearchResponseDto, DeployedProductSearchResponseDto } from "@src/types";
import {
  toDeploymentOption,
  toDeployedProductOption,
  type DeploymentOption,
  type DeployedProductOption,
} from "@src/types";
import apiClient from "./apiClient";

const SEARCH_PAGE_LIMIT = 100;

const searchDeployments = async (projectId: string): Promise<DeploymentOption[]> => {
  const { data } = await apiClient.post<DeploymentSearchResponseDto>(DEPLOYMENTS_SEARCH_ENDPOINT, {
    pagination: { offset: 0, limit: SEARCH_PAGE_LIMIT },
    projectIds: [projectId],
  });
  return data.deployments.map(toDeploymentOption);
};

const searchDeployedProducts = async (deploymentId: string): Promise<DeployedProductOption[]> => {
  const { data } = await apiClient.post<DeployedProductSearchResponseDto>(
    DEPLOYMENT_PRODUCTS_SEARCH_ENDPOINT(deploymentId),
    { pagination: { offset: 0, limit: SEARCH_PAGE_LIMIT } },
  );
  return data.deployedProducts.map(toDeployedProductOption);
};

export const deployments = {
  byProject: (projectId: string) =>
    queryOptions({
      queryKey: ["deployments", projectId],
      queryFn: () => searchDeployments(projectId),
      enabled: !!projectId,
    }),

  productsByDeployment: (deploymentId: string) =>
    queryOptions({
      queryKey: ["deployment-products", deploymentId],
      queryFn: () => searchDeployedProducts(deploymentId),
      enabled: !!deploymentId,
    }),
};
