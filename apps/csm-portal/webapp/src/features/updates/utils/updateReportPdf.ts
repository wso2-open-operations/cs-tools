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
import type { ReportData, ReportRow } from "@features/updates/utils/updateReport";

const MARGIN_X = 14;
const PAGE_H = 297; // A4 portrait mm
const BOTTOM_MARGIN = 18;
const LINE_HEIGHT = 5;

interface PdfCursor {
  doc: jsPDF;
  y: number;
}

function ensureSpace(cur: PdfCursor, needed: number): void {
  if (cur.y + needed > PAGE_H - BOTTOM_MARGIN) {
    cur.doc.addPage();
    cur.y = 16;
  }
}

function writeWrapped(
  cur: PdfCursor,
  text: string,
  opts: { size?: number; bold?: boolean; color?: [number, number, number] } = {},
): void {
  const { size = 10, bold = false, color = [40, 40, 40] } = opts;
  cur.doc.setFontSize(size);
  cur.doc.setFont("helvetica", bold ? "bold" : "normal");
  cur.doc.setTextColor(color[0], color[1], color[2]);

  const maxWidth = 210 - MARGIN_X * 2;
  const lines = cur.doc.splitTextToSize(text, maxWidth) as string[];
  for (const line of lines) {
    ensureSpace(cur, LINE_HEIGHT);
    cur.doc.text(line, MARGIN_X, cur.y);
    cur.y += LINE_HEIGHT;
  }
}

function writeHeading(cur: PdfCursor, text: string, level: 1 | 2 | 3): void {
  const size = level === 1 ? 16 : level === 2 ? 12 : 10.5;
  ensureSpace(cur, size);
  cur.y += 2;
  writeWrapped(cur, text, { size, bold: true, color: [20, 20, 20] });
  cur.y += 1;
}

function writeBulletList(cur: PdfCursor, items: string[]): void {
  for (const item of items) {
    writeWrapped(cur, `• ${item}`, { size: 9.5 });
  }
}

function writeKeyValueLine(cur: PdfCursor, label: string, value: string): void {
  writeWrapped(cur, `${label}: ${value}`, { size: 10 });
}

function writeRow(cur: PdfCursor, row: ReportRow): void {
  writeHeading(
    cur,
    `Update ${row.desc.updateNumber}  ·  Level ${row.levelKey}  ·  ${capitalize(row.updateType)}`,
    2,
  );
  writeKeyValueLine(cur, "Released", row.released);

  if (row.description) {
    writeHeading(cur, "Description", 3);
    writeWrapped(cur, row.description, { size: 10 });
  }
  if (row.instructions) {
    writeHeading(cur, "Instructions", 3);
    writeWrapped(cur, row.instructions, { size: 10 });
  }
  if (row.bugFixes.length > 0) {
    writeHeading(cur, "Bug fixes", 3);
    writeBulletList(cur, row.bugFixes);
  }
  if (row.filesAdded.length > 0) {
    writeHeading(cur, "Files added", 3);
    writeBulletList(cur, row.filesAdded);
  }
  if (row.filesModified.length > 0) {
    writeHeading(cur, "Files modified", 3);
    writeBulletList(cur, row.filesModified);
  }
  if (row.filesRemoved.length > 0) {
    writeHeading(cur, "Files removed", 3);
    writeBulletList(cur, row.filesRemoved);
  }
  if (row.securityAdvisories.length > 0) {
    writeHeading(cur, "Security advisories", 3);
    for (const a of row.securityAdvisories) {
      writeWrapped(cur, `${a.id}  [${a.severity}]`, { size: 10, bold: true });
      if (a.overview) writeWrapped(cur, a.overview, { size: 9.5 });
    }
  }
  cur.y += 4;
}

function capitalize(s: string): string {
  return s.length === 0 ? s : s.charAt(0).toUpperCase() + s.slice(1);
}

export function generateUpdateReportPdf(report: ReportData): void {
  const { params, rows, counts } = report;
  const doc = new jsPDF({ unit: "mm", format: "a4" });

  // Header
  const cur: PdfCursor = { doc, y: 18 };
  writeWrapped(cur, "Update Levels Report", { size: 18, bold: true, color: [20, 20, 20] });
  writeWrapped(
    cur,
    `${params.productName}  ${params.productVersion}`,
    { size: 12, bold: false, color: [60, 60, 60] },
  );
  writeWrapped(
    cur,
    `Levels ${params.startLevel} → ${params.endLevel}`,
    { size: 11, bold: false, color: [80, 80, 80] },
  );
  writeWrapped(
    cur,
    `${rows.length > 0 ? new Set(rows.map((r) => r.levelKey)).size : 0} update level(s) · ` +
      `${counts.security} security · ${counts.regular} regular${counts.mixed > 0 ? ` · ${counts.mixed} mixed` : ""}`,
    { size: 10, color: [100, 100, 100] },
  );
  cur.y += 2;

  // Summary table
  const tableRows = Array.from(
    rows.reduce((map, r) => {
      if (!map.has(r.levelKey)) {
        map.set(r.levelKey, { type: r.updateType, released: r.released });
      }
      return map;
    }, new Map<string, { type: string; released: string }>()),
  ).map(([level, v]) => [level, capitalize(v.type), v.released]);

  autoTable(doc, {
    startY: cur.y,
    head: [["Update Level", "Type", "Released"]],
    body: tableRows,
    styles: { fontSize: 9.5, cellPadding: 2 },
    headStyles: { fillColor: [240, 240, 240], textColor: [40, 40, 40], fontStyle: "bold" },
    margin: { left: MARGIN_X, right: MARGIN_X },
  });
  type DocWithLastTable = jsPDF & { lastAutoTable?: { finalY: number } };
  const lastTable = (doc as DocWithLastTable).lastAutoTable;
  cur.y = (lastTable?.finalY ?? cur.y) + 6;

  // Detail rows
  for (const row of rows) {
    writeRow(cur, row);
  }

  // Footer page numbers
  const pageCount = (doc as jsPDF & { internal: { getNumberOfPages: () => number } }).internal.getNumberOfPages();
  doc.setFont("helvetica", "normal");
  doc.setFontSize(8);
  doc.setTextColor(140, 140, 140);
  for (let i = 1; i <= pageCount; i++) {
    doc.setPage(i);
    doc.text(`Page ${i} of ${pageCount}`, 210 - MARGIN_X, PAGE_H - 10, { align: "right" });
  }

  const safeName = `${params.productName}-${params.productVersion}-${params.startLevel}-${params.endLevel}`
    .replace(/[^a-zA-Z0-9._-]+/g, "_");
  doc.save(`update-report-${safeName}.pdf`);
}
