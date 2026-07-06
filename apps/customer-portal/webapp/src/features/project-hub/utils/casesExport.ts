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

import { buildCsvContent, downloadCsvFile } from "@utils/csv";
import { downloadPdfFile, type PdfColumnStyle } from "@utils/pdf";
import type { GlobalSearchCase, GlobalSearchResponse } from "@features/project-hub/types/globalSearch";
import type { AuthFetchFn } from "@features/project-hub/utils/projectsExport";
import { getCaseTypeChipProps } from "@features/project-hub/utils/globalSearchNavigation";

const EXPORT_ALL_PAGE_SIZE = 50;

const EXPORT_HEADERS = [
  "Title",
  "Case ID",
  "Case Type",
  "Severity",
  "Status",
  "Created by",
  "Created on",
  "Project",
];

// A4 landscape usable width ~269mm, 8 columns.
const PDF_COLUMN_STYLES: Record<number, PdfColumnStyle> = {
  0: { cellWidth: 70 },                       // Title
  1: { cellWidth: 35 },                       // Case ID
  2: { cellWidth: 30 },                       // Case Type
  3: { cellWidth: 20, halign: "center" },     // Severity
  4: { cellWidth: 24, halign: "center" },     // Status
  5: { cellWidth: 30 },                       // Created by
  6: { cellWidth: 24, halign: "center" },     // Created on
  7: { cellWidth: 36 },                       // Project
};

const DATE_LOCALE = "en-US";
const DATE_FORMAT: Intl.DateTimeFormatOptions = {
  day: "numeric",
  month: "short",
  year: "numeric",
};

/** Formats an ISO date string for display in exported files. */
function formatDate(value: string | null | undefined): string {
  if (!value) return "--";
  const d = new Date(value);
  return isNaN(d.getTime()) ? "--" : d.toLocaleDateString(DATE_LOCALE, DATE_FORMAT);
}

/** Combines the portal case number and WSO2 internal ID into a single display string. */
function formatCaseId(number?: string | null, internalId?: string | null): string {
  const n = number?.trim() ?? "";
  const id = internalId?.trim() ?? "";
  if (n && id) return `${n} | ${id}`;
  return n || id || "--";
}

/** Builds a timestamped filename for the exported file. */
function buildFilename(ext: "csv" | "pdf"): string {
  const now = new Date();
  const yyyy = now.getFullYear();
  const mm = String(now.getMonth() + 1).padStart(2, "0");
  const dd = String(now.getDate()).padStart(2, "0");
  return `cases-${yyyy}-${mm}-${dd}.${ext}`;
}

/** Maps a list of GlobalSearchCase objects to string rows for CSV/PDF output. */
function mapCasesToRows(cases: GlobalSearchCase[]): string[][] {
  return cases.map((c) => {
    const { displayLabel: caseTypeLabel } = getCaseTypeChipProps(c.caseType?.label);
    return [
      c.title?.trim() ?? "--",
      formatCaseId(c.number, c.internalId),
      caseTypeLabel,
      c.severity?.label ?? "--",
      c.state?.label ?? "--",
      c.createdBy?.trim() ?? "--",
      formatDate(c.createdOn),
      c.project?.label ?? "--",
    ];
  });
}

/**
 * Fetches every page of cases matching an optional search query via POST /search.
 * Use this to collect the full dataset before exporting.
 */
export async function fetchAllCasesForExport(
  authFetch: AuthFetchFn,
  searchQuery?: string,
): Promise<GlobalSearchCase[]> {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");

  const allCases: GlobalSearchCase[] = [];
  let offset = 0;

  for (;;) {
    const body = {
      filters: {
        types: ["cases"],
        ...(searchQuery?.trim() ? { searchQuery: searchQuery.trim() } : {}),
      },
      casesPagination: { offset, limit: EXPORT_ALL_PAGE_SIZE },
    };

    const response = await authFetch(`${baseUrl}/search`, {
      method: "POST",
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      throw new Error(`Error fetching cases for export: ${response.statusText}`);
    }

    const data: GlobalSearchResponse = await response.json();
    const page = data.cases ?? [];
    allCases.push(...page);

    const total = data.casesTotal ?? 0;
    const nextOffset = offset + EXPORT_ALL_PAGE_SIZE;
    if (page.length === 0 || allCases.length >= total || nextOffset <= offset) {
      break;
    }
    offset = nextOffset;
  }

  return allCases;
}

/** Triggers a CSV file download containing the provided cases. */
export function downloadCaseListCsv(cases: GlobalSearchCase[]): void {
  const rows = mapCasesToRows(cases);
  const content = buildCsvContent(EXPORT_HEADERS, rows);
  downloadCsvFile(buildFilename("csv"), content);
}

/** Triggers a PDF file download containing the provided cases. */
export function downloadCaseListPdf(cases: GlobalSearchCase[]): void {
  const rows = mapCasesToRows(cases);
  downloadPdfFile(
    buildFilename("pdf"),
    "Cases",
    EXPORT_HEADERS,
    rows,
    PDF_COLUMN_STYLES,
  );
}
