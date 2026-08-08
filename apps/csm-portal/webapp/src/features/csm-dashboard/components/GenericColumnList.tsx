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

import { Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { BeDashboardWidgetColumn, BeWidgetResourceType } from "@api/backend/types";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import DashboardMiniTable from "@features/csm-dashboard/components/DashboardMiniTable";
import { formatColumnValue, resolveColumnPath } from "@features/csm-dashboard/utils/resolveWidgetColumn";

export interface GenericColumnListProps {
  items: Record<string, unknown>[];
  isLoading: boolean;
  resourceType: BeWidgetResourceType;
  columns: BeDashboardWidgetColumn[];
}

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
 * Row links reuse `WIDGET_RESOURCE_CONFIG[resourceType].detailHref` — the
 * same per-resourceType destination each hardcoded renderer already links
 * to — so a columns-configured widget's rows are still real, navigable
 * links wherever that resourceType has a standalone detail page.
 */
export default function GenericColumnList({
  items,
  isLoading,
  resourceType,
  columns,
}: GenericColumnListProps): JSX.Element {
  const config = WIDGET_RESOURCE_CONFIG[resourceType];

  return (
    <DashboardMiniTable
      isLoading={isLoading}
      emptyMessage="No records match this widget's filters."
      columns={columns.map((c) => ({ label: c.label }))}
      rows={items.map((item, i) => {
        const id = typeof item.id === "string" ? item.id : undefined;
        return {
          key: id ?? `row-${i}`,
          href: config?.detailHref?.(item),
          cells: columns.map((c) => (
            <Typography key={c.path} variant="body2" noWrap>
              {formatColumnValue(resolveColumnPath(item, c.path), c.format)}
            </Typography>
          )),
        };
      })}
    />
  );
}
