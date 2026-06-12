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
  CaseState,
  DashboardScope,
} from "@features/csm-dashboard/types/abtDashboard";
import type {
  CaseAttachment,
  CaseAuditEntry,
  CaseCustomerContext,
  CaseLinkedItem,
  CaseProductContext,
  CaseSlaClock,
  CaseTag,
  CaseTimeLogEntry,
  CaseWatcher,
  CsmCaseDetail,
  CsmCaseRow,
  CsmCasesListResponse,
} from "@features/csm-cases/types/csmCases";

const minutesAgo = (n: number): string =>
  new Date(Date.now() - n * 60_000).toISOString();

/**
 * Map a case (by subject + project name) to its affected WSO2 product. The
 * full product context (version, update level, environment) is built off this
 * in `deriveProduct` below; the list row only carries the product name.
 */
function deriveProductName(subject: string, projectName: string): string {
  const s = subject.toLowerCase();
  const p = projectName.toLowerCase();
  if (s.includes("identity server") || p.includes("iam"))
    return "WSO2 Identity Server";
  if (p.includes("asgardeo") || s.includes("asgardeo")) return "WSO2 Asgardeo";
  if (p.includes("api manager") || s.includes("api manager"))
    return "WSO2 API Manager";
  if (p.includes("choreo") || s.includes("choreo")) return "WSO2 Choreo";
  if (p.includes("integrator") || p.includes("mi") || p.includes("streaming"))
    return "WSO2 Micro Integrator";
  if (p.includes("open banking") || s.includes("openbanking"))
    return "WSO2 Open Banking";
  return "WSO2 Platform";
}

type CaseSeed = Omit<CsmCaseRow, "product" | "wso2CaseId">;

