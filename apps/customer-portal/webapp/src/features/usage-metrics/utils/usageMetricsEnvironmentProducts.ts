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

import { colors } from "@wso2/oxygen-ui";
import type {
  InstanceMetricEntry,
  InstanceUsageEntry,
  UsageEnvironmentProduct,
  UsageInstanceChartBlock,
  UsageProductInstanceRow,
} from "@features/project-details/types/usage";
import {
  buildCoreAverageTrendFromMetrics,
  buildUsageTrendFromUsages,
  computeUsageHeadlineDeltaSigned,
  formatUsageMetricCount,
} from "@features/project-details/utils/usageMetrics";
import {
  USAGE_METRICS_INSTANCE_CHART_CORE_CAPTION,
  USAGE_METRICS_INSTANCE_CHART_CORE_TITLE,
  USAGE_METRICS_INSTANCE_CHART_TX_CAPTION,
  USAGE_METRICS_INSTANCE_CHART_TX_TITLE,
  USAGE_METRICS_PRODUCT_CORE_METRIC_INSTANCES,
  USAGE_METRICS_PRODUCT_CORE_METRIC_TOTAL_CORES,
  USAGE_METRICS_PRODUCT_THIRD_METRIC_LABEL,
  USAGE_METRICS_VALUE_EM_DASH,
} from "@features/usage-metrics/constants/usageMetricsConstants";

/**
 * Short period label (MM-DD) for environment product charts.
 *
 * @param period - ISO period string.
 * @returns Short x-axis label.
 */
export function formatUsageEnvironmentPeriodLabel(period: string): string {
  return period.slice(5);
}

/**
 * Stable key for nested instance accordions under a product.
 *
 * @param productId - Product id.
 * @param instanceId - Instance id.
 * @returns Composite key.
 */
export function buildUsageProductInstanceAccordionKey(
  productId: string,
  instanceId: string,
): string {
  return `${productId}::${instanceId}`;
}

/**
 * Derives product rows from deployment-scoped usages + metrics.
 * Groups entries by deployedProduct.id (falling back to product.id then instanceId).
 *
 * @param usages - Usage rows for the deployment.
 * @param metrics - Metric rows for the deployment.
 * @returns Built product cards for the environment tab.
 */
