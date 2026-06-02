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

import {
  ENGAGEMENT_STAGE_ORDER,
  deriveProgressPct,
} from "@features/csm-engagements/utils/engagements";
import type {
  CreateCsmEngagementInput,
  CsmEngagementAttachment,
  CsmEngagementAuditEntry,
  CsmEngagementAllocation,
  CsmEngagementComment,
  CsmEngagementDeliverable,
  CsmEngagementDetail,
  CsmEngagementLinkedCase,
  CsmEngagementPaymentType,
  CsmEngagementRow,
  CsmEngagementStage,
  CsmEngagementStatusUpdate,
  CsmEngagementTask,
  CsmEngagementType,
  CsmEngagementWatcher,
  CsmEngagementsListResponse,
} from "@features/csm-engagements/types/csmEngagements";

const HOUR = 60 * 60 * 1000;
const DAY = 24 * HOUR;

function isoDaysAgo(days: number): string {
  return new Date(Date.now() - days * DAY).toISOString();
}

function dateOnlyDaysAgo(days: number): string {
  return new Date(Date.now() - days * DAY).toISOString().slice(0, 10);
}

function dateOnlyDaysFromNow(days: number): string {
  return new Date(Date.now() + days * DAY).toISOString().slice(0, 10);
}

const CURRENT_USER = {
  id: "user-me",
  name: "Sajith Ekanayaka",
};

/** Base engagement seed data — the heavy detail is derived in builders below. */
interface EngagementSeed {
  id: string;
  reference: string;
  name: string;
  type: CsmEngagementType;
  state: CsmEngagementRow["state"];
  stage: CsmEngagementStage;
  customer: string;
  accountId: string;
  projectId: string;
  projectName: string;
  ownerName: string;
  ownerIsMe?: boolean;
  deliveryMode: CsmEngagementRow["deliveryMode"];
  plannedStartDate: string;
  plannedEndDate: string;
  health?: CsmEngagementRow["health"];
  paymentType: CsmEngagementPaymentType;
  scope: string;
  description: string;
  opportunityRef?: string;
  /** Cases linked back to this engagement, if any. */
  linkedCaseIds?: Array<{ id: string; caseNumber: string; subject: string; state: string; severity: string }>;
  createdDaysAgo: number;
  updatedDaysAgo: number;
}

