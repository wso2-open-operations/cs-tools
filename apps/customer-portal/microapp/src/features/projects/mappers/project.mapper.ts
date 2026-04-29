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

import { parseApiDate } from "@shared/utils/date.utils";
import { stripHtmlTags } from "@shared/utils/string.utils";
import type {
  DeploymentProductDto,
  ProjectDeploymentDto,
  ProjectDto,
  ProjectsDto,
} from "@features/projects/types/project.dto";
import type { Deployment, Product, Project, ProjectInfo, ProjectStatus } from "@features/projects/types/project.model";

export function toProjectSummary(dto: ProjectsDto["projects"][number]): Project {
  return {
    id: dto.id,
    projectKey: dto.key,
    name: dto.name,
    createdOn: parseApiDate(dto.createdOn),
    description: dto.description ? dto.description.replace(/<\/?[^>]+(>|$)/g, "") : "",
    metrics: {
      outstanding: dto.outstandingCount,
      chats: dto.activeChatsCount,
    },
    status: dto.slaStatus as ProjectStatus,
    type: dto.type?.label ?? "N/A",
  };
}

export function toProject(dto: ProjectDto): ProjectInfo {
  return {
    id: dto.id,
    projectKey: dto.key,
    name: dto.name,
    createdOn: parseApiDate(dto.createdOn),
    description: stripHtmlTags(dto.description),
    type: dto.type?.label ?? "N/A",
    agentEnabled: dto.account.hasAgent,
    kbReferencesEnabled: dto.account.hasKbReferences,
    typeId: dto.type?.id,
  };
}

export function toDeployment(dto: ProjectDeploymentDto): Deployment {
  return {
    id: dto.id,
    name: dto.name,
    createdOn: parseApiDate(dto.createdOn),
    updatedOn: parseApiDate(dto.updatedOn),
    url: dto.url ?? undefined,
    typeId: dto.type.id,
    projectId: dto.project.id,
    type: dto.type.label,
  };
}

export function toProduct(dto: DeploymentProductDto): Product {
  return {
    id: dto.id,
    createdOn: parseApiDate(dto.createdOn),
    updatedOn: parseApiDate(dto.updatedOn),
    description: dto.description ?? undefined,
    name: dto.product.label,
    deploymentId: dto.deployment.id,
    versionId: dto.version?.id ?? undefined,
    version: dto.version?.label,
  };
}
