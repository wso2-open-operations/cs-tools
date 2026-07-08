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

export type CaseType = "case" | "service_request" | "security_report_analysis" | "engagement" | "announcement";

export type CaseSeverity = "catastrophic" | "critical" | "high" | "medium" | "low";

export type CaseState =
  | "open"
  | "work_in_progress"
  | "waiting_on_wso2"
  | "awaiting_info"
  | "reopened"
  | "solution_proposed"
  | "closed";

export type CaseWorkState = "ongoing" | "paused" | null;

export type CaseIssueType =
  | "error"
  | "partial_outage"
  | "performance_degradation"
  | "question"
  | "security_or_compliance"
  | "total_outage";

export interface EntityRefDto {
  id: string;
  name: string;
}

export interface UserRefDto {
  id: string;
  displayName: string;
  userId: string;
  email: string;
}

export interface UserIdEmailRefDto {
  id: string;
  email: string;
}

export interface AssignedEngineerRefDto {
  id: string;
  name: string;
  email: string | null;
}

export interface CaseNumberRefDto {
  id: string;
  number: string;
}

export interface AccountRefDto {
  id: string;
  name: string;
  type: string;
}

export interface DeployedProductRefDto {
  id: string;
  displayName: string;
}

export interface CaseSearchFiltersDto {
  types?: CaseType[];
  searchQuery?: string;
  projectIds?: string[];
  deploymentIds?: string[];
  states?: CaseState[];
  severities?: CaseSeverity[];
  issueTypes?: CaseIssueType[];
  assignedUserIds?: string[];
  createdByMe?: boolean;
}

export interface CaseSearchPayloadDto {
  filters?: CaseSearchFiltersDto;
  sortBy?: {
    field?: "createdOn" | "updatedOn" | "severity" | "state";
    order?: "asc" | "desc";
  };
  pagination?: {
    offset?: number;
    limit?: number;
  };
}

export interface CaseSearchViewDto {
  id: string;
  number: string;
  wso2Id: string;
  subject: string;
  description: string;
  severity: CaseSeverity;
  issueType: CaseIssueType;
  state: CaseState;
  workState: CaseWorkState;
  type: string;
  createdOn?: string;
  updatedOn?: string;
  closedOn: string | null;
  createdBy: UserIdEmailRefDto;
  project: EntityRefDto;
  deployment: EntityRefDto | null;
  deployedProduct: EntityRefDto | null;
  product: EntityRefDto | null;
  assignedEngineer: AssignedEngineerRefDto | null;
  parentCase: CaseNumberRefDto | null;
  relatedCase: CaseNumberRefDto | null;
  account: AccountRefDto | null;
}

export interface CaseSearchResponseDto {
  cases: CaseSearchViewDto[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

export interface CaseViewDto {
  id: string;
  number: string;
  wso2Id: string;
  subject: string;
  description: string;
  severity: CaseSeverity;
  issueType: CaseIssueType;
  state: CaseState;
  workState: CaseWorkState;
  type: string | null;
  engagementType: string | null;
  createdOn?: string;
  updatedOn?: string;
  closedOn: string | null;
  createdBy: UserRefDto;
  project: EntityRefDto;
  deployment: EntityRefDto | null;
  deployedProduct: DeployedProductRefDto | null;
  product: EntityRefDto | null;
  catalog: EntityRefDto | null;
  catalogItem: EntityRefDto | null;
  assignedTeam: EntityRefDto | null;
  conversation: EntityRefDto | null;
  assignedEngineer: AssignedEngineerRefDto | null;
  parentCase: CaseNumberRefDto | null;
  relatedCase: CaseNumberRefDto | null;
  account: AccountRefDto | null;
  nextStates: CaseState[];
}

export type CaseCommentType = "work_note" | "comment" | "activity";

export interface CaseCommentDto {
  id: string;
  caseId: string;
  type: CaseCommentType;
  content: string;
  createdBy: string;
  createdOn: string;
}

export interface CaseCommentSearchResponseDto {
  comments: CaseCommentDto[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}
