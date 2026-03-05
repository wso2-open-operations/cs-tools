export type ProjectStatus = "All Good" | "Needs Attention";
export type ProjectType = "Managed Cloud" | "Regular";
export type ProjectMetricKey = "cases" | "chats";
export type ProjectMetricValue = number | string;
export type ProjectMetrics = Partial<Record<ProjectMetricKey, ProjectMetricValue | undefined>>;

export interface Project {
  id: string;
  projectKey: string;
  name: string;
  createdOn: Date;
  description: string;
  metrics: ProjectMetrics;
  status?: ProjectStatus;
  type: ProjectType;
}

export interface Deployment {
  id: string;
  name: string;
  createdOn: Date;
  updatedOn: Date;
  url?: string;
  typeId: string;
  projectId: string;
  type: string;
}

export interface Product {
  id: string;
  createdOn: Date;
  updatedOn: Date;
  description?: string;
  name: string;
  deploymentId: string;
  versionId?: string;
}
