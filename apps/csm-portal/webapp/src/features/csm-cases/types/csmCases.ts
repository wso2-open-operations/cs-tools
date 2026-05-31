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
  CaseState,
  DashboardScope,
  Severity,
  SlaClockType,
} from "@features/csm-dashboard/types/abtDashboard";

export interface CsmCaseRow {
  id: string;
  caseNumber: string;
  subject: string;
  customer: string;
  accountId: string;
  projectId: string;
  projectName: string;
  severity: Severity;
  state: CaseState;
  owner: string;
  ownerIsMe: boolean;
  slaClockType: SlaClockType;
  // Minutes until breach (negative = already breached).
  minutesToBreach: number;
  createdAt: string;
  updatedAt: string;
}

export interface CsmCasesListResponse {
  scope: DashboardScope;
  cases: CsmCaseRow[];
}

export type CsmCommentAuthorRole = "customer" | "wso2_engineer" | "system";

export interface CsmCaseComment {
  id: string;
  caseId: string;
  authorName: string;
  authorRole: CsmCommentAuthorRole;
  bodyHtml: string;
  createdAt: string;
}
