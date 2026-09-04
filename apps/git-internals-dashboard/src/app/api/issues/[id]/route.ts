import { prisma } from "@/server/db";
import { requireAuth } from "@/server/auth";
import { idParam } from "@/server/schemas";
import { flattenIssue } from "@/server/lib/issue-wire";

// GET /api/issues/:id
// Return: Git issue details + its full status-event timeline.
export const GET = requireAuth(async (_req, ctx) => {
  const parsed = idParam.safeParse(await ctx.params);
  if (!parsed.success) return Response.json({ error: parsed.error.flatten() }, { status: 400 });

  const issue = await prisma.issue.findFirst({
    where: { id: parsed.data.id, repository: { is: { enabled: true } } },
    include: {
      sla: true,
      repository: { select: { owner: true, name: true } },
      events: {
        orderBy: { occurredAt: "asc" },
        select: { id: true, previousStatus: true, status: true, occurredAt: true },
      },
    },
  });
  if (!issue) return Response.json({ error: "issue not found" }, { status: 404 });

  return Response.json({ ...flattenIssue(issue), events: issue.events });
});
