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

export type DescriptionSections = {
  generalDescription: string;
  implementationDetails: string;
  impact: string;
};

/**
 * Parses a description string that may contain embedded section labels
 * ("General Description :", "Implementation details :", "Impact :") and
 * returns each section separately. Falls back to treating the whole string
 * as the general description when no labels are found.
 */
export function parseDescriptionSections(raw: string | null | undefined): DescriptionSections {
  const empty: DescriptionSections = { generalDescription: "", implementationDetails: "", impact: "" };
  if (!raw?.trim() || raw.trim().toLowerCase() === "n/a") return empty;
  raw = stripHtmlTags(raw);

  const headerRe = /(?:^|\n)\s*(General\s+Description|Implementation\s+Details?|Impact)\s*:/gi;
  if (!headerRe.test(raw)) {
    return { ...empty, generalDescription: raw.trim() };
  }
  headerRe.lastIndex = 0;

  const hits: Array<{ label: string; headerStart: number; contentStart: number }> = [];
  let m: RegExpExecArray | null;
  while ((m = headerRe.exec(raw)) !== null) {
    hits.push({
      label: m[1].replace(/\s+/g, "").toLowerCase(),
      headerStart: m.index,
      contentStart: m.index + m[0].length,
    });
  }

  const result = { ...empty };
  for (let i = 0; i < hits.length; i++) {
    const { label, contentStart } = hits[i];
    const end = i + 1 < hits.length ? hits[i + 1].headerStart : raw.length;
    const content = raw.slice(contentStart, end).trim();
    if (label === "generaldescription") result.generalDescription = content;
    else if (label.startsWith("implementationdetail")) result.implementationDetails = content;
    else if (label === "impact") result.impact = content;
  }
  return result;
}

/**
 * Parses a JSON-encoded file list string into a deduplicated, sorted array of file paths.
 * Handles both JSON array format and plain comma-separated strings.
 */
export function parseFileList(raw: string | null | undefined): string[] {
  if (!raw?.trim() || raw.trim() === "[]") return [];
  const stripBrackets = (s: string) => s.replace(/^\[+|\]+$/g, "").trim();
  try {
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) {
      return [...new Set(
        parsed.map((f: unknown) => stripBrackets(String(f))).filter(Boolean),
      )].sort();
    }
    if (typeof parsed === "string") {
      return [...new Set(
        stripBrackets(parsed).split(",").map((f) => f.trim()).filter(Boolean),
      )].sort();
    }
  } catch {
    // not JSON — try comma-split after stripping outer brackets
  }
  const stripped = stripBrackets(raw);
  return [...new Set(
    stripped.split(",").map((f) => f.trim()).filter(Boolean),
  )].sort();
}

/**
 * Strips HTML tags and decodes common HTML entities, returning plain text
 * suitable for PDF rendering. Block-level closing tags are replaced with
 * newlines first so that section boundaries are preserved for parsers like
 * parseDescriptionSections that rely on newline-anchored regex matching.
 */
export function stripHtmlTags(html: string): string {
  return html
    .replace(/<\/(p|div|li|h[1-6]|blockquote|tr)>/gi, "\n")
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<[^>]+>/g, "")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&nbsp;/g, " ")
    .replace(/[ \t]{2,}/g, " ")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

type TextRun = { text: string; bold: boolean; italic: boolean };
type PdfBlock = { runs: TextRun[]; isList?: boolean };

