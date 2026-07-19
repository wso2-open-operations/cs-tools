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
  ProductChartTrend,
  UsageEnvironmentProduct,
  UsageInstanceChartBlock,
  UsageProductInstanceRow,
} from "@features/project-details/types/usage";
import type { DeploymentProductItem } from "@features/project-details/types/deployments";
import {
  buildCoreAverageTrendFromMetrics,
  buildUsageTrendFromUsages,
  computeInstanceAndCoreSummary,
  computeSeriesSummary,
  computeUsageHeadlineDeltaSigned,
  formatUsageMetricCount,
} from "@features/project-details/utils/usageMetrics";
import {
  USAGE_METRICS_INSTANCE_CHART_CORE_CAPTION,
  USAGE_METRICS_INSTANCE_CHART_CORE_TITLE,
  USAGE_METRICS_PRODUCT_CORE_METRIC_INSTANCES,
  USAGE_METRICS_PRODUCT_CORE_METRIC_TOTAL_CORES,
  USAGE_METRICS_UNKNOWN_LABEL,
  USAGE_METRICS_VALUE_EM_DASH,
} from "@features/usage-metrics/constants/usageMetricsConstants";
import {
  CORE_CHART_CONFIG,
  METRIC_CHART_CONFIG,
  METRIC_CHART_CONFIG_FALLBACK,
  getProductMetricKeys,
} from "@features/usage-metrics/utils/usageMetricsProductClassifier";

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
 * Derives product rows for a deployment.
 * Products (name, version) come from the deployment's authoritative products/search
 * result so every deployed product appears even without usage data; usage/metric
 * entries are then attached by matching `deployedProduct.id` to the product id.
 * Metric keys and chart trends are determined per-product by the WSO2 product classifier.
 *
 * @param products - Deployed products for the deployment (from products/search).
 * @param usages - Usage rows for the deployment.
 * @param metrics - Metric rows for the deployment.
 * @returns Built product cards for the environment tab.
 */