const SEEDS: EngagementSeed[] = [
  {
    id: "eng-1001",
    reference: "ENG-1001",
    name: "Choreo onboarding — Initech Systems",
    type: "customer_onboarding",
    state: "in_progress",
    stage: "execution",
    customer: "Initech Systems",
    accountId: "acc-initech",
    projectId: "proj-initech-choreo",
    projectName: "Choreo Runtime",
    ownerName: "Sajith Ekanayaka",
    ownerIsMe: true,
    deliveryMode: "remote",
    plannedStartDate: dateOnlyDaysAgo(28),
    plannedEndDate: dateOnlyDaysFromNow(35),
    health: "amber",
    paymentType: "foc",
    scope:
      "End-to-end Choreo Runtime onboarding for Initech's two engineering teams (Payments and Identity), including environment provisioning, sample workload migration, and runbook handoff.",
    description:
      "Initech recently purchased a Choreo subscription. This engagement covers technical onboarding, deployment patterns, observability setup, and an initial production cutover for their Payments service.",
    opportunityRef: "00k4400000eKZRFAA4",
    createdDaysAgo: 32,
    updatedDaysAgo: 1,
  },
  {
    id: "eng-1002",
    reference: "ENG-1002",
    name: "QSP — API Manager rollout for Wayne Industries",
    type: "qsp",
    state: "in_progress",
    stage: "execution",
    customer: "Wayne Industries",
    accountId: "acc-wayne",
    projectId: "proj-wayne-am",
    projectName: "API Manager",
    ownerName: "Priya Mendis",
    deliveryMode: "onsite",
    plannedStartDate: dateOnlyDaysAgo(10),
    plannedEndDate: dateOnlyDaysFromNow(11),
    health: "green",
    paymentType: "paid",
    scope:
      "Two-week Quick Start Package for API Manager: install, gateway clustering, identity integration, and developer portal customisation.",
    description:
      "Wayne Industries is standardising their API gateway on WSO2 API Manager. The QSP covers gateway clustering across two AZs, configuration of an external Postgres, and a custom developer portal theme.",
    opportunityRef: "00k4400000fLW42BB1",
    createdDaysAgo: 12,
    updatedDaysAgo: 0,
  },
  {
    id: "eng-1003",
    reference: "ENG-1003",
    name: "Enterprise TAM — Cyberdyne Federation Programme",
    type: "enterprise_csm_tam",
    state: "in_progress",
    stage: "execution",
    customer: "Cyberdyne Systems",
    accountId: "acc-cyberdyne",
    projectId: "proj-cyberdyne-is",
    projectName: "Identity Server federation",
    ownerName: "Hiruni Perera",
    deliveryMode: "hybrid",
    plannedStartDate: dateOnlyDaysAgo(180),
    plannedEndDate: dateOnlyDaysFromNow(185),
    health: "green",
    paymentType: "paid",
    scope:
      "12-month Technical Account Manager engagement: monthly architecture reviews, quarterly upgrade planning, and a named TAM for escalation routing across Identity Server and Choreo deployments.",
    description:
      "Cyberdyne renewed their enterprise TAM contract. Covers all production Identity Server clusters across three regions and the Choreo subscription in eu-west.",
    opportunityRef: "00k4400000gMA88CC2",
    createdDaysAgo: 195,
    updatedDaysAgo: 5,
  },
  {
    id: "eng-1004",
    reference: "ENG-1004",
    name: "ESB → Micro Integrator migration discovery — Acme Globex",
    type: "migration",
    state: "in_progress",
    stage: "planning",
    customer: "Acme Globex",
    accountId: "acc-acme",
    projectId: "proj-acme-mi",
    projectName: "Micro Integrator",
    ownerName: "Devin Kularatne",
    deliveryMode: "remote",
    plannedStartDate: dateOnlyDaysAgo(14),
    plannedEndDate: dateOnlyDaysFromNow(45),
    health: "amber",
    paymentType: "paid",
    scope:
      "Discovery and migration design for 137 legacy ESB mediation flows targeting WSO2 Micro Integrator 4.3. Output: inventory, ESB→MI mapping, migration backlog, sample of 5 flow conversions.",
    description:
      "Acme Globex is sunsetting their legacy ESB stack. This engagement scopes the migration; a separate execution-phase engagement will follow once estimates are signed off.",
    opportunityRef: "00k4400000hNB55DD3",
    createdDaysAgo: 18,
    updatedDaysAgo: 2,
  },
  {
    id: "eng-1005",
    reference: "ENG-1005",
    name: "Choreo deployment patterns training — Stark Industries",
    type: "training",
    state: "completed",
    stage: "closure",
    customer: "Stark Industries",
    accountId: "acc-stark",
    projectId: "proj-stark-choreo",
    projectName: "Choreo Runtime",
    ownerName: "Priya Mendis",
    deliveryMode: "onsite",
    plannedStartDate: dateOnlyDaysAgo(38),
    plannedEndDate: dateOnlyDaysAgo(35),
    health: "green",
    paymentType: "paid",
    scope:
      "3-day onsite training for 18 engineers covering Choreo Runtime deployment patterns, observability, and ITSM integration.",
    description:
      "Standard training package, delivered in NYC. Feedback score 4.7/5. Completion certificates issued.",
    opportunityRef: "00k4400000iOC11EE4",
    createdDaysAgo: 60,
    updatedDaysAgo: 30,
  },
  {
    id: "eng-1006",
    reference: "ENG-1006",
    name: "Architecture review — Soylent Corp Identity Server upgrade",
    type: "architecture_review",
    state: "requested",
    stage: "pre_engagement",
    customer: "Soylent Corp",
    accountId: "acc-soylent",
    projectId: "proj-soylent-is",
    projectName: "Identity Server",
    ownerName: "Unassigned",
    deliveryMode: "remote",
    plannedStartDate: dateOnlyDaysFromNow(7),
    plannedEndDate: dateOnlyDaysFromNow(28),
    paymentType: "foc",
    scope:
      "Pre-upgrade architecture review for migration from Identity Server 5.11 to 7.0. Covers federation topology, custom adapter inventory, deprecation risks, and recommended rollout plan.",
    description:
      "Customer requested following two production incidents on IS 5.11. Awaiting scoping call to confirm assignment and timeline.",
    createdDaysAgo: 3,
    updatedDaysAgo: 1,
  },
  {
    id: "eng-1007",
    reference: "ENG-1007",
    name: "Firefighting — Vandelay API gateway P0 stabilisation",
    type: "firefighting",
    state: "in_progress",
    stage: "execution",
    customer: "Vandelay Industries",
    accountId: "acc-vandelay",
    projectId: "proj-vandelay-am",
    projectName: "API Manager",
    ownerName: "Sajith Ekanayaka",
    ownerIsMe: true,
    deliveryMode: "remote",
    plannedStartDate: dateOnlyDaysAgo(2),
    plannedEndDate: dateOnlyDaysFromNow(5),
    health: "red",
    paymentType: "paid",
    scope:
      "Emergency stabilisation for production API gateway: 5xx surge investigation, JVM heap analysis, throttling re-tune, and a 7-day on-call window with named engineers.",
    description:
      "Triggered by a P0 case where the customer's gateway saturated heap during peak traffic. Engagement covers stabilisation plus a hardening backlog.",
    opportunityRef: "00k4400000jPD22FF5",
    linkedCaseIds: [
      {
        id: "case-1015",
        caseNumber: "CS-1015",
        subject: "Choreo runtime OOM under sustained load",
        state: "Work in progress",
        severity: "S2",
      },
    ],
    createdDaysAgo: 3,
    updatedDaysAgo: 0,
  },
  {
    id: "eng-1008",
    reference: "ENG-1008",
    name: "Customer Onboarding — Hooli (multi-product)",
    type: "customer_onboarding",
    state: "on_hold",
    stage: "planning",
    customer: "Hooli",
    accountId: "acc-hooli",
    projectId: "proj-hooli-multi",
    projectName: "Multi-product",
    ownerName: "Hiruni Perera",
    deliveryMode: "hybrid",
    plannedStartDate: dateOnlyDaysAgo(7),
    plannedEndDate: dateOnlyDaysFromNow(80),
    health: "amber",
    paymentType: "foc",
    scope:
      "Joint onboarding across Choreo and Identity Server for Hooli's PieperChat and Pied Piper Cloud workloads. Custom timelines requested per product team.",
    description:
      "Paused at customer's request while procurement finalises a contract amendment. Expected to resume within 2 weeks.",
    opportunityRef: "00k4400000kQE33GG6",
    createdDaysAgo: 14,
    updatedDaysAgo: 4,
  },
  {
    id: "eng-1009",
    reference: "ENG-1009",
    name: "Consultancy — Pied Piper compression service review",
    type: "consultancy",
    state: "completed",
    stage: "closure",
    customer: "Pied Piper",
    accountId: "acc-piedpiper",
    projectId: "proj-piedpiper-ms",
    projectName: "Micro Integrator",
    ownerName: "Devin Kularatne",
    deliveryMode: "remote",
    plannedStartDate: dateOnlyDaysAgo(90),
    plannedEndDate: dateOnlyDaysAgo(50),
    health: "green",
    paymentType: "paid",
    scope:
      "Performance and resilience review of Pied Piper's compression flows on Micro Integrator. Deliverables: load test report, recommendations doc, three patched flow samples.",
    description:
      "Successfully closed; customer adopted 14/17 recommendations.",
    opportunityRef: "00k4400000lRF44HH7",
    createdDaysAgo: 100,
    updatedDaysAgo: 45,
  },
  {
    id: "eng-1010",
    reference: "ENG-1010",
    name: "Architecture review — Massive Dynamic Choreo platform fit",
    type: "architecture_review",
    state: "cancelled",
    stage: "pre_engagement",
    customer: "Massive Dynamic",
    accountId: "acc-massive",
    projectId: "proj-massive-choreo",
    projectName: "Choreo platform fit",
    ownerName: "Unassigned",
    deliveryMode: "remote",
    plannedStartDate: dateOnlyDaysAgo(45),
    plannedEndDate: dateOnlyDaysAgo(15),
    paymentType: "foc",
    scope:
      "Initial Choreo platform fit review for a potential migration off self-hosted Kubernetes.",
    description:
      "Customer cancelled before scoping completed: decision was made to stay on the existing self-hosted stack for the next budget cycle.",
    createdDaysAgo: 55,
    updatedDaysAgo: 14,
  },
];

