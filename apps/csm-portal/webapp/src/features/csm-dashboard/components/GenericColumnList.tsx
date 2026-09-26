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

import { IconButton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Eye } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type JSX, type ReactNode } from "react";
import { useLocation } from "react-router";
import type {
  BeCallRequestView,
  BeCaseSearchView,
  BeChangeRequestSearchView,
  BeDashboardWidgetColumn,
  BeIncident,
  BeProblemSearchView,
  BeProductVulnerabilityView,
  BeWidgetResourceType,
} from "@api/backend/types";
import { useNavTransition } from "@hooks/useNavTransition";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import {
  getColumnPreferencesUserKey,
  useColumnPreferences,
} from "@hooks/useColumnPreferences";
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import DashboardMiniTable from "@features/csm-dashboard/components/DashboardMiniTable";
import { formatColumnValue, resolveColumnPath } from "@features/csm-dashboard/utils/resolveWidgetColumn";
import CasePreviewDrawer from "@features/csm-cases/components/CasePreviewDrawer";
import CallRequestDetailModal from "@features/csm-cases/components/CallRequestDetailModal";
import { mapCaseSearchViewToRow } from "@features/csm-cases/utils/caseSearchPayload";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";
import IncidentPreviewDrawer from "@features/csm-operations/components/IncidentPreviewDrawer";
import ChangeRequestPreviewDrawer from "@features/csm-operations/components/ChangeRequestPreviewDrawer";
import ProblemPreviewDrawer from "@features/csm-operations/components/ProblemPreviewDrawer";
import type { Account } from "@features/csm-accounts/types/csmAccounts";
import AccountPreviewDrawer from "@features/csm-accounts/components/AccountPreviewDrawer";
import type { Project } from "@features/csm-projects/types/csmProjects";
import ProjectPreviewDrawer from "@features/csm-projects/components/ProjectPreviewDrawer";
import { normalizeUser, type SnUser, type User } from "@features/csm-users/types/csmUsers";
import UserPreviewDrawer from "@features/csm-users/components/UserPreviewDrawer";
import ProductVulnerabilityPreviewDrawer from "@features/csm-security-center/components/ProductVulnerabilityPreviewDrawer";

export interface GenericColumnListProps {
  items: Record<string, unknown>[];
  isLoading: boolean;
  resourceType: BeWidgetResourceType;
  columns: BeDashboardWidgetColumn[];
  /** This widget's own id — folded into the persisted column-preferences
   * `viewId` (see `useColumnPreferences`) so two different `columns`-
   * configured widgets (even of the same `resourceType`) never share, or
   * collide on, one saved show/hide/reorder layout. Optional only so
   * existing call sites/tests that don't care about persistence (and never
   * pass `onColumnCustomizerChange`) keep compiling; every real caller in
   * this app supplies it. */
  widgetId?: string;
  /** Same "hand the button up to the caller" contract
   * `WidgetListRendererProps.onColumnCustomizerChange` uses (see
   * `widgetListConfig.tsx`) — lets `DashboardWidgetTile`/
   * `WidgetInlineDrilldownPanel` render this list's own "Customise columns"
   * button next to their existing header controls instead of this
   * component growing a toolbar row of its own. Omitted (no current caller
   * does) means no customizer renders at all for this list — the column set
   * still respects any previously-saved preference, it just can't be
   * changed without one. */
  onColumnCustomizerChange?: (node: ReactNode | null) => void;
}

/** Raw item shape a `columns`-configured widget's own `/search` response
 * resolves to — same loose typing `widgetListConfig.tsx`'s own `WidgetItem`
 * uses, for the same reason (the real shape depends on `resourceType`). */
type WidgetItem = Record<string, unknown>;

