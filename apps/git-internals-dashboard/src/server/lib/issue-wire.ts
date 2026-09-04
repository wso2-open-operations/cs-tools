// src/server/lib/issue-wire.ts
// Shared privacy-flattened issue shape used by both /api/issues and
// /api/issues/:id — carries no title, assignees, opener, or labels.
import type { Prisma } from "@/server/db";

export const ISSUE_LIST_SELECT = {
  id: true,
  githubNumber: true,
  state: true,
  htmlUrl: true,
  priority: true,
  currentStatus: true,
  githubCreatedAt: true,
  githubUpdatedAt: true,
  repository: { select: { owner: true, name: true } },
  sla: {
    select: {
      budgetHours: true,
      consumedHours: true,
      remainingHours: true,
      pctConsumed: true,
      slaState: true,
      slaRunning: true,
    },
  },
} satisfies Prisma.IssueSelect;

export type IssueForFlatten = Prisma.IssueGetPayload<{ select: typeof ISSUE_LIST_SELECT }>;

export function flattenIssue(issue: IssueForFlatten) {
  return {
    id: issue.id,
    number: issue.githubNumber,
    state: issue.state,
    url: issue.htmlUrl,
    repo: issue.repository ? `${issue.repository.owner}/${issue.repository.name}` : null,
    priority: issue.priority,
    currentStatus: issue.currentStatus,
    githubCreatedAt: issue.githubCreatedAt,
    githubUpdatedAt: issue.githubUpdatedAt,
    sla: issue.sla && {
      budgetHours: issue.sla.budgetHours,
      consumedHours: issue.sla.consumedHours,
      remainingHours: issue.sla.remainingHours,
      pctConsumed: issue.sla.pctConsumed,
      slaState: issue.sla.slaState,
      slaRunning: issue.sla.slaRunning,
    },
  };
}