const COMMON_WATCHERS = (ownerName: string): CsmEngagementWatcher[] => [
  { id: "w-1", name: ownerName, role: "wso2_engineer", isMe: ownerName === CURRENT_USER.name },
  { id: "w-2", name: "Ramith Jayasinghe", role: "manager" },
  { id: "w-3", name: "Layla Davis", role: "customer_contact" },
];

function buildAllocations(seed: EngagementSeed): CsmEngagementAllocation[] {
  return [
    {
      id: `${seed.id}-alloc-1`,
      engagementId: seed.id,
      userId: "u-owner",
      userName: seed.ownerName,
      allocationPct: 80,
      startDate: seed.plannedStartDate,
      endDate: seed.plannedEndDate,
    },
    {
      id: `${seed.id}-alloc-2`,
      engagementId: seed.id,
      userId: "u-priya",
      userName: "Priya Mendis",
      allocationPct: 40,
      startDate: seed.plannedStartDate,
      endDate: seed.plannedEndDate,
    },
  ];
}

function buildTasks(seed: EngagementSeed): CsmEngagementTask[] {
  // Generic milestone-driven task templates per engagement type.
  const PER_TYPE: Record<CsmEngagementType, Array<{ title: string; stage: CsmEngagementStage }>> = {
    customer_onboarding: [
      { title: "Kick-off call and stakeholder map", stage: "pre_engagement" },
      { title: "Provision sandbox & production environments", stage: "planning" },
      { title: "Configure observability stack and runbooks", stage: "execution" },
      { title: "Migrate first sample workload end-to-end", stage: "execution" },
      { title: "Customer acceptance sign-off", stage: "warranty" },
      { title: "Runbook handover and CRE introduction", stage: "post_engagement" },
    ],
    migration: [
      { title: "Discovery interviews and source-system inventory", stage: "planning" },
      { title: "Target-state architecture sign-off", stage: "planning" },
      { title: "Migration runbook and rollback plan", stage: "execution" },
      { title: "Pilot migration (sample workload)", stage: "execution" },
      { title: "Bulk migration cutover", stage: "execution" },
      { title: "Stabilisation watch (2-week warranty)", stage: "warranty" },
      { title: "Post-migration review and handover", stage: "post_engagement" },
    ],
    qsp: [
      { title: "Pre-flight checklist with the customer", stage: "pre_engagement" },
      { title: "Install gateway cluster and DB", stage: "execution" },
      { title: "Identity integration walkthrough", stage: "execution" },
      { title: "Developer portal customisation", stage: "execution" },
      { title: "Hand-off doc and post-QSP review", stage: "closure" },
    ],
    enterprise_csm_tam: [
      { title: "Monthly architecture review (current month)", stage: "execution" },
      { title: "Quarterly upgrade plan refresh", stage: "execution" },
      { title: "Annual capacity planning review", stage: "execution" },
      { title: "Renewal preparation", stage: "post_engagement" },
    ],
    consultancy: [
      { title: "Discovery interviews with customer stakeholders", stage: "planning" },
      { title: "Inventory and current-state assessment", stage: "planning" },
      { title: "Target-state design", stage: "execution" },
      { title: "Sample-of-three proof points", stage: "execution" },
      { title: "Final recommendations report", stage: "closure" },
    ],
    training: [
      { title: "Pre-training survey and prerequisites check", stage: "pre_engagement" },
      { title: "Deliver day 1 — fundamentals", stage: "execution" },
      { title: "Deliver day 2 — patterns", stage: "execution" },
      { title: "Deliver day 3 — labs", stage: "execution" },
      { title: "Issue certificates and collect feedback", stage: "closure" },
    ],
    architecture_review: [
      { title: "Scoping call and signed SOW", stage: "pre_engagement" },
      { title: "Collect topology, configs, and metrics", stage: "planning" },
      { title: "Run review sessions (3 × 90 min)", stage: "execution" },
      { title: "Draft findings doc", stage: "execution" },
      { title: "Final review with stakeholders", stage: "closure" },
    ],
    firefighting: [
      { title: "Initial triage and incident bridge", stage: "execution" },
      { title: "Root-cause analysis", stage: "execution" },
      { title: "Mitigation rollout", stage: "execution" },
      { title: "7-day stabilisation watch", stage: "warranty" },
      { title: "Post-mortem and hardening backlog", stage: "post_engagement" },
    ],
  };

  const tasks = PER_TYPE[seed.type];
  return tasks.map((t, i) => {
    let state: CsmEngagementTask["state"] = "not_started";
    if (seed.state === "completed") state = "completed";
    else if (seed.state === "cancelled") state = "cancelled";
    else if (seed.state === "in_progress") {
      // Distribute progress: roughly the first 60% are done/in progress for active engagements.
      const ratio = (i + 1) / tasks.length;
      if (ratio <= 0.4) state = "completed";
      else if (ratio <= 0.7) state = "in_progress";
      else state = "not_started";
    } else if (seed.state === "on_hold") {
      const ratio = (i + 1) / tasks.length;
      state = ratio <= 0.3 ? "completed" : ratio <= 0.5 ? "blocked" : "not_started";
    }
    return {
      id: `${seed.id}-t-${i + 1}`,
      engagementId: seed.id,
      title: t.title,
      stage: t.stage,
      state,
      assigneeName: i % 2 === 0 ? seed.ownerName : "Priya Mendis",
      dueDate: dateOnlyDaysFromNow(
        Math.round(((i + 1) / tasks.length) * 30) - (seed.state === "completed" ? 30 : 0),
      ),
      completedAt: state === "completed" ? isoDaysAgo(tasks.length - i) : undefined,
    };
  });
}

