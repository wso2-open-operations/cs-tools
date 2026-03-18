import type { EntityReference, Pagination } from "@src/types";

export interface ProjectsDTO extends Pagination {
  projects: ProjectSummaryDTO[];
}

export interface ProjectDTO {
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

type ProjectSummaryDTO = Pick<ProjectDTO, "id" | "name" | "key" | "createdOn" | "description">;

export interface ProjectStatsDTO {
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

export interface ProjectDeploymentsDTO extends Pagination {
  deployments: ProjectDeploymentDTO[];
}

export interface ProjectDeploymentDTO {
  id: string;
  name: string;
  createdOn: string;
  updatedOn: string;
  description: string | null;
  url: string | null;
  project: EntityReference;
  type: EntityReference;
}

export type DeploymentProductsDTO = DeploymentProductDTO[];

export interface DeploymentProductDTO {
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
