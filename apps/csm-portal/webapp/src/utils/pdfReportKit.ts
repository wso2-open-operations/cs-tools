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
import autoTable, { type CellHookData } from "jspdf-autotable";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";

/**
 * Shared low-level A4-portrait text-layout primitives for building a
 * jsPDF report by hand (no HTML/DOM rendering — jsPDF draws text/lines at
 * explicit coordinates). Extracted from `features/updates/utils/updateReportPdf.ts`,
 * the original (and still separate) report generator, so a second report
 * (case/incident/change-request export) doesn't have to re-derive the same
 * page-break/word-wrap math. Any future report generator should build on
 * this kit rather than hand-rolling its own cursor/wrap logic again.
 */

export const PDF_MARGIN_X = 14;
export const PDF_PAGE_W = 210; // A4 portrait mm
export const PDF_PAGE_H = 297; // A4 portrait mm
export const PDF_BOTTOM_MARGIN = 18;
export const PDF_LINE_HEIGHT = 5;

/** Deep slate — the report's one accent color (header band, table headers,
 * section-heading underline). Chosen independently of the in-app theme: the
 * portal itself ships several selectable themes (`config/themeConfig.ts`),
 * so there's no single "the brand color" to pull from for a document that
 * has to look the same regardless of which theme the exporting user has
 * picked on screen. */
export const PDF_ACCENT_COLOR: [number, number, number] = [30, 41, 59];
/** Light tint of {@link PDF_ACCENT_COLOR}, for card/row backgrounds. */
export const PDF_ACCENT_TINT: [number, number, number] = [241, 244, 249];
export const PDF_TEXT_COLOR: [number, number, number] = [33, 33, 33];
export const PDF_MUTED_TEXT_COLOR: [number, number, number] = [110, 110, 110];
export const PDF_DIVIDER_COLOR: [number, number, number] = [222, 226, 232];
/** A more visible divider, for separating entries in a free-flowing list
 * (`writeActivityList`) — {@link PDF_DIVIDER_COLOR} is deliberately subtle
 * for a table's own grid lines, but reads as too faint to notice as the
 * separator between full activity entries (reported live). */
export const PDF_STRONG_DIVIDER_COLOR: [number, number, number] = [165, 171, 181];

export type PdfColorRole = "error" | "warning" | "success" | "info" | "default";

/** Approximates this app's `SemanticChip`/`SeverityChip`/`StateChip` role
 * palette (error/warning/success/info/default) as RGB, for coloring a
 * status-like value (state, severity, priority, impact) inside a report —
 * jsPDF has no concept of a themed chip, so this is the closest equivalent:
 * bold, colored text rather than a filled pill. */
export const PDF_ROLE_COLOR: Record<PdfColorRole, [number, number, number]> = {
  error: [198, 40, 40],
  warning: [204, 108, 8],
  success: [43, 130, 82],
  info: [21, 101, 192],
  default: [90, 90, 90],
};

export interface PdfCursor {
  doc: jsPDF;
  y: number;
}

/** Starts a new page once `needed` more mm would run past the bottom margin. */
export function ensureSpace(cur: PdfCursor, needed: number): void {
  if (cur.y + needed > PDF_PAGE_H - PDF_BOTTOM_MARGIN) {
    cur.doc.addPage();
    cur.y = 16;
  }
}

/**
 * A handful of "smart"/typographic Unicode characters that are common in
 * copy-pasted text (curly quotes, em/en dash, ellipsis, no-break space) and
 * that jsPDF's standard 14 fonts (WinAnsi-encoded) render correctly *only*
 * once mapped back to their plain-ASCII form.
 */