const ABT_CASE_SEEDS: CaseSeed[] = [
  {
    id: "case-1001",
    caseNumber: "CS-1001",
    subject: "Identity Server token issuance latency spike",
    customer: "Acme Financial",
    accountId: "acc-001",
    projectId: "prj-acme-iam-prod",
    projectName: "IAM Production",
    severity: "S1",
    state: "work_in_progress",
    assignee: "Sajith Ekanayaka",
    assigneeIsMe: true,
    slaClockType: "first_response",
    minutesToBreach: -22,
    createdAt: minutesAgo(60 * 8),
    updatedAt: minutesAgo(22),
  },
  {
    id: "case-1002",
    caseNumber: "CS-1002",
    subject: "API Manager gateway 502 during peak",
    customer: "Initech Systems",
    accountId: "acc-003",
    projectId: "prj-initech-apim",
    projectName: "API Manager",
    severity: "S0",
    state: "solution_proposed",
    assignee: "Sajith Ekanayaka",
    assigneeIsMe: true,
    slaClockType: "resolution",
    minutesToBreach: 12,
    createdAt: minutesAgo(60 * 3),
    updatedAt: minutesAgo(4),
  },
  {
    id: "case-1003",
    caseNumber: "CS-1003",
    subject: "MI cluster failed to start after upgrade",
    customer: "Initech Systems",
    accountId: "acc-003",
    projectId: "prj-initech-mi",
    projectName: "Micro Integrator",
    severity: "S1",
    state: "solution_proposed",
    assignee: "Sajith Ekanayaka",
    assigneeIsMe: true,
    slaClockType: "resolution",
    minutesToBreach: 600,
    createdAt: minutesAgo(60 * 6),
    updatedAt: minutesAgo(60 + 30),
  },
  {
    id: "case-1004",
    caseNumber: "CS-1004",
    subject: "OIDC userinfo claims missing groups",
    customer: "Acme Financial",
    accountId: "acc-001",
    projectId: "prj-acme-iam-prod",
    projectName: "IAM Production",
    severity: "S2",
    state: "work_in_progress",
    assignee: "Sajith Ekanayaka",
    assigneeIsMe: true,
    slaClockType: "first_response",
    minutesToBreach: 180,
    createdAt: minutesAgo(75),
    updatedAt: minutesAgo(60),
  },
  {
    id: "case-1005",
    caseNumber: "CS-1005",
    subject: "Choreo deployment stuck in 'Provisioning'",
    customer: "Umbrella Health",
    accountId: "acc-005",
    projectId: "prj-umbrella-choreo",
    projectName: "Choreo Platform",
    severity: "S2",
    state: "closed",
    assignee: "Sajith Ekanayaka",
    assigneeIsMe: true,
    slaClockType: "resolution",
    minutesToBreach: 0,
    createdAt: minutesAgo(60 * 5),
    updatedAt: minutesAgo(60),
  },
  {
    id: "case-1006",
    caseNumber: "CS-1006",
    subject: "SAML signature validation fails after SF cert rotation",
    customer: "Globex Corp",
    accountId: "acc-002",
    projectId: "prj-globex-iam",
    projectName: "IAM",
    severity: "S2",
    state: "closed",
    assignee: "Sajith Ekanayaka",
    assigneeIsMe: true,
    slaClockType: "resolution",
    minutesToBreach: 0,
    createdAt: minutesAgo(60 * 12),
    updatedAt: minutesAgo(60 * 4 + 30),
  },
  {
    id: "case-1007",
    caseNumber: "CS-1007",
    subject: "Streaming Integrator backpressure",
    customer: "Initech Systems",
    accountId: "acc-003",
    projectId: "prj-initech-si",
    projectName: "Streaming Integrator",
    severity: "S1",
    state: "awaiting_info",
    assignee: "Priya N.",
    assigneeIsMe: false,
    slaClockType: "resolution",
    minutesToBreach: -180,
    createdAt: minutesAgo(60 * 30),
    updatedAt: minutesAgo(60 * 4),
  },
  {
    id: "case-1008",
    caseNumber: "CS-1008",
    subject: "Asgardeo SCIM bulk import failing",
    customer: "Umbrella Health",
    accountId: "acc-005",
    projectId: "prj-umbrella-asgardeo",
    projectName: "Asgardeo Tenant",
    severity: "S2",
    state: "work_in_progress",
    assignee: "Dilan W.",
    assigneeIsMe: false,
    slaClockType: "first_response",
    minutesToBreach: 25,
    createdAt: minutesAgo(60 * 4),
    updatedAt: minutesAgo(35),
  },
  {
    id: "case-1009",
    caseNumber: "CS-1009",
    subject: "Customer asking about FAPI conformance",
    customer: "Acme Financial",
    accountId: "acc-001",
    projectId: "prj-acme-openbanking",
    projectName: "Open Banking",
    severity: "S3",
    state: "open",
    assignee: "Unassigned",
    assigneeIsMe: false,
    slaClockType: "ack",
    minutesToBreach: 720,
    createdAt: minutesAgo(60 * 2),
    updatedAt: minutesAgo(60 * 2),
  },
  {
    id: "case-1010",
    caseNumber: "CS-1010",
    subject: "Choreo build cache missing after region failover",
    customer: "Globex Corp",
    accountId: "acc-002",
    projectId: "prj-globex-choreo",
    projectName: "Choreo Build",
    severity: "S2",
    state: "solution_proposed",
    assignee: "Sajith Ekanayaka",
    assigneeIsMe: true,
    slaClockType: "resolution",
    minutesToBreach: 480,
    createdAt: minutesAgo(60 * 18),
    updatedAt: minutesAgo(60 * 2),
  },
  {
    id: "case-1011",
    caseNumber: "CS-1011",
    subject: "Identity Server LDAP user search slow",
    customer: "Globex Corp",
    accountId: "acc-002",
    projectId: "prj-globex-iam",
    projectName: "IAM",
    severity: "S3",
    state: "solution_proposed",
    assignee: "Sajith Ekanayaka",
    assigneeIsMe: true,
    slaClockType: "resolution",
    minutesToBreach: 1200,
    createdAt: minutesAgo(60 * 24),
    updatedAt: minutesAgo(60 * 6),
  },
  {
    id: "case-1012",
    caseNumber: "CS-1012",
    subject: "API Manager publisher UI 403 for tenant admin",
    customer: "Soylent Industries",
    accountId: "acc-004",
    projectId: "prj-soylent-apim",
    projectName: "API Manager",
    severity: "S3",
    state: "work_in_progress",
    assignee: "Tharindu A.",
    assigneeIsMe: false,
    slaClockType: "first_response",
    minutesToBreach: 360,
    createdAt: minutesAgo(60 * 3),
    updatedAt: minutesAgo(60),
  },
  {
    id: "case-1013",
    caseNumber: "CS-1013",
    subject: "Update level for IS 7.1.0 — review request",
    customer: "Acme Financial",
    accountId: "acc-001",
    projectId: "prj-acme-iam-prod",
    projectName: "IAM Production",
    severity: "S3",
    state: "open",
    assignee: "Unassigned",
    assigneeIsMe: false,
    slaClockType: "ack",
    minutesToBreach: 1080,
    createdAt: minutesAgo(60),
    updatedAt: minutesAgo(60),
  },
  {
    id: "case-1014",
    caseNumber: "CS-1014",
    subject: "Webhook signature verification failing in QA",
    customer: "Umbrella Health",
    accountId: "acc-005",
    projectId: "prj-umbrella-choreo",
    projectName: "Choreo Platform",
    severity: "S3",
    state: "work_in_progress",
    assignee: "Priya N.",
    assigneeIsMe: false,
    slaClockType: "resolution",
    minutesToBreach: 900,
    createdAt: minutesAgo(60 * 36),
    updatedAt: minutesAgo(60 * 8),
  },
  {
    id: "case-1015",
    caseNumber: "CS-1015",
    subject: "Choreo runtime OOM under sustained load",
    customer: "Initech Systems",
    accountId: "acc-003",
    projectId: "prj-initech-choreo",
    projectName: "Choreo Runtime",
    severity: "S2",
    state: "work_in_progress",
    assignee: "Sajith Ekanayaka",
    assigneeIsMe: true,
    slaClockType: "first_response",
    minutesToBreach: 60,
    createdAt: minutesAgo(60 * 2),
    updatedAt: minutesAgo(15),
  },
  {
    id: "case-1016",
    caseNumber: "CS-1016",
    subject: "How do I rotate signing keys without downtime?",
    customer: "Soylent Industries",
    accountId: "acc-004",
    projectId: "prj-soylent-iam",
    projectName: "IAM",
    severity: "S4",
    state: "open",
    assignee: "Unassigned",
    assigneeIsMe: false,
    slaClockType: "ack",
    minutesToBreach: 2160,
    createdAt: minutesAgo(60 * 6),
    updatedAt: minutesAgo(60 * 6),
  },
  {
    id: "case-1017",
    caseNumber: "CS-1017",
    subject: "Closing: migration completed successfully",
    customer: "Globex Corp",
    accountId: "acc-002",
    projectId: "prj-globex-iam",
    projectName: "IAM",
    severity: "S3",
    state: "closed",
    assignee: "Maya R.",
    assigneeIsMe: false,
    slaClockType: "resolution",
    minutesToBreach: 0,
    createdAt: minutesAgo(60 * 80),
    updatedAt: minutesAgo(60 * 10),
  },
  {
    id: "case-1018",
    caseNumber: "CS-1018",
    subject: "Tenant admin lost MFA device",
    customer: "Acme Financial",
    accountId: "acc-001",
    projectId: "prj-acme-iam-prod",
    projectName: "IAM Production",
    severity: "S2",
    state: "reopened",
    assignee: "Sajith Ekanayaka",
    assigneeIsMe: true,
    slaClockType: "ack",
    minutesToBreach: 30,
    createdAt: minutesAgo(60 * 48),
    updatedAt: minutesAgo(10),
  },
];

