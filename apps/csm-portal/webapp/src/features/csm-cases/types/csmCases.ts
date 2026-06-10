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
  /**
   * ServiceNow-style case number (e.g. "CS-1007"). The number engineers and
   * customers quote day to day.
   */
  caseNumber: string;
  /**
   * Project/subscription-scoped WSO2 case reference (e.g. "ACMESUB-123").
   * Distinct from {@link caseNumber}; shown alongside it as `wso2CaseId/caseNumber`.
   * Mirrors the customer portal's `internalId` / BE `wso2Id`.
   */
  wso2CaseId: string;
  subject: string;
  customer: string;
  accountId: string;
  projectId: string;
  projectName: string;
  /** Affected WSO2 product (e.g. "WSO2 Identity Server"). Used for list filtering. */
  product: string;
  severity: Severity;
  state: CaseState;
  /** CRE / engineer working the case. "Unassigned" for cases with no one picked up yet. */
  assignee: string;
  assigneeIsMe: boolean;
  slaClockType: SlaClockType;
  // Minutes until breach (negative = already breached).
  minutesToBreach: number;
  /**
   * Whether SLA timing is actually known for this row. The backend has no SLA
   * data yet, so LIVE rows set this false and the list renders a neutral "—"
   * instead of a misleading orange "0m left". Absent/true → render the clock.
   */
  hasSla?: boolean;
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
  /**
   * Internal work note (not visible to the customer). `false` = public comment
   * that the customer sees in their portal. Mirrors SN's
   * `sn_customerservice_case.work_notes` vs `comments` distinction.
   */
  internal?: boolean;
}

export interface CaseAttachment {
  id: string;
  filename: string;
  /** File size in bytes. */
  size: number;
  contentType: string;
  uploadedBy: string;
  uploadedAt: string;
}

/** Plan/tier the customer holds for the affected product. */
export type CustomerTier = "subscription" | "managed_cloud" | "saas" | "trial";

export type SlaClockState = "running" | "paused" | "met" | "breached";

export interface CaseSlaClock {
  clockType: SlaClockType;
  state: SlaClockState;
  /** Minutes left until breach (negative = already breached). */
  minutesToBreach: number;
  /** Target SLA duration in minutes (e.g. ack target = 30 min for S1). */
  targetMinutes: number;
}

export interface CaseWatcher {
  id: string;
  name: string;
  role: "wso2_engineer" | "customer_contact" | "manager";
  isMe?: boolean;
}

export interface CaseLinkedItem {
  id: string;
  kind: "case" | "incident" | "escalation" | "kb" | "cr" | "sr";
  reference: string;
  title: string;
  state: string;
  /** Optional relative URL for in-app navigation. */
  href?: string;
}

export interface CaseTag {
  id: string;
  label: string;
  color?: "default" | "primary" | "warning" | "info" | "success" | "error";
}

export interface CaseTimeLogEntry {
  id: string;
  engineer: string;
  hours: number;
  note: string;
  date: string;
}

export type CaseAuditKind =
  | "state_change"
  | "assignee_change"
  | "severity_change"
  | "linked"
  | "escalated"
  | "watcher_added"
  | "comment_added"
  | "attachment_added"
  | "sla_breached"
  | "created";

export interface CaseAuditEntry {
  id: string;
  kind: CaseAuditKind;
  actor: string;
  description: string;
  createdAt: string;
}

export interface CaseCustomerContext {
  accountName: string;
  tier: CustomerTier;
  region: string;
  primaryContact: string;
  primaryContactEmail: string;
  accountManager: string;
  technicalOwner?: string;
  /** Number of currently open cases against this customer (incl. this one). */
  openCases: number;
}

/**
 * Deployment classification from the fixed deployment-type list. Mirrors the
 * backend `DeploymentTypeKey` enum (`@features/support/types/case`); kept local
 * so the case product context stays self-contained.
 */
export type DeploymentCategory =
  | "primary_production"
  | "staging"
  | "qa"
  | "stress"
  | "uat"
  | "development";

export interface CaseProductContext {
  product: string;
  version: string;
  updateLevel?: string;
  /** Customer-provided deployment name (set by the customer in the project). */
  deployment: string;
  /** Fixed-list classification of the deployment (e.g. Primary Production). */
  deploymentCategory?: DeploymentCategory;
  environment: "dev" | "qa" | "staging" | "prod";
  region?: string;
}

export type CaseLifecycleAction =
  | "start_work"
  | "assign_to_me"
  | "propose_solution"
  | "request_info"
  | "wait_on_wso2"
  | "resume_work"
  | "close"
  | "close_no_response"
  | "reopen";

/**
 * Full case detail used by the case detail page. Extends the lightweight
 * row type used in lists with all the side-widget data plus a curated set
 * of state-driven primary actions.
 */
export interface CsmCaseDetail extends CsmCaseRow {
  description: string;
  assignmentGroup: string;
  /** States this case may transition into next, per the backend. */
  nextStates?: CaseState[];
  /** Display name of the person who opened the case. */
  createdBy?: string;
  /** Email of the creator — used to tell a WSO2 engineer from a customer. */
  createdByEmail?: string;
  customerContext: CaseCustomerContext;
  productContext: CaseProductContext;
  slaClocks: CaseSlaClock[];
  watchers: CaseWatcher[];
  linkedItems: CaseLinkedItem[];
  tags: CaseTag[];
  timeLogs: CaseTimeLogEntry[];
  /** Lifecycle/audit entries — distinct from threaded comments. */
  audit: CaseAuditEntry[];
  attachments: CaseAttachment[];
  /** Whether the current user is watching this case (controls Watch toggle). */
  isWatching: boolean;
}