const PDF_PUNCTUATION_FALLBACK: Record<string, string> = {
  "‘": "'",
  "’": "'",
  "‚": ",",
  "“": '"',
  "”": '"',
  "–": "-",
  "—": "-",
  "…": "...",
  " ": " ",
  // Unicode arrows (e.g. a "before → after" state-change description —
  // `describeAuditEntry` in `caseActivityFeed.ts`) are outside jsPDF's
  // standard-font range same as any other non-Latin-1 character; without a
  // fallback they were silently dropped by the 0x20-0xFF check below,
  // leaving a blank gap where the arrow should read (reported live: "State:
  // Open  Work In Progress" with nothing between the two values).
  "→": "->",
  "←": "<-",
  "⇒": "=>",
};

/**
 * Makes free text safe to draw with jsPDF's standard "helvetica" font, which
 * only supports the WinAnsi (roughly Latin-1) character set — anything
 * outside it (emoji, flag/regional-indicator sequences, CJK/Sinhala script,
 * zero-width joiners, other invisible formatting marks) doesn't render as a
 * glyph and comes out as visibly garbled/misaligned text instead (reported
 * live: a display name — apparently carrying a trailing emoji/invisible
 * character from its source system — rendered as widely letter-spaced
 * gibberish ending in stray characters like "$æ"). Iterates by Unicode code
 * point (`Array.from`, not raw UTF-16 indexing) so a surrogate pair (e.g. an
 * emoji) is consumed and dropped as one unit rather than leaving an orphaned
 * half behind. Common "smart" punctuation is mapped to its plain-ASCII
 * equivalent first so normal copy-pasted text isn't needlessly stripped;
 * everything else outside 0x20-0xFF is dropped. Every text-drawing helper in
 * this kit (`writeWrapped`, table cell values) routes through this — no call
 * site needs to remember to sanitize its own strings.
 */
export function toPdfSafeText(value: string): string {
  let out = "";
  for (const ch of Array.from(value)) {
    const mapped = PDF_PUNCTUATION_FALLBACK[ch];
    if (mapped !== undefined) {
      out += mapped;
      continue;
    }
    if (ch === "\n" || ch === "\t") {
      out += ch;
      continue;
    }
    const code = ch.codePointAt(0) ?? 0;
    if (code >= 0x20 && code <= 0xff) out += ch;
  }
  return out;
}

export function writeWrapped(
  cur: PdfCursor,
  text: string,
  opts: { size?: number; bold?: boolean; color?: [number, number, number] } = {},
): void {
  const { size = 10, bold = false, color = PDF_TEXT_COLOR } = opts;
  cur.doc.setFontSize(size);
  cur.doc.setFont("helvetica", bold ? "bold" : "normal");
  cur.doc.setTextColor(color[0], color[1], color[2]);

  const maxWidth = PDF_PAGE_W - PDF_MARGIN_X * 2;
  const lines = cur.doc.splitTextToSize(toPdfSafeText(text), maxWidth) as string[];
  for (const line of lines) {
    ensureSpace(cur, PDF_LINE_HEIGHT);
    cur.doc.text(line, PDF_MARGIN_X, cur.y);
    cur.y += PDF_LINE_HEIGHT;
  }
}

export function writeHeading(cur: PdfCursor, text: string, level: 1 | 2 | 3): void {
  const size = level === 1 ? 16 : level === 2 ? 12 : 10.5;
  ensureSpace(cur, size + 4);
  cur.y += 3;
  writeWrapped(cur, text, { size, bold: true, color: [15, 23, 42] });
  // A short accent underline beneath a level-2 section heading — a cheap way
  // to give the report visual structure without a full redesign.
  if (level === 2) {
    cur.doc.setDrawColor(...PDF_ACCENT_COLOR);
    cur.doc.setLineWidth(0.6);
    cur.doc.line(PDF_MARGIN_X, cur.y, PDF_MARGIN_X + 22, cur.y);
    cur.doc.setLineWidth(0.2);
    // `cur.y` here sits only one fixed `PDF_LINE_HEIGHT` (5mm) below the
    // heading text's own baseline — that's tight enough that the very next
    // line of body text (drawn with baseline ~2.5mm below *its own* top, at
    // a typical 10pt size) had its ascenders reaching back up past this
    // underline, rendering as a stray line struck through the first line of
    // text under every level-2 heading (reported live — visible under
    // "Description" and, on inspection, every other section in the report).
    // A larger gap than the plain (no-underline) case below is needed
    // specifically because this rule is a drawn line, not text — clear it by
    // more than any 10-10.5pt body text's ascent.
    cur.y += 4;
    return;
  }
  cur.y += 2;
}

