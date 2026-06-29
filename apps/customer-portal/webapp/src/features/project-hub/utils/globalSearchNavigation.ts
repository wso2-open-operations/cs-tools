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

import { colors } from "@wso2/oxygen-ui";
import type { GlobalSearchCase } from "@features/project-hub/types/globalSearch";

// Strips whitespace, underscores, and hyphens for label comparison.
// e.g. "Security Report Analysis" and "security_report_analysis" both → "securityreportanalysis"
function normalizeLabel(val: string): string {
  return val.toLowerCase().replace(/[\s_-]+/g, "").trim();
}

/**
 * Returns the chip color and display label for a case type.
 * Uses mid-tone colors (500–600 range) so alpha(color, 0.1) backgrounds adapt to both
 * light and dark mode — same pattern as the severity chip.
 *
 * "Query" / "Incident" are ServiceNow internal terms; display as "Case" to end users.
 */
export function getCaseTypeChipProps(label: string | null | undefined): {
  color: string;
  displayLabel: string;
} {
  const normalized = normalizeLabel(label ?? "");

  if (normalized === "engagement") {
    return { color: colors.teal?.[600] ?? "#0D9488", displayLabel: "Engagement" };
  }
  if (normalized === "servicerequest") {
    return { color: colors.amber?.[600] ?? "#D97706", displayLabel: "Service Request" };
  }
  if (normalized === "securityreportanalysis") {
    return { color: colors.purple?.[500] ?? "#A855F7", displayLabel: "Security Report Analysis" };
  }
  if (normalized === "query" || normalized === "incident") {
    return { color: colors.orange?.[500] ?? "#F97316", displayLabel: "Case" };
  }
  if (normalized === "changerequest") {
    return { color: colors.blue?.[500] ?? "#3B82F6", displayLabel: "Change Request" };
  }
  return { color: colors.grey?.[500] ?? "#6B7280", displayLabel: label?.trim() ?? "--" };
}

/**
 * Returns the correct in-app path for a global search case based on its type label.
 * Returns null when the case has no project (navigation not possible).
 *
 * We match on the normalized label because caseType.id is a ServiceNow GUID, not a constant.
 * Known labels from the API: "Engagement", "Service Request", "Security Report Analysis", "Query".
 *
 * Route mapping:
 *   engagement              → /projects/:id/engagements/:caseId
 *   service request         → /projects/:id/operations/service-requests/:caseId
 *   security report analysis → /projects/:id/security-center/security-report-analysis/:caseId
 *   default case / unknown  → /projects/:id/support/cases/:caseId
 */
export function getCaseNavigationPath(c: GlobalSearchCase): string | null {
  const projectId = c.project?.id;
  if (!projectId) return null;

  const typeLabel = normalizeLabel(c.caseType?.label ?? "");

  if (typeLabel === "engagement") {
    return `/projects/${projectId}/engagements/${c.id}`;
  }
  if (typeLabel === "servicerequest") {
    return `/projects/${projectId}/operations/service-requests/${c.id}`;
  }
  if (typeLabel === "securityreportanalysis") {
    return `/projects/${projectId}/security-center/security-report-analysis/${c.id}`;
  }
  return `/projects/${projectId}/support/cases/${c.id}`;
}
