// src/server/db/seed/synthetic.ts
// =============================================================================
// Deterministic fixtures used ONLY when SEED_GITHUB_TOKEN is empty.
// Emits the same { node, detail } pairs as github/client.ts so seed.ts has
// one ingestion path regardless of data source.
// PRIVACY: fixtures contain no titles, assignees, or actors — matching what
// the real GitHub path returns. At runtime the title proxy returns null for
// these issue numbers, so the UI shows "#<number>" only.
// =============================================================================

import type { ConfigRepo } from "../sla-config";
import type { GhIssueNode, GhIssueDetail, GhStatusEvent } from "../github/client";

const h = (hoursAgo: number) => new Date(Date.now() - hoursAgo * 3_600_000).toISOString();

interface Transition {
  status: string;
  hoursAgo: number; // when the item entered this status (oldest = largest)
}
interface Scenario {
  note: string; // human-readable intent; NOT emitted anywhere
  priority: string | null; // null => no Priority/* label => NO_SLA
  state: "OPEN" | "CLOSED";
  transitions: Transition[]; // oldest first
  // Overrides for the empty-timeline case: when `transitions` is empty
  // there's no last-transition status to derive `currentStatus` from, so
  // scenarios that need a non-null current status set it explicitly here.
  currentStatus?: string;
  currentStatusAgo?: number; // hoursAgo when currentStatus was set; required with currentStatus
}

// Expected outcomes (evaluated at "now"):
//   1 VIOLATED  2 AT_RISK/OK*  3 OK*  4 NO_SLA  5 TERMINAL  6 NO_SLA  7 VIOLATED*  8 TERMINAL  9 VIOLATED*
// * P2/P3 now accrue on a 12x5 IST business calendar, so the exact state of
//   these scenarios depends on which day/time the seed runs (nights & weekends
//   don't burn budget). P1 stays 24x7. P4 has no SLA (kept indefinitely).
const SCENARIOS: Scenario[] = [
  {
    note: "P1, ~120h product-side => VIOLATED, still running",
    priority: "Critical(P1)",
    state: "OPEN",
    transitions: [
      { status: "Open", hoursAgo: 120 },
      { status: "In Progress", hoursAgo: 100 },
      { status: "WOW", hoursAgo: 50 },
    ],
  },
  {
    note: "P2, ~20h accrued then paused on WOC => AT_RISK",
    priority: "High(P2)",
    state: "OPEN",
    transitions: [
      { status: "Open", hoursAgo: 25 },
      { status: "In Progress", hoursAgo: 20 },
      { status: "WOC", hoursAgo: 5 },
    ],
  },
  {
    note: "P3, business-hours accrual of 48h budget => OK, paused",
    priority: "Medium(P3)",
    state: "OPEN",
    transitions: [
      { status: "Open", hoursAgo: 60 },
      { status: "In Progress", hoursAgo: 40 },
      { status: "WOC", hoursAgo: 10 },
    ],
  },
  {
    note: "P4 has no SLA => NO_SLA (kept indefinitely)",
    priority: "Low(P4)",
    state: "OPEN",
    transitions: [
      { status: "Open", hoursAgo: 20 },
      { status: "In Progress", hoursAgo: 3 },
    ],
  },
  {
    note: "P1 currently Resolved => TERMINAL, excluded from alerts",
    priority: "Critical(P1)",
    state: "OPEN",
    transitions: [
      { status: "Open", hoursAgo: 200 },
      { status: "In Progress", hoursAgo: 150 },
      { status: "Resolved", hoursAgo: 2 },
    ],
  },
  {
    note: "No priority label => NO_SLA, never alerted",
    priority: null,
    state: "OPEN",
    transitions: [
      { status: "Open", hoursAgo: 48 },
      { status: "In Progress", hoursAgo: 10 },
    ],
  },
  {
    note: "P2 pause+resume; 4h + 24h = ~28h => VIOLATED, running",
    priority: "High(P2)",
    state: "OPEN",
    transitions: [
      { status: "Open", hoursAgo: 40 },
      { status: "WOC", hoursAgo: 36 },
      { status: "In Progress", hoursAgo: 24 },
    ],
  },
  {
    note: "P3 closed + Resolved => TERMINAL",
    priority: "Medium(P3)",
    state: "CLOSED",
    transitions: [
      { status: "Open", hoursAgo: 300 },
      { status: "In Progress", hoursAgo: 250 },
      { status: "Resolved", hoursAgo: 240 },
    ],
  },
  {
    note: "P2, empty timeline, current status set ~30h ago (empty-timeline path) => VIOLATED, running",
    priority: "High(P2)",
    state: "OPEN",
    transitions: [],
    currentStatus: "Open",
    currentStatusAgo: 30,
  },
];

function buildEvents(transitions: Transition[]): GhStatusEvent[] {
  return transitions.map((t, i) => ({
    createdAt: h(t.hoursAgo),
    previousStatus: i === 0 ? null : transitions[i - 1].status,
    status: t.status,
  }));
}

export function syntheticRepoIssues(
  repo: ConfigRepo,
  repoIndex: number,
): Array<{ node: GhIssueNode; detail: GhIssueDetail }> {
  return SCENARIOS.map((s, i) => {
    const number = (repoIndex + 1) * 1000 + (i + 1);
    const events = buildEvents(s.transitions);
    const createdAt = events.length ? events[0].createdAt : h(s.currentStatusAgo ?? 72);
    const lastTs = events.length ? events[events.length - 1].createdAt : createdAt;
    const currentStatus =
      s.currentStatus ?? (s.transitions.length ? s.transitions[s.transitions.length - 1].status : null);
    const currentStatusTs = s.currentStatus != null ? h(s.currentStatusAgo!) : lastTs;

    const labelNames = [...(s.priority ? [`Priority/${s.priority}`] : []), "Origin/CS"];

    const node: GhIssueNode = {
      number,
      state: s.state,
      url: `https://github.com/${repo.owner}/${repo.name}/issues/${number}`,
      createdAt,
      updatedAt: currentStatusTs,
      closedAt: s.state === "CLOSED" ? currentStatusTs : null,
      labels: { nodes: labelNames.map((name) => ({ name })) },
    };

    const detail: GhIssueDetail = {
      number,
      events,
      projectStatuses: [
        {
          projectId: repo.githubProjectId,
          status: currentStatus,
          statusUpdatedAt: currentStatusTs,
          itemCreatedAt: events.length ? events[0].createdAt : createdAt,
        },
      ],
    };

    return { node, detail };
  });
}
