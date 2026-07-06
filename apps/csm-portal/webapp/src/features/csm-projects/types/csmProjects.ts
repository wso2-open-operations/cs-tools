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
  // The search endpoint returns this as `key` (not `projectKey`).
  key: string;
  subscriptionType: SubscriptionType;
  startDate: string;
  endDate: string;
  createdAt: string;
  updatedAt: string;
}

/**
 * Parent-account reference embedded in the project detail response
 * (`GET /projects/{id}`). The backend JOINs the account, so the project view
 * gets the account name (and a few account facts) with no extra call. `tier` is
 * free-form here (e.g. "Enterprise"), not the lowercase {@link AccountTier} enum.
 */
export interface ProjectAccountRef {
  id: string;
  name: string;
  activationDate: string | null;
  tier: string;
  region?: string | null;
  agentEnabled: boolean;
  kbReferencesEnabled: boolean;
}

/**
 * Enriched single-project shape returned by `GET /projects/{id}`. Distinct from
 * {@link Project} (the search-result row): it embeds the parent `account`, and
 * uses `createdOn` / `updatedOn` rather than the search row's
 * `createdAt` / `updatedAt`.
 */
export interface ProjectDetails {
  id: string;
  account: ProjectAccountRef;
  sfId: string;
  name: string;
  key: string;
  subscriptionType: SubscriptionType;
  startDate: string;
  endDate: string;
  createdOn: string;
  updatedOn: string;
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
