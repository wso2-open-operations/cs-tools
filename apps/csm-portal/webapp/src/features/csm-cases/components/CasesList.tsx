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

import { Box, Chip, Skeleton, Typography, useTheme } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useNavigate } from "react-router";
import {
  SEVERITY_COLOR,
  SLA_CLOCK_LABEL,
  STATE_LABEL,
  formatTimeToBreach,
} from "@features/csm-dashboard/utils/abtDashboard";
import RelativeTime from "@components/RelativeTime";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

interface CasesListProps {
  cases: CsmCaseRow[];
  isLoading: boolean;
}

const HEADER_CELLS: { label: string; align?: "left" | "right" }[] = [
  { label: "Case" },
  { label: "Customer" },
  { label: "Severity" },
  { label: "State" },
  { label: "Assignee" },
  { label: "SLA", align: "right" },
  { label: "Updated", align: "right" },
];

const GRID =
  "minmax(280px, 2.5fr) minmax(140px, 1fr) auto minmax(140px, 1fr) minmax(120px, 1fr) auto auto";

export default function CasesList({
  cases,
  isLoading,
}: CasesListProps): JSX.Element {
  const navigate = useNavigate();
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
        {HEADER_CELLS.map((h) => (
          <Typography
            key={h.label}
            variant="caption"
            color="text.secondary"
            sx={{ textAlign: h.align ?? "left", fontWeight: 600 }}
          >
            {h.label}
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
          const breached = c.minutesToBreach < 0;
          const handleClick = () => {
            navigate(`/cases/${c.id}`);
          };
          return (
            <Box
              key={c.id}
              role="button"
              tabIndex={0}
              onClick={handleClick}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  handleClick();
                }
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
                "&:focus-visible": {
                  outline: `2px solid ${theme.palette.primary.main}`,
                  outlineOffset: -2,
                },
                "&:last-of-type": { borderBottom: 0 },
              }}
            >
              <Box sx={{ minWidth: 0 }}>
                <Typography variant="body2" noWrap>
                  <strong>{c.caseNumber}</strong> · {c.subject}
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap>
                  {c.projectName}
                </Typography>
              </Box>
              <Typography variant="body2" noWrap>
                {c.customer}
              </Typography>
              <Chip
                size="small"
                label={c.severity}
                color={SEVERITY_COLOR[c.severity]}
              />
              <Typography variant="body2" noWrap>
                {STATE_LABEL[c.state]}
              </Typography>
              <Typography variant="body2" noWrap>
                {c.assigneeIsMe ? (
                  <strong>{c.assignee}</strong>
                ) : c.assignee === "Unassigned" ? (
                  <em>Unassigned</em>
                ) : (
                  c.assignee
                )}
              </Typography>
              <Box sx={{ textAlign: "right" }}>
                <Typography
                  variant="body2"
                  color={breached ? "error" : c.minutesToBreach <= 60 ? "warning.main" : "text.primary"}
                  noWrap
                >
                  {c.state === "closed"
                    ? "—"
                    : formatTimeToBreach(c.minutesToBreach)}
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap>
                  {c.state === "closed" ? "Closed" : SLA_CLOCK_LABEL[c.slaClockType]}
                </Typography>
              </Box>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ textAlign: "right" }}
                noWrap
              >
                <RelativeTime iso={c.updatedAt} />
              </Typography>
            </Box>
          );
        })}
    </Box>
  );
}
