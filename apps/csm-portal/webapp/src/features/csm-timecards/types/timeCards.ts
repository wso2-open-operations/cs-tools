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
 * Approval lifecycle of a time card (mirrors ServiceNow Time Card states):
 * pending (draft) → submitted → approved/rejected; an approved card can be
 * recalled for corrections, or marked processed (expensed).
 */
export type TimeCardState =
  | "pending"
  | "submitted"
  | "approved"
  | "rejected"
  | "recalled"
  | "processed";

/**
 * The fixed activity buckets a time card splits its hours across — the activity
 * categories from the Time Management requirement (ISSU-009). Order here is the
 * canonical display order; the human labels live in
 * `timeCardConstants.ACTIVITY_BUCKETS`.
 */
export const ACTIVITY_KEYS = [
  "analysisDebugging",
  "reproduce",
  "settingUp",
  "providingSolution",
  "answering",
] as const;

export type ActivityKey = (typeof ACTIVITY_KEYS)[number];

/** Hours logged against each activity bucket. */
export type ActivityBreakdown = Record<ActivityKey, number>;

/** A team lead who must approve a time card (the ServiceNow "Approver list"). */
export interface TimeCardApprover {
  id: string;
  name: string;
}

/**
 * The kind of work a time card is logged against. ServiceNow time cards attach
 * to the whole Task table; we surface the common ones.
 */
export type TaskType = "case" | "project" | "change_request" | "incident";

/** Work category options (the ServiceNow "Category" field). */
export type TimeCardCategory =
  | "Task work"
  | "Investigation"
  | "Customer call"
  | "Documentation";

/** Issue-complexity options (the ServiceNow "Issue Complexity" field). */
export type IssueComplexity = "N/A" | "Low" | "Medium" | "High";

/** A lifecycle event on a time card (the ServiceNow "Activities" audit). */
export type TimeCardActivityAction =
  | "created"
  | "edited"
  | "submitted"
  | "approved"
  | "rejected"
  | "recalled"
  | "processed";

export interface TimeCardActivity {
  at: string; // ISO timestamp
  by: string; // display name
  action: TimeCardActivityAction;
  note?: string;
}

/** A single time card logged by an engineer against a case. */
export interface CsmTimeCard {
  id: string;
  /** What kind of task the time was logged against. */
  taskType: TaskType;
  /** Task the time was spent on (the ServiceNow "Task"); a case id for cases. */
  caseId: string;
  /** Task reference shown to humans (e.g. CS0352584, the project/CR/incident id). */
  caseNumber: string;
  /** Project the parent case belongs to. A company customer may have several
   *  projects; this drives the project filter. Sourced from the case detail. */
  projectId: string;
  projectName: string;
  /** Day the work happened (YYYY-MM-DD). */
  date: string;
  /** Engineer who logged the time. */
  userId: string;
  userName: string;
  state: TimeCardState;
  category: TimeCardCategory;
  breakdown: ActivityBreakdown;
  /** Whether the logged time is billable to the customer (ISSU-009). */
  billable: boolean;
  /** Derived sum of `breakdown`, stored for convenience. */
  totalHours: number;
  workLogComment: string;
  /** Reviewer's note, set when a lead accepts/rejects. */
  leadComment?: string;
  issueComplexity: IssueComplexity;
  approvers: TimeCardApprover[];
  /** When the engineer submitted (ISO). */
  submittedAt: string;
  /** When a lead decided (ISO); unset while pending. */
  decidedAt?: string;
  /** Display name of the deciding lead; unset while pending. */
  decidedBy?: string;
  /** Audit trail of lifecycle events, oldest first. */
  activity: TimeCardActivity[];
}

