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

/**
 * Type taxonomy mirrors SN's `u_customer_engagement_type` catalogue plus
 * the `migration` type that CRE engineers also categorise separately:
 * Customer Onboarding, Migration, QSP, Enterprise CSM / TAM, Consultancy,
 * Training, Architecture Review, Firefighting.
 */
export type CsmEngagementType =
  | "customer_onboarding"
  | "migration"
  | "qsp"
  | "enterprise_csm_tam"
  | "consultancy"
  | "training"
  | "architecture_review"
  | "firefighting";

/**
 * State machine matches SN's `u_customer_engagement.u_state` choices.
 *
 *   new → requested → in_progress → (on_hold ⇄ in_progress) → completed
 *                                                          → cancelled
 */
export type CsmEngagementState =
  | "new"
  | "requested"
  | "in_progress"
  | "on_hold"
  | "completed"
  | "cancelled";

/** SN's `u_customer_engagement_stage_type` lifecycle. */
export type CsmEngagementStage =
  | "pre_engagement"
  | "planning"
  | "execution"
  | "warranty"
  | "post_engagement"
  | "closure";

export type CsmEngagementDeliveryMode = "onsite" | "remote" | "hybrid";

/**
 * Whether the engagement is paid for separately ("paid") or delivered free
 * of charge ("foc"). FoC is common for onboarding bundled with a subscription
 * or short-form architecture reviews offered as part of CSM/TAM coverage.
 */
export type CsmEngagementPaymentType = "paid" | "foc";

/** Overall health flagged on a status update. */
export type CsmEngagementHealth = "green" | "amber" | "red";

export type CsmEngagementTaskState =
  | "not_started"
  | "in_progress"
  | "blocked"
  | "completed"
  | "cancelled";

export type CsmEngagementDeliverableStatus =
  | "pending"
  | "in_review"
  | "accepted"
  | "waived";

/** Lightweight row used in list views — keep this in sync with the table columns. */
export interface CsmEngagementRow {
  id: string;
  /** Human-readable reference number, e.g. ENG-1042. */
  reference: string;
  /** Engagement name (`u_name` in SN). */
  name: string;
  type: CsmEngagementType;
  state: CsmEngagementState;
  stage: CsmEngagementStage;
  /** Customer account name. */
  customer: string;
  accountId: string;
  /** Linked project. Every engagement belongs to a project (same as cases). */
  projectId: string;
  projectName: string;
  ownerId: string;
  ownerName: string;
  ownerIsMe: boolean;
  deliveryMode: CsmEngagementDeliveryMode;
  plannedStartDate: string;
  plannedEndDate: string;
  /** Health flag from the most recent status update, if any. */
  health?: CsmEngagementHealth;
  paymentType: CsmEngagementPaymentType;
  /** % complete (0–100), derived from stage / tasks / deliverables. */
  progressPct: number;
  createdAt: string;
  updatedAt: string;
}

export interface CsmEngagementTask {
  id: string;
  engagementId: string;
  title: string;
  description?: string;
  stage: CsmEngagementStage;
  state: CsmEngagementTaskState;
  assigneeId?: string;
  assigneeName?: string;
  dueDate?: string;
  completedAt?: string;
  /** Optional dependency on another task within the same engagement. */
  blockedByTaskId?: string;
}

export interface CsmEngagementDeliverable {
  id: string;
  engagementId: string;
  name: string;
  description?: string;
  status: CsmEngagementDeliverableStatus;
  dueDate?: string;
  completedAt?: string;
  /** Optional URL / Drive link to the artifact. */
  artifactUrl?: string;
  /** Free-text note recorded when the deliverable was waived. */
  waiverReason?: string;
}

export interface CsmEngagementStatusUpdate {
  id: string;
  engagementId: string;
  authorId: string;
  authorName: string;
  /** Health flag at the time of update. */
  health: CsmEngagementHealth;
  /** Short headline visible on the engagement list view. */
  headline: string;
  /** Long-form body (sanitised HTML or plain text). */
  bodyHtml: string;
  createdAt: string;
}

export interface CsmEngagementWatcher {
  id: string;
  name: string;
  role: "wso2_engineer" | "manager" | "customer_contact" | "services_lead";
  isMe?: boolean;
}

