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
import type {
  ChangeRequestDetailDto,
  ChangeRequestImpact,
  ChangeRequestSearchViewDto,
  ChangeRequestState,
} from "./changeRequest.dto";
import { parseOptionalBackendTimestamp } from "@utils/dateTime";

export interface ChangeRequestSummary {
  id: string;
  number: string;
  subject: string;
  project: EntityRefDto;
  case: EntityRefDto | null;
  deployment: EntityRefDto | null;
  product: EntityRefDto | null;
  assignedEngineer: EntityRefDto | null;
  assignedTeam: EntityRefDto | null;
  plannedStartOn: string | null;
  plannedEndOn: string | null;
  duration: string | null;
  impact: ChangeRequestImpact | null;
  state: ChangeRequestState | null;
  createdOn: Date;
  updatedOn: Date;
}

export interface ChangeRequestDetail extends ChangeRequestSummary {
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
  /**
   * WSO2 accepted the time the customer proposed and scheduled the change by it (see {@link isProposedTimeAccepted}).
   * `hasCustomerApproved` stays false then (no staff action records the customer's approval), so "Customer Approved: No" would
   * mislead: the page reads "Proposed time accepted". False while the change is in Customer Approval, whatever answer stands.
   */
  proposedTimeAccepted: boolean;
  approvedBy: EntityRefDto | null;
  approvedOn: Date | null;
}

export function toChangeRequestSummary(dto: ChangeRequestSearchViewDto): ChangeRequestSummary {
  return {
    id: dto.id,
    number: dto.number,
    subject: dto.subject ?? "(No subject)",
    project: dto.project,
    case: dto.case,
    deployment: dto.deployment,
    product: dto.product,
    assignedEngineer: dto.assignedEngineer,
    assignedTeam: dto.assignedTeam,
    plannedStartOn: dto.plannedStartOn,
    plannedEndOn: dto.plannedEndOn,
    duration: dto.duration,
    impact: dto.impact,
    state: dto.state,
    createdOn: parseOptionalBackendTimestamp(dto.createdOn) ?? new Date(NaN),
    updatedOn: parseOptionalBackendTimestamp(dto.updatedOn) ?? new Date(NaN),
  };
}

/** The states a change request is in once it has moved on from Customer Approval (Scheduled, then every state after it). */
const STATES_PAST_CUSTOMER_APPROVAL: readonly string[] = [
  "scheduled",
  "implement",
  "review",
  "customer_review",
  "rollback",
  "closed",
  "canceled",
];

/**
 * Whether WSO2 accepted a time the customer proposed AND the change request has moved on from Customer Approval because
 * of it (Scheduled or later). The answer (`agreed`, or the raw `agree`) is only what WSO2 once said: nothing clears it when
 * the customers are asked again, so a change that is (back) in Customer Approval, or in a state not known here, with an
 * Agree standing was NOT scheduled by it and is waiting for the customer's own answer. Reading it as accepted there would
 * tell staff there is nothing left to answer. The CSM webapp's `customerApprovedDisplay` applies the same gate.
 */
export function isProposedTimeAccepted(
  dto: Pick<ChangeRequestDetailDto, "state" | "customerProposal" | "confirmCustomerUpdatedDate">,
): boolean {
  const agreed =
    dto.customerProposal?.answer === "agreed" || dto.confirmCustomerUpdatedDate?.trim().toLowerCase() === "agree";
  return agreed && !!dto.state && STATES_PAST_CUSTOMER_APPROVAL.includes(dto.state);
}

export function toChangeRequestDetail(dto: ChangeRequestDetailDto): ChangeRequestDetail {
  return {
    ...toChangeRequestSummary(dto),
    description: dto.description,
    createdBy: dto.createdBy,
    justification: dto.justification,
    impactDescription: dto.impactDescription,
    serviceOutage: dto.serviceOutage,
    communicationPlan: dto.communicationPlan,
    rollbackPlan: dto.rollbackPlan,
    testPlan: dto.testPlan,
    hasCustomerApproved: dto.hasCustomerApproved,
    hasCustomerReviewed: dto.hasCustomerReviewed,
    proposedTimeAccepted: isProposedTimeAccepted(dto),
    approvedBy: dto.approvedBy,
    approvedOn: parseOptionalBackendTimestamp(dto.approvedOn),
  };
}