const ALL_EXTRA_CASE_SEEDS: CaseSeed[] = [
  {
    id: "case-2001",
    caseNumber: "CS-2001",
    subject: "Open Banking sandbox cert mismatch",
    customer: "Wayne Enterprises",
    accountId: "acc-101",
    projectId: "prj-wayne-openbanking",
    projectName: "Open Banking Sandbox",
    severity: "S1",
    state: "work_in_progress",
    assignee: "Tharindu A.",
    assigneeIsMe: false,
    slaClockType: "first_response",
    minutesToBreach: -60,
    createdAt: minutesAgo(60 * 10),
    updatedAt: minutesAgo(60 * 2),
  },
  {
    id: "case-2002",
    caseNumber: "CS-2002",
    subject: "Choreo build hanging on private dep",
    customer: "Stark Industries",
    accountId: "acc-102",
    projectId: "prj-stark-choreo",
    projectName: "Choreo CI",
    severity: "S2",
    state: "open",
    assignee: "Maya R.",
    assigneeIsMe: false,
    slaClockType: "ack",
    minutesToBreach: 75,
    createdAt: minutesAgo(60 * 3),
    updatedAt: minutesAgo(60 * 3),
  },
  {
    id: "case-2003",
    caseNumber: "CS-2003",
    subject: "API rate limiting policy not applying",
    customer: "Tyrell Corp",
    accountId: "acc-103",
    projectId: "prj-tyrell-apim",
    projectName: "API Manager",
    severity: "S3",
    state: "work_in_progress",
    assignee: "Dilan W.",
    assigneeIsMe: false,
    slaClockType: "resolution",
    minutesToBreach: 600,
    createdAt: minutesAgo(60 * 24),
    updatedAt: minutesAgo(60 * 4),
  },
  {
    id: "case-2004",
    caseNumber: "CS-2004",
    subject: "How to enable distributed tracing in IS?",
    customer: "Stark Industries",
    accountId: "acc-102",
    projectId: "prj-stark-iam",
    projectName: "IAM",
    severity: "S4",
    state: "closed",
    assignee: "Priya N.",
    assigneeIsMe: false,
    slaClockType: "resolution",
    minutesToBreach: 0,
    createdAt: minutesAgo(60 * 100),
    updatedAt: minutesAgo(60 * 40),
  },
];

/**
 * Build the project/subscription-scoped WSO2 case reference (e.g.
 * "ACMESUB-123") from the customer name + the numeric part of the case id.
 * Illustrative only — the real value comes from the BE `wso2Id` field.
 */
function deriveWso2CaseId(seed: CaseSeed): string {
  const prefix = (seed.customer.split(/\s+/)[0] ?? "WSO2")
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, "")
    .slice(0, 4);
  const num = seed.id.replace(/\D/g, "") || "0";
  return `${prefix}SUB-${num}`;
}

function hydrate(seed: CaseSeed): CsmCaseRow {
  return {
    ...seed,
    product: deriveProductName(seed.subject, seed.projectName),
    wso2CaseId: deriveWso2CaseId(seed),
  };
}

const ABT_CASES: CsmCaseRow[] = ABT_CASE_SEEDS.map(hydrate);
const ALL_EXTRA_CASES: CsmCaseRow[] = ALL_EXTRA_CASE_SEEDS.map(hydrate);

export function getMockCsmCases(scope: DashboardScope): CsmCasesListResponse {
  const cases =
    scope === "my_abt" ? ABT_CASES : [...ABT_CASES, ...ALL_EXTRA_CASES];
  return { scope, cases };
}

const ALL_CASES_BY_ID = new Map<string, CsmCaseRow>(
  [...ABT_CASES, ...ALL_EXTRA_CASES].map((c) => [c.id, c]),
);

const ALL_CASES_BY_NUMBER = new Map<string, CsmCaseRow>(
  [...ABT_CASES, ...ALL_EXTRA_CASES].map((c) => [c.caseNumber, c]),
);

export function getMockCsmCaseById(
  idOrNumber: string,
): CsmCaseRow | undefined {
  return ALL_CASES_BY_ID.get(idOrNumber) ?? ALL_CASES_BY_NUMBER.get(idOrNumber);
}

