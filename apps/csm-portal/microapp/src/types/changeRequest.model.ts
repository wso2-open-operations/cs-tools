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
    approvedBy: dto.approvedBy,
    approvedOn: parseOptionalBackendTimestamp(dto.approvedOn),
  };
}
