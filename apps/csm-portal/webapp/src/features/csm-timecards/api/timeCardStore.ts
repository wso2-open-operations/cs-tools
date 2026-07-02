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

//
// FE-first in-memory store for time cards. This stands in for the backend until
// `csm-portal/backend` exposes the time-card endpoints (see the API contract in
// `useTimeCards.ts`). It is a module-level singleton, so data persists across
// navigation but resets on a full page reload. Swap the hooks in `useTimeCards`
// to `useBackendApi` once the endpoints exist; this file then goes away.
//

import type {
  ApproverDelegation,
  CreateTimeCardInput,
  CsmTimeCard,
  CsmTimeSheet,
  TimeCardActivity,
  TimeCardActivityAction,
  TimeCardApprover,
  TimeCardDecisionInput,
  TimeCardDeletion,
  TimecardReports,
  TimeCardSearchFilters,
} from "@features/csm-timecards/types/timeCards";
import { MOCK_ASSIGNED_TASKS } from "@features/csm-timecards/constants/timeCardConstants";
import {
  emptyBreakdown,
  totalHours,
} from "@features/csm-timecards/utils/timeCardTotals";
import {
  weekEndOf,
  weekKey,
  weekStartOf,
} from "@features/csm-timecards/utils/timeSheetWeek";
import {
  EDITABLE_STATES,
  sheetStatus,
} from "@features/csm-timecards/utils/timeSheetState";

