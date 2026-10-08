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

import type { EntityRefDto } from "./case.dto";

// The backend's /change-requests/search filter only accepts these 8 (documented gap: `new`,
// `assess`, `authorize` — the pre-workflow states — aren't filterable, only returnable).
export type ChangeRequestFilterableState =
  | "customer_approval"
  | "scheduled"
  | "implement"
  | "review"
  | "customer_review"
  | "rollback"
  | "closed"
  | "canceled";

// The full state set a change request can actually be in (search/detail responses), including the
// 3 pre-workflow states the search filter can't target.
export type ChangeRequestState = ChangeRequestFilterableState | "new" | "assess" | "authorize";

export type ChangeRequestImpact = "high" | "medium" | "low";

export interface ChangeRequestSearchViewDto {
  id: string;
  number: string;
  subject: string | null;
  description: string | null;
  project: EntityRefDto;
  case: EntityRefDto | null;
  deployment: EntityRefDto | null;
  deployedProduct: EntityRefDto | null;
  product: EntityRefDto | null;
  assignedEngineer: EntityRefDto | null;
  assignedTeam: EntityRefDto | null;
  plannedStartOn: string | null;
  plannedEndOn: string | null;
  duration: string | null;
  impact: ChangeRequestImpact | null;
  state: ChangeRequestState | null;
  type: string | null;
  createdOn: string;
  updatedOn: string;
}

export interface ChangeRequestDetailDto extends ChangeRequestSearchViewDto {
  createdBy: string;
  justification: string | null;
  impactDescription: string | null;
  serviceOutage: string | null;
  communicationPlan: string | null;
  rollbackPlan: string | null;
  testPlan: string | null;
  hasCustomerApproved: boolean;
  hasCustomerReviewed: boolean;
  /**
   * WSO2's answer to a time the customer proposed ("agree" / "disagree"), and the backend's derived read model of that
   * conversation. A change WSO2 scheduled by accepting the customer's proposal is answered "agree" but never stamped
   * `hasCustomerApproved` (no staff action records the customer's approval).
   */
  confirmCustomerUpdatedDate?: string | null;
  customerProposal?: { answer?: string | null } | null;
  approvedBy: EntityRefDto | null;
  approvedOn: string | null;
}

export interface ChangeRequestSearchPayloadDto {
  filters?: {
    projectIds?: string[];
    searchQuery?: string;
    states?: ChangeRequestFilterableState[];
    impacts?: ChangeRequestImpact[];
    closedStartDate?: string;
    closedEndDate?: string;
  };
  sortBy?: {
    field?: "createdOn" | "updatedOn";
    order?: "asc" | "desc";
  };
  pagination?: {
    offset?: number;
    limit?: number;
  };
}

export interface ChangeRequestSearchResponseDto {
  changeRequests: ChangeRequestSearchViewDto[];
  total: number;
  limit: number;
  offset: number;
}

// minProperties: 1 on the backend — caller must send at least one field.
export interface PatchChangeRequestPayloadDto {
  /** "YYYY-MM-DD HH:MM:SS", or null to clear a previously-set planned start. */
  plannedStartOn?: string | null;
  // isCustomerApproved / isCustomerReviewed are deliberately not modeled: they are the CUSTOMER's own answer (given in the
  // Customer Portal) and the backend refuses them from staff with a 400.
}

export interface PatchChangeRequestResponseDto {
  id: string;
  updatedOn?: string;
  updatedBy?: string;
}

// Create-only enums (not part of the search/detail response's looser `type`/`state` strings) —
// mirrors the webapp's BeChangeRequestType/Priority/Risk/Category.
export type ChangeRequestType = "standard" | "normal" | "emergency" | "model" | "site_reliability_ops" | "azure";

export type ChangeRequestPriority = "critical" | "high" | "moderate" | "low";

export type ChangeRequestRisk = "high" | "moderate" | "low";

export type ChangeRequestCategory =
  | "hardware"
  | "software"
  | "service"
  | "system_software"
  | "applications_software"
  | "network"
  | "telecom"
  | "documentation"
  | "other"
  | "regular_release_cloud"
  | "hotfix_release_cloud"
  | "devops"
  | "cloud_computing";

/**
 * `POST /change-requests` body (ServiceNow data source only). Mirrors the webapp's
 * BeCreateChangeRequestPayload: `subject` is the only required field; every ID field
 * (serviceId/serviceOfferingId/configurationItemId/groupId/assignedEngineerId/requestedById) is a
 * portal UUID resolved server-side, via the matching search endpoint (see
 * services/changeRequestLookups.ts). `plannedStartDate`/`plannedEndDate` are
 * "YYYY-MM-DD HH:MM:SS" strings.
 */
export interface ChangeRequestCreatePayloadDto {
  subject: string;
  category?: ChangeRequestCategory;
  serviceId?: string;
  serviceOfferingId?: string;
  configurationItemId?: string;
  priority?: ChangeRequestPriority;
  impact?: ChangeRequestImpact;
  type?: ChangeRequestType;
  state?: ChangeRequestState;
  groupId?: string;
  assignedEngineerId?: string;
  risk?: ChangeRequestRisk;
  requestedById?: string;
  description?: string;
  justification?: string;
  implementationPlan?: string;
  riskImpactAnalysis?: string;
  backoutPlan?: string;
  testPlan?: string;
  plannedStartDate?: string;
  plannedEndDate?: string;
  comment?: string;
  workNote?: string;
}

export interface ChangeRequestCreateResponseDto {
  message: string;
  changeRequest: {
    id: string;
    number: string;
    createdOn: string;
    createdBy: string;
  };
}
