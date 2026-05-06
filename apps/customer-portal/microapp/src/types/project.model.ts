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

export type ProjectStatus = "All Good" | "Needs Attention";
export type ProjectType = "Managed Cloud" | "Regular";
export type ProjectMetricKey = "outstanding" | "chats";
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
  type: string;
}

export interface ProjectInfo {
  id: string;
  projectKey: string;
  name: string;
  createdOn: Date;
  description: string;
  type: string;
  agentEnabled: boolean;
  kbReferencesEnabled: boolean;
  typeId?: string;
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
  version?: string;
}
