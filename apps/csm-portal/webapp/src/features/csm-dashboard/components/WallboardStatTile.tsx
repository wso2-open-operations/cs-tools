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
import {
  getStatTileColors,
  WALLBOARD_FONT_SIZE,
  type WallboardSection,
} from "@features/csm-dashboard/utils/wallboardMetricStyle";

// Plain-tile (no emphasis) colors — Tailwind's own `bg-gray-700/50` /
// `border-gray-600` / `text-slate-300` label / white value, the original
// StatCard's own fallback for `hasAlert === false`.
const PLAIN_BG = "rgba(55,65,81,0.5)";
const PLAIN_BORDER = "#4b5563";
const PLAIN_LABEL = "#cbd5e1";

/** The original's two `StatCard` variants: "primary" for CRE/Security/FDE's
 * own grid cards, "sre" for SRE's own row-packed sub-section cards. Only
 * controls padding now (`p:` below) — value/label text size is the same
 * for both variants, by explicit request. */
export type WallboardStatTileVariant = "primary" | "sre";

export interface WallboardStatTileProps {
  widgetId: string;
  displayName: string;
  resourceType: BeWidgetResourceType;
  filters: Record<string, unknown>;
  /** See `UseWidgetDataOptions.distinctBy`. */
  distinctBy?: BeDashboardDistinctByConfig;
  /** Which of the four dashboard sections this tile belongs to — the color
   * lookup is keyed by (section, displayName), not displayName alone; see
   * `wallboardMetricStyle.ts` for why. */
  section: WallboardSection;
  variant?: WallboardStatTileVariant;
  /** Vertical gap (MUI spacing units) between the big value number and its
   * label underneath. Defaults to `0.6`; CRE's own primary tier passes a
   * larger value, by explicit request. */
  valueLabelGap?: number;
  /** Vertical (top/bottom) padding inside the tile — the space between the
   * tile's own edge and the value/label block, NOT the gap between the
   * value and label themselves (that's `valueLabelGap` above). Defaults to
   * the variant's previous fixed value (1.5 for "primary", 1 for "sre")
   * when omitted; CRE's own primary tier passes a larger value, by
   * explicit request. */
  paddingY?: number;
  selectedTeamCreGroupId?: string | string[];
  selectedTeamSreGroupId?: string | string[];
  selectedTeamLabel?: string;
}

/**
 * One glow-capable CS Overview stat card (CRE's primary tier, every SRE
 * card, every Security card, every FDE card). Fetches its own `shape:
 * "count"` widget data through the same `useWidgetData` hook
 * `DashboardWidgetTile` already uses — this is a different skin over the
 * identical, existing data path, not a new data source.
 */
export default function WallboardStatTile({
  widgetId,
  displayName,
  resourceType,
  filters,
  distinctBy,
  section,
  variant = "primary",
  valueLabelGap = 0.6,
  paddingY,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  selectedTeamLabel,
}: WallboardStatTileProps): JSX.Element {
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

  const colors = getStatTileColors(section, displayName);
  const hasAlert = colors !== undefined && total > 0;

  // Value/label text size is now the same for every variant (by explicit
  // request) — only padding still differs between "primary" and "sre"
  // (see `px`/`py` below), not the text itself.
  const valueFontSize = WALLBOARD_FONT_SIZE.value;
  const labelFontSize = WALLBOARD_FONT_SIZE.label;
  const defaultPaddingY = variant === "sre" ? 1 : 1.5;
  const resolvedPaddingY = paddingY ?? defaultPaddingY;

  const tileBody = (
    <Box
      data-alert={hasAlert ? "true" : undefined}
      sx={{
        flex: 1,
        borderRadius: "12px",
        px: variant === "sre" ? 1 : 1.5,
        py: resolvedPaddingY,
        height: "100%",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        textAlign: "center",
        border: "1px solid",
        borderColor: hasAlert ? colors.border : PLAIN_BORDER,
        bgcolor: hasAlert ? colors.bg : PLAIN_BG,
        transition: "border-color 0.2s ease, background-color 0.2s ease",
        // Layered like the original's StatCard: a faint white sheen on every
        // tile, plus — on an alert tile — a colored gradient that blinks
        // (the `::after` below). The tile itself never fades; only that
        // colored layer does, so the tile stays fully saturated.
        position: "relative",
        overflow: "hidden",
        boxShadow: "0 1px 2px rgba(0,0,0,0.05)",
        "&::before": {
          content: '""',
          position: "absolute",
          inset: 0,
          background: "linear-gradient(to bottom right, rgba(255,255,255,0.05), transparent)",
          pointerEvents: "none",
        },
        ...(hasAlert
          ? {
              // Tailwind's own `animate-pulse`, the original's exact blink.
              "@keyframes wallboard-pulse": {
                "0%, 100%": { opacity: 1 },
                "50%": { opacity: 0.5 },
              },
              "&::after": {
                content: '""',
                position: "absolute",
                inset: 0,
                background: colors.pulse,
                pointerEvents: "none",
                animation: "wallboard-pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite",
              },
            }
          : {}),
        // Value and label sit above both layers.
        "& > *": { position: "relative", zIndex: 1 },
      }}
    >
      {state === "loading" ? (
        <Skeleton
          variant="rounded"
          width="60%"
          // Same height as the number it stands in for (`lineHeight: 1.1`).
          sx={{ bgcolor: "rgba(255,255,255,0.08)", height: `calc(${valueFontSize} * 1.1)` }}
        />
      ) : state === "error" ? (
        <Typography variant="caption" sx={{ color: "rgba(255,255,255,0.5)" }}>
          —
        </Typography>
      ) : (
        <Typography
          sx={{
            // font-black/font-bold, as the original.
            fontWeight: hasAlert ? 900 : 700,
            fontSize: valueFontSize,
            lineHeight: 1.1,
            color: hasAlert ? colors.value : "#fff",
            // The original's `drop-shadow-[0_0_8px_…]` glow.
            filter: hasAlert ? `drop-shadow(0 0 8px ${colors.shadow})` : "none",
          }}
        >
          {total.toLocaleString()}
        </Typography>
      )}
      <Typography
        sx={{
          mt: valueLabelGap,
          fontSize: labelFontSize,
          fontWeight: 700,
          letterSpacing: "0.05em",
          textTransform: "uppercase",
          lineHeight: 1.2,
          color: hasAlert ? colors.label : PLAIN_LABEL,
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
