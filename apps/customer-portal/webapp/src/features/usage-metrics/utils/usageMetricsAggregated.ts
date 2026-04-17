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

import { colors as oxygenColors } from "@wso2/oxygen-ui";
import type {
  InstanceMetricEntry,
  InstanceUsageEntry,
  UsageAggregatedMetricDefinition,
} from "@features/project-details/types/usage";
import {
  buildDailyCoreTrendFromMetrics,
  buildUsageTrendFromUsages,
  computeUsageHeadlineDelta,
  formatIsoDateForUsageChart,
} from "@features/project-details/utils/usageMetrics";
import {
  USAGE_AGGREGATED_API_COUNT_CAPTION,
  USAGE_AGGREGATED_API_COUNT_TITLE,
  USAGE_AGGREGATED_ORG_COUNT_CAPTION,
  USAGE_AGGREGATED_ORG_COUNT_TITLE,
  USAGE_AGGREGATED_TOTAL_CORES_CAPTION,
  USAGE_AGGREGATED_TOTAL_CORES_TITLE,
  USAGE_AGGREGATED_TOTAL_TRANSACTIONS_CAPTION,
  USAGE_AGGREGATED_TOTAL_TRANSACTIONS_TITLE,
  USAGE_AGGREGATED_TOTAL_USERS_CAPTION,
  USAGE_AGGREGATED_TOTAL_USERS_TITLE,
} from "@features/usage-metrics/constants/usageMetricsConstants";
import { UsageAggregatedMetricCardId } from "@features/usage-metrics/types/usageMetrics";

const ORANGE_STROKE = oxygenColors.orange?.[600] ?? "#EA580C";
const VIOLET_STROKE = oxygenColors.purple?.[500] ?? "#8B5CF6";
const GREEN_STROKE = oxygenColors.green?.[500] ?? "#22C55E";
const CYAN_STROKE = oxygenColors.cyan?.[500] ?? "#06B6D4";
const AMBER_STROKE = oxygenColors.amber?.[500] ?? "#F59E0B";

function resolveAggregatedMetricStroke(id: UsageAggregatedMetricCardId): string {
  switch (id) {
    case UsageAggregatedMetricCardId.TOTAL_TRANSACTIONS:
      return ORANGE_STROKE;
    case UsageAggregatedMetricCardId.TOTAL_USERS:
      return GREEN_STROKE;
    case UsageAggregatedMetricCardId.API_COUNT:
      return VIOLET_STROKE;
    case UsageAggregatedMetricCardId.TOTAL_CORES:
      return AMBER_STROKE;
    case UsageAggregatedMetricCardId.TOTAL_ORGANIZATION_COUNT:
      return CYAN_STROKE;
    default:
      return ORANGE_STROKE;
  }
}

/**
 * Builds the five aggregated metric cards from project usages and metrics APIs.
 *
 * @param usages - Instance usage rows.
 * @param metrics - Instance metric rows.
 * @param isSmallScreen - Compact date labels on narrow screens.
 * @returns Metric definitions for trend cards.
 */
export function deriveAggregatedMetrics(
  usages: InstanceUsageEntry[],
  metrics: InstanceMetricEntry[],
  isSmallScreen: boolean = false,
): UsageAggregatedMetricDefinition[] {
  const periodLabel = (p: string) => formatIsoDateForUsageChart(p, isSmallScreen);
  const dateLabel = (d: string) => formatIsoDateForUsageChart(d, isSmallScreen);

  const txTrend = buildUsageTrendFromUsages(
    usages,
    "TRANSACTION_COUNT",
    periodLabel,
  );
  const apiTrend = buildUsageTrendFromUsages(usages, "API_COUNT", periodLabel);
  const userTrend = buildUsageTrendFromUsages(
    usages,
    "TOTAL_USERS",
    periodLabel,
  );
  const b2bTrend = buildUsageTrendFromUsages(
    usages,
    "TOTAL_B2B_ORGS",
    periodLabel,
  );
  const coreTrend = buildDailyCoreTrendFromMetrics(metrics, dateLabel);

  const txHD = computeUsageHeadlineDelta(txTrend);
  const apiHD = computeUsageHeadlineDelta(apiTrend);
  const userHD = computeUsageHeadlineDelta(userTrend);
  const b2bHD = computeUsageHeadlineDelta(b2bTrend);
  const coreHD = computeUsageHeadlineDelta(coreTrend);

  return [
    {
      id: UsageAggregatedMetricCardId.TOTAL_TRANSACTIONS,
      title: USAGE_AGGREGATED_TOTAL_TRANSACTIONS_TITLE,
      caption: USAGE_AGGREGATED_TOTAL_TRANSACTIONS_CAPTION,
      headlineValue: txHD.headline,
      deltaLabel: txHD.delta,
      stroke: resolveAggregatedMetricStroke(
        UsageAggregatedMetricCardId.TOTAL_TRANSACTIONS,
      ),
      data: txTrend,
    },
    {
      id: UsageAggregatedMetricCardId.TOTAL_USERS,
      title: USAGE_AGGREGATED_TOTAL_USERS_TITLE,
      caption: USAGE_AGGREGATED_TOTAL_USERS_CAPTION,
      headlineValue: userHD.headline,
      deltaLabel: userHD.delta,
      stroke: resolveAggregatedMetricStroke(
        UsageAggregatedMetricCardId.TOTAL_USERS,
      ),
      data: userTrend,
    },
    {
      id: UsageAggregatedMetricCardId.API_COUNT,
      title: USAGE_AGGREGATED_API_COUNT_TITLE,
      caption: USAGE_AGGREGATED_API_COUNT_CAPTION,
      headlineValue: apiHD.headline,
      deltaLabel: apiHD.delta,
      stroke: resolveAggregatedMetricStroke(UsageAggregatedMetricCardId.API_COUNT),
      data: apiTrend,
    },
    {
      id: UsageAggregatedMetricCardId.TOTAL_CORES,
      title: USAGE_AGGREGATED_TOTAL_CORES_TITLE,
      caption: USAGE_AGGREGATED_TOTAL_CORES_CAPTION,
      headlineValue: coreHD.headline,
      deltaLabel: coreHD.delta,
      stroke: resolveAggregatedMetricStroke(
        UsageAggregatedMetricCardId.TOTAL_CORES,
      ),
      data: coreTrend,
    },
    {
      id: UsageAggregatedMetricCardId.TOTAL_ORGANIZATION_COUNT,
      title: USAGE_AGGREGATED_ORG_COUNT_TITLE,
      caption: USAGE_AGGREGATED_ORG_COUNT_CAPTION,
      headlineValue: b2bHD.headline,
      deltaLabel: b2bHD.delta,
      stroke: resolveAggregatedMetricStroke(
        UsageAggregatedMetricCardId.TOTAL_ORGANIZATION_COUNT,
      ),
      data: b2bTrend,
    },
  ];
}