export function deriveUsageEnvironmentProducts(
  usages: InstanceUsageEntry[],
  metrics: InstanceMetricEntry[],
): UsageEnvironmentProduct[] {
  const productMap = new Map<
    string,
    {
      label: string;
      usages: InstanceUsageEntry[];
      metrics: InstanceMetricEntry[];
    }
  >();

  for (const u of usages) {
    const pid = u.deployedProduct?.id ?? u.product?.id ?? u.instanceId;
    const label = u.deployedProduct?.label ?? u.product?.label ?? u.instanceKey;
    const existing = productMap.get(pid) ?? { label, usages: [], metrics: [] };
    existing.usages.push(u);
    productMap.set(pid, existing);
  }
  for (const m of metrics) {
    const pid = m.deployedProduct?.id ?? m.product?.id ?? m.instanceId;
    const label = m.deployedProduct?.label ?? m.product?.label ?? m.instanceKey;
    const existing = productMap.get(pid) ?? { label, usages: [], metrics: [] };
    existing.metrics.push(m);
    productMap.set(pid, existing);
  }

  return Array.from(productMap.entries()).map(
    ([pid, { label, usages: pUsages, metrics: pMetrics }]) => {
      const txTrend = buildUsageTrendFromUsages(
        pUsages,
        "TRANSACTION_COUNT",
        formatUsageEnvironmentPeriodLabel,
      );
      const coreTrend = buildCoreAverageTrendFromMetrics(pMetrics);
      const totalTx = pUsages.reduce(
        (sum, u) =>
          sum +
          u.periodSummaries.reduce(
            (s, ps) => s + (ps.counts["TRANSACTION_COUNT"] ?? 0),
            0,
          ),
        0,
      );
      const totalCores = pMetrics.reduce(
        (sum, m) =>
          sum +
          m.dataPoints.reduce((s, dp) => {
            const c =
              dp.coreCount != null
                ? dp.coreCount
                : dp.deploymentMetadata?.numberOfCores != null
                  ? Number(dp.deploymentMetadata.numberOfCores)
                  : 0;
            return s + c;
          }, 0),
        0,
      );
      const totalApis = pUsages.reduce(
        (sum, u) =>
          sum +
          u.periodSummaries.reduce(
            (s, ps) => s + (ps.counts["API_COUNT"] ?? 0),
            0,
          ),
        0,
      );

      const instanceRows: UsageProductInstanceRow[] = pUsages.map((u) => {
        const instUsageTrend = buildUsageTrendFromUsages(
          [u],
          "TRANSACTION_COUNT",
          formatUsageEnvironmentPeriodLabel,
        );
        const instTxHD = computeUsageHeadlineDeltaSigned(instUsageTrend);
        const instMetric = pMetrics.find((m) => m.instanceId === u.instanceId);
        const instCoreTrend = instMetric
          ? buildCoreAverageTrendFromMetrics([instMetric])
          : [];
        const lastCore =
          instMetric?.dataPoints.at(-1)?.coreCount ??
          (instMetric?.dataPoints.at(-1)?.deploymentMetadata?.numberOfCores !=
          null
            ? Number(
                instMetric.dataPoints.at(-1)!.deploymentMetadata!.numberOfCores,
              )
            : 0) ??
          0;
        const coreHD = computeUsageHeadlineDeltaSigned(
          instCoreTrend.map((r) => ({ name: r.name, value: r.current })),
        );

        const javaVer =
          instMetric?.dataPoints.at(-1)?.jdkVersion ??
          instMetric?.dataPoints.at(-1)?.deploymentMetadata?.jdkVersion ??
          USAGE_METRICS_VALUE_EM_DASH;
        const u2Level =
          instMetric?.dataPoints.at(-1)?.deploymentMetadata?.updateLevel ??
          USAGE_METRICS_VALUE_EM_DASH;
        const instanceStroke = colors.blue?.[500] ?? "#3B82F6";

        const txBlock: UsageInstanceChartBlock = {
          title: USAGE_METRICS_INSTANCE_CHART_TX_TITLE,
          caption: USAGE_METRICS_INSTANCE_CHART_TX_CAPTION,
          headlineValue: instTxHD.headline,
          deltaLabel: instTxHD.delta,
          deltaPositive: instTxHD.deltaPositive,
          stroke: instanceStroke,
          data: instUsageTrend,
        };
        const coresBlock: UsageInstanceChartBlock = {
          title: USAGE_METRICS_INSTANCE_CHART_CORE_TITLE,
          caption: USAGE_METRICS_INSTANCE_CHART_CORE_CAPTION,
          headlineValue: String(lastCore),
          deltaLabel: coreHD.delta,
          deltaPositive: coreHD.deltaPositive,
          stroke: colors.orange?.[500] ?? "#F97316",
          data: instCoreTrend.map((r) => ({ name: r.name, value: r.current })),
        };

        return {
          id: u.instanceId,
          hostName: u.instanceId,
          javaVersion: javaVer,
          u2Level,
          transactionsLabel: instTxHD.headline,
          coreCount: lastCore,
          charts: { transactions: txBlock, cores: coresBlock },
        };
      });

      return {
        id: pid,
        name: label,
        version: "",
        runningInstances: pUsages.length,
        transactionsLabel: formatUsageMetricCount(totalTx),
        thirdMetricLabel: USAGE_METRICS_PRODUCT_THIRD_METRIC_LABEL,
        thirdMetricValue: String(totalApis),
        coreMetrics: [
          {
            label: USAGE_METRICS_PRODUCT_CORE_METRIC_TOTAL_CORES,
            value: String(totalCores),
          },
          {
            label: USAGE_METRICS_PRODUCT_CORE_METRIC_INSTANCES,
            value: String(pUsages.length),
          },
        ],
        transactionTrend: txTrend,
        coreUsageTrend: coreTrend,
        instances: instanceRows,
      };
    },
  );
}
