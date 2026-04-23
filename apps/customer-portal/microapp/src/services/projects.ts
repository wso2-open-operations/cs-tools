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
import type { ProjectCardProps } from "@components/features/projects";

import { PROJECTS_ENDPOINT } from "@config/endpoints";

export interface ProjectsResponseType {
  projects: {
    sysId: string;
    name: string;
    description: string;
    projectKey: string;
    createdOn: string;
    activeChatsCount: number;
    openCasesCount: number;
  }[];
  pagination: { offset: number; limit: number; totalRecords: number };
}

export const getProjects = async (): Promise<ProjectCardProps[]> => {
  const response = await apiClient.get<ProjectsResponseType>(PROJECTS_ENDPOINT);

  return response.data.projects.map((project) => ({
    id: project.projectKey,
    name: project.name,
    description: project.description,
    // TODO: determine project type from backend
    // Fallback to "Managed Cloud" until backend provides explicit field
    type: "Managed Cloud",
    // TODO: determine project status from backend
    // Fallback to "All Good" until backend provides explicit field
    status: "All Good",
    numberOfOpenCases: project.openCasesCount,
    metrics: {
      cases: project.openCasesCount,
      chats: project.activeChatsCount,
      // TODO: populate remaining metrics when supported by backend
    },
  }));
};
