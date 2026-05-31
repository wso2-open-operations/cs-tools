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

import type { DashboardScope } from "@features/csm-dashboard/types/abtDashboard";
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

const ABT_CASES: CsmCaseRow[] = [
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
    owner: "Sajith Ekanayaka",
    ownerIsMe: true,
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
    state: "work_in_progress",
    owner: "Sajith Ekanayaka",
    ownerIsMe: true,
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
    state: "awaiting_info",
    owner: "Sajith Ekanayaka",
    ownerIsMe: true,
    slaClockType: "ack",
    minutesToBreach: 47,
    createdAt: minutesAgo(60 * 6),
    updatedAt: minutesAgo(48),
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
    state: "open",
    owner: "Unassigned",
    ownerIsMe: false,
    slaClockType: "ack",
    minutesToBreach: 90,
    createdAt: minutesAgo(75),
    updatedAt: minutesAgo(75),
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
    state: "work_in_progress",
    owner: "Sajith Ekanayaka",
    ownerIsMe: true,
    slaClockType: "first_response",
    minutesToBreach: 145,
    createdAt: minutesAgo(60 * 5),
    updatedAt: minutesAgo(115),
  },
  {
    id: "case-1006",
    caseNumber: "CS-1006",
    subject: "SAML logout returning 500 intermittently",
    customer: "Acme Financial",
    accountId: "acc-001",
    projectId: "prj-acme-iam-prod",
    projectName: "IAM Production",
    severity: "S2",
    state: "work_in_progress",
    owner: "Priya N.",
    ownerIsMe: false,
    slaClockType: "resolution",
    minutesToBreach: 320,
    createdAt: minutesAgo(60 * 12),
    updatedAt: minutesAgo(200),
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
    owner: "Priya N.",
    ownerIsMe: false,
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
    owner: "Dilan W.",
    ownerIsMe: false,
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
    owner: "Unassigned",
    ownerIsMe: false,
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
    owner: "Sajith Ekanayaka",
    ownerIsMe: true,
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
    state: "waiting_on_wso2",
    owner: "Maya R.",
    ownerIsMe: false,
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
    owner: "Tharindu A.",
    ownerIsMe: false,
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
    owner: "Unassigned",
    ownerIsMe: false,
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
    owner: "Priya N.",
    ownerIsMe: false,
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
    owner: "Sajith Ekanayaka",
    ownerIsMe: true,
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
    owner: "Unassigned",
    ownerIsMe: false,
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
    owner: "Maya R.",
    ownerIsMe: false,
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
    state: "reopen",
    owner: "Sajith Ekanayaka",
    ownerIsMe: true,
    slaClockType: "ack",
    minutesToBreach: 30,
    createdAt: minutesAgo(60 * 48),
    updatedAt: minutesAgo(10),
  },
];

const ALL_EXTRA_CASES: CsmCaseRow[] = [
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
    owner: "Tharindu A.",
    ownerIsMe: false,
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
    owner: "Maya R.",
    ownerIsMe: false,
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
    owner: "Dilan W.",
    ownerIsMe: false,
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
    owner: "Priya N.",
    ownerIsMe: false,
    slaClockType: "resolution",
    minutesToBreach: 0,
    createdAt: minutesAgo(60 * 100),
    updatedAt: minutesAgo(60 * 40),
  },
];

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
    csm: "Lakshmi I.",
    asr: "Chathura D.",
    openCases: 6,
  },
  "acc-002": {
    accountName: "Globex Corp",
    tier: "subscription",
    region: "eu-west-1",
    primaryContact: "Helena Voss",
    primaryContactEmail: "h.voss@globex.com",
    csm: "Dilan W.",
    openCases: 3,
  },
  "acc-003": {
    accountName: "Initech Systems",
    tier: "managed_cloud",
    region: "ap-southeast-1",
    primaryContact: "Peter Gibbons",
    primaryContactEmail: "peter@initech.io",
    csm: "Lakshmi I.",
    asr: "Tharindu A.",
    openCases: 5,
  },
  "acc-004": {
    accountName: "Soylent Industries",
    tier: "subscription",
    region: "us-west-2",
    primaryContact: "Marie Sandwitch",
    primaryContactEmail: "marie@soylent.com",
    csm: "Maya R.",
    openCases: 2,
  },
  "acc-005": {
    accountName: "Umbrella Health",
    tier: "saas",
    region: "us-east-1",
    primaryContact: "Albert Wesker",
    primaryContactEmail: "a.wesker@umbrella.health",
    csm: "Priya N.",
    openCases: 4,
  },
  "acc-101": {
    accountName: "Wayne Enterprises",
    tier: "subscription",
    region: "us-east-1",
    primaryContact: "Lucius Fox",
    primaryContactEmail: "lfox@wayne.com",
    csm: "Maya R.",
    openCases: 1,
  },
  "acc-102": {
    accountName: "Stark Industries",
    tier: "managed_cloud",
    region: "us-west-1",
    primaryContact: "Pepper Potts",
    primaryContactEmail: "pepper@stark.com",
    csm: "Dilan W.",
    asr: "Asanka R.",
    openCases: 4,
  },
  "acc-103": {
    accountName: "Tyrell Corp",
    tier: "subscription",
    region: "eu-central-1",
    primaryContact: "Eldon Tyrell",
    primaryContactEmail: "eldon@tyrell.corp",
    csm: "Priya N.",
    openCases: 1,
  },
};

