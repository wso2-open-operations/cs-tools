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

import { Box, Button, Card, IconButton, Skeleton, Tooltip, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { ArrowRight, Info } from "@wso2/oxygen-ui-icons-react";
import type { JSX, KeyboardEvent, ReactNode } from "react";
import { Link as RouterLink, useLocation, useNavigate } from "react-router";
import type {
  BeDashboardPieSlice,
  BeDashboardWidgetColumn,
  BeWidgetResourceType,
  BeWidgetShape,
} from "@api/backend/types";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useWidgetData } from "@features/csm-dashboard/api/useWidgetData";
import { useWidgetPieData, type PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import { WIDGET_LIST_RENDERERS } from "@features/csm-dashboard/config/widgetListConfig";
import GenericColumnList from "@features/csm-dashboard/components/GenericColumnList";
import { buildWidgetPreviewHref } from "@features/csm-dashboard/utils/widgetPreviewUrl";
import { mergeWidgetFilters } from "@features/csm-dashboard/utils/widgetFilterMerge";
import { resolveTeamPlaceholder } from "@features/csm-dashboard/utils/teamFilterPlaceholder";
import { resolveCurrentUserPlaceholder } from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";
import { resolveWidgetText } from "@features/csm-dashboard/utils/widgetTextPlaceholder";
import DashboardPieChart from "@features/csm-dashboard/components/DashboardPieChart";
import DashboardBarChart from "@features/csm-dashboard/components/DashboardBarChart";

interface DashboardWidgetTileProps {
  widgetId: string;
  displayName: string;
  /** Explanatory copy for this widget (`WidgetTemplate.Description`).
   * Rendered as an inline subtitle under `displayName` for shape "pie"/"bar"
   * (there's room), and as a keyboard-accessible info-icon tooltip for
   * shape "count" (there isn't) — either way, only when actually set; an
   * empty/absent one renders neither. Not surfaced for shape "list" (its own
   * table already fills the space this would otherwise sit in). */
  description?: string;
  resourceType: BeWidgetResourceType;
  shape: BeWidgetShape;
  filters: Record<string, unknown>;
  /** Only meaningful for shape "list"; how many rows to render. Defaults to
   * 4 (see useWidgetData's DEFAULT_LIST_LIMIT) — set explicitly per-widget
   * via the backend's DASHBOARDS_CONFIG, not overridden here. */
  listLimit?: number;
  /** Only meaningful for shape "pie"/"bar"; one search per slice (see
   * `useWidgetPieData`). Empty/absent renders an empty chart rather than
   * crashing. */
  slices?: BeDashboardPieSlice[];
  /** Only meaningful for shape "list". When set (non-empty), this widget
   * renders through the generic `GenericColumnList` (resolving each
   * column's dot-path against every response item) instead of
   * `WIDGET_LIST_RENDERERS[resourceType]`. Absent/empty is a no-op — the
   * existing hardcoded per-resourceType renderer applies exactly as before
   * this prop existed. */
  columns?: BeDashboardWidgetColumn[];
  /** Only meaningful for shape "list". Forwarded verbatim into this
   * widget's own search request's `sortBy` (see `useWidgetData`) —
   * opaque, like `filters`. */
  sortBy?: Record<string, unknown>;
  /** The currently selected team's own `groupId` (see `BeTeam.groupId`), or
   * an array of every team's `groupId` in the current dashboard's family
   * when the "All ABTs" option is selected (see `ALL_TEAMS_SENTINEL`),
   * threaded down from `CsmDashboardPage` for resolving this widget's own
   * `__current_team__` filter placeholder (see `teamFilterPlaceholder.ts`)
   * — never the team registry key. `undefined` for a non-team-based
   * dashboard, or while the team isn't resolved yet (any `integrationCsTeam`
   * filter entry carrying the placeholder is then dropped, not sent
   * literally). */
  selectedTeamGroupId?: string | string[];
  /** Human-readable label for the selected team (its own display `name`, or
   * the literal `"All ABTs"`) — used to resolve the `{{currentTeam}}` text
   * placeholder (see `widgetTextPlaceholder.ts`) inside `displayName`/
   * `description` before render. `undefined` in the same cases
   * `selectedTeamGroupId` is (the token is then stripped rather than left
   * literally visible). */
  selectedTeamLabel?: string;
}

/**
 * Single dashboard widget tile: fetches and renders its own data
 * independently of any sibling tile, so one widget's loading/error state
 * never affects another's. Renders a big number for `shape: "count"`; for
 * `shape: "list"` renders that resource type's own real table (see
 * `widgetListConfig.tsx` — e.g. cases render through the same `CasesList`
 * the Cases tab itself uses), capped at `listLimit`, with a "View more" link
 * to that widget's own preview page (`DashboardWidgetPreviewPage`, more rows,
 * same table) — not directly to the resource's own tab (that's "View all",
 * one hop further, via `widgetResourceConfig.ts`'s `buildHref`).
 *
 * `shape: "count"` tiles are themselves one big link straight to the
 * resource's own tab; `shape: "list"` tiles can't be (their rows and "View
 * more" need their own nested links), so only they get a plain, non-link
 * `Card`.
 */
export default function DashboardWidgetTile({
  widgetId,
  displayName,
  description,
  resourceType,
  shape,
  filters,
  listLimit,
  slices,
  columns,
  sortBy,
  selectedTeamGroupId,
  selectedTeamLabel,
}: DashboardWidgetTileProps): JSX.Element {
  const theme = useTheme();
  const navigate = useNavigate();
  const location = useLocation();
  const { user } = useCurrentUser();
  const currentUserId = user?.id;
  // Resolves both client-side filter placeholders a widget's own (opaque)
  // filters may carry — `__current_team__` (see `teamFilterPlaceholder.ts`)
  // and `__current_user__` (see `currentUserFilterPlaceholder.ts`) — in one
  // pass, so every click-through href below (and the two data-fetching hooks,
  // which resolve internally the same way) never sends either placeholder
  // literally.
  const resolvePlaceholders = (f: Record<string, unknown>): Record<string, unknown> =>
    resolveCurrentUserPlaceholder(resolveTeamPlaceholder(f, selectedTeamGroupId), currentUserId);
  // Carried on every count/pie/bar click-through below so the resource's own
  // list page (which has no dashboard context of its own) can offer a Back
  // button straight to this exact dashboard — mirroring the list-shape
  // widget, whose embedded list already sets this same `from` shape (see
  // CasesList) because it lives directly on this page.
  const dashboardReturnState = { from: `${location.pathname}${location.search}` };
  // Resolve the `{{currentTeam}}` text placeholder before anything below
  // renders/reads `displayName`/`description` — every other use of those two
  // props in this component reads the resolved value, never the raw one, so
  // the token never leaks into the UI (or the "View more"/preview-page href,
  // which carries `displayName` verbatim — see `buildWidgetPreviewHref`).
  const resolvedDisplayName = resolveWidgetText(displayName, selectedTeamLabel) ?? displayName;
  const resolvedDescription = resolveWidgetText(description, selectedTeamLabel);
  // Shape "pie" resolves via useWidgetPieData instead (one search per
  // slice) — skip this one's own network call rather than wasting it, but
  // still call the hook unconditionally (rules of hooks; a widget's shape
  // never changes across this component's lifetime).
  const { data, isLoading, isError } = useWidgetData(
    widgetId,
    resourceType,
    filters,
    shape,
    listLimit,
    0,
    shape !== "pie" && shape !== "bar",
    selectedTeamGroupId,
    sortBy,
    currentUserId,
  );
  const pieData = useWidgetPieData(
    widgetId,
    resourceType,
    filters,
    shape === "pie" || shape === "bar" ? (slices ?? []) : [],
    selectedTeamGroupId,
    currentUserId,
  );
  const config = WIDGET_RESOURCE_CONFIG[resourceType];
  // Thousands separators for shape "count"'s big number -- used both in the
  // visible Typography and the tile's aria-label, so both stay in sync.
  const formattedCount = (data?.total ?? 0).toLocaleString();

  if (!config) {
    // resourceType came from a runtime-configurable backend registry (not a
    // compile-time-checked Go literal) — an unrecognized value must not
    // crash this tile's render (config.buildHref below would throw).
    return (
      <Card variant="outlined" sx={{ p: 1.75 }}>
        <Typography variant="caption" color="text.secondary">
          {resolvedDisplayName}
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          Unsupported widget type.
        </Typography>
      </Card>
    );
  }

  // The count-shape tile's own click-through href — resolved so a "View
  // all" link never carries a literal `__current_team__`/`__current_user__`
  // placeholder into the destination resource's own filters (see
  // `teamFilterPlaceholder.ts`/`currentUserFilterPlaceholder.ts`).
  const href = config.buildHref(resolvePlaceholders(filters));
  const Icon = config.icon;
  const isListShape = shape === "list";

  // Shared hover idiom for every clickable tile (the count-shape anchor and
  // the pie/bar tile-level click-through target below) — matches the
  // customer-portal app's own clickable summary-card hover (see
  // `UpdateCardBreakdown.tsx`): a border/background tint plus a small lift,
  // no `boxShadow`. `border` is transparent at rest so the accent color only
  // shows up on hover, without shifting layout (`border-box` sizing keeps
  // the box the same size whether the border is drawn transparent or
  // colored). One shared object so shape "count" and shape "pie"/"bar"
  // never drift into two different hover looks.
  const widgetHoverSx = {
    border: "1px solid transparent",
    transition: "border-color 0.2s ease, background-color 0.2s ease, transform 0.15s ease",
    "&:hover": {
      borderColor: theme.palette.primary.main,
      bgcolor: alpha(theme.palette.primary.main, 0.06),
      transform: "translateY(-1px)",
    },
  } as const;

  // Count tiles only — a list-shape tile's real table already has its own
  // header row and border right where this would otherwise sit, and a
  // pie/bar tile already shows its own `description` as an inline subtitle
  // (see the pie/bar branch below), so an icon there would just duplicate
  // it. Renders only when this widget actually has a `description` to show
  // — an empty/absent one means there's nothing to disclose.
  const infoIcon = shape === "count" && resolvedDescription && (
    <Tooltip title={resolvedDescription}>
      <IconButton
        size="small"
        aria-label={`About ${resolvedDisplayName}`}
        sx={{
          position: "absolute",
          top: 8,
          right: 8,
          zIndex: 1,
          color: "text.secondary",
        }}
      >
        <Info size={13} />
      </IconButton>
    </Tooltip>
  );

  const header = (
    <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.25 }}>
      <Box
        sx={{
          p: 0.75,
          mt: 0.25,
          borderRadius: "50%",
          bgcolor: alpha(theme.palette[config.iconColor].light, 0.1),
          color: theme.palette[config.iconColor].light,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          flexShrink: 0,
        }}
      >
        <Icon size={16} />
      </Box>
      <Typography variant="caption" color="text.secondary" sx={{ mt: 0.5 }}>
        {resolvedDisplayName}
      </Typography>
    </Box>
  );

  if (isListShape) {
    // A widget with `columns` configured renders through the generic,
    // nested-path-resolving renderer instead of the hardcoded per-
    // resourceType one — everything else about this tile (header, loading/
    // error states, "View more") is unchanged either way. No `columns`
    // (the common case, and every dashboard as of this field's addition) is
    // a no-op: ListRenderer below is untouched, so existing widgets render
    // byte-for-byte as before.
    const hasColumns = Boolean(columns && columns.length > 0);
    const ListRenderer = WIDGET_LIST_RENDERERS[resourceType];
    return (
      <Card variant="outlined" sx={{ position: "relative", p: 1.75, height: "100%" }}>
        {header}
        {isLoading ? (
          <Skeleton variant="rounded" height={28 * (listLimit ?? 4) + 40} sx={{ mt: 1 }} />
        ) : isError ? (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
            Could not load this widget.
          </Typography>
        ) : (
          <>
            {/* CasesList/TimeCardsTable (case, time_card) carry no margin of
                their own — this is the one place spacing between the header
                and the table is enforced for every resource type, so the
                header's icon never sits flush against the table's border. */}
            <Box sx={{ mt: 1.5 }}>
              {hasColumns ? (
                <GenericColumnList
                  items={data?.items ?? []}
                  isLoading={false}
                  resourceType={resourceType}
                  columns={columns ?? []}
                />
              ) : (
                <ListRenderer items={data?.items ?? []} isLoading={false} />
              )}
            </Box>
            {(data?.total ?? 0) > (listLimit ?? 4) && (
              <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 1 }}>
                <Button
                  component={RouterLink}
                  to={buildWidgetPreviewHref({
                    previewSlug: config.previewSlug,
                    widgetId,
                    displayName: resolvedDisplayName,
                    // Resolved the same way the count-shape tile's own
                    // click-through href is (see `href` above) — the preview
                    // page has no team context of its own, so an unresolved
                    // placeholder here used to get silently dropped there
                    // (teamFilterPlaceholder.ts's fail-open), returning
                    // every team's cases instead of the viewer's own.
                    filters: resolvePlaceholders(filters),
                    currentUserId,
                  })}
                  size="small"
                  variant="text"
                  endIcon={<ArrowRight size={14} />}
                >
                  View more
                </Button>
              </Box>
            )}
          </>
        )}
      </Card>
    );
  }

  if (shape === "pie" || shape === "bar") {
    // Each slice/legend row (or bar) navigates independently to that
    // slice's OWN filtered list, so — same rationale as shape "list" — this
    // can't be one big link the way shape "count" is. It's still a
    // click-through target in its own right, though: clicking the tile
    // anywhere OTHER than a slice/legend row (the header, the padding, the
    // empty state) goes to the widget's own base filters, the same
    // destination a "count" tile with those filters would produce.
    //
    // That tile-level target keeps its role="button"/Enter+Space keyboard
    // handling (a real <a> only activates on Enter, not Space, and this
    // control has always supported both) -- but it is a SIBLING of the
    // chart, not its ancestor: an element with an interactive role makes its
    // descendants' own roles presentational to assistive tech, so nesting
    // the chart's own role="button" legend rows inside it would hide them
    // from screen readers even though they're still clickable. The target is
    // absolutely positioned to fill the card and sits behind (`zIndex: 0`) a
    // `pointerEvents: "none"` content layer, so it only ever receives clicks
    // that land on genuine background -- pointer events are switched back on
    // (`pointerEvents: "auto"`) for the chart specifically, letting its own
    // click and keyboard handling work exactly as before.
    const ChartComponent = shape === "pie" ? DashboardPieChart : DashboardBarChart;
    const tileHref = config.buildHref(resolvePlaceholders(filters));
    const handleTileClick = (): void => {
      void navigate(tileHref, { state: dashboardReturnState });
    };
    const handleTileKeyDown = (e: KeyboardEvent): void => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        handleTileClick();
      }
    };
    return (
      <Card
        variant="outlined"
        sx={{
          position: "relative",
          p: 1.75,
          height: "100%",
        }}
      >
        <Box
          role="button"
          tabIndex={0}
          aria-label={`View all cases for ${resolvedDisplayName}`}
          onClick={handleTileClick}
          onKeyDown={handleTileKeyDown}
          sx={{
            position: "absolute",
            inset: 0,
            zIndex: 0,
            borderRadius: "inherit",
            cursor: "pointer",
            ...widgetHoverSx,
            "&:focus-visible": {
              outline: "2px solid",
              outlineColor: "primary.main",
              outlineOffset: -2,
            },
          }}
        />
        <Box sx={{ position: "relative", zIndex: 1, height: "100%", pointerEvents: "none" }}>
          {/* The header's own bottom padding — not just a top margin on the
              chart below it — so the chart's top edge (and, at the size this
              chart renders at, its tooltip) never sits flush against/behind
              the title row above it. */}
          <Box sx={{ pb: resolvedDescription ? 1 : 2.5 }}>{header}</Box>
          {resolvedDescription && (
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
              {resolvedDescription}
            </Typography>
          )}
          <Box sx={{ pointerEvents: "auto" }}>
            <ChartComponent
              slices={pieData.slices}
              total={pieData.total}
              isLoading={pieData.isLoading}
              isError={pieData.isError}
              onSliceClick={(slice: PieSliceResult) =>
                navigate(
                  config.buildHref(resolvePlaceholders(mergeWidgetFilters(filters, slice.query))),
                  { state: dashboardReturnState },
                )
              }
            />
          </Box>
        </Box>
      </Card>
    );
  }

  let body: ReactNode;
  if (isLoading) {
    body = <Skeleton variant="rounded" height={48} />;
  } else if (isError) {
    body = (
      <Typography variant="body2" color="text.secondary">
        Could not load this widget.
      </Typography>
    );
  } else {
    body = (
      <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.25 }}>
        <Box
          sx={{
            p: 0.75,
            mt: 0.25,
            borderRadius: "50%",
            bgcolor: alpha(theme.palette[config.iconColor].light, 0.1),
            color: theme.palette[config.iconColor].light,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
          }}
        >
          <Icon size={16} />
        </Box>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="caption" color="text.secondary" noWrap>
            {resolvedDisplayName}
          </Typography>
          <Typography
            noWrap
            sx={{
              mt: 0.5,
              lineHeight: 1.1,
              fontWeight: 400,
              fontSize: "3.25rem",
              overflow: "hidden",
              textOverflow: "ellipsis",
            }}
          >
            {formattedCount}
          </Typography>
        </Box>
      </Box>
    );
  }

  return (
    // Same sibling-target restructuring as the pie/bar branch above: the
    // whole-card click-through used to BE the (now-removed) refresh
    // IconButton's own ancestor (`Card component={RouterLink}`), which hides
    // a nested interactive control from assistive tech exactly like a nested
    // role="button" does. The anchor here is a background sibling instead;
    // see the pie/bar branch's comment for the pointer-events layering.
    <Card
      variant="outlined"
      sx={{
        position: "relative",
        p: 1.75,
        height: "100%",
      }}
    >
      <Box
        component={RouterLink}
        to={href}
        state={dashboardReturnState}
        // The visible count + label sit in the pointer-events-none content
        // layer above this anchor, not inside it as descendant text anymore
        // (that's the whole point -- see the comment above), so it needs its
        // own accessible name instead of inheriting one from its content.
        aria-label={`${resolvedDisplayName}: ${formattedCount}`}
        sx={{
          position: "absolute",
          inset: 0,
          zIndex: 0,
          display: "block",
          borderRadius: "inherit",
          cursor: "pointer",
          color: "inherit",
          textDecoration: "none",
          ...widgetHoverSx,
          "&:focus-visible": { outline: "2px solid", outlineColor: "primary.main", outlineOffset: -2 },
        }}
      />
      <Box sx={{ position: "relative", zIndex: 1, height: "100%", pointerEvents: "none" }}>
        <Box sx={{ pointerEvents: "auto" }}>{infoIcon}</Box>
        {body}
      </Box>
    </Card>
  );
}
