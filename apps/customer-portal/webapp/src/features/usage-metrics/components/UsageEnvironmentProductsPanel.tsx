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

import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Card,
  CircularProgress,
  Grid,
  Skeleton,
  Typography,
  alpha,
  colors,
} from "@wso2/oxygen-ui";
import { ChevronDown, Package, Server } from "@wso2/oxygen-ui-icons-react";
import EmptyState from "@components/empty-state/EmptyState";
import { LineChart } from "@wso2/oxygen-ui-charts-react";
import type { JSX } from "react";
import { useMemo } from "react";
import {
  USAGE_LINE_CHART_MARGIN,
  USAGE_METRICS_ENVIRONMENT_PRODUCTS_ERROR,
  USAGE_METRICS_NO_INSTANCE_DATA,
  USAGE_METRICS_NO_PRODUCTS_IN_DEPLOYMENT,
  USAGE_METRICS_PRODUCT_DATA_UNAVAILABLE,
  USAGE_METRICS_PRODUCT_INSTANCES_SECTION,
  USAGE_METRICS_PRODUCT_INSTANCE_METRICS,
  USAGE_METRICS_PRODUCT_CORE_METRICS,
  USAGE_METRICS_STAT_LABEL_AVG,
  USAGE_METRICS_STAT_LABEL_MIN,
  USAGE_METRICS_STAT_LABEL_MAX,
  USAGE_METRICS_UNKNOWN_LABEL,
} from "@features/usage-metrics/constants/usageMetricsConstants";
import { UsageChartSurface } from "@features/usage-metrics/components/UsageChartSurface";
import { usePostDeploymentProductsSearchAll } from "@features/project-details/api/usePostDeploymentProductsSearch";
import usePostDeploymentProductMetricsSearch from "@features/project-details/api/usePostDeploymentProductMetricsSearch";
import usePostDeploymentProductUsageCountsSearch from "@features/project-details/api/usePostDeploymentProductUsageCountsSearch";
import { computeSeriesSummary, formatUsageMetricCount } from "@features/project-details/utils/usageMetrics";
import type { CurrMinMaxAvg } from "@features/project-details/types/usage";
import type {
  MetricPillProps,
  UsageEnvironmentProductsPanelProps,
} from "@features/usage-metrics/types/usageMetrics";
import { getUsageEnvironmentPanelAccent } from "@features/usage-metrics/utils/usageMetricsAccent";
import {
  CORE_CHART_CONFIG,
  METRIC_CHART_CONFIG,
  METRIC_CHART_CONFIG_FALLBACK,
  getProductMetricKeys,
} from "@features/usage-metrics/utils/usageMetricsProductClassifier";

const ZERO_SUMMARY: CurrMinMaxAvg = { curr: 0, avg: 0, min: 0, max: 0 };

/** Short period label (MM-DD) for chart x-axes. */
function formatPeriodLabel(date: string): string {
  return date.slice(5);
}

// ─── Curr/Avg/Min/Max metric block ────────────────────────────────────────────

function CurrMinMaxBlock({
  label,
  summary,
}: {
  label: string;
  summary: CurrMinMaxAvg;
}): JSX.Element {
  const cols = [
    { l: USAGE_METRICS_STAT_LABEL_AVG, v: summary.avg },
    { l: USAGE_METRICS_STAT_LABEL_MIN, v: summary.min },
    { l: USAGE_METRICS_STAT_LABEL_MAX, v: summary.max },
  ];

  return (
    <Box>
      <Typography
        variant="body2"
        sx={{ fontWeight: 700, display: "block", mb: 1 }}
      >
        {label}
      </Typography>
      <Box sx={{ display: "flex", gap: 2 }}>
        {cols.map(({ l, v }) => (
          <Box key={l} sx={{ textAlign: "left" }}>
            <Typography
              variant="caption"
              color="text.secondary"
              display="block"
              sx={{ mb: 0.25 }}
            >
              {l}
            </Typography>
            <Typography variant="body1" sx={{ fontWeight: 700 }}>
              {v}
            </Typography>
          </Box>
        ))}
      </Box>
    </Box>
  );
}

function MetricPill({ icon, label, value }: MetricPillProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
      {icon}
      <Box>
        <Typography variant="caption" color="text.secondary">
          {label}
        </Typography>
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          {value}
        </Typography>
      </Box>
    </Box>
  );
}

// ─── Per-product accordion row (fetches its own metrics + usage counts) ──────

type ProductRowMeta = {
  id: string;
  name: string;
  version: string;
};

