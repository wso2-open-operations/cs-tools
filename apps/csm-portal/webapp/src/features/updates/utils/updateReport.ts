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

import type {
  SecurityAdvisory,
  UpdateDescription,
  UpdateLevelsSearchResponse,
} from "@features/updates/types/updates";

export interface ReportParams {
  productName: string;
  productVersion: string;
  startLevel: number;
  endLevel: number;
}

export interface ReportRow {
  levelKey: string;
  updateType: string;
  desc: UpdateDescription;
  description: string;
  instructions: string;
  bugFixes: string[];
  filesAdded: string[];
  filesModified: string[];
  filesRemoved: string[];
  securityAdvisories: SecurityAdvisory[];
  released: string;
}

export interface ReportData {
  params: ReportParams;
  rows: ReportRow[];
  counts: { security: number; regular: number; mixed: number };
}

function stripHtmlTags(html: string | undefined | null): string {
  if (!html) return "";
  // Decode common entities first, then strip tags, collapse whitespace.
  const decoded = html
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'");
  return decoded
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<\/p>/gi, "\n\n")
    .replace(/<\/li>/gi, "\n")
    .replace(/<li[^>]*>/gi, "• ")
    .replace(/<[^>]+>/g, "")
    .replace(/[ \t]+\n/g, "\n")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

export function parseJsonStringArray(raw: string | undefined | null): string[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as string[]) : [];
  } catch {
    return [];
  }
}

function isMeaningful(text: string): boolean {
  const t = text.trim().toLowerCase();
  return t !== "" && t !== "n/a" && t !== "na";
}

function formatReleasedDate(timestamp: number): string {
  if (!Number.isFinite(timestamp) || timestamp <= 0) return "—";
  const d = new Date(timestamp);
  return d.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "2-digit",
  });
}

export function buildReportData(
  params: ReportParams,
  data: UpdateLevelsSearchResponse,
): ReportData {
  const rows: ReportRow[] = [];
  const counts = { security: 0, regular: 0, mixed: 0 };

  const entries = Object.entries(data).sort(
    ([a], [b]) => Number(a) - Number(b),
  );

  for (const [levelKey, group] of entries) {
    const t = group.updateType.toLowerCase();
    if (t === "security") counts.security += 1;
    else if (t === "regular") counts.regular += 1;
    else if (t === "mixed") counts.mixed += 1;

    for (const desc of group.updateDescriptionLevels) {
      const description = stripHtmlTags(desc.description);
      const instructions = isMeaningful(desc.instructions ?? "")
        ? stripHtmlTags(desc.instructions)
        : "";
      rows.push({
        levelKey,
        updateType: group.updateType,
        desc,
        description,
        instructions,
        bugFixes: parseJsonStringArray(desc.bugFixes),
        filesAdded: parseJsonStringArray(desc.filesAdded),
        filesModified: parseJsonStringArray(desc.filesModified),
        filesRemoved: parseJsonStringArray(desc.filesRemoved),
        securityAdvisories: desc.securityAdvisories ?? [],
        released: formatReleasedDate(desc.timestamp),
      });
    }
  }

  return { params, rows, counts };
}
