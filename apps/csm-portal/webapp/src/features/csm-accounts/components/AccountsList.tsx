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
import type { CsmAccountRow } from "@features/csm-accounts/types/csmAccounts";
import type {
  CsmProjectStatus,
  CsmProjectTier,
} from "@features/csm-projects/types/csmProjects";

interface AccountsListProps {
  accounts: CsmAccountRow[];
  isLoading: boolean;
}

const GRID =
  "minmax(220px, 2fr) auto auto auto auto auto auto auto";

const HEADER_CELLS: { label: string; align?: "left" | "right" }[] = [
  { label: "Account" },
  { label: "Tier" },
  { label: "Status" },
  { label: "Projects", align: "right" },
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

export default function AccountsList({
  accounts,
  isLoading,
}: AccountsListProps): JSX.Element {
  const navigate = useNavigate();
  const theme = useTheme();

  return (
    <Box
      sx={{
        border: 1,
        borderColor: "divider",
        borderRadius: 1,
        overflow: "hidden",
        display: "grid",
        gridTemplateColumns: GRID,
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
        [0, 1, 2, 3].map((i) => (
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

      {!isLoading && accounts.length === 0 && (
        <Box sx={{ gridColumn: "1 / -1", px: 2, py: 4, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            No accounts match the current filters.
          </Typography>
        </Box>
      )}

      {!isLoading &&
        accounts.map((a) => {
          const handleClick = () => navigate(`/accounts/${a.id}`);
          return (
            <Box
              key={a.id}
              role="button"
              tabIndex={0}
              aria-label={`${a.name} — open account`}
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
              <Typography variant="body2" noWrap>
                <strong>{a.name}</strong>
              </Typography>
              <TierChip tier={a.tier} />
              <StatusChip status={a.status} />
              <Typography variant="body2" sx={{ textAlign: "right" }}>
                {a.projectCount}
              </Typography>
              <Typography variant="body2" sx={{ textAlign: "right" }}>
                {a.openCaseCount}
              </Typography>
              <Typography
                variant="body2"
                color={a.s0s1Count > 0 ? "error" : "text.primary"}
                sx={{ textAlign: "right" }}
              >
                {a.s0s1Count}
              </Typography>
              <Typography
                variant="body2"
                color={a.breachedCount > 0 ? "error" : "text.primary"}
                sx={{ textAlign: "right" }}
              >
                {a.breachedCount}
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ textAlign: "right" }}
                noWrap
              >
                <RelativeTime iso={a.lastActivityAt} />
              </Typography>
            </Box>
          );
        })}
    </Box>
  );
}
