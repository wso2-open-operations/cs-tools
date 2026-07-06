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

import type {
  CurrMinMaxAvg,
  InstanceMetricEntry,
  InstanceUsageEntry,
  UsageTrendRow,
} from "@features/project-details/types/usage";

/**
 * Formats ISO date (YYYY-MM-DD) for chart axis labels.
 *
 * @param isoDate - Date string from API.
 * @param isSmallScreen - When true, omit year line.
 * @returns Short label for Recharts.
 */
export function formatIsoDateForUsageChart(
  isoDate: string,
  _isSmallScreen: boolean = false,
): string {
  const date = new Date(`${isoDate}T00:00:00Z`);
  if (isNaN(date.getTime())) {
    return isoDate.slice(5);
  }
  const mm = String(date.getUTCMonth() + 1).padStart(2, "0");
  const dd = String(date.getUTCDate()).padStart(2, "0");
  return `${mm}/${dd}`;
}

/**
 * Human-readable count for metric headlines (K / M).
 *
 * @param n - Raw number.
 * @returns Compact string.
 */
export function formatUsageMetricCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

/**
 * Sums TRANSACTION_COUNT across period summaries for one usage entry.
 *
 * @param entry - Instance usage row.
 * @returns Total transactions.
 */
export function sumUsageEntryTransactions(entry: InstanceUsageEntry): number {
  return entry.periodSummaries.reduce(
    (total, ps) => total + (ps.counts["TRANSACTION_COUNT"] ?? 0),
    0,
  );
}

/**
 * Builds trend rows from usages by period, using a formatter for axis names.
 *
 * @param usages - Usage entries.
 * @param countKey - Counter key in period summaries.
 * @param formatPeriodLabel - Maps period string to chart name.
 * @returns Sorted trend rows.
 */
export function buildUsageTrendFromUsages(
  usages: InstanceUsageEntry[],
  countKey: string,
  formatPeriodLabel: (period: string) => string,
): UsageTrendRow[] {
  const periodMap = new Map<string, number>();
  for (const entry of usages) {
    for (const ps of entry.periodSummaries) {
      const prev = periodMap.get(ps.period) ?? 0;
      periodMap.set(ps.period, prev + (ps.counts[countKey] ?? 0));
    }
  }
  return Array.from(periodMap.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([period, value]) => ({
      name: formatPeriodLabel(period),
      value,
    }));
}

/**
 * Daily summed cores across instance metrics (for overview core card).
 *
 * @param metrics - Instance metric entries.
 * @param formatDateLabel - Maps date string to chart name.
 * @returns Sorted trend rows.
 */
export function buildDailyCoreTrendFromMetrics(
  metrics: InstanceMetricEntry[],
  formatDateLabel: (isoDate: string) => string,
): UsageTrendRow[] {
  const dayMap = new Map<string, number>();

  for (const m of metrics) {
    for (const dp of m.dataPoints) {
      if (!dp.date) continue;
      const c =
        dp.coreCount != null
          ? dp.coreCount
          : dp.deploymentMetadata?.numberOfCores != null
            ? Number(dp.deploymentMetadata.numberOfCores)
            : 0;
      const prev = dayMap.get(dp.date) ?? 0;
      dayMap.set(dp.date, prev + c);
    }
  }

  return Array.from(dayMap.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, total]) => ({
      name: formatDateLabel(date),
      value: total,
    }));
}

/**
 * Headline and delta from the last two trend points.
 *
 * @param trend - Chart data.
 * @returns Headline count string and signed delta percentage.
 */
export function computeUsageHeadlineDelta(trend: UsageTrendRow[]): {
  headline: string;
  delta: string;
} {
  if (trend.length === 0) return { headline: "—", delta: "—" };
  const last = trend[trend.length - 1].value ?? 0;
  if (trend.length === 1) {
    return { headline: formatUsageMetricCount(last), delta: "—" };
  }
  const prev = trend[trend.length - 2].value ?? 0;
  const pct = prev === 0 ? 0 : ((last - prev) / prev) * 100;
  const sign = pct >= 0 ? "+" : "";
  return {
    headline: formatUsageMetricCount(last),
    delta: `${sign}${pct.toFixed(1)}%`,
  };
}

/**
 * Like {@link computeUsageHeadlineDelta} but includes deltaPositive for charts.
 *
 * @param trend - Chart rows (value field).
 * @returns Headline, delta label, and trend direction flag.
 */
export function computeUsageHeadlineDeltaSigned(trend: UsageTrendRow[]): {
  headline: string;
  delta: string;
  deltaPositive: boolean;
} {
  if (trend.length === 0) {
    return { headline: "—", delta: "—", deltaPositive: true };
  }
  const last = trend[trend.length - 1].value ?? 0;
  if (trend.length === 1) {
    return {
      headline: formatUsageMetricCount(last),
      delta: "—",
      deltaPositive: true,
    };
  }
  const prev = trend[trend.length - 2].value ?? 0;
  const pct = prev === 0 ? 0 : ((last - prev) / prev) * 100;
  const sign = pct >= 0 ? "+" : "";
  return {
    headline: formatUsageMetricCount(last),
    delta: `${sign}${pct.toFixed(1)}%`,
    deltaPositive: pct >= 0,
  };
}

/**
 * Computes curr/avg/min/max from a sorted values array.
 * curr = last value, min/max/avg from all values (ignoring leading zeros only when all are zero).
 *
 * @param values - Time-ordered values (oldest → newest).
 * @returns Summary stats.
 */
