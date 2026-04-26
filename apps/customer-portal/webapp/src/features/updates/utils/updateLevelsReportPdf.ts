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
import type {
  SecurityAdvisory,
  UpdateDescriptionEntry,
  UpdateLevelsReportData,
  UpdateLevelsReportParams,
  UpdateLevelsReportTableRow,
} from "@features/updates/types/updates";
import { resolveDisplayTimeZone } from "@utils/dateTime";

export type {
  UpdateLevelsReportData,
  UpdateLevelsReportParams,
  UpdateLevelsReportTableRow,
};

/**
 * Formats a Unix timestamp (ms) as "MMM DD, YYYY" in UTC.
 *
 * @param {number} ts - Timestamp in milliseconds.
 * @returns {string} Formatted date string.
 */
function formatReleaseDate(ts: number): string {
  const d = new Date(ts);
  const months = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"];
  return `${months[d.getUTCMonth()]} ${d.getUTCDate()}, ${d.getUTCFullYear()}`;
}

function formatUpdateDate(ts: number): string {
  const d = new Date(ts);
  const year = d.getUTCFullYear();
  const month = String(d.getUTCMonth() + 1).padStart(2, "0");
  const day = String(d.getUTCDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function isInstructionsNonEmpty(text: string | null | undefined): boolean {
  if (!text) return false;
  const t = text.trim().toLowerCase();
  return t !== "" && t !== "n/a" && t !== "na";
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

  const allEntries: UpdateDescriptionEntry[] = entries
    .flatMap(([levelKey, entry]) =>
      entry.updateDescriptionLevels.map((desc) => ({
        ...desc,
        levelKey,
        formattedDate: desc.timestamp ? formatUpdateDate(desc.timestamp) : "N/A",
      })),
    )
    .sort((a, b) => a.timestamp - b.timestamp);

  const securityEntries = allEntries.filter(
    (e) => e.updateType === "security" || (e.securityAdvisories?.length ?? 0) > 0,
  );

  const entriesWithInstructions = allEntries.filter((e) =>
    isInstructionsNonEmpty(e.instructions),
  );

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
    allEntries,
    securityEntries,
    entriesWithInstructions,
  };
}

/**
 * Generates and downloads an Update Summary PDF in WSO2 format using jspdf.
 * Bullet lists link internally to their update blocks; advisory IDs link to the
 * Security Advisories section at the end of the document.
 *
 * @param {UpdateLevelsReportData} reportData - Structured report data from getUpdateLevelsReportData.
 */
export function generateUpdateLevelsReportPdf(reportData: UpdateLevelsReportData): void {
  const doc = new jsPDF();
  const pageW = doc.internal.pageSize.getWidth();
  const pageH = doc.internal.pageSize.getHeight();
  const margin = 14;
  const contentW = pageW - 2 * margin;
  let y = 20;
  let pg = 1;

  const addPage = (): void => {
    doc.addPage();
    pg++;
    y = 20;
  };

  const ensureSpace = (needed: number): void => {
    if (y + needed > pageH - 20) {
      addPage();
    }
  };

  // Deferred internal link annotations (applied after target pages are known)
  type DeferredLink = { srcPage: number; x: number; y: number; w: number; h: number; key: string };
  const pendingUpdateLinks: DeferredLink[] = [];
  const pendingAdvisoryLinks: DeferredLink[] = [];
  const updateTargetPages = new Map<string, number>();  // updateNumber -> page
  const advisoryTargetPages = new Map<string, number>(); // advisoryId -> page

  // Render blue underlined text and record a deferred link annotation.
  const recordLinkText = (
    pending: DeferredLink[],
    text: string,
    x: number,
    baseY: number,
    key: string,
  ): void => {
    const w = doc.getTextWidth(text);
    doc.setTextColor(0, 102, 204);
    doc.text(text, x, baseY);
    doc.setDrawColor(0, 102, 204);
    doc.line(x, baseY + 0.8, x + w, baseY + 0.8);
    doc.setDrawColor(0, 0, 0);
    doc.setTextColor(0, 0, 0);
    pending.push({ srcPage: pg, x, y: baseY - 4, w, h: 6, key });
  };

  // Render a labelled field, skipping if value is empty.
  const renderField = (label: string, value: string | null | undefined): void => {
    if (!value?.trim()) return;
    ensureSpace(14);
    doc.setFontSize(9);
    doc.setFont("helvetica", "bold");
    doc.setTextColor(0, 0, 0);
    doc.text(`${label}:`, margin, y);
    y += 5;
    doc.setFont("helvetica", "normal");
    const lines = doc.splitTextToSize(value.trim(), contentW - 4);
    ensureSpace(lines.length * 5 + 3);
    doc.text(lines, margin + 4, y);
    y += lines.length * 5 + 5;
  };

  // ===== TITLE =====
  const titleStr = `Update Summary between ${reportData.productName}-${reportData.productVersion}.${reportData.startLevel} and ${reportData.productName}-${reportData.productVersion}.${reportData.endLevel}`;
  doc.setFontSize(14);
  doc.setFont("helvetica", "bold");
  doc.setTextColor(0, 0, 0);
  const titleLines = doc.splitTextToSize(titleStr, contentW);
  doc.text(titleLines, margin, y);
  y += titleLines.length * 7 + 5;

  doc.setFontSize(9);
  doc.setFont("helvetica", "normal");
  doc.setTextColor(100, 100, 100);
  doc.text(`Generated on: ${reportData.generatedStr}`, margin, y);
  y += 7;

  const secCount = reportData.securityEntries.length;
  const intro = `This document summarizes the updates released for ${reportData.productName} ${reportData.productVersion} between update levels ${reportData.startLevel} and ${reportData.endLevel}. There are a total of ${reportData.totalUpdates} updates in this range${secCount > 0 ? `, of which ${secCount} are security updates` : ""}. It is highly recommended that you apply all these updates.`;
  doc.setFontSize(10);
  doc.setFont("helvetica", "normal");
  doc.setTextColor(60, 60, 60);
  const introLines = doc.splitTextToSize(intro, contentW);
  ensureSpace(introLines.length * 5 + 6);
  doc.text(introLines, margin, y);
  y += introLines.length * 5 + 8;

  // ===== LIST OF SECURITY UPDATES =====
  if (reportData.securityEntries.length > 0) {
    ensureSpace(22);
    doc.setFontSize(12);
    doc.setFont("helvetica", "bold");
    doc.setTextColor(0, 0, 0);
    doc.text("List of security updates", margin, y);
    y += 8;

    doc.setFontSize(10);
    doc.setFont("helvetica", "normal");
    for (const entry of reportData.securityEntries) {
      ensureSpace(8);
      const text = `• ${entry.formattedDate} (Update No: ${entry.updateNumber})`;
      recordLinkText(pendingUpdateLinks, text, margin + 4, y, String(entry.updateNumber));
      y += 6;
    }
    y += 6;
  }

  // ===== FOLLOWING UPDATES WITH INSTRUCTIONS =====
  if (reportData.entriesWithInstructions.length > 0) {
    ensureSpace(20);
    doc.setFontSize(10);
    doc.setFont("helvetica", "normal");
    doc.setTextColor(60, 60, 60);
    const instrIntroLines = doc.splitTextToSize(
      "Following updates contain instructions that must be followed when applying the update.",
      contentW,
    );
    doc.text(instrIntroLines, margin, y);
    y += instrIntroLines.length * 5 + 5;

    doc.setFontSize(10);
    doc.setFont("helvetica", "normal");
    for (const entry of reportData.entriesWithInstructions) {
      ensureSpace(8);
      const text = `• ${entry.formattedDate} (Update No: ${entry.updateNumber})`;
      recordLinkText(pendingUpdateLinks, text, margin + 4, y, String(entry.updateNumber));
      y += 6;
    }
    y += 8;
  }

  // ===== INDIVIDUAL UPDATE BLOCKS =====
  for (const entry of reportData.allEntries) {
    const headerText = `${entry.formattedDate} - Update No: ${entry.updateNumber}`;
    const subText = `${entry.productName}-${entry.productVersion} (${entry.channel})`;

    doc.setFontSize(11);
    doc.setFont("helvetica", "bold");
    const headerLines = doc.splitTextToSize(headerText, contentW - 6);
    const boxH = (headerLines.length + 1) * 6 + 6;
    ensureSpace(boxH + 14);

    // Record this page as the jump target for bullet list links
    updateTargetPages.set(String(entry.updateNumber), pg);

    doc.setFillColor(255, 115, 0);
    doc.rect(margin, y, contentW, boxH, "F");
    doc.setTextColor(255, 255, 255);
    doc.text(headerLines, margin + 3, y + 6);
    doc.setFontSize(9);
    doc.setFont("helvetica", "normal");
    doc.text(subText, margin + 3, y + headerLines.length * 6 + 6);
    y += boxH + 4;

    renderField("General Description", entry.description);
    renderField("Implementation Details", entry.bugFixes);

    // Impact (aggregated from security advisories)
    const advisoriesWithImpact = entry.securityAdvisories?.filter((a) => a.impact?.trim());
    if (advisoriesWithImpact && advisoriesWithImpact.length > 0) {
      ensureSpace(14);
      doc.setFontSize(9);
      doc.setFont("helvetica", "bold");
      doc.setTextColor(0, 0, 0);
      doc.text("Impact:", margin, y);
      y += 5;
      doc.setFont("helvetica", "normal");
      for (const adv of advisoriesWithImpact) {
        const impLines = doc.splitTextToSize(adv.impact.trim(), contentW - 4);
        ensureSpace(impLines.length * 5 + 3);
        doc.text(impLines, margin + 4, y);
        y += impLines.length * 5 + 3;
      }
      y += 2;
    }

    // Instructions
    if (isInstructionsNonEmpty(entry.instructions)) {
      ensureSpace(20);
      doc.setFontSize(9);
      doc.setFont("helvetica", "bold");
      doc.setTextColor(0, 0, 0);
      doc.text("Instructions:", margin, y);
      y += 5;

      doc.setFont("courier", "normal");
      const instrLines = doc.splitTextToSize(entry.instructions!.trim(), contentW - 8);
      const instrBoxH = instrLines.length * 5 + 8;
      ensureSpace(instrBoxH + 4);

      doc.setFillColor(245, 245, 245);
      doc.setDrawColor(200, 200, 200);
      doc.rect(margin, y, contentW, instrBoxH, "FD");
      doc.setTextColor(0, 0, 0);
      doc.text(instrLines, margin + 4, y + 5);
      y += instrBoxH + 5;
      doc.setFont("helvetica", "normal");
      doc.setDrawColor(0, 0, 0);
    }

    // Security advisory IDs — internal links to the Security Advisories section
    if ((entry.securityAdvisories?.length ?? 0) > 0) {
      ensureSpace(15);
      doc.setFontSize(9);
      doc.setFont("helvetica", "bold");
      doc.setTextColor(0, 0, 0);
      doc.text("Security Advisories:", margin, y);
      y += 5;
      doc.setFont("helvetica", "normal");
      for (const adv of entry.securityAdvisories) {
        ensureSpace(8);
        recordLinkText(pendingAdvisoryLinks, adv.id, margin + 4, y, adv.id);
        y += 5.5;
      }
      y += 3;
    }

    y += 8;
  }

  // ===== SECURITY ADVISORIES SECTION (bottom of document) =====
  const allAdvisories = new Map<string, SecurityAdvisory>();
  for (const entry of reportData.allEntries) {
    for (const adv of entry.securityAdvisories ?? []) {
      if (!allAdvisories.has(adv.id)) {
        allAdvisories.set(adv.id, adv);
      }
    }
  }

  if (allAdvisories.size > 0) {
    ensureSpace(25);
    doc.setFontSize(14);
    doc.setFont("helvetica", "bold");
    doc.setTextColor(0, 0, 0);
    doc.text("Security Advisories", margin, y);
    y += 10;

    for (const [, adv] of allAdvisories) {
      const advHeader = `${adv.id}  —  Severity: ${adv.severity}`;
      doc.setFontSize(11);
      doc.setFont("helvetica", "bold");
      const advHeaderLines = doc.splitTextToSize(advHeader, contentW - 6);
      const advBoxH = advHeaderLines.length * 6 + 8;
      ensureSpace(advBoxH + 10);

      // Record this page as the jump target for advisory links in update blocks
      advisoryTargetPages.set(adv.id, pg);

      doc.setFillColor(140, 0, 0);
      doc.rect(margin, y, contentW, advBoxH, "F");
      doc.setTextColor(255, 255, 255);
      doc.text(advHeaderLines, margin + 3, y + 6);
      y += advBoxH + 4;

      renderField("Overview", adv.overview);
      renderField("Description", adv.description);
      renderField("Impact", adv.impact);
      renderField("Solution", adv.solution);
      renderField("Notes", adv.notes);
      renderField("Credits", adv.credits);

      y += 6;
    }
  }

  // ===== APPLY DEFERRED INTERNAL LINK ANNOTATIONS =====
  for (const link of pendingUpdateLinks) {
    const targetPage = updateTargetPages.get(link.key);
    if (targetPage !== undefined) {
      doc.setPage(link.srcPage);
      doc.link(link.x, link.y, link.w, link.h, { pageNumber: targetPage });
    }
  }
  for (const link of pendingAdvisoryLinks) {
    const targetPage = advisoryTargetPages.get(link.key);
    if (targetPage !== undefined) {
      doc.setPage(link.srcPage);
      doc.link(link.x, link.y, link.w, link.h, { pageNumber: targetPage });
    }
  }

  // ===== PAGE NUMBERS =====
  const totalPages = doc.getNumberOfPages();
  for (let i = 1; i <= totalPages; i++) {
    doc.setPage(i);
    doc.setFontSize(8);
    doc.setFont("helvetica", "normal");
    doc.setTextColor(100, 100, 100);
    doc.text(
      `${reportData.productName} ${reportData.productVersion} - Update Summary  |  Page ${i} of ${totalPages}`,
      margin,
      pageH - 10,
    );
  }

  const fileName = `Update-Summary-${reportData.productName}-${reportData.productVersion}-${reportData.startLevel}-${reportData.endLevel}.pdf`;
  doc.save(fileName);
}
