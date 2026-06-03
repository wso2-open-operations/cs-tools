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

import type {
  CsmAbtDashboardData,
  CsmCustomerSummary,
  CsmQueueCase,
  CsmRecentActivity,
  CsmSlaAtRiskCase,
  DashboardScope,
} from "@features/csm-dashboard/types/abtDashboard";

const minutesAgo = (n: number): string =>
  new Date(Date.now() - n * 60_000).toISOString();

const ABT_CUSTOMERS: CsmCustomerSummary[] = [
  {
    accountId: "acc-001",
    accountName: "Acme Financial",
    tier: "Platinum",
    openCaseCount: 14,
    s0s1Count: 2,
    breachedCount: 1,
    lastActivityAt: minutesAgo(18),
  },
  {
    accountId: "acc-002",
    accountName: "Globex Corp",
    tier: "Gold",
    openCaseCount: 7,
    s0s1Count: 0,
    breachedCount: 0,
    lastActivityAt: minutesAgo(95),
  },
  {
    accountId: "acc-003",
    accountName: "Initech Systems",
    tier: "Platinum",
    openCaseCount: 21,
    s0s1Count: 3,
    breachedCount: 2,
    lastActivityAt: minutesAgo(7),
  },
  {
    accountId: "acc-004",
    accountName: "Soylent Industries",
    tier: "Silver",
    openCaseCount: 4,
    s0s1Count: 0,
    breachedCount: 0,
    lastActivityAt: minutesAgo(240),
  },
  {
    accountId: "acc-005",
    accountName: "Umbrella Health",
    tier: "Gold",
    openCaseCount: 9,
    s0s1Count: 1,
    breachedCount: 0,
    lastActivityAt: minutesAgo(52),
  },
];

const ALL_EXTRA_CUSTOMERS: CsmCustomerSummary[] = [
  {
    accountId: "acc-101",
    accountName: "Wayne Enterprises",
    tier: "Platinum",
    openCaseCount: 11,
    s0s1Count: 1,
    breachedCount: 0,
    lastActivityAt: minutesAgo(33),
  },
  {
    accountId: "acc-102",
    accountName: "Stark Industries",
    tier: "Gold",
    openCaseCount: 6,
    s0s1Count: 0,
    breachedCount: 0,
    lastActivityAt: minutesAgo(180),
  },
  {
    accountId: "acc-103",
    accountName: "Tyrell Corp",
    tier: "Silver",
    openCaseCount: 3,
    s0s1Count: 0,
    breachedCount: 0,
    lastActivityAt: minutesAgo(420),
  },
];

const QUEUE_TOP_CASES: CsmQueueCase[] = [
  {
    id: "case-1001",
    caseNumber: "CS-1001",
    subject: "Identity Server token issuance latency spike",
    customer: "Acme Financial",
    severity: "S1",
    state: "work_in_progress",
    slaClockType: "first_response",
    minutesToBreach: -22,
  },
  {
    id: "case-1002",
    caseNumber: "CS-1002",
    subject: "API Manager gateway 502 during peak",
    customer: "Initech Systems",
    severity: "S0",
    state: "work_in_progress",
    slaClockType: "resolution",
    minutesToBreach: 12,
  },
  {
    id: "case-1003",
    caseNumber: "CS-1003",
    subject: "MI cluster failed to start after upgrade",
    customer: "Initech Systems",
    severity: "S1",
    state: "awaiting_info",
    slaClockType: "ack",
    minutesToBreach: 47,
  },
  {
    id: "case-1004",
    caseNumber: "CS-1004",
    subject: "OIDC userinfo claims missing groups",
    customer: "Acme Financial",
    severity: "S2",
    state: "open",
    slaClockType: "ack",
    minutesToBreach: 90,
  },
  {
    id: "case-1005",
    caseNumber: "CS-1005",
    subject: "Choreo deployment stuck in 'Provisioning'",
    customer: "Umbrella Health",
    severity: "S2",
    state: "work_in_progress",
    slaClockType: "first_response",
    minutesToBreach: 145,
  },
];

