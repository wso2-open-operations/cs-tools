import { Chip } from "@mui/material";
import { SLA_BADGE_SX, SLA_STATE_LABEL } from "@/lib/sla";
import type { SlaState } from "@/lib/api";

export function SlaBadge({ state }: { state: SlaState | null | undefined }) {
  if (!state) return <span style={{ color: "var(--sla-fg3)" }}>—</span>;
  const sx = SLA_BADGE_SX[state];
  return (
    <Chip
      label={SLA_STATE_LABEL[state]}
      variant="outlined"
      size="small"
      sx={{ fontWeight: 500, borderColor: sx.borderColor, backgroundColor: sx.backgroundColor, color: sx.color }}
    />
  );
}