function ProductAccordionRow({
  product,
  deploymentId,
  dateRange,
  expanded,
  onToggle,
}: {
  product: ProductRowMeta;
  deploymentId: string;
  dateRange: { startDate: string; endDate: string };
  expanded: boolean;
  onToggle: () => void;
}): JSX.Element {
  const a = getUsageEnvironmentPanelAccent(deploymentId);

  const payload = useMemo(
    () => ({
      startDate: dateRange.startDate,
      endDate: dateRange.endDate,
    }),
    [dateRange],
  );

  const { data: metricsData, isLoading: metricsLoading } =
    usePostDeploymentProductMetricsSearch(deploymentId, product.id, payload);
  const { data: countsData, isLoading: countsLoading } =
    usePostDeploymentProductUsageCountsSearch(deploymentId, product.id, payload);

  const isLoading = metricsLoading || countsLoading;

  const metricKeys = useMemo(
    () => getProductMetricKeys(product.name, product.version),
    [product.name, product.version],
  );

  const coreSummary = useMemo((): CurrMinMaxAvg => {
    if (!metricsData) return ZERO_SUMMARY;
    return {
      curr: 0,
      avg: metricsData.summary.avgCores,
      min: metricsData.summary.minCores,
      max: metricsData.summary.maxCores,
    };
  }, [metricsData]);

  const instanceSummary = useMemo((): CurrMinMaxAvg => {
    if (!metricsData?.chartData?.length) return ZERO_SUMMARY;
    return computeSeriesSummary(metricsData.chartData.map((d) => d.instanceCount));
  }, [metricsData]);

  const countTypes = countsData?.summary?.countTypes;

  // Show every count type the API actually returned, ordered by the classifier's
  // known priority first, then any additional keys the classifier didn't anticipate.
  const countTypeKeys = useMemo(() => {
    if (!countTypes) return [];
    const keys = Object.keys(countTypes);
    const known = metricKeys.filter((key) => keys.includes(key));
    const extra = keys.filter((key) => !metricKeys.includes(key));
    return [...known, ...extra];
  }, [countTypes, metricKeys]);

  const summaryStats = useMemo(() => {
    if (!countTypes) return [];
    return countTypeKeys
      .map((key) => {
        const cfg = METRIC_CHART_CONFIG[key] ?? METRIC_CHART_CONFIG_FALLBACK;
        const stat = countTypes[key];
        let value: number;
        switch (stat.aggregation) {
          case "min":
            value = stat.min;
            break;
          case "avg":
            value = stat.avg;
            break;
          case "max":
          default:
            // "sum" and any other undocumented aggregations aren't captured as a
            // discrete stat field, so fall back to max as the most representative value.
            value = stat.max ?? 0;
            break;
        }
        return { label: cfg.title, value: formatUsageMetricCount(value) };
      });
  }, [countTypes, countTypeKeys]);

  const coreTrend = useMemo(() => {
    const sorted = [...(metricsData?.chartData ?? [])].sort((x, y) =>
      x.date.localeCompare(y.date),
    );
    return sorted.map((d) => ({ name: formatPeriodLabel(d.date), value: d.avgCores }));
  }, [metricsData]);

  const countTrends = useMemo(() => {
    const sortedCharts = [...(countsData?.chartData ?? [])].sort((x, y) =>
      x.date.localeCompare(y.date),
    );
    return countTypeKeys
      .map((key) => {
        const cfg = METRIC_CHART_CONFIG[key] ?? METRIC_CHART_CONFIG_FALLBACK;
        return {
          key,
          title: cfg.title,
          caption: cfg.caption,
          stroke: cfg.stroke,
          data: sortedCharts.map((point) => ({
            name: formatPeriodLabel(point.date),
            value: point.counts[key]?.value ?? 0,
          })),
        };
      });
  }, [countsData, countTypeKeys]);

  const chartTrends = useMemo(
    () => [
      ...countTrends,
      {
        key: "CORE_USAGE",
        title: CORE_CHART_CONFIG.title,
        caption: CORE_CHART_CONFIG.caption,
        stroke: CORE_CHART_CONFIG.stroke,
        data: coreTrend,
      },
    ],
    [countTrends, coreTrend],
  );

  const latestInstances = useMemo(() => {
    const sorted = [...(metricsData?.chartData ?? [])].sort((x, y) =>
      y.date.localeCompare(x.date),
    );
    return sorted[0]?.instances ?? [];
  }, [metricsData]);

  // Nothing to show for this product if neither source has any data for this range.
  const hasChartData =
    (metricsData?.chartData?.length ?? 0) > 0 ||
    (countsData?.chartData?.length ?? 0) > 0 ||
    summaryStats.length > 0;

  // While a date-range change is refetching, don't force-collapse a row that was
  // already expanded just because the in-flight data is momentarily empty.
  const canExpand = hasChartData || isLoading;

  return (
    <Accordion
      expanded={canExpand && expanded}
      onChange={canExpand ? () => onToggle() : undefined}
      disableGutters
      elevation={0}
      sx={{
        "&:before": { display: "none" },
        boxShadow: 1,
        overflow: "hidden",
        border: "1px solid",
        borderColor: "divider",
        borderRadius: 0,
        bgcolor: "background.paper",
        "&:hover": canExpand ? { bgcolor: "action.hover" } : undefined,
        "&.Mui-expanded": { margin: 0 },
      }}
    >
      <AccordionSummary
        expandIcon={
          canExpand ? <ChevronDown size={20} color={a.iconColor} /> : null
        }
        sx={{
          px: 2,
          py: 2,
          minHeight: 56,
          bgcolor: a.headerBg,
          borderRadius: 0,
          cursor: canExpand ? "pointer" : "default",
          "&:hover": canExpand ? { bgcolor: a.headerHoverBg } : undefined,
        }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", lg: "row" },
            alignItems: { xs: "stretch", lg: "center" },
            gap: 2,
            width: "100%",
            minWidth: 0,
            pr: 1,
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 2,
              minWidth: { lg: 240 },
            }}
          >
            <Box
              sx={{
                p: 1.5,
                bgcolor: a.iconWellBg,
                display: "flex",
                flexShrink: 0,
              }}
            >
              <Package size={20} color={a.iconColor} />
            </Box>
            <Box sx={{ textAlign: "left", minWidth: 0 }}>
              <Typography variant="body2" sx={{ fontWeight: 700 }}>
                {product.name && product.version
                  ? `${product.name} ${product.version}`
                  : (product.name || product.version)}
              </Typography>
            </Box>
          </Box>

          <Box
            sx={{
              flex: 1,
              minWidth: 0,
              display: "flex",
              flexWrap: "wrap",
              justifyContent: "flex-end",
              alignItems: "flex-start",
              gap: 5,
            }}
          >
            {isLoading ? (
              <CircularProgress size={20} sx={{ color: a.iconColor }} />
            ) : !hasChartData ? (
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ fontStyle: "italic" }}
              >
                {USAGE_METRICS_PRODUCT_DATA_UNAVAILABLE}
              </Typography>
            ) : (
              <>
                {summaryStats.map((stat) => (
                  <Box key={stat.label}>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      display="block"
                      sx={{ mb: 0.5 }}
                    >
                      {stat.label}
                    </Typography>
                    <Typography variant="body1" sx={{ fontWeight: 700 }}>
                      {stat.value}
                    </Typography>
                  </Box>
                ))}
                <CurrMinMaxBlock
                  label={USAGE_METRICS_PRODUCT_INSTANCE_METRICS}
                  summary={instanceSummary}
                />
                <CurrMinMaxBlock
                  label={USAGE_METRICS_PRODUCT_CORE_METRICS}
                  summary={coreSummary}
                />
              </>
            )}
          </Box>
        </Box>
      </AccordionSummary>
      <AccordionDetails
        sx={{
          px: 2,
          py: 2,
          bgcolor: "background.paper",
          borderTop: 1,
          borderColor: "divider",
          borderRadius: 0,
        }}
      >
        {isLoading ? (
          <Skeleton variant="rounded" height={200} />
        ) : latestInstances.length === 0 && chartTrends.every((t) => t.data.length === 0) ? (
          <Typography variant="body2" color="text.secondary">
            {USAGE_METRICS_NO_INSTANCE_DATA}
          </Typography>
        ) : (
          <>
            <Grid
              container
              spacing={2}
              sx={{ width: "100%", minWidth: 0, overflowX: "hidden" }}
            >
              {chartTrends.map((trend) => (
                <Grid
                  key={trend.key}
                  size={{
                    xs: 12,
                    lg: chartTrends.length === 1 ? 12 : chartTrends.length === 3 ? 4 : 6,
                  }}
                >
                  <Card sx={{ p: 2, borderRadius: 0 }}>
                    <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 2 }}>
                      {trend.title}
                    </Typography>
                    <UsageChartSurface minHeight={200}>
                      <LineChart
                        data={trend.data}
                        xAxisDataKey="name"
                        height={200}
                        width="100%"
                        margin={USAGE_LINE_CHART_MARGIN}
                        accessibilityLayer={false}
                        xAxis={{
                          interval: Math.max(0, Math.ceil(trend.data.length / 6) - 1),
                        }}
                        legend={{ show: false }}
                        grid={{ show: true, strokeDasharray: "3 3" }}
                        lines={[
                          {
                            dataKey: "value",
                            name: trend.title,
                            stroke: trend.stroke,
                            strokeWidth: 2.5,
                            dot: false,
                          },
                        ]}
                      />
                    </UsageChartSurface>
                  </Card>
                </Grid>
              ))}
            </Grid>

            {latestInstances.length > 0 && (
              <Box sx={{ mt: 2 }}>
                <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1.5 }}>
                  {USAGE_METRICS_PRODUCT_INSTANCES_SECTION} ({latestInstances.length})
                </Typography>
                <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                  {latestInstances.map((inst) => (
                    <InstanceRow key={inst.id} name={inst.name} cores={inst.cores} />
                  ))}
                </Box>
              </Box>
            )}
          </>
        )}
      </AccordionDetails>
    </Accordion>
  );
}