export interface CsmEngagementAllocation {
  id: string;
  engagementId: string;
  userId: string;
  userName: string;
  /** Allocation percentage of weekly capacity. */
  allocationPct: number;
  startDate: string;
  endDate: string;
}

export type CsmEngagementAuditKind =
  | "created"
  | "state_change"
  | "stage_change"
  | "owner_change"
  | "task_added"
  | "task_completed"
  | "deliverable_added"
  | "deliverable_completed"
  | "deliverable_waived"
  | "status_update"
  | "watcher_added"
  | "blocker_logged"
  | "comment_added"
  | "attachment_added"
  | "case_linked";

export interface CsmEngagementAuditEntry {
  id: string;
  kind: CsmEngagementAuditKind;
  actor: string;
  description: string;
  createdAt: string;
}

export type CsmEngagementCommentAuthorRole =
  | "wso2_engineer"
  | "customer_contact"
  | "services_lead"
  | "system";

export interface CsmEngagementComment {
  id: string;
  engagementId: string;
  authorName: string;
  authorRole: CsmEngagementCommentAuthorRole;
  bodyHtml: string;
  createdAt: string;
  /** Internal note (not customer-visible). */
  internal?: boolean;
}

export interface CsmEngagementBilling {
  paymentType: CsmEngagementPaymentType;
  /** Salesforce opportunity reference, optional — only set when the engagement
   *  originates from a Salesforce opportunity. */
  opportunityRef?: string;
}

/**
 * Lifecycle actions exposed on the action bar. Available actions are
 * state-driven, see `PRIMARY_BY_STATE` in EngagementActionBar.tsx.
 */
export type CsmEngagementLifecycleAction =
  | "approve_request"
  | "start_work"
  | "put_on_hold"
  | "resume_work"
  | "complete_engagement"
  | "cancel_engagement"
  | "reopen";

export interface CsmEngagementAttachment {
  id: string;
  filename: string;
  /** File size in bytes. */
  size: number;
  contentType: string;
  uploadedBy: string;
  uploadedAt: string;
}

/** A case linked back to an engagement (for engagement-driven work). */
export interface CsmEngagementLinkedCase {
  id: string;
  caseNumber: string;
  subject: string;
  state: string;
  severity: string;
  href: string;
}

/** Full engagement detail returned by the detail endpoint. */
export interface CsmEngagementDetail extends CsmEngagementRow {
  description: string;
  /** Free-text scope statement. */
  scope: string;
  /** Salesforce / SN identifiers and billing posture. */
  billing: CsmEngagementBilling;
  watchers: CsmEngagementWatcher[];
  allocations: CsmEngagementAllocation[];
  stages: CsmEngagementStage[];
  /** Stage progress map; absent stages are treated as not started. */
  stageStatus: Partial<Record<CsmEngagementStage, "not_started" | "in_progress" | "completed">>;
  tasks: CsmEngagementTask[];
  deliverables: CsmEngagementDeliverable[];
  statusUpdates: CsmEngagementStatusUpdate[];
  audit: CsmEngagementAuditEntry[];
  attachments: CsmEngagementAttachment[];
  linkedCases: CsmEngagementLinkedCase[];
  /** Whether the current user is watching this engagement. */
  isWatching: boolean;
}

export interface CsmEngagementsListResponse {
  engagements: CsmEngagementRow[];
}

export interface CsmEngagementListFilters {
  search: string;
  types: CsmEngagementType[];
  states: CsmEngagementState[];
  stages: CsmEngagementStage[];
  owner: "all" | "me" | "unassigned";
  health: "all" | "green" | "amber" | "red";
}

/** Shape accepted by the create-engagement endpoint. */
export interface CreateCsmEngagementInput {
  name: string;
  type: CsmEngagementType;
  /** Project is required (same as cases). The accountId/customer are
   *  derived from the project. */
  projectId: string;
  projectName: string;
  accountId: string;
  customer: string;
  ownerId?: string;
  ownerName?: string;
  deliveryMode: CsmEngagementDeliveryMode;
  plannedStartDate: string;
  plannedEndDate: string;
  scope: string;
  description: string;
  billing: CsmEngagementBilling;
}
