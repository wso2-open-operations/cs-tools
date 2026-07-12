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
  | "closed"
  /**
   * Only ever appears as a `nextStates` entry on a closed case, never as a
   * case's own `state` — it signals "Create related case" is available, not
   * an actual reopen (the data source has no such transition). See
   * `CsmCaseDetail.nextStates` and `CaseActionBar`'s `reopened` handling.
   */
  | "reopened";

/**
 * Work sub-state of a `work_in_progress` case. `null` / absent when the case is
 * not in progress. Mirrors the entity-service `CaseWorkState` enum; the backend
 * gates comment posting on `work_in_progress` + `ongoing`.
 */
export type CaseWorkState = "ongoing" | "paused";

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
  assignee: string;
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

// Multi-dashboard switcher — mirrors ServiceNow Performance Analytics where
// engineers pivot between several dashboards (Engineer / Operations / IAM /
// Security / Team Performance). See DashboardsAndReportsProposal.md.
export type DashboardKey =
  | "engineer"
  | "operations"
  | "iam"
  | "security"
  | "team_performance";

export interface DashboardOption {
  key: DashboardKey;
  name: string;
  description: string;
  scopeBased: boolean;
}

export const DASHBOARD_OPTIONS: DashboardOption[] = [
  {
    key: "engineer",
    name: "Engineer overview",
    description: "Personal queue, SLA at risk, customers in scope, recent activity.",
    scopeBased: true,
  },
  {
    key: "operations",
    name: "Operations",
    description: "Cross-team case throughput, state distribution, escalations, SLA breach trends.",
    scopeBased: false,
  },
  {
    key: "iam",
    name: "IAM CS",
    description: "Identity Server / Asgardeo case posture, top accounts, vulnerability links.",
    scopeBased: false,
  },
  {
    key: "security",
    name: "Security center",
    description: "Vulnerability posture, security report cases, response time.",
    scopeBased: false,
  },
  {
    key: "team_performance",
    name: "Team performance",
    description: "Per-team throughput, time-card distribution, on-call coverage gaps.",
    scopeBased: false,
  },
];
