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

import { CircleAlert, Shield, FileCheck } from "@wso2/oxygen-ui-icons-react";
import type { SupportStatConfig } from "@features/support/constants/supportConstants";

/**
 * Security tab identifiers for the Security Center page.
 */
export const SecurityTab = {
  COMPONENTS: "components",
  VULNERABILITIES: "vulnerabilities",
} as const;

export type SecurityTabType = (typeof SecurityTab)[keyof typeof SecurityTab];

/**
 * Case report type values.
 */
export const CaseReportType = {
  SECURITY: "security",
} as const;

export type CaseReportTypeValue =
  (typeof CaseReportType)[keyof typeof CaseReportType];

/**
 * Valid keys for security statistics.
 */
export type SecurityStatKey =
  | "totalVulnerabilities"
  | "activeSecurityReports"
  | "resolvedSecurityReports";

/**
 * Configuration for the security statistics cards.
 */
export const SECURITY_STAT_CONFIGS: SupportStatConfig<SecurityStatKey>[] = [
  {
    icon: CircleAlert,
    iconColor: "error",
    key: "totalVulnerabilities",
    label: "Total Vulnerabilities",
  },
  {
    icon: Shield,
    iconColor: "warning",
    key: "activeSecurityReports",
    label: "Active Security Reports",
  },
  {
    icon: FileCheck,
    iconColor: "success",
    key: "resolvedSecurityReports",
    label: "Resolved Security Reports (Last 30d)",
  },
];
