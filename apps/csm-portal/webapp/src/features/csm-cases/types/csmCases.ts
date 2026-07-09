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
  CaseWorkState,
  Severity,
  SlaClockType,
} from "@features/csm-dashboard/types/abtDashboard";
import type { BeCaseIssueType, BeCaseType } from "@api/backend/types";

export interface CsmCaseRow {
  /**
   * UUID primary key. Identifies the case in API paths and links only — never
   * shown to humans. Use {@link caseNumber} / {@link wso2CaseId} for display.
   */
  id: string;
  /**
   * ServiceNow-style case number (e.g. "CS-1007"). The number engineers and
   * customers quote day to day. Optional: absent until the BE assigns one, and
   * never substituted with the UUID {@link id}.
   */
  caseNumber?: string;
  /**
   * Project/subscription-scoped WSO2 case reference (e.g. "ACMESUB-123").
   * Distinct from {@link caseNumber}; shown alongside it as `wso2CaseId/caseNumber`.
   * Mirrors the customer portal's `internalId` / BE `wso2Id`. Optional: a case
   * may have no WSO2 reference, and it is never substituted with the UUID
   * {@link id}.
   */
  wso2CaseId?: string;
  subject: string;
  customer: string;
  accountId: string;
  projectId: string;
  projectName: string;
  /** Affected WSO2 product (e.g. "WSO2 Identity Server"). Used for list filtering. */
  product: string;
  severity: Severity;
  state: CaseState;
  /**
   * Case type (BE `typeKey` / search `caseType`). Optional: a legacy row may
   * omit it, in which case the type filter treats it as unmatched.
   */
  caseType?: BeCaseType;
  /**
   * Work sub-state of an in-progress case (`ongoing` / `paused`); `null` when
   * the case is not `work_in_progress`. Drives the comment gate and the paused
   * indicator. See {@link commentGateReason}.
   */
  workState?: CaseWorkState | null;
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
  cases: CsmCaseRow[];
  /** Total rows matching the query across all pages (BE `total`). */
  total: number;
  /** Page size used for this response (BE `limit`). */
  limit: number;
  /** Zero-based row offset of this page (BE `offset`). */
  offset: number;
  /** Whether more rows exist beyond this page (BE `hasMore`). */
  hasMore: boolean;
}

export type CsmCommentAuthorRole =
  | "customer"
  | "wso2_engineer"
  | "system"
  | "chatbot";

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

/**
 * Account tier as shown on a case. Free-form: the CaseView's `account.type` is
 * the PG `basic|enterprise` enum for native cases but a raw ServiceNow support
 * tier (e.g. "Enterprise") for SN-sourced ones, so render it defensively.
 */
export type CustomerTier = string;

/**
 * Stage of a case SLA record, as returned by the case SLA list endpoint. An
 * open enum: the known values below still drive autocomplete, but a stage
 * outside this set (e.g. one added later on the backend) is kept as-is
 * rather than forced into the closed set.
 */
export type SlaStage =
  | "in_progress"
  | "paused"
  | "completed"
  | "cancelled"
  | "breached"
  | (string & {});

/**
 * A single SLA record attached to a case. All time fields
 * are pre-formatted server-side (`*Label`) — the frontend renders them as-is
 * rather than recomputing.
 */
export interface CaseSla {
  id: string;
  /** SLA definition name (e.g. "S1 - Response"). */
  definition: string;
  /** Target duration as a display string (e.g. "4 Business Hours"); absent for open-ended SLAs. */
  target: string | null;
  stage: SlaStage;
  /** Human-readable stage label from the backend (e.g. "In progress"). */
  stageLabel: string;
  hasBreached: boolean;
  businessTimeLeftLabel: string;
  businessElapsedLabel: string;
  /** Percentage (0-100+) of the target consumed in business time. */
  businessElapsedPercent: number;
  /** ISO-8601 UTC; null when the SLA clock hasn't started. */
  startTime: string | null;
  /** ISO-8601 UTC; null while the SLA is still running. */
  stopTime: string | null;
}

export interface CaseSlaList {
  caseId: string;
  count: number;
  slas: CaseSla[];
}

/**
 * Raw SLA record shape returned by the task-SLA search endpoint. One record
 * per SLA definition attached to a task (a case is a task). Kept close to the
 * wire shape; {@link CaseSla} is the row model the SLA table actually renders,
 * built from this by {@link useGetCsmCaseSlas}.
 */