const FALLBACK_CUSTOMER: CaseCustomerContext = {
  accountName: "Unknown Account",
  tier: "subscription",
  region: "unknown",
  primaryContact: "(no primary contact)",
  primaryContactEmail: "—",
  csm: "Unassigned",
  openCases: 1,
};

function deriveProduct(c: CsmCaseRow): CaseProductContext {
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
  // Always include owner if known + 2-3 more deterministically.
  const base: CaseWatcher[] = [];
  if (c.ownerIsMe) base.push({ id: "w-1", name: c.owner, role: "wso2_engineer", isMe: true });
  else if (c.owner !== "Unassigned")
    base.push({ id: `w-owner-${c.id}`, name: c.owner, role: "wso2_engineer" });
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
  if (c.state === "reopen")
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
  const owner = c.owner === "Unassigned" ? "Sajith Ekanayaka" : c.owner;
  const totalMinutes = (seedIdx * 17) % 480 + 30;
  const logs: CaseTimeLogEntry[] = [
    {
      id: `tl-${c.id}-a`,
      engineer: owner,
      hours: Math.round((totalMinutes / 60) * 4) / 4,
      note: "Investigated logs, gathered customer-side traces.",
      date: new Date(Date.now() - 60 * 60 * 1000 * 6).toISOString(),
    },
  ];
  if (c.state !== "open") {
    logs.unshift({
      id: `tl-${c.id}-b`,
      engineer: owner,
      hours: 1.5,
      note: "Reviewed customer config and reproduction steps.",
      date: new Date(Date.now() - 60 * 60 * 1000 * 22).toISOString(),
    });
  }
  if (c.state === "solution_proposed" || c.state === "closed") {
    logs.unshift({
      id: `tl-${c.id}-c`,
      engineer: owner,
      hours: 2,
      note: "Drafted proposed fix and verified in staging.",
      date: new Date(Date.now() - 60 * 60 * 1000 * 2).toISOString(),
    });
  }
  return logs;
}

function deriveAudit(c: CsmCaseRow): CaseAuditEntry[] {
  const owner = c.owner === "Unassigned" ? "(unassigned)" : c.owner;
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
      kind: "owner_change",
      actor: "Routing",
      description: `Assigned to ${owner}`,
      createdAt: new Date(new Date(c.createdAt).getTime() + 5 * 60_000).toISOString(),
    });
    events.push({
      id: `a-${c.id}-2`,
      kind: "state_change",
      actor: owner,
      description: "Moved to Work in progress",
      createdAt: new Date(new Date(c.createdAt).getTime() + 15 * 60_000).toISOString(),
    });
  }
  if (c.state === "awaiting_info") {
    events.push({
      id: `a-${c.id}-3`,
      kind: "state_change",
      actor: owner,
      description: "Requested additional info from customer",
      createdAt: c.updatedAt,
    });
  }
  if (c.state === "waiting_on_wso2") {
    events.push({
      id: `a-${c.id}-3`,
      kind: "state_change",
      actor: owner,
      description: "Marked as waiting on internal WSO2 dependency",
      createdAt: c.updatedAt,
    });
  }
  if (c.state === "solution_proposed") {
    events.push({
      id: `a-${c.id}-3`,
      kind: "state_change",
      actor: owner,
      description: "Posted proposed solution to customer",
      createdAt: c.updatedAt,
    });
  }
  if (c.state === "closed") {
    events.push({
      id: `a-${c.id}-3`,
      kind: "state_change",
      actor: owner,
      description: "Case closed",
      createdAt: c.updatedAt,
    });
  }
  if (c.state === "reopen") {
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
    "Token issuance latency on the corporate IdP has spiked to >2.5s p95 since 09:30 UTC. Affecting workforce SSO across all internal apps. Mutual TLS and DB connection counts look normal.",
  "case-1002":
    "Gateway returns intermittent 502 to ~3% of traffic during 11:00-13:00 UTC peak window. No backend errors visible; suspected upstream connection pool saturation.",
  "case-1003":
    "Micro Integrator cluster fails to start after applying update level wso2mi-4.4.0.7. Carbon log shows ClassNotFoundException on a security extension.",
};

function describe(c: CsmCaseRow): string {
  return (
    DESCRIPTIONS[c.id] ??
    `${c.subject}. Reported by ${c.customer} on project ${c.projectName}. Additional reproduction details captured in the comment thread.`
  );
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
        uploadedBy: c.owner === "Unassigned" ? "Sajith Ekanayaka" : c.owner,
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

export function getMockCsmCaseDetailById(
  idOrNumber: string,
): CsmCaseDetail | undefined {
  const row = getMockCsmCaseById(idOrNumber);
  if (!row) return undefined;
  const customerContext = CUSTOMER_CONTEXTS[row.accountId] ?? FALLBACK_CUSTOMER;
  return {
    ...row,
    description: describe(row),
    assignmentGroup: row.projectName.toLowerCase().includes("choreo")
      ? "grp.choreo_sre"
      : row.projectName.toLowerCase().includes("asgardeo")
        ? "grp.asgardeo_sre"
        : "grp.cre_team",
    customerContext,
    productContext: deriveProduct(row),
    slaClocks: deriveSlaClocks(row),
    watchers: deriveWatchers(row),
    linkedItems: deriveLinkedItems(row),
    tags: deriveTags(row),
    timeLogs: deriveTimeLogs(row),
    audit: deriveAudit(row),
    attachments: deriveAttachments(row),
    isWatching: row.ownerIsMe,
  };
}
