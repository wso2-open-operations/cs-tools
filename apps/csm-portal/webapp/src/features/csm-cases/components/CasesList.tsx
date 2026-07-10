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
  Skeleton,
  TableSortLabel,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import { preloadRoute } from "@utils/routePreloaders";
import RelativeTime from "@components/RelativeTime";
import SeverityChip from "@components/SeverityChip";
import StateChip from "@components/StateChip";
import { WORK_STATE_LABEL } from "@features/csm-cases/utils/caseWorkState";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";
import type { CasesSortOrder } from "@features/csm-cases/utils/casesSort";

interface CasesListProps {
  cases: CsmCaseRow[];
  isLoading: boolean;
  /** Number of skeleton rows to show while loading. Defaults to 6. */
  skeletonCount?: number;
  /** Base path for detail links. Defaults to "/cases". */
  detailBasePath?: string;
  /** Current sort order for the "Updated" column, when the caller wants it
   * sortable. Omit both `sortOrder` and `onSortOrderChange` for a plain
   * (non-interactive) header — the list is always server-sorted by
   * `updatedOn`, this just lets the user flip the direction. */
  sortOrder?: CasesSortOrder;
  onSortOrderChange?: (order: CasesSortOrder) => void;
}

// Every column is left-aligned for a consistent scan line down the table.
const HEADER_CELLS: string[] = [
  "Case ID",
  "Subject",
  "Product",
  "Severity",
  "State",
];

// Subject gets the lion's share of the row; the ids sit in their own narrow
// column so a long subject no longer has to share one cell with them.
// The work-state chip (only present for WIP cases) stacks under the State chip
// in the State column, so it doesn't need a column of its own.
const GRID =
  "minmax(120px, 0.9fr) minmax(280px, 3fr) minmax(140px, 1fr) auto minmax(110px, 1fr) auto";

export default function CasesList({
  cases,
  isLoading,
  skeletonCount = 6,
  detailBasePath = "/cases",
  sortOrder,
  onSortOrderChange,
}: CasesListProps): JSX.Element {
  const theme = useTheme();

  return (
    <Box
      sx={{
        border: 1,
        borderColor: "divider",
        borderRadius: 1,
        overflow: "hidden",
        // Single grid context drives column tracks for header + every row.
        // Each row below uses `grid-template-columns: subgrid` so columns line up.
        display: "grid",
        gridTemplateColumns: GRID,
        columnGap: 2,
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
        {HEADER_CELLS.map((label) => (
          <Typography
            key={label}
            variant="caption"
            color="text.secondary"
            sx={{ fontWeight: 600, textAlign: "left" }}
          >
            {label}
          </Typography>
        ))}
        {sortOrder && onSortOrderChange ? (
          <TableSortLabel
            active
            direction={sortOrder}
            onClick={() =>
              onSortOrderChange(sortOrder === "desc" ? "asc" : "desc")
            }
            sx={{
              justifySelf: "start",
              "& .MuiTableSortLabel-icon": { fontSize: "1rem" },
            }}
          >
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ fontWeight: 600 }}
            >
              Updated
            </Typography>
          </TableSortLabel>
        ) : (
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{ fontWeight: 600, textAlign: "left" }}
          >
            Updated
          </Typography>
        )}
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
          return (
            // A real anchor (not a click-handler div) so the row supports
            // cmd/middle-click "open in new tab" — essential when an engineer
            // pulls up other cases for reference — and exposes a copyable URL
            // (ISSU-031). RouterLink keeps plain left-click as in-app SPA nav.
            <Box
              key={c.id}
              component={RouterLink}
              to={`${detailBasePath}/${c.id}`}
              onMouseEnter={() => preloadRoute(detailBasePath)}
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
                color: "inherit",
                textDecoration: "none",
                "&:hover": {
                  bgcolor: "action.hover",
                },
                "&:focus-visible": {
                  outline: `2px solid ${theme.palette.primary.main}`,
                  outlineOffset: -2,
                },
                "&:last-of-type": { borderBottom: 0 },
              }}
            >
              {/* Case ids: WSO2 internal id on top, CS number beneath. Never
                  the UUID. "—" when the case has neither yet. */}
              <Box sx={{ minWidth: 0 }}>
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
              {/* Subject (the widest column) + project for context. */}
              <Box sx={{ minWidth: 0 }}>
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
              <Typography variant="body2" noWrap title={c.product || undefined}>
                {c.product}
              </Typography>
              <Box sx={{ justifySelf: "start" }}>
                <SeverityChip severity={c.severity} clickable />
              </Box>
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
                {c.state === "work_in_progress" && c.workState && (
                  <Chip
                    size="small"
                    variant="outlined"
                    color={c.workState === "paused" ? "warning" : "default"}
                    label={WORK_STATE_LABEL[c.workState]}
                    sx={{ fontWeight: 600 }}
                  />
                )}
              </Box>
              <Typography variant="caption" color="text.secondary" noWrap>
                {c.updatedAtIsCreatedFallback && "Created "}
                <RelativeTime iso={c.updatedAt} />
              </Typography>
            </Box>
          );
        })}
    </Box>
  );
}