// ---------------------------------------------------------------------------
// Case detail decorations: deterministic per case id, so the same case keeps
// the same widget data across navigations.
// ---------------------------------------------------------------------------

const CUSTOMER_CONTEXTS: Record<string, CaseCustomerContext> = {
  "acc-001": {
    accountName: "Acme Financial",
    tier: "managed_cloud",
    region: "us-east-1",
    primaryContact: "Renee Park",
    primaryContactEmail: "renee.park@acmefinancial.com",
    accountManager: "Lakshmi I.",
    technicalOwner: "Chathura D.",
    openCases: 6,
  },
  "acc-002": {
    accountName: "Globex Corp",
    tier: "subscription",
    region: "eu-west-1",
    primaryContact: "Helena Voss",
    primaryContactEmail: "h.voss@globex.com",
    accountManager: "Dilan W.",
    openCases: 3,
  },
  "acc-003": {
    accountName: "Initech Systems",
    tier: "managed_cloud",
    region: "ap-southeast-1",
    primaryContact: "Peter Gibbons",
    primaryContactEmail: "peter@initech.io",
    accountManager: "Lakshmi I.",
    technicalOwner: "Tharindu A.",
    openCases: 5,
  },
  "acc-004": {
    accountName: "Soylent Industries",
    tier: "subscription",
    region: "us-west-2",
    primaryContact: "Marie Sandwitch",
    primaryContactEmail: "marie@soylent.com",
    accountManager: "Maya R.",
    openCases: 2,
  },
  "acc-005": {
    accountName: "Umbrella Health",
    tier: "saas",
    region: "us-east-1",
    primaryContact: "Albert Wesker",
    primaryContactEmail: "a.wesker@umbrella.health",
    accountManager: "Priya N.",
    openCases: 4,
  },
  "acc-101": {
    accountName: "Wayne Enterprises",
    tier: "subscription",
    region: "us-east-1",
    primaryContact: "Lucius Fox",
    primaryContactEmail: "lfox@wayne.com",
    accountManager: "Maya R.",
    openCases: 1,
  },
  "acc-102": {
    accountName: "Stark Industries",
    tier: "managed_cloud",
    region: "us-west-1",
    primaryContact: "Pepper Potts",
    primaryContactEmail: "pepper@stark.com",
    accountManager: "Dilan W.",
    technicalOwner: "Asanka R.",
    openCases: 4,
  },
  "acc-103": {
    accountName: "Tyrell Corp",
    tier: "subscription",
    region: "eu-central-1",
    primaryContact: "Eldon Tyrell",
    primaryContactEmail: "eldon@tyrell.corp",
    accountManager: "Priya N.",
    openCases: 1,
  },
};

const FALLBACK_CUSTOMER: CaseCustomerContext = {
  accountName: "Unknown Account",
  tier: "subscription",
  region: "unknown",
  primaryContact: "(no primary contact)",
  primaryContactEmail: "—",
  accountManager: "Unassigned",
  openCases: 1,
};

// Map the coarse environment lane to a fixed-list deployment category. Prod
// deployments are treated as primary production in the mock.
const CATEGORY_FROM_ENV: Record<
  CaseProductContext["environment"],
  NonNullable<CaseProductContext["deploymentCategory"]>
> = {
  prod: "primary_production",
  staging: "staging",
  qa: "qa",
  dev: "development",
};

function deriveProduct(c: CsmCaseRow): CaseProductContext {
  const base = deriveProductBase(c);
  return { ...base, deploymentCategory: CATEGORY_FROM_ENV[base.environment] };
}

function deriveProductBase(c: CsmCaseRow): CaseProductContext {
  const subject = c.subject.toLowerCase();
  const project = c.projectName.toLowerCase();
  if (subject.includes("identity server") || project.includes("iam")) {
    return {
      product: "WSO2 Identity Server",
      version: "7.1.0",
      updateLevel: "wso2is-7.1.0.42",
      deployment: c.projectName,
      environment: "prod",
      region: "us-east-1",
    };
  }
  if (project.includes("asgardeo") || subject.includes("asgardeo")) {
    return {
      product: "WSO2 Asgardeo",
      version: "saas",
      deployment: c.projectName,
      environment: "prod",
      region: "us-east-1",
    };
  }
  if (project.includes("api manager") || subject.includes("api manager")) {
    return {
      product: "WSO2 API Manager",
      version: "4.4.0",
      updateLevel: "wso2am-4.4.0.18",
      deployment: c.projectName,
      environment: "prod",
      region: "eu-west-1",
    };
  }
  if (project.includes("choreo") || subject.includes("choreo")) {
    return {
      product: "WSO2 Choreo",
      version: "saas",
      deployment: c.projectName,
      environment: "prod",
      region: "us-east-1",
    };
  }
  if (project.includes("integrator") || project.includes("mi") || project.includes("streaming")) {
    return {
      product: "WSO2 Micro Integrator",
      version: "4.4.0",
      updateLevel: "wso2mi-4.4.0.7",
      deployment: c.projectName,
      environment: "staging",
      region: "us-east-1",
    };
  }
  if (project.includes("open banking") || subject.includes("openbanking")) {
    return {
      product: "WSO2 Open Banking",
      version: "4.0.0",
      updateLevel: "wso2ob-4.0.0.21",
      deployment: c.projectName,
      environment: "prod",
      region: "eu-west-1",
    };
  }
  return {
    product: "WSO2 Platform",
    version: "n/a",
    deployment: c.projectName,
    environment: "prod",
  };
}

