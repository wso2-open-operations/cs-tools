"use client";

import { useRouter, useSearchParams } from "next/navigation";
import type { ReactNode } from "react";
import { Box, MenuItem, Select, type SelectChangeEvent } from "@mui/material";
import { useOverview } from "@/lib/api";
import { SyncButton } from "@/components/SyncButton";

const PRIORITY_OPTIONS = [
  { value: "Critical(P1)", label: "Critical · P1" },
  { value: "High(P2)", label: "High · P2" },
  { value: "Medium(P3)", label: "Medium · P3" },
  { value: "Low(P4)", label: "Low · P4" },
];

// A native <select>'s options popup is rendered by the OS/browser, not the
// page — no CSS/theme can reach it. MUI's Select renders its popup via
// Popover/MenuItem, which Oxygen UI's AcrylicBaseTheme already themes
// (background.paper + blur.medium, see MuiPopover/MuiMenuItem overrides in
// node_modules/@wso2/oxygen-ui) — no custom sx needed for the popup itself.
function FilterSelect({
  value,
  onChange,
  children,
}: {
  value: string;
  onChange: (v: string) => void;
  children: ReactNode;
}) {
  return (
    <Select
      value={value}
      onChange={(e: SelectChangeEvent) => onChange(e.target.value)}
      size="small"
      sx={{ minWidth: 150 }}
    >
      {children}
    </Select>
  );
}

export function AppShell({ children }: { children: ReactNode }) {
  const router = useRouter();
  const params = useSearchParams() ?? new URLSearchParams();
  const repo = params.get("repo") ?? undefined;
  const priority = params.get("priority") ?? undefined;

  const { data: overview } = useOverview(repo, priority);

  const setFilter = (key: "repo" | "priority", v: string) => {
    const next = new URLSearchParams(params);
    if (v) next.set(key, v);
    else next.delete(key);
    router.replace(`/?${next.toString()}`.replace(/\?$/, ""));
  };

  const repoOptions = overview?.projects ?? [];

  return (
    // No bgcolor here — Oxygen UI's MuiCssBaseline override paints the
    // Acrylic radial-gradient body backdrop (see globals.css); an opaque
    // background on this wrapper would hide it completely.
    <Box sx={{ minHeight: "100vh", color: "var(--sla-fg)" }}>
      <Box
        component="header"
        sx={{
          position: "sticky",
          top: 0,
          zIndex: 20,
          borderBottom: "1px solid var(--sla-border)",
          bgcolor: "var(--sla-card)",
          backdropFilter: "var(--sla-blur) saturate(150%)",
          WebkitBackdropFilter: "var(--sla-blur) saturate(150%)",
        }}
      >
        <Box
          sx={{
            mx: "auto",
            maxWidth: 1480,
            display: "flex",
            flexWrap: "wrap",
            alignItems: "center",
            gap: "20px",
            px: "28px",
            py: "14px",
          }}
        >
          <Box sx={{ mr: "auto", display: "flex", alignItems: "center", gap: 1.5 }}>
            <Box
              sx={{
                display: "flex", height: 34, width: 34, alignItems: "center", justifyContent: "center",
                borderRadius: "9px", background: "var(--sla-primary-gradient)", color: "var(--sla-contrast-text)", fontSize: 15, fontWeight: 700,
              }}
            >
              S
            </Box>
            <Box>
              <Box sx={{ fontSize: 16, fontWeight: 600, letterSpacing: "-0.01em" }}>SLA Monitor</Box>
              <Box sx={{ fontSize: 12, color: "var(--sla-fg3)" }}>
                CS-originated GitHub issues · product-team SLA compliance
              </Box>
            </Box>
          </Box>

          <Box sx={{ display: "flex", alignItems: "center", gap: "10px" }}>
            <FilterSelect value={repo ?? "all"} onChange={(v) => setFilter("repo", v === "all" ? "" : v)}>
              <MenuItem value="all">All projects</MenuItem>
              {repoOptions.map((r) => (
                <MenuItem key={r.repoId} value={r.repo}>
                  {r.name}
                </MenuItem>
              ))}
            </FilterSelect>

            <FilterSelect value={priority ?? "all"} onChange={(v) => setFilter("priority", v === "all" ? "" : v)}>
              <MenuItem value="all">All priorities</MenuItem>
              {PRIORITY_OPTIONS.map((p) => (
                <MenuItem key={p.value} value={p.value}>
                  {p.label}
                </MenuItem>
              ))}
            </FilterSelect>
          </Box>

          <Box sx={{ display: "flex", alignItems: "center", pl: 1 }}>
            <SyncButton />
          </Box>
        </Box>
      </Box>

      <Box component="main" sx={{ mx: "auto", maxWidth: 1480, px: "28px", pb: "60px", pt: 3 }}>
        {children}
      </Box>
    </Box>
  );
}
