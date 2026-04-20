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

import type {
  IdLabelRef,
  PaginationResponse,
  SearchRequestBase,
} from "@/types/common";

// Item type for basic project definition returned in search list responses.
export type ProjectListItem = {
  id: string;
  name: string;
  key: string;
  createdOn: string;
  description: string;
  type?: IdLabelRef;
  hasAgent: boolean;
  hasPdpSubscription?: boolean;
  hasKbReferences?: boolean;
  activeCasesCount: number;
  activeChatsCount: number;
  slaStatus: string;
};

// Item type for account nested in project details response.
export type ProjectDetailsAccount = {
  id: string;
  hasAgent?: boolean;
  hasKbReferences?: boolean;
  name: string;
  activationDate?: string | null;
  deactivationDate?: string | null;
  supportTier?: string;
  region?: string | null;
  ownerEmail?: string | null;
  technicalOwnerEmail?: string | null;
};

// Response type for detailed project information including account/subscription details.
export type ProjectDetails = {
  id: string;
  key: string;
  name: string;
  description: string;
  createdOn: string;
  hasAgent?: boolean;
  hasKbReferences?: boolean;
  type: IdLabelRef;
  sfId?: string;
  hasPdpSubscription?: boolean;
  hasSr: boolean;
  startDate?: string;
  endDate?: string;
  account?: ProjectDetailsAccount;
  totalQueryHours?: number;
  consumedQueryHours?: number;
  remainingQueryHours?: number;
  goLiveDate?: string | null;
  goLivePlanDate?: string | null;
  totalOnboardingHours?: number;
  consumedOnboardingHours?: number;
  remainingOnboardingHours?: number;
  onboardingExpiryDate?: string | null;
  onboardingStatus?: string | null;
};

// Response type for project search responses.
export type SearchProjectsResponse = PaginationResponse & {
  projects: ProjectListItem[];
};

// Filter type for project search filters.
export type ProjectSearchFilters = {
  searchQuery?: string;
};

// Request type for searching projects.
export type SearchProjectsRequest = SearchRequestBase & {
  filters?: ProjectSearchFilters;
};

// Model type for time zone option for portal metadata.
export type TimeZoneOption = {
  id: string;
  label: string;
};

// Model type for feature flags for the portal.
export type PortalFeatureFlags = {
  usageMetricsEnabled: boolean;
};

// Response type for global metadata.
export type PortalMetadataResponse = {
  timeZones: TimeZoneOption[];
  featureFlags?: PortalFeatureFlags;
};

// Response type for project support statistics.
export type ProjectSupportStats = {
  ongoingCases: number;
  resolvedPast30DaysCasesCount: number;
  resolvedChats: number;
  activeChats: number;
};

// Item type for core stats inside a project stats response.
export type ProjectStatsSummary = {
  openCases: number;
  activeChats: number;
  deployments: number;
  slaStatus: string;
};

// Item type for recent activity inside a project stats response.
export type ProjectRecentActivity = {
  totalHours: number;
  billableHours: number;
  lastDeploymentOn: string;
  systemHealth?: string;
};

// Response type for project stats responses.
export type ProjectStatsResponse = {
  projectStats: ProjectStatsSummary;
  recentActivity: ProjectRecentActivity;
};

// Request type for patching a project.
export type PatchProjectRequest = {
  hasAgent?: boolean;
  hasKbReferences?: boolean;
};