// SLA targets vary by severity. Numbers below are illustrative for the mock.
function deriveSlaClocks(c: CsmCaseRow): CaseSlaClock[] {
  const ackTarget = c.severity === "S0" ? 15 : c.severity === "S1" ? 30 : c.severity === "S2" ? 60 : 240;
  const frTarget = c.severity === "S0" ? 30 : c.severity === "S1" ? 60 : c.severity === "S2" ? 240 : 480;
  const resTarget =
    c.severity === "S0" ? 240 : c.severity === "S1" ? 480 : c.severity === "S2" ? 1440 : 4320;

  return [
    {
      clockType: "ack",
      // Ack is generally met once we've replied. Use the case's own clock
      // to drive the active stage so the timeline matches the header chip.
      state: c.slaClockType === "ack" ? (c.minutesToBreach < 0 ? "breached" : "running") : "met",
      minutesToBreach: c.slaClockType === "ack" ? c.minutesToBreach : 0,
      targetMinutes: ackTarget,
    },
    {
      clockType: "first_response",
      state:
        c.slaClockType === "first_response"
          ? c.minutesToBreach < 0
            ? "breached"
            : "running"
          : c.slaClockType === "resolution" || c.state === "closed"
            ? "met"
            : "paused",
      minutesToBreach: c.slaClockType === "first_response" ? c.minutesToBreach : 0,
      targetMinutes: frTarget,
    },
    {
      clockType: "resolution",
      state:
        c.state === "closed"
          ? "met"
          : c.slaClockType === "resolution"
            ? c.minutesToBreach < 0
              ? "breached"
              : "running"
            : "paused",
      minutesToBreach: c.slaClockType === "resolution" ? c.minutesToBreach : resTarget,
      targetMinutes: resTarget,
    },
  ];
}

const ALL_WATCHERS: CaseWatcher[] = [
  { id: "w-1", name: "Sajith Ekanayaka", role: "wso2_engineer", isMe: true },
  { id: "w-2", name: "Lakshmi I.", role: "manager" },
  { id: "w-3", name: "Asanka R.", role: "wso2_engineer" },
  { id: "w-4", name: "Priya N.", role: "wso2_engineer" },
  { id: "w-5", name: "Renee Park", role: "customer_contact" },
  { id: "w-6", name: "Maya R.", role: "wso2_engineer" },
  { id: "w-7", name: "Dilan W.", role: "wso2_engineer" },
];

function deriveWatchers(c: CsmCaseRow): CaseWatcher[] {
  // Always include the assignee if known + 2-3 more deterministically.
  const base: CaseWatcher[] = [];
  if (c.assigneeIsMe) base.push({ id: "w-1", name: c.assignee, role: "wso2_engineer", isMe: true });
  else if (c.assignee !== "Unassigned")
    base.push({ id: `w-assignee-${c.id}`, name: c.assignee, role: "wso2_engineer" });
  const seedIdx = parseInt(c.id.replace(/\D/g, ""), 10) || 0;
  const pool = ALL_WATCHERS.filter((w) => !base.some((b) => b.name === w.name));
  const extras = [
    pool[seedIdx % pool.length],
    pool[(seedIdx + 2) % pool.length],
    pool[(seedIdx + 3) % pool.length],
  ];
  const seen = new Set<string>(base.map((w) => w.name));
  for (const w of extras) {
    if (w && !seen.has(w.name)) {
      base.push(w);
      seen.add(w.name);
    }
  }
  return base;
}

function deriveLinkedItems(c: CsmCaseRow): CaseLinkedItem[] {
  const seedIdx = parseInt(c.id.replace(/\D/g, ""), 10) || 0;
  const linked: CaseLinkedItem[] = [];

  // Most cases get 1-2 related cases from the same account.
  const sameAccount = [...ALL_CASES_BY_ID.values()].filter(
    (other) => other.accountId === c.accountId && other.id !== c.id,
  );
  for (let i = 0; i < Math.min(2, sameAccount.length); i += 1) {
    const r = sameAccount[(seedIdx + i) % sameAccount.length];
    linked.push({
      id: `link-${c.id}-${r.id}`,
      kind: "case",
      reference: r.caseNumber,
      title: r.subject,
      state: r.state,
      href: `/cases/${r.id}`,
    });
  }

  // High-severity SaaS cases get a linked incident.
  if (
    (c.severity === "S0" || c.severity === "S1") &&
    (c.projectName.toLowerCase().includes("choreo") ||
      c.projectName.toLowerCase().includes("asgardeo"))
  ) {
    linked.push({
      id: `link-${c.id}-inc`,
      kind: "incident",
      reference: `INC-${5000 + seedIdx}`,
      title: "Regional latency event",
      state: "investigating",
    });
  }

  // Breached cases get an escalation.
  if (c.minutesToBreach < 0 || c.severity === "S0") {
    linked.push({
      id: `link-${c.id}-esc`,
      kind: "escalation",
      reference: `ESC-${300 + seedIdx}`,
      title: "Customer-raised escalation",
      state: "open",
    });
  }

  // ~30% get a KB article.
  if (seedIdx % 3 === 0) {
    linked.push({
      id: `link-${c.id}-kb`,
      kind: "kb",
      reference: `KB-${10000 + seedIdx}`,
      title: "Troubleshooting playbook",
      state: "published",
    });
  }
  return linked;
}

