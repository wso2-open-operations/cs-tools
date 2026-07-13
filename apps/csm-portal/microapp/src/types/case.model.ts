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
  UserRefDto,
} from "./case.dto";
import { parseBackendTimestamp, parseOptionalBackendTimestamp } from "@utils/dateTime";

// severity/issueType are only meaningful for the "case" type — the backend's create-payload docs
// say as much ("Required for case type") — so service_request/security_report_analysis/etc. cases
// come back with severity null. Beyond that, the backend's severity field is a best-effort value:
// depending on data source it can be the canonical word ("high"), a legacy ServiceNow priority
// code ("P2"), or a labeled priority ("High (P2)"). Mirrors
// apps/csm-portal/webapp/src/api/backend/mappers.ts severityFromPriority, which defaults to the
// mid severity on a garbled-but-present value rather than throwing.
function normalizeSeverity(raw: string | null | undefined): CaseSeverity | null {
  if (!raw) return null;
  const s = raw.toLowerCase();
  if (s.includes("p0") || s === "catastrophic") return "catastrophic";
  if (s.includes("p1") || s === "critical") return "critical";
  if (s.includes("p2") || s === "high") return "high";
  if (s.includes("p3") || s === "medium") return "medium";
  if (s.includes("p4") || s === "low") return "low";
  return "medium";
}

// The backend's state field is sometimes a labeled string ("Work In Progress") instead of the
// snake_case enum. Mirrors apps/csm-portal/webapp/src/api/backend/mappers.ts uiStateFromBe.
function normalizeState(raw: string): CaseState {
  if (!raw) return "open";
  return raw.trim().toLowerCase().replace(/\s+/g, "_") as CaseState;
}

export interface CaseSummary {
  id: string;
  number: string;
  wso2Id: string;
  subject: string;
  description: string;
  severity: CaseSeverity | null;
  state: CaseState;
  workState: CaseWorkState;
  type: CaseType;
  createdOn: Date;
  updatedOn: Date;
  closedOn: Date | null;
  /** Creator's email (the case-search view returns it as a plain string). */
  createdBy: string;
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
  severity: CaseSeverity | null;
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
    severity: normalizeSeverity(dto.severity),
    state: normalizeState(dto.state),
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
    severity: normalizeSeverity(dto.severity),
    state: normalizeState(dto.state),
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
