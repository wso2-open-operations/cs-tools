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

// Detailed project information including subscription details.
export interface ProjectDetails {
  id: string;
  name: string;
  key: string;
  description: string;
  createdOn: string;
  type: string;
  subscription: {
    startDate: string | null;
    endDate: string | null;
    supportTier: string | null;
  };
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
}

// User details response from API.
export interface UserDetails {
  id: string;
  email: string;
  lastName: string;
  firstName: string;
  timeZone: string;
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
  outstandingCases: {
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

// Project time tracking statistics.
export interface ProjectTimeTrackingStats {
  totalHours: number;
  billableHours: number;
  nonBillableHours: number;
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

// Case List Item
export interface CaseListItem {
  id: string;
  internalId: string;
  number: string;
  createdOn: string;
  title: string;
  description: string;
  /** API may return string or { id, label } object. */
  assignedEngineer: string | { id: string; label: string } | null;
  project: {
    id: string;
    label: string;
  };
  issueType: {
    id: string;
    label: string;
  } | null;
  deployedProduct: {
    id: string;
    label: string;
  } | null;
  deployment: {
    id: string;
    label: string;
  } | null;
  severity: {
    id: string;
    label: string;
  } | null;
  status: {
    id: string;
    label: string;
  } | null;
}

// Case Search Response
export interface CaseSearchResponse {
  cases: CaseListItem[];
  totalRecords: number;
  offset: number;
  limit: number;
  projects?: {
    id: string;
    label: string;
  }[];
}

// Case details
export interface CaseDetailsAccount {
  type: string | null;
  id: string;
  name: string | null;
}

export interface CaseDetailsProject {
  id: string;
  name: string | null;
}

export interface CaseStatus {
  id: number;
  label: string | null;
}

export interface CaseDetails {
  id: string;
  internalId: string;
  number: string | null;
  createdOn: string | null;
  updatedOn: string | null;
  title: string | null;
  description: string | null;
  slaResponseTime: string | null;
  product: string | null;
  account: CaseDetailsAccount | null;
  csManager: string | null;
  assignedEngineer: string | null;
  project: CaseDetailsProject | null;
  deployment: { id: string; label: string } | null;
  deployedProduct: string | null;
  issueType: string | null;
  state: CaseStatus | null;
  severity: CaseStatus | null;
}

// Case comment
export interface CaseComment {
  id: string;
  content: string;
  type: string;
  createdOn: string;
  createdBy: string;
  isEscalated: boolean;
}

// Response for case comments list
export interface CaseCommentsResponse {
  comments: CaseComment[];
  totalRecords: number;
  offset: number;
  limit: number;
}

// Project Stats Response
export interface ProjectStatsResponse {
  projectStats: {
    activeChats: number;
    deployments: number;
    openCases: number;
    slaStatus: string;
  };
  recentActivity: {
    billableHours: number;
    lastDeploymentOn: string;
    systemHealth: string;
    totalTimeLogged: number;
  };
}

// Metadata Item (Status, Severity, CaseType)
export interface MetadataItem {
  id: string;
  label: string;
}

// Response for case metadata (fetching possible statuses, severities, types)
export interface CaseMetadataResponse {
  statuses?: MetadataItem[];
  severities?: MetadataItem[];
  issueTypes?: MetadataItem[];
  deployments?: MetadataItem[];
}

// Chat history list item (support chat session summary).
export interface ChatHistoryItem {
  chatId: string;
  title: string;
  startedTime: string;
  messages: number;
  kbArticles: number;
  status: string;
}

// Response for project chat history list.
export interface ChatHistoryResponse {
  chatHistory: ChatHistoryItem[];
}

// Interface for all cases filters state
export interface AllCasesFilterValues {
  statusId?: string;
  severityId?: string;
  issueTypes?: string;
  deploymentId?: string;
}

// Product deployed in an environment.
export interface DeploymentProduct {
  id: string;
  name: string;
  version: string;
  supportStatus: string;
  description: string;
  cores: number;
  tps: number;
  releasedDate: string;
  endOfLifeDate: string;
  updateLevel: string;
}

// Document attached to a deployment.
export interface DeploymentDocument {
  id: string;
  name: string;
  category: string;
  sizeBytes: number;
  uploadedAt: string;
  uploadedBy: string;
}

// Single deployment environment.
export interface Deployment {
  id: string;
  name: string;
  status: "Healthy" | "Warning";
  url: string;
  version: string;
  description: string;
  products: DeploymentProduct[];
  documents: DeploymentDocument[];
  deployedAt: string;
  uptimePercent: number;
}

// Response for project deployments list.
export interface DeploymentsResponse {
  deployments: Deployment[];
}

// Case attachment item.
export interface CaseAttachment {
  id: string;
  name: string;
  type: string;
  sizeBytes: string;
  downloadUrl: string;
  createdOn: string;
  createdBy: string;
}

// Response for case attachments list.
export interface CaseAttachmentsResponse {
  limit: number;
  offset: number;
  totalRecords: number;
  attachments: CaseAttachment[];
}

// Updates statistics response.
export interface UpdatesStats {
  productsTracked: number | null;
  totalUpdatesInstalled: number | null;
  totalUpdatesInstalledBreakdown?: { regular: number; security: number };
  totalUpdatesPending: number | null;
  totalUpdatesPendingBreakdown?: { regular: number; security: number };
  securityUpdatesPending: number | null;
}

// Product update levels.
export interface ProductUpdateLevelEntry {
  "product-base-version": string;
  channel: string;
  "update-levels": number[];
}

// One product's update levels.
export interface ProductUpdateLevelsItem {
  "product-name": string;
  "product-update-levels": ProductUpdateLevelEntry[];
}

// Product update levels response.
export type ProductUpdateLevelsResponse = ProductUpdateLevelsItem[];

// Case classification response.
export interface CaseClassificationResponse {
  issueType: string;
  severityLevel: string;
  case_info: {
    description: string;
    shortDescription: string;
    productName: string;
    productVersion: string;
    environment: string;
    tier: string;
    region: string;
  };
}
