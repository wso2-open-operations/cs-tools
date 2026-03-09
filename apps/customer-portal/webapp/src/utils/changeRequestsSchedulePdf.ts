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
import type { ChangeRequestItem, ChangeRequestStats } from "@models/responses";
import { formatImpactLabel } from "@constants/supportConstants";

/**
 * Formats a date string for display in the PDF.
 * Normalizes ServiceNow format to UTC for consistent timezone handling.
 *
 * @param {string | null | undefined} dateStr - Date string in ServiceNow format (YYYY-MM-DD HH:mm:ss) or ISO format.
 * @returns {string} Formatted date string or "N/A".
 */
function formatScheduledDate(dateStr: string | null | undefined): string {
  if (!dateStr) return "N/A";

  // Normalize ServiceNow format (YYYY-MM-DD HH:mm:ss) to ISO format with UTC timezone
  let normalizedDateStr = dateStr;
  if (dateStr.includes(" ")) {
    // Replace space with "T" and append "Z" for UTC if no timezone info exists
    normalizedDateStr = dateStr.replace(" ", "T");
    if (
      !normalizedDateStr.includes("Z") &&
      !normalizedDateStr.includes("+") &&
      !normalizedDateStr.includes("-", 10)
    ) {
      normalizedDateStr += "Z";
    }
  }

  const d = new Date(normalizedDateStr);
  if (isNaN(d.getTime())) return "N/A";

  // Format with explicit UTC timezone for consistency
  return d.toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
    timeZone: "UTC",
  });
}

/**
 * Generates and downloads a Change Requests Schedule PDF.
 *
 * @param {ChangeRequestItem[]} changeRequests - List of change requests.
 * @param {ChangeRequestStats} stats - Statistics for change requests.
 */
export function generateChangeRequestsSchedulePdf(
  changeRequests: ChangeRequestItem[],
  stats: ChangeRequestStats,
): void {
  const doc = new jsPDF();

  // Title
  doc.setFontSize(18);
  doc.text("Change Requests Schedule", 14, 20);

  // Generated date
  const now = new Date();
  const generatedStr = now.toLocaleString("en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  });
  doc.setFontSize(10);
  doc.text(`Generated on: ${generatedStr}`, 14, 28);

  // Summary Statistics
  doc.setFontSize(12);
  doc.text("Summary Statistics", 14, 38);
  doc.setFontSize(10);
  doc.text(`Total Requests: ${stats.totalRequests}`, 14, 45);
  doc.text(`Scheduled: ${stats.scheduled}`, 14, 52);
  doc.text(`In Progress: ${stats.inProgress}`, 14, 59);
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

  // Add footer with WSO2 Support, page numbers, and created date
  const createdDateTime = new Date().toLocaleString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  });
  const totalPages = doc.getNumberOfPages();
  const pageWidth = doc.internal.pageSize.getWidth();

  for (let i = 1; i <= totalPages; i++) {
    doc.setPage(i);
    doc.setFontSize(8);
    const pageHeight = doc.internal.pageSize.getHeight();
    const footerY = pageHeight - 10;

    // Left: WSO2 Support 24x7
    doc.text("WSO2 Support 24x7", 14, footerY);

    // Center: Page numbers with title
    const pageText = `Change Requests Schedule - Page ${i} of ${totalPages}`;
    const textWidth = doc.getTextWidth(pageText);
    doc.text(pageText, (pageWidth - textWidth) / 2, footerY);

    // Right: Created date and time
    const dateWidth = doc.getTextWidth(createdDateTime);
    doc.text(createdDateTime, pageWidth - 14 - dateWidth, footerY);
  }

  // Download the PDF
  const fileName = `Change-Requests-Schedule-${new Date().toISOString().split("T")[0]}.pdf`;
  doc.save(fileName);
}