function deriveTags(c: CsmCaseRow): CaseTag[] {
  const tags: CaseTag[] = [];
  if (c.severity === "S0" || c.severity === "S1")
    tags.push({ id: "t-priority", label: "high-priority", color: "error" });
  if (c.minutesToBreach < 0)
    tags.push({ id: "t-breach", label: "sla-breached", color: "warning" });
  if (c.state === "reopened")
    tags.push({ id: "t-reopen", label: "reopened", color: "warning" });
  const subjLower = c.subject.toLowerCase();
  if (subjLower.includes("ldap")) tags.push({ id: "t-ldap", label: "ldap", color: "info" });
  if (subjLower.includes("saml") || subjLower.includes("oidc"))
    tags.push({ id: "t-fed", label: "federation", color: "info" });
  if (subjLower.includes("latency") || subjLower.includes("oom"))
    tags.push({ id: "t-perf", label: "performance", color: "primary" });
  if (subjLower.includes("certificate") || subjLower.includes("cert"))
    tags.push({ id: "t-pki", label: "pki", color: "primary" });
  return tags;
}

function deriveTimeLogs(c: CsmCaseRow): CaseTimeLogEntry[] {
  const seedIdx = parseInt(c.id.replace(/\D/g, ""), 10) || 0;
  const assignee = c.assignee === "Unassigned" ? "Sajith Ekanayaka" : c.assignee;
  const totalMinutes = (seedIdx * 17) % 480 + 30;
  const logs: CaseTimeLogEntry[] = [
    {
      id: `tl-${c.id}-a`,
      engineer: assignee,
      hours: Math.round((totalMinutes / 60) * 4) / 4,
      note: "Investigated logs, gathered customer-side traces.",
      date: new Date(Date.now() - 60 * 60 * 1000 * 6).toISOString(),
    },
  ];
  if (c.state !== "open") {
    logs.unshift({
      id: `tl-${c.id}-b`,
      engineer: assignee,
      hours: 1.5,
      note: "Reviewed customer config and reproduction steps.",
      date: new Date(Date.now() - 60 * 60 * 1000 * 22).toISOString(),
    });
  }
  if (c.state === "solution_proposed" || c.state === "closed") {
    logs.unshift({
      id: `tl-${c.id}-c`,
      engineer: assignee,
      hours: 2,
      note: "Drafted proposed fix and verified in staging.",
      date: new Date(Date.now() - 60 * 60 * 1000 * 2).toISOString(),
    });
  }
  return logs;
}

function deriveAudit(c: CsmCaseRow): CaseAuditEntry[] {
  const assignee = c.assignee === "Unassigned" ? "(unassigned)" : c.assignee;
  const created: CaseAuditEntry = {
    id: `a-${c.id}-0`,
    kind: "created",
    actor: "Customer",
    description: `Case created at severity ${c.severity}`,
    createdAt: c.createdAt,
  };
  const events: CaseAuditEntry[] = [created];
  if (c.state !== "open") {
    events.push({
      id: `a-${c.id}-1`,
      kind: "assignee_change",
      actor: "Routing",
      description: `Assigned to ${assignee}`,
      createdAt: new Date(new Date(c.createdAt).getTime() + 5 * 60_000).toISOString(),
    });
    events.push({
      id: `a-${c.id}-2`,
      kind: "state_change",
      actor: assignee,
      description: "Moved to Work in progress",
      createdAt: new Date(new Date(c.createdAt).getTime() + 15 * 60_000).toISOString(),
    });
  }
  if (c.state === "awaiting_info") {
    events.push({
      id: `a-${c.id}-3`,
      kind: "state_change",
      actor: assignee,
      description: "Requested additional info from customer",
      createdAt: c.updatedAt,
    });
  }
  if (c.state === "waiting_on_wso2") {
    events.push({
      id: `a-${c.id}-3`,
      kind: "state_change",
      actor: assignee,
      description: "Marked as waiting on internal WSO2 dependency",
      createdAt: c.updatedAt,
    });
  }
  if (c.state === "solution_proposed") {
    events.push({
      id: `a-${c.id}-3`,
      kind: "state_change",
      actor: assignee,
      description: "Posted proposed solution to customer",
      createdAt: c.updatedAt,
    });
  }
  if (c.state === "closed") {
    events.push({
      id: `a-${c.id}-3`,
      kind: "state_change",
      actor: assignee,
      description: "Case closed",
      createdAt: c.updatedAt,
    });
  }
  if (c.state === "reopened") {
    events.push({
      id: `a-${c.id}-3`,
      kind: "state_change",
      actor: "Customer",
      description: "Customer reopened the case",
      createdAt: c.updatedAt,
    });
  }
  if (c.minutesToBreach < 0 && c.state !== "closed") {
    events.push({
      id: `a-${c.id}-breach`,
      kind: "sla_breached",
      actor: "System",
      description: `${c.slaClockType.replace("_", " ")} SLA breached`,
      createdAt: c.updatedAt,
    });
  }
  return events;
}

