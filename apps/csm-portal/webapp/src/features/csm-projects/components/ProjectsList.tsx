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
import RelativeTime from "@components/RelativeTime";
import type {
  CsmProjectRow,
  CsmProjectStatus,
  CsmProjectTier,
} from "@features/csm-projects/types/csmProjects";

interface ProjectsListProps {
  projects: CsmProjectRow[];
  isLoading: boolean;
}

const GRID =
  "minmax(220px, 2fr) minmax(160px, 1.4fr) minmax(140px, 1fr) minmax(140px, 1fr) minmax(140px, 1fr) auto auto auto auto auto auto";

const HEADER_CELLS: { label: string; align?: "left" | "right" }[] = [
  { label: "Project" },
  { label: "Customer" },
  { label: "Product" },
  { label: "Account Manager" },
  { label: "Technical Owner" },
  { label: "Tier" },
  { label: "Status" },
  { label: "Open", align: "right" },
  { label: "S0/S1", align: "right" },
  { label: "Breached", align: "right" },
  { label: "Last activity", align: "right" },
];

function TierChip({ tier }: { tier: CsmProjectTier }): JSX.Element {
  const color: "primary" | "warning" | "default" =
    tier === "Platinum" ? "primary" : tier === "Gold" ? "warning" : "default";
  return <Chip size="small" variant="outlined" label={tier} color={color} />;
}

function StatusChip({ status }: { status: CsmProjectStatus }): JSX.Element {
  const color: "success" | "warning" | "default" =
    status === "Active"
      ? "success"
      : status === "Onboarding"
        ? "warning"
        : "default";
  return <Chip size="small" variant="outlined" label={status} color={color} />;
}

export default function ProjectsList({
  projects,
  isLoading,
}: ProjectsListProps): JSX.Element {
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

      {isLoading &&
        [0, 1, 2, 3, 4].map((i) => (
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

      {!isLoading && projects.length === 0 && (
        <Box sx={{ gridColumn: "1 / -1", px: 2, py: 4, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            No projects match the current filters.
          </Typography>
        </Box>
      )}

      {!isLoading &&
        projects.map((p) => {
          // Drill into the CSM-native project detail page. The legacy
          // customer-portal dashboard at `/projects/:id/dashboard` is still
          // reachable directly but requires a real backend project id.
          const href = `/projects/${p.id}`;
          const handleClick = () => navigate(href);
          return (
            <Box
              key={p.id}
              role="button"
              tabIndex={0}
              aria-label={`${p.name} — open project`}
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
                "&:hover": { bgcolor: "action.hover" },
                "&:focus-visible": {
                  outline: `2px solid ${theme.palette.primary.main}`,
                  outlineOffset: -2,
                },
                "&:last-of-type": { borderBottom: 0 },
              }}
            >
              <Box sx={{ minWidth: 0 }}>
                <Typography variant="body2" noWrap>
                  <strong>{p.name}</strong>
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap>
                  {p.updateLevel}
                </Typography>
              </Box>
              <Typography variant="body2" noWrap>
                {p.customer}
              </Typography>
              <Typography variant="body2" noWrap>
                {p.productType}
              </Typography>
              <Typography variant="body2" color="text.secondary" noWrap>
                {p.accountManager ?? "—"}
              </Typography>
              <Typography variant="body2" color="text.secondary" noWrap>
                {p.technicalOwner ?? "—"}
              </Typography>
              <TierChip tier={p.tier} />
              <StatusChip status={p.status} />
              <Typography variant="body2" sx={{ textAlign: "right" }}>
                {p.openCaseCount}
              </Typography>
              <Typography
                variant="body2"
                color={p.s0s1Count > 0 ? "error" : "text.primary"}
                sx={{ textAlign: "right" }}
              >
                {p.s0s1Count}
              </Typography>
              <Typography
                variant="body2"
                color={p.breachedCount > 0 ? "error" : "text.primary"}
                sx={{ textAlign: "right" }}
              >
                {p.breachedCount}
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ textAlign: "right" }}
                noWrap
              >
                <RelativeTime iso={p.lastActivityAt} />
              </Typography>
            </Box>
          );
        })}
    </Box>
  );
}
