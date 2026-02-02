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

// Basic project definition returned in search list responses.
export interface ProjectListItem {
  id: string;
  name: string;
  key: string;
  createdOn: string;
  description: string;
}

// Project Search Response.
export interface SearchProjectsResponse {
  offset: number;
  limit: number;
  projects: ProjectListItem[];
  totalRecords: number;
}

// User profile information.
export interface UserProfile {
  name: string;
  email: string;
  avatar: string;
  role: string;
}

// Project support statistics.
export interface ProjectSupportStats {
  totalCases: number;
  activeChats: number;
  sessionChats: number;
  resolvedChats: number;
}

// Project cases statistics.
export interface ProjectCasesStats {
  totalCases: number;
  openCases: number;
  averageResponseTime: number;
  activeCases: {
    workInProgress: number;
    waitingOnClient: number;
    waitingOnWso2: number;
    total: number;
  };
  outstandingIncidents: {
    medium: number;
    high: number;
    critical: number;
    total: number;
  };
  resolvedCases: {
    total: number;
    currentMonth: number;
  };
}

export interface TrendData {
  value: string;
  direction: "up" | "down";
  color: "success" | "error" | "info" | "warning";
}

export interface DashboardMockStats {
  totalCases: {
    value: number;
    trend: TrendData;
  };
  openCases: {
    value: number;
    trend: TrendData;
  };
  resolvedCases: {
    value: number;
    trend: TrendData;
  };
  avgResponseTime: {
    value: string;
    trend?: TrendData;
  };
  casesTrend: Array<{
    name: string;
    TypeA: number;
    TypeB: number;
    TypeC: number;
    TypeD: number;
  }>;
}
