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

import { Box, Button, Card, Chip, IconButton, Skeleton, Tooltip, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { ArrowRight, Info, RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { useRef, useState, type JSX, type KeyboardEvent, type MouseEvent, type ReactNode } from "react";
import { Link as RouterLink, useLocation, useNavigate } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useElementVisibleOnce } from "@hooks/useElementVisibleOnce";
import { useRelativeTimeTick } from "@components/RelativeTime";
import { formatRelativeTime } from "@features/csm-dashboard/utils/abtDashboard";
import { invalidateWidgetQueries } from "@features/csm-dashboard/utils/invalidateWidgetQueries";
import type {
  BeDashboardGroupByConfig,
  BeDashboardPieSlice,
  BeDashboardWidgetColumn,
  BeWidgetResourceType,
  BeWidgetShape,
} from "@api/backend/types";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useWidgetData } from "@features/csm-dashboard/api/useWidgetData";
import { useWidgetPieData, type PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";
import { useWidgetGroupByData } from "@features/csm-dashboard/api/useWidgetGroupByData";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import { WIDGET_LIST_RENDERERS } from "@features/csm-dashboard/config/widgetListConfig";
import GenericColumnList from "@features/csm-dashboard/components/GenericColumnList";
import { buildWidgetPreviewHref } from "@features/csm-dashboard/utils/widgetPreviewUrl";
import { mergeWidgetFilters } from "@features/csm-dashboard/utils/widgetFilterMerge";
import { resolveTeamPlaceholder } from "@features/csm-dashboard/utils/teamFilterPlaceholder";
import { resolveRelativeDateFilters } from "@features/csm-dashboard/utils/resolveRelativeDateFilters";
import {
  hasCurrentUserPlaceholder,
  resolveCurrentUserPlaceholder,
} from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";
import { resolveWidgetText } from "@features/csm-dashboard/utils/widgetTextPlaceholder";
import DashboardPieChart from "@features/csm-dashboard/components/DashboardPieChart";
import DashboardBarChart from "@features/csm-dashboard/components/DashboardBarChart";

// Hides the per-widget refresh button by default and reveals it on hover or
// keyboard focus of the ancestor Card carrying the matching
// `&:hover .dashboard-widget-refresh, &:focus-within .dashboard-widget-refresh`
// rule (each tile shape below sets that rule on its own top-level `Card`).
// `:focus-within` (rather than relying on `:hover` alone) is what makes a
// keyboard user tabbing onto the button reveal it first, rather than
// reaching an interactive control they can never see. Not `display: none`,
// so Tab order still reaches this button while it's visually hidden.
const widgetRefreshRevealSx = {
  opacity: 0,
  pointerEvents: "none",
  transition: "opacity 0.15s ease",
} as const;

const cardRefreshRevealSx = {
  "&:hover .dashboard-widget-refresh, &:focus-within .dashboard-widget-refresh": {
    opacity: 1,
    pointerEvents: "auto",
  },
} as const;

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
   * crashing. Mutually exclusive with `groupBy` — a widget carries at most
   * one of the two (backend-enforced). */
  slices?: BeDashboardPieSlice[];
  /** Only meaningful for shape "pie"/"bar"; a single server-side
   * `POST {resourceType}/aggregate` call instead of one search per slice
   * (see `useWidgetGroupByData`). Mutually exclusive with `slices`. */
  groupBy?: BeDashboardGroupByConfig;
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
  /** The currently selected team's own `creGroupId` (see
   * `BeTeam.creGroupId`), or an array of every team's `creGroupId` in the
   * current dashboard's family when the "All ABTs" option is selected (see
   * `ALL_TEAMS_SENTINEL`), threaded down from `CsmDashboardPage` for
   * resolving this widget's own `__current_team__` filter placeholder for a
   * `creTeam` filter entry (see `teamFilterPlaceholder.ts`) — never the
   * team registry key. `undefined` for a non-team-based dashboard, or while
   * the team isn't resolved yet (any `creTeam` filter entry carrying the
   * placeholder is then dropped, not sent literally). */
  selectedTeamCreGroupId?: string | string[];
  /** The currently selected team's own `sreGroupId` (see
   * `BeTeam.sreGroupId`), or an array of every team's `sreGroupId` in the
   * current dashboard's family when the "All ABTs" option is selected — the
   * `sreTeam`-filter counterpart of {@link selectedTeamCreGroupId}, resolved
   * independently. `undefined` in the same cases `selectedTeamCreGroupId`
   * is (any `sreTeam` filter entry carrying the placeholder is then
   * dropped, not sent literally). */
  selectedTeamSreGroupId?: string | string[];
  /** Human-readable label for the selected team (its own display `name`, or
   * the literal `"All ABTs"`) — used to resolve the `{{currentTeam}}` text
   * placeholder (see `widgetTextPlaceholder.ts`) inside `displayName`/
   * `description` before render. `undefined` in the same cases
   * `selectedTeamCreGroupId` is (the token is then stripped rather than left
   * literally visible). */
  selectedTeamLabel?: string;
  /** Suppresses this tile's own per-widget refresh button entirely. Exists
   * for `CsmDashboardBuilderEditorPage`, the only caller that also passes a
   * `renderWidgetAction` into `DashboardWidgetGrid` (Edit/Remove icons):
   * that action renders as a sibling `Box` absolutely positioned over the
   * same top-right corner this tile's own refresh button occupies (see
   * `DashboardWidgetGrid.tsx`'s `renderTile`), at a higher `zIndex`, so
   * without this the builder action fully covers the refresh button and an
   * admin can never click it. The builder-editor page is a config-editing
   * surface, not a live-data-monitoring one, so dropping the refresh
   * button there (rather than trying to reposition both into two separate
   * corners) is the right trade — Edit/Remove are the actions that matter
   * in that context. `undefined`/falsy is a no-op: a normal dashboard page
   * (no `renderWidgetAction`) keeps its refresh button exactly as before. */
  hideRefreshButton?: boolean;
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
  groupBy,
  columns,
  sortBy,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  selectedTeamLabel,
  hideRefreshButton,
}: DashboardWidgetTileProps): JSX.Element {
  const theme = useTheme();
  const navigate = useNavigate();
  const location = useLocation();
  const { user } = useCurrentUser();
  const currentUserId = user?.id;
  // Gates this tile's own data fetch (below) behind having actually
  // scrolled into (or near — see the hook's rootMargin) the viewport at
  // least once, rather than every widget on the dashboard firing its
  // search the instant the page mounts. Attached to whichever top-level
  // `Card` this component ends up returning (every branch below sets
  // `ref={tileRef}`) — a given tile's shape never changes across its own
  // lifetime, so exactly one of them ever mounts for a given instance.
  const tileRef = useRef<HTMLDivElement>(null);
  const isVisible = useElementVisibleOnce(tileRef);
  const queryClient = useQueryClient();
  // True only while this widget's own manually-triggered refresh (below) is
  // in flight — distinct from `isLoading`'s own first-fetch/lazy-load
  // gating, so the button disables itself for the brief span of its own
  // refetch without the rest of the tile flashing back to its skeleton
  // state.
  const [isRefreshing, setIsRefreshing] = useState(false);
  // This widget's own last-manually-refreshed timestamp (ms) — set once the
  // in-flight `invalidateWidgetQueries` call below resolves. `undefined`
  // until the user has clicked refresh at least once, which is exactly the
  // gate the "Last refreshed" label below reads (see `refreshButton`) —
  // mirrors the section-level `RefreshButton`'s own `hasManuallyRefreshed`
  // pattern, just tracked per-widget here instead of per-section.
  const [lastRefreshedAt, setLastRefreshedAt] = useState<number | undefined>(undefined);
  // Re-renders exactly when the "Last refreshed …" text below would next
  // change (adaptive shared scheduler — see RelativeTime.tsx), without
  // requiring another fetch. Passed explicitly into `formatRelativeTime`
  // below (rather than relying on its internal `Date.now()` default) so the
  // React Compiler's auto-memoization sees it as a dependency.
  const now = useRelativeTimeTick(lastRefreshedAt);
  const handleWidgetRefresh = (e: MouseEvent): void => {
    // Stops this click from also being interpreted as a click on the
    // tile's own whole-card/tile-level click-through target that this
    // button is layered above (see the count and pie/bar branches below) —
    // without this, refreshing a tile would also navigate away from it.
    e.stopPropagation();
    e.preventDefault();
    setIsRefreshing(true);
    void invalidateWidgetQueries(queryClient, new Set([widgetId])).finally(() => {
      setIsRefreshing(false);
      setLastRefreshedAt(Date.now());
    });
  };
  // Resolves every client-side filter placeholder a widget's own (opaque)
  // filters may carry — `__current_team__` (see `teamFilterPlaceholder.ts`),
  // `__current_user__` (see `currentUserFilterPlaceholder.ts`) and the
  // relative-date family `__today__`/`__daysAgo:N__`/... (see
  // `resolveRelativeDateFilters.ts`) — in one pass, in the same order and by
  // the same functions `useWidgetData`/`useWidgetPieData` use internally, so
  // every click-through href below lands on exactly the rows this tile just
  // counted.
  //
  // The relative-date resolution matters most here: it is browser-local, and
  // the destination list page has no placeholder resolution of its own, so an
  // unresolved `__today__` reaching it was forwarded verbatim and resolved by
  // the backing service against UTC — the tile and the list it links to then
  // disagreed by the offset between UTC midnight and the viewer's own, which
  // is the whole discrepancy this widget-side resolution exists to close.
  // Resolving here does freeze the instant into the href; that is the right
  // trade, because everything else in these hrefs (team ids, states, the
  // viewer's own user id) is already a snapshot of the moment of the click,
  // and the destination's URL scheme is the cases list's own concrete filter
  // encoding — carrying dashboard placeholder vocabulary into it would make
  // `__today__` a value users can type into, and hand-edit out of, a shared
  // list-page URL.
  const resolvePlaceholders = (f: Record<string, unknown>): Record<string, unknown> =>
    resolveCurrentUserPlaceholder(
      resolveRelativeDateFilters(
        resolveTeamPlaceholder(f, selectedTeamCreGroupId, selectedTeamSreGroupId),
      ),
      currentUserId,
    );
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
  // Per-widget refresh control: icon plus a "Last refreshed …" label,
  // matching the section-level `RefreshButton`'s own layout (label to the
  // left of the icon) and its "only after a manual click" gating —
  // `lastRefreshedAt` stays `undefined` until this widget's own refresh has
  // been clicked at least once, so the label never appears from this tile's
  // initial data load. Hidden by default, revealed together with any other
  // hover-gated control on the tile via the ancestor Card's own
  // `&:hover .dashboard-widget-refresh, &:focus-within .dashboard-widget-refresh`
  // rule (see `widgetRefreshRevealSx`) — kept inert (opacity 0 + pointerEvents
  // none) while hidden so it can't be clicked without being seen, but stays
  // in the keyboard Tab order throughout (unlike `display: none`, which
  // would remove it from Tab order entirely). Both the label and the icon
  // live inside this same wrapper so they hide/reveal as one unit.
  //
  // Shape "count" tiles are too narrow to fit the label inline next to the
  // icon (they're the shortest/tightest tiles, and the icon already shares
  // that corner with `infoIcon` when a description is set) — for that shape
  // only, the same "Last refreshed …" text is folded into the icon's own
  // tooltip instead of rendered as a separate label. The widget's own name
  // is deliberately left out of the tooltip text (unlike the `aria-label`,
  // which keeps it — screen-reader users still need it to tell apart the
  // many refresh buttons on one dashboard page) since it's already visible
  // right next to the tile's own title, and including it made the combined
  // "Refresh <name> — Last refreshed <time>" tooltip too long.
  const isCountShape = shape === "count";
  const lastRefreshedText = lastRefreshedAt
    ? `Last refreshed ${formatRelativeTime(new Date(lastRefreshedAt).toISOString(), now)}`
    : undefined;
  const refreshTooltipTitle =
    isCountShape && lastRefreshedText
      ? `Refresh this widget - ${lastRefreshedText}`
      : "Refresh this widget";
  // `null` (not a conditionally-skipped render below) when
  // `hideRefreshButton` is set — see that prop's own doc comment — so every
  // one of the three shapes below, which all reuse this single variable,
  // stays untouched rather than needing its own gate.
  const refreshButton = hideRefreshButton ? null : (
    <Box
      className="dashboard-widget-refresh"
      sx={{ display: "flex", alignItems: "center", gap: 0.75, ...widgetRefreshRevealSx }}
    >
      {!isCountShape && lastRefreshedText ? (
        <Typography variant="caption" color="text.secondary" noWrap>
          {lastRefreshedText}
        </Typography>
      ) : null}
      <Tooltip title={refreshTooltipTitle}>
        <span>
          <IconButton
            size="small"
            onClick={handleWidgetRefresh}
            disabled={isRefreshing}
            aria-label={`Refresh ${resolvedDisplayName}`}
            sx={{ color: "text.secondary" }}
          >
            <RefreshCw size={14} />
          </IconButton>
        </span>
      </Tooltip>
    </Box>
  );
  // Shape "pie" resolves via useWidgetPieData instead (one search per
  // slice) — skip this one's own network call rather than wasting it, but
  // still call the hook unconditionally (rules of hooks; a widget's shape
  // never changes across this component's lifetime).
  const { data, isLoading: isWidgetDataLoading, isError } = useWidgetData(
    widgetId,
    resourceType,
    filters,
    shape,
    listLimit,
    0,
    shape !== "pie" && shape !== "bar" && isVisible,
    selectedTeamCreGroupId,
    selectedTeamSreGroupId,
    sortBy,
    currentUserId,
  );
  const sliceData = useWidgetPieData(
    widgetId,
    resourceType,
    filters,
    shape === "pie" || shape === "bar" ? (slices ?? []) : [],
    selectedTeamCreGroupId,
    selectedTeamSreGroupId,
    currentUserId,
    isVisible,
  );
  // groupBy is the alternative to slices for shapes "pie"/"bar" — mutually
  // exclusive, backend-enforced (never both, never neither for those
  // shapes). Both hooks are called unconditionally (rules of hooks; a
  // widget's shape/config never changes across this component's lifetime)
  // and only one of them ever actually fires a request, gated by its own
  // `groupBy`/`slices` argument being empty/undefined.
  const groupByData = useWidgetGroupByData(
    widgetId,
    resourceType,
    filters,
    shape === "pie" || shape === "bar" ? groupBy : undefined,
    selectedTeamCreGroupId,
    selectedTeamSreGroupId,
    currentUserId,
    isVisible,
  );
  const pieData = groupBy ? groupByData : sliceData;
  // True while this widget carries a `__current_user__` filter and the
  // signed-in user's profile hasn't loaded yet. `useWidgetData`/
  // `useWidgetPieData` hold their requests in that window (see
  // `currentUserFilterPlaceholder.ts` — a user-scoped filter must never be
  // dropped, which would widen the widget to every user's records), so this
  // tile must present the wait as loading rather than paint the `0` a
  // deferred query's absent data would otherwise resolve to.
  const awaitingCurrentUser = currentUserId === undefined && hasCurrentUserPlaceholder(filters);
  // `!isVisible` (this tile hasn't scrolled into view yet, so its own fetch
  // above never fired) reports as loading rather than as react-query's own
  // `isLoading` for a disabled query — same rationale as useWidgetPieData's
  // own `isLoading`. This is the count/list-shape tile's loading state;
  // shape "pie"/"bar" reads `pieData.isLoading` instead, which already
  // folds in the same `isVisible` gate via the `enabled` argument above.
  const isLoading = isWidgetDataLoading || awaitingCurrentUser || !isVisible;
  const config = WIDGET_RESOURCE_CONFIG[resourceType];
  // Thousands separators for shape "count"'s big number -- used both in the
  // visible Typography and the tile's aria-label, so both stay in sync.
  const formattedCount = (data?.total ?? 0).toLocaleString();

  if (!config) {
    // resourceType came from a runtime-configurable backend registry (not a
    // compile-time-checked Go literal) — an unrecognized value must not
    // crash this tile's render (config.buildHref below would throw).
    return (
      <Card ref={tileRef} variant="outlined" sx={{ p: 1.75 }}>
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
  const href = config.buildHref(resolvePlaceholders(filters), {
    widgetId,
    displayName: resolvedDisplayName,
  });
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

  // Shared title row: icon + name, with an optional total-count badge sitting
  // immediately after the name (ServiceNow-style "Title 42" rather than a
  // number floated off to the card's far edge or stacked in its own line).
  // `undefined` renders no badge — shape "pie" keeps its total in the donut
  // hole instead, and list/bar themselves pass `undefined` while their own
  // total is still loading or errored.
  const renderHeader = (total?: number): JSX.Element => (
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
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, mt: 0.5, minWidth: 0 }}>
        <Typography variant="caption" color="text.secondary">
          {resolvedDisplayName}
        </Typography>
        {total !== undefined && (
          <Chip
            label={total.toLocaleString()}
            size="small"
            color="default"
            sx={{ flexShrink: 0, fontWeight: 600 }}
          />
        )}
      </Box>
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
    const total = data?.total ?? 0;
    return (
      <Card
        ref={tileRef}
        variant="outlined"
        sx={{ position: "relative", p: 1.75, height: "100%", ...cardRefreshRevealSx }}
      >
        <Box sx={{ position: "absolute", top: 8, right: 8, zIndex: 1 }}>{refreshButton}</Box>
        {/* The rows below are capped at `listLimit` (see `useWidgetData`'s
            DEFAULT_LIST_LIMIT) — this badge next to the title is the only
            place the widget's own full count is visible, since "View more"
            only appears once `total` exceeds that cap. */}
        {renderHeader(!isLoading && !isError ? total : undefined)}
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
    const tileHref = config.buildHref(resolvePlaceholders(filters), {
      widgetId,
      displayName: resolvedDisplayName,
    });
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
        ref={tileRef}
        variant="outlined"
        sx={{
          position: "relative",
          p: 1.75,
          height: "100%",
          ...cardRefreshRevealSx,
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
          <Box sx={{ position: "absolute", top: -4, right: -4, pointerEvents: "auto" }}>
            {refreshButton}
          </Box>
          {/* The header's own bottom padding — not just a top margin on the
              chart below it — so the chart's top edge (and, at the size this
              chart renders at, its tooltip) never sits flush against/behind
              the title row above it. */}
          <Box sx={{ pb: resolvedDescription ? 1 : 2.5 }}>
            {renderHeader(
              shape === "bar" && !pieData.isLoading && !pieData.isError ? pieData.total : undefined,
            )}
          </Box>
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
              onSliceClick={(slice: PieSliceResult) => {
                // See `PieSliceResult.navigable`'s own doc comment: a
                // groupBy widget's synthetic "Others" bucket has no safe
                // selector to navigate to, so it opts out of click-through
                // entirely rather than falling through to the widget's own
                // unscoped base result set.
                if (slice.navigable === false) return;
                navigate(
                  config.buildHref(resolvePlaceholders(mergeWidgetFilters(filters, slice.query)), {
                    widgetId,
                    displayName: resolvedDisplayName,
                  }),
                  { state: dashboardReturnState },
                );
              }}
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
      ref={tileRef}
      variant="outlined"
      sx={{
        position: "relative",
        p: 1.75,
        height: "100%",
        ...cardRefreshRevealSx,
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
        {/* Refresh sits to the LEFT of the (always-visible) info icon when
            both are present, rather than stacking one on top of the other —
            `infoIcon` already occupies this exact top-right spot
            (top: 8, right: 8) whenever this widget has a description. */}
        <Box
          sx={{
            position: "absolute",
            top: -4,
            right: resolvedDescription ? 28 : -4,
            pointerEvents: "auto",
          }}
        >
          {refreshButton}
        </Box>
        <Box sx={{ pointerEvents: "auto" }}>{infoIcon}</Box>
        {body}
      </Box>
    </Card>
  );
}
