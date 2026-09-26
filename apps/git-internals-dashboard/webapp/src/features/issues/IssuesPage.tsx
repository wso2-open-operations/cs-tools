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

import { useSearchParams } from "react-router";
import { Box, Button, Chip, Skeleton, TablePagination, TableSortLabel } from "@mui/material";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useOverview, useIssues, useTaxonomy, makeIsCsStatus } from "@api/hooks";
import type { BucketKey, SlaState } from "@api/types";
import { DEFAULT_ISSUE_SORT, parseIssueSortField, parseIssueSortOrder, type IssueSortField, type IssueSortOrder } from "@api/issueSort";
import { BackButton } from "@components/BackButton";
import { ErrorState } from "@components/ErrorState";
import { StaleDataAlert } from "@components/StaleDataAlert";
import { MultiSelectFilter } from "@components/MultiSelectFilter";
import { errorMessage } from "@lib/apiError";
import { useIssueListFilters, projectNameFor, priorityOptionsFrom, NO_PRIORITY_VALUE } from "@lib/filters";
import { useReportFetchProgress } from "@lib/fetchProgress";
import { IssueTimelineRow } from "@components/IssueTimelineRow";
import { gridTemplate } from "@lib/grid";
import { SLA_STATE_LABEL } from "@lib/sla";
import { acrylicSurfaceSx } from "@lib/surfaces";

const ROWS_PER_PAGE_OPTIONS = [10, 20, 50, 100];
const DEFAULT_ROWS_PER_PAGE = 10;

// SLA state dropdown options, in a fixed worst-to-best order. TERMINAL is
// excluded: this page lists open, non-terminal issues only.
const SLA_STATE_OPTIONS: { value: SlaState; label: string }[] = (["VIOLATED", "AT_RISK", "OK", "NO_SLA"] as const).map((s) => ({
  value: s,
  label: SLA_STATE_LABEL[s],
}));

const BUCKET_TITLES: Partial<Record<BucketKey, string>> = {
  violated: "Violated issues",
  at_risk: "At-risk issues",
  on_track: "On-track issues",
  cs: "On-CS-side issues",
  product_side: "On-product-side issues",
  tracked: "Open tracked issues",
  untracked: "Untracked / missing priority",
  attention: "Attention set",
};

// A bucket whose meaning can't be expressed as ticked filter-dropdown
// options (e.g. "on track" is SLA OK *and* not on the CS side) still reaches
// the backend as `bucket`, shown here as a removable scope chip instead.
const SCOPE_CHIP_LABEL: Partial<Record<BucketKey, string>> = {
  on_track: "On track (excluding CS side)",
  tracked: "Has a priority",
  attention: "Needs attention",
  cs: "On CS side",
  product_side: "On product team side",
  violated: "Violated",
  at_risk: "At risk",
  untracked: "No priority",
};

const FILTER_KEYS = ["repo", "priority", "abtTeam", "status", "slaState"] as const;

