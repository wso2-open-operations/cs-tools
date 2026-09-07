// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Port of v3's src/views/IssuesPage.tsx (Next's useRouter/useSearchParams ->
// react-router's useNavigate/useSearchParams).
import { useState, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { Box, MenuItem, Select, Skeleton, type SelectChangeEvent } from "@mui/material";
import { useOverview, useIssues, useIssueTitles, useTaxonomy, makeIsCsStatus } from "@api/hooks";
import type { BucketKey } from "@api/types";
import { BackButton } from "@components/BackButton";
import { IssueTimelineRow } from "@components/IssueTimelineRow";
import { gridTemplate } from "@lib/grid";
import { acrylicSurfaceSx } from "@lib/surfaces";

const KIND_CHIPS: { key: BucketKey; label: string }[] = [
  { key: "all", label: "All open" },
  { key: "violated", label: "Violated" },
  { key: "at_risk", label: "At risk" },
  { key: "cs", label: "On CS side" },
  { key: "untracked", label: "Untracked" },
];

const BUCKET_TITLES: Partial<Record<BucketKey, string>> = {
  violated: "Violated issues",
  at_risk: "At-risk issues",
  on_track: "On-track issues",
  cs: "On-CS-side issues",
  tracked: "Open tracked issues",
  untracked: "Untracked / missing priority",
  attention: "Attention set",
  all: "All open issues",
};

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
  children: React.ReactNode;
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

export default function IssuesPage() {
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const [qInput, setQInput] = useState(params.get("q") ?? "");

  const repo = params.get("repo") ?? undefined;
  const priority = params.get("priority") ?? undefined;
  const status = params.get("status") ?? undefined;
  const bucket = (params.get("bucket") ?? "all") as BucketKey;

  useEffect(() => {
    const t = setTimeout(() => {
      const next = new URLSearchParams(params);
      if (qInput) next.set("q", qInput);
      else next.delete("q");
      void navigate(`/issues?${next.toString()}`.replace(/\?$/, ""), { replace: true });
    }, 300);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [qInput]);

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    setParams(next, { replace: true });
  };

  // Switching kind (or "All open") clears any single-status refinement (e.g. WOC vs PPQ).
  const setBucket = (key: BucketKey) => {
    const next = new URLSearchParams(params);
    if (key === "all") next.delete("bucket");
    else next.set("bucket", key);
    next.delete("status");
    setParams(next, { replace: true });
  };

  const { data: overview } = useOverview(repo, priority);
  const { data: taxonomy } = useTaxonomy();
  const isCsStatus = makeIsCsStatus(taxonomy?.csStatuses);
  const { data: issues, isLoading } = useIssues({
    bucket,
    repo,
    priority,
    status,
    q: params.get("q") ?? undefined,
    order: "budget_desc",
    limit: 200,
  });

  const issueIds = (issues ?? []).map((i) => i.id);
  const { data: titles, isPending: titlesPending } = useIssueTitles(issueIds);

  const repoOptions = overview?.projects ?? [];
  const nameForRepo = (r: string | null) =>
    overview?.projects.find((p) => p.repo === r)?.name ?? r?.split("/")[1] ?? "—";
  const projName = repo ? nameForRepo(repo) : "All projects";

  // A single CS status gets its own titled list.
  const title = status ? `${status} issues` : (BUCKET_TITLES[bucket] ?? "Issues");
  const cols = gridTemplate(true);

  return (
    <Box>
      <BackButton />

      <Box sx={{ mb: 2, mt: "18px", display: "flex", flexWrap: "wrap", alignItems: "flex-end", justifyContent: "space-between", gap: "14px" }}>
        <Box>
          <Box component="h1" sx={{ m: 0, fontSize: 22, fontWeight: 600, lineHeight: 1.2, letterSpacing: "-0.01em" }}>{title}</Box>
          <Box sx={{ mt: 0.75, fontSize: 13, color: "var(--sla-fg3)" }}>
            {projName} ·{" "}
            <Box component="b" sx={{ fontWeight: 600, color: "var(--sla-fg2)", fontFamily: "var(--font-mono)" }}>
              {issues?.length ?? 0}
            </Box>{" "}
            matching open issues · click any row to open it on GitHub
          </Box>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: "10px" }}>
          <Box
            component="input"
            placeholder="Search by issue #…"
            value={qInput}
            onChange={(e) => setQInput((e.target as HTMLInputElement).value.replace(/\D/g, ""))}
            sx={{
              height: 36, width: 192, borderRadius: "9px", border: "1px solid var(--sla-border)", bgcolor: "var(--sla-card)",
              px: 1.5, fontSize: 13, color: "var(--sla-fg)", fontFamily: "inherit", "&:focus": { outline: "none", borderColor: "var(--sla-fg3)" },
            }}
          />
          <FilterSelect value={repo ?? "all"} onChange={(v) => setParam("repo", v === "all" ? "" : v)}>
            <MenuItem value="all">All projects</MenuItem>
            {repoOptions.map((r) => (
              <MenuItem key={r.repoId} value={r.repo}>
                {r.name}
              </MenuItem>
            ))}
          </FilterSelect>
          <FilterSelect value={priority ?? "all"} onChange={(v) => setParam("priority", v === "all" ? "" : v)}>
            <MenuItem value="all">All priorities</MenuItem>
            {PRIORITY_OPTIONS.map((p) => (
              <MenuItem key={p.value} value={p.value}>
                {p.label}
              </MenuItem>
            ))}
          </FilterSelect>
        </Box>
      </Box>

      {/* Kind chips */}
      <Box sx={{ mb: 2, display: "flex", flexWrap: "wrap", gap: 1 }}>
        {KIND_CHIPS.map((chip) => {
          const active = bucket === chip.key || (chip.key === "all" && bucket === "all");
          return (
            <Box
              key={chip.key}
              component="button"
              type="button"
              onClick={() => setBucket(chip.key)}
              sx={{
                borderRadius: "8px", border: "1px solid", px: 1.5, py: 0.75, fontSize: 12.5, fontWeight: 600, cursor: "pointer",
                borderColor: active ? "var(--sla-primary)" : "var(--sla-border)",
                bgcolor: active ? "var(--sla-primary)" : "var(--sla-card)",
                color: active ? "var(--sla-contrast-text)" : "var(--sla-fg2)",
              }}
            >
              {chip.label}
            </Box>
          );
        })}
      </Box>

      {/* Table */}
      <Box sx={{ ...acrylicSurfaceSx, overflow: "hidden", borderRadius: "16px", border: "1px solid var(--sla-border)", boxShadow: "0 1px 2px rgba(17,24,39,.04)" }}>
        <Box
          sx={{
            display: "grid", borderBottom: "1px solid var(--sla-border-soft)", bgcolor: "var(--sla-surface-muted)", px: "22px", py: 1.25,
            fontSize: 11, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--sla-fg3)",
            gridTemplateColumns: cols,
          }}
        >
          <span>Issue</span>
          <span>Project</span>
          <span>Pri</span>
          <span>Status</span>
          <span>SLA state</span>
          <span>Budget</span>
          <Box component="span" sx={{ textAlign: "right" }}>Age</Box>
        </Box>

        {isLoading ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1, p: 2 }}>
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} variant="rounded" sx={{ height: 36, width: "100%" }} />
            ))}
          </Box>
        ) : (issues?.length ?? 0) === 0 ? (
          <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 1.25, px: 2.5, py: 6 }}>
            <Box component="span" sx={{ display: "flex", height: 40, width: 40, alignItems: "center", justifyContent: "center", borderRadius: "50%", bgcolor: "var(--sla-ok-tint)", fontSize: 20, fontWeight: 700, color: "var(--sla-ok)" }}>✓</Box>
            <Box component="span" sx={{ fontSize: 14, fontWeight: 600, color: "var(--sla-ok)" }}>No matching issues</Box>
            <Box component="span" sx={{ fontSize: 12.5, color: "var(--sla-fg3)" }}>Nothing matches this filter combination right now.</Box>
          </Box>
        ) : (
          issues!.map((issue) => (
            <IssueTimelineRow
              key={issue.id}
              issue={issue}
              title={titles?.[issue.id] ?? null}
              titleLoading={titlesPending}
              showSlaState
              projectName={nameForRepo(issue.repo)}
              isCsStatus={isCsStatus}
            />
          ))
        )}
      </Box>

      <Box sx={{ mt: 3, textAlign: "center", fontSize: 11.5, color: "var(--sla-no-sla)" }}>
        Read-only · issues open in GitHub in a new tab · Closed &amp; Terminal issues excluded
      </Box>
    </Box>
  );
}
