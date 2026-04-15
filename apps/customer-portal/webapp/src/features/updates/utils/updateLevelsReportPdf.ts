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
import type { UpdateLevelsSearchResponse } from "@features/updates/types/updates";

export interface UpdateLevelsReportParams {
  productName: string;
  productVersion: string;
  startLevel: number;
  endLevel: number;
  data: UpdateLevelsSearchResponse;
}

export interface UpdateLevelsReportTableRow {
  levelKey: string;
  typeLabel: string;
  updatesCount: number;
  releaseDate: string;
  applied: string;
}

export interface UpdateLevelsReportData {
  generatedStr: string;
  productName: string;
  productVersion: string;
  startLevel: number;
  endLevel: number;
  securityCount: number;
  regularCount: number;
  mixedCount: number;
  totalUpdates: number;
  levelCount: number;
  levelsRange: string;
  tableRows: UpdateLevelsReportTableRow[];
}

/**
 * Formats a Unix timestamp (ms) as "MMM DD, YYYY" in UTC.
 *
 * @param {number} ts - Timestamp in milliseconds.
 * @returns {string} Formatted date string.
 */
function formatReleaseDate(ts: number): string {
  const d = new Date(ts);
  const months = [
    "Jan", "Feb", "Mar", "Apr", "May", "Jun",
    "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
  ];
  const m = months[d.getUTCMonth()];
  const day = d.getUTCDate();
  const year = d.getUTCFullYear();
  return `${m} ${day}, ${year}`;
}

/**
 * Computes structured report data for the Update Levels Report.
 * Used by UpdateLevelsReportModal for in-app display.
 *
 * @param {UpdateLevelsReportParams} params - Report parameters and search data.
 * @returns {UpdateLevelsReportData} Structured report data.
 */
export function getUpdateLevelsReportData(params: UpdateLevelsReportParams): UpdateLevelsReportData {
  const { productName, productVersion, startLevel, endLevel, data } = params;

  const entries = Object.entries(data).sort(
    ([a], [b]) => Number(a) - Number(b),
  );

  if (entries.length === 0) {
    throw new Error("No update data to generate report.");
  }

  const now = new Date();
  const generatedStr = now.toLocaleString("en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  });

  const securityCount = entries.filter(([, e]) => e.updateType === "security").length;
  const regularCount = entries.filter(([, e]) => e.updateType === "regular").length;
  const mixedCount = entries.filter(([, e]) => e.updateType === "mixed").length;
  const totalUpdates = entries.reduce(
    (sum, [, e]) => sum + e.updateDescriptionLevels.length,
    0,
  );

  const levelCount = entries.length;
  const levelsRange =
    levelCount === 1
      ? `${entries[0][0]}`
      : `${entries[0][0]} - ${entries[entries.length - 1][0]}`;

  const tableRows: UpdateLevelsReportTableRow[] = entries.map(([levelKey, entry]) => {
    const firstDesc = entry.updateDescriptionLevels[0];
    const releaseDate = firstDesc?.timestamp
      ? formatReleaseDate(firstDesc.timestamp)
      : "N/A";
    const typeLabel =
      entry.updateType.charAt(0).toUpperCase() + entry.updateType.slice(1);
    return {
      levelKey,
      typeLabel,
      updatesCount: entry.updateDescriptionLevels.length,
      releaseDate,
      applied: "N/A",
    };
  });

  return {
    generatedStr,
    productName,
    productVersion,
    startLevel,
    endLevel,
    securityCount,
    regularCount,
    mixedCount,
    totalUpdates,
    levelCount,
    levelsRange,
    tableRows,
  };
}

/**
 * Generates and downloads an Update Levels Report PDF using jspdf and jspdf-autotable.
 *
 * @param {UpdateLevelsReportData} reportData - Structured report data from getUpdateLevelsReportData.
 */
export function generateUpdateLevelsReportPdf(reportData: UpdateLevelsReportData): void {
  const doc = new jsPDF();

  doc.setFontSize(18);
  doc.text("Update Levels Report", 14, 20);

  doc.setFontSize(10);
  doc.text(`Generated on: ${reportData.generatedStr}`, 14, 28);

  doc.setFontSize(12);
  doc.text("Search Criteria", 14, 38);
  doc.setFontSize(10);
  doc.text(`Product: ${reportData.productName}`, 14, 45);
  doc.text(`Version: ${reportData.productVersion}`, 14, 52);
  doc.text(`Update Level Range: ${reportData.startLevel} to ${reportData.endLevel}`, 14, 59);
  doc.text("View Mode: All Updates", 14, 66);
  doc.text(`Total Updates: ${reportData.totalUpdates}`, 14, 76);
  doc.text(`Update Levels: ${reportData.levelCount} levels (${reportData.levelsRange})`, 14, 83);

  doc.setFontSize(12);
  doc.text("Update Levels Summary", 14, 95);
  doc.setFontSize(10);
  doc.text(`Security Updates: ${reportData.securityCount}`, 14, 102);
  doc.text(`Regular Updates: ${reportData.regularCount}`, 14, 109);
  doc.text(`Mixed Updates: ${reportData.mixedCount}`, 14, 116);
  doc.text("Applied Updates: N/A", 14, 123);
  doc.text(`Pending Updates: ${reportData.levelCount}`, 14, 130);

  doc.setFontSize(12);
  doc.text("Update Levels Details", 14, 142);

  const tableData = reportData.tableRows.map((row) => [
    row.levelKey,
    row.typeLabel,
    String(row.updatesCount),
    row.releaseDate,
    row.applied,
  ]);

  autoTable(doc, {
    startY: 148,
    head: [["Level", "Type", "Updates", "Release Date", "Applied"]],
    body: tableData,
    theme: "grid",
    headStyles: { fillColor: [100, 100, 100], fontSize: 9 },
    bodyStyles: { fontSize: 9 },
    columnStyles: {
      0: { cellWidth: 20 },
      1: { cellWidth: 35 },
      2: { cellWidth: 25 },
      3: { cellWidth: 45 },
      4: { cellWidth: 25 },
    },
    margin: { left: 14 },
  });

  const totalPages = doc.getNumberOfPages();
  for (let i = 1; i <= totalPages; i++) {
    doc.setPage(i);
    doc.setFontSize(8);
    doc.text(
      `${reportData.productName} ${reportData.productVersion} - Update Levels Report Page ${i} of ${totalPages}`,
      14,
      doc.internal.pageSize.getHeight() - 10,
    );
  }

  const fileName = `Update-Levels-Report-${reportData.productName}-${reportData.productVersion}-${reportData.startLevel}-${reportData.endLevel}.pdf`;
  doc.save(fileName);
}