export function writeBulletList(cur: PdfCursor, items: string[]): void {
  for (const item of items) {
    writeWrapped(cur, `• ${item}`, { size: 9.5 });
  }
}

export function writeKeyValueLine(cur: PdfCursor, label: string, value: string): void {
  writeWrapped(cur, `${label}: ${value}`, { size: 10 });
}

/**
 * A rule to visually separate one activity-list entry from the next (e.g.
 * one comment from the next). Drawn more prominently — {@link PDF_STRONG_DIVIDER_COLOR}
 * plus a heavier line width — than the subtle grid lines
 * `writeDetailsTable` draws for itself, since a very light rule reads as
 * barely-there between full activity entries (reported live); a visible
 * margin is left after it, not just before, so the next entry's own "When ·
 * Actor · Type" line doesn't feel like it's crowding the rule.
 */
export function writeDivider(cur: PdfCursor): void {
  ensureSpace(cur, 8);
  cur.y += 3;
  cur.doc.setDrawColor(...PDF_STRONG_DIVIDER_COLOR);
  cur.doc.setLineWidth(0.4);
  cur.doc.line(PDF_MARGIN_X, cur.y, PDF_PAGE_W - PDF_MARGIN_X, cur.y);
  cur.doc.setLineWidth(0.2);
  cur.y += 5;
}

/**
 * Draws a colored band across the top of the first page with the report
 * title/subtitle in it (white text on {@link PDF_ACCENT_COLOR}) — the report's
 * one deliberate "branding" touch, giving it a cover-page feel instead of
 * looking like a plain text dump. Resets the cursor to just below the band.
 */
export function writeReportHeader(cur: PdfCursor, title: string, subtitle?: string): void {
  const bandHeight = subtitle ? 28 : 22;
  cur.doc.setFillColor(...PDF_ACCENT_COLOR);
  cur.doc.rect(0, 0, PDF_PAGE_W, bandHeight, "F");
  cur.y = 16;
  cur.doc.setFontSize(17);
  cur.doc.setFont("helvetica", "bold");
  cur.doc.setTextColor(255, 255, 255);
  cur.doc.text(toPdfSafeText(title), PDF_MARGIN_X, cur.y);
  if (subtitle) {
    cur.y += 7;
    cur.doc.setFontSize(11);
    cur.doc.setFont("helvetica", "normal");
    cur.doc.setTextColor(225, 229, 235);
    const maxWidth = PDF_PAGE_W - PDF_MARGIN_X * 2;
    const lines = cur.doc.splitTextToSize(toPdfSafeText(subtitle), maxWidth) as string[];
    cur.doc.text(lines[0] ?? "", PDF_MARGIN_X, cur.y);
  }
  cur.y = bandHeight + 8;
}

export interface PdfDetailsRow {
  label: string;
  value: string;
  /** Renders the value in a status-like color (matching this app's chip
   * roles) instead of the table's default text color — for a state/severity/
   * priority/impact-type field. */
  colorRole?: PdfColorRole;
}

/**
 * Renders a set of label/value pairs (a case/incident/change-request's key
 * fields) as an actual two-column table via `jspdf-autotable`, rather than
 * plain "Label: value" text lines — reads far more like a structured report
 * (reported live: the previous plain-text layout "doesn't look good"). Rows
 * auto-paginate and word-wrap within their column exactly like the Updates
 * report's own summary table.
 */
