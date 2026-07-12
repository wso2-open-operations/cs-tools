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

import { saveBlob } from "@utils/saveBlob";
import { TIME_CARD_STATE_META } from "@features/csm-timecards/constants/timeCardConstants";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

const CSV_HEADER = [
  "Date",
  "Case",
  "Project",
  "Engineer",
  "State",
  "Billable",
  "Minutes",
  "Decided by",
];

/** Quotes a CSV field only when it needs it (contains a comma, quote, or
 * carriage return/newline), doubling any internal quotes — the minimal
 * escaping RFC 4180 requires. */
function csvField(value: string): string {
  if (!/["\r\n,]/.test(value)) return value;
  return `"${value.replace(/"/g, '""')}"`;
}

/** Builds the CSV text for a list of cards — split out from
 * {@link exportTimeCardsCsv} so it's testable without mocking the browser
 * download APIs. */
export function timeCardsToCsv(cards: CsmTimeCard[]): string {
  const rows = cards.map((c) =>
    [
      c.createdOn,
      c.caseNumber,
      c.projectName,
      c.userName,
      TIME_CARD_STATE_META[c.state].label,
      c.billable ? "Yes" : "No",
      String(c.totalMinutes),
      c.approvedByName ?? "",
    ]
      .map(csvField)
      .join(","),
  );
  return [CSV_HEADER.join(","), ...rows].join("\r\n");
}

/**
 * Downloads the given cards as a CSV file. Exports only the cards passed in
 * — callers currently only have one page loaded at a time (see the
 * pagination notes in `useTimeSheets.ts`), so this is a "current page"
 * export, not a full report across the whole scope.
 */
export function exportTimeCardsCsv(cards: CsmTimeCard[], filename: string): void {
  const csv = timeCardsToCsv(cards);
  saveBlob(new Blob([csv], { type: "text/csv;charset=utf-8;" }), filename);
}
