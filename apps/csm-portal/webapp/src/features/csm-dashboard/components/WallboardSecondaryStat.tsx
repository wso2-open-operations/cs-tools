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

import { Box, Skeleton, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import type { BeDashboardDistinctByConfig, BeWidgetResourceType } from "@api/backend/types";
import { useWallboardTileData } from "@features/csm-dashboard/api/useWallboardTileData";
import { WALLBOARD_FONT_SIZE, wb } from "@features/csm-dashboard/utils/wallboardMetricStyle";

export interface WallboardSecondaryStatProps {
  widgetId: string;
  displayName: string;
  resourceType: BeWidgetResourceType;
  filters: Record<string, unknown>;
  /** See `UseWidgetDataOptions.distinctBy`. */
  distinctBy?: BeDashboardDistinctByConfig;
  selectedTeamCreGroupId?: string | string[];
  selectedTeamSreGroupId?: string | string[];
  selectedTeamLabel?: string;
}

/**
 * One CRE "secondary tier" stat card — the 3-column row below CRE's
 * primary 2x2 grid. Ported from the reference wallboard's `SecondaryStat`, which
 * (unlike `StatCard`) has no `alertType`/glow concept at all: always plain
 * white value text on a neutral gray tile, regardless of count.
 * Deliberately a separate component from `WallboardStatTile` rather than
 * that component with emphasis disabled — this tier's tile is visually
 * distinct in the original (different border-radius and background
 * opacity), not just "the same tile with no color." Its value/label text
 * sizes DO match `WallboardStatTile`'s own "primary" variant now (by
 * explicit request) — only the tile chrome around the text stays smaller,
 * not the text itself.
 */
export default function WallboardSecondaryStat({
  widgetId,
  displayName,
  resourceType,
  filters,
  distinctBy,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  selectedTeamLabel,
}: WallboardSecondaryStatProps): JSX.Element {
  const { total, resolvedDisplayName, state, linkHref } = useWallboardTileData({
    widgetId,
    displayName,
    resourceType,
    filters,
    distinctBy,
    selectedTeamCreGroupId,
    selectedTeamSreGroupId,
    selectedTeamLabel,
  });

  const tileBody = (
    <Box
      sx={{
        flex: 1,
        borderRadius: "8px",
        p: wb(6),
        height: "100%",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        textAlign: "center",
        border: "1px solid #4b5563",
        bgcolor: "rgba(55,65,81,0.6)",
      }}
    >
      {state === "loading" ? (
        // Same height as the number it stands in for — the same value size
        // and `lineHeight: 1.1` as WallboardStatTile's own skeleton.
        <Skeleton
          variant="rounded"
          width="60%"
          sx={{ bgcolor: "rgba(255,255,255,0.08)", height: `calc(${WALLBOARD_FONT_SIZE.value} * 1.1)` }}
        />
      ) : state === "error" ? (
        <Typography variant="caption" sx={{ color: "rgba(255,255,255,0.5)" }}>
          —
        </Typography>
      ) : (
        <Typography sx={{ fontWeight: 700, fontSize: WALLBOARD_FONT_SIZE.value, lineHeight: 1.1, color: "#fff" }}>
          {total.toLocaleString()}
        </Typography>
      )}
      <Typography
        sx={{
          mt: wb(3.2),
          fontSize: WALLBOARD_FONT_SIZE.label,
          fontWeight: 700,
          letterSpacing: "0.04em",
          textTransform: "uppercase",
          lineHeight: 1.15,
          color: "#cbd5e1",
        }}
      >
        {resolvedDisplayName}
      </Typography>
    </Box>
  );

  // `linkHref` is already `undefined` while loading / errored / awaiting the
  // current user (see `useWallboardTileData`), so a skeleton or dash is
  // never wrapped in a navigating link.
  if (!linkHref) return tileBody;

  return (
    <Box
      component={RouterLink}
      to={linkHref}
      sx={{ display: "flex", flex: 1, textDecoration: "none", color: "inherit", height: "100%" }}
    >
      {tileBody}
    </Box>
  );
}