function buildDeliverables(seed: EngagementSeed): CsmEngagementDeliverable[] {
  const generic = [
    "Scoping document signed",
    "Mid-engagement progress report",
    "Final findings / handover document",
  ];
  const extras: Record<CsmEngagementType, string[]> = {
    customer_onboarding: ["Runbook bundle", "Observability dashboards"],
    migration: ["Migration runbook", "Cutover report", "Source-system inventory"],
    qsp: ["Configuration as code repo", "Developer portal theme"],
    enterprise_csm_tam: ["Quarterly architecture review notes"],
    consultancy: ["Recommendations report", "Sample-of-three artifacts"],
    training: ["Course slides", "Lab repository"],
    architecture_review: ["Findings deck", "Risk matrix"],
    firefighting: ["RCA document", "Hardening backlog"],
  };
  const names = [...generic, ...extras[seed.type]];
  return names.map((n, i) => {
    let status: CsmEngagementDeliverable["status"] = "pending";
    if (seed.state === "completed") status = i === names.length - 1 ? "accepted" : "accepted";
    else if (seed.state === "in_progress") {
      const ratio = (i + 1) / names.length;
      if (ratio <= 0.5) status = "accepted";
      else if (ratio <= 0.7) status = "in_review";
      else status = "pending";
    } else if (seed.state === "cancelled") {
      status = "waived";
    } else if (seed.state === "on_hold") {
      status = i === 0 ? "accepted" : "pending";
    }
    return {
      id: `${seed.id}-d-${i + 1}`,
      engagementId: seed.id,
      name: n,
      status,
      dueDate: dateOnlyDaysFromNow(i * 14 - (seed.state === "completed" ? 60 : 0)),
      completedAt: status === "accepted" ? isoDaysAgo(i * 7 + 2) : undefined,
      waiverReason:
        status === "waived"
          ? "Engagement cancelled before deliverable was scoped."
          : undefined,
    };
  });
}

