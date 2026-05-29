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

import type { CaseListItem } from "@features/support/types/cases";
import {
  formatDateTime,
  getAssignedEngineerLabel,
  mapSeverityToDisplay,
  stripHtml,
} from "@features/support/utils/support";
import { buildCsvContent, downloadCsvFile } from "@utils/csv";
import { downloadPdfFile } from "@utils/pdf";

export const ALL_CASES_CSV_HEADERS = [
  "Number",
  "WSO2 Case ID",
  "State",
  "Short Description",
  "Severity",
  "Created by",
  "Assigned to",
  "updated",
  "created",
] as const;

export const CASE_LIST_EXPORT_CSV_HEADERS = [
  "Number",
  "WSO2 Case ID",
  "State",
  "Short Description",
  "Type",
  "Created by",
  "Assigned to",
  "updated",
  "created",
] as const;

export type CaseListCsvExportVariant = "allCases" | "withType";

/**
 * Resolves the short description column for CSV export.
 *
 * @param caseItem - Case list item.
 * @returns Plain-text short description.
 */
export function resolveCaseCsvShortDescription(caseItem: CaseListItem): string {
  const title = stripHtml(caseItem.title).trim();
  if (title) {
    return title;
  }
  return stripHtml(caseItem.description).trim();
}

/**
 * Formats a timestamp for CSV (empty when missing).
 *
 * @param dateStr - API timestamp.
 * @returns Formatted date/time or empty string.
 */
export function formatCaseCsvDateTime(
  dateStr: string | null | undefined,
): string {
  if (!dateStr) {
    return "";
  }
  const formatted = formatDateTime(dateStr);
  return formatted === "Not Available" ? "" : formatted;
}

/**
 * Resolves the Type column for case list CSV export.
 *
 * @param caseItem - Case list item.
 * @returns Display type label.
 */
export function resolveCaseCsvTypeLabel(caseItem: CaseListItem): string {
  return (
    caseItem.engagementType?.label ??
    caseItem.type?.label ??
    caseItem.issueType?.label ??
    ""
  );
}

/**
 * Maps case list items to CSV rows aligned with `ALL_CASES_CSV_HEADERS`.
 *
 * @param cases - Cases to export.
 * @returns CSV data rows.
 */
export function mapCasesToCsvRows(cases: CaseListItem[]): string[][] {
  return cases.map((caseItem) => [
    caseItem.number ?? "",
    caseItem.internalId ?? "",
    caseItem.status?.label ?? "",
    resolveCaseCsvShortDescription(caseItem),
    caseItem.severity?.label
      ? mapSeverityToDisplay(caseItem.severity.label)
      : "",
    caseItem.createdBy ?? "",
    getAssignedEngineerLabel(caseItem.assignedEngineer),
    formatCaseCsvDateTime(caseItem.updatedOn),
    formatCaseCsvDateTime(caseItem.createdOn),
  ]);
}

/**
 * Maps case list items to CSV rows with a Type column (engagements, SR, SRA).
 *
 * @param cases - Cases to export.
 * @returns CSV data rows.
 */
export function mapCaseListExportCsvRows(cases: CaseListItem[]): string[][] {
  return cases.map((caseItem) => [
    caseItem.number ?? "",
    caseItem.internalId ?? "",
    caseItem.status?.label ?? "",
    resolveCaseCsvShortDescription(caseItem),
    resolveCaseCsvTypeLabel(caseItem),
    caseItem.createdBy ?? "",
    getAssignedEngineerLabel(caseItem.assignedEngineer),
    formatCaseCsvDateTime(caseItem.updatedOn),
    formatCaseCsvDateTime(caseItem.createdOn),
  ]);
}

/**
 * Builds CSV content for the All Cases export.
 *
 * @param cases - Cases to export.
 * @returns CSV file body.
 */
export function buildAllCasesListCsv(cases: CaseListItem[]): string {
  return buildCsvContent([...ALL_CASES_CSV_HEADERS], mapCasesToCsvRows(cases));
}

/**
 * Builds CSV content for engagements, service requests, or security reports.
 *
 * @param cases - Cases to export.
 * @returns CSV file body.
 */
export function buildCaseListExportCsv(cases: CaseListItem[]): string {
  return buildCsvContent(
    [...CASE_LIST_EXPORT_CSV_HEADERS],
    mapCaseListExportCsvRows(cases),
  );
}

