import type { NextRequest } from "next/server";
import { prisma, Prisma } from "@/server/db";
import { requireAuth } from "@/server/auth";
import { issueListQuery } from "@/server/schemas";
import { getCsStatuses } from "@/server/lib/taxonomy";
import { ISSUE_LIST_SELECT, flattenIssue } from "@/server/lib/issue-wire";

// GET /api/issues — filterable list
export const GET = requireAuth(async (req: NextRequest) => {
  const parsed = issueListQuery.safeParse(Object.fromEntries(req.nextUrl.searchParams));
  if (!parsed.success) return Response.json({ error: parsed.error.flatten() }, { status: 400 });
  const q = parsed.data;

  const csStatuses = await getCsStatuses();

  // Base scope: open non-terminal issues from enabled repos.
  const where: Prisma.IssueWhereInput = {
    state: "OPEN",
    repository: { is: { enabled: true } },
    sla: { is: { slaState: { not: "TERMINAL" } } },
  };

  if (q.repo) {
    const [owner, name] = q.repo.split("/"); // format guaranteed by zod
    where.repository = { is: { owner, name, enabled: true } };
  }
  if (q.priority) where.priority = q.priority;
  if (q.status) where.currentStatus = q.status;
  if (q.q) where.githubNumber = Number(q.q);

  // Bucket overrides the sla_state/state/status conditions.
  switch (q.bucket) {
    case "violated":
      where.sla = { is: { slaState: "VIOLATED" } };
      break;
    case "at_risk":
      where.sla = { is: { slaState: "AT_RISK" } };
      break;
    case "on_track":
      where.sla = { is: { slaState: "OK" } };
      where.currentStatus = { notIn: csStatuses };
      break;
    case "cs":
      // Narrow to a single CS status when one is requested, otherwise show all
      // CS statuses. The sla filter is cleared so NO_SLA issues currently on
      // the CS side are still included.
      where.currentStatus = q.status && csStatuses.includes(q.status) ? q.status : { in: csStatuses };
      where.sla = undefined;
      break;
    case "tracked":
      where.priority = { not: null };
      break;
    case "untracked":
      where.priority = null;
      where.sla = undefined;
      break;
    case "attention":
      where.OR = [
        { sla: { is: { slaState: { in: ["VIOLATED", "AT_RISK"] } } } },
        { currentStatus: { in: csStatuses } },
      ];
      where.sla = undefined;
      break;
    default:
      // "all" or undefined — keep base scope; honour explicit params.
      if (q.sla_state) where.sla = { is: { slaState: q.sla_state } };
      if (q.state) where.state = q.state;
      break;
  }

  // budget_desc: highest % consumed first, nulls last (uses the pct_consumed index).
  const orderBy: Prisma.IssueOrderByWithRelationInput =
    q.order === "budget_desc"
      ? { sla: { pctConsumed: { sort: "desc", nulls: "last" } } }
      : { githubUpdatedAt: "desc" };

  const rows = await prisma.issue.findMany({
    where,
    take: q.limit,
    orderBy,
    select: ISSUE_LIST_SELECT,
  });
  return Response.json(rows.map(flattenIssue));
});
