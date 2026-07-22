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

import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import { PROJECTS_SEARCH_ENDPOINT, PROJECT_ENDPOINT } from "@config/endpoints";
import type { ProjectDetailDto, ProjectSearchResponseDto } from "@src/types";
import {
  toProject,
  toProjectDetail,
  toProjectSummary,
  type Project,
  type ProjectDetail,
  type ProjectSummary,
} from "@src/types";
import apiClient from "./apiClient";

const PROJECTS_PAGE_LIMIT = 20;

const searchProjects = async (searchQuery: string): Promise<Project[]> => {
  const { data } = await apiClient.post<ProjectSearchResponseDto>(PROJECTS_SEARCH_ENDPOINT, {
    pagination: { offset: 0, limit: PROJECTS_PAGE_LIMIT },
    ...(searchQuery && { searchQuery }),
  });
  return data.projects.map(toProject);
};

export interface ProjectListResult {
  items: ProjectSummary[];
  total: number;
  offset: number;
  limit: number;
  hasMore: boolean;
}

// Full, richer list for the Customers > Projects page — distinct from `search` above (which
// stays a lean one-shot type-ahead for filter pickers). Paged via infinite scroll, mirroring
// services/accounts.ts.
async function listProjects(searchQuery: string, offset: number): Promise<ProjectListResult> {
  const q = searchQuery.trim();
  const { data } = await apiClient.post<ProjectSearchResponseDto>(PROJECTS_SEARCH_ENDPOINT, {
    pagination: { offset, limit: PROJECTS_PAGE_LIMIT },
    ...(q ? { searchQuery: q } : {}),
  });
  const items = data.projects.map(toProjectSummary);
  return {
    items,
    total: data.total,
    offset: data.offset,
    limit: data.limit,
    hasMore: items.length > 0 && (data.hasMore ?? data.offset + items.length < data.total),
  };
}

const getProject = async (id: string): Promise<ProjectDetail> => {
  const { data } = await apiClient.get<ProjectDetailDto>(PROJECT_ENDPOINT(id));
  return toProjectDetail(data);
};

export const projects = {
  search: (searchQuery: string) =>
    queryOptions({
      queryKey: ["projects", "search", searchQuery],
      queryFn: () => searchProjects(searchQuery),
    }),

  list: (searchQuery: string) =>
    infiniteQueryOptions({
      queryKey: ["projects", "list", searchQuery],
      queryFn: ({ pageParam }) => listProjects(searchQuery, pageParam),
      initialPageParam: 0,
      getNextPageParam: (last) => (last.hasMore ? last.offset + last.limit : undefined),
    }),

  get: (id: string) =>
    queryOptions({
      queryKey: ["project", id],
      queryFn: () => getProject(id),
    }),
};
