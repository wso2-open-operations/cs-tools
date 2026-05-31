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

export type Severity = "S0" | "S1" | "S2" | "S3" | "S4";

export type CaseState =
  | "open"
  | "work_in_progress"
  | "solution_proposed"
  | "awaiting_info"
  | "waiting_on_wso2"
  | "reopen"
  | "closed";

export type SlaClockType = "ack" | "first_response" | "resolution";

export type DashboardScope = "my_abt" | "all_customers";

export interface CsmDashboardEngineer {
  name: string;
  email: string;
  abtName: string;
}

export interface CsmQueueCase {
  id: string;
  caseNumber: string;
  subject: string;
  customer: string;
  severity: Severity;
  state: CaseState;
  slaClockType: SlaClockType;
  // Minutes until the active SLA clock breaches. Negative => already breached.
  minutesToBreach: number;
}

export interface CsmQueueSummary {
  actionRequiredCount: number;
  inProgressCount: number;
  awaitingInfoCount: number;
  totalOpenCount: number;
  topCases: CsmQueueCase[];
}

export interface CsmSlaAtRiskCase {
  id: string;
  caseNumber: string;
  subject: string;
  customer: string;
  severity: Severity;
  owner: string;
  slaClockType: SlaClockType;
  minutesToBreach: number;
  state: CaseState;
}

export interface CsmCustomerSummary {
  accountId: string;
  accountName: string;
  tier: string;
  openCaseCount: number;
  s0s1Count: number;
  breachedCount: number;
  lastActivityAt: string;
}

export type CsmRecentActivityType =
  | "comment"
  | "state_change"
  | "case_created"
  | "case_closed"
  | "sla_breach";

export interface CsmRecentActivity {
  id: string;
  caseId: string;
  caseNumber: string;
  customer: string;
  type: CsmRecentActivityType;
  who: string;
  whenAt: string;
  summary: string;
}

export interface CsmAbtDashboardData {
  engineer: CsmDashboardEngineer;
  scope: DashboardScope;
  queue: CsmQueueSummary;
  slaAtRisk: CsmSlaAtRiskCase[];
  customers: CsmCustomerSummary[];
  recentActivity: CsmRecentActivity[];
}
