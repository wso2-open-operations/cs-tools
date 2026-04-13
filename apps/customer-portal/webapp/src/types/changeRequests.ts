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
  AuditMetadata,
  IdLabelRef,
  PaginationResponse,
  SearchRequestBase,
} from "@/types/common";


// Item type for a change request.
export type ChangeRequestItem = AuditMetadata & {
  id: string;
  number: string;
  title: string;
  project: IdLabelRef | null;
  case: IdLabelRef | null;
  deployment: IdLabelRef | null;
  deployedProduct: IdLabelRef | null;
  product: IdLabelRef | null;
  assignedEngineer: IdLabelRef | null;
  assignedTeam: IdLabelRef | null;
  startDate: string;
  endDate: string;
  duration: string | null;
  hasServiceOutage: boolean;
  impact: IdLabelRef | null;
  state: IdLabelRef | null;
  type: IdLabelRef | null;
};


// Response type for detailed change request information.
export type ChangeRequestDetails = ChangeRequestItem & {
  description: string | null;
  createdBy: string;
  justification: string | null;
  impactDescription: string | null;
  serviceOutage: string | null;
  communicationPlan: string | null;
  rollbackPlan: string | null;
  testPlan: string | null;
  hasCustomerApproved: boolean;
  hasCustomerReviewed: boolean;
  approvedBy: IdLabelRef | null;
  approvedOn: string | null;
};

// Response type for change request search results.
export type ChangeRequestSearchResponse = PaginationResponse & {
  changeRequests: ChangeRequestItem[];
};

// Response type for change request statistics.
export type ChangeRequestStats = {
  totalRequests: number;
  awaitingYourAction: number;
  ongoing: number;
  completed: number;
};

// Item type for change request state count.
export type ChangeRequestStateCount = IdLabelRef & { count: number };


// Response type for change request statistics breakdown.
export type ChangeRequestStatsResponse = {
  totalCount: number;
  activeCount?: number;
  outstandingCount?: number;
  stateCount: ChangeRequestStateCount[];
};

// Response type for patching a change request.
export type PatchChangeRequestResponse = AuditMetadata & {
  id: string;
};

// Model type for change request filters state.
export type ChangeRequestFilterValues = {
  stateId?: string;
  impactId?: string;
}

// Filter type for searching change requests.
export type ChangeRequestSearchFilters = {
  impactKey?: number;
  searchQuery?: string;
  stateKeys?: number[];
};

// Request type for searching change requests.
export type ChangeRequestSearchRequest = SearchRequestBase & {
  filters?: ChangeRequestSearchFilters;
};

// Request type for patching a change request.
export type PatchChangeRequestRequest = {
  plannedStartOn?: string;
  isCustomerApproved?: boolean;
  isCustomerReviewed?: boolean;
};

// Enum for change request decision mode.
export enum ChangeRequestDecisionMode {
  CUSTOMER_APPROVAL = "customerApproval",
  CUSTOMER_REVIEW = "customerReview",
  NONE = "none",
}

// Item type for a change request workflow stage.
export type ChangeRequestWorkflowStage = {
  name: string;
  description: string;
  completed: boolean;
  current: boolean;
  disabled: boolean;
}
