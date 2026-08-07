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

import { Box, Skeleton, Typography, useTheme } from "@wso2/oxygen-ui";
import type { JSX, KeyboardEvent, ReactNode } from "react";
import { Link as RouterLink } from "react-router";

export interface DashboardMiniTableColumn {
  label: string;
  /** CSS grid track for this column. Defaults to an equal-share flexible track. */
  width?: string;
}

export interface DashboardMiniTableRow {
  key: string;
  /** Row's own detail page — the row is a real link (not a click handler),
   * same rationale as `CasesList`: supports cmd/middle-click and exposes a
   * copyable URL. Omit when the row's id is unknown (e.g. a nullable `id`
   * field on the search response) — the row then renders inert rather than
   * linking to a broken `/…/undefined` URL. Mutually exclusive with `onClick`
   * — a row with no standalone detail *page* (e.g. a task, only ever shown
   * in a dialog) uses `onClick` instead. */
  href?: string;
  /** Router state carried on `href`'s navigation — e.g. `{ from: <dashboard
   * path> }`, so the destination page's own Back button can return here
   * instead of falling through to a hardcoded/generic destination. Ignored
   * when `href` is unset. */
  state?: unknown;
  /** Opens something in place (a dialog) rather than navigating — for a
   * resource with no standalone detail route. See `href`'s doc comment for
   * when to use which. */
  onClick?: () => void;
  cells: ReactNode[];
}

interface DashboardMiniTableProps {
  columns: DashboardMiniTableColumn[];
  rows: DashboardMiniTableRow[];
  isLoading: boolean;
  /** Number of skeleton rows to show while loading. Defaults to 4 — dashboard
   * list widgets are always capped at 4 records. */
  skeletonCount?: number;
  emptyMessage: string;
}

/**
 * Compact grid-based table for a dashboard `shape: "list"` widget — same
 * visual pattern as `CasesList`/`TimeCardsTable` (a single CSS grid context
 * driving header + row column tracks via `subgrid`, rather than a full MUI
 * `Table`), but generic over columns/cells so each resource type only
 * supplies its own column labels and cell content instead of a bespoke
 * table component.
 */
export default function DashboardMiniTable({
  columns,
  rows,
  isLoading,
  skeletonCount = 4,
  emptyMessage,
}: DashboardMiniTableProps): JSX.Element {
  const theme = useTheme();
  const gridTemplateColumns = columns.map((c) => c.width ?? "minmax(80px, 1fr)").join(" ");

  return (
    <Box
      sx={{
        border: 1,
        borderColor: "divider",
        borderRadius: 1,
        overflow: "hidden",
        display: "grid",
        gridTemplateColumns,
        columnGap: 2,
      }}
    >
      <Box
        sx={{
          gridColumn: "1 / -1",
          display: "grid",
          gridTemplateColumns: "subgrid",
          columnGap: 2,
          alignItems: "center",
          px: 1.5,
          py: 1,
          bgcolor: "action.hover",
          borderBottom: 1,
          borderColor: "divider",
        }}
      >
        {columns.map((c) => (
          <Typography key={c.label} variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
            {c.label}
          </Typography>
        ))}
      </Box>

      {isLoading &&
        Array.from({ length: skeletonCount }).map((_, i) => (
          <Box
            key={i}
            sx={{
              gridColumn: "1 / -1",
              px: 1.5,
              py: 1,
              borderBottom: 1,
              borderColor: "divider",
              "&:last-of-type": { borderBottom: 0 },
            }}
          >
            <Skeleton variant="rounded" height={20} />
          </Box>
        ))}

      {!isLoading && rows.length === 0 && (
        <Box sx={{ gridColumn: "1 / -1", px: 1.5, py: 2.5, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            {emptyMessage}
          </Typography>
        </Box>
      )}

      {!isLoading &&
        rows.map((row) => {
          const isInteractive = Boolean(row.href) || Boolean(row.onClick);
          return (
            <Box
              key={row.key}
              {...(row.href
                ? { component: RouterLink, to: row.href, state: row.state }
                : row.onClick
                  ? { onClick: row.onClick, role: "button", tabIndex: 0 }
                  : {})}
              onKeyDown={
                row.onClick
                  ? (e: KeyboardEvent) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        row.onClick?.();
                      }
                    }
                  : undefined
              }
              sx={{
                gridColumn: "1 / -1",
                display: "grid",
                gridTemplateColumns: "subgrid",
                columnGap: 2,
                alignItems: "center",
                px: 1.5,
                py: 1,
                borderBottom: 1,
                borderColor: "divider",
                color: "inherit",
                textDecoration: "none",
                "&:last-of-type": { borderBottom: 0 },
                ...(isInteractive && {
                  cursor: "pointer",
                  "&:hover": { bgcolor: "action.hover" },
                  "&:focus-visible": {
                    outline: `2px solid ${theme.palette.primary.main}`,
                    outlineOffset: -2,
                  },
                }),
              }}
            >
              {row.cells.map((cell, i) => (
                <Box key={i} sx={{ minWidth: 0 }}>
                  {cell}
                </Box>
              ))}
            </Box>
          );
        })}
    </Box>
  );
}
