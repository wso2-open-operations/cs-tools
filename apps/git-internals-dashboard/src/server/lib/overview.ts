// src/server/lib/overview.ts
import { loadConfig } from "@/server/config";
import { prisma, Prisma } from "@/server/db";
import { getCsStatuses, getProductSideStatuses } from "@/server/lib/taxonomy";

const P_CODE_RE = /\((P[1-4])\)/;
const pCode = (p: string) => P_CODE_RE.exec(p)?.[1] ?? p;
const pLabel = (p: string) => p.replace(/\s*\(P[1-4]\)\s*$/, "").trim();
const PRIORITY_RANK: Record<string, number> = { P1: 1, P2: 2, P3: 3, P4: 4 };

interface SparkRow { snapshot_date: Date; n: bigint }

// Build a 16-element array (oldest -> newest) for the last 16 days, gap-filled.
function fillSpark(rows: SparkRow[], days = 16): number[] {
  const map = new Map<string, number>();
  for (const r of rows) map.set(r.snapshot_date.toISOString().slice(0, 10), Number(r.n));
  const out: number[] = [];
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date();
    d.setUTCHours(0, 0, 0, 0);
    d.setUTCDate(d.getUTCDate() - i);
    out.push(map.get(d.toISOString().slice(0, 10)) ?? 0);
  }
  return out;
}

