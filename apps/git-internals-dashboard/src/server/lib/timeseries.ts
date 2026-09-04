// src/server/lib/timeseries.ts
import { loadConfig } from "@/server/config";
import { prisma, Prisma } from "@/server/db";

const P_CODE_RE = /\((P[1-4])\)/;

export async function buildTimeseries(
  repo: string | undefined,
  days: number,
  groupBy: "priority" | "none",
  metric: "violated" | "at_risk" | "total",
) {
  let repoOwner: string | null = null;
  let repoName: string | null = null;
  if (repo) [repoOwner, repoName] = repo.split("/");

  const repoFilter =
    repoOwner && repoName
      ? Prisma.sql`AND rep.owner = ${repoOwner} AND rep.name = ${repoName}`
      : Prisma.empty;

  const metricFilter =
    metric === "violated"
      ? Prisma.sql`AND s.sla_state = 'VIOLATED'`
      : metric === "at_risk"
        ? Prisma.sql`AND s.sla_state = 'AT_RISK'`
        : Prisma.sql`AND s.priority IS NOT NULL`; // "total" = tracked only

  interface RawRow { snapshot_date: Date; priority: string | null; n: bigint }

  const rows = await prisma.$queryRaw<RawRow[]>`
      SELECT
        s.snapshot_date,
        ${groupBy === "priority" ? Prisma.sql`s.priority` : Prisma.sql`NULL::text AS priority`},
        COUNT(*)::int AS n
      FROM igid_sla_snapshots s
      JOIN igid_repositories rep ON rep.id = s.repository_id
      WHERE s.snapshot_date >= (now() AT TIME ZONE 'UTC')::date - ${days - 1}::int
        AND rep.enabled = true
        ${repoFilter}
        ${metricFilter}
      GROUP BY s.snapshot_date${groupBy === "priority" ? Prisma.sql`, s.priority` : Prisma.empty}
      ORDER BY s.snapshot_date`;

  const budgets =
    groupBy === "priority"
      ? [...loadConfig().budgets].sort((a, b) => a.rank - b.rank).map((b) => ({ priority: b.priority }))
      : [];

  // Complete date range for the window (no gaps for days with no data).
  const dates: string[] = [];
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date();
    d.setUTCHours(0, 0, 0, 0);
    d.setUTCDate(d.getUTCDate() - i);
    dates.push(d.toISOString().slice(0, 10));
  }

  // Pre-seed all series keys so every priority appears even with zero data.
  const seriesMap = new Map<string, number[]>();
  if (groupBy === "priority") {
    for (const b of budgets) seriesMap.set(b.priority, new Array(dates.length).fill(0));
  } else {
    seriesMap.set("all", new Array(dates.length).fill(0));
  }
  const dateIndex = new Map(dates.map((d, i) => [d, i])); // O(1) instead of indexOf
  for (const r of rows) {
    const key = r.priority ?? "all";
    if (!seriesMap.has(key)) seriesMap.set(key, new Array(dates.length).fill(0));
    const idx = dateIndex.get(r.snapshot_date.toISOString().slice(0, 10));
    if (idx !== undefined) seriesMap.get(key)![idx] = Number(r.n);
  }

  const rank: Record<string, number> = { P1: 1, P2: 2, P3: 3, P4: 4, all: 5 };
  const series = [...seriesMap.entries()]
    .map(([key, points]) => ({ key, label: P_CODE_RE.exec(key)?.[1] ?? key, points }))
    .sort(
      (a, b) =>
        (rank[P_CODE_RE.exec(a.key)?.[1] ?? "all"] ?? 5) -
        (rank[P_CODE_RE.exec(b.key)?.[1] ?? "all"] ?? 5),
    );

  return { window: days, metric, groupBy, dates, series };
}