/**
 * ResourceType-agnostic `shape: "list"` renderer, used only when a widget's
 * config sets `columns` (see `BeDashboardWidget.columns`) — resolves each
 * column's dot-path against every item and renders the result in a
 * `DashboardMiniTable`, rather than dispatching through the hardcoded
 * `WIDGET_LIST_RENDERERS[resourceType]`. A widget with no `columns` never
 * reaches this component (see `DashboardWidgetTile`'s list-shape branch),
 * so every existing dashboard renders exactly as it did before this
 * component existed.
 *
 * Row navigation reuses `WIDGET_RESOURCE_CONFIG[resourceType].detailHref` —
 * the same per-resourceType destination each hardcoded renderer already
 * links to — via a row-level `onClick` (not `href`) so the quick-preview
 * `Eye` icon below can sit as a genuine sibling button rather than nested
 * inside an anchor (see `DashboardWidgetTile`'s own count/pie-tile comments
 * on why a nested interactive control inside a link is avoided throughout
 * this feature).
 *
 * The preview icon opens that resourceType's own preview drawer (built
 * alongside `WIDGET_LIST_RENDERERS`' renderers in `widgetListConfig.tsx`) —
 * `case` is the only resourceType actually configured with `columns` today
 * (the ABT "Patches" widgets), but every other resourceType `WIDGET_LIST_
 * RENDERERS` supports is wired here too, so a future `columns`-configured
 * widget of any of those types gets the same quick-preview affordance for
 * free rather than silently regressing to none.
 */
