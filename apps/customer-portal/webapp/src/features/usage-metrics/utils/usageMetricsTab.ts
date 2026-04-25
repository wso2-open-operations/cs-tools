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

import type { TabOption } from "@components/tab-bar/TabBar";
import { ChartColumn } from "@wso2/oxygen-ui-icons-react";
import { UsageTimeRange } from "@features/project-details/types/usage";
import {
  USAGE_METRICS_DEPLOYMENT_TAB_PREFIX,
  USAGE_METRICS_TAB_OVERVIEW_LABEL,
} from "@features/usage-metrics/constants/usageMetricsConstants";
import {
  UsageMetricsInnerTabId,
  type UsageDateRangeIso,
} from "@features/usage-metrics/types/usageMetrics";

export type { UsageDateRangeIso };

type DeploymentTabSource = { id: string; name: string };

/**
 * ISO date strings (YYYY-MM-DD) for a non-custom preset relative to today.
 *
 * @param preset - Three, six, or twelve month window (not CUSTOM).
 * @returns Start and end dates in local calendar terms via ISO slice.
 */
export function resolveUsagePresetDateRange(
  preset: UsageTimeRange,
): UsageDateRangeIso {
  const end = new Date();
  const start = new Date(end);

  switch (preset) {
    case UsageTimeRange.ONE_MONTH:
      start.setMonth(start.getMonth() - 1);
      break;
    case UsageTimeRange.THREE_MONTHS:
      start.setMonth(start.getMonth() - 3);
      break;
    case UsageTimeRange.SIX_MONTHS:
      start.setMonth(start.getMonth() - 6);
      break;
    case UsageTimeRange.TWELVE_MONTHS:
      start.setFullYear(start.getFullYear() - 1);
      break;
    default:
      start.setMonth(start.getMonth() - 3);
      break;
  }

  return {
    startDate: start.toISOString().slice(0, 10),
    endDate: end.toISOString().slice(0, 10),
  };
}

/**
 * Overview tab plus one tab per deployment for Usage & Metrics inner navigation.
 *
 * @param deployments - Deployments from search-all API.
 * @returns Tab options for TabBar.
 */
export function buildUsageInnerTabs(
  deployments: DeploymentTabSource[] | undefined,
): TabOption[] {
  const overviewTab: TabOption = {
    id: UsageMetricsInnerTabId.OVERVIEW,
    label: USAGE_METRICS_TAB_OVERVIEW_LABEL,
    icon: ChartColumn,
  };
  if (!deployments || deployments.length === 0) {
    return [overviewTab];
  }
  const envTabs: TabOption[] = deployments.map((dep) => ({
    id: `${USAGE_METRICS_DEPLOYMENT_TAB_PREFIX}${dep.id}`,
    label: dep.name,
  }));
  return [overviewTab, ...envTabs];
}

/**
 * Resolves selected deployment id from inner tab id, or null for overview.
 *
 * @param innerTab - Active inner tab id.
 * @returns Deployment id when an environment tab is selected.
 */
export function getActiveUsageDeploymentId(innerTab: string): string | null {
  if (innerTab === UsageMetricsInnerTabId.OVERVIEW) {
    return null;
  }
  if (innerTab.startsWith(USAGE_METRICS_DEPLOYMENT_TAB_PREFIX)) {
    return innerTab.slice(USAGE_METRICS_DEPLOYMENT_TAB_PREFIX.length);
  }
  return null;
}

/**
 * Short button label for fixed presets (3M / 6M / 12M).
 *
 * @param preset - Time range enum value.
 * @returns Two-character style label for toolbar buttons.
 */
export function getUsagePresetShortLabel(preset: UsageTimeRange): string {
  switch (preset) {
    case UsageTimeRange.ONE_MONTH:
      return "1M";
    case UsageTimeRange.THREE_MONTHS:
      return "3M";
    case UsageTimeRange.SIX_MONTHS:
      return "6M";
    case UsageTimeRange.TWELVE_MONTHS:
      return "12M";
    default:
      return "";
  }
}
