import type { EntityReference, Pagination } from "@src/types";

export interface ProjectsDTO extends Pagination {
  projects: ProjectDTO[];
}

interface ProjectDTO {
  id: string;
  name: string;
  key: string;
  createdOn: string;
  description: string;
}

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

export type ProjectDeploymentsDTO = ProjectDeploymentDTO[];

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
