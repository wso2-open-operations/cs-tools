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
  BeStandardChangeCabComment,
  BeStandardChangeCabHistory,
  BeStandardChangeCabRejection,
  BeStandardChangeSuggestion,
} from "@api/backend/types";

// The CAB's pushback on a standard change (entity-service, "The CAB's
// pushback"): its rejections and comments on a suggestion's change requests and
// look-alikes, and on a proposal's source and evidence. It is shown, like a
// deviation from the plan, and decides nothing.

function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}

/** "2 CAB rejections", or null for none. */
export function cabRejectionsLabel(n?: number | null): string | null {
  return n ? plural(n, "CAB rejection", "CAB rejections") : null;
}

/** "3 CAB comments", or null for none. */
export function cabCommentsLabel(n?: number | null): string | null {
  return n ? plural(n, "CAB comment", "CAB comments") : null;
}

/** "1 deviated from plan", or null for none. */
export function deviationLabel(n?: number | null): string | null {
  return n ? `${n} deviated from plan` : null;
}

/**
 * How many of a suggestion's members deviated from their plan: the count
 * entity-service sends (for a snapshot computed before it kept one, it counts
 * the listed members itself), else, from a backend that sends none, the
 * listed members that did.
 */
export function suggestionDeviationCount(s: Pick<BeStandardChangeSuggestion, "deviationCount" | "members">): number {
  return s.deviationCount ?? s.members.filter((m) => m.hasDeviation).length;
}

/** The lists and counts of a CAB history, with absent or null read as none. */
export function cabHistoryOf(h?: Partial<BeStandardChangeCabHistory> | null): {
  rejections: BeStandardChangeCabRejection[];
  rejectionCount: number;
  comments: BeStandardChangeCabComment[];
  commentCount: number;
} {
  const rejections = h?.cabRejections ?? [];
  const comments = h?.cabComments ?? [];
  return {
    rejections,
    rejectionCount: Math.max(h?.cabRejectionCount ?? 0, rejections.length),
    comments,
    commentCount: Math.max(h?.cabCommentCount ?? 0, comments.length),
  };
}

/** Whether the CAB said anything at all. */
export function hasCabPushback(h?: Partial<BeStandardChangeCabHistory> | null): boolean {
  const { rejectionCount, commentCount } = cabHistoryOf(h);
  return rejectionCount > 0 || commentCount > 0;
}

export const NO_CAB_PUSHBACK = "No CAB rejections or comments";

/** The headline over a proposal's CAB history: "The CAB rejected 2 change requests like this one". */
export function cabHistoryHeadline(h?: Partial<BeStandardChangeCabHistory> | null): string {
  const { rejectionCount, commentCount } = cabHistoryOf(h);
  if (rejectionCount > 0) {
    return `The CAB rejected ${plural(rejectionCount, "change request", "change requests")} like this one`;
  }
  if (commentCount > 0) {
    return `The CAB left ${plural(commentCount, "comment", "comments")} on change requests like this one`;
  }
  return NO_CAB_PUSHBACK;
}

/**
 * One short line for the decision dialog: "The CAB rejected 2 change requests
 * like this one and left 3 comments. Latest: "Plan unclear"".
 */
export function cabHistorySummary(h?: Partial<BeStandardChangeCabHistory> | null): string {
  const { rejections, rejectionCount, comments, commentCount } = cabHistoryOf(h);
  if (rejectionCount === 0 && commentCount === 0) return `${NO_CAB_PUSHBACK}.`;
  const parts: string[] = [];
  if (rejectionCount > 0) {
    parts.push(`The CAB rejected ${plural(rejectionCount, "change request", "change requests")} like this one`);
    if (commentCount > 0) parts.push(`and left ${plural(commentCount, "comment", "comments")}`);
  } else {
    parts.push(`The CAB left ${plural(commentCount, "comment", "comments")} on change requests like this one`);
  }
  // The newest comment of any decision (a rejection's own comment is among
  // them): an approval written after a rejection is the latest word. A
  // rejection's comment stands in only when no comment is listed.
  const latest = comments[0]?.comment || rejections.find((r) => r.comment)?.comment;
  return `${parts.join(" ")}.${latest ? ` Latest: "${latest}"` : ""}`;
}

/** An approver's decision in words: "approved", "rejected", "asked" (still to decide). */
export function cabDecisionText(state: string): string {
  switch (state.toUpperCase()) {
    case "APPROVED":
      return "approved";
    case "REJECTED":
      return "rejected";
    case "REQUESTED":
      return "asked";
    case "CANCELLED":
      return "cancelled";
    default:
      return state.toLowerCase().replace(/_/g, " ");
  }
}