export interface TaskSlaView {
  id: string;
  slaDefinition: {
    id?: string | null;
    name?: string | null;
    type?: string | null;
    target?: string | null;
  } | null;
  stage: string | null;
  task: {
    id?: string | null;
    number?: string | null;
  } | null;
  businessTimeLeft: string | null;
  businessElapsedTime: string | null;
  businessElapsedPercentage: number | null;
  startTime: string | null;
  endTime: string | null;
}

export interface TaskSlaSearchPayload {
  filters?: {
    taskIds?: string[];
  };
  pagination?: {
    limit?: number;
    offset?: number;
  };
}

export interface TaskSlaSearchResponse {
  slas: TaskSlaView[];
  total: number;
  limit: number;
  offset: number;
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
  | "created"
  | "field_change";

/** One field changed within a single audited save-transaction. */
export interface CaseAuditFieldChange {
  field: string;
  fieldLabel: string;
  /** Absent/empty when the field was previously unset (a "set" change). */
  previousValue?: string;
  /** Absent/empty when the field was cleared. */
  newValue?: string;
}

export interface CaseAuditEntry {
  id: string;
  kind: CaseAuditKind;
  actor: string;
  /** Free-text summary; used when `changes` is absent (older/synthetic entries). */
  description?: string;
  createdAt: string;
  /** Populated for `kind === "field_change"`: one save-transaction may touch
   * several fields at once (e.g. state + assignee in the same update). */
  changes?: CaseAuditFieldChange[];
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
  /** Deployment UUID, when the case is linked to one — drives the detail link. */
  deploymentId?: string;
  /** Deployed-product UUID, when the case is linked to one. */
  deployedProductId?: string;
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
  // Closed-case replacement for reopening: the backend surfaces this via a
  // `reopened` entry in `nextStates` (a real reopen is never valid — see
  // that field's doc), but it must NOT be PATCHed like a real transition —
  // it opens the new-case form pre-filled with relatedCaseId instead.
  | "create_related_case"
  // Generic transition into a state the frontend has no curated action for
  // (e.g. a state added on the backend). Drives the post-transition toast only;
  // the PATCH target always comes from the backend `nextStates` value.
  | "transition";

/**
 * Router (`navigate(..., { state })`) payload carried from a closed case's
 * "Create related case" action to `/cases/new`, so the new-case form can
 * prefill from the case it's related to without a query-string round trip
 * or a full page load. All fields but the ids are just starting values —
 * the form leaves every one of them editable. See CsmCaseDetailPage.tsx's
 * `create_related_case` handler and CsmCaseCreatePage.tsx's read of
 * `useLocation().state`.
 */
export interface CreateRelatedCaseNavState {
  projectId: string;
  relatedCaseId: string;
  relatedCaseNumber?: string;
  deploymentId?: string;
  deployedProductId?: string;
  severity?: Severity;
  issueType?: BeCaseIssueType;
  subject?: string;
}

/**
 * Full case detail used by the case detail page. Extends the lightweight
 * row type used in lists with all the side-widget data plus a curated set
 * of state-driven primary actions.
 */
export interface CsmCaseDetail extends CsmCaseRow {
  description: string;
  assignmentGroup: string;
  /** Category of issue reported, when set (e.g. "total_outage", "question"). */
  issueType?: BeCaseIssueType;
  /**
   * Id of the chat conversation this case was spawned from, when any. Drives
   * loading the Novera chat transcript as the earliest entries in the activity
   * feed (mirrors the customer portal). Absent when the case has no linked
   * conversation (e.g. non-ServiceNow source, or a case opened without chat).
   */
  conversationId?: string;
  /**
   * States this case may transition into next, per the backend. For a closed
   * case, a `reopened` entry is not a real reopen (the data source has none)
   * — it signals "Create related case" is available within its 60-day window.
   */
  nextStates?: CaseState[];
  /** The case this one was created as related to, when any. */
  relatedCase?: { id: string; caseNumber?: string };
  /** Display name of the person who opened the case. */
  createdBy?: string;
  /** Email of the creator — used to tell a WSO2 engineer from a customer. */
  createdByEmail?: string;
  customerContext: CaseCustomerContext;
  productContext: CaseProductContext;
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
