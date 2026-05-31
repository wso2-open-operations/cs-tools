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

import type { ComponentType } from "react";
import type { DashboardMockStats } from "@features/support/types/cases";

// Valid color types for the stat card icons.
export type StatCardColor =
  | "primary"
  | "secondary"
  | "error"
  | "warning"
  | "info"
  | "success";

// Configuration for a single statistic card.
export type StatConfigItem = {
  id: Exclude<keyof DashboardMockStats, "casesTrend">;
  label: string;
  icon: ComponentType<{ size?: number }>;
  iconColor: StatCardColor;
  tooltipText: string;
};

// Type definition for Cases Trend Chart data item.
export type CasesTrendChartDataItem = {
  name: string;
  key: string;
  color: string;
  radius?: [number, number, number, number];
  border?: boolean;
};

// Case type chip display configuration for type column.
export type CaseTypeChipConfig = {
  displayLabel: string;
  Icon: ComponentType<{ size?: number }>;
  bgColor: string;
  textColor: string;
  borderColor: string;
};

// Severity legend key.
export enum SeverityLegendKey {
  Catastrophic = "catastrophic",
  Critical = "critical",
  High = "high",
  Medium = "medium",
  Low = "low",
}

// Discriminant for dashboard case-type chip styling (API label → chip variant).
export enum CaseTypeChipKind {
  IncidentOrQuery = "incidentOrQuery",
  SecurityReportAnalysis = "securityReportAnalysis",
  ServiceRequest = "serviceRequest",
  ChangeRequest = "changeRequest",
  Fallback = "fallback",
}

// Row shape for {@link SEVERITY_LEGEND_ORDER} (severity legend / charts).
export type SeverityLegendEntry = {
  key: SeverityLegendKey;
  label: string;
  displayName: string;
  color: string;
};
