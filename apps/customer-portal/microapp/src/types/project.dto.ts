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

import type { EntityReference, Pagination } from "@src/types";

export interface ProjectsDto extends Pagination {
  projects: ProjectSummaryDto[];
}

export interface ProjectDto {
  id: string;
  key: string;
  name: string;
  createdOn: string;
  description: string;
  type: EntityReference | null;
  sfId: string;
  hasSr: boolean;
  startDate: string | null;
  endDate: string | null;
  account: {
    id: string;
    hasAgent: boolean;
    hasKbReferences: boolean;
    name: string;
    activationDate: string | null;
    deactivationDate: string | null;
    supportTier: string | null;
    region: string | null;
    ownerEmail: string | null;
    technicalOwnerEmail: string | null;
  };
  totalQueryHours: number;
  consumedQueryHours: number;
  remainingQueryHours: number;
  goLiveDate: string | null;
  goLivePlanDate: string | null;
  totalOnboardingHours: number;
  remainingOnboardingHours: number;
  onboardingExpiryDate: string | null;
  onboardingStatus: string | null;
}

type ProjectSummaryDto = Pick<ProjectDto, "id" | "name" | "key" | "createdOn" | "description" | "type"> & {
  activeCasesCount: number;
  outstandingCount: number;
  activeChatsCount: number;
  slaStatus: string;
};

export interface ProjectStatsDto {
  projectStats: {
    openCases: number;
    activeChats: number;
    deployments: number;
    slaStatus: string;
  };
  recentActivity: {
    totalTimeLogged: number;
    billableHours: number;
    lastDeploymentOn: string;
    systemHealth: string;
  };
}

export interface ProjectDeploymentsDto extends Pagination {
  deployments: ProjectDeploymentDto[];
}

export interface ProjectDeploymentDto {
  id: string;
  name: string;
  createdOn: string;
  updatedOn: string;
  description: string | null;
  url: string | null;
  project: EntityReference;
  type: EntityReference;
}

export interface DeploymentProductsDto extends Pagination {
  deployedProducts: DeploymentProductDto[];
}

export interface DeploymentProductDto {
  id: string;
  createdOn: string;
  updatedOn: string;
  description: string | null;
  product: EntityReference;
  deployment: EntityReference;
  version: EntityReference | null;
  cores: number | null;
  tps: number | null;
  releasedOn: string | null;
  endOfLifeOn: string | null;
  updateLevel: string;
}

export interface ProjectFeaturesDto {
  acceptedSeverityValues: EntityReference[];
  hasServiceRequestWriteAccess: boolean;
  hasServiceRequestReadAccess: boolean;
  hasSraWriteAccess: boolean;
  hasChangeRequestReadAccess: boolean;
  hasEngagementsReadAccess: boolean;
  hasUpdatesReadAccess: boolean;
  hasTimeLogsReadAccess: boolean;
  hasDeploymentWriteAccess: boolean;
  hasDeploymentReadAccess: boolean;
  defaultCaseProductCategories: string[] | null;
}
