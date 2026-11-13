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
import autoTable from "jspdf-autotable";
import type {
  ChangeRequestItem,
  ChangeRequestStats,
} from "@features/operations/types/changeRequests";
import { formatImpactLabel } from "@features/operations/utils/changeRequestUi";
import { formatBackendTimestampForDisplay, resolveDisplayTimeZone } from "@utils/dateTime";
import { drawReportSupportFooters, loadReportLogo } from "@utils/pdfBranding";

/**
 * Formats a date string for display in the PDF.
 * Normalizes ServiceNow format to UTC for consistent timezone handling.
 *
 * @param {string | null | undefined} dateStr - Date string in ServiceNow format (YYYY-MM-DD HH:mm:ss) or ISO format.
 * @returns {string} Formatted date string or "N/A".
 */
function formatScheduledDate(dateStr: string | null | undefined): string {
  if (!dateStr) return "N/A";
  const formatted = formatBackendTimestampForDisplay(dateStr, {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  });
  return formatted ?? "N/A";
}

/**
 * Generates and downloads a Change Requests Schedule PDF. The footer carries the
 * configured logo and company name ("WSO2 Support 24x7" when none are set).
 *
 * @param {ChangeRequestItem[]} changeRequests - List of change requests.
 * @param {ChangeRequestStats} stats - Statistics for change requests.
 * @returns {Promise<void>} Resolves once the PDF is saved.
 */
export async function generateChangeRequestsSchedulePdf(
  changeRequests: ChangeRequestItem[],
  stats: ChangeRequestStats,
): Promise<void> {
  const logo = await loadReportLogo();
  const doc = new jsPDF();

  // Title
  doc.setFontSize(18);
  doc.text("Change Requests Schedule", 14, 20);

  // Generated date
  const now = new Date();
  const displayTimeZone = resolveDisplayTimeZone();
  const generatedStr = now.toLocaleString("en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
    timeZone: displayTimeZone,
  });
  doc.setFontSize(10);
  doc.text(`Generated on: ${generatedStr}`, 14, 28);

  // Summary Statistics
  doc.setFontSize(12);
  doc.text("Summary Statistics", 14, 38);
  doc.setFontSize(10);
  doc.text(`Total Requests: ${stats.totalRequests}`, 14, 45);
  doc.text(`Awaiting Your Action: ${stats.awaitingYourAction}`, 14, 52);
  doc.text(`Ongoing: ${stats.ongoing}`, 14, 59);
  doc.text(`Completed: ${stats.completed}`, 14, 66);

  // Change Requests Details
  doc.setFontSize(12);
  doc.text("Change Requests Details", 14, 78);

  // Prepare table data
  const tableData = changeRequests.map((cr) => [
    cr.number || "N/A",
    cr.title || "N/A",
    cr.state?.label || "N/A",
    cr.impact?.label ? formatImpactLabel(cr.impact.label) : "N/A",
    formatScheduledDate(cr.startDate),
    cr.deployment?.label || "N/A",
  ]);

  // Generate table
  autoTable(doc, {
    startY: 84,
    head: [
      [
        "CR Number",
        "Title",
        "Status",
        "Impact",
        "Scheduled Date",
        "Deployment",
      ],
    ],
    body: tableData,
    theme: "grid",
    headStyles: { fillColor: [100, 100, 100], fontSize: 9 },
    bodyStyles: { fontSize: 9 },
    columnStyles: {
      0: { cellWidth: 25 },
      1: { cellWidth: 50 },
      2: { cellWidth: 25 },
      3: { cellWidth: 20 },
      4: { cellWidth: 40 },
      5: { cellWidth: 30 },
    },
    margin: { left: 14 },
  });

  // Footer: logo and "<company> Support 24x7", page numbers, and created date
  const createdDateTime = new Date().toLocaleString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
    timeZone: displayTimeZone,
  });
  drawReportSupportFooters(
    doc,
    logo,
    (page, totalPages) => `Change Requests Schedule - Page ${page} of ${totalPages}`,
    createdDateTime,
  );

  // Download the PDF
  const fileName = `Change-Requests-Schedule-${new Date().toISOString().split("T")[0]}.pdf`;
  doc.save(fileName);
}
