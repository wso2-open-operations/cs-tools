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
import type { ServiceRequestDto, ServiceRequestsDto } from "@features/service-requests/types/service-request.dto";
import type { ServiceRequest, ServiceRequestSummary } from "@features/service-requests/types/service-request.model";

export function toServiceRequestSummary(dto: ServiceRequestsDto["cases"][number]): ServiceRequestSummary {
  return {
    id: dto.id,
    internalId: dto.internalId,
    number: dto.number,
    createdOn: parseApiDate(dto.createdOn),
    createdBy: dto.createdBy,
    title: dto.title,
    description: dto.description ?? "",
    assignee: dto.assignedTeam?.label,
    issueType: dto.issueType?.label,
    statusId: dto.status?.id,
    severityId: dto.severity?.id,
  };
}

export function toServiceRequest(dto: ServiceRequestDto): ServiceRequest {
  return {
    id: dto.id,
    internalId: dto.internalId,
    number: dto.number,
    createdOn: parseApiDate(dto.createdOn),
    updatedOn: parseApiDate(dto.updatedOn),
    createdBy: dto.createdBy,
    title: dto.title,
    description: dto.description ?? "",
    assignee: dto.assignedTeam?.label,
    issueType: dto.issueType?.label,
    statusId: dto.status?.id,
    severityId: dto.severity?.id,
    deployment: dto.deployment?.label,
    product: dto.deployedProduct ? `${dto.deployedProduct.label} ${dto.deployedProduct.version}` : undefined,
  };
}
