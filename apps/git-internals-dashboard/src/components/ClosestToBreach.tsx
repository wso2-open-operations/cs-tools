import { Box } from "@mui/material";
import { useIssues } from "@/lib/api";
import type { OverviewProject } from "@/lib/api";
import { shortPriority } from "@/lib/sla";
import { acrylicSurfaceSx } from "@/lib/surfaces";

const MONO = "var(--font-mono)";

const STATE_COLOR: Record<string, string> = {
  VIOLATED: "var(--sla-violated)",
  AT_RISK: "var(--sla-at-risk)",
  OK: "var(--sla-ok)",
};

// Bar scale: budget bar maxes out at 140%; the dashed marker sits at 100% (1/1.4).
const SCALE = 1.4;
const MARKER_PCT = (1 / SCALE) * 100;

interface ClosestToBreachProps {
  repo?: string;
  priority?: string;
  projects: OverviewProject[];
}

export function ClosestToBreach({ repo, priority, projects }: ClosestToBreachProps) {
  const { data: issues } = useIssues({ bucket: "tracked", order: "budget_desc", limit: 9, repo, priority });

  const nameForRepo = (r: string | null) =>
    projects.find((p) => p.repo === r)?.name ?? r?.split("/")[1] ?? "—";

  const pool = (issues ?? []).filter((i) => i.sla?.pctConsumed != null);
  const rows = pool.some((i) => (i.sla!.pctConsumed as number) >= 0.75) ? pool : [];

  return (
    <Box sx={{ ...acrylicSurfaceSx, borderRadius: "16px", border: "1px solid var(--sla-border)", px: "22px", py: 2.5, boxShadow: "0 1px 2px rgba(17,24,39,.04)" }}>
      <Box sx={{ display: "flex", alignItems: "baseline", justifyContent: "space-between" }}>
        <Box component="h2" sx={{ m: 0, fontSize: 15, fontWeight: 600, lineHeight: 1.2, letterSpacing: "-0.01em" }}>
          Closest to breach
        </Box>
        <Box component="span" sx={{ fontSize: 12, color: "var(--sla-fg3)" }}>% of SLA budget consumed · dashed line = 100%</Box>
      </Box>
      <Box sx={{ mb: 2, mt: 0.25, fontSize: 12.5, color: "var(--sla-fg3)" }}>
        Top open tracked issues ranked by budget consumed
      </Box>

      {rows.length === 0 ? (
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, borderRadius: "10px", border: "1px solid color-mix(in srgb, var(--sla-ok) 35%, transparent)", bgcolor: "var(--sla-ok-tint)", px: 2, py: 1.75 }}>
          <Box component="span" sx={{ display: "flex", height: 24, width: 24, alignItems: "center", justifyContent: "center", borderRadius: "50%", bgcolor: "var(--sla-ok)", fontSize: 14, fontWeight: 700, color: "var(--sla-ok-contrast-text)" }}>✓</Box>
          <Box component="span" sx={{ fontSize: 13, fontWeight: 600, color: "var(--sla-ok)" }}>
            Nothing is within 75% of its SLA budget — no issues approaching breach.
          </Box>
        </Box>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: "9px" }}>
          {rows.map((issue) => {
            const pct = issue.sla?.pctConsumed ?? 0;
            const color = STATE_COLOR[issue.sla?.slaState ?? "OK"] ?? STATE_COLOR.OK;
            const widthPct = (Math.min(pct, SCALE) / SCALE) * 100;
            return (
              <Box
                key={issue.id}
                component="a"
                href={issue.url ?? "#"}
                target="_blank"
                rel="noopener noreferrer"
                title="Open issue on GitHub"
                sx={{ mx: "-5px", display: "flex", alignItems: "center", gap: "14px", borderRadius: "6px", px: "5px", py: "3px", fontSize: 12.5, textDecoration: "none", color: "inherit", "&:hover": { bgcolor: "color-mix(in srgb, var(--sla-fg) 4%, transparent)" } }}
              >
                <Box sx={{ display: "flex", width: 252, flexShrink: 0, alignItems: "center", gap: 1, overflow: "hidden" }}>
                  <Box component="span" sx={{ flexShrink: 0, fontWeight: 600, lineHeight: 1, color: "var(--sla-primary)", fontFamily: MONO }}>
                    #{issue.number}
                  </Box>
                  <Box component="span" sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "var(--sla-fg3)" }}>
                    {nameForRepo(issue.repo)}
                  </Box>
                  <Box component="span" sx={{ flexShrink: 0, borderRadius: "5px", bgcolor: "var(--sla-surface-track)", px: 0.75, py: 0.25, fontSize: 10, fontWeight: 600, lineHeight: 1, color: "var(--sla-fg2)", fontFamily: MONO }}>
                    {shortPriority(issue.priority)}
                  </Box>
                </Box>
                <Box sx={{ position: "relative", height: 24, flex: 1, overflow: "hidden", borderRadius: "6px", bgcolor: "var(--sla-surface-track)" }}>
                  <Box sx={{ position: "absolute", left: 0, top: 0, height: "100%", borderRadius: "6px", width: `${widthPct}%`, bgcolor: color }} />
                  <Box sx={{ position: "absolute", top: 0, bottom: 0, width: 0, borderLeft: "2px dashed color-mix(in srgb, var(--sla-fg) 30%, transparent)", left: `${MARKER_PCT}%` }} />
                </Box>
                <Box sx={{ width: 46, flexShrink: 0, textAlign: "right", fontWeight: 600, lineHeight: 1, fontFamily: MONO, fontSize: 13, color }}>
                  {Math.round(pct * 100)}%
                </Box>
              </Box>
            );
          })}
        </Box>
      )}
    </Box>
  );
}
