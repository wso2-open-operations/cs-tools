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

import { Shield, FileCheck, Siren, Package } from "@wso2/oxygen-ui-icons-react";
import type { SupportStatConfig } from "@features/support/constants/supportConstants";
import type { FilterDefinition } from "@components/list-view/ListFiltersPanel";
import {
  SecurityStatKey,
  SecurityTabId,
} from "@features/security/types/security";

/** Maps status label to API statusId (per product vulnerabilities API). */
export const VULNERABILITY_STATUS_IDS: Record<string, number> = {
  Open: 1,
  "In Progress": 2,
  Resolved: 3,
};

/**
 * Configuration for the security statistics cards.
 */
export const SECURITY_STAT_CONFIGS: SupportStatConfig<SecurityStatKey>[] = [
  {
    icon: Shield,
    iconColor: "warning",
    key: SecurityStatKey.activeSecurityReports,
    label: "Outstanding Security Reports",
  },
  {
    icon: FileCheck,
    iconColor: "success",
    key: SecurityStatKey.resolvedSecurityReports,
    label: "Resolved Security Reports (Last 30d)",
  },
];

export const SECURITY_STATS_ENTITY_NAME = "security";

export const SECURITY_PAGE_TABS = [
  {
    id: SecurityTabId.VULNERABILITIES,
    label: "Security Report Analysis",
    icon: Siren,
  },
  {
    id: SecurityTabId.COMPONENTS,
    label: "Component Analysis",
    icon: Package,
  },
] as const;

export const PRODUCT_VULNERABILITIES_SEARCH_DEBOUNCE_MS = 350;

export const PRODUCT_VULNERABILITIES_DEFAULT_ROWS_PER_PAGE = 10;

export const PRODUCT_VULNERABILITIES_TABLE_COLUMN_COUNT = 12;

export const PRODUCT_VULNERABILITIES_TABLE_PAGINATION_OPTIONS = [5, 10, 25, 50] as const;

export const PRODUCT_VULNERABILITIES_FETCH_ERROR_MESSAGE =
  "Failed to fetch product vulnerabilities";

export const PRODUCT_VULNERABILITIES_EMPTY_MESSAGE = "No vulnerabilities found.";

export const PRODUCT_VULNERABILITIES_TABLE_TITLE = "Component Analysis";

export const PRODUCT_VULNERABILITIES_TABLE_DESCRIPTION =
  "Third-party components with known vulnerabilities and remediation status";

export const PRODUCT_VULNERABILITIES_SEARCH_PLACEHOLDER = "Search CVE or component";

export const PRODUCT_VULNERABILITIES_FILTERS_BUTTON_LABEL = "Filters";

export const PRODUCT_VULNERABILITIES_CLEAR_FILTERS_LABEL = "Clear Filters";

export const PRODUCT_VULNERABILITIES_SEVERITY_LABEL = "Severity";

export const PRODUCT_VULNERABILITIES_SEVERITY_ALL_LABEL = "All Severity";

export const PRODUCT_VULNERABILITIES_PRODUCT_LABEL = "Product";

export const PRODUCT_VULNERABILITIES_PRODUCT_PLACEHOLDER = "Select a Product";

export const PRODUCT_VULNERABILITIES_PRODUCT_VERSION_LABEL = "Product Version";

export const PRODUCT_VULNERABILITIES_PRODUCT_VERSION_PLACEHOLDER = "Select a Product Version";

export const PRODUCT_VULNERABILITIES_ALL_PRODUCTS_LABEL = "All Products";

export const PRODUCT_VULNERABILITIES_ALL_VERSIONS_LABEL = "All Versions";

export const PRODUCT_VULNERABILITIES_ALL_FETCH_LIMIT = 5000;

export const SECURITY_REPORT_ANALYSIS_PAGE_SIZE = 10;

export const SECURITY_REPORT_ANALYSIS_TITLE = "Security Report Analysis";