export async function buildOverview(repo: string | undefined, priority: string | undefined) {
  const csStatuses = await getCsStatuses();
  const isCsStatus = (s: string) => csStatuses.includes(s);
  const productSideStatuses = await getProductSideStatuses();
  const isProductSideStatus = (s: string) => productSideStatuses.includes(s);

  let repoOwner: string | null = null;
  let repoName: string | null = null;
  if (repo) [repoOwner, repoName] = repo.split("/");

  // ── 1. All open non-terminal issues from enabled repos (narrow select) ──────
  const allIssues = await prisma.issue.findMany({
    where: {
      state: "OPEN",
      repository: { is: { enabled: true } },
      sla: { is: { slaState: { not: "TERMINAL" } } },
    },
    select: {
      id: true,
      priority: true,
      currentStatus: true,
      repository: {
        select: { id: true, owner: true, name: true, slaProject: { select: { title: true } } },
      },
      sla: { select: { slaState: true } },
    },
  });
  // File presence is enablement now — no `enabled` filter needed.
  const budgets = loadConfig().budgets.map((b) => ({ priority: b.priority, budgetHours: b.budgetHours }));

  // Filter scopes: hero + spark honor repo + priority; priorities + matrix honor repo only.
  const heroIssues = allIssues.filter(
    (i) =>
      (!repoOwner || (i.repository.owner === repoOwner && i.repository.name === repoName)) &&
      (!priority || i.priority === priority),
  );
  const repoFilteredIssues = allIssues.filter(
    (i) => !repoOwner || (i.repository.owner === repoOwner && i.repository.name === repoName),
  );

  // ── 2. Spark + delta (last 16 days; delta = today − yesterday) ─────────────
  const repoFilterSql =
    repoOwner && repoName
      ? Prisma.sql`AND s.repository_id = (SELECT id FROM igid_repositories WHERE owner = ${repoOwner} AND name = ${repoName})`
      : Prisma.empty;
  const priorityFilterSql = priority ? Prisma.sql`AND s.priority = ${priority}` : Prisma.empty;

  const sparkQuery = (slaState: string) => prisma.$queryRaw<SparkRow[]>`
    SELECT s.snapshot_date, COUNT(*)::int AS n
    FROM igid_sla_snapshots s
    JOIN igid_issues i ON i.id = s.issue_id
    JOIN igid_repositories r ON r.id = s.repository_id
    WHERE s.sla_state = ${slaState}
      AND s.snapshot_date >= (now() AT TIME ZONE 'UTC')::date - 15
      AND i.state = 'OPEN'
      AND r.enabled = true
      ${repoFilterSql}
      ${priorityFilterSql}
    GROUP BY s.snapshot_date
    ORDER BY s.snapshot_date`;

  // Product-side spark uses the status list, not sla_state — same
  // sla_snapshots table/gap-fill/repo+priority-scoping pattern as
  // violated/atRisk, just filtered by current_status IN (...) instead.
  // Prisma.join() throws on an empty array (unlike a plain SQL `IN ()`), so an
  // empty PRODUCT_SIDE category (a valid taxonomy.yaml edit) must short-circuit
  // rather than reach the query.
  const productSideSparkQuery = () =>
    productSideStatuses.length === 0
      ? Promise.resolve([])
      : prisma.$queryRaw<SparkRow[]>`
    SELECT s.snapshot_date, COUNT(*)::int AS n
    FROM igid_sla_snapshots s
    JOIN igid_issues i ON i.id = s.issue_id
    JOIN igid_repositories r ON r.id = s.repository_id
    WHERE s.current_status IN (${Prisma.join(productSideStatuses)})
      AND s.snapshot_date >= (now() AT TIME ZONE 'UTC')::date - 15
      AND i.state = 'OPEN'
      AND r.enabled = true
      ${repoFilterSql}
      ${priorityFilterSql}
    GROUP BY s.snapshot_date
    ORDER BY s.snapshot_date`;

  const [violatedSparkRows, atRiskSparkRows, productSideSparkRows] = await Promise.all([
    sparkQuery("VIOLATED"),
    sparkQuery("AT_RISK"),
    productSideSparkQuery(),
  ]);

  const violatedSpark = fillSpark(violatedSparkRows);
  const atRiskSpark = fillSpark(atRiskSparkRows);
  const productSideSpark = fillSpark(productSideSparkRows);
  const violatedDelta = violatedSpark[15]! - violatedSpark[14]!;
  const atRiskDelta = atRiskSpark[15]! - atRiskSpark[14]!;
  const productSideDelta = productSideSpark[15]! - productSideSpark[14]!;

  // ── 3. Hero aggregation (repo + priority filtered) ─────────────────────────
  let heroViolated = 0, heroAtRisk = 0, heroCs = 0, heroProductSide = 0;
  const heroCsByStatus = new Map<string, number>(csStatuses.map((s) => [s, 0]));
  for (const issue of heroIssues) {
    const state = issue.sla?.slaState ?? "NO_SLA";
    const status = issue.currentStatus ?? "";
    if (state === "VIOLATED") heroViolated++;
    if (state === "AT_RISK") heroAtRisk++;
    if (isCsStatus(status)) {
      heroCs++;
      heroCsByStatus.set(status, (heroCsByStatus.get(status) ?? 0) + 1);
    }
    if (isProductSideStatus(status)) heroProductSide++;
  }

  // ── 4. Projects (always all enabled repos; per-card counts honor priority) ─
  type RepoAgg = {
    repoId: number; name: string; repo: string;
    violated: number; atRisk: number; cs: number;
    onTrack: number; openTracked: number; untracked: number;
  };
  const repoMap = new Map<number, RepoAgg>();

  for (const issue of allIssues) {
    const rId = issue.repository.id;
    if (!repoMap.has(rId)) {
      repoMap.set(rId, {
        repoId: rId,
        name: issue.repository.slaProject?.title ?? issue.repository.name,
        repo: `${issue.repository.owner}/${issue.repository.name}`,
        violated: 0, atRisk: 0, cs: 0,
        onTrack: 0, openTracked: 0, untracked: 0,
      });
    }
  }

  const projectScope = priority ? allIssues.filter((i) => i.priority === priority) : allIssues;
  for (const issue of projectScope) {
    const state = issue.sla?.slaState ?? "NO_SLA";
    const status = issue.currentStatus ?? "";
    const isCs = isCsStatus(status);
    const r = repoMap.get(issue.repository.id)!;
    if (state === "VIOLATED") r.violated++;
    if (state === "AT_RISK") r.atRisk++;
    if (isCs) r.cs++;
    if (state === "OK" && !isCs) r.onTrack++;
    if (issue.priority) r.openTracked++; else r.untracked++;
  }

  const projectsArr = [...repoMap.values()];
  const worstScore = (p: RepoAgg) => p.violated * 3 + p.atRisk;
  const maxScore = Math.max(...projectsArr.map(worstScore), 0);
  const projects = projectsArr.map((p) => ({
    ...p,
    worst: maxScore > 0 && worstScore(p) === maxScore,
    allClear: p.violated + p.atRisk + p.cs === 0,
  }));

  // ── 5. Priorities (honors repo; always the 4 canonical tiers) ──────────────
  type PriAgg = { violated: number; atRisk: number; cs: number; onTrack: number; total: number };
  const budgetMap = new Map(budgets.map((b) => [b.priority, b.budgetHours]));
  const priorityMap = new Map<string, PriAgg>();
  for (const b of budgets) {
    priorityMap.set(b.priority, { violated: 0, atRisk: 0, cs: 0, onTrack: 0, total: 0 });
  }

  for (const issue of repoFilteredIssues) {
    if (!issue.priority) continue;
    const p = priorityMap.get(issue.priority);
    if (!p) continue; // non-canonical priority label — not one of the 4 tiers
    const state = issue.sla?.slaState ?? "NO_SLA";
    const isCs = isCsStatus(issue.currentStatus ?? "");
    p.total++;
    if (state === "VIOLATED") p.violated++;
    if (state === "AT_RISK") p.atRisk++;
    if (isCs) p.cs++;
    if (state === "OK" && !isCs) p.onTrack++;
  }

  const priorities = [...priorityMap.entries()]
    .map(([key, counts]) => ({
      key,
      code: pCode(key) as "P1" | "P2" | "P3" | "P4",
      label: pLabel(key),
      budgetHours: budgetMap.get(key) ?? 0,
      ...counts,
    }))
    .sort((a, b) => (PRIORITY_RANK[a.code] ?? 99) - (PRIORITY_RANK[b.code] ?? 99));

  // ── 6. Matrix (honors repo; all 4 tiers, independent cells) ────────────────
  const matrixRows = priorities.map((p) => {
    const group = repoFilteredIssues.filter((i) => i.priority === p.key);
    return {
      key: p.key,
      code: p.code,
      cells: {
        violated: group.filter((i) => i.sla?.slaState === "VIOLATED").length,
        atRisk: group.filter((i) => i.sla?.slaState === "AT_RISK").length,
        onTrack: group.filter((i) => i.sla?.slaState === "OK" && !isCsStatus(i.currentStatus ?? "")).length,
        cs: group.filter((i) => isCsStatus(i.currentStatus ?? "")).length,
      },
      total: group.length,
    };
  });

  const tracked = repoFilteredIssues.filter((i) => i.priority && budgetMap.has(i.priority));
  const grandTotal = tracked.length;
  const matrixTotals = {
    violated: tracked.filter((i) => i.sla?.slaState === "VIOLATED").length,
    atRisk: tracked.filter((i) => i.sla?.slaState === "AT_RISK").length,
    onTrack: tracked.filter((i) => i.sla?.slaState === "OK" && !isCsStatus(i.currentStatus ?? "")).length,
    cs: tracked.filter((i) => isCsStatus(i.currentStatus ?? "")).length,
  };

  // ── 7. Volume (ignores both filters — 12 UTC weeks of tracked issues) ──────
  interface WeekRow { repository_id: number; wk: Date; priority: string; n: bigint }
  const weekRows = await prisma.$queryRaw<WeekRow[]>`
    SELECT
      i.repository_id,
      date_trunc('week', i.github_created_at AT TIME ZONE 'UTC')::date AS wk,
      i.priority AS priority,
      COUNT(*)::int AS n
    FROM igid_issues i
    JOIN igid_repositories r ON r.id = i.repository_id
    WHERE i.github_created_at >= (date_trunc('week', now() AT TIME ZONE 'UTC') - INTERVAL '11 weeks') AT TIME ZONE 'UTC'
      AND r.enabled = true
      AND i.priority IS NOT NULL
    GROUP BY i.repository_id, wk, i.priority
    ORDER BY i.repository_id, wk`;

  const today = new Date();
  today.setUTCHours(0, 0, 0, 0);
  const daysToMonday = (today.getUTCDay() + 6) % 7;
  const currentMonday = new Date(today.getTime() - daysToMonday * 86_400_000);
  const weekKeys = Array.from({ length: 12 }, (_, i) => {
    const w = new Date(currentMonday.getTime() - (11 - i) * 7 * 86_400_000);
    return w.toISOString().slice(0, 10);
  });

  type WeekBuckets = { P1: number; P2: number; P3: number; P4: number };
  const emptyBuckets = (): WeekBuckets => ({ P1: 0, P2: 0, P3: 0, P4: 0 });
  const weekByRepo = new Map<number, Map<string, WeekBuckets>>();
  for (const row of weekRows) {
    const rId = Number(row.repository_id);
    if (!weekByRepo.has(rId)) weekByRepo.set(rId, new Map());
    const wkMap = weekByRepo.get(rId)!;
    const wkKey = row.wk.toISOString().slice(0, 10);
    if (!wkMap.has(wkKey)) wkMap.set(wkKey, emptyBuckets());
    const code = pCode(row.priority) as keyof WeekBuckets;
    const b = wkMap.get(wkKey)!;
    if (code in b) b[code] += Number(row.n);
  }

  const volume = projectsArr.map((r) => {
    const weeks = weekKeys.map((wk) => {
      const b = weekByRepo.get(r.repoId)?.get(wk) ?? emptyBuckets();
      return { weekStart: wk, byPriority: b, total: b.P1 + b.P2 + b.P3 + b.P4 };
    });
    return { repoId: r.repoId, name: r.name, total: weeks.reduce((s, w) => s + w.total, 0), weeks };
  });

  return {
    refreshedAt: new Date().toISOString(),
    filters: { repo: repo ?? null, priority: priority ?? null },
    hero: {
      violated: { n: heroViolated, delta: violatedDelta, spark: violatedSpark },
      atRisk: { n: heroAtRisk, delta: atRiskDelta, spark: atRiskSpark },
      cs: { n: heroCs, byStatus: csStatuses.map((s) => ({ status: s, n: heroCsByStatus.get(s) ?? 0 })) },
      productSide: { n: heroProductSide, delta: productSideDelta, spark: productSideSpark },
    },
    projects,
    priorities,
    matrix: { rows: matrixRows, totals: matrixTotals, grandTotal },
    volume,
  };
}
