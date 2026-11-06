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


import { formatDateOnlyForDisplay } from "@utils/dateTime";
import type {
  CatalogCarbon,
  CatalogSupportStatus,
} from "@features/csm-products/types/productCatalog";

/** Display order of the four support statuses (the enum's lifecycle order). */
export const SUPPORT_STATUSES: readonly CatalogSupportStatus[] = [
  "available",
  "extended",
  "deprecated",
  "discontinued",
];

export const SUPPORT_STATUS_LABEL: Record<CatalogSupportStatus, string> = {
  available: "Available",
  extended: "Extended",
  deprecated: "Deprecated",
  discontinued: "Discontinued",
};

/** Chip colour per status: routine values stay calm, only the end states warn. */
export const SUPPORT_STATUS_COLOR: Record<CatalogSupportStatus, "success" | "info" | "warning" | "default"> = {
  available: "success",
  extended: "info",
  deprecated: "warning",
  discontinued: "default",
};

/** Human labels for every change payload key, across the three entity types. */
export const CATALOG_FIELD_LABEL: Record<string, string> = {
  productId: "Product",
  version: "Version",
  deploymentProfile: "Deployment profile",
  publicId: "Public listing id",
  currentSupportStatus: "Support status",
  releaseDate: "Release date",
  earliestPossibleSupportEolDate: "Earliest possible EOL",
  supportEolDate: "EOL date",
  supportEolAnnouncementDate: "EOL announcement",
  publicName: "Public name",
  publicAbbreviation: "Public abbreviation",
  description: "Description",
  releaseLinks: "Release links",
  isPublic: "On Release Matrix",
  showInSupportMatrix: "On Support Matrix",
  tags: "Tags",
  legacyStatus: "Legacy status",
  releaseLineId: "Release line",
  carbon: "Carbon platform",
  retired: "Retired",
  name: "Name",
  startDate: "Start date",
  status: "Status",
  class: "Class",
  code: "Code",
  unit: "Unit",
  businessUnit: "Business unit",
  manufacturer: "Manufacturer",
};

export const CARBON_FIELD_LABEL: Record<keyof CatalogCarbon, string> = {
  platformRelease: "Platform release",
  platformVersion: "Platform version",
  carbonKernelVersion: "Kernel version",
  releaseChunk: "Release chunk",
  kernelPatchNo: "Kernel patch",
};

export const EMPTY_VALUE = "—";

/** A value and its label, for a select. */
export interface CatalogOption {
  value: string;
  label: string;
}

/** Product units, as their lowercase API strings. */
export const PRODUCT_UNIT_OPTIONS: readonly CatalogOption[] = [
  { value: "integration", label: "Integration" },
  { value: "iot", label: "IoT" },
  { value: "iam", label: "IAM" },
  { value: "choreo", label: "Choreo" },
  { value: "asgardeo", label: "Asgardeo" },
  { value: "apim", label: "API Management" },
  { value: "analytics", label: "Analytics" },
  { value: "devant", label: "Devant" },
  { value: "bijira", label: "Bijira" },
];

export const PRODUCT_BUSINESS_UNIT_OPTIONS: readonly CatalogOption[] = [
  { value: "integration_software", label: "Integration software" },
  { value: "integration_cloud", label: "Integration cloud" },
  { value: "iam", label: "IAM" },
  { value: "corporate", label: "Corporate" },
];

export const PRODUCT_MANUFACTURER_OPTIONS: readonly CatalogOption[] = [
  { value: "wso2", label: "WSO2" },
  { value: "wso2_cloud", label: "WSO2 Cloud" },
];

/** A label for a payload key, falling back to the key itself. */
export function catalogFieldLabel(key: string): string {
  return CATALOG_FIELD_LABEL[key] ?? key;
}

/**
 * A catalogue date (`YYYY-MM-DD`) for display, e.g. "Aug 1, 2026". An
 * unparseable value is shown as stored rather than hidden, so a bad imported
 * date stays visible.
 */
export function formatCatalogDate(value: string | null | undefined): string {
  if (!value) return EMPTY_VALUE;
  return formatDateOnlyForDisplay(value) ?? value;
}

function isDateKey(key: string): boolean {
  return key === "startDate" || /Date$/.test(key);
}

/**
 * One catalogue value as display text, keyed by its camelCase field name.
 * `idNames` resolves a release line or product id to its name.
 */
export function formatCatalogValue(
  key: string,
  value: unknown,
  idNames?: ReadonlyMap<string, string>,
): string {
  if (value === null || value === undefined || value === "") return EMPTY_VALUE;
  if (typeof value === "boolean") {
    if (key === "retired") return value ? "Retired" : "Live";
    return value ? "Yes" : "No";
  }
  if (Array.isArray(value)) return value.length ? value.map(String).join(", ") : EMPTY_VALUE;
  if (typeof value === "object") {
    const parts = Object.entries(value as Record<string, unknown>)
      .filter(([, v]) => v !== null && v !== undefined && v !== "")
      .map(([k, v]) => `${CARBON_FIELD_LABEL[k as keyof CatalogCarbon] ?? k}: ${String(v)}`);
    return parts.length ? parts.join("; ") : EMPTY_VALUE;
  }
  const text = String(value);
  if (key === "currentSupportStatus") {
    const lower = text.toLowerCase();
    return lower in SUPPORT_STATUS_LABEL ? SUPPORT_STATUS_LABEL[lower as CatalogSupportStatus] : titleCase(text);
  }
  if ((key === "releaseLineId" || key === "productId") && idNames?.has(text)) {
    return idNames.get(text) as string;
  }
  if (isDateKey(key) && /^\d{4}-\d{2}-\d{2}/.test(text)) return formatCatalogDate(text.slice(0, 10));
  if (/On$/.test(key) && /^\d{4}-\d{2}-\d{2}T/.test(text)) return formatCatalogDate(text.slice(0, 10));
  return text;
}


/** "AVAILABLE" or "available" -> "Available"; "in_progress" -> "In progress". */
export function titleCase(value: string): string {
  const words = value.replace(/_/g, " ").trim().toLowerCase();
  return words ? words[0].toUpperCase() + words.slice(1) : words;
}
