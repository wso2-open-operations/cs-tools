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

import { Box } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { BeDashboardWidget } from "@api/backend/types";
import WallboardStatTile, {
  type WallboardStatTileVariant,
} from "@features/csm-dashboard/components/WallboardStatTile";
import type { WallboardSection } from "@features/csm-dashboard/utils/wallboardMetricStyle";

export interface WallboardStatGridProps {
  widgets: BeDashboardWidget[];
  section: WallboardSection;
  columns: number;
  variant?: WallboardStatTileVariant;
  /** Grid gap (MUI spacing units). Defaults to `1` (8px). CRE's own
   * primary 2x2 grid passes a smaller value — reducing the gap between
   * columns is the only real lever for "wider tiles" in an already
   * full-width, fixed-column-count grid; the tiles can't exceed their
   * `1fr` share of the row without breaking the NxM shape. */
  gap?: number;
  /** Forwarded to each tile's own `valueLabelGap` (see
   * `WallboardStatTile`'s own doc comment) — CRE's primary grid passes a
   * larger value than the default. */
  valueLabelGap?: number;
  /** Forwarded to each tile's own `paddingY` (see `WallboardStatTile`'s
   * own doc comment) — CRE's primary grid passes a larger value than the
   * default. */
  paddingY?: number;
  selectedTeamCreGroupId?: string | string[];
  selectedTeamSreGroupId?: string | string[];
  selectedTeamLabel?: string;
}

/**
 * A plain N-column grid of glow-capable `WallboardStatTile`s — CRE's own
 * primary 2x2 grid, Security Report's 2x2 grid, and FDE's 3-column grid
 * are all this same shape in the original, just with a different `columns`
 * and widget set. SRE (four separately-labeled sub-rows) and CRE's
 * secondary tier (plain, non-glow tiles) are NOT this shape — see
 * `WallboardSreSection` / `WallboardCreSection` instead.
 */
export default function WallboardStatGrid({
  widgets,
  section,
  columns,
  variant = "primary",
  gap = 1,
  valueLabelGap,
  paddingY,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  selectedTeamLabel,
}: WallboardStatGridProps): JSX.Element {
  return (
    <Box
      sx={{
        display: "grid",
        gridTemplateColumns: `repeat(${columns}, 1fr)`,
        gap,
      }}
    >
      {widgets.map((widget) => (
        <WallboardStatTile
          key={widget.widgetId}
          widgetId={widget.widgetId}
          displayName={widget.displayName}
          resourceType={widget.resourceType}
          filters={widget.query}
          section={section}
          variant={variant}
          valueLabelGap={valueLabelGap}
          paddingY={paddingY}
          selectedTeamCreGroupId={selectedTeamCreGroupId}
          selectedTeamSreGroupId={selectedTeamSreGroupId}
          selectedTeamLabel={selectedTeamLabel}
        />
      ))}
    </Box>
  );
}
