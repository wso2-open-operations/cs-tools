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
  BeCaseComment,
  BeCaseCommentType,
  BeCasePriority,
  BeCaseState,
} from "@api/backend/types";
import type {
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
export function severityFromPriority(
  priority: BeCasePriority | undefined,
): Severity {
  switch (priority) {
    case "catastrophic":
      return "S0";
    case "critical":
      return "S1";
    case "high":
      return "S2";
    case "medium":
      return "S3";
    case "low":
      return "S4";
    default:
      return "S3";
  }
}

export function priorityFromSeverity(severity: Severity): BeCasePriority {
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
 * The backend state list and the UI state list overlap except for the trailing
 * "ed" on `reopened`. Normalise both directions.
 */
export function uiStateFromBe(state: BeCaseState | undefined): CaseState {
  switch (state) {
    case "reopened":
      return "reopen";
    case "open":
    case "work_in_progress":
    case "waiting_on_wso2":
    case "awaiting_info":
    case "solution_proposed":
    case "closed":
      return state;
    default:
      return "open";
  }
}

export function beStateFromUi(state: CaseState): BeCaseState {
  return state === "reopen" ? "reopened" : (state as BeCaseState);
}

// ---------------------------------------------------------------------------
// Comments
// ---------------------------------------------------------------------------

function htmlFromPlain(body: string): string {
  const escaped = body
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\n/g, "<br/>");
  return `<p>${escaped}</p>`;
}

export function commentTypeFromInternal(
  internal: boolean,
): BeCaseCommentType {
  return internal ? "work_note" : "comment";
}

export function uiCommentFromBe(comment: BeCaseComment): CsmCaseComment {
  const role: CsmCommentAuthorRole =
    comment.commentType === "work_note"
      ? "wso2_engineer"
      : comment.commentType === "activity"
        ? "system"
        : "wso2_engineer";
  return {
    id: comment.id,
    caseId: comment.caseId,
    // BE returns a user id; the UI shows whatever it gets. A future user
    // lookup can hydrate this to a display name without changing the shape.
    authorName: comment.createdBy,
    authorRole: role,
    bodyHtml: htmlFromPlain(comment.body),
    createdAt: comment.createdAt,
    internal: comment.commentType === "work_note",
  };
}