export function deriveUsageEnvironmentProducts(
  products: DeploymentProductItem[],
  usages: InstanceUsageEntry[],
  metrics: InstanceMetricEntry[],
): UsageEnvironmentProduct[] {
  const productMap = new Map<
    string,
    {
      label: string;
      version: string;
      usages: InstanceUsageEntry[];
      metrics: InstanceMetricEntry[];
    }
  >();

  for (const p of products) {
    const label = p.product?.label ?? USAGE_METRICS_UNKNOWN_LABEL;
    const version =
      typeof p.version === "string"
        ? p.version
        : (p.version?.label ?? "");
    productMap.set(p.id, { label, version, usages: [], metrics: [] });
  }

  for (const u of usages) {
    const pid = u.deployedProduct?.id;
    if (!pid || !productMap.has(pid)) continue;
    productMap.get(pid)!.usages.push(u);
  }
  for (const m of metrics) {
    const pid = m.deployedProduct?.id;
    if (!pid || !productMap.has(pid)) continue;
    productMap.get(pid)!.metrics.push(m);
  }

  return Array.from(productMap.entries()).map(
    ([pid, { label, version, usages: pUsages, metrics: pMetrics }]) => {
      const metricKeys = getProductMetricKeys(label, version);

      // Compute total for each metric key across all usage entries
      const metricTotals = new Map<string, number>();
      for (const key of metricKeys) {
        const total = pUsages.reduce(
          (sum, u) =>
            sum + u.periodSummaries.reduce((s, ps) => s + (ps.counts[key] ?? 0), 0),
          0,
        );
        metricTotals.set(key, total);
      }

      // Product-level summary stats (shown in accordion header)
      const summaryStats = metricKeys.map((key) => {
        const cfg = METRIC_CHART_CONFIG[key] ?? METRIC_CHART_CONFIG_FALLBACK;
        const total = metricTotals.get(key) ?? 0;
        return {
          label: cfg.title,
          value: formatUsageMetricCount(total),
        };
      });

      // Product-level chart trends (shown in expanded view)
      const coreTrend = buildCoreAverageTrendFromMetrics(pMetrics);
      const chartTrends: ProductChartTrend[] = [
        ...metricKeys.map((key) => {
          const cfg = METRIC_CHART_CONFIG[key] ?? METRIC_CHART_CONFIG_FALLBACK;
          return {
            title: cfg.title,
            caption: cfg.caption,
            stroke: cfg.stroke,
            data: buildUsageTrendFromUsages(
              pUsages,
              key,
              formatUsageEnvironmentPeriodLabel,
            ),
          };
        }),
        {
          title: CORE_CHART_CONFIG.title,
          caption: CORE_CHART_CONFIG.caption,
          stroke: CORE_CHART_CONFIG.stroke,
          data: coreTrend.map((r) => ({ name: r.name, value: r.current })),
        },
      ];

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

      const instanceRows: UsageProductInstanceRow[] = pUsages.map((u) => {
        const instMetric = pMetrics.find((m) => m.instanceId === u.instanceId);

        // Instance-level stats: one formatted value per metric key
        const instSummaryStats = metricKeys.map((key) => {
          const cfg = METRIC_CHART_CONFIG[key] ?? METRIC_CHART_CONFIG_FALLBACK;
          const total = u.periodSummaries.reduce(
            (s, ps) => s + (ps.counts[key] ?? 0),
            0,
          );
          return { label: cfg.title, value: formatUsageMetricCount(total) };
        });

        // Instance-level charts: one per metric key + cores
        const instCoreTrend = instMetric
          ? buildCoreAverageTrendFromMetrics([instMetric])
          : [];
        const instCoreValues = (instMetric?.dataPoints ?? [])
          .slice()
          .sort((a, b) => a.date.localeCompare(b.date))
          .map((dp) =>
            dp.coreCount != null
              ? dp.coreCount
              : dp.deploymentMetadata?.numberOfCores != null
                ? Number(dp.deploymentMetadata.numberOfCores)
                : 0,
          );
        const instCoreSummary = computeSeriesSummary(instCoreValues);
        const coreHD = computeUsageHeadlineDeltaSigned(
          instCoreTrend.map((r) => ({ name: r.name, value: r.current })),
        );

        const instCharts: UsageInstanceChartBlock[] = [
          ...metricKeys.map((key) => {
            const cfg = METRIC_CHART_CONFIG[key] ?? METRIC_CHART_CONFIG_FALLBACK;
            const instTrend = buildUsageTrendFromUsages(
              [u],
              key,
              formatUsageEnvironmentPeriodLabel,
            );
            const instHD = computeUsageHeadlineDeltaSigned(instTrend);
            return {
              title: cfg.title,
              caption: cfg.caption,
              headlineValue: instHD.headline,
              deltaLabel: instHD.delta,
              deltaPositive: instHD.deltaPositive,
              stroke: cfg.stroke,
              data: instTrend,
            };
          }),
          {
            title: USAGE_METRICS_INSTANCE_CHART_CORE_TITLE,
            caption: USAGE_METRICS_INSTANCE_CHART_CORE_CAPTION,
            headlineValue: String(instCoreSummary.curr),
            deltaLabel: coreHD.delta,
            deltaPositive: coreHD.deltaPositive,
            stroke: colors.orange?.[500] ?? "#F97316",
            data: instCoreTrend.map((r) => ({ name: r.name, value: r.current })),
          },
        ];

        // Sort ascending so .at(-1) is always the most recent data point,
        // regardless of whether SN returns newest-first or oldest-first.
        const sortedDps = [...(instMetric?.dataPoints ?? [])].sort((a, b) =>
          (a.date ?? "").localeCompare(b.date ?? ""),
        );
        const lastDp = sortedDps.at(-1);
        const javaVer = (
          lastDp?.jdkVersion ??
          lastDp?.deploymentMetadata?.jdkVersion ??
          USAGE_METRICS_VALUE_EM_DASH
        ).replace(/^"|"$/g, "");
        const u2Level =
          lastDp?.deploymentMetadata?.updateLevel ?? USAGE_METRICS_VALUE_EM_DASH;
        const dm = lastDp?.deploymentMetadata;
        const os = dm?.os
          ? dm.osVersion
            ? `${dm.os} ${dm.osVersion}`
            : dm.os
          : USAGE_METRICS_VALUE_EM_DASH;

        return {
          id: u.instanceId,
          hostName: u.instanceKey,
          os,
          javaVersion: javaVer,
          u2Level,
          summaryStats: instSummaryStats,
          coreSummary: instCoreSummary,
          charts: instCharts,
        };
      });

      const { instanceSummary, coreSummary } =
        computeInstanceAndCoreSummary(pMetrics);

      return {
        id: pid,
        name: label,
        version,
        runningInstances: pUsages.length,
        metricKeys,
        summaryStats,
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
        chartTrends,
        instances: instanceRows,
        instanceSummary,
        coreSummary,
      };
    },
  );
}
