"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Box, Skeleton } from "@mui/material";
import { useOverview, useTaxonomy, makeIsCsStatus } from "@/lib/api";
import { HeroCard, CsHeroCard } from "@/components/HeroCard";
import { ProjectCard } from "@/components/ProjectCard";
import { PriorityTierCard } from "@/components/PriorityTierCard";
import { PriorityStateMatrix } from "@/components/PriorityStateMatrix";
import { TimeseriesChart } from "@/components/TimeseriesChart";
import { ClosestToBreach } from "@/components/ClosestToBreach";
import { VolumePanel } from "@/components/VolumePanel";
import { AttentionSet } from "@/components/AttentionSet";
import { acrylicSurfaceSx } from "@/lib/surfaces";

function buildDrillUrl(
  bucket: string,
  opts: { repo?: string | null; priority?: string | null; status?: string | null },
  currentSearch: string,
): string {
  const base = new URLSearchParams(currentSearch);
  const next = new URLSearchParams();
  next.set("bucket", bucket);
  const resolve = (key: "repo" | "priority") => {
    const v = opts[key] === undefined ? base.get(key) : opts[key];
    if (v) next.set(key, v);
  };
  resolve("repo");
  resolve("priority");
  if (opts.status) next.set("status", opts.status);
  return `/issues?${next.toString()}`;
}

const VOL_LEGEND = [
  { code: "P1", label: "Critical", color: "var(--sla-p1)" },
  { code: "P2", label: "High", color: "var(--sla-p2)" },
  { code: "P3", label: "Medium", color: "var(--sla-p3)" },
  { code: "P4", label: "Low", color: "var(--sla-p4)" },
];

const sectionLabelSx = { px: "2px", fontSize: 11, fontWeight: 600, textTransform: "uppercase" as const, letterSpacing: "0.1em", color: "var(--sla-fg3)" };

