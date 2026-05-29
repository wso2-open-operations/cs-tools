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

import jsPDF from "jspdf";
import autoTable from "jspdf-autotable";

export type PdfColumnStyle = {
  cellWidth: number;
  halign?: "left" | "center" | "right";
};

/**
 * Generates and downloads a PDF file with a titled table.
 *
 * @param filename - Download filename (should end with .pdf).
 * @param title - Heading text printed above the table.
 * @param headers - Column header labels.
 * @param rows - Table data rows aligned with headers.
 * @param columnStyles - Per-column width/alignment overrides (keyed by column index).
 */
export function downloadPdfFile(
  filename: string,
  title: string,
  headers: string[],
  rows: string[][],
  columnStyles?: Record<number, PdfColumnStyle>,
): void {
  const doc = new jsPDF({ orientation: "landscape", unit: "mm", format: "a4" });
  doc.setFontSize(12);
  doc.setTextColor(40, 40, 40);
  doc.text(title, 14, 14);
  autoTable(doc, {
    head: [headers],
    body: rows,
    startY: 20,
    margin: { left: 14, right: 14 },
    styles: {
      fontSize: 7.5,
      cellPadding: { top: 3, bottom: 3, left: 3, right: 3 },
      overflow: "linebreak",
      valign: "middle",
    },
    headStyles: {
      fillColor: [33, 83, 138],
      textColor: 255,
      fontStyle: "bold",
      fontSize: 7.5,
    },
    alternateRowStyles: { fillColor: [245, 247, 250] },
    columnStyles: columnStyles ?? {},
    tableWidth: "auto",
  });
  doc.save(filename);
}
