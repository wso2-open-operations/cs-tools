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
import type { ProjectListItem, SearchProjectsResponse } from "@features/project-hub/types/projects";

export type AuthFetchFn = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

const EXPORT_ALL_PAGE_SIZE = 50;

const EXPORT_HEADERS = [
  "Project Key",
  "Name",
  "Status",
  "Start Date",
  "End Date",
  "Action Required Items",
  "Outstanding Items",
];

// A4 landscape usable width ~269mm, 7 columns.
const PDF_COLUMN_STYLES: Record<number, PdfColumnStyle> = {
  0: { cellWidth: 30 },                          // Project Key
  1: { cellWidth: 80 },                          // Name
  2: { cellWidth: 24, halign: "center" },        // Status
  3: { cellWidth: 28, halign: "center" },        // Start Date
  4: { cellWidth: 28, halign: "center" },        // End Date
  5: { cellWidth: 30, halign: "right" },         // Action Required Items
  6: { cellWidth: 35, halign: "right" },         // Outstanding Items
};

const DATE_LOCALE = "en-US";
const DATE_FORMAT: Intl.DateTimeFormatOptions = {
  year: "numeric",
  month: "short",
  day: "numeric",
};

function formatDate(value: string | null | undefined): string {
  if (!value) return "--";
  // Append T00:00:00 so the string is parsed as local time, not UTC.
  const normalized = /^\d{4}-\d{2}-\d{2}$/.test(value) ? `${value}T00:00:00` : value;
  const d = new Date(normalized);
  return isNaN(d.getTime()) ? "--" : d.toLocaleDateString(DATE_LOCALE, DATE_FORMAT);
}

function buildFilename(ext: "csv" | "pdf"): string {
  const now = new Date();
  const yyyy = now.getFullYear();
  const mm = String(now.getMonth() + 1).padStart(2, "0");
  const dd = String(now.getDate()).padStart(2, "0");
  return `projects-${yyyy}-${mm}-${dd}.${ext}`;
}

function mapProjectsToRows(projects: ProjectListItem[]): string[][] {
  return projects.map((p) => [
    p.key,
    p.name,
    p.closureState ?? "Active",
    formatDate(p.startDate),
    formatDate(p.endDate),
    String(p.actionRequiredCount ?? 0),
    String(p.outstandingCount ?? 0),
  ]);
}

/**
 * Fetches every page of projects matching an optional search query.
 * Use this to collect the full dataset before exporting.
 */
export async function fetchAllProjectsForExport(
  authFetch: AuthFetchFn,
  searchQuery?: string,
): Promise<ProjectListItem[]> {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");

  const allProjects: ProjectListItem[] = [];
  let offset = 0;

  for (;;) {
    const body = {
      pagination: { offset, limit: EXPORT_ALL_PAGE_SIZE },
      ...(searchQuery?.trim() ? { filters: { searchQuery: searchQuery.trim() } } : {}),
    };

    const response = await authFetch(`${baseUrl}/projects/search`, {
      method: "POST",
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      throw new Error(`Error fetching projects for export: ${response.statusText}`);
    }

    const data: SearchProjectsResponse = await response.json();
    const page = data.projects ?? [];
    allProjects.push(...page);

    const nextOffset = offset + EXPORT_ALL_PAGE_SIZE;
    if (page.length === 0 || allProjects.length >= data.totalRecords || nextOffset <= offset) {
      break;
    }
    offset = nextOffset;
  }

  return allProjects;
}

export function downloadProjectListCsv(projects: ProjectListItem[]): void {
  const rows = mapProjectsToRows(projects);
  const content = buildCsvContent(EXPORT_HEADERS, rows);
  downloadCsvFile(buildFilename("csv"), content);
}

export function downloadProjectListPdf(projects: ProjectListItem[]): void {
  const rows = mapProjectsToRows(projects);
  downloadPdfFile(
    buildFilename("pdf"),
    "Projects",
    EXPORT_HEADERS,
    rows,
    PDF_COLUMN_STYLES,
  );
}
