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
  BeCaseComment,
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

/** Best display name from the comment author block, falling back to the id. */
function authorDisplayName(author: BeCaseComment["createdBy"]): string {
  if (!author) return "Unknown";
  const full = author.fullName?.trim();
  if (full) return full;
  const composed = [author.firstName, author.lastName]
    .filter((p) => p && p.trim())
    .join(" ")
    .trim();
  return composed || author.id || "Unknown";
}

export function uiCommentFromBe(comment: BeCaseComment): CsmCaseComment {
  const role: CsmCommentAuthorRole =
    comment.type === "activity" ? "system" : "wso2_engineer";
  return {
    id: comment.id,
    caseId: comment.caseId,
    authorName: authorDisplayName(comment.createdBy),
    authorRole: role,
    // `content` is already rich-text HTML; the bubble sanitises it on render.
    bodyHtml: comment.content ?? "",
    createdAt: comment.createdOn,
    internal: comment.type === "work_note",
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
