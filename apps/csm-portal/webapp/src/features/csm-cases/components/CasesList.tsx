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
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import RelativeTime from "@components/RelativeTime";
import SeverityChip from "@components/SeverityChip";
import StateChip from "@components/StateChip";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

interface CasesListProps {
  cases: CsmCaseRow[];
  isLoading: boolean;
  /** Base path for detail links. Defaults to "/cases". */
  detailBasePath?: string;
}

// Every column is left-aligned for a consistent scan line down the table.
const HEADER_CELLS: string[] = [
  "Case ID",
  "Subject",
  "Product",
  "Severity",
  "State",
  "Updated",
];

// Subject gets the lion's share of the row; the ids sit in their own narrow
// column so a long subject no longer has to share one cell with them.
const GRID =
  "minmax(120px, 0.9fr) minmax(280px, 3fr) minmax(140px, 1fr) auto minmax(110px, 1fr) auto";

export default function CasesList({
  cases,
  isLoading,
  detailBasePath = "/cases",
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
      </Box>

      {/* Rows */}
      {isLoading &&
        [0, 1, 2, 3, 4, 5].map((i) => (
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
            <Skeleton variant="rectangular" height={28} />
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
                    sx={{ fontFamily: "monospace", display: "block" }}
                  >
                    {c.caseNumber}
                  </Typography>
                )}
                {!c.wso2CaseId && !c.caseNumber && (
                  <Typography variant="body2" color="text.secondary">
                    —
                  </Typography>
                )}
              </Box>
              {/* Subject (the widest column) + project for context. */}
              <Box sx={{ minWidth: 0 }}>
                <Typography variant="body2" noWrap>
                  {c.subject}
                </Typography>
                <Typography
                  variant="caption"
                  color="text.secondary"
                  noWrap
                  sx={{ display: "block" }}
                >
                  {c.projectName}
                </Typography>
              </Box>
              <Typography variant="body2" noWrap>
                {c.product}
              </Typography>
              <Box sx={{ justifySelf: "start" }}>
                <SeverityChip severity={c.severity} clickable />
              </Box>
              <Box sx={{ justifySelf: "start" }}>
                <StateChip state={c.state} clickable />
              </Box>
              <Typography variant="caption" color="text.secondary" noWrap>
                <RelativeTime iso={c.updatedAt} />
              </Typography>
            </Box>
          );
        })}
    </Box>
  );
}