export default function GenericColumnList({
  items,
  isLoading,
  resourceType,
  columns,
  widgetId,
  onColumnCustomizerChange,
}: GenericColumnListProps): JSX.Element {
  const config = WIDGET_RESOURCE_CONFIG[resourceType];
  const location = useLocation();
  const navigate = useNavTransition();
  const dashboardReturnState = { from: `${location.pathname}${location.search}` };
  const [previewItem, setPreviewItem] = useState<WidgetItem | null>(null);
  const currentUserId = useCurrentUser().user?.id;
  const currentUserEmail = useIdTokenClaims()?.email;

  // Every configured column is shown by default (no `columns` entry is
  // "optional" the way `CaseWidgetList`'s hardcoded product/type/severity/…
  // set is) — this only lets a viewer show/hide/reorder among the SAME set
  // the widget's own config already defines, never invents a column beyond
  // it. `path` is this widget's own stable, unique-per-column id (already
  // used as the cell's own React key below).
  const columnOptions = useMemo(
    () => columns.map((c) => ({ id: c.path, label: c.label })),
    [columns],
  );
  const defaultVisibleIds = useMemo(() => columns.map((c) => c.path), [columns]);
  const columnPrefs = useColumnPreferences({
    // Falls back to the bare resourceType when no `widgetId` is supplied
    // (see that prop's own doc comment) — every real caller passes one, so
    // this fallback only matters for a caller/test that doesn't care about
    // per-widget disambiguation in the first place.
    viewId: `dashboard-generic-list:${widgetId ?? resourceType}`,
    userKey: getColumnPreferencesUserKey({ id: currentUserId, email: currentUserEmail }),
    columns: columnOptions,
    defaultVisibleIds,
  });
  // Re-derive the full `BeDashboardWidgetColumn` (format included) for each
  // visible id, in the user's own chosen order — `columnPrefs` only ever
  // hands back the `{id, label}` shape `useColumnPreferences` was given, not
  // this widget's own `format`, so it's looked back up here rather than lost.
  const columnsById = useMemo(() => new Map(columns.map((c) => [c.path, c])), [columns]);
  const visibleColumns = useMemo(
    () =>
      columnPrefs.visibleColumns
        .map((c) => columnsById.get(c.id))
        .filter((c): c is BeDashboardWidgetColumn => !!c),
    [columnPrefs.visibleColumns, columnsById],
  );

  const columnCustomizerButton = (
    <ColumnCustomizerButton
      allColumns={columnPrefs.allColumns}
      isVisible={columnPrefs.isVisible}
      onToggle={columnPrefs.toggleColumn}
      onMove={columnPrefs.moveColumn}
      onReorder={columnPrefs.reorderColumn}
      onReset={columnPrefs.resetToDefault}
      label="Customise columns"
    />
  );

  // Same "hand it up, clear it on unmount" contract `CaseWidgetList` uses
  // (see `widgetListConfig.tsx`) — re-fires exactly when the button's own
  // inputs change.
  useEffect(() => {
    if (!onColumnCustomizerChange) return;
    onColumnCustomizerChange(columnCustomizerButton);
    return () => onColumnCustomizerChange(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- re-fires exactly when the button's own inputs change, per the comment above.
  }, [
    onColumnCustomizerChange,
    columnPrefs.allColumns,
    columnPrefs.isVisible,
    columnPrefs.toggleColumn,
    columnPrefs.moveColumn,
    columnPrefs.reorderColumn,
    columnPrefs.resetToDefault,
  ]);

  return (
    <>
      <DashboardMiniTable
        isLoading={isLoading}
        emptyMessage="No records match this widget's filters."
        columns={[
          { label: "Preview", width: "auto" },
          ...visibleColumns.map((c) => ({ label: c.label })),
        ]}
        rows={items.map((item, i) => {
          const id = typeof item.id === "string" ? item.id : undefined;
          const href = config?.detailHref?.(item);
          const label = config ? config.primaryLabel(item) : "this record";
          return {
            key: id ?? `row-${i}`,
            onClick: href ? () => navigate(href, { state: dashboardReturnState }) : undefined,
            cells: [
              <Tooltip key="preview" title={`Quick preview ${label}`}>
                <IconButton
                  size="small"
                  aria-label={`Quick preview ${label}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    setPreviewItem(item);
                  }}
                >
                  <Eye size={16} />
                </IconButton>
              </Tooltip>,
              ...visibleColumns.map((c) => (
                <Typography key={c.path} variant="body2" noWrap>
                  {formatColumnValue(resolveColumnPath(item, c.path), c.format)}
                </Typography>
              )),
            ],
          };
        })}
      />
      {renderPreview(resourceType, previewItem, () => setPreviewItem(null))}
    </>
  );
}

/** Dispatches `previewItem` to the right resourceType's own preview drawer —
 * every entry mirrors the equivalent `XxxWidgetList` renderer in
 * `widgetListConfig.tsx`. `task`/`time_card` are omitted: neither has a
 * standalone detail route (see `WIDGET_RESOURCE_CONFIG`'s own `detailHref:
 * () => undefined` for both), so a `columns`-configured widget of either type
 * would have no row navigation either — out of scope for this generic
 * renderer, unchanged from before this preview column existed. */
function renderPreview(
  resourceType: BeWidgetResourceType,
  item: WidgetItem | null,
  onClose: () => void,
): JSX.Element | null {
  switch (resourceType) {
    case "case":
    case "service_request":
    case "security_report_analysis":
    case "announcement":
    case "engagement": {
      const row: CsmCaseRow | null = item
        ? mapCaseSearchViewToRow(item as unknown as BeCaseSearchView, undefined)
        : null;
      return <CasePreviewDrawer row={row} onClose={onClose} />;
    }
    case "incident":
      return <IncidentPreviewDrawer incident={item as unknown as BeIncident | null} onClose={onClose} />;
    case "change_request":
      return (
        <ChangeRequestPreviewDrawer
          changeRequest={item as unknown as BeChangeRequestSearchView | null}
          onClose={onClose}
        />
      );
    case "problem":
      return (
        <ProblemPreviewDrawer problem={item as unknown as BeProblemSearchView | null} onClose={onClose} />
      );
    case "account":
      return <AccountPreviewDrawer account={item as unknown as Account | null} onClose={onClose} />;
    case "project":
      return <ProjectPreviewDrawer project={item as unknown as Project | null} onClose={onClose} />;
    case "user": {
      const user = item ? normalizeUser(item as unknown as User | SnUser) : null;
      return <UserPreviewDrawer user={user} onClose={onClose} />;
    }
    case "product_vulnerability":
      return (
        <ProductVulnerabilityPreviewDrawer
          vulnerability={item as unknown as BeProductVulnerabilityView | null}
          onClose={onClose}
        />
      );
    case "call_request":
      return item ? (
        <CallRequestDetailModal callRequest={item as unknown as BeCallRequestView} onClose={onClose} />
      ) : null;
    default:
      // "task"/"time_card" — see this function's doc comment.
      return null;
  }
}