/** Parses an HTML string into paragraph-level blocks with styled inline runs. */
export function htmlToBlocks(html: string): PdfBlock[] {
  const parser = new DOMParser();
  const parsed = parser.parseFromString(`<body>${html}</body>`, "text/html");

  const BLOCK_TAGS = new Set(["p", "div", "li", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote"]);

  function collectRuns(node: Node, bold: boolean, italic: boolean): TextRun[] {
    if (node.nodeType === Node.TEXT_NODE) {
      const text = (node.textContent ?? "").replace(/\n/g, " ");
      if (!text.trim()) return [];
      return [{ text, bold, italic }];
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return [];
    const el = node as Element;
    const tag = el.tagName.toLowerCase();
    if (tag === "br") return [{ text: "\n", bold: false, italic: false }];
    const nextBold = bold || tag === "strong" || tag === "b";
    const nextItalic = italic || tag === "em" || tag === "i";
    const runs: TextRun[] = [];
    for (const child of el.childNodes) {
      runs.push(...collectRuns(child, nextBold, nextItalic));
    }
    return runs;
  }

  const blocks: PdfBlock[] = [];

  function walkTop(node: Node): void {
    if (node.nodeType === Node.ELEMENT_NODE) {
      const el = node as Element;
      const tag = el.tagName.toLowerCase();
      if (BLOCK_TAGS.has(tag)) {
        const runs = collectRuns(el, false, false);
        if (runs.some((r) => r.text.trim())) {
          blocks.push({ runs, isList: tag === "li" });
        }
      } else {
        for (const child of el.childNodes) walkTop(child);
      }
    } else if (node.nodeType === Node.TEXT_NODE) {
      const text = (node.textContent ?? "").trim();
      if (text) blocks.push({ runs: [{ text, bold: false, italic: false }] });
    }
  }

  for (const child of parsed.body.childNodes) walkTop(child);
  return blocks;
}

export function isSafeHttpUrl(url: string): boolean {
  try {
    const { protocol } = new URL(url);
    return protocol === "http:" || protocol === "https:";
  } catch {
    return false;
  }
}

/**
 * Parses the bugFixes field, which may be a JSON-encoded array of URL strings
 * or a plain string, filtering out "N/A" values and non-http(s) schemes.
 */
export function parseBugFixes(raw: string | null | undefined): string[] {
  if (!raw?.trim()) return [];
  try {
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) {
      return parsed
        .map((item) => String(item).trim())
        .filter((item) => item && item.toLowerCase() !== "n/a" && isSafeHttpUrl(item));
    }
  } catch {
    // not JSON
  }
  const t = raw.trim();
  return t && t.toLowerCase() !== "n/a" && isSafeHttpUrl(t) ? [t] : [];
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

  // Render a labelled list of URLs as blue underlined external hyperlinks.
  const renderUrlListField = (label: string, urls: string[]): void => {
    if (urls.length === 0) return;
    ensureSpace(14);
    doc.setFontSize(9);
    doc.setFont("helvetica", "bold");
    doc.setTextColor(0, 0, 0);
    doc.text(`${label}:`, margin, y);
    y += 5;
    doc.setFont("helvetica", "normal");
    for (const url of urls) {
      const urlLines = doc.splitTextToSize(url, contentW - 4);
      ensureSpace(urlLines.length * 5 + 3);
      for (const line of urlLines as string[]) {
        const lineW = doc.getTextWidth(line);
        doc.setTextColor(0, 102, 204);
        doc.text(line, margin + 4, y);
        doc.setDrawColor(0, 102, 204);
        doc.line(margin + 4, y + 0.8, margin + 4 + lineW, y + 0.8);
        doc.setDrawColor(0, 0, 0);
        doc.link(margin + 4, y - 4, lineW, 5.5, { url });
        y += 5;
      }
      doc.setTextColor(0, 0, 0);
      y += 2;
    }
    y += 3;
  };

  // Render a labelled HTML field by parsing into paragraph blocks and
  // rendering each block with proper bold/italic inline runs.
  const renderHtmlField = (label: string, html: string | null | undefined): void => {
    if (!html?.trim()) return;
    const blocks = htmlToBlocks(html);
    const hasContent = blocks.some((b) => b.runs.some((r) => r.text.trim()));
    if (!hasContent) return;

    ensureSpace(14);
    doc.setFontSize(9);
    doc.setFont("helvetica", "bold");
    doc.setTextColor(0, 0, 0);
    doc.text(`${label}:`, margin, y);
    y += 5;

    const indent = margin + 4;
    const blockW = contentW - 4;

    for (const block of blocks) {
      // Collect the full text of the block to split into lines.
      // We track a parallel run-index so we can re-apply bold/italic per line.
      const prefix = block.isList ? "• " : "";

      // Flatten runs into word tokens while preserving run metadata.
      type Token = { word: string; bold: boolean; italic: boolean };
      const tokens: Token[] = [];
      for (const run of block.runs) {
        if (run.text === "\n") {
          tokens.push({ word: "\n", bold: false, italic: false });
          continue;
        }
        const words = run.text.split(/(\s+)/);
        for (const w of words) {
          if (w) tokens.push({ word: w, bold: run.bold, italic: run.italic });
        }
      }

      // Build lines respecting word-wrap and forced \n breaks.
      type StyledLine = { segments: { text: string; bold: boolean; italic: boolean }[] };
      const lines: StyledLine[] = [{ segments: [] }];
      let curLineWidth = indent - margin + doc.getTextWidth(prefix);

      const pushSegment = (text: string, bold: boolean, italic: boolean): void => {
        const cur = lines[lines.length - 1];
        const last = cur.segments[cur.segments.length - 1];
        if (last && last.bold === bold && last.italic === italic) {
          last.text += text;
        } else {
          cur.segments.push({ text, bold, italic });
        }
      };

      let firstToken = true;
      for (const token of tokens) {
        if (token.word === "\n") {
          lines.push({ segments: [] });
          curLineWidth = 0;
          firstToken = true;
          continue;
        }
        if (/^\s+$/.test(token.word)) {
          if (!firstToken) {
            const spW = doc.getTextWidth(" ");
            if (curLineWidth + spW <= blockW) {
              pushSegment(" ", token.bold, token.italic);
              curLineWidth += spW;
            }
          }
          continue;
        }
        const wordW = doc.getTextWidth(token.word);
        if (!firstToken && curLineWidth + wordW > blockW) {
          lines.push({ segments: [] });
          curLineWidth = 0;
          firstToken = true;
        }
        if (firstToken && prefix && lines.length === 1 && lines[0].segments.length === 0) {
          pushSegment(prefix, false, false);
          curLineWidth += doc.getTextWidth(prefix);
        }
        pushSegment(token.word, token.bold, token.italic);
        curLineWidth += wordW;
        firstToken = false;
      }

      const URL_RE = /https?:\/\/[^\s]+/g;

      for (const line of lines) {
        const lineText = line.segments.map((s) => s.text).join("");
        if (!lineText.trim()) continue;
        const lineH = 5;
        ensureSpace(lineH + 2);
        let x = indent;
        for (const seg of line.segments) {
          const font = seg.bold ? "bold" : seg.italic ? "italic" : "normal";
          doc.setFont("helvetica", font);
          URL_RE.lastIndex = 0;
          const parts: { text: string; isUrl: boolean }[] = [];
          let last = 0;
          let m: RegExpExecArray | null;
          while ((m = URL_RE.exec(seg.text)) !== null) {
            if (m.index > last) parts.push({ text: seg.text.slice(last, m.index), isUrl: false });
            parts.push({ text: m[0], isUrl: true });
            last = m.index + m[0].length;
          }
          if (last < seg.text.length) parts.push({ text: seg.text.slice(last), isUrl: false });

          for (const part of parts) {
            if (part.isUrl) {
              doc.setTextColor(0, 102, 204);
              doc.text(part.text, x, y);
              const w = doc.getTextWidth(part.text);
              doc.setDrawColor(0, 102, 204);
              doc.line(x, y + 0.8, x + w, y + 0.8);
              doc.setDrawColor(0, 0, 0);
              doc.link(x, y - 4, w, 5.5, { url: part.text });
              x += w;
            } else {
              doc.setTextColor(60, 60, 60);
              doc.text(part.text, x, y);
              x += doc.getTextWidth(part.text);
            }
          }
        }
        y += lineH;
      }
      y += 2; // paragraph gap
    }

    doc.setFont("helvetica", "normal");
    doc.setTextColor(0, 0, 0);
    y += 5;
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

    const parsedDesc = parseDescriptionSections(entry.description);
    const bugFixUrls = parseBugFixes(entry.bugFixes);

    if (parsedDesc.generalDescription) renderHtmlField("General Description", parsedDesc.generalDescription);
    if (parsedDesc.implementationDetails) renderHtmlField("Implementation Details", parsedDesc.implementationDetails);
    if (parsedDesc.impact) renderHtmlField("Impact", parsedDesc.impact);
    if (bugFixUrls.length > 0) renderUrlListField("Bug Fixes", bugFixUrls);

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

      renderHtmlField("Overview", adv.overview);
      renderHtmlField("Description", adv.description);
      renderHtmlField("Impact", adv.impact);
      renderHtmlField("Solution", adv.solution);
      renderHtmlField("Notes", adv.notes);
      renderHtmlField("Credits", adv.credits);

      y += 6;
    }
  }

  // ===== FILE SUMMARY SECTIONS =====
  const allFilesModified = new Set<string>();
  const allFilesAdded = new Set<string>();
  const allFilesRemoved = new Set<string>();

  for (const entry of reportData.allEntries) {
    for (const f of parseFileList(entry.filesModified)) allFilesModified.add(f);
    for (const f of parseFileList(entry.filesAdded)) allFilesAdded.add(f);
    for (const f of parseFileList(entry.filesRemoved)) allFilesRemoved.add(f);
  }

  const hasFileSections =
    allFilesModified.size > 0 || allFilesAdded.size > 0 || allFilesRemoved.size > 0;

  if (hasFileSections) {
    ensureSpace(20);
    doc.setDrawColor(180, 180, 180);
    doc.setLineWidth(0.5);
    doc.line(margin, y, pageW - margin, y);
    doc.setLineWidth(0.2);
    doc.setDrawColor(0, 0, 0);
    y += 10;

    const renderFileSection = (title: string, files: Set<string>): void => {
      if (files.size === 0) return;
      const sorted = [...files].sort();
      ensureSpace(25);
      doc.setFontSize(12);
      doc.setFont("helvetica", "bold");
      doc.setTextColor(0, 102, 204);
      doc.text(title, margin, y);
      doc.setTextColor(0, 0, 0);
      y += 8;
      doc.setFontSize(9);
      doc.setFont("helvetica", "normal");
      for (const file of sorted) {
        const lines = doc.splitTextToSize(`• ${file}`, contentW - 4);
        ensureSpace(lines.length * 5 + 2);
        doc.text(lines, margin + 2, y);
        y += lines.length * 5 + 1;
      }
      y += 6;
    };

    renderFileSection("Updated Files", allFilesModified);
    renderFileSection("Added Files", allFilesAdded);
    renderFileSection("Removed Files", allFilesRemoved);
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
