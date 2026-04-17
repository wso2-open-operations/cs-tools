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
