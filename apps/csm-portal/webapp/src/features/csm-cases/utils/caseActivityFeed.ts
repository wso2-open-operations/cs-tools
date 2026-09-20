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

import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import type {
  CaseAttachment,
  CaseAuditEntry,
  CaseFeedbackEntry,
  CsmCaseComment,
} from "@features/csm-cases/types/csmCases";

/** A single entry in the unified case activity feed. */
export type FeedEntry =
  | { kind: "comment"; at: string; comment: CsmCaseComment }
  | { kind: "audit"; at: string; entry: CaseAuditEntry }
  | { kind: "attachment"; at: string; attachment: CaseAttachment }
  | { kind: "feedback"; at: string; feedback: CaseFeedbackEntry };

export function feedEntryId(e: FeedEntry): string {
  if (e.kind === "comment") return e.comment.id;
  if (e.kind === "audit") return e.entry.id;
  if (e.kind === "attachment") return e.attachment.id;
  return e.feedback.id;
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

// Matches the handful of backend timestamp shapes `parseBackendTimestamp`
// understands (space-separated, "M/D/YYYY h:m:s", ISO "T"-separated). Plain
// text values ("High", "3", "2026") must NOT match — `new Date(...)` parses
// bare years/numbers as valid dates, which would misclassify them. Mirrors
// `CaseActivitiesFeed.tsx`'s own (unexported) `TIMESTAMP_VALUE_PATTERN` —
// duplicated rather than imported from a component module.
const AUDIT_TIMESTAMP_VALUE_PATTERN =
  /^(\d{4}-\d{1,2}-\d{1,2}[T ]\d{1,2}:\d{1,2}(:\d{1,2})?(\.\d+)?(Z|[+-]\d{2}:?\d{2})?|\d{1,2}\/\d{1,2}\/\d{4}\s+\d{1,2}:\d{1,2}(:\d{1,2})?)$/;

function formatAuditChangeValue(value: string): string {
  if (!AUDIT_TIMESTAMP_VALUE_PATTERN.test(value.trim())) return value;
  return (
    formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "short" }) ?? value
  );
}

/**
 * Single-line plain-text description of an audit/field-change entry — e.g.
 * "State: New → Work in Progress; Assignee: cleared → Jane Doe" — mirroring
 * `CaseActivitiesFeed.tsx`'s own `FieldChangeLine` JSX but as one plain
 * string, for a consumer that can't render JSX (the PDF report export).
 * Falls back to the entry's own `description` for an older/synthetic entry
 * with no structured `changes`.
 */
export function describeAuditEntry(entry: CaseAuditEntry): string {
  if (entry.changes && entry.changes.length > 0) {
    return entry.changes
      .map((c) => {
        const previous = c.previousValue?.trim();
        const next = c.newValue?.trim();
        const to = next ? formatAuditChangeValue(next) : "cleared";
        return previous ? `${c.fieldLabel}: ${formatAuditChangeValue(previous)} → ${to}` : `${c.fieldLabel}: ${to}`;
      })
      .join("; ");
  }
  return entry.description ?? "";
}
