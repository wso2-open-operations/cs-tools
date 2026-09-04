import type { NextRequest } from "next/server";
import { requireAuth } from "@/server/auth";
import { timeseriesQuery } from "@/server/schemas";
import { TtlCache } from "@/server/lib/ttl-cache";
import { buildTimeseries } from "@/server/lib/timeseries";

const timeseriesCache = new TtlCache<string, unknown>(60_000, 128);

// GET /api/metrics/timeseries
// Return: Daily SLA trend for the line chart
export const GET = requireAuth(async (req: NextRequest) => {
  const parsed = timeseriesQuery.safeParse(Object.fromEntries(req.nextUrl.searchParams));
  if (!parsed.success) return Response.json({ error: parsed.error.flatten() }, { status: 400 });
  const { repo, days, groupBy, metric } = parsed.data;

  const key = `${repo ?? ""}|${days}|${groupBy}|${metric}`;
  const result = await timeseriesCache.getOrSet(key, () => buildTimeseries(repo, days, groupBy, metric));
  return Response.json(result);
});
