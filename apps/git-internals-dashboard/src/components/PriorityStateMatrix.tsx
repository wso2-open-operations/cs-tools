import { Box } from "@mui/material";
import type { Overview } from "@/lib/api";
import { acrylicSurfaceSx } from "@/lib/surfaces";

const MONO = "var(--font-mono)";

// CSS-var references (theme-following, like every other accent in the app) —
// safe here because these only ever feed a plain `bgcolor`, never rgba() math.
const P_ACCENT: Record<string, string> = {
  P1: "var(--sla-p1)",
  P2: "var(--sla-p2)",
  P3: "var(--sla-p3)",
  P4: "var(--sla-p4)",
};

// column key → { label, drill bucket, hex color }. Literal hex (not a CSS
// var) is required here: `rgba()` below parses it for the heat-map alpha
// blend, which `var(--sla-*)` strings can't do. Kept in sync with the
// AcrylicPurpleTheme values in globals.css (error/warning/success/primary).
const COLS = [
  { key: "violated", label: "Violated", bucket: "violated", hex: "#d32f2f" },
  { key: "atRisk", label: "At risk", bucket: "at_risk", hex: "#ed6c02" },
  { key: "onTrack", label: "On track", bucket: "on_track", hex: "#2e7d32" },
  { key: "cs", label: "CS side", bucket: "cs", hex: "#646cff" },
] as const;

type ColKey = (typeof COLS)[number]["key"];

function rgba(hex: string, a: number): string {
  const n = parseInt(hex.slice(1), 16);
  return `rgba(${(n >> 16) & 255},${(n >> 8) & 255},${n & 255},${a})`;
}

interface MatrixProps {
  matrix: Overview["matrix"];
  onDrill: (bucket: string, priority?: string) => void;
}

export function PriorityStateMatrix({ matrix, onDrill }: MatrixProps) {
  // Per-column max across the priority rows drives the heat-map intensity.
  const colMax: Record<ColKey, number> = { violated: 1, atRisk: 1, onTrack: 1, cs: 1 };
  for (const c of COLS) {
    colMax[c.key] = Math.max(1, ...matrix.rows.map((r) => r.cells[c.key]));
  }

  const gridCols = "64px repeat(4, 1fr) 58px";

  return (
    <Box sx={{ ...acrylicSurfaceSx, borderRadius: "16px", border: "1px solid var(--sla-border)", px: "22px", py: 2.5, boxShadow: "0 1px 2px rgba(17,24,39,.04)" }}>
      <Box component="h2" sx={{ m: 0, fontSize: 15, fontWeight: 600, lineHeight: 1.2, letterSpacing: "-0.01em" }}>
        Priority × SLA state
      </Box>
      <Box sx={{ mb: 2, mt: 0.25, fontSize: 12.5, color: "var(--sla-fg3)" }}>
        Where the load sits. Darker cell = more issues.
      </Box>

      <Box sx={{ display: "grid", alignItems: "center", gap: "6px", gridTemplateColumns: gridCols }}>
        {/* header row */}
        <span />
        {COLS.map((c) => (
          <Box key={c.key} sx={{ textAlign: "center", fontSize: 10.5, fontWeight: 600, lineHeight: 1.2, color: "var(--sla-fg3)" }}>
            {c.label}
          </Box>
        ))}
        <Box sx={{ textAlign: "center", fontSize: 10.5, fontWeight: 600, lineHeight: 1.2, color: "var(--sla-fg2)" }}>Total</Box>

        {/* priority rows */}
        {matrix.rows.map((row) => (
          <Row key={row.key} row={row} colMax={colMax} onDrill={onDrill} />
        ))}

        {/* All row */}
        <Box sx={{ pt: 0.5, textAlign: "center", fontSize: 10.5, fontWeight: 600, lineHeight: 1, color: "var(--sla-fg3)" }}>All</Box>
        {COLS.map((c) => {
          const n = matrix.totals[c.key];
          return (
            <Box
              key={c.key}
              component="button"
              type="button"
              onClick={() => onDrill(c.bucket)}
              sx={{
                pt: 0.5, textAlign: "center", lineHeight: 1, transition: "opacity 0.15s", fontFamily: MONO,
                fontSize: 14, fontWeight: 600, color: n > 0 ? c.hex : "var(--sla-fg3)", background: "none", border: "none",
                cursor: "pointer", "&:hover": { opacity: 0.6 },
              }}
            >
              {n}
            </Box>
          );
        })}
        <Box
          component="button"
          type="button"
          onClick={() => onDrill("tracked")}
          sx={{ pt: 0.5, textAlign: "center", lineHeight: 1, color: "var(--sla-fg)", transition: "opacity 0.15s", fontFamily: MONO, fontSize: 14, fontWeight: 600, background: "none", border: "none", cursor: "pointer", "&:hover": { opacity: 0.6 } }}
        >
          {matrix.grandTotal}
        </Box>
      </Box>
    </Box>
  );
}

function Row({
  row,
  colMax,
  onDrill,
}: {
  row: Overview["matrix"]["rows"][number];
  colMax: Record<ColKey, number>;
  onDrill: (bucket: string, priority?: string) => void;
}) {
  const accent = P_ACCENT[row.code] ?? "var(--sla-no-sla)";
  return (
    <>
      {/* White text is a deliberate, documented exception: this badge's
          background rotates across 4 distinct accent hues (error/warning/
          info/grey per priority), and no single Oxygen UI contrastText token
          maps to "whichever of 4 hues is active" — the same approach MUI's
          own multi-hue Chip/Avatar examples take. Pre-existing, unchanged by
          this pass: contrast is weakest on the P4/grey tile, a known
          tradeoff of the shared-white-text approach, not a regression here.
          See --sla-contrast-text for the single-hue case (used elsewhere)
          where a real token does apply. */}
      <Box component="span" sx={{ borderRadius: "6px", py: 0.75, textAlign: "center", fontSize: 12, fontWeight: 700, lineHeight: 1, color: "#fff", fontFamily: MONO, bgcolor: accent }}>
        {row.code}
      </Box>
      {COLS.map((c) => {
        const n = row.cells[c.key];
        const a = n > 0 ? 0.1 + 0.55 * (n / colMax[c.key]) : 0;
        return (
          <Box
            key={c.key}
            component="button"
            type="button"
            onClick={() => onDrill(c.bucket, row.key)}
            sx={{
              borderRadius: "8px", py: "11px", textAlign: "center", lineHeight: 1, transition: "opacity 0.15s",
              fontFamily: MONO, fontSize: 15, fontWeight: 600,
              background: n > 0 ? rgba(c.hex, a) : "var(--sla-surface-muted)", color: n > 0 ? c.hex : "var(--sla-fg3)",
              border: "none", cursor: "pointer", "&:hover": { opacity: 0.7 },
            }}
          >
            {n}
          </Box>
        );
      })}
      <Box
        component="button"
        type="button"
        onClick={() => onDrill("tracked", row.key)}
        sx={{ textAlign: "center", lineHeight: 1, color: "var(--sla-fg)", transition: "opacity 0.15s", fontFamily: MONO, fontSize: 15, fontWeight: 600, background: "none", border: "none", cursor: "pointer", "&:hover": { opacity: 0.6 } }}
      >
        {row.total}
      </Box>
    </>
  );
}
