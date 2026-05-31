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

export type SubscriptionType =
  | "development_support"
  | "managed_cloud_subscription"
  | "evaluation_subscription"
  | "subscription"
  | "cloud_evaluation_support"
  | "internal"
  | "platformer_subscription"
  | "cloud_support"
  | "professional_services";

export interface Project {
  id: string;
  accountId: string;
  sfId: string;
  name: string;
  projectKey: string;
  subscriptionType: SubscriptionType;
  startDate: string;
  endDate: string;
  createdAt: string;
  updatedAt: string;
}

export interface SearchProjectsRequest {
  pagination?: {
    limit?: number;
    offset?: number;
  };
  searchQuery?: string;
}

export interface SearchProjectsResponse {
  projects: Project[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

// ---------------------------------------------------------------------------
// Mock-only domain types used by csm-portal-webapp-wip pages (project detail,
// list filters, etc.). Kept alongside the real entity-service types above
// until the BFF exposes the cross-customer project model needed by those views.
// ---------------------------------------------------------------------------

import type { DashboardScope } from "@features/csm-dashboard/types/abtDashboard";

export type CsmProjectTier = "Platinum" | "Gold" | "Silver" | "Bronze";

export type CsmProjectStatus = "Active" | "Suspended" | "Onboarding";

export interface CsmProjectRow {
  id: string;
  name: string;
  customer: string;
  accountId: string;
  tier: CsmProjectTier;
  productType: string;
  status: CsmProjectStatus;
  updateLevel: string;
  openCaseCount: number;
  s0s1Count: number;
  breachedCount: number;
  lastActivityAt: string;
}

export interface CsmProjectsListResponse {
  scope: DashboardScope;
  projects: CsmProjectRow[];
}
