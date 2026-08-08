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

import { Box, Card, Divider, Skeleton, Typography } from "@wso2/oxygen-ui";
import { useQueryClient } from "@tanstack/react-query";
import { Fragment, useState, type JSX } from "react";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { BeDashboardWidget } from "@api/backend/types";
import { useDashboard } from "@features/csm-dashboard/api/useDashboard";
import DashboardWidgetTile from "@features/csm-dashboard/components/DashboardWidgetTile";
import SectionCard from "@features/csm-dashboard/components/SectionCard";
import RefreshButton from "@components/RefreshButton";
import { resolveWidgetText } from "@features/csm-dashboard/utils/widgetTextPlaceholder";

/** Placeholder tile count while the dashboard detail is in flight. */
const PILOT_TILE_COUNT = 3;

/** 12-column grid, matching each widget's own `gridWidth`; on very small
 * screens there's only room for 4 columns, so a wide widget there wraps to
 * (at most) one extra row rather than overflowing. */
const WIDGET_GRID_SX = {
  display: "grid",
  gap: 1.5,
  gridTemplateColumns: {
    xs: "repeat(4, minmax(0, 1fr))",
    sm: "repeat(12, minmax(0, 1fr))",
  },
} as const;

interface WidgetGroup {
  /** `undefined` for the untitled/default group — every widget with no
   * `section` set lands here, rendered exactly as before this field
   * existed (no heading). */
  section?: string;
  widgets: BeDashboardWidget[];
}

/** Groups widgets by `section`, preserving the order each distinct section
 * value (including the untitled default) first appears among `widgets` —
 * see `BeDashboardWidget.section`. */
function groupWidgetsBySection(widgets: BeDashboardWidget[]): WidgetGroup[] {
  const groups: WidgetGroup[] = [];
  const indexBySection = new Map<string | undefined, number>();
  for (const widget of widgets) {
    const key = widget.section || undefined;
    let index = indexBySection.get(key);
    if (index === undefined) {
      index = groups.length;
      indexBySection.set(key, index);
      groups.push({ section: key, widgets: [] });
    }
    groups[index].widgets.push(widget);
  }
  return groups;
}

function widgetGridColumnSx(widget: BeDashboardWidget) {
  // A list-shape widget renders a real table (4 rows, several columns) —
  // its configured `gridWidth` was sized for the old compact text list, so
  // it always spans the full row here regardless of that value.
  return widget.shape === "list"
    ? { gridColumn: "1 / -1" }
    : {
        gridColumn: {
          xs: `span ${Math.min(widget.gridWidth, 4)}`,
          sm: `span ${widget.gridWidth}`,
        },
      };
}

interface AgentsLandingPagePilotProps {
  /** Id of the dashboard to render (e.g. "agents_pilot"). */
  dashboardId: string;
  /** The currently selected team's own `groupId` (see `BeTeam.groupId`), or
   * an array of every team's `groupId` in the current dashboard's family
   * when the "All ABTs" option is selected (see `ALL_TEAMS_SENTINEL` in
   * `teamFilterPlaceholder.ts`) — only meaningful for an `isTeamBased`
   * dashboard, threaded straight through to every tile so each can resolve
   * its own `__current_team__` filter placeholder. `undefined` for a
   * non-team-based dashboard, or while the team isn't resolved yet. */
  selectedTeamGroupId?: string | string[];
  /** Human-readable label for the selected team (its own display `name`,
   * or the literal `"All ABTs"`) — threaded down for each tile's own
   * `{{currentTeam}}` widget text placeholder (see
   * `widgetTextPlaceholder.ts`). `undefined` in the same cases
   * `selectedTeamGroupId` is. */
  selectedTeamLabel?: string;
}

/**
 * Pilot section for the config-driven dashboard widget system: renders
 * whichever dashboard's real `single_score` widgets are passed in via
 * `dashboardId`. The dashboard's metadata plus its widget templates —
 * display metadata and each widget's filter criteria — are fetched once via
 * {@link useDashboard}; each rendered tile then resolves its own data
 * independently. Today only the "agents_pilot" dashboard has real widgets
 * (see CsmDashboardPage.tsx), but this component is generic over any
 * dashboard id with widgets.
 */
