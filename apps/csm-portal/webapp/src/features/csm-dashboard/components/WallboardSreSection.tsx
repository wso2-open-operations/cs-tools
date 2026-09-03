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

import { Box, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { BeDashboardWidget } from "@api/backend/types";
import WallboardStatTile from "@features/csm-dashboard/components/WallboardStatTile";
import {
  sortByFixedOrder,
  SRE_SUBROW_ORDER,
  SRE_SUBSECTION_ORDER,
  sreSubsectionFor,
} from "@features/csm-dashboard/utils/wallboardMetricStyle";

export interface WallboardSreSectionProps {
  widgets: BeDashboardWidget[];
  selectedTeamCreGroupId?: string | string[];
  selectedTeamSreGroupId?: string | string[];
  selectedTeamLabel?: string;
}

/**
 * SRE's own layout: four separately-labeled sub-rows (Incidents / Problems
 * / Change Requests / Service Requests), each a flex row of glow-capable
 * tiles — matching the original's four `<div>` blocks, each with its own
 * small uppercase heading above a row of `StatCard`s. A sub-row with no
 * widgets in it (a dashboard that doesn't configure, say, any Problems
 * widgets) is skipped entirely rather than rendered empty.
 */
export default function WallboardSreSection({
  widgets,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  selectedTeamLabel,
}: WallboardSreSectionProps): JSX.Element {
  const groups = new Map<string, BeDashboardWidget[]>();
  for (const widget of widgets) {
    const subsection = sreSubsectionFor(widget.resourceType);
    const group = groups.get(subsection);
    if (group) group.push(widget);
    else groups.set(subsection, [widget]);
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1, flex: 1, minHeight: 0 }}>
      {SRE_SUBSECTION_ORDER.map((subsection) => {
        const rawRowWidgets = groups.get(subsection);
        if (!rawRowWidgets || rawRowWidgets.length === 0) return null;
        const rowWidgets = sortByFixedOrder(rawRowWidgets, SRE_SUBROW_ORDER[subsection]);
        return (
          <Box key={subsection} sx={{ display: "flex", flexDirection: "column", gap: 0.5, flex: 1, minHeight: 0 }}>
            <Typography
              sx={{
                fontSize: "0.55rem",
                fontWeight: 700,
                textTransform: "uppercase",
                letterSpacing: "0.05em",
                color: "#34d399",
                borderBottom: "1px solid rgba(4,120,87,0.5)",
                pb: 0.25,
              }}
            >
              {subsection}
            </Typography>
            <Box sx={{ display: "flex", gap: 0.75, flex: 1, minHeight: 0 }}>
              {rowWidgets.map((widget) => (
                <WallboardStatTile
                  key={widget.widgetId}
                  widgetId={widget.widgetId}
                  displayName={widget.displayName}
                  resourceType={widget.resourceType}
                  filters={widget.query}
                  section="sre"
                  variant="sre"
                  selectedTeamCreGroupId={selectedTeamCreGroupId}
                  selectedTeamSreGroupId={selectedTeamSreGroupId}
                  selectedTeamLabel={selectedTeamLabel}
                />
              ))}
            </Box>
          </Box>
        );
      })}
    </Box>
  );
}