export function DashboardPage() {
  const params = useSearchParams() ?? new URLSearchParams();
  const router = useRouter();

  const repo = params.get("repo") ?? undefined;
  const priority = params.get("priority") ?? undefined;

  const { data: overview, isLoading } = useOverview(repo, priority);
  const { data: taxonomy } = useTaxonomy();
  const isCsStatus = makeIsCsStatus(taxonomy?.csStatuses);

  const setFilter = (key: "repo" | "priority", value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    router.replace(`/?${next.toString()}`.replace(/\?$/, ""));
  };

  const drill = (
    bucket: string,
    opts: { repo?: string | null; priority?: string | null; status?: string | null } = {},
  ) => {
    router.push(buildDrillUrl(bucket, opts, params.toString()));
  };

  if (isLoading || !overview) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Box sx={{ display: "grid", gap: "18px", gridTemplateColumns: { lg: "1.55fr .9fr" } }}>
          <Box sx={{ display: "grid", gap: 1.75, gridTemplateColumns: { sm: "1fr 1fr", lg: "1fr 1fr 1fr" } }}>
            <Skeleton variant="rounded" sx={{ height: 144, borderRadius: "16px" }} />
            <Skeleton variant="rounded" sx={{ height: 144, borderRadius: "16px" }} />
            <Skeleton variant="rounded" sx={{ height: 144, borderRadius: "16px" }} />
          </Box>
          <Skeleton variant="rounded" sx={{ height: 144, borderRadius: "16px" }} />
        </Box>
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { md: "repeat(3, 1fr)" } }}>
          {[0, 1, 2].map((i) => <Skeleton key={i} variant="rounded" sx={{ height: 176, borderRadius: "16px" }} />)}
        </Box>
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "repeat(2, 1fr)", lg: "repeat(4, 1fr)" }, gap: 1.75 }}>
          {[0, 1, 2, 3].map((i) => <Skeleton key={i} variant="rounded" sx={{ height: 192, borderRadius: "16px" }} />)}
        </Box>
      </Box>
    );
  }

  const allClear = overview.hero.violated.n + overview.hero.atRisk.n + overview.hero.cs.n === 0;
  const worst = overview.projects.find((p) => p.worst);
  const volMax = Math.max(1, ...overview.volume.flatMap((v) => v.weeks.map((w) => w.total)));
  const repoForId = (id: number) => overview.projects.find((p) => p.repoId === id)?.repo;

  return (
    <Box>
      {/* All-clear banner */}
      {allClear && (
        <Box sx={{ mb: "22px", display: "flex", alignItems: "center", gap: 1.5, borderRadius: "12px", border: "1px solid color-mix(in srgb, var(--sla-ok) 35%, transparent)", bgcolor: "var(--sla-ok-tint)", px: "18px", py: 1.75 }}>
          <Box component="span" sx={{ display: "flex", height: 26, width: 26, alignItems: "center", justifyContent: "center", borderRadius: "50%", bgcolor: "var(--sla-ok)", fontSize: 15, fontWeight: 700, color: "var(--sla-ok-contrast-text)" }}>✓</Box>
          <Box>
            <Box sx={{ fontSize: 14, fontWeight: 600, color: "var(--sla-ok)" }}>All three targets are at zero.</Box>
            <Box sx={{ fontSize: 12.5, color: "var(--sla-ok)" }}>
              Product team is within SLA across every area — no violated, at-risk, or CS-side issues right now.
            </Box>
          </Box>
        </Box>
      )}

      {/* Hero attention bar */}
      <Box component="section" sx={{ mb: "26px", display: "grid", gap: "18px", gridTemplateColumns: { lg: "1.55fr .9fr" } }}>
        <Box sx={{ display: "grid", gap: 1.75, gridTemplateColumns: { sm: "1fr 1fr", lg: "1fr 1fr 1fr" } }}>
          <HeroCard
            label="Violated"
            n={overview.hero.violated.n}
            delta={overview.hero.violated.delta}
            spark={overview.hero.violated.spark}
            accent="var(--sla-violated)"
            onClick={() => drill("violated")}
          />
          <HeroCard
            label="At risk"
            n={overview.hero.atRisk.n}
            delta={overview.hero.atRisk.delta}
            spark={overview.hero.atRisk.spark}
            accent="var(--sla-at-risk)"
            onClick={() => drill("at_risk")}
          />
          {/* No onClick: there's no bucket=product_side drill-down filter in
              the API today (see /api/issues's bucket enum) and adding one
              wasn't part of this widget's ask ("displays a single count") —
              flagged rather than silently inventing a new backend filter. */}
          <HeroCard
            label="On Product Team Side"
            n={overview.hero.productSide.n}
            delta={overview.hero.productSide.delta}
            spark={overview.hero.productSide.spark}
            accent="var(--sla-primary)"
          />
        </Box>
        <CsHeroCard
          n={overview.hero.cs.n}
          byStatus={overview.hero.cs.byStatus}
          onDrill={(status) => drill("cs", { status })}
        />
      </Box>

      {/* Per-project comparison */}
      <Box component="section" sx={{ mb: "30px" }}>
        <Box sx={{ mb: 1.5, display: "flex", alignItems: "baseline", justifyContent: "space-between" }}>
          <Box sx={sectionLabelSx}>SLA load by project</Box>
          {worst && (
            <Box component="span" sx={{ fontSize: 12.5, color: "var(--sla-fg3)" }}>
              Needs attention: <Box component="b" sx={{ color: "var(--sla-violated)" }}>{worst.name}</Box>
            </Box>
          )}
        </Box>
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { md: "repeat(3, 1fr)" } }}>
          {overview.projects.map((p) => (
            <ProjectCard
              key={p.repoId}
              project={p}
              focused={repo === p.repo}
              dim={!!repo && repo !== p.repo}
              onFocus={() => setFilter("repo", repo === p.repo ? "" : p.repo)}
              onDrill={(bucket) => drill(bucket, { repo: p.repo })}
            />
          ))}
        </Box>
      </Box>

      {/* Priority breakdown */}
      <Box component="section" sx={{ mb: "14px" }}>
        <Box sx={{ ...sectionLabelSx, mb: 1.25 }}>SLA load by incident priority</Box>
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "repeat(2, 1fr)", lg: overview.priorities.length >= 4 ? "repeat(4, 1fr)" : "repeat(3, 1fr)" }, gap: 1.75 }}>
          {overview.priorities.map((tier) => (
            <PriorityTierCard
              key={tier.key}
              tier={tier}
              focused={priority === tier.key}
              dim={!!priority && priority !== tier.key}
              onFocusToggle={() => setFilter("priority", priority === tier.key ? "" : tier.key)}
              onDrill={(bucket) => drill(bucket, { priority: tier.key })}
            />
          ))}
        </Box>
      </Box>

      {/* Matrix + trend */}
      <Box component="section" sx={{ mb: "22px", display: "grid", gap: "18px", gridTemplateColumns: { lg: ".92fr 1.08fr" } }}>
        <PriorityStateMatrix
          matrix={overview.matrix}
          onDrill={(bucket, pKey) => drill(bucket, { priority: pKey ?? null })}
        />
        <TimeseriesChart
          repo={repo}
          activePriority={priority}
          onPriorityFilter={(pKey) => setFilter("priority", pKey)}
        />
      </Box>

      {/* Closest to breach */}
      <Box sx={{ mb: "22px" }}>
        <ClosestToBreach repo={repo} priority={priority} projects={overview.projects} />
      </Box>

      {/* New-issue volume */}
      {overview.volume.length > 0 && (
        <Box component="section" sx={{ ...acrylicSurfaceSx, mb: "22px", borderRadius: "16px", border: "1px solid var(--sla-border)", px: "22px", py: 2.5, boxShadow: "0 1px 2px rgba(17,24,39,.04)" }}>
          <Box component="h2" sx={{ m: 0, fontSize: 15, fontWeight: 600, lineHeight: 1.2, letterSpacing: "-0.01em" }}>New-issue volume</Box>
          <Box sx={{ mb: "18px", mt: 0.25, fontSize: 12.5, color: "var(--sla-fg3)" }}>CS issues created per week (last 12 weeks)</Box>
          <Box sx={{ display: "grid", gap: "18px", gridTemplateColumns: { md: "repeat(3, 1fr)" } }}>
            {overview.volume.map((v) => (
              <VolumePanel
                key={v.repoId}
                item={v}
                maxWeek={volMax}
                onFilter={() => {
                  const r = repoForId(v.repoId);
                  if (r) setFilter("repo", repo === r ? "" : r);
                }}
              />
            ))}
          </Box>
          <Box sx={{ mt: 2, display: "flex", gap: 2, borderTop: "1px solid var(--sla-border-soft)", pt: 1.75, fontSize: 12, color: "var(--sla-fg2)" }}>
            {VOL_LEGEND.map((l) => (
              <Box key={l.code} component="span" sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                <Box component="span" sx={{ height: 10, width: 10, borderRadius: "2px", bgcolor: l.color }} />
                {l.code} {l.label}
              </Box>
            ))}
          </Box>
        </Box>
      )}

      {/* Attention set */}
      <AttentionSet
        hero={overview.hero}
        projects={overview.projects}
        repo={repo}
        priority={priority}
        isCsStatus={isCsStatus}
      />

      {/* Footer */}
      <Box sx={{ mt: 3, textAlign: "center", fontSize: 11.5, color: "var(--sla-no-sla)" }}>
        Read-only monitoring surface · Closed &amp; Terminal issues excluded from all KPIs and graphs · KPIs are computed independently and may overlap
      </Box>
    </Box>
  );
}