function buildStatusUpdates(seed: EngagementSeed): CsmEngagementStatusUpdate[] {
  if (seed.state === "new" || seed.state === "cancelled") return [];
  const headlines: Array<{ headline: string; body: string; health: "green" | "amber" | "red" }> = [
    {
      headline: "Kick-off complete; environments provisioned",
      body: "<p>All sandbox + production environments are provisioned. Identity integration tested on sandbox; production cutover scheduled for next sprint.</p>",
      health: "green",
    },
    {
      headline: "Halfway checkpoint — minor risk on observability",
      body: "<p>Workload migration tracking ahead of plan. Observability stack delayed by one week due to networking review with the customer's SecOps team. Mitigation: staggered rollout instead of big-bang.</p>",
      health: "amber",
    },
    {
      headline: "Stabilisation window in effect",
      body: "<p>Production deploy completed last week. We're inside the 7-day stabilisation window. No incidents so far; one customer-reported regression closed within the day.</p>",
      health: "green",
    },
  ];
  const limit = seed.state === "in_progress" ? 3 : seed.state === "on_hold" ? 2 : 3;
  return headlines.slice(0, limit).map((h, i, arr) => ({
    id: `${seed.id}-su-${i + 1}`,
    engagementId: seed.id,
    authorId: "u-owner",
    authorName: seed.ownerName,
    health: seed.state === "on_hold" ? "amber" : seed.health ?? h.health,
    headline: h.headline,
    bodyHtml: h.body,
    createdAt: isoDaysAgo((arr.length - i) * 5),
  }));
}

