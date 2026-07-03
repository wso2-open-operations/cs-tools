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
  CaseAttachment,
  CaseAuditEntry,
  CsmCaseComment,
} from "@features/csm-cases/types/csmCases";

/** A single entry in the unified case activity feed. */
export type FeedEntry =
  | { kind: "comment"; at: string; comment: CsmCaseComment }
  | { kind: "audit"; at: string; entry: CaseAuditEntry }
  | { kind: "attachment"; at: string; attachment: CaseAttachment };

export function feedEntryId(e: FeedEntry): string {
  if (e.kind === "comment") return e.comment.id;
  if (e.kind === "audit") return e.entry.id;
  return e.attachment.id;
}

/**
 * Chronological (ascending) order with a stable tie-break. When two entries
 * share a timestamp — common for a chat question/answer pair the transcript
 * imports at the same second — a chatbot comment sorts AFTER a non-bot one so
 * the customer's question reads before Novera's answer. Falls back to id for a
 * deterministic order (a strict improvement over a bare string compare, which
 * left tied entries in arbitrary insertion order).
 *
 * Negate the result for a newest-first view.
 */
export function compareFeedEntries(a: FeedEntry, b: FeedEntry): number {
  const t = a.at.localeCompare(b.at);
  if (t !== 0) return t;
  const aBot = a.kind === "comment" && a.comment.authorRole === "chatbot";
  const bBot = b.kind === "comment" && b.comment.authorRole === "chatbot";
  if (aBot !== bBot) return aBot ? 1 : -1;
  return feedEntryId(a).localeCompare(feedEntryId(b));
}
