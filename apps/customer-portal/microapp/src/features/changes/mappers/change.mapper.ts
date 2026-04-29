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

import { parseApiDate } from "@shared/utils/date.utils";
import type { ChangeRequestDto, ChangeRequestsDto } from "@features/changes/types/change.dto";
import type { ChangeRequest, ChangeRequestSummary } from "@features/changes/types/change.model";

export function toChangeRequestSummary(dto: ChangeRequestsDto["changeRequests"][number]): ChangeRequestSummary {
  return {
    id: dto.id,
    internalId: dto.case?.internalId,
    number: dto.number,
    title: dto.title,
    description: dto.case?.label ?? "",
    requestType: dto.type?.label,
    impactId: dto.impact?.id,
    statusId: dto.state?.id,
    assignedTeam: dto.assignedTeam?.label,
    endDate: dto.endDate ? parseApiDate(dto.endDate) : undefined,
    createdOn: parseApiDate(dto.createdOn),
    updatedOn: parseApiDate(dto.updatedOn),
  };
}

export function toChangeRequest(dto: ChangeRequestDto): ChangeRequest {
  return {
    id: dto.id,
    internalId: dto.case?.internalId,
    number: dto.number,
    title: dto.title,
    description: dto.case?.label ?? "",
    requestType: dto.type?.label,
    impactId: dto.impact?.id,
    statusId: dto.state?.id,
    endDate: dto.endDate ? parseApiDate(dto.endDate) : undefined,
    createdOn: parseApiDate(dto.createdOn),
    updatedOn: parseApiDate(dto.updatedOn),
    createdBy: dto.createdBy,
    approvedOn: dto.approvedOn ? parseApiDate(dto.approvedOn) : undefined,
    approvedBy: dto.approvedBy?.label ?? undefined,
    duration: dto.duration ?? undefined,
    hasCustomerApproved: dto.hasCustomerApproved,
    hasCustomerReviewed: dto.hasCustomerReviewed,
    assignedTeam: dto.assignedTeam?.label,
    serviceOutage: dto.serviceOutage ?? undefined,
    rollbackPlan: dto.rollbackPlan ?? undefined,
    communicationPlan: dto.communicationPlan ?? undefined,
    testPlan: dto.testPlan ?? undefined,
    deployment: dto.deployment?.label,
  };
}