// ─── Flat instance row ────────────────────────────────────────────────────────

function InstanceRow({ name, cores }: { name: string; cores: number }): JSX.Element {
  const wellBg = alpha(colors.grey?.[500] ?? "#6B7280", 0.04);

  return (
    <Box
      sx={{
        px: 2,
        py: 1.5,
        display: "flex",
        flexDirection: { xs: "column", lg: "row" },
        alignItems: { xs: "stretch", lg: "center" },
        justifyContent: "space-between",
        gap: 2,
        border: "1px solid",
        borderColor: "divider",
        bgcolor: wellBg,
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
        <Server size={18} color={colors.grey?.[500]} />
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          {name}
        </Typography>
      </Box>
      <MetricPill
        icon={<Package size={16} color={colors.grey?.[400]} />}
        label={USAGE_METRICS_PRODUCT_CORE_METRICS}
        value={String(cores)}
      />
    </Box>
  );
}

// ─── Main panel ─────────────────────────────────────────────────────────────────

export default function UsageEnvironmentProductsPanel({
  deploymentId,
  dateRange,
  expandedProductIds,
  onToggleProduct,
}: UsageEnvironmentProductsPanelProps): JSX.Element {
  const {
    data: deployedProducts,
    isLoading,
    isError,
  } = usePostDeploymentProductsSearchAll(deploymentId);

  const products = useMemo((): ProductRowMeta[] => {
    if (!deployedProducts) return [];
    return deployedProducts.map((p) => ({
      id: p.id,
      name: p.product?.label ?? USAGE_METRICS_UNKNOWN_LABEL,
      version:
        typeof p.version === "string" ? p.version : (p.version?.label ?? ""),
    }));
  }, [deployedProducts]);

  if (isError) {
    return (
      <Alert severity="error" sx={{ mt: 1 }}>
        {USAGE_METRICS_ENVIRONMENT_PRODUCTS_ERROR}
      </Alert>
    );
  }

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        {[1, 2, 3].map((i) => (
          <Card
            key={i}
            variant="outlined"
            sx={{ borderRadius: 0, p: 1.5, borderColor: "divider" }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
              <Box
                sx={{
                  flex: 1,
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                }}
              >
                <Box>
                  <Skeleton variant="text" width={180} height={24} />
                  <Skeleton variant="text" width={120} height={20} />
                </Box>
                <Box sx={{ display: { xs: "none", sm: "flex" }, gap: 3 }}>
                  <Skeleton
                    variant="rectangular"
                    width={100}
                    height={32}
                    sx={{ borderRadius: 1 }}
                  />
                  <Skeleton
                    variant="rectangular"
                    width={100}
                    height={32}
                    sx={{ borderRadius: 1 }}
                  />
                </Box>
              </Box>
              <Skeleton variant="circular" width={24} height={24} />
            </Box>
          </Card>
        ))}
      </Box>
    );
  }

  if (products.length === 0) {
    return (
      <Box
        sx={{
          flex: 1,
          minHeight: "60vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <EmptyState description={USAGE_METRICS_NO_PRODUCTS_IN_DEPLOYMENT} />
      </Box>
    );
  }

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 1.5,
        width: "100%",
        maxWidth: "100%",
        minWidth: 0,
        overflowX: "hidden",
      }}
    >
      {products.map((product) => (
        <ProductAccordionRow
          key={product.id}
          product={product}
          deploymentId={deploymentId}
          dateRange={dateRange}
          expanded={expandedProductIds.has(product.id)}
          onToggle={() => onToggleProduct(product.id)}
        />
      ))}
    </Box>
  );
}