/**
 * Builds a filename for a case list CSV export.
 *
 * @param prefix - File name prefix (e.g. engagements, service-requests).
 * @param projectId - Project id (optional segment).
 * @returns Filename with .csv extension.
 */
export function buildCaseListCsvFilename(
  prefix: string,
  projectId?: string,
): string {
  const datePart = new Date().toISOString().slice(0, 10);
  const projectPart = projectId ? `-${projectId}` : "";
  return `${prefix}${projectPart}-${datePart}.csv`;
}

/**
 * Builds a filename for the All Cases CSV export.
 *
 * @param projectId - Project id (optional segment).
 * @returns Filename with .csv extension.
 */
export function buildAllCasesCsvFilename(projectId?: string): string {
  return buildCaseListCsvFilename("cases", projectId);
}

/**
 * Downloads case list rows as CSV.
 *
 * @param cases - Cases to export.
 * @param variant - Column set (all cases includes severity; withType includes type).
 * @param filenamePrefix - Download file prefix.
 * @param projectId - Project id for filename.
 */
export function downloadCaseListCsv(
  cases: CaseListItem[],
  variant: CaseListCsvExportVariant,
  filenamePrefix: string,
  projectId?: string,
  projectName?: string,
): void {
  const content =
    variant === "allCases"
      ? buildAllCasesListCsv(cases)
      : buildCaseListExportCsv(cases);
  const datePart = new Date().toISOString().slice(0, 10);
  const namePart = projectName
    ? `-${projectName.toLowerCase().replace(/\s+/g, "-").replace(/[^a-z0-9-]/g, "")}`
    : projectId
      ? `-${projectId}`
      : "";
  downloadCsvFile(`${filenamePrefix}${namePart}-${datePart}.csv`, content);
}

/**
 * Downloads the All Cases list as a CSV file.
 *
 * @param cases - Cases to export.
 * @param projectId - Project id for filename.
 */
export function downloadAllCasesListCsv(
  cases: CaseListItem[],
  projectId?: string,
): void {
  downloadCaseListCsv(cases, "allCases", "cases", projectId);
}

/**
 * Downloads case list rows as a PDF file.
 *
 * @param cases - Cases to export.
 * @param variant - Column set (all cases includes severity; withType includes type).
 * @param filenamePrefix - Download file prefix.
 * @param projectId - Project id for filename.
 */
// Col widths (mm) for 9-column case table on A4 landscape (269mm usable):
// Number(22) WSO2CaseID(28) State(22) ShortDesc(72) Severity/Type(20)
// CreatedBy(30) AssignedTo(30) Updated(23) Created(22)
const CASE_PDF_COLUMN_STYLES = {
  0: { cellWidth: 22 },
  1: { cellWidth: 28 },
  2: { cellWidth: 22 },
  3: { cellWidth: 72 },
  4: { cellWidth: 20 },
  5: { cellWidth: 30 },
  6: { cellWidth: 30 },
  7: { cellWidth: 23 },
  8: { cellWidth: 22 },
} as const;

export function downloadCaseListPdf(
  cases: CaseListItem[],
  variant: CaseListCsvExportVariant,
  filenamePrefix: string,
  projectId?: string,
  projectName?: string,
): void {
  const headers =
    variant === "allCases"
      ? [...ALL_CASES_CSV_HEADERS]
      : [...CASE_LIST_EXPORT_CSV_HEADERS];
  const rows =
    variant === "allCases"
      ? mapCasesToCsvRows(cases)
      : mapCaseListExportCsvRows(cases);
  const datePart = new Date().toISOString().slice(0, 10);
  const namePart = projectName
    ? `-${projectName.toLowerCase().replace(/\s+/g, "-").replace(/[^a-z0-9-]/g, "")}`
    : projectId
      ? `-${projectId}`
      : "";
  const filename = `${filenamePrefix}${namePart}-${datePart}.pdf`;
  const label = filenamePrefix.replace(/-/g, " ");
  const title = projectName
    ? `${label} — ${projectName} — ${datePart}`
    : `${label} — ${datePart}`;
  downloadPdfFile(filename, title, headers, rows, CASE_PDF_COLUMN_STYLES);
}
