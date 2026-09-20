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

import { Box, Button, Card, IconButton, Skeleton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { ArrowRight, X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import type { BeDashboardWidgetColumn, BeWidgetResourceType } from "@api/backend/types";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useWidgetData } from "@features/csm-dashboard/api/useWidgetData";
import type { PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import { WIDGET_LIST_RENDERERS } from "@features/csm-dashboard/config/widgetListConfig";
import GenericColumnList from "@features/csm-dashboard/components/GenericColumnList";
import { buildWidgetPreviewHref } from "@features/csm-dashboard/utils/widgetPreviewUrl";
import { mergeWidgetFilters } from "@features/csm-dashboard/utils/widgetFilterMerge";
import { resolveTeamPlaceholder } from "@features/csm-dashboard/utils/teamFilterPlaceholder";
import { resolveRelativeDateFilters } from "@features/csm-dashboard/utils/resolveRelativeDateFilters";
import { resolveCurrentUserPlaceholder } from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";
import { resolveWidgetText } from "@features/csm-dashboard/utils/widgetTextPlaceholder";

export interface WidgetInlineDrilldownPanelProps {
  widgetId: string;
  /** This widget's own raw `displayName` (not yet `{{currentTeam}}`-resolved
   * — resolved here, the same way `DashboardWidgetTile` resolves its own
   * copy of the same prop, via {@link selectedTeamLabel}). */
  displayName: string;
  resourceType: BeWidgetResourceType;
  /** This widget's own base filters (same shape as `DashboardWidgetTile`'s
   * `filters` prop) — merged under the expanded slice's own `query` (slice
   * keys win on conflict; see `mergeWidgetFilters`) before either fetching
   * or building the "View more" href. */
  filters: Record<string, unknown>;
  /** The slice this panel is expanding — whichever one the tile's own click
   * handler last reported via `onExpandChange`. */
  slice: PieSliceResult;
  listLimit?: number;
  columns?: BeDashboardWidgetColumn[];
  selectedTeamCreGroupId?: string | string[];
  selectedTeamSreGroupId?: string | string[];
  selectedTeamLabel?: string;
  /** Called when the panel's own close control is activated, or when the
   * caller wants this panel collapsed for any other reason (e.g. clicking
   * the same slice again on the tile above). */
  onClose: () => void;
}

/**
 * The full-width panel a `DashboardWidgetTile`'s `inlineDrilldown` slice
 * click expands into — rendered by `DashboardWidgetGrid` as a standalone
 * grid item directly below the (narrow, `gridWidth`-sized) chart tile that
 * triggered it, rather than nested inside that tile's own `Card` (see
 * `DashboardWidgetGrid.tsx`'s `renderTile`, which is what actually
 * positions this full-width).
 *
 * Owns the expanded slice's own data fetch (the same `useWidgetData` shape
 * "list" call `DashboardWidgetTile` used to fire directly) and the
 * loading/error/"View more" rendering that goes with it — extracted out of
 * `DashboardWidgetTile` unchanged in behavior, just relocated so it can
 * render as this component's own full-width sibling instead of nested
 * inside the chart tile.
 */
export default function WidgetInlineDrilldownPanel({
  widgetId,
  displayName,
  resourceType,
  filters,
  slice,
  listLimit,
  columns,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  selectedTeamLabel,
  onClose,
}: WidgetInlineDrilldownPanelProps): JSX.Element {
  const { user } = useCurrentUser();
  const currentUserId = user?.id;
  const config = WIDGET_RESOURCE_CONFIG[resourceType];
  const ListRenderer = WIDGET_LIST_RENDERERS[resourceType];
  const hasColumns = Boolean(columns && columns.length > 0);
  const resolvedDisplayName = resolveWidgetText(displayName, selectedTeamLabel) ?? displayName;

  // Same placeholder-resolution pipeline `DashboardWidgetTile` applies to
  // every filter object it builds a request/href from (see that
  // component's own `resolvePlaceholders`) — kept identical here so this
  // panel's request and "View more" href resolve `__current_team__`/
  // `__current_user__`/relative-date placeholders exactly the way the tile
  // itself already does.
  const resolvePlaceholders = (f: Record<string, unknown>): Record<string, unknown> =>
    resolveCurrentUserPlaceholder(
      resolveRelativeDateFilters(
        resolveTeamPlaceholder(f, selectedTeamCreGroupId, selectedTeamSreGroupId),
      ),
      currentUserId,
    );

  const mergedFilters = mergeWidgetFilters(filters, slice.query);
  const listLimitValue = listLimit ?? 4;
  const { data, isLoading, isError } = useWidgetData(
    widgetId,
    resourceType,
    mergedFilters,
    "list",
    listLimitValue,
    0,
    true,
    selectedTeamCreGroupId,
    selectedTeamSreGroupId,
    undefined,
    currentUserId,
  );
  const total = data?.total ?? 0;

  if (!config) {
    // Same defensive guard `DashboardWidgetTile` applies for an
    // unrecognized resourceType — never crash the panel over a
    // runtime-configurable value.
    return (
      <Card variant="outlined" sx={{ p: 1.75 }}>
        <Typography variant="body2" color="text.secondary">
          Unsupported widget type.
        </Typography>
      </Card>
    );
  }

  return (
    <Card variant="outlined" sx={{ p: 1.75 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          gap: 1,
          mb: 1.5,
        }}
      >
        {/* Names both the widget and the specific slice being shown — this
            panel no longer sits physically attached to the chart above it
            (it's a full-width sibling row, possibly separated by other
            tiles in the same section), so the reader needs to know what
            it's showing without scrolling back up. */}
        <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
          {resolvedDisplayName} — {slice.label}
        </Typography>
        <Tooltip title="Close">
          <IconButton
            size="small"
            aria-label={`Close ${resolvedDisplayName} — ${slice.label}`}
            onClick={onClose}
            sx={{ color: "text.secondary", flexShrink: 0 }}
          >
            <X size={16} />
          </IconButton>
        </Tooltip>
      </Box>
      {isLoading ? (
        <Skeleton variant="rounded" height={28 * listLimitValue + 40} />
      ) : isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load this widget.
        </Typography>
      ) : (
        <>
          {hasColumns ? (
            <GenericColumnList
              items={data?.items ?? []}
              isLoading={false}
              resourceType={resourceType}
              columns={columns ?? []}
            />
          ) : (
            <ListRenderer items={data?.items ?? []} isLoading={false} resourceType={resourceType} />
          )}
          {total > listLimitValue && (
            <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 1 }}>
              <Button
                component={RouterLink}
                to={buildWidgetPreviewHref({
                  previewSlug: config.previewSlug,
                  widgetId,
                  displayName: resolvedDisplayName,
                  filters: resolvePlaceholders(mergedFilters),
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
