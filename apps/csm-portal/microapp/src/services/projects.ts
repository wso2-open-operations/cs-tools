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
import { PROJECTS_SEARCH_ENDPOINT } from "@config/endpoints";
import type { ProjectSearchResponseDto } from "@src/types";
import { toProject, type Project } from "@src/types";
import apiClient from "./apiClient";

const PROJECTS_PAGE_LIMIT = 20;

const searchProjects = async (searchQuery: string): Promise<Project[]> => {
  const { data } = await apiClient.post<ProjectSearchResponseDto>(PROJECTS_SEARCH_ENDPOINT, {
    pagination: { offset: 0, limit: PROJECTS_PAGE_LIMIT },
    ...(searchQuery && { searchQuery }),
  });
  return data.projects.map(toProject);
};

export const projects = {
  search: (searchQuery: string) =>
    queryOptions({
      queryKey: ["projects", "search", searchQuery],
      queryFn: () => searchProjects(searchQuery),
    }),
};
