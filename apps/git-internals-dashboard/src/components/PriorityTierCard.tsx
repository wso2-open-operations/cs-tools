import { Box } from "@mui/material";
import type { OverviewPriority } from "@/lib/api";
import { fmtBudget } from "@/lib/sla";
import { acrylicSurfaceSx } from "@/lib/surfaces";

const MONO = "var(--font-mono)";

const P_ACCENT: Record<string, string> = {
  P1: "var(--sla-p1)",
  P2: "var(--sla-p2)",
  P3: "var(--sla-p3)",
  P4: "var(--sla-p4)",
};
const P_TINT: Record<string, string> = {
  P1: "color-mix(in srgb, var(--sla-p1) 14%, transparent)",
  P2: "color-mix(in srgb, var(--sla-p2) 14%, transparent)",
  P3: "color-mix(in srgb, var(--sla-p3) 14%, transparent)",
  P4: "color-mix(in srgb, var(--sla-p4) 14%, transparent)",
};

interface PriorityTierCardProps {
  tier: OverviewPriority;
  focused: boolean;
  dim: boolean;
  onFocusToggle: () => void;
  onDrill: (bucket: string) => void;
}

export function PriorityTierCard({ tier, focused, dim, onFocusToggle, onDrill }: PriorityTierCardProps) {
  const accent = P_ACCENT[tier.code] ?? "var(--sla-fg)";
  const tint = P_TINT[tier.code] ?? "color-mix(in srgb, var(--sla-fg) 8%, transparent)";
  const denom = Math.max(1, tier.total);
  const pct = (n: number) => (n / denom) * 100;

  const drill = (bucket: string) => (e: React.MouseEvent) => {
    e.stopPropagation();
    onDrill(bucket);
  };

  return (
    <Box
      onClick={onFocusToggle}
      sx={{
        ...acrylicSurfaceSx,
        cursor: "pointer", borderRadius: "16px", px: "17px", pb: "15px", pt: 2,
        transition: "opacity 0.15s", border: `1px solid ${focused ? accent : "var(--sla-border)"}`,
        borderTopWidth: 4, borderTopColor: accent, borderTopStyle: "solid",
        boxShadow: focused ? `0 1px 2px rgba(17,24,39,.06), 0 0 0 1px ${accent}` : "0 1px 2px rgba(17,24,39,.04)",
        opacity: dim ? 0.5 : 1,
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: "10px" }}>
          {/* White text: deliberate exception, same reasoning as
              PriorityStateMatrix's row badge — 4 rotating accent hues, no
              single contrastText token applies. */}
          <Box component="span" sx={{ borderRadius: "7px", px: 1, py: "5px", fontSize: 13, fontWeight: 700, lineHeight: 1, color: "#fff", fontFamily: MONO, bgcolor: accent }}>
            {tier.code}
          </Box>
          <Box sx={{ lineHeight: 1.15 }}>
            <Box sx={{ fontSize: 13.5, fontWeight: 600 }}>{tier.label}</Box>
            <Box sx={{ mt: 0.25, fontSize: 11, lineHeight: 1, color: "var(--sla-no-sla)", fontFamily: MONO }}>
              SLA {fmtBudget(tier.budgetHours)}
            </Box>
          </Box>
        </Box>
        {focused && (
          <Box component="span" sx={{ borderRadius: "5px", px: "7px", py: "3px", fontSize: 9, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", bgcolor: tint, color: accent }}>
            Focused
          </Box>
        )}
      </Box>

      <Box sx={{ mt: "15px", display: "flex", alignItems: "flex-end", gap: "14px" }}>
        <Box>
          <Box
            component="button"
            type="button"
            onClick={drill("violated")}
            sx={{
              display: "block", lineHeight: 0.85, letterSpacing: "-0.02em", transition: "opacity 0.15s",
              fontFamily: MONO, fontSize: 44, fontWeight: 600, color: tier.violated > 0 ? "var(--sla-violated)" : "var(--sla-ok)",
              background: "none", border: "none", cursor: "pointer", p: 0, "&:hover": { opacity: 0.6 },
            }}
          >
            {tier.violated}
          </Box>
          <Box sx={{ mt: 0.75, fontSize: 11, fontWeight: 500, color: "var(--sla-fg3)" }}>Violated</Box>
        </Box>
        <Box sx={{ height: 42, width: "1px", bgcolor: "var(--sla-border-soft)" }} />
        <Box>
          <Box
            component="button"
            type="button"
            onClick={drill("at_risk")}
            sx={{
              display: "block", lineHeight: 0.9, letterSpacing: "-0.02em", transition: "opacity 0.15s",
              fontFamily: MONO, fontSize: 26, fontWeight: 600, color: tier.atRisk > 0 ? "var(--sla-at-risk)" : "var(--sla-no-sla)",
              background: "none", border: "none", cursor: "pointer", p: 0, "&:hover": { opacity: 0.6 },
            }}
          >
            {tier.atRisk}
          </Box>
          <Box sx={{ mt: 0.5, fontSize: 11, fontWeight: 500, color: "var(--sla-fg3)" }}>At risk</Box>
        </Box>
        {tier.violated === 0 && (
          <Box component="span" sx={{ ml: "auto", display: "inline-flex", alignItems: "center", gap: 0.5, fontSize: 11.5, fontWeight: 600, color: "var(--sla-ok)" }}>
            ✓
          </Box>
        )}
      </Box>

      <Box sx={{ mt: "14px", display: "flex", height: 7, overflow: "hidden", borderRadius: "6px", bgcolor: "var(--sla-surface-track)" }}>
        <Box sx={{ width: `${pct(tier.violated)}%`, bgcolor: "var(--sla-violated)" }} />
        <Box sx={{ width: `${pct(tier.atRisk)}%`, bgcolor: "var(--sla-at-risk)" }} />
        <Box sx={{ width: `${pct(tier.cs)}%`, bgcolor: "var(--sla-cs)" }} />
        <Box sx={{ width: `${pct(tier.onTrack)}%`, bgcolor: "var(--sla-ok)" }} />
      </Box>

      <Box sx={{ mt: 1, display: "flex", justifyContent: "space-between", fontSize: 11, color: "var(--sla-fg3)" }}>
        <span>{tier.total} open tracked</span>
        <span>{tier.cs} on CS side</span>
      </Box>
    </Box>
  );
}
