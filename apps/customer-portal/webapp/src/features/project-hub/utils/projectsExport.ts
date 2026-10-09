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
import type {
  GlobalSearchPayload,
  GlobalSearchResponse,
} from "@features/project-hub/types/globalSearch";

export type AuthFetchFn = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

const EXPORT_ALL_PAGE_SIZE = 50;

/**
 * What an exported project row needs. Both a project row of the global search (what the
 * Projects tables on screen show) and a ProjectListItem satisfy it, so the export can be fed
 * either without a conversion.
 */
export type ProjectExportRow = {
  key: string;
  name: string;
  closureState?: string | null;
  startDate?: string | null;
  endDate?: string | null;
  actionRequiredCount?: number | null;
  outstandingCount?: number | null;
};

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

/**
 * The status as the export has always printed it: "Open", "Read Only", "Pending Notified".
 * POST /search returns the raw lowercase value ("read_only"); the endpoint the export used to
 * read title-cased it (INITCAP(REPLACE(state, '_', ' '))), so do the same here to keep the
 * exported files unchanged. No status is "Active".
 */
function formatClosureState(value: string | null | undefined): string {
  if (!value) return "Active";
  return value
    .replace(/_/g, " ")
    .toLowerCase()
    .replace(/(^|[^a-z0-9])([a-z])/g, (_m, sep: string, ch: string) => sep + ch.toUpperCase());
}

function buildFilename(ext: "csv" | "pdf"): string {
  const now = new Date();
  const yyyy = now.getFullYear();
  const mm = String(now.getMonth() + 1).padStart(2, "0");
  const dd = String(now.getDate()).padStart(2, "0");
  return `projects-${yyyy}-${mm}-${dd}.${ext}`;
}

function mapProjectsToRows(projects: ProjectExportRow[]): string[][] {
  return projects.map((p) => [
    p.key,
    p.name,
    formatClosureState(p.closureState),
    formatDate(p.startDate),
    formatDate(p.endDate),
    String(p.actionRequiredCount ?? 0),
    String(p.outstandingCount ?? 0),
  ]);
}

/**
 * Fetches every page of projects matching an optional search query.
 * Use this to collect the full dataset before exporting.
 *
 * It asks the same endpoint the Projects tables on screen do (POST /search, projects only), so
 * the file carries the counts the table shows. POST /projects/search, which this used to call,
 * returns no Action Required / Outstanding counts at all, so every exported row said 0.
 */
export async function fetchAllProjectsForExport(
  authFetch: AuthFetchFn,
  searchQuery?: string,
): Promise<ProjectExportRow[]> {
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");

  const allProjects: ProjectExportRow[] = [];
  let offset = 0;

  for (;;) {
    const body: GlobalSearchPayload = {
      filters: {
        types: ["projects"],
        ...(searchQuery?.trim() ? { searchQuery: searchQuery.trim() } : {}),
      },
      projectsPagination: { offset, limit: EXPORT_ALL_PAGE_SIZE },
    };

    const response = await authFetch(`${baseUrl}/search`, {
      method: "POST",
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      throw new Error(`Error fetching projects for export: ${response.statusText}`);
    }

    const data: GlobalSearchResponse = await response.json();
    const page = data.projects ?? [];
    allProjects.push(...page);

    // Advance by what the page actually held, not by the page size we asked for: if any layer
    // ever returned fewer than asked, stepping by the full size would skip the rows between.
    if (page.length === 0 || allProjects.length >= data.projectsTotal) {
      break;
    }
    offset += page.length;
  }

  return allProjects;
}

export function downloadProjectListCsv(projects: ProjectExportRow[]): void {
  const rows = mapProjectsToRows(projects);
  const content = buildCsvContent(EXPORT_HEADERS, rows);
  downloadCsvFile(buildFilename("csv"), content);
}

export function downloadProjectListPdf(projects: ProjectExportRow[]): void {
  const rows = mapProjectsToRows(projects);
  downloadPdfFile(
    buildFilename("pdf"),
    "Projects",
    EXPORT_HEADERS,
    rows,
    PDF_COLUMN_STYLES,
  );
}