const DESCRIPTIONS: Record<string, string> = {
  "case-1001":
    `Hi WSO2 support,

Token issuance latency on our corporate IdP has spiked to ~2.5s p95 since 09:30 UTC. This is impacting workforce SSO across all internal apps — engineers can't get into Jira, GitHub, or the deployment console.

What we've checked so far:
  - Mutual TLS handshake times look normal (~40ms)
  - JDBC userstore connection pool sits at 18/100 — not exhausted
  - GC pauses on IS nodes are under 50ms, heap usage at 45%
  - No deployment on our side in the last 72 hours
  - LDAP-backed userstore latency unchanged

We tried rolling restart of the IS cluster at 10:15 UTC; latency dropped for ~3 minutes then climbed back. Attaching the access log slice from 09:00–10:30 UTC and a thread dump from is-prod-2 captured during the spike.

Please treat as high priority — our CISO is asking for an ETA.

Thanks,
Rohan Mehta
Platform Engineering, Acme Financial`,
  "case-1002":
    `Hi WSO2 team,

API Manager gateway is returning intermittent 502s to roughly 3% of traffic during our 11:00–13:00 UTC peak window. Backend services behind the gateway are healthy (5xx-free, latency normal). Looks like the gateway itself is dropping connections.

Observed pattern:
  - 5xx rate climbs starting ~11:05 UTC, peaks around 12:20 UTC
  - Two of three gateway pods show CPU at ~85% during the spike
  - Heap usage steady at 60%, no OOM
  - Suspect upstream connection pool saturation but we can't see pool metrics from outside

Can you take a look at the gateway pod logs and pool config? We have the change window open if you need us to apply anything.

Best regards,
Janet Park
Initech SRE`,
  "case-1003":
    `Hi WSO2 support,

Our Micro Integrator cluster fails to start after we applied update level wso2mi-4.4.0.7 last night. Two pods stay in CrashLoopBackOff.

The carbon log shows a ClassNotFoundException on org.wso2.carbon.security.user.api.UserStoreException coming from a custom security extension we built about a year ago. The extension was working fine on wso2mi-4.4.0.4.

Can you tell us what changed in 4.4.0.7 around the user-API classloader? If it's a class move or rename, we can rebuild the extension; just need to know what to retarget.

We're holding the rollout to other regions until we hear back.

Thanks,
Bill Lumbergh
Initech Platform Team`,
  "case-1004":
    `Hi WSO2 support,

The userinfo endpoint stopped returning the "groups" claim for users authenticated via our corporate IdP. This worked last week and we haven't changed our client config.

Repro:
  1. Authenticate as bob@acmefinancial.com via the corporate IdP federation
  2. Exchange code for tokens at /oauth2/token
  3. Call /oauth2/userinfo with the access token
  4. Response includes sub, email, name — but no "groups"

Expected: groups should be a JSON array of LDAP DN strings.

Could a federated IdP attribute mapping have been changed, or is this a known issue in 7.1.0? Happy to provide a test user account if it helps.

Best regards,
Rohan Mehta
Acme Financial`,
  "case-1005":
    `Hi support,

Our Choreo deployment for the "checkout-v2" component has been stuck at "Provisioning" for about 45 minutes. The component is a Go service, build succeeded, but the pod never reaches Ready.

We've already tried:
  - Deleting and redeploying — same outcome
  - Smaller resource ask (256Mi → 128Mi) — same outcome
  - Switching the build profile — no effect

There's nothing useful in the Choreo UI beyond "Provisioning". Could you check the underlying pod state? I'll attach kubectl describe output if you can pull it from the platform side.

Thanks,
Daniel Owens
Engineering, Acme Choreo`,
  "case-1015":
    `Hi WSO2 support,

Our Choreo runtime is going OOM under sustained load (~120 RPS sustained, no bursts). The crash takes the whole pod down and the autoscaler can't keep up — we're seeing ~40-minute cycles of healthy / OOM / new pod.

Started this morning around 05:40 UTC right after the rollout of release v4.2.1. Reverting to v4.2.0 resolves it; we've confirmed the rollback on staging.

Profile so far:
  - All pods OOM under steady-state, no specific instance pattern
  - 2Gi memory limit, 1Gi request, 1500m CPU limit
  - Workload is 117–122 RPS for the last 6 hours of traffic
  - Heap dump captured (heap-prod-01.hprof, ~480MB) — attaching

Need this resolved or worked around urgently — our SLO budget for the month is ~70% spent already.

Thanks,
Daniel Owens
Engineering, Acme Choreo`,
  "case-1006":
    `Hi WSO2 team,

We're seeing the same SAML response signature validation error on the Asgardeo tenant for our staging Salesforce SP. The error in the IS logs is:

  org.wso2.carbon.identity.sso.saml.exception.IdentityException: Signature validation failed for SAML response

This started after we rotated our Salesforce signing certificate yesterday. We re-uploaded the new cert into the SP configuration in Asgardeo. The new cert is RSA-2048, SHA-256 signed.

Production tenant is still on the old cert and working fine; this is staging only. Can you check whether the cert metadata was actually picked up, or if there's a propagation delay we should wait through?

Best regards,
Helena Voss
Globex IAM Team`,
  "case-1011":
    `Hi WSO2 support,

We need help tuning the LDAP userstore for our IS deployment. Search queries against the userstore are coming back in 1.2–1.8s for queries that should be instant. We have ~85,000 user entries in the LDAP.

What we've configured:
  - ConnectionPoolingEnabled = true
  - MaxActiveConnections = 50
  - ConnectionPoolMinIdle = 10
  - SearchScope = SUB on ou=people,dc=globex,dc=com

Index status on the LDAP side looks fine — pres/sub/eq indexes on uid, mail, cn, member. Network RTT to the LDAP from IS pods is consistent at ~2ms.

Suspect we're paginating poorly or doing redundant searches. Can you advise on the right pooling config + tracing the actual LDAP queries IS is firing?

Thanks,
Marcus Liang
Globex Security Engineering`,
};

