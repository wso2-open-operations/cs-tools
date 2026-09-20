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
import {
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentStateColor,
  incidentStateLabel,
} from "@features/csm-operations/utils/incidents";
import {
  hasDisplayableContent,
  preprocessCommentBodyHtml,
} from "@features/csm-cases/utils/commentContent";
import { describeAuditEntry } from "@features/csm-cases/utils/caseActivityFeed";
import type { BeIncidentDetail } from "@api/backend/types";
import type {
  CaseAttachment,
  CaseAuditEntry,
  CsmCaseComment,
} from "@features/csm-cases/types/csmCases";

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "short" }) ?? "—"
  );
}

/**
 * Renders an incident plus its full activity trail — comments, audited
 * state/field changes, and attachments, merged into one chronological list,
 * mirroring exactly what `CsmIncidentDetailPage.tsx` passes to its own
 * on-screen `CaseActivitiesFeed` (`comments`/`audit`/`attachments`; no
 * feedback lane for incidents) — as a standalone, readable PDF report. See
 * `caseReportPdf.ts`'s doc comment for the full rationale (including why
 * every one of the feed's lanes has to be included, not just
 * comments; why HTML content goes through `htmlToPdfPlainText`, not
 * `stripHtmlTags`; and why the activity trail is a free-flowing list —
 * `writeActivityList` — rather than a table); this is the incident-specific
 * counterpart, built on the same shared `@utils/pdfReportKit` (colored
 * header band, a "Field/Value" details table for the compact fields).
 */
export function generateIncidentReportPdf(
  incident: BeIncidentDetail,
  comments: CsmCaseComment[],
  activity: CaseAuditEntry[] = [],
  attachments: CaseAttachment[] = [],
): void {
  const doc = new jsPDF({ unit: "mm", format: "a4" });
  const cur: PdfCursor = { doc, y: 0 };

  const title = incident.number ?? incident.id ?? "incident";
  writeReportHeader(cur, `Incident Report — ${title}`, incident.subject ?? undefined);

  const rows: PdfDetailsRow[] = [
    { label: "State", value: incidentStateLabel(incident.state), colorRole: incidentStateColor(incident.state) },
    {
      label: "Priority",
      value: incidentPriorityLabel(incident.priority),
      colorRole: incidentPriorityColor(incident.priority),
    },
    { label: "Category", value: incident.category ?? "—" },
  ];
  if (incident.subcategory) rows.push({ label: "Subcategory", value: incident.subcategory });
  rows.push({ label: "Caller", value: incident.caller?.name || "—" });
  rows.push({ label: "Service", value: incident.service?.name || "—" });
  if (incident.serviceOffering?.name) {
    rows.push({ label: "Service offering", value: incident.serviceOffering.name });
  }
  if (incident.configurationItem?.name) {
    rows.push({ label: "Configuration item", value: incident.configurationItem.name });
  }
  if (incident.contactType) rows.push({ label: "Contact type", value: incident.contactType });
  if (incident.impact) rows.push({ label: "Impact", value: incident.impact });
  if (incident.urgency) rows.push({ label: "Urgency", value: incident.urgency });
  rows.push({ label: "Assignment group", value: incident.assignmentGroup?.name || "—" });
  rows.push({ label: "Assigned to", value: incident.assignedTo?.name || "Unassigned" });
  if (incident.changeRequest?.name) {
    rows.push({ label: "Related change request", value: incident.changeRequest.name });
  }
  if (incident.problem?.name) rows.push({ label: "Related problem", value: incident.problem.name });
  if (incident.causedBy?.name) rows.push({ label: "Caused by", value: incident.causedBy.name });
  if (incident.createdBy) rows.push({ label: "Created by", value: incident.createdBy });
  rows.push({ label: "Created", value: formatDateTime(incident.createdOn ?? incident.openedOn) });
  rows.push({ label: "Last updated", value: formatDateTime(incident.updatedOn) });
  writeDetailsTable(cur, rows);

  if (incident.additionalComments && !isBlankHtml(incident.additionalComments)) {
    writeHeading(cur, "Additional comments", 2);
    writeWrapped(cur, condenseBlankLines(htmlToPdfPlainText(incident.additionalComments)), { size: 10 });
  }
  if (incident.workNotes && !isBlankHtml(incident.workNotes)) {
    writeHeading(cur, "Work notes", 2);
    writeWrapped(cur, condenseBlankLines(htmlToPdfPlainText(incident.workNotes)), { size: 10 });
  }

  // Some backend-synced entries carry no real message at all — just the
  // sync layer's own "Customer comment added" label (a marker the on-screen
  // feed already hides for the same reason, via `hasDisplayableContent`) — so
  // filter those out rather than list them as if they were real comments.
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
  // Sort by the raw ISO timestamp, not the already-formatted `when` string —
  // a formatted display string ("Jul 1, 2026, 2:09 PM") doesn't sort
  // chronologically as plain text.
  const allRows = [...commentRows, ...activityRows, ...attachmentRows].sort((a, b) =>
    a.sortAt.localeCompare(b.sortAt),
  );
  // Matches the on-screen Activities tab's own "N entries" badge wording.
  writeHeading(cur, `Activity (${allRows.length} entries)`, 2);
  writeActivityList(cur, allRows);

  stampFooterPageNumbers(doc);

  const safeName = safeFileNamePart(title);
  doc.save(`incident-${safeName}.pdf`);
}
