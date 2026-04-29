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

import { infiniteQueryOptions, mutationOptions, queryOptions } from "@tanstack/react-query";
import type { Pagination } from "@shared/types";
import type { GetProductsRequestDto } from "@features/projects/types/project.dto";
import {
  editProject,
  getAllProjects,
  getDeploymentsByProject,
  getProject,
  getProductsByDeployment,
  getProjectFeatures,
} from "@features/projects/api/projects.api";

export const projects = {
  all: () => queryOptions({ queryKey: ["projects"], queryFn: getAllProjects }),

  get: (id: string) => queryOptions({ queryKey: ["project", id], queryFn: () => getProject(id) }),

  features: (id: string) =>
    queryOptions({ queryKey: ["project", id, "features"], queryFn: () => getProjectFeatures(id) }),

  edit: (id: string) =>
    mutationOptions({ mutationFn: (body: { hasAgent?: boolean; hasKbReferences?: boolean }) => editProject(id, body) }),

  deployments: (id: string, body: Partial<Omit<Pagination, "totalRecords">> = {}) =>
    queryOptions({ queryKey: ["deployments", id, body], queryFn: () => getDeploymentsByProject(id, body) }),

  deploymentsPaginated: (id: string, body: Partial<Omit<Pagination, "totalRecords">> = {}) =>
    infiniteQueryOptions({
      queryKey: ["deployments", "paginated", id, body],
      queryFn: ({ pageParam }) => getDeploymentsByProject(id, { ...body, offset: pageParam }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        return nextOffset >= Math.ceil(totalRecords / limit) ? undefined : nextOffset;
      },
    }),

  products: (deploymentId: string, body: GetProductsRequestDto = {}) =>
    queryOptions({ queryKey: ["products", deploymentId], queryFn: () => getProductsByDeployment(deploymentId, body) }),

  productsPaginated: (deploymentId: string, body: GetProductsRequestDto = {}) =>
    infiniteQueryOptions({
      queryKey: ["products", "paginated", deploymentId, body],
      queryFn: ({ pageParam }) =>
        getProductsByDeployment(deploymentId, { ...body, pagination: { ...body.pagination, offset: pageParam } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        return nextOffset >= Math.ceil(totalRecords / limit) ? undefined : nextOffset;
      },
    }),
};
