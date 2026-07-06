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
 * Mapping helpers between backend shapes (`Be*` from `api/backend/types.ts`) and the
 * legacy/UI shapes consumers expect. Keep mapping logic here so individual
 * hooks stay focused on the network call.
 */

import type {
  BeAttachment,
  BeCaseCommentAuthor,
  BeComment,
  BeCreatableCommentType,
  BeCaseSeverity,
  BeCaseState,
} from "@api/backend/types";
import type {
  CaseAttachment,
  CsmCaseComment,
  CsmCommentAuthorRole,
} from "@features/csm-cases/types/csmCases";
import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

/**
 * Best-effort mapping from the backend's five-step priority taxonomy onto the
 * UI's S0-S4 severity scale. Until the BE adds explicit severity, priority
 * doubles as the source.
 */
export function severityFromPriority(priority: string | undefined): Severity {
  if (!priority) return "S3";
  const s = priority.toLowerCase();
  // Match both P-notation ("Low (P4)", "P4") and legacy English names.
  if (s.includes("p0") || s === "catastrophic") return "S0";
  if (s.includes("p1") || s === "critical") return "S1";
  if (s.includes("p2") || s === "high") return "S2";
  if (s.includes("p3") || s === "medium") return "S3";
  if (s.includes("p4") || s === "low") return "S4";
  return "S3";
}

export function priorityFromSeverity(severity: Severity): BeCaseSeverity {
  switch (severity) {
    case "S0":
      return "catastrophic";
    case "S1":
      return "critical";
    case "S2":
      return "high";
    case "S3":
      return "medium";
    case "S4":
      return "low";
    default:
      return "medium";
  }
}

/**
 * Map a backend case state onto the UI `CaseState` vocabulary.
 *
 * The Postgres source already sends the domain enum (`work_in_progress`), but
 * the ServiceNow case-search view sends the raw SN label (`"Work In Progress"`)
 * because the entity-service normalizes every sibling field (severity, work
 * state, issue type) EXCEPT state. So we normalize label → enum here at the
 * boundary — lowercase and collapse whitespace to underscores — so both sources
 * render with the curated label/colour and downstream `state === "…"` checks
 * (e.g. the in-progress work-state indicator) work regardless of source.
 * (Ideal fix is BE-side: normalize `state` in the SN search view like the other
 * fields — tracked as a follow-up.)
 *
 * A state the frontend has not been taught about still passes through (in
 * normalized form) so `stateLabel`/`stateColor` can humanize it — that is what
 * lets the backend introduce a new state with no frontend change. A genuinely
 * absent value defaults to `open`.
 */
export function uiStateFromBe(state: string | undefined): CaseState {
  if (!state) return "open";
  return state.trim().toLowerCase().replace(/\s+/g, "_") as CaseState;
}

export function beStateFromUi(state: CaseState): BeCaseState {
  return state;
}

// ---------------------------------------------------------------------------
// Comments
// ---------------------------------------------------------------------------

export function commentTypeFromInternal(
  internal: boolean,
): BeCreatableCommentType {
  return internal ? "work_note" : "comment";
}

/** Best display name from a nested comment-author block, falling back to id. */
function authorDisplayName(author: BeCaseCommentAuthor): string {
  const full = author.fullName?.trim();
  if (full) return full;
  const composed = [author.firstName, author.lastName]
    .filter((p) => p && p.trim())
    .join(" ")
    .trim();
  return composed || author.id || "Unknown";
}

/**
 * Best display name from a {@link BeComment}. The search/messages endpoints
 * embed a nested `createdBy` object (`{id, firstName, lastName, fullName}`); the
 * comment-create ack echoes `createdBy` as a bare string. Handle both.
 */
function commentAuthorName(comment: BeComment): string {
  const cb = comment.createdBy;
  if (cb && typeof cb === "object") return authorDisplayName(cb);
  if (typeof cb === "string" && cb.trim()) return cb;
  return "Unknown";
}

/**
 * Novera/bot sender detection — mirrors the customer portal's
 * `isNoveraOrBotSender`. Chat messages carry no role field, and the backend
 * normalizes `type` to `comment`/`work_note`/`activity` (a `bot` type never
 * survives), so the bot is identified by author NAME: the nested
 * `createdBy.id` or `createdBy.fullName` (or the bare string on the create ack)
 * equalling `"Novera"`. The `type === "bot"` check is a defensive fallback.
 */
function isBotSender(comment: BeComment): boolean {
  const cb = comment.createdBy;
  const names =
    cb && typeof cb === "object"
      ? [cb.id, cb.fullName]
      : [typeof cb === "string" ? cb : ""];
  const isNovera = names.some(
    (n) => (n ?? "").trim().toLowerCase() === "novera",
  );
  const ty = (comment.type ?? "").trim().toLowerCase();
  return ty === "bot" || isNovera;
}

// The backend normalizes `type` to the singular enum (`work_note`/`comment`/
// `activity`); the plural forms are kept as a defensive fallback in case an
// un-normalized SN value slips through.
const WORK_NOTE_TYPES = new Set(["work_note", "work_notes"]);
const ACTIVITY_TYPES = new Set(["activity", "activities"]);

/**
 * Map a backend comment onto the UI shape. Used for both case comments and
 * conversation (chat) messages — they share the `/comments/search` shape.
 *
 * `context` sets the default author role for a non-bot, non-system message:
 * chat participants default to `"customer"`, while a case comment defaults to
 * `"wso2_engineer"` (the FE has no reliable customer-vs-engineer signal on a
 * case comment yet). A Novera/bot sender always maps to `"chatbot"`.
 */
export function uiCommentFromBe(
  comment: BeComment,
  opts?: { context?: "case" | "conversation" },
): CsmCaseComment {
  const ty = (comment.type ?? "").trim().toLowerCase();
  let role: CsmCommentAuthorRole;
  if (isBotSender(comment)) {
    role = "chatbot";
  } else if (ACTIVITY_TYPES.has(ty)) {
    role = "system";
  } else {
    role = opts?.context === "conversation" ? "customer" : "wso2_engineer";
  }
  return {
    id: comment.id,
    caseId: comment.referenceId ?? "",
    authorName: commentAuthorName(comment),
    // For a chatbot the body is Markdown; the bubble renders it as Markdown.
    // Otherwise it is rich-text HTML, sanitised on render.
    bodyHtml: comment.content ?? "",
    authorRole: role,
    createdAt: comment.createdOn,
    internal: WORK_NOTE_TYPES.has(ty),
  };
}

// ---------------------------------------------------------------------------
// Attachments
// ---------------------------------------------------------------------------

/** Map a backend attachment onto the UI's `CaseAttachment` shape. */
export function uiAttachmentFromBe(att: BeAttachment): CaseAttachment {
  return {
    id: att.id,
    filename: att.name,
    size: att.sizeBytes,
    contentType: att.type,
    uploadedBy: att.createdBy || "Unknown",
    uploadedAt: att.createdOn,
  };
}
