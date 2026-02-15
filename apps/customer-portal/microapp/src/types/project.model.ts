export type ProjectStatus = "All Good";
export type ProjectType = "Managed Cloud" | "Regular";
export type ProjectMetricKey = "cases" | "chats";
export type ProjectMetricValue = number | string;
export type ProjectMetrics = Partial<Record<ProjectMetricKey, ProjectMetricValue>>;

export interface Project {
  id: string;
  projectKey: string;
  name: string;
  createdOn: Date;
  description: string;
  metrics: ProjectMetrics;
  status: ProjectStatus;
  type: ProjectType;
}
