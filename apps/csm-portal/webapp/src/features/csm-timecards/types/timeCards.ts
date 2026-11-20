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
 * Approval lifecycle of a time card, per the entity-service contract. A card
 * is created already `submitted` — there is no draft/pending step via the
 * portal API. A lead then decides `approved` or `rejected` via `PATCH
 * /time-cards/{id}`. `recalled` and `processed` appear in the backend's state
 * enum but the portal exposes no endpoint to set them.
 */
export type TimeCardState =
  | "pending"
  | "submitted"
  | "approved"
  | "rejected"
  | "recalled"
  | "processed";

/**
 * The fixed activity buckets a time card splits its time across — the
 * activity categories from the Time Management requirement (ISSU-009).
 * Shapes the log/edit-time form; also the shape `CsmTimeCard.breakdown` reads
 * an existing card's minutes into (see `mapTimeCard` in `useTimeSheets.ts`),
 * now that the backend is confirmed to echo these back on read.
 */
export const ACTIVITY_KEYS = [
  "analysisDebugging",
  "reproduce",
  "settingUp",
  "providingSolution",
  "answering",
] as const;

export type ActivityKey = (typeof ACTIVITY_KEYS)[number];

/** Whole minutes logged against each activity bucket (log-time form state
 * only) — matches the backend's own unit for these fields directly. */
export type ActivityBreakdown = Record<ActivityKey, number>;

/** A team lead eligible to approve a time card (ServiceNow "approver_list"). */
export interface TimeCardApprover {
  id: string;
  name: string;
}

/** Issue-complexity options (the ServiceNow "Issue Complexity" field). */
export type IssueComplexity = "N/A" | "Low" | "Medium" | "High";

/**
 * A time card as returned by `POST /time-cards/search` and the mutation
 * endpoints (the backend's `TimeCardView`). `breakdown` and
 * `issueComplexity` are confirmed live to round-trip: a card's per-activity
 * minutes and issue complexity come back unchanged on the next search after
 * being set, which is what makes editing an existing card (see
 * `LogTimeCardDialog`'s edit mode) safe — prefilling the edit form from these
 * fields shows the card's real current values, not stale zeros. A lead
 * comment other than {@link rejectionReason} is still write-only/unconfirmed.
 */
export interface CsmTimeCard {
  id: string;
  /** Case the time was spent on. */
  caseId: string;
  /** Case reference shown to humans (e.g. CS0352584). */
  caseNumber: string;
  projectId: string;
  projectName: string;
  /**
   * The date the work was actually carried out (ISO, YYYY-MM-DD) — what the
   * engineer picked in the log form, so it can be backdated. This is the field
   * to display and to sort by; it replaces the deprecated `createdOn`, which
   * currently holds the same value. Occasionally unparseable on real records
   * (confirmed live); see `groupTimeCards` in `timeCardGrouping.ts`, which
   * sorts by it but tolerates a bad value rather than dropping the card.
   */
  workDate: string;
  userId: string;
  userName: string;
  state: TimeCardState;
  /** Whether the logged time is billable to the customer (ISSU-009). */
  billable: boolean;
  /** Whole minutes — the backend's own unit for this field (see
   * `mapTimeCard` in `useTimeSheets.ts`). */
  totalMinutes: number;
  /**
   * The approver who accepted the card — set **only** when `state` is
   * `approved`. ServiceNow doesn't record who rejected a card, so there is
   * deliberately no `rejectedBy`: a rejection surfaces {@link rejectionReason}
   * instead (see `decisionSummary`).
   */
  approvedById?: string;
  approvedByName?: string;
  /**
   * The approver's comment when rejecting — set **only** when `state` is
   * `rejected`. It's the only trace a rejection leaves (no rejecter identity or
   * timestamp exists upstream).
   */
  rejectionReason?: string;
  /**
   * The engineers eligible to decide this card (SN `approver_list`). Used to
   * gate the "Review" action: the backend 403s a decision from anyone not
   * in this list, regardless of team-lead status — the approval queue tab
   * already scopes server-side (`approverId` filter), but a case's own
   * "Time tracking" panel shows every submitted card on the case, so it
   * needs this to know which ones the signed-in lead can actually decide.
   */
  approvers?: TimeCardApprover[];
  /** The engineer's own comment entered when submitting the card, as
   * ServiceNow rich-text HTML — sanitize with `sanitizeRichTextHtml` before
   * rendering. Absent on a card logged before this field was mapped. */
  workLogComment?: string;
  /** Per-activity minute breakdown — absent on a card logged before this
   * field was mapped, otherwise always present alongside `totalMinutes`. */
  breakdown?: ActivityBreakdown;
  /** The "Issue Complexity" picked when the card was logged (or last
   * edited). Absent on a card logged before this field was mapped. */
  issueComplexity?: IssueComplexity;
}

/**
 * Payload to create a new time card from the log dialog. The card is created
 * already `submitted` — there is no separate submit step, and no support for
 * logging against anything other than a case (the backend's `caseId` is a
 * required, case-only reference).
 */
export interface CreateTimeCardInput {
  caseId: string;
  caseNumber: string;
  projectId: string;
  projectName: string;
  date: string;
  breakdown: ActivityBreakdown;
  /** Billable classification (ISSU-009). */
  billable: boolean;
  workLogComment: string;
  issueComplexity: IssueComplexity;
  approver: TimeCardApprover;
}

/**
 * Payload to edit an already-submitted card's own content — the backend
 * enforces submitter-only + `submitted`-state-only server-side (matching
 * ServiceNow's own edit-in-place behavior), so this is only ever offered to
 * the card's own submitter while it's still `submitted` (see `cardActions`).
 * No `approver` field: the approver is shown read-only in the edit form
 * (matching ServiceNow's own UX once a card is submitted) even though the
 * backend would technically accept a change.
 */
export interface UpdateTimeCardInput {
  cardId: string;
  date: string;
  breakdown: ActivityBreakdown;
  billable: boolean;
  workLogComment: string;
  issueComplexity: IssueComplexity;
}

/** Input to change only the billable flag of an already-approved card. */
export interface UpdateTimeCardBillableInput {
  cardId: string;
  billable: boolean;
}

/** Payload for a lead's accept/reject decision. */
export interface TimeCardDecisionInput {
  cardId: string;
  state: Extract<TimeCardState, "approved" | "rejected">;
  leadComment?: string;
}

/**
 * Filters for the time-card search. Sent in the POST body (never as query
 * params). `projectIds`, `caseId`, `userId`/`userIds`, `approverId`, `states`,
 * and `from`/`to` are all real server-side filters — every one of them is
 * forwarded on the wire, none filtered client-side. `from`/`to` currently has
 * no UI control wired to it on most tabs; kept so a date-range filter has
 * somewhere to plug in (the Time Cards page does wire it up).
 */
export interface TimeCardSearchFilters {
  /** Projects to include (company customer may own several). */
  projectIds?: string[];
  /** Scope to a single case's own time cards (see `useCaseTimeCards`). */
  caseId?: string;
  /** Only cards submitted by this user. */
  userId?: string;
  /** Only cards submitted by any of these users — the Engineer multi-select
   * filter (All/Approvals tabs), distinct from the single-user `userId`. */
  userIds?: string[];
  /** Only cards this user is eligible to approve (backend: SN `approver_list`);
   * the caller's own cards are excluded unconditionally when this is set. */
  approverId?: string;
  /** Lifecycle states to include. */
  states?: TimeCardState[];
  /** Inclusive date range (YYYY-MM-DD), matched against the card's work date. */
  from?: string;
  to?: string;
}
