import { Box } from "@mui/material";
import type { OverviewProject } from "@/lib/api";
import { acrylicSurfaceSx } from "@/lib/surfaces";

const MONO = "var(--font-mono)";

interface ProjectCardProps {
  project: OverviewProject;
  focused: boolean;
  dim: boolean;
  onFocus: () => void;
  onDrill: (bucket: string) => void;
}

interface StatCellProps {
  label: string;
  value: number;
  color: string;
  hoverBorder: string;
  onClick: (e: React.MouseEvent) => void;
}

function StatCell({ label, value, color, hoverBorder, onClick }: StatCellProps) {
  return (
    <Box
      component="button"
      type="button"
      onClick={onClick}
      sx={{
        borderRadius: "9px", border: "1px solid var(--sla-border-soft)",
        bgcolor: "color-mix(in srgb, var(--sla-fg) 4%, transparent)",
        px: 1, py: 1.25, textAlign: "center", transition: "background-color 0.15s",
        cursor: "pointer",
        "&:hover": { bgcolor: "color-mix(in srgb, var(--sla-fg) 8%, transparent)", borderColor: hoverBorder },
      }}
    >
      <Box sx={{ lineHeight: 1, fontFamily: MONO, fontSize: 24, fontWeight: 600, color }}>{value}</Box>
      <Box sx={{ mt: 0.75, fontSize: 10.5, fontWeight: 500, color: "var(--sla-fg3)" }}>{label}</Box>
    </Box>
  );
}

export function ProjectCard({ project, focused, dim, onFocus, onDrill }: ProjectCardProps) {
  const drill = (bucket: string) => (e: React.MouseEvent) => {
    e.stopPropagation();
    onDrill(bucket);
  };

  const borderColor = focused
    ? "var(--sla-primary)"
    : project.worst
      ? "color-mix(in srgb, var(--sla-violated) 35%, transparent)"
      : "var(--sla-border)";
  const boxShadow = focused
    ? "0 1px 2px rgba(17,24,39,.06), 0 0 0 1px var(--sla-primary)"
    : project.worst
      ? "0 1px 2px rgba(220,43,43,.10), 0 0 0 1px color-mix(in srgb, var(--sla-violated) 35%, transparent)"
      : "0 1px 2px rgba(17,24,39,.04)";

  return (
    <Box
      onClick={onFocus}
      title="Focus this project"
      sx={{
        ...acrylicSurfaceSx,
        cursor: "pointer", borderRadius: "14px", px: 2, pb: 0.25, pt: 2,
        transition: "opacity 0.15s", border: `1px solid ${borderColor}`, boxShadow, opacity: dim ? 0.5 : 1,
      }}
    >
      <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: "10px" }}>
        <Box sx={{ minWidth: 0 }}>
          <Box sx={{ fontSize: 14.5, fontWeight: 600, letterSpacing: "-0.01em" }}>{project.name}</Box>
          <Box sx={{ mt: 0.25, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 11, fontWeight: 500, color: "var(--sla-no-sla)", fontFamily: MONO }}>
            {project.repo}
          </Box>
        </Box>
        {focused ? (
          <Box component="span" sx={{ flexShrink: 0, borderRadius: "6px", bgcolor: "color-mix(in srgb, var(--sla-primary) 14%, transparent)", px: 1, py: 0.5, fontSize: 10, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--sla-primary)" }}>
            Focused
          </Box>
        ) : project.worst ? (
          <Box component="span" sx={{ flexShrink: 0, borderRadius: "6px", bgcolor: "var(--sla-violated-tint)", px: 1, py: 0.5, fontSize: 10, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--sla-violated)" }}>
            Worst
          </Box>
        ) : project.allClear ? (
          <Box component="span" sx={{ flexShrink: 0, borderRadius: "6px", bgcolor: "var(--sla-ok-tint)", px: 1, py: 0.5, fontSize: 10, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--sla-ok)" }}>
            Clear
          </Box>
        ) : null}
      </Box>

      <Box sx={{ mt: 2, display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 1 }}>
        <StatCell label="Violated" value={project.violated} color="var(--sla-violated)" hoverBorder="var(--sla-violated)" onClick={drill("violated")} />
        <StatCell label="At risk" value={project.atRisk} color="var(--sla-at-risk)" hoverBorder="var(--sla-at-risk)" onClick={drill("at_risk")} />
        <StatCell label="On CS side" value={project.cs} color="var(--sla-cs)" hoverBorder="var(--sla-cs)" onClick={drill("cs")} />
      </Box>

      <Box sx={{ mt: 1.5, display: "flex", alignItems: "center", justifyContent: "space-between", borderTop: "1px solid var(--sla-border-soft)", px: 0.125, py: 1.5 }}>
        <Box
          component="button"
          type="button"
          onClick={drill("tracked")}
          sx={{ fontSize: 12, color: "var(--sla-fg2)", transition: "color 0.15s", background: "none", border: "none", cursor: "pointer", p: 0, "&:hover": { color: "var(--sla-fg)" } }}
        >
          Open tracked{" "}
          <Box component="b" sx={{ fontWeight: 600, color: "var(--sla-fg)", fontFamily: MONO }}>{project.openTracked}</Box>
        </Box>
        <Box
          component="button"
          type="button"
          onClick={drill("untracked")}
          title="Issues with no priority label — not SLA-tracked"
          sx={{ display: "inline-flex", alignItems: "center", gap: 0.75, fontSize: 12, color: "var(--sla-fg3)", transition: "color 0.15s", background: "none", border: "none", cursor: "pointer", p: 0, "&:hover": { color: "var(--sla-fg2)" } }}
        >
          <Box component="span" sx={{ height: 7, width: 7, borderRadius: "50%", bgcolor: "var(--sla-no-sla)" }} />
          Untracked{" "}
          <Box component="b" sx={{ fontWeight: 600, color: "var(--sla-fg2)", fontFamily: MONO }}>{project.untracked}</Box>
        </Box>
      </Box>
    </Box>
  );
}
