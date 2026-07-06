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

import type { CaseTimeCard } from "@features/usage-metrics/types/timeTracking";
import { formatMinutesAsHrMin } from "@features/project-details/utils/projectDetails";
import { buildCsvContent, downloadCsvFile } from "@utils/csv";
import { downloadPdfFile } from "@utils/pdf";

export const TIME_CARDS_CSV_HEADERS = [
  "Case Number",
  "Case Title",
  "Billable Hours",
  "Non-billable Hours",
  "Total Hours",
] as const;

function buildTimeCardRows(cards: CaseTimeCard[]): string[][] {
  const rows = cards.map((card) => [
    card.case.number,
    card.case.name,
    formatMinutesAsHrMin(card.billable.totalTime),
    formatMinutesAsHrMin(card.nonBillable.totalTime),
    formatMinutesAsHrMin(card.totalTime),
  ]);

  const totalBillable = cards.reduce((sum, c) => sum + c.billable.totalTime, 0);
  const totalNonBillable = cards.reduce((sum, c) => sum + c.nonBillable.totalTime, 0);
  const totalAll = cards.reduce((sum, c) => sum + c.totalTime, 0);

  rows.push([
    "Total",
    "",
    formatMinutesAsHrMin(totalBillable),
    formatMinutesAsHrMin(totalNonBillable),
    formatMinutesAsHrMin(totalAll),
  ]);

  return rows;
}

function buildFilename(prefix: string, ext: "csv" | "pdf"): string {
  const now = new Date();
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, "0");
  const d = String(now.getDate()).padStart(2, "0");
  return `${prefix}-time-cards-${y}-${m}-${d}.${ext}`;
}

/**
 * Builds and downloads a CSV file of case time cards.
 */
export function downloadTimeCardsCsv(
  cards: CaseTimeCard[],
  filenamePrefix: string,
): void {
  const rows = buildTimeCardRows(cards);
  const content = buildCsvContent([...TIME_CARDS_CSV_HEADERS], rows);
  downloadCsvFile(buildFilename(filenamePrefix, "csv"), content);
}

/**
 * Builds and downloads a PDF file of case time cards.
 */
export function downloadTimeCardsPdf(
  cards: CaseTimeCard[],
  filenamePrefix: string,
  projectName?: string,
): void {
  const rows = buildTimeCardRows(cards);
  const title = projectName ? `Time Tracking — ${projectName}` : "Time Tracking";
  downloadPdfFile(
    buildFilename(filenamePrefix, "pdf"),
    title,
    [...TIME_CARDS_CSV_HEADERS],
    rows,
    {
      0: { cellWidth: 30 },
      1: { cellWidth: 80 },
      2: { cellWidth: 35, halign: "right" },
      3: { cellWidth: 40, halign: "right" },
      4: { cellWidth: 30, halign: "right" },
    },
  );
}
