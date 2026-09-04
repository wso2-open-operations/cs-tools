"use client";

import { useState } from "react";
import { Box } from "@mui/material";
import { useIssues, useIssueTitles } from "@/lib/api";
import type { Overview, OverviewProject } from "@/lib/api";
import { IssueTimelineRow } from "@/components/IssueTimelineRow";
import { gridTemplate } from "@/lib/grid";
import { acrylicSurfaceSx } from "@/lib/surfaces";

const MONO = "var(--font-mono)";

type Category = "violated" | "at_risk" | "cs";

const CHIPS: { key: Category; label: string; color: string; tint: string }[] = [
  { key: "violated", label: "Violated", color: "var(--sla-violated)", tint: "var(--sla-violated-tint)" },
  { key: "at_risk", label: "At risk", color: "var(--sla-at-risk)", tint: "var(--sla-at-risk-tint)" },
  { key: "cs", label: "On CS side", color: "var(--sla-cs)", tint: "var(--sla-cs-tint)" },
];

interface AttentionSetProps {
  hero: Overview["hero"];
  projects: OverviewProject[];
  repo?: string;
  priority?: string;
  isCsStatus: (status: string | null | undefined) => boolean;
}

export function AttentionSet({ hero, projects, repo, priority, isCsStatus }: AttentionSetProps) {
  const [active, setActive] = useState<Set<Category>>(new Set(["violated", "at_risk", "cs"]));

  const { data: issues } = useIssues({ bucket: "attention", order: "budget_desc", repo, priority });

  const issueIds = (issues ?? []).map((i) => i.id);
  const { data: titles, isPending: titlesPending } = useIssueTitles(issueIds);

  const nameForRepo = (r: string | null) =>
    projects.find((p) => p.repo === r)?.name ?? r?.split("/")[1] ?? "—";

  const toggle = (key: Category) =>
    setActive((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const counts: Record<Category, number> = {
    violated: hero.violated.n,
    at_risk: hero.atRisk.n,
    cs: hero.cs.n,
  };

  const rows = (issues ?? []).filter((i) => {
    if (active.has("violated") && i.sla?.slaState === "VIOLATED") return true;
    if (active.has("at_risk") && i.sla?.slaState === "AT_RISK") return true;
    if (active.has("cs") && isCsStatus(i.currentStatus)) return true;
    return false;
  });

  const cols = gridTemplate(false);

  return (
    <Box sx={{ ...acrylicSurfaceSx, overflow: "hidden", borderRadius: "16px", border: "1px solid var(--sla-border)", boxShadow: "0 1px 2px rgba(17,24,39,.04)" }}>
      <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: "14px", px: "22px", pb: 2, pt: 2.5 }}>
        <Box component="h2" sx={{ m: 0, fontSize: 15, fontWeight: 600, lineHeight: 1.2, letterSpacing: "-0.01em" }}>
          Attention set{" "}
          <Box component="span" sx={{ fontSize: 13, color: "var(--sla-fg3)", fontFamily: MONO }}>· {rows.length} issues</Box>
        </Box>
        <Box sx={{ display: "flex", gap: 1 }}>
          {CHIPS.map((ch) => {
            const on = active.has(ch.key);
            return (
              <Box
                key={ch.key}
                component="button"
                type="button"
                onClick={() => toggle(ch.key)}
                sx={{
                  display: "flex", alignItems: "center", gap: 0.75, borderRadius: "8px", px: "11px", py: 0.75,
                  fontSize: 12.5, fontWeight: 500, cursor: "pointer",
                  border: `1px solid ${on ? ch.color : "var(--sla-border)"}`,
                  bgcolor: on ? ch.tint : "var(--sla-card)",
                  color: on ? ch.color : "var(--sla-fg3)",
                }}
              >
                <Box component="span" sx={{ height: 8, width: 8, borderRadius: "50%", bgcolor: ch.color, opacity: on ? 1 : 0.3 }} />
                {ch.label} <Box component="b" sx={{ fontWeight: 600, fontFamily: MONO }}>{counts[ch.key]}</Box>
              </Box>
            );
          })}
        </Box>
      </Box>

      {/* Column header */}
      <Box
        sx={{
          display: "grid", borderTop: "1px solid var(--sla-border-soft)", borderBottom: "1px solid var(--sla-border-soft)",
          bgcolor: "var(--sla-surface-muted)", px: "22px", py: 1.25, fontSize: 11, fontWeight: 600, textTransform: "uppercase",
          letterSpacing: "0.05em", color: "var(--sla-fg3)", gridTemplateColumns: cols,
        }}
      >
        <span>Issue</span>
        <span>Project</span>
        <span>Pri</span>
        <span>Status</span>
        <span>Budget</span>
        <Box component="span" sx={{ textAlign: "right" }}>Age</Box>
      </Box>

      {rows.length === 0 ? (
        <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 1.25, px: 2.5, py: 6 }}>
          <Box component="span" sx={{ display: "flex", height: 40, width: 40, alignItems: "center", justifyContent: "center", borderRadius: "50%", bgcolor: "var(--sla-ok-tint)", fontSize: 20, fontWeight: 700, color: "var(--sla-ok)" }}>✓</Box>
          <Box component="span" sx={{ fontSize: 14, fontWeight: 600, color: "var(--sla-ok)" }}>No issues in the attention set</Box>
          <Box component="span" sx={{ fontSize: 12.5, color: "var(--sla-fg3)" }}>Nothing is violated, at risk, or waiting on CS for the current filters.</Box>
        </Box>
      ) : (
        rows.map((issue) => (
          <IssueTimelineRow
            key={issue.id}
            issue={issue}
            title={titles?.[issue.id] ?? null}
            titleLoading={titlesPending}
            projectName={nameForRepo(issue.repo)}
            isCsStatus={isCsStatus}
          />
        ))
      )}
    </Box>
  );
}