/** Payload to create a new time card from the log dialog. */
export interface CreateTimeCardInput {
  taskType: TaskType;
  caseId: string;
  caseNumber: string;
  /** Project of the parent case (sourced from case detail at log time). */
  projectId: string;
  projectName: string;
  date: string;
  category: TimeCardCategory;
  breakdown: ActivityBreakdown;
  /** Billable classification (ISSU-009). */
  billable: boolean;
  workLogComment: string;
  issueComplexity: IssueComplexity;
  approver: TimeCardApprover;
}

/** Payload for a lead's accept/reject decision. */
export interface TimeCardDecisionInput {
  cardId: string;
  state: Extract<TimeCardState, "approved" | "rejected">;
  leadComment?: string;
}

/** Filters for the workload / list views. */
export interface TimeCardFilters {
  state?: TimeCardState | "all";
  /** Restrict to a single engineer (team view). */
  engineerId?: string;
  /** Time window the cards fall in. */
  period?: "week" | "all";
}

/** Rolled-up status of a weekly time sheet, derived from its cards. */
export type TimeSheetState =
  | "open" // editable cards, nothing submitted yet
  | "submitted" // awaiting approver decisions
  | "approved" // all cards approved/processed
  | "rejected" // contains rejected card(s) needing rework
  | "recalled"; // contains recalled card(s)

/** A user's time cards for one ISO week (Mon–Sun), the unit of submission. */
export interface CsmTimeSheet {
  /** `${userId}:${weekStart}`. */
  id: string;
  userId: string;
  userName: string;
  /** Monday of the week (YYYY-MM-DD). */
  weekStart: string;
  /** Sunday of the week (YYYY-MM-DD). */
  weekEnd: string;
  state: TimeSheetState;
  cards: CsmTimeCard[];
  totalHours: number;
}

/**
 * A permanent record of a deleted time card (ISSU-009 audit). Deleting a card
 * removes it from active sheets but leaves this tombstone so the actor, hours
 * removed, and timestamp survive.
 */
export interface TimeCardDeletion {
  cardId: string;
  caseNumber: string;
  userName: string;
  totalHours: number;
  /** Who performed the delete (display name). */
  deletedBy: string;
  /** When the delete happened (ISO). */
  at: string;
}

/** An approver handing their approvals to another approver for a period. */
export interface ApproverDelegation {
  approverId: string;
  delegateId: string;
  delegateName: string;
  /** Inclusive date range the delegation is active (YYYY-MM-DD). */
  from: string;
  to: string;
}

/**
 * Filters for the time-card search. Sent in the POST body (never as query
 * params) so case / work-item ids and engineer ids stay out of URLs and logs.
 * All fields are optional; an empty object returns everything in scope.
 */
export interface TimeCardSearchFilters {
  /** Projects to include (company customer may own several). */
  projectIds?: string[];
  /** Work item / case reference (e.g. CS0352584); substring match. */
  workItemId?: string;
  /** Engineer id to scope to a single user. */
  engineerId?: string;
  /** Lifecycle states to include. */
  states?: TimeCardState[];
  /** Inclusive date range (YYYY-MM-DD). */
  from?: string;
  to?: string;
}

/** Aggregated figures for the reports / exceptions view. */
export interface TimecardReports {
  totalHours: number;
  /** Hours flagged billable to the customer. */
  billableHours: number;
  /** Hours flagged non-billable. */
  nonBillableHours: number;
  /** Cards awaiting a decision (submitted). */
  pendingApproval: number;
  approved: number;
  rejected: number;
  /** Mean days between submission and decision, over decided cards. */
  avgApprovalLagDays: number;
  /** Share of cards that have moved past draft (submitted+), 0–1. */
  submissionRate: number;
  byCategory: { category: string; hours: number }[];
  byUser: {
    userId: string;
    userName: string;
    hours: number;
    pending: number;
    approved: number;
  }[];
  exceptions: {
    id: string;
    userName: string;
    caseNumber: string;
    kind: "overdue" | "rejected";
    detail: string;
  }[];
  /** Audit of recently deleted cards (newest first). */
  deletions: TimeCardDeletion[];
}
