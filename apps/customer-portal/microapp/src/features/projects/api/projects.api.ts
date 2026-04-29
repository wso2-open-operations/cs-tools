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

import apiClient from "@infrastructure/api/client";
import type { Pagination, PaginatedArray } from "@shared/types";
import type {
  DeploymentProductsDto,
  GetProductsRequestDto,
  ProjectDeploymentsDto,
  ProjectDto,
  ProjectFeaturesDto,
  ProjectsDto,
} from "@features/projects/types/project.dto";
import type { Deployment, Product, Project, ProjectInfo } from "@features/projects/types/project.model";
import { toDeployment, toProduct, toProject, toProjectSummary } from "@features/projects/mappers/project.mapper";
import {
  PROJECT_DEPLOYMENTS_ENDPOINT,
  PROJECT_DEPLOYMENT_PRODUCTS_ENDPOINT,
  PROJECT_DETAILS_ENDPOINT,
  PROJECT_FEATURES_ENDPOINT,
  PROJECTS_ENDPOINT,
} from "@config/endpoints";

export const getAllProjects = async (): Promise<Project[]> => {
  const projects = (await apiClient.post<ProjectsDto>(PROJECTS_ENDPOINT, {})).data.projects;
  return projects.map(toProjectSummary);
};

export const getProject = async (id: string): Promise<ProjectInfo> => {
  const response = (await apiClient.get<ProjectDto>(PROJECT_DETAILS_ENDPOINT(id))).data;
  return toProject(response);
};

export const editProject = async (
  id: string,
  body: { hasAgent?: boolean; hasKbReferences?: boolean },
): Promise<void> => {
  await apiClient.patch(PROJECT_DETAILS_ENDPOINT(id), body);
};

export const getProjectFeatures = async (id: string): Promise<ProjectFeaturesDto> => {
  return (await apiClient.get<ProjectFeaturesDto>(PROJECT_FEATURES_ENDPOINT(id))).data;
};

export const getDeploymentsByProject = async (
  id: string,
  body: Partial<Omit<Pagination, "totalRecords">>,
): Promise<PaginatedArray<Deployment>> => {
  const response = (await apiClient.post<ProjectDeploymentsDto>(PROJECT_DEPLOYMENTS_ENDPOINT(id), { pagination: body }))
    .data;
  const result = response.deployments.map(toDeployment) as PaginatedArray<Deployment>;
  result.pagination = { totalRecords: response.totalRecords, offset: response.offset, limit: response.limit };
  return result;
};

export const getProductsByDeployment = async (
  deploymentId: string,
  body: GetProductsRequestDto,
): Promise<PaginatedArray<Product>> => {
  const response = (
    await apiClient.post<DeploymentProductsDto>(PROJECT_DEPLOYMENT_PRODUCTS_ENDPOINT(deploymentId), body)
  ).data;
  const result = response.deployedProducts.map(toProduct) as PaginatedArray<Product>;
  result.pagination = { totalRecords: response.totalRecords, offset: response.offset, limit: response.limit };
  return result;
};