export function computeSeriesSummary(values: number[]): CurrMinMaxAvg {
  if (values.length === 0) return { curr: 0, avg: 0, min: 0, max: 0 };
  const curr = values[values.length - 1];
  const min = Math.min(...values);
  const max = Math.max(...values);
  const avg = Math.round(values.reduce((a, b) => a + b, 0) / values.length);
  return { curr, min, max, avg };
}

/**
 * Derives per-period counts for instances, products, and environments.
 * - Instances: derived from metric dataPoints (per date, count distinct instanceIds).
 * - Products / Environments: derived from usage periodSummaries (per period, count distinct IDs).
 *   Usages reliably carry deployment.id and deployedProduct.id; metrics often have them null.
 *
 * @param metrics - Project-wide instance metric entries.
 * @param usages - Project-wide instance usage entries.
 * @returns Summary stats for instances, products, and environments.
 */
export function computeOverviewSummaries(
  metrics: InstanceMetricEntry[],
  usages: InstanceUsageEntry[],
): {
  instanceSummary: CurrMinMaxAvg;
  productSummary: CurrMinMaxAvg;
  environmentSummary: CurrMinMaxAvg;
} {
  // Instances — from metric dataPoints (daily granularity)
  const dayInstances = new Map<string, Set<string>>();
  for (const m of metrics) {
    for (const dp of m.dataPoints) {
      if (!dp.date) continue;
      const s = dayInstances.get(dp.date) ?? new Set<string>();
      s.add(m.instanceId);
      dayInstances.set(dp.date, s);
    }
  }
  const sortedDays = Array.from(dayInstances.keys()).sort();
  const instanceSummary = computeSeriesSummary(
    sortedDays.map((d) => dayInstances.get(d)?.size ?? 0),
  );

  // Products / Environments — from usage periodSummaries (period granularity)
  const periodProducts = new Map<string, Set<string>>();
  const periodEnvironments = new Map<string, Set<string>>();
  for (const u of usages) {
    const prodId = u.deployedProduct?.id ?? u.product?.id;
    const envId = u.deployment?.id;
    for (const ps of u.periodSummaries) {
      if (prodId) {
        const s = periodProducts.get(ps.period) ?? new Set<string>();
        s.add(prodId);
        periodProducts.set(ps.period, s);
      }
      if (envId) {
        const s = periodEnvironments.get(ps.period) ?? new Set<string>();
        s.add(envId);
        periodEnvironments.set(ps.period, s);
      }
    }
  }
  const sortedPeriods = Array.from(
    new Set([...periodProducts.keys(), ...periodEnvironments.keys()]),
  ).sort();
  const productSummary = computeSeriesSummary(
    sortedPeriods.map((p) => periodProducts.get(p)?.size ?? 0),
  );
  const environmentSummary = computeSeriesSummary(
    sortedPeriods.map((p) => periodEnvironments.get(p)?.size ?? 0),
  );

  return { instanceSummary, productSummary, environmentSummary };
}

/**
 * Derives instance-count and core-count curr/avg/min/max from metric data points.
 * Instance count per date = number of distinct instances that have a data point on that date.
 * Core count per date = sum of coreCount across all data points on that date.
 *
 * @param metrics - Instance metric entries for a product or project scope.
 * @returns Instance and core summaries.
 */
export function computeInstanceAndCoreSummary(metrics: InstanceMetricEntry[]): {
  instanceSummary: CurrMinMaxAvg;
  coreSummary: CurrMinMaxAvg;
} {
  const dayInstances = new Map<string, Set<string>>();
  const dayCores = new Map<string, number>();

  for (const m of metrics) {
    for (const dp of m.dataPoints) {
      if (!dp.date) continue;
      const instSet = dayInstances.get(dp.date) ?? new Set<string>();
      instSet.add(m.instanceId);
      dayInstances.set(dp.date, instSet);

      const cores =
        dp.coreCount != null
          ? dp.coreCount
          : dp.deploymentMetadata?.numberOfCores != null
            ? Number(dp.deploymentMetadata.numberOfCores)
            : 0;
      dayCores.set(dp.date, (dayCores.get(dp.date) ?? 0) + cores);
    }
  }

  const sortedDates = Array.from(
    new Set([...dayInstances.keys(), ...dayCores.keys()]),
  ).sort();

  const instanceValues = sortedDates.map((d) => dayInstances.get(d)?.size ?? 0);
  const coreValues = sortedDates.map((d) => dayCores.get(d) ?? 0);

  return {
    instanceSummary: computeSeriesSummary(instanceValues),
    coreSummary: computeSeriesSummary(coreValues),
  };
}

/**
 * Core + average trend from deployment-scoped metrics (product drill-down).
 *
 * @param metrics - Instance metrics for one product.
 * @returns Rows with current vs average cores per day.
 */
export function buildCoreAverageTrendFromMetrics(
  metrics: InstanceMetricEntry[],
): UsageTrendRow[] {
  const periodMap = new Map<string, { total: number; count: number }>();
  for (const entry of metrics) {
    for (const dp of entry.dataPoints) {
      const cores =
        dp.coreCount != null
          ? dp.coreCount
          : dp.deploymentMetadata?.numberOfCores != null
            ? Number(dp.deploymentMetadata.numberOfCores)
            : 0;
      const existing = periodMap.get(dp.date) ?? { total: 0, count: 0 };
      existing.total += cores;
      existing.count += 1;
      periodMap.set(dp.date, existing);
    }
  }
  return Array.from(periodMap.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, { total, count }]) => ({
      name: date.slice(5),
      current: total,
      average: count > 0 ? Math.round(total / count) : 0,
    }));
}
