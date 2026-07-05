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
 * The fixed activity buckets a time card splits its hours across — the
 * activity categories from the Time Management requirement (ISSU-009).
 * Write-only: the backend accepts these on create (`timeAnalyzing`,
 * `timeSettingUp`, …) but never returns them on read, so they shape the
 * log-time form only, not {@link CsmTimeCard}.
 */
export const ACTIVITY_KEYS = [
  "analysisDebugging",
  "reproduce",
  "settingUp",
  "providingSolution",
  "answering",
] as const;

export type ActivityKey = (typeof ACTIVITY_KEYS)[number];

/** Hours logged against each activity bucket (log-time form state only). */
export type ActivityBreakdown = Record<ActivityKey, number>;

/** A team lead eligible to approve a time card (ServiceNow "approver_list"). */
export interface TimeCardApprover {
  id: string;
  name: string;
}

/**
 * Work category options (the ServiceNow "Category" field). Write-only — see
 * {@link ActivityBreakdown}.
 */
export type TimeCardCategory =
  | "Task work"
  | "Investigation"
  | "Customer call"
  | "Documentation";

/** Issue-complexity options (the ServiceNow "Issue Complexity" field). Write-only. */
export type IssueComplexity = "N/A" | "Low" | "Medium" | "High";

/**
 * A time card as returned by `POST /time-cards/search` and the mutation
 * endpoints (the backend's `TimeCardView`). This is the complete set of
 * fields the backend ever returns for a card — `category`, `issueComplexity`,
 * `workLogComment`, the hour breakdown, and any lead comment are accepted on
 * write but never read back, so editing an existing card isn't supported
 * (it would silently blank those fields).
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
   * The work date (ISO, YYYY-MM-DD), despite the name — confirmed live by
   * backdating a test card and reading it back: the backend returns
   * whatever date was submitted on create under this field, not a separate
   * system-generated creation timestamp. Occasionally unparseable on real
   * records (confirmed live); see `groupIntoSheets` in `useTimeSheets.ts`
   * for how that's handled.
   */
  createdOn: string;
  userId: string;
  userName: string;
  state: TimeCardState;
  /** Whether the logged time is billable to the customer (ISSU-009). */
  billable: boolean;
  totalHours: number;
  /** The deciding approver, once a decision has been made. */
  approvedById?: string;
  approvedByName?: string;
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

/**
 * Rolled-up status of a weekly time sheet, derived from its cards. Purely a
 * frontend display grouping — the backend has no "sheet" concept, and there
 * is no bulk endpoint, so sheets carry no bulk actions.
 */
export type TimeSheetState = "submitted" | "approved" | "rejected";

/** A user's time cards for one ISO week (Mon–Sun), a display-only grouping. */
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
 * Filters for the time-card search. Sent in the POST body (never as query
 * params). `projectIds`, `userId`, `approverId`, and `from`/`to` are all
 * real, confirmed-working server-side filters. `states` is filtered
 * client-side instead — see the note on `searchTimeCards` in
 * `useTimeSheets.ts` for why. `from`/`to` currently has no UI control wired
 * to it; kept so a future date-range filter has somewhere to plug in.
 *
 * There is deliberately no `caseId` here even though `entity-service`
 * documents and implements one — confirmed live to be non-functional
 * (always `total: 0`); case scoping goes through `projectIds` plus a
 * client-side filter instead (see `useCaseTimeCards` in `useTimeCards.ts`).
 */
export interface TimeCardSearchFilters {
  /** Projects to include (company customer may own several). */
  projectIds?: string[];
  /** Only cards submitted by this user. */
  userId?: string;
  /** Only cards this user is eligible to approve (backend: SN `approver_list`). */
  approverId?: string;
  /** Lifecycle states to include. */
  states?: TimeCardState[];
  /** Inclusive date range (YYYY-MM-DD), matched against `createdOn`. */
  from?: string;
  to?: string;
}
