import type { Pagination } from "@src/types";

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
