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
  LinearProgress,
  Skeleton,
  Tooltip,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import { Circle } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useNavigate } from "react-router";
import RelativeTime from "@components/RelativeTime";
import {
  ENGAGEMENT_DELIVERY_LABEL,
  ENGAGEMENT_HEALTH_COLOR,
  ENGAGEMENT_HEALTH_LABEL,
  ENGAGEMENT_STAGE_LABEL,
  ENGAGEMENT_STATE_COLOR,
  ENGAGEMENT_STATE_LABEL,
  ENGAGEMENT_TYPE_LABEL,
  formatDateOnly,
} from "@features/csm-engagements/utils/engagements";
import type { CsmEngagementRow } from "@features/csm-engagements/types/csmEngagements";

interface EngagementsListProps {
  engagements: CsmEngagementRow[];
  isLoading: boolean;
}

const HEADER_CELLS: { label: string; align?: "left" | "right" }[] = [
  { label: "Engagement" },
  { label: "Type" },
  { label: "State" },
  { label: "Customer" },
  { label: "Owner" },
  { label: "Health" },
  { label: "Timeline", align: "right" },
  { label: "Progress", align: "right" },
  { label: "Updated", align: "right" },
];

// 9 columns matching HEADER_CELLS in order.
const GRID =
  "minmax(280px, 2.4fr) minmax(140px, 1fr) auto minmax(160px, 1.1fr) minmax(140px, 1fr) auto minmax(140px, 1fr) minmax(140px, 1fr) auto";

export default function EngagementsList({
  engagements,
  isLoading,
}: EngagementsListProps): JSX.Element {
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

      {!isLoading && engagements.length === 0 && (
        <Box sx={{ gridColumn: "1 / -1", px: 2, py: 4, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            No engagements match the current filters.
          </Typography>
        </Box>
      )}

      {!isLoading &&
        engagements.map((e) => {
          const handleClick = (): void => {
            navigate(`/engagements/${e.id}`);
          };
          return (
            <Box
              key={e.id}
              role="button"
              tabIndex={0}
              onClick={handleClick}
              onKeyDown={(ev) => {
                if (ev.key === "Enter" || ev.key === " ") {
                  ev.preventDefault();
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
                <Typography variant="body2" noWrap sx={{ fontWeight: 500 }}>
                  <code style={{ fontSize: "0.78rem", marginRight: 6 }}>{e.reference}</code>
                  {e.name}
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap>
                  {e.projectName ? `${e.projectName} · ` : ""}
                  {ENGAGEMENT_DELIVERY_LABEL[e.deliveryMode]}
                  {" · "}
                  {ENGAGEMENT_STAGE_LABEL[e.stage]}
                </Typography>
              </Box>
              <Typography variant="body2" noWrap>
                {ENGAGEMENT_TYPE_LABEL[e.type]}
              </Typography>
              <Chip
                size="small"
                label={ENGAGEMENT_STATE_LABEL[e.state]}
                color={ENGAGEMENT_STATE_COLOR[e.state]}
                variant="outlined"
              />
              <Typography variant="body2" noWrap>
                {e.customer}
              </Typography>
              <Typography variant="body2" noWrap>
                {e.ownerIsMe ? <strong>{e.ownerName}</strong> : e.ownerName}
              </Typography>
              <Box sx={{ display: "flex", justifyContent: "center" }}>
                {e.health ? (
                  <Tooltip title={ENGAGEMENT_HEALTH_LABEL[e.health]}>
                    <Box
                      sx={{
                        display: "inline-flex",
                        color: `${ENGAGEMENT_HEALTH_COLOR[e.health]}.main`,
                      }}
                    >
                      <Circle size={12} fill="currentColor" />
                    </Box>
                  </Tooltip>
                ) : (
                  <Typography variant="caption" color="text.disabled">
                    —
                  </Typography>
                )}
              </Box>
              <Typography variant="caption" sx={{ textAlign: "right" }} noWrap>
                {formatDateOnly(e.plannedStartDate)} → {formatDateOnly(e.plannedEndDate)}
              </Typography>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, minWidth: 0 }}>
                <LinearProgress
                  variant="determinate"
                  value={e.progressPct}
                  sx={{ flex: 1, height: 6, borderRadius: 3 }}
                />
                <Typography variant="caption" sx={{ minWidth: 28, textAlign: "right" }}>
                  {e.progressPct}%
                </Typography>
              </Box>
              <Typography variant="caption" sx={{ textAlign: "right" }} noWrap>
                <RelativeTime iso={e.updatedAt} />
              </Typography>
            </Box>
          );
        })}
    </Box>
  );
}
