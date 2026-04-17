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
  SecurityReportCaseSortField,
  SecurityReportViewMode,
  SecurityTabId,
} from "@features/security/types/security";

/**
 * Resolves the active Security Center tab from the URL `tab` query value.
 *
 * @param tabParam - Raw `tab` search param, or null when absent.
 * @returns {SecurityTabId} The tab id to render.
 */
export function parseSecurityTabQueryParam(
  tabParam: string | null,
): SecurityTabId {
  switch (tabParam) {
    case SecurityTabId.COMPONENTS:
    case SecurityTabId.VULNERABILITIES:
      return tabParam;
    default:
      return SecurityTabId.COMPONENTS;
  }
}

/**
 * Maps a tab id from the report view `TabBar` to `SecurityReportViewMode`.
 *
 * @param tabId - Tab id (`my` or `all`).
 * @returns {SecurityReportViewMode} View mode for case search.
 */
export function parseSecurityReportViewMode(
  tabId: string,
): SecurityReportViewMode {
  switch (tabId) {
    case SecurityReportViewMode.MY:
      return SecurityReportViewMode.MY;
    case SecurityReportViewMode.ALL:
      return SecurityReportViewMode.ALL;
    default:
      return SecurityReportViewMode.ALL;
  }
}

/**
 * Coerces a sort field string from the results bar into a known API field.
 *
 * @param value - Sort field from `ListResultsBar`.
 * @returns {SecurityReportCaseSortField} Field to send in `sortBy`.
 */
export function parseSecurityReportCaseSortField(
  value: string,
): SecurityReportCaseSortField {
  switch (value) {
    case SecurityReportCaseSortField.createdOn:
      return SecurityReportCaseSortField.createdOn;
    case SecurityReportCaseSortField.updatedOn:
      return SecurityReportCaseSortField.updatedOn;
    case SecurityReportCaseSortField.severity:
      return SecurityReportCaseSortField.severity;
    case SecurityReportCaseSortField.state:
      return SecurityReportCaseSortField.state;
    default:
      return SecurityReportCaseSortField.createdOn;
  }
}
