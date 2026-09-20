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

import { jsPDF } from "jspdf";
import {
  type PdfActivityRow,
  type PdfCursor,
  type PdfDetailsRow,
  condenseBlankLines,
  htmlToPdfPlainText,
  safeFileNamePart,
  stampFooterPageNumbers,
  writeActivityList,
  writeDetailsTable,
  writeHeading,
  writeReportHeader,
  writeWrapped,
} from "@utils/pdfReportKit";
import { isBlankHtml } from "@utils/sanitizeHtml";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { formatBytes } from "@utils/formatBytes";
import { stateLabel } from "@features/csm-dashboard/utils/abtDashboard";
import { SEVERITY_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import { caseTypeTransferLabel } from "@features/csm-cases/utils/caseTypeTransfer";
import {
  hasDisplayableContent,
  preprocessCommentBodyHtml,
} from "@features/csm-cases/utils/commentContent";
import { describeAuditEntry } from "@features/csm-cases/utils/caseActivityFeed";
import type {
  CaseAttachment,
  CaseAuditEntry,
  CaseFeedbackEntry,
  CsmCaseDetail,
  CsmCaseComment,
} from "@features/csm-cases/types/csmCases";

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "short" }) ?? "—"
  );
}

function severityLabel(severity: CsmCaseDetail["severity"]): string {
  return severity === "unset" ? "Unset" : SEVERITY_LABEL[severity];
}

/**
 * Renders a case (or engagement/service-request/announcement/security-report —
 * every `CsmCaseDetail`-backed kind `CsmCaseDetailPage` renders) plus its full
 * activity trail as a standalone, readable PDF report, for sharing with a
 * customer or keeping as compliance evidence.
 *
 * "Full activity trail" means *every* lane `CaseActivitiesFeed.tsx` merges
 * into its own on-screen feed, not just comments — that component's own "N
 * entries" badge is `comments + audit + attachments + feedback` in exactly
 * that combination (see `CsmCaseDetailPage.tsx`'s `safeComments`/`activityAudit`/
 * `attachmentList`/`caseFeedback`), and this export mirrors it entry for
 * entry: the pre-case chat transcript (already merged into `comments` by the
 * caller, matching `mergedComments` on that page), audited state/field
 * changes (`activity`), uploaded attachments, and CSAT feedback submissions —
 * all four merged into one chronological list. Two earlier versions of this
 * export were reported live as still missing things after each fix: first it
 * only had comments at all (no audit trail); once the audit trail was added,
 * attachments/feedback/chat were still missing — this version is the first to
 * cover all four lanes the on-screen feed does. Distinct from the browser's
 * native Ctrl+P/Cmd+P print (see `src/styles/print.css`), which stays as-is;
 * this produces an actual formatted document via `jsPDF`, not a snapshot of
 * the on-screen UI. Follows the same manual cursor/page-break approach as
 * `features/updates/utils/updateReportPdf.ts` (the only other PDF export in
 * this app), via the shared `@utils/pdfReportKit` helpers — a colored header
 * band, a "Field/Value" details **table** for the compact case fields
 * (`writeDetailsTable`), and a free-flowing "When · Actor · Type" + full-width
 * paragraph **list** (`writeActivityList`, deliberately NOT a table — a table
 * cell has to fit the single longest wrapped entry, which ballooned the
 * report once a few long comments were involved; reported live) for the
 * activity trail. Every string routes through the kit's `toPdfSafeText` so a
 * stray emoji/invisible character in a display name can't render as garbled
 * text (reported live, e.g. a name coming out as widely letter-spaced
 * gibberish), and rich-text HTML goes through `htmlToPdfPlainText` (not the
 * unrelated `stripHtmlTags`) so a real multi-paragraph/bulleted description
 * still reads as one, with its line breaks intact, instead of every
 * paragraph/bullet mashed into one run-on line (also reported live).
 */
