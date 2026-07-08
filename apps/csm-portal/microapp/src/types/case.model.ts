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
  AccountRefDto,
  AssignedEngineerRefDto,
  CaseCommentDto,
  CaseCommentType,
  CaseNumberRefDto,
  CaseSearchViewDto,
  CaseSeverity,
  CaseState,
  CaseType,
  CaseViewDto,
  CaseWorkState,
  DeployedProductRefDto,
  EntityRefDto,
  UserIdEmailRefDto,
  UserRefDto,
} from "./case.dto";
import { parseBackendTimestamp, parseOptionalBackendTimestamp } from "@utils/dateTime";

export interface CaseSummary {
  id: string;
  number: string;
  wso2Id: string;
  subject: string;
  description: string;
  severity: CaseSeverity;
  state: CaseState;
  workState: CaseWorkState;
  type: CaseType;
  createdOn: Date;
  updatedOn: Date;
  closedOn: Date | null;
  createdBy: UserIdEmailRefDto;
  project: EntityRefDto;
  deployment: EntityRefDto | null;
  product: EntityRefDto | null;
  assignedEngineer: AssignedEngineerRefDto | null;
  account: AccountRefDto | null;
  parentCase: CaseNumberRefDto | null;
  relatedCase: CaseNumberRefDto | null;
}

export interface CaseDetail {
  id: string;
  number: string;
  wso2Id: string;
  subject: string;
  description: string;
  severity: CaseSeverity;
  state: CaseState;
  workState: CaseWorkState;
  type: CaseType | null;
  engagementType: string | null;
  createdOn: Date;
  updatedOn: Date;
  closedOn: Date | null;
  createdBy: UserRefDto;
  project: EntityRefDto;
  deployment: EntityRefDto | null;
  deployedProduct: DeployedProductRefDto | null;
  product: EntityRefDto | null;
  assignedEngineer: AssignedEngineerRefDto | null;
  account: AccountRefDto | null;
  parentCase: CaseNumberRefDto | null;
  relatedCase: CaseNumberRefDto | null;
  nextStates: CaseState[];
}

export interface Comment {
  id: string;
  caseId: string;
  type: CaseCommentType;
  content: string;
  createdBy: string;
  createdOn: Date;
}

export function toCaseSummary(dto: CaseSearchViewDto): CaseSummary {
  return {
    id: dto.id,
    number: dto.number,
    wso2Id: dto.wso2Id,
    subject: dto.subject,
    description: dto.description,
    severity: dto.severity,
    state: dto.state,
    workState: dto.workState,
    type: dto.type as CaseType,
    // The search/detail views don't always populate updatedOn (e.g. a case that hasn't been
    // touched since creation); fall back to createdOn rather than showing an invalid time.
    createdOn: parseBackendTimestamp(dto.createdOn ?? ""),
    updatedOn: parseBackendTimestamp(dto.updatedOn ?? dto.createdOn ?? ""),
    closedOn: parseOptionalBackendTimestamp(dto.closedOn),
    createdBy: dto.createdBy,
    project: dto.project,
    deployment: dto.deployment,
    product: dto.product,
    assignedEngineer: dto.assignedEngineer,
    account: dto.account,
    parentCase: dto.parentCase,
    relatedCase: dto.relatedCase,
  };
}

export function toCaseDetail(dto: CaseViewDto): CaseDetail {
  return {
    id: dto.id,
    number: dto.number,
    wso2Id: dto.wso2Id,
    subject: dto.subject,
    description: dto.description,
    severity: dto.severity,
    state: dto.state,
    workState: dto.workState,
    type: dto.type as CaseType | null,
    engagementType: dto.engagementType,
    // The search/detail views don't always populate updatedOn (e.g. a case that hasn't been
    // touched since creation); fall back to createdOn rather than showing an invalid time.
    createdOn: parseBackendTimestamp(dto.createdOn ?? ""),
    updatedOn: parseBackendTimestamp(dto.updatedOn ?? dto.createdOn ?? ""),
    closedOn: parseOptionalBackendTimestamp(dto.closedOn),
    createdBy: dto.createdBy,
    project: dto.project,
    deployment: dto.deployment,
    deployedProduct: dto.deployedProduct,
    product: dto.product,
    assignedEngineer: dto.assignedEngineer,
    account: dto.account,
    parentCase: dto.parentCase,
    relatedCase: dto.relatedCase,
    nextStates: dto.nextStates,
  };
}

export function toComment(dto: CaseCommentDto): Comment {
  return {
    id: dto.id,
    caseId: dto.caseId,
    type: dto.type,
    content: dto.content,
    createdBy: dto.createdBy,
    createdOn: parseBackendTimestamp(dto.createdOn),
  };
}
