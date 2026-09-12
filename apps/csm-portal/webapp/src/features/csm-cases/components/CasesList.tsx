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

import {
  Box,
  Chip,
  IconButton,
  Skeleton,
  TableSortLabel,
  Tooltip,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import { Eye } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";
import { Link as RouterLink, useLocation } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import EscalationLevelChip from "@components/EscalationLevelChip";
import RelativeTime from "@components/RelativeTime";
import SeverityChip from "@components/SeverityChip";
import StateChip from "@components/StateChip";
import WorkStateChip from "@components/WorkStateChip";
import CasePreviewDrawer from "@features/csm-cases/components/CasePreviewDrawer";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";
import type {
  CasesSortField,
  CasesSortOrder,
} from "@features/csm-cases/utils/casesSort";
import {
  CASE_TYPE_COLOR,
  CASE_TYPE_LABEL,
  caseTypeDetailBasePath,
  caseTypeHasSeverity,
} from "@features/csm-cases/utils/caseType";
import { ISSUE_TYPE_LABEL } from "@features/csm-cases/utils/caseIssueType";
import { effectiveWorkState } from "@features/csm-cases/utils/caseWorkState";
import {
  CASE_OPTIONAL_COLUMNS,
  type CaseOptionalColumnId,
} from "@features/csm-cases/utils/caseListColumns";

interface CasesListProps {
  cases: CsmCaseRow[];
  isLoading: boolean;
  /** Number of skeleton rows to show while loading. Defaults to 6. */
  skeletonCount?: number;
  /** Base path for detail links. Omit to have each row route to its own
   * case type's own detail page (see `caseTypeDetailBasePath`) — the right
   * choice for a mixed-type list. Pass an explicit value only when every row
   * is already locked to one type (e.g. the Engagements/Security Center
   * tabs), to route there directly without a per-row type check. */
  detailBasePath?: string;
  /** Hide the Severity column. Severity (S1-S4) is a support-case concept, so
   * non-case lists (service requests, engagements, security reports) hide it —
   * the main case list keeps it. Ignored when `optionalColumns` is passed
   * explicitly (that list is the sole source of truth for which optional
   * columns render, and in what order). */
  hideSeverityColumn?: boolean;
  /** Which optional columns to render, and in what order — driven by a
   * caller's `useColumnPreferences` (e.g. `CsmIssuesView`'s "Customise
   * columns" picker on the Engagements list). Omit to keep this list's
   * long-standing fixed set, gated only by `hideSeverityColumn`, exactly as
   * every other caller of `CasesList` already gets it. */
  optionalColumns?: CaseOptionalColumnId[];
  /** Which column is currently driving the server-side sort — Updated and
   * State are always present; Created and Severity only sort when their own
   * optional column is visible (see `OPTIONAL_COLUMN_SORT_FIELD`). Pass all
   * four of `sortField` / `sortOrder` / `onSortFieldChange` /
   * `onSortOrderChange` together for sortable headers, or omit all four for
   * plain (non-interactive) ones — a caller that only wants the legacy
   * "Updated" toggle can still do that by wiring only the "Updated" click
   * through, since every header's click handler always reports back through
   * these same two callbacks. */
  sortField?: CasesSortField;
  onSortFieldChange?: (field: CasesSortField) => void;
  sortOrder?: CasesSortOrder;
  onSortOrderChange?: (order: CasesSortOrder) => void;
  /** A "Customise columns" trigger (`ColumnCustomizerButton`), rendered in a
   * small toolbar row directly above this table. Owned by the caller (e.g.
   * `CsmIssuesView`, or a dashboard widget preview) since only it knows this
   * view's own `useColumnPreferences` wiring (viewId, available/default
   * columns) — this component just reserves the slot next to the table it
   * actually controls, rather than the control living in a page header far
   * away from the table it affects. Omit to render no toolbar row at all. */
  columnCustomizer?: ReactNode;
}

/** Maps the optional columns that double as sort headers to the field they
 * sort by. Columns not listed here (Product, Type, Assignee, Customer) have
 * no server-side sort of their own. */
const OPTIONAL_COLUMN_SORT_FIELD: Partial<Record<CaseOptionalColumnId, CasesSortField>> = {
  createdAt: "createdOn",
  severity: "severity",
};

function renderOptionalCell(id: CaseOptionalColumnId, c: CsmCaseRow): JSX.Element {
  switch (id) {
    case "product":
      return (
        <Typography variant="body2" noWrap title={c.product || undefined}>
          {c.product}
        </Typography>
      );
    case "type":
      return (
        <Box sx={{ justifySelf: "start" }}>
          {c.caseType ? (
            <Chip
              size="small"
              variant="outlined"
              color={CASE_TYPE_COLOR[c.caseType]}
              label={CASE_TYPE_LABEL[c.caseType]}
            />
          ) : (
            "—"
          )}
        </Box>
      );
    case "severity":
      return (
        <Box sx={{ justifySelf: "start" }}>
          {caseTypeHasSeverity(c.caseType) ? (
            <SeverityChip severity={c.severity} clickable />
          ) : (
            "—"
          )}
        </Box>
      );
    case "issueType":
      return (
        <Typography variant="body2" noWrap>
          {c.issueType ? ISSUE_TYPE_LABEL[c.issueType] : "—"}
        </Typography>
      );
    case "assignee":
      return (
        <Typography variant="body2" noWrap title={c.assignee || undefined}>
          {c.assignee}
        </Typography>
      );
    case "createdBy":
      return (
        <Typography variant="body2" noWrap title={c.createdBy || undefined}>
          {c.createdBy || "—"}
        </Typography>
      );
    case "customer":
      return (
        <Typography variant="body2" noWrap title={c.customer || undefined}>
          {c.customer}
        </Typography>
      );
    case "createdAt":
      return (
        <Typography variant="caption" color="text.secondary" noWrap>
          <RelativeTime iso={c.createdAt} />
        </Typography>
      );
    case "escalationLevel":
      // Blank (not a "not escalated" chip, not even a "—" dash) for a
      // non-escalated row — matches the work-state column's "only render for
      // the rows it's relevant to" precedent, so a mostly-empty column
      // doesn't clutter every non-escalated row.
      return (
        <Box sx={{ justifySelf: "start" }}>
          {c.escalationLevel && c.escalationLevel !== "0" && (
            <EscalationLevelChip level={c.escalationLevel} short />
          )}
        </Box>
      );
  }
}

// Subject gets the lion's share of the row; the ids sit in their own narrow
// column so a long subject no longer has to share one cell with them.
// The work-state chip (only present for WIP cases) stacks under the State chip
// in the State column, so it doesn't need a column of its own. The leading
// `auto` track (unlabeled in the header) holds the per-row quick-preview
// action — kept at the left edge so it's reachable without hunting across
// the row, with the preview drawer itself opening on the right.
const CASE_ID_TRACK = "minmax(120px, 0.9fr)";
const SUBJECT_TRACK = "minmax(280px, 3fr)";
const STATE_TRACK = "minmax(110px, 1fr)";

export default function CasesList({
  cases,
  isLoading,
  skeletonCount = 6,
  detailBasePath,
  hideSeverityColumn = false,
  optionalColumns,
  sortField,
  onSortFieldChange,
  sortOrder,
  onSortOrderChange,
  columnCustomizer,
}: CasesListProps): JSX.Element {
  const theme = useTheme();
  const location = useLocation();
  const navigate = useNavTransition();
  const [previewRow, setPreviewRow] = useState<CsmCaseRow | null>(null);

  // Legacy fixed sets, kept byte-for-byte identical to this list's original
  // behavior for every caller that doesn't pass `optionalColumns` explicitly
  // (dashboard widgets, mini tables, project work items, …).
  const effectiveOptionalColumns: CaseOptionalColumnId[] =
    optionalColumns ?? (hideSeverityColumn ? ["product"] : ["product", "type", "severity"]);

  // Sortable headers only render as such when the caller wired up all four
  // sort props — a caller that passes none of them (dashboard widgets, mini
  // tables) keeps every header as plain, non-interactive text.
  const sortInteractive =
    sortField !== undefined &&
    sortOrder !== undefined &&
    onSortFieldChange !== undefined &&
    onSortOrderChange !== undefined;

  const handleSortClick = (field: CasesSortField): void => {
    if (!onSortFieldChange || !onSortOrderChange) return;
    if (field === sortField) {
      onSortOrderChange(sortOrder === "desc" ? "asc" : "desc");
    } else {
      onSortFieldChange(field);
      onSortOrderChange("desc");
    }
  };

  function renderHeaderCell(label: string, field?: CasesSortField): JSX.Element {
    if (!field || !sortInteractive) {
      return (
        <Typography
          key={label}
          variant="caption"
          color="text.secondary"
          sx={{ fontWeight: 600, textAlign: "left" }}
        >
          {label}
        </Typography>
      );
    }
    const isActive = field === sortField;
    return (
      <TableSortLabel
        key={label}
        active={isActive}
        direction={isActive ? sortOrder : "desc"}
        onClick={() => handleSortClick(field)}
        sx={{
          justifySelf: "start",
          "& .MuiTableSortLabel-icon": { fontSize: "1rem" },
        }}
      >
        <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
          {label}
        </Typography>
      </TableSortLabel>
    );
  }

  const headerCells: { label: string; sortableField?: CasesSortField }[] = [
    { label: "Case ID" },
    { label: "Subject" },
    ...effectiveOptionalColumns.map((id) => ({
      label: CASE_OPTIONAL_COLUMNS[id].label,
      sortableField: OPTIONAL_COLUMN_SORT_FIELD[id],
    })),
    { label: "State", sortableField: "state" as CasesSortField },
  ];
  const gridTemplateColumns = [
    "auto",
    CASE_ID_TRACK,
    SUBJECT_TRACK,
    ...effectiveOptionalColumns.map((id) => CASE_OPTIONAL_COLUMNS[id].track),
    STATE_TRACK,
    "auto",
  ].join(" ");

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      {columnCustomizer && (
        <Box sx={{ display: "flex", justifyContent: "flex-end" }}>{columnCustomizer}</Box>
      )}
      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        {/* Scrolls horizontally on its own, independent of the rest of the
            page, once the grid below can't fit every visible column at its
            own minimum width — without this, the grid's tracks were free to
            shrink past their `minmax(...)` floors to fit the container,
            which read as column content getting squished/clipped instead of
            the table scrolling (reported live once most optional columns
            were turned on at once). */}
        <Box sx={{ overflowX: "auto" }}>
          <Box
            sx={{
              // Single grid context drives column tracks for header + every row.
              // Each row below uses `grid-template-columns: subgrid` so columns line up.
              display: "grid",
              gridTemplateColumns,
              columnGap: 2,
              // The actual fix: a grid container's automatic minimum width
              // otherwise lets its tracks shrink below their own `minmax`
              // floor to fit whatever space is available. Forcing the grid
              // itself to its content's true minimum width means it can only
              // ever overflow its `overflowX: auto` parent above — never
              // shrink its own columns to fit.
              minWidth: "max-content",
            }}
          >
            {/* Header */}
            <Box
              sx={{
                gridColumn: "1 / -1",
                display: "grid",
                gridTemplateColumns: "subgrid",
                columnGap: 2,
                alignItems: "center",
                px: 2,
                py: 1.25,
                bgcolor: "action.hover",
                borderBottom: 1,
                borderColor: "divider",
              }}
            >
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ fontWeight: 600, textAlign: "left" }}
              >
                Preview
              </Typography>
              {headerCells.map((cell) => renderHeaderCell(cell.label, cell.sortableField))}
              {renderHeaderCell("Updated", "updatedOn")}
            </Box>

            {/* Rows */}
            {isLoading &&
              Array.from({ length: skeletonCount }).map((_, i) => (
                <Box
                  key={i}
                  sx={{
                    gridColumn: "1 / -1",
                    px: 2,
                    py: 1.25,
                    borderBottom: 1,
                    borderColor: "divider",
                    "&:last-of-type": { borderBottom: 0 },
                  }}
                >
                  <Skeleton variant="rounded" height={28} />
                </Box>
              ))}

            {!isLoading && cases.length === 0 && (
              <Box sx={{ gridColumn: "1 / -1", px: 2, py: 4, textAlign: "center" }}>
                <Typography variant="body2" color="text.secondary">
                  No cases match the current filters.
                </Typography>
              </Box>
            )}

            {!isLoading &&
              cases.map((c) => {
                const rowBasePath = detailBasePath ?? caseTypeDetailBasePath(c.caseType);
                const rowState = { from: `${location.pathname}${location.search}` };
                const rowLabel = c.wso2CaseId || c.caseNumber || c.subject;
                return (
                  <Box
                    key={c.id}
                    onClick={(e) => {
                      // The case-id block below is a real RouterLink: a plain click
                      // on it already navigates via its own handler (which calls
                      // preventDefault), and that click still bubbles up here — do
                      // nothing in that case, or this would push a second, duplicate
                      // history entry for the same destination. A modified click
                      // (cmd/ctrl/shift/alt) on that link deliberately does *not*
                      // preventDefault — react-router leaves it to the browser to
                      // open a new tab — so skip here too, or this would navigate
                      // the current tab away from the list while a new tab is also
                      // opening, defeating the point of cmd-click "open in new tab
                      // for reference" the link exists for.
                      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
                      navigate(`${rowBasePath}/${c.id}`, { state: rowState });
                    }}
                    sx={{
                      gridColumn: "1 / -1",
                      display: "grid",
                      gridTemplateColumns: "subgrid",
                      columnGap: 2,
                      alignItems: "center",
                      px: 2,
                      py: 1.25,
                      borderBottom: 1,
                      borderColor: "divider",
                      cursor: "pointer",
                      "&:hover": {
                        bgcolor: "action.hover",
                      },
                      "&:last-of-type": { borderBottom: 0 },
                    }}
                  >
                    {/* Quick preview, at the row's left edge so it's the first
                        thing reachable without hunting across the row; the drawer
                        itself opens on the right. `stopPropagation` keeps the click
                        from also bubbling up into the row's own onClick (which
                        would open the full case instead of just previewing it).
                        Clicking the eye for the row already being previewed closes
                        it instead of re-opening the same preview. */}
                    <Tooltip title={`Quick preview ${rowLabel}`}>
                      <IconButton
                        size="small"
                        aria-label={`Quick preview ${rowLabel}`}
                        aria-pressed={previewRow?.id === c.id}
                        data-quick-preview-eye="true"
                        onClick={(e) => {
                          e.stopPropagation();
                          setPreviewRow((prev) => (prev?.id === c.id ? null : c));
                        }}
                        sx={{ justifySelf: "center" }}
                      >
                        <Eye size={16} />
                      </IconButton>
                    </Tooltip>
                    {/* Case ids: WSO2 internal id on top, CS number beneath. Never
                        the UUID. "—" when the case has neither yet. This block is
                        the row's one real anchor — cmd/middle-click "open in new
                        tab" (essential when pulling up other cases for reference),
                        a copyable URL (ISSU-031), and a keyboard tab stop. The rest
                        of the row relies on the onClick above for a mouse click
                        anywhere in it; making the whole row a real anchor (or a
                        role="button" override) would either force the quick-preview
                        button below into invalid button-inside-anchor nesting, or
                        strip the row's cells of their own semantics. */}
                    <Box
                      component={RouterLink}
                      to={`${rowBasePath}/${c.id}`}
                      state={rowState}
                      aria-label={`Open ${rowLabel}`}
                      sx={{
                        minWidth: 0,
                        color: "inherit",
                        textDecoration: "none",
                        display: "block",
                        "&:focus-visible": {
                          outline: `2px solid ${theme.palette.primary.main}`,
                          outlineOffset: 2,
                          borderRadius: 0.5,
                        },
                      }}
                    >
                      {c.wso2CaseId && (
                        <Typography
                          variant="body2"
                          noWrap
                          title={c.wso2CaseId}
                          sx={{ fontFamily: "monospace", fontWeight: 600 }}
                        >
                          {c.wso2CaseId}
                        </Typography>
                      )}
                      {c.caseNumber && (
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          noWrap
                          title={c.caseNumber}
                          sx={{ fontFamily: "monospace", display: "block" }}
                        >
                          {c.caseNumber}
                        </Typography>
                      )}
                    </Box>
                    {/* Subject (the widest column) + project for context.
                        `maxWidth` here (not just `noWrap`'s ellipsis, which
                        only clips paint — it doesn't affect intrinsic sizing)
                        is what keeps one long subject line from blowing up
                        this column's width, and via the grid's own
                        `minWidth: "max-content"` above, the whole grid's: a
                        grid item's max-content *contribution* to track sizing
                        is clamped by its own specified max-width, so without
                        one here a single unusually long subject forced this
                        default, always-visible column to scroll horizontally
                        even with zero optional columns turned on. 360 (not a
                        more generous first attempt of 480) matches
                        ChangeRequestsTab.tsx/IncidentsTab.tsx's own Subject
                        column cap — a cap that's merely "bounded" instead of
                        "small enough" still adds up: 480 (Subject) + 260
                        (one default-visible optional column) + the other
                        default columns' own floors summed to more than a
                        normal viewport width, forcing horizontal scroll on
                        the *default* view even with everyday-length values,
                        not just pathologically long ones. */}
                    <Box sx={{ minWidth: 0, maxWidth: 360 }}>
                      <Typography variant="body2" noWrap title={c.subject}>
                        {c.subject}
                      </Typography>
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        noWrap
                        title={c.projectName || undefined}
                        sx={{ display: "block" }}
                      >
                        {c.projectName}
                      </Typography>
                    </Box>
                    {/* Same fix as Subject above, for the same reason: every
                        optional column's own track is `minmax(140px, 1fr)`
                        (see caseListColumns.ts) — mechanically identical to
                        Subject's `minmax(280px, 3fr)` — so without a real
                        max-width here too, one long product/customer/person
                        name in an optional column can blow up the grid's
                        width exactly like an unbounded Subject used to
                        (reported live for Security Reports' Product column,
                        but the bug is shared by every CasesList caller). */}
                    {effectiveOptionalColumns.map((id) => (
                      <Box key={id} sx={{ minWidth: 0, maxWidth: 260 }}>
                        {renderOptionalCell(id, c)}
                      </Box>
                    ))}
                    {/* State chip, with the work-state chip stacked beneath it (the
                        latter only for WIP cases). */}
                    <Box
                      sx={{
                        justifySelf: "start",
                        display: "flex",
                        flexDirection: "column",
                        alignItems: "flex-start",
                        gap: 0.5,
                      }}
                    >
                      <StateChip state={c.state} variant="outlined" clickable />
                      {c.state === "work_in_progress" && (
                        <WorkStateChip workState={effectiveWorkState(c.workState)} />
                      )}
                    </Box>
                    <Typography variant="caption" color="text.secondary" noWrap>
                      <RelativeTime iso={c.updatedAt} />
                    </Typography>
                  </Box>
                );
              })}
            <CasePreviewDrawer row={previewRow} onClose={() => setPreviewRow(null)} />
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
