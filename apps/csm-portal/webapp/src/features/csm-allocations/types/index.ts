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

/** Allocation list sort fields the API accepts. */
export type AllocationSortField =
  | "startDate"
  | "endDate"
  | "state"
  | "allocationId"
  | "accountName"
  | "engineerName"
  | "utilization";

export type SortOrder = "asc" | "desc";

export interface AllocationPerson {
  id: string;
  name: string;
  email?: string;
}

export interface AllocationEngagement {
  id: string;
  displayId?: string;
  name?: string;
  type?: string;
  modeOfDelivery?: string;
  state?: string;
  businessUnit?: string;
  isPaid?: boolean;
  projectId?: string;
  plannedStartDate?: string;
  plannedEndDate?: string;
  actualStartDate?: string;
  actualEndDate?: string;
}

/** One engineer's allocation to a customer engagement. */
export interface Allocation {
  id: string;
  /** The human-readable reference, not the id. */
  allocationId?: string;
  state?: string;
  startDate?: string;
  endDate?: string;
  startTime?: string;
  endTime?: string;
  timezone?: string;
  /** Percent of the engineer's time. */
  utilization?: number;
  engagement: AllocationEngagement;
  account?: { id: string; name: string };
  engineer?: AllocationPerson;
  latestUpdateDate?: string;
}

export interface AllocationFilters {
  searchQuery?: string;
  accountId?: string;
  accountName?: string;
  engineerId?: string;
  engagementId?: string;
  states?: string[];
  /** YYYY-MM-DD. */
  from?: string;
  to?: string;
}

export interface SearchAllocationsPayload {
  pagination: { limit: number; offset: number };
  filters: AllocationFilters;
  sortBy: { field: AllocationSortField; order: SortOrder };
}

export interface SearchAllocationsResponse {
  allocations: Allocation[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

export interface AllocationUpdate {
  id: string;
  subject?: string;
  content?: string;
  state?: string;
  visibility?: string;
  cycleStartDate?: string;
  publishedDate?: string;
  createdOn: string;
  author?: AllocationPerson;
}

export interface AllocationUpdatesResponse {
  updates: AllocationUpdate[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

export interface PostAllocationUpdatePayload {
  subject: string;
  content: string;
}