export function writeDetailsTable(cur: PdfCursor, rows: PdfDetailsRow[]): void {
  autoTable(cur.doc, {
    startY: cur.y,
    theme: "grid",
    head: [["Field", "Value"]],
    body: rows.map((r) => [toPdfSafeText(r.label), toPdfSafeText(r.value)]),
    styles: {
      fontSize: 9.5,
      cellPadding: 2.4,
      textColor: PDF_TEXT_COLOR,
      lineColor: PDF_DIVIDER_COLOR,
      lineWidth: 0.1,
    },
    headStyles: {
      fillColor: PDF_ACCENT_COLOR,
      textColor: [255, 255, 255],
      fontStyle: "bold",
      fontSize: 9,
    },
    alternateRowStyles: { fillColor: PDF_ACCENT_TINT },
    columnStyles: {
      0: { fontStyle: "bold", cellWidth: 48, textColor: [70, 78, 92] },
    },
    margin: { left: PDF_MARGIN_X, right: PDF_MARGIN_X },
    didParseCell: (data: CellHookData) => {
      if (data.section !== "body" || data.column.index !== 1) return;
      const role = rows[data.row.index]?.colorRole;
      if (role) {
        data.cell.styles.textColor = PDF_ROLE_COLOR[role];
        data.cell.styles.fontStyle = "bold";
      }
    },
  });
  const lastTable = (cur.doc as jsPDF & { lastAutoTable?: { finalY: number } }).lastAutoTable;
  cur.y = (lastTable?.finalY ?? cur.y) + 7;
}

export interface PdfActivityRow {
  actor: string;
  /** Already formatted for display (e.g. via `formatBackendTimestampForDisplay`) — this kit has no date-formatting opinion of its own. */
  when: string;
  /** Short label for the row's own column — e.g. "Comment", "Internal note", "State change". */
  type: string;
  /** Already HTML-stripped/condensed plain text — see `condenseBlankLines`. */
  detail: string;
  /** Renders the Type cell in the "warning" role color — for a customer-invisible entry (an internal work note). */
  internal?: boolean;
}

/**
 * Renders a case/incident's full activity trail — comments *and* audit
 * (state/field-change) entries merged into one chronological list — as a
 * plain, free-flowing list rather than a table. This *used* to be a
 * `jspdf-autotable` table (When/Actor/Type/Details columns): that reads
 * fine for short entries, but a table cell has to accommodate the single
 * longest wrapped comment in the whole report, and jsPDF/autotable can't
 * reflow a table's remaining columns once a comment runs long — with a
 * handful of long comments the report ballooned in page count and the
 * narrow Details column made even moderately-long text wrap awkwardly.
 * Reported live as a request to drop the table specifically for this
 * section (while keeping {@link writeDetailsTable}'s table for the compact
 * key/value fields, where that concern doesn't apply). Each entry is a
 * bold "When · Actor · Type" line (colored "warning" for an internal-only
 * entry) followed by its own full-width paragraph, separated by a thin
 * divider — the same shape `writeCommentsTable`'s predecessor used before
 * merging comments and audit entries into one table.
 */
export function writeActivityList(cur: PdfCursor, rows: PdfActivityRow[]): void {
  if (rows.length === 0) {
    writeWrapped(cur, "No activity.", { size: 10, color: PDF_MUTED_TEXT_COLOR });
    return;
  }
  for (const row of rows) {
    writeDivider(cur);
    writeWrapped(cur, `${row.when}  ·  ${row.actor}  ·  ${row.type}`, {
      size: 9.5,
      bold: true,
      color: row.internal ? PDF_ROLE_COLOR.warning : [20, 20, 20],
    });
    writeWrapped(cur, row.detail, { size: 10 });
  }
}

/**
 * Collapses 3+ consecutive blank lines down to one blank line (2 newlines),
 * and trims the ends. Real-world comment/description text pasted from
 * elsewhere (a copied log, a product/catalog dump used as test data) can
 * carry long runs of blank lines that otherwise blow up a report with mostly
 * empty vertical space for no informational value.
 */
export function condenseBlankLines(text: string): string {
  return text.replace(/\n{3,}/g, "\n\n").trim();
}