export default function AgentsLandingPagePilot({
  dashboardId,
  selectedTeamGroupId,
  selectedTeamLabel,
}: AgentsLandingPagePilotProps): JSX.Element {
  const queryClient = useQueryClient();
  const { data, isLoading, isError } = useDashboard(dashboardId);
  // Per-section refresh (below) tracks its own in-flight state, keyed by
  // section, independently of the dashboard-metadata refresh above — a
  // section refresh never touches `useDashboard`'s own refetch (config is
  // not re-pulled), only that section's own widgets' data queries.
  const [refreshingSections, setRefreshingSections] = useState<Set<string>>(new Set());

  /**
   * Invalidates only the widget-data queries belonging to `widgetIds` —
   * both shapes' query keys carry a widget id, just at a different
   * position: `[KEY, widgetId, ...]` for count/list (see `useWidgetData`),
   * `[KEY, "pie-slice", widgetId, ...]` for pie/bar (see
   * `useWidgetPieData`). Never touches `useDashboard`'s own metadata query.
   */
  const invalidateWidgets = (widgetIds: Set<string>): Promise<void> =>
    queryClient.invalidateQueries({
      predicate: (query) => {
        const key = query.queryKey;
        if (key[0] !== ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA) return false;
        const widgetId = key[1] === "pie-slice" ? key[2] : key[1];
        return typeof widgetId === "string" && widgetIds.has(widgetId);
      },
    });

  const handleSectionRefresh = async (sectionKey: string, widgetIds: Set<string>): Promise<void> => {
    setRefreshingSections((prev) => new Set(prev).add(sectionKey));
    try {
      await invalidateWidgets(widgetIds);
    } finally {
      setRefreshingSections((prev) => {
        const next = new Set(prev);
        next.delete(sectionKey);
        return next;
      });
    }
  };

  return (
    <SectionCard>
      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load the widget pilot.
        </Typography>
      ) : isLoading ? (
        <Box sx={WIDGET_GRID_SX}>
          {Array.from({ length: PILOT_TILE_COUNT }, (_, i) => (
            <Card key={i} variant="outlined" sx={{ p: 1.75, gridColumn: "span 4" }}>
              <Skeleton variant="rounded" height={48} />
            </Card>
          ))}
        </Box>
      ) : (
        (() => {
          const widgets = data?.widgets ?? [];
          const renderTile = (widget: BeDashboardWidget) => {
            return (
              <Box key={widget.widgetId} sx={widgetGridColumnSx(widget)}>
                <DashboardWidgetTile
                  widgetId={widget.widgetId}
                  displayName={widget.displayName}
                  description={widget.description}
                  resourceType={widget.resourceType}
                  shape={widget.shape}
                  filters={widget.query}
                  listLimit={widget.listLimit}
                  slices={widget.slices}
                  columns={widget.columns}
                  sortBy={widget.sortBy}
                  selectedTeamGroupId={selectedTeamGroupId}
                  selectedTeamLabel={selectedTeamLabel}
                />
              </Box>
            );
          };

          const groups = groupWidgetsBySection(widgets);

          return (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
              {groups.map((group, i) => {
                // Chart-shaped widgets (pie/bar) are visually grouped into
                // their own row below this group's count/list tiles, rather
                // than sharing one undifferentiated grid with them.
                const mainWidgets = group.widgets.filter((w) => w.shape !== "pie" && w.shape !== "bar");
                const chartWidgets = group.widgets.filter((w) => w.shape === "pie" || w.shape === "bar");
                if (mainWidgets.length === 0 && chartWidgets.length === 0) return null;

                const sectionKey = group.section ?? `__default_${i}`;
                const sectionWidgetIds = new Set(group.widgets.map((w) => w.widgetId));
                // Section titles support the same {{currentTeam}} text token as
                // an individual widget's own displayName/description (see
                // widgetTextPlaceholder.ts) — resolve it once here so both the
                // visible heading and the refresh button's label stay in sync.
                const resolvedSectionTitle = resolveWidgetText(group.section, selectedTeamLabel);

                return (
                  <Fragment key={sectionKey}>
                    {i > 0 && <Divider />}
                    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
                      <Box
                        sx={{
                          display: "flex",
                          alignItems: "center",
                          justifyContent: resolvedSectionTitle ? "space-between" : "flex-end",
                        }}
                      >
                        {resolvedSectionTitle && (
                          <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                            {resolvedSectionTitle}
                          </Typography>
                        )}
                        <RefreshButton
                          onRefresh={() => void handleSectionRefresh(sectionKey, sectionWidgetIds)}
                          isFetching={refreshingSections.has(sectionKey)}
                          label={
                            resolvedSectionTitle
                              ? `Refresh ${resolvedSectionTitle}`
                              : "Refresh section"
                          }
                        />
                      </Box>
                      {mainWidgets.length > 0 && (
                        <Box sx={WIDGET_GRID_SX}>{mainWidgets.map(renderTile)}</Box>
                      )}
                      {chartWidgets.length > 0 && (
                        <>
                          {mainWidgets.length > 0 && <Divider sx={{ my: 0.5 }} />}
                          <Box sx={WIDGET_GRID_SX}>{chartWidgets.map(renderTile)}</Box>
                        </>
                      )}
                    </Box>
                  </Fragment>
                );
              })}
            </Box>
          );
        })()
      )}
    </SectionCard>
  );
}
