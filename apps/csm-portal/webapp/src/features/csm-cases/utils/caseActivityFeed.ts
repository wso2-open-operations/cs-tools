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
import { severityFromBe } from "@api/backend/mappers";
import { SEVERITY_LABEL, stateLabel } from "@features/csm-dashboard/utils/abtDashboard";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import type {
  CaseAttachment,
  CaseAuditEntry,
  CaseAuditFieldChange,
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

/** Entries rendered up front, and added per scroll step, by a windowed feed. */
export const ACTIVITY_PAGE_SIZE = 50;

/**
 * The window size after one "load more" step. `Infinity` ("load all") is
 * already as large as it gets and stays put.
 */
export function nextActivityLimit(limit: number, pageSize = ACTIVITY_PAGE_SIZE): number {
  return limit + pageSize;
}

/**
 * Smallest window (a whole number of pages) that includes the entry at
 * `index` of the sorted list, or `current` when it is already inside it. Used
 * to expand the window so a permalinked entry beyond it can render.
 */
export function activityLimitToReveal(
  index: number,
  current: number,
  pageSize = ACTIVITY_PAGE_SIZE,
): number {
  if (index < 0 || index < current) return current;
  return Math.ceil((index + 1) / pageSize) * pageSize;
}

/**
 * Same ordering/tie-break as {@link compareFeedEntries}, for call sites that
 * only ever have a flat `CsmCaseComment[]` to sort (no `FeedEntry` wrapper) —
 * the standalone conversation transcript views (`ConversationDetailPage.tsx`,
 * `ConversationPreviewContent.tsx`), which never go through the case activity
 * feed's merge step. The backend returns conversation messages
 * `created_on DESC` with a random-UUID tie-break (`comment.id`, unrelated to
 * insertion order), so without this sort a Novera reply and the user message
 * that triggered it can render in either order when they share a timestamp.
 */
export function compareCommentsChronologically(a: CsmCaseComment, b: CsmCaseComment): number {
  const t = a.createdAt.localeCompare(b.createdAt);
  if (t !== 0) return t;
  const aBot = a.authorRole === "chatbot";
  const bBot = b.authorRole === "chatbot";
  if (aBot !== bBot) return aBot ? 1 : -1;
  return a.id.localeCompare(b.id);
}

// Matches the handful of backend timestamp shapes `parseBackendTimestamp`
// understands (space-separated, "M/D/YYYY h:m:s", ISO "T"-separated). Plain
// text values ("High", "3", "2026") must NOT match — `new Date(...)` parses
// bare years/numbers as valid dates, which would misclassify them. Mirrors
// `CaseActivitiesFeed.tsx`'s own (unexported) `TIMESTAMP_VALUE_PATTERN` —
// duplicated rather than imported from a component module.
const AUDIT_TIMESTAMP_VALUE_PATTERN =
  /^(\d{4}-\d{1,2}-\d{1,2}[T ]\d{1,2}:\d{1,2}(:\d{1,2})?(\.\d+)?(Z|[+-]\d{2}:?\d{2})?|\d{1,2}\/\d{1,2}\/\d{4}\s+\d{1,2}:\d{1,2}(:\d{1,2})?)$/;

/**
 * Display label for a severity field-change value — mirrors
 * `CaseActivitiesFeed.tsx`'s own (unexported) `severityChangeLabel`,
 * duplicated for the same reason `AUDIT_TIMESTAMP_VALUE_PATTERN` above is.
 * `severityFromBe` handles a humanized domain word ("Critical") and
 * ServiceNow P-notation ("P1") but NOT a bare S0-S4 code — it returns
 * `"unset"` for `severityFromBe("S1")` — so that shape is checked directly
 * first. Anything else `severityFromBe` doesn't recognize is already
 * human-readable text, so it passes through unchanged rather than becoming
 * "Unset".
 */
export function severityChangeLabel(value: string): string {
  const upper = value.trim().toUpperCase();
  if (upper in SEVERITY_LABEL) return SEVERITY_LABEL[upper as Severity];
  const severity = severityFromBe(value);
  return severity === "unset" ? value : SEVERITY_LABEL[severity];
}

function formatAuditChangeValue(value: string, field?: string): string {
  if (AUDIT_TIMESTAMP_VALUE_PATTERN.test(value.trim())) {
    return (
      formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "short" }) ?? value
    );
  }
  if (field === "state") return stateLabel(value);
  if (field === "severity") return severityChangeLabel(value);
  return value;
}

/** "Impact Low → High · Urgency Low → Medium": the changes shown beneath another one. */
export function describeChangeDetails(details: CaseAuditFieldChange[]): string {
  return details
    .map((d) => `${d.fieldLabel} ${d.previousValue?.trim() || "none"} → ${d.newValue?.trim() || "none"}`)
    .join(" · ");
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
        const to = next ? formatAuditChangeValue(next, c.field) : "cleared";
        const line = previous
          ? `${c.fieldLabel}: ${formatAuditChangeValue(previous, c.field)} → ${to}`
          : `${c.fieldLabel}: ${to}`;
        return c.details && c.details.length > 0 ? `${line} (${describeChangeDetails(c.details)})` : line;
      })
      .join("; ");
  }
  return entry.description ?? "";
}
