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
  DeploymentProductDTO,
  DeploymentProductsDTO,
  Product,
  Project,
  ProjectDeploymentDTO,
  ProjectDeploymentsDTO,
  ProjectsDTO,
  ProjectStatsDTO,
  ProjectStatus,
} from "@src/types";
import {
  PROJECT_DEPLOYMENTS_ENDPOINT,
  PROJECT_STATS_ENDPOINT,
  PROJECTS_ENDPOINT,
  PROJECT_DEPLOYMENT_PRODUCTS_ENDPOINT,
} from "@config/endpoints";
import apiClient from "@src/services/apiClient";
import { queryOptions } from "@tanstack/react-query";

const getAllProjects = async (): Promise<Project[]> => {
  const projects = (await apiClient.post<ProjectsDTO>(PROJECTS_ENDPOINT, {})).data.projects;
  const projectsWithStats = await Promise.all(
    projects.map(async (project) => {
      const stats: ProjectStatsDTO | undefined = await apiClient
        .get<ProjectStatsDTO>(PROJECT_STATS_ENDPOINT(project.id), { timeout: 3000 })
        .then((res) => res.data)
        .catch(() => undefined);
      return mapProjectAndStatsDTOToProject(project, stats);
    }),
  );

  return projectsWithStats;
};

const getDeploymentsByProject = async (id: string): Promise<Deployment[]> => {
  const deployments = (await apiClient.get<ProjectDeploymentsDTO>(PROJECT_DEPLOYMENTS_ENDPOINT(id))).data;

  return deployments.map(toDeployment);
};

const getProductsByDeployment = async (deploymentId: string): Promise<Product[]> => {
  const products = (await apiClient.get<DeploymentProductsDTO>(PROJECT_DEPLOYMENT_PRODUCTS_ENDPOINT(deploymentId)))
    .data;

  return products.map(toProduct);
};

/* Mappers */
function mapProjectAndStatsDTOToProject(project: ProjectsDTO["projects"][number], stats?: ProjectStatsDTO): Project {
  return {
    id: project.id,
    projectKey: project.key,
    name: project.name,
    createdOn: new Date(project.createdOn.replace(" ", "T")),
    description: project.description ? project.description.replace(/<\/?[^>]+(>|$)/g, "") : "",
    metrics: {
      cases: stats?.projectStats.openCases,
      chats: stats?.projectStats.activeChats,
    },
    status: stats?.projectStats.slaStatus as ProjectStatus,
    type: "Regular", // TODO:
  };
}

function toDeployment(deployment: ProjectDeploymentDTO): Deployment {
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

function toProduct(product: DeploymentProductDTO): Product {
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

  deployments: (id: string) =>
    queryOptions({
      queryKey: ["deployments", id],
      queryFn: () => getDeploymentsByProject(id),
    }),

  products: (deploymentId: string) =>
    queryOptions({
      queryKey: ["products", deploymentId],
      queryFn: () => getProductsByDeployment(deploymentId),
    }),
};