export function generateCaseReportPdf(
  caseDetail: CsmCaseDetail,
  comments: CsmCaseComment[],
  activity: CaseAuditEntry[] = [],
  attachments: CaseAttachment[] = [],
  feedback: CaseFeedbackEntry[] = [],
): void {
  const doc = new jsPDF({ unit: "mm", format: "a4" });
  const cur: PdfCursor = { doc, y: 0 };

  const title = caseDetail.caseNumber ?? caseDetail.wso2CaseId ?? caseDetail.id;
  writeReportHeader(cur, `Case Report — ${title}`, caseDetail.subject);

  const rows: PdfDetailsRow[] = [
    { label: "State", value: stateLabel(caseDetail.state), colorRole: "info" },
    { label: "Severity", value: severityLabel(caseDetail.severity), colorRole: "error" },
    { label: "Type", value: caseTypeTransferLabel(caseDetail.caseType ?? "case") },
    { label: "Customer", value: caseDetail.customer || "—" },
    { label: "Project", value: caseDetail.projectName || "—" },
    { label: "Product", value: caseDetail.product || "—" },
    { label: "Assignee", value: caseDetail.assignee || "Unassigned" },
    { label: "Assignment group", value: caseDetail.assignmentGroup || "—" },
  ];
  if (caseDetail.wso2CaseId) rows.push({ label: "WSO2 case Id", value: caseDetail.wso2CaseId });
  if (caseDetail.createdBy) rows.push({ label: "Created by", value: caseDetail.createdBy });
  rows.push({ label: "Created", value: formatDateTime(caseDetail.createdAt) });
  rows.push({ label: "Last updated", value: formatDateTime(caseDetail.updatedAt) });
  if (caseDetail.escalationLevel && caseDetail.escalationLevel !== "0") {
    rows.push({ label: "Escalation level", value: `EL${caseDetail.escalationLevel}`, colorRole: "warning" });
  }
  if (caseDetail.acknowledgedBy?.name) {
    rows.push({ label: "Acknowledged by", value: caseDetail.acknowledgedBy.name });
  }
  if (caseDetail.workaroundProvidedOn) {
    rows.push({
      label: "Workaround provided",
      value: `${formatDateTime(caseDetail.workaroundProvidedOn)}${
        caseDetail.workaroundProvidedBy?.name ? ` by ${caseDetail.workaroundProvidedBy.name}` : ""
      }`,
    });
  }
  writeDetailsTable(cur, rows);

  if (!isBlankHtml(caseDetail.description)) {
    writeHeading(cur, "Description", 2);
    writeWrapped(cur, condenseBlankLines(htmlToPdfPlainText(caseDetail.description)), { size: 10 });
  }

  // Some backend-synced entries carry no real message at all — just the
  // sync layer's own "Customer comment added" label (a marker the on-screen
  // feed already hides for the same reason, via `hasDisplayableContent`) — so
  // filter those out rather than list them as if they were real comments.
  // `comments` is expected to already include the pre-case chat transcript,
  // merged in by the caller exactly as `CsmCaseDetailPage.tsx`'s own
  // `mergedComments` does — both lanes render through the same
  // `hasDisplayableContent`/`preprocessCommentBodyHtml` pipeline either way
  // (a chatbot entry is just a `CsmCaseComment` with `authorRole: "chatbot"`).
  const displayableComments = comments.filter(hasDisplayableContent);
  type SortableRow = PdfActivityRow & { sortAt: string };
  const commentRows: SortableRow[] = displayableComments.map((c) => ({
    actor: c.authorName,
    when: formatDateTime(c.createdAt),
    sortAt: c.createdAt,
    type: c.internal ? "Internal note" : "Comment",
    internal: c.internal,
    detail: (() => {
      const body = condenseBlankLines(htmlToPdfPlainText(preprocessCommentBodyHtml(c)));
      return body.length > 0 ? body : "(see attached image)";
    })(),
  }));
  const activityRows: SortableRow[] = activity.map((a) => ({
    actor: a.actor,
    when: formatDateTime(a.createdAt),
    sortAt: a.createdAt,
    type: "State change",
    detail: describeAuditEntry(a) || "(no details)",
  }));
  const attachmentRows: SortableRow[] = attachments.map((a) => ({
    actor: a.uploadedBy,
    when: formatDateTime(a.uploadedAt),
    sortAt: a.uploadedAt,
    type: "Attachment",
    detail: `${a.filename} (${formatBytes(a.size)})`,
  }));
  const feedbackRows: SortableRow[] = feedback.map((f) => ({
    actor: f.submitterName ?? "Customer",
    when: formatDateTime(f.submittedAt),
    sortAt: f.submittedAt,
    type: "Feedback",
    detail: f.comment ? `${f.ratingLabel}: ${f.comment}` : f.ratingLabel,
  }));
  // Sort by the raw ISO timestamp, not the already-formatted `when` string —
  // a formatted display string ("Jul 1, 2026, 2:09 PM") doesn't sort
  // chronologically as plain text.
  const allRows = [...commentRows, ...activityRows, ...attachmentRows, ...feedbackRows].sort(
    (a, b) => a.sortAt.localeCompare(b.sortAt),
  );
  // Matches the on-screen Activities tab's own "N entries" badge wording
  // (`CsmCaseDetailPage.tsx`), which counts across these same four lanes.
  writeHeading(cur, `Activity (${allRows.length} entries)`, 2);
  writeActivityList(cur, allRows);

  stampFooterPageNumbers(doc);

  const safeName = safeFileNamePart(title);
  doc.save(`case-${safeName}.pdf`);
}
