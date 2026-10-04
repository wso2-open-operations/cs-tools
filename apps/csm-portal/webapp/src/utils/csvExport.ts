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

/**
 * Quotes a CSV field only when it needs it (contains a comma, quote, or
 * carriage return/newline), doubling any internal quotes — the minimal
 * escaping RFC 4180 requires. Shared by every CSV export on the portal so the
 * escaping rules can't drift between them (originally lived only in the time
 * cards export).
 *
 * A value that starts with `=`, `+`, `-`, `@`, a tab or a carriage return would
 * be read as a formula by spreadsheet applications, so it is prefixed with a
 * single quote (shown as plain text by Excel/Sheets) before quoting.
 */
export function csvField(value: string): string {
  const safe = /^[=+\-@\t\r]/.test(value) ? `'${value}` : value;
  if (!/["\r\n,]/.test(safe)) return safe;
  return `"${safe.replace(/"/g, '""')}"`;
}

/** Builds RFC 4180-ish CSV text (CRLF line endings) from a header row and a
 * list of already-stringified data rows. */
export function rowsToCsvText(header: string[], rows: string[][]): string {
  const lines = [header, ...rows].map((row) => row.map(csvField).join(","));
  return lines.join("\r\n");
}

/** Builds the CSV text and triggers a browser download for it. */
export function downloadCsv(header: string[], rows: string[][], filename: string): void {
  const csv = rowsToCsvText(header, rows);
  saveBlob(new Blob([csv], { type: "text/csv;charset=utf-8;" }), filename);
}

/**
 * Safety cap on the number of rows a single filtered export will fetch.
 *
 * There is no server-side export endpoint (see the design note on
 * `useFilteredCsvExport`), so exporting "the whole filtered result set" means
 * paging the existing list-search endpoint client-side until it's exhausted.
 * An unbounded loop is a real risk for a wide-open filter (or none at all), so
 * this caps it — but per the product decision that shaped this feature, a cap
 * must never produce a CSV that silently looks complete. When it's hit, the
 * export still downloads what it fetched, but the filename and the on-screen
 * notice both say so explicitly (see `useFilteredCsvExport`).
 */
export const CSV_EXPORT_ROW_CAP = 5000;

/** One page of results from a list-search endpoint, as far as the exporter
 * cares — just the items and the server-reported total. */
export interface CsvExportPage<T> {
  items: T[];
  total: number;
}

/** A single fetch of one page, `[offset, limit)`, of the current filtered
 * search — bind the caller's current filters/sort into this closure. */
export type CsvExportPageFetcher<T> = (
  offset: number,
  limit: number,
) => Promise<CsvExportPage<T>>;

export interface FetchAllPagesOptions {
  /** Rows requested per page. Defaults to 50 (the backend's page-size cap on
   * every search endpoint today); override only if a specific endpoint's cap
   * differs. */
  pageSize?: number;
  /** See {@link CSV_EXPORT_ROW_CAP}. */
  maxRows?: number;
  /** Invoked after each page lands, with rows fetched so far and the
   * server-reported total (best-effort — a filter that changes counts
   * mid-export can move the total; the loop always trusts the *first*
   * observed total to size the progress bar and to decide when it's done, so
   * a shrinking total from concurrent data changes can't wedge it early or
   * make the reported progress un-monotonic). */
  onProgress?: (loaded: number, total: number) => void;
}

export interface FetchAllPagesResult<T> {
  items: T[];
  /** True when {@link CSV_EXPORT_ROW_CAP} (or a caller override) was hit
   * before every matching row had been fetched — the result is a prefix of
   * the full filtered set, not the whole thing. */
  truncated: boolean;
  /** The server-reported total for the filter, at the time of the first page. */
  total: number;
}

/**
 * Pages a list-search endpoint via `fetchPage` until the full filtered result
 * set has been retrieved (or the safety cap is hit) and returns every item
 * fetched. Never returns a "silently short" result: hitting the cap is
 * reported via `truncated`, and it is the caller's job to communicate that to
 * the user, not swallow it.
 */
export async function fetchAllPages<T>(
  fetchPage: CsvExportPageFetcher<T>,
  options: FetchAllPagesOptions = {},
): Promise<FetchAllPagesResult<T>> {
  const { pageSize = 50, maxRows = CSV_EXPORT_ROW_CAP, onProgress } = options;
  const items: T[] = [];
  let offset = 0;
  let total = 0;
  let firstPage = true;
  let truncated = false;

  for (;;) {
    const page = await fetchPage(offset, pageSize);
    if (firstPage) {
      total = page.total;
      firstPage = false;
    }
    items.push(...page.items);
    onProgress?.(items.length, total);

    // Defensive: an empty page (regardless of what `total` claims) always
    // ends the loop rather than looping forever on a backend inconsistency.
    if (page.items.length === 0) break;
    offset += page.items.length;
    if (offset >= total) break;

    if (items.length >= maxRows) {
      truncated = true;
      break;
    }
  }

  return { items, truncated, total };
}