/** Block-level elements that should introduce a line break in plain text —
 * everything else (bold/italic/links/spans) is purely inline and must NOT
 * break the surrounding line. */
const PDF_BLOCK_TAGS = new Set([
  "P",
  "DIV",
  "LI",
  "UL",
  "OL",
  "TR",
  "TABLE",
  "BLOCKQUOTE",
  "H1",
  "H2",
  "H3",
  "H4",
  "H5",
  "H6",
  "SECTION",
  "ARTICLE",
  "HEADER",
  "FOOTER",
]);

/**
 * Converts rich-text HTML (a case/CR description, a comment body, a plan
 * field) into plain text *with real line breaks preserved* — a paragraph, a
 * list item, a heading each end their own line, and `<li>` gets a leading
 * "• " — for drawing with jsPDF, which only draws plain text.
 *
 * `@utils/sanitizeHtml`'s own `stripHtmlTags` is NOT the right tool for this:
 * it's built for "a subject line stored as plain text" (documented as such)
 * and works by fully removing every tag via DOMPurify (`ALLOWED_TAGS: []`)
 * before reading `textContent` — which has no concept of block boundaries,
 * so `<p>First paragraph.</p><p>Second paragraph.</p>` collapses to "First
 * paragraph.Second paragraph." with nothing between them. Reported live:
 * real multi-paragraph/bulleted case and change-request content (a
 * paragraph intro, a blank line, then a bulleted details list — normal,
 * legitimate rich-text authoring) rendered in the PDF as a single run-on
 * wall of text with every paragraph/bullet mashed together, next to the same
 * content rendering correctly, properly spaced, on screen.
 *
 * This walks the sanitized (`sanitizeRichTextHtml` — the same pass used for
 * on-screen `dangerouslySetInnerHTML` rendering) DOM tree itself: a block
 * element's own text is followed by a newline once its children are done: an
 * inline element (`<strong>`/`<em>`/`<a>`/`<span>`) contributes no break of
 * its own, so "**Label:** some value" still reads as one
 * line, matching how it looks on screen. Callers should still run the
 * result through `condenseBlankLines` afterward — nested block structure
 * (e.g. a `<ul>` immediately followed by its own last `<li>`'s line break)
 * can still produce more consecutive blank lines than intended.
 */
export function htmlToPdfPlainText(html: string): string {
  if (!html) return "";
  const safeHtml = sanitizeRichTextHtml(html);
  const container = document.createElement("div");
  container.innerHTML = safeHtml;

  let out = "";
  const walk = (node: ChildNode): void => {
    if (node.nodeType === Node.TEXT_NODE) {
      out += node.textContent ?? "";
      return;
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return;
    const el = node as Element;
    if (el.tagName === "BR") {
      out += "\n";
      return;
    }
    if (el.tagName === "LI") out += "• ";
    el.childNodes.forEach(walk);
    if (PDF_BLOCK_TAGS.has(el.tagName)) out += "\n";
  };
  container.childNodes.forEach(walk);

  return out;
}

/** Stamps "Page X of Y" bottom-right on every page of the finished document. */
export function stampFooterPageNumbers(doc: jsPDF): void {
  const pageCount = (doc as jsPDF & { internal: { getNumberOfPages: () => number } })
    .internal.getNumberOfPages();
  doc.setFont("helvetica", "normal");
  doc.setFontSize(8);
  doc.setTextColor(...PDF_MUTED_TEXT_COLOR);
  for (let i = 1; i <= pageCount; i++) {
    doc.setPage(i);
    doc.text(`Page ${i} of ${pageCount}`, PDF_PAGE_W - PDF_MARGIN_X, PDF_PAGE_H - 10, {
      align: "right",
    });
  }
}

/** Strips characters unsafe for a downloaded filename, collapsing them to `_`. */
export function safeFileNamePart(value: string): string {
  return value.replace(/[^a-zA-Z0-9._-]+/g, "_");
}
