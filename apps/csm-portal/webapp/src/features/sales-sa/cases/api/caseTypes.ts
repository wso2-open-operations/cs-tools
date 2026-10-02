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

// Ported from apps/support-portal-lite/webapp's own features/spl/cases/api/caseTypes.ts
// (itself ported from the original SupportPortalLite app's data/utils/types.ts) —
// case-domain slice only. Field names match the Go backend's
// internal/servicenow.CaseDetails/CommentsResponse/AttachmentInfo JSON tags exactly
// (cs-tools/apps/csm-portal/backend, branch feat/support-portal-lite-backend).
export interface DataStruct {
  [key: string]: unknown;
}

export interface CaseDetails extends DataStruct {
  /** entity-service's internal UUID -- the id every navigation link now uses. */
  id: string;
  caseId: string;
  caseType: string;
  number: string;
  openedAt: string;
  openedBy: string;
  priority: string;
  shortDescription: string;
  state: string;
  description: string;
  assignedTo: string;
  accountNumber: string;
  accountName: string;
  /** entity-service's internal UUID for the linked account -- used for navigation, not display. */
  accountId?: string;
  projectNumber: string;
  projectKey: string;
  /** entity-service's internal UUID for the linked project -- used for navigation, not display. */
  projectId?: string;
  productName: string;
  lastWSO2CommentTime: string;
  lastCustomerCommentTime: string;
  projectDeploymentName: string;
  projectDeploymentType: string;
}

export interface CaseDetailsWithCount extends DataStruct {
  count: number;
  cases: CaseDetails[];
}

export interface CaseCommentDetails extends DataStruct {
  /** entity-service's own id for this comment -- stable identity for dedup, since createdOn/createdBy/type alone can collide across different comments. */
  id: string;
  createdOn: string;
  caseType: string;
  type: "comments" | "work_notes";
  value: string;
  createdBy: string;
}

// The Go backend reuses the same shape for attachments-info too, but it's a
// different set of fields — given its own type here instead.
export interface AttachmentDetails extends DataStruct {
  sysId: string;
  fileName: string;
  createdOn: string;
  createdBy: string;
  updatedOn: string;
  updatedBy: string;
  contentType: string;
  state: string;
}

export interface ProjectSummary extends DataStruct {
  /** entity-service's internal UUID -- used for navigation, not display. */
  id: string;
  number: string;
  name: string;
}

export interface AccountSummary extends DataStruct {
  /** entity-service's internal UUID -- used for navigation, not display. */
  id: string;
  number: string;
  name: string;
}

export type SearchResult = CaseDetailsWithCount | ProjectSummary[] | AccountSummary[];

export const CASE_CLOSED_STATE = "Closed";