function buildAudit(seed: EngagementSeed, tasks: CsmEngagementTask[]): CsmEngagementAuditEntry[] {
  const out: CsmEngagementAuditEntry[] = [];
  out.push({
    id: `${seed.id}-a-created`,
    kind: "created",
    actor: seed.ownerName,
    description: `Engagement ${seed.reference} created.`,
    createdAt: isoDaysAgo(seed.createdDaysAgo),
  });
  if (seed.state !== "new") {
    out.push({
      id: `${seed.id}-a-state-1`,
      kind: "state_change",
      actor: seed.ownerName,
      description: "State moved to Requested.",
      createdAt: isoDaysAgo(seed.createdDaysAgo - 1),
    });
  }
  if (
    seed.state === "in_progress" ||
    seed.state === "on_hold" ||
    seed.state === "completed"
  ) {
    out.push({
      id: `${seed.id}-a-state-2`,
      kind: "state_change",
      actor: seed.ownerName,
      description: "Work started.",
      createdAt: isoDaysAgo(seed.createdDaysAgo - 3),
    });
  }
  if (seed.state === "on_hold") {
    out.push({
      id: `${seed.id}-a-state-3`,
      kind: "state_change",
      actor: seed.ownerName,
      description: "Put on hold per customer request.",
      createdAt: isoDaysAgo(seed.updatedDaysAgo),
    });
  }
  if (seed.state === "completed") {
    out.push({
      id: `${seed.id}-a-state-4`,
      kind: "state_change",
      actor: seed.ownerName,
      description: "Engagement marked complete.",
      createdAt: isoDaysAgo(seed.updatedDaysAgo),
    });
  }
  if (seed.state === "cancelled") {
    out.push({
      id: `${seed.id}-a-state-5`,
      kind: "state_change",
      actor: seed.ownerName,
      description: "Engagement cancelled.",
      createdAt: isoDaysAgo(seed.updatedDaysAgo),
    });
  }
  tasks
    .filter((t) => t.state === "completed")
    .slice(0, 3)
    .forEach((t, i) =>
      out.push({
        id: `${seed.id}-a-task-${i + 1}`,
        kind: "task_completed",
        actor: t.assigneeName ?? seed.ownerName,
        description: `Task completed: ${t.title}`,
        createdAt: t.completedAt ?? isoDaysAgo(5),
      }),
    );
  return out.sort(
    (a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime(),
  );
}

function buildStageStatus(
  seed: EngagementSeed,
): CsmEngagementDetail["stageStatus"] {
  const stages = ENGAGEMENT_STAGE_ORDER;
  const currentIdx = stages.indexOf(seed.stage);
  const result: CsmEngagementDetail["stageStatus"] = {};
  stages.forEach((s, i) => {
    if (seed.state === "completed") result[s] = "completed";
    else if (seed.state === "cancelled") result[s] = i <= currentIdx ? "in_progress" : "not_started";
    else if (i < currentIdx) result[s] = "completed";
    else if (i === currentIdx) result[s] = "in_progress";
    else result[s] = "not_started";
  });
  return result;
}

function buildComments(seed: EngagementSeed): CsmEngagementComment[] {
  if (seed.state === "new" || seed.state === "cancelled") return [];
  return [
    {
      id: `${seed.id}-c-1`,
      engagementId: seed.id,
      authorName: "Layla Davis",
      authorRole: "customer_contact",
      bodyHtml:
        "<p>Thanks for the kick-off. The team is happy with the plan. One question: can we move the production cutover one day later to align with our change window?</p>",
      createdAt: isoDaysAgo(Math.max(2, seed.createdDaysAgo - 5)),
    },
    {
      id: `${seed.id}-c-2`,
      engagementId: seed.id,
      authorName: seed.ownerName,
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Yes that works for us. I've moved the cutover to the following Friday and updated the plan.</p>",
      createdAt: isoDaysAgo(Math.max(1, seed.createdDaysAgo - 6)),
    },
    {
      id: `${seed.id}-c-3`,
      engagementId: seed.id,
      authorName: seed.ownerName,
      authorRole: "wso2_engineer",
      internal: true,
      bodyHtml:
        "<p><strong>Internal note:</strong> heads-up to the on-call rota — peak traffic window during cutover; keep the bridge open for 2 hours after release.</p>",
      createdAt: isoDaysAgo(Math.max(0, seed.createdDaysAgo - 7)),
    },
  ];
}

function buildAttachments(seed: EngagementSeed): CsmEngagementAttachment[] {
  if (seed.state === "new" || seed.state === "cancelled") return [];
  return [
    {
      id: `${seed.id}-att-1`,
      filename: "scope-statement.pdf",
      size: 184_320,
      contentType: "application/pdf",
      uploadedBy: seed.ownerName,
      uploadedAt: isoDaysAgo(Math.max(1, seed.createdDaysAgo - 2)),
    },
    {
      id: `${seed.id}-att-2`,
      filename: "stakeholder-map.png",
      size: 72_104,
      contentType: "image/png",
      uploadedBy: seed.ownerName,
      uploadedAt: isoDaysAgo(Math.max(1, seed.createdDaysAgo - 4)),
    },
  ];
}

function buildLinkedCases(seed: EngagementSeed): CsmEngagementLinkedCase[] {
  if (!seed.linkedCaseIds) return [];
  return seed.linkedCaseIds.map((l) => ({
    ...l,
    href: `/cases/${l.id}`,
  }));
}

function rowFromSeed(seed: EngagementSeed): CsmEngagementRow {
  const stageStatus = buildStageStatus(seed);
  const tasks = buildTasks(seed);
  const completedTasks = tasks.filter((t) => t.state === "completed").length;
  return {
    id: seed.id,
    reference: seed.reference,
    name: seed.name,
    type: seed.type,
    state: seed.state,
    stage: seed.stage,
    customer: seed.customer,
    accountId: seed.accountId,
    projectId: seed.projectId,
    projectName: seed.projectName,
    ownerId: seed.ownerIsMe ? CURRENT_USER.id : `u-${seed.ownerName.toLowerCase().replace(/\s+/g, "-")}`,
    ownerName: seed.ownerName,
    ownerIsMe: !!seed.ownerIsMe,
    deliveryMode: seed.deliveryMode,
    plannedStartDate: seed.plannedStartDate,
    plannedEndDate: seed.plannedEndDate,
    health: seed.health,
    paymentType: seed.paymentType,
    progressPct: deriveProgressPct({
      stageStatus,
      tasksTotal: tasks.length,
      tasksCompleted: completedTasks,
    }),
    createdAt: isoDaysAgo(seed.createdDaysAgo),
    updatedAt: isoDaysAgo(seed.updatedDaysAgo),
  };
}

function detailFromSeed(seed: EngagementSeed): CsmEngagementDetail {
  const row = rowFromSeed(seed);
  const tasks = buildTasks(seed);
  const stageStatus = buildStageStatus(seed);
  const audit = buildAudit(seed, tasks);
  const attachments = buildAttachments(seed);
  const linkedCases = buildLinkedCases(seed);
  // Add attachment + case-link audit entries
  attachments.forEach((a, i) =>
    audit.push({
      id: `${seed.id}-a-att-${i + 1}`,
      kind: "attachment_added",
      actor: a.uploadedBy,
      description: `Attached ${a.filename}.`,
      createdAt: a.uploadedAt,
    }),
  );
  linkedCases.forEach((c, i) =>
    audit.push({
      id: `${seed.id}-a-link-${i + 1}`,
      kind: "case_linked",
      actor: seed.ownerName,
      description: `Linked case ${c.caseNumber}: ${c.subject}.`,
      createdAt: isoDaysAgo(Math.max(0, seed.updatedDaysAgo - 1)),
    }),
  );
  audit.sort(
    (a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime(),
  );
  return {
    ...row,
    description: seed.description,
    scope: seed.scope,
    billing: {
      paymentType: seed.paymentType,
      opportunityRef: seed.opportunityRef,
    },
    watchers: COMMON_WATCHERS(seed.ownerName),
    allocations: buildAllocations(seed),
    stages: ENGAGEMENT_STAGE_ORDER,
    stageStatus,
    tasks,
    deliverables: buildDeliverables(seed),
    statusUpdates: buildStatusUpdates(seed),
    audit,
    attachments,
    linkedCases,
    isWatching: !!seed.ownerIsMe,
  };
}

const ROWS: CsmEngagementRow[] = SEEDS.map(rowFromSeed);
const DETAIL_BY_ID = new Map<string, CsmEngagementDetail>(
  SEEDS.map((s) => [s.id, detailFromSeed(s)]),
);
const COMMENTS_BY_ID = new Map<string, CsmEngagementComment[]>(
  SEEDS.map((s) => [s.id, buildComments(s)]),
);

/** Mock helper: list all engagements (cross-customer). */
export function getMockCsmEngagements(): CsmEngagementsListResponse {
  return { engagements: ROWS };
}

/** Mock helper: fetch one engagement detail. */
export function getMockCsmEngagementDetail(
  id: string,
): CsmEngagementDetail | null {
  return DETAIL_BY_ID.get(id) ?? null;
}

/** Mock helper: fetch comments for one engagement. */
export function getMockCsmEngagementComments(
  id: string,
): CsmEngagementComment[] {
  return COMMENTS_BY_ID.get(id) ?? [];
}

/** Mock helper: create a new engagement (in-memory). */
export function createMockCsmEngagement(
  input: CreateCsmEngagementInput,
): CsmEngagementDetail {
  const id = `eng-${Math.floor(Date.now() / 1000)}`;
  const seed: EngagementSeed = {
    id,
    reference: `ENG-${1000 + ROWS.length + 1}`,
    name: input.name,
    type: input.type,
    state: "new",
    stage: "pre_engagement",
    customer: input.customer,
    accountId: input.accountId,
    projectId: input.projectId,
    projectName: input.projectName,
    ownerName: input.ownerName ?? CURRENT_USER.name,
    ownerIsMe: !input.ownerId || input.ownerId === CURRENT_USER.id,
    deliveryMode: input.deliveryMode,
    plannedStartDate: input.plannedStartDate,
    plannedEndDate: input.plannedEndDate,
    paymentType: input.billing.paymentType,
    scope: input.scope,
    description: input.description,
    opportunityRef: input.billing.opportunityRef,
    createdDaysAgo: 0,
    updatedDaysAgo: 0,
  };
  const detail = detailFromSeed(seed);
  ROWS.unshift(rowFromSeed(seed));
  DETAIL_BY_ID.set(id, detail);
  COMMENTS_BY_ID.set(id, []);
  return detail;
}

/** Mock helper: patch a small set of fields on the in-memory record. */
export function patchMockCsmEngagement(
  id: string,
  patch: Partial<
    Pick<
      CsmEngagementDetail,
      "state" | "stage" | "ownerName" | "ownerId" | "plannedEndDate" | "isWatching"
    >
  >,
): CsmEngagementDetail | null {
  const cur = DETAIL_BY_ID.get(id);
  if (!cur) return null;
  const next: CsmEngagementDetail = {
    ...cur,
    ...patch,
    updatedAt: new Date().toISOString(),
  };
  DETAIL_BY_ID.set(id, next);
  const rowIdx = ROWS.findIndex((r) => r.id === id);
  if (rowIdx >= 0) {
    ROWS[rowIdx] = {
      ...ROWS[rowIdx],
      state: next.state,
      stage: next.stage,
      ownerName: next.ownerName,
      ownerId: next.ownerId,
      plannedEndDate: next.plannedEndDate,
      updatedAt: next.updatedAt,
    };
  }
  return next;
}

/** Mock helper: append a new comment to the in-memory store. */
export function postMockCsmEngagementComment(input: {
  engagementId: string;
  authorName: string;
  bodyHtml: string;
  internal?: boolean;
}): CsmEngagementComment {
  const comment: CsmEngagementComment = {
    id: `${input.engagementId}-c-${Date.now()}`,
    engagementId: input.engagementId,
    authorName: input.authorName,
    authorRole: "wso2_engineer",
    bodyHtml: input.bodyHtml,
    internal: input.internal,
    createdAt: new Date().toISOString(),
  };
  const existing = COMMENTS_BY_ID.get(input.engagementId) ?? [];
  COMMENTS_BY_ID.set(input.engagementId, [...existing, comment]);
  return comment;
}
