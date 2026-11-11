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

// Response shapes for the Timelogs report, ported from the source app's
// data/utils/types.ts (the reports-relevant subset only — kept local to this
// domain rather than a shared cross-domain file, per the port's scoping).
/* eslint-disable @typescript-eslint/no-explicit-any -- mirrors the source's own loosely-typed index signature */

export interface DataStruct {
  [key: string]: any;
}

export interface TimeLogBreakdownDetails extends DataStruct {
  projectName: string;
  projectKey: string;
  projectType: string;
  remainingQueryHours: string;
  totalQueryHours: string;
  /** Inclusive start of the period the report covers (YYYY-MM-DD); empty or absent when unbounded. */
  periodStart?: string;
  /** Inclusive end of the period the report covers (YYYY-MM-DD); empty or absent when unbounded. */
  periodEnd?: string;
  cases: TimeLogBreakdownCase[];
}

/** Optional period for the time report. No fields set means the backend default (the project's subscription period). */
export interface TimelogsPeriodParams {
  /** YYYY-MM-DD, inclusive, filters by time card work date. */
  from?: string;
  /** YYYY-MM-DD, inclusive. */
  to?: string;
  /** Disable the subscription-period defaults and report all time. */
  allTime?: boolean;
}

export interface TimeLogBreakdownCase extends DataStruct {
  caseSysId: string;
  caseNumber: string;
  caseId: string;
  caseType: string;
  shortDescription: string;
  priority: string;
  state: string;
  totalHours: string;
  consumedQueryHours: string;
  timeCards: TimeCardDetails[];
}

export interface TimeCardDetails extends DataStruct {
  total: string;
  createdOn: string;
  createdBy: string;
  isBillable: string;
  state: string;
}

export interface CSReportDetailsResponse extends DataStruct {
  subscriptionDetails: ReportInfo;
  casesRecords: CaseDetails[];
  slaDetails: SlaDetails;
  projectDeployments: ProjectDeployment[];
  monthlyCounts: MonthlyCounts[];
}

export interface ReportInfo extends DataStruct {
  projectName: string;
  projectKey: string;
  projectType: string;
  accountName: string;
  startDate: string;
  endDate: string;
  supportTier: string;
  subscription: string;
  totalQueryHours: string;
  consumedQueryHours: string;
}

export interface CaseDetails extends DataStruct {
  caseSysId: string;
  caseNumber: string;
  engagementType: string;
  caseType: string;
  casePriority: string;
  caseState: string;
  opened: string;
  description: string;
  updated: string;
  deployment: string;
  productName: string;
}

export interface SlaDetails extends DataStruct {
  slaRecords: SlaRecords[];
  slaPerformanceStats?: SlaPerformanceStats;
}

export interface SlaRecords extends DataStruct {
  task: string;
  slaDefinition: string;
  businessElapsedPercentage: string;
}

export interface SlaPerformanceStats extends DataStruct {
  Workaround: SlaMetric;
  Resolution: SlaMetric;
  Response: SlaMetric;
}

export interface SlaMetric extends DataStruct {
  fraction: number | string;
  percentage: string;
}

export interface ProjectDeployment extends DataStruct {
  name: string;
  products: ProductDetails[];
}

export interface ProductDetails extends DataStruct {
  name: string;
  version: string;
  supportStatus: string;
  eolDate: string;
  cores: string;
  tps: string;
  updateLevelInfo?: number | null;
  earliestPossibleSupportEOLDate?: string;
}

export interface MonthlyCounts extends DataStruct {
  yearAndMonth: string;
  counts: { incidentCount: number; queryCount: number };
}

// ---- SLA report ----
// Mirrors the backend's viewer.SLAReportDetails (GET /generate-sla-report).

export interface SlaReportMetric {
  /** 95th percentile of the elapsed time of the finished clocks, or "—". */
  elapsed: string;
  /** The policy duration the clocks run against, or "Not applicable". */
  target: string;
  /** Clocks that finished within the target. */
  met: number;
  /** Clocks that went past the target (finished late, or still running and already over). */
  missed: number;
  /** Clocks still inside the target (in progress or paused). */
  running: number;
}

export interface SlaReportSummaryRow {
  label: string;
  caseType: "incident" | "query";
  severity: string;
  cases: number;
  response: SlaReportMetric;
  workaround: SlaReportMetric;
  resolution: SlaReportMetric;
}

export type SlaClockStatus = "" | "Met" | "Missed" | "Running" | "Paused" | "Cancelled";

export interface SlaReportClock {
  elapsed: string;
  /** Empty when the case has no clock for the target. */
  status: SlaClockStatus;
}

export interface SlaReportCase {
  caseId: string;
  caseNumber: string;
  caseType: string;
  severity: string;
  state: string;
  /** RFC 3339. */
  openedOn: string;
  response: SlaReportClock;
  workaround: SlaReportClock;
  resolution: SlaReportClock;
}

export interface SlaReportDetails {
  projectName: string;
  projectKey: string;
  /** YYYY-MM-DD. */
  periodStart: string;
  /** YYYY-MM-DD. */
  periodEnd: string;
  summary: SlaReportSummaryRow[];
  cases: SlaReportCase[];
}

/** The period of an SLA report: cases created from..to, both YYYY-MM-DD and inclusive. */
export interface SlaReportPeriodParams {
  from: string;
  to: string;
}
