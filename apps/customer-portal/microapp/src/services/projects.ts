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

import apiClient from "@src/services/apiClient";
import type { Project, ProjectsDTO, ProjectStatsDTO, ProjectStatus } from "@src/types";

import { PROJECT_STATS_ENDPOINT, PROJECTS_ENDPOINT } from "@config/endpoints";
import { queryOptions } from "@tanstack/react-query";

const getAllProjects = async (): Promise<Project[]> => {
  const projects = (await apiClient.post<ProjectsDTO>(PROJECTS_ENDPOINT, {})).data.projects;
  const projectsWithStats = await Promise.all(
    projects.map(async (project) => {
      const stats = (await apiClient.get<ProjectStatsDTO>(PROJECT_STATS_ENDPOINT(project.id))).data;
      return mapProjectAndStatsDTOToProject(project, stats);
    }),
  );

  return projectsWithStats;
};

/* Mappers */
function mapProjectAndStatsDTOToProject(project: ProjectsDTO["projects"][number], stats: ProjectStatsDTO): Project {
  return {
    id: project.id,
    projectKey: project.key,
    name: project.name,
    createdOn: new Date(project.createdOn.replace(" ", "T")),
    description: project.description.replace(/<\/?[^>]+(>|$)/g, ""),
    metrics: {
      cases: stats.projectStats.openCases,
      chats: stats.projectStats.activeChats,
    },
    status: stats.projectStats.slaStatus as ProjectStatus,
    type: "Regular", // TODO:
  };
}

/* Query Options */
export const projects = {
  all: () =>
    queryOptions({
      queryKey: ["projects"],
      queryFn: getAllProjects,
    }),
};