function describe(c: CsmCaseRow): string {
  const seeded = DESCRIPTIONS[c.id];
  if (seeded) return seeded;

  // Fallback template: greeting + body + sign-off so every case reads like
  // a real ticket even without a hand-written description. The greeting is
  // intentionally generic — the customer doesn't know who will pick up the
  // case at the time of creation.
  const customerFirst = c.customer.split(/\s+/)[0] ?? c.customer;
  return [
    `Hi WSO2 support,`,
    "",
    `${c.subject} — reported on project ${c.projectName}.`,
    "",
    "What we're observing:",
    `  - Issue first noticed earlier today during normal operations`,
    `  - Severity ${c.severity} based on customer impact`,
    `  - Workaround status: investigating, no permanent mitigation yet`,
    "",
    "Additional reproduction details and logs are captured in the comment thread below. Happy to jump on a call if it helps move things along.",
    "",
    "Thanks,",
    `${customerFirst}`,
    `${c.customer}`,
  ].join("\n");
}

function deriveAttachments(c: CsmCaseRow): CaseAttachment[] {
  const seedIdx = parseInt(c.id.replace(/\D/g, ""), 10) || 0;
  const sets: Record<number, CaseAttachment[]> = {
    0: [
      {
        id: `att-${c.id}-1`,
        filename: "thread-dump.log",
        size: 154_321,
        contentType: "text/plain",
        uploadedBy: c.customer.split(" ")[0],
        uploadedAt: new Date(Date.now() - 60 * 60 * 1000 * 5).toISOString(),
      },
    ],
    1: [
      {
        id: `att-${c.id}-1`,
        filename: "gw-pod-metrics.csv",
        size: 21_440,
        contentType: "text/csv",
        uploadedBy: c.customer.split(" ")[0],
        uploadedAt: new Date(Date.now() - 60 * 60 * 1000 * 3).toISOString(),
      },
      {
        id: `att-${c.id}-2`,
        filename: "carbon.log",
        size: 982_133,
        contentType: "text/plain",
        uploadedBy: c.assignee === "Unassigned" ? "Sajith Ekanayaka" : c.assignee,
        uploadedAt: new Date(Date.now() - 60 * 60 * 1000 * 2).toISOString(),
      },
    ],
    2: [
      {
        id: `att-${c.id}-1`,
        filename: "screenshot.png",
        size: 312_558,
        contentType: "image/png",
        uploadedBy: c.customer.split(" ")[0],
        uploadedAt: new Date(Date.now() - 60 * 60 * 1000 * 1.2).toISOString(),
      },
    ],
    3: [],
  };
  return sets[seedIdx % 4] ?? [];
}

/**
 * Mirror of the backend transition graph in
 * `apps/csm-portal/backend/internal/handler/state.go` (`nextStates`). The mock
 * stands in for the backend, so it owns this copy the way the real backend does;
 * the webapp itself never re-derives the graph — it renders from `nextStates`.
 */
const MOCK_NEXT_STATES: Record<CaseState, CaseState[]> = {
  open: ["work_in_progress"],
  work_in_progress: [
    "waiting_on_wso2",
    "awaiting_info",
    "solution_proposed",
    "closed",
  ],
  waiting_on_wso2: ["work_in_progress"],
  awaiting_info: ["waiting_on_wso2"],
  reopened: ["waiting_on_wso2"],
  solution_proposed: ["closed", "waiting_on_wso2"],
  closed: [],
};

export function getMockCsmCaseDetailById(
  idOrNumber: string,
): CsmCaseDetail | undefined {
  const row = getMockCsmCaseById(idOrNumber);
  if (!row) return undefined;
  const customerContext = CUSTOMER_CONTEXTS[row.accountId] ?? FALLBACK_CUSTOMER;
  return {
    ...row,
    nextStates: MOCK_NEXT_STATES[row.state] ?? [],
    description: describe(row),
    assignmentGroup: row.projectName.toLowerCase().includes("choreo")
      ? "grp.choreo_sre"
      : row.projectName.toLowerCase().includes("asgardeo")
        ? "grp.asgardeo_sre"
        : "grp.cre_team",
    createdBy: customerContext.primaryContact,
    customerContext,
    productContext: deriveProduct(row),
    slaClocks: deriveSlaClocks(row),
    watchers: deriveWatchers(row),
    linkedItems: deriveLinkedItems(row),
    tags: deriveTags(row),
    timeLogs: deriveTimeLogs(row),
    audit: deriveAudit(row),
    attachments: deriveAttachments(row),
    isWatching: row.assigneeIsMe,
  };
}
