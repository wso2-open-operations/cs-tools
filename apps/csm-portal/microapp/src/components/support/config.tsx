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
import type { CaseSeverity, CaseState, CaseType, CaseWorkState } from "@src/types";

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

// Labels/colors mirror the webapp's STATE_LABEL/STATE_COLOR
// (apps/csm-portal/webapp/src/features/csm-dashboard/utils/abtDashboard.ts). `reopened` has
// no webapp equivalent (its 6-state UI enum omits it) so it keeps a distinct color of its own.
export const STATE_LABELS: Record<CaseState, string> = {
  open: "Open",
  work_in_progress: "Work in progress",
  waiting_on_wso2: "Waiting on WSO2",
  awaiting_info: "Awaiting info",
  reopened: "Reopened",
  solution_proposed: "Solution proposed",
  closed: "Closed",
};

export const STATE_CHIP_COLOR_CONFIG: Record<CaseState, NonNullable<ChipProps["color"]>> = {
  open: "info",
  work_in_progress: "info",
  waiting_on_wso2: "warning",
  awaiting_info: "default",
  reopened: "error",
  solution_proposed: "default",
  closed: "success",
};

// Format/order mirrors the webapp's SeverityChip withLabel mode ("S2 (High)", code first) and
// SEVERITY_LABEL (S4 is "Low / Query", not just "Low").
export const SEVERITY_LABELS: Record<CaseSeverity, string> = {
  catastrophic: "S0 (Catastrophic)",
  critical: "S1 (Critical)",
  high: "S2 (High)",
  medium: "S3 (Medium)",
  low: "S4 (Low / Query)",
};

export const SEVERITY_CHIP_COLOR_CONFIG: Record<CaseSeverity, NonNullable<ChipProps["color"]>> = {
  catastrophic: "error",
  critical: "error",
  high: "warning",
  medium: "info",
  low: "default",
};

// Filter option lists, mirroring apps/csm-portal/webapp's CasesFilterBar.tsx (ALL_SEVERITIES,
// PRIMARY_STATES, ALL_WORK_STATES, WORK_STATE_LABEL) — same option order as the webapp.
export const ALL_SEVERITIES: CaseSeverity[] = ["catastrophic", "critical", "high", "medium", "low"];

export const FILTERABLE_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "awaiting_info",
  "solution_proposed",
  "waiting_on_wso2",
  "closed",
  "reopened",
];

export const ALL_WORK_STATES: NonNullable<CaseWorkState>[] = ["ongoing", "paused"];

export const WORK_STATE_LABEL: Record<NonNullable<CaseWorkState>, string> = {
  ongoing: "Ongoing",
  paused: "Paused",
};
