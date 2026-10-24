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

import {
  VulnerabilitySeverityToken,
  VulnerabilityStatusToken,
  type AffectedProductGroup,
  type ProductVulnerability,
} from "@features/security/types/security";

/**
 * Returns the Oxygen UI color path for vulnerability severity.
 *
 * @param severity - Severity label (Critical, High, Medium, Low).
 * @returns The Oxygen UI color path.
 */
export const getVulnerabilitySeverityColor = (severity?: string): string => {
  const normalized = severity?.toLowerCase().trim() || "";
  switch (normalized) {
    case VulnerabilitySeverityToken.CRITICAL:
      return "error.main";
    case VulnerabilitySeverityToken.HIGH:
      return "warning.main";
    case VulnerabilitySeverityToken.MEDIUM:
      return "text.disabled";
    case VulnerabilitySeverityToken.LOW:
      return "info.main";
    default:
      return "text.secondary";
  }
};

/**
 * Returns the Oxygen UI color path for vulnerability status.
 * Uses exact matches so "unresolved" does not match "resolved".
 *
 * @param status - Status label (In Progress, Open, Resolved).
 * @returns The Oxygen UI color path.
 */
export const getVulnerabilityStatusColor = (status?: string): string => {
  const normalized = status?.toLowerCase().trim() || "";
  switch (normalized) {
    case VulnerabilityStatusToken.IN_PROGRESS:
      return "warning.main";
    case VulnerabilityStatusToken.OPEN:
      return "info.main";
    case VulnerabilityStatusToken.RESOLVED:
      return "success.main";
    default:
      return "text.secondary";
  }
};

const normalizeKey = (value?: string | null): string =>
  value?.trim().toLowerCase() ?? "";

// A fixed locale keeps the order identical for every user.
const compareText = (a?: string | null, b?: string | null): number =>
  (a ?? "").localeCompare(b ?? "", "en", { numeric: true, sensitivity: "base" });

/**
 * Whether two records describe the same vulnerability. Symmetric on purpose, so
 * opening any record of a group lists the same group: two records with a CVE
 * match on it, and records without a CVE match on a non-empty vulnerability id.
 * A record with a CVE never groups with one without.
 */
const isSameVulnerability = (
  a: ProductVulnerability,
  b: ProductVulnerability,
): boolean => {
  const cveA = normalizeKey(a.cveId);
  const cveB = normalizeKey(b.cveId);
  if (cveA || cveB) return cveA === cveB;

  const vulnerabilityA = normalizeKey(a.vulnerabilityId);
  return vulnerabilityA !== "" && vulnerabilityA === normalizeKey(b.vulnerabilityId);
};

/**
 * Returns every record that describes the same vulnerability as `current`.
 *
 * The backend stores one row per product, product version and component, so a
 * single CVE appears as many rows. `current` is always part of the result and
 * replaces any copy of itself in `all`, because it was fetched directly while
 * the list may be a few minutes old. Each id appears once. Rows are ordered by product,
 * product version, component, component version, update level and finally id,
 * so the order never depends on how the backend returned them.
 *
 * @param all - Candidate records (the records the server returned for the CVE).
 * @param current - The vulnerability the details page is showing.
 * @returns The affected product records, including `current`.
 */
export const getAffectedProducts = (
  all: ProductVulnerability[],
  current: ProductVulnerability | undefined,
): ProductVulnerability[] => {
  if (!current) return [];

  const byId = new Map<string, ProductVulnerability>();
  for (const item of all) {
    if (item.id !== current.id && isSameVulnerability(current, item)) {
      byId.set(item.id, item);
    }
  }
  byId.set(current.id, current);

  return [...byId.values()].sort(
    (a, b) =>
      compareText(a.productName, b.productName) ||
      compareText(a.productVersion, b.productVersion) ||
      compareText(a.componentName, b.componentName) ||
      compareText(a.version, b.version) ||
      compareText(a.updateLevel, b.updateLevel) ||
      (a.id < b.id ? -1 : a.id > b.id ? 1 : 0),
  );
};

/**
 * Key of the product + product version a record belongs to. Normalised the same
 * way as the CVE match, so "WSO2 IS" and " wso2 is " are one product.
 */
export const getAffectedProductGroupKey = (
  record: ProductVulnerability,
): string =>
  `${normalizeKey(record.productName)}\u0000${normalizeKey(record.productVersion)}`;

/**
 * Groups a vulnerability's records by product and product version, so the
 * several components of one product version appear as one entry. Groups keep
 * the order in which each product version first appears, which is the sorted
 * order produced by `getAffectedProducts`.
 *
 * @param records - The records of one vulnerability.
 * @returns One group per product version, each holding its component records.
 */
export const groupAffectedProducts = (
  records: ProductVulnerability[],
): AffectedProductGroup[] => {
  const groups = new Map<string, AffectedProductGroup>();

  for (const record of records) {
    const key = getAffectedProductGroupKey(record);
    let group = groups.get(key);
    if (!group) {
      group = {
        key,
        productName: record.productName?.trim() ?? "",
        productVersion: record.productVersion?.trim() ?? "",
        updateLevels: [],
        records: [],
      };
      groups.set(key, group);
    }

    group.records.push(record);
    const updateLevel = record.updateLevel?.trim();
    if (updateLevel && !group.updateLevels.includes(updateLevel)) {
      group.updateLevels.push(updateLevel);
    }
  }

  return [...groups.values()];
};