export const SECURITY_REPORT_ANALYSIS_SUBTITLE =
  "Security vulnerability reports uploaded for analysis";

export const SECURITY_REPORT_SEARCH_PLACEHOLDER =
  "Search reports by case number, title, or description...";

export const SECURITY_REPORT_ANALYSIS_EMPTY_MESSAGE = "No reports found.";

export const SECURITY_REPORT_ANALYSIS_CREATE_BUTTON_LABEL = "Create";

export const SECURITY_REPORT_VIEW_TABS = [
  { id: "my", label: "My Reports" },
  { id: "all", label: "All Reports" },
] as const;

export const SECURITY_REPORT_SORT_OPTIONS = [
  { value: "createdOn", label: "Created date", kind: "chronological" },
  { value: "updatedOn", label: "Updated date", kind: "chronological" },
  { value: "state", label: "Status", kind: "ordinal" },
] as const;

export const SECURITY_REPORT_ENTITY_LABEL = "reports";

export const SECURITY_REPORT_FILTER_DEFINITIONS: FilterDefinition[] = [
  {
    id: "status",
    filterKey: "statusId",
    metadataKey: "caseStates",
  },
];

export const VULNERABILITY_DETAILS_CARD_TITLE = "Product Vulnerabilities";

export const VULNERABILITY_DETAILS_CVE_LABEL = "CVE";

export const VULNERABILITY_DETAILS_VULNERABILITY_ID_LABEL = "Vulnerability ID";

export const VULNERABILITY_DETAILS_SEVERITY_LABEL = "Severity";

export const VULNERABILITY_DETAILS_NVD_BASE_URL =
  "https://nvd.nist.gov/vuln/detail";

export const VULNERABILITY_DETAILS_NVD_BUTTON_LABEL = "NVD";

export const VULNERABILITY_DETAILS_HEADER_SUBTITLE = "Vulnerability details";

export const VULNERABILITY_DETAILS_PRODUCT_NAME_LABEL = "Product Name";

export const VULNERABILITY_DETAILS_PRODUCT_VERSION_LABEL = "Product Version";

export const VULNERABILITY_DETAILS_WSO2_RESOLUTION_LABEL = "WSO2 Resolution";

export const VULNERABILITY_DETAILS_COMPONENT_SECTION_TITLE = "Component Information";

export const VULNERABILITY_DETAILS_RESOLUTION_SECTION_TITLE =
  "Resolution & Mitigation";

export const VULNERABILITY_DETAILS_USE_CASE_LABEL = "Use Case";

export const VULNERABILITY_DETAILS_COMPONENT_TYPE_LABEL = "Component Type";

export const VULNERABILITY_DETAILS_COMPONENT_NAME_LABEL = "Component Name";

export const VULNERABILITY_DETAILS_COMPONENT_VERSION_LABEL = "Component Version";

export const VULNERABILITY_DETAILS_UPDATE_LEVEL_LABEL = "Update Level";

export const VULNERABILITY_DETAILS_JUSTIFICATION_LABEL = "Justification";

export const VULNERABILITY_DETAILS_RESOLUTION_LABEL = "Resolution";

export const VULNERABILITY_DETAILS_NOT_APPLICABLE_LABEL = "Not Applicable";

export const VULNERABILITY_DETAILS_NO_RESOLUTION_MESSAGE =
  "No justification or resolution information available.";

export const VULNERABILITY_DETAILS_LOAD_ERROR_TITLE =
  "Failed to load vulnerability details";

export const VULNERABILITY_DETAILS_LOAD_ERROR_CAPTION =
  "Something went wrong while fetching the information.";

export const VULNERABILITY_DETAILS_PAGE_LOAD_ERROR =
  "Could not load vulnerability details.";

export const COMPONENT_ANALYSIS_PLACEHOLDER_MESSAGE =
  "Component Analysis coming soon.";

export const SECURITY_ADVISORIES_PLACEHOLDER_MESSAGE =
  "Security Advisories table coming soon.";