/** The filtered/drill-down issue list page, URL-driven by the five filter dropdowns, bucket, sort/order, and q. */
export default function IssuesPage() {
  const [params, setParams] = useSearchParams();

  const { repo, priority, abtTeam, status, slaState, setFilter } = useIssueListFilters();
  const bucket = params.get("bucket") as BucketKey | null;
  const sort = parseIssueSortField(params.get("sort"));
  const order = parseIssueSortOrder(params.get("order"));
  const rowsPerPage = Number(params.get("pageSize")) || DEFAULT_ROWS_PER_PAGE;
  const page = Number(params.get("page")) || 0;

  // TablePagination's page is 0-indexed; the URL stores it 1-indexed-minus-1
  // implicitly (0 = unset = page 1) so a bare /issues URL has no ?page=0 noise.
  const setPage = (newPage: number) => {
    const next = new URLSearchParams(params);
    if (newPage > 0) next.set("page", String(newPage));
    else next.delete("page");
    setParams(next, { replace: true });
  };

  const setRowsPerPage = (newRowsPerPage: number) => {
    const next = new URLSearchParams(params);
    if (newRowsPerPage !== DEFAULT_ROWS_PER_PAGE) next.set("pageSize", String(newRowsPerPage));
    else next.delete("pageSize");
    next.delete("page");
    setParams(next, { replace: true });
  };

  // Omits `sort`/`order` from the URL when they're the default, so a
  // never-touched sort stays invisible in a shared/bookmarked link.
  const setSort = (field: IssueSortField, nextOrder: IssueSortOrder) => {
    const next = new URLSearchParams(params);
    if (field === DEFAULT_ISSUE_SORT.field) next.delete("sort");
    else next.set("sort", field);
    if (nextOrder === "desc") next.delete("order");
    else next.set("order", nextOrder);
    next.delete("page");
    setParams(next, { replace: true });
  };

  // Clicking an inactive column sorts by it, descending; clicking the
  // already-active column toggles its direction.
  const handleSortClick = (field: IssueSortField) => {
    if (field === sort) setSort(field, order === "asc" ? "desc" : "asc");
    else setSort(field, "desc");
  };

  const removeScopeChip = () => {
    const next = new URLSearchParams(params);
    next.delete("bucket");
    next.delete("page");
    setParams(next, { replace: true });
  };

  const clearFilters = () => {
    const next = new URLSearchParams(params);
    for (const key of FILTER_KEYS) next.delete(key);
    next.delete("bucket");
    next.delete("page");
    setParams(next, { replace: true });
  };

  // Unfiltered: this page's own dropdowns own repo/priority/abtTeam, so its
  // option lists (and every row's project name) must never shrink as the
  // user ticks filters, and this request must stay independent of the
  // header's own (elsewhere-rendered) filtered overview.
  const { data: overview } = useOverview();
  const { data: taxonomy } = useTaxonomy();
  const isCsStatus = makeIsCsStatus(taxonomy?.csStatuses);
  const {
    data,
    isLoading,
    isPlaceholderData,
    isError,
    error,
    errorUpdatedAt,
    refetch,
  } = useIssues({
    bucket: bucket ?? undefined,
    repo,
    priority,
    abtTeam,
    status,
    slaState,
    q: params.get("q") ?? undefined,
    sort,
    order,
    limit: rowsPerPage,
    offset: page * rowsPerPage,
  });
  useReportFetchProgress(isPlaceholderData);

  const issues = data?.issues;
  const total = data?.total ?? 0;

  const projectOptions = (overview?.projects ?? []).map((p) => ({ value: p.repo, label: p.name }));
  const priorityOptions = [...priorityOptionsFrom(overview?.priorities), { value: NO_PRIORITY_VALUE, label: "No priority" }];
  const abtTeamOptions = (overview?.abtTeams ?? []).map((t) => ({ value: t, label: t }));
  const statusOptions = (taxonomy?.statuses ?? [])
    .filter((s) => !s.isTerminal && s.name !== "")
    .slice()
    .sort((a, b) => a.sortOrder - b.sortOrder)
    .map((s) => ({ value: s.name, label: s.name }));

  const hasScopeChip = !!bucket && bucket !== "all";
  const activeFilterCount =
    [repo, priority, abtTeam, status, slaState].filter((vs) => vs.length > 0).length + (hasScopeChip ? 1 : 0);

  const title = hasScopeChip ? (BUCKET_TITLES[bucket] ?? "Open issues") : "Open issues";
  const cols = gridTemplate("full");

  return (
    <Box aria-busy={isPlaceholderData}>
      <BackButton />

      {isError && issues && (
        <StaleDataAlert key={errorUpdatedAt} message={errorMessage(error, "Failed to refresh the issue list")} />
      )}

      <Box sx={{ mb: 2, mt: "18px" }}>
        <Box component="h1" sx={{ m: 0, fontSize: 22, fontWeight: 600, lineHeight: 1.2, letterSpacing: "-0.01em" }}>{title}</Box>
        <Box sx={{ mt: 0.75, fontSize: 13, color: "var(--sla-fg3)" }}>
          <Box component="b" sx={{ fontWeight: 600, color: "var(--sla-fg2)", fontFamily: "var(--font-mono)" }}>
            {total}
          </Box>{" "}
          matching open issues · click any row to open it on GitHub
        </Box>
      </Box>

      {hasScopeChip && (
        <Box sx={{ mb: 1.5 }}>
          <Chip
            label={SCOPE_CHIP_LABEL[bucket] ?? bucket}
            size="small"
            onDelete={removeScopeChip}
            sx={{ borderRadius: "8px", bgcolor: "var(--sla-surface-muted)", color: "var(--sla-fg2)", border: "1px solid var(--sla-border)" }}
          />
        </Box>
      )}

      {/* Filter row */}
      <Box sx={{ mb: 2, display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))", gap: 1.25, alignItems: "end" }}>
        <MultiSelectFilter id="filter-repo" label="Project" values={repo} options={projectOptions} onChange={(v) => setFilter("repo", v)} />
        <MultiSelectFilter id="filter-priority" label="Priority" values={priority} options={priorityOptions} onChange={(v) => setFilter("priority", v)} />
        <MultiSelectFilter id="filter-abtTeam" label="ABT Team" values={abtTeam} options={abtTeamOptions} onChange={(v) => setFilter("abtTeam", v)} />
        <MultiSelectFilter id="filter-status" label="Status" values={status} options={statusOptions} onChange={(v) => setFilter("status", v)} />
        <MultiSelectFilter id="filter-slaState" label="SLA state" values={slaState} options={SLA_STATE_OPTIONS} onChange={(v) => setFilter("slaState", v as SlaState[])} />
        {activeFilterCount > 0 && (
          <Button
            variant="outlined"
            size="small"
            onClick={clearFilters}
            startIcon={<X size={14} />}
            sx={{ height: 36, borderColor: "var(--sla-border)", color: "var(--sla-fg2)", textTransform: "none", whiteSpace: "nowrap" }}
          >
            Clear filters ({activeFilterCount})
          </Button>
        )}
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
          <span>Opened by</span>
          <span>Pri</span>
          <span>Status</span>
          <span>SLA state</span>
          <SortHeaderCell label="SLA Elapsed %" field="sla_consumption" sort={sort} order={order} onSort={handleSortClick} />
          <SortHeaderCell label="Created" field="created" sort={sort} order={order} onSort={handleSortClick} align="right" />
          <SortHeaderCell label="Updated" field="updated" sort={sort} order={order} onSort={handleSortClick} align="right" />
        </Box>

        {isError && !issues ? (
          <ErrorState message={errorMessage(error, "Failed to load issues")} onRetry={() => void refetch()} />
        ) : isLoading ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1, p: 2 }}>
            {Array.from({ length: rowsPerPage }).map((_, i) => (
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
              variant="full"
              projectName={projectNameFor(overview?.projects, issue.repo)}
              isCsStatus={isCsStatus}
            />
          ))
        )}

        {total > 0 && (
          <TablePagination
            component="div"
            count={total}
            page={page}
            onPageChange={(_, newPage) => setPage(newPage)}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={(e) => setRowsPerPage(Number(e.target.value))}
            rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
            showFirstButton
            showLastButton
            sx={{ borderTop: "1px solid var(--sla-border-soft)" }}
          />
        )}
      </Box>

      <Box sx={{ mt: 3, textAlign: "center", fontSize: 11.5, color: "var(--sla-no-sla)" }}>
        Read-only · issues open in GitHub in a new tab · Closed &amp; Terminal issues excluded
      </Box>
    </Box>
  );
}

/**
 * One clickable, sortable column header. The active column shows its arrow
 * fixed in the current direction; an inactive column shows MUI's default
 * hover-only arrow, pointing descending (what clicking it would do first).
 */
function SortHeaderCell({
  label,
  field,
  sort,
  order,
  onSort,
  align = "left",
}: {
  label: string;
  field: IssueSortField;
  sort: IssueSortField;
  order: IssueSortOrder;
  onSort: (field: IssueSortField) => void;
  align?: "left" | "right";
}) {
  const active = field === sort;
  return (
    <Box
      component="span"
      aria-sort={active ? (order === "asc" ? "ascending" : "descending") : "none"}
      sx={{ display: "flex", justifyContent: align === "right" ? "flex-end" : "flex-start" }}
    >
      <TableSortLabel
        active={active}
        direction={active ? order : "desc"}
        onClick={() => onSort(field)}
        sx={{
          flexDirection: align === "right" ? "row-reverse" : "row",
          color: "inherit",
          "&:hover": { color: "var(--sla-fg2)" },
          "&.Mui-active": { color: "var(--sla-fg)" },
          "& .MuiTableSortLabel-icon": { color: "inherit" },
        }}
      >
        {label}
      </TableSortLabel>
    </Box>
  );
}
