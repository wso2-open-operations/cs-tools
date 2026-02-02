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

/**
 * Basic project definition returned in search list responses.
 */
export interface ProjectListItem {
  /**
   * Unique identifier for the project.
   */
  id: string;
  /**
   * Name of the project.
   */
  name: string;
  /**
   * Key of the project.
   */
  key: string;
  /**
   * Timestamp when the project was created.
   */
  createdOn: string;
  /**
   * Description of the project.
   */
  description: string;
}

/**
 * Project Search Response.
 *
 * Response structure for paginated project search queries.
 * Contains pagination metadata along with the list of projects.
 */
export interface SearchProjectsResponse {
  /**
   * Index of the first record in the returned page.
   */
  offset: number;
  /**
   * Maximum number of records that can be returned in a page.
   */
  limit: number;
  /**
   * Array of projects matching the search criteria.
   */
  projects: ProjectListItem[];
  /**
   * Total number of records that match the search query across all pages.
   */
  totalRecords: number;
}

/**
 * User profile information.
 */
export interface UserProfile {
  name: string;
  email: string;
  avatar: string;
  role: string;
}

/**
 * Project support statistics.
 */
export interface ProjectSupportStats {
  /**
   * Total number of cases in the project.
   */
  totalCases: number;
  /**
   * Number of active chats in the project.
   */
  activeChats: number;
  /**
   * Number of chat sessions in the project.
   */
  sessionChats: number;
  /**
   * Number of resolved chats in the project.
   */
  resolvedChats: number;
}

/**
 * Project cases statistics.
 */
export interface ProjectCasesStats {
  /**
   * Total number of cases.
   */
  totalCases: number;
  /**
   * Number of open cases.
   */
  openCases: number;
  /**
   * Average response time in hours.
   */
  averageResponseTime: number;
  /**
   * Active cases breakdown.
   */
  activeCases: {
    workInProgress: number;
    waitingOnClient: number;
    waitingOnWso2: number;
    total: number;
  };
  /**
   * Outstanding incidents by severity.
   */
  outstandingIncidents: {
    medium: number;
    high: number;
    critical: number;
    total: number;
  };
  /**
   * Resolved cases info.
   */
  resolvedCases: {
    total: number;
    currentMonth: number;
  };
}

/**
 * Trend information for a statistic - Mock.
 */
export interface TrendData {
  value: string;
  direction: "up" | "down";
  color: "success" | "error" | "info" | "warning";
}

/**
 * Dashboard mock statistics.
 */
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
}
