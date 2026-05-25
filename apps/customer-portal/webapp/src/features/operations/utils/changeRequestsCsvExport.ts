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

import type { ChangeRequestItem } from "@features/operations/types/changeRequests";
import { formatCaseCsvDateTime } from "@features/support/utils/casesCsvExport";
import { getAssignedEngineerLabel, stripHtml } from "@features/support/utils/support";
import { buildCsvContent, downloadCsvFile } from "@utils/csv";

export const CHANGE_REQUEST_EXPORT_CSV_HEADERS = [
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

/**
 * Resolves short description for a change request CSV row.
 *
 * @param item - Change request list item.
 * @returns Plain-text description.
 */
export function resolveChangeRequestCsvShortDescription(
  item: ChangeRequestItem,
): string {
  return stripHtml(item.title).trim() || stripHtml(item.description).trim();
}

/**
 * Maps change requests to CSV rows.
 *
 * @param items - Change requests to export.
 * @returns CSV data rows.
 */
export function mapChangeRequestsToCsvRows(
  items: ChangeRequestItem[],
): string[][] {
  return items.map((item) => [
    item.number ?? "",
    item.internalId ?? "",
    item.state?.label ?? "",
    resolveChangeRequestCsvShortDescription(item),
    item.type?.label ?? "",
    item.createdBy ?? "",
    getAssignedEngineerLabel(item.assignedEngineer),
    formatCaseCsvDateTime(item.updatedOn),
    formatCaseCsvDateTime(item.createdOn),
  ]);
}

/**
 * Builds CSV content for change request list export.
 *
 * @param items - Change requests to export.
 * @returns CSV file body.
 */
export function buildChangeRequestsExportCsv(items: ChangeRequestItem[]): string {
  return buildCsvContent(
    [...CHANGE_REQUEST_EXPORT_CSV_HEADERS],
    mapChangeRequestsToCsvRows(items),
  );
}

/**
 * Builds a filename for change request CSV export.
 *
 * @param projectId - Project id (optional segment).
 * @returns Filename with .csv extension.
 */
export function buildChangeRequestsCsvFilename(projectId?: string): string {
  const datePart = new Date().toISOString().slice(0, 10);
  const projectPart = projectId ? `-${projectId}` : "";
  return `change-requests${projectPart}-${datePart}.csv`;
}

/**
 * Downloads change request list rows as CSV.
 *
 * @param items - Change requests to export.
 * @param projectId - Project id for filename.
 */
export function downloadChangeRequestsCsv(
  items: ChangeRequestItem[],
  projectId?: string,
): void {
  const content = buildChangeRequestsExportCsv(items);
  downloadCsvFile(buildChangeRequestsCsvFilename(projectId), content);
}
