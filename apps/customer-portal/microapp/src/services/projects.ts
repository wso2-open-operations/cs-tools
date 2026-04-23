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
  Deployment,
  DeploymentProductDto,
  DeploymentProductsDto,
  PaginatedArray,
  Pagination,
  Product,
  Project,
  ProjectDeploymentDto,
  ProjectDeploymentsDto,
  ProjectDto,
  ProjectFeaturesDto,
  ProjectInfo,
  ProjectsDto,
  ProjectStatus,
} from "@src/types";
import {
  PROJECT_DEPLOYMENTS_ENDPOINT,
  PROJECTS_ENDPOINT,
  PROJECT_DEPLOYMENT_PRODUCTS_ENDPOINT,
  PROJECT_DETAILS_ENDPOINT,
  PROJECT_FEATURES_ENDPOINT,
} from "@config/endpoints";
import apiClient from "@src/services/apiClient";
import { infiniteQueryOptions, mutationOptions, queryOptions } from "@tanstack/react-query";
import { stripHtmlTags } from "@utils/others";

const getAllProjects = async (): Promise<Project[]> => {
  const projects = (await apiClient.post<ProjectsDto>(PROJECTS_ENDPOINT, {})).data.projects;
  const projectsWithStats = await Promise.all(projects.map(mapProjectDtoToProjectSummary));

  return projectsWithStats;
};

const getProject = async (id: string): Promise<ProjectInfo> => {
  const response = (await apiClient.get<ProjectDto>(PROJECT_DETAILS_ENDPOINT(id))).data;
  return mapProjectDtoToProject(response);
};

const editProject = async (id: string, body: { hasAgent?: boolean; hasKbReferences?: boolean }): Promise<void> => {
  await apiClient.patch(PROJECT_DETAILS_ENDPOINT(id), body);
};

const getProjectFeatures = async (id: string): Promise<ProjectFeaturesDto> => {
  return (await apiClient.get<ProjectFeaturesDto>(PROJECT_FEATURES_ENDPOINT(id))).data;
};

const getDeploymentsByProject = async (
  id: string,
  body: Partial<Omit<Pagination, "totalRecords">>,
): Promise<PaginatedArray<Deployment>> => {
  const response = (await apiClient.post<ProjectDeploymentsDto>(PROJECT_DEPLOYMENTS_ENDPOINT(id), { pagination: body }))
    .data;
  const result = response.deployments.map(toDeployment) as PaginatedArray<Deployment>;
  result.pagination = {
    totalRecords: response.totalRecords,
    offset: response.offset,
    limit: response.limit,
  };

  return result;
};

const getProductsByDeployment = async (
  deploymentId: string,
  body: Partial<Omit<Pagination, "totalRecords">>,
): Promise<PaginatedArray<Product>> => {
  const response = (
    await apiClient.post<DeploymentProductsDto>(PROJECT_DEPLOYMENT_PRODUCTS_ENDPOINT(deploymentId), {
      pagination: body,
    })
  ).data;
  const result = response.deployedProducts.map(toProduct) as PaginatedArray<Product>;
  result.pagination = {
    totalRecords: response.totalRecords,
    offset: response.offset,
    limit: response.limit,
  };

  return result;
};

/* Mappers */
function mapProjectDtoToProjectSummary(project: ProjectsDto["projects"][number]): Project {
  return {
    id: project.id,
    projectKey: project.key,
    name: project.name,
    createdOn: new Date(project.createdOn.replace(" ", "T")),
    description: project.description ? project.description.replace(/<\/?[^>]+(>|$)/g, "") : "",
    metrics: {
      cases: project.activeCasesCount,
      chats: project.activeChatsCount,
    },
    status: project.slaStatus as ProjectStatus,
    type: project.type?.label ?? "N/A",
  };
}

function mapProjectDtoToProject(project: ProjectDto): ProjectInfo {
  return {
    id: project.id,
    projectKey: project.key,
    name: project.name,
    createdOn: new Date(project.createdOn.replace(" ", "T")),
    description: stripHtmlTags(project.description),
    type: project.type?.label ?? "N/A",
    agentEnabled: project.account.hasAgent,
    kbReferencesEnabled: project.account.hasKbReferences,
    typeId: project.type?.id,
  };
}

function toDeployment(deployment: ProjectDeploymentDto): Deployment {
  return {
    id: deployment.id,
    name: deployment.name,
    createdOn: new Date(deployment.createdOn.replace(" ", "T")),
    updatedOn: new Date(deployment.updatedOn.replace(" ", "T")),
    url: deployment.url ?? undefined,
    typeId: deployment.type.id,
    projectId: deployment.project.id,
    type: deployment.type.label,
  };
}

function toProduct(product: DeploymentProductDto): Product {
  return {
    id: product.id,
    createdOn: new Date(product.createdOn.replace(" ", "T")),
    updatedOn: new Date(product.updatedOn.replace(" ", "T")),
    description: product.description ?? undefined,
    name: product.product.label,
    deploymentId: product.deployment.id,
    versionId: product.version?.id ?? undefined,
  };
}

/* Query Options */
export const projects = {
  all: () =>
    queryOptions({
      queryKey: ["projects"],
      queryFn: getAllProjects,
    }),

  get: (id: string) =>
    queryOptions({
      queryKey: ["project", id],
      queryFn: () => getProject(id),
    }),

  features: (id: string) =>
    queryOptions({
      queryKey: ["project", id, "features"],
      queryFn: () => getProjectFeatures(id),
    }),

  edit: (id: string) =>
    mutationOptions({
      mutationFn: (body: { hasAgent?: boolean; hasKbReferences?: boolean }) => editProject(id, body),
    }),

  deployments: (id: string, body: Partial<Omit<Pagination, "totalRecords">> = {}) =>
    queryOptions({
      queryKey: ["deployments", id, body],
      queryFn: () => getDeploymentsByProject(id, body),
    }),

  deploymentsPaginated: (id: string, body: Partial<Omit<Pagination, "totalRecords">> = {}) =>
    infiniteQueryOptions({
      queryKey: ["deployments", "paginated", id, body],
      queryFn: ({ pageParam }) => getDeploymentsByProject(id, { ...body, offset: pageParam }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        const totalPages = Math.ceil(totalRecords / limit);
        return nextOffset >= totalPages ? undefined : nextOffset;
      },
    }),

  products: (deploymentId: string, body: Partial<Omit<Pagination, "totalRecords">> = {}) =>
    queryOptions({
      queryKey: ["products", deploymentId],
      queryFn: () => getProductsByDeployment(deploymentId, body),
    }),

  productsPaginated: (deploymentId: string, body: Partial<Omit<Pagination, "totalRecords">> = {}) =>
    infiniteQueryOptions({
      queryKey: ["products", "paginated", deploymentId, body],
      queryFn: ({ pageParam }) => getProductsByDeployment(deploymentId, { ...body, offset: pageParam }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        const totalPages = Math.ceil(totalRecords / limit);
        return nextOffset >= totalPages ? undefined : nextOffset;
      },
    }),
};
