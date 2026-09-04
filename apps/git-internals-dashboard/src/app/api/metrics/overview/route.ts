import type { NextRequest } from "next/server";
import { requireAuth } from "@/server/auth";
import { overviewQuery } from "@/server/schemas";
import { TtlCache } from "@/server/lib/ttl-cache";
import { buildOverview } from "@/server/lib/overview";

// Response micro-cache
const overviewCache = new TtlCache<string, unknown>(30_000, 64);

// GET /api/metrics/overview
// Return: Overview of metrics for the given repo and priority
export const GET = requireAuth(async (req: NextRequest) => {
  const parsed = overviewQuery.safeParse(Object.fromEntries(req.nextUrl.searchParams));
  if (!parsed.success) return Response.json({ error: parsed.error.flatten() }, { status: 400 });
  const { repo, priority } = parsed.data;

  const result = await overviewCache.getOrSet(`${repo ?? ""}|${priority ?? ""}`, () =>
    buildOverview(repo, priority),
  );
  return Response.json(result);
});
