import { Box } from "@mui/material";
import type { VolumeProject } from "@/lib/api";

const MONO = "var(--font-mono)";

const PRIOS = [
  { code: "P1", color: "var(--sla-p1)" },
  { code: "P2", color: "var(--sla-p2)" },
  { code: "P3", color: "var(--sla-p3)" },
  { code: "P4", color: "var(--sla-p4)" },
] as const;

function weekLabel(iso: string): string {
  return new Date(iso + "T00:00:00Z").toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  });
}

interface VolumePanelProps {
  item: VolumeProject;
  maxWeek: number; // shared across panels for a comparable y-scale
  onFilter: () => void;
}

export function VolumePanel({ item, maxWeek, onFilter }: VolumePanelProps) {
  const max = Math.max(1, maxWeek);
  const yTicks = [max, Math.round(max / 2), 0];

  return (
    <Box sx={{ borderRadius: "12px", border: "1px solid var(--sla-border-soft)", bgcolor: "color-mix(in srgb, var(--sla-fg) 3%, transparent)", px: 2, py: 1.75 }}>
      <Box sx={{ mb: 1.5, display: "flex", alignItems: "baseline", justifyContent: "space-between" }}>
        <Box
          component="button"
          type="button"
          onClick={onFilter}
          sx={{ fontSize: 13, fontWeight: 600, transition: "color 0.15s", background: "none", border: "none", cursor: "pointer", p: 0, "&:hover": { color: "var(--sla-cs)" } }}
        >
          {item.name}
        </Box>
        <Box component="span" sx={{ fontSize: 11.5, color: "var(--sla-fg3)", fontFamily: MONO }}>
          {item.total} total
        </Box>
      </Box>

      <Box sx={{ display: "flex", gap: 1 }}>
        <Box sx={{ display: "flex", height: 96, flexDirection: "column", justifyContent: "space-between", textAlign: "right", fontSize: 9.5, color: "var(--sla-no-sla)", fontFamily: MONO }}>
          {yTicks.map((t, i) => (
            <span key={i}>{t}</span>
          ))}
        </Box>

        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Box sx={{ display: "flex", height: 96, alignItems: "flex-end", gap: 0.5, borderBottom: "1px solid var(--sla-border)" }}>
            {item.weeks.map((w, i) => (
              <Box
                key={i}
                title={`Week of ${weekLabel(w.weekStart)} · ${w.total} created`}
                sx={{ display: "flex", height: "100%", flex: 1, flexDirection: "column-reverse", overflow: "hidden", borderTopLeftRadius: "3px", borderTopRightRadius: "3px" }}
              >
                {PRIOS.map((p) => {
                  const n = w.byPriority[p.code];
                  if (n <= 0) return null;
                  return (
                    <Box
                      key={p.code}
                      title={`${p.code} · ${n} created`}
                      sx={{ height: `${(n / max) * 100}%`, bgcolor: p.color }}
                    />
                  );
                })}
              </Box>
            ))}
          </Box>

          <Box sx={{ mt: 0.75, display: "flex", gap: 0.5 }}>
            {item.weeks.map((w, i) => (
              <Box key={i} component="span" sx={{ flex: 1, textAlign: "center", fontSize: 9, lineHeight: 1.2, color: "var(--sla-no-sla)", fontFamily: MONO }}>
                {i % 3 === 0 || i === item.weeks.length - 1 ? weekLabel(w.weekStart) : ""}
              </Box>
            ))}
          </Box>
          <Box sx={{ mt: 0.5, textAlign: "center", fontSize: 10, letterSpacing: "0.05em", color: "var(--sla-no-sla)" }}>Week beginning →</Box>
        </Box>
      </Box>
    </Box>
  );
}
