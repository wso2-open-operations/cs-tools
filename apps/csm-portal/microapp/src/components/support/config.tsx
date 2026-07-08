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

import { colors, type ChipProps } from "@wso2/oxygen-ui";
import { Briefcase, Megaphone, OctagonAlert, Settings, Shield, type LucideIcon } from "@wso2/oxygen-ui-icons-react";
import type { CaseSeverity, CaseState, CaseType } from "@src/types";

export const TABS: CaseType[] = ["case", "service_request", "security_report_analysis", "engagement", "announcement"];

export const TYPE_CONFIG: Record<CaseType, { icon: LucideIcon; color: string }> = {
  case: { icon: OctagonAlert, color: colors.red[500] },
  service_request: { icon: Settings, color: colors.purple[500] },
  security_report_analysis: { icon: Shield, color: colors.orange[500] },
  engagement: { icon: Briefcase, color: colors.grey[600] },
  announcement: { icon: Megaphone, color: colors.blue[500] },
};

export const TAB_CONFIG: Record<CaseType, { title: string; subtitle: string; emptyMessage: string }> = {
  case: {
    title: "Cases",
    subtitle: "Standard support cases",
    emptyMessage: "No cases found.",
  },
  service_request: {
    title: "Service Requests",
    subtitle: "Catalog-based service requests",
    emptyMessage: "No service requests found.",
  },
  security_report_analysis: {
    title: "Security Reports",
    subtitle: "Security report analyses",
    emptyMessage: "No security report analyses found.",
  },
  engagement: {
    title: "Engagements",
    subtitle: "Ongoing engagements",
    emptyMessage: "No engagements found.",
  },
  announcement: {
    title: "Announcements",
    subtitle: "Broadcast announcements",
    emptyMessage: "No announcements found.",
  },
};

export const STATE_LABELS: Record<CaseState, string> = {
  open: "Open",
  work_in_progress: "In Progress",
  waiting_on_wso2: "Waiting on WSO2",
  awaiting_info: "Awaiting Info",
  reopened: "Reopened",
  solution_proposed: "Solution Proposed",
  closed: "Closed",
};

export const STATE_CHIP_COLOR_CONFIG: Record<CaseState, NonNullable<ChipProps["color"]>> = {
  open: "info",
  work_in_progress: "primary",
  waiting_on_wso2: "warning",
  awaiting_info: "warning",
  reopened: "error",
  solution_proposed: "success",
  closed: "default",
};

export const SEVERITY_LABELS: Record<CaseSeverity, string> = {
  catastrophic: "Catastrophic (S0)",
  critical: "Critical (S1)",
  high: "High (S2)",
  medium: "Medium (S3)",
  low: "Low (S4)",
};

export const SEVERITY_CHIP_COLOR_CONFIG: Record<CaseSeverity, NonNullable<ChipProps["color"]>> = {
  catastrophic: "error",
  critical: "error",
  high: "warning",
  medium: "info",
  low: "default",
};
