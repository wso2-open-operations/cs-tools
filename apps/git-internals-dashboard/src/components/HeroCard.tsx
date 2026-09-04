import { Box } from "@mui/material";
import { SparkLine } from "@/components/SparkLine";
import { acrylicSurfaceSx } from "@/lib/surfaces";

const MONO = "var(--font-mono)";

interface HeroCardProps {
  label: string; // "Violated" | "At risk"
  n: number;
  delta: number; // today − yesterday
  spark: number[];
  accent: string; // CSS color when non-zero
  onClick?: () => void;
}

// Delta semantics: more violations/at-risk than yesterday is bad (red ▲); fewer is good (green ▼).
function Delta({ delta }: { delta: number }) {
  if (delta === 0) {
    return (
      <Box component="span" sx={{ fontWeight: 600, color: "var(--sla-fg3)" }}>
        • 0
      </Box>
    );
  }
  const up = delta > 0;
  return (
    <Box component="span" sx={{ fontWeight: 600, color: up ? "var(--sla-violated)" : "var(--sla-ok)" }}>
      {up ? "▲" : "▼"} {up ? "+" : ""}
      {delta}
    </Box>
  );
}

export function HeroCard({ label, n, delta, spark, accent, onClick }: HeroCardProps) {
  const isZero = n === 0;
  const numColor = isZero ? "var(--sla-ok)" : accent;
  const topBorder = isZero ? "var(--sla-ok)" : accent;

  return (
    <Box
      sx={{
        ...acrylicSurfaceSx,
        borderRadius: "16px", border: "1px solid var(--sla-border)",
        p: "18px", boxShadow: "0 1px 2px rgba(17,24,39,.04)", borderTopWidth: 3, borderTopColor: topBorder, borderTopStyle: "solid",
      }}
    >
      <Box component="span" sx={{ whiteSpace: "nowrap", fontSize: 13, fontWeight: 600, color: "var(--sla-fg2)" }}>
        {label}
      </Box>

      <Box sx={{ mt: 1, display: "flex", alignItems: "flex-end", justifyContent: "space-between" }}>
        <Box sx={{ display: "flex", alignItems: "baseline", gap: "10px" }}>
          <Box
            component={onClick ? "button" : "span"}
            type={onClick ? "button" : undefined}
            onClick={onClick}
            title={onClick ? `View ${label.toLowerCase()} issues` : undefined}
            sx={{
              lineHeight: 0.9, letterSpacing: "-0.02em", transition: "opacity 0.15s", fontFamily: MONO,
              fontSize: 52, fontWeight: 600, color: numColor, background: "none", border: "none",
              p: 0,
              ...(onClick && { cursor: "pointer", "&:hover": { opacity: 0.6 } }),
            }}
          >
            {n}
          </Box>
          {isZero && (
            <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: 0.5, fontSize: 12, fontWeight: 600, color: "var(--sla-ok)" }}>
              ✓ at target
            </Box>
          )}
        </Box>
        <SparkLine points={spark} color={numColor} />
      </Box>

      <Box sx={{ mt: 1, display: "flex", alignItems: "center", gap: 0.75, fontSize: 12, color: "var(--sla-fg3)" }}>
        <Delta delta={delta} />
        <span>vs. yesterday</span>
      </Box>
    </Box>
  );
}

interface CsHeroCardProps {
  n: number;
  byStatus: Array<{ status: string; n: number }>;
  onDrill?: (status: string) => void;
}

// Alternates between the two CS accent colors by index; 2 entries reproduces
// the original WOC/PPQ two-tile layout pixel-for-pixel.
const CS_COLORS = ["var(--sla-cs)", "var(--sla-cs-lite)"];

export function CsHeroCard({ n, byStatus, onDrill }: CsHeroCardProps) {
  const isZero = n === 0;
  const accent = isZero ? "var(--sla-ok)" : "var(--sla-cs)";
  const chipTint = isZero ? "var(--sla-ok-tint)" : "var(--sla-cs-tint)";

  return (
    <Box
      sx={{
        ...acrylicSurfaceSx,
        height: "100%", borderRadius: "16px", border: "1px solid var(--sla-border)",
        p: "18px", boxShadow: "0 1px 2px rgba(17,24,39,.04)", borderTopWidth: 3, borderTopColor: accent, borderTopStyle: "solid",
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <Box component="span" sx={{ whiteSpace: "nowrap", fontSize: 13, fontWeight: 600, color: "var(--sla-fg2)" }}>
          On CS side
        </Box>
        <Box
          component="span"
          sx={{
            borderRadius: "6px", px: "7px", py: "3px", fontSize: 10, fontWeight: 600,
            textTransform: "uppercase", letterSpacing: "0.05em", fontFamily: MONO, bgcolor: chipTint, color: accent,
          }}
        >
          POINT-IN-TIME
        </Box>
      </Box>

      <Box sx={{ mt: 2, display: "flex", alignItems: "flex-start", gap: "14px" }}>
        {byStatus.map(({ status, n: count }, i) => {
          const color = CS_COLORS[i % CS_COLORS.length];
          return (
            <Box key={status} sx={{ display: "flex", flex: 1, alignItems: "flex-start", gap: "14px" }}>
              {i > 0 && <Box sx={{ alignSelf: "stretch", width: "1px", bgcolor: "var(--sla-border-soft)" }} />}
              <Box
                component="button"
                type="button"
                onClick={() => onDrill?.(status)}
                title={`View ${status} issues`}
                sx={{ flex: 1, textAlign: "left", transition: "opacity 0.15s", background: "none", border: "none", cursor: "pointer", p: 0, "&:hover": { opacity: 0.6 } }}
              >
                <Box sx={{ lineHeight: 0.85, letterSpacing: "-0.02em", fontFamily: MONO, fontSize: 44, fontWeight: 600, color }}>
                  {count}
                </Box>
                <Box sx={{ mt: 1, display: "flex", alignItems: "center", gap: 0.75, fontSize: 12, color: "var(--sla-fg2)" }}>
                  <Box component="span" sx={{ height: 10, width: 10, borderRadius: "2px", bgcolor: color }} />
                  {status}
                </Box>
              </Box>
            </Box>
          );
        })}

        {isZero && (
          <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: 0.5, fontSize: 12, fontWeight: 600, color: "var(--sla-ok)" }}>
            ✓
          </Box>
        )}
      </Box>
    </Box>
  );
}