function genId(): string {
  return typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `tc-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

/** A few days back from `base`, as YYYY-MM-DD. */
function daysAgo(base: Date, n: number): string {
  const d = new Date(base);
  d.setDate(d.getDate() - n);
  return d.toISOString().slice(0, 10);
}

const LEAD: TimeCardApprover = { id: "lead-chathura", name: "Chathura Wijesinghe" };

/** Demo projects the seeded cases belong to (a company customer with several). */
const SEED_PROJECTS = {
  apim: { projectId: "proj-apim", projectName: "API Manager Platform" },
  iam: { projectId: "proj-iam", projectName: "Identity & Access" },
  integration: { projectId: "proj-int", projectName: "Integration Hub" },
} as const;

/** Append a lifecycle event to a card's audit trail. */
function logActivity(
  card: CsmTimeCard,
  action: TimeCardActivityAction,
  by: string,
  note?: string,
): CsmTimeCard {
  return {
    ...card,
    activity: [
      ...card.activity,
      { at: new Date().toISOString(), by, action, ...(note ? { note } : {}) },
    ],
  };
}

/** Reconstruct a plausible audit trail for a seeded card from its state. */
function deriveSeedActivity(c: CsmTimeCard): TimeCardActivity[] {
  const out: TimeCardActivity[] = [
    { at: c.submittedAt, by: c.userName, action: "created" },
  ];
  if (c.state !== "pending") {
    out.push({ at: c.submittedAt, by: c.userName, action: "submitted" });
  }
  if (c.decidedAt) {
    if (c.state === "approved" || c.state === "processed") {
      out.push({
        at: c.decidedAt,
        by: c.decidedBy ?? LEAD.name,
        action: "approved",
        ...(c.leadComment ? { note: c.leadComment } : {}),
      });
    } else if (c.state === "rejected") {
      out.push({
        at: c.decidedAt,
        by: c.decidedBy ?? LEAD.name,
        action: "rejected",
        ...(c.leadComment ? { note: c.leadComment } : {}),
      });
    } else if (c.state === "recalled") {
      out.push({ at: c.decidedAt, by: c.decidedBy ?? LEAD.name, action: "recalled" });
    }
  }
  if (c.state === "processed") {
    out.push({
      at: c.decidedAt ?? c.submittedAt,
      by: "Time-card admin",
      action: "processed",
    });
  }
  return out;
}

function seed(): CsmTimeCard[] {
  const now = new Date();
  const mk = (
    partial: Omit<
      CsmTimeCard,
      "id" | "totalHours" | "submittedAt" | "approvers" | "taskType" | "activity"
    >,
  ): CsmTimeCard => {
    const base: CsmTimeCard = {
      id: genId(),
      taskType: "case",
      approvers: [LEAD],
      submittedAt: new Date(`${partial.date}T09:00:00Z`).toISOString(),
      totalHours: totalHours(partial.breakdown),
      activity: [],
      ...partial,
    };
    return { ...base, activity: deriveSeedActivity(base) };
  };

  return [
    mk({
      caseId: "seed-case-1",
      caseNumber: "CS0352584",
      ...SEED_PROJECTS.apim,
      date: daysAgo(now, 0),
      userId: "eng-sajith",
      userName: "Sajith Ekanayaka",
      state: "submitted",
      category: "Task work",
      billable: true,
      breakdown: {
        analysisDebugging: 1,
        reproduce: 1.5,
        settingUp: 0.5,
        providingSolution: 0.5,
        answering: 0,
      },
      workLogComment: "Investigated the OOM, reproduced locally and narrowed it to the cache layer.",
      issueComplexity: "Medium",
    }),
    mk({
      caseId: "seed-case-2",
      caseNumber: "CS0351002",
      ...SEED_PROJECTS.iam,
      date: daysAgo(now, 1),
      userId: "eng-sajith",
      userName: "Sajith Ekanayaka",
      state: "approved",
      category: "Task work",
      billable: false,
      breakdown: {
        analysisDebugging: 0.5,
        reproduce: 1,
        settingUp: 0,
        providingSolution: 0.5,
        answering: 0,
      },
      workLogComment: "Answered the customer's clustering question and shared a config sample.",
      issueComplexity: "Low",
      leadComment: "Looks good — thanks for the quick turnaround.",
      decidedAt: new Date(`${daysAgo(now, 0)}T11:00:00Z`).toISOString(),
      decidedBy: LEAD.name,
    }),
    mk({
      caseId: "seed-case-3",
      caseNumber: "CS0349881",
      ...SEED_PROJECTS.integration,
      date: daysAgo(now, 2),
      userId: "eng-nimal",
      userName: "Nimal Perera",
      state: "submitted",
      category: "Investigation",
      billable: true,
      breakdown: {
        analysisDebugging: 2,
        reproduce: 1,
        settingUp: 1,
        providingSolution: 0,
        answering: 0,
      },
      workLogComment: "Deep dive into the gateway latency spike; collected thread dumps.",
      issueComplexity: "High",
    }),
    mk({
      caseId: "seed-case-4",
      caseNumber: "CS0348220",
      ...SEED_PROJECTS.apim,
      date: daysAgo(now, 3),
      userId: "eng-nimal",
      userName: "Nimal Perera",
      state: "approved",
      category: "Task work",
      billable: true,
      breakdown: {
        analysisDebugging: 0.5,
        reproduce: 0,
        settingUp: 0,
        providingSolution: 1,
        answering: 1.5,
      },
      workLogComment: "Built and verified the hotfix patch; handed over to release.",
      issueComplexity: "Medium",
      leadComment: "Approved.",
      decidedAt: new Date(`${daysAgo(now, 2)}T15:00:00Z`).toISOString(),
      decidedBy: LEAD.name,
    }),
  ];
}

let store: CsmTimeCard[] = seed();

function clone(card: CsmTimeCard): CsmTimeCard {
  return {
    ...card,
    breakdown: { ...card.breakdown },
    approvers: [...card.approvers],
    activity: card.activity.map((a) => ({ ...a })),
  };
}

const adoptedUsers = new Set<string>();

/**
 * Mock convenience: the first time we see a signed-in user with no cards of
 * their own, seed two demo cards under their identity so "My cards" isn't empty
 * on first visit. One-time per user per session; a no-op once they have any
 * cards. Goes away with the rest of this file when the backend lands.
 */
export function ensureDemoCardsForUser(userId: string, userName: string): void {
  if (adoptedUsers.has(userId)) return;
  adoptedUsers.add(userId);
  if (store.some((c) => c.userId === userId)) return;

  const now = new Date();
  const pending: CsmTimeCard = {
    id: genId(),
    taskType: "case",
    activity: [],
    caseId: "seed-case-mine-1",
    caseNumber: "CS0353001",
    ...SEED_PROJECTS.apim,
    date: daysAgo(now, 0),
    userId,
    userName,
    state: "pending",
    category: "Task work",
    billable: true,
    breakdown: {
      analysisDebugging: 1,
      reproduce: 1,
      settingUp: 0.5,
      providingSolution: 0.5,
      answering: 0,
    },
    totalHours: 3,
    workLogComment: "Triaged the customer's report and reproduced the issue locally.",
    issueComplexity: "Medium",
    approvers: [LEAD],
    submittedAt: new Date(`${daysAgo(now, 0)}T10:00:00Z`).toISOString(),
  };
  const approved: CsmTimeCard = {
    id: genId(),
    taskType: "case",
    activity: [],
    caseId: "seed-case-mine-2",
    caseNumber: "CS0352900",
    ...SEED_PROJECTS.iam,
    date: daysAgo(now, 1),
    userId,
    userName,
    state: "approved",
    category: "Investigation",
    billable: false,
    breakdown: {
      analysisDebugging: 1.5,
      reproduce: 0.5,
      settingUp: 0,
      providingSolution: 1,
      answering: 0,
    },
    totalHours: 3,
    workLogComment: "Reviewed logs and confirmed the configuration fix with the customer.",
    issueComplexity: "Low",
    leadComment: "Nice work.",
    approvers: [LEAD],
    submittedAt: new Date(`${daysAgo(now, 1)}T10:00:00Z`).toISOString(),
    decidedAt: new Date(`${daysAgo(now, 0)}T09:00:00Z`).toISOString(),
    decidedBy: LEAD.name,
  };
  pending.activity = deriveSeedActivity(pending);
  approved.activity = deriveSeedActivity(approved);
  store = [pending, approved, ...store];
}

/** All time cards, newest submission first. */
export function listAllTimeCards(): CsmTimeCard[] {
  return [...store]
    .sort((a, b) => b.submittedAt.localeCompare(a.submittedAt))
    .map(clone);
}

/** Time cards for a single case, newest first. */
export function listCaseTimeCards(caseId: string): CsmTimeCard[] {
  return listAllTimeCards().filter((c) => c.caseId === caseId);
}

/** Create a new pending time card on behalf of `engineer`. */
export function createTimeCard(
  input: CreateTimeCardInput,
  engineer: { id: string; name: string },
): CsmTimeCard {
  const now = new Date().toISOString();
  const card: CsmTimeCard = {
    id: genId(),
    taskType: input.taskType,
    caseId: input.caseId,
    caseNumber: input.caseNumber,
    projectId: input.projectId,
    projectName: input.projectName,
    date: input.date,
    userId: engineer.id,
    userName: engineer.name,
    state: "pending",
    category: input.category,
    billable: input.billable,
    breakdown: { ...input.breakdown },
    totalHours: totalHours(input.breakdown),
    workLogComment: input.workLogComment,
    issueComplexity: input.issueComplexity,
    approvers: [input.approver],
    submittedAt: now,
    activity: [{ at: now, by: engineer.name, action: "created" }],
  };
  store = [card, ...store];
  return clone(card);
}

/** Apply a lead's accept/reject decision. Throws if the card is missing. */
export function decideTimeCard(
  decision: TimeCardDecisionInput,
  decidedBy: string,
): CsmTimeCard {
  const idx = store.findIndex((c) => c.id === decision.cardId);
  if (idx < 0) throw new Error("Time card not found.");
  const comment = decision.leadComment?.trim() || undefined;
  const updated = logActivity(
    {
      ...store[idx],
      state: decision.state,
      leadComment: comment,
      decidedAt: new Date().toISOString(),
      decidedBy,
    },
    decision.state === "approved" ? "approved" : "rejected",
    decidedBy,
    comment,
  );
  store = store.map((c, i) => (i === idx ? updated : c));
  return clone(updated);
}

// ---------------------------------------------------------------------------
// Time sheets (weekly grouping)
// ---------------------------------------------------------------------------

function round2(n: number): number {
  return Math.round(n * 100) / 100;
}

function buildSheet(
  userId: string,
  userName: string,
  weekStart: string,
  cards: CsmTimeCard[],
): CsmTimeSheet {
  const weekCards = cards
    .filter((c) => weekStartOf(c.date) === weekStart)
    .sort((a, b) => a.date.localeCompare(b.date))
    .map(clone);
  return {
    id: `${userId}:${weekStart}`,
    userId,
    userName,
    weekStart,
    weekEnd: weekEndOf(weekStart),
    state: sheetStatus(weekCards),
    cards: weekCards,
    totalHours: round2(weekCards.reduce((s, c) => s + c.totalHours, 0)),
  };
}

/**
 * Whether a card matches the search filters. Empty/omitted fields match
 * everything. Mirrors what the backend `POST /time-cards/search` body will do,
 * so the FE filter contract is identical when the mock is swapped out.
 */
function matchesFilters(c: CsmTimeCard, f?: TimeCardSearchFilters): boolean {
  if (!f) return true;
  if (f.projectIds?.length && !f.projectIds.includes(c.projectId)) return false;
  if (f.workItemId) {
    const q = f.workItemId.trim().toLowerCase();
    if (q && !c.caseNumber.toLowerCase().includes(q)) return false;
  }
  if (f.engineerId && c.userId !== f.engineerId) return false;
  if (f.states?.length && !f.states.includes(c.state)) return false;
  if (f.from && c.date < f.from) return false;
  if (f.to && c.date > f.to) return false;
  return true;
}

/**
 * Mock implementation of `POST /time-cards/search`. Returns the filtered cards
 * (newest submission first). All criteria travel in the request body, never as
 * query params, so case / work-item / engineer ids stay out of URLs and logs.
 */
export function searchTimeCards(filters?: TimeCardSearchFilters): CsmTimeCard[] {
  return listAllTimeCards().filter((c) => matchesFilters(c, filters));
}

function groupUserSheets(
  userId: string,
  filters?: TimeCardSearchFilters,
): CsmTimeSheet[] {
  const mine = store.filter(
    (c) => c.userId === userId && matchesFilters(c, filters),
  );
  const userName = mine[0]?.userName ?? "";
  const weeks = [...new Set(mine.map((c) => weekKey(c.date)))].sort((a, b) =>
    b.localeCompare(a),
  );
  return weeks.map((w) => buildSheet(userId, userName, w, mine));
}

/** A user's weekly time sheets, newest week first; optionally filtered. */
export function listMyTimeSheets(
  userId: string,
  filters?: TimeCardSearchFilters,
): CsmTimeSheet[] {
  return groupUserSheets(userId, filters);
}

/**
 * Seed the target week with zero-hour pending cards for each distinct task the
 * user logged in their most recent prior week (ServiceNow "copy from previous
 * time sheet"). Skips tasks already present in the target week. Returns the
 * number of cards created.
 */
export function copyPreviousWeek(
  userId: string,
  targetWeekStart: string,
): number {
  const mine = store.filter((c) => c.userId === userId);
  if (mine.length === 0) return 0;
  const userName = mine[0].userName;

  const prevWeek = [...new Set(mine.map((c) => weekStartOf(c.date)))]
    .filter((w) => w < targetWeekStart)
    .sort((a, b) => b.localeCompare(a))[0];
  if (!prevWeek) return 0;

  const present = new Set(
    mine
      .filter((c) => weekStartOf(c.date) === targetWeekStart)
      .map((c) => `${c.taskType}:${c.caseNumber}`),
  );

  const tasks = new Map<string, CsmTimeCard>();
  for (const c of mine.filter((c) => weekStartOf(c.date) === prevWeek)) {
    const key = `${c.taskType}:${c.caseNumber}`;
    if (!present.has(key) && !tasks.has(key)) tasks.set(key, c);
  }

  const now = new Date().toISOString();
  const created: CsmTimeCard[] = [...tasks.values()].map((t) => ({
    id: genId(),
    taskType: t.taskType,
    caseId: t.caseId,
    caseNumber: t.caseNumber,
    projectId: t.projectId,
    projectName: t.projectName,
    date: targetWeekStart,
    userId,
    userName,
    state: "pending",
    category: t.category,
    billable: t.billable,
    breakdown: emptyBreakdown(),
    totalHours: 0,
    workLogComment: "",
    issueComplexity: t.issueComplexity,
    approvers: [...t.approvers],
    submittedAt: now,
    activity: [{ at: now, by: userName, action: "created" }],
  }));
  store = [...created, ...store];
  return created.length;
}

/**
 * Generate zero-hour pending cards for the user's assigned tasks (mock list)
 * not already present in the target week (ServiceNow "auto-generate time
 * cards"). Returns the number of cards created.
 */
export function autoGenerateCards(
  userId: string,
  userName: string,
  weekStart: string,
): number {
  const present = new Set(
    store
      .filter((c) => c.userId === userId && weekStartOf(c.date) === weekStart)
      .map((c) => `${c.taskType}:${c.caseNumber}`),
  );
  const now = new Date().toISOString();
  const created: CsmTimeCard[] = MOCK_ASSIGNED_TASKS.filter(
    (t) => !present.has(`${t.taskType}:${t.reference}`),
  ).map((t) => ({
    id: genId(),
    taskType: t.taskType,
    caseId: t.reference,
    caseNumber: t.reference,
    projectId: SEED_PROJECTS.apim.projectId,
    projectName: SEED_PROJECTS.apim.projectName,
    date: weekStart,
    userId,
    userName,
    state: "pending",
    category: t.category,
    billable: true,
    breakdown: emptyBreakdown(),
    totalHours: 0,
    workLogComment: "",
    issueComplexity: "N/A",
    approvers: [LEAD],
    submittedAt: now,
    activity: [{ at: now, by: userName, action: "created" }],
  }));
  store = [...created, ...store];
  return created.length;
}

function mutateCard(
  cardId: string,
  fn: (c: CsmTimeCard) => CsmTimeCard,
): CsmTimeCard {
  const idx = store.findIndex((c) => c.id === cardId);
  if (idx < 0) throw new Error("Time card not found.");
  const updated = fn(store[idx]);
  store = store.map((c, i) => (i === idx ? updated : c));
  return clone(updated);
}

/** Submit every editable card in a user's week (pending/rejected/recalled →
 * submitted), clearing any prior decision. */
export function submitSheet(userId: string, weekStart: string): void {
  const now = new Date().toISOString();
  store = store.map((c) =>
    c.userId === userId &&
    weekStartOf(c.date) === weekStart &&
    EDITABLE_STATES.includes(c.state)
      ? logActivity(
          {
            ...c,
            state: "submitted",
            submittedAt: now,
            decidedAt: undefined,
            decidedBy: undefined,
            leadComment: undefined,
          },
          "submitted",
          c.userName,
        )
      : c,
  );
}

/** Approve all still-submitted cards in a sheet ("approve remaining"). */
export function approveSheet(
  userId: string,
  weekStart: string,
  by: string,
  comment?: string,
): void {
  const now = new Date().toISOString();
  store = store.map((c) =>
    c.userId === userId &&
    weekStartOf(c.date) === weekStart &&
    c.state === "submitted"
      ? logActivity(
          {
            ...c,
            state: "approved",
            decidedAt: now,
            decidedBy: by,
            leadComment: comment?.trim() || c.leadComment,
          },
          "approved",
          by,
          comment?.trim() || undefined,
        )
      : c,
  );
}

/** Reject every still-submitted card in a sheet ("reject sheet"). */
export function rejectSheet(
  userId: string,
  weekStart: string,
  by: string,
  comment?: string,
): void {
  const now = new Date().toISOString();
  store = store.map((c) =>
    c.userId === userId &&
    weekStartOf(c.date) === weekStart &&
    c.state === "submitted"
      ? logActivity(
          {
            ...c,
            state: "rejected",
            decidedAt: now,
            decidedBy: by,
            leadComment: comment?.trim() || undefined,
          },
          "rejected",
          by,
          comment?.trim() || undefined,
        )
      : c,
  );
}

/** Recall every approved card in a sheet back to the user. */
export function recallSheet(
  userId: string,
  weekStart: string,
  by: string,
): void {
  const now = new Date().toISOString();
  store = store.map((c) =>
    c.userId === userId &&
    weekStartOf(c.date) === weekStart &&
    c.state === "approved"
      ? logActivity(
          { ...c, state: "recalled", decidedAt: now, decidedBy: by },
          "recalled",
          by,
        )
      : c,
  );
}

/** Recall a single approved card. */
export function recallCard(cardId: string, by: string): CsmTimeCard {
  return mutateCard(cardId, (c) =>
    logActivity(
      { ...c, state: "recalled", decidedAt: new Date().toISOString(), decidedBy: by },
      "recalled",
      by,
    ),
  );
}

/** Mark an approved card as processed (expensed). Admin action. */
export function processCard(cardId: string): CsmTimeCard {
  return mutateCard(cardId, (c) =>
    logActivity({ ...c, state: "processed" }, "processed", "Time-card admin"),
  );
}

/** Edit an editable card's fields (owner or admin). */
export function updateCard(
  cardId: string,
  patch: Partial<
    Pick<
      CsmTimeCard,
      | "date"
      | "category"
      | "billable"
      | "breakdown"
      | "workLogComment"
      | "issueComplexity"
    >
  >,
): CsmTimeCard {
  return mutateCard(cardId, (c) => {
    const breakdown = patch.breakdown ?? c.breakdown;
    return logActivity(
      { ...c, ...patch, breakdown, totalHours: totalHours(breakdown) },
      "edited",
      c.userName,
    );
  });
}

/**
 * Delete an editable card (owner or admin). Only Pending/Rejected/Recalled cards
 * may be removed — submitted/approved/processed work stays. The card leaves the
 * active store but a {@link TimeCardDeletion} tombstone is kept so the actor,
 * hours removed, and timestamp survive (ISSU-009 audit). Throws if the card is
 * missing or not in an editable state.
 */
export function deleteCard(cardId: string, by: string): void {
  const card = store.find((c) => c.id === cardId);
  if (!card) throw new Error("Time card not found.");
  if (!EDITABLE_STATES.includes(card.state)) {
    throw new Error("Only editable time cards can be deleted.");
  }
  deletionAudit = [
    {
      cardId: card.id,
      caseNumber: card.caseNumber,
      userName: card.userName,
      totalHours: card.totalHours,
      deletedBy: by,
      at: new Date().toISOString(),
    },
    ...deletionAudit,
  ];
  store = store.filter((c) => c.id !== cardId);
}

/** Permanent audit of deleted cards (newest first). */
let deletionAudit: TimeCardDeletion[] = [];

/** Read the deletion audit log (newest first). */
export function listDeletionAudit(): TimeCardDeletion[] {
  return deletionAudit.map((d) => ({ ...d }));
}

/** Submitted sheets awaiting a decision, excluding the viewer's own. */
export function listApprovalQueue(
  viewerId: string,
  filters?: TimeCardSearchFilters,
): CsmTimeSheet[] {
  const others = [
    ...new Set(store.filter((c) => c.userId !== viewerId).map((c) => c.userId)),
  ];
  const sheets: CsmTimeSheet[] = [];
  for (const uid of others) {
    for (const s of groupUserSheets(uid, filters)) {
      if (s.cards.some((c) => c.state === "submitted")) sheets.push(s);
    }
  }
  return sheets.sort((a, b) => a.weekStart.localeCompare(b.weekStart));
}

// ---------------------------------------------------------------------------
// Delegation
// ---------------------------------------------------------------------------

let delegations: ApproverDelegation[] = [];

export function setDelegation(d: ApproverDelegation): void {
  delegations = [...delegations.filter((x) => x.approverId !== d.approverId), d];
}

export function clearDelegation(approverId: string): void {
  delegations = delegations.filter((x) => x.approverId !== approverId);
}

/** The active delegation for an approver on a given day, if any. */
export function activeDelegationFor(
  approverId: string,
  onIso: string = new Date().toISOString().slice(0, 10),
): ApproverDelegation | undefined {
  return delegations.find(
    (d) => d.approverId === approverId && d.from <= onIso && onIso <= d.to,
  );
}

// ---------------------------------------------------------------------------
// Reports
// ---------------------------------------------------------------------------

/** Days between two ISO timestamps (>= 0). */
function daysBetween(fromIso: string, toIso: string): number {
  const ms = new Date(toIso).getTime() - new Date(fromIso).getTime();
  return Math.max(0, ms / (24 * 60 * 60 * 1000));
}

export function computeReports(filter?: { from?: string; to?: string }): TimecardReports {
  const all = store.filter((c) => {
    if (filter?.from && c.date < filter.from) return false;
    if (filter?.to && c.date > filter.to) return false;
    return true;
  });
  const isApproved = (s: CsmTimeCard["state"]): boolean =>
    s === "approved" || s === "processed";

  // Rejected time is invalidated work, so it's excluded from effort/billing
  // aggregates (hours, category & engineer rollups). Counts (pending/approved/
  // rejected) below still reflect every card.
  const counted = all.filter((c) => c.state !== "rejected");

  const decided = all.filter((c) => c.decidedAt && (isApproved(c.state) || c.state === "rejected"));
  const avgApprovalLagDays = decided.length
    ? round2(
        decided.reduce(
          (s, c) => s + (c.decidedAt ? daysBetween(c.submittedAt, c.decidedAt) : 0),
          0,
        ) / decided.length,
      )
    : 0;

  const submittedPlus = all.filter((c) => c.state !== "pending").length;

  const byCategoryMap = new Map<string, number>();
  for (const c of counted) {
    byCategoryMap.set(c.category, (byCategoryMap.get(c.category) ?? 0) + c.totalHours);
  }

  const byUserMap = new Map<
    string,
    { userId: string; userName: string; hours: number; pending: number; approved: number }
  >();
  for (const c of all) {
    const u =
      byUserMap.get(c.userId) ??
      { userId: c.userId, userName: c.userName, hours: 0, pending: 0, approved: 0 };
    if (c.state !== "rejected") u.hours += c.totalHours;
    if (c.state === "submitted") u.pending += 1;
    if (isApproved(c.state)) u.approved += 1;
    byUserMap.set(c.userId, u);
  }

  const nowIso = new Date().toISOString();
  const exceptions: TimecardReports["exceptions"] = [];
  for (const c of all) {
    if (c.state === "submitted" && daysBetween(c.submittedAt, nowIso) > 3) {
      exceptions.push({
        id: c.id,
        userName: c.userName,
        caseNumber: c.caseNumber,
        kind: "overdue",
        detail: `Awaiting approval ${Math.floor(daysBetween(c.submittedAt, nowIso))}d`,
      });
    } else if (c.state === "rejected") {
      exceptions.push({
        id: c.id,
        userName: c.userName,
        caseNumber: c.caseNumber,
        kind: "rejected",
        detail: c.leadComment || "Rejected — needs rework",
      });
    }
  }

  const billableHours = round2(
    counted.filter((c) => c.billable).reduce((s, c) => s + c.totalHours, 0),
  );
  const nonBillableHours = round2(
    counted.filter((c) => !c.billable).reduce((s, c) => s + c.totalHours, 0),
  );

  return {
    totalHours: round2(counted.reduce((s, c) => s + c.totalHours, 0)),
    billableHours,
    nonBillableHours,
    pendingApproval: all.filter((c) => c.state === "submitted").length,
    approved: all.filter((c) => isApproved(c.state)).length,
    rejected: all.filter((c) => c.state === "rejected").length,
    avgApprovalLagDays,
    submissionRate: all.length ? round2(submittedPlus / all.length) : 0,
    byCategory: [...byCategoryMap.entries()]
      .map(([category, hours]) => ({ category, hours: round2(hours) }))
      .sort((a, b) => b.hours - a.hours),
    byUser: [...byUserMap.values()]
      .map((u) => ({ ...u, hours: round2(u.hours) }))
      .sort((a, b) => b.hours - a.hours),
    exceptions,
    deletions: listDeletionAudit(),
  };
}
