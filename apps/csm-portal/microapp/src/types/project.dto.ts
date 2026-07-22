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

export type ProjectSubscriptionType =
  | "development_support"
  | "managed_cloud_subscription"
  | "evaluation_subscription"
  | "subscription"
  | "cloud_evaluation_support"
  | "internal"
  | "platformer_subscription"
  | "cloud_support"
  | "professional_services";

export interface ProjectDto {
  id: string;
  accountId?: string;
  sfId?: string;
  name: string;
  // openapi.yaml documents this field as `projectKey`, but the live search
  // endpoint actually returns `key` — verified in practice, matching the
  // webapp's own csmProjects.ts, which carries the same correction.
  key?: string;
  subscriptionType: ProjectSubscriptionType;
  startDate?: string;
  endDate?: string;
  createdOn?: string;
  updatedOn?: string;
}

export interface ProjectSearchPayloadDto {
  pagination?: { offset?: number; limit?: number };
  searchQuery?: string;
}

export interface ProjectSearchResponseDto {
  projects: ProjectDto[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

/** Parent-account reference embedded in GET /projects/{id} — the entity
 * service JOINs the account, so the project detail view gets the account
 * name (and a few account facts) with no extra call. `tier` is a free-form
 * label here (e.g. "Enterprise"), not the lowercase AccountTier enum. */
export interface ProjectAccountRefDto {
  id: string;
  name: string;
  activationDate?: string | null;
  tier?: string;
  region?: string | null;
  agentEnabled?: boolean;
  kbReferencesEnabled?: boolean;
}

/** GET /projects/{id} — enriched single-project shape, distinct from the
 * search-result row: embeds the parent `account` instead of a bare `accountId`. */
export interface ProjectDetailDto {
  id: string;
  sfId?: string;
  name: string;
  key?: string;
  subscriptionType: ProjectSubscriptionType;
  startDate?: string;
  endDate?: string;
  createdOn?: string;
  updatedOn?: string;
  account?: ProjectAccountRefDto;
}
