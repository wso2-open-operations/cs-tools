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

import { Box, Divider, Typography } from "@wso2/oxygen-ui";
import { useQueryClient } from "@tanstack/react-query";
import { Fragment, useRef, useState, type JSX, type ReactNode } from "react";
import type { BeDashboardWidget } from "@api/backend/types";
import DashboardWidgetTile from "@features/csm-dashboard/components/DashboardWidgetTile";
import WidgetInlineDrilldownPanel from "@features/csm-dashboard/components/WidgetInlineDrilldownPanel";
import type { PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";
import RefreshButton from "@components/RefreshButton";
import { resolveWidgetText } from "@features/csm-dashboard/utils/widgetTextPlaceholder";
import { invalidateWidgetQueries } from "@features/csm-dashboard/utils/invalidateWidgetQueries";
import {
  WIDGET_GRID_SX,
  groupWidgetsBySection,
  type WidgetGroup,
} from "@features/csm-dashboard/utils/dashboardWidgetGridLayout";
import { resolveDateRangeFilterPlaceholder } from "@features/csm-dashboard/utils/dateRangeFilterPlaceholder";

// Hides the section refresh button + its "Last refreshed" label by default
// and reveals both together on hover/focus of an ancestor carrying this sx
// (see `sectionHeaderSx` below, applied to the row wide enough that hovering
// anywhere near the section title reveals the control). Kept inert (not
// clickable, not hit-testable) while hidden so it can't be triggered by a
// stray click that happens to land where it would render once visible, but
// stays reachable by keyboard Tab order throughout — `display: none` would
// remove it from the tab sequence entirely, which is why opacity +
// pointerEvents is used instead.
const hoverRevealSx = {
  opacity: 0,
  pointerEvents: "none",
  transition: "opacity 0.15s ease",
} as const;

const sectionHeaderSx = {
  display: "flex",
  alignItems: "center",
  gap: 1,
  "&:hover .dashboard-section-refresh, &:focus-within .dashboard-section-refresh": {
    opacity: 1,
    pointerEvents: "auto",
  },
} as const;

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

export interface DashboardWidgetGridProps {
  widgets: BeDashboardWidget[];
  /** The currently selected team's own `creGroupId` (see
   * `BeTeam.creGroupId`), or an array of every team's `creGroupId` in the
   * current dashboard's family when the "All ABTs" option is selected (see
   * `ALL_TEAMS_SENTINEL` in `teamFilterPlaceholder.ts`) — only meaningful
   * for an `isTeamBased` dashboard, threaded straight through to every tile
   * so each can resolve its own `__current_team__` filter placeholder for a
   * `creTeam` filter entry. `undefined` for a non-team-based dashboard, or
   * while the team isn't resolved yet. */
  selectedTeamCreGroupId?: string | string[];
  /** The currently selected team's own `sreGroupId` (see
   * `BeTeam.sreGroupId`), or an array of every team's `sreGroupId` in the
   * current dashboard's family when the "All ABTs" option is selected — the
   * `sreTeam`-filter counterpart of {@link selectedTeamCreGroupId}, resolved
   * independently. `undefined` in the same cases `selectedTeamCreGroupId`
   * is. */
  selectedTeamSreGroupId?: string | string[];
  /** Human-readable label for the selected team (its own display `name`,
   * or the literal `"All ABTs"`) — threaded down for each tile's own
   * `{{currentTeam}}` widget text placeholder (see
   * `widgetTextPlaceholder.ts`). `undefined` in the same cases
   * `selectedTeamCreGroupId` is. */
  selectedTeamLabel?: string;
  /** Per-widget action rendered as a small overlay on that widget's own
   * tile (e.g. the dashboard builder's "Edit widget" gear) — absent
   * renders every tile exactly as the live dashboard does, with no overlay
   * at all. Positioned by the caller; this component only decides where in
   * the DOM it renders (a positioned wrapper around the tile). */
  renderWidgetAction?: (widget: BeDashboardWidget) => ReactNode;
  /** Per-section actions rendered in that section's own header row,
   * alongside its refresh button (e.g. the builder's "Add widget to this
   * section" / "Remove section"). Receives the section's own RAW,
   * unresolved `widget.section` value (`undefined` for the untitled default
   * group) — the same identity `groupWidgetsBySection` groups by and the
   * draft's own `widget.section`/`emptySections` are keyed on — followed by
   * the display-resolved title (post `{{currentTeam}}` substitution, for
   * rendering only) and every widget id currently in the section. A caller
   * that uses the resolved title as an identity key instead of the raw one
   * splits a placeholder-named section in two the moment it's edited — see
   * `groupWidgetsBySection` in `dashboardWidgetGridLayout.ts`. */
  renderSectionActions?: (
    rawSection: string | undefined,
    resolvedSectionTitle: string | undefined,
    sectionWidgetIds: Set<string>,
  ) => ReactNode;
  /** Rendered once, after every existing section — e.g. the builder's own
   * "Add section" entry point, or an empty section shell that has no
   * widgets in it yet. */
  trailingContent?: ReactNode;
  /** The dashboard's own currently-selected date range (see
   * `DateRangeFilter`, owned by `AgentsLandingPagePilot`), resolved into
   * every widget's own `query`/base filters here — the single merge point
   * both a `shape: "list"` grid widget and a `shape: "bar"` trend widget
   * share, since both read their own base filters off the same
   * `widget.query` (see `renderTile`'s `filters` prop below). `undefined`
   * for "no range selected" (all time) or a dashboard with no widget that
   * references the placeholder at all (see `hasDateRangeFilterPlaceholder`),
   * in which case this is a no-op — every existing dashboard's widgets carry
   * no `__dateRangeFrom__`/`__dateRangeTo__` value, so nothing about them
   * changes. See `dateRangeFilterPlaceholder.ts`. */
  dateRangeFrom?: string;
  /** The `to` half of {@link dateRangeFrom}. */
  dateRangeTo?: string;
}

/**
 * The dashboard widget grid: groups `widgets` by `section` and renders one
 * `DashboardWidgetTile` per widget, each resolving its own data
 * independently. Extracted out of `AgentsLandingPagePilot` (the live
 * dashboard's own renderer) so the dashboard builder can render an
 * in-progress draft's widgets through the exact same component instead of a
 * forked copy — the only two things that differ between "live" and
 * "editing" are the optional `renderWidgetAction`/`renderSectionActions`/
 * `trailingContent` overlays, which are no-ops when omitted.
 */
export default function DashboardWidgetGrid({
  widgets,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  selectedTeamLabel,
  renderWidgetAction,
  renderSectionActions,
  trailingContent,
  dateRangeFrom,
  dateRangeTo,
}: DashboardWidgetGridProps): JSX.Element {
  const queryClient = useQueryClient();
  // Which widget's own slice (if any) is currently expanded into a
  // full-width inline-drilldown panel (see `WidgetInlineDrilldownPanel`,
  // rendered once per section — after that section's ENTIRE widget list,
  // not as a sibling inserted right after the specific widget that was
  // clicked — see `renderExpandedPanel`'s own doc comment for why) —
  // lifted up here, out of `DashboardWidgetTile`, specifically so the
  // expanded list can render at full grid width instead of nested inside
  // the tile's own (narrow, `gridWidth`-sized) `Card`. Singular by design:
  // only one widget's own
  // slice is ever expanded at a time, across the whole grid — expanding a
  // different widget's slice (or a different slice of the SAME widget)
  // replaces whatever was previously expanded, it does not add a second
  // panel. `null` means nothing is expanded anywhere in this grid.
  const [expanded, setExpanded] = useState<{ widgetId: string; slice: PieSliceResult } | null>(
    null,
  );
  // Per-section refresh tracks its own in-flight state, keyed by section.
  const [refreshingSections, setRefreshingSections] = useState<Set<string>>(new Set());
  // A section can bundle multiple widgets/queries, so there's no single
  // query's `dataUpdatedAt` to hand to that section's `RefreshButton` the
  // way single-widget call sites do — track our own "last refreshed" epoch
  // per section instead, set once `invalidateWidgets` below actually
  // resolves (its default `refetchType: "active"` means the promise only
  // resolves after the matched queries have refetched, not just been
  // marked stale).
  const [sectionLastRefreshedAt, setSectionLastRefreshedAt] = useState<Record<string, number>>({});

  // Per-widget-id caches so an UNRELATED tile's own props stay referentially
  // stable across an `expanded` state change elsewhere in the grid — without
  // this, `resolveDateRangeFilterPlaceholder` returning a fresh object and a
  // fresh `onExpandChange` closure on every render of this component would
  // give every tile new prop identities on every click anywhere on the
  // page, which is exactly what `DashboardWidgetTile`'s own `React.memo`
  // (see that component) needs to NOT be true in order to actually skip
  // re-rendering a widget nowhere near the one that was clicked. Both are
  // plain `useRef` maps (not `useMemo`) because they're populated from
  // inside `renderTile`, which itself runs inside a `.map()` over a
  // per-section widget list — calling a memoizing hook from inside that
  // loop would violate the rules of hooks (a variable number of hook calls
  // across renders whenever the widget count differs from render to
  // render); a manually-invalidated cache sidesteps that entirely.
  const resolvedFiltersCache = useRef(new Map<string, { key: string; value: Record<string, unknown> }>());
  const getResolvedFilters = (widget: BeDashboardWidget): Record<string, unknown> => {
    // `dateRangeFrom`/`dateRangeTo` are the only two things outside
    // `widget.query` itself that `resolveDateRangeFilterPlaceholder` reads —
    // both folded into the cache key so a date-range change still recomputes
    // (and so still reaches every widget that references the placeholder),
    // while an unrelated `expanded` change (which touches neither) reuses
    // the previous render's own object.
    const key = JSON.stringify([widget.query ?? {}, dateRangeFrom, dateRangeTo]);
    const cached = resolvedFiltersCache.current.get(widget.widgetId);
    if (cached && cached.key === key) return cached.value;
    const value = resolveDateRangeFilterPlaceholder(widget.query ?? {}, dateRangeFrom, dateRangeTo);
    resolvedFiltersCache.current.set(widget.widgetId, { key, value });
    return value;
  };
  // One stable `onExpandChange` closure per widget id, for the same reason
  // as `getResolvedFilters` above — `setExpanded` itself is guaranteed
  // referentially stable by React (a `useState` setter), so a closure
  // captured once per widget id and reused forever needs no dependency
  // array/invalidation of its own; it never has stale-closure risk because
  // it never reads any state directly, it only ever calls `setExpanded`
  // with an updater-free, fully-computed next value derived from its own
  // arguments.
  const onExpandChangeCache = useRef(new Map<string, (slice: PieSliceResult | null) => void>());
  const getOnExpandChange = (widgetId: string): ((slice: PieSliceResult | null) => void) => {
    let handler = onExpandChangeCache.current.get(widgetId);
    if (!handler) {
      handler = (slice) => setExpanded(slice ? { widgetId, slice } : null);
      onExpandChangeCache.current.set(widgetId, handler);
    }
    return handler;
  };

  const handleSectionRefresh = async (sectionKey: string, widgetIds: Set<string>): Promise<void> => {
    setRefreshingSections((prev) => new Set(prev).add(sectionKey));
    try {
      await invalidateWidgetQueries(queryClient, widgetIds);
      setSectionLastRefreshedAt((prev) => ({ ...prev, [sectionKey]: Date.now() }));
    } finally {
      setRefreshingSections((prev) => {
        const next = new Set(prev);
        next.delete(sectionKey);
        return next;
      });
    }
  };

  // Renders ONLY this widget's own tile — no longer also renders the
  // expanded inline-drilldown panel as an immediately-following sibling
  // (see `renderExpandedPanel` below for where that moved and why). Kept as
  // a plain `Box`, not a `Fragment`, now that there's only ever one grid
  // item per widget here: a `Fragment` wrapper existed only to let a widget
  // contribute two grid items (its own tile plus, conditionally, the panel
  // right after it) from one `.map()` call, which is exactly the layout bug
  // this fix removes.
  const renderTile = (widget: BeDashboardWidget) => {
    const action = renderWidgetAction?.(widget);
    const resolvedFilters = getResolvedFilters(widget);
    const thisWidgetExpandedSlice =
      expanded?.widgetId === widget.widgetId ? expanded.slice : null;
    return (
      <Box key={widget.widgetId} sx={{ position: "relative", ...widgetGridColumnSx(widget) }}>
        <DashboardWidgetTile
          widgetId={widget.widgetId}
          displayName={widget.displayName}
          description={widget.description}
          resourceType={widget.resourceType}
          shape={widget.shape}
          // `widget.query` is legally absent for a slices-only pie/bar
          // widget (see `BeDashboardWidget.query`'s doc comment) —
          // default to `{}` here too, at the source, on top of
          // `mergeWidgetFilters` and `useWidgetData`/`useWidgetPieData`
          // already tolerating it. `resolveDateRangeFilterPlaceholder` is
          // a no-op for every widget that doesn't carry
          // `__dateRangeFrom__`/`__dateRangeTo__` (every widget today
          // except `case_feedback`'s own two) — see that function's own
          // doc comment.
          filters={resolvedFilters}
          listLimit={widget.listLimit}
          slices={widget.slices}
          groupBy={widget.groupBy}
          columns={widget.columns}
          sortBy={widget.sortBy}
          inlineDrilldown={widget.inlineDrilldown}
          inlineLabels={widget.inlineLabels}
          selectedTeamCreGroupId={selectedTeamCreGroupId}
          selectedTeamSreGroupId={selectedTeamSreGroupId}
          selectedTeamLabel={selectedTeamLabel}
          // The builder action below renders as a sibling absolutely
          // positioned over this same top-right corner (at a higher
          // zIndex), fully covering the tile's own refresh button — so
          // suppress the tile's refresh button exactly when (and only
          // when) a builder action actually exists for this widget. See
          // `hideRefreshButton`'s own doc comment on `DashboardWidgetTile`.
          hideRefreshButton={Boolean(action)}
          expandedSlice={thisWidgetExpandedSlice}
          onExpandChange={getOnExpandChange(widget.widgetId)}
        />
        {action && (
          <Box sx={{ position: "absolute", top: 6, right: 6, zIndex: 2 }}>{action}</Box>
        )}
      </Box>
    );
  };

  // Renders the expanded inline-drilldown panel for `group` — but only if
  // `expanded`'s own widget actually belongs to THIS section, and only
  // ONCE, after every one of that section's own tiles rather than as a
  // sibling inserted immediately after the specific widget that was
  // clicked. That placement is the actual fix for the reported layout bug:
  // inserting a `gridColumn: "1 / -1"` full-width item mid-row (right after
  // whichever widget triggered it) forces CSS grid auto-placement to push
  // every OTHER widget still queued for that same row onto a new row below
  // it, even though none of them have anything to do with the click — the
  // grid's own auto-flow can't place a later item beside an earlier one
  // once a full-width item sits between them. Rendering the panel once,
  // after the section's entire widget list, means every other tile in that
  // section keeps its original row/position no matter which of the
  // section's own widgets was expanded — only one full-width row ever
  // appears, at the very end of that section.
  const renderExpandedPanel = (group: WidgetGroup): ReactNode => {
    if (!expanded) return null;
    const widget = group.widgets.find((w) => w.widgetId === expanded.widgetId);
    if (!widget) return null;
    return (
      <Box sx={{ gridColumn: "1 / -1" }}>
        <WidgetInlineDrilldownPanel
          widgetId={widget.widgetId}
          displayName={widget.displayName}
          resourceType={widget.resourceType}
          filters={getResolvedFilters(widget)}
          slice={expanded.slice}
          listLimit={widget.listLimit}
          columns={widget.columns}
          selectedTeamCreGroupId={selectedTeamCreGroupId}
          selectedTeamSreGroupId={selectedTeamSreGroupId}
          selectedTeamLabel={selectedTeamLabel}
          onClose={() => setExpanded(null)}
        />
      </Box>
    );
  };

  const groups = groupWidgetsBySection(widgets);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {groups.map((group, i) => {
        if (group.widgets.length === 0) return null;

        // Namespaced so a real section named e.g. `"__default_0"` can never
        // collide with the synthetic key generated for the unnamed section —
        // a collision there would cross-contaminate the two sections'
        // refresh state (`refreshingSections`, `sectionLastRefreshedAt`),
        // disabling/timestamping the wrong one. A constant (not `i`-based)
        // key for the unnamed case: `groupWidgetsBySection` collapses every
        // section-less widget into exactly one group, so there is never more
        // than one "unnamed" section to collide with itself — but an
        // index-based key broke when a named section was inserted/removed/
        // reordered ahead of it, since that shifts `i` for the same logical
        // group across renders, silently orphaning its in-flight
        // `refreshingSections` entry and `sectionLastRefreshedAt` value.
        const sectionKey = group.section != null ? `named:${group.section}` : "unnamed";
        const sectionWidgetIds = new Set(group.widgets.map((w) => w.widgetId));
        // Section titles support the same {{currentTeam}} text token as an
        // individual widget's own displayName/description (see
        // widgetTextPlaceholder.ts) — resolve it once here so both the
        // visible heading and the refresh button's label stay in sync.
        const resolvedSectionTitle = resolveWidgetText(group.section, selectedTeamLabel);

        return (
          <Fragment key={sectionKey}>
            {i > 0 && <Divider />}
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Box
                sx={{
                  ...sectionHeaderSx,
                  justifyContent: resolvedSectionTitle ? "space-between" : "flex-end",
                }}
              >
                {resolvedSectionTitle && (
                  <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                    {resolvedSectionTitle}
                  </Typography>
                )}
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  {renderSectionActions?.(group.section, resolvedSectionTitle, sectionWidgetIds)}
                  <Box className="dashboard-section-refresh" sx={hoverRevealSx}>
                    <RefreshButton
                      onRefresh={() => void handleSectionRefresh(sectionKey, sectionWidgetIds)}
                      isFetching={refreshingSections.has(sectionKey)}
                      updatedAt={sectionLastRefreshedAt[sectionKey]}
                      label={
                        resolvedSectionTitle
                          ? `Refresh ${resolvedSectionTitle}`
                          : "Refresh section"
                      }
                    />
                  </Box>
                </Box>
              </Box>
              {/* Rendered in the config's own array order — see this
                  component's doc comment on why the widget grouping/order
                  must follow `group.widgets` as-is, not a shape-based
                  split. The expanded inline-drilldown panel (if this
                  section owns the currently-expanded widget) renders once,
                  after every tile — see `renderExpandedPanel`'s own doc
                  comment for why. */}
              <Box sx={WIDGET_GRID_SX}>
                {group.widgets.map(renderTile)}
                {renderExpandedPanel(group)}
              </Box>
            </Box>
          </Fragment>
        );
      })}
      {trailingContent}
    </Box>
  );
}
