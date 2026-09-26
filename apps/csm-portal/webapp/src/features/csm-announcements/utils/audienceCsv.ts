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

import type { ResolvedAudienceProject } from "@features/csm-announcements/api/useResolveAnnouncementAudience";

/**
 * Wraps a field in double quotes and escapes any embedded quote, per RFC
 * 4180. Also neutralizes a leading formula-trigger character (`=`, `+`,
 * `-`, `@`, tab, CR, or LF) by prefixing a single quote — project/account
 * names here come from ServiceNow data this app doesn't control, and CSV
 * quoting alone doesn't stop Excel/Sheets from treating a cell starting
 * with one of those as a formula when this export is opened (CWE-1236).
 */
function csvField(value: string): string {
  const neutralized = /^[=+\-@\t\r\n]/.test(value) ? `'${value}` : value;
  return `"${neutralized.replace(/"/g, '""')}"`;
}

/** Builds the resolved-audience CSV as a plain string, for both download and tests. */
export function buildAudienceCsv(projects: ResolvedAudienceProject[]): string {
  const header = ["Project key", "Project name", "Account"].map(csvField).join(",");
  const rows = projects.map((p) =>
    [p.key ?? "", p.name, p.accountName ?? ""].map(csvField).join(","),
  );
  return [header, ...rows].join("\r\n");
}

/** Triggers a browser download of the resolved audience as a CSV file. */
export function downloadAudienceCsv(projects: ResolvedAudienceProject[], fileName: string): void {
  const csv = buildAudienceCsv(projects);
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  try {
    const link = document.createElement("a");
    link.href = url;
    link.download = fileName;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  } finally {
    URL.revokeObjectURL(url);
  }
}
