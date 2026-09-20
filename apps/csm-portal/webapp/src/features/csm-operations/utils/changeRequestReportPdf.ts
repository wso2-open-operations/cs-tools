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
import {
  changeRequestImpactColor,
  changeRequestImpactLabel,
  changeRequestStateColor,
  changeRequestStateLabel,
} from "@features/csm-operations/utils/changeRequests";
import {
  hasDisplayableContent,
  preprocessCommentBodyHtml,
} from "@features/csm-cases/utils/commentContent";
import type { BeChangeRequestDetail } from "@api/backend/types";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "short" }) ?? "—"
  );
}

/** A rich-text plan field, rendered only when it has real (non-blank) content. */
function writePlanSection(cur: PdfCursor, title: string, html?: string | null): void {
  if (!html || isBlankHtml(html)) return;
  writeHeading(cur, title, 2);
  writeWrapped(cur, condenseBlankLines(htmlToPdfPlainText(html)), { size: 10 });
}

/**
 * Renders a change request plus its full comment trail as a standalone,
 * readable PDF report — see `caseReportPdf.ts`'s doc comment for the full
 * rationale (including why HTML content goes through
 * `htmlToPdfPlainText`, not `stripHtmlTags`, and why the comment trail is a
 * free-flowing list — `writeActivityList` — rather than a table); this is
 * the change-request-specific counterpart, built on the same shared
 * `@utils/pdfReportKit` (colored header band, a "Field/Value" details table
 * for the compact fields). Unlike the case/incident counterparts, there is
 * no separate state/field-change audit endpoint for change requests today
 * (`CsmChangeRequestDetailPage.tsx` itself passes `audit={[]}` to its own
 * on-screen `CaseActivitiesFeed`) — this report is comments-only because
 * that's genuinely everything the API has for a change request's activity.
 */
export function generateChangeRequestReportPdf(
  cr: BeChangeRequestDetail,
  comments: CsmCaseComment[],
): void {
  const doc = new jsPDF({ unit: "mm", format: "a4" });
  const cur: PdfCursor = { doc, y: 0 };

  const title = cr.number ?? cr.id;
  writeReportHeader(cur, `Change Request Report — ${title}`, cr.subject ?? undefined);

  const rows: PdfDetailsRow[] = [
    { label: "State", value: changeRequestStateLabel(cr.state), colorRole: changeRequestStateColor(cr.state) },
    { label: "Impact", value: changeRequestImpactLabel(cr.impact), colorRole: changeRequestImpactColor(cr.impact) },
    { label: "Type", value: cr.type ?? "—" },
  ];
  if (cr.category?.label) rows.push({ label: "Category", value: cr.category.label });
  if (cr.priority?.label) rows.push({ label: "Priority", value: cr.priority.label });
  rows.push({ label: "Project", value: cr.project?.name || "—" });
  rows.push({ label: "Linked case", value: cr.case?.name || "—" });
  if (cr.deployment?.name) rows.push({ label: "Deployment", value: cr.deployment.name });
  if (cr.deployedProduct?.name) rows.push({ label: "Deployed product", value: cr.deployedProduct.name });
  rows.push({ label: "Assigned engineer", value: cr.assignedEngineer?.name || "Unassigned" });
  rows.push({ label: "Assigned team", value: cr.assignedTeam?.name || "—" });
  if (cr.requestedBy?.name) rows.push({ label: "Requested by", value: cr.requestedBy.name });
  rows.push({ label: "Planned start", value: formatDateTime(cr.plannedStartOn) });
  rows.push({ label: "Planned end", value: formatDateTime(cr.plannedEndOn) });
  if (cr.duration) rows.push({ label: "Duration", value: cr.duration });
  if (cr.affectedServicesText) rows.push({ label: "Affected services", value: cr.affectedServicesText });
  if (cr.affectedComponentsText) {
    rows.push({ label: "Affected components", value: cr.affectedComponentsText });
  }
  if (cr.hasCustomerApproved !== undefined) {
    rows.push({ label: "Customer approved", value: cr.hasCustomerApproved ? "Yes" : "No" });
  }
  if (cr.approvedBy?.name) {
    rows.push({
      label: "Approved by",
      value: `${cr.approvedBy.name}${cr.approvedOn ? ` on ${formatDateTime(cr.approvedOn)}` : ""}`,
    });
  }
  if (cr.createdBy) rows.push({ label: "Created by", value: cr.createdBy });
  rows.push({ label: "Created", value: formatDateTime(cr.createdOn) });
  rows.push({ label: "Last updated", value: formatDateTime(cr.updatedOn) });
  writeDetailsTable(cur, rows);

  writePlanSection(cur, "Description", cr.description);
  writePlanSection(cur, "Justification", cr.justification);
  writePlanSection(cur, "Impact description", cr.impactDescription);
  writePlanSection(cur, "Implementation plan", cr.implementationPlan);
  writePlanSection(cur, "Test plan", cr.testPlan);
  writePlanSection(cur, "Rollback plan", cr.rollbackPlan);
  writePlanSection(cur, "Communication plan", cr.communicationPlan);
  writePlanSection(cur, "Service outage", cr.serviceOutage);

  // Some backend-synced entries carry no real message at all — just the
  // sync layer's own "Customer comment added" label (a marker the on-screen
  // feed already hides for the same reason, via `hasDisplayableContent`) — so
  // filter those out rather than list them as if they were real comments.
  const displayable = comments.filter(hasDisplayableContent);
  writeHeading(cur, `Comments (${displayable.length})`, 2);
  const sorted = [...displayable].sort(
    (a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime(),
  );
  const commentRows: PdfActivityRow[] = sorted.map((c) => ({
    actor: c.authorName,
    when: formatDateTime(c.createdAt),
    type: c.internal ? "Internal note" : "Comment",
    internal: c.internal,
    detail: (() => {
      const body = condenseBlankLines(htmlToPdfPlainText(preprocessCommentBodyHtml(c)));
      return body.length > 0 ? body : "(see attached image)";
    })(),
  }));
  writeActivityList(cur, commentRows);

  stampFooterPageNumbers(doc);

  const safeName = safeFileNamePart(title);
  doc.save(`change-request-${safeName}.pdf`);
}