const SLA_AT_RISK_ABT: CsmSlaAtRiskCase[] = [
  {
    id: "case-1001",
    caseNumber: "CS-1001",
    subject: "Identity Server token issuance latency spike",
    customer: "Acme Financial",
    severity: "S1",
    assignee: "Me",
    slaClockType: "first_response",
    minutesToBreach: -22,
    state: "work_in_progress",
  },
  {
    id: "case-1002",
    caseNumber: "CS-1002",
    subject: "API Manager gateway 502 during peak",
    customer: "Initech Systems",
    severity: "S0",
    assignee: "Me",
    slaClockType: "resolution",
    minutesToBreach: 12,
    state: "work_in_progress",
  },
  {
    id: "case-1007",
    caseNumber: "CS-1007",
    subject: "Streaming Integrator backpressure",
    customer: "Initech Systems",
    severity: "S1",
    assignee: "Priya N.",
    slaClockType: "resolution",
    minutesToBreach: -180,
    state: "awaiting_info",
  },
  {
    id: "case-1008",
    caseNumber: "CS-1008",
    subject: "Asgardeo SCIM bulk import failing",
    customer: "Umbrella Health",
    severity: "S2",
    assignee: "Dilan W.",
    slaClockType: "first_response",
    minutesToBreach: 25,
    state: "work_in_progress",
  },
];

const SLA_AT_RISK_ALL_EXTRA: CsmSlaAtRiskCase[] = [
  {
    id: "case-2001",
    caseNumber: "CS-2001",
    subject: "Open Banking sandbox cert mismatch",
    customer: "Wayne Enterprises",
    severity: "S1",
    assignee: "Tharindu A.",
    slaClockType: "first_response",
    minutesToBreach: -60,
    state: "work_in_progress",
  },
  {
    id: "case-2002",
    caseNumber: "CS-2002",
    subject: "Choreo build hanging on private dep",
    customer: "Stark Industries",
    severity: "S2",
    assignee: "Maya R.",
    slaClockType: "ack",
    minutesToBreach: 75,
    state: "open",
  },
];

const RECENT_ACTIVITY_ABT: CsmRecentActivity[] = [
  {
    id: "act-1",
    caseId: "case-1002",
    caseNumber: "CS-1002",
    customer: "Initech Systems",
    type: "comment",
    who: "Customer (J. Doe)",
    whenAt: minutesAgo(4),
    summary: "Provided thread dump and gateway logs from peak window.",
  },
  {
    id: "act-2",
    caseId: "case-1001",
    caseNumber: "CS-1001",
    customer: "Acme Financial",
    type: "sla_breach",
    who: "System",
    whenAt: minutesAgo(22),
    summary: "First-response SLA breached.",
  },
  {
    id: "act-3",
    caseId: "case-1003",
    caseNumber: "CS-1003",
    customer: "Initech Systems",
    type: "state_change",
    who: "Me",
    whenAt: minutesAgo(48),
    summary: "Moved to awaiting_info — requested replication steps.",
  },
  {
    id: "act-4",
    caseId: "case-1004",
    caseNumber: "CS-1004",
    customer: "Acme Financial",
    type: "case_created",
    who: "Customer (R. Patel)",
    whenAt: minutesAgo(75),
    summary: "New case opened: OIDC userinfo claims missing groups.",
  },
  {
    id: "act-5",
    caseId: "case-1005",
    caseNumber: "CS-1005",
    customer: "Umbrella Health",
    type: "comment",
    who: "Me",
    whenAt: minutesAgo(115),
    summary: "Asked customer for kubectl describe output on stuck pod.",
  },
];

const ENGINEER = {
  name: "Sajith Ekanayaka",
  email: "sajithe@wso2.com",
  abtName: "ABT-IAM-East",
};

const sumQueue = (cases: CsmQueueCase[]) => {
  let actionRequired = 0;
  let inProgress = 0;
  let awaitingInfo = 0;
  for (const c of cases) {
    if (c.state === "open" || c.state === "reopen") actionRequired += 1;
    else if (c.state === "work_in_progress") inProgress += 1;
    else if (c.state === "awaiting_info") awaitingInfo += 1;
  }
  return { actionRequired, inProgress, awaitingInfo };
};

export function getMockAbtDashboard(
  scope: DashboardScope,
): CsmAbtDashboardData {
  const customers =
    scope === "my_abt"
      ? ABT_CUSTOMERS
      : [...ABT_CUSTOMERS, ...ALL_EXTRA_CUSTOMERS];

  const slaAtRisk =
    scope === "my_abt"
      ? SLA_AT_RISK_ABT
      : [...SLA_AT_RISK_ABT, ...SLA_AT_RISK_ALL_EXTRA];

  // My Queue is always "me", independent of scope.
  const { actionRequired, inProgress, awaitingInfo } = sumQueue(QUEUE_TOP_CASES);
  const totalOpen = actionRequired + inProgress + awaitingInfo;

  return {
    engineer: ENGINEER,
    scope,
    queue: {
      actionRequiredCount: actionRequired,
      inProgressCount: inProgress,
      awaitingInfoCount: awaitingInfo,
      totalOpenCount: totalOpen,
      topCases: QUEUE_TOP_CASES,
    },
    slaAtRisk,
    customers,
    recentActivity: RECENT_ACTIVITY_ABT,
  };
}
