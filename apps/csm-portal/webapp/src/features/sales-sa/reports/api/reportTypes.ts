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

// Response shapes for the three reports, ported from the source app's
// data/utils/types.ts (the reports-relevant subset only — kept local to this
// domain rather than a shared cross-domain file, per the port's scoping).
/* eslint-disable @typescript-eslint/no-explicit-any -- mirrors the source's own loosely-typed index signature */

export interface DataStruct {
  [key: string]: any;
}

// ---- SLA report ----

export interface SLAReportResponse extends DataStruct {
  projectName: string;
  projectKey: string;
  percentileDataList: PercentileDataList[];
  caseDataList: CaseDataList[];
}

export interface PercentileDataList extends DataStruct {
  key: string;
  caseType: string;
  priority: string;
  responseTime: string;
  workaroundTime: string;
  resolutionTime: string;
}

export interface CaseDataList extends DataStruct {
  caseSysId: string;
  caseId: string;
  caseNumber: string;
  caseType: string;
  casepriority: string;
  caseState: string;
  opened: string;
  response: string;
  responseSysId: string;
  workaround: string;
  workaroundSysId: string;
  resolution: string;
  resolutionSysId: string;
}

// ---- CS report ----

export interface CSReportDetailsResponse extends DataStruct {
  subscriptionDetails: ReportInfo;
  casesRecords: CaseDetails[];
  slaDetails: SlaDetails;
  projectDeployments: ProjectDeployment[];
  // FLAG: quarterlyCounts does not exist on the real backend's
  // CSReportDetails (verified against both the new Go port's
  // internal/servicenow/reports.go — no QuarterlyCounts field at all — and
  // the original Ballerina modules/operations/types.bal, which never had
  // one either). No component in this app actually reads it (checked via
  // grep), so this has been dead/always-undefined since it was ported.
  // Marked optional rather than deleted in case a future page adds a
  // quarterly view; if that happens, it needs a new backend field first.
  quarterlyCounts?: QuarterlyCounts[];
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
  slaPerformanceStats: SlaPerformanceStats;
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
  fraction: number;
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
  updateLevelInfo: number | null;
}

export interface QuarterlyCounts extends DataStruct {
  yearAndQuarter: string;
  counts: { incidentCount: number; queryCount: number };
}

export interface MonthlyCounts extends DataStruct {
  yearAndMonth: string;
  counts: { incidentCount: number; queryCount: number };
}

// ---- Timelogs report ----

export interface TimeLogBreakdownDetails extends DataStruct {
  projectName: string;
  projectKey: string;
  projectType: string;
  remainingQueryHours: string;
  totalQueryHours: string;
  cases: TimeLogBreakdownCase[];
}

export interface TimeLogBreakdownCase extends DataStruct {
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
